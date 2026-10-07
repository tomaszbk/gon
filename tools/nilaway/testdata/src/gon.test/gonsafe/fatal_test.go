package main

import "testing"

func TestFatalPropagation(t *testing.T) {
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
}
