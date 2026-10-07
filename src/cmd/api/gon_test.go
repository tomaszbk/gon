package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestGonNestedParameterNames(t *testing.T) {
	const source = `package p
func Factory() interface { Apply(value int) } { return nil }
var Callback struct { Run func(first int) struct { Next func(second int) } }
`
	check := func(source string) string {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "p.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		pkg, err := new(types.Config).Check("p", fset, []*ast.File{file}, nil)
		if err != nil {
			t.Fatal(err)
		}
		w := &Walker{parameterNames: true}
		return w.signatureString(pkg.Scope().Lookup("Factory").Type().(*types.Signature)) + ";" + w.typeString(pkg.Scope().Lookup("Callback").Type())
	}
	original := check(source)
	for _, label := range []string{"value", "first", "second"} {
		if !strings.Contains(original, label+" int") {
			t.Fatalf("missing label %s: %s", label, original)
		}
		if changed := check(strings.ReplaceAll(source, label+" int", "renamed int")); changed == original {
			t.Fatalf("label change %s is absent from API inventory", label)
		}
	}
}

func TestGonReachableParameterNames(t *testing.T) {
	const source = `package p
type private struct { Run func(field int) }
func (*private) Apply(argument int) {}
type hidden interface { Process(hiddenArg int) }
func Factory() (*private, hidden) { return nil, nil }
`
	check := func(source string) string {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "p.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		pkg, err := new(types.Config).Check("p", fset, []*ast.File{file}, nil)
		if err != nil {
			t.Fatal(err)
		}
		w := &Walker{parameterNames: true, parameterSeen: make(map[types.Type]bool), features: make(map[string]bool), current: &apiPackage{Package: pkg}}
		w.reachableParameterAPI(pkg.Scope().Lookup("Factory").Type())
		return strings.Join(w.Features(), "\n")
	}
	original := check(source)
	for _, label := range []string{"field", "argument", "hiddenArg"} {
		if !strings.Contains(original, label+" int") {
			t.Fatalf("missing reachable label %s: %s", label, original)
		}
		if changed := check(strings.ReplaceAll(source, label+" int", "renamed int")); changed == original {
			t.Fatalf("reachable label change %s is absent from API inventory", label)
		}
	}
}
