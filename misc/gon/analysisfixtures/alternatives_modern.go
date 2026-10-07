package main

type Event enum {
	Text(string)
	default Idle
	Count  {
		Number int
		Ready  bool
	}
}
type User struct{ Name string }
type Box[T any] enum {
	default Empty
	Full(T)
}

var trace string

func mark(s string, n int) int { trace += s; return n }
func check(ok bool) {
	if !ok {
		panic("alternatives mismatch")
	}
}
func classify(v Event) int {
	return switch v {
	case Event.Idle => 0
	case Event.Text(s) if s == "yes" => mark("G", len(s))
	case Event.Text(_) => -1
	case Event.Count{Number: n, Ready: true} => n
	case Event.Count{Number: _, Ready: false} => -2
	}
}
func option(v int?) (out int?) {
	out = (int?)((int)(99))
	defer func() { trace += "D" }()
	n := v?
	return (int?)((int)(n + 1))
}
func patternNames(b bool, p *int) int {
	const item = 99
	true, false, nil := 10, 20, 30
	chosen := switch b {
	case true => true
	case false => false
	}
	pointer := switch ((*int)?)((*int)(p)) {
	case (nil)? => nil
	case (_)? => 0
	case nil => -1
	}
	flags := switch (bool?)((bool)(b)) {
	case (true)? => true
	case (false)? => false
	case nil => nil
	}
	bound := switch (Event.Count{Number: 7}) {
	case Event.Count{Number: item, ...} => item
	default => 0
	}
	ordinary := 0
	switch true {
	case 10:
		ordinary = false + nil
	}
	return chosen + pointer + flags + bound + ordinary + item
}

type Presence[T any] = T?

func absent[T any](v Presence[T]) bool { return v == nil }
func present[T any](v T?) bool         { return nil != v }
func nilComparisons() {
	var missing int?
	var zero int? = 0
	var pointer (*User)? = (*User)(nil)
	var slice ([]int)? = ([]int)(nil)
	var mapping (map[int]int)? = (map[int]int)(nil)
	var callback (func())? = (func())(nil)
	var iface any? = (any)(nil)
	var nested (int?)? = (int?)(nil)
	check(missing == nil && nil == missing && !(missing != nil))
	check(zero != nil && pointer != nil && slice != nil && mapping != nil && callback != nil && iface != nil)
	iface = any([]int{1})
	check(iface != nil)
	{
		nil := zero
		check(zero == nil && missing != nil)
	}
	check(nested != nil && (nested ?? (int?)(1)) == nil)
	slice = nil
	check(absent(slice) && present(zero) && absent(missing))
	trace = ""
	produce := func(label string, exists bool) int? {
		trace += label
		if exists {
			return 0
		}
		return nil
	}
	check(produce("A", false) == nil && nil != produce("B", true) && trace == "AB")
	check(!(produce("C", true) == nil && produce("X", false) == nil) && trace == "ABC")
	trace = ""
}
func main() {
	nilComparisons()
	check(patternNames(true, nil) == 206)
	one := 1
	check(patternNames(false, &one) == 196)
	var zero Event
	check(zero == Event.Idle)
	ctor := Event.Text
	check(classify(ctor("yes")) == 3 && trace == "G")
	check(classify(Event.Text("no")) == -1)
	trace = ""
	record := Event.Count{Ready: true, Number: mark("N", 4)}
	check(classify(record) == 4 && trace == "N")
	check(classify(Event.Count{}) == -2)
	var callback func() int
	switch record {
	case Event.Count{Number: n, ...} => {
		callback = func() int { return n }
		n++
	}
	default => {
		panic("record")
	}
	}
	check(callback() == 5 && classify(record) == 4)
	box := Box[Event].Full(record)
	check((switch box {
	case Box[Event].Empty => 0
	case Box[Event].Full(e) => classify(e)
	}) == 4)
	trace = ""
	check((option((int?)((int)(3))) ?? 0) == 4)
	check((option((int?)(nil)) ?? -1) == -1 && trace == "DD")
	p := ((*User)?)((*User)(nil))
	check((p ?? &User{Name: "fallback"}) == nil)
	check(((User?)((User)(User{Name: "Ada"}))?.Name ?? "absent") == "Ada")
	check(((User?)(nil)?.Name ?? "absent") == "absent")
	var x int?
	trace = ""
	x ??= mark("A", 7)
	x ??= mark("B", 8)
	check((x ?? 0) == 7 && trace == "A")
	println("PASS")
}
