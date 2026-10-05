package main

import (
	"errors"
	"fmt"
)

type apiError enum {
	default Internal(error)
	Rejected  {
		Status  int
		Code    string
		Message string
	}
	Missing
	Retry(int, string)
}

func (e apiError) Error() string {
	return switch e {
	case apiError.Internal(inner) => fmt.Sprintf("internal(%v)", inner)
	case apiError.Rejected{Status: status, Code: code, Message: message} => fmt.Sprintf("rejected(%d,%s,%s)", status, code, message)
	case apiError.Missing => "missing"
	case apiError.Retry(n, why) => fmt.Sprintf("retry(%d,%s)", n, why)
	}
}

type dbError enum {
	default Unknown
	NotFound(string)
	Conflict  {
		Table string
		Key   int
	}
}

func (e dbError) Error() string {
	return switch e {
	case dbError.Unknown => "db-unknown"
	case dbError.NotFound(name) => "db-not-found(" + name + ")"
	case dbError.Conflict{Table: table, Key: key} => fmt.Sprintf("db-conflict(%s,%d)", table, key)
	}
}

func newInternal(inner error) apiError { return apiError.Internal(inner) }
func newRejected(status int, code, message string) apiError {
	return apiError.Rejected{Status: status, Code: code, Message: message}
}
func newMissing() apiError { return apiError.Missing }
func newRetry(n int, why string) apiError {
	return apiError.Retry(n, why)
}
func newNotFound(name string) dbError { return dbError.NotFound(name) }
func newConflict(table string, key int) dbError {
	return dbError.Conflict{Table: table, Key: key}
}

// Each arm searches the error tree again, in source order.
func classify(err error) string {
	return switch err {
	case apiError.Rejected{Status: status, Code: code, ...} if status >= 500 => fmt.Sprintf("server:%d:%s", status, code)
	case dbError.NotFound(name) => "db-not-found:" + name
	case apiError.Rejected{Status: status, Message: message, ...} => fmt.Sprintf("rejected:%d:%s", status, message)
	case apiError.Internal(dbError.NotFound(name)) => "internal-not-found:" + name
	case apiError.Internal(_) => "internal"
	case dbError.Conflict{Table: table, Key: 0} => "conflict-zero:" + table
	case apiError.Retry(n, why) => fmt.Sprintf("retry:%d:%s", n, why)
	case apiError.Missing => "missing"
	case dbError.Conflict{Table: table, Key: key} => fmt.Sprintf("conflict:%s:%d", table, key)
	case dbError.Unknown => "db-unknown"
	case _ if errors.Is(err, io_EOF) => "eof"
	default => "other"
	}
}

func statusOf(err error) (status int) {
	switch err {
	case apiError.Rejected{Status: code, ...} => {
		status = code
	}
	case apiError.Missing => {
		status = 404
	}
	case dbError.NotFound(_) => {
		status = 404
	}
	case _ if err == nil => {
		status = 0
	}
	default => {
		status = 500
	}
	}
	return status
}

func onceKind(counter *int, err error) string {
	return switch observed(counter, err) {
	case apiError.Missing => "missing"
	case dbError.NotFound(name) => "nf:" + name
	case apiError.Retry(n, _) if n > 1 => "retry"
	case _ if *counter != 1 => "multiple"
	default => "other"
	}
}

func escaped(err error) func() string {
	return switch err {
	case apiError.Rejected{Message: message, Status: status, ...} => () => fmt.Sprintf("%s:%d", message, status)
	default => () => "none"
	}
}

type circle enum {
	default Dot
	Radius(int)
}

func (c circle) Area() int {
	return switch c {
	case circle.Dot => 0
	case circle.Radius(r) => r * r * 3
	}
}

type square enum {
	default Unit
	Side(int)
}

func (s square) Area() int {
	return switch s {
	case square.Unit => 0
	case square.Side(n) => n * n
	}
}

func newRadius(r int) circle { return circle.Radius(r) }
func newDot() circle         { return circle.Dot }
func newSide(n int) square   { return square.Side(n) }
func newUnit() square        { return square.Unit }

func shapeName(s shape) string {
	return switch s {
	case circle.Radius(r) if r > 10 => "big-circle"
	case circle.Radius(r) => fmt.Sprintf("circle:%d", r)
	case square.Unit => "unit-square"
	case square.Side(n) => fmt.Sprintf("side:%d", n)
	case circle.Dot => "dot"
	case _ if s == nil => "nil-shape"
	default => "other-shape"
	}
}

func anyKind(v any) string {
	return switch v {
	case square.Side(n) => fmt.Sprintf("square:%d", n)
	default => "other"
	}
}

type boxed[T any] enum {
	default Empty
	Full(T)
}

func (boxed[T]) Error() string { return "boxed" }

func newFull[T any](v T) boxed[T]  { return boxed[T].Full(v) }
func newEmptyBox[T any]() boxed[T] { return boxed[T].Empty }

func boxKind[T any](err error) string {
	return switch err {
	case boxed[T].Full(value) => fmt.Sprintf("full:%v", value)
	case boxed[T].Empty => "empty"
	default => "none"
	}
}
