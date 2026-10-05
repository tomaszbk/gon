package main

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
)

// Legacy form: nullable columns use pointers.
type Role string

func role(text string) Role { return Role(text) }

func pointer[T any](value opt[T]) *T {
	if !value.ok {
		return nil
	}
	return &value.value
}

func unpointer[T any](value *T) opt[T] {
	if value == nil {
		return none[T]()
	}
	return some(*value)
}

// arg passes a pointer for an optional value.
func arg[T any](value opt[T]) any { return pointer(value) }

// scanInto scans into a pointer that starts out as start.
func scanInto[T any](start opt[T], scan func(dest any) error) (opt[T], error) {
	p := pointer(start)
	err := scan(&p)
	return unpointer(p), err
}

func scanTuple(rows *sql.Rows) (string, string, string, string) {
	var a *int
	var b *float64
	var c *string
	var d *bool
	require(rows.Scan(&a, &b, &c, &d))
	return unpointer(a).String(), unpointer(b).String(), unpointer(c).String(), unpointer(d).String()
}

// A driver accepting arbitrary arguments converts pointers itself.
func checkDriverValue(value any) (driver.Value, error) {
	return driver.DefaultParameterConverter.ConvertValue(value)
}
func checkColumnDestination(any) {}

func checkModern(*sql.DB, *store) {}

var _ = fmt.Sprint
