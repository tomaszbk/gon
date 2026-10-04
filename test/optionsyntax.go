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
	invalid := []string{
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
