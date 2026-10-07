package ssa

import (
	"go/ast"
	"go/token"
	"go/types"
)

// exprList keeps the successful components of a Gon expression as separate
// SSA values. The operand's trailing error must not leak into calls or returns.
func (b *builder) exprList(fn *Function, e ast.Expr) []Value {
	if e, ok := ast.Unparen(e).(*ast.ErrorExpr); ok {
		return b.errorExpr(fn, e)
	}
	tuple := b.exprN(fn, e)
	emitDebugRef(fn, e, tuple, false)
	values := make([]Value, tuple.Type().(*types.Tuple).Len())
	for i := range values {
		values[i] = emitExtract(fn, tuple, i)
	}
	return values
}

func (b *builder) errorExpr(fn *Function, e *ast.ErrorExpr) []Value {
	var values []Value
	if _, ok := fn.typeOf(e.X).(*types.Tuple); ok {
		values = b.exprList(fn, e.X)
	} else {
		values = []Value{b.expr(fn, e.X)}
	}
	err := values[len(values)-1]
	failed := fn.newBasicBlock("error.handler")
	done := fn.newBasicBlock("error.done")
	emitIf(fn, emitCompare(fn, token.NEQ, err, zeroConst(err.Type()), e.OpPos), failed, done)
	fn.currentBlock = failed
	if e.Body != nil || e.Context != nil {
		if !isBlankIdent(e.Err) {
			addr := emitLocalVar(fn, identVar(fn, e.Err))
			emitStore(fn, addr, err, e.OpPos)
			emitDebugRef(fn, e.Err, addr, true)
		}
	}
	if e.Context != nil {
		failure := emitConv(fn, b.expr(fn, e.Context), types.Universe.Lookup("error").Type())
		b.propagateFailure(fn, e, failure)
	} else if e.Body != nil {
		b.stmt(fn, e.Body)
		emitJump(fn, done)
	} else {
		b.propagateFailure(fn, e, err)
	}
	fn.currentBlock = done
	return values[:len(values)-1]
}

// condExpr lowers the conditional expression "if Cond { Then } else { Else }"
// like the value of a && or || expression: it evaluates the condition, then
// only the selected branch, converted to the type of the expression as in an
// assignment, and joins the branches with a phi. Constant conditional
// expressions do not get here.
func (b *builder) condExpr(fn *Function, e *ast.CondExpr, tv types.TypeAndValue) Value {
	// An untyped result, such as a condition of an if statement or an
	// untyped string operand of len or range, has its default type.
	t := types.Default(fn.typ(tv.Type))
	then := fn.newBasicBlock("condexpr.then")
	els := fn.newBasicBlock("condexpr.else")
	done := fn.newBasicBlock("condexpr.done")
	b.cond(fn, e.Cond, then, els)

	var edges []Value
	for _, branch := range [...]struct {
		block *BasicBlock
		x     ast.Expr
	}{{then, e.Then}, {els, e.Else}} {
		fn.currentBlock = branch.block
		edges = append(edges, emitConv(fn, b.expr(fn, branch.x), t))
		emitJump(fn, done)
	}
	fn.currentBlock = done

	phi := &Phi{Edges: edges, Comment: "condexpr"}
	phi.pos = e.If
	phi.typ = t
	return done.emit(phi)
}
