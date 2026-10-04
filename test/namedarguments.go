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
	dir := filepath.Join(runtime.GOROOT(), "test", "namedarguments.dir")
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
		cmd.Env = append(cmd.Env, "GOENV=off", "GOTOOLCHAIN=local")
		out, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("%s %v: %v\n%s", command, args, err, out))
		}
		return out
	}
	run := func(command string, args ...string) []byte { return runAt("", command, args...) }
	common, legacy, modern := filepath.Join(dir, "common.go"), filepath.Join(dir, "legacy.go"), filepath.Join(dir, "modern.go")
	want := run(tool, "run", common, legacy)
	for _, got := range [][]byte{run(baseline, "run", common, legacy), run(tool, "run", common, modern), run(tool, "run", "-gcflags=-l", common, modern)} {
		if !bytes.Equal(want, got) {
			panic(fmt.Sprintf("named calls differ\nwant %s\ngot %s", want, got))
		}
	}
	exports, err := os.MkdirTemp("", "gon-named-exports-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(exports)
	if err := os.Mkdir(filepath.Join(exports, "lib"), 0700); err != nil {
		panic(err)
	}
	write := func(name, source string) {
		if err := os.WriteFile(filepath.Join(exports, name), []byte(source), 0600); err != nil {
			panic(err)
		}
	}
	write("go.mod", "module namedexports\n\ngo 1.27\n")
	write("lib/lib.go", `package lib
func Pair(first, second int) int { return first*10+second }
func Generic[T ~int](first T, second []T) T { return first+second[0] }
type Operation func(left,right int) int
type Alias = Operation
var Op Alias = Pair
type Number int
func (n Number) Apply(first, second int) int { return int(n)*100+Pair(first,second) }
type Applier interface { Apply(left,right int) int }
`)
	write("main.go", `package main
import("fmt";"namedexports/lib")
func main(){ var i lib.Applier = lib.Number(3); fmt.Println(lib.Pair(1,2),lib.Generic(3,[]int{4}),lib.Op(1,2),i.Apply(1,2),lib.Number.Apply(3,1,2)) }
`)
	exportedLegacy := runAt(exports, baseline, "run", ".")
	if got := runAt(exports, tool, "run", "."); !bytes.Equal(exportedLegacy, got) {
		panic("exported legacy differs")
	}
	write("main.go", `package main
import("fmt";"namedexports/lib")
func main(){ var i lib.Applier = lib.Number(3); fmt.Println(lib.Pair(second:2,first:1),lib.Generic(second:[]int{4},first:3),lib.Op(right:2,left:1),i.Apply(right:2,left:1),lib.Number.Apply(second:2,n:3,first:1)) }
`)
	for _, args := range [][]string{{"run", "."}, {"run", "-gcflags=-l", "."}} {
		if got := runAt(exports, tool, args...); !bytes.Equal(exportedLegacy, got) {
			panic(fmt.Sprintf("exports differ: %s vs %s", exportedLegacy, got))
		}
	}
	invalid := []string{
		`f(a: 1, a: 2)`, `f(a: 1)`, `f(other: 1, b: 2)`, `f(a: 1, 2)`,
		`f(1, a: 2)`, `v(values: 1)`, `len(value: "x")`, `int(value: 1)`,
		`var x func(int,int) = f; x(a: 1,b: 2)`, `f(a: pair(), b: 2)`,
	}
	tmp, err := os.MkdirTemp("", "gon-named-invalid-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)
	for _, call := range invalid {
		filename := filepath.Join(tmp, "main.go")
		source := "package main\nfunc f(a,b int){}\nfunc v(values ...int){}\nfunc pair()(int,int){return 1,2}\nfunc main(){" + call + "}\n"
		if err := os.WriteFile(filename, []byte(source), 0600); err != nil {
			panic(err)
		}
		cmd := exec.Command(tool, "build", "-o", filepath.Join(tmp, "bad"), filename)
		if out, err := cmd.CombinedOutput(); err == nil {
			panic(fmt.Sprintf("accepted invalid %s: %s", call, out))
		}
	}
}
