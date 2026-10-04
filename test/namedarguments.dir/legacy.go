package main

import "strconv"

func main() {
	f := getFunction()
	b := mark("B", 2)
	a := mark("A", 1)
	report("function", f(a, b))
	r := getReceiver()
	b = mark("B", 2)
	a = mark("A", 1)
	report("method", r.Apply(a, b))
	var i applier = receiver(3)
	b = mark("B", 2)
	a = mark("A", 1)
	report("interface", i.Apply(a, b))
	var operation func(left, right int) int = pair
	b = mark("B", 2)
	a = mark("A", 1)
	report("static", operation(a, b))
	report("method expression", receiver.Apply(receiver(3), 1, 2))
	report("mixed", pair(mark("A", 1), mark("B", 2)))
	report("generic", generic(3, []int{4}))
	data := []int{1, 2}
	report("variadic", variadic(4, data...))
	report("alias", data[0])
	report("empty", variadic(7))
	report("nil", variadic(8, nil...))
	report("target", callback(3, func(n int) int { return n * 2 }))
	report("multiple", pair(values()))
	n, err := strconv.Atoi("123")
	if err != nil {
		panic(err)
	}
	report("import", n)
	defer func() { report("defer result", events) }()
	f = getFunction()
	b = mark("B", 2)
	a = mark("A", 1)
	defer pair(a, b)
	report("defer arguments", events)
	ch := make(chan int, 1)
	f = getFunction()
	b = mark("B", 2)
	a = mark("A", 1)
	go func(x, y int) { ch <- f(x, y) }(a, b)
	report("go", <-ch)
}
