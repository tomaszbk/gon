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

// The paired programs match error and interface subjects against enum
// variants. The legacy program searches the error tree with errors.As and
// uses type assertions; the modern program uses `switch ... { case E.V => }`.
// Both must print the same results and the same trace of As method calls.
func main() {
	root := runtime.GOROOT()
	tool := filepath.Join(root, "bin", "go")
	fixtures := filepath.Join(root, "test", "matchinterface.dir")
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
	if !bytes.Contains(want, []byte("classify/shim-db db-not-found:s1")) {
		panic(fmt.Sprintf("legacy program did not run its scenarios:\n%s", want))
	}
	compare := func(name string, got []byte) {
		if !bytes.Equal(want, got) {
			panic(fmt.Sprintf("%s mismatch\nwant %s\ngot %s", name, want, got))
		}
	}
	compare("modern", run(tool, false, "run", common, modernFile))
	compare("modern no inline", run(tool, false, "run", "-gcflags=all=-l", common, modernFile))
	compare("modern no optimization", run(tool, false, "run", "-gcflags=-N", common, modernFile))
	compare("legacy baseline", run(baseline, true, "run", common, legacyFile))

	// An enum type declared in another package, an inlinable function that
	// matches an error, and a generic enum instantiated across packages.
	dir, err := os.MkdirTemp("", "gon-match-interface-")
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
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module matchinterfaceexport\n\ngo 1.27\n"), 0600); err != nil {
		panic(err)
	}
	copy("export_main.go", "main.go")
	copy("export_legacy.go", filepath.Join("lib", "lib.go"))
	exportedLegacy := run(tool, false, "run", dir)
	if got := run(baseline, true, "run", dir); !bytes.Equal(got, exportedLegacy) {
		panic("export baseline behavior differs")
	}
	if !bytes.Contains(exportedLegacy, []byte("main:coded:y")) {
		panic(fmt.Sprintf("legacy export program did not run its scenarios:\n%s", exportedLegacy))
	}
	copy("export_modern.go", filepath.Join("lib", "lib.go"))
	copy("export_main_modern.go", "main.go")
	for _, args := range [][]string{{"run", dir}, {"run", "-gcflags=all=-l", dir}} {
		if got := run(tool, false, args...); !bytes.Equal(got, exportedLegacy) {
			panic(fmt.Sprintf("export modern behavior differs: %s\nwant %s", got, exportedLegacy))
		}
	}

	const prefix = `type E enum{default A;B(int)};func (E) Error() string{return ""};type N enum{default A};type P enum{default A};func (*P) Error() string{return ""};type S struct{X int};`
	for _, test := range []struct{ source, want string }{
		{`func f(err error)int{return switch err{case E.A=>0;case E.B(_)=>1}}`, "non-exhaustive"},
		{`func f(err error)int{return switch err{case E.A=>0;case E.B(_)=>1}}`, "enum alternatives never cover the interface type error"},
		{`func f(err error){switch err{case E.A=>{}}}`, "non-exhaustive"},
		{`func f(v any)int{return switch v{case E.A=>0}}`, "enum alternatives never cover the interface type any"},
		{`func f(err error)int{return switch err{case N.A=>0;default=>1}}`, "N does not implement error"},
		{`func f(err error)int{return switch err{case P.A=>0;default=>1}}`, "pointer receiver"},
		{`func f(err error)int{return switch err{case S.X=>0;default=>1}}`, "qualified by an enum type"},
		{`func f(err error)int{return switch err{case Result[int,error].Ok(v)=>v;default=>1}}`, "can never match interface error"},
		{`func f(err error)int{return switch err{case v?=>1;default=>0}}`, "presence pattern requires an optional value"},
		{`func f(err error)int{return switch err{case E.A=>0;case E.A=>1;default=>2}}`, "unreachable"},
		{`func f(err error)int{return switch err{default=>0;case E.A=>1}}`, "unreachable"},
		{`func f(err error)int{return switch err{case E.C=>1;default=>0}}`, "unknown or inaccessible alternative C"},
		{`func f(err error)int{return switch err{case B(n)=>n;default=>0}}`, "qualified enum alternative"},
		{`func f(err error)int{return switch err{case E.A:return 0}}`, "syntax error"},
	} {
		dir, err := os.MkdirTemp("", "gon-match-interface-invalid-")
		if err != nil {
			panic(err)
		}
		file := filepath.Join(dir, "bad.go")
		if err := os.WriteFile(file, []byte("package p;"+prefix+test.source), 0600); err != nil {
			panic(err)
		}
		cmd := exec.Command(tool, "tool", "compile", "-o", filepath.Join(dir, "bad.o"), file)
		out, err := cmd.CombinedOutput()
		os.RemoveAll(dir)
		if err == nil || !bytes.Contains(out, []byte(test.want)) || bytes.Contains(out, []byte("internal compiler error")) {
			panic(fmt.Sprintf("invalid interface match %s: %v\n%s", test.source, err, out))
		}
	}
}
