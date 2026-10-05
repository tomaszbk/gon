package ir_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
)

// TestGonInterfaceMatch builds IR for variant patterns on interface subjects.
// Error-subject patterns call a synthetic function that performs the
// errors.AsType search with interface method calls; other interface subjects
// use a plain comma-ok type assertion.
func TestGonInterfaceMatch(t *testing.T) {
	root := os.Getenv("GON_ROOT")
	if root == "" {
		t.Skip("set GON_ROOT for executable pair fixtures")
	}
	for _, variant := range []string{"legacy", "modern"} {
		t.Run(variant, func(t *testing.T) {
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, filepath.Join(root, "misc/gon/analysisfixtures/interfacematch_"+variant+".go"), nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			p, _, err := irutil.BuildPackage(&types.Config{}, fs, types.NewPackage("main", "main"), []*ast.File{f}, ir.SanityCheckFunctions|ir.InstantiateGenerics|ir.GlobalDebug)
			if err != nil {
				t.Fatal(err)
			}
			if p.Func("main") == nil || p.Func("classify") == nil {
				t.Fatal("missing IR")
			}
			var searches, asserts int
			seen := map[*ir.Function]bool{}
			for _, name := range []string{"classify", "status", "kind"} {
				for _, block := range p.Func(name).Blocks {
					for _, instr := range block.Instrs {
						switch instr := instr.(type) {
						case *ir.Call:
							if callee, ok := instr.Call.Value.(*ir.Function); ok && callee.Synthetic == "error tree search" {
								searches++
								if seen[callee] {
									continue
								}
								seen[callee] = true
								var as, unwrap, descend int
								for _, b := range callee.Blocks {
									for _, i := range b.Instrs {
										if call, ok := i.(*ir.Call); ok {
											switch {
											case call.Call.IsInvoke() && call.Call.Method.Name() == "As":
												as++
											case call.Call.IsInvoke() && call.Call.Method.Name() == "Unwrap":
												unwrap++
											case call.Call.Value == callee:
												descend++
											}
										}
									}
								}
								if as != 1 || unwrap != 2 || descend != 1 {
									t.Errorf("search %s: As %d, Unwrap %d and recursive calls %d, want 1, 2 and 1", callee.Name(), as, unwrap, descend)
								}
							}
						case *ir.TypeAssert:
							if instr.CommaOk && types.IsInterface(instr.X.Type()) && !types.Identical(instr.X.Type(), types.Universe.Lookup("error").Type()) {
								asserts++
							}
						}
					}
				}
			}
			if variant == "modern" {
				if searches == 0 {
					t.Error("error-subject patterns did not search the error tree")
				}
				if asserts == 0 {
					t.Error("interface-subject patterns did not use a type assertion")
				}
			}
		})
	}
}
