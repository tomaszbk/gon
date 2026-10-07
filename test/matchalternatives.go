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
	fixtures := filepath.Join(runtime.GOROOT(), "test", "matchalternatives.dir")
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
			panic(fmt.Sprintf("match alternatives differ\nwant %s\ngot %s", want, got))
		}
	}
	run(tool, "", "vet", modern)
	dir, err := os.MkdirTemp("", "gon-matchalternatives-")
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
	write("go.mod", "module matchalternatives\n\ngo 1.27\n")
	write("main.go", `package main
import "matchalternatives/lib"
func main(){ for kind:=0;kind<3;kind++{println(lib.Radius(kind,7))} }
`)
	write(filepath.Join("lib", "lib.go"), `package lib
type Shape[T any] struct { Kind int; Radius T }
func Radius[T ~int](kind int,radius T) T {
 value:=Shape[T]{kind,radius}
 switch value.Kind {case 0,1:return value.Radius;default:return 0}
}
`)
	want = run(baseline, dir, "run", ".")
	if got := run(tool, dir, "run", "."); !bytes.Equal(want, got) {
		panic("legacy exports differ")
	}
	write(filepath.Join("lib", "lib.go"), `package lib
type Shape[T any] enum { default Empty; Circle(T); Sphere(T) }
func Radius[T ~int](kind int,radius T) T {
 var value Shape[T]
 if kind==0 {value=Shape[T].Circle(radius)}
 if kind==1 {value=Shape[T].Sphere(radius)}
 return switch value {case Shape[T].Circle(number),Shape[T].Sphere(number)=>number;case Shape[T].Empty=>0}
}
`)
	for _, args := range [][]string{{"run", "."}, {"run", "-gcflags=all=-l", "."}} {
		if got := run(tool, dir, args...); !bytes.Equal(want, got) {
			panic(fmt.Sprintf("modern exports differ: %s", got))
		}
	}
	for _, tc := range []struct{ source, want string }{
		{`type E enum{default A;B(int);C(string);D(int)};func f(v E)int{return switch v{case E.B(x),E.C(x)=>0;default=>1}}`, "identical types"},
		{`type E enum{default A;B(int);C(int)};func f(v E)int{return switch v{case E.B(x),E.C(y)=>0;default=>1}}`, "same names"},
		{`type E enum{default A;B(int);C(int)};func f(v E)int{return switch v{case E.B(x),E.C(_)=>0;default=>1}}`, "missing x"},
		{`type E enum{default A};func f(v E)int{return switch v{case _,E.A=>0}}`, "wildcard cannot"},
		{`type E enum{default A};func f(v E)int{return switch v{case E.A,E.A=>0}}`, "unreachable match alternative"},
		{`func f(v bool)int{return switch v{case true,true=>0;case false=>1}}`, "unreachable match alternative"},
		{`type E enum{default A;B;C};func f(v E)int{return switch v{case E.A,E.B=>0}}`, "non-exhaustive"},
		{`type E enum{default A};func f(v E)int{return switch v{default,E.A=>0}}`, "syntax error"},
	} {
		file := filepath.Join(dir, "bad.go")
		if err := os.WriteFile(file, []byte("package p;"+tc.source), 0600); err != nil {
			panic(err)
		}
		cmd := exec.Command(tool, "tool", "compile", "-o", filepath.Join(dir, "bad.o"), file)
		out, err := cmd.CombinedOutput()
		if err == nil || !bytes.Contains(out, []byte(tc.want)) || bytes.Contains(out, []byte("internal compiler error")) {
			panic(fmt.Sprintf("invalid alternative: %v\n%s", err, out))
		}
	}
}
