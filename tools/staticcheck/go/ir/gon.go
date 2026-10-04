package ir

import (
	"go/ast"
	"go/token"
	"go/types"
)

// exprList keeps the successful components of a Gon expression as separate
// SSA values. The operand's trailing error must not leak into calls or returns.
func (b *builder) exprList(fn *Function, e ast.Expr) []Value {
	if e, ok := unparen(e).(*ast.ErrorExpr); ok {
		return b.errorExpr(fn, e)
	}
	tuple := b.exprN(fn, e)
	emitDebugRef(fn, e, tuple, false)
	values := make([]Value, tuple.Type().(*types.Tuple).Len())
	for i := range values {
		values[i] = emitExtract(fn, tuple, i, e)
	}
	return values
}

func (b *builder) errorExpr(fn *Function, e *ast.ErrorExpr) []Value {
	if types.IsCanonicalResult(fn.typeOf(e.X)) {
		return b.resultExpr(fn, e)
	}
	var values []Value
	if _, ok := fn.typeOf(e.X).(*types.Tuple); ok {
		values = b.exprList(fn, e.X)
	} else {
		values = []Value{b.expr(fn, e.X)}
	}
	err := values[len(values)-1]
	failed := fn.newBasicBlock("error.handler")
	done := fn.newBasicBlock("error.done")
	emitIf(fn, emitCompare(fn, token.NEQ, err, zeroConst(err.Type(), e), e), failed, done, e)
	fn.currentBlock = failed
	if e.Body != nil {
		if !isBlankIdent(e.Err) {
			addr := emitLocalVar(fn, identVar(fn, e.Err), e)
			emitStore(fn, addr, err, e)
			emitDebugRef(fn, e.Err, addr, true)
		}
		b.stmt(fn, e.Body)
		emitJump(fn, done, e)
	} else {
		results := make([]Value, fn.source.Signature.Results().Len())
		for i := range results {
			results[i] = zeroConst(fn.typ(fn.source.Signature.Results().At(i).Type()), e)
		}
		results[len(results)-1] = err
		b.returnValues(fn, &ast.ReturnStmt{Return: e.OpPos}, results)
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
		edges = append(edges, emitConv(fn, b.expr(fn, branch.x), t, branch.x))
		emitJump(fn, done, e)
	}
	fn.currentBlock = done

	phi := &Phi{Edges: edges}
	phi.typ = t
	phi.comment = "condexpr"
	return done.emit(phi, e)
}
