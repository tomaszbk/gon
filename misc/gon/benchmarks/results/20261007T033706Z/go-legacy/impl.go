package main

import (
	"fmt"
	"unsafe"
)

func propagate(seed int, e *effects) (int, error) {
	value, err := readAmount(seed, e)
	if err != nil {
		return 0, err
	}
	value, err = validateAmount(value, e)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func handle(seed int, e *effects) (int, error) {
	value, err := readAmount(seed, e)
	if err != nil {
		return 0, fmt.Errorf("read amount: %w", err)
	}
	return value + 7, nil
}

func conditional(active bool, value int, e *effects) int {
	if active {
		return trueValue(value, e)
	}
	return falseValue(value, e)
}

func capturedLambda(values []int, offset int) int {
	var transform func(int) int = func(value int) int { return value*2 + offset }
	offset += 2 // The closure observes the updated captured variable.
	return sumMapped(values, transform)
}

func safeField(rule *policy, e *effects) int {
	if rule == nil {
		return defaultNumber(e)
	}
	return rule.Factor
}

func safeCall(callback func(int) int, value int, e *effects) int {
	if callback == nil {
		return defaultNumber(e)
	}
	return callback(argument(value, e))
}

func coalesceAssign(slots []*policy, index int, e *effects) int {
	index = location(index, e)
	if slots[index] == nil {
		slots[index] = defaultPolicy(e)
	}
	return slots[index].Factor
}

func pipeline(item input, e *effects) (int, error) {
	value, err := parseAmount(item.Text, e)
	if err != nil {
		return 0, fmt.Errorf("parse amount: %w", err)
	}
	rule := item.Rule
	if rule == nil {
		rule = defaultPolicy(e)
	}
	if item.Active {
		value = trueValue(value, e)
	} else {
		value = falseValue(value, e)
	}
	var adjust func(int) int = func(value int) int { return value * rule.Factor }
	value = adjust(value)
	if item.Callback != nil {
		value = item.Callback(argument(value, e))
	}
	value, err = validateAmount(value, e)
	if err != nil {
		return 0, err
	}
	return value, nil
}

type benchEvent struct {
	tag    uint
	number int
	x, y   int
}
type benchUnit struct{ tag uint }
type intOption struct {
	present bool
	value   int
}
type pointerOption struct {
	present bool
	value   *int
}
type emptyOption struct {
	present bool
	value   struct{}
}
type nestedOption struct {
	present bool
	value   intOption
}

func constructEvent(seed int, e *effects) benchEvent {
	if e != nil {
		e.Constructions++
	}
	switch seed % 3 {
	case 0:
		return benchEvent{}
	case 1:
		return benchEvent{tag: 1, number: seed}
	default:
		return benchEvent{tag: 2, x: seed, y: seed + 1}
	}
}

func matchEvent(value benchEvent, guarded bool, e *effects) int {
	switch value.tag {
	case 0:
		return matchBody(0, e)
	case 1:
		if guarded && matchGuard(value.number, e) {
			return matchBody(value.number*2, e)
		}
		return matchBody(value.number, e)
	case 2:
		return matchBody(value.x+value.y, e)
	default:
		panic("invalid legacy event tag")
	}
}

func makeOption(seed int, present bool, e *effects) intOption {
	e.OptionReads++
	if !present {
		return intOption{}
	}
	return intOption{present: true, value: seed}
}
func transformOption(input intOption, e *effects) intOption {
	if !input.present {
		return intOption{}
	}
	e.OptionTransforms++
	return intOption{present: true, value: input.value * 2}
}
func optionScenario(seed int, present bool, e *effects) int {
	value := transformOption(makeOption(seed, present, e), e)
	if !value.present {
		return optionDefault(e)
	}
	return value.value
}
func optionAssignmentBatch(e *effects) int {
	var slots [batchSize]intOption
	for i := range slots {
		if i%2 == 0 {
			slots[i] = intOption{true, i}
		}
	}
	total := 0
	for i := range slots {
		index := location(i, e)
		if !slots[index].present {
			slots[index] = intOption{true, optionDefault(e)}
		}
		total += slots[i].value
	}
	return total
}
func optionNilPresent() bool {
	present := pointerOption{present: true}
	return present.present && present.value == nil
}
func optionNestedAbsent() int {
	outer := nestedOption{present: true}
	if !outer.present {
		return 13
	}
	if !outer.value.present {
		return 7
	}
	return outer.value.value
}

func namedReordered(seed int, e *effects) int {
	callee := namedCallee(e)
	right := namedArgument(3, seed+1, e)
	left := namedArgument(2, seed, e)
	return callee(left, right)
}
func namedVariadic(seed int, e *effects) int {
	e.ArgumentTrace = 1
	prefix := namedArgument(2, seed, e)
	items := [2]int{seed + 1, seed + 2}
	return variadicTarget(prefix, items[:]...)
}

func representationSizes() map[string]uintptr {
	return map[string]uintptr{
		"enum/record": unsafe.Sizeof(benchEvent{}), "enum/unit": unsafe.Sizeof(benchUnit{}),
		"int?": unsafe.Sizeof(intOption{}), "(*int)?": unsafe.Sizeof(pointerOption{}),
		"struct{}?": unsafe.Sizeof(emptyOption{}),
	}
}
func retainedPointers(count int) (any, uint64) {
	values := make([]pointerOption, count)
	for i := range values {
		n := i
		values[i] = pointerOption{true, &n}
	}
	return values, uint64(count) * uint64(unsafe.Sizeof(pointerOption{}))
}
func retainedChecksum(root any) int64 {
	total := int64(0)
	for _, value := range root.([]pointerOption) {
		if value.present {
			total += int64(*value.value)
		}
	}
	return total
}

func formattedBatch() int {
	total := 0
	for i := 0; i < batchSize; i++ {
		total += len(fmt.Sprintf("%v:%04d", i, i+1))
	}
	return total
}
