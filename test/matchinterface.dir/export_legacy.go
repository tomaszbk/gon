package lib

import "errors"

type Failure struct {
	kind int
	code string
}

func (Failure) Error() string { return "failure" }

func NewMissing() error       { return Failure{} }
func NewCoded(c string) error { return Failure{kind: 1, code: c} }

func Code(err error) string {
	var f Failure
	if errors.As(err, &f) && f.kind == 1 {
		return f.code
	}
	if errors.As(err, &f) && f.kind == 0 {
		return "missing"
	}
	return "other"
}

// Kind reports what a match in another package would extract.
func Kind(err error) string {
	var f Failure
	if errors.As(err, &f) && f.kind == 1 {
		return "coded:" + f.code
	}
	if errors.As(err, &f) && f.kind == 0 {
		return "missing:"
	}
	return "other:"
}

type Box[T any] struct {
	full  bool
	value T
}

func (Box[T]) Error() string { return "box" }

func Full[T any](v T) error { return Box[T]{full: true, value: v} }

func Unbox[T any](err error, fallback T) T {
	var b Box[T]
	if errors.As(err, &b) && b.full {
		return b.value
	}
	return fallback
}
