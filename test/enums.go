// run

//go:build !js && !wasip1 && gc

// Execute equivalent legacy and Gon programs, including a required unmodified
// Go baseline, and check invalid contexts and cross-package export bodies.
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
	fixtures := filepath.Join(runtime.GOROOT(), "test", "enums.dir")
	common := filepath.Join(fixtures, "common.go")
	legacyFile := filepath.Join(fixtures, "legacy.go")
	modernFile := filepath.Join(fixtures, "modern.go")
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		panic("GON_BASELINE_GO must name an unmodified compatible Go toolchain")
	}
	legacy := run(goTool, false, "run", common, legacyFile)
	compare("legacy/modern", legacy, run(goTool, false, "run", common, modernFile))
	compare("legacy/modern without inlining", legacy, run(goTool, false, "run", "-gcflags=-l", common, modernFile))
	compare("legacy baseline", legacy, run(baseline, true, "run", common, legacyFile))
	reflection := run(goTool, false, "run", common, filepath.Join(fixtures, "reflect_legacy.go"))
	compare("reflection modern", reflection, run(goTool, false, "run", common, filepath.Join(fixtures, "reflect_modern.go")))
	compare("reflection baseline", reflection, run(baseline, true, "run", common, filepath.Join(fixtures, "reflect_legacy.go")))
	serialization := filepath.Join(fixtures, "serialization_common.go")
	serializationLegacy := filepath.Join(fixtures, "serialization_legacy.go")
	serializationModern := filepath.Join(fixtures, "serialization_modern.go")
	wire := run(goTool, false, "run", serialization, serializationLegacy)
	compare("explicit serialization modern", wire, run(goTool, false, "run", serialization, serializationModern))
	compare("explicit serialization without inlining", wire, run(goTool, false, "run", "-gcflags=-l", serialization, serializationModern))
	compare("explicit serialization baseline", wire, run(baseline, true, "run", serialization, serializationLegacy))
	checkExports(goTool, baseline, fixtures)
	checkInvalid(goTool)
	checkCgo(goTool, baseline, fixtures)
}

func compare(what string, want, got []byte) {
	if !bytes.Equal(want, got) {
		panic(fmt.Sprintf("%s behavior differs\nwant:\n%s\ngot:\n%s", what, want, got))
	}
}

func run(goTool string, baseline bool, args ...string) []byte {
	return runAt(goTool, baseline, "", args...)
}

func runAt(goTool string, baseline bool, dir string, args ...string) []byte {
	cmd := exec.Command(goTool, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local")
	if dir != "" {
		cmd.Env = append(cmd.Env, "GO111MODULE=on")
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

func checkInvalid(goTool string) {
	dir, err := os.MkdirTemp("", "gon-enum-invalid-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	for _, test := range invalidPrograms {
		file := filepath.Join(dir, test.name+".go")
		if err := os.WriteFile(file, []byte(test.source), 0600); err != nil {
			panic(err)
		}
		cmd := exec.Command(goTool, "build", "-o", filepath.Join(dir, "invalid.exe"), file)
		cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local")
		out, err := cmd.CombinedOutput()
		if err == nil {
			panic(test.name + ": invalid program compiled")
		}
		for _, crash := range []string{"internal compiler error", "panic:", "unexpected type", "goroutine "} {
			if strings.Contains(string(out), crash) {
				panic(fmt.Sprintf("%s crashed compiler:\n%s", test.name, out))
			}
		}
	}
}

var invalidPrograms = []struct{ name, source string }{
	{"record_type_alias", `package main; type E enum { default Empty; Record { Field int } }; type Alias = E.Record; func main(){}`},
	{"record_variable_type", `package main; type E enum { default Empty; Record { Field int } }; var x E.Record; func main(){}`},
	{"record_new_type", `package main; type E enum { default Empty; Record { Field int } }; func main(){_ = new(E.Record)}`},
	{"record_conversion", `package main; type E enum { default Empty; Record { Field int } }; func main(){_ = E.Record(E.Empty)}`},
	{"record_first_class", `package main; type E enum { default Empty; Record { Field int } }; func main(){_ = E.Record}`},
	{"method_collision", `package main; type X enum { default A; B(int) }; func (X) B(){}; func main(){}`},
	{"missing_default", `package main; type X enum { A; B }; func main(){}`},
	{"double_default", `package main; type X enum { default A; default B }; func main(){}`},
	{"duplicate_variant", `package main; type X enum { default A; A }; func main(){}`},
	{"record_call", `package main; type X enum { default A; B { Field int } }; func main(){_ = X.B(1)}`},
	{"positional_record", `package main; type X enum { default A; B(int) }; func main(){_ = X.B{Field: 1}}`},
	{"record_unkeyed", `package main; type X enum { default A; B { Field int } }; func main(){_ = X.B{1}}`},
	{"direct_enum_literal", `package main; type X enum { default A; B(int) }; func main(){_ = X{}}`},
	{"duplicate_payload", `package main; type X enum { default A; B { Field int } }; func main(){_ = X.B{Field: 1, Field: 2}}`},
	{"unknown_payload", `package main; type X enum { default A; B { Field int } }; func main(){_ = X.B{Missing: 1}}`},
	{"wrong_payload", `package main; type X enum { default A; B(int) }; func main(){_ = X.B("wrong")}`},
	{"noncomparable", `package main; type X enum { default A; B([]int) }; func main(){_ = X.A == X.A}`},
}

func checkExports(goTool, baseline, fixtures string) {
	dir, err := os.MkdirTemp("", "gon-enum-exports-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if err := os.Mkdir(filepath.Join(dir, "lib"), 0700); err != nil {
		panic(err)
	}
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			panic(err)
		}
	}
	copy := func(source, dest string) {
		data, err := os.ReadFile(filepath.Join(fixtures, source))
		if err != nil {
			panic(err)
		}
		write(dest, data)
	}
	write("go.mod", []byte("module featuretest\n\ngo 1.26\n"))
	copy("export_main.go", "main.go")
	copy("export_legacy.go", filepath.Join("lib", "lib.go"))
	legacy := runAt(goTool, false, dir, "run", ".")
	compare("exports baseline", legacy, runAt(baseline, true, dir, "run", "."))
	copy("export_modern.go", filepath.Join("lib", "lib.go"))
	compare("exports modern", legacy, runAt(goTool, false, dir, "run", "."))
	compare("exports modern without inlining", legacy, runAt(goTool, false, dir, "run", "-gcflags=-l", "."))
}

func checkCgo(goTool, baseline, fixtures string) {
	if strings.TrimSpace(string(run(goTool, false, "env", "CGO_ENABLED"))) != "1" {
		panic("enum cgo gate requires CGO_ENABLED=1")
	}
	legacy := run(goTool, false, "run", filepath.Join(fixtures, "cgo_legacy.go"))
	compare("cgo modern adapter", legacy, run(goTool, false, "run", filepath.Join(fixtures, "cgo_modern.go")))
	compare("cgo baseline adapter", legacy, run(baseline, true, "run", filepath.Join(fixtures, "cgo_legacy.go")))
	dir, err := os.MkdirTemp("", "gon-enum-cgo-invalid-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, []byte(`package main
/* typedef struct { int value; } Holder;
static int takeInt(int x) { return x; }
static int takeStruct(Holder x) { return x.value; }
*/
import "C"
type Input enum { default Missing; Value(int) }
func main() { C.takeInt(Input.Value(1)); C.takeStruct(Input.Value(1)) }
`), 0600); err != nil {
		panic(err)
	}
	cmd := exec.Command(goTool, "build", "-o", filepath.Join(dir, "invalid.exe"), file)
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "cannot use") {
		panic(fmt.Sprintf("raw enum cgo boundary must fail cleanly: %v\n%s", err, out))
	}
}
