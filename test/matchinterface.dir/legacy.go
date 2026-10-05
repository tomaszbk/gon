package main

import (
	"errors"
	"fmt"
)

const (
	apiInternal = iota
	apiRejected
	apiMissing
	apiRetry
)

type apiError struct {
	kind    int
	inner   error
	status  int
	code    string
	message string
	count   int
	reason  string
}

func (e apiError) Error() string {
	switch e.kind {
	case apiInternal:
		return fmt.Sprintf("internal(%v)", e.inner)
	case apiRejected:
		return fmt.Sprintf("rejected(%d,%s,%s)", e.status, e.code, e.message)
	case apiMissing:
		return "missing"
	default:
		return fmt.Sprintf("retry(%d,%s)", e.count, e.reason)
	}
}

const (
	dbUnknown = iota
	dbNotFound
	dbConflict
)

type dbError struct {
	kind  int
	name  string
	table string
	key   int
}

func (e dbError) Error() string {
	switch e.kind {
	case dbUnknown:
		return "db-unknown"
	case dbNotFound:
		return "db-not-found(" + e.name + ")"
	default:
		return fmt.Sprintf("db-conflict(%s,%d)", e.table, e.key)
	}
}

func newInternal(inner error) apiError { return apiError{kind: apiInternal, inner: inner} }
func newRejected(status int, code, message string) apiError {
	return apiError{kind: apiRejected, status: status, code: code, message: message}
}
func newMissing() apiError { return apiError{kind: apiMissing} }
func newRetry(n int, why string) apiError {
	return apiError{kind: apiRetry, count: n, reason: why}
}
func newNotFound(name string) dbError { return dbError{kind: dbNotFound, name: name} }
func newConflict(table string, key int) dbError {
	return dbError{kind: dbConflict, table: table, key: key}
}

func asAPI(err error) (apiError, bool) {
	var e apiError
	ok := errors.As(err, &e)
	return e, ok
}
func asDB(err error) (dbError, bool) {
	var e dbError
	ok := errors.As(err, &e)
	return e, ok
}

// Each arm searches the error tree again, in source order.
func classify(err error) string {
	if a, ok := asAPI(err); ok && a.kind == apiRejected && a.status >= 500 {
		return fmt.Sprintf("server:%d:%s", a.status, a.code)
	}
	if d, ok := asDB(err); ok && d.kind == dbNotFound {
		return "db-not-found:" + d.name
	}
	if a, ok := asAPI(err); ok && a.kind == apiRejected {
		return fmt.Sprintf("rejected:%d:%s", a.status, a.message)
	}
	if a, ok := asAPI(err); ok && a.kind == apiInternal {
		if d, ok := asDB(a.inner); ok && d.kind == dbNotFound {
			return "internal-not-found:" + d.name
		}
	}
	if a, ok := asAPI(err); ok && a.kind == apiInternal {
		return "internal"
	}
	if d, ok := asDB(err); ok && d.kind == dbConflict && d.key == 0 {
		return "conflict-zero:" + d.table
	}
	if a, ok := asAPI(err); ok && a.kind == apiRetry {
		return fmt.Sprintf("retry:%d:%s", a.count, a.reason)
	}
	if a, ok := asAPI(err); ok && a.kind == apiMissing {
		return "missing"
	}
	if d, ok := asDB(err); ok && d.kind == dbConflict {
		return fmt.Sprintf("conflict:%s:%d", d.table, d.key)
	}
	if d, ok := asDB(err); ok && d.kind == dbUnknown {
		return "db-unknown"
	}
	if errors.Is(err, io_EOF) {
		return "eof"
	}
	return "other"
}

func statusOf(err error) (status int) {
	if a, ok := asAPI(err); ok && a.kind == apiRejected {
		status = a.status
	} else if a, ok := asAPI(err); ok && a.kind == apiMissing {
		status = 404
	} else if d, ok := asDB(err); ok && d.kind == dbNotFound {
		status = 404
	} else if err == nil {
		status = 0
	} else {
		status = 500
	}
	return status
}

func onceKind(counter *int, err error) string {
	subject := observed(counter, err)
	if a, ok := asAPI(subject); ok && a.kind == apiMissing {
		return "missing"
	}
	if d, ok := asDB(subject); ok && d.kind == dbNotFound {
		return "nf:" + d.name
	}
	if a, ok := asAPI(subject); ok && a.kind == apiRetry && a.count > 1 {
		return "retry"
	}
	if *counter != 1 {
		return "multiple"
	}
	return "other"
}

func escaped(err error) func() string {
	subject := err
	if a, ok := asAPI(subject); ok && a.kind == apiRejected {
		message, status := a.message, a.status
		return func() string { return fmt.Sprintf("%s:%d", message, status) }
	}
	return func() string { return "none" }
}

const (
	circleDot = iota
	circleRadius
)

type circle struct{ kind, r int }

func (c circle) Area() int { return c.r * c.r * 3 }

const (
	squareUnit = iota
	squareSide
)

type square struct{ kind, n int }

func (s square) Area() int { return s.n * s.n }

func newRadius(r int) circle { return circle{kind: circleRadius, r: r} }
func newDot() circle         { return circle{kind: circleDot} }
func newSide(n int) square   { return square{kind: squareSide, n: n} }
func newUnit() square        { return square{kind: squareUnit} }

func shapeName(s shape) string {
	if c, ok := s.(circle); ok && c.kind == circleRadius && c.r > 10 {
		return "big-circle"
	}
	if c, ok := s.(circle); ok && c.kind == circleRadius {
		return fmt.Sprintf("circle:%d", c.r)
	}
	if q, ok := s.(square); ok && q.kind == squareUnit {
		return "unit-square"
	}
	if q, ok := s.(square); ok && q.kind == squareSide {
		return fmt.Sprintf("side:%d", q.n)
	}
	if c, ok := s.(circle); ok && c.kind == circleDot {
		return "dot"
	}
	if s == nil {
		return "nil-shape"
	}
	return "other-shape"
}

func anyKind(v any) string {
	if q, ok := v.(square); ok && q.kind == squareSide {
		return fmt.Sprintf("square:%d", q.n)
	}
	return "other"
}

type boxed[T any] struct {
	full  bool
	value T
}

func (boxed[T]) Error() string { return "boxed" }

func newFull[T any](v T) boxed[T]  { return boxed[T]{full: true, value: v} }
func newEmptyBox[T any]() boxed[T] { return boxed[T]{} }

func boxKind[T any](err error) string {
	var b boxed[T]
	if errors.As(err, &b) && b.full {
		return fmt.Sprintf("full:%v", b.value)
	}
	if errors.As(err, &b) && !b.full {
		return "empty"
	}
	return "none"
}
