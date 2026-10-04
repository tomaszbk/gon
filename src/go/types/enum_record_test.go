package types_test

import (
	"go/ast"
	"go/token"
	. "go/types"
	"strings"
	"testing"
)

func TestEnumRecordRestrictedHead(t *testing.T) {
	const prefix = `package p; type E enum { default Empty; Record { Field int } }; `
	for _, source := range []string{
		`type Alias = E.Record`,
		`var x E.Record`,
		`var _ = new(E.Record)`,
		`var _ = E.Record(E.Empty)`,
		`var _ = E.Record`,
	} {
		t.Run(source, func(t *testing.T) {
			fset := token.NewFileSet()
			f := mustParse(fset, prefix+source)
			_, err := new(Config).Check("p", fset, []*ast.File{f}, nil)
			if err == nil || !strings.Contains(err.Error(), "record enum variant requires a keyed literal") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
