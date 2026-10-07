// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package seq_test

import (
	"encoding/json"
	"gon/seq"
	"iter"
	"reflect"
	"slices"
	"testing"
)

type numbers []int

func requireEqual(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func payload[T any](t *testing.T, value T?) T {
	t.Helper()
	if value == nil {
		t.Fatal("expected a present value")
	}
	var zero T
	return value ?? zero
}

func TestSliceTransforms(t *testing.T) {
	input := numbers{3, 1, 2, 1}
	var visited []int
	requireEqual(t, seq.Map(input, func(n int) string {
		visited = append(visited, n)
		return string(rune('a' + n))
	}), []string{"d", "b", "c", "b"})
	requireEqual(t, visited, []int{3, 1, 2, 1})

	visited = nil
	filtered := seq.Filter(input, func(n int) bool {
		visited = append(visited, n)
		return n%2 != 0
	})
	requireEqual(t, filtered, numbers{3, 1, 1})
	requireEqual(t, visited, []int{3, 1, 2, 1})
	filtered[0] = 99
	requireEqual(t, input, numbers{3, 1, 2, 1})

	visited = nil
	requireEqual(t, seq.FlatMap(input, func(n int) []int {
		visited = append(visited, n)
		if n == 2 {
			return nil
		}
		return []int{n, -n}
	}), []int{3, -3, 1, -1, 1, -1})
	requireEqual(t, visited, []int{3, 1, 2, 1})

	visited = nil
	requireEqual(t, seq.Reduce(input, "start", func(acc string, n int) string {
		visited = append(visited, n)
		return acc + string(rune('0'+n))
	}), "start3121")
	requireEqual(t, visited, []int{3, 1, 2, 1})

	visited = nil
	kept, rest := seq.Partition(input, func(n int) bool {
		visited = append(visited, n)
		return n%2 == 0
	})
	requireEqual(t, kept, numbers{2})
	requireEqual(t, rest, numbers{3, 1, 1})
	requireEqual(t, visited, []int{3, 1, 2, 1})
	kept[0], rest[0] = 99, 99
	requireEqual(t, input, numbers{3, 1, 2, 1})

	distinct := seq.Distinct(input)
	requireEqual(t, distinct, numbers{3, 1, 2})
	distinct[0] = 99
	requireEqual(t, input, numbers{3, 1, 2, 1})
	requireEqual(t, seq.ToSet(input), map[int]struct{}{1: {}, 2: {}, 3: {}})
}

func TestGrouping(t *testing.T) {
	input := numbers{3, 1, 2, 4, 1}
	var visited []int
	groups := seq.GroupBy(input, func(n int) int {
		visited = append(visited, n)
		return n % 2
	})
	requireEqual(t, groups, map[int]numbers{0: {2, 4}, 1: {3, 1, 1}})
	requireEqual(t, visited, []int{3, 1, 2, 4, 1})
	groups[1][0] = 99
	requireEqual(t, input, numbers{3, 1, 2, 4, 1})
	visited = nil
	requireEqual(t, seq.KeyBy(input, func(n int) int {
		visited = append(visited, n)
		return n % 2
	}), map[int]int{0: 4, 1: 1})
	requireEqual(t, visited, []int{3, 1, 2, 4, 1})
}

func TestOptionalLookups(t *testing.T) {
	input := numbers{0, 2, 0}
	var visited []int
	requireEqual(t, payload(t, seq.Find(input, func(n int) bool {
		visited = append(visited, n)
		return n == 2
	})), 2)
	requireEqual(t, visited, []int{0, 2})
	visited = nil
	if seq.Find(input, func(n int) bool {
		visited = append(visited, n)
		return n == 9
	}) != nil {
		t.Fatal("Find returned a missing element")
	}
	requireEqual(t, visited, []int{0, 2, 0})
	requireEqual(t, payload(t, seq.First(input)), 0)
	requireEqual(t, payload(t, seq.Last(input)), 0)
	requireEqual(t, payload(t, seq.At(input, 1)), 2)
	for _, index := range []int{-1, len(input), int(^uint(0) >> 1)} {
		if seq.At(input, index) != nil {
			t.Fatalf("At(%d) returned an out-of-bounds element", index)
		}
	}
	if seq.First(numbers(nil)) != nil || seq.Last(numbers{}) != nil {
		t.Fatal("empty slice has an element")
	}

	var pointer *int
	nilPointers := []*int{pointer}
	if seq.Find(nilPointers, func(p *int) bool { return p == nil }) == nil || payload(t, seq.First(nilPointers)) != nil || seq.Last(nilPointers) == nil || seq.At(nilPointers, 0) == nil {
		t.Fatal("typed nil element lost presence")
	}
	type dictionary map[string]*int
	values := dictionary{"nil": nil}
	if seq.Lookup(values, "nil") == nil || payload(t, seq.Lookup(values, "nil")) != nil || seq.Lookup(values, "missing") != nil {
		t.Fatal("map lookup lost typed nil presence or missing key")
	}
	if seq.Lookup(map[string]int{"zero": 0}, "zero") == nil || seq.Lookup(map[string]int(nil), "zero") != nil {
		t.Fatal("map lookup lost zero presence or nil map absence")
	}

	// A missing inner optional is still a present element in the outer lookup.
	var inner int? = nil
	nested := seq.First([]int?{inner})
	if nested == nil || payload(t, nested) != nil {
		t.Fatal("optional element was flattened")
	}
	if seq.Lookup(map[string]int?{"inner": nil}, "inner") == nil {
		t.Fatal("optional map value was flattened")
	}
}

func TestPredicates(t *testing.T) {
	input := numbers{1, 2, 3}
	var visited []int
	if seq.All(input, func(n int) bool {
		visited = append(visited, n)
		return n < 2
	}) {
		t.Fatal("All ignored a false predicate")
	}
	requireEqual(t, visited, []int{1, 2})
	visited = nil
	if !seq.All(input, func(n int) bool {
		visited = append(visited, n)
		return n > 0
	}) {
		t.Fatal("All rejected true predicates")
	}
	requireEqual(t, visited, []int{1, 2, 3})
	visited = nil
	requireEqual(t, seq.Count(input, func(n int) bool {
		visited = append(visited, n)
		return n != 2
	}), 2)
	requireEqual(t, visited, []int{1, 2, 3})
}

func TestEmptyInputs(t *testing.T) {
	var input numbers
	panicCallback := func(int) bool { panic("empty callback") }
	for name, value := range map[string]any{
		"Map":      seq.Map(input, func(int) int { panic("empty callback") }),
		"Filter":   seq.Filter(input, panicCallback),
		"FlatMap":  seq.FlatMap(input, func(int) []int { panic("empty callback") }),
		"Distinct": seq.Distinct(input),
	} {
		data, err := json.Marshal(value)
		if err != nil || string(data) != "[]" {
			t.Errorf("%s empty slice JSON = %s, %v", name, data, err)
		}
	}
	kept, rest := seq.Partition(input, panicCallback)
	if kept == nil || rest == nil || len(kept) != 0 || len(rest) != 0 {
		t.Fatal("empty Partition returned nil or non-empty slices")
	}
	if seq.Reduce(input, 17, func(int, int) int { panic("empty callback") }) != 17 || !seq.All(input, panicCallback) || seq.Count(input, panicCallback) != 0 || seq.Find(input, panicCallback) != nil {
		t.Fatal("empty reduction, predicate or lookup")
	}
	if groups := seq.GroupBy(input, func(int) int { panic("empty callback") }); groups == nil || len(groups) != 0 {
		t.Fatal("empty GroupBy map")
	}
	if indexed := seq.KeyBy(input, func(int) int { panic("empty callback") }); indexed == nil || len(indexed) != 0 {
		t.Fatal("empty KeyBy map")
	}
	if set := seq.ToSet(input); set == nil || len(set) != 0 {
		t.Fatal("empty ToSet map")
	}
}

func TestLazyIterators(t *testing.T) {
	var visits, mapped, filtered []int
	input := func(yield func(int) bool) {
		for _, n := range []int{1, 2, 3, 4, 5} {
			visits = append(visits, n)
			if !yield(n) {
				return
			}
		}
	}
	output := seq.MapSeq(seq.FilterSeq(iter.Seq[int](input), func(n int) bool {
		filtered = append(filtered, n)
		return n%2 == 0
	}), func(n int) int {
		mapped = append(mapped, n)
		return n * 10
	})
	if len(visits)+len(mapped)+len(filtered) != 0 {
		t.Fatal("iterators evaluated eagerly")
	}
	for n := range output {
		requireEqual(t, n, 20)
		break
	}
	requireEqual(t, visits, []int{1, 2})
	requireEqual(t, filtered, []int{1, 2})
	requireEqual(t, mapped, []int{2})
	visits, mapped, filtered = nil, nil, nil
	requireEqual(t, slices.Collect(output), []int{20, 40})
	requireEqual(t, visits, []int{1, 2, 3, 4, 5})
	requireEqual(t, filtered, []int{1, 2, 3, 4, 5})
	requireEqual(t, mapped, []int{2, 4})

	var empty iter.Seq[int] = func(func(int) bool) {}
	if values := slices.Collect(seq.FilterSeq(seq.MapSeq(empty, func(int) int { panic("empty callback") }), func(int) bool { panic("empty callback") })); len(values) != 0 {
		t.Fatal("empty iterator produced values")
	}
}

func TestNamedArgumentsAndLambdas(t *testing.T) {
	var order []string
	input := func() numbers { order = append(order, "input"); return numbers{1, 2} }
	callback := func() func(int) int { order = append(order, "callback"); return (n) => n * 2 }
	requireEqual(t, seq.Map(f: callback(), s: input()), []int{2, 4})
	requireEqual(t, order, []string{"callback", "input"})
	requireEqual(t, seq.Filter(numbers{1, 2, 3}, (n) => n%2 != 0), numbers{1, 3})
	requireEqual(t, payload(t, seq.Lookup(key: "zero", m: map[string]int{"zero": 0})), 0)
	requireEqual(t, slices.Collect(seq.MapSeq(f: (n) => n * 2, seq: slices.Values([]int{1, 2}))), []int{2, 4})
}
