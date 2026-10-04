package main

import (
	"errors"
	"fmt"
	"strings"
)

var trace []string

func mark(s string)               { trace = append(trace, s) }
func side(s string, n int) int    { mark(s); return n }
func guard(s string, b bool) bool { mark(s); return b }
func source(n int, fail bool) (int, error) {
	mark(fmt.Sprintf("source:%d", n))
	if fail {
		return n, errors.New("failure")
	}
	return n * 10, nil
}
func sequence(yield func(int) bool) {
	for _, n := range []int{1, 2, 3} {
		if !yield(n) {
			return
		}
	}
}
func expect(s string, got, want any) {
	if got != want {
		panic(fmt.Sprintf("%s got %v want %v", s, got, want))
	}
	fmt.Println(s, got)
}
func main() {
	var zero State
	expect("zero", label(zero), "idle")
	expect("count", label(makeCount(3)), "count:3")
	expect("record", label(makeRecord("name", 4)), "name:4")
	expect("nested", nestedLabel(makeNested(makeCount(5))), "count:5")
	expect("single/effects", effects(), 9)
	expect("closure", escapeBinding()(), 7)
	expect("copy", copyBinding(), 8)
	expect("break", breakMatch(zero), 3)
	expect("nil", nilBranch(false), true)
	expect("typed-nil", nilBranch(true), false)
	expect("interface-kind", interfaceKind(), "float64")
	expect("legacy-switch", classicSwitch(), 5)
	expect("contextual-patterns-present-nil", patternNames(true, nil), 206)
	value := 1
	expect("contextual-patterns-false-present", patternNames(false, &value), 196)
	expect("option-present", optionBoundary(true), 4)
	expect("option-absent", optionBoundary(false), 0)
	expect("result-ok", resultBoundary(true), "8")
	expect("result-error", resultBoundary(false), "failure")
	n, err := iterate(false)
	expect("iterate-success", n, 24)
	expect("iterate-success-error", err == nil, true)
	n, err = iterate(true)
	expect("iterate-failure", n, 0)
	expect("iterate-failure-error", err.Error(), "failure")
	fmt.Println("trace", strings.Join(trace, "/"))
}
