// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflect_test

import (
	. "reflect"
	"testing"
)

type optionalPoint struct{ X, Y int }

type optionalHolder struct {
	Exported   int?
	unexported int?
}

func optionalMustPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s did not panic", name)
		}
	}()
	f()
}

func TestOptionalValueSetPayload(t *testing.T) {
	var number int?
	v := ValueOf(&number).Elem()
	if OptionalValuePresent(v) {
		t.Fatal("zero optional is present")
	}
	OptionalValueSetPayload(v, ValueOf(0))
	if !OptionalValuePresent(v) || OptionalValuePayload(v).Int() != 0 || number != (int?)(0) {
		t.Fatalf("present zero = %v", number)
	}
	OptionalValueSetPayload(v, ValueOf(7))
	if got := OptionalValuePayload(v).Int(); got != 7 || number != (int?)(7) {
		t.Fatalf("payload = %d, %v", got, number)
	}
	// The optional stores a copy of the payload.
	payload := ValueOf(new(int)).Elem()
	payload.SetInt(9)
	OptionalValueSetPayload(v, payload)
	payload.SetInt(10)
	if got := OptionalValuePayload(v).Int(); got != 9 {
		t.Fatalf("stored payload aliases its source: %d", got)
	}
	v.SetZero()
	if OptionalValuePresent(v) || !v.IsZero() {
		t.Fatal("SetZero did not make the optional absent")
	}
}

func TestOptionalValueSetPayloadTypes(t *testing.T) {
	var text string?
	var point optionalPoint?
	var wantPoint optionalPoint? = optionalPoint{1, 2}
	var pointer (*int)?
	var slice ([]int)?
	var iface any?
	n := 3
	for _, test := range []struct {
		name    string
		dest    any
		payload any
	}{
		{"string", &text, ""},
		{"struct", &point, optionalPoint{1, 2}},
		{"pointer", &pointer, &n},
		{"nil pointer", &pointer, (*int)(nil)},
		{"nil slice", &slice, []int(nil)},
		{"interface", &iface, "x"},
		{"nil interface", &iface, nil},
	} {
		dest := ValueOf(test.dest).Elem()
		payload := ValueOf(test.payload)
		if test.payload == nil {
			payload = Zero(TypeFor[any]())
		}
		OptionalValueSetPayload(dest, payload)
		if !OptionalValuePresent(dest) {
			t.Errorf("%s: optional is absent", test.name)
			continue
		}
		if !DeepEqual(OptionalValuePayload(dest).Interface(), test.payload) {
			t.Errorf("%s: payload = %#v; want %#v", test.name, OptionalValuePayload(dest).Interface(), test.payload)
		}
	}
	if point != wantPoint {
		t.Errorf("struct payload = %v", point)
	}
	// A present nil payload remains distinct from absence.
	if got := OptionalValuePayload(ValueOf(&slice).Elem()); !got.IsNil() {
		t.Error("nil slice payload lost")
	}
	// Elements of containers and struct fields are settable optionals.
	elements := make([]int?, 2)
	OptionalValueSetPayload(ValueOf(elements).Index(1), ValueOf(5))
	if !DeepEqual(elements, []int?{nil, 5}) {
		t.Errorf("elements = %v", elements)
	}
	var holder optionalHolder
	OptionalValueSetPayload(ValueOf(&holder).Elem().Field(0), ValueOf(6))
	if holder.Exported != (int?)(6) {
		t.Errorf("field = %v", holder.Exported)
	}
}

func TestOptionalValueSetPayloadPanics(t *testing.T) {
	var number int?
	var holder optionalHolder
	optionalMustPanic(t, "invalid optional", func() { OptionalValueSetPayload(Value{}, ValueOf(1)) })
	optionalMustPanic(t, "non-optional", func() {
		x := 1
		OptionalValueSetPayload(ValueOf(&x).Elem(), ValueOf(1))
	})
	optionalMustPanic(t, "unaddressable", func() { OptionalValueSetPayload(ValueOf(number), ValueOf(1)) })
	optionalMustPanic(t, "unexported field", func() {
		OptionalValueSetPayload(ValueOf(&holder).Elem().Field(1), ValueOf(1))
	})
	optionalMustPanic(t, "mismatched payload", func() {
		OptionalValueSetPayload(ValueOf(&number).Elem(), ValueOf("x"))
	})
	optionalMustPanic(t, "invalid payload", func() { OptionalValueSetPayload(ValueOf(&number).Elem(), Value{}) })
	if OptionalValuePresent(ValueOf(&number).Elem()) {
		t.Error("failed set changed the optional")
	}
}

func TestOptionalValueSetPayloadZeroSized(t *testing.T) {
	var unit struct{}?
	var array ([0]int)?
	for _, test := range []struct {
		dest    any
		payload any
	}{
		{&unit, struct{}{}},
		{&array, [0]int{}},
	} {
		dest := ValueOf(test.dest).Elem()
		OptionalValueSetPayload(dest, ValueOf(test.payload))
		if !OptionalValuePresent(dest) || OptionalValuePayload(dest).Type() != TypeOf(test.payload) {
			t.Errorf("%T: zero-sized payload was not stored", test.dest)
		}
		dest.SetZero()
		if OptionalValuePresent(dest) {
			t.Errorf("%T: SetZero left the optional present", test.dest)
		}
	}
}
