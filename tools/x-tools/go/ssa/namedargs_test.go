package ssa_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/interp"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestGonNamedArguments(t *testing.T) {
	root, baseline := os.Getenv("GON_ROOT"), os.Getenv("GON_BASELINE_GO")
	if root == "" || baseline == "" {
		t.Skip("set GON_ROOT and GON_BASELINE_GO for executable pairs")
	}
	gonName := "gon"
	if runtime.GOOS == "windows" {
		gonName += ".exe"
	}
	for _, variant := range []string{"legacy", "modern"} {
		t.Run(variant, func(t *testing.T) {
			path := filepath.Join(root, "misc/gon/analysisfixtures/namedarguments_"+variant+".go")
			toolchains := []string{filepath.Join(root, "gon/bin", gonName)}
			if variant == "legacy" {
				toolchains = append(toolchains, baseline)
			}
			for _, tool := range toolchains {
				cmd := exec.Command(tool, "run", path)
				cmd.Dir = t.TempDir()
				for _, entry := range os.Environ() {
					key, _, _ := strings.Cut(entry, "=")
					switch key {
					case "GOROOT", "GOTOOLDIR", "GOTOOLCHAIN", "GOWORK", "GOFLAGS", "GOENV":
						continue
					}
					cmd.Env = append(cmd.Env, entry)
				}
				cmd.Env = append(cmd.Env, "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=", "GOENV=off")
				out, err := cmd.CombinedOutput()
				if err != nil || string(out) != "PASS\n" {
					t.Fatalf("%s: %v\n%s", tool, err, out)
				}
			}
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, path, nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			p, _, err := ssautil.BuildPackage(&types.Config{}, fs, types.NewPackage("main", "main"), []*ast.File{f}, ssa.SanityCheckFunctions|ssa.InstantiateGenerics|ssa.GlobalDebug)
			if err != nil {
				t.Fatal(err)
			}
			if code := interp.Interpret(p, 0, types.SizesFor("gc", runtime.GOARCH), "main", nil); code != 0 {
				t.Fatalf("SSA execution failed: %d", code)
			}
		})
	}
}
