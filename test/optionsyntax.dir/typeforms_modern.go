package main

import (
	"fmt"
	"time"
)

// Optional type expressions in every position where a type may be written:
// type switch cases, assertions, composite literal types, conversions, generic
// instantiation and the built-ins that take a type.

type Box[T any] struct{ V T }

type meter struct{}

func (meter) Meter() int? { return 8 }

// walker's methods mention optional types inside anonymous interfaces.
type walker interface {
	Next() interface{ Peek() int? }?
}

type walk struct{}

func (walk) Next() interface{ Peek() int? }? { return meter2{} }

type meter2 struct{}

func (meter2) Peek() int? { return 9 }

func classify(x any) string {
	switch v := x.(type) {
	case int?:
		return fmt.Sprint("int? ", v ?? -1)
	case *int?:
		return fmt.Sprint("*int? ", (*v) ?? -1)
	case []int?:
		return fmt.Sprint("[]int? ", len(v), " ", v[1] ?? -1)
	case ([]int)?:
		return fmt.Sprint("([]int)? ", len(v ?? nil))
	case (int?)?:
		return fmt.Sprint("(int?)? ", (v ?? (int?)(nil)) ?? -1)
	case map[string]int?:
		return fmt.Sprint("map ", len(v), " ", v["a"] ?? -1, " ", v["zz"] ?? -2)
	case func() int?:
		return fmt.Sprint("func ", v() ?? -1)
	case func(int?) int?:
		return fmt.Sprint("func(int?) ", v(nil) ?? -3, " ", v(4) ?? -3)
	case chan int?:
		return fmt.Sprint("chan ", cap(v))
	case [2]int?:
		return fmt.Sprint("array ", v[0] ?? -1, " ", v[1] ?? -1)
	case struct{ X int? }:
		return fmt.Sprint("struct ", v.X ?? -1)
	case interface{ Meter() int? }:
		return fmt.Sprint("meter ", v.Meter() ?? -1)
	case Box[int?]:
		return fmt.Sprint("box ", v.V ?? -1)
	case Box[(int?)?]:
		return fmt.Sprint("box2 ", (v.V ?? (int?)(nil)) ?? -1)
	case time.Duration?:
		return fmt.Sprint("duration ", v ?? 0)
	case string?, bool?:
		return "string-or-bool"
	}
	return "other"
}

func generic[T any](x any) string {
	switch v := x.(type) {
	case T?:
		return fmt.Sprint("T? ", has(v))
	case []T?:
		return fmt.Sprint("[]T? ", len(v))
	case *T?:
		return "*T?"
	}
	return "none"
}

func identity[T any](v T) T { return v }

func has[T any](v T?) bool {
	return switch v {
	case nil => false
	default => true
	}
}

func scenario() {
	var present int? = 5
	var absent int?
	var pointer = new(int?)
	*pointer = 6
	var nested (int?)? = (int?)(3)
	var wrapped ([]int)? = []int{1, 2}
	var duration time.Duration? = time.Second
	var text string? = "x"
	var flag bool? = true

	emit("switch-1", classify(present)+"|"+classify(absent)+"|"+classify(pointer)+"|"+classify([]int?{1, nil}))
	emit("switch-2", classify(wrapped)+"|"+classify(nested)+"|"+classify(map[string]int?{"a": 1, "b": nil}))
	emit("switch-3", classify(func() int? { return 1 })+"|"+classify(func(v int?) int? { return v })+"|"+classify(make(chan int?, 2)))
	emit("switch-4", classify([2]int?{1, nil})+"|"+classify(struct{ X int? }{3})+"|"+classify(meter{}))
	emit("switch-5", classify(Box[int?]{4})+"|"+classify(Box[(int?)?]{(int?)(5)})+"|"+classify(duration))
	emit("switch-6", classify(text)+"|"+classify(flag)+"|"+classify(7)+"|"+classify([]int{1}))
	emit("generic", generic[int](present)+"|"+generic[int](absent)+"|"+generic[string]([]string?{nil})+"|"+generic[int](new(int?))+"|"+generic[int](text))

	n, ok := any(present).(int?)
	emit("assert-1", fmt.Sprint(n ?? 0, " ", ok))
	p, ok := any(pointer).(*int?)
	emit("assert-2", fmt.Sprint(p == pointer, " ", ok))
	slice := any([]int?{1, 2, 3}).([]int?)
	emit("assert-3", len(slice))
	_, ok = any(present).(string?)
	emit("assert-4", ok)
	outer, ok := any(nested).((int?)?)
	emit("assert-5", fmt.Sprint((outer ?? (int?)(nil)) ?? -1, " ", ok))
	var shape any = struct{ X int? }{7}
	emit("assert-6", shape.(struct{ X int? }).X ?? -1)
	var service any = walk{}
	if w, ok := service.(walker); ok {
		next := w.Next()
		emit("assert-7", (next ?? interface{ Peek() int? }(meter2{})).Peek() ?? -1)
	}

	emit("literal", fmt.Sprint(len([]int?{1, nil, 3}), " ", len(map[string]int?{"a": 1}), " ", [2]int?{nil, 2}[1] ?? -1, " ", (struct{ X int? }{4}).X ?? -1, " ", Box[int?]{5}.V ?? -1))
	emit("conversion", fmt.Sprint((int?)(7) ?? 0, " ", (int?)(nil) ?? 9, " ", ([]int?)(nil) == nil, " ", len(([]int?)([]int?{1}))))
	emit("instantiation", fmt.Sprint(identity[int?](5) ?? 0, " ", identity[int?](nil) ?? 6, " ", has(identity[(int?)?](nested)), " ", len(identity[[]int?]([]int?{1, nil})), " ", len(identity[map[string]int?](nil)), " ", identity[func() int?](func() int? { return 2 })() ?? 0))
	emit("builtins", fmt.Sprint(len(make([]int?, 3)), " ", len(make(map[string]int?)), " ", cap(make(chan int?, 2)), " ", !has(*new(int?))))
}
