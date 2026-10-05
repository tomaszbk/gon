package errors_test

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrNilResult(t *testing.T) {
	if errors.ErrNilResult == nil {
		t.Fatal("ErrNilResult is nil")
	}
	if msg := errors.ErrNilResult.Error(); msg == "" {
		t.Fatal("ErrNilResult has no message")
	}
	if !errors.Is(errors.ErrNilResult, errors.ErrNilResult) {
		t.Fatal("ErrNilResult does not match itself")
	}
	wrapped := fmt.Errorf("context: %w", errors.ErrNilResult)
	if !errors.Is(wrapped, errors.ErrNilResult) {
		t.Fatal("wrapped ErrNilResult is not found")
	}
	if errors.Is(errors.New(errors.ErrNilResult.Error()), errors.ErrNilResult) {
		t.Fatal("an unrelated error with the same text matches ErrNilResult")
	}
}
