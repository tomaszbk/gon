package ir_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
)

// TestGonErrorBridge builds IR for the executable fixtures of postfix !
// across Go error tuples and Result.
func TestGonErrorBridge(t *testing.T) {
	root := os.Getenv("GON_ROOT")
	if root == "" {
		t.Skip("set GON_ROOT for executable pair fixtures")
	}
	for _, variant := range []string{"legacy", "modern"} {
		t.Run(variant, func(t *testing.T) {
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, filepath.Join(root, "misc/gon/analysisfixtures/errorbridge_"+variant+".go"), nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			p, _, err := irutil.BuildPackage(&types.Config{}, fs, types.NewPackage("main", "main"), []*ast.File{f}, ir.SanityCheckFunctions|ir.InstantiateGenerics|ir.GlobalDebug)
			if err != nil {
				t.Fatal(err)
			}
			if p.Func("main") == nil {
				t.Fatal("missing main IR")
			}
		})
	}
}

// TestGonTestFunctionPropagation checks that postfix ! in a test function
// lowers to a call of the first parameter's Fatal method followed by a return,
// and that Result propagation into an error result substitutes
// errors.ErrNilResult for a nil error.
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
	pkg, _, err := irutil.BuildPackage(conf, fs, types.NewPackage("p", "p"), []*ast.File{f}, ir.SanityCheckFunctions|ir.InstantiateGenerics)
	if err != nil {
		t.Fatal(err)
	}
	fatalCalls := func(fn *ir.Function) (lines []int) {
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				call, ok := instr.(*ir.Call)
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
					lines = append(lines, fs.Position(call.Source().Pos()).Line)
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
	fn := pkg.Func("toError")
	if fn == nil {
		t.Fatal("missing toError")
	}
	found := false
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			if phi, ok := instr.(*ir.Phi); ok && phi.Comment() == "nil Result error" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("toError does not substitute a nil Result error:\n%s", fn)
	}
}
