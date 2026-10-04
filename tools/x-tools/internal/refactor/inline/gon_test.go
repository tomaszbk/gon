package inline_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/internal/refactor/inline"
)

func TestGonCalleeSafety(t *testing.T) {
	for _, body := range []string{"return if c { 1 } else { 2 }, nil", "return read()!, nil", "return read() or err { return 0, err }, nil", "var cb func() int = () => 1; return cb(), nil", "var p *int; return *p ?? 1, nil", "return 1, nil"} {
		t.Run(body, func(t *testing.T) {
			src := []byte("package p; func read() (int,error) { return 0,nil }; func f(c bool) (int,error) { " + body + " }")
			fs := token.NewFileSet()
			file, err := parser.ParseFile(fs, "p.go", src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			info := &types.Info{OptionalConversions: map[ast.Expr]types.Type{}, Implicits: map[ast.Node]types.Object{}, Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Scopes: map[ast.Node]*types.Scope{}, Selections: map[*ast.SelectorExpr]*types.Selection{}, Instances: map[*ast.Ident]types.Instance{}, FileVersions: map[*ast.File]string{}}
			pkg, err := new(types.Config).Check("p", fs, []*ast.File{file}, info)
			if err != nil {
				t.Fatal(err)
			}
			_, err = inline.AnalyzeCallee(t.Logf, fs, pkg, info, file.Decls[1].(*ast.FuncDecl), src)
			if body == "return 1, nil" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "Gon control-flow") {
				t.Fatalf("got %v, want conservative refusal", err)
			}
		})
	}
}

func TestGonCallerSafety(t *testing.T) {
	const refuse = "error: cannot inline call .*Gon control-flow expressions"
	runTests(t, []testcase{
		{"contextual constructor", `func f(x int) int { return x }`, `func g() Result[int,string] { return .Ok(f(1)) }`, refuse},
		{"implicit Option argument", `func f(x int?) int? { return x }`, `func g() int? { return f(1) }`, "error: cannot inline function containing Gon control-flow expressions"},
		{"lambda argument", `func f(cb func(int) int) int { return cb(1) }`, `func g() int { return f((x) => x + 1) }`, refuse},
		{"coalesce argument", `func f(x *int) *int { return x }`, `func g(p, q *int) *int { return f(p ?? q) }`, refuse},
		{"safe navigation", `func f() int { return 1 }`, `type S struct{ N int }; func g(p *S) int { return p?.N ?? f() }`, refuse},
		{"conditional argument", `func f(x int) int { return x + x }`, `func g(c bool) int { return f(if c { 1 } else { 2 }) }`, refuse},
		{"nil target argument", `func f(x *int) *int { return x }`, `func g(c bool) *int { return f(if c { nil } else { nil }) }`, refuse},
		{"then branch", `func f() int { return 1 }`, `func g(c bool) int { return if c { f() } else { 2 } }`, refuse},
		{"else branch", `func f() int { return 1 }`, `func g(c bool) int { return if c { 2 } else { f() } }`, refuse},
		{"condition", `func f() bool { return true }`, `func g() int { return if f() { 1 } else { 2 } }`, refuse},
		{"sibling operand", `func f() int { return 1 }`, `func g(c bool) int { return f() + if c { 1 } else { 2 } }`, refuse},
		{"package initializer", `func f(x int) int { return x }`, `var c bool; var x = f(if c { 1 } else { 2 })`, refuse},
		{"propagation argument", `func f(x int) int { return x }`, `func read() (int, error) { return 1, nil }; func g() (int, error) { return f(read()!), nil }`, refuse},
		{"propagated call", `func f() (int, error) { return 1, nil }`, `func g() (int, error) { return f()!, nil }`, refuse},
		{"handler call", `func f() int { return 1 }`, `func read() (int, error) { return 1, nil }; func g() int { return read() or err { return f() } }`, refuse},
		{"unrelated conditional", `func f() int { return 1 }`, `func g(c bool) int { x := if c { 1 } else { 2 }; return x + f() }`, "func g(c bool) int { x := if c { 1 } else { 2 }; return x + 1 }"},
	})
}

// TestGonCallerExecution checks that conservative refusal leaves the modern
// program intact and that the equivalent legacy call still supports inlining.
// Both programs assert lazy evaluation, evaluation order and early returns.
func TestGonCallerExecution(t *testing.T) {
	baseline := os.Getenv("GON_BASELINE_GO")
	if baseline == "" {
		t.Skip("set GON_BASELINE_GO for the unmodified Go executable pair")
	}
	const common = `package main
var trace int
type failure struct{}
func (failure) Error() string { return "failure" }
func mark(n int) int { trace = trace*10+n; return n }
func read(fail bool) (int, error) { mark(2); if fail { return 99, failure{} }; return 4, nil }
func f(x int) int { return x + x }
func main() {
	for _, selected := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			trace = 0
			value, err := run(selected, fail)
			if selected && fail {
				if value != 0 || err == nil || trace != 12 { panic("early return") }
			} else if selected {
				if value != 8 || err != nil || trace != 125 { panic("then branch") }
			} else if value != 6 || err != nil || trace != 135 { panic("else branch") }
		}
	}
}
`
	sources := map[string]string{
		"legacy": common + `func run(selected, fail bool) (int, error) {
		mark(1)
		var x int
		if selected { var err error; x, err = read(fail); if err != nil { return 0, err } } else { x = mark(3) }
		v := f(x)
		mark(5)
		return v, nil
	}`,
		"modern": common + `func run(selected, fail bool) (int, error) {
		mark(1)
		v := f(if selected { read(fail)! } else { mark(3) })
		mark(5)
		return v, nil
	}`,
	}
	for _, variant := range []string{"legacy", "modern"} {
		t.Run(variant, func(t *testing.T) {
			src := []byte(sources[variant])
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "main.go", src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			info := &types.Info{OptionalConversions: map[ast.Expr]types.Type{}, Implicits: map[ast.Node]types.Object{}, Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Scopes: map[ast.Node]*types.Scope{}, Selections: map[*ast.SelectorExpr]*types.Selection{}, Instances: map[*ast.Ident]types.Instance{}, FileVersions: map[*ast.File]string{}}
			pkg, err := new(types.Config).Check("main", fset, []*ast.File{file}, info)
			if err != nil {
				t.Fatal(err)
			}
			var decl *ast.FuncDecl
			var call *ast.CallExpr
			for n := range ast.Preorder(file) {
				switch n := n.(type) {
				case *ast.FuncDecl:
					if n.Name.Name == "f" {
						decl = n
					}
				case *ast.CallExpr:
					if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "f" {
						call = n
					}
				}
			}
			callee, err := inline.AnalyzeCallee(t.Logf, fset, pkg, info, decl, src)
			if err != nil {
				t.Fatal(err)
			}
			res, err := inline.Inline(&inline.Caller{Fset: fset, Types: pkg, Info: info, File: file, Call: call}, callee, nil)
			programs := map[string][]byte{"original": src}
			if variant == "modern" {
				if err == nil || res != nil || !strings.Contains(err.Error(), "Gon control-flow") {
					t.Fatalf("got %v, %v; want refusal without edits", res, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				programs["inlined"], err = applyEdits(pkg, file.FileStart, src, res.Edits)
				if err != nil {
					t.Fatal(err)
				}
			}
			for name, program := range programs {
				dir := t.TempDir()
				path := filepath.Join(dir, "main.go")
				if err := os.WriteFile(path, program, 0600); err != nil {
					t.Fatal(err)
				}
				gonName := "gon"
				if runtime.GOOS == "windows" {
					gonName += ".exe"
				}
				toolchains := []string{filepath.Join(runtime.GOROOT(), "gon", "bin", gonName)}
				if variant == "legacy" {
					toolchains = append(toolchains, baseline)
				}
				for _, toolchain := range toolchains {
					cmd := exec.Command(toolchain, "run", path)
					cmd.Dir = dir
					for _, kv := range os.Environ() {
						key, _, _ := strings.Cut(kv, "=")
						switch key {
						case "GOROOT", "GOTOOLDIR", "GOENV", "GOTOOLCHAIN", "GOWORK", "GOFLAGS":
							continue
						}
						cmd.Env = append(cmd.Env, kv)
					}
					cmd.Env = append(cmd.Env, "GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=")
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("%s %s: %v\n%s", name, toolchain, err, out)
					}
				}
			}
		})
	}
}

func TestGonOptionalCalleeWithoutMetadata(t *testing.T) {
	const source = `package p; func f() int? { return 1 }`
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "p.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Implicits: map[ast.Node]types.Object{}, Scopes: map[ast.Node]*types.Scope{}, Selections: map[*ast.SelectorExpr]*types.Selection{}, Instances: map[*ast.Ident]types.Instance{}, FileVersions: map[*ast.File]string{}}
	pkg, err := new(types.Config).Check("p", fs, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	_, err = inline.AnalyzeCallee(t.Logf, fs, pkg, info, file.Decls[0].(*ast.FuncDecl), []byte(source))
	if err == nil || !strings.Contains(err.Error(), "contextual Option conversion") {
		t.Fatalf("unsafe inline without conversion metadata: %v", err)
	}
}
