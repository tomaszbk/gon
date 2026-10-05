// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"testing"
	"uuid"
)

type gonDriverOptRole enum string {
	default Unknown(string)
	Teacher = "teacher"
}

type gonDriverOptValuer struct{ n int }

func (v gonDriverOptValuer) Value() (Value, error) { return int64(v.n * 2), nil }

func gonDriverOptSome[T any](value T) T? { return value }

func TestGonOptionalParameterConverter(t *testing.T) {
	var absent int?
	zero := gonDriverOptSome(0)
	var nilPointer *int?
	var nilRole *gonDriverOptRole
	id := uuid.MustParse("46cd2740-6081-4289-a659-03b61ebb92f7")
	for _, test := range []struct{ input, want any }{
		{absent, nil},
		{zero, int64(0)},
		{&zero, int64(0)},
		{&absent, nil},
		{nilPointer, nil},
		{gonDriverOptSome(7), int64(7)},
		{gonDriverOptSome(uint8(7)), int64(7)},
		{gonDriverOptSome(0.0), 0.0},
		{gonDriverOptSome(""), ""},
		{gonDriverOptSome(false), false},
		{gonDriverOptSome([]byte(nil)), []byte(nil)},
		{gonDriverOptSome(id), id.String()},
		{gonDriverOptSome(gonDriverOptRole.Teacher), "teacher"},
		{gonDriverOptSome(gonDriverOptRole.Unknown("")), ""},
		{gonDriverOptSome(gonDriverOptValuer{4}), int64(8)},
		{gonDriverOptSome(nilRole), nil},
		{gonDriverOptSome((*int)(nil)), nil},
	} {
		got, err := DefaultParameterConverter.ConvertValue(test.input)
		if err != nil {
			t.Errorf("ConvertValue(%#v): %v", test.input, err)
			continue
		}
		if b, ok := test.want.([]byte); ok {
			if gb, ok := got.([]byte); !ok || len(gb) != len(b) {
				t.Errorf("ConvertValue(%#v) = %#v; want %#v", test.input, got, test.want)
			}
		} else if got != test.want {
			t.Errorf("ConvertValue(%#v) = %#v; want %#v", test.input, got, test.want)
		}
		if IsValue(test.input) {
			t.Errorf("native optional %T incorrectly accepted as driver.Value", test.input)
		}
	}
	nested := gonDriverOptSome(gonDriverOptSome(1))
	if _, err := DefaultParameterConverter.ConvertValue(nested); err == nil {
		t.Error("nested optional accepted")
	}
	// Other converters keep rejecting an optional.
	for _, converter := range []ValueConverter{Int32, Bool} {
		if _, err := converter.ConvertValue(gonDriverOptSome(1)); err == nil {
			t.Errorf("converter %T unexpectedly accepted an optional", converter)
		}
	}
	// A payload without a conversion is an error, as when passed directly.
	if _, err := DefaultParameterConverter.ConvertValue(gonDriverOptSome(struct{}{})); err == nil {
		t.Error("unsupported payload accepted")
	}
	// Absence is NULL even when the payload type has no conversion.
	var unsupported struct{}?
	if got, err := DefaultParameterConverter.ConvertValue(unsupported); err != nil || got != nil {
		t.Errorf("absent unsupported payload = %v, %v", got, err)
	}
}
