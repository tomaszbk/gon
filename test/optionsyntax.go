// run

//go:build !js && !wasip1 && gc

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
	tool := filepath.Join(runtime.GOROOT(), "bin", "go")
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		panic("GON_BASELINE_GO must name an unmodified compatible Go toolchain")
	}
	dir := filepath.Join(runtime.GOROOT(), "test", "optionsyntax.dir")
	common, legacy, modern := filepath.Join(dir, "common.go"), filepath.Join(dir, "legacy.go"), filepath.Join(dir, "modern.go")
	run := func(command string, args ...string) []byte {
		cmd := exec.Command(command, args...)
		for _, variable := range os.Environ() {
			if !strings.HasPrefix(variable, "GOROOT=") && !strings.HasPrefix(variable, "GOTOOLDIR=") && !strings.HasPrefix(variable, "GOFLAGS=") {
				cmd.Env = append(cmd.Env, variable)
			}
		}
		cmd.Env = append(cmd.Env, "GOENV=off", "GOTOOLCHAIN=local")
		out, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("%s %v: %v\n%s", command, args, err, out))
		}
		return out
	}
	want := run(baseline, "run", common, legacy)
	for _, got := range [][]byte{run(tool, "run", common, legacy), run(tool, "run", common, modern), run(tool, "run", "-gcflags=-l", common, modern), run(tool, "run", "-gcflags=-N -l", common, modern)} {
		if !bytes.Equal(want, got) {
			panic(fmt.Sprintf("optional syntax differs\nwant %s\ngot %s", want, got))
		}
	}
	run(tool, "vet", common, modern)
	boundaryLegacy, boundaryModern := filepath.Join(dir, "boundaries_legacy.go"), filepath.Join(dir, "boundaries_modern.go")
	want = run(baseline, "run", boundaryLegacy)
	for _, got := range [][]byte{run(tool, "run", boundaryLegacy), run(tool, "run", boundaryModern), run(tool, "run", "-gcflags=-l", boundaryModern), run(tool, "run", "-gcflags=-N -l", boundaryModern)} {
		if !bytes.Equal(want, got) {
			panic(fmt.Sprintf("optional syntax boundaries differ\nwant %s\ngot %s", want, got))
		}
	}
	run(tool, "vet", boundaryModern)

	// Optional type expressions in type switch cases, assertions, generic
	// instantiation and conversions; the program prints one line per form.
	typeformsLegacy, typeformsModern := filepath.Join(dir, "typeforms_legacy.go"), filepath.Join(dir, "typeforms_modern.go")
	want = run(baseline, "run", common, typeformsLegacy)
	for _, got := range [][]byte{run(tool, "run", common, typeformsLegacy), run(tool, "run", common, typeformsModern), run(tool, "run", "-gcflags=-l", common, typeformsModern), run(tool, "run", "-gcflags=all=-N -l", common, typeformsModern)} {
		if !bytes.Equal(want, got) {
			panic(fmt.Sprintf("optional type forms differ\nwant %s\ngot %s", want, got))
		}
	}
	run(tool, "vet", common, typeformsModern)

	// Each optional type form must compile on its own in every position that
	// takes a type expression (an earlier compiler crashed in the type switch).
	formsDir, err := os.MkdirTemp("", "gon-option-type-forms-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(formsDir)
	for _, form := range []string{
		"int?", "*int?", "(*int)?", "*(int?)", "[]int?", "([]int)?", "[]*int?", "[][]int?", "(int?)?", "((int?)?)?",
		"map[string]int?", "(map[string]int)?", "map[string](int?)?", "func() int?", "func(int?) int?", "func(...int?)",
		"(func())?", "chan int?", "<-chan int?", "(chan int)?", "[2]int?", "([2]int)?", "struct{ X int? }", "(struct{})?",
		"interface{ M() int? }", "interface{ M() interface{ N() int? }? }", "(interface{ M() })?", "any?", "error?",
		"Box[int?]", "Box[(int?)?]", "Box[[]int?]", "(Box[int])?", "time.Duration?", "*time.Duration?", "fmt.Stringer?",
	} {
		source := "package p\n\nimport (\n\t\"fmt\"\n\t\"time\"\n)\n\nvar (\n\t_ fmt.Stringer\n\t_ time.Duration\n)\n\ntype Box[T any] struct{ V T }\n\n" +
			"func f(x any) {\n" +
			"\tswitch v := x.(type) {\n\tcase " + form + ":\n\t\t_ = v\n\t}\n" +
			"\tswitch v := x.(type) {\n\tcase string, " + form + ":\n\t\t_ = v\n\t}\n" +
			"\tswitch x.(type) {\n\tdefault:\n\tcase " + form + ":\n\t}\n" +
			"\tswitch v := x.(type) {\n\tcase " + form + ":\n\t\tfunc() { _ = v }()\n\tdefault:\n\t}\n" +
			"\t_, _ = x.(" + form + ")\n\t_ = x.(" + form + ")\n}\n\n" +
			"func g[T any](x any) {\n\tswitch v := x.(type) {\n\tcase " + form + ", T?, *T?, []T?:\n\t\t_ = v\n\t}\n}\n"
		filename := filepath.Join(formsDir, "forms.go")
		if err := os.WriteFile(filename, []byte(source), 0600); err != nil {
			panic(err)
		}
		cmd := exec.Command(tool, "build", filename)
		if out, err := cmd.CombinedOutput(); err != nil || bytes.Contains(out, []byte("internal compiler error")) {
			panic(fmt.Sprintf("optional type form %q does not compile: %v\n%s", form, err, out))
		}
	}
	// An anonymous interface that refers to itself through an optional type is
	// diagnosed like the plain cycle, not crashed on or accepted.
	for _, source := range []string{
		"type T interface{ m() interface{ T }? }",
		"type T interface{ m() (interface{ T })? }",
		"type T interface{ m() []interface{ T }? }",
		"type T interface{ m(interface{ T }?) }",
	} {
		filename := filepath.Join(formsDir, "cycle.go")
		if err := os.WriteFile(filename, []byte("package p\n\n"+source+"\n"), 0600); err != nil {
			panic(err)
		}
		out, err := exec.Command(tool, "build", filename).CombinedOutput()
		if err == nil || !bytes.Contains(out, []byte("anonymous interface refers to itself")) || bytes.Contains(out, []byte("internal compiler error")) {
			panic(fmt.Sprintf("optional interface cycle %q: %v\n%s", source, err, out))
		}
	}
	invalid := []string{
		`func f(v int?) bool { nil := 0; return v == nil }`,
		`func f(v (*int)?) bool { return v == (*int)(nil) }`,
		`func f(v ([]int)?) bool { return v == ([]int)(nil) }`,
		`func f(v int?) bool { return v < nil }`,
		`func f(v int?) bool { return nil >= v }`,
		`func f(a,b ([]int)?) bool { return a == b }`,
		`func f(){r:=.Ok(1);_ = r}`,
		`func f(){r:=.Some(1);_ = r}`,
		`func f() any {return .None}`,
		`func f() int? {return .Ok(1)}`,
		`func f() Result[int,error] {return .Err(1)}`,
		`func f() int? {return .Some(nil)}`,
		`func f() int? {return .Some(1,2)}`,
		`func f() int? {return .None()}`,
		`func f() int? {return .Some}`,
		`func f() (int?)? {return 1}`,
		`func f() int?? {return nil}`,
		`func f() Result[port int,error] {panic("unsupported label")}`,
		`type Option[T any] struct{};func f() Option[int] {return .Some(1)}`,
		`type Value enum{default None;Some(int)};func f() Value{return .Some(1)}`,
		`func f() int? {return "bad"}`,
		`func g()(int,int){return 1,2};func f() int? {return .Some(g())}`,
		`func f(v int?)int{return switch v {case .None => 0;case .Some(n) => n}}`,
		`func g[T,E any](x Result[T,E]){};func f(){g(.Ok(1))}`,
		`func g()(int,int){return 1,2};func f(){var a,b int?;a,b=g();_,_=a,b}`,
		`func f(m map[string]int){var value int?;var ok bool;value,ok=m["x"];_,_=value,ok}`,
	}
	tmp, err := os.MkdirTemp("", "gon-option-syntax-invalid-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)
	for _, source := range invalid {
		filename := filepath.Join(tmp, "main.go")
		if err := os.WriteFile(filename, []byte("package main\n"+source+"\nfunc main(){}\n"), 0600); err != nil {
			panic(err)
		}
		cmd := exec.Command(tool, "build", "-o", filepath.Join(tmp, "bad"), filename)
		if out, err := cmd.CombinedOutput(); err == nil {
			panic(fmt.Sprintf("accepted invalid optional syntax: %s\n%s", source, out))
		}
	}
}
