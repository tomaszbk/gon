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
	fixtures := filepath.Join(runtime.GOROOT(), "test", "seq.dir")
	common := filepath.Join(fixtures, "common.go")
	legacy, modern := filepath.Join(fixtures, "legacy.go"), filepath.Join(fixtures, "modern.go")
	run := func(tool string, args ...string) []byte {
		cmd := exec.Command(tool, args...)
		for _, entry := range os.Environ() {
			key, _, _ := strings.Cut(entry, "=")
			switch key {
			case "GOROOT", "GOTOOLDIR", "GOTOOLCHAIN", "GOENV", "GOFLAGS", "GOWORK":
				continue
			}
			cmd.Env = append(cmd.Env, entry)
		}
		cmd.Env = append(cmd.Env, "GOENV=off", "GOTOOLCHAIN=local", "GOFLAGS=", "GOWORK=off")
		out, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("%s %v: %v\n%s", tool, args, err, out))
		}
		return out
	}
	want := run(baseline, "run", common, legacy)
	if string(want) != "PASS\n" {
		panic(fmt.Sprintf("legacy scenario did not pass: %s", want))
	}
	for _, got := range [][]byte{
		run(tool, "run", common, legacy),
		run(tool, "run", common, modern),
		run(tool, "run", "-gcflags=gon/seq=-l", common, modern),
	} {
		if !bytes.Equal(want, got) {
			panic(fmt.Sprintf("seq behavior differs: want %s, got %s", want, got))
		}
	}
	run(tool, "vet", common, modern)
}
