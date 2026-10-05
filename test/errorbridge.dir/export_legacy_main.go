package main

import (
	"errorbridge/lib"
	"fmt"
)

func main() {
	for _, fail := range []bool{false, true} {
		r := lib.TupleToResult(fail)
		n, err := r.Value, r.Problem
		if r.Failed {
			n = 0
		}
		fmt.Println("tuple", fail, err != nil, n, err)
		g := lib.GenericTuple("value", fail)
		s, err := g.Value, g.Problem
		if g.Failed {
			s = ""
		}
		fmt.Printf("generic tuple %v %v %q %v\n", fail, err != nil, s, err)
	}
	for kind := 0; kind < 3; kind++ {
		n, err := lib.ResultToError(kind)
		fmt.Println("result", kind, n, err, lib.IsNilResult(err))
		s, err := lib.GenericResultToError("value", kind)
		fmt.Printf("generic result %d %q %v %v\n", kind, s, err, lib.IsNilResult(err))
	}
}
