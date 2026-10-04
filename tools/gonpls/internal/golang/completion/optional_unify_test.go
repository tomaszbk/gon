package completion

import (
	"go/token"
	"go/types"
	"testing"
)

func TestGonOptionalInference(t *testing.T) {
	tp := types.NewTypeParam(types.NewTypeName(token.NoPos, nil, "T", nil), types.NewInterfaceType(nil, nil).Complete())
	x := types.NewOptional(tp)
	u := newUnifier([]*types.TypeParam{tp}, nil)
	if !u.unify(x, types.NewOptional(types.Typ[types.Int]), 0) || *u.handles[tp] != types.Typ[types.Int] {
		t.Fatal("optional payload was not inferred")
	}
	if u.unify(x, types.NewOptional(types.Typ[types.String]), 0) || u.unify(x, types.NewPointer(types.Typ[types.Int]), 0) {
		t.Fatal("conflicting optional inference accepted")
	}
	sig := types.NewSignatureType(nil, nil, []*types.TypeParam{tp}, types.NewTuple(types.NewParam(token.NoPos, nil, "value", x)), nil, false)
	if !inferableTypeParams(sig)[tp] {
		t.Fatal("completion omitted inferable optional parameter")
	}
}
