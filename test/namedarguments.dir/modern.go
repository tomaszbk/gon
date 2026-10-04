package main

import "strconv"

func main() {
	report("function", getFunction()(second: mark("B", 2), first: mark("A", 1)))
	report("method", getReceiver().Apply(second: mark("B", 2), first: mark("A", 1)))
	var i applier = receiver(3)
	report("interface", i.Apply(right: mark("B", 2), left: mark("A", 1)))
	var operation func(left, right int) int = pair
	report("static", operation(right: mark("B", 2), left: mark("A", 1)))
	report("method expression", receiver.Apply(second: 2, r: receiver(3), first: 1))
	report("mixed", pair(mark("A", 1), second: mark("B", 2)))
	report("generic", generic(second: []int{4}, first: 3))
	data := []int{1, 2}
	report("variadic", variadic(prefix: 4, values: data...))
	report("alias", data[0])
	report("empty", variadic(prefix: 7))
	report("nil", variadic(prefix: 8, values: nil...))
	report("target", callback(fn: (n) => n * 2, first: 3))
	report("multiple", pair(values()))
	n, err := strconv.Atoi(s: "123")
	if err != nil {
		panic(err)
	}
	report("import", n)
	defer func() { report("defer result", events) }()
	defer getFunction()(second: mark("B", 2), first: mark("A", 1))
	report("defer arguments", events)
	ch := make(chan int, 1)
	f := getFunction()
	go func(x, y int) { ch <- f(x, y) }(y: mark("B", 2), x: mark("A", 1))
	report("go", <-ch)
}
