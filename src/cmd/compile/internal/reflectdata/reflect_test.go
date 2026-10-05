// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflectdata

import (
	"slices"
	"testing"

	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
)

func init() {
	// Initialize just enough of the universe and the types package to make
	// these tests function, as the ssa package tests do.
	types.PtrSize = 8
	types.RegSize = 8
	types.MaxWidth = 1 << 50
	typecheck.InitUniverse()
}

func typeAndString(t *types.Type) typeAndStr {
	return typeAndStr{t: t, short: types.TypeSymName(t), regular: t.String()}
}

// TestTypesStrCmpNoalg checks that the order in which type descriptors are
// written does not depend on the order in which the concurrent backend
// registered them, even for a noalg type, such as the array that backs a slice
// literal, and the regular type of the same shape. They have identical strings
// and share one descriptor symbol, and which one is written first changes the
// order of the symbols in the object file, and so the object file. An
// unstable object file makes the compiler differ from the compiler it built
// (see make.bash's staleness check) for package noder, which has both a noalg
// and a regular [3]ir.Node.
func TestTypesStrCmpNoalg(t *testing.T) {
	newArray := func(noalg bool) *types.Type {
		arr := types.NewArray(types.Types[types.TINT], 3)
		arr.SetNoalg(noalg)
		return arr
	}
	regular, noalg1, noalg2 := newArray(false), newArray(true), newArray(true)
	rs, n1, n2 := typeAndString(regular), typeAndString(noalg1), typeAndString(noalg2)
	if rs.short != n1.short || rs.regular != n1.regular {
		t.Fatalf("noalg and regular arrays differ in name: %q, %q vs %q, %q", rs.short, rs.regular, n1.short, n1.regular)
	}

	if got := typesStrCmp(rs, n1); got >= 0 {
		t.Errorf("typesStrCmp(regular, noalg) = %d, want < 0", got)
	}
	if got := typesStrCmp(n1, rs); got <= 0 {
		t.Errorf("typesStrCmp(noalg, regular) = %d, want > 0", got)
	}
	if got := typesStrCmp(n1, n2); got != 0 {
		t.Errorf("typesStrCmp(noalg, noalg) = %d, want 0", got)
	}

	// Whatever the registration order, the regular type is written first.
	for _, order := range [][]typeAndStr{{rs, n1, n2}, {n1, rs, n2}, {n1, n2, rs}, {n2, n1, rs}} {
		got := slices.Clone(order)
		slices.SortFunc(got, typesStrCmp)
		if got[0].t != regular {
			t.Errorf("registered in order %v, sorted %v: the regular type is not first", describe(order), describe(got))
		}
	}
}

func describe(ts []typeAndStr) []string {
	var d []string
	for _, x := range ts {
		s := x.regular
		if types.TypeHasNoAlg(x.t) {
			s += " (noalg)"
		}
		d = append(d, s)
	}
	return d
}
