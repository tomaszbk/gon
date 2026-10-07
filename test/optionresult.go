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
	dir := filepath.Join(runtime.GOROOT(), "test", "optionresult.dir")
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
	for _, got := range [][]byte{run(baseline, "run", common, legacy), run(tool, "run", common, modern), run(tool, "run", "-gcflags=-l", common, modern), run(tool, "run", "-gcflags=-N -l", common, modern)} {
		if !bytes.Equal(want, got) {
			panic(fmt.Sprintf("Optional differs\nwant %s\ngot %s", want, got))
		}
	}
	invalid := []string{
		`type User struct{Name string};type Node struct{User User?};func f(x *Node)string{return x?.User?.Name ?? "none"}`,
		`type User struct{Name string};type Node struct{User *User};func f(x Node?)string{return x?.User?.Name ?? "none"}`,

		`func f(x int?) int{return x?}`,
		`func f(){var x int?;x??=(int?)((int)(3))}`,
		`func source()int?{return (int?)(nil)};func f(){source()??=3}`,
		`func f(){defer func()int?{return nil}()?}`,
		`func f(){go func()int?{return nil}()?}`,
	}

	tmp, err := os.MkdirTemp("", "gon-optionresult-invalid-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)
	for _, call := range invalid {
		filename := filepath.Join(tmp, "main.go")
		source := "package main\n" + call + "\nfunc main(){}\n"
		if err := os.WriteFile(filename, []byte(source), 0600); err != nil {
			panic(err)
		}
		cmd := exec.Command(tool, "build", "-o", filepath.Join(tmp, "bad"), filename)
		if out, err := cmd.CombinedOutput(); err == nil {
			panic(fmt.Sprintf("accepted invalid Optional %s: %s", call, out))
		}
	}
}
