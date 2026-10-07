// Postfix ! on Go error tuples has the same return protocol in every file.

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

// A testing parameter cannot replace an error result.
func Test(t *testing.T) {
	only /* ERROR "final result of type error" */ ()!
}

func (Plain) Helper(t *testing.T) {
	only /* ERROR "final result of type error" */ ()!
}
