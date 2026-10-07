// run

//go:build !js && !wasip1 && gc

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

func main() {
	tool := filepath.Join(runtime.GOROOT(), "bin", "go")
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		panic("GON_BASELINE_GO must name an unmodified compatible Go toolchain")
	}
	fixtures := filepath.Join(runtime.GOROOT(), "test", "errorcontext.dir")
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
	if !bytes.HasSuffix(want, []byte("PASS\n")) {
		panic(fmt.Sprintf("legacy scenario did not pass: %s", want))
	}
	for _, got := range [][]byte{
		run(tool, "run", common, legacy),
		run(tool, "run", common, modern),
		run(tool, "run", "-gcflags=-l", common, modern),
	} {
		if !bytes.Equal(want, got) {
			panic(fmt.Sprintf("error context behavior differs: want %s, got %s", want, got))
		}
	}
	run(tool, "vet", common, modern)
	checkFatal(tool, baseline, fixtures)
	checkInvalid(tool)
}

func checkFatal(tool, baseline, fixtures string) {
	dir, err := os.MkdirTemp("", "gon-error-context-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	write := func(name string, content []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0600); err != nil {
			panic(err)
		}
	}
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(fixtures, name))
		if err != nil {
			panic(err)
		}
		return data
	}
	write("go.mod", []byte("module errorcontexttest\n\ngo 1.27\n"))
	write("common.go", read("common.go"))
	write("functions.go", read("legacy.go"))
	linePattern := regexp.MustCompile(`x_test\.go:(\d+): (.*)`)
	markerPattern := regexp.MustCompile(`// MARK:(\w+)`)
	var want []string
	for _, tc := range []struct{ tool, source string }{{baseline, "fatal_legacy.go"}, {tool, "fatal_legacy.go"}, {tool, "fatal_modern.go"}} {
		source := read(tc.source)
		write("x_test.go", source)
		markers := map[int]string{}
		for i, line := range strings.Split(string(source), "\n") {
			if marker := markerPattern.FindStringSubmatch(line); marker != nil {
				markers[i+1] = marker[1]
			}
		}
		var got []string
		for _, args := range [][]string{{"test", "-v", "-vet=off", "-count=1", "."}, {"test", "-v", "-vet=off", "-run=^$", "-bench=.", "-benchtime=1x", "-count=1", "."}} {
			cmd := exec.Command(tc.tool, args...)
			cmd.Dir = dir
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
			if err == nil {
				panic("Fatal tests unexpectedly passed")
			}
			if bytes.Contains(out, []byte("unreachable")) {
				panic(fmt.Sprintf("failure resumed: %s", out))
			}
			if len(args) == 5 && !bytes.Contains(out, []byte("defer ran")) {
				panic(fmt.Sprintf("Fatal did not run defer: %s", out))
			}
			matches := linePattern.FindAllStringSubmatch(string(out), -1)
			if len(matches) == 0 {
				panic(fmt.Sprintf("missing Fatal report: %s", out))
			}
			for _, match := range matches {
				line, err := strconv.Atoi(match[1])
				if err != nil {
					panic(err)
				}
				marker := markers[line]
				if marker == "" {
					panic(fmt.Sprintf("report is not at the handler line: %s", out))
				}
				got = append(got, marker+":"+match[2])
			}
		}
		sort.Strings(got)
		if want == nil {
			want = got
		} else if !reflectStrings(want, got) {
			panic(fmt.Sprintf("Fatal differs: want %v, got %v", want, got))
		}
	}
	if len(want) != 6 {
		panic(fmt.Sprintf("missing Fatal boundary reports: %v", want))
	}
}
func reflectStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func checkInvalid(tool string) {
	dir, err := os.MkdirTemp("", "gon-error-context-invalid-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	const prelude = `package main; func read()(int,error){return 1,nil}; func only()error{return nil}; `
	for _, body := range []string{
		`func main(){read() or err => err}`,
		`func f()error{read() or err => 1;return nil};func main(){}`,
		`func f()error{read() or err => err;_ = err;return nil};func main(){}`,
		`func f()error{go only() or err => err;return nil};func main(){}`,
		`func f()error{defer only() or err => err;return nil};func main(){}`,
		`func f()error{_ = func() int{return read() or err => err};return nil};func main(){}`,
	} {
		path := filepath.Join(dir, "invalid.go")
		if err := os.WriteFile(path, []byte(prelude+body), 0600); err != nil {
			panic(err)
		}
		cmd := exec.Command(tool, "build", "-o", filepath.Join(dir, "invalid.exe"), path)
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOENV=off")
		out, err := cmd.CombinedOutput()
		if err == nil {
			panic("invalid context compiled: " + body)
		}
		for _, crash := range []string{"panic:", "internal compiler error", "goroutine "} {
			if bytes.Contains(out, []byte(crash)) {
				panic(fmt.Sprintf("invalid context crashed: %s", out))
			}
		}
	}
}
