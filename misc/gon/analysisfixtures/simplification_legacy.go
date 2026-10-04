package main

type optional[T any] struct {
	present bool
	value   T
}
type result[T any] struct {
	failed  bool
	value   T
	problem string
}

func identity[T any](value T) T { return value }

type item struct{ N int }

var calls int

func next() int { calls++; return calls }
func port(empty bool) result[optional[int]] {
	if empty {
		return result[optional[int]]{}
	}
	return result[optional[int]]{value: optional[int]{true, next()}}
}
func main() {
	if p := port(true); p.failed || p.value.present || calls != 0 {
		panic("absence")
	}
	p := port(false)
	if !p.value.present || p.value.value != 1 || calls != 1 {
		panic("single evaluation")
	}
	var ptr *int
	present := optional[*int]{true, ptr}
	if !present.present || present.value != nil {
		panic("typed nil")
	}
	explicit := optional[*int]{true, nil}
	if !explicit.present {
		panic("explicit nil")
	}
	nested := optional[optional[int]]{true, optional[int]{true, next()}}
	if !nested.present || !nested.value.present || nested.value.value != 2 || calls != 2 {
		panic("nested")
	}
	bad := result[int]{failed: true, problem: "bad"}
	if !bad.failed || bad.problem != "bad" {
		panic("failure")
	}
	boxed := optional[item]{true, item{N: next()}}
	if !boxed.present || boxed.value.N != 3 || calls != 3 {
		panic("composite payload")
	}
	generic := identity[optional[int]](optional[int]{})
	if generic.present {
		panic("generic absence")
	}
	println("PASS")
}
