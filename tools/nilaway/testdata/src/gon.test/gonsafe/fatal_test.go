package main

import "testing"

func safeTestReturns(t *testing.T) error {
	value := raw()!
	if value != nil {
		_ = *value
	}
	missing := nilTuple()!
	if missing != nil {
		_ = *missing
	}
	contextual := nilTuple() or err => err
	if contextual != nil {
		_ = *contextual
	}
	return nil
}
func TestReturnPropagation(t *testing.T) {
	if err := safeTestReturns(t); err != nil {
		t.Fatal(err)
	}
}
