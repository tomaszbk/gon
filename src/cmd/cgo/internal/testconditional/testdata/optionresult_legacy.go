package main

/*
#include <errno.h>
typedef struct { int *p; int n; } c_buf;
static int add(c_buf *b, int n) { return (b ? b->n : 0) + n; }
static int divide(int a, int b) { if (!b) { errno = EDOM; return -1; } return a / b; }
*/
import "C"
import "fmt"

func add(present bool) (int, bool) {
	var value C.int
	if present {
		value = 2
	} else {
		return 0, false
	}
	var buffer C.c_buf
	buffer.n = 10
	return int(C.add(&buffer, value)), true
}
func divide(divisor C.int) (int, error) {
	value, problem := C.divide(9, divisor)
	if problem != nil {
		return 0, problem
	}
	return int(value), nil
}
func main() {
	var missing struct {
		value   C.int
		present bool
	}
	zero := struct {
		value   C.int
		present bool
	}{0, true}
	pointer := struct {
		value   *C.int
		present bool
	}{nil, true}
	values := struct {
		value   []C.int
		present bool
	}{nil, true}
	if missing.present || !zero.present || !pointer.present || !values.present {
		panic("optional presence")
	}
	for _, present := range []bool{true, false} {
		value, exists := add(present)
		if exists != present {
			panic("Cgo optional presence")
		}
		fmt.Println(value)
	}
	for _, divisor := range []C.int{3, 0} {
		value, problem := divide(divisor)
		if problem != nil {
			fmt.Println(fmt.Sprint("error:", problem != nil))
		} else {
			fmt.Println(value)
		}
	}
	fmt.Println("PASS")
}
