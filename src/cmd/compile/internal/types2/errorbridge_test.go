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

// The postfix ! boundaries: test functions, Go error tuples and Result.
func TestErrorHandlingBoundaries(t *testing.T) {
	const prelude = `package p
import "testing"
func one() (int, error) { return 1, nil }
func many() (int, string, error) { return 1, "", nil }
func only() error { return nil }
func ptr() (*int, error) { return nil, nil }
func res() Result[int, error] { return .Ok(1) }
func resString() Result[int, string] { return .Ok(1) }
func resUnit() Result[struct{}, error] { return .Ok(struct{}{}) }
type E struct{}
func (E) Error() string { return "" }
func resConcrete() Result[int, E] { return .Ok(1) }
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
		// Test functions: Result operands.
		{"test result", test, `func TestX(t *testing.T) { n := res()!; _ = n; m := resString()!; _ = m }`, ""},
		{"test result concrete", test, `func TestX(t *testing.T) { _ = resConcrete()! }`, ""},
		{"test result unit", test, `func TestX(t *testing.T) { resUnit()! }`, ""},
		// Outside the test-function rule.
		{"not a test file", plain, `func TestX(t *testing.T) { only()! }`, "a _test.go file"},
		{"not a test file result", plain, `func TestX(t *testing.T) { _ = res()! }`, "a _test.go file"},
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
		{"result outside test", test, `func f() int { return res()! }`, "enclosing Result"},
		{"result string outside test", plain, `func f(t *testing.T) { _ = res()! }`, "enclosing Result"},
		{"test returning Result keeps result rule", test, `func f(t *testing.T) Result[int, string] { return .Ok(one()!) }`, "assignable to its error type"},
		{"test returning error keeps error rule", test, `func f(t *testing.T) error { _ = resString()!; return nil }`, "assignable to error"},
		// Go error tuples into Result.
		{"tuple to Result", plain, `func f() Result[int, error] { n := one()!; return .Ok(n) }`, ""},
		{"tuple to Result unit", plain, `func f() Result[struct{}, error] { only()!; return .Ok(struct{}{}) }`, ""},
		{"tuple to Result multiple", plain, `func f() Result[int, error] { n, s := many()!; _ = s; return .Ok(n) }`, ""},
		{"tuple to Result any", plain, `func f() Result[int, any] { n := one()!; return .Ok(n) }`, ""},
		{"tuple to Result interface", plain, `func f() Result[int, interface{ Error() string }] { n := one()!; return .Ok(n) }`, ""},
		{"tuple to Result handler", plain, `func f() Result[int, error] { n := one() or err { return .Err(err) }; return .Ok(n) }`, ""},
		{"tuple to Result string", plain, `func f() Result[int, string] { n := one()!; return .Ok(n) }`, "assignable to its error type"},
		{"tuple to Result concrete", plain, `func f() Result[int, E] { n := one()!; return .Ok(n) }`, "assignable to its error type"},
		{"tuple to Result lambda", plain, `func f() { var g func() Result[int, error] = () => { n := one()!; return .Ok(n) }; _ = g }`, ""},
		{"tuple to Result lambda string", plain, `func f() { var g func() Result[int, string] = () => { n := one()!; return .Ok(n) }; _ = g }`, "assignable to its error type"},
		{"tuple two results", plain, `func f() (Result[int, error], error) { n := one()!; return .Ok(n), nil }`, ""},
		{"tuple needs error or one Result", plain, `func f() (Result[int, error], int) { n := one()!; return .Ok(n), 0 }`, "exactly one Result"},
		// Result into error tuples.
		{"Result to error", plain, `func f() (int, error) { n := res()!; return n, nil }`, ""},
		{"Result to error only", plain, `func f() error { res()!; _ = resUnit()!; return nil }`, ""},
		{"Result to error concrete", plain, `func f() error { n := resConcrete()!; _ = n; return nil }`, ""},
		{"Result to error lambda", plain, `func f() { var g func() (int, error) = () => { n := res()!; return n, nil }; _ = g }`, ""},
		{"Result string to error", plain, `func f() (int, error) { n := resString()!; return n, nil }`, "assignable to error"},
		{"Result to non-final error", plain, `func f() (error, int) { n := res()!; return nil, n }`, "enclosing Result"},
		{"Result to Result", plain, `func f() Result[int, error] { n := res()!; return .Ok(n) }`, ""},
		{"Result to Result string", plain, `func f() Result[int, error] { n := resString()!; return .Ok(n) }`, "assignable"},
		{"Result handler unaffected", plain, `func f() int { return resString() or e { _ = e; return 0 } }`, ""},
		// Ordinary tuple propagation is unchanged.
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
