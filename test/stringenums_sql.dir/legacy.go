package main

import (
	"database/sql/driver"
	"fmt"
)

// Go's ordinary string types already cross database/sql's text boundary.
type Role string

type Alias = Role

type Custom Role

type Generic[T any] string

func (r Role) String() string       { return string(r) }
func (r Generic[T]) String() string { return string(r) }

// Legacy code supplies scanners to preserve the enum's strict text contract.
func (r *Role) Scan(src any) error {
	if r == nil {
		return fmt.Errorf("nil destination")
	}
	text, err := scanText(src)
	if err != nil {
		return err
	}
	*r = Role(text)
	return nil
}
func (r *Generic[T]) Scan(src any) error {
	if r == nil {
		return fmt.Errorf("nil destination")
	}
	text, err := scanText(src)
	if err != nil {
		return err
	}
	*r = Generic[T](text)
	return nil
}
func scanText(src any) (string, error) {
	switch src := src.(type) {
	case string:
		return src, nil
	case []byte:
		return string(src), nil
	default:
		return "", fmt.Errorf("non-text SQL input %T", src)
	}
}

// A driver accepting arbitrary arguments must convert ordinary named strings.
// The modern fixture instead verifies that database/sql normalizes native enums.
func checkDriverValue(value any) (driver.Value, error) {
	return driver.DefaultParameterConverter.ConvertValue(value)
}
func checkColumnDestination(any) {}

func teacher() Role                      { return Role("teacher") }
func student() Role                      { return Role("student") }
func unknown(text string) Role           { return Role(text) }
func ready[T any]() Generic[T]           { return Generic[T]("ready") }
func generic[T any](s string) Generic[T] { return Generic[T](s) }
func customKnown() Custom                { return Custom("teacher") }

func ordinaryEnum() any { return struct{ Payload string }{"other"} }
