package main

import (
	"errorbridge/lib"
	"fmt"
)

func unpack[T any](r Result[T, error]) (T, error) {
	v := r or e {
		var zero T
		return zero, e
	}
	return v, nil
}

func main() {
	for _, fail := range []bool{false, true} {
		n, err := unpack(lib.TupleToResult(fail))
		fmt.Println("tuple", fail, err != nil, n, err)
		s, err := unpack(lib.GenericTuple("value", fail))
		fmt.Printf("generic tuple %v %v %q %v\n", fail, err != nil, s, err)
	}
	for kind := 0; kind < 3; kind++ {
		n, err := lib.ResultToError(kind)
		fmt.Println("result", kind, n, err, lib.IsNilResult(err))
		s, err := lib.GenericResultToError("value", kind)
		fmt.Printf("generic result %d %q %v %v\n", kind, s, err, lib.IsNilResult(err))
	}
}
