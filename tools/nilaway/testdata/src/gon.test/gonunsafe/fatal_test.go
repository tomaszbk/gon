package main

import "testing"

func testSuccessNil(t *testing.T) error {
	pointer := nilTuple()!
	_ = *pointer // want "dereferenced"
	return nil
}
func contextTestSuccessNil(t *testing.T) error {
	pointer := nilTuple() or err => err
	_ = *pointer // want "dereferenced"
	return nil
}
func TestReturnSuccessNil(t *testing.T) {
	expectPanic(func() { _ = testSuccessNil(t) })
	expectPanic(func() { _ = contextTestSuccessNil(t) })
}
