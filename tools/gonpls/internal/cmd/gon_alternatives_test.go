package cmd_test

import (
	"strconv"
	"strings"
	"testing"
)

func TestGonAlternativesQuery(t *testing.T) {
	const source = `package p
import "fmt"
type E enum { default Empty; Value(int) }
func matched(e E) int { return switch e { case E.Empty => 0; case E.Value(n) => n } }
func optional(o int?) int? { n:=o?; return (int?)((int)(n)) }
func fallback(o int?) int { return o ?? 3 }
func read() (int,error) { return 1,nil }
func propagated() (int,error) { n:=read()!; return n,nil }
func handled() int { return read() or err { return 0 } }
func contextual() (int,error) { return read() or problem => problem,nil }
func tested(e E) bool { return e is E.Empty }
func interpolated(n int) string { return $"value: ${n:%d}" }
func simplified() (number int?) { return (int)(3) }
`
	tree := writeTree(t, "-- go.mod --\nmodule example.com/alternatives\n\ngo 1.27\n-- p.go --\n"+source)
	for _, test := range []struct{ needle, kind, typ string }{
		{"enum {", "enum-type", ""},
		{"switch e", "match-expression", "int"},
		{"o?", "optional-propagation", "int"},
		{"o ??", "optional-coalescing", "int"},
		{"int?", "optional-type", "int?"},
		{"read()!", "error-propagation", "int"},
		{"read() or", "error-handler", "int"},
		{"read() or problem", "error-context", "int"},
		{`$"value:`, "string-interpolation", "string"},
		{"e is", "pattern-test", "bool"},
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
	case "e is":
		return 2
	case "o?":
		return 1
	case "read()!", "read() or", "read() or problem":
		return 6
	case "int?":
		return 3
	case "o ??":
		return 2
	}
	return 0
}

func TestGonTestPropagationQuery(t *testing.T) {
	const source = `package p
import "testing"
func parse() (int,error) { return 1,nil }
func helper(t *testing.T) int { return parse()! }
func returnsError(t *testing.T) (int,error) { return parse()!,nil }
func nested(t *testing.T) { _ = func() error { parse()!; return nil } }
func lambda(t *testing.T) { t.Run("sub", (u) => { parse()! }) }
`
	tree := writeTree(t, "-- go.mod --\nmodule example.com/testpropagation\n\ngo 1.27\n-- p_test.go --\n"+source)
	for _, test := range []struct{ function, kind string }{
		{"helper", "test-error-propagation"},
		{"returnsError", "error-propagation"},
		{"nested", "error-propagation"},
		{"lambda", "test-error-propagation"},
	} {
		start := strings.Index(source, "func "+test.function)
		offset := start + strings.Index(source[start:], "parse()!") + len("parse()")
		var response gonQuery
		gonJSON(t, tree, nil, &response, "query", "type", "p_test.go:#"+strconv.Itoa(offset)).checkCode(0)
		if len(response.Results) != 1 || response.Results[0].Type == nil || response.Results[0].Type.Construct != test.kind {
			t.Fatalf("%s: got %+v, want %s", test.function, response, test.kind)
		}
	}
}
