package main

import (
	"fmt"
	"gon/seq"
	"unsafe"
)

func propagate(seed int, e *effects) (int, error) {
	value := readAmount(seed, e)!
	value = validateAmount(value, e)!
	return value, nil
}

func handle(seed int, e *effects) (int, error) {
	value := readAmount(seed, e) or err => fmt.Errorf("read amount: %w", err)
	return value + 7, nil
}

func conditional(active bool, value int, e *effects) int {
	return if active { trueValue(value, e) } else { falseValue(value, e) }
}

func capturedLambda(values []int, offset int) int {
	var transform func(int) int = (value) => value*2 + offset
	offset += 2 // The closure observes the updated captured variable.
	return seq.Reduce(values, 0, (sum, value) => sum + transform(value))
}

func safeField(rule *policy, e *effects) int {
	return rule?.Factor ?? defaultNumber(e)
}

func safeCall(callback func(int) int, value int, e *effects) int {
	return callback?(argument(value, e)) ?? defaultNumber(e)
}

func coalesceAssign(slots []*policy, index int, e *effects) int {
	slots[location(index, e)] ??= defaultPolicy(e)
	return slots[index].Factor
}

func pipeline(item input, e *effects) (int, error) {
	value := parseAmount(item.Text, e) or err {
		return 0, fmt.Errorf("parse amount: %w", err)
	}
	rule := item.Rule ?? defaultPolicy(e)
	value = if item.Active { trueValue(value, e) } else { falseValue(value, e) }
	var adjust func(int) int = (value) => value * rule.Factor
	value = adjust(value)
	value = item.Callback?(argument(value, e)) ?? value
	value = validateAmount(value, e)!
	return value, nil
}

type benchEvent enum {
	default Empty
	Number(int)
	Point { X, Y int }
}
type benchUnit enum {
	default Idle
	Active
}
type intOption = int?
type pointerOption = (*int)?

func constructEvent(seed int, e *effects) benchEvent {
	if e != nil {
		e.Constructions++
	}
	switch seed % 3 {
	case 0:
		return benchEvent.Empty
	case 1:
		return benchEvent.Number(seed)
	default:
		return benchEvent.Point{X: seed, Y: seed + 1}
	}
}
func matchEvent(value benchEvent, guarded bool, e *effects) int {
	if guarded {
		return switch value {
		case benchEvent.Empty => matchBody(0, e)
		case benchEvent.Number(n) if matchGuard(n, e) => matchBody(n*2, e)
		case benchEvent.Number(n) => matchBody(n, e)
		case benchEvent.Point{X: x, Y: y} => matchBody(x+y, e)
		}
	}
	return switch value {
	case benchEvent.Empty, benchEvent.Number(0) => matchBody(0, e)
	case benchEvent.Number(n) => matchBody(n, e)
	case benchEvent.Point{X: x, Y: y} => matchBody(x+y, e)
	}
}
func makeOption(seed int, present bool, e *effects) intOption {
	e.OptionReads++
	if !present {
		return nil
	}
	return seed
}
func transformOption(input intOption, e *effects) intOption {
	n := input?
	e.OptionTransforms++
	return n * 2
}
func optionScenario(seed int, present bool, e *effects) int {
	return transformOption(makeOption(seed, present, e), e) ?? optionDefault(e)
}
func optionAssignmentBatch(e *effects) int {
	var slots [batchSize]intOption
	for i := range slots {
		if i%2 == 0 {
			slots[i] = i
		}
	}
	total := 0
	for i := range slots {
		slots[location(i, e)] ??= optionDefault(e)
		if slots[i] is value? {
			total += value
		} else {
			total -= 1
		}
	}
	return total
}
func optionNilPresent() bool {
	var present pointerOption = (*int)(nil)
	return (present ?? new(int)) == nil
}
func optionNestedAbsent() int {
	var outer (int?)? = (int?)(nil)
	return (outer ?? (int?)(13)) ?? 7
}
func namedReordered(seed int, e *effects) int {
	return namedCallee(e)(right: namedArgument(3, seed+1, e), left: namedArgument(2, seed, e))
}
func namedVariadic(seed int, e *effects) int {
	e.ArgumentTrace = 1
	items := [2]int{seed + 1, seed + 2}
	return variadicTarget(prefix: namedArgument(2, seed, e), values: items[:]...)
}
func representationSizes() map[string]uintptr {
	return map[string]uintptr{
		"enum/record": unsafe.Sizeof(benchEvent.Empty), "enum/unit": unsafe.Sizeof(benchUnit.Idle),
		"int?": unsafe.Sizeof((intOption)(nil)), "(*int)?": unsafe.Sizeof((pointerOption)(nil)),
		"struct{}?": unsafe.Sizeof((struct{}?)(nil)),
	}
}
func retainedPointers(count int) (any, uint64) {
	values := make([]pointerOption, count)
	for i := range values {
		n := i
		values[i] = &n
	}
	return values, uint64(count) * uint64(unsafe.Sizeof((pointerOption)(nil)))
}
func retainedChecksum(root any) int64 {
	total := int64(0)
	for _, value := range root.([]pointerOption) {
		total += int64(*(value ?? nil))
	}
	return total
}

func formattedBatch() int {
	total := 0
	for i := 0; i < batchSize; i++ {
		total += len($"${i}:${i + 1:%04d}")
	}
	return total
}
