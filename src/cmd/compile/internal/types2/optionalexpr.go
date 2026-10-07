package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

func optionType(payload Type) Type { return NewOptional(payload) }

func (check *Checker) optionExpr(x *operand, e *syntax.OptionalExpr) exprKind {
	check.exprOrType(x, e.X, false)
	if x.mode() == typexpr {
		x.typ_, x.expr = optionType(x.typ()), e
		return expression
	}
	check.singleValue(x)
	if !x.isValid() {
		x.expr = e
		return statement
	}
	if !IsOptional(x.typ()) {
		check.error(e, InvalidNilSafety, "absence propagation requires an optional value")
		x.invalidate()
		x.expr = e
		return statement
	}
	sig := check.sig
	valid := func() bool {
		return sig != nil && sig.Results().Len() == 1 && IsOptional(sig.Results().At(0).Type())
	}
	if check.inferLambdaSig != nil && check.inferLambdaSig == sig {
		check.later(func() {
			if !valid() {
				check.error(e, InvalidNilSafety, "absence propagation requires an enclosing function returning exactly one optional")
			}
		}).describef(e, "lambda absence propagation")
	} else if !valid() {
		check.error(e, InvalidNilSafety, "absence propagation requires an enclosing function returning exactly one optional")
		x.invalidate()
		x.expr = e
		return statement
	}
	x.typ_ = OptionalOf(x.typ()).Elem()
	x.mode_, x.expr = value, e
	check.hasCallOrRecv = true
	return statement
}
