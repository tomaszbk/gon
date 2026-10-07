package main

import (
	"gon/seq"
	"iter"
)

func transform(s ints, f func(int) string) []string        { return seq.Map(s, f) }
func filter(s ints, f func(int) bool) ints                 { return seq.Filter(s, f) }
func flatten(s ints, f func(int) []int) []int              { return seq.FlatMap(s, f) }
func reduce(s ints, initial int, f func(int, int) int) int { return seq.Reduce(s, initial, f) }
func group(s ints, f func(int) int) map[int]ints           { return seq.GroupBy(s, f) }
func indexBy(s ints, f func(int) int) map[int]int          { return seq.KeyBy(s, f) }
func partition(s ints, f func(int) bool) (ints, ints)      { return seq.Partition(s, f) }
func unpack[T any](value T?) (T, bool) {
	var zero T
	return value ?? zero, value != nil
}
func find(s ints, f func(int) bool) (int, bool)                       { return unpack(seq.Find(s, f)) }
func first(s ints) (int, bool)                                        { return unpack(seq.First(s)) }
func last(s ints) (int, bool)                                         { return unpack(seq.Last(s)) }
func at(s ints, index int) (int, bool)                                { return unpack(seq.At(s, index)) }
func all(s ints, f func(int) bool) bool                               { return seq.All(s, f) }
func count(s ints, f func(int) bool) int                              { return seq.Count(s, f) }
func distinct(s ints) ints                                            { return seq.Distinct(s) }
func toSet(s ints) map[int]struct{}                                   { return seq.ToSet(s) }
func lookup(m pointers, key string) (*int, bool)                      { return unpack(seq.Lookup(m, key)) }
func mapSeq(input iter.Seq[int], f func(int) string) iter.Seq[string] { return seq.MapSeq(input, f) }
func filterSeq(input iter.Seq[int], f func(int) bool) iter.Seq[int]   { return seq.FilterSeq(input, f) }
