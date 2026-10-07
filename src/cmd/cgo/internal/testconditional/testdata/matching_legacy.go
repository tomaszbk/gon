package main

/*
static int add(int x, int y) { return x + y; }
*/
import "C"
import "fmt"

type Value struct {
	kind int
	n    C.int
}

func choose(v Value) C.int {
	switch v.kind {
	case 0:
		return C.int(0)
	case 1, 2:
		if C.add(v.n, 1) == 4 {
			return C.add(v.n, 2)
		}
		return v.n
	}
	panic("bad")
}
func main() {
	v := Value{kind: 1, n: C.add(1, 2)}
	n := choose(v)
	r := Value{kind: 2, n: C.add(4, 5)}
	if r.kind == 2 {
		number := r.n
		if C.add(number, 0) != 9 {
			panic("bad cgo pattern test")
		}
	}
	var chosen C.int
	if r.kind == 2 {
		chosen = r.n
	}
	b := C.add(chosen, n)
	if n != 5 || b != 14 || choose(Value{}) != 0 || choose(Value{kind: 2, n: C.int(3)}) != 5 {
		panic("bad cgo matching")
	}
	fmt.Println(n, b, "PASS")
}
