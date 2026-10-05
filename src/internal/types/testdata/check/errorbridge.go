// Postfix ! across Go error tuples and Result, and the test-function rule
// outside a _test.go file.

package errorbridge

import "testing"

type E struct{}

func (E) Error() string { return "" }

type Plain struct{}

func one() (int, error)                  { return 1, nil }
func many() (int, string, error)         { return 1, "", nil }
func only() error                        { return nil }
func res() Result[int, error]            { return .Ok(1) }
func resString() Result[int, string]     { return .Ok(1) }
func resConcrete() Result[int, E]        { return .Ok(1) }
func resUnit() Result[struct{}, error]   { return .Ok(struct{}{}) }

// Go error tuple into a function returning exactly one Result.
func tupleToResult() Result[int, error] {
	n := one()!
	only()!
	a, b := many()!
	_ = b
	return .Ok(n + a)
}

func tupleToResultAny() Result[int, any]                           { return .Ok(one()!) }
func tupleToResultInterface() Result[int, interface{ Error() string }] { return .Ok(one()!) }
func tupleToResultUnit() Result[struct{}, error] {
	only()!
	return .Ok(struct{}{})
}

func tupleToResultString() Result[int, string] {
	return .Ok(one /* ERROR "error propagation into a Result requires error to be assignable" */ ()!)
}

func tupleToResultConcrete() Result[int, E] {
	return .Ok(one /* ERROR "error propagation into a Result requires error to be assignable" */ ()!)
}

func tupleToResultAndError() (Result[int, error], int) {
	return .Ok(one /* ERROR "exactly one Result" */ ()!), 0
}

// Result into a function returning error last.
func resultToError() (int, error) {
	n := res()!
	resUnit()!
	return n, nil
}

func resultToErrorOnly() error {
	res()!
	return nil
}

func resultToErrorConcrete() error {
	n := resConcrete()!
	_ = n
	return nil
}

func resultToErrorString() (int, error) {
	n := resString /* ERROR "Result propagation into a final result of type error requires an error type assignable to error" */ ()!
	return n, nil
}

func resultToNonFinalError() (error, int) {
	n := res /* ERROR "enclosing Result" */ ()!
	return nil, n
}

func resultToPlain() int {
	return res /* ERROR "enclosing Result" */ ()!
}

// Result into Result keeps its rule.
func resultToResult() Result[int, error] {
	n := res()!
	return .Ok(n)
}

func resultToResultMismatch() Result[int, error] {
	n := resString /* ERROR "assignable error type" */ ()!
	return .Ok(n)
}

// Lambdas.
func lambdas() {
	var tuple func() Result[int, error] = () => {
		n := one()!
		return .Ok(n)
	}
	var result func() (int, error) = () => {
		n := res()!
		return n, nil
	}
	var bad func() Result[int, string] = () => {
		n := one /* ERROR "error propagation into a Result requires error to be assignable" */ ()!
		return .Ok(n)
	}
	_, _, _ = tuple, result, bad
}

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
	_ = res /* ERROR "a _test.go file" */ ()!
}

func (Plain) Helper(t *testing.T) {
	only /* ERROR "a _test.go file" */ ()!
}
