package main

import "fmt"

type State enum {
	Count(int)
	default Idle
	Record  {
		Name string
		N    int
	}
}
type Nested enum {
	default Empty
	Item(State)
}
type Box struct{ N int }
type Boxes enum {
	default Empty
	Value(Box)
}

func makeCount(n int) State            { return State.Count(n) }
func makeRecord(s string, n int) State { return State.Record{Name: s, N: n} }
func makeNested(s State) Nested        { return Nested.Item(s) }
func label(s State) string {
	return switch s {
	case State.Idle => "idle"
	case State.Count(n) => fmt.Sprintf("count:%d", n)
	case State.Record{Name: name, N: n} => fmt.Sprintf("%s:%d", name, n)
	}
}
func nestedLabel(s Nested) string {
	return switch s {
	case Nested.Empty => "empty"
	case Nested.Item(State.Idle) => "idle"
	case Nested.Item(State.Count(n)) => fmt.Sprintf("count:%d", n)
	case Nested.Item(State.Record{...}) => "record"
	}
}
func effects() int {
	return side("before", 1) + switch makeCount(side("tag", 2)) {
	case State.Count(n) if guard("guard-one", false) => side("bad-one", n)
	case State.Count(n) if guard("guard-two", n == 2) => side("selected", 3)
	default => side("bad-default", 0)
	} + side("after", 5)
}
func escapeBinding() func() int {
	var f func() int = switch makeCount(7) {
	case State.Count(n) => () => n
	default => () => 0
	}
	return f
}
func copyBinding() int {
	x := Boxes.Value(Box{N: 8})
	switch x {
	case Boxes.Value(b) => {
		b.N = 9
	}
	default => {
	}
	}
	return switch x {
	case Boxes.Value(b) => b.N
	default => 0
	}
}
func breakMatch(s State) int {
L:
	switch s {
	case State.Idle => {
		mark("break")
		break L
	}
	default => {
		panic("bad")
	}
	}
	return 3
}
func nilBranch(b bool) bool {
	var p *int
	var value any = switch b {
	case true => p
	case false => nil
	}
	return value == nil
}
func interfaceKind() string {
	var value any = switch false {
	case true => 1
	case false => 2.5
	}
	return fmt.Sprintf("%T", value)
}
func classicSwitch() int {
	enum := 5
	switch enum {
	case 1, 2:
		return 0
	case 5:
		return 5
	default:
		return 3
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
	bound := switch makeCount(7) {
	case State.Count(item) => item
	default => 0
	}
	ordinary := 0
	switch true {
	case 10:
		ordinary = false + nil
	}
	return chosen + pointer + flags + bound + ordinary + item
}
func iterate(fail bool) (total int, err error) {
	defer func() { mark(fmt.Sprintf("defer:%d:%v", total, err)) }()
	for i := range sequence {
		total += switch makeCount(i) {
		case State.Count(n) if n == 2 => source(n, fail)!
		default => i
		}
	}
	return total, nil
}

func optionArm(ok bool) int? {
	return (int?)((int)(switch ok {
	case true => (int?)((int)(4))?
	case false => (int?)(nil)?
	}))
}
func optionBoundary(ok bool) int {
	return switch optionArm(ok) {
	case (value)? => value
	case nil => 0
	}
}
func resultArm(ok bool) Result[int, string] {
	return Result[int, string].Ok(switch ok {
	case true => Result[int, string].Ok(8)!
	case false => Result[int, string].Err("failure")!
	})
}
func resultBoundary(ok bool) string {
	return switch resultArm(ok) {
	case Result[int, string].Ok(value) => fmt.Sprint(value)
	case Result[int, string].Err(problem) => problem
	}
}
