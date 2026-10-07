package main

import (
	"fmt"
	"testing"
)

func TestContextFatal(t *testing.T) {
	defer t.Log("defer ran") // MARK:deferred
	n, err := read(true)
	if err != nil {
		t.Fatal(fmt.Errorf("test context: %w", err)) // MARK:failure
	}
	t.Log("unreachable", n)
}
func TestContextNil(t *testing.T) {
	if err := only(true); err != nil {
		t.Fatal(error(nil)) // MARK:nil
	}
	t.Log("unreachable")
}
func contextHelper(tb testing.TB) int {
	n, err := read(true)
	if err != nil {
		tb.Fatal(fmt.Errorf("helper context: %w", err)) // MARK:helper
		return 0
	}
	return n
}
func TestContextHelper(t *testing.T) { contextHelper(t) }
func TestContextSuccess(t *testing.T) {
	n, err := read(false)
	if err != nil {
		t.Fatal(wrap(err))
	}
	if n != 7 {
		t.Fatal(n)
	}
}
func TestContextLambda(t *testing.T) {
	t.Run("nested", func(t *testing.T) {
		if err := only(true); err != nil {
			t.Fatal(fmt.Errorf("lambda context: %w", err)) // MARK:lambda
		}
	})
}
func FuzzContextDirect(f *testing.F) {
	if err := only(true); err != nil {
		f.Fatal(fmt.Errorf("fuzz context: %w", err)) // MARK:fuzz
	}
}
func BenchmarkContextFailure(b *testing.B) {
	if err := only(true); err != nil {
		b.Fatal(fmt.Errorf("benchmark context: %w", err)) // MARK:benchmark
	}
}
