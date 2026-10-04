package main

import (
	"fmt"
	"matchingexport/lib"
)

func main() {
	fmt.Println(lib.Size(lib.Count(7)), lib.Get(lib.Wrap("value"), "zero"), lib.Get(lib.Empty[string](), "zero"))
}
