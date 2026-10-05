// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

type gonRole enum string {
	default Unknown(string)
	Teacher = "teacher"
	Student = "student"
}

type gonPoint struct {
	X, Y int
}

type gonByValue struct{ text string }

func (v gonByValue) MarshalJSON() ([]byte, error) { return []byte(`"value:` + v.text + `"`), nil }
func (v *gonByValue) UnmarshalJSON(data []byte) error {
	v.text = "decoded:" + string(data)
	return nil
}

type gonByPointer struct{ text string }

func (v *gonByPointer) MarshalJSON() ([]byte, error) { return []byte(`"pointer:` + v.text + `"`), nil }
func (v *gonByPointer) UnmarshalJSON(data []byte) error {
	v.text = "decoded:" + string(data)
	return nil
}

type gonText struct{ text string }

func (v gonText) MarshalText() ([]byte, error) { return []byte("text:" + v.text), nil }
func (v *gonText) UnmarshalText(data []byte) error {
	v.text = "decoded:" + string(data)
	return nil
}

type gonNested struct {
	Pointer gonByPointer
}

type gonNullMarshaler struct{}

func (gonNullMarshaler) MarshalJSON() ([]byte, error) { return []byte(" null "), nil }

type gonFailingMarshaler struct{}

var errGonMarshal = errors.New("gon marshal failure")

func (gonFailingMarshaler) MarshalJSON() ([]byte, error) { return nil, errGonMarshal }

type gonOptionalFields struct {
	Int     int?            `json:"int"`
	Float   float64?        `json:"float"`
	String  string?         `json:"string"`
	Bool    bool?           `json:"bool"`
	Point   gonPoint?       `json:"point"`
	Slice   ([]int)?        `json:"slice"`
	Pointer (*int)?         `json:"pointer"`
	Elems   []int?          `json:"elems"`
	Map     map[string]int? `json:"map"`
	Role    gonRole?        `json:"role"`
}

type gonOmit struct {
	Empty int?     `json:"empty,omitempty"`
	Zero  int?     `json:"zero,omitzero"`
	Both  string?  `json:"both,omitempty,omitzero"`
	Plain float64? `json:"plain"`
	Slice ([]int)? `json:"slice,omitempty"`
	Role  gonRole? `json:"role,omitempty"`
}

// writerOnly hides the concrete writer type from the encoder.
type writerOnly struct{ io.Writer }

// absent reports whether the native optional value is absent.
func absent(value any) bool { return !reflect.OptionalValuePresent(reflect.ValueOf(value)) }

// some constructs a present optional holding value, whatever its type.
func some[T any](value T) T? { return value }

func marshalString(t *testing.T, value any) string {
	t.Helper()
	data, err := Marshal(value)
	if err != nil {
		t.Fatalf("Marshal(%#v): %v", value, err)
	}
	return string(data)
}

func TestGonOptionalMarshal(t *testing.T) {
	var absent int?
	zero := some[int](0)
	seven := some[int](7)
	for _, test := range []struct {
		name  string
		value any
		want  string
	}{
		{"absent", absent, `null`},
		{"zero", zero, `0`},
		{"present", seven, `7`},
		{"pointer to absent", &absent, `null`},
		{"pointer to present", &seven, `7`},
		{"nil pointer to optional", (*int?)(nil), `null`},
		{"interface", []any{absent, zero, "x"}, `[null,0,"x"]`},
		{"slice", []int?{nil, 0, 5}, `[null,0,5]`},
		{"array", [3]string?{nil, "", "a"}, `[null,"","a"]`},
		{"map", map[string]float64?{"a": nil, "b": 0, "c": 1.5}, `{"a":null,"b":0,"c":1.5}`},
		{"struct payload", some[gonPoint](gonPoint{}), `{"X":0,"Y":0}`},
		{"empty slice payload", some([]int{}), `[]`},
		{"empty map payload", some(map[string]int{}), `{}`},
		{"bool payload", some[bool](false), `false`},
		{"string payload", some[string](""), `""`},
		{"html escaping", some[string]("<&>"), `"\u003c\u0026\u003e"`},
		{"enum payload", some[gonRole](gonRole.Teacher), `"teacher"`},
		{"unknown enum payload", some[gonRole](gonRole.Unknown("future")), `"future"`},
		{"value marshaler", some[gonByValue](gonByValue{"a"}), `"value:a"`},
		{"pointer marshaler", some[gonByPointer](gonByPointer{"a"}), `"pointer:a"`},
		{"text marshaler", some[gonText](gonText{"a"}), `"text:a"`},
		{"nested pointer marshaler", some[gonNested](gonNested{gonByPointer{"a"}}), `{"Pointer":"pointer:a"}`},
		{"payload pointer", some(new(int)), `0`},
	} {
		if got := marshalString(t, test.value); got != test.want {
			t.Errorf("%s: Marshal = %s; want %s", test.name, got, test.want)
		}
	}
	// Indented output and html escaping options use the same encoders.
	data, err := MarshalIndent(map[string]int?{"a": 1, "b": nil}, "", " ")
	if err != nil || string(data) != "{\n \"a\": 1,\n \"b\": null\n}" {
		t.Errorf("MarshalIndent = %q, %v", data, err)
	}
}

func TestGonOptionalMarshalStruct(t *testing.T) {
	var fields gonOptionalFields
	want := `{"int":null,"float":null,"string":null,"bool":null,"point":null,"slice":null,"pointer":null,"elems":null,"map":null,"role":null}`
	if got := marshalString(t, fields); got != want {
		t.Errorf("absent fields = %s; want %s", got, want)
	}
	n := 3
	fields = gonOptionalFields{
		Int: 0, Float: 0.0, String: "", Bool: false, Point: gonPoint{},
		Slice: []int{}, Pointer: &n, Elems: []int?{nil, 1}, Map: map[string]int?{"k": nil},
		Role: gonRole.Student,
	}
	want = `{"int":0,"float":0,"string":"","bool":false,"point":{"X":0,"Y":0},"slice":[],"pointer":3,"elems":[null,1],"map":{"k":null},"role":"student"}`
	if got := marshalString(t, fields); got != want {
		t.Errorf("present fields = %s; want %s", got, want)
	}
	// Optional fields are encoded through addressable and unaddressable paths.
	if got := marshalString(t, &fields); got != want {
		t.Errorf("addressable = %s; want %s", got, want)
	}
}

func TestGonOptionalOmit(t *testing.T) {
	if got := marshalString(t, gonOmit{}); got != `{"plain":null}` {
		t.Errorf("absent = %s", got)
	}
	// A present zero payload is kept, like a non-nil pointer to a zero value.
	present := gonOmit{Empty: 0, Zero: 0, Both: "", Plain: 0, Slice: []int{}, Role: gonRole.Unknown("")}
	want := `{"empty":0,"zero":0,"both":"","plain":0,"slice":[],"role":""}`
	if got := marshalString(t, present); got != want {
		t.Errorf("present zero = %s; want %s", got, want)
	}
	if got := marshalString(t, &present); got != want {
		t.Errorf("present zero through pointer = %s; want %s", got, want)
	}
	var nilSlice ([]int)?
	if got := marshalString(t, gonOmit{Slice: nilSlice}); got != `{"plain":null}` {
		t.Errorf("absent slice = %s", got)
	}
}

func TestGonOptionalStringOption(t *testing.T) {
	type quoted struct {
		A int?     `json:"a,string"`
		B float64? `json:"b,string"`
		C bool?    `json:"c,string"`
		D string?  `json:"d,string"`
		E *int?    `json:"e,string"`
	}
	zero := some[int](0)
	got := marshalString(t, quoted{A: 5, B: 0.5, C: false, D: "s", E: &zero})
	if want := `{"a":"5","b":"0.5","c":"false","d":"\"s\"","e":"0"}`; got != want {
		t.Errorf("Marshal = %s; want %s", got, want)
	}
	if got := marshalString(t, quoted{}); got != `{"a":null,"b":null,"c":null,"d":null,"e":null}` {
		t.Errorf("absent = %s", got)
	}
	var back quoted
	if err := Unmarshal([]byte(`{"a":"5","b":"0.5","c":"false","d":"\"s\"","e":"0"}`), &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, quoted{A: 5, B: 0.5, C: false, D: "s", E: &zero}) {
		t.Errorf("round trip = %+v", back)
	}
	if err := Unmarshal([]byte(`{"a":"null","b":null}`), &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, quoted{C: false, D: "s", E: &zero}) {
		t.Errorf("null = %+v", back)
	}
	if err := Unmarshal([]byte(`{"a":5}`), &back); err == nil {
		t.Error("unquoted number accepted with ,string")
	}
}

func TestGonOptionalMarshalErrors(t *testing.T) {
	var nilPointer *int
	var nilSlice []int
	var nilMap map[string]int
	var nilInterface any
	for _, test := range []struct {
		name  string
		value any
	}{
		{"nil pointer", some(nilPointer)},
		{"nil slice", some(nilSlice)},
		{"nil map", some(nilMap)},
		{"nil interface", some[any](nilInterface)},
		{"marshaler null", some[gonNullMarshaler](gonNullMarshaler{})},
		{"in struct", struct{ A (*int)? }{A: nilPointer}},
		{"in slice", []any{[]int?{1}, []any{some(nilPointer)}}},
		{"in map", map[string]any{"k": some(nilPointer)}},
	} {
		_, err := Marshal(test.value)
		var unsupported *UnsupportedValueError
		if !errors.As(err, &unsupported) {
			t.Errorf("%s: error = %v; want *UnsupportedValueError", test.name, err)
		}
		// Streaming encoders may flush eagerly, but must report the same error.
		err = NewEncoder(writerOnly{io.Discard}).Encode(test.value)
		if !errors.As(err, &unsupported) {
			t.Errorf("%s: Encode error = %v; want *UnsupportedValueError", test.name, err)
		}
	}
	var nested (int?)?
	var inner int?
	for name, value := range map[string]any{
		"absent nested":  nested,
		"present nested": some(inner),
		"in struct":      struct{ A (int?)? }{},
		"slice":          []any{some(some(7))},
	} {
		_, err := Marshal(value)
		var unsupported *UnsupportedTypeError
		if !errors.As(err, &unsupported) {
			t.Errorf("%s: error = %v; want *UnsupportedTypeError", name, err)
		}
	}
	for name, value := range map[string]any{
		"key":         map[int?]string{},
		"present key": map[int?]string{7: "x"},
		"nested key":  map[string]map[int?]int{"a": {1: 2}},
	} {
		_, err := Marshal(value)
		var unsupported *UnsupportedTypeError
		if !errors.As(err, &unsupported) {
			t.Errorf("%s: error = %v; want *UnsupportedTypeError", name, err)
		}
	}
	_, err := Marshal(some(gonFailingMarshaler{}))
	if !errors.Is(err, errGonMarshal) {
		t.Errorf("payload marshaler error = %v", err)
	}
	var marshalerError *MarshalerError
	if !errors.As(err, &marshalerError) {
		t.Errorf("payload marshaler error type = %#v", err)
	}
	// An absent optional never reaches an unsupported payload.
	var channel (chan int)?
	if got := marshalString(t, channel); got != `null` {
		t.Errorf("absent channel = %s", got)
	}
	if _, err := Marshal(some(make(chan int))); err == nil {
		t.Error("present channel accepted")
	}
}

func unmarshalInto(t *testing.T, text string, target any) {
	t.Helper()
	if err := Unmarshal([]byte(text), target); err != nil {
		t.Fatalf("Unmarshal(%s): %v", text, err)
	}
}

func TestGonOptionalUnmarshal(t *testing.T) {
	var value int?
	unmarshalInto(t, `7`, &value)
	if !reflect.DeepEqual(value, some[int](7)) {
		t.Errorf("present = %v", value)
	}
	unmarshalInto(t, `0`, &value)
	if absent(value) || value != some[int](0) {
		t.Errorf("zero = %v", value)
	}
	unmarshalInto(t, `null`, &value)
	if !absent(value) {
		t.Errorf("null = %v", value)
	}
	unmarshalInto(t, `null`, &value)
	if !absent(value) {
		t.Errorf("null on absent = %v", value)
	}
	var pointer *int?
	unmarshalInto(t, `4`, &pointer)
	if pointer == nil || *pointer != some[int](4) {
		t.Errorf("pointer to optional = %v", pointer)
	}
	unmarshalInto(t, `null`, &pointer)
	if pointer != nil {
		t.Errorf("null into pointer to optional = %v", pointer)
	}
	var fields gonOptionalFields
	unmarshalInto(t, `{"int":0,"float":0,"string":"","bool":false,"point":{},"slice":[],"pointer":3,"elems":[null,1],"map":{"k":null,"j":2},"role":"student"}`, &fields)
	n := 3
	want := gonOptionalFields{
		Int: 0, Float: 0.0, String: "", Bool: false, Point: gonPoint{},
		Slice: []int{}, Pointer: &n, Elems: []int?{nil, 1}, Map: map[string]int?{"k": nil, "j": 2},
		Role: gonRole.Student,
	}
	if !reflect.DeepEqual(fields, want) {
		t.Errorf("fields = %+v; want %+v", fields, want)
	}
	// null clears present fields, while missing fields remain unchanged.
	unmarshalInto(t, `{"int":null,"string":null,"role":null,"elems":null}`, &fields)
	want.Int, want.String, want.Role, want.Elems = nil, nil, nil, nil
	if !reflect.DeepEqual(fields, want) {
		t.Errorf("after null = %+v; want %+v", fields, want)
	}
	unmarshalInto(t, `{"float":2.5}`, &fields)
	want.Float = 2.5
	if !reflect.DeepEqual(fields, want) {
		t.Errorf("after missing = %+v; want %+v", fields, want)
	}
	// An optional nested in a larger value is decoded through the same path.
	var nested struct {
		Items []struct{ A int? }
		Any   any
	}
	unmarshalInto(t, `{"Items":[{"A":1},{"A":null},{}],"Any":[1,null]}`, &nested)
	if len(nested.Items) != 3 || nested.Items[0].A != some[int](1) || !absent(nested.Items[1].A) || !absent(nested.Items[2].A) {
		t.Errorf("nested = %+v", nested)
	}
}

func TestGonOptionalUnmarshalExistingPayload(t *testing.T) {
	type record struct {
		Point gonPoint?         `json:"point"`
		List  ([]int)?          `json:"list"`
		Map   (map[string]int)? `json:"map"`
	}
	r := record{Point: gonPoint{1, 2}, List: []int{1, 2, 3}, Map: map[string]int{"a": 1}}
	shared := map[string]int{"a": 1}
	r.Map = shared
	unmarshalInto(t, `{"point":{"Y":9},"list":[7],"map":{"b":2}}`, &r)
	if got := r.Point; got != some[gonPoint](gonPoint{1, 9}) {
		t.Errorf("point decoded into existing payload = %v", got)
	}
	if !reflect.DeepEqual(r.List, some([]int{7})) {
		t.Errorf("list = %v", r.List)
	}
	if len(shared) != 2 || shared["b"] != 2 {
		t.Errorf("existing map payload was not reused: %v", shared)
	}
	var fresh record
	unmarshalInto(t, `{"point":{"Y":9}}`, &fresh)
	if got := fresh.Point; got != some[gonPoint](gonPoint{0, 9}) {
		t.Errorf("point decoded into zero payload = %v", got)
	}
}

func TestGonOptionalUnmarshalPayloadProtocols(t *testing.T) {
	var value struct {
		ByValue   gonByValue?   `json:"v"`
		ByPointer gonByPointer? `json:"p"`
		Text      gonText?      `json:"t"`
		Role      gonRole?      `json:"r"`
		Nested    gonNested?    `json:"n"`
	}
	unmarshalInto(t, `{"v":1,"p":"x","t":"abc","r":"teacher","n":{"Pointer":2}}`, &value)
	if value.ByValue != some[gonByValue](gonByValue{"decoded:1"}) ||
		value.ByPointer != some[gonByPointer](gonByPointer{`decoded:"x"`}) ||
		value.Text != some[gonText](gonText{"decoded:abc"}) ||
		value.Role != some[gonRole](gonRole.Teacher) ||
		value.Nested != some[gonNested](gonNested{gonByPointer{"decoded:2"}}) {
		t.Errorf("payload protocols = %+v", value)
	}
	// JSON null never reaches the payload's Unmarshaler.
	unmarshalInto(t, `{"v":null,"p":null,"t":null,"r":null,"n":null}`, &value)
	if !absent(value.ByValue) || !absent(value.ByPointer) || !absent(value.Text) || !absent(value.Role) || !absent(value.Nested) {
		t.Errorf("null = %+v", value)
	}
	unmarshalInto(t, `{"r":"future"}`, &value)
	if value.Role != some[gonRole](gonRole.Unknown("future")) {
		t.Errorf("unknown enum = %v", value.Role)
	}
	// Map keys keep their ordinary TextUnmarshaler contract with optional values.
	var keyed map[gonText]gonRole?
	unmarshalInto(t, `{"a":"teacher","b":null}`, &keyed)
	if len(keyed) != 2 || keyed[gonText{"decoded:a"}] != some[gonRole](gonRole.Teacher) || !absent(keyed[gonText{"decoded:b"}]) {
		t.Errorf("map = %v", keyed)
	}
}

func TestGonOptionalUnmarshalTypeErrors(t *testing.T) {
	type record struct {
		Count int?      `json:"count"`
		Name  string?   `json:"name"`
		Role  gonRole?  `json:"role"`
		Point gonPoint? `json:"point"`
	}
	r := record{Count: 5, Point: gonPoint{3, 4}}
	err := Unmarshal([]byte(`{"count":"x","name":1,"role":2,"point":{"X":"bad"}}`), &r)
	var typeError *UnmarshalTypeError
	if !errors.As(err, &typeError) || typeError.Value != "string" || typeError.Type != reflect.TypeFor[int]() || typeError.Field != "count" || typeError.Struct != "record" {
		t.Fatalf("error = %#v", err)
	}
	// Type errors leave every optional unchanged.
	if r.Count != some[int](5) || !absent(r.Name) || !absent(r.Role) || r.Point != some[gonPoint](gonPoint{3, 4}) {
		t.Errorf("destination changed: %+v", r)
	}
	// Errors are those of the payload type.
	for _, test := range []struct {
		text  string
		value any
		want  string
		typ   reflect.Type
	}{
		{`"x"`, new(int?), "string", reflect.TypeFor[int]()},
		{`true`, new(int?), "bool", reflect.TypeFor[int]()},
		{`[1]`, new(int?), "array", reflect.TypeFor[int]()},
		{`{}`, new(string?), "object", reflect.TypeFor[string]()},
		{`1.5`, new(int?), "number 1.5", reflect.TypeFor[int]()},
		{`300`, new(uint8?), "number 300", reflect.TypeFor[uint8]()},
		{`1`, new(gonRole?), "number", reflect.TypeFor[gonRole]()},
		{`{}`, new(gonRole?), "object", reflect.TypeFor[gonRole]()},
	} {
		err := Unmarshal([]byte(test.text), test.value)
		if !errors.As(err, &typeError) || typeError.Value != test.want || typeError.Type != test.typ {
			t.Errorf("Unmarshal(%s, %T) = %#v; want %s into %v", test.text, test.value, err, test.want, test.typ)
		}
		if got := reflect.ValueOf(test.value).Elem(); !got.IsZero() {
			t.Errorf("Unmarshal(%s, %T) changed the destination to %v", test.text, test.value, got)
		}
	}
	// Syntax errors and unknown fields use the ordinary decoder paths.
	if err := Unmarshal([]byte(`{"count":`), &r); err == nil {
		t.Error("truncated input accepted")
	}
	decoder := NewDecoder(strings.NewReader(`{"point":{"Z":1}}`))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&r); err == nil || !strings.Contains(err.Error(), `unknown field "Z"`) {
		t.Errorf("DisallowUnknownFields error = %v", err)
	}
	if r.Point != some[gonPoint](gonPoint{3, 4}) {
		t.Errorf("unknown field changed destination: %v", r.Point)
	}
}

func TestGonOptionalUnmarshalFatalError(t *testing.T) {
	var value struct{ B failingUnmarshaler? }
	err := Unmarshal([]byte(`{"B":1}`), &value)
	if !errors.Is(err, errGonUnmarshal) {
		t.Errorf("error = %v", err)
	}
	if !absent(value.B) {
		t.Errorf("failed Unmarshaler produced a present value: %v", value.B)
	}
}

type failingUnmarshaler struct{}

var errGonUnmarshal = errors.New("gon unmarshal failure")

func (*failingUnmarshaler) UnmarshalJSON([]byte) error { return errGonUnmarshal }

func TestGonOptionalUnmarshalUnsupported(t *testing.T) {
	var nested (int?)?
	for _, text := range []string{`1`, `null`, `[]`, `{}`, `"x"`, `true`} {
		err := Unmarshal([]byte(text), &nested)
		var typeError *UnmarshalTypeError
		if !errors.As(err, &typeError) || typeError.Type != reflect.TypeFor[(int?)?]() {
			t.Errorf("Unmarshal(%s) into nested optional = %v", text, err)
		}
		if !absent(nested) {
			t.Errorf("nested optional changed: %v", nested)
		}
	}
	// The rest of the document is still consumed and decoded.
	var value struct {
		Nested (int?)?
		After  int?
	}
	err := Unmarshal([]byte(`{"Nested":[1,{"a":2}],"After":3}`), &value)
	var typeError *UnmarshalTypeError
	if !errors.As(err, &typeError) || value.After != some[int](3) {
		t.Errorf("error = %v, after = %v", err, value.After)
	}
	// A missing nested field is not an error.
	if err := Unmarshal([]byte(`{"After":4}`), &value); err != nil || value.After != some[int](4) {
		t.Errorf("missing nested field: %v, %v", err, value.After)
	}
	// Optional map keys are not supported.
	var keyed map[int?]string
	if err := Unmarshal([]byte(`{"1":"x"}`), &keyed); !errors.As(err, &typeError) {
		t.Errorf("optional map key error = %v", err)
	}
	// Decoding into an interface keeps the standard dynamic values.
	var anyValue any
	unmarshalInto(t, `[1,null]`, &anyValue)
	if !reflect.DeepEqual(anyValue, []any{1.0, nil}) {
		t.Errorf("interface = %#v", anyValue)
	}
}

func TestGonOptionalRoundTrip(t *testing.T) {
	n := 9
	value := gonOptionalFields{
		Int: 0, Float: 1.5, String: "", Point: gonPoint{1, 2}, Slice: []int{},
		Pointer: &n, Elems: []int?{nil, 0, 2}, Map: map[string]int?{"a": nil, "b": 0},
		Role: gonRole.Unknown("future"),
	}
	data, err := Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var back gonOptionalFields
	if err := Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, value) {
		t.Errorf("round trip = %+v; want %+v", back, value)
	}
	again, err := Marshal(back)
	if err != nil || !bytes.Equal(again, data) {
		t.Errorf("second encoding = %s, %v; want %s", again, err, data)
	}
}

func TestGonOptionalDecoderStream(t *testing.T) {
	decoder := NewDecoder(strings.NewReader(`1 null 2.5 "x"`))
	var a int?
	var b float64? = 3
	var c float64?
	var d string?
	for _, target := range []any{&a, &b, &c, &d} {
		if err := decoder.Decode(target); err != nil {
			t.Fatal(err)
		}
	}
	if a != some[int](1) {
		t.Errorf("a = %v", a)
	}
	// null clears the existing value.
	if !absent(b) {
		t.Errorf("b = %v", b)
	}
	if c != some[float64](2.5) || d != some[string]("x") {
		t.Errorf("c, d = %v, %v", c, d)
	}
	var buffer bytes.Buffer
	encoder := NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	for _, value := range []any{a, b, d, some[string]("<>")} {
		if err := encoder.Encode(value); err != nil {
			t.Fatal(err)
		}
	}
	if got := buffer.String(); got != "1\nnull\n\"x\"\n\"<>\"\n" {
		t.Errorf("Encode = %q", got)
	}
}

func TestGonOptionalStreamingPayloads(t *testing.T) {
	// Payloads that need inspection are written intact through a generic
	// io.Writer, including top-level values and values within containers.
	n := 4
	values := []any{
		some(&n),
		some(&[]int{1, 2, 3}),
		some(any(map[string]int{"a": 1})),
		some(gonByPointer{"x"}),
		[]any{some(&n), some(gonByValue{"y"}), some(&[]string{"z"})},
		map[string]any{"k": some(&n)},
		struct {
			A (*int)?
			B gonByPointer?
		}{A: &n, B: gonByPointer{"w"}},
	}
	for _, value := range values {
		want, err := Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := NewEncoder(writerOnly{&out}).Encode(value); err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSuffix(out.String(), "\n"); got != string(want) {
			t.Errorf("Encode(%T) = %q; want %q", value, got, want)
		}
	}
	// A large payload behind a pointer crosses the encoder's flush threshold.
	big := make([]int, 20000)
	var out bytes.Buffer
	if err := NewEncoder(writerOnly{&out}).Encode(some(&big)); err != nil {
		t.Fatal(err)
	}
	var back []int
	if err := Unmarshal(out.Bytes(), &back); err != nil || len(back) != len(big) {
		t.Errorf("large payload round trip: %d elements, %v", len(back), err)
	}
}

func TestGonOptionalInterfaceAndRaw(t *testing.T) {
	var value any?
	unmarshalInto(t, `{"a":[1,null,"x"]}`, &value)
	if got, want := marshalString(t, value), `{"a":[1,null,"x"]}`; got != want {
		t.Errorf("interface payload = %s; want %s", got, want)
	}
	unmarshalInto(t, `null`, &value)
	if !absent(value) {
		t.Errorf("null interface payload = %v", value)
	}
	unmarshalInto(t, `7`, &value)
	if got := marshalString(t, value); got != `7` {
		t.Errorf("number payload = %s", got)
	}
	var raw RawMessage?
	unmarshalInto(t, `{"x": 1}`, &raw)
	if got := marshalString(t, raw); got != `{"x":1}` {
		t.Errorf("raw payload = %s", got)
	}
	unmarshalInto(t, `null`, &raw)
	if !absent(raw) {
		t.Errorf("null raw payload = %v", raw)
	}
	// A raw null payload would read back as absence.
	if _, err := Marshal(some(RawMessage(nil))); err == nil {
		t.Error("present nil RawMessage accepted")
	}
	var number Number?
	unmarshalInto(t, `12.5`, &number)
	if got := marshalString(t, number); got != `12.5` {
		t.Errorf("Number payload = %s", got)
	}
}

// gonPointerText has methods on pointer receivers although it is not a struct.
type gonPointerText string

func (v *gonPointerText) MarshalText() ([]byte, error) { return []byte("ptext:" + string(*v)), nil }
func (v *gonPointerText) UnmarshalText(data []byte) error {
	*v = gonPointerText(strings.TrimPrefix(string(data), "ptext:"))
	return nil
}

func TestGonOptionalPointerReceiverScalars(t *testing.T) {
	var value gonPointerText?
	unmarshalInto(t, `"ptext:abc"`, &value)
	if got := marshalString(t, value); got != `"ptext:abc"` {
		t.Errorf("round trip = %s", got)
	}
	if got := marshalString(t, []gonPointerText?{value, nil}); got != `["ptext:abc",null]` {
		t.Errorf("elements = %s", got)
	}
	// An empty struct payload is present, not absent.
	var unit struct{}?
	unmarshalInto(t, `{}`, &unit)
	if got := marshalString(t, unit); got != `{}` {
		t.Errorf("empty struct payload = %s", got)
	}
	unmarshalInto(t, `null`, &unit)
	if got := marshalString(t, unit); got != `null` {
		t.Errorf("empty struct absence = %s", got)
	}
}
