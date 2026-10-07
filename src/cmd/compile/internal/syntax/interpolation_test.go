package syntax

import (
	"strings"
	"testing"
)

func TestInterpolationSyntax(t *testing.T) {
	for _, source := range []string{
		`package p; var _ = $"{ } $ 50% ${x} ${price:%.2f}"`,
		`package p; var _ = $"${map[string]int{\"x\":2}[\"x\"]} ${f(name: x)} ${x[1:3]}"`,
		"package p; var _ = $`raw\n\t${if true { 1 } else { 2 }} ${\"${\"} ${$\"nested ${x}\"}`",
	} {
		source = strings.ReplaceAll(source, `\"`, `"`)
		file, err := Parse(NewFileBase("source.go"), strings.NewReader(source), nil, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		var count int
		Inspect(file, func(n Node) bool {
			if _, ok := n.(*InterpolatedStringExpr); ok {
				count++
			}
			return true
		})
		if count == 0 {
			t.Fatal("missing interpolation")
		}
		var printed strings.Builder
		if _, err := Fprint(&printed, file, LineForm); err != nil {
			t.Fatal(err)
		}
		output := printed.String()
		if _, err := Parse(NewFileBase("formatted.go"), strings.NewReader(output), nil, nil, 0); err != nil {
			t.Fatalf("bad formatting %q: %v", output, err)
		}
	}
}
func TestInterpolationInvalidSyntax(t *testing.T) {
	for _, source := range []string{`$ "x"`, `$"${}"`, `$"${x:}"`, `$"unclosed`, "$\"${x\n+y}\"", "$\"${x // comment\n}\""} {
		if _, err := Parse(NewFileBase("bad.go"), strings.NewReader("package p;var _ = "+source), nil, nil, 0); err == nil {
			t.Errorf("accepted %q", source)
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
		} {
			for _, form := range []Form{0, LineForm} {
				source := "package p; func f() (string, error) { return $" + quote + "before ${" + operand + "} after" + quote + ", nil }"
				file, err := Parse(NewFileBase("source.go"), strings.NewReader(source), nil, nil, 0)
				if err != nil {
					t.Fatal(err)
				}
				var first, second strings.Builder
				if _, err := Fprint(&first, file, form); err != nil {
					t.Fatal(err)
				}
				reparsed, err := Parse(NewFileBase("formatted.go"), strings.NewReader(first.String()), nil, nil, 0)
				if err != nil {
					t.Fatalf("bad formatting %q: %v", first.String(), err)
				}
				if _, err := Fprint(&second, reparsed, form); err != nil {
					t.Fatal(err)
				}
				if first.String() != second.String() {
					t.Fatalf("non-idempotent: %q then %q", first.String(), second.String())
				}
			}
		}
	}
}
