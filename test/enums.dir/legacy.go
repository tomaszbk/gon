package main

type Color struct{ tag uint }

func red() Color   { return Color{} }
func green() Color { return Color{1} }

type Payment struct {
	tag     uint
	reason  string
	receipt string
	amount  int
	pointer *int
}

func pending() Payment               { return Payment{} }
func rejected(reason string) Payment { return Payment{tag: 1, reason: reason} }
func paid(receipt string, amount int) Payment {
	return Payment{tag: 2, receipt: receipt, amount: amount}
}
func pointer(p *int) Payment { return Payment{tag: 3, pointer: p} }

type Box[T any] struct {
	present bool
	value   T
}

func full[T any](v T) Box[T] { return Box[T]{present: true, value: v} }

type optional[T any] struct {
	present bool
	value   T
}

func some[T any](v T) optional[T] { return optional[T]{present: true, value: v} }

func main() {
	var color Color
	assert(color == red() && color != green(), "stable default")
	amount, receipt := count("amount", 5), text("receipt")
	first := paid(receipt, amount)
	assert(first == paid("receipt", 5), "record source order")
	assert(paid("x", 0) == paid("x", 0), "record omission zeros")
	ctor := rejected
	assert(ctor(text("callback")) == rejected("callback"), "first class constructor")
	b := full(count("generic", 7))
	assert(b != (Box[int]{}) && b == full(7), "generic variant")
	var none optional[int]
	assert(none == (optional[int]{}) && none != some(0), "Some zero is present")
	assert(some[*int](nil) != (optional[*int]{}), "Some nil is present")
	n := 42
	value := pointer(&n)
	copied := value
	collect()
	assert(copied == pointer(&n), "typed GC pointer storage")
	n = 43
	assert(copied == value, "pointer alias copy")
	array := [2]Payment{first, pending()}
	assert(array[0] == first && array[1] == pending(), "array copy")
	assert(map[Payment]int{first: 9}[first] == 9, "comparable enum map key")
	finish()
}
