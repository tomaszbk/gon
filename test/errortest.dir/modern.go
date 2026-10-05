package x

import (
	"fmt"
	"testing"
)

// Postfix ! in a function whose first parameter is a named *testing.T,
// *testing.B, *testing.F or testing.TB reports a failure with Fatal.

type IntResult = Result[int, error]
type TextResult = Result[int, string]
type CodedResult = Result[int, *CodedError]

func okResult() IntResult  { return .Ok(3) }
func badResult() IntResult { return .Err(errBoom) }
func nilResult() IntResult { return .Err(nil) }
func textResult() TextResult {
	return .Err("text payload")
}
func codedResult() CodedResult {
	return .Err(&CodedError{Code: 9})
}

func TestSuccess(t *testing.T) {
	n := parse("12")! // MARK:never
	if err := fail(); err == nil {
		t.Fatal("fail succeeded")
	}
	if n != 12 {
		t.Fatal(n)
	}
}

func TestValue(t *testing.T) {
	t.Cleanup(func() { t.Log("cleanup ran") }) // MARK:cleanup
	defer func() { t.Log("deferred ran") }()   // MARK:deferred
	n := parse("bad")!                         // MARK:value
	t.Log("unreachable", n)                    // MARK:unreachable
}

func TestErrorOnly(t *testing.T) {
	fail()!              // MARK:only
	t.Log("unreachable") // MARK:unreachable2
}

func TestMultiple(t *testing.T) {
	a, b := many()!            // MARK:multiple
	t.Log("unreachable", a, b) // MARK:unreachable3
}

func TestMultiline(t *testing.T) {
	n := parse(
		"bad",
	)! // MARK:multiline
	t.Log("unreachable", n) // MARK:unreachable4
}

func newPointer(t *testing.T) *int {
	return pointer()! // MARK:helper
}

func TestHelper(t *testing.T) {
	p := newPointer(t)
	t.Log("unreachable", p) // MARK:unreachable5
}

func namedResults(t *testing.T) (n int, p *int) {
	n = 7
	p = pointer()! // MARK:named
	return
}

func TestNamedResults(t *testing.T) {
	n, p := namedResults(t)
	t.Log("unreachable", n, p) // MARK:unreachable6
}

func TestSubtests(t *testing.T) {
	t.Run("failing", func(t *testing.T) {
		fail()! // MARK:subtest
	})
	t.Run("passing", func(t *testing.T) {
		parse("1")! // MARK:never2
	})
	t.Log("parent continues") // MARK:parent
	t.Run("literal", (t) => {
		fail()! // MARK:lambda
	})
}

type suite struct{ name string }

func (s suite) check(t *testing.T) {
	fail()! // MARK:method
}

func TestMethod(t *testing.T) { suite{"s"}.check(t) }

func TestResult(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		okResult()! // MARK:never3
	})
	t.Run("error", func(t *testing.T) {
		badResult()! // MARK:resulterror
	})
	t.Run("nil", func(t *testing.T) {
		nilResult()! // MARK:resultnil
	})
	t.Run("text", func(t *testing.T) {
		textResult()! // MARK:resulttext
	})
	t.Run("coded", func(t *testing.T) {
		codedResult()! // MARK:coded
	})
}

func must(tb testing.TB, s string) int {
	return parse(s)! // MARK:tb
}

func TestTB(t *testing.T) {
	t.Run("one", func(t *testing.T) { must(t, "1") })
	t.Run("two", func(t *testing.T) { must(t, "bad") })
}

func FuzzParse(f *testing.F) {
	f.Add("1")
	f.Add("bad")
	f.Fuzz(func(t *testing.T, s string) {
		parse(s)! // MARK:fuzz
	})
}

func BenchmarkFail(b *testing.B) {
	fail()!              // MARK:benchmark
	b.Log("unreachable") // MARK:unreachable7
}

func BenchmarkTB(b *testing.B) { must(b, "bad") }

func BenchmarkOK(b *testing.B) {
	for range b.N {
		parse("1")! // MARK:never4
	}
}

// A handler is unaffected and still decides locally.
func TestHandler(t *testing.T) {
	_ = parse("bad") or err {
		t.Log(fmt.Sprintf("wrapped: %v", err)) // MARK:handler
		return
	}
}
