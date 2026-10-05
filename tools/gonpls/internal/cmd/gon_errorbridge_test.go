package cmd_test

import (
	"strings"
	"testing"
)

// TestGonErrorBridgeCheck runs the semantic checker on postfix ! across Go
// error tuples and Result, and in test functions: valid contexts are clean and
// an ordinary file with a testing parameter reports the test-function rule.
func TestGonErrorBridgeCheck(t *testing.T) {
	t.Parallel()
	tree := writeTree(t, `
-- go.mod --
module example.com/bridge

go 1.26
-- bridge/bridge.go --
package bridge

import "strconv"

func parse(s string) (int, error) { return strconv.Atoi(s) }

func provide(s string) Result[int, error] { return .Ok(1) }

// Go error tuples fail as Result.Err.
func Tuple(s string) Result[int, error] {
	n := parse(s)!
	return .Ok(n + 1)
}

// A failed Result fails as its error; a nil error is never returned.
func Unwrap(s string) (int, error) {
	n := provide(s)!
	return n + 1, nil
}
-- bridge/bridge_test.go --
package bridge

import "testing"

func TestTuple(t *testing.T) {
	n := parse("1")!
	if n != 1 {
		t.Fatal(n)
	}
	t.Run("sub", func(t *testing.T) {
		parse("2")!
	})
	t.Run("lambda", (u) => {
		provide("3")!
	})
}

func newValue(tb testing.TB) int {
	return parse("4")!
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
	if check.Summary.Errors != 1 || len(check.Diagnostics) != 1 {
		t.Fatalf("diagnostics: %+v", check)
	}
	d := check.Diagnostics[0]
	if d.Code != "InvalidErrorHandling" || !strings.HasSuffix(d.Location.Path, "bridge/helper.go") ||
		!strings.Contains(d.Message, "in a _test.go file, a first named parameter of type *testing.T") {
		t.Errorf("unexpected diagnostic: %+v", d)
	}
}
