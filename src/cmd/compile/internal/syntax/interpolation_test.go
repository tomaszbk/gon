package syntax

import (
	"strings"
	"testing"
)

func TestInterpolationSyntax(t *testing.T) {
	for _, source := range []string{
		`package p; var _ = $"{ } $ 50% ${x} ${price:%.2f}"`,
		`package p; var _ = $"${map[string]int{\"x\":2}[\"x\"]} ${f(name: x)} ${x[1:3]}"`,
		`package p; var _ = $"${f?(3) ?? 0:%03d}"`,
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

func TestInterpolationUnterminatedRecovery(t *testing.T) {
	for _, literal := range []string{`$"unterminated`, `$"unterminated\`, `$"${x`, `$"${(x`, `$"${x:%03d`} {
		source := "package p\nvar bad = " + literal + "\nvar next = 1\nfunc retained() {}\n"
		var errors []error
		file, _ := Parse(NewFileBase("bad.go"), strings.NewReader(source), func(err error) { errors = append(errors, err) }, nil, 0)
		if len(errors) == 0 {
			t.Fatal("accepted unterminated interpolation")
		}
		if file == nil || len(file.DeclList) != 3 {
			t.Fatalf("lost following declarations after %q: %#v; %v", literal, file, errors)
		}
		if declaration, ok := file.DeclList[2].(*FuncDecl); !ok || declaration.Name.Value != "retained" {
			t.Fatalf("lost following function after %q: %#v", literal, file.DeclList[2])
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
