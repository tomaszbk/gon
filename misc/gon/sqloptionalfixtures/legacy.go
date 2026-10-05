// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"time"
)

// Legacy form: nullable columns use pointers.
type Role string

func role(text string) Role      { return Role(text) }
func roleText(value Role) string { return string(value) }

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

func arg[T any](value opt[T]) any { return pointer(value) }

func scanInto[T any](start opt[T], scan func(dest any) error) (opt[T], error) {
	p := pointer(start)
	err := scan(&p)
	return unpointer(p), err
}

func scanRow(scan func(dest ...any) error, start row) (row, error) {
	label, age, score, active := pointer(start.label), pointer(start.age), pointer(start.score), pointer(start.active)
	data, seen, role := pointer(start.data), pointer(start.seen), pointer(start.role)
	err := scan(&label, &age, &score, &active, &data, &seen, &role)
	return row{unpointer(label), unpointer(age), unpointer(score), unpointer(active), unpointer(data), unpointer(seen), unpointer(role)}, err
}

func scanOne(scan func(dest ...any) error) (opt[string], error) {
	var label *string
	err := scan(&label)
	return unpointer(label), err
}

// Pointer destinations are replaced before a conversion fails, so ordinary Go
// has no equivalent of the unchanged guarantee.
func failedScanKeepsValue(*sql.DB) bool { return true }

var _ = time.Time{}
