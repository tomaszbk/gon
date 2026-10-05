package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

func canonicalPayload(t Type, name string) Type {
	e := EnumStorageOf(t)
	if e == nil {
		return Typ[Invalid]
	}
	v := e.Lookup(name, nil)
	if v == nil || v.NumFields() != 1 {
		return Typ[Invalid]
	}
	return v.Field(0).Type()
}

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
	x.typ_ = canonicalPayload(x.typ(), "$present")
	x.mode_, x.expr = value, e
	check.hasCallOrRecv = true
	return statement
}

func (check *Checker) resultErrorExpr(x *operand, e *syntax.ErrorExpr) exprKind {
	failure := func(message string) exprKind {
		check.error(e, InvalidErrorHandling, message)
		x.invalidate()
		x.expr = e
		return statement
	}
	if check.sig == nil {
		return failure("error handling is only permitted inside a function")
	}
	success, problem := canonicalPayload(x.typ(), "Ok"), canonicalPayload(x.typ(), "Err")
	if e.Body == nil {
		sig := check.sig
		// Failure leaves the function through a Result, through a final
		// error that accepts the payload, or, in a test function, through Fatal.
		invalid := func() string {
			switch check.propagationTarget(sig, e) {
			case propagateToTest:
				return ""
			case propagateToResult:
				if AssignableTo(problem, enclosingResultError(sig)) {
					return ""
				}
			case propagateToError:
				if AssignableTo(problem, universeError) {
					return ""
				}
				return "Result propagation into a final result of type error requires an error type assignable to error"
			}
			return "Result propagation requires exactly one enclosing Result with an assignable error type, a final result of type error, or, in a _test.go file, a first named parameter of type *testing.T, *testing.B, *testing.F or testing.TB"
		}
		if check.inferLambdaSig != nil && check.inferLambdaSig == sig {
			check.later(func() {
				if msg := invalid(); msg != "" {
					check.error(e, InvalidErrorHandling, msg)
				}
			}).describef(e, "lambda Result propagation")
		} else if msg := invalid(); msg != "" {
			return failure(msg)
		}
	} else {
		if e.Err == nil {
			return failure("error handler requires an explicit error binding")
		}
		check.openScope(e.Body, "Result error handler")
		defer check.closeScope()
		obj := newVar(LocalVar, e.Err.Pos(), check.pkg, e.Err.Value, problem)
		check.declare(check.scope, e.Err, obj, e.Body.Pos())
		check.usedVars[obj] = true
		syntax.Inspect(e.Body, func(n syntax.Node) bool {
			switch n := n.(type) {
			case *syntax.FuncLit, *syntax.LambdaExpr:
				return false
			case *syntax.LabeledStmt:
				check.error(n, InvalidErrorHandling, "labels are not permitted in error handlers")
			case *syntax.BranchStmt:
				if n.Tok == syntax.Goto || n.Label != nil {
					check.error(n, InvalidErrorHandling, "branches to labels are not permitted in error handlers")
				}
			}
			return true
		})
		if check.inferLambdaSig == check.sig {
			env := check.environment
			check.later(func() {
				saved := check.environment
				check.environment = env
				check.stmtList(0, e.Body.List)
				check.environment = saved
			}).describef(e, "lambda Result handler")
		} else {
			check.stmtList(0, e.Body.List)
		}
		if !check.isTerminating(e.Body, "") {
			return failure("Result error handler must terminate")
		}
	}
	x.mode_, x.typ_, x.expr = value, success, e
	check.hasCallOrRecv = true
	return statement
}
