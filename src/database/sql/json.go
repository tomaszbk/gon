// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sql

import (
	"encoding/json"
	"reflect"
)

// jsonColumnType selects composite payloads that have no standard scalar SQL
// representation. Explicit Scanners and decimal composers, bytes, time, enums
// and optionals retain their existing conversion rules. This fallback is used
// only by native optionals and ScanStruct, never by Scan into ordinary Go
// destinations.
func jsonColumnType(t reflect.Type) bool {
	decimalType := reflect.TypeFor[decimalCompose]()
	for {
		if t.Implements(scannerType) || reflect.PointerTo(t).Implements(scannerType) || t.Implements(decimalType) || reflect.PointerTo(t).Implements(decimalType) || t == timeType || t == reflect.TypeFor[Rows]() || reflect.IsEnum(t) || reflect.IsOptional(t) {
			return false
		}
		if t.Kind() != reflect.Pointer {
			break
		}
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct, reflect.Map, reflect.Array:
		return true
	case reflect.Slice:
		return t.Elem().Kind() != reflect.Uint8
	}
	return false
}

func columnJSON(src any) ([]byte, bool) {
	switch src := src.(type) {
	case string:
		return []byte(src), true
	case []byte:
		return src, true
	}
	return nil, false
}

// jsonColumnScanner gives drivers with a direct column scanner the same JSON
// fallback as drivers returning ordinary driver.Values. Decode into a fresh
// value so a failed conversion or custom UnmarshalJSON cannot change dest.
type jsonColumnScanner struct{ dest any }

func (s jsonColumnScanner) Scan(src any) error {
	dest := reflect.ValueOf(s.dest).Elem()
	value := reflect.New(dest.Type())
	err := convertAssignRows(value.Interface(), src, nil)
	if err != nil {
		data, ok := columnJSON(src)
		if !ok {
			return err
		}
		value = reflect.New(dest.Type())
		if err := json.Unmarshal(data, value.Interface()); err != nil {
			return err
		}
	}
	dest.Set(value.Elem())
	return nil
}
