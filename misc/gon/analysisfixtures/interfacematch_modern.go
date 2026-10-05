package main

type apiError enum {
	default Internal(error)
	Rejected  {
		Status  int
		Message string
	}
	Missing
	Retry(int, string)
}

func (e apiError) Error() string { return "api" }

type dbError enum {
	default Unknown
	NotFound(string)
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
		*t = dbError.NotFound(s.name)
		return true
	}
	return false
}

type boxed[T any] enum {
	default Empty
	Full(T)
}

func (boxed[T]) Error() string { return "box" }

func boxKind[T any](err error) string {
	return switch err {
	case boxed[T].Full(_) => "full"
	case boxed[T].Empty => "empty"
	default => "none"
	}
}

type shape interface{ Area() int }
type square enum {
	default Unit
	Side(int)
}

func (s square) Area() int { return 1 }

type box struct{}

func (box) Area() int { return 2 }

var calls int

func subject(err error) error { calls++; return err }

func classify(err error) string {
	return switch subject(err) {
	case apiError.Rejected{Status: status, ...} if status >= 500 => "server"
	case dbError.NotFound(name) => "db:" + name
	case apiError.Rejected{Message: message, ...} => "rejected:" + message
	case apiError.Internal(dbError.NotFound(name)) => "internal-db:" + name
	case apiError.Internal(_) => "internal"
	case apiError.Retry(n, _) if n > 1 => "retry"
	case apiError.Missing => "missing"
	case _ if err == nil => "nil"
	default => "other"
	}
}

func status(err error) (code int) {
	switch err {
	case apiError.Rejected{Status: s, ...} => {
		code = s
	}
	case apiError.Missing => {
		code = 404
	}
	case dbError.NotFound(_) => {
		code = 404
	}
	default => {
		code = 500
	}
	}
	return code
}

func kind(s shape) string {
	return switch s {
	case square.Side(n) if n > 3 => "big"
	case square.Side(_) => "side"
	case square.Unit => "unit"
	case _ if s == nil => "nil"
	default => "other"
	}
}

func check(ok bool) {
	if !ok {
		panic("interface match mismatch")
	}
}

func main() {
	check(classify(apiError.Rejected{Status: 503, Message: "busy"}) == "server")
	check(classify(apiError.Rejected{Status: 400, Message: "no"}) == "rejected:no")
	check(classify(dbError.NotFound("t")) == "db:t")
	check(classify(wrap{wrap{apiError.Missing}}) == "missing")
	check(classify(many{nil, plain("x"), wrap{dbError.NotFound("deep")}}) == "db:deep")
	check(classify(apiError.Internal(dbError.NotFound("inner"))) == "internal-db:inner")
	check(classify(apiError.Internal(plain("boom"))) == "internal")
	check(classify(apiError.Retry(3, "again")) == "retry")
	check(classify(apiError.Retry(1, "once")) == "other")
	check(classify(shim{"s"}) == "db:s")
	check(classify(wrap{}) == "other")
	check(classify(nil) == "nil")
	check(classify(plain("x")) == "other")
	check(calls == 13)
	check(status(wrap{apiError.Rejected{Status: 418}}) == 418)
	check(status(many{apiError.Retry(1, "a"), apiError.Missing}) == 500)
	check(status(many{apiError.Missing, apiError.Retry(1, "a")}) == 404)
	check(status(nil) == 500)
	check(boxKind[int](boxed[int].Full(1)) == "full")
	check(boxKind[int](wrap{boxed[int].Empty}) == "empty")
	check(boxKind[string](boxed[int].Full(1)) == "none")
	check(boxKind[int](nil) == "none")
	check(kind(square.Side(5)) == "big")
	check(kind(square.Side(1)) == "side")
	check(kind(square.Unit) == "unit")
	check(kind(box{}) == "other")
	check(kind(nil) == "nil")
	println("PASS")
}
