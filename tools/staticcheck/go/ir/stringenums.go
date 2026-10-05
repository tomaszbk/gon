package ir

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"maps"
)

func stringEnumMethod(sig *types.Signature, name string) bool {
	if sig.Recv() == nil {
		return false
	}
	typ := sig.Recv().Type()
	if pointer, ok := types.Unalias(typ).(*types.Pointer); ok {
		typ = pointer.Elem()
	}
	desc := types.EnumOf(typ)
	return desc != nil && desc.IsString() && (name == "String" || name == "MarshalText" || name == "UnmarshalText")
}

// A local named type is recreated for each enclosing generic instantiation.
// Go local types have no methods, but a string enum owns implicit methods that
// must be recreated with that type. Receiver parameters still belong to each
// method, including when the local type has its own explicit type parameters.
func (subst *subster) stringEnumMethods(original, fresh *types.Named) {
	if enum := types.EnumOf(original); enum == nil || !enum.IsString() {
		return
	}
	for i := range original.NumMethods() {
		method := original.Method(i)
		sig := method.Signature()
		methodSubst := *subst
		methodSubst.cache = maps.Clone(subst.cache)
		methodSubst.replacements = maps.Clone(subst.replacements)
		parameters := make([]*types.TypeParam, sig.RecvTypeParams().Len())
		for i := range parameters {
			original := sig.RecvTypeParams().At(i)
			obj := original.Obj()
			parameter := types.NewTypeParam(types.NewTypeName(obj.Pos(), obj.Pkg(), obj.Name(), nil), nil)
			parameters[i] = parameter
			methodSubst.cache[original] = parameter
			methodSubst.replacements[original] = parameter
		}
		for i, parameter := range parameters {
			parameter.SetConstraint(methodSubst.typ(sig.RecvTypeParams().At(i).Constraint()))
		}
		receiver := methodSubst.var_(sig.Recv())
		params, results := methodSubst.tuple(sig.Params()), methodSubst.tuple(sig.Results())
		cloned := types.NewSignatureType(receiver, parameters, nil, params, results, sig.Variadic())
		fresh.AddMethod(types.NewFunc(method.Pos(), method.Pkg(), method.Name(), cloned))
	}
}

// Parse has no source declaration or receiver: synthesize its normal control
// flow and typed enum storage, as for an enum payload constructor.
func (b *builder) stringEnumParser(fn *Function, e ast.Expr) Value {
	sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Parse" || !fn.info.Types[sel.X].IsType() {
		return nil
	}
	typ := fn.typeOf(sel.X)
	desc := types.EnumOf(typ)
	if desc == nil || !desc.IsString() {
		return nil
	}
	sig := fn.typeOf(e).(*types.Signature)
	parser := fn.Prog.NewFunction(types.TypeString(typ, nil)+".Parse", sig, "string enum parser")
	parser.Pkg = fn.Pkg
	parser.startBody()
	text := parser.addParamVar(sig.Params().At(0), e)
	value := stringEnumParse(parser, typ, text, e)
	parser.emit(&Return{Results: []Value{value}}, e)
	parser.finishBody()
	return parser
}

func stringEnumParse(fn *Function, typ types.Type, text Value, source ast.Node) Value {
	desc := types.EnumOf(typ)
	result := emitNew(fn, typ, source, "parsed string enum")
	done := fn.newBasicBlock("stringenum.parsed")
	for i := range desc.NumVariants() {
		variant := desc.Variant(i)
		label, ok := variant.StringValue()
		if !ok {
			continue
		}
		matched := fn.newBasicBlock("stringenum.known")
		next := fn.newBasicBlock("stringenum.next")
		spelling := NewConst(constant.MakeString(label), types.Typ[types.String], source)
		emitIf(fn, emitCompare(fn, token.EQL, text, spelling, source), matched, next, source)
		fn.currentBlock = matched
		emitStore(fn, result, enumValue(fn, typ, variant, nil, source), source)
		emitJump(fn, done, source)
		fn.currentBlock = next
	}
	emitStore(fn, result, enumValue(fn, typ, desc.Default(), []Value{text}, source), source)
	emitJump(fn, done, source)
	fn.currentBlock = done
	return emitLoad(fn, result, source)
}

func stringEnumFormat(fn *Function, value Value, source ast.Node) Value {
	desc := types.EnumOf(value.Type())
	result := emitNew(fn, types.Typ[types.String], source, "string enum text")
	tag := enumField(fn, value, 0, source)
	done := fn.newBasicBlock("stringenum.text")
	for i := range desc.NumVariants() {
		variant := desc.Variant(i)
		label, ok := variant.StringValue()
		if !ok {
			continue
		}
		matched := fn.newBasicBlock("stringenum.known")
		next := fn.newBasicBlock("stringenum.next")
		emitIf(fn, emitCompare(fn, token.EQL, tag, emitConv(fn, intConst(int64(variant.Tag()), source), tag.Type(), source), source), matched, next, source)
		fn.currentBlock = matched
		emitStore(fn, result, NewConst(constant.MakeString(label), types.Typ[types.String], source), source)
		emitJump(fn, done, source)
		fn.currentBlock = next
	}
	text := enumField(fn, enumField(fn, value, desc.Default().StorageIndex(), source), 0, source)
	emitStore(fn, result, text, source)
	emitJump(fn, done, source)
	fn.currentBlock = done
	return emitLoad(fn, result, source)
}

func (b *builder) buildStringEnumMethod(fn *Function) {
	fn.startBody()
	receiver := fn.addParamVar(fn.Signature.Recv(), nil)
	for i := range fn.Signature.Params().Len() {
		fn.addParamVar(fn.Signature.Params().At(i), nil)
	}
	var results []Value
	switch fn.Name() {
	case "String", "MarshalText":
		text := stringEnumFormat(fn, receiver, nil)
		if fn.Name() == "MarshalText" {
			results = []Value{emitConv(fn, text, fn.Signature.Results().At(0).Type(), nil), zeroConst(fn.Signature.Results().At(1).Type(), nil)}
		} else {
			results = []Value{text}
		}
	case "UnmarshalText":
		typ := receiver.Type().Underlying().(*types.Pointer).Elem()
		text := emitConv(fn, fn.Params[1], types.Typ[types.String], nil)
		value := stringEnumParse(fn, typ, text, nil)
		emitStore(fn, receiver, value, nil)
		results = []Value{zeroConst(fn.Signature.Results().At(0).Type(), nil)}
	}
	fn.emit(&Return{Results: results}, nil)
	fn.finishBody()
}
