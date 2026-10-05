package main

import (
	"fmt"
	"time"
)

// The scenario of typeforms_modern.go with explicit presence structs.

type Opt[T any] struct {
	Present bool
	Value   T
}

func some[T any](v T) Opt[T] { return Opt[T]{true, v} }

func (o Opt[T]) or(fallback T) T {
	if o.Present {
		return o.Value
	}
	return fallback
}

type Box[T any] struct{ V T }

type meter struct{}

func (meter) Meter() Opt[int] { return some(8) }

type walker interface {
	Next() Opt[interface{ Peek() Opt[int] }]
}

type walk struct{}

func (walk) Next() Opt[interface{ Peek() Opt[int] }] {
	return some[interface{ Peek() Opt[int] }](meter2{})
}

type meter2 struct{}

func (meter2) Peek() Opt[int] { return some(9) }

func classify(x any) string {
	switch v := x.(type) {
	case Opt[int]:
		return fmt.Sprint("int? ", v.or(-1))
	case *Opt[int]:
		return fmt.Sprint("*int? ", (*v).or(-1))
	case []Opt[int]:
		return fmt.Sprint("[]int? ", len(v), " ", v[1].or(-1))
	case Opt[[]int]:
		return fmt.Sprint("([]int)? ", len(v.or(nil)))
	case Opt[Opt[int]]:
		return fmt.Sprint("(int?)? ", v.or(Opt[int]{}).or(-1))
	case map[string]Opt[int]:
		return fmt.Sprint("map ", len(v), " ", v["a"].or(-1), " ", v["zz"].or(-2))
	case func() Opt[int]:
		return fmt.Sprint("func ", v().or(-1))
	case func(Opt[int]) Opt[int]:
		return fmt.Sprint("func(int?) ", v(Opt[int]{}).or(-3), " ", v(some(4)).or(-3))
	case chan Opt[int]:
		return fmt.Sprint("chan ", cap(v))
	case [2]Opt[int]:
		return fmt.Sprint("array ", v[0].or(-1), " ", v[1].or(-1))
	case struct{ X Opt[int] }:
		return fmt.Sprint("struct ", v.X.or(-1))
	case interface{ Meter() Opt[int] }:
		return fmt.Sprint("meter ", v.Meter().or(-1))
	case Box[Opt[int]]:
		return fmt.Sprint("box ", v.V.or(-1))
	case Box[Opt[Opt[int]]]:
		return fmt.Sprint("box2 ", v.V.or(Opt[int]{}).or(-1))
	case Opt[time.Duration]:
		return fmt.Sprint("duration ", v.or(0))
	case Opt[string], Opt[bool]:
		return "string-or-bool"
	}
	return "other"
}

func generic[T any](x any) string {
	switch v := x.(type) {
	case Opt[T]:
		return fmt.Sprint("T? ", v.Present)
	case []Opt[T]:
		return fmt.Sprint("[]T? ", len(v))
	case *Opt[T]:
		return "*T?"
	}
	return "none"
}

func identity[T any](v T) T { return v }

func scenario() {
	var present = some(5)
	var absent Opt[int]
	var pointer = new(Opt[int])
	*pointer = some(6)
	var nested = some(some(3))
	var wrapped = some([]int{1, 2})
	var duration = some(time.Second)
	var text = some("x")
	var flag = some(true)

	emit("switch-1", classify(present)+"|"+classify(absent)+"|"+classify(pointer)+"|"+classify([]Opt[int]{some(1), {}}))
	emit("switch-2", classify(wrapped)+"|"+classify(nested)+"|"+classify(map[string]Opt[int]{"a": some(1), "b": {}}))
	emit("switch-3", classify(func() Opt[int] { return some(1) })+"|"+classify(func(v Opt[int]) Opt[int] { return v })+"|"+classify(make(chan Opt[int], 2)))
	emit("switch-4", classify([2]Opt[int]{some(1), {}})+"|"+classify(struct{ X Opt[int] }{some(3)})+"|"+classify(meter{}))
	emit("switch-5", classify(Box[Opt[int]]{some(4)})+"|"+classify(Box[Opt[Opt[int]]]{some(some(5))})+"|"+classify(duration))
	emit("switch-6", classify(text)+"|"+classify(flag)+"|"+classify(7)+"|"+classify([]int{1}))
	emit("generic", generic[int](present)+"|"+generic[int](absent)+"|"+generic[string]([]Opt[string]{{}})+"|"+generic[int](new(Opt[int]))+"|"+generic[int](text))

	n, ok := any(present).(Opt[int])
	emit("assert-1", fmt.Sprint(n.or(0), " ", ok))
	p, ok := any(pointer).(*Opt[int])
	emit("assert-2", fmt.Sprint(p == pointer, " ", ok))
	slice := any([]Opt[int]{some(1), some(2), some(3)}).([]Opt[int])
	emit("assert-3", len(slice))
	_, ok = any(present).(Opt[string])
	emit("assert-4", ok)
	outer, ok := any(nested).(Opt[Opt[int]])
	emit("assert-5", fmt.Sprint(outer.or(Opt[int]{}).or(-1), " ", ok))
	var shape any = struct{ X Opt[int] }{some(7)}
	emit("assert-6", shape.(struct{ X Opt[int] }).X.or(-1))
	var service any = walk{}
	if w, ok := service.(walker); ok {
		next := w.Next()
		emit("assert-7", next.or(meter2{}).Peek().or(-1))
	}

	emit("literal", fmt.Sprint(len([]Opt[int]{some(1), {}, some(3)}), " ", len(map[string]Opt[int]{"a": some(1)}), " ", [2]Opt[int]{{}, some(2)}[1].or(-1), " ", (struct{ X Opt[int] }{some(4)}).X.or(-1), " ", Box[Opt[int]]{some(5)}.V.or(-1)))
	emit("conversion", fmt.Sprint(some(7).or(0), " ", Opt[int]{}.or(9), " ", ([]Opt[int])(nil) == nil, " ", len(([]Opt[int])([]Opt[int]{some(1)}))))
	emit("instantiation", fmt.Sprint(identity[Opt[int]](some(5)).or(0), " ", identity[Opt[int]](Opt[int]{}).or(6), " ", identity[Opt[Opt[int]]](nested).Present, " ", len(identity[[]Opt[int]]([]Opt[int]{some(1), {}})), " ", len(identity[map[string]Opt[int]](nil)), " ", identity[func() Opt[int]](func() Opt[int] { return some(2) })().or(0)))
	emit("builtins", fmt.Sprint(len(make([]Opt[int], 3)), " ", len(make(map[string]Opt[int])), " ", cap(make(chan Opt[int], 2)), " ", !(*new(Opt[int])).Present))
}
