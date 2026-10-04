package main

/*
#include <errno.h>
typedef struct { int *p; int n; } c_buf;
static int add(c_buf *b, int n) { return (b ? b->n : 0) + n; }
static int divide(int a, int b) { if (!b) { errno = EDOM; return -1; } return a / b; }
*/
import "C"
import "fmt"

func add(present bool) (number int?) {
	var value C.int?
	if present {
		value = 2
	}
	delta := value?
	var buffer C.c_buf
	buffer.n = 10
	return int(C.add(&buffer, delta))
}
func divide(divisor C.int) Result[int, error] {
	value := C.divide(9, divisor) or problem {
		return .Err(problem)
	}
	return .Ok(int(value))
}
func label(value Result[int, error]) string {
	return fmt.Sprint(value or problem {
		return fmt.Sprint("error:", problem != nil)
	})
}
func main() {
	for _, present := range []bool{true, false} {
		fmt.Println(add(present) ?? 0)
	}
	for _, divisor := range []C.int{3, 0} {
		fmt.Println(label(divide(divisor)))
	}
	fmt.Println("PASS")
}
