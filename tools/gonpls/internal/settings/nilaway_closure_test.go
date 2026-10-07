package settings_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/nilaway"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
)

func TestNilAwayClosuresWithoutObjectResolution(t *testing.T) {
	dir := t.TempDir()
	for name, contents := range map[string]string{
		"go.mod": "module example.com/closures\n\ngo 1.27\n",
		"p.go": `package p
func missing() *int { return nil }
func unsafe() int {
 p := missing()
 f := func() int { return *p }
 return f()
}
func safe() int {
 value := 1
 p := &value
 f := func() int { var s struct { Field int }; return *p + s.Field }
 return f()
}
`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	pkgs, err := packages.Load(&packages.Config{
		Dir: dir, Mode: packages.LoadAllSyntax,
		Env: append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local"),
		ParseFile: func(fset *token.FileSet, name string, contents []byte) (*ast.File, error) {
			return parser.ParseFile(fset, name, contents, parser.ParseComments|parser.SkipObjectResolution)
		},
	}, "./...")
	if err != nil {
		t.Fatal(err)
	}
	if packages.PrintErrors(pkgs) != 0 {
		t.Fatal("fixture failed to load")
	}
	graph, err := checker.Analyze([]*analysis.Analyzer{nilaway.Analyzer}, pkgs, nil)
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics int
	for _, action := range graph.Roots {
		if action.Err != nil {
			t.Fatal(action.Err)
		}
		for _, diagnostic := range action.Diagnostics {
			if strings.Contains(diagnostic.Message, "INTERNAL") {
				t.Fatal(diagnostic.Message)
			}
			line := action.Package.Fset.Position(diagnostic.Pos).Line
			if line >= 8 {
				t.Fatalf("safe closure received diagnostic: %s", diagnostic.Message)
			}
			diagnostics++
		}
	}
	if diagnostics == 0 {
		t.Fatal("ordinary Go closure nil dereference was omitted")
	}
}
