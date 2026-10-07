package main

func identity[T any](value T) T { return value }

type item struct{ N int }

var calls int

func next() int { calls++; return calls }
func port(empty bool) int? {
	if empty {
		return nil
	}
	return next()
}
func main() {
	p := port(true)
	switch p {
	case nil => {
	}
	default => {
		panic("absence")
	}
	}
	if calls != 0 {
		panic("absence effects")
	}
	p = port(false)
	switch p {
	case n? if n == 1 => {
	}
	default => {
		panic("single evaluation")
	}
	}
	if calls != 1 {
		panic("single evaluation effects")
	}
	var ptr *int
	var present (*int)? = ptr
	switch present {
	case value? if value == nil => {
	}
	default => {
		panic("typed nil")
	}
	}
	var explicit (*int)? = (*int)(nil)
	switch explicit {
	case value? if value == nil => {
	}
	default => {
		panic("explicit nil")
	}
	}
	var inner int? = next()
	var nested (int?)? = inner
	switch nested {
	case (n?)? if n == 2 => {
	}
	default => {
		panic("nested")
	}
	}
	if calls != 2 {
		panic("nested effects")
	}
	var boxed item? = item{N: next()}
	switch boxed {
	case value? if value.N == 3 => {
	}
	default => {
		panic("composite payload")
	}
	}
	if calls != 3 {
		panic("composite effects")
	}
	generic := identity[int?](nil)
	switch generic {
	case nil => {
	}
	default => {
		panic("generic absence")
	}
	}
	println("PASS")
}
