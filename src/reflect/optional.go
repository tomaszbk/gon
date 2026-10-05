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

// OptionalValueSetPayload makes the optional v present and stores a copy of
// payload, which must be assignable to the optional's element type. Use
// v.SetZero to make an optional absent. It panics if v is not an optional
// value, if v is not settable, or if payload is not assignable to
// [OptionalElement](v.Type()). Reference payloads keep Go aliasing.
func OptionalValueSetPayload(v Value, payload Value) {
	if !v.IsValid() || !IsOptional(v.Type()) {
		panic("reflect: OptionalValueSetPayload of non-optional value")
	}
	if v.flag&flagRO != 0 {
		panic("reflect: OptionalValueSetPayload using value obtained using unexported field")
	}
	if v.flag&flagAddr == 0 {
		panic("reflect: OptionalValueSetPayload using unaddressable value")
	}
	payload.mustBeExported() // do not let an unexported payload leak
	st := (*structType)(unsafe.Pointer(v.typ()))
	storage := &st.Fields[2]
	_, tag := enumVariantMetadata(*storage)
	field := &(*structType)(unsafe.Pointer(storage.Typ)).Fields[0]
	payload = payload.assignTo("reflect.OptionalValueSetPayload", field.Typ, nil)
	// Store the payload before the discriminator so that the optional is never
	// present with stale storage.
	dst := add(v.ptr, storage.Offset+field.Offset, "optional payload")
	if payload.flag&flagIndir != 0 {
		if payload.ptr == unsafe.Pointer(&zeroVal[0]) {
			typedmemclr(field.Typ, dst)
		} else {
			typedmemmove(field.Typ, dst, payload.ptr)
		}
	} else {
		*(*unsafe.Pointer)(dst) = payload.ptr
	}
	*(*uint)(add(v.ptr, st.Fields[0].Offset, "optional discriminator")) = tag
}
