// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sql

import (
	"fmt"
	"reflect"
)

// A Gon native optional T? is SQL NULL when absent. When present, it behaves
// as its payload would: an argument is converted as if the payload had been
// passed, and a destination receives a value converted as for a *T.

// optionalType reports whether t, possibly behind pointers, is a native
// optional.
func optionalType(t reflect.Type) bool {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t != nil && t.Kind() == reflect.Struct && reflect.IsOptional(t)
}

// optionalArgument unwraps a native optional argument, including a pointer to
// one, before driver-specific checkers see it. Absence and nil pointers become
// SQL NULL; a present optional becomes its payload, which is then converted
// like any other argument. It reports false for every other argument.
func optionalArgument(arg any) (value any, ok bool, err error) {
	if !optionalType(reflect.TypeOf(arg)) {
		return nil, false, nil
	}
	v := reflect.ValueOf(arg)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, true, nil
		}
		v = v.Elem()
	}
	if !reflect.OptionalValuePresent(v) {
		return nil, true, nil
	}
	payload := reflect.OptionalValuePayload(v)
	if reflect.IsOptional(payload.Type()) {
		return nil, true, fmt.Errorf("nested optional %s is unsupported", v.Type())
	}
	return payload.Interface(), true, nil
}

// optionalDestination identifies a pointer to a native optional that is not an
// explicit Scanner, so that direct column scanners receive the standard
// conversion instead of an unknown struct.
func optionalDestination(dest any) bool {
	if _, ok := dest.(Scanner); ok {
		return false
	}
	typ := reflect.TypeOf(dest)
	return typ != nil && typ.Kind() == reflect.Pointer && optionalType(typ)
}

// scanOptional stores src in the optional dest. NULL makes it absent. Any
// other value is converted into a temporary payload with the rules for a *T,
// including string enums and Scanner payloads, and stored as present only if
// the conversion succeeded, so a failure leaves dest unchanged.
func scanOptional(dest reflect.Value, src any, rows *Rows) error {
	elem := reflect.OptionalElement(dest.Type())
	if reflect.IsOptional(elem) {
		return fmt.Errorf("converting to nested optional %s is unsupported", dest.Type())
	}
	if elem == reflect.TypeFor[RawBytes]() {
		// Rows.Scan must hold the connection while RawBytes is in use, which
		// it can only tell for a direct *RawBytes destination.
		return fmt.Errorf("converting to %s is unsupported; scan into *RawBytes", dest.Type())
	}
	if src == nil {
		dest.SetZero()
		return nil
	}
	payload := reflect.New(elem)
	if err := convertAssignRows(payload.Interface(), src, rows); err != nil {
		return err
	}
	reflect.OptionalValueSetPayload(dest, payload.Elem())
	return nil
}
