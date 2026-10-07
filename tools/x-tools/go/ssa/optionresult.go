package ssa

import (
	"go/ast"
	"go/token"
	"go/types"
)

func enumAlternative(typ types.Type, name string) *types.EnumVariant {
	e := types.EnumStorageOf(typ)
	if e == nil {
		panic("missing canonical enum")
	}
	for i := range e.NumVariants() {
		v := e.Variant(i)
		if v.Name() == name {
			return v
		}
	}
	panic("missing canonical alternative")
}
func canonicalPayload(fn *Function, value Value, variant *types.EnumVariant, pos token.Pos) Value {
	return enumField(fn, enumField(fn, value, variant.StorageIndex(), pos), 0, pos)
}
func (b *builder) optionNilCompare(fn *Function, e *ast.BinaryExpr) (Value, bool) {
	if e.Op != token.EQL && e.Op != token.NEQ {
		return nil, false
	}
	optional, other := e.X, e.Y
	if !types.IsOptional(fn.typeOf(optional)) {
		optional, other = other, optional
	}
	if !types.IsOptional(fn.typeOf(optional)) || fn.typeOf(other) != types.Typ[types.UntypedNil] {
		return nil, false
	}
	value := b.expr(fn, optional)
	tag := enumField(fn, value, 0, e.OpPos)
	return emitCompare(fn, e.Op, tag, emitConv(fn, intConst(0), tag.Type()), e.OpPos), true
}
func (b *builder) optionExpr(fn *Function, e *ast.OptionalExpr) Value {
	value := b.expr(fn, e.X)
	none := fn.newBasicBlock("option.none")
	some := fn.newBasicBlock("option.some")
	tag := enumField(fn, value, 0, e.Question)
	emitIf(fn, emitCompare(fn, token.EQL, tag, emitConv(fn, intConst(0), tag.Type()), e.Question), none, some)
	fn.currentBlock = none
	result := fn.typ(fn.source.Signature.Results().At(0).Type())
	b.returnValues(fn, &ast.ReturnStmt{Return: e.Question}, []Value{zeroConst(result)})
	fn.currentBlock = some
	return canonicalPayload(fn, value, enumAlternative(value.Type(), "$present"), e.Question)
}
func (b *builder) optionPresent(fn *Function, value Value, absent *BasicBlock, e ast.Node) Value {
	present := fn.newBasicBlock("option.present")
	tag := enumField(fn, value, 0, e.Pos())
	emitIf(fn, emitCompare(fn, token.NEQ, tag, emitConv(fn, intConst(0), tag.Type()), e.Pos()), present, absent)
	fn.currentBlock = present
	return canonicalPayload(fn, value, enumAlternative(value.Type(), "$present"), e.Pos())
}
