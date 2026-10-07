// Postfix ! always returns error from the nearest function, including tests.

package errorbridge

import "testing"

type E struct{}

func (E) Error() string { return "" }

type S struct{}
type TT = testing.T
type Named testing.T

func one() (int, error)          { return 1, nil }
func many() (int, string, error) { return 1, "", nil }
func only() error                { return nil }
func ptr() (*int, error)         { return nil, nil }

func TestTuple(t *testing.T) {
	n := one /* ERROR "final result of type error" */ ()!
	only /* ERROR "final result of type error" */ ()!
	a, b := many /* ERROR "final result of type error" */ ()!
	_, _, _ = n, a, b
}

func BenchmarkTuple(b *testing.B) {
	for range b.N {
		only /* ERROR "final result of type error" */ ()!
	}
}

func FuzzTuple(f *testing.F) {
	f.Add([]byte("x"))
	f.Fuzz(func(t *testing.T, data []byte) {
		only /* ERROR "final result of type error" */ ()!
		_ = data
	})
}

func newDB(tb testing.TB) *int {
	return ptr /* ERROR "final result of type error" */ ()!
}

func named(t *testing.T) (db *int, n int) {
	db = ptr /* ERROR "final result of type error" */ ()!
	return
}

func Subtests(t *testing.T) {
	t.Run("one", func(u *testing.T) {
		only /* ERROR "final result of type error" */ ()!
	})
	t.Run("lambda", (u) => {
		only /* ERROR "final result of type error" */ ()!
	})
	only /* ERROR "final result of type error" */ ()!
}

func aliased(t *TT) { only /* ERROR "final result of type error" */ ()! }

func (S) TestMethod(t *testing.T) { only /* ERROR "final result of type error" */ ()! }

func otherResults(t *testing.T) (int, string) {
	one /* ERROR "final result of type error" */ ()!
	return 1, ""
}

func handlerUnaffected(t *testing.T) {
	n := one() or err {
		t.Fatal(err)
		return
	}
	_ = n
}

// Functions that return error last keep their rules.
func returnsError(t *testing.T) error {
	only()!
	return nil
}

// Not a test function.
func blank(_ *testing.T)         { only /* ERROR "final result of type error" */ ()! }
func unnamed(*testing.T)         { only /* ERROR "final result of type error" */ ()! }
func second(n int, t *testing.T) { only /* ERROR "final result of type error" */ ()! }
func otherType(t *S)             { only /* ERROR "final result of type error" */ ()! }
func valueType(t testing.T)      { only /* ERROR "final result of type error" */ ()! }
func testingM(m *testing.M)      { only /* ERROR "final result of type error" */ ()! }
func definedFromT(t *Named)      { only /* ERROR "final result of type error" */ ()! }
func variadic(t ...*testing.T)   { only /* ERROR "final result of type error" */ ()! }
func noParams()                  { only /* ERROR "final result of type error" */ ()! }

func nested(t *testing.T) {
	func() {
		only /* ERROR "final result of type error" */ ()!
	}()
	var g func() = () => {
		only /* ERROR "final result of type error" */ ()!
	}
	g()
}

func testingError(tb testing.TB) (n int, err error) { n = one()!; return n, nil }
func contextError(t *testing.T) (int, error)        { return one() or err => err, nil }
func contextTest(t *testing.T)                      { only /* ERROR "final result of type error" */ () or err => err }
func nestedError(t *testing.T) {
	_ = func() error { only()!; return nil }
	var lambda func(*testing.T) error = (u) => { only()!; return nil }
	_ = lambda
}
