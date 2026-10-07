package main

import (
	"fmt"
	"iter"
	"reflect"
	"slices"
)

type ints []int
type pointers map[string]*int

func equal(got, want any) {
	if !reflect.DeepEqual(got, want) {
		panic(fmt.Sprintf("got %#v, want %#v", got, want))
	}
}

func main() {
	input := ints{3, 1, 2, 1}
	var trace []int
	record := func(n int) int {
		trace = append(trace, n)
		return n
	}
	equal(transform(input, func(n int) string { return fmt.Sprint(record(n)) }), []string{"3", "1", "2", "1"})
	equal(trace, []int{3, 1, 2, 1})
	trace = nil
	filtered := filter(input, func(n int) bool { return record(n)%2 != 0 })
	equal(filtered, ints{3, 1, 1})
	equal(trace, []int{3, 1, 2, 1})
	filtered[0] = 99
	equal(input, ints{3, 1, 2, 1})
	trace = nil
	equal(flatten(input, func(n int) []int {
		record(n)
		if n == 2 {
			return nil
		}
		return []int{n, -n}
	}), []int{3, -3, 1, -1, 1, -1})
	equal(trace, []int{3, 1, 2, 1})
	trace = nil
	equal(reduce(input, 5, func(acc, n int) int { return acc*10 + record(n) }), 53121)
	equal(trace, []int{3, 1, 2, 1})
	trace = nil
	groups := group(input, func(n int) int { return record(n) % 2 })
	equal(groups, map[int]ints{0: {2}, 1: {3, 1, 1}})
	equal(trace, []int{3, 1, 2, 1})
	groups[1][0] = 99
	equal(input, ints{3, 1, 2, 1})
	trace = nil
	equal(indexBy(input, func(n int) int { return record(n) % 2 }), map[int]int{0: 2, 1: 1})
	equal(trace, []int{3, 1, 2, 1})
	trace = nil
	kept, rest := partition(input, func(n int) bool { return record(n)%2 == 0 })
	equal(kept, ints{2})
	equal(rest, ints{3, 1, 1})
	equal(trace, []int{3, 1, 2, 1})
	trace = nil
	value, present := find(input, func(n int) bool { return record(n) == 1 })
	if !present || value != 1 {
		panic("Find")
	}
	equal(trace, []int{3, 1})
	trace = nil
	if _, exists := find(input, func(n int) bool { return record(n) == 9 }); exists {
		panic("missing Find")
	}
	equal(trace, []int{3, 1, 2, 1})
	if v, ok := first(ints{0}); !ok || v != 0 {
		panic("zero First")
	}
	if v, ok := last(input); !ok || v != 1 {
		panic("Last")
	}
	if v, ok := at(input, 2); !ok || v != 2 {
		panic("At")
	}
	for _, n := range []int{-1, len(input)} {
		if _, ok := at(input, n); ok {
			panic("invalid At")
		}
	}
	trace = nil
	if all(input, func(n int) bool { return record(n) != 2 }) {
		panic("All")
	}
	equal(trace, []int{3, 1, 2})
	trace = nil
	equal(count(input, func(n int) bool { return record(n)%2 != 0 }), 3)
	equal(trace, []int{3, 1, 2, 1})
	dedup := distinct(input)
	equal(dedup, ints{3, 1, 2})
	dedup[0] = 99
	equal(input, ints{3, 1, 2, 1})
	equal(toSet(input), map[int]struct{}{1: {}, 2: {}, 3: {}})
	if v, ok := lookup(pointers{"nil": nil}, "nil"); !ok || v != nil {
		panic("present nil")
	}
	if _, ok := lookup(pointers(nil), "missing"); ok {
		panic("missing Lookup")
	}

	var empty ints
	never := func(int) bool { panic("empty callback") }
	if transform(empty, func(int) string { panic("empty callback") }) == nil || filter(empty, never) == nil || flatten(empty, func(int) []int { panic("empty callback") }) == nil || distinct(empty) == nil {
		panic("nil empty slice")
	}
	if v, ok := first(empty); ok || v != 0 {
		panic("empty First")
	}
	if v, ok := last(empty); ok || v != 0 {
		panic("empty Last")
	}
	if _, ok := find(empty, never); ok {
		panic("empty Find")
	}
	if !all(empty, never) || count(empty, never) != 0 || reduce(empty, 7, func(int, int) int { panic("empty callback") }) != 7 {
		panic("empty predicates")
	}
	a, b := partition(empty, never)
	if a == nil || b == nil || group(empty, func(int) int { panic("empty callback") }) == nil || indexBy(empty, func(int) int { panic("empty callback") }) == nil || toSet(empty) == nil {
		panic("nil empty collection")
	}

	var visited, mapped, tested []int
	var source iter.Seq[int] = func(yield func(int) bool) {
		for _, n := range []int{1, 2, 3, 4, 5} {
			visited = append(visited, n)
			if !yield(n) {
				return
			}
		}
	}
	lazy := mapSeq(filterSeq(source, func(n int) bool {
		tested = append(tested, n)
		return n%2 == 0
	}), func(n int) string {
		mapped = append(mapped, n)
		return fmt.Sprint(n)
	})
	if len(visited)+len(tested)+len(mapped) != 0 {
		panic("eager iterator")
	}
	for value := range lazy {
		if value != "2" {
			panic("lazy value")
		}
		break
	}
	equal(visited, []int{1, 2})
	equal(tested, []int{1, 2})
	equal(mapped, []int{2})
	visited, mapped, tested = nil, nil, nil
	equal(slices.Collect(lazy), []string{"2", "4"})
	equal(visited, []int{1, 2, 3, 4, 5})
	equal(tested, []int{1, 2, 3, 4, 5})
	equal(mapped, []int{2, 4})
	fmt.Println("PASS")
}
