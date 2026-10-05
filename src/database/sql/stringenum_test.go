// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sql

import (
	"database/sql/driver"
	"errors"
	"testing"
	"time"
)

type gonSQLRole enum string {
	default Unknown(string)
	Teacher = "teacher"
	Student = "student"
}

type gonSQLAlias = gonSQLRole

type gonSQLGeneric[T any] enum string {
	default Unknown(string)
	Ready = "ready"
}

func TestGonStringEnumScan(t *testing.T) {
	for _, test := range []struct {
		name string
		src  any
		want gonSQLRole
	}{
		{"known", "teacher", gonSQLRole.Teacher},
		{"unknown", "future", gonSQLRole.Unknown("future")},
		{"bytes", []byte("student"), gonSQLRole.Student},
		{"unknown-bytes", []byte("future"), gonSQLRole.Unknown("future")},
		{"empty", "", gonSQLRole.Unknown("")},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := gonSQLRole.Student
			if err := convertAssign(&value, test.src); err != nil {
				t.Fatal(err)
			}
			if value != test.want {
				t.Fatalf("Scan = %v; want %v", value, test.want)
			}
		})
	}
	data := []byte("borrowed")
	var alias gonSQLAlias
	if err := convertAssign(&alias, data); err != nil {
		t.Fatal(err)
	}
	data[0] = 'X'
	if alias != gonSQLRole.Unknown("borrowed") {
		t.Fatal("retained mutable driver buffer")
	}
	var generic gonSQLGeneric[int]
	if err := convertAssign(&generic, "ready"); err != nil {
		t.Fatal(err)
	}
	if generic != gonSQLGeneric[int].Ready {
		t.Fatal("generic enum did not parse known variant")
	}
}

func TestGonStringEnumScanRejectsNonText(t *testing.T) {
	for _, src := range []any{nil, int64(1), float64(1), true, time.Unix(0, 0), struct{}{}, gonSQLRole.Teacher} {
		value := gonSQLRole.Student
		if err := convertAssign(&value, src); err == nil {
			t.Fatalf("accepted non-text input %T", src)
		}
		if value != gonSQLRole.Student {
			t.Fatalf("failed input %T changed destination", src)
		}
	}
	var nilDest *gonSQLRole
	if err := convertAssign(nilDest, "teacher"); !errors.Is(err, errNilPtr) {
		t.Fatalf("nil destination error = %v; want %v", err, errNilPtr)
	}
	if err := convertAssign(gonSQLRole.Student, "teacher"); err == nil {
		t.Fatal("accepted non-pointer destination")
	}
}

func TestGonStringEnumSQLPointersAndNull(t *testing.T) {
	var pointer *gonSQLRole
	if err := convertAssign(&pointer, "teacher"); err != nil {
		t.Fatal(err)
	}
	if pointer == nil || *pointer != gonSQLRole.Teacher {
		t.Fatal("pointer destination did not allocate enum")
	}
	if err := convertAssign(&pointer, nil); err != nil {
		t.Fatal(err)
	}
	if pointer != nil {
		t.Fatal("SQL NULL did not clear pointer")
	}
	var nullable Null[gonSQLRole]
	if err := nullable.Scan("teacher"); err != nil {
		t.Fatal(err)
	}
	if !nullable.Valid || nullable.V != gonSQLRole.Teacher {
		t.Fatal("generic Null failed known text")
	}
	value, err := nullable.Value()
	if err != nil || value != "teacher" {
		t.Fatalf("generic Null Value = %v, %v", value, err)
	}
	if err := nullable.Scan(nil); err != nil {
		t.Fatal(err)
	}
	value, err = nullable.Value()
	if err != nil || nullable.Valid || value != nil {
		t.Fatalf("generic Null absence = %+v, %v, %v", nullable, value, err)
	}
	if err := nullable.Scan(""); err != nil {
		t.Fatal(err)
	}
	if !nullable.Valid || nullable.V != gonSQLRole.Unknown("") {
		t.Fatal("empty text was confused with SQL NULL")
	}
}

type gonSQLCustom enum string {
	default Unknown(string)
	Known = "known"
}

func (*gonSQLCustom) Scan(src any) error          { return gonSQLScanError }
func (gonSQLCustom) Value() (driver.Value, error) { return "custom-valuer", nil }

var gonSQLScanError = errors.New("custom scanner")

func TestGonStringEnumExplicitSQLProtocols(t *testing.T) {
	value := gonSQLCustom.Known
	if err := convertAssign(&value, "other"); !errors.Is(err, gonSQLScanError) {
		t.Fatalf("explicit Scanner error = %v; want %v", err, gonSQLScanError)
	}
	if value != gonSQLCustom.Known {
		t.Fatal("explicit Scanner was bypassed")
	}
	converted, err := driver.DefaultParameterConverter.ConvertValue(value)
	if err != nil || converted != "custom-valuer" {
		t.Fatalf("explicit Valuer = %v, %v", converted, err)
	}
	if stringEnumArgument(value) || stringEnumArgument(&value) {
		t.Fatal("explicit Valuer selected for enum pre-conversion")
	}
	if stringEnumDestination(&value) {
		t.Fatal("explicit Scanner selected for enum driver wrapper")
	}
}

type gonSQLTextStruct struct{ value string }

func (v gonSQLTextStruct) MarshalText() ([]byte, error)     { return []byte(v.value), nil }
func (v *gonSQLTextStruct) UnmarshalText(data []byte) error { v.value = string(data); return nil }

func TestGonStringEnumSQLBoundaryOptIn(t *testing.T) {
	value := gonSQLTextStruct{"original"}
	if err := convertAssign(&value, "changed"); err == nil {
		t.Fatal("plain TextUnmarshaler unexpectedly became Scanner")
	}
	if value.value != "original" {
		t.Fatal("plain text destination changed")
	}
	if _, err := driver.DefaultParameterConverter.ConvertValue(value); err == nil {
		t.Fatal("plain TextMarshaler unexpectedly became Valuer")
	}
	if stringEnumArgument(value) || stringEnumDestination(&value) {
		t.Fatal("ordinary Go text type treated as native enum")
	}
}
