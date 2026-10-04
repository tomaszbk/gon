package reflect

import (
	"internal/abi"
	"strconv"
	"unicode"
	"unicode/utf8"
	"unsafe"
)

// EnumField describes a payload position. Name is empty for positional payloads.
// No storage offsets are exposed: enum layout is a compiler implementation detail.
type EnumField struct {
	Name string
	Type Type
}

// EnumVariant describes one alternative of a closed Gon enum.
// Fields are in declaration order. Default identifies the zero-value alternative.
type EnumVariant struct {
	Name    string
	Record  bool
	Default bool
	Fields  []EnumField
}

// IsEnum reports whether t is a closed Gon enum. Enum Kind remains Struct.
// Enum storage is hidden from ordinary struct field APIs: NumField is zero,
// field lookup finds nothing, and direct Field access panics. Use EnumVariants
// and EnumValuePayload to inspect alternatives and their active payload.
func IsEnum(t Type) bool { return t != nil && t.common().TFlag&abi.TFlagEnum != 0 && !IsOptional(t) }

// EnumVariants returns alternatives in declaration order and panics for a non-enum.
func EnumVariants(t Type) []EnumVariant {
	if !IsEnum(t) {
		panic("reflect: EnumVariants of non-enum type")
	}
	st := (*structType)(unsafe.Pointer(t.common()))
	variants := make([]EnumVariant, len(st.Fields)-1)
	for i, field := range st.Fields[1:] {
		variant, _ := enumVariantMetadata(field)
		variants[i] = variant
	}
	return variants
}

// enumVariantMetadata decodes private compiler metadata. Ordinary struct fields
// never interpret these tags. The format is not a stable public representation.
func enumVariantMetadata(field abi.StructField) (EnumVariant, uint) {
	metadata := field.Name.Tag()
	end := len(metadata) - 1
	for end >= 0 && metadata[end] != ':' {
		end--
	}
	if end < 2 || metadata[end-2] != ':' {
		panic("reflect: invalid enum metadata")
	}
	tag, err := strconv.Atoi(metadata[end+1:])
	if err != nil || tag < 0 {
		panic("reflect: invalid enum metadata")
	}
	variant := EnumVariant{Name: metadata[:end-2], Record: metadata[end-1] == '1', Default: tag == 0}
	payload := (*structType)(unsafe.Pointer(field.Typ))
	variant.Fields = make([]EnumField, len(payload.Fields))
	for i, f := range payload.Fields {
		variant.Fields[i] = EnumField{Name: f.Name.Tag(), Type: toType(f.Typ)}
	}
	return variant, uint(tag)
}

func (v Value) enumVariant() (EnumVariant, *abi.StructField) {
	if !v.IsValid() || v.typ().TFlag&abi.TFlagEnum == 0 {
		panic("reflect: Variant of non-enum value")
	}
	st := (*structType)(unsafe.Pointer(v.typ()))
	// Enums always contain the discriminator and at least one payload struct,
	// so they are stored indirectly in interfaces, including unit-only enums.
	tag := *(*uint)(add(v.ptr, st.Fields[0].Offset, "enum discriminator"))
	for i := 1; i < len(st.Fields); i++ {
		variant, expected := enumVariantMetadata(st.Fields[i])
		if tag == expected {
			return variant, &st.Fields[i]
		}
	}
	panic("reflect: invalid enum discriminator")
}

// EnumValueVariant returns the active alternative. It panics for a non-enum value or an
// invalid discriminator produced through unsafe memory manipulation.
func EnumValueVariant(v Value) EnumVariant {
	if !v.IsValid() || !IsEnum(v.Type()) {
		panic("reflect: EnumValueVariant of non-enum value")
	}
	variant, _ := v.enumVariant()
	return variant
}

// EnumValuePayload returns a copy of the i'th payload of the active alternative.
// It panics for a non-enum value or an out-of-range position. The returned value
// is not addressable or settable; reference payloads retain ordinary Go aliases.
// Unexported variant or record field payloads cannot be converted to Interface.
func EnumValuePayload(v Value, i int) Value {
	if !v.IsValid() || !IsEnum(v.Type()) {
		panic("reflect: EnumValuePayload of non-enum value")
	}
	return alternativePayload(v, i)
}

func alternativePayload(v Value, i int) Value {
	variant, storage := v.enumVariant()
	payload := (*structType)(unsafe.Pointer(storage.Typ))
	if uint(i) >= uint(len(payload.Fields)) {
		panic("reflect: EnumPayload index out of range")
	}
	field := payload.Fields[i]
	data := unsafe_New(field.Typ)
	typedmemmove(field.Typ, data, add(v.ptr, storage.Offset+field.Offset, "active enum payload"))
	fl := flagIndir | flag(field.Typ.Kind()) | v.flag&flagRO
	if !IsOptional(v.Type()) && (!enumExported(variant.Name) || variant.Record && !enumExported(variant.Fields[i].Name)) {
		fl |= flagStickyRO
	}
	return Value{field.Typ, data, fl}
}

func enumExported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}
