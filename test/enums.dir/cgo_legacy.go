package main

/*
static int twice(int value) { return 2 * value; }
*/
import "C"
import "fmt"

type Input struct {
	present bool
	value   int
}

func twice(input Input) int {
	if !input.present {
		return 0
	}
	return int(C.twice(C.int(input.value)))
}
func main() {
	if twice(Input{}) != 0 || twice(Input{true, 21}) != 42 {
		panic("cgo adapter")
	}
	fmt.Println("enum cgo adapter: 0 42")
}
