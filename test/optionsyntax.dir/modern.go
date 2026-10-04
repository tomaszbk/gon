package main

import (
	"fmt"
	"strconv"
)

type Parsed = Result[int?, error]
type Maybe = int?
type Container struct{ Value int? }

func optional(present bool) (number int?) {
	defer func() { effects += "D" + optionalLabel(number) }()
	if !present {
		return nil
	}
	return mark("V", 5)
}
func parsePort(text string) (port Parsed) {
	defer func() { effects += "P" + parsedLabel(port) }()
	if text == "" {
		return .Ok(nil)
	}
	value := strconv.Atoi(text) or problem {
		return .Err(problem)
	}
	return .Ok(value)
}
func optionalLabel(value int?) string {
	return switch value {
	case nil => "None"
	case (number)? => fmt.Sprint("Some:", number)
	}
}
func parsedLabel(value Parsed) string {
	return switch value {
	case Parsed.Ok(nil) => "empty"
	case Parsed.Ok((number)?) => fmt.Sprint("port:", number)
	case Parsed.Err(problem) => fmt.Sprint("error:", problem != nil)
	}
}
func inferred[T any](anchor T, value T?) T? { _ = anchor; return value }
func shadowed() int?                                      { nil := 6; return nil }
func shadowedOption() int?                                { type Option[T any] struct{ Value T }; _ = Option[int]{3}; return 7 }
func groupedPointer(value *int) (*int)?                   { return value }
func groupedSlice(value []int) ([]int)?                   { return value }
func groupedNested(value int?) (int?)?                    { return value }

func scenario() {
	emit("grouped pointer", switch groupedPointer(nil) {
	case (nil)? => true
	default => false
	})
	emit("grouped slice", switch groupedSlice(nil) {
	case (nil)? => true
	default => false
	})
	emit("grouped nested", switch groupedNested(nil) {
	case (nil)? => true
	default => false
	})
	emit("absent", optionalLabel(optional(false)))
	emit("present", optionalLabel(optional(true)))
	for _, text := range []string{"", "8080", "bad"} {
		emit("parse", parsedLabel(parsePort(text)))
	}
	var number Maybe = mark("A", 0)
	emit("zero present", optionalLabel(number))
	number = nil
	emit("assigned absent", optionalLabel(number))
	number = (int)(mark("B", 4))
	emit("assigned present", optionalLabel(number))
	var pointer *int
	var presentPointer (*int)? = pointer
	var explicitPointer (*int)? = (*int)(nil)
	var absentPointer (*int)? = nil
	emit("typed nil present", (presentPointer ?? new(int)) == nil)
	emit("explicit nil present", (explicitPointer ?? new(int)) == nil)
	emit("nil absent", (absentPointer ?? new(int)) == nil)
	var inner int? = nil
	var outer (int?)? = inner
	emit("nested present", switch outer {
	case (nil)? => true
	default => false
	})
	var nested (int?)? = (int?)(3)
	emit("nested value", (nested ?? nil) ?? 0)
	values := []int?{mark("C", 1), nil, (int)(mark("E", 3))}
	for _, value := range values {
		emit("slice", optionalLabel(value))
	}
	box := Container{Value: mark("F", 8)}
	emit("field", optionalLabel(box.Value))
	table := map[string]int?{"a": 9}
	table["b"] = nil
	emit("map", optionalLabel(table["a"])+":"+optionalLabel(table["b"]))
	emit("generic", optionalLabel(inferred(1, (int)(12))))
	emit("generic named", optionalLabel(inferred(value: (int)(13), anchor: 1)))
	var callback func(int) int? = (value) => value
	emit("lambda", optionalLabel(callback(14)))
	var resultCallback func(int) Result[int, string] = (value) => .Ok(value)
	emit("result lambda", resultCallback(15) or problem {
		panic(problem)
	})
	var conditional int? = if false { mark("X", 99) } else { nil }
	emit("conditional", optionalLabel(conditional))
	var matched int? = switch true {
	case true => 16
	case false => nil
	}
	emit("match", optionalLabel(matched))
	emit("nil shadow", optionalLabel(shadowed()))
	emit("Option shadow", optionalLabel(shadowedOption()))
	var failure Result[int, error] = .Err(nil)
	emit("Err nil", switch failure {
	case Result[int, error].Err(nil) => true
	default => false
	})
	var old Maybe = (int?)((int)(17))
	emit("old syntax", optionalLabel(old))
}
