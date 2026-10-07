package satisfy

import (
	"go/ast"
	"go/types"
)

func optionPayload(t types.Type) types.Type {
	if o := types.OptionalOf(t); o != nil {
		return o.Elem()
	}
	return t
}

func (f *Finder) matchExpr(e *ast.MatchExpr) {
	subject := f.expr(e.Tag)
	for _, arm := range e.Arms {
		for _, pattern := range arm.Patterns {
			f.matchPattern(pattern, subject)
		}
		if arm.Guard != nil {
			f.expr(arm.Guard)
		}
		if arm.Value != nil {
			f.assign(f.info.TypeOf(e), f.expr(arm.Value))
		}
		if arm.Body != nil {
			f.stmt(arm.Body)
		}
	}
}

func (f *Finder) matchPattern(p *ast.MatchPattern, subject types.Type) {
	if p == nil {
		return
	}
	if p.Inner != nil {
		if p.Question.IsValid() {
			subject = types.OptionalOf(subject).Elem()
		}
		f.matchPattern(p.Inner, subject)
		return
	}
	if id, ok := p.Value.(*ast.Ident); ok {
		if id.Name == "nil" && types.IsOptional(subject) {
			return
		}
		if id.Name == "_" {
			return
		}
		obj, _ := f.info.Defs[id].(*types.Var)
		if obj == nil {
			obj, _ = f.info.Uses[id].(*types.Var)
		}
		if obj != nil {
			f.assign(obj.Type(), subject)
			return
		}
	}
	if sel, ok := p.Value.(*ast.SelectorExpr); ok {
		if e := types.EnumOf(f.info.TypeOf(sel.X)); e != nil {
			if types.IsInterface(subject) && !isTypeParam(subject) {
				// A variant pattern on an interface subject is a type test of the
				// subject for the variant's enum type, like a type assertion.
				f.typeAssert(subject, f.info.TypeOf(sel.X))
			}
			for i := range e.NumVariants() {
				v := e.Variant(i)
				if v.Name() != sel.Sel.Name {
					continue
				}
				for j, arg := range p.Args {
					f.matchPattern(arg, v.Field(j).Type())
				}
				for _, field := range p.Fields {
					for j := range v.NumFields() {
						if v.Field(j).Name() == field.Name.Name {
							f.matchPattern(field.Pattern, v.Field(j).Type())
						}
					}
				}
				return
			}
		}
	}
	f.compare(subject, f.expr(p.Value))
}

func isTypeParam(t types.Type) bool {
	_, ok := types.Unalias(t).(*types.TypeParam)
	return ok
}
