package main

import "iter"

func transform(s ints, f func(int) string) []string {
	result := make([]string, len(s))
	for i, n := range s {
		result[i] = f(n)
	}
	return result
}
func filter(s ints, f func(int) bool) ints {
	result := make(ints, 0)
	for _, n := range s {
		if f(n) {
			result = append(result, n)
		}
	}
	return result
}
func flatten(s ints, f func(int) []int) []int {
	result := make([]int, 0)
	for _, n := range s {
		result = append(result, f(n)...)
	}
	return result
}
func reduce(s ints, initial int, f func(int, int) int) int {
	result := initial
	for _, n := range s {
		result = f(result, n)
	}
	return result
}
func group(s ints, f func(int) int) map[int]ints {
	result := make(map[int]ints)
	for _, n := range s {
		k := f(n)
		result[k] = append(result[k], n)
	}
	return result
}
func indexBy(s ints, f func(int) int) map[int]int {
	result := make(map[int]int)
	for _, n := range s {
		result[f(n)] = n
	}
	return result
}
func partition(s ints, f func(int) bool) (ints, ints) {
	kept, rest := make(ints, 0), make(ints, 0)
	for _, n := range s {
		if f(n) {
			kept = append(kept, n)
		} else {
			rest = append(rest, n)
		}
	}
	return kept, rest
}
func find(s ints, f func(int) bool) (int, bool) {
	for _, n := range s {
		if f(n) {
			return n, true
		}
	}
	return 0, false
}
func first(s ints) (int, bool) { return at(s, 0) }
func last(s ints) (int, bool)  { return at(s, len(s)-1) }
func at(s ints, index int) (int, bool) {
	if index < 0 || index >= len(s) {
		return 0, false
	}
	return s[index], true
}
func all(s ints, f func(int) bool) bool {
	for _, n := range s {
		if !f(n) {
			return false
		}
	}
	return true
}
func count(s ints, f func(int) bool) int {
	result := 0
	for _, n := range s {
		if f(n) {
			result++
		}
	}
	return result
}
func distinct(s ints) ints {
	result := make(ints, 0)
	seen := make(map[int]bool)
	for _, n := range s {
		if !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}
	return result
}
func toSet(s ints) map[int]struct{} {
	result := make(map[int]struct{})
	for _, n := range s {
		result[n] = struct{}{}
	}
	return result
}
func lookup(m pointers, key string) (*int, bool) { value, exists := m[key]; return value, exists }
func mapSeq(input iter.Seq[int], f func(int) string) iter.Seq[string] {
	return func(yield func(string) bool) {
		input(func(n int) bool { return yield(f(n)) })
	}
}
func filterSeq(input iter.Seq[int], f func(int) bool) iter.Seq[int] {
	return func(yield func(int) bool) {
		input(func(n int) bool { return !f(n) || yield(n) })
	}
}
