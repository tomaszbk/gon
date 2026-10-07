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
	fixtures := filepath.Join(runtime.GOROOT(), "test", "patterntest.dir")
	run := func(command, directory string, args ...string) []byte {
		cmd := exec.Command(command, args...)
		cmd.Dir = directory
		for _, variable := range os.Environ() {
			if !strings.HasPrefix(variable, "GOROOT=") && !strings.HasPrefix(variable, "GOTOOLDIR=") && !strings.HasPrefix(variable, "GOFLAGS=") {
				cmd.Env = append(cmd.Env, variable)
			}
		}
		cmd.Env = append(cmd.Env, "GOENV=off", "GOTOOLCHAIN=local", "GOFLAGS=")
		out, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("%s %v: %v\n%s", command, args, err, out))
		}
		return out
	}
	legacy, modern := filepath.Join(fixtures, "legacy.go"), filepath.Join(fixtures, "modern.go")
	want := run(baseline, "", "run", legacy)
	for _, args := range [][]string{{"run", legacy}, {"run", modern}, {"run", "-gcflags=-l", modern}, {"run", "-gcflags=-N -l", modern}} {
		if got := run(tool, "", args...); !bytes.Equal(want, got) {
			panic(fmt.Sprintf("pattern tests differ\nwant %s\ngot %s", want, got))
		}
	}
	run(tool, "", "vet", modern)
	dir, err := os.MkdirTemp("", "gon-patterntest-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if err := os.Mkdir(filepath.Join(dir, "lib"), 0700); err != nil {
		panic(err)
	}
	write := func(path, source string) {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(source), 0600); err != nil {
			panic(err)
		}
	}
	write("go.mod", "module patterntest\n\ngo 1.27\n")
	write("main.go", `package main
import "patterntest/lib"
func main(){ for kind:=0;kind<3;kind++{println(lib.Radius(kind,7))} }
`)
	write(filepath.Join("lib", "lib.go"), `package lib
func Radius[T ~int](kind int, radius T) T {
 if kind<2 {return radius};return 0
}
`)

	want = run(baseline, dir, "run", ".")
	if got := run(tool, dir, "run", "."); !bytes.Equal(want, got) {
		panic("legacy exports differ")
	}
	write(filepath.Join("lib", "lib.go"), `package lib
func Radius[T ~int](kind int, radius T) T {
 var value T?
 if kind<2 {value=radius}
 if value is number? {return number}
 return 0
}
`)

	for _, args := range [][]string{{"run", "."}, {"run", "-gcflags=all=-l", "."}} {
		if got := run(tool, dir, args...); !bytes.Equal(want, got) {
			panic(fmt.Sprintf("modern exports differ: %s", got))
		}
	}
	for _, tc := range []struct{ source, want string }{
		{`var optional int?;var _ = optional is number?`, "bindings require an if"},
		{`var optional int?;func f(){if optional is number? || true {_=number}}`, "bindings require an if"},
		{`var optional int?;func f(){if !(optional is number?) {_=number}}`, "bindings require an if"},
		{`var optional int?;func f(){for optional is number? {_=number}}`, "bindings require an if"},
		{`var optional int?;func f(){if optional is number? {} else {_=number}}`, "undefined"},
		{`var optional int?;var _ = optional is _`, "always matches"},
		{`var optional int?;func f(){if optional is number {_=number}}`, "always matches"},
		{`type Unit enum{default Only};var value Unit;var _ = value is Unit.Only`, "always matches"},
		{`var optional int?;var _ = optional is "wrong"`, "cannot use"},
	} {
		file := filepath.Join(dir, "bad.go")
		if err := os.WriteFile(file, []byte("package p;"+tc.source), 0600); err != nil {
			panic(err)
		}
		cmd := exec.Command(tool, "tool", "compile", "-o", filepath.Join(dir, "bad.o"), file)
		out, err := cmd.CombinedOutput()
		if err == nil || !bytes.Contains(out, []byte(tc.want)) || bytes.Contains(out, []byte("internal compiler error")) {
			panic(fmt.Sprintf("invalid pattern test: %v\n%s", err, out))
		}
	}
}
