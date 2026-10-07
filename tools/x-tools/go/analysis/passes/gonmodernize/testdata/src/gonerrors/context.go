package gonerrors

import "fmt"

func context() (int, error) {
	// want +1 "replace error handler with Gon error context"
	value := read() or err {
		return 0, fmt.Errorf("read: %w", err)
	}
	return value, nil
}

func errorContextOnly() error {
	// want +1 "replace error handler with Gon error context"
	flush() or err {
		return fmt.Errorf("flush: %w", err)
	}
	return nil
}

func contextPartial() (int, error) {
	value := read() or err { return 3, err }
	return value, nil
}

func contextComments() (int, error) {
	value := read() or err {
		// Preserve this explanation.
		return 0, err
	}
	return value, nil
}

type wrappedValue struct{ Inner int }
type wrappedError struct{ Inner error }

func (w wrappedError) Error() string { return w.Inner.Error() }
func readValue() (wrappedValue, error) { return wrappedValue{42}, nil }

func contextSelector() (int, error) {
	// want +1 "replace error handler with Gon error context"
	value := readValue() or err { return 0, wrappedError{err} }.Inner
	return value, nil
}

func contextBinary() (int, error) {
	// want +1 "replace error handler with Gon error context"
	value := read() or err { return 0, wrappedError{err} } + 1
	return value, nil
}
