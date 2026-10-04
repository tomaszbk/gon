package types

import (
	"go/ast"
	"go/token"
	. "internal/types/errors"
)

func isTargetExpr(e ast.Expr) bool {
	if isOptionContextExpr(e) {
		return true
	}
	switch e := e.(type) {
	case *ast.LambdaExpr, *ast.SafeNavExpr:
		return true
	case *ast.BinaryExpr:
		return e.Op == token.COALESCE
	case *ast.ParenExpr:
		return isNilTargetExpr(e.X) || isCondExpr(e) || isMatchExpr(e)
	}
	return isCondExpr(e) || isMatchExpr(e)
}

func isNilTargetExpr(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.SafeNavExpr:
		return true
	case *ast.BinaryExpr:
		return e.Op == token.COALESCE
	case *ast.ParenExpr:
		return isNilTargetExpr(e.X)
	}
	return false
}

func (check *Checker) nilGuard(x *operand, e *ast.NilGuardExpr) {
	if check.nilGuardDepth == 0 {
		check.error(e, InvalidSyntaxTree, "nil guard outside safe-navigation chain")
		return
	}
	check.expr(nil, x, e.X)
	if x.isValid() {
		if IsOptional(x.typ()) {
			if check.nilLegacyGuardSeen {
				check.error(e, InvalidNilSafety, "mixed Option and nil navigation requires an explicit boundary")
				x.invalidate()
				x.expr = e
				return
			}
			check.nilOptionGuardSeen = true
			x.typ_ = canonicalPayload(x.typ(), "$present")
			x.mode_, x.expr = value, e
			return
		}
		if check.nilOptionGuardSeen {
			check.error(e, InvalidNilSafety, "mixed Option and nil navigation requires an explicit boundary")
			x.invalidate()
			x.expr = e
			return
		}
		check.nilLegacyGuardSeen = true
		ok := underIs(x.typ(), func(t Type) bool {
			switch t.(type) {
			case *Pointer, *Signature:
				return true
			case *Interface:
				return !isTypeParam(x.typ())
			}
			return false
		})
		if !ok {
			check.errorf(e, InvalidNilSafety, "invalid operation: safe navigation requires a pointer, interface or function, not %s", x)
			x.invalidate()
		} else {
			x.mode_ = value
		}
	}
	x.expr = e
}

func (check *Checker) safeNavExpr(T *target, x *operand, e *ast.SafeNavExpr, consume bool) exprKind {
	previousOption, previousLegacy := check.nilOptionGuardSeen, check.nilLegacyGuardSeen
	check.nilOptionGuardSeen, check.nilLegacyGuardSeen = false, false
	defer func() { check.nilOptionGuardSeen, check.nilLegacyGuardSeen = previousOption, previousLegacy }()
	check.nilGuardDepth++
	kind := check.rawExpr(nil, x, e.X, false)
	check.nilGuardDepth--
	check.hasCallOrRecv = true
	if !x.isValid() {
		x.expr = e
		return kind
	}
	if check.nilOptionGuardSeen {
		check.singleValue(x)
		if !x.isValid() {
			x.expr = e
			return kind
		}
		if x.mode() != novalue {
			x.typ_ = optionType(x.typ())
		}
	}
	if !check.nilOptionGuardSeen && !consume && (x.mode() == novalue || !hasNil(x.typ())) {
		check.errorf(e, InvalidNilSafety, "%s cannot be nil; use ?? to provide a value when it is absent", x)
		x.invalidate()
		x.expr = e
		return kind
	}
	if T != nil && T.kind != inferTarget && x.mode() != novalue {
		check.condBranchTo(x, T.typ, T.kind == convTarget)
		if x.isValid() {
			x.typ_ = T.typ
		}
	}
	if x.mode() != novalue {
		x.mode_ = value
	}
	x.expr = e
	return kind
}

func (check *Checker) coalesceExpr(T *target, x *operand, e *ast.BinaryExpr) {
	check.hasCallOrRecv = true
	var a, b operand
	guarded := false
	switch left := ast.Unparen(e.X).(type) {
	case *ast.SafeNavExpr:
		check.safeNavExpr(nil, &a, left, true)
		check.record(&a)
		guarded = true
	case *ast.StarExpr:
		guarded = true
		check.expr(nil, &a, e.X)
	case *ast.TypeAssertExpr:
		check.error(e.X, InvalidNilSafety, "type assertion on the left of ?? is reserved; use v, ok := x.(T)")
		check.expr(nil, &a, e.X)
		a.invalidate()
	default:
		check.expr(nil, &a, e.X)
	}
	if guarded && a.isValid() {
		if a.mode() != novalue {
			a.mode_ = value
		}
		check.recordTypeAndValue(e.X, a.mode(), a.typ(), nil)
	}
	check.exclude(&a, 1<<novalue|1<<builtin|1<<typexpr)
	check.singleValue(&a)
	if a.isValid() && IsOptional(a.typ()) {
		a.typ_ = canonicalPayload(a.typ(), "$present")
		a.mode_ = value
		guarded = true
	}
	if a.isValid() && (!guarded && !hasNil(a.typ()) || a.isNil()) {
		check.errorf(e.X, InvalidNilSafety, "invalid operation: operator ?? not defined on %s", &a)
		a.invalidate()
	}
	bt := T
	if bt != nil && bt.kind == inferTarget {
		bt = nil
	}
	if bt == nil && a.isValid() {
		bt = newTarget(a.typ(), "coalescing operand")
	}
	check.expr(bt, &b, e.Y)
	x.expr = e
	if !a.isValid() || !b.isValid() {
		x.invalidate()
		return
	}
	typ := a.typ()
	if T != nil && T.kind != inferTarget {
		if isNonTypeParamInterface(T.typ) && !b.isNil() && isUntyped(b.typ()) {
			dest := Default(b.typ())
			if !isNonTypeParamInterface(a.typ()) {
				dest = a.typ()
			}
			check.assignment(&b, dest, "coalescing operand")
		}
		check.condBranchTo(&a, T.typ, T.kind == convTarget)
		check.condBranchTo(&b, T.typ, T.kind == convTarget)
		typ = T.typ
	} else if isUntyped(b.typ()) {
		check.assignment(&b, typ, "coalescing operand")
	} else if !Identical(typ, b.typ()) {
		check.errorf(e, MismatchedTypes, "invalid operation: %v (mismatched types %s and %s)", e, typ, b.typ())
		b.invalidate()
	}
	if !a.isValid() || !b.isValid() {
		x.invalidate()
		return
	}
	x.mode_, x.typ_ = value, typ
}

func (check *Checker) coalesceAssign(lhs, rhs ast.Expr) {
	var a, b operand
	check.expr(nil, &a, lhs)
	if !a.isValid() {
		check.use(rhs)
		return
	}
	if IsOptional(a.typ()) {
		payload := canonicalPayload(a.typ(), "$present")
		check.lhsVar(lhs)
		check.expr(newTarget(payload, "Option coalescing assignment"), &b, rhs)
		check.assignment(&b, payload, "Option coalescing assignment")
		return
	}
	if !hasNil(a.typ()) {
		check.errorf(lhs, InvalidNilSafety, "invalid operation: operator ??= not defined on %s", &a)
		return
	}
	check.expr(newTarget(a.typ(), "assignment"), &b, rhs)
	check.assignVar(lhs, nil, &b, "assignment")
}
