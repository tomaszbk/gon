package main

import (
	"errors"
	"fmt"
)

var trace string
var boom = errors.New("boom")

func mark(label string, n int) int { trace += label; return n }
func call(left, right int) int     { return left*10 + right }
func source(fail bool) (int, error) {
	trace += "S"
	if fail {
		return 99, boom
	}
	return 7, nil
}
func main() {
	for _, fail := range []bool{false, true} {
		trace = ""
		out, err := propagation(fail)
		fmt.Println(out, errors.Is(err, boom), trace)
	}
	trace = ""
	fmt.Println(scenario(), trace)
	fmt.Println(shadowed(), multiline())
	fmt.Println(empty(), captured())
	for _, fail := range []bool{false, true} {
		trace = ""
		out, err := blocks(fail)
		fmt.Println(out, errors.Is(err, boom), trace)
	}
	fmt.Println(rawBlocks())
	fmt.Println(ordered())
	trace = ""
	fmt.Println(safeCalls(), trace)
}
