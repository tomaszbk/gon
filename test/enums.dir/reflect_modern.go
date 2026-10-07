package main

import (
	"fmt"
	"reflect"
)

type Ref enum {
	Text(string)
	default Empty
	Pair  {
		X, Y   int
		hidden string
	}
	Pointer(*int)
	private(int)
}
type Generic[T any] enum {
	default None
	Some(T)
}

func inspectGeneric[T any](v Generic[T]) T {
	r := reflect.ValueOf(v)
	assert(reflect.IsEnum(r.Type()) && reflect.EnumValueVariant(r).Name == "Some", "generic enum runtime metadata")
	return reflect.EnumValuePayload(r, 0).Interface().(T)
}

type RefSlice enum {
	default Empty
	Values([]int)
}

func mustPanic(f func()) {
	defer func() { assert(recover() != nil, "reflection operation must be rejected") }()
	f()
}

func main() {
	assert(inspectGeneric(Generic[string].Some("generic")) == "generic", "generic payload reflection")
	t := reflect.TypeFor[Ref]()
	assert(reflect.IsEnum(t) && t.Kind() == reflect.Struct && t.Name() == "Ref", "enum runtime identity")
	variants := reflect.EnumVariants(t)
	assert(len(variants) == 5 && variants[1].Name == "Empty" && variants[1].Default, "declaration order and default")
	assert(variants[2].Record && len(variants[2].Fields) == 3 && variants[2].Fields[1].Name == "Y", "grouped record metadata")
	assert(variants[0].Fields[0].Name == "" && variants[0].Fields[0].Type == reflect.TypeFor[string](), "positional payload metadata")
	variants[0].Name = "edited"
	assert(reflect.EnumVariants(t)[0].Name == "Text", "metadata is detached")
	v := reflect.ValueOf(Ref.Pair{X: 7, Y: 9, hidden: "private"})
	assert(reflect.EnumValueVariant(v).Name == "Pair" && reflect.EnumValuePayload(v, 0).Int() == 7, "checked active payload")
	assert(!reflect.EnumValuePayload(v, 0).CanSet() && !reflect.EnumValuePayload(v, 0).CanAddr(), "payload copied")
	assert(!reflect.EnumValuePayload(v, 2).CanInterface() && !reflect.EnumValuePayload(reflect.ValueOf(Ref.private(3)), 0).CanInterface(), "private visibility")
	assert(t.NumField() == 0 && v.NumField() == 0, "backing fields hidden")
	_, found := t.FieldByName("$gonTag")
	assert(!found, "tag is inaccessible")
	for range t.Fields() {
		panic("enum backing field leaked")
	}
	mustPanic(func() { _ = v.Field(0) })
	mustPanic(func() { _ = t.Field(0) })
	mustPanic(func() { _ = reflect.EnumValuePayload(v, 3) })
	mustPanic(func() { _ = reflect.EnumValuePayload(reflect.ValueOf(Ref.Empty), 0) })
	assert(reflect.ValueOf(Ref.Empty).IsZero() && !reflect.ValueOf(Ref.Text("")).IsZero(), "zero policy")
	assert(v.Equal(reflect.ValueOf(Ref.Pair{X: 7, Y: 9, hidden: "private"})) && !v.Equal(reflect.ValueOf(Ref.Pair{X: 8, Y: 9, hidden: "private"})), "reflect equality")
	assert(reflect.DeepEqual(RefSlice.Values([]int{1, 2}), RefSlice.Values([]int{1, 2})) && !reflect.DeepEqual(RefSlice.Values(nil), RefSlice.Empty), "active deep equality")
	assert(!reflect.ValueOf(RefSlice.Empty).Comparable(), "whole enum comparability")
	mustPanic(func() { reflect.ValueOf(RefSlice.Empty).Equal(reflect.ValueOf(RefSlice.Empty)) })
	assert(reflect.ValueOf(RefSlice.Empty).IsZero() && !reflect.ValueOf(RefSlice.Values(nil)).IsZero(), "noncomparable enum zero")
	n := 41
	boxed := any(Ref.Pointer(&n))
	copy := reflect.EnumValuePayload(reflect.ValueOf(boxed), 0)
	collect()
	assert(copy.Interface().(*int) == &n && *copy.Interface().(*int) == 41, "payload GC retention")
	*copy.Interface().(*int) = 42
	assert(*reflect.EnumValuePayload(reflect.ValueOf(boxed), 0).Interface().(*int) == 42, "ordinary pointer aliases")
	var dest Ref
	reflect.ValueOf(&dest).Elem().Set(reflect.ValueOf(Ref.Text("set")))
	assert(dest == Ref.Text("set"), "whole enum copy")
	reflect.ValueOf(&dest).Elem().SetZero()
	assert(dest == Ref.Empty, "whole enum zero")
	optionalType := reflect.TypeFor[(*int)?]()
 optionalValue := reflect.ValueOf(((*int)?)( (*int)(nil)))
 assert(reflect.IsOptional(optionalType) && !reflect.IsEnum(optionalType) && reflect.OptionalElement(optionalType) == reflect.TypeFor[*int](), "native optional metadata")
 assert(reflect.OptionalValuePresent(optionalValue) && reflect.OptionalValuePayload(optionalValue).IsNil(), "present nil reflection")
 assert(!reflect.OptionalValuePayload(optionalValue).CanSet() && !reflect.OptionalValuePayload(optionalValue).CanAddr(), "optional payload detached")
 var missing int?
 assert(!reflect.OptionalValuePresent(reflect.ValueOf(missing)) && reflect.ValueOf(missing).IsZero(), "optional absence reflection")
 mustPanic(func() { _ = reflect.OptionalValuePayload(reflect.ValueOf(missing)) })
 mustPanic(func() { _ = reflect.EnumVariants(optionalType) })
 mustPanic(func() { _ = optionalValue.Field(0) })
 assert(optionalType.NumField() == 0 && optionalType.String() == "(*int)?", "private optional representation")
 assert(reflect.ValueOf((int?)(0)).Comparable() && reflect.ValueOf((int?)(0)).Equal(reflect.ValueOf((int?)(0))), "optional equality reflection")
 assert(!reflect.ValueOf((int?)(0)).IsZero() && !reflect.DeepEqual((int?)(nil), (int?)(0)), "present zero retained")
 assert(fmt.Sprint((int?)(nil)) == "nil" && fmt.Sprint((int?)(7)) == "7", "optional formatting")
 assert(fmt.Sprint(map[int?]int{(int?)(2):2, (int?)(nil):0, (int?)(1):1}) == "map[nil:0 1:1 2:2]", "optional map ordering")
	assert(fmt.Sprint(Ref.Text("value")) == "main.Ref.Text(value)" && fmt.Sprint(Ref.Empty) == "main.Ref.Empty", "active enum formatting")
	assert(fmt.Sprint(Ref.Pair{X: 1, Y: 2}) == "main.Ref.Pair{X:1, Y:2, hidden:}", "record enum formatting")
	assert(fmt.Sprint(map[Ref]int{Ref.Text("b"): 2, Ref.Text("a"): 1}) == "map[main.Ref.Text(a):1 main.Ref.Text(b):2]", "enum map key ordering")
	finish()
}
