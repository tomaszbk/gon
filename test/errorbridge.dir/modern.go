package main

import (
	"errors"
	"fmt"
)

type IntResult = Result[int, error]
type StringResult = Result[string, error]
type AnyResult = Result[int, any]
type CoderResult = Result[int, Coder]
type PointerResult = Result[int, *CodedError]
type UnitResult = Result[struct{}, error]

func isNilResult(err error) bool { return errors.Is(err, errors.ErrNilResult) }

func describe(r IntResult) string {
	n := r or e {
		return fmt.Sprint("err:", e)
	}
	return fmt.Sprint("ok:", n)
}

func describeString(r StringResult) string {
	s := r or e {
		return fmt.Sprint("err:", e)
	}
	return "ok:" + s
}

func describeAny(r AnyResult) string {
	n := r or e {
		return fmt.Sprint("err:", e)
	}
	return fmt.Sprint("ok:", n)
}

func describeCoder(r CoderResult) string {
	n := r or e {
		return fmt.Sprint("err:", e)
	}
	return fmt.Sprint("ok:", n)
}

func describeGeneric[T any](r Result[T, error]) string {
	x := r or e {
		return fmt.Sprint("err:", e)
	}
	return fmt.Sprint("ok:", x)
}

// kind 0 succeeds, 1 fails with failure, 2 fails with a nil error.
func provide(label string, kind int) IntResult {
	mark(label)
	switch kind {
	case 0:
		return .Ok(7)
	case 1:
		return .Err(failure)
	}
	return .Err(nil)
}

func provideCoded(label string, kind int) PointerResult {
	mark(label)
	switch kind {
	case 0:
		return .Ok(7)
	case 1:
		return .Err(&CodedError{Code: 3})
	}
	return .Err(nil)
}

func provideCoder(label string, kind int) CoderResult {
	mark(label)
	switch kind {
	case 0:
		return .Ok(7)
	case 1:
		return .Err(&CodedError{Code: 4})
	}
	return .Err(nil)
}

func provideUnit(label string, fail bool) UnitResult {
	mark(label)
	if fail {
		return .Err(failure)
	}
	return .Ok(struct{}{})
}

// Go error tuples into a function returning one Result.

func tupleOne(fail bool) IntResult {
	n := count("one", 5, fail)!
	return .Ok(n * 2)
}

func tupleMultiple(fail bool) IntResult {
	n, s := pair("pair", fail)!
	return .Ok(n + len(s))
}

func tupleErrorOnly(fail bool) StringResult {
	action("action", fail)!
	mark("after")
	return .Ok("done")
}

func tupleOrder(failAt int) IntResult {
	return .Ok(side("a", 1) + count("b", 2, failAt == 2)! + count("c", 3, failAt == 3)! + side("d", 4))
}

func tupleNamed(fail bool) (r IntResult) {
	r = .Ok(100)
	defer func() { mark("defer " + describe(r)) }()
	n := count("named", 1, fail)!
	return .Ok(n)
}

func tupleAny(fail bool) AnyResult {
	n := count("any", 6, fail)!
	return .Ok(n)
}

func tupleCoder(fail bool) CoderResult {
	n := count("coder", 6, fail)!
	return .Ok(n)
}

func tupleTypedNil() IntResult {
	n := typedNil()!
	return .Ok(n)
}

func tupleLoop() IntResult {
	for n := range seq {
		count(fmt.Sprint("loop:", n), n, n == 2)!
	}
	return .Ok(0)
}

func tupleClosure() IntResult {
	inner := func() IntResult {
		n := count("closure", 1, true)!
		return .Ok(n)
	}
	_ = inner() or e {
		mark("outer sees failure")
		return .Ok(-1)
	}
	return .Ok(0)
}

func tupleHandler(fail bool) IntResult {
	n := count("handler", 2, fail) or err {
		mark("wrap")
		return .Err(fmt.Errorf("wrapped: %w", err))
	}
	return .Ok(n)
}

func tupleGeneric[T any](v T, fail bool) Result[T, error] {
	x := wrapGeneric(v, fail)!
	return .Ok(x)
}

// Result into a function returning an error tuple.

func resultToError(kind int) (int, error) {
	n := provide("provide", kind)!
	return n + 1, nil
}

func resultToErrorMany(kind int) (string, int, error) {
	mark("start")
	n := provide("provide", kind)!
	return "ok", n, nil
}

func resultToErrorOnly(fail bool) error {
	provideUnit("unit", fail)!
	mark("after")
	return nil
}

func resultToErrorNamed(kind int) (n int, err error) {
	n = 99
	defer func() { mark(fmt.Sprint("defer ", n, " ", err)) }()
	v := provide("named", kind)!
	return v, nil
}

func resultToErrorOrder(kind int) (int, error) {
	return provide("first", 0)! + provide("second", kind)! + side("sum", 0), nil
}

func resultToErrorCoded(kind int) (int, error) {
	n := provideCoded("coded", kind)!
	return n, nil
}

func resultToErrorCoder(kind int) (int, error) {
	n := provideCoder("coder", kind)!
	return n, nil
}

func resultToErrorLoop(kind int) (int, error) {
	total := 0
	for n := range seq {
		k := 0
		if n == 2 {
			k = kind
		}
		total += provide(fmt.Sprint("loop:", n), k)!
	}
	return total, nil
}

func resultToErrorHandler(kind int) (int, error) {
	n := provide("handler", kind) or e {
		mark("wrap")
		return 0, fmt.Errorf("wrapped: %w", e)
	}
	return n, nil
}

func resultToErrorClosure() (int, error) {
	inner := func() (int, error) {
		n := provide("closure", 1)!
		return n, nil
	}
	if _, err := inner(); err != nil {
		mark("outer sees failure")
		return -1, nil
	}
	return 0, nil
}

func resultToErrorGeneric[T any](v T, kind int) (T, error) {
	provide("generic", kind)!
	return v, nil
}

// The payload is a type parameter: converting it to error needs its type
// from the dictionary.
func resultToErrorTypeParam[E error](r Result[int, E]) (int, error) {
	n := r!
	return n, nil
}

func typeParamCoded(kind int) (int, error) {
	return resultToErrorTypeParam(provideCoded("typeParam", kind))
}
func typeParamError(kind int) (int, error) { return resultToErrorTypeParam(provide("typeParam", kind)) }
