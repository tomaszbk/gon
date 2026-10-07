package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// conditionPatternTests permits only tests that form the condition itself or
// are leaves in its top-level && chain. Other expressions introduce no binds.
func conditionPatternTests(condition syntax.Expr) map[*syntax.PatternTestExpr]bool {
	permitted := make(map[*syntax.PatternTestExpr]bool)
	var walk func(syntax.Expr)
	walk = func(expression syntax.Expr) {
		switch expression := syntax.Unparen(expression).(type) {
		case *syntax.PatternTestExpr:
			permitted[expression] = true
		case *syntax.Operation:
			if expression.Op == syntax.AndAnd && expression.Y != nil {
				walk(expression.X)
				walk(expression.Y)
			}
		}
	}
	walk(condition)
	return permitted
}

func (check *Checker) patternTestExpr(x *operand, e *syntax.PatternTestExpr) {
	var subject operand
	check.expr(nil, &subject, e.X)
	if subject.isValid() {
		check.assignment(&subject, nil, "pattern test operand")
	}
	var bindings []matchBinding
	coverage := check.matchPattern(e.Pattern, subject.typ(), false, &bindings)
	if subject.isValid() {
		budget := 10000
		if matchUseful([][]*matchCoverage{{coverage}}, []*matchCoverage{{wild: true}}, []Type{subject.typ()}, &budget) == nil {
			check.error(e.Pattern, InvalidMatch, "pattern test must not use a pattern that always matches")
		}
	}
	if len(bindings) != 0 && !check.patternTestContexts[e] {
		check.error(e.Pattern, InvalidMatch, "pattern test bindings require an if condition or a top-level && operand of that condition")
	} else {
		for _, binding := range bindings {
			obj := newVar(LocalVar, binding.name.Pos(), check.pkg, binding.name.Value, binding.typ)
			check.declare(check.scope, binding.name, obj, syntax.EndPos(e))
		}
	}
	x.expr, x.mode_, x.typ_ = e, value, Typ[UntypedBool]
	if !subject.isValid() {
		x.invalidate()
	}
	check.hasCallOrRecv = true
}

func (check *Checker) openPatternScope(condition syntax.Expr, body syntax.Node) {
	scope := NewScope(check.scope, condition.Pos(), syntax.EndPos(body), "if pattern bindings")
	check.recordScope(condition, scope)
	check.scope = scope
}
