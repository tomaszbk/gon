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
		w.matchPattern(a.Pattern, w.p.typeOf(e.Tag))
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
	if a.Pattern == nil {
		return true
	}
	n, ok := a.Pattern.Value.(*syntax.Name)
	return ok && pNoPresence(a.Pattern) && n.Value == "_" && !a.Pattern.Lparen.IsKnown() && !a.Pattern.Lbrace.IsKnown()
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
		return
	}
	w.Len(matchValue)
	w.implicitConvExpr(t, p.Value)
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
		condition, bindings := r.matchPattern(armPos, input, &declarations)
		guard := r.optExpr()
		if guard != nil {
			guard = typecheck.DefaultLit(guard, types.Types[types.TBOOL])
		}
		if !catchall && (len(bindings) > 0 || guard != nil) {
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
			body.Append(declarations...)
			body.Append(bindings...)
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
