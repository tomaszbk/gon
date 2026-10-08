package noder

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types2"
)

// condExpr writes the conditional expression expr, "if Cond { Then } else
// { Else }", which is not constant: expr writes constant conditional
// expressions as constants, so their branches are never evaluated.
func (w *writer) condExpr(expr *syntax.CondExpr) {
	tv := w.p.typeAndValue(expr)
	typ := tv.Type
	if isUntyped(typ) {
		// Like other untyped expressions, a conditional expression with
		// untyped branches is left untyped by a few contexts, such as an if
		// or for condition, or ranging over, indexing, slicing or measuring
		// a string.
		typ = idealType(tv)
		assert(typ != nil)
	}

	w.Code(exprCond)
	w.pos(expr)
	w.typ(typ)
	w.expr(expr.Cond)
	w.condBranch(typ, expr.Then)
	w.condBranch(typ, expr.Else)
}

// condBranch writes a branch of a conditional expression converted to the
// type typ of the conditional expression. The conversion is implicit, as
// for an assignment, unless the conditional expression is the operand of a
// conversion, which then applies to each branch separately.
func (w *writer) condBranch(typ types2.Type, branch syntax.Expr) {
	implicit := types2.AssignableTo(w.p.typeOf(branch), typ)
	if lift := w.p.info.OptionalConversions[branch]; lift != nil && types2.Identical(lift, typ) {
		implicit = true
	}
	w.convertExpr(typ, branch, implicit)
}

// condExpr lowers a conditional expression to a temporary assigned by an
// if statement:
//
//	var tmp T
//	if cond {
//		tmp = then
//	} else {
//		tmp = else
//	}
//
// represented as an inline expression with result tmp, like errorExpr. The
// ordering pass evaluates the statements where the expression appears in the
// lexical left-to-right order of evaluation: the condition first and exactly
// once, then only the selected branch. InlinedCallExpr is only an IR
// container here: no function or closure is made, so return statements in a
// branch (from error propagation) return from the enclosing function.
func (r *reader) condExpr() ir.Node {
	pos := r.pos()
	typ := r.typ()
	cond := r.expr()
	then := r.expr()
	els := r.expr()

	// The temporary is assigned on both paths, so it has no single
	// defining assignment (Defn): its value is not static.
	tmp := r.temp(pos, typ)
	assign := func(x ir.Node) []ir.Node {
		return []ir.Node{typecheck.Stmt(r.curfn, ir.NewAssignStmt(x.Pos(), tmp, x))}
	}
	body := []ir.Node{
		typecheck.Stmt(r.curfn, ir.NewDecl(pos, ir.ODCL, tmp)),
		typecheck.Stmt(r.curfn, ir.NewIfStmt(pos, cond, assign(then), assign(els))),
	}
	res := ir.NewInlinedCallExpr(pos, body, []ir.Node{tmp})
	res.SetType(typ)
	res.SetTypecheck(1)
	return res
}
