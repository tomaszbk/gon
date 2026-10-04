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
	fixtures := filepath.Join(root, "test", "matching.dir")
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		panic("GON_BASELINE_GO must name an unmodified compatible Go toolchain")
	}
	common := filepath.Join(fixtures, "common.go")
	legacyFile, modernFile := filepath.Join(fixtures, "legacy.go"), filepath.Join(fixtures, "modern.go")
	run := func(tool string, baseline bool, args ...string) []byte {
		var workdir string
		if len(args) > 0 && filepath.IsAbs(args[len(args)-1]) && filepath.Ext(args[len(args)-1]) != ".go" {
			workdir = args[len(args)-1]
			args = append(args[:len(args)-1], ".")
		}
		cmd := exec.Command(tool, args...)
		cmd.Dir = workdir
		var env []string
		for _, s := range os.Environ() {
			if baseline && (strings.HasPrefix(s, "GOROOT=") || strings.HasPrefix(s, "GOTOOLDIR=")) {
				continue
			}
			env = append(env, s)
		}
		cmd.Env = append(env, "GOENV=off", "GOTOOLCHAIN=local", "GOFLAGS=")
		out, err := cmd.CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("%s %v: %v\n%s", tool, args, err, out))
		}
		return out
	}
	want := run(tool, false, "run", common, legacyFile)
	compare := func(name string, got []byte) {
		if !bytes.Equal(want, got) {
			panic(fmt.Sprintf("%s mismatch\nwant %s\ngot %s", name, want, got))
		}
	}
	compare("modern", run(tool, false, "run", common, modernFile))
	compare("modern no inline", run(tool, false, "run", "-gcflags=-l", common, modernFile))
	compare("legacy baseline", run(baseline, true, "run", common, legacyFile))
	// Import descriptors, generic substitution, and serialized match bodies.
	dir, err := os.MkdirTemp("", "gon-match-exports-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if err := os.Mkdir(filepath.Join(dir, "lib"), 0700); err != nil {
		panic(err)
	}
	copy := func(from, to string) {
		data, err := os.ReadFile(filepath.Join(fixtures, from))
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(dir, to), data, 0600); err != nil {
			panic(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module matchingexport\n\ngo 1.27\n"), 0600); err != nil {
		panic(err)
	}
	copy("export_main.go", "main.go")
	copy("export_legacy.go", filepath.Join("lib", "lib.go"))
	exportedLegacy := run(tool, false, "run", dir)
	if got := run(baseline, true, "run", dir); !bytes.Equal(got, exportedLegacy) {
		panic("export baseline behavior differs")
	}
	copy("export_modern.go", filepath.Join("lib", "lib.go"))
	copy("export_main_modern.go", "main.go")
	for _, args := range [][]string{{"run", dir}, {"run", "-gcflags=all=-l", dir}} {
		if got := run(tool, false, args...); !bytes.Equal(got, exportedLegacy) {
			panic(fmt.Sprintf("export modern behavior differs: %s", got))
		}
	}
	for _, test := range []struct{ source, want string }{
		{`type E enum {default A;B(int)};func f(e E)int{return switch e{case E.A=>0}}`, "non-exhaustive"},
		{`func f(b bool)int{return switch b{case true=>1;case false=>"bad"}}`, "match expression"},
		{`type E enum{default A;B(int)};func f(e E)int{return switch e{default=>0;case E.B(_)=>1}}`, "unreachable"},
		{`const item=1;func f()int{return switch 1{case item=>2;default=>0}}`, "qualified alternative"},
		{`type E enum{default A;B(int)};func f(e E)int{return switch e{case A=>1;default=>0}}`, "qualified alternative"},
		{`type E enum{default A;B(int)};func f(e E)int{return switch e{case B(n)=>n;default=>0}}`, "qualified enum alternative"},
		{`type E enum{default A;B(int)};func f(e E)int{return switch e{case .A=>1;default=>0}}`, "syntax error"},
		{`type E enum{default A;B(int)};const true=1;func f(e E)int{return switch e{case E.B(true)=>1;default=>0}}`, "cannot use true"},
		{`type E enum{default A;B(int)};const nil=1;func f(e E)int{return switch e{case E.B(nil)=>1;default=>0}}`, "cannot use nil"},
	} {
		dir, err := os.MkdirTemp("", "gon-match-invalid-")
		if err != nil {
			panic(err)
		}
		file := filepath.Join(dir, "bad.go")
		if err := os.WriteFile(file, []byte("package p;"+test.source), 0600); err != nil {
			panic(err)
		}
		cmd := exec.Command(tool, "tool", "compile", "-o", filepath.Join(dir, "bad.o"), file)
		out, err := cmd.CombinedOutput()
		os.RemoveAll(dir)
		if err == nil || !bytes.Contains(out, []byte(test.want)) || bytes.Contains(out, []byte("internal compiler error")) {
			panic(fmt.Sprintf("invalid match: %v\n%s", err, out))
		}
	}
}
