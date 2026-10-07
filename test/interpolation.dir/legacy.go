package main

import "fmt"

func propagation(fail bool) (out string, err error) {
	out = "stale"
	defer func() { trace += "D" + out }()
	first := mark("A", 1)
	value, problem := source(fail)
	if problem != nil {
		return "", problem
	}
	return fmt.Sprintf("%v:%v:%04d", first, value, mark("B", 2)), nil
}
func scenario() string {
	x := 3
	price := 2.5
	first := mark("C", x)
	right, left := mark("R", 2), mark("L", 1)
	choice := "no"
	if x > 0 {
		choice = "yes"
	}
	return fmt.Sprintf("{ } $ 50%% %v %.2f %v %v %v %v %v %[8]d %[9]d ${literal} %q", first, price, call(left, right), map[string]int{"key": 5}["key"], []int{1, 2, 3}[1:], choice, fmt.Sprintf("nested:%v", x), mark("I", 11), mark("J", 22), "line\n")
}
func shadowed() string  { alias := 5; return fmt.Sprintf("shadow:%v", alias) }
func multiline() string { return fmt.Sprintf("raw\n\t{\"empty\": []} %v %v", "${", 3) }
func empty() string     { return fmt.Sprintf("") }

func ordered() string {
	n := 1
	values := []int{4}
	mutate := func() int { n = 2; values[0] = 5; return 3 }
	first, second := n, values[0]
	third := mutate()
	fourth, fifth := n, values[0]
	return fmt.Sprintf("%v:%v:%v:%v:%v", first, second, third, fourth, fifth)
}

func safeCalls() string {
	var callback func(int) int
	first := 0
	if callback != nil {
		first = callback(mark("skip", 3))
	}
	callback = func(n int) int { return n + 1 }
	second := callback(mark("safe", 3))
	return fmt.Sprintf("%03d/%03d", first, second)
}

func blocks(fail bool) (string, error) {
	choice := 1
	if fail {
		choice = 2
	}
	value, err := source(fail)
	if err != nil {
		return "", err
	}
	last := func() int {
		var (
			n = 1
			m = 2
		)
		type S struct {
			A int
			B int
		}
		_ = S{}
		switch n {
		case 1:
			n += m
		default:
			n = 0
		}
		return n
	}()
	return fmt.Sprintf("%v:%v:%v", choice, value, last), nil
}
func rawBlocks() string {
	value := func() int {
		n := 1
		switch n {
		case 1:
			n++
		default:
			n--
		}
		return n
	}()
	return fmt.Sprintf("raw\n%v %v\nend", 1, value)
}

//line interpolation-generated.go:1
func captured() string {
	offset := 1
	f := func(n int) string { return fmt.Sprintf("%v", n+offset) }
	offset = 2
	return f(3)
}
