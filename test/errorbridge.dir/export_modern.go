package lib

import "errors"

var Failure = errors.New("export failure")

type IntResult = Result[int, error]

func IsNilResult(err error) bool { return errors.Is(err, errors.ErrNilResult) }

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
		return .Ok(7)
	case 1:
		return .Err(Failure)
	}
	return .Err(nil)
}

func TupleToResult(fail bool) IntResult {
	n := Source(fail)!
	return .Ok(n + 1)
}

func ResultToError(kind int) (int, error) {
	n := Provide(kind)!
	return n + 1, nil
}

func GenericTuple[T any](v T, fail bool) Result[T, error] {
	Source(fail)!
	return .Ok(v)
}

func GenericResultToError[T any](v T, kind int) (T, error) {
	Provide(kind)!
	return v, nil
}
