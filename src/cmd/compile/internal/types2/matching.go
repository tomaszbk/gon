// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"go/constant"
	. "internal/types/errors"
	"strings"
)

// matchCoverage is the checked structural pattern consumed by coverage. It is
// deliberately independent of executable expression nodes and lowering.
type matchCoverage struct {
	wild       bool
	key        string
	label      string
	fields     []*matchCoverage
	fieldTypes []Type
	// typ is the enum type of a variant pattern whose subject is an
	// interface. Patterns for different enum types never share a key.
	typ Type
}

type matchBinding struct {
	name *syntax.Name
	typ  Type
}

func (check *Checker) matchPattern(p *syntax.MatchPattern, t Type, top bool, bindings *[]matchBinding) *matchCoverage {
	if p == nil {
		return &matchCoverage{wild: true}
	}
	if t == nil || !isValid(t) {
		return &matchCoverage{wild: true}
	}
	if p.Inner != nil {
		if !p.Question.IsKnown() {
			return check.matchPattern(p.Inner, t, top, bindings)
		}
		o := OptionalOf(t)
		if o == nil {
			check.error(p, InvalidMatch, "presence pattern requires an optional value")
			return &matchCoverage{wild: true}
		}
		payload := check.matchPattern(p.Inner, o.Elem(), false, bindings)
		return &matchCoverage{key: "variant:1", label: "present", fields: []*matchCoverage{payload}, fieldTypes: []Type{o.Elem()}}
	}
	if n, ok := p.Value.(*syntax.Name); ok && n.Value == "nil" && IsOptional(t) {
		check.recordUse(n, Universe.Lookup("nil"))
		check.recordTypeAndValue(n, value, t, nil)
		return &matchCoverage{key: "variant:0", label: "nil"}
	}
	if n, ok := p.Value.(*syntax.Name); ok && !p.Lparen.IsKnown() && !p.Lbrace.IsKnown() {
		if n.Value == "_" {
			return &matchCoverage{wild: true}
		}
		if n.Value != "true" && n.Value != "false" && n.Value != "nil" {
			if top {
				check.error(p, InvalidMatch, "top-level pattern requires a qualified alternative or _")
				return &matchCoverage{wild: true}
			}
			*bindings = append(*bindings, matchBinding{n, t})
			check.recordTypeAndValue(n, value, t, nil)
			return &matchCoverage{wild: true}
		}
	}
	if sel, ok := p.Value.(*syntax.SelectorExpr); ok && check.sourceEnum(t) != nil {
		var q operand
		check.exprOrType(&q, sel.X, false)
		if !q.isValid() {
			return &matchCoverage{wild: true}
		}
		if q.mode() != typexpr || !Identical(q.typ(), t) {
			check.errorf(sel.X, InvalidMatch, "pattern alternative must belong to %s", t)
			return &matchCoverage{wild: true}
		}
		return check.matchVariant(p, sel, t, bindings)
	}
	if sel, ok := p.Value.(*syntax.SelectorExpr); ok && isNonTypeParamInterface(t) && !check.matchPackageQualifier(sel.X) {
		return check.matchInterfaceVariant(p, sel, t, bindings)
	}
	if p.Lparen.IsKnown() || p.Lbrace.IsKnown() {
		check.error(p, InvalidMatch, "payload pattern requires a qualified enum alternative")
		return &matchCoverage{wild: true}
	}
	var x operand
	if n, ok := p.Value.(*syntax.Name); ok && (n.Value == "true" || n.Value == "false" || n.Value == "nil") {
		// These three bare names are values in the pattern grammar, even
		// when ordinary Go expressions in the same scope shadow them. Record
		// their predeclared identities for lowering and editor consumers.
		obj := Universe.Lookup(n.Value)
		check.recordUse(n, obj)
		x.expr, x.typ_ = n, obj.Type()
		switch obj := obj.(type) {
		case *Const:
			x.mode_, x.val = constant_, obj.Val()
		case *Nil:
			x.mode_ = value
			if isTypes2 {
				x.mode_ = nilvalue
			}
		default:
			panic("unexpected contextual pattern value")
		}
		check.record(&x)
	} else {
		check.expr(nil, &x, p.Value)
	}
	if !x.isValid() {
		return &matchCoverage{wild: true}
	}
	if x.mode() != constant_ && !x.isNil() {
		check.error(p, InvalidMatch, "value pattern must be a literal or qualified constant")
		return &matchCoverage{wild: true}
	}
	isNil := x.isNil()
	check.assignment(&x, t, "match pattern")
	if !x.isValid() {
		return &matchCoverage{wild: true}
	}
	if isNil {
		return &matchCoverage{key: "nil", label: "nil"}
	}
	if !Comparable(t) {
		check.error(p, InvalidMatch, "value pattern requires a comparable payload")
		return &matchCoverage{wild: true}
	}
	label := x.val.ExactString()
	return &matchCoverage{key: fmt.Sprintf("literal:%d:%s", x.val.Kind(), label), label: label}
}

// matchVariant checks a qualified variant pattern against the enum type t. The
// qualifier has already been resolved to t.
func (check *Checker) matchVariant(p *syntax.MatchPattern, sel *syntax.SelectorExpr, t Type, bindings *[]matchBinding) *matchCoverage {
	e := check.sourceEnum(t)
	v := e.Lookup(sel.Sel.Value, check.pkg)
	if v == nil {
		check.errorf(sel.Sel, InvalidMatch, "unknown or inaccessible alternative %s of %s", sel.Sel.Value, t)
		return &matchCoverage{wild: true}
	}
	check.recordUse(sel.Sel, v.Object())
	check.recordTypeAndValue(p.Value, value, t, nil)
	c := &matchCoverage{key: fmt.Sprintf("variant:%d", v.Tag()), label: TypeString(t, nil) + "." + v.Name()}
	for i := 0; i < v.NumFields(); i++ {
		c.fields = append(c.fields, &matchCoverage{wild: true})
		c.fieldTypes = append(c.fieldTypes, v.Field(i).Type())
	}
	if v.IsRecord() {
		if !p.Lbrace.IsKnown() || p.Lparen.IsKnown() {
			check.error(p, InvalidMatch, "record alternative requires a record pattern")
			return c
		}
		seen := make(map[int]bool)
		for _, f := range p.Fields {
			index := -1
			for i := 0; i < v.NumFields(); i++ {
				field := v.Field(i)
				if field.Name() == f.Name.Value && (field.Exported() || field.Pkg() == check.pkg) {
					index = i
					break
				}
			}
			if index < 0 {
				check.errorf(f.Name, InvalidMatch, "unknown or inaccessible pattern field %s", f.Name.Value)
				continue
			}
			if seen[index] {
				check.errorf(f.Name, InvalidMatch, "duplicate pattern field %s", f.Name.Value)
				continue
			}
			seen[index] = true
			check.recordUse(f.Name, v.Field(index))
			c.fields[index] = check.matchPattern(f.Pattern, c.fieldTypes[index], false, bindings)
		}
		if len(seen) != v.NumFields() && !p.Rest.IsKnown() {
			check.error(p, InvalidMatch, "record pattern must list every field or explicitly ignore the rest with ...")
		}
	} else if v.NumFields() == 0 {
		if p.Lparen.IsKnown() || p.Lbrace.IsKnown() {
			check.error(p, InvalidMatch, "unit alternative does not have a payload pattern")
		}
	} else {
		if !p.Lparen.IsKnown() || p.Lbrace.IsKnown() || len(p.Args) != v.NumFields() {
			check.errorf(p, InvalidMatch, "positional pattern requires %d payload patterns", v.NumFields())
			return c
		}
		for i, a := range p.Args {
			c.fields[i] = check.matchPattern(a, c.fieldTypes[i], false, bindings)
		}
	}
	return c
}

// matchPackageQualifier reports whether x names an imported package, which
// makes a qualified pattern an ordinary constant value pattern.
func (check *Checker) matchPackageQualifier(x syntax.Expr) bool {
	n, ok := syntax.Unparen(x).(*syntax.Name)
	if !ok {
		return false
	}
	_, isPackage := check.lookup(n.Value).(*PkgName)
	return isPackage
}

// matchNever stands for an invalid interface variant pattern. It matches nothing
// and shares no key with another arm, so it cannot make later arms unreachable
// or satisfy exhaustiveness; the pattern error is the only diagnostic.
func matchNever(p *syntax.MatchPattern) *matchCoverage {
	return &matchCoverage{key: fmt.Sprintf("invalid:%p", p), label: "_"}
}

// matchInterfaceVariant checks a qualified enum variant pattern whose subject
// has interface type t. The variant's enum type must implement t, so its
// dynamic type test can succeed. Such a pattern never covers an interface.
func (check *Checker) matchInterfaceVariant(p *syntax.MatchPattern, sel *syntax.SelectorExpr, t Type, bindings *[]matchBinding) *matchCoverage {
	var q operand
	check.exprOrType(&q, sel.X, false)
	if !q.isValid() {
		return matchNever(p)
	}
	if q.mode() != typexpr || check.sourceEnum(q.typ()) == nil {
		check.errorf(sel.X, InvalidMatch, "pattern alternative on interface %s must be qualified by an enum type", t)
		return matchNever(p)
	}
	enum := q.typ()
	var cause string
	if !check.implements(enum, t, false, &cause) {
		check.errorf(sel.X, InvalidMatch, "pattern alternative can never match interface %s: %s", t, cause)
		return matchNever(p)
	}
	c := check.matchVariant(p, sel, enum, bindings)
	if !c.wild {
		c.typ = enum
	}
	return c
}

// matchConstructors enumerates a finite, closed domain. Other Go types retain
// open domains and require a wildcard to establish exhaustiveness.
func matchConstructors(t Type) []*matchCoverage {
	if o := OptionalOf(t); o != nil {
		return []*matchCoverage{
			{key: "variant:0", label: "nil"},
			{key: "variant:1", label: "present", fieldTypes: []Type{o.Elem()}},
		}
	}
	if e := EnumOf(t); e != nil {
		var cs []*matchCoverage
		for i := 0; i < e.NumVariants(); i++ {
			v := e.Variant(i)
			c := &matchCoverage{key: fmt.Sprintf("variant:%d", v.Tag()), label: TypeString(t, nil) + "." + v.Name()}
			for j := 0; j < v.NumFields(); j++ {
				c.fieldTypes = append(c.fieldTypes, v.Field(j).Type())
			}
			cs = append(cs, c)
		}
		return cs
	}
	if allBoolean(t) {
		return []*matchCoverage{{key: fmt.Sprintf("literal:%d:false", constant.Bool), label: "false"}, {key: fmt.Sprintf("literal:%d:true", constant.Bool), label: "true"}}
	}
	return nil
}

// matchSameType distinguishes variant patterns of different enum types that
// share a tag when their subject is an interface.
func matchSameType(p, c *matchCoverage) bool {
	if p.typ == nil || c.typ == nil {
		return p.typ == nil && c.typ == nil
	}
	return Identical(p.typ, c.typ)
}

func matchSpecialize(rows [][]*matchCoverage, c *matchCoverage) [][]*matchCoverage {
	var result [][]*matchCoverage
	for _, row := range rows {
		p := row[0]
		var head []*matchCoverage
		if p.wild {
			for range c.fieldTypes {
				head = append(head, &matchCoverage{wild: true})
			}
		} else if p.key == c.key && matchSameType(p, c) {
			head = p.fields
		} else {
			continue
		}
		result = append(result, append(append([]*matchCoverage(nil), head...), row[1:]...))
	}
	return result
}

// matchUseful implements recursive constructor-matrix coverage. A non-nil
// result is a witness accepted by query but not covered by rows. Guards never
// enter rows. The budget limits exponential products conservatively: an
// inconclusive proof returns a witness and requires an irrefutable arm.
func matchUseful(rows [][]*matchCoverage, query []*matchCoverage, ts []Type, budget *int) []string {
	*budget--
	if *budget < 0 {
		w := make([]string, len(query))
		for i := range w {
			w[i] = "_"
		}
		return w
	}
	if len(query) == 0 {
		if len(rows) == 0 {
			return []string{}
		}
		return nil
	}
	q := query[0]
	if q.wild {
		if cs := matchConstructors(ts[0]); cs != nil {
			for _, c := range cs {
				qs := make([]*matchCoverage, len(c.fieldTypes))
				for i := range qs {
					qs[i] = &matchCoverage{wild: true}
				}
				qs = append(qs, query[1:]...)
				types := append(append([]Type(nil), c.fieldTypes...), ts[1:]...)
				if w := matchUseful(matchSpecialize(rows, c), qs, types, budget); w != nil {
					label := c.label
					if len(c.fieldTypes) > 0 {
						label += "(" + strings.Join(w[:len(c.fieldTypes)], ", ") + ")"
					}
					return append([]string{label}, w[len(c.fieldTypes):]...)
				}
			}
			return nil
		}
		var defaults [][]*matchCoverage
		for _, row := range rows {
			if row[0].wild {
				defaults = append(defaults, row[1:])
			}
		}
		if w := matchUseful(defaults, query[1:], ts[1:], budget); w != nil {
			return append([]string{"_"}, w...)
		}
		return nil
	}
	types := append(append([]Type(nil), q.fieldTypes...), ts[1:]...)
	qs := append(append([]*matchCoverage(nil), q.fields...), query[1:]...)
	if w := matchUseful(matchSpecialize(rows, q), qs, types, budget); w != nil {
		label := q.label
		if len(q.fields) > 0 {
			label += "(" + strings.Join(w[:len(q.fields)], ", ") + ")"
		}
		return append([]string{label}, w[len(q.fields):]...)
	}
	return nil
}

func (check *Checker) matchExpr(T *target, x *operand, e *syntax.MatchExpr, ctxt stmtContext, statement bool) {
	var tag operand
	check.expr(nil, &tag, e.Tag)
	if tag.isValid() {
		check.assignment(&tag, nil, "match operand")
	}
	var rows [][]*matchCoverage
	var values []*operand
	var bt *target
	conv := false
	if T != nil && isTyped(T.typ) {
		switch T.kind {
		case assignTarget:
			bt = newTarget(T.typ, T.desc)
		case condOnlyTarget:
			bt = T
		case convTarget:
			if it, _ := T.typ.Underlying().(*Interface); it == nil || isTypeParam(T.typ) || it.IsMethodSet() {
				bt, conv = T, true
			}
		}
	}
	for _, a := range e.Arms {
		check.openScope(a, "match arm")
		var bindings []matchBinding
		p := check.matchPattern(a.Pattern, tag.typ(), true, &bindings)
		for _, b := range bindings {
			obj := newVar(LocalVar, b.name.Pos(), check.pkg, b.name.Value, b.typ)
			check.declare(check.scope, b.name, obj, a.Pos())
		}
		if tag.isValid() {
			budget := 10000
			if matchUseful(rows, []*matchCoverage{p}, []Type{tag.typ()}, &budget) == nil {
				check.error(a, InvalidMatch, "unreachable match arm")
			}
			if a.Guard == nil {
				rows = append(rows, []*matchCoverage{p})
			}
		}
		if a.Guard != nil {
			var g operand
			check.expr(nil, &g, a.Guard)
			if g.isValid() && !allBoolean(g.typ()) {
				check.error(a.Guard, InvalidMatch, "match guard must be boolean")
			}
		}
		if statement {
			if a.Body == nil {
				check.error(a, InvalidMatch, "statement match arm requires a block")
			} else {
				check.stmt(ctxt|breakOk, a.Body)
			}
		} else {
			v := new(operand)
			check.expr(bt, v, a.Value)
			values = append(values, v)
		}
		check.closeScope()
	}
	if tag.isValid() {
		budget := 10000
		if w := matchUseful(rows, []*matchCoverage{{wild: true}}, []Type{tag.typ()}, &budget); w != nil {
			if isNonTypeParamInterface(tag.typ()) {
				check.errorf(e, InvalidMatch, "non-exhaustive match: missing %s (add a default or case _ arm to cover the remainder; enum alternatives never cover the interface type %s)", w[0], tag.typ())
			} else {
				check.errorf(e, InvalidMatch, "non-exhaustive match: missing %s (add an irrefutable arm to cover the remainder)", w[0])
			}
		}
	}
	if statement {
		return
	}
	x.expr = e
	x.mode_, x.typ_ = value, check.matchValueType(T, bt, conv, e, values)
	if !tag.isValid() || x.typ_ == nil {
		x.invalidate()
		x.typ_ = Typ[Invalid]
	}
}

func (check *Checker) matchValueType(T, bt *target, conv bool, e *syntax.MatchExpr, values []*operand) Type {
	if len(values) == 0 {
		check.error(e, InvalidMatch, "match expression requires a value arm")
		return nil
	}
	for _, v := range values {
		if !v.isValid() {
			return nil
		}
	}
	var typed, untyped Type
	typedConflict := false
	for _, v := range values {
		if isTyped(v.typ()) {
			if typed == nil {
				typed = v.typ()
			} else if !Identical(typed, v.typ()) {
				typedConflict = true
			}
		} else if !v.isNil() {
			if untyped == nil {
				untyped = v.typ()
			} else if m := maxType(untyped, v.typ()); m != nil {
				untyped = m
			} else {
				check.error(e, MismatchedTypes, "incompatible untyped branches in match expression")
				return nil
			}
		}
	}
	if bt != nil {
		if isNonTypeParamInterface(T.typ) {
			for _, v := range values {
				if isUntyped(v.typ()) && !v.isNil() {
					to := Default(untyped)
					if typed != nil && !typedConflict && !isNonTypeParamInterface(typed) {
						if !check.condUntypedFits(v, typed) {
							check.error(e, MismatchedTypes, "incompatible branches in match expression")
							return nil
						}
						to = typed
					}
					check.assignment(v, to, "match expression")
				}
			}
		}
		for _, v := range values {
			check.condBranchTo(v, T.typ, conv)
			if !v.isValid() {
				return nil
			}
		}
		return T.typ
	}
	if typedConflict {
		check.error(e, MismatchedTypes, "typed branches in match expression must have identical types")
		return nil
	}
	result := typed
	if result == nil {
		result = untyped
	}
	if result == nil {
		check.error(e, UntypedNilUse, "use of untyped nil in match expression")
		return nil
	}
	for _, v := range values {
		if isUntyped(v.typ()) {
			if !check.condUntypedFits(v, result) {
				check.error(e, MismatchedTypes, "incompatible branches in match expression")
				return nil
			}
			check.assignment(v, result, "match expression")
			if !v.isValid() {
				return nil
			}
		}
	}
	return result
}

func isMatchExpr(e syntax.Expr) bool { _, ok := syntax.Unparen(e).(*syntax.MatchExpr); return ok }

// matchNilArg preserves the distinction between untyped nil and a typed nil
// when generic inference fixes a match's type before an interface parameter.
func (check *Checker) matchNilArg(x *operand, T Type, context string) bool {
	e, _ := syntax.Unparen(x.expr).(*syntax.MatchExpr)
	if e == nil || !x.isValid() || !isNonTypeParamInterface(T) || Identical(x.typ(), T) {
		return false
	}
	var hasNil, hasValue bool
	for _, a := range e.Arms {
		if check.isNil(a.Value) {
			hasNil = true
		} else {
			hasValue = true
		}
	}
	if !hasNil || !hasValue {
		return false
	}
	check.errorf(x, IncompatibleAssign, "cannot use match expression with nil branch as %s value in %s (its type %s is fixed before inference; convert a branch explicitly)", T, context, x.typ())
	x.invalidate()
	return true
}
