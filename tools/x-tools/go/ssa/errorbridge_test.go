package ssa_test

import (
	"go/ast"
	"go/importer"
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

// TestGonErrorBridge executes the legacy and modern spellings of postfix !
// across Go error tuples and Result with the compiler, the baseline and the
// SSA interpreter.
func TestGonErrorBridge(t *testing.T) {
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
			path := filepath.Join(root, "misc/gon/analysisfixtures/errorbridge_"+variant+".go")
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

// TestGonTestFunctionPropagation checks that postfix ! in a test function
// lowers to a call of the first parameter's Fatal method followed by a return
// of zero values, for a pointer parameter (a promoted method) and an
// interface parameter, and that Result propagation into an error result
// substitutes errors.ErrNilResult for a nil error.
func TestGonTestFunctionPropagation(t *testing.T) {
	const src = `package p

import ("errors"; "testing")

func parse() (int, error) { return 1, nil }
func result() Result[int, error] { return .Ok(1) }

func helper(t *testing.T) int {
	return parse()!
}

func generic(tb testing.TB) (int, string) {
	n := result()!
	return n, ""
}

func lambda(t *testing.T) {
	t.Run("x", (u) => { parse()! })
}

func toError() (int, error) {
	n := result()!
	return n, nil
}

var _ = errors.ErrNilResult
`
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "p_test.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	conf := &types.Config{Importer: importer.ForCompiler(fs, "source", nil)}
	pkg, _, err := ssautil.BuildPackage(conf, fs, types.NewPackage("p", "p"), []*ast.File{f}, ssa.SanityCheckFunctions|ssa.InstantiateGenerics)
	if err != nil {
		t.Fatal(err)
	}
	// fatalCalls returns the line numbers of calls to a method named Fatal.
	fatalCalls := func(fn *ssa.Function) (lines []int) {
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				call, ok := instr.(*ssa.Call)
				if !ok {
					continue
				}
				c := call.Common()
				name := ""
				if c.IsInvoke() {
					name = c.Method.Name()
				} else if callee := c.StaticCallee(); callee != nil {
					name = callee.Name()
				}
				if name == "Fatal" {
					lines = append(lines, fs.Position(call.Pos()).Line)
				}
			}
		}
		return
	}
	for _, test := range []struct {
		fn   string
		line int
	}{{"helper", 9}, {"generic", 13}, {"lambda$1", 18}} {
		fn := pkg.Func(test.fn)
		if fn == nil {
			for _, anon := range pkg.Func("lambda").AnonFuncs {
				if anon.Name() == test.fn {
					fn = anon
				}
			}
		}
		if fn == nil {
			t.Fatalf("missing function %s", test.fn)
		}
		lines := fatalCalls(fn)
		if len(lines) != 1 {
			t.Errorf("%s: want one call of Fatal, got lines %v\n%s", test.fn, lines, fn)
			continue
		}
		if lines[0] != test.line {
			t.Errorf("%s: Fatal is reported at line %d, want the line %d of the !", test.fn, lines[0], test.line)
		}
	}
	if fn := pkg.Func("toError"); fn == nil {
		t.Fatal("missing toError")
	} else {
		found := false
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				if phi, ok := instr.(*ssa.Phi); ok && phi.Comment == "nil Result error" {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("toError does not substitute a nil Result error:\n%s", fn)
		}
	}
}
