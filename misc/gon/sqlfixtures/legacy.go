// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"database/sql/driver"
	"errors"
)

// Ordinary Go needs the SQL protocol on each closed textual representation.
type Role struct{ text string }

func parseRole(text string) Role                { return Role{text: text} }
func (value Role) String() string               { return value.text }
func (value Role) Value() (driver.Value, error) { return value.text, nil }
func (value *Role) Scan(source any) error {
	if value == nil {
		return errors.New("nil Role destination")
	}
	text, err := scanText(source)
	if err != nil {
		return err
	}
	*value = parseRole(text)
	return nil
}

type Generic[T any] struct{ text string }

func parseGeneric[T any](text string) Generic[T]      { return Generic[T]{text: text} }
func (value Generic[T]) String() string               { return value.text }
func (value Generic[T]) Value() (driver.Value, error) { return value.text, nil }
func (value *Generic[T]) Scan(source any) error {
	if value == nil {
		return errors.New("nil Generic destination")
	}
	text, err := scanText(source)
	if err != nil {
		return err
	}
	*value = parseGeneric[T](text)
	return nil
}
