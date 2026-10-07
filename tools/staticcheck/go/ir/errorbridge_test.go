package ir_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"testing"
)

// TestGonTestFunctionPropagation verifies that error propagation returns the
// failure in test files and never introduces a Fatal call. Explicit handlers
// keep their ordinary testing method calls.
func TestGonTestFunctionPropagation(t *testing.T) {
	const src = `package p
import "testing"
func parse() (int, error) { return 1, nil }
func helper(t *testing.T) (int,error) { return parse()!,nil }
func generic(tb testing.TB) (int,string,error) { n:=parse()!; return n,"",nil }
func lambda(t *testing.T) { var f func(*testing.T) error = (u) => { parse()!; return nil }; _ = f }
func helperContext(t *testing.T) (int,error) { return parse() or err => err,nil }
func explicit(t *testing.T) { n:=parse() or err { t.Fatal(err); return }; _ = n }
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
	for _, test := range []struct {
		name   string
		fatals int
	}{{"helper", 0}, {"generic", 0}, {"lambda$1", 0}, {"helperContext", 0}, {"explicit", 1}} {
		fn := pkg.Func(test.name)
		if fn == nil {
			for _, anon := range pkg.Func("lambda").AnonFuncs {
				if anon.Name() == test.name {
					fn = anon
				}
			}
		}
		if fn == nil {
			t.Fatalf("missing function %s", test.name)
		}
		fatals, returns := 0, 0
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if _, ok := instr.(*ir.Return); ok {
					returns++
				}
				if call, ok := instr.(*ir.Call); ok {
					common := call.Common()
					name := ""
					if common.IsInvoke() {
						name = common.Method.Name()
					} else if callee := common.StaticCallee(); callee != nil {
						name = callee.Name()
					}
					if name == "Fatal" {
						fatals++
					}
				}
			}
		}
		if fatals != test.fatals || returns == 0 {
			t.Errorf("%s: got %d Fatal calls and %d returns; want %d Fatal calls and a return", test.name, fatals, returns, test.fatals)
		}
	}
}
