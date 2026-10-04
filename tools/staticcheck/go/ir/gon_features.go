package ir

import (
	"go/ast"
	"go/token"
	"go/types"
)

// lambdaSyntax supplies only the syntax needed by createSyntacticParams and
// statement lowering. Parameter objects and all expression types remain those
// recorded by the checker; no type names or source re-checking are needed.
func lambdaSyntax(e *ast.LambdaExpr, sig *types.Signature) (*ast.FuncType, *ast.BlockStmt) {
	ft := &ast.FuncType{Func: e.Lparen, Params: &ast.FieldList{Opening: e.Lparen, Closing: e.Rparen}, Results: &ast.FieldList{}}
	for _, id := range e.Params {
		ft.Params.List = append(ft.Params.List, &ast.Field{Names: []*ast.Ident{id}})
	}
	for range sig.Results().Len() {
		ft.Results.List = append(ft.Results.List, &ast.Field{})
	}
	if e.Block != nil {
		return ft, e.Block
	}
	var stmt ast.Stmt = &ast.ExprStmt{X: e.Body}
	if sig.Results().Len() != 0 {
		stmt = &ast.ReturnStmt{Return: e.Arrow, Results: []ast.Expr{e.Body}}
	}
	return ft, &ast.BlockStmt{Lbrace: e.Arrow, List: []ast.Stmt{stmt}, Rbrace: e.End()}
}

func (b *builder) nilPresent(fn *Function, v Value, absent *BasicBlock, e ast.Node) {
	present := fn.newBasicBlock("nil.present")
	emitIf(fn, emitCompare(fn, token.NEQ, v, zeroConst(v.Type(), e), e), present, absent, e)
	fn.currentBlock = present
}

// nilOperand leaves only the present path active. In particular it must not
// box a nil pointer into an interface before testing it, or eagerly dereference
// a guarded pointer. Nested primary chains have independent guard scopes.
func (b *builder) nilOperand(fn *Function, e ast.Expr, absent *BasicBlock) Value {
	switch e := ast.Unparen(e).(type) {
	case *ast.SafeNavExpr:
		saved := fn.nilAbsent
		fn.nilAbsent = absent
		v := b.expr(fn, e.X)
		fn.nilAbsent = saved
		return v
	case *ast.StarExpr:
		ptr := b.nilOperand(fn, e.X, absent)
		b.nilPresent(fn, ptr, absent, e)
		return emitLoad(fn, ptr, e)
	default:
		return b.expr(fn, e)
	}
}

func (b *builder) safeNav(fn *Function, e *ast.SafeNavExpr, discard bool) Value {
	absent := fn.newBasicBlock("nil.absent")
	done := fn.newBasicBlock("nil.done")
	saved := fn.nilAbsent
	fn.nilAbsent = absent
	var value Value
	if errExpr, ok := ast.Unparen(e.X).(*ast.ErrorExpr); ok && discard {
		b.errorExpr(fn, errExpr)
	} else {
		value = b.expr(fn, e.X)
	}
	fn.nilAbsent = saved
	var t types.Type
	if !discard {
		t = fn.typeOf(e)
		if types.IsOptional(t) {
			value = enumValue(fn, t, enumAlternative(t, "$present"), []Value{value}, e)
		} else {
			value = emitConv(fn, value, t, e)
		}
	}
	emitJump(fn, done, e)
	fn.currentBlock = absent
	emitJump(fn, done, e)
	fn.currentBlock = done
	if discard {
		return nil
	}
	phi := &Phi{Edges: []Value{value, zeroConst(t, e)}}
	phi.typ = t
	phi.comment = "safe navigation"
	return done.emit(phi, e)
}

func (b *builder) coalesce(fn *Function, e *ast.BinaryExpr) Value {
	fallback := fn.newBasicBlock("coalesce.fallback")
	done := fn.newBasicBlock("coalesce.done")
	value := b.nilOperand(fn, e.X, fallback)
	_, safe := ast.Unparen(e.X).(*ast.SafeNavExpr)
	optionChain := safe && types.IsOptional(fn.typeOf(e.X))
	if optionChain {
		// The outer Option was already tested; preserve its final payload.
	} else if types.IsOptional(value.Type()) {
		value = b.optionPresent(fn, value, fallback, e.X)
	} else if nillable(value.Type()) {
		b.nilPresent(fn, value, fallback, e.X)
	}
	t := fn.typeOf(e)
	value = emitConv(fn, value, t, e)
	emitJump(fn, done, e)
	fn.currentBlock = fallback
	other := emitConv(fn, b.expr(fn, e.Y), t, e.Y)
	emitJump(fn, done, e)
	fn.currentBlock = done
	phi := &Phi{Edges: []Value{value, other}}
	phi.typ = t
	phi.comment = "coalesce"
	return done.emit(phi, e)
}

func (b *builder) coalesceAssign(fn *Function, s *ast.AssignStmt) {
	loc := b.addr(fn, s.Lhs[0], false)
	value := loc.load(fn, s)
	fallback := fn.newBasicBlock("coalesce.assign")
	done := fn.newBasicBlock("coalesce.done")
	if types.IsOptional(value.Type()) {
		tag := enumField(fn, value, 0, s)
		emitIf(fn, emitCompare(fn, token.NEQ, tag, emitConv(fn, intConst(0, s), tag.Type(), s), s), done, fallback, s)
	} else {
		emitIf(fn, emitCompare(fn, token.NEQ, value, zeroConst(value.Type(), s), s), done, fallback, s)
	}
	fn.currentBlock = fallback
	rhs := b.expr(fn, s.Rhs[0])
	if types.IsOptional(value.Type()) {
		rhs = enumValue(fn, value.Type(), enumAlternative(value.Type(), "$present"), []Value{rhs}, s)
	}
	loc.store(fn, rhs, s)
	emitJump(fn, done, s)
	fn.currentBlock = done
}
