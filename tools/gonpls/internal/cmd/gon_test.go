package cmd_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// These tests run the "gon" tooling commands in child processes, as the
// public launcher does, against modules that use Gon error handling.

const gonModule = `
-- go.mod --
module example.com/gon

go 1.26
-- store/store.go --
package store

import (
	"errors"
	"fmt"
)

// Reader reads values.
type Reader interface {
	Read(key string) (int, error)
}

// Memory is an in-memory Reader.
type Memory struct {
	Values map[string]int
}

// Read returns the value stored for key.
func (m *Memory) Read(key string) (int, error) {
	v, ok := m.Values[key]
	if !ok {
		return 0, errors.New("missing " + key)
	}
	return v, nil
}

// Sum adds two stored values.
func Sum(r Reader, a, b string) (int, error) {
	x := r.Read(a)!
	y := r.Read(b) or problem {
		return 0, fmt.Errorf("second value: %w", problem)
	}
	return x + y, nil
}

// Flush has only an error result.
func Flush() error { return nil }

// Close propagates an error-only result.
func Close() error {
	Flush()!
	return nil
}
-- main.go --
package main

import (
	"fmt"

	"example.com/gon/store"
)

func main() {
	m := &store.Memory{Values: map[string]int{"a": 1, "b": 2}}
	label, sum := "ñ", store.Sum
	fmt.Println(label)
	fmt.Println(sum(m, "a", "b"))
	fmt.Println(store.Sum(m, "a", "c"))
}
`

// The legacy spelling of store.Sum, for paired checks.
const gonLegacySum = `func Sum(r Reader, a, b string) (int, error) {
	x, err := r.Read(a)
	if err != nil {
		return 0, err
	}
	y, problem := r.Read(b)
	if problem != nil {
		return 0, fmt.Errorf("second value: %w", problem)
	}
	return x + y, nil
}`

func gon(t *testing.T, dir string, env []string, args ...string) *result {
	t.Helper()
	return goplsWithEnv(t, dir, env, append([]string{"gon"}, args...)...)
}

// gonJSON runs a command with --json and decodes its result.
func gonJSON(t *testing.T, dir string, env []string, v any, args ...string) *result {
	t.Helper()
	res := gon(t, dir, env, append(args, "--json")...)
	if err := json.Unmarshal([]byte(res.stdout), v); err != nil {
		t.Fatalf("%s: invalid JSON: %v", res, err)
	}
	return res
}

func (res *result) checkCode(code int) {
	res.t.Helper()
	if res.exitcode != code {
		res.t.Errorf("%s: exit code %d, want %d", res, res.exitcode, code)
	}
}

type gonLoc struct {
	Path                             string
	Line, Column, EndLine, EndColumn int
	Offset, EndOffset                int
}

type gonQuery struct {
	SchemaVersion int
	Operation     string
	OK            bool
	Results       []struct {
		Target struct {
			Spec, Kind string
			Location   gonLoc
			Object     struct{ Name, Kind, Signature string }
		}
		Error *struct{ Kind, Message string }
		Total int
		Items []struct {
			Location    gonLoc
			Text        string
			Declaration bool
			Object      struct{ Name, Kind, Signature string }
		}
		Type *struct {
			Expression, Type, Mode, Construct, Underlying string
		}
		Truncated *struct{ Omitted, NextOffset int }
	}
	Revision map[string]string
}

func TestGonQuery(t *testing.T) {
	t.Parallel()
	tree := writeTree(t, gonModule)
	store := filepath.Join(tree, "store")

	// Every symbol spelling resolves to the same declaration.
	for _, test := range []struct{ dir, spec string }{
		{tree, "store.Sum"},
		{tree, "./store.Sum"},
		{tree, "example.com/gon/store.Sum"},
		{store, "Sum"},
		{store, "../store.Sum"},
	} {
		var q gonQuery
		gonJSON(t, test.dir, nil, &q, "query", "refs", test.spec).checkCode(0)
		if len(q.Results) != 1 || q.Results[0].Total != 3 || q.Results[0].Target.Object.Kind != "func" {
			t.Errorf("refs %s from %s: %+v", test.spec, test.dir, q)
		}
	}

	// Positions: line:column in UTF-8 bytes and #offset address the same
	// handler binding; locations report byte columns.
	src, _ := os.ReadFile(filepath.Join(store, "store.go"))
	offset := strings.Index(string(src), "problem {")
	var byOffset, byLine gonQuery
	gonJSON(t, tree, nil, &byOffset, "query", "refs", "store/store.go:#"+strconv.Itoa(offset)).checkCode(0)
	gonJSON(t, tree, nil, &byLine, "query", "refs", "store/store.go:30:20").checkCode(0)
	if byOffset.Results[0].Total != 2 || byLine.Results[0].Total != 2 ||
		byOffset.Results[0].Items[0].Location != byLine.Results[0].Items[0].Location {
		t.Errorf("handler binding references differ: %+v / %+v", byOffset, byLine)
	}
	main, _ := os.ReadFile(filepath.Join(tree, "main.go"))
	var refs gonQuery
	gonJSON(t, tree, nil, &refs, "query", "refs", "store.Sum").checkCode(0)
	for _, it := range refs.Results[0].Items {
		if strings.HasSuffix(it.Location.Path, "main.go") && it.Location.Line == 11 {
			line := strings.Split(string(main), "\n")[10]
			if want := strings.Index(line, "Sum") + 1; it.Location.Column != want {
				t.Errorf("UTF-8 column = %d, want %d", it.Location.Column, want)
			}
			if it.Text != `label, sum := "ñ", store.Sum` {
				t.Errorf("reference text = %q", it.Text)
			}
		}
		if strings.HasSuffix(it.Location.Path, "store.go") && !it.Declaration {
			t.Errorf("declaration not marked: %+v", it)
		}
	}
	sum := sha256.Sum256(src)
	if got := refs.Revision["./store/store.go"]; got != hex.EncodeToString(sum[:]) {
		t.Errorf("revision of store.go = %q, want the SHA-256 of its content", got)
	}
	if refs.SchemaVersion != 1 || refs.Operation != "query.refs" || !refs.OK {
		t.Errorf("envelope: %+v", refs)
	}

	// Flags may follow the targets; pagination reports the next offset.
	var page gonQuery
	gonJSON(t, tree, nil, &page, "query", "refs", "store.Sum", "--limit", "1", "--offset=1").checkCode(0)
	if it := page.Results[0]; len(it.Items) != 1 || it.Truncated == nil || it.Truncated.NextOffset != 2 || it.Truncated.Omitted != 1 {
		t.Errorf("pagination: %+v", page)
	}

	// Definition, types of Gon constructs and implementations.
	var def gonQuery
	gonJSON(t, tree, nil, &def, "query", "def", "store/store.go:31:45").checkCode(0)
	if it := def.Results[0].Items; len(it) != 1 || it[0].Location.Line != 30 || it[0].Object.Signature != "var problem error" {
		t.Errorf("def of handler binding use: %+v", def)
	}
	var types gonQuery
	gonJSON(t, tree, nil, &types, "query", "type", "store/store.go:29:16", "store/store.go:30:18", "store/store.go:41:9").checkCode(0)
	for i, want := range []struct{ typ, construct string }{{"int", "error-propagation"}, {"int", "error-handler"}, {"(no value)", "error-propagation"}} {
		if ty := types.Results[i].Type; ty == nil || ty.Type != want.typ || ty.Construct != want.construct {
			t.Errorf("type #%d: %+v, want %+v", i, ty, want)
		}
	}
	var impls gonQuery
	gonJSON(t, tree, nil, &impls, "query", "impls", "store.Reader", "store.Memory").checkCode(0)
	if impls.Results[0].Total != 1 || !strings.Contains(impls.Results[0].Items[0].Text, "type Memory struct") ||
		impls.Results[1].Total != 1 || !strings.Contains(impls.Results[1].Items[0].Text, "type Reader interface") {
		t.Errorf("implementations: %+v", impls)
	}
	var syms gonQuery
	gonJSON(t, tree, nil, &syms, "query", "symbols", "Memory").checkCode(0)
	if it := syms.Results[0].Items; len(it) == 0 || it[0].Object.Name != "example.com/gon/store.Memory" {
		t.Errorf("symbols: %+v", syms)
	}
	for _, it := range syms.Results[0].Items {
		if !strings.HasPrefix(it.Location.Path, "./") {
			t.Errorf("symbol outside the workspace: %+v", it)
		}
	}

	// Text output is compact: one location per line.
	res := gon(t, tree, nil, "query", "refs", "store.Sum")
	res.checkCode(0)
	res.checkStdout(`(?m)^main.go:14:20: fmt.Println\(store.Sum\(m, "a", "c"\)\)$`)

	// Failures: unresolved targets exit 1; usage errors exit 2.
	var missing gonQuery
	gonJSON(t, tree, nil, &missing, "query", "refs", "NoSuch").checkCode(1)
	if missing.OK || missing.Results[0].Error == nil || missing.Results[0].Error.Kind != "not-found" {
		t.Errorf("missing symbol: %+v", missing)
	}
	gon(t, tree, nil, "query", "refs", "store/store.go:99:1").checkCode(1)
	gon(t, tree, nil, "query", "refs", "store/store.go:12").checkCode(2)
	gon(t, tree, nil, "query", "refs").checkCode(2)
	gon(t, tree, nil, "query", "refs", "store.Sum", "--bogus").checkCode(2)
	gon(t, tree, nil, "query", "type", "store.Sum").checkCode(2)
	res = gon(t, tree, nil, "query", "nosuch")
	res.checkCode(2)
	res.checkStderr("unknown command")
}

func TestGonConditionalQuery(t *testing.T) {
	t.Parallel()
	const src = `package p
var flag bool
var value = if flag { 1 } else { 2 }
const size = if true { 1 } else { 2.5 }
`
	tree := writeTree(t, "-- go.mod --\nmodule example.com/conditional\n\ngo 1.26\n-- p.go --\n"+src)
	for _, test := range []struct{ needle, typ, mode string }{
		{"if flag", "int", "value"},
		{"else { 2 }", "int", "value"},
		{"if true", "untyped float", "constant"},
	} {
		var q gonQuery
		gonJSON(t, tree, nil, &q, "query", "type", "p.go:#"+strconv.Itoa(strings.Index(src, test.needle))).checkCode(0)
		if len(q.Results) != 1 {
			t.Fatalf("results: %+v", q)
		}
		ty := q.Results[0].Type
		if ty == nil || ty.Construct != "conditional-expression" || ty.Type != test.typ || ty.Mode != test.mode {
			t.Errorf("type at %q = %+v, want %s (%s), conditional-expression", test.needle, ty, test.typ, test.mode)
		}
	}
}

type gonCheck struct {
	OK          bool
	Packages    []struct{ ImportPath string }
	Verified    []string
	NotVerified []string
	Warnings    []string
	Summary     struct{ Errors, Warnings, Infos, Hints int }
	Total       int
	Diagnostics []struct {
		Location                                  gonLoc
		Severity, Category, Source, Code, Message string
	}
	Revision map[string]string
}

func TestGonCheck(t *testing.T) {
	t.Parallel()
	tree := writeTree(t, gonModule+`
-- bad/bad.go --
package bad

import "fmt"

func value() int { return 1 }

func notError() error {
	value()!
	return nil
}

func noErrorResult() {
	fmt.Println("propagating")
	notError()!
	_ = missing
}
-- warn/warn.go --
package warn

import "fmt"

func Print() { fmt.Printf("%d\n", "text") }
`)
	var clean gonCheck
	gonJSON(t, tree, nil, &clean, "check", ".", "./store").checkCode(0)
	if !clean.OK || clean.Summary.Errors != 0 || len(clean.Packages) != 2 || len(clean.Verified) != 3 ||
		!strings.Contains(strings.Join(clean.NotVerified, "\n"), "gon build") {
		t.Errorf("clean check: %+v", clean)
	}

	var bad gonCheck
	gonJSON(t, tree, nil, &bad, "check", "./...").checkCode(1)
	codes := map[string]int{}
	for _, d := range bad.Diagnostics {
		codes[d.Category+" "+d.Code]++
	}
	if codes["language InvalidErrorHandling"] != 2 || codes["language UndeclaredName"] != 1 || codes["analysis printf"] != 1 || bad.Summary.Errors != 3 {
		t.Errorf("diagnostics: %v %+v", codes, bad)
	}
	if !strings.Contains(strings.Join(bad.NotVerified, "\n"), "type-correct") {
		t.Errorf("incomplete analysis not reported: %v", bad.NotVerified)
	}

	// Warnings never fail; filters change what is reported and counted.
	var analysis gonCheck
	gonJSON(t, tree, nil, &analysis, "check", "./bad", "./warn", "--category", "analysis").checkCode(0)
	if analysis.Total != 1 || analysis.Diagnostics[0].Source != "printf" || analysis.Diagnostics[0].Severity != "warning" {
		t.Errorf("analysis only: %+v", analysis)
	}
	var coded gonCheck
	gonJSON(t, tree, nil, &coded, "check", "./bad", "--code=InvalidErrorHandling", "--severity", "error").checkCode(1)
	if coded.Total != 2 {
		t.Errorf("code filter: %+v", coded)
	}
	var file gonCheck
	gonJSON(t, tree, nil, &file, "check", "store/store.go").checkCode(0)
	if file.Total != 0 {
		t.Errorf("single file: %+v", file)
	}

	// A pattern that cannot be loaded is a load error, not a diagnostic of
	// the analyzed code.
	var nope gonCheck
	gonJSON(t, tree, nil, &nope, "check", "./nope").checkCode(1)
	if nope.Total != 1 || nope.Diagnostics[0].Category != "load" {
		t.Errorf("missing directory: %+v", nope)
	}
	gon(t, tree, nil, "check", "--severity=fatal").checkCode(2)

	// Paths stay canonical and relative even when the working directory is
	// reached through a symbolic link, as with macOS temporary directories.
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(tree, link); err != nil {
		t.Fatal(err)
	}
	res := gon(t, tree, []string{"PWD=" + link}, "check", "./bad", "./warn")
	res.checkCode(1)
	res.checkStdout(`(?m)^bad/bad.go:8:2: error:`)
	res = gon(t, tree, nil, "check", "./bad", "./warn")
	res.checkCode(1)
	res.checkStdout(`bad/bad.go:8:2: error: error handling requires a final result of type error \(compiler InvalidErrorHandling\)`)
	res.checkStdout(`(?m)^3 error\(s\), 1 warning\(s\)`)

	// The legacy and modern spellings of the same package are both clean.
	legacy := writeTree(t, gonModule)
	path := filepath.Join(legacy, "store", "store.go")
	src, _ := os.ReadFile(path)
	start := strings.Index(string(src), "func Sum(")
	end := start + strings.Index(string(src[start:]), "\n}\n") + 2
	os.WriteFile(path, []byte(string(src[:start])+gonLegacySum+string(src[end:])), 0o666)
	for _, dir := range []string{tree, legacy} {
		var pair gonCheck
		gonJSON(t, dir, nil, &pair, "check", "./store", ".").checkCode(0)
		if pair.Total != 0 {
			t.Errorf("%s: %+v", dir, pair)
		}
	}
}

type gonPlanResult struct {
	OK      bool
	Status  string
	Error   *struct{ Kind, Message string }
	Summary struct{ Files, Edits int }
	Files   []struct {
		Path, SHA256, NewSHA256, Formatting string
		Edits                               []struct{ NewText string }
	}
}

func TestGonRename(t *testing.T) {
	t.Parallel()
	tree := writeTree(t, gonModule+`
-- pair.go --
package main

type pair struct {
	A    int    // first
	Long string // second
}

var _ = pair{A: 1, Long: "x"}
-- messy.go --
package main

type messy struct {
	B int
}

var _ =    messy{B: 1}
`)
	read := func(name string) string {
		data, _ := os.ReadFile(filepath.Join(tree, name))
		return string(data)
	}
	before := read("store/store.go")

	// A dry run prints a diff and changes nothing.
	res := gon(t, tree, nil, "refactor", "rename", "store/store.go:30:20", "failure", "--dry-run")
	res.checkCode(0)
	res.checkStdout(`\+\s+y := r.Read\(b\) or failure \{`)
	res.checkStdout(`would change 2 edit\(s\) in 1 file\(s\)`)
	if read("store/store.go") != before {
		t.Fatal("dry run modified a file")
	}

	// A plan applies only to the analyzed revision.
	var plan gonPlanResult
	res = gonJSON(t, tree, nil, &plan, "refactor", "rename", "store.Memory.Values", "Entries", "--dry-run")
	res.checkCode(0)
	if plan.Status != "planned" || plan.Summary.Files != 2 || plan.Summary.Edits != 3 {
		t.Fatalf("plan: %+v", plan)
	}
	planFile := filepath.Join(t.TempDir(), "plan.json")
	os.WriteFile(planFile, []byte(res.stdout), 0o666)
	staleFile := filepath.Join(t.TempDir(), "stale.json")
	os.WriteFile(staleFile, []byte(res.stdout), 0o666)

	os.WriteFile(filepath.Join(tree, "main.go"), []byte(read("main.go")+"\n// concurrent edit\n"), 0o666)
	mainBefore := read("main.go")
	var stale gonPlanResult
	gonJSON(t, tree, nil, &stale, "refactor", "apply", staleFile).checkCode(1)
	if stale.OK || stale.Error == nil || stale.Error.Kind != "stale" || read("store/store.go") != before || read("main.go") != mainBefore {
		t.Fatalf("stale plan was not rejected atomically: %+v", stale)
	}

	// Recompute and apply; reapplying the same plan is stale.
	res = gonJSON(t, tree, nil, &plan, "refactor", "rename", "store.Memory.Values", "Entries", "--dry-run")
	os.WriteFile(planFile, []byte(res.stdout), 0o666)
	gon(t, tree, nil, "refactor", "apply", planFile).checkCode(0)
	if !strings.Contains(read("store/store.go"), "m.Entries[key]") || !strings.Contains(read("main.go"), "Entries: map") {
		t.Fatal("plan not applied")
	}
	gon(t, tree, nil, "refactor", "apply", planFile).checkCode(1)

	// Direct application; gofmt realigns only files that were gofmt-clean.
	var applied gonPlanResult
	gonJSON(t, tree, nil, &applied, "refactor", "rename", "pair.A", "Abcdefghij").checkCode(0)
	if applied.Status != "applied" || applied.Files[0].Formatting != "gofmt" ||
		!strings.Contains(read("pair.go"), "\tAbcdefghij int    // first\n\tLong       string // second\n") {
		t.Errorf("realignment: %+v\n%s", applied, read("pair.go"))
	}
	gonJSON(t, tree, nil, &applied, "refactor", "rename", "messy.B", "Bee").checkCode(0)
	if applied.Files[0].Formatting != "skipped" || !strings.Contains(read("messy.go"), "var _ =    messy{Bee: 1}") {
		t.Errorf("unrelated formatting changed: %+v\n%s", applied, read("messy.go"))
	}

	// Rejected renames modify nothing.
	snapshot := read("store/store.go")
	var conflict gonPlanResult
	gonJSON(t, tree, nil, &conflict, "refactor", "rename", "store.Sum", "Close").checkCode(1)
	if conflict.Error == nil || conflict.Error.Kind != "rejected" || read("store/store.go") != snapshot {
		t.Errorf("conflict: %+v", conflict)
	}
	gon(t, tree, nil, "refactor", "rename", "store.Sum", "9bad").checkCode(2)

	// The renamed program still builds and runs.
	run := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "run", ".")
	run.Dir, run.Env = tree, append(os.Environ(), "GOTOOLCHAIN=local")
	out, err := run.CombinedOutput()
	if err != nil || string(out) != "ñ\n3 <nil>\n0 second value: missing c\n" {
		t.Errorf("go run after renames: %v\n%s", err, out)
	}
}

func TestGonExplain(t *testing.T) {
	t.Parallel()
	tree := writeTree(t, "-- go.mod --\nmodule example.com/x\n")
	var ex struct {
		OK           bool
		Explanations []struct {
			Code, Kind, Summary, Documentation string
			Number                             *int
			Cases                              []struct{ Message, Meaning, Fix string }
			References                         []string
			DefaultEnabled                     *bool
			Error                              *struct{ Kind string }
		}
	}
	gonJSON(t, tree, nil, &ex, "explain", "InvalidErrorHandling", "ErrorCode(10000)", "UndeclaredName", "unusedwrite", "SA4006").checkCode(0)
	e := ex.Explanations
	if e[0].Kind != "type-error" || e[0].Number == nil || *e[0].Number != 10000 || len(e[0].Cases) != 9 || len(e[0].References) != 1 ||
		e[1].Code != "InvalidErrorHandling" || e[2].Kind != "type-error" ||
		e[3].Kind != "analyzer" || e[3].DefaultEnabled == nil || e[4].Kind != "analyzer" {
		t.Errorf("explanations: %+v", ex)
	}
	gonJSON(t, tree, nil, &ex, "explain", "NoSuchCode").checkCode(1)
	if ex.OK || ex.Explanations[0].Error == nil {
		t.Errorf("unknown code: %+v", ex)
	}

	// The explained messages are those of both type checkers.
	gonJSON(t, tree, nil, &ex, "explain", "InvalidErrorHandling")
	for _, files := range [][]string{
		{"src/go/types/errorhandling.go", "src/go/types/optionresult.go"},
		{"src/cmd/compile/internal/types2/errorexpr.go", "src/cmd/compile/internal/types2/optionresult.go"},
	} {
		var src strings.Builder
		for _, file := range files {
			data, err := os.ReadFile(filepath.Join(runtime.GOROOT(), file))
			if err != nil {
				t.Fatal(err)
			}
			src.Write(data)
		}
		for _, c := range ex.Explanations[0].Cases {
			if !strings.Contains(src.String(), strconv.Quote(c.Message)) {
				t.Errorf("%s do not report %q", files, c.Message)
			}
		}
	}
}

func TestGonCapabilities(t *testing.T) {
	t.Parallel()
	tree := writeTree(t, "")
	var caps struct {
		SchemaVersion int
		Operation     string
		Capabilities  struct {
			Queries                            []string
			Rename, Check, Persistent, Explain bool
		}
		Toolchain struct{ Root, LanguageServer string }
	}
	gonJSON(t, tree, nil, &caps, "capabilities").checkCode(0)
	if caps.SchemaVersion != 1 || caps.Operation != "capabilities" || !caps.Capabilities.Rename || !caps.Capabilities.Check ||
		caps.Capabilities.Persistent ||
		len(caps.Capabilities.Queries) != 5 || !filepath.IsAbs(caps.Toolchain.Root) ||
		caps.Toolchain.LanguageServer != filepath.Join(caps.Toolchain.Root, "gon", "bin", "gonpls") {
		t.Errorf("capabilities: %+v", caps)
	}
	res := gon(t, tree, nil, "help", "query", "refs")
	res.checkCode(0)
	res.checkStdout("usage: gon query refs <target>...")
}
