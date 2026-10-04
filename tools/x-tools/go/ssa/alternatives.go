package ssa

import (
	"go/ast"
	"go/token"
	"go/types"
)

func enumSelector(fn *Function, expr ast.Expr) (types.Type, *types.EnumVariant) {
	sel, ok := ast.Unparen(expr).(*ast.SelectorExpr)
	if !ok {
		return nil, nil
	}
	typ := fn.typeOf(sel.X)
	desc := types.EnumOf(typ)
	if desc == nil {
		return nil, nil
	}
	for i := range desc.NumVariants() {
		v := desc.Variant(i)
		if v.Name() == sel.Sel.Name {
			return typ, v
		}
	}
	return nil, nil
}

func alternativeStruct(t types.Type) *types.Struct {
	if o := types.OptionalStorage(t); o != nil {
		return o
	}
	return t.Underlying().(*types.Struct)
}

func enumField(fn *Function, v Value, index int, pos token.Pos) Value {
	f := &Field{X: v, Field: index}
	f.setType(alternativeStruct(v.Type()).Field(index).Type())
	f.setPos(pos)
	return fn.emit(f)
}
func enumFieldAddr(fn *Function, v Value, index int, pos token.Pos) Value {
	t := alternativeStruct(v.Type().Underlying().(*types.Pointer).Elem()).Field(index).Type()
	f := &FieldAddr{X: v, Field: index}
	f.setType(types.NewPointer(t))
	f.setPos(pos)
	return fn.emit(f)
}
func enumValue(fn *Function, typ types.Type, variant *types.EnumVariant, values []Value, pos token.Pos) Value {
	alloc := emitNew(fn, typ, pos, "enum value")
	tag := enumFieldAddr(fn, alloc, 0, pos)
	emitStore(fn, tag, emitConv(fn, intConst(int64(variant.Tag())), types.Typ[types.Uint]), pos)
	payload := enumFieldAddr(fn, alloc, variant.StorageIndex(), pos)
	for i, value := range values {
		if value != nil {
			addr := enumFieldAddr(fn, payload, i, pos)
			emitStore(fn, addr, emitConv(fn, value, variant.Field(i).Type()), pos)
		}
	}
	return emitLoad(fn, alloc)
}
func (b *builder) enumConstructor(fn *Function, typ types.Type, v *types.EnumVariant, e ast.Expr) Value {
	if v.NumFields() == 0 {
		return enumValue(fn, typ, v, nil, e.Pos())
	}
	sig := fn.typeOf(e).(*types.Signature)
	constructor := fn.Prog.NewFunction(types.TypeString(typ, nil)+"."+v.Name(), sig, "enum constructor")
	constructor.Pkg = fn.Pkg
	constructor.startBody()
	args := make([]Value, sig.Params().Len())
	i := 0
	for param := range sig.Params().Variables() {
		args[i] = constructor.addParamVar(param)
		i++
	}
	value := enumValue(constructor, typ, v, args, e.Pos())
	constructor.emit(&Return{Results: []Value{value}})
	constructor.finishBody()
	return constructor
}
func (b *builder) enumLiteral(fn *Function, e *ast.CompositeLit, typ types.Type, v *types.EnumVariant) Value {
	values := make([]Value, v.NumFields())
	for _, element := range e.Elts {
		kv := element.(*ast.KeyValueExpr)
		name := kv.Key.(*ast.Ident).Name
		for i := range v.NumFields() {
			if v.Field(i).Name() == name {
				values[i] = b.expr(fn, kv.Value)
				break
			}
		}
	}
	return enumValue(fn, typ, v, values, e.Pos())
}

type patternBinding struct {
	object *types.Var
	value  Value
	pos    token.Pos
}

func (b *builder) matchPattern(fn *Function, p *ast.MatchPattern, value Value, next *BasicBlock, bindings *[]patternBinding) {
	if p == nil {
		return
	}
	if p.Inner != nil {
		if !p.Question.IsValid() {
			b.matchPattern(fn, p.Inner, value, next, bindings)
			return
		}
		tag := enumField(fn, value, 0, p.Pos())
		body := fn.newBasicBlock("match.present")
		emitIf(fn, emitCompare(fn, token.EQL, tag, emitConv(fn, intConst(1), tag.Type()), p.Pos()), body, next)
		fn.currentBlock = body
		payload := enumField(fn, enumField(fn, value, 2, p.Pos()), 0, p.Pos())
		b.matchPattern(fn, p.Inner, payload, next, bindings)
		return
	}
	if id, ok := p.Value.(*ast.Ident); ok && id.Name == "nil" && types.IsOptional(value.Type()) {
		tag := enumField(fn, value, 0, p.Pos())
		body := fn.newBasicBlock("match.absent")
		emitIf(fn, emitCompare(fn, token.EQL, tag, emitConv(fn, intConst(0), tag.Type()), p.Pos()), body, next)
		fn.currentBlock = body
		return
	}
	if id, ok := p.Value.(*ast.Ident); ok {
		if id.Name == "_" {
			return
		}
		if obj, ok := fn.info.Defs[id].(*types.Var); ok {
			*bindings = append(*bindings, patternBinding{obj, value, id.Pos()})
			return
		}
	}
	if _, variant := enumSelector(fn, p.Value); variant != nil {
		tag := enumField(fn, value, 0, p.Pos())
		body := fn.newBasicBlock("match.variant")
		emitIf(fn, emitCompare(fn, token.EQL, tag, emitConv(fn, intConst(int64(variant.Tag())), tag.Type()), p.Pos()), body, next)
		fn.currentBlock = body
		payload := enumField(fn, value, variant.StorageIndex(), p.Pos())
		for i, arg := range p.Args {
			b.matchPattern(fn, arg, enumField(fn, payload, i, arg.Pos()), next, bindings)
		}
		for _, field := range p.Fields {
			for i := range variant.NumFields() {
				if variant.Field(i).Name() == field.Name.Name {
					b.matchPattern(fn, field.Pattern, enumField(fn, payload, i, field.Pos()), next, bindings)
					break
				}
			}
		}
		return
	}
	success := fn.newBasicBlock("match.literal")
	emitIf(fn, emitCompare(fn, token.EQL, value, emitConv(fn, b.expr(fn, p.Value), value.Type()), p.Pos()), success, next)
	fn.currentBlock = success
}
func (b *builder) match(fn *Function, e *ast.MatchExpr, statement bool, label *lblock) Value {
	value := b.expr(fn, e.Tag)
	done := fn.newBasicBlock("match.done")
	var result *Alloc
	if !statement {
		result = emitNew(fn, fn.typeOf(e), e.Pos(), "match result")
	}
	saved := fn.targets
	if statement {
		fn.targets = &targets{tail: saved, _break: done}
		if label != nil {
			label._break = done
		}
	}
	for _, arm := range e.Arms {
		next := fn.newBasicBlock("match.next")
		var bindings []patternBinding
		b.matchPattern(fn, arm.Pattern, value, next, &bindings)
		for _, binding := range bindings {
			addr := emitLocalVar(fn, binding.object)
			emitStore(fn, addr, binding.value, binding.pos)
		}
		if arm.Guard != nil {
			body := fn.newBasicBlock("match.guard")
			emitIf(fn, b.expr(fn, arm.Guard), body, next)
			fn.currentBlock = body
		}
		if statement {
			b.stmt(fn, arm.Body)
		} else {
			emitStore(fn, result, emitConv(fn, b.expr(fn, arm.Value), fn.typeOf(e)), arm.Value.Pos())
		}
		emitJump(fn, done)
		fn.currentBlock = next
	}
	// Exhaustive, well-typed values cannot reach this continuation. Keeping a
	// CFG edge matches ordinary switch construction without a synthetic panic.
	emitJump(fn, done)
	fn.targets = saved
	fn.currentBlock = done
	if statement {
		return nil
	}
	return emitLoad(fn, result)
}
