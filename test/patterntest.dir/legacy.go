package main

type Status struct {
	kind, points int
	reason       string
}

func valid(status Status) bool { return status.kind != 0 }
func score(status Status) func() int {
	if status.kind == 1 {
		points := status.points
		if points > 0 {
			return func() int { return points }
		}
	}
	return func() int { return 0 }
}

type Optional struct {
	present bool
	number  int
}

func observed(counter *int, value Optional) Optional { *counter++; return value }
func tick(counter *int) bool                         { *counter++; return true }
func optional(value Optional, evaluations, conditions *int) int {
	number := 42
	found := observed(evaluations, value)
	if found.present {
		number := found.number
		if number > 0 && tick(conditions) {
			return number
		}
	}
	return number
}
func chain(first, second Optional) int {
	if first.present {
		one := first.number
		if second.present {
			two := second.number
			if two > one {
				return one + two
			}
		}
	}
	return 0
}

type Fault struct{ kind, code int }

func (Fault) Error() string { return "fault" }

type wrapper struct {
	inner error
	hits  *int
}

func (wrapper) Error() string   { return "wrapper" }
func (w wrapper) Unwrap() error { *w.hits++; return w.inner }
func asFault(err error) (Fault, bool) {
	for err != nil {
		if value, ok := err.(Fault); ok {
			return value, true
		}
		if value, ok := err.(interface{ Unwrap() error }); ok {
			err = value.Unwrap()
		} else {
			break
		}
	}
	return Fault{}, false
}
func errorCode(err error) int {
	if value, ok := asFault(err); ok && value.kind == 1 {
		return value.code
	} else if value, ok := asFault(err); ok && value.kind == 0 {
		return value.code
	}
	return 0
}
func optionalRecord(present bool, value Status) int {
	if present && value.kind == 1 && value.points > 0 {
		return value.points
	}
	return 0
}
func repeated(value Status) bool {
	if (value.kind == 1) == true {
		return true
	}
	return false
}
func main() {
	for _, value := range []Status{{}, {kind: 1, points: 7}, {kind: 2, reason: "bad"}} {
		println(valid(value), score(value)())
	}
	evaluations, conditions := 0, 0
	for _, value := range []Optional{{}, {present: true}, {present: true, number: 3}} {
		println(optional(value, &evaluations, &conditions))
	}
	if evaluations != 3 || conditions != 1 {
		panic("subject or short-circuit count")
	}
	println(evaluations, conditions, chain(Optional{true, 2}, Optional{true, 5}), chain(Optional{true, 2}, Optional{}))
	absent, presentNil := false, true
	var payload *int
	println(!absent, !presentNil, presentNil && payload == nil)
	hits := 0
	println(errorCode(wrapper{Fault{0, 9}, &hits}), hits, errorCode(nil))
	is := 3
	if is == 3 {
		println("contextual")
	}
	for _, value := range []struct {
		present bool
		status  Status
	}{{}, {true, Status{}}, {true, Status{kind: 1, points: 7}}, {true, Status{kind: 1}}} {
		println(optionalRecord(value.present, value.status))
	}
	println(repeated(Status{kind: 1, points: 7}), repeated(Status{}))
	println("PASS")
}
