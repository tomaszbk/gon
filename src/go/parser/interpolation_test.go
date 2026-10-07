package parser_test

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestInterpolationSyntax(t *testing.T) {
	for _, source := range []string{
		`$"text { } $ 50% ${x} ${price:%.2f}"`,
		`$"${map[string]int{\"key\": 2}[\"key\"]} ${slice[1:3]} ${f(name: x)}"`,
		"$`raw\n\t${if true { 1 } else { 2 }} ${\"${\"} ${$\"nested ${x}\"}`",
		`$"\${literal} ${\"quoted\"} \\${x}"`,
		`$"${f?(3) ?? 0:%03d}"`,
	} {
		source = strings.ReplaceAll(source, `\"`, `"`)
		fset := token.NewFileSet()
		expr, err := parser.ParseExprFrom(fset, "source.go", source, 0)
		if err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		interpolation, ok := expr.(*ast.InterpolatedStringExpr)
		if !ok || interpolation.Pos() != 1 || int(interpolation.End()) != len(source)+1 {
			t.Fatalf("bad shape/positions: %#v", expr)
		}
		var parts, operands int
		ast.Inspect(expr, func(n ast.Node) bool {
			switch n.(type) {
			case *ast.InterpolationPart:
				parts++
			case *ast.Ident:
				operands++
			}
			return true
		})
		if parts < 2 || operands == 0 && !strings.Contains(source, "quoted") {
			t.Fatalf("missing children: %s", source)
		}
		var first, second bytes.Buffer
		if err := format.Node(&first, fset, expr); err != nil {
			t.Fatal(err)
		}
		reparsed, err := parser.ParseExprFrom(fset, "formatted.go", first.String(), 0)
		if err != nil {
			t.Fatalf("bad formatting %q: %v", first.String(), err)
		}
		if err := format.Node(&second, fset, reparsed); err != nil {
			t.Fatal(err)
		}
		if first.String() != second.String() {
			t.Fatalf("non-idempotent: %q then %q", first.String(), second.String())
		}
		if strings.Contains(source, "raw\n\t") && !strings.Contains(first.String(), "raw\n\t") {
			t.Fatalf("raw text changed: %q", first.String())
		}
	}
}

func TestInterpolationFormattingLineStarts(t *testing.T) {
	const want = `package p

func f(x int) {
	call(
		$"${x}",
		$` + "`raw\n${x}\ntext`" + `,
	)
	_ = "prefix" +
		$"${x}"
	_ = []string{
		$"${x}",
	}
}
`
	first, err := format.Source([]byte(want))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != want {
		t.Fatalf("wrong interpolation indentation:\n%s\nwant:\n%s", first, want)
	}
	second, err := format.Source(first)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("non-idempotent interpolation formatting: %v\n%s", err, second)
	}
}

func TestInterpolationUnterminatedRecovery(t *testing.T) {
	for _, literal := range []string{`$"unterminated`, `$"unterminated\`, `$"${x`, `$"${(x`, `$"${x:%03d`} {
		source := "package p\nvar bad = " + literal + "\nvar next = 1\nfunc retained() {}\n"
		file, err := parser.ParseFile(token.NewFileSet(), "bad.go", source, parser.AllErrors)
		if err == nil {
			t.Fatal("accepted unterminated interpolation")
		}
		if file == nil || len(file.Decls) != 3 {
			t.Fatalf("lost declarations after %q: %#v; %v", literal, file, err)
		}
		if declaration, ok := file.Decls[2].(*ast.FuncDecl); !ok || declaration.Name.Name != "retained" {
			t.Fatalf("lost following function after %q: %#v", literal, file.Decls[2])
		}
	}
}

func TestErrorContextFormattingContinuation(t *testing.T) {
	const want = "package p\n\nfunc f() (int, error) {\n\treturn call() or err =>\n\t\twrap(err), nil\n}\n"
	formatted, err := format.Source([]byte(want))
	if err != nil {
		t.Fatal(err)
	}
	if string(formatted) != want {
		t.Fatalf("lost continuation after =>:\n%s", formatted)
	}
}

func TestInterpolationInvalidSyntax(t *testing.T) {
	for _, source := range []string{`$ "x"`, `$"${}"`, `$"${x:}"`, `$"${x"`, `$"unclosed`, "$\"${x\n+y}\"", "$\"${x // comment\n}\"", `$"${x; y}"`} {
		if _, err := parser.ParseExpr(source); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
}

func TestInterpolationRawFormattingPreservesText(t *testing.T) {
	source := "package p\nfunc f(x int)string{return $`start\n${ x + 1 }\n\t${x}\nend\n`}\n"
	want := "$`start\n${x + 1}\n\t${x}\nend\n`"
	first, err := format.Source([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), want) {
		t.Fatalf("raw text changed: %q", first)
	}
	second, err := format.Source(first)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("non-idempotent raw interpolation: %q then %q", first, second)
	}
}

func TestInterpolationFormattingPreservesComments(t *testing.T) {
	source := "package p\nfunc f(x int)string{return $`start\n${ /*before*/ x /*after*/ }\nend`}\n"
	formatted, err := format.Source([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "formatted.go", formatted, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var part *ast.InterpolationPart
	ast.Inspect(file, func(n ast.Node) bool {
		if p, ok := n.(*ast.InterpolationPart); ok && p.Expr != nil {
			part = p
		}
		return true
	})
	if part == nil || len(file.Comments) != 2 {
		t.Fatalf("comments missing: %s", formatted)
	}
	for _, group := range file.Comments {
		if group.Pos() < part.Pos() || group.End() > part.End() {
			t.Fatalf("comment became literal text: %s", formatted)
		}
	}
	if !strings.Contains(string(formatted), "start\n${") || !strings.Contains(string(formatted), "}\nend") {
		t.Fatalf("text changed: %q", formatted)
	}
}

func TestInterpolationFormattingPreservesFormatComments(t *testing.T) {
	for _, quote := range []string{"\"", "`"} {
		source := "package p\nfunc f(x int) string { return $" + quote + "before ${x /*comment: keep*/:%04d} after" + quote + " }\n"
		formatted, err := format.Source([]byte(source))
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "formatted.go", formatted, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		var part *ast.InterpolationPart
		ast.Inspect(file, func(n ast.Node) bool {
			if p, ok := n.(*ast.InterpolationPart); ok && p.Expr != nil {
				part = p
			}
			return true
		})
		if part == nil || part.Format != "%04d" || len(file.Comments) != 1 || file.Comments[0].End() >= part.End()-token.Pos(len(part.Format))-2 {
			t.Fatalf("comment moved into format: %s", formatted)
		}
		second, err := format.Source(formatted)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(formatted, second) {
			t.Fatalf("non-idempotent: %s then %s", formatted, second)
		}
	}
}

func TestInterpolationFormattingNestedBlocks(t *testing.T) {
	for _, quote := range []string{"\"", "`"} {
		for _, operand := range []string{
			`switch b { case true => 1; case false => 2 }`,
			`call() or err { _ = err; return "", err }`,
			`func() int { n := 1; switch n { case 1: n++; default: n-- }; return n }()`,
			`func() int { var (a = 1; b = 2); type T struct { a int; b int }; type I interface { A(); B() }; type E enum { A; B }; return a + b }()`,
			`func() int { if b { return 1 } else { return 2 } }()`,
		} {
			t.Run(quote+operand, func(t *testing.T) {
				source := "package p\nfunc f() (string, error) { return $" + quote + "before ${" + operand + "} after" + quote + ", nil }\n"
				formatted, err := format.Source([]byte(source))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := parser.ParseFile(token.NewFileSet(), "formatted.go", formatted, parser.ParseComments); err != nil {
					t.Fatalf("invalid formatted interpolation: %s\n%v", formatted, err)
				}
				second, err := format.Source(formatted)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(formatted, second) {
					t.Fatalf("non-idempotent: %s then %s", formatted, second)
				}
			})
		}
	}
}

func TestInterpolationRawExpressionContinuation(t *testing.T) {
	// Newlines separate no tokens inside ${}; they can continue an expression
	// after an identifier, as well as delimit explicitly separated statements.
	for _, operand := range []string{
		"x\n+ 1 /* comment */\n+ 2",
		"x // trailing comment\n",
		"func() int { n := 1; // first\nn++; // second\nreturn n }()",
	} {
		source := "package p\nfunc f(x int) string { return $`text ${" + operand + "}` }\n"
		formatted, err := format.Source([]byte(source))
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), "formatted.go", formatted, parser.ParseComments)
		if err != nil {
			t.Fatalf("expression continuation changed: %s\n%v", formatted, err)
		}
		var part *ast.InterpolationPart
		ast.Inspect(file, func(n ast.Node) bool {
			if p, ok := n.(*ast.InterpolationPart); ok && p.Expr != nil {
				part = p
			}
			return true
		})
		for _, group := range file.Comments {
			if part == nil || group.Pos() < part.Pos() || group.End() > part.End() {
				t.Fatalf("comment escaped interpolation operand: %s", formatted)
			}
		}
		second, err := format.Source(formatted)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(formatted, second) {
			t.Fatalf("non-idempotent: %s then %s", formatted, second)
		}
	}
}
