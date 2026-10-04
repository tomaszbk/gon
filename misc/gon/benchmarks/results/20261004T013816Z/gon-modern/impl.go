package main

import (
	"fmt"
	"unsafe"
)

func propagate(seed int, e *effects) (int, error) {
	value := readAmount(seed, e)!
	value = validateAmount(value, e)!
	return value, nil
}

func handle(seed int, e *effects) (int, error) {
	value := readAmount(seed, e) or err {
		return 0, fmt.Errorf("read amount: %w", err)
	}
	return value + 7, nil
}

func conditional(active bool, value int, e *effects) int {
	return if active { trueValue(value, e) } else { falseValue(value, e) }
}

func capturedLambda(values []int, offset int) int {
	var transform func(int) int = (value) => value*2 + offset
	offset += 2 // The closure observes the updated captured variable.
	return sumMapped(values, transform)
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
type intResult = Result[int, error]

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
	case benchEvent.Empty => matchBody(0, e)
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
		total += slots[i] ?? -1
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
func makeResult(seed int, failed, nilFailure bool, e *effects) intResult {
	e.ResultReads++
	if failed {
		var problem error
		if !nilFailure {
			problem = errLimit
		}
		return .Err(problem)
	}
	return .Ok(seed)
}
func transformResult(input intResult, e *effects) intResult {
	n := input!
	e.ResultTransforms++
	return .Ok(n + 3)
}
func resultScenario(seed int, failed, nilFailure bool, e *effects) (int, bool, error) {
	n := transformResult(makeResult(seed, failed, nilFailure, e), e) or problem {
		return 0, true, problem
	}
	return n, false, nil
}
func resultHandle(seed int, failed, nilFailure bool, e *effects) (int, bool) {
	n := makeResult(seed, failed, nilFailure, e) or problem {
		e.ResultHandlers++
		if problem == nil {
			e.NilFailures++
		}
		return resultDefault(e), true
	}
	return n, false
}
func resultZeroValue() (int, bool, error) {
	var value intResult
	return switch value {
	case intResult.Ok(n) => n
	case intResult.Err(_) => -1
	}, false, nil
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
		"struct{}?":         unsafe.Sizeof((struct{}?)(nil)),
		"Result[int,error]": unsafe.Sizeof(intResult.Ok(0)), "Result[int,string]": unsafe.Sizeof(Result[int, string].Ok(0)),
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
