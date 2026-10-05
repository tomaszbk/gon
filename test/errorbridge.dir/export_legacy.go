package lib

import "errors"

var Failure = errors.New("export failure")

// IntResult is the legacy spelling of Result[int, error].
type IntResult struct {
	Value   int
	Problem error
	Failed  bool
}

// GenericResult is the legacy spelling of Result[T, error].
type GenericResult[T any] struct {
	Value   T
	Problem error
	Failed  bool
}

var errNilResult = errors.New("failed Result carries a nil error")

func IsNilResult(err error) bool { return err == errNilResult }

func Source(fail bool) (int, error) {
	if fail {
		return 99, Failure
	}
	return 7, nil
}

// kind 0 succeeds, 1 fails with Failure, 2 fails with a nil error.
func Provide(kind int) IntResult {
	switch kind {
	case 0:
		return IntResult{Value: 7}
	case 1:
		return IntResult{Problem: Failure, Failed: true}
	}
	return IntResult{Failed: true}
}

func TupleToResult(fail bool) IntResult {
	n, err := Source(fail)
	if err != nil {
		return IntResult{Problem: err, Failed: true}
	}
	return IntResult{Value: n + 1}
}

func ResultToError(kind int) (int, error) {
	r := Provide(kind)
	if r.Failed {
		if r.Problem == nil {
			return 0, errNilResult
		}
		return 0, r.Problem
	}
	return r.Value + 1, nil
}

func GenericTuple[T any](v T, fail bool) GenericResult[T] {
	if _, err := Source(fail); err != nil {
		return GenericResult[T]{Problem: err, Failed: true}
	}
	return GenericResult[T]{Value: v}
}

func GenericResultToError[T any](v T, kind int) (T, error) {
	r := Provide(kind)
	if r.Failed {
		var zero T
		if r.Problem == nil {
			return zero, errNilResult
		}
		return zero, r.Problem
	}
	return v, nil
}
