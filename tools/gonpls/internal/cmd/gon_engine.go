package cmd

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/cache/metadata"
	"golang.org/x/tools/gopls/internal/cache/parsego"
	"golang.org/x/tools/gopls/internal/golang"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/server"
	"golang.org/x/tools/gopls/internal/settings"
)

// gonEngineConfig selects the analyzed build configuration.
type gonEngineConfig struct {
	Root             string // workspace root: nearest go.work or go.mod directory
	Tags             string
	Staticcheck      bool
	MigrateOptionals bool
}

// A gonEngine is an in-process gonpls session with a command-line client.
// Files are never opened as editor overlays: every request analyzes saved
// files, and unsaved editor buffers cannot leak into results.
type gonEngine struct {
	cfg  gonEngineConfig
	cli  *client
	sess *cache.Session
}

func newGonEngine(ctx context.Context, cfg gonEngineConfig) (*gonEngine, error) {
	app := New()
	app.options = func(o *settings.Options) {
		o.PreferredContentFormat = protocol.PlainText
		o.SymbolStyle = settings.FullyQualifiedSymbols
		o.SymbolScope = settings.WorkspaceSymbolScope
		o.RelatedInformationSupported = true
		o.OnDemandDiagnostics = true
		o.MigrateOptionals = cfg.MigrateOptionals
		if cfg.Tags != "" {
			o.BuildFlags = append(o.BuildFlags, "-tags="+cfg.Tags)
		}
		if cfg.Staticcheck {
			o.Staticcheck, o.StaticcheckProvided = true, true
		}
	}
	options := settings.DefaultOptions(app.options)
	cli := newClient(app)
	sess := cache.NewSession(ctx, cache.New(nil))
	svr := server.New(sess, cli, options)
	if err := cli.initialize(ctx, svr, initParams(cfg.Root, options)); err != nil {
		sess.Shutdown(ctx)
		return nil, gonErrorf(gonKindWorkspace, gonExitInfra, "starting gonpls: %v", err)
	}
	return &gonEngine{cfg: cfg, cli: cli, sess: sess}, nil
}

func (e *gonEngine) close(ctx context.Context) { e.cli.terminate(ctx) }

// gonWorkspaceRoot returns the directory that identifies the workspace of dir.
func gonWorkspaceRoot(dir string, env []string) string {
	gowork := gonGetenv(env, "GOWORK")
	if gowork != "" && gowork != "off" && filepath.IsAbs(gowork) {
		if resolved, err := filepath.EvalSymlinks(gowork); err == nil {
			gowork = resolved
		}
		return filepath.Dir(gowork)
	}
	names := []string{"go.work", "go.mod"}
	if gowork == "off" {
		names = names[1:]
	}
	for _, name := range names {
		for d := dir; ; d = filepath.Dir(d) {
			if st, err := os.Stat(filepath.Join(d, name)); err == nil && !st.IsDir() {
				return d
			}
			if filepath.Dir(d) == d {
				break
			}
		}
	}
	return dir
}

func (r *gonRequest) engineConfig() gonEngineConfig {
	return gonEngineConfig{
		Root:             gonWorkspaceRoot(r.inv.cwd, r.inv.env),
		Tags:             r.str("tags"),
		Staticcheck:      r.bool("staticcheck"),
		MigrateOptionals: r.cmd.path == "refactor optionals",
	}
}

// ensureEngine starts the session of this request, once.
func (r *gonRequest) ensureEngine(ctx context.Context) (*gonEngine, error) {
	if r.engine != nil {
		return r.engine, nil
	}
	e, err := newGonEngine(ctx, r.engineConfig())
	if err != nil {
		return nil, err
	}
	r.engine = e
	r.holds = append(r.holds, func() { e.close(context.WithoutCancel(ctx)) })
	return e, nil
}

// release frees request resources in reverse order of acquisition.
func (r *gonRequest) release() {
	for i := len(r.holds) - 1; i >= 0; i-- {
		r.holds[i]()
	}
	r.holds = nil
}

func (r *gonRequest) configuration() *gonConfigInfo {
	cfg := r.engineConfig()
	info := &gonConfigInfo{Workspace: cfg.Root, Staticcheck: cfg.Staticcheck}
	if cfg.Tags != "" {
		info.BuildFlags = []string{"-tags=" + cfg.Tags}
	}
	if r.engine != nil {
		for _, v := range r.engine.sess.Views() {
			env := v.Folder().Env
			info.GOOS, info.GOARCH, info.GOFLAGS = env.GOOS, env.GOARCH, env.GOFLAGS
			break
		}
	}
	if info.GOOS == "" {
		info.GOOS, info.GOARCH = gonGetenv(r.inv.env, "GOOS"), gonGetenv(r.inv.env, "GOARCH")
		info.GOFLAGS = gonGetenv(r.inv.env, "GOFLAGS")
	}
	return info
}

// -- documents, locations and revisions --

// A gonDoc is the content of a file exactly as the analysis saw it.
type gonDoc struct {
	uri     protocol.DocumentURI
	content []byte
	mapper  *protocol.Mapper
	hash    string // SHA-256 of content, in hex
}

// doc returns the analyzed content of a file. Positions, line text and
// revisions are all derived from it, never from a separate disk read.
func (r *gonRequest) doc(ctx context.Context, uri protocol.DocumentURI) (*gonDoc, error) {
	if d := r.docs[string(uri)]; d != nil {
		return d, nil
	}
	e, err := r.ensureEngine(ctx)
	if err != nil {
		return nil, err
	}
	fh, _, release, err := e.sess.FileOf(ctx, uri)
	if err != nil {
		return nil, gonErrorf(gonKindWorkspace, gonExitInfra, "%s: %v", uri.Path(), err)
	}
	defer release()
	content, err := fh.Content()
	if err != nil {
		return nil, gonErrorf(gonKindNotFound, gonExitFindings, "%s: %v", uri.Path(), err)
	}
	d := &gonDoc{uri: uri, content: content, mapper: protocol.NewMapper(uri, content), hash: fh.Identity().Hash.String()}
	if r.docs == nil {
		r.docs = make(map[string]*gonDoc)
	}
	r.docs[string(uri)] = d
	return d, nil
}

// gonLocation is a source range. Lines and columns are 1-based; columns count
// UTF-8 bytes, as in compiler diagnostics. Offsets are 0-based bytes.
type gonLocation struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndLine   int    `json:"endLine"`
	EndColumn int    `json:"endColumn"`
	Offset    int    `json:"offset"`
	EndOffset int    `json:"endOffset"`
}

func (l gonLocation) String() string { return fmt.Sprintf("%s:%d:%d", l.Path, l.Line, l.Column) }

func (d *gonDoc) offsets(start, end int) gonLocation {
	sl, sc := d.mapper.OffsetLineCol8(start)
	el, ec := d.mapper.OffsetLineCol8(end)
	return gonLocation{Path: d.uri.Path(), Line: sl, Column: sc, EndLine: el, EndColumn: ec, Offset: start, EndOffset: end}
}

func (d *gonDoc) location(rng protocol.Range) (gonLocation, error) {
	start, end, err := d.mapper.RangeOffsets(rng)
	if err != nil {
		return gonLocation{}, err
	}
	return d.offsets(start, end), nil
}

// line returns the trimmed text of a 1-based line.
func (d *gonDoc) line(n int) string {
	lines := strings.SplitAfterN(string(d.content), "\n", n+1)
	if n < 1 || n > len(lines) {
		return ""
	}
	text := strings.TrimSpace(lines[n-1])
	if len(text) > 200 {
		text = text[:200] + "..."
	}
	return text
}

// locate converts a protocol location, recording the analyzed revision.
func (r *gonRequest) locate(ctx context.Context, loc protocol.Location, rev gonRevision) (gonLocation, *gonDoc, error) {
	d, err := r.doc(ctx, loc.URI)
	if err != nil {
		return gonLocation{}, nil, err
	}
	l, err := d.location(loc.Range)
	if err != nil {
		return gonLocation{}, nil, err
	}
	rev.add(d)
	return l, d, nil
}

// gonRevision maps each file used by a result to the SHA-256 of the content
// that was analyzed. A later change to any of these files invalidates the
// result.
type gonRevision map[string]string

func (rev gonRevision) add(d *gonDoc) {
	if rev != nil {
		rev[d.uri.Path()] = d.hash
	}
}

// -- targets --

// A gonTarget is a resolved command-line target: a position or a symbol.
type gonTarget struct {
	Spec     string         `json:"spec"`
	Kind     string         `json:"kind"` // "position" or "symbol"
	Location *gonLocation   `json:"location,omitempty"`
	Object   *gonObjectInfo `json:"object,omitempty"`
	loc      protocol.Location
	obj      types.Object
	pkg      *cache.Package
	pgf      *parsego.File
	path     []ast.Node // for positions: enclosing syntax, innermost first
}

// gonObjectInfo describes a declared entity.
type gonObjectInfo struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Package   string `json:"package,omitempty"`
	Signature string `json:"signature"`
}

var (
	gonPositionRE = regexp.MustCompile(`^(.+\.go):(?:(\d+):(\d+)|#(\d+))$`)
	gonFileLineRE = regexp.MustCompile(`\.go:\d+$`) // a position without a column
)

func (r *gonRequest) resolveTarget(ctx context.Context, spec string, rev gonRevision) (*gonTarget, error) {
	if m := gonPositionRE.FindStringSubmatch(spec); m != nil {
		return r.resolvePosition(ctx, spec, m, rev)
	}
	if strings.HasSuffix(spec, ".go") || gonFileLineRE.MatchString(spec) {
		return nil, gonErrorf(gonKindUsage, gonExitUsage, "invalid position %q: use file.go:line:column or file.go:#offset", spec)
	}
	return r.resolveSymbol(ctx, spec, rev)
}

func (r *gonRequest) resolvePosition(ctx context.Context, spec string, m []string, rev gonRevision) (*gonTarget, error) {
	path := r.abs(m[1])
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		return nil, gonErrorf(gonKindNotFound, gonExitFindings, "%s: no such file", m[1])
	}
	uri := protocol.URIFromPath(path)
	d, err := r.doc(ctx, uri)
	if err != nil {
		return nil, err
	}
	var offset int
	if m[4] != "" {
		offset, _ = strconv.Atoi(m[4])
		if offset > len(d.content) {
			return nil, gonErrorf(gonKindNotFound, gonExitFindings, "%s: offset beyond end of file", spec)
		}
	} else {
		line, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		pos, err := d.mapper.LineCol8Position(line, col)
		if err != nil {
			return nil, gonErrorf(gonKindNotFound, gonExitFindings, "%s: %v", spec, err)
		}
		if offset, err = d.mapper.PositionOffset(pos); err != nil {
			return nil, gonErrorf(gonKindNotFound, gonExitFindings, "%s: %v", spec, err)
		}
	}
	p, err := d.mapper.OffsetPosition(offset)
	if err != nil {
		return nil, gonErrorf(gonKindNotFound, gonExitFindings, "%s: %v", spec, err)
	}
	t := &gonTarget{Spec: spec, Kind: "position", loc: protocol.Location{URI: uri, Range: protocol.Range{Start: p, End: p}}}
	l := d.offsets(offset, offset)
	t.Location = &l
	rev.add(d)

	// Find the enclosing syntax and the object of an identifier at the
	// position, using the package gonpls type-checked for the file.
	snapshot, err := r.snapshot(ctx, uri)
	if err != nil {
		return nil, err
	}
	pkg, pgf, err := golang.NarrowestPackageForFile(ctx, snapshot, uri)
	if err != nil {
		return nil, gonErrorf(gonKindWorkspace, gonExitInfra, "%s: %v", m[1], err)
	}
	pos, err := pgf.PositionPos(p)
	if err != nil {
		return nil, gonErrorf(gonKindNotFound, gonExitFindings, "%s: %v", spec, err)
	}
	t.pkg, t.pgf = pkg, pgf
	t.path, _ = astutil.PathEnclosingInterval(pgf.File, pos, pos)
	if len(t.path) > 0 {
		if id, ok := t.path[0].(*ast.Ident); ok {
			if obj := pkg.TypesInfo().ObjectOf(id); obj != nil {
				t.obj = obj
				t.Object = gonDescribe(obj, pkg.Types())
			}
		}
	}
	return t, nil
}

// snapshot returns a snapshot for uri that stays valid until the request ends.
func (r *gonRequest) snapshot(ctx context.Context, uri protocol.DocumentURI) (*cache.Snapshot, error) {
	e, err := r.ensureEngine(ctx)
	if err != nil {
		return nil, err
	}
	snapshot, release, err := e.sess.SnapshotOf(ctx, uri)
	if err != nil {
		return nil, gonErrorf(gonKindWorkspace, gonExitInfra, "loading workspace for %s: %v", uri.Path(), err)
	}
	r.holds = append(r.holds, release)
	return snapshot, nil
}

// gonSymbolCandidate is one interpretation of a symbol target.
type gonSymbolCandidate struct {
	pkg   string   // "" for the package in the working directory
	names []string // object name, optionally followed by a field or method
}

// gonSymbolCandidates splits a symbol target. Package paths may contain
// dots in their final element (gopkg.in/yaml.v3.Node), so every split that
// leaves one or two identifiers is a candidate; the longest package wins.
func gonSymbolCandidates(spec string) []gonSymbolCandidate {
	slash := strings.LastIndex(spec, "/")
	head, tail := spec[:slash+1], spec[slash+1:]
	parts := strings.Split(tail, ".")
	valid := func(names []string) bool {
		for _, n := range names {
			if !token.IsIdentifier(n) {
				return false
			}
		}
		return len(names) > 0 && len(names) <= 2
	}
	var cands []gonSymbolCandidate
	if head == "" {
		if valid(parts) {
			cands = append(cands, gonSymbolCandidate{"", parts})
		}
		if len(parts) >= 2 && valid(parts[1:]) && parts[0] != "" {
			cands = append(cands, gonSymbolCandidate{parts[0], parts[1:]})
		}
		return cands
	}
	for k := len(parts) - 1; k >= 1; k-- {
		if valid(parts[k:]) && parts[k-1] != "" {
			cands = append(cands, gonSymbolCandidate{head + strings.Join(parts[:k], "."), parts[k:]})
		}
	}
	return cands
}

func (r *gonRequest) resolveSymbol(ctx context.Context, spec string, rev gonRevision) (*gonTarget, error) {
	cands := gonSymbolCandidates(spec)
	if len(cands) == 0 {
		return nil, gonErrorf(gonKindUsage, gonExitUsage, "invalid target %q: want a position or a symbol such as pkg.Name or Type.Method", spec)
	}
	type found struct {
		obj types.Object
		pkg *cache.Package
		snp *cache.Snapshot
	}
	var hits []found
	var misses []string
	for _, c := range cands {
		obj, pkg, snapshot, err := r.lookupSymbol(ctx, c)
		if err != nil {
			if ge := asGonError(err); ge.exit == gonExitInfra {
				return nil, err
			}
			misses = append(misses, err.Error())
			continue
		}
		dup := false
		for _, h := range hits {
			dup = dup || h.obj == obj
		}
		if !dup {
			hits = append(hits, found{obj, pkg, snapshot})
		}
		if c.pkg != "" && strings.Contains(c.pkg, "/") {
			break // the longest matching package path wins
		}
	}
	switch len(hits) {
	case 0:
		return nil, gonErrorf(gonKindNotFound, gonExitFindings, "%s: %s", spec, strings.Join(misses, "; "))
	case 1:
	default:
		var names []string
		for _, h := range hits {
			names = append(names, gonQualified(h.obj))
		}
		return nil, gonErrorf(gonKindAmbiguous, gonExitFindings, "%s is ambiguous: %s", spec, strings.Join(names, ", "))
	}
	h := hits[0]
	loc, err := golang.ObjectLocation(ctx, h.pkg.FileSet(), h.snp, h.obj)
	if err != nil {
		return nil, gonErrorf(gonKindNotFound, gonExitFindings, "%s: no source location: %v", spec, err)
	}
	t := &gonTarget{Spec: spec, Kind: "symbol", loc: loc, obj: h.obj, pkg: h.pkg, Object: gonDescribe(h.obj, nil)}
	l, _, err := r.locate(ctx, loc, rev)
	if err != nil {
		return nil, err
	}
	t.Location = &l
	return t, nil
}

func (r *gonRequest) lookupSymbol(ctx context.Context, c gonSymbolCandidate) (types.Object, *cache.Package, *cache.Snapshot, error) {
	lookup := func(scope *types.Scope, pkg *cache.Package, snapshot *cache.Snapshot) (types.Object, *cache.Package, *cache.Snapshot, error) {
		obj := scope.Lookup(c.names[0])
		if obj == nil {
			return nil, nil, nil, fmt.Errorf("no %s in package %s", c.names[0], pkg.Types().Path())
		}
		if len(c.names) == 2 {
			member, _, _ := types.LookupFieldOrMethod(obj.Type(), true, obj.Pkg(), c.names[1])
			if enum := types.EnumOf(obj.Type()); enum != nil && c.names[1] == "Parse" && enum.IsString() {
				if parser := enum.StringParser(); parser != nil {
					member = parser
				}
			}
			if member == nil {
				return nil, nil, nil, fmt.Errorf("%s has no field or method %s", gonQualified(obj), c.names[1])
			}
			obj = member
		}
		return obj, pkg, snapshot, nil
	}
	switch {
	case c.pkg == "":
		pkg, snapshot, err := r.packageInDir(ctx, r.inv.cwd)
		if err != nil {
			return nil, nil, nil, err
		}
		return lookup(pkg.Types().Scope(), pkg, snapshot)
	case strings.HasPrefix(c.pkg, ".") || filepath.IsAbs(c.pkg):
		pkg, snapshot, err := r.packageInDir(ctx, r.abs(c.pkg))
		if err != nil {
			return nil, nil, nil, err
		}
		return lookup(pkg.Types().Scope(), pkg, snapshot)
	}
	// An import path known to the workspace's metadata graph.
	snapshot, ctxPkg, err := r.contextSnapshot(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	graph, err := snapshot.LoadMetadataGraph(ctx)
	if err != nil {
		return nil, nil, nil, gonErrorf(gonKindWorkspace, gonExitInfra, "loading packages: %v", err)
	}
	var ids []metadata.PackageID
	for id, mp := range graph.Packages {
		if string(mp.PkgPath) == c.pkg && mp.ForTest == "" {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) > 0 {
		pkgs, err := snapshot.TypeCheck(ctx, ids[0])
		if err != nil {
			return nil, nil, nil, gonErrorf(gonKindWorkspace, gonExitInfra, "type-checking %s: %v", c.pkg, err)
		}
		return lookup(pkgs[0].Types().Scope(), pkgs[0], snapshot)
	}
	// A package name imported by the package in the working directory.
	if ctxPkg != nil && !strings.Contains(c.pkg, "/") {
		for _, imp := range ctxPkg.Types().Imports() {
			if imp.Name() == c.pkg {
				return lookup(imp.Scope(), ctxPkg, snapshot)
			}
		}
	}
	return nil, nil, nil, fmt.Errorf("package %s is not loaded in this workspace", c.pkg)
}

// contextSnapshot returns a snapshot for resolving import paths and, when the
// working directory contains one, its package.
func (r *gonRequest) contextSnapshot(ctx context.Context) (*cache.Snapshot, *cache.Package, error) {
	if pkg, snapshot, err := r.packageInDir(ctx, r.inv.cwd); err == nil {
		return snapshot, pkg, nil
	}
	snapshot, err := r.snapshot(ctx, protocol.URIFromPath(r.inv.cwd))
	return snapshot, nil, err
}

// packageInDir type-checks the package whose non-test files are in dir.
func (r *gonRequest) packageInDir(ctx context.Context, dir string) (*cache.Package, *cache.Snapshot, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, gonErrorf(gonKindNotFound, gonExitFindings, "%s: no such directory", r.rel(dir))
	}
	var file string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			file = filepath.Join(dir, name)
			break
		}
	}
	if file == "" {
		return nil, nil, fmt.Errorf("no Go package in %s", r.rel(dir))
	}
	uri := protocol.URIFromPath(file)
	snapshot, err := r.snapshot(ctx, uri)
	if err != nil {
		return nil, nil, err
	}
	pkg, _, err := golang.NarrowestPackageForFile(ctx, snapshot, uri)
	if err != nil {
		return nil, nil, gonErrorf(gonKindWorkspace, gonExitInfra, "loading package in %s: %v", r.rel(dir), err)
	}
	return pkg, snapshot, nil
}

// gonDescribe summarizes an object. from, if non-nil, qualifies names
// relative to that package.
func gonDescribe(obj types.Object, from *types.Package) *gonObjectInfo {
	qual := func(p *types.Package) string {
		if p == from {
			return ""
		}
		return p.Name()
	}
	info := &gonObjectInfo{Name: obj.Name(), Signature: types.ObjectString(obj, qual)}
	if owner := gonStringEnumGeneratedOwner(obj); owner != nil {
		if fn := obj.(*types.Func); fn.Signature().Recv() == nil {
			info.Signature = types.ObjectString(types.NewFunc(fn.Pos(), fn.Pkg(), owner.Name()+"."+fn.Name(), fn.Signature()), qual)
		}
	}
	if obj.Pkg() != nil {
		info.Package = obj.Pkg().Path()
	}
	switch obj := obj.(type) {
	case *types.Func:
		info.Kind = "func"
		if sig, ok := obj.Type().(*types.Signature); ok && sig.Recv() != nil {
			info.Kind = "method"
		}
	case *types.Var:
		switch {
		case obj.IsField():
			info.Kind = "field"
		case obj.Kind() == types.ParamVar || obj.Kind() == types.RecvVar:
			info.Kind = "param"
		case obj.Kind() == types.ResultVar:
			info.Kind = "result"
		default:
			info.Kind = "var"
		}
	case *types.Const:
		info.Kind = "const"
	case *types.TypeName:
		info.Kind = "type"
	case *types.PkgName:
		info.Kind = "package"
		info.Package = obj.Imported().Path()
	case *types.Label:
		info.Kind = "label"
	case *types.Builtin:
		info.Kind = "builtin"
	default:
		info.Kind = "object"
	}
	return info
}

// gonQualified formats an object name as a symbol target.
func gonQualified(obj types.Object) string {
	name := obj.Name()
	if fn, ok := obj.(*types.Func); ok {
		if owner := gonStringEnumGeneratedOwner(fn); owner != nil && fn.Signature().Recv() == nil {
			name = owner.Name() + "." + name
		}
		if recv := fn.Signature().Recv(); recv != nil {
			t := recv.Type()
			if p, ok := t.(*types.Pointer); ok {
				t = p.Elem()
			}
			if n, ok := types.Unalias(t).(*types.Named); ok {
				name = n.Obj().Name() + "." + name
			}
		}
	}
	if obj.Pkg() != nil {
		return obj.Pkg().Path() + "." + name
	}
	return name
}

func gonStringEnumGeneratedOwner(obj types.Object) *types.TypeName {
	fn, ok := obj.(*types.Func)
	if !ok {
		return nil
	}
	sig := fn.Signature()
	var typ types.Type
	if sig.Recv() != nil {
		typ = sig.Recv().Type()
		if ptr, ok := typ.(*types.Pointer); ok {
			typ = ptr.Elem()
		}
	} else if fn.Name() == "Parse" && sig.Results().Len() == 1 {
		typ = sig.Results().At(0).Type()
	} else {
		return nil
	}
	if named, ok := types.Unalias(typ).(*types.Named); ok {
		if enum := types.EnumOf(named); enum != nil && enum.IsString() && fn.Pos() == named.Obj().Pos() {
			return named.Obj()
		}
	}
	return nil
}
