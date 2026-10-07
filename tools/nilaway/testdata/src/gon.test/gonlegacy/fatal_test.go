package main

import "testing"

func TestFatalPropagation(t *testing.T) {
	value, err := raw()
	if err != nil {
		t.Fatal(err)
		return
	}
	if value != nil {
		_ = *value
	}
	missing, err := nilTuple()
	if err != nil {
		t.Fatal(err)
		return
	}
	if missing != nil {
		_ = *missing
	}
	contextual, err := nilTuple()
	if err != nil {
		t.Fatal(err)
		return
	}
	if contextual != nil {
		_ = *contextual
	}
}
