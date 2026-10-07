package main

import (
	"fmt"
)

type Maybe[T any] struct {
	Present bool
	Value   T
}
type Container struct{ Value Maybe[int] }

func some[T any](value T) Maybe[T] { return Maybe[T]{true, value} }
func optional(present bool) (number Maybe[int]) {
	defer func() { effects += "D" + optionalLabel(number) }()
	if !present {
		return Maybe[int]{}
	}
	return some(mark("V", 5))
}
func optionalLabel(value Maybe[int]) string {
	if !value.Present {
		return "None"
	}
	return fmt.Sprint("Some:", value.Value)
}
func inferred[T any](anchor T, value Maybe[T]) Maybe[T] { _ = anchor; return value }
func shadowed() Maybe[int]                              { nil := 6; return some(nil) }
func shadowedOption() Maybe[int] {
	type Option[T any] struct{ Value T }
	_ = Option[int]{3}
	return some(7)
}
func groupedPointer(value *int) Maybe[*int]            { return some(value) }
func groupedSlice(value []int) Maybe[[]int]            { return some(value) }
func groupedNested(value Maybe[int]) Maybe[Maybe[int]] { return some(value) }

func scenario() {
	pointerGroup := groupedPointer(nil)
	emit("grouped pointer", pointerGroup.Present && pointerGroup.Value == nil)
	sliceGroup := groupedSlice(nil)
	emit("grouped slice", sliceGroup.Present && sliceGroup.Value == nil)
	nestedGroup := groupedNested(Maybe[int]{})
	emit("grouped nested", nestedGroup.Present && !nestedGroup.Value.Present)
	emit("absent", optionalLabel(optional(false)))
	emit("present", optionalLabel(optional(true)))
	number := some(mark("A", 0))
	emit("zero present", optionalLabel(number))
	number = Maybe[int]{}
	emit("assigned absent", optionalLabel(number))
	number = some(mark("B", 4))
	emit("assigned present", optionalLabel(number))
	var pointer *int
	presentPointer := some(pointer)
	explicitPointer := some[*int](nil)
	absentPointer := Maybe[*int]{}
	ptr := func(value Maybe[*int]) *int {
		if value.Present {
			return value.Value
		}
		return new(int)
	}
	emit("typed nil present", ptr(presentPointer) == nil)
	emit("explicit nil present", ptr(explicitPointer) == nil)
	emit("nil absent", ptr(absentPointer) == nil)
	inner := Maybe[int]{}
	outer := some(inner)
	emit("nested present", outer.Present && !outer.Value.Present)
	nested := some(some(3))
	emit("nested value", nested.Value.Value)
	values := []Maybe[int]{some(mark("C", 1)), {}, some(mark("E", 3))}
	for _, value := range values {
		emit("slice", optionalLabel(value))
	}
	box := Container{Value: some(mark("F", 8))}
	emit("field", optionalLabel(box.Value))
	table := map[string]Maybe[int]{"a": some(9)}
	table["b"] = Maybe[int]{}
	emit("map", optionalLabel(table["a"])+":"+optionalLabel(table["b"]))
	emit("generic", optionalLabel(inferred(1, some(12))))
	emit("generic named", optionalLabel(inferred(1, some(13))))
	callback := func(value int) Maybe[int] { return some(value) }
	emit("lambda", optionalLabel(callback(14)))
	conditional := Maybe[int]{}
	if false {
		conditional = some(mark("X", 99))
	}
	emit("conditional", optionalLabel(conditional))
	matched := Maybe[int]{}
	switch true {
	case true:
		matched = some(16)
	}
	emit("match", optionalLabel(matched))
	emit("nil shadow", optionalLabel(shadowed()))
	emit("Option shadow", optionalLabel(shadowedOption()))
	old := some(17)
	emit("old syntax", optionalLabel(old))
}
