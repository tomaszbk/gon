package main

import (
	"fmt"
	"gon/seq"
	"runtime"
	"strings"
)

type Value enum {
	default Empty
	First(*int)
	Second(*int)
}

func lambda() {
	var callback func() int = () => {
		var pointer *int
		return *pointer // want "dereferenced"
	}
	_ = callback()
}
func consume(unused *int, value *int) int {
	return *value // want "dereferenced"
}
func named() { _ = consume(value: nil, unused: new(int)) }
func optional() {
	var value (*int)? = (*int)(nil)
	if value is pointer? {
		_ = *pointer // want "dereferenced"
	}
}
func matching() {
	value := Value.Second((*int)(nil))
	_ = switch value {
	case Value.First(pointer), Value.Second(pointer) => *pointer // want "dereferenced"
	default => 0
	}
}
func conditional() {
	pointer := if true { (*int)(nil) } else { new(int) }
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
	var value ((*int)?)? = ((*int)?)((*int)(nil))
	if value is inner? && inner is pointer? {
		_ = *pointer // want "dereferenced"
	}
}
func lookup() {
	values := map[string]*int{"nil": nil}
	if seq.Lookup(values, "nil") is pointer? {
		_ = *pointer // want "dereferenced"
	}
}
func find() {
	if seq.Find([]*int{nil}, (_) => true) is pointer? {
		_ = *pointer // want "dereferenced"
	}
}

func unwrap() int? {
	var value (*int)? = (*int)(nil)
	pointer := value?
	return *pointer // want "dereferenced"
}
func propagation() { _ = unwrap() }

type Node struct{ Pointer *int }

func navigation() {
	node := &Node{}
	pointer := node?.Pointer
	_ = *pointer // want "dereferenced"
}
func safeCall() {
	var callback func() *int = () => nil
	pointer := callback?()
	_ = *pointer // want "dereferenced"
}
func defaultPointer(calls *int) *int { *calls++; return new(int) }
func coalesce() {
	var present (*int)? = (*int)(nil)
	calls := 0
	pointer := present ?? defaultPointer(&calls)
	if calls != 0 {
		panic("coalesce treated present nil as absence")
	}
	_ = *pointer // want "dereferenced"
}
func coalesceAssign() {
	var present (*int)? = (*int)(nil)
	calls := 0
	present ??= defaultPointer(&calls)
	if calls != 0 {
		panic("coalesce assignment treated present nil as absence")
	}
	pointer := present ?? nil
	_ = *pointer // want "dereferenced"
}
func nilTuple() (*int, error) { return nil, nil }
func bangValue() error {
	pointer := nilTuple()!
	_ = *pointer // want "dereferenced"
	return nil
}
func bang() { _ = bangValue() }
func blockValue() error {
	pointer := nilTuple() or err {
		return err
	}
	_ = *pointer // want "dereferenced"
	return nil
}
func blockHandler() { _ = blockValue() }
func contextValue() error {
	pointer := nilTuple() or err => err
	_ = *pointer // want "dereferenced"
	return nil
}
func contextHandler()  { _ = contextValue() }
func nilPointer() *int { return nil }
func interpolation() {
	_ = $"${*nilPointer()}" // want "dereferenced"
}
