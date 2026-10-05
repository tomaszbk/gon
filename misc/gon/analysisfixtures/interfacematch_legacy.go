package main

type apiError struct {
	kind    int // 0 Internal, 1 Rejected, 2 Missing, 3 Retry
	inner   error
	status  int
	message string
	count   int
}

func (e apiError) Error() string { return "api" }

type dbError struct {
	kind int // 0 Unknown, 1 NotFound
	name string
}

func (e dbError) Error() string { return "db" }

type plain string

func (p plain) Error() string { return string(p) }

// wrap unwraps to one error; a nil chain ends the search.
type wrap struct{ next error }

func (w wrap) Error() string { return "wrap" }
func (w wrap) Unwrap() error { return w.next }

// many unwraps to several errors, some of them nil.
type many []error

func (m many) Error() string   { return "many" }
func (m many) Unwrap() []error { return m }

// shim reports a database error through its As method.
type shim struct{ name string }

func (s shim) Error() string { return "shim" }
func (s shim) As(target any) bool {
	if t, ok := target.(*dbError); ok {
		*t = dbError{kind: 1, name: s.name}
		return true
	}
	return false
}

// find is errors.As without reflection: test recognizes the wanted dynamic
// type and target is what As methods receive.
func find(err error, test func(error) bool, target any) bool {
	for err != nil {
		if test(err) {
			return true
		}
		if x, ok := err.(interface{ As(any) bool }); ok && x.As(target) {
			return true
		}
		switch x := err.(type) {
		case interface{ Unwrap() error }:
			err = x.Unwrap()
		case interface{ Unwrap() []error }:
			for _, err := range x.Unwrap() {
				if err != nil && find(err, test, target) {
					return true
				}
			}
			return false
		default:
			return false
		}
	}
	return false
}

func asAPI(err error) (apiError, bool) {
	var out apiError
	ok := find(err, func(e error) bool {
		v, ok := e.(apiError)
		if ok {
			out = v
		}
		return ok
	}, &out)
	return out, ok
}

func asDB(err error) (dbError, bool) {
	var out dbError
	ok := find(err, func(e error) bool {
		v, ok := e.(dbError)
		if ok {
			out = v
		}
		return ok
	}, &out)
	return out, ok
}

type boxed[T any] struct {
	full  bool
	value T
}

func (boxed[T]) Error() string { return "box" }

func asBox[T any](err error) (boxed[T], bool) {
	var out boxed[T]
	ok := find(err, func(e error) bool {
		v, ok := e.(boxed[T])
		if ok {
			out = v
		}
		return ok
	}, &out)
	return out, ok
}

func boxKind[T any](err error) string {
	if b, ok := asBox[T](err); ok && b.full {
		return "full"
	}
	if b, ok := asBox[T](err); ok && !b.full {
		return "empty"
	}
	return "none"
}

type shape interface{ Area() int }
type square struct{ kind, n int }

func (s square) Area() int { return 1 }

type box struct{}

func (box) Area() int { return 2 }

var calls int

func subject(err error) error { calls++; return err }

func classify(err error) string {
	e := subject(err)
	if a, ok := asAPI(e); ok && a.kind == 1 && a.status >= 500 {
		return "server"
	}
	if d, ok := asDB(e); ok && d.kind == 1 {
		return "db:" + d.name
	}
	if a, ok := asAPI(e); ok && a.kind == 1 {
		return "rejected:" + a.message
	}
	if a, ok := asAPI(e); ok && a.kind == 0 {
		if d, ok := asDB(a.inner); ok && d.kind == 1 {
			return "internal-db:" + d.name
		}
	}
	if a, ok := asAPI(e); ok && a.kind == 0 {
		return "internal"
	}
	if a, ok := asAPI(e); ok && a.kind == 3 && a.count > 1 {
		return "retry"
	}
	if a, ok := asAPI(e); ok && a.kind == 2 {
		return "missing"
	}
	if err == nil {
		return "nil"
	}
	return "other"
}

func status(err error) (code int) {
	if a, ok := asAPI(err); ok && a.kind == 1 {
		code = a.status
	} else if a, ok := asAPI(err); ok && a.kind == 2 {
		code = 404
	} else if d, ok := asDB(err); ok && d.kind == 1 {
		code = 404
	} else {
		code = 500
	}
	return code
}

func kind(s shape) string {
	if q, ok := s.(square); ok && q.kind == 1 && q.n > 3 {
		return "big"
	}
	if q, ok := s.(square); ok && q.kind == 1 {
		return "side"
	}
	if q, ok := s.(square); ok && q.kind == 0 {
		return "unit"
	}
	if s == nil {
		return "nil"
	}
	return "other"
}

func check(ok bool) {
	if !ok {
		panic("interface match mismatch")
	}
}

func main() {
	check(classify(apiError{kind: 1, status: 503, message: "busy"}) == "server")
	check(classify(apiError{kind: 1, status: 400, message: "no"}) == "rejected:no")
	check(classify(dbError{kind: 1, name: "t"}) == "db:t")
	check(classify(wrap{wrap{apiError{kind: 2}}}) == "missing")
	check(classify(many{nil, plain("x"), wrap{dbError{kind: 1, name: "deep"}}}) == "db:deep")
	check(classify(apiError{kind: 0, inner: dbError{kind: 1, name: "inner"}}) == "internal-db:inner")
	check(classify(apiError{kind: 0, inner: plain("boom")}) == "internal")
	check(classify(apiError{kind: 3, count: 3}) == "retry")
	check(classify(apiError{kind: 3, count: 1}) == "other")
	check(classify(shim{"s"}) == "db:s")
	check(classify(wrap{}) == "other")
	check(classify(nil) == "nil")
	check(classify(plain("x")) == "other")
	check(calls == 13)
	check(status(wrap{apiError{kind: 1, status: 418}}) == 418)
	check(status(many{apiError{kind: 3, count: 1}, apiError{kind: 2}}) == 500)
	check(status(many{apiError{kind: 2}, apiError{kind: 3, count: 1}}) == 404)
	check(status(nil) == 500)
	check(boxKind[int](boxed[int]{full: true, value: 1}) == "full")
	check(boxKind[int](wrap{boxed[int]{}}) == "empty")
	check(boxKind[string](boxed[int]{full: true, value: 1}) == "none")
	check(boxKind[int](nil) == "none")
	check(kind(square{kind: 1, n: 5}) == "big")
	check(kind(square{kind: 1, n: 1}) == "side")
	check(kind(square{kind: 0}) == "unit")
	check(kind(box{}) == "other")
	check(kind(nil) == "nil")
	println("PASS")
}
