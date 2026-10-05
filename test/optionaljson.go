// run

//go:build !js && !wasip1 && gc

// Native optionals use the standard encoding/json boundary. The legacy form
// uses pointers and the modern form uses T?; both must produce the same wire
// data and decoded state. The modern form also runs with the v1 JSON
// implementation, since the v2 backed one is the default in this tree.
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
	fixtures := filepath.Join(runtime.GOROOT(), "test", "optionaljson.dir")
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		panic("GON_BASELINE_GO must name an unmodified compatible Go toolchain")
	}
	common := filepath.Join(fixtures, "common.go")
	legacy := filepath.Join(fixtures, "legacy.go")
	modern := filepath.Join(fixtures, "modern.go")
	want := run(goTool, false, "", "run", common, legacy)
	compare("legacy baseline", want, run(baseline, true, "", "run", common, legacy))
	compare("modern", want, run(goTool, false, "", "run", common, modern))
	compare("modern without inlining", want, run(goTool, false, "", "run", "-gcflags=all=-l", common, modern))
	// The default JSON implementation is the v2 based one; check the v1 one too.
	compare("legacy v1 JSON", want, run(goTool, false, "nojsonv2", "run", common, legacy))
	compare("modern v1 JSON", want, run(goTool, false, "nojsonv2", "run", common, modern))
}

func compare(name string, want, got []byte) {
	if !bytes.Equal(want, got) {
		panic(fmt.Sprintf("%s behavior differs\nwant:\n%s\ngot:\n%s", name, want, got))
	}
}

func run(goTool string, baseline bool, experiment string, args ...string) []byte {
	cmd := exec.Command(goTool, args...)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local")
	if experiment != "" {
		cmd.Env = append(cmd.Env, "GOEXPERIMENT="+experiment)
	}
	if baseline {
		var env []string
		for _, entry := range cmd.Env {
			if !strings.HasPrefix(entry, "GOROOT=") && !strings.HasPrefix(entry, "GOTOOLDIR=") {
				env = append(env, entry)
			}
		}
		cmd.Env = append(env, "GOFLAGS=")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("%s %v: %v\n%s", goTool, args, err, out))
	}
	return out
}
