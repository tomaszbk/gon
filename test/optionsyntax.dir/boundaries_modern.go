package main

import "fmt"

type Identity = interface{ ID() int }
type box struct{}

func (*box) ID() int { return 7 }

type Optional[T any] = T?

var effects string
var globalValue int? = mark("G", 11)
var globalAbsent int? = nil
var globalPointer (*box)? = (*box)(nil)

func mark[T any](name string, value T) T   { effects += name; return value }
func generic[T any](anchor T, value T?) T? { _ = anchor; return value }
func optionLambda[T any](anchor T, callback func() T?) T? { _ = anchor; return callback() }
func target() func(anchor int, value int?) int? {
	effects += "F"
	return func(anchor int, value int?) int? { _ = anchor; return value }
}
func propagated(value int?) int?    { return value? }
func optionalType[T any](value T) T { return value }

func main() {
	fmt.Println("globals", globalValue ?? 0, globalAbsent ?? 12, (globalPointer ?? &box{}) == nil)
	var nilInterface Identity
	var typedNil *box
	var fromNilInterface Optional[Identity] = nilInterface
	var fromTypedNil Optional[Identity] = typedNil
	var explicit Optional[Identity] = (Identity)(nil)
	var absent Optional[Identity] = nil
	fallback := Identity(&box{})
	fmt.Println("interfaces", (fromNilInterface ?? fallback) == nil, (fromTypedNil ?? fallback) == nil, (explicit ?? fallback) == nil, (absent ?? fallback) == nil)
	fmt.Println("typed dynamic", (fromTypedNil ?? fallback).ID())
	var nested (Identity?)? = (Identity?)(nil)
	var nestedPresent (Identity?)? = (Identity?)((Identity)(nil))
	var nestedLift (Identity?)? = explicit
	fmt.Println("nested", switch nested {
	case nil? => true
	default => false
	}, switch nestedPresent {
	case (nil?)? => true
	default => false
	}, nestedLift == nestedPresent)
	var cond Optional[Identity] = if true { mark("C", nilInterface) } else { mark("X", typedNil) }
	var matched Optional[Identity] = switch false {
	case true => mark("Y", nilInterface)
	case false => mark("M", typedNil)
	}
	fmt.Println("branches", (cond ?? fallback) == nil, (matched ?? fallback) == nil)
	value := generic(value: (int)(mark("V", 2)), anchor: mark("A", 1))
	named := target()(value: mark("N", 3), anchor: mark("B", 4))
	fmt.Println("calls", value ?? 0, named ?? 0)
	optional := optionLambda(1, () => mark("O", 6))
	fmt.Println("lambdas", optional ?? 0)
	var grouped int? = (7)
	var suffix = optionalType[int?]((int)(8))
	fmt.Println("type-propagation", propagated(grouped) ?? 0, propagated(nil) ?? 9, suffix ?? 0)
	fmt.Println("effects", effects)
}
