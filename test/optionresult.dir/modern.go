package main

import "fmt"

type IntOption = int?
type Holder struct {
	Inner   IntOption
	Pointer *User
}
type IntResult = Result[int, error]
type Presence[T any] = T?

func isAbsent[T any](value Presence[T]) bool { return value == nil }
func isPresent[T any](value T?) bool         { return nil != value }
func presenceFlag(value bool) bool           { return value }

func optionalNilComparisons() {
	var absent int?
	var zero int? = 0
	var sentinel int? = -1
	var empty string? = ""
	var falsy bool? = false
	check("nil absence", absent == nil && nil == absent && !(absent != nil), true, "")
	check("nil zero payloads", zero != nil && sentinel != nil && empty != nil && falsy != nil, true, "")
	check("nil named comparison", presenceFlag(value: zero != nil), true, "")
	{
		nil := sentinel
		check("nil shadowed optional", zero == nil || sentinel != nil, false, "")
	}
	var slice ([]int)? = ([]int)(nil)
	var mapping (map[string]int)? = (map[string]int)(nil)
	var function (func())? = (func())(nil)
	var pointer (*int)? = (*int)(nil)
	var iface any? = (any)(nil)
	check("nil present payloads", slice != nil && mapping != nil && function != nil && pointer != nil && iface != nil, true, "")
	iface = any([]int{1})
	check("nil dynamic noncomparable payload", iface != nil, true, "")
	slice = nil
	check("nil noncomparable absence", nil == slice && !isPresent(slice), true, "")
	var nested (int?)? = (int?)(nil)
	check("nil nested presence", nested != nil && (nested ?? (int?)(1)) == nil, true, "")
	nested = nil
	check("nil generic aliases", isAbsent(nested) && isPresent(zero) && isAbsent(slice), true, "")
	produce := func(label string, present bool) int? {
		effects += label
		if present {
			return 0
		}
		return nil
	}
	check("nil evaluation", produce("A", false) == nil && nil != produce("B", true), true, "AB")
	check("nil short circuit", produce("C", true) == nil && produce("X", false) == nil, false, "C")
	check("nil short circuit or", produce("D", true) != nil || produce("X", false) == nil, true, "D")
	items := []int?{nil, 0}
	check("nil indexed evaluation", items[mark("I", 1)] != nil, true, "I")
}

func maybe(present bool) IntOption {
	effects += "M"
	if !present {
		return (IntOption)(nil)
	}
	return (IntOption)((int)(3))
}
func propagate(present bool) (out string?) {
	out = (string?)((string)("stale"))
	defer func() { effects += "D" + (out ?? "none") }()
	n := maybe(present)?
	return (string?)((string)(fmt.Sprint(n)))
}
func getResult(failed bool) IntResult {
	effects += "R"
	if failed {
		return IntResult.Err(nil)
	}
	return IntResult.Ok(4)
}
func resultLabel(input Result[string, error]) string {
	return input or problem {
		return fmt.Sprintf("error:%v", problem)
	}
}
func resultPropagate(failed bool) (out Result[string, error]) {
	out = Result[string, error].Ok("stale")
	defer func() { effects += "D" + resultLabel(out) }()
	n := getResult(failed)!
	return Result[string, error].Ok(fmt.Sprint(n * 2))
}
func rangeOption() IntOption {
	for n := range iter {
		if n == 2 {
			_ = (IntOption)(nil)?
		}
		effects += fmt.Sprint(n)
	}
	return (IntOption)((int)(7))
}
func rangeResult() IntResult {
	for n := range iter {
		if n == 2 {
			_ = IntResult.Err(nil)!
		}
		effects += fmt.Sprint(n)
	}
	return IntResult.Ok(7)
}
func bridge(failed bool) (int, error) {
	n := getResult(failed) or problem {
		if problem == nil {
			problem = fmt.Errorf("empty failure")
		}
		return 0, problem
	}
	return n, nil
}
func adapt(input Result[int, string]) Result[int, any] { n := input!; return Result[int, any].Ok(n) }
func domainLabel(input Result[int, any]) string {
	n := input or problem {
		return fmt.Sprintf("%T:%v", problem, problem)
	}
	return fmt.Sprint(n)
}

func optionOptimizationCases() {
	slots := []IntOption{(IntOption)(nil), (IntOption)((int)(0)), (IntOption)((int)(8))}
	sum := 0
	for i := range slots {
		slots[i] ??= mark("A", 7)
		sum += slots[i] ?? fallback()
	}
	check("assign then read slots", sum, 15, "A")

	slots = []IntOption{(IntOption)(nil), (IntOption)(nil)}
	alias := &slots[0]
	slots[0] ??= func() int {
		effects += "R"
		*alias = (IntOption)((int)(99))
		slots[1] = (IntOption)((int)(13))
		return 5
	}()
	check("assign aliases in default", (*alias ?? 9)+(slots[1] ?? 9), 18, "R")
	*alias = (IntOption)(nil)
	check("assign alias invalidation", *alias ?? fallback(), 9, "F")

	left, right := (IntOption)(nil), (IntOption)(nil)
	pointer := &left
	*pointer ??= func() int {
		effects += "R"
		pointer = &right
		return 3
	}()
	check("assign rebind pointer", fmt.Sprintf("%d:%d", left ?? 9, *pointer ?? fallback()), "3:9", "RF")

	slots = []IntOption{(IntOption)(nil)}
	before := slots
	slots[0] ??= func() int {
		effects += "R"
		slots = []IntOption{(IntOption)(nil)}
		return 4
	}()
	check("assign rebind slice", fmt.Sprintf("%d:%d", before[0] ?? 9, slots[0] ?? fallback()), "4:9", "RF")

	m := map[int]IntOption{}
	key := func() int { effects += "K"; return 2 }
	m[key()] ??= func() int {
		effects += "I"
		for i := 3; i < 67; i++ {
			m[i] = (IntOption)((int)(i * 4))
		}
		return 5
	}()
	check("assign map insertion", fmt.Sprintf("%d:%d", (m[2] ?? 9)+(m[3] ?? 9), len(m)), "17:65", "KI")

	var optional (*User)?
	optional ??= func() *User { effects += "N"; return nil }()
	user := optional ?? func() *User { effects += "F"; return &User{Name: "fallback"} }()
	check("assign present nil", user == nil, true, "N")

	var missing *IntOption
	check("assign nil target", panics(func() { *missing ??= mark("F", 5) }), true, "")
	slots = nil
	check("assign index out of bounds", panics(func() { slots[0] ??= mark("F", 5) }), true, "")
}

func conditionalResult(ok bool, payload *int) Result[*int, error] {
	effects += "C"
	if ok {
		effects += "O"
		return Result[*int, error].Ok(payload)
	}
	effects += "E"
	return Result[*int, error].Err(nil)
}

func handleConditionalResult(ok bool, payload *int) (out string) {
	defer func() { effects += "D" + out }()
	pointer := conditionalResult(ok, payload) or problem {
		effects += "H"
		if problem != nil {
			panic(problem)
		}
		*payload += 10
		return "failure"
	}
	if pointer == nil {
		effects += "N"
		return "nil"
	}
	effects += "S"
	*pointer += 1
	return fmt.Sprint(*pointer)
}

func resultOptimizationCases() {
	for _, ok := range []bool{true, false} {
		payload := 7
		label := handleConditionalResult(ok, &payload)
		if ok {
			check("conditional Result handler", fmt.Sprintf("%s:%d", label, payload), "8:8", "COSD8")
		} else {
			check("conditional Result handler", fmt.Sprintf("%s:%d", label, payload), "failure:17", "CEHDfailure")
		}
	}
	check("conditional Result Ok nil", handleConditionalResult(true, nil), "nil", "CONDnil")

	payload := 7
	r := conditionalResult(true, &payload)
	copy := r
	payload = 9
	pointer := r or problem {
		panic(problem)
	}
	copiedPointer := copy or problem {
		panic(problem)
	}
	*pointer = 11
	check("Result copied pointer payload", fmt.Sprintf("%d:%d", *pointer, *copiedPointer), "11:11", "CO")
}

func main() {
	optionalNilComparisons()
	var zero IntOption
	emit("zero Option", zero ?? fallback())
	zero ??= mark("A", 2)
	zero ??= fallback()
	emit("assign Some", zero ?? 9)
	emit("Some zero", (IntOption)((int)(0)) ?? fallback())
	var p (*User)? = ((*User)?)((*User)(nil))
	emit("Some nil", (p ?? &User{Name: "fallback"}) == nil)
	emit("Some nil panic", panics(func() { _ = p?.Name }))
	for _, input := range [](*User)?{((*User)?)(nil), p, ((*User)?)((*User)(&User{Name: "Ada"}))} {
		pointer := input ?? nil
		emit("explicit Option nil", pointer?.Name ?? "anonymous")
	}

	locations := 0
	m := map[int]IntOption{}
	key := func() int { locations++; return 2 }
	m[key()] ??= mark("A", 5)
	m[key()] ??= fallback()
	emit("map assign", fmt.Sprintf("%d:%d", m[2] ?? 0, locations))
	emit("typed domain", domainLabel(adapt(Result[int, string].Err("domain"))))

	emit("Some user", (User?)((User)(User{Name: "Ada"}))?.Name ?? "none")
	emit("None user", (User?)(nil)?.Len() ?? fallback())
	nested := ((int?)?)((int?)((IntOption)(nil)))
	emit("nested", (nested ?? (IntOption)((int)(8))) ?? 7)
	holder := (Holder?)((Holder)(Holder{Inner: (IntOption)(nil)}))
	nestedNav := holder?.Inner
	emit("nested nav", (nestedNav ?? (IntOption)((int)(8))) ?? 7)
	emit("Some nil nav", (holder?.Pointer ?? &User{Name: "bad"}) == nil)
	noneCallback := ((func(int) int)?)(nil)
	emit("None callback", noneCallback?(mark("A", 3)) ?? fallback())
	someCallback := ((func(int) int)?)((func(int) int)(func(n int) int { return mark("C", n) }))
	emit("Some callback", someCallback?(mark("A", 3)) ?? fallback())

	for _, present := range []bool{true, false} {
		emit("propagate", propagate(present) ?? "none")
	}
	var result IntResult
	emit("zero Result", func(input IntResult) int {
		return input or problem {
			panic(problem)
		}
	}(result))
	for _, failed := range []bool{false, true} {
		emit("Result propagate", resultLabel(resultPropagate(failed)))
	}
	var lift func(int?) int? = (input) => { n := input?; return (IntOption)((int)(n + 1)) }
	emit("lambda Some", lift((IntOption)((int)(2))) ?? 0)
	emit("lambda None", lift((IntOption)(nil)) ?? 0)
	emit("range Option", rangeOption() ?? 0)
	emit("range Result", resultLabel(func() Result[string, error] { n := rangeResult()!; return Result[string, error].Ok(fmt.Sprint(n)) }()))
	for _, failed := range []bool{false, true} {
		n, err := bridge(failed)
		emit("bridge", fmt.Sprintf("%d:%v", n, err))
	}
	var boxed any = (User?)(nil)?.Name
	emit("boxed None", boxed != nil)
	optionOptimizationCases()
	resultOptimizationCases()
}
