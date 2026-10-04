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
	check(target()(second: mark("B", 2), first: mark("A", 1)) == 12)
	check(events == "FBA")
	events = ""
	check(recv().apply(second: mark("B", 2), first: mark("A", 1)) == 312)
	check(events == "RBA")
	var i applier = receiver(4)
	check(i.apply(right: 6, left: 5) == 456)
	var operation func(left, right int) int = pair
	check(operation(right: 8, left: 7) == 78)
	check(pair(9, second: 1) == 91)
	check(generic(second: []int{2}, first: 1) == 1)
	data := []int{1}
	check(variadic(prefix: 2, values: data...) == 3)
	check(data[0] == 3)
	check(variadic(prefix: 4) == 4)
	check(variadic(prefix: 5, values: nil...) == 5)
	events = ""
	func() {
		defer func() { check(events == "FBA") }()
		defer target()(second: mark("B", 2), first: mark("A", 1))
		check(events == "FBA")
	}()
	events = ""
	ch := make(chan int, 1)
	go func(first, second int) { ch <- pair(first, second) }(second: mark("B", 2), first: mark("A", 1))
	check(<-ch == 12)
	check(events == "BA")
	println("PASS")
}
