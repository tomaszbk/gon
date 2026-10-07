package main

import "fmt"

type Value struct {
	kind int
	p    *int
}

func read(value Value) int {
	switch value.kind {
	case 1, 2:
		if value.p != nil {
			return *value.p
		}
	}
	return 0
}
func named(unused *int, value *int) int {
	if value != nil {
		return *value
	}
	return 0
}
func raw() (*int, error) { return new(int), nil }
func tuple() error {
	p, err := raw()
	if err != nil {
		return err
	}
	if p != nil {
		_ = *p
	}
	return nil
}
func context() error {
	p, err := raw()
	if err != nil {
		return err
	}
	if p != nil {
		_ = *p
	}
	return nil
}
func main() {
	callback := func(p *int) int {
		if p != nil {
			return *p
		}
		return 0
	}
	pointer := new(int)
	*pointer = 7
	present, payload := true, (*int)(nil)
	if present && payload != nil {
		println(*payload)
	}
	var missing *int
	println(callback(pointer), callback(nil), named(nil, pointer), read(Value{kind: 2, p: pointer}), read(Value{kind: 1}), missing == nil)
	var choice *int
	if false {
		choice = pointer
	} else {
		choice = (*int)(nil)
	}
	if choice != nil {
		println(*choice)
	}
	fmt.Println(fmt.Sprintf("value %v", callback(pointer)))
	if tuple() != nil || context() != nil {
		panic("unexpected error")
	}
	if navigation() != 0 || !nested() || block() != nil || collections() != 5 {
		panic("extended constructs")
	}
	if successNil() != nil || nilPayloads() != 0 || interpolatedNil() != "nil" {
		panic("guarded nil payloads")
	}
	if result, exists := propagation(false, nil); result != 0 || exists {
		panic("absence propagation")
	}
	if result, exists := propagation(true, payload); result != 0 || !exists {
		panic("present nil propagation")
	}
	if result, exists := propagation(true, pointer); result != 7 || !exists {
		panic("payload propagation")
	}
	println("PASS")
}

type Node struct{ Pointer *int }

func navigation() int {
	var node *Node
	var callback func(*int) int
	var value *int
	if node != nil {
		value = node.Pointer
	}
	if value == nil {
		value = new(int)
	}
	if value == nil {
		value = new(int)
	}
	result := 0
	if callback != nil {
		result = callback(value)
	}
	return *value + result
}
func nested() bool {
	outerPresent, innerPresent, number := true, false, 0
	if outerPresent && innerPresent {
		return number > 0
	}
	return outerPresent && !innerPresent
}
func block() error {
	value, problem := raw()
	if problem != nil {
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
	if value, exists := values["number"]; exists && value != nil {
		var found *int
		for _, candidate := range []*int{nil, value} {
			if candidate != nil {
				found = candidate
				break
			}
		}
		if found == nil {
			found = new(int)
		}
		if found != nil {
			return *found
		}
	}
	return 0
}

func propagation(present bool, pointer *int) (int, bool) {
	if !present {
		return 0, false
	}
	if pointer != nil {
		return *pointer, true
	}
	return 0, true
}

func nilTuple() (*int, error) { return nil, nil }
func successNil() error {
	value, err := nilTuple()
	if err != nil {
		return err
	}
	if value != nil {
		_ = *value
	}
	blocked, err := nilTuple()
	if err != nil {
		return err
	}
	if blocked != nil {
		_ = *blocked
	}
	contextual, err := nilTuple()
	if err != nil {
		return err
	}
	if contextual != nil {
		_ = *contextual
	}
	return nil
}
func defaultPointer(calls *int) *int { *calls++; return new(int) }
func nilPayloads() int {
	node := &Node{}
	callback := func() *int { return nil }
	present := true
	var payload, navigation, called, coalesced *int
	calls := 0
	if node != nil {
		navigation = node.Pointer
	}
	if callback != nil {
		called = callback()
	}
	coalesced = payload
	if !present {
		coalesced = defaultPointer(&calls)
	}
	values := []*int{navigation, called, coalesced}
	if !present {
		present, payload = true, defaultPointer(&calls)
	}
	if calls != 0 {
		panic("present nil default evaluated")
	}
	var retained *int
	if present {
		retained = payload
	}
	values = append(values, retained)
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
		return fmt.Sprintf("%v", "nil")
	}
	return fmt.Sprintf("%v", *pointer)
}
