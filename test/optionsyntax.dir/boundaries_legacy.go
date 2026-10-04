package main

import "fmt"

type Identity = interface{ ID() int }
type box struct{}

func (*box) ID() int { return 7 }

type Optional[T any] struct {
	Present bool
	Value   T
}

func some[T any](value T) Optional[T] { return Optional[T]{true, value} }
func fallback[T any](value Optional[T], other T) T {
	if value.Present {
		return value.Value
	}
	return other
}

var effects string
var globalValue = some(mark("G", 11))
var globalAbsent Optional[int]
var globalPointer = some((*box)(nil))

func mark[T any](name string, value T) T                     { effects += name; return value }
func generic[T any](anchor T, value Optional[T]) Optional[T] { _ = anchor; return value }
func target() func(anchor int, value Optional[int]) Optional[int] {
	effects += "F"
	return func(anchor int, value Optional[int]) Optional[int] { _ = anchor; return value }
}
func propagated(value Optional[int]) Optional[int] {
	if !value.Present {
		return Optional[int]{}
	}
	return some(value.Value)
}
func optionalType[T any](value T) T { return value }
func main() {
	fmt.Println("globals", fallback(globalValue, 0), fallback(globalAbsent, 12), fallback(globalPointer, &box{}) == nil)
	var nilInterface Identity
	var typedNil *box
	fromNilInterface := some(nilInterface)
	fromTypedNil := some[Identity](typedNil)
	explicit := some[Identity](nil)
	absent := Optional[Identity]{}
	replacement := Identity(&box{})
	fmt.Println("interfaces", fallback(fromNilInterface, replacement) == nil, fallback(fromTypedNil, replacement) == nil, fallback(explicit, replacement) == nil, fallback(absent, replacement) == nil)
	fmt.Println("typed dynamic", fallback(fromTypedNil, replacement).ID())
	nested := some(Optional[Identity]{})
	nestedPresent := some(some[Identity](nil))
	nestedLift := some(explicit)
	fmt.Println("nested", nested.Present && !nested.Value.Present, nestedPresent.Present && nestedPresent.Value.Present && nestedPresent.Value.Value == nil, nestedLift == nestedPresent)
	var cond Optional[Identity]
	if true {
		cond = some(mark("C", nilInterface))
	} else {
		cond = some[Identity](mark("X", typedNil))
	}
	var matched Optional[Identity]
	switch false {
	case true:
		matched = some(mark("Y", nilInterface))
	case false:
		matched = some[Identity](mark("M", typedNil))
	}
	fmt.Println("branches", fallback(cond, replacement) == nil, fallback(matched, replacement) == nil)
	writtenValue := some(mark("V", 2))
	writtenAnchor := mark("A", 1)
	value := generic(writtenAnchor, writtenValue)
	callee := target()
	namedValue := some(mark("N", 3))
	namedAnchor := mark("B", 4)
	named := callee(namedAnchor, namedValue)
	fmt.Println("calls", fallback(value, 0), fallback(named, 0))
	result := func() int { return mark("L", 5) }()
	optional := func() Optional[int] { return some(mark("O", 6)) }()
	fmt.Println("lambdas", result, fallback(optional, 0))
	grouped := some(7)
	suffix := optionalType[Optional[int]](some(8))
	fmt.Println("type-propagation", fallback(propagated(grouped), 0), fallback(propagated(Optional[int]{}), 9), fallback(suffix, 0))
	fmt.Println("effects", effects)
}
