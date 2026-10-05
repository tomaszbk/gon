package settings_test

import (
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/gopls/internal/settings"
)

// TestGonAnalyzers intentionally uses the registry, not a second analyzer list.
// A newly enabled or optional analyzer automatically becomes part of this gate.
func TestGonAnalyzers(t *testing.T) {
	root := os.Getenv("GON_ROOT")
	if root == "" {
		t.Fatal("set GON_ROOT to the Gon repository")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/gonanalysis\n\ngo 1.26\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"legacy", "modern", "conditional_legacy", "conditional_modern", "features_legacy", "features_modern", "namedarguments_legacy", "namedarguments_modern", "alternatives_legacy", "alternatives_modern", "stringenums_legacy", "stringenums_modern", "simplification_legacy", "simplification_modern"} {
		content, err := os.ReadFile(filepath.Join(root, "misc/gon/analysisfixtures", name+".go"))
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(dir, name)
		if err := os.Mkdir(dst, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, "main.go"), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	content, err := os.ReadFile("testdata/gonanalysis/modern.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "modern.go"), content, 0600); err != nil {
		t.Fatal(err)
	}
	pkgs, err := packages.Load(&packages.Config{Dir: dir, Mode: packages.LoadAllSyntax, Env: append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")}, "./...")
	if err != nil {
		t.Fatal(err)
	}
	if packages.PrintErrors(pkgs) != 0 {
		t.Fatal("corpus failed to load")
	}
	// Assert the modern corpus really reaches every currently implemented node.
	seen := map[string]bool{}
	nodeTypes := map[string]bool{}
	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			for n := range ast.Preorder(f) {
				nodeTypes[reflect.TypeOf(n).Elem().Name()] = true
				switch n := n.(type) {
				case *ast.CondExpr:
					seen["conditional"] = true
				case *ast.ErrorExpr:
					if n.Body == nil {
						seen["propagation"] = true
					} else {
						seen["handler"] = true
					}
				}
			}
		}
	}
	for _, kind := range []string{"conditional", "propagation", "handler"} {
		if !seen[kind] {
			t.Fatalf("missing %s in corpus", kind)
		}
	}
	// The fork API is the inventory: adding a new public node requires extending
	// this corpus even if every analyzer happens to ignore it.
	api, err := os.ReadFile(filepath.Join(root, "api/fork.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(api), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 5 && fields[1] == "go/ast," && fields[2] == "type" && fields[4] == "struct" && !nodeTypes[fields[3]] {
			t.Errorf("new Gon AST node %s is missing from analyzer corpus", fields[3])
		}
	}
	var analyzers []*analysis.Analyzer
	for _, a := range settings.AllAnalyzers {
		analyzers = append(analyzers, a.Analyzer())
	}
	graph, err := checker.Analyze(analyzers, pkgs, &checker.Options{Sequential: true, SanityCheck: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, act := range graph.Roots {
		t.Run(act.Analyzer.Name+"/"+act.Package.PkgPath, func(t *testing.T) {
			if act.Err != nil {
				t.Fatal(act.Err)
			}
		})
	}
	t.Logf("ran %d registered analyzers on %d corpus packages", len(analyzers), len(pkgs))
}
