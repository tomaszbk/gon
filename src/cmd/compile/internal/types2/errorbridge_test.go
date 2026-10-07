package types2_test

import (
	"cmd/compile/internal/syntax"
	"strings"
	"testing"

	. "cmd/compile/internal/types2"
)

// checkFile type checks source as the file name and returns the error messages.
func checkFile(t *testing.T, name, source string) []string {
	t.Helper()
	f, err := syntax.Parse(syntax.NewFileBase(name), strings.NewReader(source), nil, nil, 0)
	if err != nil {
		t.Fatalf("%s: %v", source, err)
	}
	var errs []string
	conf := Config{Error: func(err error) { errs = append(errs, err.Error()) }, Importer: defaultImporter()}
	conf.Check(f.PkgName.Value, []*syntax.File{f}, nil)
	return errs
}

// Postfix ! always requires error last, including in test files.
func TestErrorHandlingBoundaries(t *testing.T) {
	const prelude = `package p
import "testing"
func one() (int, error) { return 1, nil }
func many() (int, string, error) { return 1, "", nil }
func only() error { return nil }
func ptr() (*int, error) { return nil, nil }
type E struct{}
func (E) Error() string { return "" }
type S struct{}
type TT = testing.T
type Named testing.T
`
	const test, plain = "x_test.go", "x.go"
	for _, tt := range []struct{ name, file, body, want string }{
		// Testing parameters never authorize implicit Fatal.
		{"test", test, `func TestX(t *testing.T) { n := one()!; _ = n; only()!; a, b := many()!; _, _ = a, b }`, "final result of type error"},
		{"benchmark", test, `func BenchmarkX(b *testing.B) { only()! }`, "final result of type error"},
		{"fuzz", test, `func FuzzX(f *testing.F) { f.Add(1); f.Fuzz(func(t *testing.T, b []byte) { only()! }) }`, "final result of type error"},
		{"tb helper", test, `func newDB(tb testing.TB) *int { return ptr()! }`, "final result of type error"},
		{"subtest", test, `func TestX(t *testing.T) { t.Run("x", func(u *testing.T) { only()!; u.Log() }); only()! }`, "final result of type error"},
		{"named results", test, `func helper(t *testing.T) (db *int, n int) { db = ptr()!; return }`, "final result of type error"},
		{"alias", test, `func f(t *TT) { only()! }`, "final result of type error"},
		{"method", test, `func (S) TestY(t *testing.T) { only()! }`, "final result of type error"},
		{"lambda", test, `func f(t *testing.T) { var g func(*testing.T) = (u) => { only()! }; g(t) }`, "final result of type error"},
		{"lambda arg", test, `func TestX(t *testing.T) { t.Run("x", (u) => { only()! }) }`, "final result of type error"},
		{"results other than error", test, `func f(t *testing.T) (int, string) { one()!; return 1, "" }`, "final result of type error"},
		{"handler unaffected", test, `func f(t *testing.T) { n := one() or err { t.Fatal(err); return }; _ = n }`, ""},
		// Other signatures reject propagation for the same reason.
		{"not a test file", plain, `func TestX(t *testing.T) { only()! }`, "final result of type error"},
		{"blank first param", test, `func f(_ *testing.T) { only()! }`, "final result of type error"},
		{"unnamed first param", test, `func f(*testing.T) { only()! }`, "final result of type error"},
		{"second param", test, `func f(n int, t *testing.T) { only()! }`, "final result of type error"},
		{"other type", test, `func f(t *S) { only()! }`, "final result of type error"},
		{"value type", test, `func f(t testing.T) { only()! }`, "final result of type error"},
		{"testing.M", test, `func f(m *testing.M) { only()! }`, "final result of type error"},
		{"defined from T", test, `func f(t *Named) { only()! }`, "final result of type error"},
		{"variadic", test, `func f(t ...*testing.T) { only()! }`, "final result of type error"},
		{"no params", test, `func f() { only()! }`, "final result of type error"},
		{"nested function without t", test, `func f(t *testing.T) { func() { only()! }() }`, "final result of type error"},
		{"nested lambda without t", test, `func f(t *testing.T) { var g func() = () => { only()! }; g() }`, "final result of type error"},
		{"test error tuple", test, `func f(t *testing.T) (int,error) { return one()!,nil }`, ""},
		{"tb error tuple", test, `func f(tb testing.TB) (int,string,error) { a,b:=many()!; return a,b,nil }`, ""},
		{"context return", test, `func f(t *testing.T) error { only() or err => err; return nil }`, ""},
		{"context test rejected", test, `func f(t *testing.T) { only() or err => err }`, "final result of type error"},
		{"error lambda", test, `func f(t *testing.T) { var g func(*testing.T) error = (u) => { only()!; return nil }; _ = g(t) }`, ""},
		{"nested error function", test, `func f(t *testing.T) { _ = func() error { only()!; return nil }() }`, ""},
		// Ordinary tuple propagation is unchanged.
		{"test returning error keeps error rule", test, `func f(t *testing.T) error { only()!; return nil }`, ""},
		{"tuple to error", plain, `func f() (int, error) { n := one()!; return n, nil }`, ""},
		{"tuple outside", plain, `func f() int { return one()! }`, "enclosing function"},
		{"tuple no results", plain, `func f() { only()! }`, "enclosing function"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			errs := checkFile(t, tt.file, prelude+tt.body)
			got := strings.Join(errs, "\n")
			if tt.want == "" {
				if got != "" {
					t.Fatalf("unexpected errors:\n%s", got)
				}
				return
			}
			if !strings.Contains(got, tt.want) {
				t.Fatalf("want error containing %q, got:\n%s", tt.want, got)
			}
		})
	}
}
