package main

import (
	"fmt"
	"reflect"
	"runtime"
	"unitlayout/lib"
	"unsafe"
)

func assert(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func main() {
	var zero lib.Unit
	active := lib.UnitAlternate()
	assert(zero == lib.UnitDefault() && zero != active, "imported unit default")
	assert(unsafe.Sizeof(zero) == 1 && unsafe.Alignof(zero) == 1, "unit size and alignment")
	assert(reflect.TypeFor[lib.Unit]().Size() == 1 && reflect.TypeFor[lib.Unit]().Align() == 1, "reflect unit layout")
	var single lib.Single
	assert(unsafe.Sizeof(single) == 1 && single == lib.SingleDefault() && reflect.ValueOf(single).IsZero(), "single-variant unit layout")
	assert(unsafe.Sizeof([3]lib.Unit{}) == 3, "dense unit array")
	packed := struct {
		Prefix byte
		Value  lib.Unit
		Suffix byte
	}{}
	assert(unsafe.Sizeof(packed) == 3 && unsafe.Offsetof(packed.Value) == 1, "dense embedded unit")

	var alias lib.Alias = active
	assert(alias == active && reflect.TypeFor[lib.Alias]() == reflect.TypeFor[lib.Unit](), "unit alias identity")
	var generic lib.Flag[[]byte]
	assert(generic == lib.DefaultFlag[[]byte]() && generic != lib.ReadyFlag[[]byte](), "generic unit with noncomparable type argument")
	assert(unsafe.Sizeof(generic) == 1 && reflect.TypeFor[lib.Flag[[]byte]]().Comparable(), "generic unit layout and comparability")
	var genericAlias lib.FlagAlias[int] = lib.ReadyFlag[int]()
	assert(genericAlias == lib.ReadyFlag[int]() && unsafe.Sizeof(genericAlias) == 1, "generic alias layout")

	values := make([]lib.Unit, 1024)
	for i := range values {
		assert(values[i] == zero, "dense slice zero initialization")
		if i%2 != 0 {
			values[i] = active
		}
	}
	assert(uintptr(unsafe.Pointer(&values[1]))-uintptr(unsafe.Pointer(&values[0])) == 1, "dense slice element stride")
	copied := append([]lib.Unit(nil), values...)
	assert(reflect.DeepEqual(values, copied), "dense slice copy")
	reflect.Copy(reflect.ValueOf(copied), reflect.ValueOf(values))
	runtime.GC()
	for i, v := range copied {
		assert((i%2 == 0 && v == zero) || (i%2 != 0 && v == active), "dense slice retention")
	}
	array := [3]lib.Unit{zero, active, zero}
	assert(reflect.ValueOf(array).Equal(reflect.ValueOf(array)), "unit array reflection equality")
	assert(map[lib.Unit]int{zero: 7, active: 9}[active] == 9, "unit map hashing")
	assert(map[any]int{zero: 7, active: 9}[active] == 9, "unit interface map hashing")

	boxed := any(active)
	runtime.GC()
	assert(boxed.(lib.Unit) == active && reflect.ValueOf(boxed).Equal(reflect.ValueOf(active)), "boxed unit copy and equality")
	assert(reflect.ValueOf(zero).IsZero() && !reflect.ValueOf(active).IsZero(), "unit reflection zero")
	var destination lib.Unit
	reflect.ValueOf(&destination).Elem().Set(reflect.ValueOf(boxed))
	assert(destination == active, "whole unit reflection copy")
	reflect.ValueOf(&destination).Elem().SetZero()
	assert(destination == zero, "whole unit reflection SetZero")
	assert(reflect.Zero(reflect.TypeFor[lib.Unit]()).Interface() == zero, "unit reflected zero construction")
	assert(fmt.Sprint(active) == "lib.Unit.Green" && fmt.Sprint(zero) == "lib.Unit.Red", "unit formatting")
	assert(lib.UnitMetadata() == "Green,Red,Blue", "unit declaration metadata")

	checkImportedUnitConstructors()
	assert(lib.BoundaryCheck() == 65536, "all boundary variants round trip")
	assert(unsafe.Sizeof(lib.Last256()) == 1 && unsafe.Sizeof(lib.Last257()) == 2, "256/257 variant widths")
	assert(unsafe.Sizeof([3]lib.Wide256{}) == 3 && unsafe.Sizeof([3]lib.Wide257{}) == 6, "dense boundary arrays")
	assert(map[lib.Wide256]int{lib.Last256(): 255}[lib.Last256()] == 255, "last uint8 discriminator key")
	assert(map[lib.Wide257]int{lib.Last257(): 256}[lib.Last257()] == 256, "last uint16 discriminator key")
	assert(reflect.ValueOf(lib.Last256()).Equal(reflect.ValueOf(lib.Last256())) && reflect.ValueOf(lib.Last257()).Equal(reflect.ValueOf(lib.Last257())), "boundary reflection equality")
	var reset lib.Wide257 = lib.Last257()
	reflect.ValueOf(&reset).Elem().SetZero()
	assert(reset == lib.Default257(), "wide discriminator reflection zero")
	fmt.Println("unit enum layout ok: sizes=1,1,1,2; variants=" + lib.UnitMetadata())
}
