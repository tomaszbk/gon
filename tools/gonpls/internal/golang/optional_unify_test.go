package golang

import (
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/gopls/internal/util/fingerprint"
)

func TestGonOptionalUnify(t *testing.T) {
	tp := types.NewTypeParam(types.NewTypeName(token.NoPos, nil, "T", nil), types.NewInterfaceType(nil, nil).Complete())
	_ = types.NewSignatureType(nil, nil, []*types.TypeParam{tp}, nil, nil, false)
	payload := types.Typ[types.Int]
	x, y := types.NewOptional(tp), types.NewOptional(payload)
	bindings := make(map[*types.TypeParam]types.Type)
	if !unify(x, y, bindings) || bindings[tp] != payload {
		t.Fatalf("optional inference: %v", bindings)
	}
	if unify(x, types.NewPointer(payload), nil) || unify(payload, y, nil) {
		t.Fatal("optional identity lost")
	}
	if unify(tp, types.NewOptional(tp), nil) {
		t.Fatal("optional occurs check failed")
	}
	for _, test := range []struct {
		x, y  types.Type
		match bool
	}{
		{x, y, true}, {y, types.NewOptional(types.Typ[types.String]), false},
		{y, types.NewOptional(types.NewOptional(payload)), false},
		{y, types.NewPointer(payload), false},
	} {
		a, _ := fingerprint.Encode(test.x)
		b, _ := fingerprint.Encode(test.y)
		at, bt := fingerprint.Parse(a), fingerprint.Parse(b)
		if at.String() != a || bt.String() != b || fingerprint.Matches(at, bt) != test.match {
			t.Fatalf("optional fingerprint %s / %s", a, b)
		}
	}
}
