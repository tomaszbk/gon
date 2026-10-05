// Postfix ! in test functions: a named first parameter of type *testing.T,
// *testing.B, *testing.F or testing.TB in a _test.go file.

package errorbridge

import "testing"

type E struct{}

func (E) Error() string { return "" }

type S struct{}
type TT = testing.T
type Named testing.T

func one() (int, error)              { return 1, nil }
func many() (int, string, error)     { return 1, "", nil }
func only() error                    { return nil }
func ptr() (*int, error)             { return nil, nil }
func res() Result[int, error]        { return .Ok(1) }
func resString() Result[int, string] { return .Ok(1) }
func resConcrete() Result[int, E]    { return .Ok(1) }

func TestTuple(t *testing.T) {
	n := one()!
	only()!
	a, b := many()!
	_, _, _ = n, a, b
}

func BenchmarkTuple(b *testing.B) {
	for range b.N {
		only()!
	}
}

func FuzzTuple(f *testing.F) {
	f.Add([]byte("x"))
	f.Fuzz(func(t *testing.T, data []byte) {
		only()!
		_ = data
	})
}

func newDB(tb testing.TB) *int {
	return ptr()!
}

func named(t *testing.T) (db *int, n int) {
	db = ptr()!
	return
}

func Subtests(t *testing.T) {
	t.Run("one", func(u *testing.T) {
		only()!
	})
	t.Run("lambda", (u) => {
		only()!
	})
	only()!
}

func aliased(t *TT) { only()! }

func (S) TestMethod(t *testing.T) { only()! }

func otherResults(t *testing.T) (int, string) {
	one()!
	return 1, ""
}

func TestResult(t *testing.T) {
	n := res()!
	m := resString()!
	c := resConcrete()!
	_, _, _ = n, m, c
}

func handlerUnaffected(t *testing.T) {
	n := one() or err {
		t.Fatal(err)
		return
	}
	_ = n
}

// Functions that return error last or exactly one Result keep their rules.
func returnsError(t *testing.T) error {
	only()!
	_ = res()!
	return nil
}

func returnsErrorString(t *testing.T) error {
	_ = resString /* ERROR "assignable to error" */ ()!
	return nil
}

func returnsResult(t *testing.T) Result[int, string] {
	return .Ok(one /* ERROR "error propagation into a Result requires error to be assignable" */ ()!)
}

// Not a test function.
func blank(_ *testing.T) { only /* ERROR "first named parameter" */ ()! }
func unnamed(*testing.T) { only /* ERROR "first named parameter" */ ()! }
func second(n int, t *testing.T) { only /* ERROR "first named parameter" */ ()! }
func otherType(t *S) { only /* ERROR "first named parameter" */ ()! }
func valueType(t testing.T) { only /* ERROR "first named parameter" */ ()! }
func testingM(m *testing.M) { only /* ERROR "first named parameter" */ ()! }
func definedFromT(t *Named) { only /* ERROR "first named parameter" */ ()! }
func variadic(t ...*testing.T) { only /* ERROR "first named parameter" */ ()! }
func noParams() { only /* ERROR "first named parameter" */ ()! }

func nested(t *testing.T) {
	func() {
		only /* ERROR "first named parameter" */ ()!
	}()
	var g func() = () => {
		only /* ERROR "first named parameter" */ ()!
	}
	g()
}

func resultOutside() int {
	return res /* ERROR "enclosing Result" */ ()!
}
