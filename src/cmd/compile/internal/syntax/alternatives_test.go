package syntax

import (
	"bytes"
	"strings"
	"testing"
)

// These tests cover native syntax and legacy switch preservation. They do not
// claim executable support for alternatives, which needs checker and lowering.
func TestAlternativesSyntax(t *testing.T) {
	cases := []struct{ name, source string }{
		{"variants", `package p; type Payment enum { default Pending; Rejected(string); Paid { Receipt string; Amount int64 } }; type Maybe[T any] enum { default None; Some(T) }`},
		{"stringvariants", `package p; type Role enum string { default Unknown(string); Teacher = "teach" + "er"; Student = text }; const text = "student"`},
		{"expression", `package p; var label = switch payment { case Payment.Pending => "pending"; case Payment.Rejected(reason) if reason != "" => reason; case Payment.Paid{Receipt: receipt, ...} => receipt; default => "other" }`},
		{"statement", `package p; func f() { switch payment { case Payment.Pending => {}; case Payment.Rejected(reason) => { println(reason) }; case Payment.Paid{Receipt: receipt, ...} => { println(receipt) } } }`},
		{"nested", `package p; var x = switch value { case Option[Option[int]].Some(Option[int].Some(x)) => x; case Option[Option[int]].Some(Option[int].None) => 0; case Option[Option[int]].None => 0 }`},
		{"legacy", `package p; type enum int; type X enum; var enumValue X; func f() { switch f() { case f() + 1: return; default: return }; switch x := f(); x { case []int{1}[0]: return }; switch x.(type) { case int: return } }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := Parse(NewFileBase(tc.name), strings.NewReader(tc.source), nil, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			if _, err := Fprint(&b, file, 0); err != nil {
				t.Fatal(err)
			}
			printed := b.String()
			again, err := Parse(NewFileBase(tc.name), strings.NewReader(printed), nil, nil, 0)
			if err != nil {
				t.Fatalf("printed syntax does not parse: %v\n%s", err, printed)
			}
			var second bytes.Buffer
			Fprint(&second, again, 0)
			if printed != second.String() {
				t.Fatalf("unstable syntax print:\n%s\n%s", printed, second.String())
			}
			var patterns, arms int
			Inspect(file, func(n Node) bool {
				switch n.(type) {
				case *MatchPattern:
					patterns++
				case *MatchArm:
					arms++
				}
				return true
			})
			if tc.name != "legacy" && tc.name != "variants" && tc.name != "stringvariants" && (patterns == 0 || arms == 0) {
				t.Fatalf("pattern traversal missed nodes: %d patterns, %d arms", patterns, arms)
			}
		})
	}
}

func TestAlternativesSyntaxErrors(t *testing.T) {
	for _, src := range []string{
		`package p; type X enum\n{ default A }`,
		`package p; type X = enum { default A }`,
		"package p; type X enum string\n{ default Unknown(string); A = \"a\" }",
		`package p; type X = enum string { default Unknown(string); A = "a" }`,
		`package p; func f() (one int, two int)? { return nil }`,
		`package p; func f() (number int)? { return nil }`,
		`package p; func f() ()? { return nil }`,
		`package p; type X enum { default A; B { Embedded } }`,
		`package p; type X enum { default A; B { *Embedded } }`,
		"package p; type X enum { default A; B { Field int `json:\"field\"` } }",
		`package p; func f() { switch x { case X.A => {}; default: return } }`,
		`package p; func f() { switch x := y; x { case X.A => {} } }`,
		`package p; func f() { switch x { case X.A{...}: return } }`,
		`package p; var x = switch v { case X.A => { println(v) } }`,
	} {
		if _, err := Parse(NewFileBase("invalid"), strings.NewReader(src), func(error) {}, nil, 0); err == nil {
			t.Errorf("accepted invalid syntax %s", src)
		}
	}
}

func TestAlternativesGroupedRecord(t *testing.T) {
	const source = `package p; type E enum { default V { X, Y int } }`
	file, err := Parse(NewFileBase("grouped"), strings.NewReader(source), nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	fields := file.DeclList[0].(*TypeDecl).Type.(*EnumType).Variants[0].Payload
	if len(fields) != 2 || fields[0].Name.Value != "X" || fields[1].Name.Value != "Y" || fields[0].Type != fields[1].Type {
		t.Fatalf("grouped payload fields: %#v", fields)
	}
	var out bytes.Buffer
	if _, err := Fprint(&out, file, 0); err != nil {
		t.Fatal(err)
	}
	again, err := Parse(NewFileBase("grouped-printed"), strings.NewReader(out.String()), nil, nil, 0)
	if err != nil {
		t.Fatalf("grouped payload print: %v\n%s", err, out.String())
	}
	if got := len(again.DeclList[0].(*TypeDecl).Type.(*EnumType).Variants[0].Payload); got != 2 {
		t.Fatalf("printed grouped field count = %d", got)
	}
}

func TestSimplifiedAlternativesSyntax(t *testing.T) {
	const source = `package p
 type Ptr = *int?
 type OptionalPtr = (*int)?
 type OptionalSlice = ([]int)?
 type Nested = (int?)?
 type F func(int?, (*int)?, (int?)?) (int?, Result[int?, error])
 type OptionalReturns func(*int) (*int)?
 type SliceReturns func([]int) ([]int)?
 type NestedReturns func(int?) (int?)?
 func groupedPointer(value *int) (*int)? { return (*int)(value) }
 func groupedSlice(value []int) ([]int)? { return value }
 func groupedNested(value int?) (int?)? { return value }
 func legacyTuple() (first *int, second []int) { return nil, nil }
 var a int? = (int)(3)
 var b int? = nil
 func f(x int?, y (int?)?, z []int?) (value Result[int?, error]) {
     return .Ok(if true { (int)(x ?? 0) } else { nil })
 }
 func g() Result[int, error] { return .Err(nil) }
`
	file, err := Parse(NewFileBase("simplified.go"), strings.NewReader(source), nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var variants int
	Inspect(file, func(node Node) bool {
		if node, ok := node.(*ContextualVariantExpr); ok {
			variants++
			if StartPos(node).Cmp(node.Name.Pos()) >= 0 || EndPos(node).Cmp(node.Pos()) <= 0 {
				t.Errorf("incorrect positions: %#v", node)
			}
		}
		return true
	})
	if variants != 2 {
		t.Fatalf("found %d contextual variants", variants)
	}
	ptr := file.DeclList[0].(*TypeDecl).Type.(*Operation)
	if _, ok := ptr.X.(*OptionalExpr); !ok {
		t.Fatalf("*int? has wrong association: %T", ptr.X)
	}
	optionalPtr := file.DeclList[1].(*TypeDecl).Type.(*OptionalExpr)
	if _, ok := optionalPtr.X.(*ParenExpr); !ok {
		t.Fatalf("(*int)? lost grouping: %T", optionalPtr.X)
	}
	var out bytes.Buffer
	if _, err := Fprint(&out, file, 0); err != nil {
		t.Fatal(err)
	}
	printed := out.String()
	file, err = Parse(NewFileBase("printed.go"), strings.NewReader(printed), nil, nil, 0)
	if err != nil {
		t.Fatalf("printed syntax failed: %v\n%s", err, printed)
	}
	out.Reset()
	if _, err := Fprint(&out, file, 0); err != nil {
		t.Fatal(err)
	}
	if out.String() != printed {
		t.Fatalf("unstable syntax:\n%s\n%s", printed, out.String())
	}
}
