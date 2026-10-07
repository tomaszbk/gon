package main

type Shape enum {
	default Empty
	Circle(int)
	Sphere(int)
	Record { Radius int }
	Invalid(string)
}

func selected(value Shape, guards *int) int {
	return switch value {
	case Shape.Circle(radius), Shape.Sphere(radius), Shape.Record{Radius: radius} if reject(guards) => radius
	case Shape.Empty, Shape.Invalid(_) => -1
	default => 0
	}
}

func reject(counter *int) bool { *counter++; return false }

func captured(value Shape) func() int {
	return switch value {
	case Shape.Circle(radius), Shape.Sphere(radius), Shape.Record{Radius: radius} => () => radius
	case Shape.Empty, Shape.Invalid(_) => () => 0
	}
}

func statement(value Shape) int {
	result := 0
L:
	switch value {
	case Shape.Circle(radius), Shape.Sphere(radius), Shape.Record{Radius: radius} => {
		result = radius
		break L
	}
	case Shape.Empty, Shape.Invalid(_) => {
		result = -1
	}
	}
	return result
}

func nested(kind int) int {
	var value (int?)?
	if kind == 1 {
		value = (int?)(nil)
	}
	if kind == 2 {
		value = (int?)(7)
	}
	return switch value {
	case nil, nil? => 0
	case (number?)? => number
	}
}

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

func forward(err error, guards *int) int {
	return switch err {
	case Fault.First(number), Fault.Second(number) if reject(guards) => number
	default => -1
	}
}
func reversed(err error, guards *int) int {
	return switch err {
	case Fault.Second(number), Fault.First(number) if reject(guards) => number
	default => -1
	}
}
func observed(counter *int, value Shape) Shape { *counter++; return value }

func main() {
	guards, observations := 0, 0
	values := []Shape{Shape.Empty, Shape.Circle(3), Shape.Sphere(5), Shape.Record{Radius: 8}, Shape.Invalid("bad")}
	for _, value := range values {
		radius := switch observed(&observations, value) {
		case Shape.Empty, Shape.Invalid(_) => 0
		case Shape.Circle(number), Shape.Sphere(number), Shape.Record{Radius: number} => number
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
		value := wrapper{Fault.First(11), &hits}
		result := if reversedOrder { reversed(value, &count) } else { forward(value, &count) }
		expected := if reversedOrder { 2 } else { 1 }
		if result != -1 || count != 1 || hits != expected {
			panic("alternative order or guard evaluation")
		}
		println(result, hits, count)
	}
	println(switch true {
	case false, true => 9
	})
	println("PASS")
}
