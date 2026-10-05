// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !goexperiment.jsonv2

package json

import "reflect"

// Gon native optionals T? map onto JSON null and the payload's own encoding.
// The optional storage stays private: only the checked reflect optional API is
// used, never ordinary struct-field access.

// optionalEncoder encodes absence as null and a present payload as its
// element type would encode. Nested optionals are rejected before this
// encoder is built, because null could not tell the layers apart.
type optionalEncoder struct {
	elemType reflect.Type
	elemEnc  encoderFunc
	// addr is set when the payload's method set or nested fields may depend
	// on addressability. The payload is then encoded through an addressable
	// copy, so pointer-receiver marshalers are honored as they are for a *T.
	addr bool
}

func (oe optionalEncoder) encode(e *encodeState, v reflect.Value, opts encOpts) {
	if !reflect.OptionalValuePresent(v) {
		e.WriteString("null")
		return
	}
	payload := reflect.OptionalValuePayload(v)
	if oe.addr {
		addressable := reflect.New(oe.elemType).Elem()
		addressable.Set(payload)
		payload = addressable
	}
	start := e.Len()
	oe.elemEnc(e, payload, opts)
	// Presence must survive a round trip, but null decodes as absence.
	if e.Len()-start == len("null") && string(e.Bytes()[start:]) == "null" {
		e.error(&UnsupportedValueError{v, "present optional " + v.Type().String() + " whose payload encodes as null"})
	}
}

func newOptionalEncoder(t reflect.Type) encoderFunc {
	elem := reflect.OptionalElement(t)
	if reflect.IsOptional(elem) {
		return unsupportedTypeEncoder
	}
	return optionalEncoder{elem, typeEncoder(elem), optionalPayloadNeedsAddr(elem)}.encode
}

// optionalPayloadNeedsAddr reports whether encoding a T needs an addressable
// value to behave as it does for *T.
func optionalPayloadNeedsAddr(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer:
		return false
	case reflect.Struct, reflect.Array:
		// Fields or elements may have pointer-receiver marshalers.
		return true
	}
	p := reflect.PointerTo(t)
	return p.Implements(marshalerType) || p.Implements(textMarshalerType)
}

// isOptionalValue reports whether v is a native optional.
func isOptionalValue(v reflect.Value) bool {
	return v.Kind() == reflect.Struct && reflect.IsOptional(v.Type())
}

// optionalStore decodes one JSON value into the native optional v. JSON null
// makes it absent. Any other value is decoded into a temporary payload, which
// starts as the existing payload if v is present and as the zero value
// otherwise, and is stored as the present payload only if decoding succeeded.
// A type error therefore leaves v unchanged. kind names the JSON value for
// errors, decode reads the value into its argument and skip consumes the value
// if v cannot hold any.
func (d *decodeState) optionalStore(v reflect.Value, null bool, kind string, decode func(reflect.Value) error, skip func()) error {
	t := v.Type()
	elem := reflect.OptionalElement(t)
	if reflect.IsOptional(elem) {
		// Absence of the outer layer cannot be told from absence of the inner,
		// so no JSON value, not even null, can be stored.
		d.saveError(&UnmarshalTypeError{Value: kind, Type: t, Offset: int64(d.readIndex())})
		skip()
		return nil
	}
	if null {
		v.SetZero()
		return nil
	}
	payload := reflect.New(elem).Elem()
	if reflect.OptionalValuePresent(v) {
		payload.Set(reflect.OptionalValuePayload(v))
	}
	saved := d.savedError
	d.savedError = nil
	err := decode(payload)
	failed := d.savedError != nil
	if saved != nil {
		d.savedError = saved
	}
	if err != nil || failed {
		return err
	}
	reflect.OptionalValueSetPayload(v, payload)
	return nil
}
