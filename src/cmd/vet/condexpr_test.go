package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"internal/testenv"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestCondExpr checks vet on Gon's conditional expressions
// "if c { a } else { b }".
//
// The packages in testdata/condexpr hold the same scenarios twice:
// legacy/legacy.go with if statements, as in standard Go, and
// modern/modern.go with conditional expressions. Each must get exactly the
// diagnostics of its ERROR comments, and corresponding functions must get
// the same diagnostics: the analyzers that depend on control flow (lostcancel)
// or on the copied expression (copylocks) or on dead code (shift) treat the
// branches as alternatives, as in an if statement. modern/smoke.go uses
// conditional expressions in many other contexts and gets no diagnostics,
// so that every analyzer traverses them without crashing.
//
// If GON_BASELINE_GO is set to an unmodified go command,
// its own vet must report the same for the legacy package.
func TestCondExpr(t *testing.T) {
	t.Parallel()

	got := make(map[string][]string)
	for _, variant := range []string{"legacy", "modern"} {
		dir := filepath.Join("testdata", "condexpr", variant)
		output, err := vetCmd(t, uncachedPrintfuncs(t), "condexpr/"+variant).CombinedOutput()
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("%s: vet did not report diagnostics: %v\n%s", variant, err, output)
		}
		checkNoCrash(t, variant, output)
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		var fullshort []string
		for _, f := range files {
			fullshort = append(fullshort, f, filepath.Base(f))
		}
		if err := errorCheck(string(output), false, fullshort...); err != nil {
			t.Errorf("%s: error check failed: %s", variant, err)
		}
		got[variant] = scenarioDiagnostics(t, filepath.Join(dir, variant+".go"), output)
	}
	if len(got["legacy"]) == 0 {
		t.Fatal("no diagnostics for the legacy scenarios")
	}
	if !slices.Equal(got["legacy"], got["modern"]) {
		t.Errorf("legacy and modern scenarios get different diagnostics\nlegacy:\n\t%s\nmodern:\n\t%s",
			strings.Join(got["legacy"], "\n\t"), strings.Join(got["modern"], "\n\t"))
	}

	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		t.Log("GON_BASELINE_GO is not set; the legacy scenarios were not vetted with an unmodified toolchain")
		return
	}
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "condexpr", "legacy", "legacy.go"))
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"go.mod": []byte("module condexpr\n\ngo 1.26\n"), "legacy.go": src} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o666); err != nil {
			t.Fatal(err)
		}
	}
	cmd := testenv.Command(t, baseline, "vet", ".")
	cmd.Dir = dir
	for _, kv := range os.Environ() {
		switch {
		case strings.HasPrefix(kv, "GOROOT="), strings.HasPrefix(kv, "GOTOOLDIR="),
			strings.HasPrefix(kv, "GOFLAGS="), strings.HasPrefix(kv, "GOENV="),
			strings.HasPrefix(kv, "GOTOOLCHAIN="), strings.HasPrefix(kv, "GO_VETTEST_IS_VET="):
		default:
			cmd.Env = append(cmd.Env, kv)
		}
	}
	cmd.Env = append(cmd.Env, "GOENV=off", "GOTOOLCHAIN=local", "GOFLAGS=", "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if _, ok := err.(*exec.ExitError); !ok {
		t.Fatalf("baseline vet did not report diagnostics: %v\n%s", err, output)
	}
	upstream := scenarioDiagnostics(t, filepath.Join(dir, "legacy.go"), output)
	if !slices.Equal(got["legacy"], upstream) {
		t.Errorf("the baseline toolchain %s reports different diagnostics for the legacy scenarios\nfork:\n\t%s\nbaseline:\n\t%s",
			baseline, strings.Join(got["legacy"], "\n\t"), strings.Join(upstream, "\n\t"))
	} else {
		t.Logf("the baseline toolchain %s reports the same %d diagnostics for the legacy scenarios", baseline, len(upstream))
	}
}

// uncachedPrintfuncs returns a -printfuncs flag that also names a function
// unique to this test binary. A test binary reports an empty build ID to the
// go command, which then keys its cache of vet results on the inputs and the
// vet flags only, and would replay the diagnostics of an earlier build.
func uncachedPrintfuncs(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(vetPath(t))
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("-printfuncs=Warn,Warnf,vettest%x", sha256.Sum256(data))
}

func checkNoCrash(t *testing.T, variant string, output []byte) {
	t.Helper()
	for _, crash := range []string{"panic:", "goroutine ", "internal error", "unexpected node type"} {
		if bytes.Contains(output, []byte(crash)) {
			t.Fatalf("%s: vet crashed (%q):\n%s", variant, crash, output)
		}
	}
}

var diagnosticRE = regexp.MustCompile(`^(?:\./)?(\S+\.go):(\d+):\d+: (.*)$`)

// scenarioDiagnostics returns the diagnostics that vet reported in output
// for file, as "function: message" with line numbers in the message
// replaced by N, sorted.
func scenarioDiagnostics(t *testing.T, file string, output []byte) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	enclosing := func(line int) string {
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fset.Position(fn.Pos()).Line <= line && line <= fset.Position(fn.End()).Line {
				return fn.Name.Name
			}
		}
		return "?"
	}
	lineRE := regexp.MustCompile(`line \d+`)
	var diags []string
	for _, line := range strings.Split(string(output), "\n") {
		m := diagnosticRE.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || filepath.Base(m[1]) != filepath.Base(file) {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		diags = append(diags, enclosing(n)+": "+lineRE.ReplaceAllString(m[3], "line N"))
	}
	slices.Sort(diags)
	return diags
}
