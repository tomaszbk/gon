package types

import (
	"cmd/internal/src"
	"testing"
)

func TestGonOptionalShapeLinkIdentity(t *testing.T) {
	optional := func(elem *Type) *Type {
		payload := NewStruct([]*Field{NewField(src.NoXPos, nil, elem)})
		typ := NewStruct([]*Field{NewField(src.NoXPos, nil, NewStruct(nil)), NewField(src.NoXPos, nil, NewStruct(nil)), NewField(src.NoXPos, nil, payload)})
		typ.SetIsOptional(true)
		return typ
	}
	base := NewStruct(nil)
	baseName := NewNamed(&testTypeName{sym: ShapePkg.Lookup(base.LinkString())})
	baseName.SetUnderlying(base)
	wrapped := optional(base)
	wrappedName := NewNamed(&testTypeName{sym: ShapePkg.Lookup(wrapped.LinkString())})
	wrappedName.SetUnderlying(wrapped)
	shapePayload := optional(baseName)
	if wrappedName.LinkString() == shapePayload.LinkString() {
		t.Fatalf("shape of optional and optional of shape collided: %s", wrappedName.LinkString())
	}
}
