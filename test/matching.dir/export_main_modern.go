package main

import (
	"fmt"
	"matchingexport/lib"
)

func main() {
	x := lib.Count(7)
	fmt.Println(switch x {
	case lib.State.Empty => 0
	case lib.State.Count(n) => n
	}, lib.Get(lib.Wrap("value"), "zero"), switch lib.Empty[string]() {
	case lib.Maybe[string].None => "zero"
	case lib.Maybe[string].Some(value) => value
	})
}
