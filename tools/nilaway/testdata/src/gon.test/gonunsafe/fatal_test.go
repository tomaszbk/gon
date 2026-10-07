package main

import "testing"

func fatalSuccessNil(t *testing.T) {
	pointer := nilTuple()!
	_ = *pointer // want "dereferenced"
}
func contextFatalSuccessNil(t *testing.T) {
	pointer := nilTuple() or err => err
	_ = *pointer // want "dereferenced"
}
func TestFatalSuccessNil(t *testing.T) {
	expectPanic(func() { fatalSuccessNil(t) })
	expectPanic(func() { contextFatalSuccessNil(t) })
}
