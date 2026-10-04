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
func (b *builder) resultExpr(fn *Function, e *ast.ErrorExpr) []Value {
	value := b.expr(fn, e.X)
	v := enumAlternative(value.Type(), "Err")
	failed := fn.newBasicBlock("result.err")
	done := fn.newBasicBlock("result.ok")
	tag := enumField(fn, value, 0, e.OpPos)
	emitIf(fn, emitCompare(fn, token.EQL, tag, emitConv(fn, intConst(int64(v.Tag())), tag.Type()), e.OpPos), failed, done)
	fn.currentBlock = failed
	problem := canonicalPayload(fn, value, v, e.OpPos)
	if e.Body != nil {
		if !isBlankIdent(e.Err) {
			addr := emitLocalVar(fn, identVar(fn, e.Err))
			emitStore(fn, addr, problem, e.OpPos)
		}
		b.stmt(fn, e.Body)
		emitJump(fn, done)
	} else {
		result := fn.typ(fn.source.Signature.Results().At(0).Type())
		err := enumAlternative(result, "Err")
		resultValue := enumValue(fn, result, err, []Value{emitConv(fn, problem, err.Field(0).Type())}, e.OpPos)
		b.returnValues(fn, &ast.ReturnStmt{Return: e.OpPos}, []Value{resultValue})
	}
	fn.currentBlock = done
	return []Value{canonicalPayload(fn, value, enumAlternative(value.Type(), "Ok"), e.OpPos)}
}
func (b *builder) optionPresent(fn *Function, value Value, absent *BasicBlock, e ast.Node) Value {
	present := fn.newBasicBlock("option.present")
	tag := enumField(fn, value, 0, e.Pos())
	emitIf(fn, emitCompare(fn, token.NEQ, tag, emitConv(fn, intConst(0), tag.Type()), e.Pos()), present, absent)
	fn.currentBlock = present
	return canonicalPayload(fn, value, enumAlternative(value.Type(), "$present"), e.Pos())
}
