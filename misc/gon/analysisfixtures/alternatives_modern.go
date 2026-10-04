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
func result(v Result[int, error]) Result[int, error] {
	n := v!
	return Result[int, error].Ok(n + 2)
}
func unwrap(v Result[int, error]) int {
	return v or err {
		check(err == nil)
		return -1
	}
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
func main() {
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
	check(unwrap(result(Result[int, error].Ok(3))) == 5)
	check(unwrap(result(Result[int, error].Err(nil))) == -1)
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
