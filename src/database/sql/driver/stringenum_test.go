// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import "testing"

type gonDriverRole enum string {
	default Unknown(string)
	Teacher = "teacher"
}

type gonDriverAlias = gonDriverRole

type gonDriverGeneric[T any] enum string {
	default Unknown(string)
	Ready = "ready"
}

func TestGonStringEnumParameterConverter(t *testing.T) {
	role := gonDriverRole.Teacher
	pointer := &role
	var nilRole *gonDriverRole
	for _, test := range []struct{ input, want any }{
		{role, "teacher"}, {pointer, "teacher"}, {&pointer, "teacher"}, {nilRole, nil},
		{gonDriverRole.Unknown("future"), "future"}, {gonDriverRole.Unknown(""), ""},
		{gonDriverAlias.Teacher, "teacher"}, {gonDriverGeneric[int].Ready, "ready"},
		{gonDriverGeneric[string].Unknown("future-generic"), "future-generic"},
	} {
		got, err := DefaultParameterConverter.ConvertValue(test.input)
		if err != nil || got != test.want {
			t.Errorf("ConvertValue(%T) = %v, %v; want %v", test.input, got, err, test.want)
		}
		if IsValue(test.input) {
			t.Errorf("native enum %T incorrectly accepted as driver.Value", test.input)
		}
	}
}

type gonDriverCustom enum string {
	default Unknown(string)
	Known = "known"
}

func (gonDriverCustom) Value() (Value, error) { return int64(73), nil }

func TestGonStringEnumExplicitValuer(t *testing.T) {
	value := gonDriverCustom.Known
	for _, input := range []any{value, &value} {
		got, err := DefaultParameterConverter.ConvertValue(input)
		if err != nil || got != int64(73) {
			t.Fatalf("explicit Value = %v, %v", got, err)
		}
	}
	var nilValue *gonDriverCustom
	got, err := DefaultParameterConverter.ConvertValue(nilValue)
	if err != nil || got != nil {
		t.Fatalf("nil explicit value receiver = %v, %v", got, err)
	}
}

type gonDriverTextStruct struct{}

func (gonDriverTextStruct) MarshalText() ([]byte, error) { return []byte("plain"), nil }

func TestGonStringEnumParameterOptIn(t *testing.T) {
	if _, err := DefaultParameterConverter.ConvertValue(gonDriverTextStruct{}); err == nil {
		t.Fatal("plain TextMarshaler unexpectedly accepted")
	}
	type Ordinary enum {
		default Missing
		Known(string)
	}
	if _, err := DefaultParameterConverter.ConvertValue(Ordinary.Known("text")); err == nil {
		t.Fatal("ordinary enum unexpectedly accepted")
	}
	for _, converter := range []ValueConverter{Int32, Bool} {
		if _, err := converter.ConvertValue(gonDriverRole.Teacher); err == nil {
			t.Fatalf("non-text converter %T unexpectedly accepted enum", converter)
		}
	}
}
