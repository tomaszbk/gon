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
