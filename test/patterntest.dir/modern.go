package main

import is "cmp"

type isConstraint interface{ ~int }
type Generic[P isConstraint] struct{ value P }
type Ordered[P is.Ordered] struct{ value P }
type MyBool bool

func contextualBool(o int?) MyBool { return o is 4? }
func boolContexts(o int?) {
	var mb MyBool = o is 4?
	if mb != contextualBool(o) {
		panic("named boolean")
	}
	println(mb, !(o is 4?), contextualBool(o))
}

func namedConstraint() {
	type is interface{ ~int }
	type T[P is] struct{ value P }
	println(T[int]{5}.value)
}

type Status enum {
	default Unknown
	Score { Points int }
	Rejected(string)
}

func valid(status Status) bool { return !(status is Status.Unknown) }
func score(status Status) func() int {
	if status is Status.Score{Points: points} && points > 0 {
		return () => points
	}
	return () => 0
}
func observed(counter *int, value int?) int? { *counter++; return value }
func tick(counter *int) bool                 { *counter++; return true }
func optional(value int?, evaluations, conditions *int) int {
	number := 42
	if observed(evaluations, value) is number? && number > 0 && tick(conditions) {
		return number
	} else {
		return number
	}
}
func chain(first, second int?) int {
	if first is one? && second is two? && two > one {
		return one + two
	}
	return 0
}
func optionalRecord(value Status?) int {
	if value is Status.Score{Points: points}? && points > 0 {
		return points
	}
	return 0
}
func repeated(value Status) bool {
	if value is Status.Score{...} is true {
		return true
	}
	return false
}
func presence[T any](value T?) bool { return value is nil }

type Fault enum {
	default First(int)
	Second(int)
}

func (Fault) Error() string { return "fault" }

type wrapper struct {
	inner error
	hits  *int
}

func (wrapper) Error() string   { return "wrapper" }
func (w wrapper) Unwrap() error { *w.hits++; return w.inner }
func errorCode(err error) int {
	if err is Fault.Second(code) {
		return code
	} else if err is Fault.First(code) {
		return code
	}
	return 0
}
func main() {
	namedConstraint()
	println(Generic[int]{3}.value, Ordered[int]{4}.value)
	for _, o := range []int?{nil, 4, 5} {
		boolContexts(o)
	}
	for _, value := range []Status{Status.Unknown, Status.Score{Points: 7}, Status.Rejected("bad")} {
		println(valid(value), score(value)())
	}
	evaluations, conditions := 0, 0
	for _, value := range []int?{nil, 0, 3} {
		println(optional(value, &evaluations, &conditions))
	}
	if evaluations != 3 || conditions != 1 {
		panic("subject or short-circuit count")
	}
	println(evaluations, conditions, chain(2, 5), chain(2, nil))
	var absent (*int)?
	var presentNil (*int)? = (*int)(nil)
	println(presence(absent), presence(presentNil), presentNil is nil?)
	hits := 0
	println(errorCode(wrapper{Fault.First(9), &hits}), hits, errorCode(nil))
	is := 3
	if is == 3 {
		println("contextual")
	}
	for _, value := range []Status?{nil, Status.Unknown, Status.Score{Points: 7}, Status.Score{Points: 0}} {
		println(optionalRecord(value))
	}
	println(repeated(Status.Score{Points: 7}), repeated(Status.Unknown))
	println("PASS")
}
