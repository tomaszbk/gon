package main

import (
	"errors"
	"fmt"
	"matchinterfaceexport/lib"
)

func main() {
	for _, err := range []error{lib.NewMissing(), lib.NewCoded("x"), fmt.Errorf("w: %w", lib.NewCoded("y")), errors.New("z"), nil, errors.Join(errors.New("a"), lib.NewMissing())} {
		fmt.Println(lib.Code(err), "main:"+lib.Kind(err))
	}
	fmt.Println(lib.Unbox(lib.Full(5), 0), lib.Unbox(lib.Full("s"), "d"), lib.Unbox(lib.Full("s"), 7), lib.Unbox[int](nil, 8))
}
