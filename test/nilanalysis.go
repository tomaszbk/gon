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
	root := runtime.GOROOT()
	tool := filepath.Join(root, "bin", "go")
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		panic("GON_BASELINE_GO must name an unmodified compatible Go toolchain")
	}
	fixtures := filepath.Join(root, "tools", "nilaway", "testdata", "src", "gon.test")
	legacy, modern := filepath.Join(fixtures, "gonlegacy", "main.go"), filepath.Join(fixtures, "gonsafe", "main.go")
	run := func(command string, args ...string) []byte {
		cmd := exec.Command(command, args...)
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "GOROOT=") && !strings.HasPrefix(entry, "GOTOOLDIR=") && !strings.HasPrefix(entry, "GOFLAGS=") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "GOENV=off", "GOTOOLCHAIN=local", "GOFLAGS=")
		if goos := os.Getenv("GON_PAIR_GOOS"); goos != "" {
			wrapperRoot := root
			if command == baseline {
				env := exec.Command(command, "env", "GOROOT")
				env.Env = append([]string{}, cmd.Env...)
				output, err := env.Output()
				if err != nil {
					panic(err)
				}
				wrapperRoot = strings.TrimSpace(string(output))
			}
			cmd.Env = append(cmd.Env, "GOOS="+goos, "GOARCH="+os.Getenv("GON_PAIR_GOARCH"), "CGO_ENABLED=0", "PATH="+filepath.Join(wrapperRoot, "lib", "wasm")+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("%s %v: %v\n%s", command, args, err, out))
		}
		return out
	}
	for _, scenario := range [][2]string{{legacy, modern}, {filepath.Join(fixtures, "gonunsafelegacy", "main.go"), filepath.Join(fixtures, "gonunsafe", "main.go")}} {
		want := run(baseline, "run", scenario[0])
		for _, args := range [][]string{{"run", scenario[0]}, {"run", scenario[1]}, {"run", "-gcflags=-l", scenario[1]}, {"run", "-gcflags=-N -l", scenario[1]}} {
			if got := run(tool, args...); !bytes.Equal(want, got) {
				panic(fmt.Sprintf("nil analysis executable examples differ\nwant %s\ngot %s", want, got))
			}
		}
		run(baseline, "test", scenario[0], filepath.Join(filepath.Dir(scenario[0]), "fatal_test.go"))
		run(tool, "test", scenario[0], filepath.Join(filepath.Dir(scenario[0]), "fatal_test.go"))
		run(tool, "test", scenario[1], filepath.Join(filepath.Dir(scenario[1]), "fatal_test.go"))
	}
}
