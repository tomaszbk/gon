package gonapi

import "errors"

func Optional[T any](value T) T? { return value }

func Outcome(fail bool) (*int, error) {
	if fail {
		return nil, errors.New("failed")
	}
	return new(int), nil
}

func Partial() (*int, error) { return nil, nil }
