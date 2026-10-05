package main

import (
	"errors"
	"fmt"
)

// Without Result, a legacy program spells the exclusive alternatives with a
// struct and converts at every boundary by hand.
type IntResult struct {
	Value   int
	Problem error
	Failed  bool
}
type StringResult struct {
	Value   string
	Problem error
	Failed  bool
}
type AnyResult struct {
	Value   int
	Problem any
	Failed  bool
}
type CoderResult struct {
	Value   int
	Problem Coder
	Failed  bool
}
type PointerResult struct {
	Value   int
	Problem *CodedError
	Failed  bool
}
type UnitResult struct {
	Problem error
	Failed  bool
}
type GenericResult[T any] struct {
	Value   T
	Problem error
	Failed  bool
}

// The legacy equivalent of errors.ErrNilResult: a failed Result with a nil
// error still fails, so it becomes this error.
var errNilResult = errors.New("failed Result carries a nil error")

func isNilResult(err error) bool { return err == errNilResult }

func orNilResult(err error) error {
	if err == nil {
		return errNilResult
	}
	return err
}

func describe(r IntResult) string {
	if r.Failed {
		return fmt.Sprint("err:", r.Problem)
	}
	return fmt.Sprint("ok:", r.Value)
}

func describeString(r StringResult) string {
	if r.Failed {
		return fmt.Sprint("err:", r.Problem)
	}
	return "ok:" + r.Value
}

func describeAny(r AnyResult) string {
	if r.Failed {
		return fmt.Sprint("err:", r.Problem)
	}
	return fmt.Sprint("ok:", r.Value)
}

func describeCoder(r CoderResult) string {
	if r.Failed {
		return fmt.Sprint("err:", r.Problem)
	}
	return fmt.Sprint("ok:", r.Value)
}

func describeGeneric[T any](r GenericResult[T]) string {
	if r.Failed {
		return fmt.Sprint("err:", r.Problem)
	}
	return fmt.Sprint("ok:", r.Value)
}

func provide(label string, kind int) IntResult {
	mark(label)
	switch kind {
	case 0:
		return IntResult{Value: 7}
	case 1:
		return IntResult{Problem: failure, Failed: true}
	}
	return IntResult{Failed: true}
}

func provideCoded(label string, kind int) PointerResult {
	mark(label)
	switch kind {
	case 0:
		return PointerResult{Value: 7}
	case 1:
		return PointerResult{Problem: &CodedError{Code: 3}, Failed: true}
	}
	return PointerResult{Failed: true}
}

func provideCoder(label string, kind int) CoderResult {
	mark(label)
	switch kind {
	case 0:
		return CoderResult{Value: 7}
	case 1:
		return CoderResult{Problem: &CodedError{Code: 4}, Failed: true}
	}
	return CoderResult{Failed: true}
}

func provideUnit(label string, fail bool) UnitResult {
	mark(label)
	if fail {
		return UnitResult{Problem: failure, Failed: true}
	}
	return UnitResult{}
}

// Go error tuples into a function returning one Result.

func tupleOne(fail bool) IntResult {
	n, err := count("one", 5, fail)
	if err != nil {
		return IntResult{Problem: err, Failed: true}
	}
	return IntResult{Value: n * 2}
}

func tupleMultiple(fail bool) IntResult {
	n, s, err := pair("pair", fail)
	if err != nil {
		return IntResult{Problem: err, Failed: true}
	}
	return IntResult{Value: n + len(s)}
}

func tupleErrorOnly(fail bool) StringResult {
	if err := action("action", fail); err != nil {
		return StringResult{Problem: err, Failed: true}
	}
	mark("after")
	return StringResult{Value: "done"}
}

func tupleOrder(failAt int) IntResult {
	a := side("a", 1)
	b, err := count("b", 2, failAt == 2)
	if err != nil {
		return IntResult{Problem: err, Failed: true}
	}
	c, err := count("c", 3, failAt == 3)
	if err != nil {
		return IntResult{Problem: err, Failed: true}
	}
	return IntResult{Value: a + b + c + side("d", 4)}
}

func tupleNamed(fail bool) (r IntResult) {
	r = IntResult{Value: 100}
	defer func() { mark("defer " + describe(r)) }()
	n, err := count("named", 1, fail)
	if err != nil {
		return IntResult{Problem: err, Failed: true}
	}
	return IntResult{Value: n}
}

func tupleAny(fail bool) AnyResult {
	n, err := count("any", 6, fail)
	if err != nil {
		return AnyResult{Problem: err, Failed: true}
	}
	return AnyResult{Value: n}
}

func tupleCoder(fail bool) CoderResult {
	n, err := count("coder", 6, fail)
	if err != nil {
		return CoderResult{Problem: err, Failed: true}
	}
	return CoderResult{Value: n}
}

func tupleTypedNil() IntResult {
	n, err := typedNil()
	if err != nil {
		return IntResult{Problem: err, Failed: true}
	}
	return IntResult{Value: n}
}

func tupleLoop() IntResult {
	for n := range seq {
		if _, err := count(fmt.Sprint("loop:", n), n, n == 2); err != nil {
			return IntResult{Problem: err, Failed: true}
		}
	}
	return IntResult{Value: 0}
}

func tupleClosure() IntResult {
	inner := func() IntResult {
		n, err := count("closure", 1, true)
		if err != nil {
			return IntResult{Problem: err, Failed: true}
		}
		return IntResult{Value: n}
	}
	if r := inner(); r.Failed {
		mark("outer sees failure")
		return IntResult{Value: -1}
	}
	return IntResult{Value: 0}
}

func tupleHandler(fail bool) IntResult {
	n, err := count("handler", 2, fail)
	if err != nil {
		mark("wrap")
		return IntResult{Problem: fmt.Errorf("wrapped: %w", err), Failed: true}
	}
	return IntResult{Value: n}
}

func tupleGeneric[T any](v T, fail bool) GenericResult[T] {
	x, err := wrapGeneric(v, fail)
	if err != nil {
		return GenericResult[T]{Problem: err, Failed: true}
	}
	return GenericResult[T]{Value: x}
}

// Result into a function returning an error tuple.

func resultToError(kind int) (int, error) {
	r := provide("provide", kind)
	if r.Failed {
		return 0, orNilResult(r.Problem)
	}
	return r.Value + 1, nil
}

func resultToErrorMany(kind int) (string, int, error) {
	mark("start")
	r := provide("provide", kind)
	if r.Failed {
		return "", 0, orNilResult(r.Problem)
	}
	return "ok", r.Value, nil
}

func resultToErrorOnly(fail bool) error {
	r := provideUnit("unit", fail)
	if r.Failed {
		return orNilResult(r.Problem)
	}
	mark("after")
	return nil
}

func resultToErrorNamed(kind int) (n int, err error) {
	n = 99
	defer func() { mark(fmt.Sprint("defer ", n, " ", err)) }()
	r := provide("named", kind)
	if r.Failed {
		return 0, orNilResult(r.Problem)
	}
	return r.Value, nil
}

func resultToErrorOrder(kind int) (int, error) {
	a := provide("first", 0)
	if a.Failed {
		return 0, orNilResult(a.Problem)
	}
	b := provide("second", kind)
	if b.Failed {
		return 0, orNilResult(b.Problem)
	}
	return a.Value + b.Value + side("sum", 0), nil
}

func resultToErrorCoded(kind int) (int, error) {
	r := provideCoded("coded", kind)
	if r.Failed {
		var err error = r.Problem // A nil *CodedError is a non-nil error.
		return 0, err
	}
	return r.Value, nil
}

func resultToErrorCoder(kind int) (int, error) {
	r := provideCoder("coder", kind)
	if r.Failed {
		var err error = r.Problem // A nil Coder converts to a nil error.
		return 0, orNilResult(err)
	}
	return r.Value, nil
}

func resultToErrorLoop(kind int) (int, error) {
	total := 0
	for n := range seq {
		k := 0
		if n == 2 {
			k = kind
		}
		r := provide(fmt.Sprint("loop:", n), k)
		if r.Failed {
			return 0, orNilResult(r.Problem)
		}
		total += r.Value
	}
	return total, nil
}

func resultToErrorHandler(kind int) (int, error) {
	r := provide("handler", kind)
	if r.Failed {
		mark("wrap")
		return 0, fmt.Errorf("wrapped: %w", r.Problem)
	}
	return r.Value, nil
}

func resultToErrorClosure() (int, error) {
	inner := func() (int, error) {
		r := provide("closure", 1)
		if r.Failed {
			return 0, orNilResult(r.Problem)
		}
		return r.Value, nil
	}
	if _, err := inner(); err != nil {
		mark("outer sees failure")
		return -1, nil
	}
	return 0, nil
}

func resultToErrorGeneric[T any](v T, kind int) (T, error) {
	r := provide("generic", kind)
	if r.Failed {
		var zero T
		return zero, orNilResult(r.Problem)
	}
	return v, nil
}

// The payload is a type parameter.
type TypeParamResult[E error] struct {
	Value   int
	Problem E
	Failed  bool
}

func resultToErrorTypeParam[E error](r TypeParamResult[E]) (int, error) {
	if r.Failed {
		var err error = r.Problem
		return 0, orNilResult(err)
	}
	return r.Value, nil
}

func typeParamCoded(kind int) (int, error) {
	r := provideCoded("typeParam", kind)
	return resultToErrorTypeParam(TypeParamResult[*CodedError]{r.Value, r.Problem, r.Failed})
}

func typeParamError(kind int) (int, error) {
	r := provide("typeParam", kind)
	return resultToErrorTypeParam(TypeParamResult[error]{r.Value, r.Problem, r.Failed})
}
