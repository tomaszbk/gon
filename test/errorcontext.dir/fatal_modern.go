package main

import (
	"fmt"
	"testing"
)

func TestContextFatal(t *testing.T) {
	defer t.Log("defer ran")                                      // MARK:deferred
	n := read(true) or err => fmt.Errorf("test context: %w", err) // MARK:failure
	t.Log("unreachable", n)
}
func TestContextNil(t *testing.T) {
	only(true) or _ => nil // MARK:nil
	t.Log("unreachable")
}
func contextHelper(tb testing.TB) int {
	return read(true) or err => fmt.Errorf("helper context: %w", err) // MARK:helper
}
func TestContextHelper(t *testing.T) { contextHelper(t) }
func TestContextSuccess(t *testing.T) {
	n := read(false) or err => wrap(err)
	if n != 7 {
		t.Fatal(n)
	}
}
func TestContextLambda(t *testing.T) {
	t.Run("nested", (t) => {
		only(true) or err => fmt.Errorf("lambda context: %w", err) // MARK:lambda
	})
}
func FuzzContextDirect(f *testing.F) {
	only(true) or err => fmt.Errorf("fuzz context: %w", err) // MARK:fuzz
}
func BenchmarkContextFailure(b *testing.B) {
	only(true) or err => fmt.Errorf("benchmark context: %w", err) // MARK:benchmark
}
