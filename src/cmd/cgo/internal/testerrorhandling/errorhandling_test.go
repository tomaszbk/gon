// Package testerrorhandling checks that cgo handles the error-handling syntax
// (postfix ! and "or err { ... }") on C calls. A C call consumed by either
// construct uses cgo's two-result form, so the error is errno (a
// syscall.Errno) or nil, exactly as in "r, err := C.f()".
//
// The executable pair testdata/legacy.go and testdata/modern.go implements the
// same scenarios with explicit error checks and with the new syntax. Both
// programs, together with testdata/common.go and testdata/clib.c, must build and
// pass their own assertions, and must print identical output. Set
// GON_BASELINE_GO to an unmodified go command to additionally run
// the legacy program with it and compare the output with this toolchain.
package testerrorhandling

import (
	"bytes"
	"internal/testenv"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// newModule returns a directory holding a module made of the shared test files
// and, if program is not empty, the named testdata file as prog.go.
func newModule(t *testing.T, program string) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o666); err != nil {
			t.Fatal(err)
		}
	}
	copyFile := func(src, dst string) {
		data, err := os.ReadFile(filepath.Join("testdata", src))
		if err != nil {
			t.Fatal(err)
		}
		write(dst, data)
	}
	write("go.mod", []byte("module errorhandlingcgo\n\ngo 1.26\n"))
	for _, name := range []string{"common.go", "clib.h", "clib.c"} {
		copyFile(name, name)
	}
	if program != "" {
		copyFile(program, "prog.go")
	}
	return dir
}

// goCommand runs goTool in dir. A baseline toolchain is run without the
// environment variables that would point it at this repository's GOROOT.
func goCommand(t *testing.T, goTool string, baseline bool, dir string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := exec.Command(goTool, args...)
	cmd.Dir = dir
	for _, entry := range os.Environ() {
		if baseline && (strings.HasPrefix(entry, "GOROOT=") || strings.HasPrefix(entry, "GOTOOLDIR=")) {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "GOENV=off", "GOFLAGS=", "GOTOOLCHAIN=local", "GO111MODULE=on")
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err = cmd.Run()
	return out.String(), errOut.String(), err
}

func runProgram(t *testing.T, goTool string, baseline bool, program string) string {
	t.Helper()
	dir := newModule(t, program)
	stdout, stderr, err := goCommand(t, goTool, baseline, dir, "run", ".")
	if err != nil {
		t.Fatalf("%s (baseline=%v): go run: %v\nstdout:\n%s\nstderr:\n%s", program, baseline, err, stdout, stderr)
	}
	if !strings.HasSuffix(stdout, "\nPASS\n") {
		t.Fatalf("%s (baseline=%v): program did not report PASS:\n%s", program, baseline, stdout)
	}
	return stdout
}

func TestPairedCgoErrorHandling(t *testing.T) {
	testenv.MustHaveGoRun(t)
	testenv.MustHaveCGO(t)
	goTool := testenv.GoToolPath(t)

	legacy := runProgram(t, goTool, false, "legacy.go")
	modern := runProgram(t, goTool, false, "modern.go")
	if legacy != modern {
		t.Errorf("legacy and modern programs behave differently\nlegacy:\n%s\nmodern:\n%s", legacy, modern)
	}

	t.Run("vet", func(t *testing.T) {
		dir := newModule(t, "modern.go")
		if stdout, stderr, err := goCommand(t, goTool, false, dir, "vet", "."); err != nil {
			t.Errorf("go vet: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}
	})

	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		t.Log("GON_BASELINE_GO is not set; the legacy program was not run with an unmodified toolchain")
		return
	}
	upstream := runProgram(t, baseline, true, "legacy.go")
	if legacy != upstream {
		t.Errorf("legacy program behaves differently with the baseline toolchain %s\nfork:\n%s\nbaseline:\n%s", baseline, legacy, upstream)
	} else {
		t.Logf("legacy program output is identical with the baseline toolchain %s", baseline)
	}
}

const invalidPrologue = `package main

/*
#include <errno.h>
#include <stdlib.h>
typedef struct { int *p; int n; } c_buf;
static int c_div(int a, int b) { if (b == 0) { errno = EDOM; return -1; } return a / b; }
static void c_touch(int x) { if (x < 0) errno = EINVAL; }
static int c_bufsum(c_buf *b, int k) { return b->n + k; }
static int c_answer = 42;
*/
import "C"

import "strconv"

var _ = strconv.Atoi

func main() { _ = f }

`

// diagnostics holds programs with invalid uses of the new syntax in a cgo file.
// Each must fail with an ordinary source diagnostic that matches want, not with
// a cgo or compiler crash. An empty want marks a valid control program.
var diagnostics = []struct {
	name   string
	source string
	want   string
}{
	// Controls: valid uses next to the invalid ones.
	{"control_propagation", `func f(s string) (int, error) { n := strconv.Atoi(s)!; r := C.c_div(C.int(n), 1)!; return int(r), nil }`, ""},
	{"control_pointer_call", `func f(s string) (int, error) { var b C.c_buf; n := strconv.Atoi(s)!; r := C.c_bufsum(&b, C.int(n))!; return int(r), nil }`, ""},
	{"control_literal_in_pointer_call", `func f(s string) (int, error) { var b C.c_buf; r := C.c_bufsum(&b, C.int(func() int { n := strconv.Atoi(s) or err { return -1 }; return n }()))!; return int(r), nil }`, ""},

	// Enclosing function and handler rules are enforced for C calls too.
	{"c_call_without_error_result", `func f() int { return int(C.c_div(1, 1)!) }`, `error propagation requires an enclosing function with a final result of type error`},
	{"go_call_without_error_result", `func f() int { return strconv.Atoi("1")! }`, `error propagation requires an enclosing function with a final result of type error`},
	{"void_handler_falls_through", `func f() error { C.c_touch(1) or err { _ = err }; return nil }`, `error handler for a value-producing call must terminate`},
	{"value_handler_falls_through", `func f() (int, error) { v := C.c_div(1, 1) or err { _ = err }; return int(v), nil }`, `error handler for a value-producing call must terminate`},
	{"missing_binding", `func f() error { C.c_touch(1) or { return nil }; return nil }`, `expected 'IDENT'`},
	{"double_propagation", `func f() error { _ = C.c_div(1, 1)!!; return nil }`, `error handling requires a function or method call`},
	{"defer_propagation", `func f() error { defer C.c_touch(1)!; return nil }`, `expression in defer must be function call`},
	{"go_propagation", `func f() error { go C.c_touch(1)!; return nil }`, `expression in go must be function call`},
	{"c_variable_operand", `func f() (int, error) { v := C.c_answer!; return int(v), nil }`, `error handling requires a function or method call`},
	{"c_conversion_operand", `func f() (int, error) { v := C.int(3)!; return int(v), nil }`, `error handling requires a function or method call`},

	// C calls without a two-result form, and unknown names, are reported by cgo.
	{"builtin_without_two_result_form", `func f() error { s := C.CString("x")!; _ = s; return nil }`, `no two-result form for C\.CString`},
	{"unknown_c_function", `func f() (int, error) { v := C.no_such(3)!; return int(v), nil }`, `could not determine what C\.no_such refers to`},

	// cgo evaluates the arguments of a call it rewrites for pointer checking
	// inside a function literal, where ! and "or" handlers would return from
	// the literal instead of from the enclosing function.
	{"pointer_call_argument_propagation", `func f(s string) (int, error) { var b C.c_buf; r := C.c_bufsum(&b, C.int(strconv.Atoi(s)!))!; return int(r), nil }`, `cannot use error propagation or an "or" handler in an argument of a C call that cgo must rewrite`},
	{"pointer_call_argument_handler", `func f(s string) (int, error) { var b C.c_buf; r := C.c_bufsum(&b, C.int(strconv.Atoi(s) or err { return 0, err }))!; return int(r), nil }`, `cannot use error propagation or an "or" handler in an argument of a C call that cgo must rewrite`},
	{"pointer_call_argument_c_call", `func f() (int, error) { var b C.c_buf; r := C.c_bufsum(&b, C.c_div(1, 1)!)!; return int(r), nil }`, `cannot use error propagation or an "or" handler in an argument of a C call that cgo must rewrite`},
	{"deferred_pointer_call_argument", `func f(s string) error { var b C.c_buf; defer C.c_bufsum(&b, C.int(strconv.Atoi(s)!)); return nil }`, `cannot use error propagation or an "or" handler in an argument of a C call that cgo must rewrite`},
}

func TestCgoErrorHandlingDiagnostics(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	testenv.MustHaveCGO(t)
	goTool := testenv.GoToolPath(t)

	for _, test := range diagnostics {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for name, contents := range map[string]string{
				"go.mod":     "module invalid\n\ngo 1.26\n",
				"invalid.go": invalidPrologue + test.source + "\n",
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o666); err != nil {
					t.Fatal(err)
				}
			}
			stdout, stderr, err := goCommand(t, goTool, false, dir, "build", "-o", filepath.Join(dir, "invalid.exe"), ".")
			out := stdout + stderr
			if test.want == "" {
				if err != nil {
					t.Fatalf("valid program failed to build: %v\n%s", err, out)
				}
				return
			}
			if err == nil {
				t.Fatalf("invalid program built successfully")
			}
			for _, crash := range []string{"panic:", "goroutine ", "unexpected type", "internal compiler error"} {
				if strings.Contains(out, crash) {
					t.Fatalf("invalid program crashed a tool (%q):\n%s", crash, out)
				}
			}
			re := regexp.MustCompile(`invalid\.go:\d+:\d+: .*` + test.want)
			if !re.MatchString(out) {
				t.Fatalf("output does not report %q at invalid.go:\n%s", test.want, out)
			}
		})
	}
}
