package main

type Color enum {
	Green
	default Red
	Blue
}
type Payment enum {
	Rejected(string)
	default Pending
	Paid  {
		Receipt string
		Amount  int
	}
	Pointer(*int)
}
type Box[T any] enum {
	default Empty
	Full(T)
}

func main() {
	var color Color
	assert(color == Color.Red && color != Color.Green, "stable default")
	first := Payment.Paid{Amount: count("amount", 5), Receipt: text("receipt")}
	assert(first == (Payment.Paid{Receipt: "receipt", Amount: 5}), "record source order")
	assert(Payment.Paid{Receipt: "x"} == (Payment.Paid{Receipt: "x", Amount: 0}), "record omission zeros")
	ctor := Payment.Rejected
	assert(ctor(text("callback")) == Payment.Rejected("callback"), "first class constructor")
	b := Box[int].Full(count("generic", 7))
	assert(b != Box[int].Empty && b == Box[int].Full(7), "generic variant")
	var none int?
	assert(none == (int?)(nil) && none != (int?)((int)(0)), "Some zero is present")
	assert(((*int)?)((*int)(nil)) != ((*int)?)(nil), "Some nil is present")
	n := 42
	pointer := Payment.Pointer(&n)
	copied := pointer
	collect()
	assert(copied == Payment.Pointer(&n), "typed GC pointer storage")
	n = 43
	assert(copied == pointer, "pointer alias copy")
	array := [2]Payment{first, Payment.Pending}
	assert(array[0] == first && array[1] == Payment.Pending, "array copy")
	assert(map[Payment]int{first: 9}[first] == 9, "comparable enum map key")
	finish()
}
