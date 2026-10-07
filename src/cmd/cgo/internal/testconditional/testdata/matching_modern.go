package main

/*
static int add(int x, int y) { return x + y; }
*/
import "C"
import "fmt"

type Value enum {
	default Empty
	Number(C.int)
	Record { N C.int }
}

func choose(v Value) C.int {
	return switch v {
	case Value.Empty => C.int(0)
	case Value.Number(n), Value.Record{N: n} if C.add(n, 1) == 4 => C.add(n, 2)
	case Value.Number(n), Value.Record{N: n} => n
	}
}
func main() {
	v := Value.Number(C.add(1, 2))
	n := choose(v)
	r := Value.Record{N: C.add(4, 5)}
	if r is Value.Record{N: number} && C.add(number, 0) != 9 { panic("bad cgo pattern test") }
 b := C.add(C.int(switch r {
	case Value.Record{N: n} => n
	default => C.int(0)
	}), n)
	if n != 5 || b != 14 || choose(Value.Empty) != 0 || choose(Value.Record{N: C.int(3)}) != 5 {
		panic("bad cgo matching")
	}
	fmt.Println(n, b, "PASS")
}
