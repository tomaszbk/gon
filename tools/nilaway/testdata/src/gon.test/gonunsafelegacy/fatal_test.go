package main

import "testing"

func testSuccessNil(t *testing.T) error {
	pointer, err := nilTuple()
	if err != nil {
		return err
	}
	_ = *pointer // want "dereferenced"
	return nil
}
func contextTestSuccessNil(t *testing.T) error {
	pointer, err := nilTuple()
	if err != nil {
		return err
	}
	_ = *pointer // want "dereferenced"
	return nil
}
func TestReturnSuccessNil(t *testing.T) {
	expectPanic(func() { _ = testSuccessNil(t) })
	expectPanic(func() { _ = contextTestSuccessNil(t) })
}
