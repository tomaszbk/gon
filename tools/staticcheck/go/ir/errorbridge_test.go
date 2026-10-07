package ir_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
)

// TestGonTestFunctionPropagation checks that postfix ! in a test function
// lowers to a call of the first parameter's Fatal method followed by a return,
// including promoted and interface methods.
func TestGonTestFunctionPropagation(t *testing.T) {
	const src = `package p

import "testing"

func parse() (int, error) { return 1, nil }

func helper(t *testing.T) int {
	return parse()!
}

func generic(tb testing.TB) (int, string) {
	n := parse()!
	return n, ""
}

func lambda(t *testing.T) {
	t.Run("x", (u) => { parse()! })
}

func helperContext(t *testing.T) int { return parse() or err => err }
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
	}{{"helper", 8}, {"generic", 12}, {"lambda$1", 17}, {"helperContext", 20}} {
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
}
