package main

/*
#include <errno.h>
static int divide(int a, int b) { if (!b) { errno = EDOM; return -1; } return a / b; }
static int tag(void) { return 42; }
*/
import "C"
import "fmt"

var contexts int
func context(err error) error { contexts++; return fmt.Errorf("C %d: %w", C.tag(), err) }
func divide(divisor C.int) (value int, err error) {
	value = 88
	defer func() { if err != nil && value != 0 { panic("named result not zeroed") } }()
	result, problem := C.divide(9, divisor)
	if problem != nil { return 0, context(problem) }
	return int(result), nil
}
func forget() (int, error) {
	value, err := C.divide(9, 0)
	if err != nil { return 0, nil }
	panic(value)
}
func main() {
	for _, divisor := range []C.int{3, 0} {
		value, err := divide(divisor)
		fmt.Println(value, err != nil)
	}
	if contexts != 1 { panic("context not lazy or repeated") }
	if value, err := forget(); value != 0 || err != nil { panic("nil context") }
	fmt.Println("PASS")
}
