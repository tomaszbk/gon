package cmd

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/gopls/internal/golang"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/internal/diff"
)

// migrateOptionals shares gonpls's parser, checker, package loading and edit
// protocol. The compatibility checker is scoped to this command and has a
// separate cache key; normal checking rejects the retired constructors.
func (r *gonRequest) migrateOptionals(ctx context.Context) (gonResult, error) {
	args := r.args
	if len(args) == 0 {
		args = []string{"."}
	}
	wanted := make(map[protocol.DocumentURI]bool)
	var patterns []string
	for _, arg := range args {
		if strings.HasSuffix(arg, ".go") {
			wanted[protocol.URIFromPath(r.abs(arg))] = true
		} else {
			patterns = append(patterns, arg)
		}
	}
	if len(patterns) > 0 {
		pkgs, _, err := r.goList(ctx, patterns)
		if err != nil {
			return nil, err
		}
		for _, p := range pkgs {
			if p.Error != nil {
				return nil, fmt.Errorf("loading %s: %s", p.ImportPath, p.Error.Err)
			}
			for _, list := range [][]string{p.GoFiles, p.CgoFiles, p.TestGoFiles, p.XTestGoFiles} {
				for _, name := range list {
					wanted[protocol.URIFromPath(filepath.Join(p.Dir, name))] = true
				}
			}
		}
	}
	plan := &gonPlan{Status: "planned", Files: []*gonPlanFile{}, old: make(map[string][]byte)}
	plan.gonEnvelope = *r.envelope("refactor.optionals")
	for uri := range wanted {
		snapshot, err := r.snapshot(ctx, uri)
		if err != nil {
			return nil, err
		}
		pkg, _, err := golang.NarrowestPackageForFile(ctx, snapshot, uri)
		if err != nil {
			return nil, err
		}
		if len(pkg.ParseErrors()) > 0 || len(pkg.TypeErrors()) > 0 {
			return nil, gonErrorf(gonKindRejected, gonExitFindings, "%s: migration requires valid source under the retired optional contract: %v", uri.Path(), pkg.TypeErrors())
		}
		var file *ast.File
		for _, f := range pkg.Syntax() {
			if pkg.FileSet().Position(f.Pos()).Filename == uri.Path() {
				file = f
				break
			}
		}
		if file == nil {
			return nil, fmt.Errorf("%s: no checked syntax", uri.Path())
		}
		d, err := r.doc(ctx, uri)
		if err != nil {
			return nil, err
		}
		edits, err := gonOptionalEdits(pkg.FileSet(), file, pkg.TypesInfo(), pkg.Types(), d.content)
		if err != nil {
			return nil, gonErrorf(gonKindRejected, gonExitFindings, "%s: %v", uri.Path(), err)
		}
		if len(edits) == 0 {
			continue
		}
		pf := &gonPlanFile{Path: uri.Path(), SHA256: d.hash, Formatting: "unchanged"}
		diff.SortEdits(edits)
		for _, e := range edits {
			pf.Edits = append(pf.Edits, gonEdit{Location: d.offsets(e.Start, e.End), NewText: e.New})
		}
		updated, err := gonApplyEdits(d.content, pf.Edits)
		if err != nil {
			return nil, err
		}
		pf.NewSHA256 = gonHash(updated)
		plan.old[pf.Path] = d.content
		plan.Files = append(plan.Files, pf)
		plan.Summary.Edits += len(pf.Edits)
	}
	sort.Slice(plan.Files, func(i, j int) bool { return plan.Files[i].Path < plan.Files[j].Path })
	plan.Summary.Files = len(plan.Files)
	if !r.bool("dry-run") {
		if err := gonApplyPlan(plan); err != nil {
			return nil, err
		}
		plan.Status = "applied"
	}
	return plan, nil
}

// gonOptionalEdits rewrites only nodes resolved as native optional identities
// by the migration checker. User-declared Option/Some/None names are untouched.
func gonOptionalEdits(fset *token.FileSet, file *ast.File, info *types.Info, pkg *types.Package, src []byte) ([]diff.Edit, error) {
	tf := fset.File(file.Pos())
	aliases := make(map[string]string)
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		if imp.Name != nil {
			aliases[path] = imp.Name.Name
		} else {
			for _, p := range pkg.Imports() {
				if p.Path() == path {
					aliases[path] = p.Name()
				}
			}
		}
	}
	qual := func(p *types.Package) string {
		if p == pkg {
			return ""
		}
		if aliases[p.Path()] == "." {
			return ""
		}
		return aliases[p.Path()]
	}
	typeText := func(t types.Type) (string, error) {
		bad := false
		gonWalkOptionalTypes(t, func(t types.Type) bool { // checked below without guessing inaccessible names
			if n, ok := types.Unalias(t).(*types.Named); ok && n.Obj().Pkg() != nil && n.Obj().Pkg() != pkg {
				if !n.Obj().Exported() || aliases[n.Obj().Pkg().Path()] == "" {
					bad = true
				}
			}
			return true
		})
		if bad {
			return "", fmt.Errorf("migration cannot name payload type %s at this file; write an explicit adapter", t)
		}
		return types.TypeString(t, qual), nil
	}
	var rewrite func(ast.Node) (string, error)
	var editsFor func(ast.Node) ([]diff.Edit, error)
	textOf := func(n ast.Node) string { return string(src[tf.Offset(n.Pos()):tf.Offset(n.End())]) }
	rewrite = func(n ast.Node) (string, error) {
		if p, ok := n.(*ast.MatchPattern); ok {
			if sel, ok := p.Value.(*ast.SelectorExpr); ok && types.IsOptional(info.TypeOf(sel)) {
				switch sel.Sel.Name {
				case "None":
					return "nil", nil
				case "Some":
					if len(p.Args) == 1 {
						s, err := rewrite(p.Args[0])
						if strings.HasSuffix(s, "?") {
							s = "(" + s + ")"
						}
						return s + "?", err
					}
				}
			}
		}
		if call, ok := n.(*ast.CallExpr); ok && len(call.Args) == 1 {
			var target types.Type
			switch head := ast.Unparen(call.Fun).(type) {
			case *ast.SelectorExpr:
				if head.Sel.Name == "Some" && info.Types[head.X].IsType() {
					target = info.TypeOf(head.X)
				}
			case *ast.ContextualVariantExpr:
				_ = head // contextual constructors use their own node
			}
			if o := types.OptionalOf(target); o != nil {
				dst, err := typeText(target)
				if err != nil {
					return "", err
				}
				payload, err := typeText(o.Elem())
				if err != nil {
					return "", err
				}
				arg, err := rewrite(call.Args[0])
				return "(" + dst + ")((" + payload + ")(" + arg + "))", err
			}
		}
		if e, ok := n.(*ast.ContextualVariantExpr); ok {
			if o := types.OptionalOf(info.TypeOf(e)); o != nil {
				dst, err := typeText(info.TypeOf(e))
				if err != nil {
					return "", err
				}
				if e.Name.Name == "None" {
					return "(" + dst + ")(nil)", nil
				}
				if e.Name.Name == "Some" && len(e.Args) == 1 {
					p, err := typeText(o.Elem())
					if err != nil {
						return "", err
					}
					arg, err := rewrite(e.Args[0])
					return "(" + dst + ")((" + p + ")(" + arg + "))", err
				}
			}
		}
		if e, ok := n.(ast.Expr); ok {
			if sel, ok := e.(*ast.SelectorExpr); ok && info.Types[sel.X].IsType() {
				if o := types.OptionalOf(info.TypeOf(sel.X)); o != nil {
					dst, err := typeText(info.TypeOf(sel.X))
					if err != nil {
						return "", err
					}
					switch sel.Sel.Name {
					case "None":
						return "(" + dst + ")(nil)", nil
					case "Some":
						p, err := typeText(o.Elem())
						return "func(value " + p + ") " + dst + " { return value }", err
					}
				}
			}
			var base ast.Expr
			switch e := e.(type) {
			case *ast.IndexExpr:
				base = e.X
			case *ast.IndexListExpr:
				base = e.X
			}
			if id, ok := base.(*ast.Ident); ok {
				obj := info.Uses[id]
				if obj != nil && obj.Name() == "Option" && obj.Pkg() == nil && obj.Parent() == types.Universe && types.IsOptional(info.TypeOf(e)) {
					return typeText(info.TypeOf(e))
				}
			}
		}
		children, err := editsFor(n)
		if err != nil {
			return "", err
		}
		// Child edits refer to original file offsets. Render them relative to n.
		s := textOf(n)
		sort.Slice(children, func(i, j int) bool { return children[i].Start > children[j].Start })
		start := tf.Offset(n.Pos())
		for _, e := range children {
			s = s[:e.Start-start] + e.New + s[e.End-start:]
		}
		return s, nil
	}
	editsFor = func(n ast.Node) ([]diff.Edit, error) {
		var edits []diff.Edit
		for c := range ast.Children(n) {
			replacement, err := rewrite(c)
			if err != nil {
				return nil, err
			}
			if replacement != textOf(c) {
				edits = append(edits, diff.Edit{Start: tf.Offset(c.Pos()), End: tf.Offset(c.End()), New: replacement})
			}
		}
		return edits, nil
	}
	edits, err := editsFor(file)
	if err != nil || len(edits) == 0 {
		return edits, err
	}
	updated, err := diff.Apply(string(src), edits)
	if err != nil {
		return nil, err
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), "migrated.go", updated, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("cannot represent migration in current syntax: %v", err)
	}
	comments := func(f *ast.File) map[string]int {
		counts := make(map[string]int)
		for _, group := range f.Comments {
			for _, comment := range group.List {
				counts[comment.Text]++
			}
		}
		return counts
	}
	before, after := comments(file), comments(parsed)
	for text, count := range before {
		if after[text] != count {
			return nil, fmt.Errorf("migration would remove an embedded comment; move the comment or migrate this declaration manually")
		}
	}
	return edits, nil
}

// Walk only type constructors that appear in a spelling, without following a
// named type's underlying representation or methods.
func gonWalkOptionalTypes(t types.Type, visit func(types.Type) bool) {
	if !visit(t) {
		return
	}
	switch t := t.(type) {
	case *types.Alias:
		gonWalkOptionalTypes(types.Unalias(t), visit)
	case *types.Named:
		for i := 0; i < t.TypeArgs().Len(); i++ {
			gonWalkOptionalTypes(t.TypeArgs().At(i), visit)
		}
	case *types.Optional:
		gonWalkOptionalTypes(t.Elem(), visit)
	case *types.Pointer:
		gonWalkOptionalTypes(t.Elem(), visit)
	case *types.Slice:
		gonWalkOptionalTypes(t.Elem(), visit)
	case *types.Array:
		gonWalkOptionalTypes(t.Elem(), visit)
	case *types.Chan:
		gonWalkOptionalTypes(t.Elem(), visit)
	case *types.Map:
		gonWalkOptionalTypes(t.Key(), visit)
		gonWalkOptionalTypes(t.Elem(), visit)
	case *types.Signature:
		gonWalkOptionalTypes(t.Params(), visit)
		gonWalkOptionalTypes(t.Results(), visit)
	case *types.Tuple:
		for i := 0; i < t.Len(); i++ {
			gonWalkOptionalTypes(t.At(i).Type(), visit)
		}
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			gonWalkOptionalTypes(t.Field(i).Type(), visit)
		}
	case *types.Interface:
		for i := 0; i < t.NumExplicitMethods(); i++ {
			gonWalkOptionalTypes(t.ExplicitMethod(i).Type(), visit)
		}
		for i := 0; i < t.NumEmbeddeds(); i++ {
			gonWalkOptionalTypes(t.EmbeddedType(i), visit)
		}
	}
}
