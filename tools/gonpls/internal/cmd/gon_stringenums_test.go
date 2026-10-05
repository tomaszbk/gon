package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestGonStringEnumParserQualifiedName(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "role.go", `package lib
type Role enum string { default Unknown(string); Teacher = "teacher" }
func Parse(text string) Role { return Role.Parse(text) }
`, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := (&types.Config{}).Check("example/lib", fset, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	user := pkg.Scope().Lookup("Parse")
	if got := gonQualified(user); got != "example/lib.Parse" {
		t.Fatalf("user Parse qualified name = %q", got)
	}
	if owner := gonStringEnumGeneratedOwner(user); owner != nil {
		t.Fatalf("user Parse identified as an automatic member of %v", owner)
	}
	if got := gonDescribe(user, pkg).Signature; strings.Contains(got, "Role.Parse") {
		t.Fatalf("user Parse signature = %q", got)
	}
	automatic := types.EnumOf(pkg.Scope().Lookup("Role").Type()).StringParser()
	if got := gonQualified(automatic); got != "example/lib.Role.Parse" {
		t.Fatalf("automatic Parse qualified name = %q", got)
	}
	if got := gonDescribe(automatic, pkg).Signature; !strings.Contains(got, "Role.Parse") {
		t.Fatalf("automatic Parse signature = %q", got)
	}
}
