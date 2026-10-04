package main

/*
static int twice(int value) { return 2 * value; }
*/
import "C"
import "fmt"

type Input enum {
	default Missing
	Value(int)
}

// The boundary adapter inspects the active alternative before passing a C scalar.
func twice(input Input) int {
	return switch input {
	case Input.Missing => 0
	case Input.Value(value) => int(C.twice(C.int(value)))
	}
}
func main() {
	if twice(Input.Missing) != 0 || twice(Input.Value(21)) != 42 {
		panic("cgo adapter")
	}
	fmt.Println("enum cgo adapter: 0 42")
}
