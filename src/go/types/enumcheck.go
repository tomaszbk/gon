package types

import (
	"fmt"
	"go/ast"
	. "internal/types/errors"
)

func (check *Checker) enumType(e *ast.EnumType, def *TypeName) *Struct {
	desc := &Enum{defaultIndex: -1}
	st := &Struct{enum: desc}
	st.fields = []*Var{NewField(e.Pos(), check.pkg, "$gonTag", Typ[Uint], false)}
	names := map[string]bool{}
	for i, v := range e.Variants {
		name := v.Name.Name
		if name == "_" || names[name] {
			check.errorf(v.Name, DuplicateDecl, "invalid or duplicate enum variant %s", name)
		}
		names[name] = true
		variant := &EnumVariant{name: name, pkg: check.pkg, record: v.Record, storage: i + 1}
		variant.obj = NewVar(v.Name.Pos(), check.pkg, name, Typ[Invalid])
		if def != nil {
			variant.obj.(*Var).typ = def.Type()
		}
		check.recordDef(v.Name, variant.obj)
		if v.Default.IsValid() {
			if desc.defaultIndex >= 0 {
				check.error(atPos(v.Default), DuplicateDecl, "enum requires exactly one default variant")
			}
			desc.defaultIndex = i
		}
		fields := map[string]bool{}
		var payloadFields []*ast.Field
		if v.Payload != nil {
			for _, field := range v.Payload.List {
				if len(field.Names) <= 1 {
					payloadFields = append(payloadFields, field)
				} else {
					for _, name := range field.Names {
						payloadFields = append(payloadFields, &ast.Field{Names: []*ast.Ident{name}, Type: field.Type})
					}
				}
			}
		}
		for _, f := range payloadFields {
			fieldName := ""
			if len(f.Names) > 0 {
				fieldName = f.Names[0].Name
			}
			if v.Record {
				if fieldName == "" || fieldName == "_" || fields[fieldName] {
					check.error(f, DuplicateDecl, "enum record fields require distinct names")
				}
				fields[fieldName] = true
			}
			payload := NewField(f.Pos(), check.pkg, fieldName, check.varType(f.Type), false)
			variant.fields = append(variant.fields, payload)
			if len(f.Names) > 0 {
				check.recordDef(f.Names[0], payload)
			}
		}
		backing := make([]*Var, len(variant.fields))
		for j, f := range variant.fields {
			backing[j] = NewField(f.pos, check.pkg, fmt.Sprintf("$gonPayload%d", j), f.typ, false)
		}
		st.fields = append(st.fields, NewField(v.Pos(), check.pkg, fmt.Sprintf("$gonVariant%d", i), NewStruct(backing, nil), false))
		desc.variants = append(desc.variants, variant)
	}
	if desc.defaultIndex < 0 {
		check.error(e, InvalidSyntaxTree, "enum requires exactly one default variant")
		if len(desc.variants) != 0 {
			desc.defaultIndex = 0
		}
	}
	next := 1
	for i, v := range desc.variants {
		if i == desc.defaultIndex {
			v.tag = 0
		} else {
			v.tag = next
			next++
		}
	}
	st.markComplete()
	if def != nil {
		bindEnumOwner(desc, def.Type())
		for i, v := range e.Variants {
			check.recordDef(v.Name, desc.variants[i].obj)
		}
		if named, ok := def.Type().(*Named); ok {
			named.enumCache = desc
			check.later(func() {
				for i := 0; i < named.NumMethods(); i++ {
					method := named.Method(i)
					for _, variant := range desc.variants {
						if method.Name() == variant.name {
							check.errorf(method, DuplicateDecl, "enum method %s collides with a variant", method.Name())
						}
					}
				}
			}).describef(e, "check enum method names")
		}
	}
	return st
}

func (check *Checker) enumSelector(x *operand, e *ast.SelectorExpr, wantType bool) bool {
	if x.mode() != typexpr {
		return false
	}
	// An invalid selector in a generic declaration may reach us while the
	// origin's RHS is still being checked. Do not expand that instance before
	// the ordinary selector checker reports that it is not a type.
	if n, ok := Unalias(x.typ()).(*Named); ok && n.Origin().unpack().fromRHS == nil {
		return false
	}
	enum := check.sourceEnum(x.typ())
	if enum == nil {
		return false
	}
	v := enum.Lookup(e.Sel.Name, check.pkg)
	if v == nil {
		return false
	} // ordinary method expressions remain available
	container := x.typ()
	check.recordUse(e.Sel, v.obj)
	if v.record {
		// Record heads are resolved directly by keyed literals and patterns.
		// They are neither independent types nor callable/value constructors.
		check.error(e, NotAType, "record enum variant requires a keyed literal")
		x.invalidate()
	} else if len(v.fields) == 0 {
		if wantType {
			check.error(e, NotAType, "unit enum variant is a value")
			x.invalidate()
		} else {
			x.mode_, x.typ_ = value, container
		}
	} else {
		if wantType {
			check.error(e, NotAType, "positional enum variant is a constructor")
			x.invalidate()
			return true
		}
		params := make([]*Var, len(v.fields))
		for i, f := range v.fields {
			params[i] = NewParam(f.pos, f.pkg, "", f.typ)
		}
		x.mode_, x.typ_ = value, NewSignatureType(nil, nil, nil, NewTuple(params...), NewTuple(NewParam(e.Pos(), check.pkg, "", container)), false)
	}
	return true
}

func (check *Checker) enumCompositeLit(x *operand, e *ast.CompositeLit) bool {
	sel, ok := e.Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	qualifier := ast.Unparen(sel.X)
	if indexed, ok := qualifier.(*ast.IndexExpr); ok {
		qualifier = indexed.X
	}
	if name, ok := qualifier.(*ast.Ident); ok {
		if _, isType := check.lookup(name.Name).(*TypeName); !isType {
			return false
		}
	}
	var q operand
	check.exprOrType(&q, sel.X, false)
	if q.mode() != typexpr || !check.isComplete(q.typ()) {
		return false
	}
	enum := EnumOf(q.typ())
	if enum == nil {
		return false
	}
	v := enum.Lookup(sel.Sel.Name, check.pkg)
	if v == nil {
		return false
	}
	if !v.record {
		check.error(sel, InvalidLit, "enum variant does not have record payload")
		x.invalidate()
		return true
	}
	check.recordUse(sel.Sel, v.obj)
	check.recordTypeAndValue(e.Type, typexpr, q.typ(), nil)
	seen := map[string]bool{}
	for _, elem := range e.Elts {
		kv, ok := elem.(*ast.KeyValueExpr)
		if !ok {
			check.error(elem, InvalidLit, "enum record construction requires field names")
			continue
		}
		name, ok := kv.Key.(*ast.Ident)
		if !ok {
			check.error(kv.Key, InvalidLit, "enum record field must be a name")
			continue
		}
		var field *Var
		for _, f := range v.fields {
			if f.name == name.Name && (f.pkg == check.pkg || isExported(f.name)) {
				field = f
				break
			}
		}
		if field == nil {
			check.errorf(name, MissingLitField, "unknown or inaccessible enum payload field %s", name.Name)
			continue
		}
		if seen[name.Name] {
			check.errorf(name, DuplicateLitField, "duplicate enum payload field %s", name.Name)
		}
		seen[name.Name] = true
		check.recordUse(name, field)
		var value operand
		check.expr(newTarget(field.typ, "enum payload"), &value, kv.Value)
		check.assignment(&value, field.typ, "enum record literal")
	}
	x.mode_, x.typ_ = value, q.typ()
	return true
}
