package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/gopls/internal/cache"
	"golang.org/x/tools/gopls/internal/cache/metadata"
	"golang.org/x/tools/gopls/internal/golang"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/settings"
)

type gonCheckResult struct {
	gonEnvelope
	Patterns    []string         `json:"patterns"`
	Packages    []gonPackageInfo `json:"packages"`
	Verified    []string         `json:"verified"`
	Analyzers   []string         `json:"analyzers"`
	NotVerified []string         `json:"notVerified"`
	Warnings    []string         `json:"warnings,omitempty"`
	Summary     gonCheckSummary  `json:"summary"`
	Total       int              `json:"total"`
	Diagnostics []*gonDiagnostic `json:"diagnostics"`
	More        *gonPagination   `json:"truncated,omitempty"`
	Revision    gonRevision      `json:"revision"`
}

type gonPackageInfo struct {
	ImportPath string `json:"importPath"`
	Dir        string `json:"dir"`
	Files      int    `json:"files"`
}

type gonCheckSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
	Hints    int `json:"hints"`
}

// gonDiagnostic is a diagnostic reported by "gon check". Category separates
// language errors (syntax, types, package loading) from optional analyses.
type gonDiagnostic struct {
	Location gonLocation  `json:"location"`
	Severity string       `json:"severity"`
	Category string       `json:"category"` // language, analysis or load
	Source   string       `json:"source"`
	Code     string       `json:"code,omitempty"` // accepted by "gon explain"
	Message  string       `json:"message"`
	Text     string       `json:"text"`
	Tags     []string     `json:"tags,omitempty"`
	Related  []gonRelated `json:"related,omitempty"`
	Fixes    []gonFix     `json:"fixes,omitempty"`
	severity protocol.DiagnosticSeverity
}

type gonRelated struct {
	Location gonLocation `json:"location"`
	Message  string      `json:"message"`
}

// gonFix is a mechanical edit proposed by an analyzer. Fixes are never
// applied by "gon check"; design alternatives are not reported as fixes.
type gonFix struct {
	Title string    `json:"title"`
	Edits []gonEdit `json:"edits"`
}

type gonEdit struct {
	Location gonLocation `json:"location"`
	NewText  string      `json:"newText"`
}

var gonSeverities = map[string]protocol.DiagnosticSeverity{
	"error": protocol.SeverityError, "warning": protocol.SeverityWarning,
	"info": protocol.SeverityInformation, "hint": protocol.SeverityHint,
}

func gonSeverityName(s protocol.DiagnosticSeverity) string {
	for name, v := range gonSeverities {
		if v == s {
			return name
		}
	}
	return "error"
}

// goListPackage is the subset of 'go list -json' used by "gon check".
type goListPackage struct {
	ImportPath   string
	Dir          string
	GoFiles      []string
	CgoFiles     []string
	TestGoFiles  []string
	XTestGoFiles []string
	Error        *struct{ Err string }
}

func (r *gonRequest) check(ctx context.Context) (gonResult, error) {
	cutoff, ok := gonSeverities[r.str("severity")]
	if !ok {
		return nil, gonErrorf(gonKindUsage, gonExitUsage, "invalid --severity %q: want error, warning, info or hint", r.str("severity"))
	}
	category := r.str("category")
	if category != "all" && category != "language" && category != "analysis" {
		return nil, gonErrorf(gonKindUsage, gonExitUsage, "invalid --category %q: want language, analysis or all", category)
	}
	args := r.args
	if len(args) == 0 {
		args = []string{"."}
	}
	result := &gonCheckResult{Patterns: args, Revision: make(gonRevision), Diagnostics: []*gonDiagnostic{}}

	// Select files: .go arguments directly, other arguments as package
	// patterns resolved by the selected toolchain's go command.
	wanted := make(map[protocol.DocumentURI]bool)
	var roots []protocol.DocumentURI // one file per package variant to diagnose
	var patterns []string
	var loadErrors []*gonDiagnostic
	for _, arg := range args {
		if strings.HasSuffix(arg, ".go") {
			path := r.abs(arg)
			if st, err := os.Stat(path); err != nil || st.IsDir() {
				return nil, gonErrorf(gonKindNotFound, gonExitFindings, "%s: no such file", arg)
			}
			uri := protocol.URIFromPath(path)
			wanted[uri] = true
			roots = append(roots, uri)
		} else {
			patterns = append(patterns, arg)
		}
	}
	if len(patterns) > 0 {
		pkgs, warnings, err := r.goList(ctx, patterns)
		if err != nil {
			return nil, err
		}
		result.Warnings = warnings
		for _, p := range pkgs {
			if p.Error != nil {
				// Such packages also fail 'gon build' and 'gon vet'.
				loadErrors = append(loadErrors, &gonDiagnostic{
					Severity: "error", severity: protocol.SeverityError, Category: "load", Source: "go list",
					Message: fmt.Sprintf("%s: %s", p.ImportPath, strings.TrimSpace(p.Error.Err)),
				})
			}
			if p.Dir == "" {
				continue
			}
			info := gonPackageInfo{ImportPath: p.ImportPath, Dir: p.Dir}
			uri := func(name string) protocol.DocumentURI { return protocol.URIFromPath(filepath.Join(p.Dir, name)) }
			for _, group := range [][]string{p.GoFiles, p.CgoFiles, p.TestGoFiles, p.XTestGoFiles} {
				for _, name := range group {
					wanted[uri(name)] = true
					info.Files++
				}
			}
			// One file selects the package and its test variant; an
			// external test file selects the _test package.
			if files := append(append(append([]string{}, p.GoFiles...), p.CgoFiles...), p.TestGoFiles...); len(files) > 0 {
				roots = append(roots, uri(files[0]))
			}
			if len(p.XTestGoFiles) > 0 {
				roots = append(roots, uri(p.XTestGoFiles[0]))
			}
			if info.Files > 0 {
				result.Packages = append(result.Packages, info)
			}
		}
	}
	if len(roots) == 0 && len(loadErrors) == 0 && len(result.Warnings) == 0 {
		result.Warnings = append(result.Warnings, "no Go files matched")
	}

	// Diagnose every non-intermediate package variant of the selected files,
	// grouped by the build (View) that owns them.
	if _, err := r.ensureEngine(ctx); err != nil {
		return nil, err
	}
	type group struct {
		snapshot *cache.Snapshot
		pkgs     map[metadata.PackageID]*metadata.Package
	}
	groups := make(map[*cache.View]*group)
	var order []*cache.View
	for _, uri := range roots {
		snapshot, err := r.snapshot(ctx, uri)
		if err != nil {
			return nil, err
		}
		mps, err := snapshot.MetadataForFile(ctx, uri, true)
		if err != nil {
			return nil, gonErrorf(gonKindWorkspace, gonExitInfra, "loading %s: %v", r.rel(uri.Path()), err)
		}
		g := groups[snapshot.View()]
		if g == nil {
			g = &group{snapshot: snapshot, pkgs: make(map[metadata.PackageID]*metadata.Package)}
			groups[snapshot.View()] = g
			order = append(order, snapshot.View())
		}
		for _, mp := range mps {
			g.pkgs[mp.ID] = mp
		}
	}
	analyzers := make(map[string]bool)
	seen := make(map[string]bool)
	var diags []*gonDiagnostic
	for _, v := range order {
		g := groups[v]
		ids := make([]metadata.PackageID, 0, len(g.pkgs))
		for id := range g.pkgs {
			ids = append(ids, id)
		}
		byFile, err := g.snapshot.PackageDiagnostics(ctx, ids...)
		if err != nil {
			return nil, gonErrorf(gonKindWorkspace, gonExitInfra, "type-checking: %v", err)
		}
		analysis, err := golang.Analyze(ctx, g.snapshot, g.pkgs, nil)
		if err != nil {
			return nil, gonErrorf(gonKindWorkspace, gonExitInfra, "analysis: %v", err)
		}
		for _, a := range settings.AllAnalyzers {
			if a.Enabled(g.snapshot.Options()) {
				analyzers[a.Analyzer().Name] = true
			}
		}
		uris := make(map[protocol.DocumentURI]bool)
		for uri := range byFile {
			uris[uri] = true
		}
		for uri := range analysis {
			uris[uri] = true
		}
		for uri := range uris {
			for _, d := range golang.CombineDiagnostics(byFile[uri], analysis[uri]) {
				// A selected package can establish a nil flow whose consumer is
				// in a dependency outside the requested package patterns.
				if !wanted[uri] && d.Source != "nilaway" {
					continue
				}
				gd, err := r.convertDiagnostic(ctx, d, result.Revision)
				if err != nil {
					return nil, err
				}
				key := fmt.Sprintf("%s|%d|%d|%s|%s|%s", gd.Location.Path, gd.Location.Offset, gd.Location.EndOffset, gd.Severity, gd.Source, gd.Message)
				if !seen[key] {
					seen[key] = true
					diags = append(diags, gd)
				}
			}
		}
	}
	diags = append(loadErrors, diags...)

	// Filter, then count, sort and paginate what is reported.
	var kept []*gonDiagnostic
	for _, d := range diags {
		if d.severity > cutoff || category != "all" && d.Category != category && d.Category != "load" ||
			r.str("code") != "" && d.Code != r.str("code") {
			continue
		}
		kept = append(kept, d)
		switch d.severity {
		case protocol.SeverityError:
			result.Summary.Errors++
		case protocol.SeverityWarning:
			result.Summary.Warnings++
		case protocol.SeverityInformation:
			result.Summary.Infos++
		default:
			result.Summary.Hints++
		}
	}
	sort.SliceStable(kept, func(i, j int) bool {
		a, b := kept[i].Location, kept[j].Location
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Offset != b.Offset {
			return a.Offset < b.Offset
		}
		return kept[i].severity < kept[j].severity
	})
	result.Total = len(kept)
	offset, limit := min(r.int("offset"), len(kept)), r.int("limit")
	kept = kept[offset:]
	if limit > 0 && len(kept) > limit {
		result.More = &gonPagination{Omitted: len(kept) - limit, NextOffset: offset + limit}
		kept = kept[:limit]
	}
	if kept != nil {
		result.Diagnostics = kept
	}

	result.Analyzers = make([]string, 0, len(analyzers))
	for name := range analyzers {
		result.Analyzers = append(result.Analyzers, name)
	}
	sort.Strings(result.Analyzers)
	result.Verified = []string{"parse", "type-check", "analysis"}
	result.NotVerified = []string{
		"build: compiler type checking (types2), code generation and linking; run 'gon build'",
		"tests; run 'gon test'",
		"cmd/vet as run by 'gon vet'",
	}
	if !r.bool("staticcheck") {
		result.NotVerified = append(result.NotVerified, "Staticcheck analyzers outside the default set; rerun with --staticcheck")
	}
	if !r.bool("nilaway") {
		result.NotVerified = append(result.NotVerified, "NilAway nil-flow analysis; rerun with --nilaway")
	}
	result.NotVerified = append(result.NotVerified, "other GOOS/GOARCH and build tag configurations")
	for _, d := range diags {
		if d.Category != "analysis" && d.severity == protocol.SeverityError {
			result.NotVerified = append(result.NotVerified,
				"analyzers that need type-correct code, in packages with errors")
			break
		}
	}
	result.gonEnvelope = *r.envelope("check")
	return result, nil
}

// goList resolves package patterns exactly as the selected go command does.
func (r *gonRequest) goList(ctx context.Context, patterns []string) ([]goListPackage, []string, error) {
	args := []string{"list", "-e", "-find", "-json=ImportPath,Dir,GoFiles,CgoFiles,TestGoFiles,XTestGoFiles,Error"}
	if tags := r.str("tags"); tags != "" {
		args = append(args, "-tags="+tags)
	}
	args = append(args, "--")
	args = append(args, patterns...)
	goCmd := filepath.Join(gonRoot(r.inv.env), "bin", "go")
	cmd := exec.CommandContext(ctx, goCmd, args...)
	cmd.Dir = r.inv.cwd
	// The go command reports paths under $PWD when it names the working
	// directory; set it to the canonical directory used for everything else.
	cmd.Env = append(append([]string{}, r.inv.env...), "GOTOOLCHAIN=local", "PWD="+r.inv.cwd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, nil, gonErrorf(gonKindWorkspace, gonExitInfra, "resolving packages: %s", msg)
	}
	var pkgs []goListPackage
	dec := json.NewDecoder(&stdout)
	for dec.More() {
		var p goListPackage
		if err := dec.Decode(&p); err != nil {
			return nil, nil, gonErrorf(gonKindInternal, gonExitInfra, "decoding go list output: %v", err)
		}
		if dir, err := filepath.EvalSymlinks(p.Dir); err == nil {
			p.Dir = dir
		}
		pkgs = append(pkgs, p)
	}
	var warnings []string
	for _, line := range strings.Split(strings.TrimSpace(stderr.String()), "\n") {
		if line = strings.TrimSpace(strings.TrimPrefix(line, "go: ")); line != "" {
			warnings = append(warnings, strings.TrimPrefix(line, "warning: "))
		}
	}
	return pkgs, warnings, nil
}

func (r *gonRequest) convertDiagnostic(ctx context.Context, d *cache.Diagnostic, rev gonRevision) (*gonDiagnostic, error) {
	loc, doc, err := r.locate(ctx, protocol.Location{URI: d.URI, Range: d.Range}, rev)
	if err != nil {
		return nil, err
	}
	gd := &gonDiagnostic{
		Location: loc, Text: doc.line(loc.Line), Message: d.Message,
		Severity: gonSeverityName(d.Severity), severity: d.Severity, Source: string(d.Source),
	}
	switch d.Source {
	case cache.ListError, cache.ParseError, cache.TypeError, cache.WorkFileError:
		gd.Category, gd.Code = "language", d.Code
	default:
		gd.Category, gd.Code = "analysis", string(d.Source)
	}
	if gd.Code == "default" {
		gd.Code = ""
	}
	for _, tag := range d.Tags {
		switch tag {
		case protocol.Unnecessary:
			gd.Tags = append(gd.Tags, "unnecessary")
		case protocol.Deprecated:
			gd.Tags = append(gd.Tags, "deprecated")
		}
	}
	for _, rel := range d.Related {
		l, _, err := r.locate(ctx, rel.Location, rev)
		if err != nil {
			continue // related information outside readable files is optional
		}
		gd.Related = append(gd.Related, gonRelated{Location: l, Message: rel.Message})
	}
	for _, fix := range d.SuggestedFixes {
		if fix.Command != nil || len(fix.Edits) == 0 {
			continue
		}
		gf := gonFix{Title: fix.Title}
		for uri, edits := range fix.Edits {
			for _, e := range edits {
				l, _, err := r.locate(ctx, protocol.Location{URI: uri, Range: e.Range}, rev)
				if err != nil {
					return nil, err
				}
				gf.Edits = append(gf.Edits, gonEdit{Location: l, NewText: e.NewText})
			}
		}
		sort.Slice(gf.Edits, func(i, j int) bool { return gonLess(gf.Edits[i].Location, gf.Edits[j].Location) })
		gd.Fixes = append(gd.Fixes, gf)
	}
	return gd, nil
}

func (c *gonCheckResult) exitCode() int {
	if c.Summary.Errors > 0 {
		return gonExitFindings
	}
	return gonExitOK
}

func (c *gonCheckResult) text(w io.Writer, r *gonRequest) {
	for _, d := range c.Diagnostics {
		where := "gon check"
		if d.Location.Path != "" {
			where = fmt.Sprintf("%s:%d:%d", r.rel(d.Location.Path), d.Location.Line, d.Location.Column)
		}
		label := d.Source
		if d.Code != "" && d.Code != d.Source {
			label += " " + d.Code
		}
		fmt.Fprintf(w, "%s: %s: %s (%s)\n", where, d.Severity, d.Message, label)
	}
	if c.More != nil {
		fmt.Fprintf(w, "... %d more; rerun with --offset %d\n", c.More.Omitted, c.More.NextOffset)
	}
	for _, warning := range c.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warning)
	}
	fmt.Fprintf(w, "checked %d package(s): parse, type-check, %d analyzers\n", len(c.Packages), len(c.Analyzers))
	fmt.Fprintf(w, "not verified: build, tests, gon vet")
	if !r.bool("staticcheck") {
		fmt.Fprint(w, ", staticcheck")
	}
	if !r.bool("nilaway") {
		fmt.Fprint(w, ", nilaway")
	}
	fmt.Fprintln(w, ", other configurations")
	s := c.Summary
	fmt.Fprintf(w, "%d error(s), %d warning(s), %d info, %d hint(s)\n", s.Errors, s.Warnings, s.Infos, s.Hints)
}
