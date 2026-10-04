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
