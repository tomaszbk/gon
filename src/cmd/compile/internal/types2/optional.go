package types2

// Optional is the native presence/absence type T?. Its underlying type is
// itself, so it is distinct from a struct and from every user-declared enum.
// The payload and presence tag retain typed, GC-safe backing storage.
type Optional struct {
	elem    Type
	storage *Struct
}

// NewOptional returns the native optional type with the given payload type.
func NewOptional(elem Type) *Optional {
	if elem == nil {
		panic("nil optional element")
	}
	return &Optional{elem: elem, storage: NewEnum([]*EnumVariant{
		NewEnumVariant("$absent", nil, nil, false),
		NewEnumVariant("$present", nil, []*Var{NewField(nopos, nil, "", elem, false)}, false),
	}, 0)}
}

func (t *Optional) Elem() Type       { return t.elem }
func (t *Optional) Underlying() Type { return t }
func (t *Optional) String() string   { return TypeString(t, nil) }

// OptionalOf returns the native optional type denoted by t, including aliases,
// or nil. A separately defined type does not adopt the optional protocol.
func OptionalOf(t Type) *Optional {
	if t == nil {
		return nil
	}
	o, _ := Unalias(t).(*Optional)
	return o
}

// IsOptional reports whether t denotes a native optional, including aliases.
func IsOptional(t Type) bool { return OptionalOf(t) != nil }

// EnumStorageOf returns private typed storage metadata used by lowering and
// analysis of closed alternatives. Optional storage names are not constructors
// or patterns. Use OptionalOf and Elem for source-language introspection.
func EnumStorageOf(t Type) *Enum {
	if o := OptionalOf(t); o != nil {
		return o.storage.enum
	}
	return EnumOf(t)
}

// OptionalStorage returns the private GC-safe backing type of an optional.
// This is representation metadata for compiler/analysis tools, not a stable ABI.
func OptionalStorage(t Type) *Struct {
	if o := OptionalOf(t); o != nil {
		return o.storage
	}
	return nil
}
