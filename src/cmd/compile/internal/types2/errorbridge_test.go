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

// The postfix ! boundaries: test functions and Go error tuples.
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
		// Test functions: tuple operands.
		{"test", test, `func TestX(t *testing.T) { n := one()!; _ = n; only()!; a, b := many()!; _, _ = a, b }`, ""},
		{"benchmark", test, `func BenchmarkX(b *testing.B) { only()! }`, ""},
		{"fuzz", test, `func FuzzX(f *testing.F) { f.Add(1); f.Fuzz(func(t *testing.T, b []byte) { only()! }) }`, ""},
		{"tb helper", test, `func newDB(tb testing.TB) *int { return ptr()! }`, ""},
		{"subtest", test, `func TestX(t *testing.T) { t.Run("x", func(u *testing.T) { only()!; u.Log() }); only()! }`, ""},
		{"named results", test, `func helper(t *testing.T) (db *int, n int) { db = ptr()!; return }`, ""},
		{"alias", test, `func f(t *TT) { only()! }`, ""},
		{"method", test, `func (S) TestY(t *testing.T) { only()! }`, ""},
		{"lambda", test, `func f(t *testing.T) { var g func(*testing.T) = (u) => { only()! }; g(t) }`, ""},
		{"lambda arg", test, `func TestX(t *testing.T) { t.Run("x", (u) => { only()! }) }`, ""},
		{"results other than error", test, `func f(t *testing.T) (int, string) { one()!; return 1, "" }`, ""},
		{"handler unaffected", test, `func f(t *testing.T) { n := one() or err { t.Fatal(err); return }; _ = n }`, ""},
		// Outside the test-function rule.
		{"not a test file", plain, `func TestX(t *testing.T) { only()! }`, "a _test.go file"},
		{"blank first param", test, `func f(_ *testing.T) { only()! }`, "first named parameter"},
		{"unnamed first param", test, `func f(*testing.T) { only()! }`, "first named parameter"},
		{"second param", test, `func f(n int, t *testing.T) { only()! }`, "first named parameter"},
		{"other type", test, `func f(t *S) { only()! }`, "first named parameter"},
		{"value type", test, `func f(t testing.T) { only()! }`, "first named parameter"},
		{"testing.M", test, `func f(m *testing.M) { only()! }`, "first named parameter"},
		{"defined from T", test, `func f(t *Named) { only()! }`, "first named parameter"},
		{"variadic", test, `func f(t ...*testing.T) { only()! }`, "first named parameter"},
		{"no params", test, `func f() { only()! }`, "first named parameter"},
		{"nested function without t", test, `func f(t *testing.T) { func() { only()! }() }`, "first named parameter"},
		{"nested lambda without t", test, `func f(t *testing.T) { var g func() = () => { only()! }; g() }`, "first named parameter"},
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
