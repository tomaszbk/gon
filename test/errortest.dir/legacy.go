package x

import (
	"fmt"
	"testing"
)

// Without postfix !, every failing call in a test needs this boilerplate. The
// handlers deliberately stay on the MARK line, so this file is not gofmt'd: the
// harness compares the line each failure is reported at with its marker.

type IntResult struct {
	Value   int
	Problem error
	Failed  bool
}
type TextResult struct {
	Value   int
	Problem string
	Failed  bool
}
type CodedResult struct {
	Value   int
	Problem *CodedError
	Failed  bool
}

func okResult() IntResult  { return IntResult{Value: 3} }
func badResult() IntResult { return IntResult{Problem: errBoom, Failed: true} }
func nilResult() IntResult { return IntResult{Failed: true} }
func textResult() TextResult {
	return TextResult{Problem: "text payload", Failed: true}
}
func codedResult() CodedResult {
	return CodedResult{Problem: &CodedError{Code: 9}, Failed: true}
}

func TestSuccess(t *testing.T) {
	n, err := parse("12")
	if err != nil { t.Fatal(err) } // MARK:never
	if err := fail(); err == nil { t.Fatal("fail succeeded") }
	if n != 12 {
		t.Fatal(n)
	}
}

func TestValue(t *testing.T) {
	t.Cleanup(func() { t.Log("cleanup ran") }) // MARK:cleanup
	defer func() { t.Log("deferred ran") }()   // MARK:deferred
	n, err := parse("bad")
	if err != nil { t.Fatal(err) } // MARK:value
	t.Log("unreachable", n)        // MARK:unreachable
}

func TestErrorOnly(t *testing.T) {
	if err := fail(); err != nil { t.Fatal(err) } // MARK:only
	t.Log("unreachable")                          // MARK:unreachable2
}

func TestMultiple(t *testing.T) {
	a, b, err := many()
	if err != nil { t.Fatal(err) } // MARK:multiple
	t.Log("unreachable", a, b)     // MARK:unreachable3
}

func TestMultiline(t *testing.T) {
	n, err := parse(
		"bad",
	)
	if err != nil { t.Fatal(err) } // MARK:multiline
	t.Log("unreachable", n)        // MARK:unreachable4
}

func newPointer(t *testing.T) *int {
	p, err := pointer()
	if err != nil { t.Fatal(err) } // MARK:helper
	return p
}

func TestHelper(t *testing.T) {
	p := newPointer(t)
	t.Log("unreachable", p) // MARK:unreachable5
}

func namedResults(t *testing.T) (n int, p *int) {
	n = 7
	p, err := pointer()
	if err != nil { t.Fatal(err) } // MARK:named
	return
}

func TestNamedResults(t *testing.T) {
	n, p := namedResults(t)
	t.Log("unreachable", n, p) // MARK:unreachable6
}

func TestSubtests(t *testing.T) {
	t.Run("failing", func(t *testing.T) {
		if err := fail(); err != nil { t.Fatal(err) } // MARK:subtest
	})
	t.Run("passing", func(t *testing.T) {
		if _, err := parse("1"); err != nil { t.Fatal(err) } // MARK:never2
	})
	t.Log("parent continues") // MARK:parent
	t.Run("literal", func(t *testing.T) {
		if err := fail(); err != nil { t.Fatal(err) } // MARK:lambda
	})
}

type suite struct{ name string }

func (s suite) check(t *testing.T) {
	if err := fail(); err != nil { t.Fatal(err) } // MARK:method
}

func TestMethod(t *testing.T) { suite{"s"}.check(t) }

func TestResult(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		r := okResult()
		if r.Failed { t.Fatal(r.Problem) } // MARK:never3
	})
	t.Run("error", func(t *testing.T) {
		r := badResult()
		if r.Failed { t.Fatal(r.Problem) } // MARK:resulterror
	})
	t.Run("nil", func(t *testing.T) {
		r := nilResult()
		if r.Failed { t.Fatal(r.Problem) } // MARK:resultnil
	})
	t.Run("text", func(t *testing.T) {
		r := textResult()
		if r.Failed { t.Fatal(r.Problem) } // MARK:resulttext
	})
	t.Run("coded", func(t *testing.T) {
		r := codedResult()
		if r.Failed { t.Fatal(r.Problem) } // MARK:coded
	})
}

func must(tb testing.TB, s string) int {
	n, err := parse(s)
	if err != nil { tb.Fatal(err) } // MARK:tb
	return n
}

func TestTB(t *testing.T) {
	t.Run("one", func(t *testing.T) { must(t, "1") })
	t.Run("two", func(t *testing.T) { must(t, "bad") })
}

func FuzzParse(f *testing.F) {
	f.Add("1")
	f.Add("bad")
	f.Fuzz(func(t *testing.T, s string) {
		if _, err := parse(s); err != nil { t.Fatal(err) } // MARK:fuzz
	})
}

func BenchmarkFail(b *testing.B) {
	if err := fail(); err != nil { b.Fatal(err) } // MARK:benchmark
	b.Log("unreachable") // MARK:unreachable7
}

func BenchmarkTB(b *testing.B) { must(b, "bad") }

func BenchmarkOK(b *testing.B) {
	for range b.N {
		if _, err := parse("1"); err != nil { b.Fatal(err) } // MARK:never4
	}
}

// A handler is unaffected and still decides locally.
func TestHandler(t *testing.T) {
	if _, err := parse("bad"); err != nil {
		t.Log(fmt.Sprintf("wrapped: %v", err)) // MARK:handler
	}
}
