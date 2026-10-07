package main

/*
static int twice(int x) { return x*2; }
typedef struct { int n; } Buffer;
static int size(Buffer *b) { return b->n; }
*/
import "C"
import "fmt"

func main() {
	var buffer C.Buffer
	buffer.n = 7
	label := fmt.Sprintf("%v:%04d", C.twice(3), C.size(&buffer))
	if label != "6:0007" {
		panic(label)
	}
	fmt.Println(label)
	fmt.Println("PASS")
}
