package reflect

import (
	"internal/abi"
	"unsafe"
)

// IsOptional reports whether t has Gon native optional representation. Its
// Kind remains Struct, while ordinary struct-field APIs hide its storage.
func IsOptional(t Type) bool {
	if t == nil || t.common().TFlag&abi.TFlagEnum == 0 {
		return false
	}
	st := (*structType)(unsafe.Pointer(t.common()))
	return len(st.Fields) == 3 && st.Fields[0].Name.Name() == "$gonOptional"
}

// OptionalElement returns the payload type and panics for a non-optional type.
func OptionalElement(t Type) Type {
	if !IsOptional(t) {
		panic("reflect: OptionalElement of non-optional type")
	}
	st := (*structType)(unsafe.Pointer(t.common()))
	payload := (*structType)(unsafe.Pointer(st.Fields[2].Typ))
	return toType(payload.Fields[0].Typ)
}

// OptionalValuePresent reports presence, preserving a present zero or nil.
// It panics for a non-optional value or an invalid discriminator.
func OptionalValuePresent(v Value) bool {
	if !v.IsValid() || !IsOptional(v.Type()) {
		panic("reflect: OptionalValuePresent of non-optional value")
	}
	variant, _ := v.enumVariant()
	return !variant.Default
}

// OptionalValuePayload returns a detached, non-addressable copy of the present
// payload. It panics for an absent or non-optional value. Reference payloads
// keep Go aliasing, and access restrictions of the original value are retained.
func OptionalValuePayload(v Value) Value {
	if !OptionalValuePresent(v) {
		panic("reflect: OptionalValuePayload of absent value")
	}
	return alternativePayload(v, 0)
}
