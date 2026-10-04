package main

type Event struct {
	tag    int
	text   string
	number int
	ready  bool
}
type User struct{ Name string }
type Box[T any] struct {
	full  bool
	value T
}
type IntOption struct {
	present bool
	value   int
}
type IntResult struct {
	failed bool
	value  int
	err    error
}

var trace string

func mark(s string, n int) int { trace += s; return n }
func check(ok bool) {
	if !ok {
		panic("alternatives mismatch")
	}
}
func classify(v Event) int {
	switch v.tag {
	case 0:
		return 0
	case 1:
		if v.text == "yes" {
			return mark("G", len(v.text))
		}
		return -1
	case 2:
		if v.ready {
			return v.number
		}
		return -2
	}
	panic("invalid")
}
func option(v IntOption) (out IntOption) {
	out = IntOption{true, 99}
	defer func() { trace += "D" }()
	if !v.present {
		return IntOption{}
	}
	return IntOption{true, v.value + 1}
}
func result(v IntResult) IntResult {
	if v.failed {
		return IntResult{failed: true, err: v.err}
	}
	return IntResult{value: v.value + 2}
}
func unwrap(v IntResult) int {
	if v.failed {
		check(v.err == nil)
		return -1
	}
	return v.value
}
func coalesce(v IntOption, fallback int) int {
	if v.present {
		return v.value
	}
	return fallback
}
func patternNames(b bool, p *int) int {
	var absent *int
	const item = 99
	true, false, nil := 10, 20, 30
	chosen, flags := false, false
	if b {
		chosen, flags = true, true
	}
	pointer := 0
	if p == absent {
		pointer = nil
	}
	bound := 0
	if event := (Event{tag: 2, number: 7}); event.tag == 2 {
		item := event.number
		bound = item
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
	check(zero == Event{})
	ctor := func(s string) Event { return Event{tag: 1, text: s} }
	check(classify(ctor("yes")) == 3 && trace == "G")
	check(classify(ctor("no")) == -1)
	trace = ""
	record := Event{tag: 2, ready: true, number: mark("N", 4)}
	check(classify(record) == 4 && trace == "N")
	check(classify(Event{tag: 2}) == -2)
	var callback func() int
	switch record.tag {
	case 2:
		n := record.number
		callback = func() int { return n }
		n++
	default:
		panic("record")
	}
	check(callback() == 5 && classify(record) == 4)
	box := Box[Event]{true, record}
	check(func() int {
		if box.full {
			return classify(box.value)
		}
		return 0
	}() == 4)
	trace = ""
	check(coalesce(option(IntOption{true, 3}), 0) == 4)
	check(coalesce(option(IntOption{}), -1) == -1 && trace == "DD")
	check(unwrap(result(IntResult{value: 3})) == 5)
	check(unwrap(result(IntResult{failed: true, err: nil})) == -1)
	p := struct {
		present bool
		value   *User
	}{true, nil}
	check(func() *User {
		if p.present {
			return p.value
		}
		return &User{Name: "fallback"}
	}() == nil)
	u := struct {
		present bool
		value   User
	}{true, User{Name: "Ada"}}
	check(func() string {
		if u.present {
			return u.value.Name
		}
		return "absent"
	}() == "Ada")
	u.present = false
	check(func() string {
		if u.present {
			return u.value.Name
		}
		return "absent"
	}() == "absent")
	var x IntOption
	trace = ""
	if !x.present {
		x = IntOption{true, mark("A", 7)}
	}
	if !x.present {
		x = IntOption{true, mark("B", 8)}
	}
	check(coalesce(x, 0) == 7 && trace == "A")
	println("PASS")
}
