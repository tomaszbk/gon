package typeutil

import (
	"go/token"
	"go/types"
	"testing"
)

func TestGonOptionalUnify(t *testing.T) {
	tp := types.NewTypeParam(types.NewTypeName(token.NoPos, nil, "T", nil), types.NewInterfaceType(nil, nil).Complete())
	x := types.NewOptional(tp)
	bindings := make(map[*types.TypeParam]types.Type)
	if !Unify(x, types.NewOptional(types.Typ[types.Int]), bindings) || bindings[tp] != types.Typ[types.Int] {
		t.Fatal("optional inference failed")
	}
	if Unify(x, types.NewPointer(types.Typ[types.Int]), nil) || Unify(tp, x, nil) {
		t.Fatal("optional identity/occurs check failed")
	}
}
