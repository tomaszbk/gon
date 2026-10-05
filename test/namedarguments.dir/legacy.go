package main

import "strconv"

func main() {
	untyped()
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

func validate(title string, n int) error {
	if err := require(len(title) >= 2, 422, "invalid_title", "title"); err != nil {
		return err
	}
	count, err := affected(n)
	if err != nil {
		return err
	}
	if err := require(count != 0, 404, "not_found", "missing"); err != nil {
		return err
	}
	return nil
}

func untyped() {
	a, b, title := 3, 4, "ab"
	var missing *int
	var failure error
	list := []int{1, 2}
	report("untyped validate ok", validate("ab", 1))
	report("untyped validate title", validate("a", 1))
	report("untyped validate missing", validate("ab", 0))
	report("untyped validate error", validate("ab", -1))
	report("untyped less", describe(a < b, "less", 1))
	report("untyped reordered", describe(a >= b, "reordered", 2))
	report("untyped prefix", describe(a == b, "prefix", 3))
	report("untyped logic", describe(!(a == b) && (title != "" || a > b), "logic", 4))
	report("untyped strings", describe(title < "b", "strings", 5))
	report("untyped nil", describe(missing == nil && failure == nil, "nil", 6))
	report("untyped constant", describe(1 == 1, "constant", 7))
	report("untyped named bool", tagged(a != b, "named"))
	report("untyped any", anything(a < b, 'x'))
	report("untyped method", receiver(3).Valid(a < b, "method"))
	var v validator = receiver(3)
	report("untyped interface", v.Valid(a > b, "interface"))
	report("untyped expression", receiver.Valid(receiver(4), a < b, "expression"))
	var operation func(ok bool, label string) string = receiver(5).Valid
	report("untyped value", operation(a < b, "value"))
	report("untyped generic", choose(a < b, "text"))
	report("untyped generic explicit", choose[int](a > b, 9))
	report("untyped variadic", listed(a < b, list...))
	report("untyped variadic empty", listed(a > b))
	report("untyped constants", constants(1<<3, 'a', "n"+"m", 1<<2, 1<<63, 1<<3, nil, nil))
	count := mark("C", 1)
	less := mark("A", 1) < mark("B", 2)
	report("untyped order", describe(less, "order", count))
	short := markBool("X", false) && markBool("Y", true)
	count = mark("Z", 1)
	report("untyped short circuit", describe(short, "short", count))
	func() {
		later := mark("B", 2) > mark("A", 1)
		defer record(later, "deferred")
		report("untyped defer arguments", events)
	}()
	report("untyped defer ran", events)
	ch := make(chan string, 1)
	go func(ok bool, label string) { ch <- describe(ok, label, 0) }(a < b, "go")
	report("untyped go", <-ch)
	report("untyped lambda", apply(1, func(n int) bool { return n < 3 }))
	check := func(n int) error {
		if err := require(n < 3, 400, "lambda", "limit"); err != nil {
			return err
		}
		return nil
	}
	report("untyped lambda propagate", check(5))
	choice, flagged := 2, false
	if a < b {
		choice, flagged = 1, a > 1
	}
	report("untyped conditional", describe(flagged, "conditional", choice))
	verdict := "no"
	if a < b {
		verdict = "yes"
	}
	report("untyped match", verdict)
}
