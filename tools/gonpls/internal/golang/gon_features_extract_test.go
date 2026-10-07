package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"golang.org/x/tools/go/ast/inspector"
)

func TestGonFeatureExtraction(t *testing.T) {
	for _, test := range []struct{ name, body, selected, want string }{
		{"whole lambda", `var g func(int) int = (n) => touch(n); _ = g; return 0`, `(n) => touch(n)`, "target and evaluation order"},
		{"lambda expression body", `var g func(int) int = (n) => touch(touch(n)); _ = g; return 0`, `touch(n)`, "lambda expression body"},
		{"lambda block boundary", `var g func(int) int = (n) => { return touch(n) }; _ = g; return 0`, `touch(n)`, ""},
		{"safe chain whole", `var p *S; return p?.V ?? 0`, `p?.V ?? 0`, "target and evaluation order"},
		{"safe chain argument", `var p *S; return p?.M(touch(1)) ?? 0`, `touch(1)`, "safe-navigation chain"},
		{"coalesce right operand", `var p *S; return p?.V ?? touch(1)`, `touch(1)`, "nil-coalescing operand"},
		{"match whole", `return switch true { case true => touch(1); case false => 0 }`, `switch true { case true => touch(1); case false => 0 }`, "target and evaluation order"},
		{"match arm", `return switch true { case true => touch(1); case false => 0 }`, `touch(1)`, "match arm expression"},
		{"option whole", `func() int? { n:=(int?)(1)?; return n }();return 0`, `(int?)(1)?`, "target and evaluation order"},
		{"pattern test whole", `var o int? = 1; if o is n? { return n }; return 0`, `o is n?`, "target and evaluation order"},
		{"pattern test operand", `var o int? = 1; if o is n? && touch(n) > 0 { return n }; return 0`, `touch(n)`, "pattern-test condition"},
		{"pattern binding operand", `var o int? = 1; if o is n? && n > 0 { return n }; return 0`, `n > 0`, "pattern-test condition"},
		{"pattern test body", `var o int? = 1; if o is n? { return touch(n) }; return 0`, `touch(n)`, ""},
		{"ordinary code", `return touch(1)`, `touch(1)`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			src := `package p; type S struct{V int};func (*S) M(v int) int{return v};func touch(v int) int{return v};func f() int{` + test.body + `}`
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "p.go", src, 0)
			if err != nil {
				t.Fatal(err)
			}
			info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue), Uses: make(map[*ast.Ident]types.Object), Defs: make(map[*ast.Ident]types.Object)}
			if _, err := new(types.Config).Check("p", fset, []*ast.File{file}, info); err != nil {
				t.Fatal(err)
			}
			cur, _ := inspector.New([]*ast.File{file}).Root().FindNode(file)
			start := fset.File(file.Pos()).Pos(strings.Index(src, test.selected))
			_, err = canExtractVariable(info, cur, start, start+token.Pos(len(test.selected)), false)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want %s", err, test.want)
			}
		})
	}
}
