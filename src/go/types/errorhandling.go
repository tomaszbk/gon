package types

import (
	"go/ast"
	"go/token"
	. "internal/types/errors"
)

// errorExpr removes the last, error-typed result of a call. The operand's
// original type is still recorded, so lowering can evaluate the complete call
// once before choosing the success or failure control-flow path.
func (check *Checker) errorExpr(x *operand, e *ast.ErrorExpr) exprKind {
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

	if IsCanonicalResult(x.typ()) {
		return check.resultErrorExpr(x, e)
	}
	if kind == conversion {
		return fail("error handling requires a function or method call")
	}
	if _, ok := ast.Unparen(e.X).(*ast.CallExpr); !ok {
		return fail("error handling requires a function or method call or canonical Result")
	}
	if check.sig == nil {
		return fail("error handling is only permitted inside a function")
	}

	errorType := universeError
	var results []*Var
	if tuple, ok := x.typ().(*Tuple); ok {
		results = tuple.vars
	} else if x.mode() == value {
		results = []*Var{NewVar(e.Pos(), check.pkg, "", x.typ())}
	}
	if len(results) == 0 || !Identical(results[len(results)-1].typ, errorType) {
		return fail("error handling requires a final result of type error")
	}
	success := results[:len(results)-1]
	if e.Body == nil {

		sig := check.sig
		valid := func() bool { n := sig.results.Len(); return n > 0 && Identical(sig.results.At(n-1).typ, errorType) }
		if check.inferLambdaSig == sig {
			check.later(func() {
				if !valid() {
					check.error(e, InvalidErrorHandling, "error propagation requires an enclosing function with a final result of type error")
				}
			}).describef(e, "lambda error propagation")
		} else if !valid() {
			return fail("error propagation requires an enclosing function with a final result of type error")
		}

	} else {
		if e.Err == nil {
			return fail("error handler requires an explicit error binding")
		}
		// During Eval, handler locals belong to the newly parsed expression,
		// while lookups in the surrounding scopes still use the caller's
		// original position. Preserve an existing boundary for nested handlers.
		if check.exprPos.IsValid() && check.exprScope == nil {
			check.exprScope = check.scope
			defer func() { check.exprScope = nil }()
		}
		check.openScope(e.Body, "error handler")
		defer check.closeScope()
		obj := newVar(LocalVar, e.Err.Pos(), check.pkg, e.Err.Name, errorType)
		check.declare(check.scope, e.Err, obj, e.Body.Pos())
		// The binding can be intentionally ignored, just as a type-switch
		// guard variable need not be used in every case.
		check.usedVars[obj] = true

		// Handler labels would need to participate in the containing
		// function's label pass. Until their semantics are specified, reject
		// them and branches to labels explicitly. Nested functions retain
		// all the ordinary Go branch rules.
		ast.Inspect(e.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncLit, *ast.LambdaExpr:
				return false
			case *ast.LabeledStmt:
				check.error(n, InvalidErrorHandling, "labels are not permitted in error handlers")
			case *ast.BranchStmt:
				if n.Tok == token.GOTO || n.Label != nil {
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
