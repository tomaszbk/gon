// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.jsonv2

package json

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"encoding/json/internal/jsonflags"
	"encoding/json/jsontext"
)

type gonOptRole enum string {
	default Unknown(string)
	Teacher = "teacher"
}

type gonOptByPointer struct{ text string }

func (v *gonOptByPointer) MarshalJSON() ([]byte, error) {
	return []byte(`"pointer:` + v.text + `"`), nil
}
func (v *gonOptByPointer) UnmarshalJSON(data []byte) error {
	v.text = "decoded:" + string(data)
	return nil
}

type gonOptNullMarshaler struct{}

func (gonOptNullMarshaler) MarshalJSON() ([]byte, error) { return []byte(`null`), nil }

type gonOptPoint struct{ X, Y int }

type gonOptWriterOnly struct{ io.Writer }

func gonOptSome[T any](value T) T? { return value }

func gonOptAbsent(value any) bool { return !reflect.OptionalValuePresent(reflect.ValueOf(value)) }

func TestGonOptionalMarshal(t *testing.T) {
	var absent int?
	n := 4
	for _, test := range []struct {
		name  string
		value any
		want  string
	}{
		{"absent", absent, `null`},
		{"zero", gonOptSome(0), `0`},
		{"string", gonOptSome(""), `""`},
		{"false", gonOptSome(false), `false`},
		{"struct", gonOptSome(gonOptPoint{}), `{"X":0,"Y":0}`},
		{"elements", []int?{nil, 0, 5}, `[null,0,5]`},
		{"map values", map[string]int?{"a": nil, "b": 0}, `{"a":null,"b":0}`},
		{"enum", gonOptSome(gonOptRole.Teacher), `"teacher"`},
		{"pointer receiver", gonOptSome(gonOptByPointer{"a"}), `"pointer:a"`},
		{"pointer payload", gonOptSome(&n), `4`},
		// Native v2 encodes nil slices and maps as empty values, not as null.
		{"nil slice payload", gonOptSome([]int(nil)), `[]`},
		{"nil map payload", gonOptSome(map[string]int(nil)), `{}`},
	} {
		got, err := Marshal(test.value)
		if err != nil || string(got) != test.want {
			t.Errorf("%s: Marshal = %s, %v; want %s", test.name, got, err, test.want)
		}
	}
}

func TestGonOptionalMarshalOptions(t *testing.T) {
	type fields struct {
		Plain  int?    `json:"plain"`
		Empty  string? `json:"empty,omitempty"`
		Zero   int?    `json:"zero,omitzero"`
		Number int?    `json:"number,string"`
	}
	got, err := Marshal(fields{})
	if err != nil || string(got) != `{"plain":null,"number":null}` {
		t.Errorf("absent = %s, %v", got, err)
	}
	// omitzero keeps a present zero. omitempty follows v2: an empty JSON
	// value is omitted, which includes a present empty string.
	got, err = Marshal(fields{Plain: 0, Empty: "x", Zero: 0, Number: 0})
	if err != nil || string(got) != `{"plain":0,"empty":"x","zero":0,"number":"0"}` {
		t.Errorf("present = %s, %v", got, err)
	}
	got, err = Marshal(fields{Empty: ""})
	if err != nil || string(got) != `{"plain":null,"number":null}` {
		t.Errorf("empty string = %s, %v", got, err)
	}
	// With the legacy definition, only absence is empty.
	got, err = Marshal(fields{Empty: ""}, jsonflags.OmitEmptyWithLegacySemantics|1)
	if err != nil || string(got) != `{"plain":null,"empty":"","number":null}` {
		t.Errorf("legacy omitempty = %s, %v", got, err)
	}
}

func TestGonOptionalMarshalErrors(t *testing.T) {
	var nilPointer *int
	var nilInterface any
	for name, value := range map[string]any{
		"nil pointer":    gonOptSome(nilPointer),
		"nil interface":  gonOptSome(nilInterface),
		"marshaler null": gonOptSome(gonOptNullMarshaler{}),
		"in struct":      struct{ A (*int)? }{A: nilPointer},
		"in slice":       []any{gonOptSome(1), []any{gonOptSome(nilPointer)}},
		"in map":         map[string]any{"k": gonOptSome(nilPointer)},
	} {
		_, err := Marshal(value)
		var semantic *SemanticError
		if !errors.As(err, &semantic) || !strings.Contains(err.Error(), "encodes as null") {
			t.Errorf("%s: Marshal error = %v", name, err)
		}
		// Streaming must not flush a null before the check can run.
		var out bytes.Buffer
		err = MarshalWrite(gonOptWriterOnly{&out}, value)
		if !errors.As(err, &semantic) || !strings.Contains(err.Error(), "encodes as null") {
			t.Errorf("%s: MarshalWrite error = %v", name, err)
		}
	}
	// With the legacy definition, a nil slice is null and therefore rejected.
	if _, err := Marshal(gonOptSome([]int(nil)), FormatNilSliceAsNull(true)); err == nil {
		t.Error("nil slice written as null accepted")
	}
	var nested (int?)?
	for name, value := range map[string]any{
		"absent":  nested,
		"present": gonOptSome(gonOptSome(1)),
		"key":     map[int?]string{},
	} {
		var semantic *SemanticError
		if _, err := Marshal(value); !errors.As(err, &semantic) {
			t.Errorf("%s: error = %v", name, err)
		}
	}
}

func TestGonOptionalStreaming(t *testing.T) {
	n := 4
	big := make([]int, 20000)
	for _, value := range []any{
		gonOptSome(&n),
		gonOptSome(&[]int{1, 2}),
		gonOptSome(any(gonOptPoint{1, 2})),
		[]any{gonOptSome(&n), gonOptSome(gonOptByPointer{"x"})},
		struct{ A (*int)? }{A: &n},
		gonOptSome(&big),
	} {
		want, err := Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := MarshalWrite(gonOptWriterOnly{&out}, value); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out.Bytes(), want) {
			t.Errorf("MarshalWrite(%T) differs from Marshal", value)
		}
		out.Reset()
		enc := jsontext.NewEncoder(gonOptWriterOnly{&out})
		if err := MarshalEncode(enc, value); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(bytes.TrimSpace(out.Bytes()), want) {
			t.Errorf("MarshalEncode(%T) differs from Marshal", value)
		}
	}
}

func TestGonOptionalUnmarshal(t *testing.T) {
	var value struct {
		Count  int?                `json:"count"`
		Role   gonOptRole?         `json:"role"`
		Point  gonOptPoint?        `json:"point"`
		List   []int?              `json:"list"`
		ByPtr  gonOptByPointer?    `json:"byptr"`
		Number int?                `json:"number,string"`
		Map    map[string]float64? `json:"map"`
	}
	in := `{"count":0,"role":"teacher","point":{"X":1},"list":[null,2],"byptr":3,"number":"7","map":{"a":null,"b":1.5}}`
	if err := Unmarshal([]byte(in), &value); err != nil {
		t.Fatal(err)
	}
	if value.Count != gonOptSome(0) || value.Role != gonOptSome(gonOptRole.Teacher) || value.Point != gonOptSome(gonOptPoint{1, 0}) ||
		len(value.List) != 2 || !gonOptAbsent(value.List[0]) || value.List[1] != gonOptSome(2) ||
		value.ByPtr != gonOptSome(gonOptByPointer{"decoded:3"}) || value.Number != gonOptSome(7) ||
		len(value.Map) != 2 || !gonOptAbsent(value.Map["a"]) || value.Map["b"] != gonOptSome(1.5) {
		t.Errorf("Unmarshal = %+v", value)
	}
	if err := Unmarshal([]byte(`{"count":null,"role":null,"point":null,"number":null}`), &value); err != nil {
		t.Fatal(err)
	}
	if !gonOptAbsent(value.Count) || !gonOptAbsent(value.Role) || !gonOptAbsent(value.Point) || !gonOptAbsent(value.Number) {
		t.Errorf("null = %+v", value)
	}
	// A type error leaves the optional unchanged.
	value.Count = 5
	err := Unmarshal([]byte(`{"count":"x"}`), &value)
	var semantic *SemanticError
	if !errors.As(err, &semantic) || value.Count != gonOptSome(5) {
		t.Errorf("type error = %v, count = %v", err, value.Count)
	}
	value.Count = nil
	if err := Unmarshal([]byte(`{"count":"x"}`), &value); err == nil || !gonOptAbsent(value.Count) {
		t.Errorf("type error on absent = %v, count = %v", err, value.Count)
	}
	var nested (int?)?
	for _, text := range []string{`1`, `null`, `[]`} {
		if err := Unmarshal([]byte(text), &nested); !errors.As(err, &semantic) {
			t.Errorf("nested optional %s = %v", text, err)
		}
	}
	var keyed map[int?]string
	if err := Unmarshal([]byte(`{"1":"x"}`), &keyed); !errors.As(err, &semantic) {
		t.Errorf("optional map key = %v", err)
	}
}
