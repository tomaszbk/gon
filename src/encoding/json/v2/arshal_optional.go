// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.jsonv2

package json

import (
	"fmt"
	"reflect"
	"sync"

	"encoding/json/internal"
	"encoding/json/internal/jsonflags"
	"encoding/json/internal/jsonopts"
	"encoding/json/jsontext"
)

// A Gon native optional T? is a JSON null when absent and otherwise exactly
// the encoding of its payload as a T. Only the checked reflect optional API
// touches the private optional storage.

// makeOptionalArshaler returns the arshaler for a native optional type t.
// Nested optionals are unsupported because null could not distinguish the
// absence of the outer layer from the absence of the inner one.
func makeOptionalArshaler(t reflect.Type) *arshaler {
	var fncs arshaler
	elem := reflect.OptionalElement(t)
	if reflect.IsOptional(elem) {
		fncs.marshal = func(enc *jsontext.Encoder, va addressableValue, mo *jsonopts.Struct) error {
			return newMarshalErrorBefore(enc, t, nil)
		}
		fncs.unmarshal = func(dec *jsontext.Decoder, va addressableValue, uo *jsonopts.Struct) error {
			// Unlike other unsupported types, even null is rejected: it could
			// not say which layer is absent.
			if uo.Flags.Get(jsonflags.ReportErrorsWithLegacySemantics) {
				if _, err := dec.ReadValue(); err != nil {
					return err
				}
				return newUnmarshalErrorAfter(dec, t, nil)
			}
			return newUnmarshalErrorBefore(dec, t, nil)
		}
		return &fncs
	}
	var (
		once    sync.Once
		valFncs *arshaler
	)
	init := func() {
		valFncs = lookupArshaler(elem)
	}
	fncs.marshal = func(enc *jsontext.Encoder, va addressableValue, mo *jsonopts.Struct) error {
		// NOTE: Struct.Format is forwarded to the payload.
		if !reflect.OptionalValuePresent(va.Value) {
			return enc.WriteToken(jsontext.Null)
		}
		if mo.Flags.Get(jsonflags.StringTag) && mo.Flags.Get(jsonflags.StringifyWithLegacySemantics) && elem.Kind() == reflect.Pointer {
			// Like a pointer, the optional is one layer: the `string` tag
			// option does not apply to a pointer payload.
			if !mo.Flags.Get(jsonflags.ReportErrorsWithLegacySemantics) {
				return newMarshalErrorBefore(enc, t, errInvalidStringTag)
			}
			mo.Flags.Clear(jsonflags.StringTag)
		}
		once.Do(init)
		marshal := valFncs.marshal
		if mo.Marshalers != nil {
			marshal, _ = mo.Marshalers.(*Marshalers).lookup(marshal, elem)
		}
		// Like a dereferenced pointer, the detached payload copy is
		// addressable, so methods on pointer receivers are called.
		payload := addressableValue{reflect.New(elem).Elem(), false}
		payload.Set(reflect.OptionalValuePayload(va.Value))
		if !valFncs.nonDefault && mo.Marshalers == nil && !payloadMayEncodeNull(payload.Value, mo) {
			return marshal(enc, payload, mo)
		}

		// Presence must survive a round trip, but a null payload would read
		// back as absence. Inspect what was written, which is only reliable
		// if nothing is flushed meanwhile.
		xe := export.Encoder(enc)
		start := len(xe.Buf)
		previous := xe.HoldFlush(true)
		err := marshal(enc, payload, mo)
		if err == nil {
			if b := xe.Buf; len(b) > start && len(b) >= len(`null`) && string(b[len(b)-len(`null`):]) == `null` {
				err = newMarshalErrorBefore(enc, t, &internal.ValueError{Val: va.Interface(),
					Err: fmt.Errorf("unsupported value: present optional %v whose payload encodes as null", t)})
			}
		}
		xe.HoldFlush(previous)
		if err == nil && xe.NeedFlush() {
			err = xe.Flush()
		}
		return err
	}
	fncs.unmarshal = func(dec *jsontext.Decoder, va addressableValue, uo *jsonopts.Struct) error {
		// NOTE: Struct.Format is forwarded to the payload.
		if dec.PeekKind() == 'n' {
			if _, err := dec.ReadToken(); err != nil {
				return err
			}
			va.SetZero()
			return nil
		}
		if uo.Flags.Get(jsonflags.StringTag) && uo.Flags.Get(jsonflags.StringifyWithLegacySemantics) && elem.Kind() == reflect.Pointer {
			if !uo.Flags.Get(jsonflags.ReportErrorsWithLegacySemantics) {
				return newUnmarshalErrorBeforeWithSkipping(dec, t, errInvalidStringTag)
			}
			uo.Flags.Clear(jsonflags.StringTag) // the `string` tag option does not apply to a pointer payload
		}
		once.Do(init)
		unmarshal := valFncs.unmarshal
		if uo.Unmarshalers != nil {
			unmarshal, _ = uo.Unmarshalers.(*Unmarshalers).lookup(unmarshal, elem)
		}
		// Decode into a temporary payload, starting from the existing one,
		// and store it only if decoding succeeded.
		payload := addressableValue{reflect.New(elem).Elem(), false}
		if reflect.OptionalValuePresent(va.Value) {
			payload.Set(reflect.OptionalValuePayload(va.Value))
		}
		if err := unmarshal(dec, payload, uo); err != nil {
			return err
		}
		if uo.Flags.Get(jsonflags.StringTag) && uo.Flags.Get(jsonflags.StringifyWithLegacySemantics) &&
			string(export.Decoder(dec).PreviousTokenOrValue()) == `"null"` {
			// A JSON null quoted within a JSON string makes the optional absent.
			va.SetZero()
			return nil
		}
		reflect.OptionalValueSetPayload(va.Value, payload.Value)
		return nil
	}
	return &fncs
}

// payloadMayEncodeNull reports whether a payload without custom marshalers can
// encode as null: a nil pointer or interface, a nil slice or map written as
// null, or any pointer or interface whose target may do so or has marshalers.
func payloadMayEncodeNull(v reflect.Value, mo *jsonopts.Struct) bool {
	for {
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			if v.IsNil() {
				return true
			}
			v = v.Elem()
			if lookupArshaler(v.Type()).nonDefault {
				return true
			}
			continue
		case reflect.Slice:
			return v.IsNil() && mo.Flags.Get(jsonflags.FormatNilSliceAsNull)
		case reflect.Map:
			return v.IsNil() && mo.Flags.Get(jsonflags.FormatNilMapAsNull)
		}
		return false
	}
}
