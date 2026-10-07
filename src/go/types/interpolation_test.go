package types_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestInterpolationTypes(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{`import "fmt";func f(x int)string{return $"${x:%04d}"}`, ""},
		{`import alias "fmt";func f()string{alias:=1;return $"${alias}"}`, ""},
		{`import _ "fmt";var _ = $"empty"`, ""},
		{`func f()string{return $"${1}"}`, "requires an explicit import"},
		{`import "fmt";const x = $"${1}"`, "not constant"},
		{`import "fmt";var _ = $"${1:%%}"`, "one fmt verb"},
		{`import "fmt";var _ = $"${1:%*d}"`, "one fmt verb"},
		{`import "fmt";var _ = $"${1:%[2]d}"`, "one fmt verb"},
		{`import "fmt";var _ = $"bad\z"`, "invalid interpolated string escape"},
	} {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "source.go", "package p;"+tc.source, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", tc.source, err)
		}
		info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Uses: map[*ast.Ident]types.Object{}, Interpolations: map[*ast.InterpolatedStringExpr]*ast.CallExpr{}}
		conf := types.Config{Importer: importer.Default()}
		_, err = conf.Check("p", fset, []*ast.File{f}, info)
		if tc.want == "" {
			if err != nil {
				t.Fatal(err)
			}
			if len(info.Interpolations) != 1 {
				t.Fatalf("missing lowered call: %#v", info)
			}
		} else if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("got %v, want %s", err, tc.want)
		}
	}
}

// Line directives change diagnostics, never the physical file that imports fmt.
func TestInterpolationImportFile(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files []string
		want  string
	}{
		{"same file", []string{"package p\nimport \"fmt\"\n//line generated.go:20\nfunc f() string { return $\"${1}\" }\n"}, ""},
		{"different file", []string{"package p\nimport \"fmt\"\nvar _ = fmt.Sprintf\n", "package p\n//line fmt.go:50\nvar _ = $\"${1}\"\n"}, "requires an explicit import"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			var files []*ast.File
			for i, source := range tc.files {
				name := "fmt.go"
				if i > 0 {
					name = "missing.go"
				}
				file, err := parser.ParseFile(fset, name, source, parser.SkipObjectResolution)
				if err != nil {
					t.Fatal(err)
				}
				files = append(files, file)
			}
			_, err := (&types.Config{Importer: importer.Default()}).Check("p", fset, files, nil)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}
