// run

//go:build !js && !wasip1 && gc

// String enums retain exhaustive matching while supplying textual interfaces.
// Execute both forms and a pristine Go baseline, including export round trips.
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
	fixtures := filepath.Join(runtime.GOROOT(), "test", "stringenums.dir")
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		panic("GON_BASELINE_GO must name an unmodified compatible Go toolchain")
	}
	common := filepath.Join(fixtures, "common.go")
	legacyFile := filepath.Join(fixtures, "legacy.go")
	modernFile := filepath.Join(fixtures, "modern.go")
	legacy := run(goTool, false, "", "run", common, legacyFile)
	compare("legacy baseline", legacy, run(baseline, true, "", "run", common, legacyFile))
	compare("modern", legacy, run(goTool, false, "", "run", common, modernFile))
	compare("modern without inlining", legacy, run(goTool, false, "", "run", "-gcflags=-l", common, modernFile))
	jsonv2 := runExperiment(goTool, false, "", "jsonv2", "run", common, legacyFile)
	compare("JSON v2 legacy", legacy, jsonv2)
	compare("JSON v2 baseline", jsonv2, runExperiment(baseline, true, "", "jsonv2", "run", common, legacyFile))
	compare("JSON v2 modern", jsonv2, runExperiment(goTool, false, "", "jsonv2", "run", common, modernFile))
	compare("JSON v2 modern without inlining", jsonv2, runExperiment(goTool, false, "", "jsonv2", "run", "-gcflags=-l", common, modernFile))
	checkExports(goTool, baseline, fixtures)
	checkInvalid(goTool)
}

func compare(what string, want, got []byte) {
	if !bytes.Equal(want, got) {
		panic(fmt.Sprintf("%s behavior differs\nwant:\n%s\ngot:\n%s", what, want, got))
	}
}

func run(goTool string, baseline bool, dir string, args ...string) []byte {
	return runExperiment(goTool, baseline, dir, "", args...)
}

func runExperiment(goTool string, baseline bool, dir, experiment string, args ...string) []byte {
	cmd := exec.Command(goTool, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local")
	if experiment != "" {
		cmd.Env = append(cmd.Env, "GOEXPERIMENT="+experiment)
	}
	if dir != "" {
		cmd.Env = append(cmd.Env, "GO111MODULE=on", "GOWORK=off")
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

func checkExports(goTool, baseline, fixtures string) {
	dir, err := os.MkdirTemp("", "gon-string-enum-exports-")
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
	write("go.mod", []byte("module featuretest\n\ngo 1.27\n"))
	copy("export_common.go", "main.go")
	copy("export_legacy_main.go", "helpers.go")
	copy("export_legacy.go", filepath.Join("lib", "lib.go"))
	legacy := run(goTool, false, dir, "run", ".")
	compare("exports baseline", legacy, run(baseline, true, dir, "run", "."))
	copy("export_modern_main.go", "helpers.go")
	copy("export_modern.go", filepath.Join("lib", "lib.go"))
	compare("exports modern", legacy, run(goTool, false, dir, "run", "."))
	compare("exports modern without inlining", legacy, run(goTool, false, dir, "run", "-gcflags=all=-l", "."))
}

func checkInvalid(goTool string) {
	dir, err := os.MkdirTemp("", "gon-string-enum-invalid-")
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
		for _, crash := range []string{"internal compiler error", "panic:", "panic happened", "unexpected type", "goroutine "} {
			if strings.Contains(string(out), crash) {
				panic(fmt.Sprintf("%s crashed compiler:\n%s", test.name, out))
			}
		}
	}
}

var invalidPrograms = []struct{ name, source string }{
	{"empty_parse", `package main; type E enum string {}; var e = E.Parse("x"); func main(){}`},
	{"missing_label", `package main; type Role enum string { default Unknown(string); Teacher }; func main(){}`},
	{"duplicate_label", `package main; type Role enum string { default Unknown(string); Teacher = "same"; Student = "same" }; func main(){}`},
	{"other_payload", `package main; type Role enum string { default Unknown(string); Teacher(int) = "teacher" }; func main(){}`},
	{"record_payload", `package main; type Role enum string { default Unknown(string); Teacher { Name string } = "teacher" }; func main(){}`},
	{"no_fallback", `package main; type Role enum string { Teacher = "teacher" }; func main(){}`},
	{"wrong_default", `package main; type Role enum string { default Unknown(int); Teacher = "teacher" }; func main(){}`},
	{"unit_default", `package main; type Role enum string { default Unknown; Teacher = "teacher" }; func main(){}`},
	{"fallback_label", `package main; type Role enum string { default Unknown(string) = "unknown"; Teacher = "teacher" }; func main(){}`},
	{"non_string_label", `package main; type Role enum string { default Unknown(string); Teacher = 1 }; func main(){}`},
	{"nonexhaustive", `package main; type Role enum string { default Unknown(string); Teacher = "teacher" }; func main(){_ = switch Role.Teacher { case Role.Teacher => 1 }}`},
	{"parse_collision", `package main; type Role enum string { default Unknown(string); Parse = "parse" }; func main(){}`},
	{"parse_method_collision", `package main; type Role enum string { default Unknown(string); Teacher = "teacher" }; func (Role) Parse() {}; func main(){}`},
	{"derived_parse_method_collision", `package main; type Role enum string { default Unknown(string); Teacher = "teacher" }; type Derived Role; func (Derived) Parse() {}; func main(){}`},
	{"string_collision", `package main; type Role enum string { default Unknown(string); String = "string" }; func main(){}`},
	{"string_method_collision", `package main; type Role enum string { default Unknown(string); Teacher = "teacher" }; func (Role) String() string {return "other"}; func main(){}`},
	{"marshal_method_collision", `package main; type Role enum string { default Unknown(string); Teacher = "teacher" }; func (Role) MarshalText() ([]byte,error) {return nil,nil}; func main(){}`},
	{"unmarshal_method_collision", `package main; type Role enum string { default Unknown(string); Teacher = "teacher" }; func (*Role) UnmarshalText([]byte) error {return nil}; func main(){}`},
}
