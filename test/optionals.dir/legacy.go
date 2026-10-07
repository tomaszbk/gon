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
type Presence[T any] struct {
	Value   T
	Present bool
}

func isAbsent[T any](value Presence[T]) bool  { return !value.Present }
func isPresent[T any](value Presence[T]) bool { return value.Present }
func presenceFlag(value bool) bool            { return value }

func optionalNilComparisons() {
	var absent Presence[int]
	zero := Presence[int]{0, true}
	sentinel := Presence[int]{-1, true}
	empty := Presence[string]{"", true}
	falsy := Presence[bool]{false, true}
	check("nil absence", !absent.Present && !absent.Present && !absent.Present, true, "")
	check("nil zero payloads", zero.Present && sentinel.Present && empty.Present && falsy.Present, true, "")
	check("nil named comparison", presenceFlag(zero.Present), true, "")
	{
		nil := sentinel
		check("nil shadowed optional", zero == nil || sentinel != nil, false, "")
	}
	slice := Presence[[]int]{nil, true}
	mapping := Presence[map[string]int]{nil, true}
	function := Presence[func()]{nil, true}
	pointer := Presence[*int]{nil, true}
	iface := Presence[any]{nil, true}
	check("nil present payloads", slice.Present && mapping.Present && function.Present && pointer.Present && iface.Present, true, "")
	iface = Presence[any]{[]int{1}, true}
	check("nil dynamic noncomparable payload", iface.Present, true, "")
	slice = Presence[[]int]{}
	check("nil noncomparable absence", !slice.Present && !isPresent(slice), true, "")
	nested := Presence[Presence[int]]{Presence[int]{}, true}
	check("nil nested presence", nested.Present && !nested.Value.Present, true, "")
	nested = Presence[Presence[int]]{}
	check("nil generic aliases", isAbsent(nested) && isPresent(zero) && isAbsent(slice), true, "")
	produce := func(label string, present bool) Presence[int] {
		effects += label
		return Presence[int]{0, present}
	}
	check("nil evaluation", !produce("A", false).Present && produce("B", true).Present, true, "AB")
	check("nil short circuit", !produce("C", true).Present && !produce("X", false).Present, false, "C")
	check("nil short circuit or", produce("D", true).Present || !produce("X", false).Present, true, "D")
	items := []Presence[int]{{}, {0, true}}
	check("nil indexed evaluation", items[mark("I", 1)].Present, true, "I")
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
func rangeOption() IntOption {
	for n := range iter {
		if n == 2 {
			return IntOption{}
		}
		effects += fmt.Sprint(n)
	}
	return IntOption{7, true}
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

func main() {
	optionalNilComparisons()
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
	lift := func(input IntOption) IntOption {
		if !input.Present {
			return IntOption{}
		}
		return IntOption{input.Value + 1, true}
	}
	emit("lambda Some", optionValue(lift(IntOption{2, true}), 0))
	emit("lambda None", optionValue(lift(IntOption{}), 0))
	emit("range Option", optionValue(rangeOption(), 0))
	var boxed any = StringOption{}
	emit("boxed None", boxed != nil)
	optionOptimizationCases()
}
