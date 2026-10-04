package cfg

import (
	"go/ast"
	"go/token"
	"testing"
)

func TestGonAlternatives(t *testing.T) {
	for _, tt := range []struct {
		body     string
		noReturn bool
	}{
		{`_ = Option[int].None?; panic(0)`, false},
		{`_ = switch value { case Event.A => fail()?; default => 1 }; panic(0)`, false},
		{`switch value { case Event.A => { return }; default => { panic(0) } }`, false},
		{`switch value { case Event.A if guard() => { panic(0) }; default => { panic(0) } }`, true},
		{`L: switch value { case Event.A => { break L }; default => { panic(0) } }; panic(0)`, true},
	} {
		t.Run(tt.body, func(t *testing.T) {
			g := newCondCFG(t, tt.body)
			if g.NoReturn() != tt.noReturn {
				t.Fatalf("NoReturn=%v, want %v\n%s", g.NoReturn(), tt.noReturn, g.Format(token.NewFileSet()))
			}
		})
	}
	g := newCondCFG(t, `_ = switch value { case Event.A(x) if guard(x) => first()?; default => second()? }`)
	var guards, first, second []*Block
	for _, block := range g.Blocks {
		if !block.Live {
			continue
		}
		for _, n := range block.Nodes {
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok {
					switch id.Name {
					case "guard":
						guards = append(guards, block)
					case "first":
						first = append(first, block)
					case "second":
						second = append(second, block)
					}
				}
			}
		}
	}
	if len(guards) != 1 || len(first) != 1 || len(second) != 1 {
		t.Fatalf("guard/body evaluation inventory: %d/%d/%d", len(guards), len(first), len(second))
	}
	if reaches(first[0], second[0]) || reaches(second[0], first[0]) {
		t.Fatal("selected arms must not execute one another")
	}
	if len(guards[0].Succs) != 2 || !reaches(guards[0].Succs[1], second[0]) {
		t.Fatal("false guard must continue to next arm")
	}
}
