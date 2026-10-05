package main

import "strconv"

func main() {
	untyped()
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

func validate(title string, n int) error {
	require(valid: len(title) >= 2, status: 422, code: "invalid_title", message: "title")!
	require(message: "missing", valid: affected(n)! != 0, code: "not_found", status: 404)!
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
	report("untyped less", describe(ok: a < b, label: "less", count: 1))
	report("untyped reordered", describe(count: 2, label: "reordered", ok: a >= b))
	report("untyped prefix", describe(a == b, count: 3, label: "prefix"))
	report("untyped logic", describe(label: "logic", ok: !(a == b) && (title != "" || a > b), count: 4))
	report("untyped strings", describe(count: 5, ok: title < "b", label: "strings"))
	report("untyped nil", describe(label: "nil", count: 6, ok: missing == nil && failure == nil))
	report("untyped constant", describe(ok: 1 == 1, count: 7, label: "constant"))
	report("untyped named bool", tagged(label: "named", ok: a != b))
	report("untyped any", anything(second: 'x', first: a < b))
	report("untyped method", receiver(3).Valid(label: "method", ok: a < b))
	var v validator = receiver(3)
	report("untyped interface", v.Valid(label: "interface", ok: a > b))
	report("untyped expression", receiver.Valid(label: "expression", ok: a < b, r: receiver(4)))
	var operation func(ok bool, label string) string = receiver(5).Valid
	report("untyped value", operation(label: "value", ok: a < b))
	report("untyped generic", choose(value: "text", ok: a < b))
	report("untyped generic explicit", choose[int](value: 9, ok: a > b))
	report("untyped variadic", listed(ok: a < b, extra: list...))
	report("untyped variadic empty", listed(ok: a > b))
	report("untyped constants", constants(err: nil, ptr: nil, text: 1 << 3, wide: 1 << 63, ratio: 1 << 2, name: "n" + "m", ch: 'a', small: 1 << 3))
	report("untyped order", describe(count: mark("C", 1), ok: mark("A", 1) < mark("B", 2), label: "order"))
	report("untyped short circuit", describe(label: "short", ok: markBool("X", false) && markBool("Y", true), count: mark("Z", 1)))
	func() {
		defer record(label: "deferred", ok: mark("B", 2) > mark("A", 1))
		report("untyped defer arguments", events)
	}()
	report("untyped defer ran", events)
	ch := make(chan string, 1)
	go func(ok bool, label string) { ch <- describe(ok, label, 0) }(label: "go", ok: a < b)
	report("untyped go", <-ch)
	report("untyped lambda", apply(fn: (n) => n < 3, value: 1))
	var check func(int) error = (n) => {
		require(valid: n < 3, status: 400, code: "lambda", message: "limit")!
		return nil
	}
	report("untyped lambda propagate", check(5))
	report("untyped conditional", describe(label: "conditional", count: if a < b { 1 } else { 2 }, ok: if a < b { a > 1 } else { false }))
	verdict := switch a < b {
	case true => "yes"
	case false => "no"
	}
	report("untyped match", verdict)
}
