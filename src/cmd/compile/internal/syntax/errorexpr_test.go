package syntax

import (
	"strings"
	"testing"
)

func TestErrorHandlingSyntax(t *testing.T) {
	for _, src := range []string{
		`func f() { f()!; f() or err { return }; (f())! }`,
		"func f() { f()!\n f()! // comment\n f()! /* comment */\n f()! /*\n comment */\n}",
		`func f() { _ = !true; _ = ! false; _ = true != false }`,
		`type or int; func f(or or) { _ = or }; type G[P or] struct{}`,
		`type or[T any] interface{ ~int }; type G[P or[int]] struct{ Value P }`,
		`type or interface{ ~int }; type G[P or | ~string] struct{ Value P }`,
		`func f() { if f() or e { _ = e; return } { }; for f() or e { return } { } }`,
		`func f() { f() or e { for { break }; switch { default: break }; return } }`,
	} {
		f, err := Parse(NewFileBase("test.go"), strings.NewReader("package p; "+src), nil, nil, CheckBranches)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		verifyPrint(t, "test.go", f)
		Inspect(f, func(n Node) bool {
			if e, ok := n.(*ErrorExpr); ok {
				_ = StartPos(e)
				_ = EndPos(e)
			}
			return true
		})
	}
}

func TestNegationNewline(t *testing.T) {
	for _, tail := range []string{"\n", " // comment\n", " /*\n*/", " /* comment */\n"} {
		src := "package p; var _ = !" + tail + "false"
		_, err := Parse(NewFileBase("test.go"), strings.NewReader(src), nil, nil, 0)
		if err == nil {
			t.Errorf("accepted newline after prefix !: %q", src)
		}
	}
}

func TestErrorHandlerBranchBoundary(t *testing.T) {
	for _, body := range []string{"break", "continue", "fallthrough"} {
		src := "package p; func f() { for { f() or err { " + body + " } } }"
		_, err := Parse(NewFileBase("test.go"), strings.NewReader(src), nil, nil, CheckBranches)
		if err == nil {
			t.Errorf("accepted escaping handler branch: %s", body)
		}
	}
}

func TestGenericOrConstraintErrors(t *testing.T) {
	for _, src := range []string{
		`package p; type G[P or]`,
		`package p; type G[P or,]`,
		`package p; type G[P or |] struct{}`,
		`package p; type G[P or[int] struct{}`,
	} {
		if _, err := Parse(NewFileBase("bad.go"), strings.NewReader(src), func(error) {}, nil, 0); err == nil {
			t.Errorf("accepted malformed generic constraint: %s", src)
		}
	}
}
