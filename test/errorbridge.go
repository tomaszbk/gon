// run

//go:build !js && !wasip1 && gc

// Postfix ! across Go error tuples and Result: a tuple whose last result is
// error fails as Result.Err inside a function returning one Result, and a
// failed Result fails as its error inside a function returning error last. A
// nil Err payload is still a failure: it becomes errors.ErrNilResult.
//
// The legacy program spells the same scenarios without Result and runs on the
// baseline toolchain too. GON_BASELINE_GO must name that unmodified Go.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

func main() {
	tool := filepath.Join(runtime.GOROOT(), "bin", "go")
	dir := filepath.Join(runtime.GOROOT(), "test", "errorbridge.dir")
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		panic("GON_BASELINE_GO must name an unmodified compatible Go toolchain")
	}
	runAt := func(workdir, command string, args ...string) []byte {
		cmd := exec.Command(command, args...)
		cmd.Dir = workdir
		for _, e := range os.Environ() {
			if strings.HasPrefix(e, "GOROOT=") || strings.HasPrefix(e, "GOTOOLDIR=") || strings.HasPrefix(e, "GOFLAGS=") {
				continue
			}
			cmd.Env = append(cmd.Env, e)
		}
		cmd.Env = append(cmd.Env, "GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off")
		out, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("%s %v: %v\n%s", command, args, err, out))
		}
		return out
	}
	run := func(command string, args ...string) []byte { return runAt("", command, args...) }

	common, legacy, modern := filepath.Join(dir, "common.go"), filepath.Join(dir, "legacy.go"), filepath.Join(dir, "modern.go")
	want := run(tool, "run", common, legacy)
	if len(want) == 0 {
		panic("the legacy program printed nothing")
	}
	for name, got := range map[string][]byte{
		"baseline legacy":         run(baseline, "run", common, legacy),
		"modern":                  run(tool, "run", common, modern),
		"modern without inlining": run(tool, "run", "-gcflags=-l", common, modern),
	} {
		if !bytes.Equal(want, got) {
			panic(fmt.Sprintf("%s differs from the legacy program\nwant:\n%s\ngot:\n%s", name, want, got))
		}
	}
	run(tool, "vet", common, modern)

	checkExports(tool, baseline, dir, runAt)
	checkInvalid(tool)
}

// checkExports builds a library and a main package, so that propagation is
// also exercised across the export data and inlining of another package.
func checkExports(tool, baseline, fixtures string, runAt func(string, string, ...string) []byte) {
	dir, err := os.MkdirTemp("", "gon-errorbridge-exports-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if err := os.Mkdir(filepath.Join(dir, "lib"), 0700); err != nil {
		panic(err)
	}
	copyFixture := func(source, destination string) {
		data, err := os.ReadFile(filepath.Join(fixtures, source))
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(dir, destination), data, 0600); err != nil {
			panic(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module errorbridge\n\ngo 1.23\n"), 0600); err != nil {
		panic(err)
	}
	copyFixture("export_legacy_main.go", "main.go")
	copyFixture("export_legacy.go", filepath.Join("lib", "lib.go"))
	legacy := runAt(dir, tool, "run", ".")
	if len(legacy) == 0 {
		panic("the legacy export program printed nothing")
	}
	if upstream := runAt(dir, baseline, "run", "."); !bytes.Equal(legacy, upstream) {
		panic(fmt.Sprintf("cross-package legacy baseline behavior differs\nfork:\n%s\nupstream:\n%s", legacy, upstream))
	}
	copyFixture("export_modern_main.go", "main.go")
	copyFixture("export_modern.go", filepath.Join("lib", "lib.go"))
	for name, flags := range map[string][]string{"optimized": nil, "without inlining": {"-gcflags=all=-l"}} {
		modern := runAt(dir, tool, append(append([]string{"run"}, flags...), ".")...)
		if !bytes.Equal(legacy, modern) {
			panic(fmt.Sprintf("cross-package legacy/modern behavior differs (%s)\nlegacy:\n%s\nmodern:\n%s", name, legacy, modern))
		}
	}
	runAt(dir, tool, "vet", "./...")
}

// Invalid contexts must be rejected by both type checkers (the compiler and
// vet) with a source diagnostic naming the file, never an internal compiler
// error.
func checkInvalid(tool string) {
	const prelude = `
import "testing"
var _ testing.TB
func tuple() (int, error)  { return 1, nil }
func only() error          { return nil }
func plain() int           { return 1 }
func text() Result[int, string] { return .Ok(1) }
func fine() Result[int, error]  { return .Ok(1) }
type concrete struct{}
func (concrete) Error() string { return "" }
`
	var wg sync.WaitGroup
	limit := make(chan struct{}, 4)
	for _, test := range invalid {
		wg.Add(1)
		limit <- struct{}{}
		go func() {
			defer func() { <-limit; wg.Done() }()
			dir, err := os.MkdirTemp("", "gon-errorbridge-invalid-")
			if err != nil {
				panic(err)
			}
			defer os.RemoveAll(dir)
			file := "main.go"
			if test.testFile {
				file = "main_test.go"
			}
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module invalid\n\ngo 1.23\n"), 0600); err != nil {
				panic(err)
			}
			src := "package invalid\n" + prelude + test.source + "\n"
			if err := os.WriteFile(filepath.Join(dir, file), []byte(src), 0600); err != nil {
				panic(err)
			}
			for _, args := range [][]string{{"vet", "./..."}, {"test", "-vet=off", "-run=^$", "./..."}} {
				cmd := exec.Command(tool, args...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=")
				out, err := cmd.CombinedOutput()
				if err == nil {
					panic(fmt.Sprintf("go %v accepted invalid construct %s:\n%s", args, test.name, test.source))
				}
				if !bytes.Contains(out, []byte(file+":")) || !bytes.Contains(out, []byte(test.want)) ||
					bytes.Contains(out, []byte("internal compiler error")) || bytes.Contains(out, []byte("panic:")) {
					panic(fmt.Sprintf("go %v did not report %q for %s:\n%s\n%s", args, test.want, test.name, test.source, out))
				}
			}
		}()
	}
	wg.Wait()
}

var invalid = []struct {
	name, source, want string
	testFile           bool
}{
	{"result_error_string_into_error", `func f() error { _ = text()!; return nil }`, "assignable to error", false},
	{"result_into_nonfinal_error", `func f() (error, int) { n := fine()!; return nil, n }`, "enclosing Result", false},
	{"result_in_plain_function", `func f() int { return fine()! }`, "enclosing Result", false},
	{"tuple_into_result_string", `func f() Result[int, string] { n := tuple()!; return .Ok(n) }`, "assignable to its error type", false},
	{"tuple_into_result_concrete", `func f() Result[int, concrete] { n := tuple()!; return .Ok(n) }`, "assignable to its error type", false},
	{"tuple_with_result_and_extra", `func f() (Result[int, error], int) { n := tuple()!; return .Ok(n), 0 }`, "exactly one Result", false},
	{"tuple_in_plain_function", `func f() int { return tuple()! }`, "enclosing function", false},
	{"non_error_operand_in_result", `func f() Result[int, error] { n := plain()!; return .Ok(n) }`, "final result of type error", false},
	{"defer_in_result_function", `func f() Result[int, error] { defer only()!; return .Ok(1) }`, "expression in defer must be function call", false},
	{"test_rule_needs_test_file", `func f(t *testing.T) { only()! }`, "a _test.go file", false},
	{"test_rule_result_needs_test_file", `func f(t *testing.T) { _ = fine()! }`, "a _test.go file", false},
	{"test_blank_parameter", `func f(_ *testing.T) { only()! }`, "first named parameter", true},
	{"test_second_parameter", `func f(n int, t *testing.T) { only()! }`, "first named parameter", true},
	{"test_other_type", `func f(t *concrete) { only()! }`, "first named parameter", true},
	{"test_value_type", `func f(t testing.T) { only()! }`, "first named parameter", true},
	{"test_receiver_is_not_a_parameter", `type S struct{ t *testing.T }; func (s S) g() { only()! }`, "first named parameter", true},
	{"test_nested_function_without_parameter", `func f(t *testing.T) { func() { only()! }() }`, "first named parameter", true},
	{"test_result_keeps_result_rule", `func f(t *testing.T) Result[int, string] { n := tuple()!; return .Ok(n) }`, "assignable to its error type", true},
	{"test_error_keeps_error_rule", `func f(t *testing.T) error { _ = text()!; return nil }`, "assignable to error", true},
}
