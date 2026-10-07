// Postfix ! on Go error tuples, and the test-function rule
// outside a _test.go file.

package errorbridge

import "testing"

type E struct{}

func (E) Error() string { return "" }

type Plain struct{}

func one() (int, error)          { return 1, nil }
func many() (int, string, error) { return 1, "", nil }
func only() error                { return nil }

// Ordinary tuple propagation is unchanged.
func tupleToError() (int, error) {
	n := one()!
	return n, nil
}

func tupleOutside() int {
	return one /* ERROR "error propagation requires an enclosing function" */ ()!
}

// The test-function rule needs a _test.go file.
func Test(t *testing.T) {
	only /* ERROR "a _test.go file" */ ()!
}

func (Plain) Helper(t *testing.T) {
	only /* ERROR "a _test.go file" */ ()!
}
