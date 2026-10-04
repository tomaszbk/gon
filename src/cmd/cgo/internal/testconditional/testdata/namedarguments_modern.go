package main

/*
typedef struct { int *p; int n; } c_buf;
static int add(c_buf *b, int n) { return (b ? b->n : 0) + n; }
*/
import "C"
import "fmt"

var trace string

func mark(s string, n C.int) C.int            { trace += s; return n }
func invoke(buffer *C.c_buf, delta C.int) int { return int(C.add(buffer, delta)) }
func main() {
	var b C.c_buf
	b.n = 10
	result := invoke(delta: mark(n: 2, s: "D"), buffer: func() *C.c_buf { trace += "B"; return &b }())
	if result != 12 || trace != "DB" {
		panic(fmt.Sprintf("bad result=%d trace=%s", result, trace))
	}
	fmt.Println(result, trace)
	fmt.Println("PASS")
}
