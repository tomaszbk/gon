package main

type Shape struct {
	kind, radius int
	reason       string
}

const (
	Empty = iota
	Circle
	Sphere
	Record
	Invalid
)

func selected(value Shape, guards *int) int {
	switch value.kind {
	case Circle, Sphere, Record:
		if reject(guards) {
			return value.radius
		}
		return 0
	case Empty, Invalid:
		return -1
	}
	panic("invalid shape")
}

type MyBool bool

func reject(counter *int) MyBool { *counter++; return false }
func captured(value Shape) func() int {
	switch value.kind {
	case Circle, Sphere, Record:
		radius := value.radius
		return func() int { return radius }
	case Empty, Invalid:
		return func() int { return 0 }
	}
	panic("invalid shape")
}
func statement(value Shape) int {
	result := 0
L:
	switch value.kind {
	case Circle, Sphere, Record:
		result = value.radius
		break L
	case Empty, Invalid:
		result = -1
	}
	return result
}
func nested(kind int) int {
	present, innerPresent, number := false, false, 0
	if kind == 1 {
		present = true
	}
	if kind == 2 {
		present, innerPresent, number = true, true, 7
	}
	if !present || !innerPresent {
		return 0
	}
	return number
}

type Fault struct{ kind, number int }

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
func forward(err error, guards *int) int {
	matched, number := false, 0
	if value, ok := asFault(err); ok && value.kind == 0 {
		matched, number = true, value.number
	}
	if !matched {
		if value, ok := asFault(err); ok && value.kind == 1 {
			matched, number = true, value.number
		}
	}
	if matched && bool(reject(guards)) {
		return number
	}
	return -1
}
func reversed(err error, guards *int) int {
	matched, number := false, 0
	if value, ok := asFault(err); ok && value.kind == 1 {
		matched, number = true, value.number
	}
	if !matched {
		if value, ok := asFault(err); ok && value.kind == 0 {
			matched, number = true, value.number
		}
	}
	if matched && bool(reject(guards)) {
		return number
	}
	return -1
}
func observed(counter *int, value Shape) Shape { *counter++; return value }
func main() {
	guards, observations := 0, 0
	values := []Shape{{kind: Empty}, {kind: Circle, radius: 3}, {kind: Sphere, radius: 5}, {kind: Record, radius: 8}, {kind: Invalid, reason: "bad"}}
	for _, value := range values {
		subject, radius := observed(&observations, value), 0
		switch subject.kind {
		case Circle, Sphere, Record:
			radius = subject.radius
		}
		println(radius, captured(value)(), statement(value), selected(value, &guards))
	}
	if observations != 5 || guards != 3 {
		panic("subject or guard evaluation count")
	}
	for kind := 0; kind < 3; kind++ {
		println(nested(kind))
	}
	for _, reversedOrder := range []bool{false, true} {
		hits, count := 0, 0
		value := wrapper{Fault{0, 11}, &hits}
		result, expected := -1, 1
		if reversedOrder {
			result, expected = reversed(value, &count), 2
		} else {
			result = forward(value, &count)
		}
		if result != -1 || count != 1 || hits != expected {
			panic("alternative order or guard evaluation")
		}
		println(result, hits, count)
	}
	value := 0
	switch true {
	case false, true:
		value = 9
	}
	println(value)
	println("PASS")
}
