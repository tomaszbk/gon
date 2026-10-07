package main

import (
	"fmt"
	"runtime"
	"strings"
)

func lambdaBody() int {
	var pointer *int
	return *pointer // want "dereferenced"
}
func lambda() { callback := func() int { return lambdaBody() }; _ = callback() }
func consume(unused *int, value *int) int {
	return *value // want "dereferenced"
}
func named() { _ = consume(new(int), nil) }
func optional() {
	var pointer *int
	present := true
	if present {
		_ = *pointer // want "dereferenced"
	}
}
func matching() {
	var pointer *int
	kind := 2
	switch kind {
	case 1, 2:
		_ = *pointer // want "dereferenced"
	}
}
func conditional() {
	var pointer *int
	if true {
		pointer = nil
	} else {
		pointer = new(int)
	}
	_ = *pointer // want "dereferenced"
}
func expectPanic(run func()) {
	recovered := false
	func() {
		defer func() {
			failure := recover()
			_, runtimeFailure := failure.(runtime.Error)
			recovered = runtimeFailure && strings.Contains(fmt.Sprint(failure), "nil pointer dereference")
		}()
		if run != nil {
			run()
		}
	}()
	if !recovered {
		panic("nil dereference did not panic")
	}
}
func main() {
	for _, run := range []func(){lambda, named, optional, matching, conditional, nested, lookup, find, propagation, navigation, safeCall, coalesce, coalesceAssign, bang, blockHandler, contextHandler, interpolation} {
		expectPanic(run)
	}
	println("PASS unsafe")
}
func nested() {
	var pointer *int
	outerPresent, innerPresent := true, true
	if outerPresent && innerPresent {
		_ = *pointer // want "dereferenced"
	}
}
func lookup() {
	values := make(map[string]*int)
	var missing *int
	values["nil"] = missing
	if pointer, exists := values["nil"]; exists {
		_ = *pointer // want "dereferenced"
	}
}
func find() {
	var missing *int
	values := make([]*int, 1)
	values[0] = missing
	for _, pointer := range values {
		_ = *pointer // want "dereferenced"
		break
	}
}

func unwrap() (int, bool) {
	var pointer *int
	present := true
	if !present {
		return 0, false
	}
	return *pointer, true // want "dereferenced"
}
func propagation() { _, _ = unwrap() }

type Node struct{ Pointer *int }

func navigation() {
	node := &Node{}
	var pointer *int
	if node != nil {
		pointer = node.Pointer
	}
	_ = *pointer // want "dereferenced"
}
func safeCall() {
	callback := func() *int { return nil }
	var pointer *int
	if callback != nil {
		pointer = callback()
	}
	_ = *pointer // want "dereferenced"
}
func defaultPointer(calls *int) *int { *calls++; return new(int) }
func coalesce() {
	present := true
	var payload *int
	calls := 0
	pointer := payload
	if !present {
		pointer = defaultPointer(&calls)
	}
	if calls != 0 {
		panic("coalesce treated present nil as absence")
	}
	_ = *pointer // want "dereferenced"
}
func coalesceAssign() {
	present := true
	var payload *int
	calls := 0
	if !present {
		present, payload = true, defaultPointer(&calls)
	}
	if calls != 0 {
		panic("coalesce assignment treated present nil as absence")
	}
	pointer := payload
	_ = *pointer // want "dereferenced"
}
func nilTuple() (*int, error) { return nil, nil }
func bangValue() error {
	pointer, err := nilTuple()
	if err != nil {
		return err
	}
	_ = *pointer // want "dereferenced"
	return nil
}
func bang() { _ = bangValue() }
func blockValue() error {
	pointer, err := nilTuple()
	if err != nil {
		return err
	}
	_ = *pointer // want "dereferenced"
	return nil
}
func blockHandler() { _ = blockValue() }
func contextValue() error {
	pointer, err := nilTuple()
	if err != nil {
		return err
	}
	_ = *pointer // want "dereferenced"
	return nil
}
func contextHandler()  { _ = contextValue() }
func nilPointer() *int { return nil }
func interpolation() {
	_ = fmt.Sprintf("%v", *nilPointer()) // want "dereferenced"
}
