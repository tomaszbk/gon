package cmd_test

import (
	"strconv"
	"strings"
	"testing"
)

func TestGonAlternativesQuery(t *testing.T) {
	const source = `package p
type E enum { default Empty; Value(int) }
func matched(e E) int { return switch e { case E.Empty => 0; case E.Value(n) => n } }
func optional(o int?) int? { n:=o?; return (int?)((int)(n)) }
func fallback(o int?) int { return o ?? 3 }
func propagated(r Result[int,error]) Result[int,error] { n:=r!; return Result[int,error].Ok(n) }
func handled(r Result[int,error]) int { return r or err { return 0 } }
func simplified() (number int?) { return (int)(3) }
func shortResult() (number Result[int,error]) { return .Ok(4) }
`
	tree := writeTree(t, "-- go.mod --\nmodule example.com/alternatives\n\ngo 1.27\n-- p.go --\n"+source)
	for _, test := range []struct{ needle, kind, typ string }{
		{"enum {", "enum-type", ""},
		{"switch e", "match-expression", "int"},
		{"o?", "optional-propagation", "int"},
		{"o ??", "optional-coalescing", "int"},
		{"r!", "result-propagation", "int"},
		{"r or", "result-handler", "int"},
		{"int?", "optional-type", "int?"},
		{".Ok(4)", "contextual-constructor", "Result[int, error]"},
	} {
		var response gonQuery
		gonJSON(t, tree, nil, &response, "query", "type", "p.go:#"+strconv.Itoa(strings.Index(source, test.needle)+queryOperatorOffset(test.needle))).checkCode(0)
		if len(response.Results) != 1 || response.Results[0].Type == nil {
			t.Fatalf("missing type for %s: %+v", test.needle, response)
		}
		actual := response.Results[0].Type
		if actual.Construct != test.kind || test.typ != "" && actual.Type != test.typ {
			t.Fatalf("%s: got %+v, want %s/%s", test.needle, actual, test.kind, test.typ)
		}
		if strings.Contains(actual.Underlying, "$gon") {
			t.Fatalf("enum backing leaked into semantic query: %+v", actual)
		}
	}
}

func queryOperatorOffset(needle string) int {
	switch needle {
	case "o?", "r!":
		return 1
	case "int?":
		return 3
	case "o ??", "r or":
		return 2
	}
	return 0
}

// TestGonResultMethods drives the semantic commands over methods whose
// signatures mention the predeclared Result, a named type with no package:
// pointer and value receivers, a generic receiver, interface methods and an
// optional Result. Indexing their method sets once crashed the engine.
func TestGonResultMethods(t *testing.T) {
	t.Parallel()
	tree := writeTree(t, `-- go.mod --
module example.com/results

go 1.27
-- svc/svc.go --
package svc

import (
	"context"
	"errors"
)

type Course struct{ Name string }

type Server struct{ courses []Course }

// ListCourses returns the courses.
func (s *Server) ListCourses(ctx context.Context) Result[[]Course, error] {
	return .Ok(s.courses)
}

func (s Server) ValueRecv(n int) Result[int, error] {
	if n < 0 {
		return .Err(errors.New("negative"))
	}
	return .Ok(n)
}

func (s *Server) listOptional(ctx context.Context) Result[Course, error]? { return nil }

type Store interface {
	Load(id int) Result[Course, error]
	Save(c Course) Result[struct{}, error]
}

type memStore struct{}

func (memStore) Load(id int) Result[Course, error]     { return .Ok(Course{}) }
func (memStore) Save(c Course) Result[struct{}, error] { return .Ok(struct{}{}) }

type Box[T any] struct{ v T }

func (b Box[T]) Get() Result[T, error] { return .Ok(b.v) }

func Use(s *Server, st Store) Result[int, error] {
	c := st.Load(1)!
	_ = c
	courses := s.ListCourses(context.Background())!
	return .Ok(len(courses) + s.ValueRecv(2)!)
}
`)

	res := gon(t, tree, nil, "check", "./...")
	res.checkCode(0)
	res.checkStdout("0 error")

	var syms gonQuery
	gonJSON(t, tree, nil, &syms, "query", "symbols", "ListCourses", "Load", "Get").checkCode(0)
	for i, want := range []string{
		"example.com/results/svc.Server.ListCourses",
		"example.com/results/svc.Store.Load",
		"example.com/results/svc.Box.Get",
	} {
		items := syms.Results[i].Items
		if len(items) == 0 || items[0].Object.Name != want {
			t.Errorf("symbols #%d: %+v, want %s first", i, items, want)
		}
	}

	// Declarations of methods whose signature is spelled with Result.
	var def gonQuery
	gonJSON(t, tree, nil, &def, "query", "def", "./svc.Server.ListCourses", "./svc.Server.listOptional", "./svc.Box.Get").checkCode(0)
	for i, want := range []string{
		"func (*Server).ListCourses(ctx context.Context) Result[[]Course, error]",
		"func (*Server).listOptional(ctx context.Context) Result[Course, error]?",
		"func (Box[T]).Get() Result[T, error]",
	} {
		if it := def.Results[i].Items; len(it) != 1 || it[0].Object.Signature != want {
			t.Errorf("def #%d: %+v, want signature %q", i, it, want)
		}
	}

	// References to a method that returns Result.
	var refs gonQuery
	gonJSON(t, tree, nil, &refs, "query", "refs", "./svc.Server.ListCourses", "./svc.Store.Load").checkCode(0)
	if refs.Results[0].Total != 2 || refs.Results[1].Total != 2 {
		t.Errorf("refs: %+v", refs)
	}

	// Implementations are found through the method-set fingerprints.
	var impls gonQuery
	gonJSON(t, tree, nil, &impls, "query", "impls", "./svc.Store", "./svc.memStore", "./svc.Store.Load").checkCode(0)
	if r := impls.Results[0]; r.Total != 1 || !strings.Contains(r.Items[0].Text, "type memStore struct") {
		t.Errorf("implementations of Store: %+v", r)
	}
	if r := impls.Results[1]; r.Total != 1 || !strings.Contains(r.Items[0].Text, "type Store interface") {
		t.Errorf("types implemented by memStore: %+v", r)
	}
	if r := impls.Results[2]; r.Total != 1 || !strings.Contains(r.Items[0].Text, "func (memStore) Load") {
		t.Errorf("implementations of Store.Load: %+v", r)
	}
}
