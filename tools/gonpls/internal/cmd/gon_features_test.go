package cmd_test

import (
	"strconv"
	"strings"
	"testing"
)

func TestGonFeatureQuery(t *testing.T) {
	t.Parallel()
	const src = `package p
type S struct { Next *S }
var p *S
var f func(int) int = (x) => x + 1
var q = p?.Next
var v = p ?? q
`
	tree := writeTree(t, "-- go.mod --\nmodule example.com/features\n\ngo 1.26\n-- p.go --\n"+src)
	for _, test := range []struct{ needle, typ, construct string }{
		{"=>", "func(x int) int", "lambda"},
		{"?.", "*S", "nil-guard"},
		{"??", "*S", "nil-coalescing"},
	} {
		var q gonQuery
		gonJSON(t, tree, nil, &q, "query", "type", "p.go:#"+strconv.Itoa(strings.Index(src, test.needle))).checkCode(0)
		if len(q.Results) != 1 {
			t.Fatalf("results: %+v", q)
		}
		ty := q.Results[0].Type
		if ty == nil || ty.Type != test.typ || ty.Construct != test.construct || ty.Mode != "value" {
			t.Errorf("%s: %+v, want %s %s value", test.needle, ty, test.typ, test.construct)
		}
	}
}

func TestGonFeatureExplain(t *testing.T) {
	t.Parallel()
	tree := writeTree(t, "-- go.mod --\nmodule example.com/features\n")
	var ex struct {
		OK           bool
		Explanations []struct {
			Code, Kind, Summary string
			Number              *int
			Cases               []struct{ Message, Meaning, Fix string }
		}
	}
	gonJSON(t, tree, nil, &ex, "explain", "InvalidLambda", "InvalidNilSafety", "InvalidMatch").checkCode(0)
	for i, code := range []string{"InvalidLambda", "InvalidNilSafety", "InvalidMatch"} {
		e := ex.Explanations[i]
		if e.Code != code || e.Kind != "type-error" || e.Number == nil || len(e.Cases) == 0 || e.Summary == "" {
			t.Errorf("%s: %+v", code, e)
		}
	}
}
