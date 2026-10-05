// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "database/sql"

// Modern form: nullable columns use native optionals, with no adapters.
type Role enum string {
	default Unknown(string)
	Teacher = "teacher"
	Student = "student"
}

func role(text string) Role      { return Role.Parse(text) }
func roleText(value Role) string { return value.String() }

func optional[T any](value opt[T]) T? {
	if !value.ok {
		return nil
	}
	return value.value
}

func unoptional[T any](value T?) opt[T] {
	return switch value {
	case nil => none[T]()
	case payload? => some(payload)
	}
}

func arg[T any](value opt[T]) any { return optional(value) }

func scanInto[T any](start opt[T], scan func(dest any) error) (opt[T], error) {
	value := optional(start)
	err := scan(&value)
	return unoptional(value), err
}

func scanRow(scan func(dest ...any) error, start row) (row, error) {
	label, age, score, active := optional(start.label), optional(start.age), optional(start.score), optional(start.active)
	data, seen, role := optional(start.data), optional(start.seen), optional(start.role)
	err := scan(&label, &age, &score, &active, &data, &seen, &role)
	return row{unoptional(label), unoptional(age), unoptional(score), unoptional(active), unoptional(data), unoptional(seen), unoptional(role)}, err
}

func scanOne(scan func(dest ...any) error) (opt[string], error) {
	var label string?
	err := scan(&label)
	return unoptional(label), err
}

// A failed conversion leaves the optional exactly as it was.
func failedScanKeepsValue(db *sql.DB) bool {
	present := optional(some(5))
	if db.QueryRow("SELECT 'abc'::text").Scan(&present) == nil || unoptional(present) != some(5) {
		return false
	}
	var absent int?
	if db.QueryRow("SELECT true").Scan(&absent) == nil || unoptional(absent) != none[int]() {
		return false
	}
	return true
}
