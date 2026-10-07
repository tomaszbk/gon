package main

import "testing"

func fatalSuccessNil(t *testing.T) {
	pointer, err := nilTuple()
	if err != nil {
		t.Fatal(err)
		return
	}
	_ = *pointer // want "dereferenced"
}
func contextFatalSuccessNil(t *testing.T) {
	pointer, err := nilTuple()
	if err != nil {
		t.Fatal(err)
		return
	}
	_ = *pointer // want "dereferenced"
}
func TestFatalSuccessNil(t *testing.T) {
	expectPanic(func() { fatalSuccessNil(t) })
	expectPanic(func() { contextFatalSuccessNil(t) })
}
