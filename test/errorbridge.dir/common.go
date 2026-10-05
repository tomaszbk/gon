package main

import (
	"errors"
	"fmt"
	"strings"
)

var failure = errors.New("failure")
var trace []string

func mark(s string) { trace = append(trace, s) }

// emit prints a scenario's observable result and its side-effect trace, and
// resets the trace for the next scenario.
func emit(name string, got any) {
	fmt.Printf("%s: %v [%s]\n", name, got, strings.Join(trace, " "))
	trace = nil
}

func side(label string, n int) int { mark(label); return n }

// CodedError is a concrete error type used as a Result error type.
type CodedError struct{ Code int }

func (e *CodedError) Error() string {
	if e == nil {
		return "coded:nil"
	}
	return fmt.Sprint("coded:", e.Code)
}

// Coder is a named interface that error implements.
type Coder interface{ Error() string }

// Conventional Go callees: an error is exactly the last result.
func count(label string, n int, fail bool) (int, error) {
	mark(label)
	if fail {
		// A failed conventional call can carry a useful partial result.
		return n, failure
	}
	return n, nil
}

func pair(label string, fail bool) (int, string, error) {
	mark(label)
	if fail {
		return 41, "partial", failure
	}
	return 42, "pair", nil
}

func action(label string, fail bool) error {
	mark(label)
	if fail {
		return failure
	}
	return nil
}

type typedNilError struct{}

func (*typedNilError) Error() string { return "typed nil" }

// typedNil fails with a non-nil error interface that holds a nil pointer.
func typedNil() (int, error) {
	mark("typedNil")
	var e *typedNilError
	return 9, e
}

func seq(yield func(int) bool) {
	mark("iterator begin")
	for i := 1; i <= 3; i++ {
		mark(fmt.Sprint("yield:", i))
		if !yield(i) {
			mark("iterator stopped")
			return
		}
	}
}

func wrapGeneric[T any](v T, fail bool) (T, error) {
	mark("generic")
	if fail {
		return v, failure
	}
	return v, nil
}

// errorKind describes an error. The nil-Result sentinel is recognized by the
// variant's isNilResult, so the text proves that both spellings agree.
func errorKind(err error) string {
	switch {
	case err == nil:
		return "nil"
	case isNilResult(err):
		return "nilResult:" + err.Error()
	}
	var coded *CodedError
	if errors.As(err, &coded) {
		return fmt.Sprint("coded[", coded != nil, "]:", err)
	}
	return "error:" + err.Error()
}

func pairKind(n int, err error) string { return fmt.Sprint(n, " ", errorKind(err)) }

func scenarios() {
	// Go error tuples into a function returning exactly one Result.
	for _, fail := range []bool{false, true} {
		prefix := fmt.Sprint(" fail=", fail)
		emit("tupleOne"+prefix, describe(tupleOne(fail)))
		emit("tupleMultiple"+prefix, describe(tupleMultiple(fail)))
		emit("tupleErrorOnly"+prefix, describeString(tupleErrorOnly(fail)))
		emit("tupleNamed"+prefix, describe(tupleNamed(fail)))
		emit("tupleAny"+prefix, describeAny(tupleAny(fail)))
		emit("tupleCoder"+prefix, describeCoder(tupleCoder(fail)))
		emit("tupleHandler"+prefix, describe(tupleHandler(fail)))
		emit("tupleGeneric int"+prefix, describeGeneric(tupleGeneric(3, fail)))
		emit("tupleGeneric string"+prefix, describeGeneric(tupleGeneric("s", fail)))
	}
	for _, failAt := range []int{0, 2, 3} {
		emit(fmt.Sprint("tupleOrder failAt=", failAt), describe(tupleOrder(failAt)))
	}
	emit("tupleTypedNil", describe(tupleTypedNil()))
	emit("tupleLoop", describe(tupleLoop()))
	emit("tupleClosure", describe(tupleClosure()))

	// Result into a function returning an error tuple. Kind 0 succeeds,
	// kind 1 fails with an error, and kind 2 fails with a nil error.
	for kind := 0; kind < 3; kind++ {
		prefix := fmt.Sprint(" kind=", kind)
		emit("resultToError"+prefix, pairKind(resultToError(kind)))
		s, n, err := resultToErrorMany(kind)
		emit("resultToErrorMany"+prefix, fmt.Sprintf("%q %s", s, pairKind(n, err)))
		emit("resultToErrorNamed"+prefix, pairKind(resultToErrorNamed(kind)))
		emit("resultToErrorOrder"+prefix, pairKind(resultToErrorOrder(kind)))
		emit("resultToErrorCoded"+prefix, pairKind(resultToErrorCoded(kind)))
		emit("resultToErrorCoder"+prefix, pairKind(resultToErrorCoder(kind)))
		emit("resultToErrorLoop"+prefix, pairKind(resultToErrorLoop(kind)))
		emit("resultToErrorHandler"+prefix, pairKind(resultToErrorHandler(kind)))
		g, err := resultToErrorGeneric("g", kind)
		emit("resultToErrorGeneric"+prefix, fmt.Sprintf("%q %s", g, errorKind(err)))
		emit("typeParamCoded"+prefix, pairKind(typeParamCoded(kind)))
		emit("typeParamError"+prefix, pairKind(typeParamError(kind)))
	}
	for _, fail := range []bool{false, true} {
		emit(fmt.Sprint("resultToErrorOnly fail=", fail), errorKind(resultToErrorOnly(fail)))
	}
	emit("resultToErrorClosure", pairKind(resultToErrorClosure()))
}

func main() { scenarios() }
