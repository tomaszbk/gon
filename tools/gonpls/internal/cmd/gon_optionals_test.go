package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"golang.org/x/tools/internal/diff"
)

func TestGonNativeOptionalMigration(t *testing.T) {
	source := `package p
// Preserve this comment and user-defined names.
type Custom struct { Some int; None bool }
type Nested = Option[Option[int]]
var present = Option[*int].Some(nil)
var absent = Option[*int].None
var constructor = Option[int].Some
var contextual int? = .Some(7)
var nested Nested = .Some(nil)
func label(n Nested) int { return switch n { case Nested.None => 0; case Nested.Some(Option[int].None) => 1; case Nested.Some(Option[int].Some(value)) => value } }
func shadow() { type Option[T any] struct{Some T}; _ = Option[int]{Some: 3} }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", source, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue), Uses: make(map[*ast.Ident]types.Object), Defs: make(map[*ast.Ident]types.Object), OptionalConversions: make(map[ast.Expr]types.Type)}
	conf := types.Config{MigrateOptionals: true}
	pkg, err := conf.Check("p", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	edits, err := gonOptionalEdits(fset, file, info, pkg, []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	updated, err := diff.Apply(source, edits)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(updated, "Option[int]{Some: 3}") || !strings.Contains(updated, "// Preserve this comment") {
		t.Fatalf("changed user names/comment: %s", updated)
	}
	if strings.Contains(updated, "Nested.Some") || strings.Contains(updated, ".Some(7)") || strings.Contains(updated, ".None\n") {
		t.Fatalf("retired syntax remains: %s", updated)
	}
	next := token.NewFileSet()
	f, err := parser.ParseFile(next, "p.go", updated, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := new(types.Config).Check("p", next, []*ast.File{f}, nil); err != nil {
		t.Fatalf("native output: %v\n%s", err, updated)
	}
	if types.Universe.Lookup("Option") != nil {
		t.Fatal("migration polluted Universe")
	}
	if _, err := new(types.Config).Check("p", fset, []*ast.File{file}, nil); err == nil {
		t.Fatal("ordinary checker accepted retired constructors")
	}
	// A type/constructor rewrite must not silently discard embedded comments.
	for _, source := range []string{
		"package p; var n Option[/* payload note */ int]",
		"package p; var n = Option[int].Some(7 /* argument note */)",
	} {
		fs := token.NewFileSet()
		f, err := parser.ParseFile(fs, "p.go", source, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue), Uses: make(map[*ast.Ident]types.Object)}
		pkg, err := conf.Check("p", fs, []*ast.File{f}, info)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := gonOptionalEdits(fs, f, info, pkg, []byte(source)); err == nil || !strings.Contains(err.Error(), "embedded comment") {
			t.Fatalf("comment loss was not declined: %v", err)
		}
	}
}
