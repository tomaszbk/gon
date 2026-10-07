package types2

import (
	"runtime"
	"strconv"
	"testing"
)

func TestEnumUnitLayout(t *testing.T) {
	sizes := SizesFor("gc", runtime.GOARCH)
	for _, test := range []struct {
		count int
		size  int64
		kind  BasicKind
	}{
		{1, 1, Uint8}, {2, 1, Uint8}, {256, 1, Uint8},
		{257, 2, Uint16}, {65536, 2, Uint16}, {65537, 4, Uint32},
	} {
		t.Run(strconv.Itoa(test.count), func(t *testing.T) {
			variants := make([]*EnumVariant, test.count)
			for i := range variants {
				variants[i] = NewEnumVariant("V"+strconv.Itoa(i), nil, nil, false)
			}
			enum := NewEnum(variants, test.count-1)
			if got := sizes.Sizeof(enum); got != test.size {
				t.Fatalf("Sizeof = %d, want %d", got, test.size)
			}
			if got := sizes.Alignof(enum); got != test.size {
				t.Fatalf("Alignof = %d, want %d", got, test.size)
			}
			if got := enum.Field(0).Type(); got != Typ[test.kind] {
				t.Fatalf("tag type = %v, want %v", got, Typ[test.kind])
			}
			if got := sizes.Sizeof(NewArray(enum, 100)); got != 100*test.size {
				t.Fatalf("array Sizeof = %d, want %d", got, 100*test.size)
			}
			if enum.enum.Default().Tag() != 0 || enum.enum.Variant(0).Tag() != 1 && test.count > 1 {
				t.Fatal("compact storage changed default or declaration-order tags")
			}
		})
	}
	// An observable payload of size zero still follows ordinary struct layout.
	payload := NewStruct(nil, nil)
	enum := NewEnum([]*EnumVariant{
		NewEnumVariant("Empty", nil, nil, false),
		NewEnumVariant("Value", nil, []*Var{NewField(nopos, nil, "", payload, false)}, false),
	}, 0)
	if enum.Field(0).Type() != Typ[Uint] {
		t.Fatal("payload-bearing enum tag changed")
	}
	if got, want := sizes.Sizeof(enum), 2*sizes.Sizeof(Typ[Uint]); got != want {
		t.Fatalf("zero-size payload enum = %d, want %d", got, want)
	}
	optional := NewOptional(payload)
	if got, want := sizes.Sizeof(optional), sizes.Sizeof(enum); got != want {
		t.Fatalf("optional storage = %d, want unchanged %d", got, want)
	}
}
