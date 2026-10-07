package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// errorExpr removes the last, error-typed result of a call. The operand's
// original type is still recorded, so lowering can evaluate the complete call
// once before choosing the success or failure control-flow path.
func (check *Checker) errorExpr(x *operand, e *syntax.ErrorExpr) exprKind {
	fail := func(msg string) exprKind {
		check.error(e, InvalidErrorHandling, msg)
		x.invalidate()
		x.typ_ = Typ[Invalid]
		x.expr = e
		return statement
	}
	kind := check.rawExpr(nil, x, e.X, false)
	if !x.isValid() {
		x.expr = e
		return statement
	}

	if kind == conversion {
		return fail("error handling requires a function or method call")
	}
	if _, ok := syntax.Unparen(e.X).(*syntax.CallExpr); !ok {
		return fail("error handling requires a function or method call")
	}
	if check.sig == nil {
		return fail("error handling is only permitted inside a function")
	}

	errorType := universeError
	var results []*Var
	if tuple, ok := x.typ().(*Tuple); ok {
		results = tuple.vars
	} else if x.mode() == commaerr {
		// Error handling explicitly requests a C call's errno result.
		results = []*Var{NewVar(e.Pos(), check.pkg, "", x.typ()), NewVar(e.Pos(), check.pkg, "", errorType)}
		check.recordCommaOkTypes(e.X, []*operand{{mode_: value, typ_: x.typ()}, {mode_: value, typ_: errorType}})
	} else if x.mode() == value {
		results = []*Var{NewVar(e.Pos(), check.pkg, "", x.typ())}
	}
	if len(results) == 0 || !Identical(results[len(results)-1].typ, errorType) {
		return fail("error handling requires a final result of type error")
	}
	success := results[:len(results)-1]
	if e.Body == nil {
		sig := check.sig
		// Failure leaves the function through its final error or, in a test
		// function, through Fatal.
		problem := func() string {
			switch check.propagationTarget(sig, e) {
			case propagateToError, propagateToTest:
				return ""

			}
			return "error propagation requires an enclosing function with a final result of type error or, in a _test.go file, a first named parameter of type *testing.T, *testing.B, *testing.F or testing.TB"
		}
		if check.inferLambdaSig == sig {
			check.later(func() {
				if msg := problem(); msg != "" {
					check.error(e, InvalidErrorHandling, msg)
				}
			}).describef(e, "lambda error propagation")
		} else if msg := problem(); msg != "" {
			return fail(msg)
		}
	}
	if e.Context != nil {
		if e.Err == nil {
			return fail("error context requires an explicit error binding")
		}
		check.openScope(e, "error context")
		defer check.closeScope()
		obj := newVar(LocalVar, e.Err.Pos(), check.pkg, e.Err.Value, errorType)
		check.declare(check.scope, e.Err, obj, e.Context.Pos())
		check.usedVars[obj] = true
		var context operand
		check.expr(nil, &context, e.Context)
		check.assignment(&context, errorType, "error context")
	} else if e.Body != nil {
		if e.Err == nil {
			return fail("error handler requires an explicit error binding")
		}
		check.openScope(e.Body, "error handler")
		defer check.closeScope()
		obj := newVar(LocalVar, e.Err.Pos(), check.pkg, e.Err.Value, errorType)
		check.declare(check.scope, e.Err, obj, e.Body.Pos())
		// The binding can be intentionally ignored, just as a type-switch
		// guard variable need not be used in every case.
		check.usedVars[obj] = true

		// Handler labels would need to participate in the containing
		// function's label pass. Until their semantics are specified, reject
		// them and branches to labels explicitly. Nested functions retain
		// all the ordinary Go branch rules.
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
		// Do not inherit the surrounding loop or switch context: a handler
		// cannot resume evaluation of a value-producing expression by
		// breaking or continuing the enclosing statement.
		if check.inferLambdaSig == check.sig {
			env := check.environment
			check.later(func() {
				saved := check.environment
				check.environment = env
				check.stmtList(0, e.Body.List)
				check.environment = saved
			}).describef(e, "lambda error handler")
		} else {
			check.stmtList(0, e.Body.List)
		}
		if len(success) > 0 && !check.isTerminating(e.Body, "") {
			return fail("error handler for a value-producing call must terminate")
		}
	}

	x.expr = e
	x.mode_ = value
	switch len(success) {
	case 0:
		x.mode_ = novalue
		x.typ_ = NewTuple()
	case 1:
		x.typ_ = success[0].typ
	default:
		x.typ_ = NewTuple(success...)
	}
	check.hasCallOrRecv = true
	return statement
}
