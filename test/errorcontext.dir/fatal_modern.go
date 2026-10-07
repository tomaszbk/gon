package main

import (
	"fmt"
	"testing"
)

func TestContextFatal(t *testing.T) {
	defer t.Log("defer ran") // MARK:deferred
	n := read(true) or err {
		t.Fatal(fmt.Errorf("test context: %w", err)) // MARK:failure
		return
	}
	t.Log("unreachable", n)
}
func TestContextNil(t *testing.T) {
	only(true) or _ {
		t.Fatal(error(nil)) // MARK:nil
		return
	}
	t.Log("unreachable")
}
func contextHelper(tb testing.TB) int {
	return read(true) or err {
		tb.Fatal(fmt.Errorf("helper context: %w", err)) // MARK:helper
		return 0
	}
}
func TestContextHelper(t *testing.T) { contextHelper(t) }
func TestContextSuccess(t *testing.T) {
	n := read(false) or err {
		t.Fatal(wrap(err))
		return
	}
	if n != 7 {
		t.Fatal(n)
	}
}
func TestContextLambda(t *testing.T) {
	t.Run("nested", (t) => {
		only(true) or err {
			t.Fatal(fmt.Errorf("lambda context: %w", err)) // MARK:lambda
			return
		}
	})
}
func FuzzContextDirect(f *testing.F) {
	only(true) or err {
		f.Fatal(fmt.Errorf("fuzz context: %w", err)) // MARK:fuzz
		return
	}
}
func BenchmarkContextFailure(b *testing.B) {
	only(true) or err {
		b.Fatal(fmt.Errorf("benchmark context: %w", err)) // MARK:benchmark
		return
	}
}
