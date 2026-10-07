package x

import (
	"fmt"
	"testing"
)

// Test failures are handled explicitly. Postfix ! returns an error from
// helpers whose signatures end in error, regardless of testing parameters.

func TestSuccess(t *testing.T) {
	n := parse("12") or err {
		t.Fatal(err) // MARK:never
		return
	}
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
	n := parse("bad") or err {
		t.Fatal(err) // MARK:value
		return
	}
	t.Log("unreachable", n) // MARK:unreachable
}

func TestErrorOnly(t *testing.T) {
	fail() or err {
		t.Fatal(err) // MARK:only
		return
	}
	t.Log("unreachable") // MARK:unreachable2
}

func TestMultiple(t *testing.T) {
	a, b := many() or err {
		t.Fatal(err) // MARK:multiple
		return
	}
	t.Log("unreachable", a, b) // MARK:unreachable3
}

func TestMultiline(t *testing.T) {
	n := parse(
		"bad",
	) or err {
		t.Fatal(err) // MARK:multiline
		return
	}
	t.Log("unreachable", n) // MARK:unreachable4
}

func newPointer(t *testing.T) *int {
	return pointer() or err {
		t.Fatal(err) // MARK:helper
		return nil
	}
}

func TestHelper(t *testing.T) {
	p := newPointer(t)
	t.Log("unreachable", p) // MARK:unreachable5
}

func namedResults(t *testing.T) (n int, p *int) {
	n = 7
	p = pointer() or err {
		t.Fatal(err) // MARK:named
		return
	}
	return
}

func TestNamedResults(t *testing.T) {
	n, p := namedResults(t)
	t.Log("unreachable", n, p) // MARK:unreachable6
}

func TestSubtests(t *testing.T) {
	t.Run("failing", func(t *testing.T) {
		fail() or err {
			t.Fatal(err) // MARK:subtest
			return
		}
	})
	t.Run("passing", func(t *testing.T) {
		parse("1") or err {
			t.Fatal(err) // MARK:never2
			return
		}
	})
	t.Log("parent continues") // MARK:parent
	t.Run("literal", (t) => {
		fail() or err {
			t.Fatal(err) // MARK:lambda
			return
		}
	})
}

type suite struct{ name string }

func (s suite) check(t *testing.T) {
	fail() or err {
		t.Fatal(err) // MARK:method
		return
	}
}

func TestMethod(t *testing.T) { suite{"s"}.check(t) }

func must(tb testing.TB, s string) int {
	return parse(s) or err {
		tb.Fatal(err) // MARK:tb
		return 0
	}
}

func TestTB(t *testing.T) {
	t.Run("one", func(t *testing.T) { must(t, "1") })
	t.Run("two", func(t *testing.T) { must(t, "bad") })
}

func FuzzParse(f *testing.F) {
	f.Add("1")
	f.Add("bad")
	f.Fuzz(func(t *testing.T, s string) {
		parse(s) or err {
			t.Fatal(err) // MARK:fuzz
			return
		}
	})
}

func BenchmarkFail(b *testing.B) {
	fail() or err {
		b.Fatal(err) // MARK:benchmark
		return
	}
	b.Log("unreachable") // MARK:unreachable7
}

func BenchmarkTB(b *testing.B) { must(b, "bad") }

func BenchmarkOK(b *testing.B) {
	for range b.N {
		parse("1") or err {
			b.Fatal(err) // MARK:never4
			return
		}
	}
}

// A handler is unaffected and still decides locally.
func TestHandler(t *testing.T) {
	_ = parse("bad") or err {
		t.Log(fmt.Sprintf("wrapped: %v", err)) // MARK:handler
		return
	}
}

func returnOnly(t *testing.T, text string) (n int, err error) {
	n = 99
	defer func() {
		if err != nil && n != 0 {
			panic("failure did not zero named result")
		}
	}()
	n = parse(text)!
	return n, nil
}
func contextReturn(t *testing.T) error { fail() or err => fmt.Errorf("context: %w", err); return nil }
func TestReturnOnly(t *testing.T) {
	n, err := returnOnly(t, "12")
	if n != 12 || err != nil || t.Failed() {
		panic("success failed")
	}
	n, err = returnOnly(t, "bad")
	if n != 0 || err == nil || t.Failed() {
		panic("! invoked Fatal or failed to return error")
	}
	if contextReturn(t) == nil || t.Failed() {
		panic("context invoked Fatal")
	}
	var literal func(*testing.T) error = func(u *testing.T) error { fail()!; return nil }
	var lambda func(*testing.T) error = (u) => { fail()!; return nil }
	if literal(t) == nil || lambda(t) == nil || t.Failed() {
		panic("nested propagation invoked Fatal")
	}
}
