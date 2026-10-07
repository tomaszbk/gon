package ssa_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"runtime"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/interp"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestGonErrorContext(t *testing.T) {
	const common = `package main
type fault string
func (e fault) Error() string { return string(e) }
var calls, wraps, observed int
func read(fail bool) (int,error) { calls++; if fail { return 99,fault("source") }; return 4,nil }
func wrap(err error) error { wraps++; return fault("context") }
`
	const legacy = `func run(fail bool) (n int, err error) {
 n = 8
 defer func() { observed = n }()
 value, problem := read(fail)
 if problem != nil { return 0, wrap(problem) }
 n = value
 return n,nil
}
func nilContext() (int,error) {
 n,err:=read(true)
 if err != nil { return 0,nil }
 return n,nil
}
func nested(fail bool) (int,error) {
 run := func() (int,error) {
  n,err:=read(fail)
  if err != nil { return 0,wrap(err) }
  return n,nil
 }
 return run()
}
`
	const modern = `func run(fail bool) (n int, err error) {
 n = 8
 defer func() { observed = n }()
 n = read(fail) or problem => wrap(problem)
 return n,nil
}
func nilContext() (int,error) { return read(true) or _ => nil,nil }
func nested(fail bool) (int,error) {
 var run func() (int,error) = () => {
  return read(fail) or err => wrap(err),nil
 }
 return run()
}
`
	const main = `func main() {
 if n,err:=run(false); n != 4 || err != nil || observed != 4 || calls != 1 || wraps != 0 { panic("success") }
 if n,err:=run(true); n != 0 || err != fault("context") || observed != 0 || calls != 2 || wraps != 1 { panic("failure or named zeros") }
 if n,err:=nilContext(); n != 0 || err != nil || calls != 3 || wraps != 1 { panic("nil context") }
 if n,err:=nested(true); n != 0 || err != fault("context") || calls != 4 || wraps != 2 { panic("nested boundary") }
 println("PASS")
}
`
	for _, pair := range []struct{ name, source string }{{"legacy", legacy}, {"modern", modern}} {
		t.Run(pair.name, func(t *testing.T) {
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, "context.go", common+pair.source+main, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			pkg, _, err := ssautil.BuildPackage(&types.Config{}, fs, types.NewPackage("main", "main"), []*ast.File{f}, ssa.SanityCheckFunctions|ssa.InstantiateGenerics|ssa.GlobalDebug)
			if err != nil {
				t.Fatal(err)
			}
			if code := interp.Interpret(pkg, 0, types.SizesFor("gc", runtime.GOARCH), "main", nil); code != 0 {
				t.Fatalf("SSA execution failed: %d", code)
			}
		})
	}
}
