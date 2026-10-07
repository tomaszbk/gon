// run

//go:build !js && !wasip1 && gc

// Execute both spellings of the same scenarios and compare their checked traces.
// Set GON_BASELINE_GO to a compatible unmodified go executable to
// additionally verify the legacy program against the upstream language.
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
	fixtures := filepath.Join(runtime.GOROOT(), "test", "errorhandling.dir")
	legacy := run(goTool, false, "run", filepath.Join(fixtures, "common.go"), filepath.Join(fixtures, "legacy.go"))
	modern := run(goTool, false, "run", filepath.Join(fixtures, "common.go"), filepath.Join(fixtures, "modern.go"))
	if !bytes.Equal(legacy, modern) {
		panic(fmt.Sprintf("legacy/modern behavior differs\nlegacy:\n%s\nmodern:\n%s", legacy, modern))
	}
	unoptimized := run(goTool, false, "run", "-gcflags=-l", filepath.Join(fixtures, "common.go"), filepath.Join(fixtures, "modern.go"))
	if !bytes.Equal(legacy, unoptimized) {
		panic(fmt.Sprintf("legacy/modern behavior without inlining differs\nlegacy:\n%s\nmodern:\n%s", legacy, unoptimized))
	}
	run(goTool, false, "vet", filepath.Join(fixtures, "common.go"), filepath.Join(fixtures, "modern.go"))
	if baseline := os.Getenv("GON_BASELINE_GO"); baseline != "" {
		upstream := run(baseline, true, "run", filepath.Join(fixtures, "common.go"), filepath.Join(fixtures, "legacy.go"))
		if !bytes.Equal(legacy, upstream) {
			panic(fmt.Sprintf("legacy baseline behavior differs\nfork:\n%s\nupstream:\n%s", legacy, upstream))
		}
	}
	checkExports(goTool, fixtures)
	checkVetErrorPaths(goTool)

	dir, err := os.MkdirTemp("", "go-errorhandling-invalid-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	for _, test := range invalid {
		file := filepath.Join(dir, test.name+".go")
		if err := os.WriteFile(file, []byte("package invalid\n"+test.source), 0600); err != nil {
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

func checkExports(goTool, fixtures string) {
	dir, err := os.MkdirTemp("", "go-errorhandling-exports-")
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
	write("go.mod", []byte("module errorhandling\n\ngo 1.23\n"))
	copyFixture("export_main.go", "main.go")
	copyFixture("export_legacy.go", filepath.Join("lib", "lib.go"))
	legacy := runAt(goTool, false, dir, "run", ".")
	if baseline := os.Getenv("GON_BASELINE_GO"); baseline != "" {
		upstream := runAt(baseline, true, dir, "run", ".")
		if !bytes.Equal(legacy, upstream) {
			panic("cross-package legacy baseline behavior differs")
		}
	}
	copyFixture("export_modern.go", filepath.Join("lib", "lib.go"))
	modern := runAt(goTool, false, dir, "run", ".")
	if !bytes.Equal(legacy, modern) {
		panic(fmt.Sprintf("cross-package legacy/modern behavior differs\nlegacy:\n%s\nmodern:\n%s", legacy, modern))
	}
	runAt(goTool, false, dir, "vet", "./...")
}

// Vet's control-flow graph must include both the success and the early-return
// path, including when cancellation happens only in a local error handler.
func checkVetErrorPaths(goTool string) {
	dir, err := os.MkdirTemp("", "go-errorhandling-vet-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	for _, test := range []struct {
		name, body string
		wantLeak   bool
	}{
		{"propagation_leak", `v := source()!; cancel(); return v, nil`, true},
		{"handler_success_leak", `v := source() or err { cancel(); return 0, err }; return v, nil`, true},
		{"deferred_cancel", `defer cancel(); v := source()!; return v, nil`, false},
	} {
		file := filepath.Join(dir, test.name+".go")
		source := `package vetcase
import "context"
func source() (int, error) { return 1, nil }
func f() (int, error) {
    ctx, cancel := context.WithCancel(context.Background())
    _ = ctx
    ` + test.body + "\n}\n"
		if err := os.WriteFile(file, []byte(source), 0600); err != nil {
			panic(err)
		}
		cmd := exec.Command(goTool, "vet", "-lostcancel", file)
		cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local")
		out, err := cmd.CombinedOutput()
		if test.wantLeak {
			if err == nil || !bytes.Contains(out, []byte("cancel function is not used on all paths")) || bytes.Contains(out, []byte("panic:")) {
				panic(fmt.Sprintf("vet did not identify %s cancellation leak: %v\n%s", test.name, err, out))
			}
		} else if err != nil {
			panic(fmt.Sprintf("vet rejected safe %s: %v\n%s", test.name, err, out))
		}
	}
}

var invalid = []struct{ name, source string }{
	{"noncall", `func f() error { _ = 1!; return nil }`},
	{"no_error_result", `func source() int { return 1 }; func f() error { _ = source()!; return nil }`},
	{"nonfinal_error", `func source() (error, int) { return nil, 1 }; func f() error { _ = source()!; return nil }`},
	{"concrete_error", `type concrete struct{}; func (concrete) Error() string { return "" }; func source() (int, concrete) { return 1, concrete{} }; func f() error { _ = source()!; return nil }`},
	{"distinct_error_interface", `type other interface{ Error() string }; func source() (int, other) { return 1, nil }; func f() error { _ = source()!; return nil }`},
	{"no_enclosing_error", `func source() (int, error) { return 1, nil }; func f() int { return source()! }`},
	{"no_enclosing_results", `func source() error { return nil }; func f() { source()! }`},
	{"wrong_enclosing_error", `type other interface{ Error() string }; func source() error { return nil }; func f() other { source()!; return nil }`},
	{"package_initializer", `func source() (int, error) { return 1, nil }; var v = source()!`},
	{"nested_wrong_return", `func source() error { return nil }; func outer() error { func() { source()! }(); return nil }`},
	{"double_propagation", `func source() (int, error) { return 1, nil }; func f() error { _ = source()!!; return nil }`},
	{"missing_binding", `func source() error { return nil }; func f() { source() or { return } }`},
	{"handler_fallthrough", `func source() (int, error) { return 1, nil }; func f() int { v := source() or err { _ = err }; return v }`},
	{"handler_partial_termination", `func source() (int, error) { return 1, nil }; func f(b bool) int { v := source() or err { if b { return 0 } }; return v }`},
	{"binding_scope", `func source() error { return nil }; func f() { source() or problem { _ = problem }; _ = problem }`},
	{"initializer_scope", `func source() (int, error) { return 1, nil }; func f() (int, error) { v := source() or problem { return v, problem }; return v, nil }`},
	{"go_propagation", `func source() error { return nil }; func f() error { go source()!; return nil }`},
	{"defer_propagation", `func source() error { return nil }; func f() error { defer source()!; return nil }`},
	{"handler_outer_continue", `func source() (int, error) { return 1, nil }; func f() { for { _ = source() or err { continue } } }`},
	{"handler_outer_break", `func source() (int, error) { return 1, nil }; func f() { for { _ = source() or err { break } } }`},
	{"handler_goto", `func source() error { return nil }; func f() { source() or err { goto end }; end: }`},
	{"handler_label", `func source() error { return nil }; func f() { source() or err { label: for { break label } } }`},
	{"prefix_newline", "func f(b bool) bool { return !\nb }"},
	{"prefix_line_comment", "func f(b bool) bool { return ! // comment\nb }"},
	{"prefix_block_comment", "func f(b bool) bool { return ! /* comment\n*/ b }"},
	{"prefix_trailing_block", "func f(b bool) bool { return ! /* comment */\nb }"},
}
