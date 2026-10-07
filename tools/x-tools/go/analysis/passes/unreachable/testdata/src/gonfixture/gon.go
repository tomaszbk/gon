package gon

var a func() = func() {
	return
	println(1) // want "unreachable code"
}
var b func() = () => {
	return
	println(1) // want "unreachable code"
}

func exhaustive(v bool) {
	switch v {
	case true => {
		return
	}
	case false => {
		panic(0)
	}
	}
	println(1) // want "unreachable code"
}
func guarded(v bool) {
	switch v {
	case true if v => {
		return
	}
	default => {
		return
	}
	}
	println(1) // want "unreachable code"
}
func withBreak(v bool) {
L:
	switch v {
	case true => {
		break L
	}
	case false => {
		return
	}
	}
	println(1)
}

func read() (int, error) { return 1, nil }
func save() error        { return nil }

func handler() int {
	n := read() or err {
		return 0
		println(err) // want "unreachable code"
	}
	println(n) // the successful path remains reachable
	return n
}

func errorOnly() {
	save() or err {
		panic(err)
		println("dead") // want "unreachable code"
	}
	println("success")
}

func inReturn() int {
	return read() or err {
		return 0
		println(err) // want "unreachable code"
	}
}

func inCondition() {
	if (read() or err {
		return
		println(err) /* want "unreachable code" */
	}) > 0 {
		println("success")
	}
}
