package cmd

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"sort"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/gopls/internal/golang"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/internal/typesinternal"
)

// gonQueryResult is the result of every "gon query" subcommand.
type gonQueryResult struct {
	gonEnvelope
	Results  []*gonQueryItem `json:"results"`
	Revision gonRevision     `json:"revision"`
}

// gonQueryItem holds the answer for one target or search query.
type gonQueryItem struct {
	Target *gonTarget     `json:"target,omitempty"`
	Query  string         `json:"query,omitempty"`
	Error  *gonErrorInfo  `json:"error,omitempty"`
	Total  int            `json:"total"`
	Items  []*gonLocated  `json:"items"`
	Type   *gonTypeInfo   `json:"type,omitempty"`
	More   *gonPagination `json:"truncated,omitempty"`
	exit   int
}

// gonLocated is a location with its source line and optional details.
type gonLocated struct {
	Location    gonLocation    `json:"location"`
	Text        string         `json:"text"`
	Declaration bool           `json:"declaration,omitempty"`
	Object      *gonObjectInfo `json:"object,omitempty"`
	Hover       string         `json:"hover,omitempty"`
	Container   string         `json:"container,omitempty"`
}

// gonPagination reports omitted items and how to retrieve them.
type gonPagination struct {
	Omitted    int `json:"omitted"`
	NextOffset int `json:"nextOffset"`
}

// gonTypeInfo describes the innermost typed expression at a position.
type gonTypeInfo struct {
	Expression string         `json:"expression"`
	Type       string         `json:"type"`
	Underlying string         `json:"underlying,omitempty"`
	Mode       string         `json:"mode"`
	Value      string         `json:"value,omitempty"`
	Location   gonLocation    `json:"location"`
	Construct  string         `json:"construct,omitempty"` // Gon language construct, if any
	Object     *gonObjectInfo `json:"object,omitempty"`
}

func (q *gonQueryResult) exitCode() int {
	code := gonExitOK
	for _, item := range q.Results {
		code = max(code, item.exit)
	}
	return code
}

func (r *gonRequest) query(ctx context.Context, kind string) (gonResult, error) {
	if _, err := r.ensureEngine(ctx); err != nil {
		return nil, err
	}
	result := &gonQueryResult{Revision: make(gonRevision)}
	for _, arg := range r.args {
		item := &gonQueryItem{}
		var err error
		if kind == "symbols" {
			item.Query = arg
			err = r.querySymbols(ctx, item, arg, result.Revision)
		} else {
			err = r.queryTarget(ctx, kind, item, arg, result.Revision)
		}
		if err != nil {
			ge := asGonError(err)
			if ge.exit == gonExitInfra {
				return nil, err
			}
			if item.Target == nil && item.Query == "" {
				item.Target = &gonTarget{Spec: arg}
			}
			item.Error = &gonErrorInfo{Kind: ge.kind, Message: ge.msg}
			item.exit = ge.exit
			item.Items = []*gonLocated{}
		}
		result.Results = append(result.Results, item)
	}
	result.gonEnvelope = *r.envelope(r.operation())
	return result, nil
}

func (r *gonRequest) queryTarget(ctx context.Context, kind string, item *gonQueryItem, spec string, rev gonRevision) error {
	t, err := r.resolveTarget(ctx, spec, rev)
	if err != nil {
		return err
	}
	item.Target = t
	server := r.engine.cli.server
	pos := protocol.LocationTextDocumentPositionParams(t.loc)
	var locs []protocol.Location
	switch kind {
	case "def":
		if t.Kind == "symbol" {
			locs = []protocol.Location{t.loc}
		} else if locs, err = server.Definition(ctx, &protocol.DefinitionParams{TextDocumentPositionParams: pos}); err != nil {
			return err
		}
		if len(locs) == 0 {
			return gonErrorf(gonKindNotFound, gonExitFindings, "%s: no declaration (not an identifier?)", spec)
		}
	case "refs":
		locs, err = server.References(ctx, &protocol.ReferenceParams{
			TextDocumentPositionParams: pos,
			Context:                    protocol.ReferenceContext{IncludeDeclaration: true},
		})
		if err != nil {
			return err
		}
	case "impls":
		if locs, err = server.Implementation(ctx, &protocol.ImplementationParams{TextDocumentPositionParams: pos}); err != nil {
			return err
		}
	case "type":
		return r.queryType(ctx, item, t, rev)
	}

	// The declaration is reported by "def" and marked in "refs".
	var decl protocol.Location
	if kind != "impls" {
		if defs, err := server.Definition(ctx, &protocol.DefinitionParams{TextDocumentPositionParams: pos}); err == nil && len(defs) > 0 {
			decl = defs[0]
		} else if t.Kind == "symbol" {
			decl = t.loc
		}
	}
	seen := make(map[protocol.Location]bool)
	var items []*gonLocated
	for _, loc := range locs {
		if seen[loc] {
			continue
		}
		seen[loc] = true
		l, d, err := r.locate(ctx, loc, rev)
		if err != nil {
			return err
		}
		items = append(items, &gonLocated{Location: l, Text: d.line(l.Line), Declaration: loc == decl})
	}
	sort.Slice(items, func(i, j int) bool { return gonLess(items[i].Location, items[j].Location) })
	if kind == "def" {
		for i, it := range items {
			// Describe the object declared at each definition, which is
			// exact even when the position was not on an identifier.
			it.Object = t.Object
			if obj := r.declaredAt(ctx, locs[min(i, len(locs)-1)]); obj != nil {
				it.Object = obj
			}
			if h, err := server.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: protocol.LocationTextDocumentPositionParams(locs[min(i, len(locs)-1)])}); err == nil && h != nil {
				it.Hover = strings.TrimSpace(h.Contents.Value)
			}
		}
	}
	item.Total, item.Items, item.More = r.paginate(items)
	return nil
}

// declaredAt describes the object declared by the identifier at loc.
func (r *gonRequest) declaredAt(ctx context.Context, loc protocol.Location) *gonObjectInfo {
	snapshot, err := r.snapshot(ctx, loc.URI)
	if err != nil {
		return nil
	}
	pkg, pgf, err := golang.NarrowestPackageForFile(ctx, snapshot, loc.URI)
	if err != nil {
		return nil
	}
	pos, err := pgf.PositionPos(loc.Range.Start)
	if err != nil {
		return nil
	}
	path, _ := astutil.PathEnclosingInterval(pgf.File, pos, pos)
	if len(path) == 0 {
		return nil
	}
	if id, ok := path[0].(*ast.Ident); ok {
		if obj := pkg.TypesInfo().ObjectOf(id); obj != nil {
			return gonDescribe(obj, pkg.Types())
		}
	}
	return nil
}

func gonLess(a, b gonLocation) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	return a.Offset < b.Offset
}

// paginate applies --offset and --limit.
func (r *gonRequest) paginate(items []*gonLocated) (int, []*gonLocated, *gonPagination) {
	total := len(items)
	offset, limit := min(r.int("offset"), total), r.int("limit")
	items = items[offset:]
	var more *gonPagination
	if limit > 0 && len(items) > limit {
		more = &gonPagination{Omitted: len(items) - limit, NextOffset: offset + limit}
		items = items[:limit]
	}
	if items == nil {
		items = []*gonLocated{}
	}
	return total, items, more
}

func (r *gonRequest) queryType(ctx context.Context, item *gonQueryItem, t *gonTarget, rev gonRevision) error {
	if t.Kind != "position" {
		return gonErrorf(gonKindUsage, gonExitUsage, "query type needs a position target, not a symbol")
	}
	info := t.pkg.TypesInfo()
	qual := typesinternal.FileQualifier(t.pgf.File, t.pkg.Types())
	d, err := r.doc(ctx, t.loc.URI)
	if err != nil {
		return err
	}
	for _, n := range t.path {
		expr, ok := n.(ast.Expr)
		if !ok {
			continue
		}
		tv, ok := info.Types[expr]
		typ := tv.Type
		var obj types.Object
		if id, isIdent := expr.(*ast.Ident); isIdent {
			obj = info.ObjectOf(id)
			if typ == nil && obj != nil {
				typ = obj.Type()
			}
		}
		if typ == nil {
			continue
		}
		start, end, err := t.pgf.NodeOffsets(expr)
		if err != nil {
			return err
		}
		ti := &gonTypeInfo{
			Expression: gonTruncate(string(t.pgf.Src[start:end]), 200),
			Type:       types.TypeString(typ, qual),
			Location:   d.offsets(start, end),
			Mode:       gonMode(tv, ok, obj),
		}
		if u := types.TypeString(typ.Underlying(), qual); u != ti.Type {
			ti.Underlying = u
		}
		if tv.Value != nil {
			ti.Value = tv.Value.ExactString()
		}
		switch e := expr.(type) {
		case *ast.EnumType:
			ti.Construct = "enum-type"
		case *ast.MatchExpr:
			ti.Construct = "match-expression"
		case *ast.OptionalExpr:
			ti.Construct = "optional-propagation"
			if tv.IsType() {
				ti.Construct = "optional-type"
			}
		case *ast.ContextualVariantExpr:
			ti.Construct = "contextual-constructor"
		case *ast.LambdaExpr:
			ti.Construct = "lambda"
		case *ast.NilGuardExpr:
			ti.Construct = "nil-guard"
			if types.IsOptional(info.TypeOf(e.X)) {
				ti.Construct = "option-guard"
			}
		case *ast.SafeNavExpr:
			ti.Construct = "safe-navigation"
		case *ast.BinaryExpr:
			if e.Op == token.COALESCE {
				ti.Construct = "nil-coalescing"
				if types.IsOptional(info.TypeOf(e.X)) {
					ti.Construct = "optional-coalescing"
				}
			}
		case *ast.CondExpr:
			ti.Construct = "conditional-expression"
		case *ast.ErrorExpr:
			ti.Construct = "error-propagation"
			if e.Body != nil {
				ti.Construct = "error-handler"
			}
			if types.IsCanonicalResult(info.TypeOf(e.X)) {
				if e.Body != nil {
					ti.Construct = "result-handler"
				} else {
					ti.Construct = "result-propagation"
				}
			}
			if tup, isTuple := typ.(*types.Tuple); isTuple && tup.Len() == 0 {
				ti.Type = "(no value)"
			}
		}
		if obj != nil {
			ti.Object = gonDescribe(obj, t.pkg.Types())
		}
		item.Type = ti
		item.Total = 1
		item.Items = []*gonLocated{}
		rev.add(d)
		return nil
	}
	return gonErrorf(gonKindNotFound, gonExitFindings, "%s: no typed expression at this position", t.Spec)
}

func gonMode(tv types.TypeAndValue, recorded bool, obj types.Object) string {
	switch {
	case recorded && tv.IsType():
		return "type"
	case recorded && tv.Value != nil:
		return "constant"
	case recorded && tv.IsVoid():
		return "no value"
	case recorded && tv.IsBuiltin():
		return "builtin"
	case recorded:
		return "value"
	case obj != nil:
		return "declaration"
	}
	return "value"
}

func gonTruncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func (r *gonRequest) querySymbols(ctx context.Context, item *gonQueryItem, query string, rev gonRevision) error {
	syms, err := r.engine.cli.server.Symbol(ctx, &protocol.WorkspaceSymbolParams{Query: query})
	if err != nil {
		return err
	}
	var items []*gonLocated
	for _, s := range syms {
		l, d, err := r.locate(ctx, s.Location, rev)
		if err != nil {
			return err
		}
		items = append(items, &gonLocated{
			Location: l, Text: d.line(l.Line), Container: s.ContainerName,
			Object: &gonObjectInfo{Name: s.Name, Kind: gonSymbolKind(s.Kind)},
		})
	}
	// Keep the server's relevance order; it is deterministic for a revision.
	item.Total, item.Items, item.More = r.paginate(items)
	return nil
}

func (q *gonQueryResult) text(w io.Writer, r *gonRequest) {
	for i, item := range q.Results {
		if len(q.Results) > 1 {
			if i > 0 {
				fmt.Fprintln(w)
			}
			label := item.Query
			if item.Target != nil {
				label = item.Target.Spec
			}
			fmt.Fprintf(w, "== %s ==\n", label)
		}
		if item.Error != nil {
			if len(q.Results) == 1 {
				fmt.Fprintf(r.inv.stderr, "gon %s: %s\n", r.cmd.path, item.Error.Message)
			} else {
				fmt.Fprintf(w, "error: %s\n", item.Error.Message)
			}
			continue
		}
		if ti := item.Type; ti != nil {
			expr, _, multiline := strings.Cut(ti.Expression, "\n")
			if multiline {
				expr += " ..."
			}
			fmt.Fprintf(w, "%s:%d:%d: %s: %s", r.rel(ti.Location.Path), ti.Location.Line, ti.Location.Column, expr, ti.Type)
			if ti.Construct != "" {
				fmt.Fprintf(w, " (%s)", ti.Construct)
			}
			if ti.Value != "" {
				fmt.Fprintf(w, " = %s", ti.Value)
			}
			fmt.Fprintln(w)
			if ti.Object != nil {
				fmt.Fprintf(w, "\t%s\n", ti.Object.Signature)
			}
			continue
		}
		for _, it := range item.Items {
			text := it.Text
			if it.Object != nil && r.cmd.path == "query def" {
				text = it.Object.Signature
			} else if it.Object != nil && it.Container != "" {
				text = fmt.Sprintf("%s %s", it.Object.Kind, it.Object.Name)
			}
			fmt.Fprintf(w, "%s:%d:%d: %s\n", r.rel(it.Location.Path), it.Location.Line, it.Location.Column, text)
			if r.cmd.path == "query def" && r.bool("doc") && it.Hover != "" {
				for _, line := range strings.Split(it.Hover, "\n") {
					fmt.Fprintf(w, "\t%s\n", line)
				}
			}
		}
		if item.More != nil {
			fmt.Fprintf(w, "... %d more; rerun with --offset %d\n", item.More.Omitted, item.More.NextOffset)
		}
		if len(item.Items) == 0 {
			fmt.Fprintln(w, "no results")
		}
	}
}

func gonSymbolKind(k protocol.SymbolKind) string {
	names := []string{"file", "module", "namespace", "package", "class", "method", "property", "field",
		"constructor", "enum", "interface", "function", "variable", "constant", "string", "number", "boolean",
		"array", "object", "key", "null", "enum member", "struct", "event", "operator", "type parameter"}
	if k >= 1 && int(k) <= len(names) {
		return names[k-1]
	}
	return "symbol"
}
