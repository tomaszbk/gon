package types2

import (
	"reflect"
	"testing"
)

// Signal size changes of important structures.

func TestSizeof(t *testing.T) {
	const _64bit = ^uint(0)>>32 != 0

	// Objects have two position fields (syntax.Pos vs token.Pos), accounting
	// for 16 extra bytes in types2 on both 32-bit and 64-bit platforms.
	// Gon scopes have a third position field for the function body.
	var extra, scopeExtra uintptr
	if isTypes2 {
		extra = 16
		scopeExtra = 24 // Gon scopes have three position fields
	}

	var tests = []struct {
		val    any     // type as a value
		_32bit uintptr // size on 32bit platforms
		_64bit uintptr // size on 64bit platforms
	}{
		// Types
		{Basic{}, 16, 32},
		{Array{}, 16, 24},
		{Slice{}, 8, 16},
		{Struct{}, 28, 56},
		{Pointer{}, 8, 16},
		{Tuple{}, 12, 24},
		{Signature{}, 32, 64},
		{Union{}, 12, 24},
		{Interface{}, 40, 80},
		{Map{}, 16, 32},
		{Chan{}, 12, 24},
		{Named{}, 72, 136},
		{TypeParam{}, 28, 48},
		{term{}, 12, 24},

		// Objects
		{PkgName{}, 40 + extra, 80 + extra},
		{Const{}, 44 + extra, 88 + extra},
		{TypeName{}, 36 + extra, 72 + extra},
		{Var{}, 44 + extra, 88 + extra},
		{Func{}, 44 + extra, 88 + extra},
		{Label{}, 40 + extra, 80 + extra},
		{Builtin{}, 40 + extra, 80 + extra},
		{Nil{}, 36 + extra, 72 + extra},

		// Misc
		{Scope{}, 56 + scopeExtra, 112 + scopeExtra}, // Gon function signature/body position
		{Package{}, 44, 88},
		{_TypeSet{}, 28, 56},
	}

	for _, test := range tests {
		got := reflect.TypeOf(test.val).Size()
		want := test._32bit
		if _64bit {
			want = test._64bit
		}
		if got != want {
			t.Errorf("unsafe.Sizeof(%T) = %d, want %d", test.val, got, want)
		}
	}
}
