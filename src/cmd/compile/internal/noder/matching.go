// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/compile/internal/types2"
	"cmd/internal/src"
)

const (
	matchWildcard = iota
	matchBinding
	matchValue
	matchVariant
	// matchInterface wraps a variant pattern whose subject is an interface. It
	// is followed by a flag for the error-tree search, the runtime type
	// information that search or a dynamic type assertion needs, and the
	// ordinary variant pattern for the enum type that was found.
	matchInterface
)

func (w *writer) matchExpr(e *syntax.MatchExpr, statement bool) {
	if statement {
		w.Code(stmtMatch)
	} else {
		w.Code(exprMatch)
	}
	w.pos(e)
	var result types2.Type
	if !statement {
		tv := w.p.typeAndValue(e)
		result = tv.Type
		if isUntyped(result) {
			result = idealType(tv)
		}
		w.typ(result)
	}
	w.expr(e.Tag)
	w.Len(len(e.Arms))
	for i, a := range e.Arms {
		w.openScope(a.Pos())
		w.pos(a)
		w.Bool(matchCatchall(a) || i+1 == len(e.Arms) && a.Guard == nil)
		patterns := a.Patterns
		if len(patterns) == 0 {
			patterns = []*syntax.MatchPattern{nil}
		}
		w.Len(len(patterns))
		for _, pattern := range patterns {
			w.matchPattern(pattern, w.p.typeOf(e.Tag))
		}
		w.optExpr(a.Guard)
		if statement {
			w.blockStmt(a.Body)
		} else {
			w.condBranch(result, a.Value)
		}
		w.closeScope(syntax.EndPos(a))
	}
}

func matchCatchall(a *syntax.MatchArm) bool {
	if a.Guard != nil {
		return false
	}
	if len(a.Patterns) == 0 {
		return true
	}
	if len(a.Patterns) != 1 {
		return false
	}
	pattern := a.Patterns[0]
	n, ok := pattern.Value.(*syntax.Name)
	return ok && pNoPresence(pattern) && n.Value == "_" && !pattern.Lparen.IsKnown() && !pattern.Lbrace.IsKnown()
}

func pNoPresence(p *syntax.MatchPattern) bool { return p.Inner == nil && !p.Question.IsKnown() }

func (w *writer) matchPattern(p *syntax.MatchPattern, t types2.Type) {
	if p == nil {
		w.Len(matchWildcard)
		return
	}
	if p.Inner != nil {
		if !p.Question.IsKnown() {
			w.matchPattern(p.Inner, t)
			return
		}
		w.Len(matchVariant)
		w.Len(1)
		w.Len(2)
		w.Len(1)
		w.Len(0)
		w.matchPattern(p.Inner, types2.OptionalOf(t).Elem())
		return
	}
	if n, ok := p.Value.(*syntax.Name); ok && n.Value == "nil" && types2.IsOptional(t) {
		w.Len(matchVariant)
		w.Len(0)
		w.Len(1)
		w.Len(0)
		return
	}
	if n, ok := p.Value.(*syntax.Name); ok && !p.Lparen.IsKnown() && !p.Lbrace.IsKnown() {
		if n.Value == "_" {
			w.Len(matchWildcard)
			return
		}
		if n.Value != "true" && n.Value != "false" && n.Value != "nil" {
			w.Len(matchBinding)
			w.assign(n)
			return
		}
	}
	if sel, ok := p.Value.(*syntax.SelectorExpr); ok && types2.EnumOf(t) != nil {
		w.matchVariant(p, sel, t)
		return
	}
	if sel, ok := p.Value.(*syntax.SelectorExpr); ok {
		if tv, ok := w.p.maybeTypeAndValue(sel.X); ok && tv.IsType() && types2.EnumOf(tv.Type) != nil {
			w.matchInterface(p, sel, t, tv.Type)
			return
		}
	}
	w.Len(matchValue)
	w.implicitConvExpr(t, p.Value)
}

func (w *writer) matchVariant(p *syntax.MatchPattern, sel *syntax.SelectorExpr, t types2.Type) {
	v := types2.EnumOf(t).Lookup(sel.Sel.Value, w.p.curpkg)
	assert(v != nil)
	w.Len(matchVariant)
	w.Len(v.Tag())
	w.Len(v.StorageIndex())
	if v.IsRecord() {
		w.Len(len(p.Fields))
		for _, f := range p.Fields {
			index := -1
			for i := 0; i < v.NumFields(); i++ {
				if v.Field(i).Name() == f.Name.Value {
					index = i
					break
				}
			}
			assert(index >= 0)
			w.Len(index)
			w.matchPattern(f.Pattern, v.Field(index).Type())
		}
	} else {
		w.Len(len(p.Args))
		for i, a := range p.Args {
			w.Len(i)
			w.matchPattern(a, v.Field(i).Type())
		}
	}
}

// matchInterface writes a variant pattern of enum type enum whose subject has
// interface type iface. The enum type implements the interface, so a dynamic
// type test can succeed. For the predeclared error the test is the search of
// the error tree that errors.As performs, found by a runtime helper that needs
// no import of package errors.
func (w *writer) matchInterface(p *syntax.MatchPattern, sel *syntax.SelectorExpr, iface, enum types2.Type) {
	w.Len(matchInterface)
	w.pos(p)
	if w.Bool(isErrorType(iface)) {
		w.rtype(enum)
		w.rtype(types2.NewPointer(enum))
	} else {
		w.exprType(iface, sel.X)
		w.rtype(iface)
	}
	w.matchVariant(p, sel, enum)
}

// isErrorType reports whether t is identical to the predeclared error type.
func isErrorType(t types2.Type) bool {
	return types2.Identical(t, types2.Universe.Lookup("error").Type())
}

// matchPattern builds a lazy condition and the bindings to initialise after
// all of its tests have succeeded. The input is a saved value. No pattern
// head is executed as a constructor, and every nested payload test follows
// the tag test that proves that payload is active.
func (r *reader) matchPattern(pos src.XPos, input ir.Node, declarations *ir.Nodes) (ir.Node, ir.Nodes) {
	switch r.Len() {
	case matchWildcard:
		return ir.NewBool(pos, true), nil
	case matchBinding:
		bound, def := r.assign()
		if def {
			name := bound.(*ir.Name)
			// Every path that reaches a later case body has an SSA definition.
			// The zero is private to a failed pattern; only a successful pattern
			// exposes the copied payload to its guard and selected arm.
			declarations.Append(typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, name)), typecheck.Stmt(ir.NewAssignStmt(pos, name, ir.NewZero(pos, name.Type()))))
		}
		return ir.NewBool(pos, true), ir.Nodes{typecheck.Stmt(ir.NewAssignStmt(pos, bound, input))}
	case matchValue:
		return typecheck.DefaultLit(typecheck.Expr(ir.NewBinaryExpr(pos, ir.OEQ, input, r.expr())), types.Types[types.TBOOL]), nil
	case matchInterface:
		return r.matchInterface(input, declarations)
	case matchVariant:
		tag, storage := r.Len(), r.Len()
		condition := enumTagTest(pos, input, tag, ir.OEQ)
		var bindings ir.Nodes
		for n := r.Len(); n > 0; n-- {
			field := r.Len()
			child, init := r.matchPattern(pos, enumPayload(pos, input, storage, field), declarations)
			condition = typecheck.DefaultLit(typecheck.Expr(ir.NewLogicalExpr(pos, ir.OANDAND, condition, child)), types.Types[types.TBOOL])
			bindings.Append(init...)
		}
		return condition, bindings
	}
	panic("invalid match pattern encoding")
}

// matchInterface lowers a variant pattern on an interface input. It evaluates,
// once and only when this pattern is tried, a dynamic type test that saves the
// enum value, and then the ordinary variant pattern against that saved value.
// A nil interface holds no enum value and fails the test. The saved value is
// private to the pattern; payload bindings copy from it.
//
// For the predeclared error the test is the runtime's search of the error
// tree, with the semantics of errors.AsType, instead of a plain type assertion.
//
// The saved value is declared and zeroed where every path reaches it. When a
// binding reads it after the test, that is the arm's declaration list, as for
// bound variables, because a nested pattern's test runs only on some paths.
// Otherwise nothing outside the test reads it and it stays local to the test.
func (r *reader) matchInterface(input ir.Node, declarations *ir.Nodes) (ir.Node, ir.Nodes) {
	pos := r.pos()
	matched := r.temp(pos, types.Types[types.TBOOL])
	body := ir.Nodes{
		typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, matched)),
		typecheck.Stmt(ir.NewAssignStmt(pos, matched, ir.NewBool(pos, false))),
	}
	var head ir.Nodes // computes the value and its presence
	var value *ir.Name
	var present ir.Node // reports whether value holds an enum, once head ran
	if r.Bool() {
		typ, rtype := r.rtype0(pos)
		_, ptrRType := r.rtype0(pos)
		pointer := r.temp(pos, types.Types[types.TUNSAFEPTR])
		head.Append(typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, pointer)))
		call := typecheck.Call(pos, typecheck.LookupRuntime("matchErrorAs"), []ir.Node{input, rtype, ptrRType}, false)
		head.Append(typecheck.Stmt(ir.NewAssignStmt(pos, pointer, call)))
		value = r.temp(pos, typ)
		present = nilTest(pos, pointer, ir.ONE)
		// The helper reports a pointer to an immutable enum value.
		copied := typecheck.Expr(ir.NewStarExpr(pos, typecheck.Expr(ir.NewConvExpr(pos, ir.OCONVNOP, types.NewPtr(typ), pointer))))
		head.Append(typecheck.Stmt(ir.NewIfStmt(pos, nilTest(pos, pointer, ir.ONE), []ir.Node{typecheck.Stmt(ir.NewAssignStmt(pos, value, copied))}, nil)))
	} else {
		target := r.exprType()
		srcRType := r.rtype(pos)
		var assert ir.Node
		if dt, ok := target.(*ir.DynamicType); ok && dt.Op() == ir.ODYNAMICTYPE {
			x := ir.NewDynamicTypeAssertExpr(pos, ir.ODYNAMICDOTTYPE, input, dt.RType)
			x.SrcRType = srcRType
			x.ITab = dt.ITab
			assert = typed(dt.Type(), x)
		} else {
			assert = typecheck.Expr(ir.NewTypeAssertExpr(pos, input, target.Type()))
		}
		value = r.temp(pos, assert.Type())
		ok := r.temp(pos, types.Types[types.TBOOL])
		head.Append(typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, ok)))
		head.Append(typecheck.Stmt(ir.NewAssignListStmt(pos, ir.OAS2, []ir.Node{value, ok}, []ir.Node{assert})))
		present = ok
	}
	child, bindings := r.matchPattern(pos, value, declarations)
	declare := ir.Nodes{typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, value)), typecheck.Stmt(ir.NewAssignStmt(pos, value, ir.NewZero(pos, value.Type())))}
	if len(bindings) > 0 {
		declarations.Append(declare...)
	} else {
		body.Append(declare...)
	}
	body.Append(head...)
	body.Append(typecheck.Stmt(ir.NewIfStmt(pos, present, []ir.Node{typecheck.Stmt(ir.NewAssignStmt(pos, matched, child))}, nil)))
	return nilInline(pos, body, matched), bindings
}

func (r *reader) matchExpr(statement bool, label *types.Sym) ir.Node {
	pos := r.pos()
	var result *ir.Name
	var init ir.Nodes
	if !statement {
		result = r.temp(pos, r.typ())
		init.Append(typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, result)))
	}
	input := r.tempCopy(pos, r.expr(), &init)
	cases := make([]*ir.CaseClause, r.Len())
	for i := range cases {
		r.openScope()
		armPos := r.pos()
		catchall := r.Bool()
		var declarations ir.Nodes
		var condition ir.Node
		var bindings, selection ir.Nodes
		count := r.Len()
		if count == 1 {
			condition, bindings = r.matchPattern(armPos, input, &declarations)
		} else {
			matched := r.temp(armPos, types.Types[types.TBOOL])
			var attempts ir.Nodes
			for range count {
				test, init := r.matchPattern(armPos, input, &declarations)
				pending := typecheck.Expr(ir.NewUnaryExpr(armPos, ir.ONOT, matched))
				test = typecheck.Expr(ir.NewLogicalExpr(armPos, ir.OANDAND, pending, test))
				init.Append(typecheck.Stmt(ir.NewAssignStmt(armPos, matched, ir.NewBool(armPos, true))))
				attempts.Append(typecheck.Stmt(ir.NewIfStmt(armPos, test, init, nil)))
			}
			selection = append(declarations, ir.Nodes{
				typecheck.Stmt(ir.NewDecl(armPos, ir.ODCL, matched)),
				typecheck.Stmt(ir.NewAssignStmt(armPos, matched, ir.NewBool(armPos, false))),
			}...)
			selection.Append(attempts...)
			condition = matched
		}
		guard := r.optExpr()
		if guard != nil {
			guard = typecheck.DefaultLit(guard, types.Types[types.TBOOL])
		}
		if count > 1 {
			if guard != nil {
				// The guard runs once after the first successful alternative. Failure
				// skips the whole arm instead of trying another overlapping alternative.
				assign := typecheck.Stmt(ir.NewAssignStmt(armPos, condition, typecheck.Conv(guard, condition.Type())))
				selection.Append(typecheck.Stmt(ir.NewIfStmt(armPos, condition, []ir.Node{assign}, nil)))
			}
			if !catchall {
				condition = nilInline(armPos, selection, condition.(*ir.Name))
			}
		} else if !catchall && (len(bindings) > 0 || guard != nil) {
			matched := r.temp(armPos, types.Types[types.TBOOL])
			body := append(declarations, ir.Nodes{
				typecheck.Stmt(ir.NewDecl(armPos, ir.ODCL, matched)),
				typecheck.Stmt(ir.NewAssignStmt(armPos, matched, ir.NewBool(armPos, false))),
			}...)
			set := []ir.Node{typecheck.Stmt(ir.NewAssignStmt(armPos, matched, ir.NewBool(armPos, true)))}
			if guard != nil {
				bindings.Append(typecheck.Stmt(ir.NewIfStmt(armPos, guard, set, nil)))
			} else {
				bindings.Append(set...)
			}
			body.Append(typecheck.Stmt(ir.NewIfStmt(armPos, condition, bindings, nil)))
			condition = nilInline(armPos, body, matched)
		}
		var body ir.Nodes
		// Exhaustiveness proves the last unguarded arm covers every value
		// that can reach it. Encode it as the switch default, so ordinary
		// IR control flow retains that fact and its bindings dominate the
		// selected body (including returns and escaping closures).
		if catchall {
			if count > 1 {
				body.Append(selection...)
			} else {
				body.Append(declarations...)
				body.Append(bindings...)
			}
		}
		if statement {
			body.Append(r.blockStmt()...)
		} else {
			body.Append(typecheck.Stmt(ir.NewAssignStmt(armPos, result, r.expr())))
		}
		var tests []ir.Node
		if !catchall {
			tests = []ir.Node{condition}
		}
		cases[i] = ir.NewCaseStmt(armPos, tests, body)
		r.closeScope()
	}
	match := ir.NewSwitchStmt(pos, nil, cases)
	match.Label = label
	if statement {
		match.SetInit(init)
		return match
	}
	init.Append(typecheck.Stmt(match))
	return nilInline(pos, init, result)
}

func (r *reader) patternTestExpr() ir.Node {
	pos := r.pos()
	typ := r.typ()
	var init, declarations ir.Nodes
	input := r.tempCopy(pos, r.expr(), &init)
	condition, bindings := r.matchPattern(pos, input, &declarations)
	matched := r.temp(pos, types.Types[types.TBOOL])
	if r.patternTestDeclarations != nil {
		r.patternTestDeclarations.Append(declarations...)
	} else {
		init.Append(declarations...)
	}
	init.Append(typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, matched)), typecheck.Stmt(ir.NewAssignStmt(pos, matched, condition)))
	if len(bindings) > 0 {
		init.Append(typecheck.Stmt(ir.NewIfStmt(pos, matched, bindings, nil)))
	}
	// The checker treats a pattern test like a comparison: its untyped
	// boolean result can acquire a named boolean type from its context.
	result := typecheck.Expr(ir.NewBinaryExpr(pos, ir.OEQ, nilInline(pos, init, matched), ir.NewBool(pos, true)))
	if typ.IsUntyped() {
		return result
	}
	return typecheck.DefaultLit(result, typ)
}
