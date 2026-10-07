// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package seq provides eager slice transformations, optional lookups and lazy
// iterator transformations. Slice results are always non-nil and preserve the
// input order. Callbacks run once per visited element in input order.
package seq

import "iter"

// Map returns a new slice containing f applied to each element of s.
func Map[S ~[]E, E, R any](s S, f func(E) R) []R {
	result := make([]R, len(s))
	for i, value := range s {
		result[i] = f(value)
	}
	return result
}

// Filter returns a new slice containing the elements for which keep is true.
func Filter[S ~[]E, E any](s S, keep func(E) bool) S {
	result := make(S, 0, len(s))
	for _, value := range s {
		if keep(value) {
			result = append(result, value)
		}
	}
	return result
}

// FlatMap concatenates the slices returned by f for each element of s.
func FlatMap[S ~[]E, E, R any](s S, f func(E) []R) []R {
	result := make([]R, 0)
	for _, value := range s {
		result = append(result, f(value)...)
	}
	return result
}

// Reduce folds s from left to right, starting with initial.
func Reduce[S ~[]E, E, A any](s S, initial A, f func(A, E) A) A {
	result := initial
	for _, value := range s {
		result = f(result, value)
	}
	return result
}

// GroupBy groups elements by key. Elements within each group retain their
// input order. An empty input produces a non-nil empty map.
func GroupBy[S ~[]E, E any, K comparable](s S, key func(E) K) map[K]S {
	result := make(map[K]S)
	for _, value := range s {
		k := key(value)
		result[k] = append(result[k], value)
	}
	return result
}

// KeyBy indexes elements by key. If keys repeat, the last element wins.
// An empty input produces a non-nil empty map.
func KeyBy[S ~[]E, E any, K comparable](s S, key func(E) K) map[K]E {
	result := make(map[K]E)
	for _, value := range s {
		result[key(value)] = value
	}
	return result
}

// Partition returns new slices containing elements for which keep is true
// and false, respectively. Both results preserve input order and are non-nil.
func Partition[S ~[]E, E any](s S, keep func(E) bool) (kept, rest S) {
	kept, rest = make(S, 0), make(S, 0)
	for _, value := range s {
		if keep(value) {
			kept = append(kept, value)
		} else {
			rest = append(rest, value)
		}
	}
	return
}

// Find returns the first element for which match is true, or absence.
// It stops calling match as soon as an element is found. A zero or typed nil
// element remains present.
func Find[S ~[]E, E any](s S, match func(E) bool) E? {
	for _, value := range s {
		if match(value) {
			return value
		}
	}
	return nil
}

// First returns the first element of s, or absence if s is empty.
func First[S ~[]E, E any](s S) E? {
	return At(s, 0)
}

// Last returns the last element of s, or absence if s is empty.
func Last[S ~[]E, E any](s S) E? {
	return At(s, len(s)-1)
}

// At returns the element at index, or absence if index is out of bounds.
func At[S ~[]E, E any](s S, index int) E? {
	if index < 0 || index >= len(s) {
		return nil
	}
	return s[index]
}

// All reports whether match is true for every element. It stops at the first
// false result and returns true for an empty input.
func All[S ~[]E, E any](s S, match func(E) bool) bool {
	for _, value := range s {
		if !match(value) {
			return false
		}
	}
	return true
}

// Count returns the number of elements for which match is true.
// It visits every element, including when all or none match.
func Count[S ~[]E, E any](s S, match func(E) bool) int {
	count := 0
	for _, value := range s {
		if match(value) {
			count++
		}
	}
	return count
}

// Distinct returns a new slice with duplicate elements removed, retaining the
// first occurrence of each element in input order.
func Distinct[S ~[]E, E comparable](s S) S {
	result := make(S, 0, len(s))
	seen := make(map[E]struct{}, len(s))
	for _, value := range s {
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

// ToSet returns a non-nil map containing every distinct element of s.
func ToSet[S ~[]E, E comparable](s S) map[E]struct{} {
	result := make(map[E]struct{}, len(s))
	for _, value := range s {
		result[value] = struct{}{}
	}
	return result
}

// Lookup returns the value stored under key, or absence if key is missing.
// A stored zero or typed nil remains present.
func Lookup[M ~map[K]V, K comparable, V any](m M, key K) V? {
	value, exists := m[key]
	if !exists {
		return nil
	}
	return value
}

// MapSeq returns an iterator applying f to each element of input. It does not
// visit input until iterated and stops as soon as its consumer stops.
func MapSeq[E, R any](seq iter.Seq[E], f func(E) R) iter.Seq[R] {
	return func(yield func(R) bool) {
		for value := range seq {
			if !yield(f(value)) {
				return
			}
		}
	}
}

// FilterSeq returns an iterator yielding the elements for which keep is true.
// It does not visit input until iterated and stops when its consumer stops.
func FilterSeq[E any](seq iter.Seq[E], keep func(E) bool) iter.Seq[E] {
	return func(yield func(E) bool) {
		for value := range seq {
			if keep(value) && !yield(value) {
				return
			}
		}
	}
}
