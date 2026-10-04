package main

var events string

func mark(label string, value int) int    { events += label; return value }
func pair(first, second int) int          { return first*10 + second }
func target() func(first, second int) int { events += "F"; return pair }

type receiver int

func (r receiver) apply(first, second int) int { return int(r)*100 + pair(first, second) }
func recv() receiver                           { events += "R"; return 3 }

type applier interface{ apply(left, right int) int }

func generic[T any](first T, second []T) T { return first }
func variadic(prefix int, values ...int) int {
	if len(values) == 0 {
		return prefix
	}
	values[0] += prefix
	return values[0]
}
func check(ok bool) {
	if !ok {
		panic(events)
	}
}
func main() {
	events = ""
	f := target()
	b := mark("B", 2)
	a := mark("A", 1)
	check(f(a, b) == 12)
	check(events == "FBA")
	events = ""
	r := recv()
	b = mark("B", 2)
	a = mark("A", 1)
	check(r.apply(a, b) == 312)
	check(events == "RBA")
	var i applier = receiver(4)
	check(i.apply(5, 6) == 456)
	var operation func(left, right int) int = pair
	check(operation(7, 8) == 78)
	check(pair(9, 1) == 91)
	second := []int{2}
	first := 1
	check(generic(first, second) == 1)
	data := []int{1}
	check(variadic(2, data...) == 3)
	check(data[0] == 3)
	check(variadic(4) == 4)
	check(variadic(5, nil...) == 5)
	events = ""
	func() {
		defer func() { check(events == "FBA") }()
		f := target()
		b := mark("B", 2)
		a := mark("A", 1)
		defer f(a, b)
		check(events == "FBA")
	}()
	events = ""
	ch := make(chan int, 1)
	b = mark("B", 2)
	a = mark("A", 1)
	go func(first, second int) { ch <- pair(first, second) }(a, b)
	check(<-ch == 12)
	check(events == "BA")
	println("PASS")
}
