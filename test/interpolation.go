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
	dir := filepath.Join(runtime.GOROOT(), "test", "interpolation.dir")
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		panic("GON_BASELINE_GO required")
	}
	run := func(command string, args ...string) []byte {
		cmd := exec.Command(command, args...)
		for _, e := range os.Environ() {
			if !strings.HasPrefix(e, "GOROOT=") && !strings.HasPrefix(e, "GOTOOLDIR=") {
				cmd.Env = append(cmd.Env, e)
			}
		}
		cmd.Env = append(cmd.Env, "GOTOOLCHAIN=local", "GOENV=off")
		out, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("%v: %v\n%s", args, err, out))
		}
		return out
	}
	common, legacy, modern := filepath.Join(dir, "common.go"), filepath.Join(dir, "legacy.go"), filepath.Join(dir, "modern.go")
	want := run(baseline, "run", common, legacy)
	for _, args := range [][]string{{"run", common, legacy}, {"run", common, modern}, {"run", "-gcflags=-l", common, modern}, {"run", "-gcflags=-N -l", common, modern}} {
		if got := run(tool, args...); !bytes.Equal(want, got) {
			panic(fmt.Sprintf("different interpolation effects/results\nwant %s\ngot %s", want, got))
		}
	}
	run(tool, "vet", common, modern)
	tmp, err := os.MkdirTemp("", "gon-interpolation-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)
	for _, source := range []string{
		`package main;func main(){_ = $"${1}"}`,
		`package main;import "fmt";const x = $"${1}";func main(){}`,
		`package main;import "fmt";var _ = $"${1:%%}";func main(){}`,
		`package main;import "fmt";var _ = $"${1:%*d}";func main(){}`,
		`package main;import "fmt";var _ = $"${1:%[2]d}";func main(){}`,
		`package main;import "fmt";var _ = $"bad\z";func main(){}`,
		`package main;import $"fmt";func main(){}`,
		`package main;type T struct{F int $"tag"};func main(){}`,
	} {
		name := filepath.Join(tmp, "main.go")
		if err := os.WriteFile(name, []byte(source), 0600); err != nil {
			panic(err)
		}
		if out, err := exec.Command(tool, "build", "-o", filepath.Join(tmp, "bad"), name).CombinedOutput(); err == nil || bytes.Contains(out, []byte("internal compiler error")) {
			panic(fmt.Sprintf("bad rejection: %s\n%s", source, out))
		}
	}
}
