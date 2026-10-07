package cmd_test

import (
	"strings"
	"testing"
)

// TestGonTestFunctionPropagationCheck runs the semantic checker on postfix !
// in test files: error-last helpers are clean; test signatures cannot propagate.
func TestGonTestFunctionPropagationCheck(t *testing.T) {
	t.Parallel()
	tree := writeTree(t, `
-- go.mod --
module example.com/bridge

go 1.27
-- bridge/bridge.go --
package bridge

import "strconv"

func parse(s string) (int, error) { return strconv.Atoi(s) }
-- bridge/bridge_test.go --
package bridge

import "testing"

func TestTuple(t *testing.T) {
	n := parse("1") or err { t.Fatal(err); return }
	if n != 1 {
		t.Fatal(n)
	}
	t.Run("sub", func(t *testing.T) {
		parse("2")!
	})
	t.Run("lambda", (u) => {
		parse("3") or err => err
	})
}

func newValue(tb testing.TB) (int,error) {
	return parse("4")!,nil
}

func BenchmarkTuple(b *testing.B) { parse("5")! }
-- bridge/helper.go --
package bridge

import "testing"

func Helper(t *testing.T) int {
	return parse("6")!
}
`)
	var check gonCheck
	gonJSON(t, tree, nil, &check, "check", "./bridge").checkCode(1)
	if check.Summary.Errors != 4 || len(check.Diagnostics) != 4 {
		t.Fatalf("diagnostics: %+v", check)
	}
	for _, d := range check.Diagnostics {
		if d.Code != "InvalidErrorHandling" || !strings.Contains(d.Message, "final result of type error") {
			t.Errorf("unexpected diagnostic: %+v", d)
		}
	}
}
