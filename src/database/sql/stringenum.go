// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sql

import (
	"database/sql/driver"
	"encoding"
	"fmt"
	"reflect"
)

// stringEnumArgument identifies only the opt-in native type, including
// indirect pointers. Explicit Valuer implementations keep the driver's normal
// conversion path, rather than being replaced by the enum's text spelling.
func stringEnumArgument(value any) bool {
	for typ := reflect.TypeOf(value); typ != nil; typ = typ.Elem() {
		if typ.Implements(reflect.TypeFor[driver.Valuer]()) {
			return false
		}
		if typ.Kind() != reflect.Pointer {
			return reflect.IsStringEnum(typ)
		}
	}
	return false
}

func stringEnumDestination(dest any) bool {
	if _, ok := dest.(Scanner); ok {
		return false
	}
	typ := reflect.TypeOf(dest)
	if typ == nil || typ.Kind() != reflect.Pointer {
		return false
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return reflect.IsStringEnum(typ)
}

// scanStringEnum uses a fresh value so invalid SQL input cannot change the
// destination. NULL is distinct from the enum's present empty-string zero.
func scanStringEnum(dest reflect.Value, src any) error {
	var text []byte
	switch src := src.(type) {
	case string:
		text = []byte(src)
	case []byte:
		text = src
	case nil:
		return fmt.Errorf("converting NULL to string enum %s is unsupported", dest.Type())
	default:
		return fmt.Errorf("unsupported Scan, storing driver.Value type %T into string enum %s", src, dest.Type())
	}
	value := reflect.New(dest.Type())
	decoder := value.Interface().(encoding.TextUnmarshaler)
	if err := decoder.UnmarshalText(text); err != nil {
		return err
	}
	dest.Set(value.Elem())
	return nil
}

// A RowsColumnScanner may decode straight into a Scanner instead of calling
// ConvertAssign. Give native enum destinations the same standard interface,
// without changing user-defined Scanners or the driver's handling of Go types.
type stringEnumScanner struct {
	dest any
	rows *Rows
}

func (s stringEnumScanner) Scan(src any) error {
	return convertAssignRows(s.dest, src, s.rows)
}
