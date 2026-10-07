package main

import "fmt"

type State struct {
	kind int
	n    int
	name string
}
type Nested struct {
	some  bool
	value State
}
type Box struct{ N int }

func makeCount(n int) State            { return State{kind: 1, n: n} }
func makeRecord(s string, n int) State { return State{kind: 2, n: n, name: s} }
func makeNested(s State) Nested        { return Nested{some: true, value: s} }
func label(s State) string {
	switch s.kind {
	case 0:
		return "idle"
	case 1:
		return fmt.Sprintf("count:%d", s.n)
	case 2:
		return fmt.Sprintf("%s:%d", s.name, s.n)
	default:
		panic("bad")
	}
}
func nestedLabel(s Nested) string {
	if !s.some {
		return "empty"
	}
	switch s.value.kind {
	case 0:
		return "idle"
	case 1:
		return fmt.Sprintf("count:%d", s.value.n)
	case 2:
		return "record"
	default:
		panic("bad")
	}
}
func effects() int {
	before := side("before", 1)
	s := makeCount(side("tag", 2))
	var selected int
	if s.kind == 1 && guard("guard-one", false) {
		selected = side("bad-one", s.n)
	} else if s.kind == 1 && guard("guard-two", s.n == 2) {
		selected = side("selected", 3)
	} else {
		selected = side("bad-default", 0)
	}
	return before + selected + side("after", 5)
}
func escapeBinding() func() int {
	s := makeCount(7)
	if s.kind == 1 {
		n := s.n
		return func() int { return n }
	}
	return func() int { return 0 }
}
func copyBinding() int { x := Box{N: 8}; b := x; b.N = 9; return x.N }
func breakMatch(s State) int {
L:
	switch s.kind {
	case 0:
		mark("break")
		break L
	default:
		panic("bad")
	}
	return 3
}
func nilBranch(b bool) bool {
	var p *int
	var value any
	if b {
		value = p
	}
	return value == nil
}
func interfaceKind() string {
	var value any
	if false {
		value = float64(1)
	} else {
		value = 2.5
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
	var absent *int
	const item = 99
	true, false, nil := 10, 20, 30
	chosen := false
	if b {
		chosen = true
	}
	pointer := 0
	if p == absent {
		pointer = nil
	}
	flags := false
	if b {
		flags = true
	}
	bound := 0
	if s := makeCount(7); s.kind == 1 {
		item := s.n
		bound = item
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
		s := makeCount(i)
		value := i
		if s.kind == 1 && s.n == 2 {
			value, err = source(s.n, fail)
			if err != nil {
				total = 0
				return total, err
			}
		}
		total += value
	}
	return total, nil
}

func optionBoundary(ok bool) int {
	if !ok {
		return 0
	}
	return 4
}
