package main

import (
	"fmt"
	"gon/seq"
)

type Value enum {
	default Empty
	Pointer(*int)
	Record { Ptr *int }
}

func read(value Value) int {
	return switch value {
	case Value.Pointer(p), Value.Record{Ptr: p} if p != nil => *p
	default => 0
	}
}
func named(unused *int, value *int) int {
	if value != nil {
		return *value
	}
	return 0
}
func raw() (*int, error) { return new(int), nil }
func tuple() error {
	p := raw()!
	if p != nil {
		_ = *p
	}
	return nil
}
func context() error {
	p := raw() or err => err
	if p != nil {
		_ = *p
	}
	return nil
}
func main() {
	var callback func(*int) int = (p) => {
		if p != nil {
			return *p
		}
		return 0
	}
	pointer := new(int)
	*pointer = 7
	var absent (*int)?
	var present (*int)? = (*int)(nil)
	if present is p? && p != nil {
		println(*p)
	}
	missing := absent ?? nil
	println(callback(pointer), callback(nil), named(value: pointer, unused: nil), read(Value.Record{Ptr: pointer}), read(Value.Pointer(nil)), missing == nil)
	choice := if false { pointer } else { (*int)(nil) }
	if choice != nil {
		println(*choice)
	}
	fmt.Println($"value ${callback(pointer)}")
	if tuple() != nil || context() != nil {
		panic("unexpected error")
	}
	if navigation() != 0 || !nested() || block() != nil || collections() != 5 {
		panic("extended constructs")
	}
	if successNil() != nil || nilPayloads() != 0 || interpolatedNil() != "nil" {
		panic("guarded nil payloads")
	}
	if (propagation(absent) ?? -1) != -1 || (propagation(present) ?? -1) != 0 || (propagation(pointer) ?? -1) != 7 {
		panic("optional propagation")
	}
	println("PASS")
}

type Node struct{ Pointer *int }

func navigation() int {
	var node *Node
	var callback func(*int) int
	value := node?.Pointer ?? new(int)
	value ??= new(int)
	return *value + (callback?(value) ?? 0)
}
func nested() bool {
	var value ((int?)?) = (int?)(nil)
	if value is inner? && inner is number? {
		return number > 0
	}
	return value is nil?
}
func block() error {
	value := raw() or problem {
		return problem
	}
	if value != nil {
		_ = *value
	}
	return nil
}
func collections() int {
	pointer := new(int)
	*pointer = 5
	values := map[string]*int{"number": pointer}
	if seq.Lookup(values, "number") is value? && value != nil {
		found := seq.Find([]*int{nil, value}, (candidate) => candidate != nil) ?? new(int)
		if found != nil {
			return *found
		}
	}
	return 0
}

func propagation(value (*int)?) int? {
	pointer := value?
	if pointer != nil {
		return *pointer
	}
	return 0
}

func nilTuple() (*int, error) { return nil, nil }
func successNil() error {
	value := nilTuple()!
	if value != nil {
		_ = *value
	}
	blocked := nilTuple() or err {
		return err
	}
	if blocked != nil {
		_ = *blocked
	}
	contextual := nilTuple() or err => err
	if contextual != nil {
		_ = *contextual
	}
	return nil
}
func defaultPointer(calls *int) *int { *calls++; return new(int) }
func nilPayloads() int {
	node := &Node{}
	var callback func() *int = () => nil
	var present (*int)? = (*int)(nil)
	calls := 0
	values := []*int{node?.Pointer, callback?(), present ?? defaultPointer(&calls)}
	present ??= defaultPointer(&calls)
	if calls != 0 {
		panic("present nil default evaluated")
	}
	values = append(values, present ?? nil)
	result := 0
	for _, pointer := range values {
		if pointer != nil {
			result += *pointer
		}
	}
	return result
}
func nilPointer() *int { return nil }
func interpolatedNil() string {
	pointer := nilPointer()
	if pointer == nil {
		return $"${"nil"}"
	}
	return $"${*pointer}"
}
