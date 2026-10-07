// run

//go:build !js && !wasip1 && gc

// Execute both spellings of the same conditional expression scenarios and
// compare their checked traces. Set GON_BASELINE_GO to a compatible unmodified
// go executable to additionally verify the legacy programs against the upstream
// language.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	goTool := filepath.Join(runtime.GOROOT(), "bin", "go")
	fixtures := filepath.Join(runtime.GOROOT(), "test", "conditional.dir")
	common := filepath.Join(fixtures, "common.go")
	legacyFile := filepath.Join(fixtures, "legacy.go")
	modernFile := filepath.Join(fixtures, "modern.go")

	legacy := run(goTool, false, "run", common, legacyFile)
	compare("legacy/modern", legacy, run(goTool, false, "run", common, modernFile))
	compare("legacy/modern without inlining", legacy, run(goTool, false, "run", "-gcflags=-l", common, modernFile))
	compare("legacy/modern without optimizations", legacy, run(goTool, false, "run", "-gcflags=-N -l", common, modernFile))
	run(goTool, false, "vet", common, modernFile)
	if baseline := baselineGo(); baseline != "" {
		compare("legacy baseline", legacy, run(baseline, true, "run", common, legacyFile))
	}
	checkFormat(fixtures)
	checkExports(goTool, fixtures)
	checkInvalid(goTool)
}

func baselineGo() string {
	return os.Getenv("GON_BASELINE_GO")
}

func compare(what string, want, got []byte) {
	if !bytes.Equal(want, got) {
		panic(fmt.Sprintf("%s behavior differs\nwant:\n%s\ngot:\n%s", what, want, got))
	}
}

func run(goTool string, baseline bool, args ...string) []byte {
	return runAt(goTool, baseline, "", args...)
}

func runAt(goTool string, baseline bool, dir string, args ...string) []byte {
	cmd := exec.Command(goTool, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local")
	if dir != "" {
		cmd.Env = append(cmd.Env, "GO111MODULE=on")
	}
	if baseline {
		var env []string
		for _, entry := range cmd.Env {
			if !strings.HasPrefix(entry, "GOROOT=") && !strings.HasPrefix(entry, "GOTOOLDIR=") {
				env = append(env, entry)
			}
		}
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("%s %v: %v\n%s", goTool, args, err, out))
	}
	return out
}

// gofmt keeps the single-line and multi-line conditional expressions of the
// modern fixtures as they are written.
func checkFormat(fixtures string) {
	gofmt := filepath.Join(runtime.GOROOT(), "bin", "gofmt")
	files, err := filepath.Glob(filepath.Join(fixtures, "*.go"))
	if err != nil {
		panic(err)
	}
	if out := run(gofmt, false, append([]string{"-l"}, files...)...); len(out) != 0 {
		panic(fmt.Sprintf("fixtures are not gofmt-formatted:\n%s", out))
	}
}

// checkExports runs a program using a package whose exported functions,
// generic functions, methods, constants and variables use conditional
// expressions, and requires that importing packages inline the small
// functions through export data, as they do for the legacy spelling.
func checkExports(goTool, fixtures string) {
	dir, err := os.MkdirTemp("", "go-conditional-exports-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if err := os.Mkdir(filepath.Join(dir, "lib"), 0700); err != nil {
		panic(err)
	}
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			panic(err)
		}
	}
	copyFixture := func(source, destination string) {
		data, err := os.ReadFile(filepath.Join(fixtures, source))
		if err != nil {
			panic(err)
		}
		write(destination, data)
	}
	inlined := func(variant string) {
		out := runAt(goTool, false, dir, "build", "-gcflags=-m", "-o", filepath.Join(dir, "main.exe"), ".")
		lines := strings.Split(string(out), "\n")
	calls:
		for _, call := range []string{
			"lib.Pick", "lib.Label", "lib.Lazy", "lib.ErrorOf", "lib.Value",
			"lib.Generic[go.shape.string]", "lib.Generic[go.shape.float64]", "lib.Generic[go.shape.*uint8]",
			"lib.OrDefault[", "lib.Box[go.shape.string].Get", "lib.Box[go.shape.int].Get",
		} {
			for _, line := range lines {
				if strings.Contains(line, "main.go:") && strings.Contains(line, "inlining call to "+call) {
					continue calls
				}
			}
			panic(fmt.Sprintf("%s: call to %s was not inlined across packages:\n%s", variant, call, out))
		}
	}
	write("go.mod", []byte("module conditional\n\ngo 1.26\n"))
	copyFixture("export_main.go", "main.go")
	copyFixture("export_legacy.go", filepath.Join("lib", "lib.go"))
	legacy := runAt(goTool, false, dir, "run", ".")
	if baseline := baselineGo(); baseline != "" {
		compare("cross-package legacy baseline", legacy, runAt(baseline, true, dir, "run", "."))
	}
	inlined("legacy")
	copyFixture("export_modern.go", filepath.Join("lib", "lib.go"))
	compare("cross-package legacy/modern", legacy, runAt(goTool, false, dir, "run", "."))
	compare("cross-package legacy/modern without inlining", legacy, runAt(goTool, false, dir, "run", "-gcflags=all=-l", "."))
	inlined("modern")
	runAt(goTool, false, dir, "vet", "./...")
}

// checkInvalid compiles invalid uses of conditional expressions, which must
// be rejected with a source diagnostic, never an internal compiler error.
func checkInvalid(goTool string) {
	dir, err := os.MkdirTemp("", "go-conditional-invalid-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	for _, test := range invalid {
		file := filepath.Join(dir, test.name+".go")
		if err := os.WriteFile(file, []byte("package invalid\n"+test.source+"\n"), 0600); err != nil {
			panic(err)
		}
		cmd := exec.Command(goTool, "tool", "compile", "-e", "-o", filepath.Join(dir, "invalid.o"), file)
		out, err := cmd.CombinedOutput()
		if err == nil {
			panic(fmt.Sprintf("invalid construct %s compiled successfully:\n%s", test.name, test.source))
		}
		if !bytes.Contains(out, []byte(test.name+".go:")) || bytes.Contains(out, []byte("internal compiler error")) || bytes.Contains(out, []byte("panic:")) {
			panic(fmt.Sprintf("invalid construct %s did not produce a source diagnostic: %v\n%s", test.name, err, out))
		}
		if !bytes.Contains(out, []byte(test.want)) {
			panic(fmt.Sprintf("invalid construct %s: diagnostic does not contain %q:\n%s", test.name, test.want, out))
		}
	}
}

// Shared declarations for the invalid programs, which are compiled without
// imports.
const reader = `type Reader interface{ Read([]byte) (int, error) }
type File struct{}
func (*File) Read([]byte) (int, error) { return 0, nil }
type Buffer struct{}
func (*Buffer) Read([]byte) (int, error) { return 0, nil }
`

const myErr = `type E struct{}
func (*E) Error() string { return "" }
`

var invalid = []struct{ name, source, want string }{
	// Syntax.
	{"missing_else", `func f(c bool) int { return if c { 1 } }`, "conditional expression requires an else branch"},
	{"missing_else_multiline", "func f(c bool) int {\n\treturn if c {\n\t\t1\n\t}\n}", "conditional expression requires an else branch"},
	{"chained", `func f(c, d bool) int { return if c { 1 } else if d { 2 } else { 3 } }`, "conditional expressions cannot be chained or nested; use a switch statement"},
	{"nested_branch", `func f(c, d bool) int { return if c { 1 } else { if d { 2 } else { 3 } } }`, "conditional expressions cannot be chained or nested; use a switch statement"},
	{"nested_branch_parenthesized", `func f(c, d bool) int { return if c { (if d { 2 } else { 3 }) } else { 1 } }`, "conditional expressions cannot be chained or nested; use a switch statement"},
	{"nested_condition", `func f(c, d bool) int { return if (if c { d } else { !d }) { 1 } else { 2 } }`, "conditional expressions cannot be chained or nested; use a switch statement"},
	{"init_statement", `func f(m map[string]int) int { return if v, ok := m["k"]; ok { v } else { 0 } }`, "conditional expression cannot have an init statement"},
	{"assignment_in_branch", `func f(c bool) int { x := 0; return if c { x = 1 } else { 2 } }`, "conditional expression branch must be a single expression"},
	{"statements_in_branch", `func g() int { return 1 }; func f(c bool) int { return if c { g(); g() } else { 2 } }`, "conditional expression branch must be a single expression"},
	{"declaration_in_branch", `func f(c bool) int { return if c { var x = 1 } else { 2 } }`, "conditional expression branch must be a single expression"},
	{"empty_branch", `func f(c bool) int { return if c { } else { 2 } }`, "conditional expression branch must be a single expression"},

	// Branch values and types.
	{"multi_value_branch", `func two() (int, error) { return 1, nil }; func f(c bool) int { return if c { two() } else { 2 } }`, "multiple-value two()"},
	{"multi_value_assignment", `func two() (int, error) { return 1, nil }; func f(c bool) (int, error) { v, err := if c { two() } else { two() }; return v, err }`, "multiple-value two()"},
	{"no_value_branch", `func none() {}; func f(c bool) int { return if c { none() } else { 2 } }`, "none() (no value) used as value"},
	{"mismatched_untyped", `func f(c bool) { x := if c { 1 } else { "one" }; _ = x }`, "mismatched types untyped int and untyped string in conditional expression"},
	{"mismatched_typed", `func f(c bool, a int, b string) { x := if c { a } else { b }; _ = x }`, "mismatched types int and string in conditional expression"},
	{"mismatched_interface", reader + `func f(c bool, file *File, buf *Buffer) { r := if c { Reader(file) } else { buf }; _ = r }`, "mismatched types Reader and *Buffer in conditional expression"},
	{"interface_untyped_kinds", `func f(c bool) any { return any(if c { 1 } else { "one" }) }`, "mismatched types untyped int and untyped string in conditional expression"},
	{"untyped_nils", `func f(c bool) { x := if c { nil } else { nil }; _ = x }`, "use of untyped nil in conditional expression"},
	{"type_parameter_nil", `func f[T any](c bool, x T) T { return if c { x } else { nil } }`, "cannot use nil as T value in conditional expression"},
	{"generic_nil_branch", myErr + `func g[T, U any](a U, b T) T { return b }; func f(c bool, p *E) error { return g[error](1, if c { p } else { nil }) }`, "cannot use conditional expression with nil branch as error value"},
	{"generic_inferred_mismatch", myErr + `func g[T any](a, b T) T { return a }; func f(c bool, err error, p *E) error { return g(err, if c { p } else { nil }) }`, "does not match inferred type error for T"},

	// A value, not a variable.
	{"address_of_result", `func f(c bool, a, b int) *int { return &(if c { a } else { b }) }`, "cannot take address of"},
	{"assign_to_result", `func f(c bool, a, b int) { (if c { a } else { b }) = 1 }`, "cannot assign to"},
	{"pointer_method_on_result", `type T struct{}; func (*T) M() {}; func f(c bool, a, b T) { (if c { a } else { b }).M() }`, "cannot call pointer method M on T"},
	{"statement_position", `func f(c bool) { if c { 1 } else { 2 } }`, "is not used"},

	// Constants: both branches are checked.
	{"unselected_division_by_zero", `const n, d = 1, 0; const r = if d != 0 { n / d } else { 0 }`, "division by zero"},
	{"unselected_overflow", `func f(c bool) byte { return if c { 1 } else { 300 } }`, "overflows"},
	{"negative_shift_count", `func f(c bool) int { return 1 << if c { -1 } else { 2 } }`, "overflows uint"},
	{"constant_mismatch", `const c = if true { 1 } else { "one" }`, "mismatched types untyped int and untyped string in conditional expression"},
	{"nonconstant_array_length", `func f(c bool) { var a [if c { 1 } else { 2 }]int; _ = a }`, "must be constant"},

	// Conditions.
	{"non_boolean_condition", `func f() int { return if 1 { 2 } else { 3 } }`, "non-boolean condition in conditional expression"},
	{"non_boolean_typed_condition", `func f(n int) int { return if n { 2 } else { 3 } }`, "non-boolean condition in conditional expression"},

	// Error handling inside a branch keeps its rules.
	{"propagation_without_error_result", `func source() (int, error) { return 1, nil }; func f(c bool) int { return if c { source()! } else { 0 } }`, "error propagation requires an enclosing function"},
	{"handler_fallthrough", `func source() (int, error) { return 1, nil }; func f(c bool) int { return if c { source() or err { _ = err } } else { 0 } }`, "must terminate"},
	{"package_propagation", `func source() (int, error) { return 1, nil }; var v = if true { source()! } else { 0 }`, "only permitted inside a function"},
}
