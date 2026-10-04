package main

import "fmt"

type IntOption struct {
	Value   int
	Present bool
}
type StringOption struct {
	Value   string
	Present bool
}
type IntResult struct {
	Value   int
	Problem error
	Failed  bool
}
type StringResult struct {
	Value   string
	Problem error
	Failed  bool
}

func maybe(present bool) IntOption { effects += "M"; return IntOption{3, present} }
func propagate(present bool) (out StringOption) {
	out = StringOption{"stale", true}
	defer func() {
		label := "none"
		if out.Present {
			label = out.Value
		}
		effects += "D" + label
	}()
	o := maybe(present)
	if !o.Present {
		return StringOption{}
	}
	return StringOption{fmt.Sprint(o.Value), true}
}
func getResult(failed bool) IntResult { effects += "R"; return IntResult{4, nil, failed} }
func resultLabel(input StringResult) string {
	if input.Failed {
		return fmt.Sprintf("error:%v", input.Problem)
	}
	return input.Value
}
func resultPropagate(failed bool) (out StringResult) {
	out = StringResult{Value: "stale"}
	defer func() { effects += "D" + resultLabel(out) }()
	r := getResult(failed)
	if r.Failed {
		return StringResult{Problem: r.Problem, Failed: true}
	}
	return StringResult{Value: fmt.Sprint(r.Value * 2)}
}
func rangeOption() IntOption {
	for n := range iter {
		if n == 2 {
			return IntOption{}
		}
		effects += fmt.Sprint(n)
	}
	return IntOption{7, true}
}
func rangeResult() IntResult {
	for n := range iter {
		if n == 2 {
			return IntResult{Failed: true}
		}
		effects += fmt.Sprint(n)
	}
	return IntResult{Value: 7}
}
func bridge(failed bool) (int, error) {
	r := getResult(failed)
	if r.Failed {
		problem := r.Problem
		if problem == nil {
			problem = fmt.Errorf("empty failure")
		}
		return 0, problem
	}
	return r.Value, nil
}
func optionValue(x IntOption, fallbackValue int) int {
	if x.Present {
		return x.Value
	}
	return fallbackValue
}

func optionOptimizationCases() {
	slots := []IntOption{{}, {Value: 0, Present: true}, {Value: 8, Present: true}}
	sum := 0
	for i := range slots {
		if !slots[i].Present {
			slots[i] = IntOption{mark("A", 7), true}
		}
		if slots[i].Present {
			sum += slots[i].Value
		} else {
			sum += fallback()
		}
	}
	check("assign then read slots", sum, 15, "A")

	slots = []IntOption{{}, {}}
	alias := &slots[0]
	if !slots[0].Present {
		slots[0] = IntOption{func() int {
			effects += "R"
			*alias = IntOption{99, true}
			slots[1] = IntOption{13, true}
			return 5
		}(), true}
	}
	check("assign aliases in default", optionValue(*alias, 9)+optionValue(slots[1], 9), 18, "R")
	*alias = IntOption{}
	value := alias.Value
	if !alias.Present {
		value = fallback()
	}
	check("assign alias invalidation", value, 9, "F")

	left, right := IntOption{}, IntOption{}
	pointer := &left
	location := pointer
	if !location.Present {
		*location = IntOption{func() int {
			effects += "R"
			pointer = &right
			return 3
		}(), true}
	}
	value = pointer.Value
	if !pointer.Present {
		value = fallback()
	}
	check("assign rebind pointer", fmt.Sprintf("%d:%d", left.Value, value), "3:9", "RF")

	slots = []IntOption{{}}
	before := slots
	location = &slots[0]
	if !location.Present {
		*location = IntOption{func() int {
			effects += "R"
			slots = []IntOption{{}}
			return 4
		}(), true}
	}
	value = slots[0].Value
	if !slots[0].Present {
		value = fallback()
	}
	check("assign rebind slice", fmt.Sprintf("%d:%d", before[0].Value, value), "4:9", "RF")

	m := map[int]IntOption{}
	key := func() int { effects += "K"; return 2 }
	k := key()
	if !m[k].Present {
		m[k] = IntOption{func() int {
			effects += "I"
			for i := 3; i < 67; i++ {
				m[i] = IntOption{i * 4, true}
			}
			return 5
		}(), true}
	}
	check("assign map insertion", fmt.Sprintf("%d:%d", optionValue(m[2], 9)+optionValue(m[3], 9), len(m)), "17:65", "KI")

	var optional struct {
		Value   *User
		Present bool
	}
	if !optional.Present {
		optional.Value = func() *User { effects += "N"; return nil }()
		optional.Present = true
	}
	var user *User
	if optional.Present {
		user = optional.Value
	} else {
		effects += "F"
		user = &User{Name: "fallback"}
	}
	check("assign present nil", user == nil, true, "N")

	var missing *IntOption
	check("assign nil target", panics(func() {
		if !missing.Present {
			*missing = IntOption{mark("F", 5), true}
		}
	}), true, "")
	slots = nil
	check("assign index out of bounds", panics(func() {
		if !slots[0].Present {
			slots[0] = IntOption{mark("F", 5), true}
		}
	}), true, "")
}

type PointerResult struct {
	Value   *int
	Problem error
	Failed  bool
}

func conditionalResult(ok bool, payload *int) PointerResult {
	effects += "C"
	if ok {
		effects += "O"
		return PointerResult{Value: payload}
	}
	effects += "E"
	return PointerResult{Failed: true}
}

func handleConditionalResult(ok bool, payload *int) (out string) {
	defer func() { effects += "D" + out }()
	r := conditionalResult(ok, payload)
	if r.Failed {
		effects += "H"
		if r.Problem != nil {
			panic(r.Problem)
		}
		*payload += 10
		return "failure"
	}
	pointer := r.Value
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
	if r.Failed || copy.Failed {
		panic("unexpected failure")
	}
	*r.Value = 11
	check("Result copied pointer payload", fmt.Sprintf("%d:%d", *r.Value, *copy.Value), "11:11", "CO")
}

func main() {
	var zero IntOption
	z := zero.Value
	if !zero.Present {
		z = fallback()
	}
	emit("zero Option", z)
	if !zero.Present {
		zero = IntOption{mark("A", 2), true}
	}
	if !zero.Present {
		zero = IntOption{fallback(), true}
	}
	emit("assign Some", optionValue(zero, 9))
	emit("Some zero", 0)
	var ptr *User
	emit("Some nil", ptr == nil)
	emit("Some nil panic", panics(func() { _ = ptr.Name }))
	for _, input := range []struct {
		present bool
		pointer *User
	}{{}, {true, nil}, {true, &User{Name: "Ada"}}} {
		var pointer *User
		if input.present {
			pointer = input.pointer
		}
		name := "anonymous"
		if pointer != nil {
			name = pointer.Name
		}
		emit("explicit Option nil", name)
	}

	locations := 0
	m := map[int]IntOption{}
	key := func() int { locations++; return 2 }
	k := key()
	if !m[k].Present {
		m[k] = IntOption{mark("A", 5), true}
	}
	k = key()
	if !m[k].Present {
		m[k] = IntOption{fallback(), true}
	}
	emit("map assign", fmt.Sprintf("%d:%d", m[2].Value, locations))
	var domain any = "domain"
	emit("typed domain", fmt.Sprintf("%T:%v", domain, domain))

	emit("Some user", User{Name: "Ada"}.Name)
	emit("None user", fallback())
	nested := IntOption{}
	emit("nested", optionValue(nested, 7))
	emit("nested nav", optionValue(IntOption{}, 7))
	emit("Some nil nav", true)
	emit("None callback", fallback())
	callback := func(n int) int { return mark("C", n) }
	emit("Some callback", callback(mark("A", 3)))

	for _, present := range []bool{true, false} {
		o := propagate(present)
		label := "none"
		if o.Present {
			label = o.Value
		}
		emit("propagate", label)
	}
	var result IntResult
	emit("zero Result", resultLabel(StringResult{Value: fmt.Sprint(result.Value)}))
	for _, failed := range []bool{false, true} {
		emit("Result propagate", resultLabel(resultPropagate(failed)))
	}
	lift := func(input IntOption) IntOption {
		if !input.Present {
			return IntOption{}
		}
		return IntOption{input.Value + 1, true}
	}
	emit("lambda Some", optionValue(lift(IntOption{2, true}), 0))
	emit("lambda None", optionValue(lift(IntOption{}), 0))
	emit("range Option", optionValue(rangeOption(), 0))
	r := rangeResult()
	out := StringResult{Value: fmt.Sprint(r.Value), Problem: r.Problem, Failed: r.Failed}
	emit("range Result", resultLabel(out))
	for _, failed := range []bool{false, true} {
		n, err := bridge(failed)
		emit("bridge", fmt.Sprintf("%d:%v", n, err))
	}
	var boxed any = StringOption{}
	emit("boxed None", boxed != nil)
	optionOptimizationCases()
	resultOptimizationCases()
}
