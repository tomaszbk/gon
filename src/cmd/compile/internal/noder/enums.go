package noder

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/compile/internal/types2"
	"cmd/internal/src"
	"go/constant"
	"strconv"
	"strings"
)

func (w *writer) enumType(desc *types2.Enum) {
	w.Len(desc.NumVariants())
	defaultIndex := 0
	for i := 0; i < desc.NumVariants(); i++ {
		if desc.Variant(i) == desc.Default() {
			defaultIndex = i
		}
	}
	w.Len(defaultIndex)
	for i := 0; i < desc.NumVariants(); i++ {
		v := desc.Variant(i)
		w.pos(v.Object())
		w.pkg(v.Pkg())
		w.String(v.Name())
		w.Bool(v.IsRecord())
		if desc.IsString() {
			text, ok := v.StringValue()
			w.Bool(ok)
			if ok {
				w.String(text)
			}
		}
		w.Len(v.NumFields())
		for j := 0; j < v.NumFields(); j++ {
			f := v.Field(j)
			w.pos(f)
			w.pkg(f.Pkg())
			w.String(f.Name())
			w.typ(f.Type())
		}
	}
}

func (r *reader) enumType(stringEnum bool) *types.Type {
	n, defaultIndex := r.Len(), r.Len()
	tagName := "$gonTag"
	if stringEnum {
		tagName = "$gonStringEnum"
	}
	fields := []*types.Field{types.NewField(src.NoXPos, types.BuiltinPkg.Lookup(tagName), types.Types[types.TUINT])}
	for i := 0; i < n; i++ {
		pos := r.pos()
		pkg := r.pkg()
		if i == 0 {
			fields[0].Sym = pkg.Lookup(tagName)
		}
		name := r.String()
		record := r.Bool()
		var text *string
		if stringEnum && r.Bool() {
			value := r.String()
			text = &value
		}
		payload := make([]*types.Field, r.Len())
		for j := range payload {
			p := r.pos()
			_ = r.pkg()
			fieldName := r.String()
			payload[j] = types.NewField(p, pkg.LookupNum("$gonPayload", j), r.typ())
			payload[j].Note = fieldName
		}
		field := types.NewField(pos, pkg.LookupNum("$gonVariant", i), types.NewStruct(payload))
		tag := i + 1
		if i == defaultIndex {
			tag = 0
		} else if i > defaultIndex {
			tag = i
		}
		field.Note = enumVariantNote(name, record, tag)
		if text != nil {
			field.Note += ":" + strconv.Quote(*text)
		}
		fields = append(fields, field)
	}
	typ := types.NewStruct(fields)
	typ.SetIsEnum(true)
	return typ
}

func (r *reader) canonicalEnumType() *types.Type {
	name := r.String()
	args := make([]*types.Type, r.Len())
	parts := make([]string, len(args))
	for i := range args {
		args[i] = r.typ()
		parts[i] = args[i].LinkString()
	}
	sym := types.BuiltinPkg.Lookup(name + "[" + strings.Join(parts, ",") + "]")
	if sym.Def != nil {
		return sym.Def.(*ir.Name).Type()
	}
	decl := ir.NewDeclNameAt(src.NoXPos, ir.OTYPE, sym)
	typ := types.NewNamed(decl)
	typ.SetIsFullyInstantiated(true)
	for _, arg := range args {
		if arg.HasShape() {
			typ.SetHasShape(true)
		}
	}
	typ.SetUnderlying(canonicalEnumBacking(name, args))
	decl.SetType(typ)
	decl.SetTypecheck(1)
	sym.Def = decl
	return typ
}

func canonicalEnumBacking(name string, args []*types.Type) *types.Type {
	if name != "Result" {
		panic("unknown canonical enum")
	}
	return alternativeBacking(args[0], args[1], "Ok", "Err", false)
}

func optionalBacking(elem *types.Type) *types.Type {
	return alternativeBacking(nil, elem, "$absent", "$present", true)
}

func alternativeBacking(left, right *types.Type, leftName, rightName string, optional bool) *types.Type {
	payload := func(arg *types.Type) *types.Type {
		if arg == nil {
			return types.NewStruct(nil)
		}
		return types.NewStruct([]*types.Field{types.NewField(src.NoXPos, types.BuiltinPkg.Lookup("$gonPayload0"), arg)})
	}
	tagName := "$gonTag"
	if optional {
		tagName = "$gonOptional"
	}
	fields := []*types.Field{
		types.NewField(src.NoXPos, types.BuiltinPkg.Lookup(tagName), types.Types[types.TUINT]),
		types.NewField(src.NoXPos, types.BuiltinPkg.Lookup("$gonVariant0"), payload(left)),
		types.NewField(src.NoXPos, types.BuiltinPkg.Lookup("$gonVariant1"), payload(right)),
	}
	fields[1].Note = enumVariantNote(leftName, false, 0)
	fields[2].Note = enumVariantNote(rightName, false, 1)
	typ := types.NewStruct(fields)
	typ.SetIsEnum(true)
	typ.SetIsOptional(optional)
	return typ
}

// This metadata is private to the compiler and reflect; it is not an enum ABI.
func enumVariantNote(name string, record bool, tag int) string {
	shape := "0"
	if record {
		shape = "1"
	}
	return name + ":" + shape + ":" + strconv.Itoa(tag)
}

func (w *writer) enumHead(expr syntax.Expr) (types2.Type, *types2.EnumVariant) {
	sel, ok := syntax.Unparen(expr).(*syntax.SelectorExpr)
	if !ok {
		return nil, nil
	}
	tv, ok := w.p.maybeTypeAndValue(sel.X)
	if !ok || !tv.IsType() {
		return nil, nil
	}
	desc := types2.EnumOf(tv.Type)
	if desc == nil {
		return nil, nil
	}
	return tv.Type, desc.Lookup(sel.Sel.Value, w.p.curpkg)
}

func (w *writer) tryEnumExpr(expr syntax.Expr) bool {
	switch expr := expr.(type) {
	case *syntax.ContextualVariantExpr:
		typ := w.p.typeOf(expr)
		variant := types2.EnumOf(typ).Lookup(expr.Name.Value, w.p.curpkg)
		w.enumConstruct(expr.Pos(), typ, variant, expr.ArgList, nil)
		return true
	case *syntax.EnumConstructExpr:
		typ := w.p.typeOf(expr)
		v := types2.EnumOf(typ).Variant(expr.Variant)
		w.enumConstruct(expr.Pos(), typ, v, expr.ArgList, nil)
		return true
	case *syntax.SelectorExpr:
		typ, v := w.enumHead(expr)
		if v == nil {
			return false
		}
		if v.NumFields() == 0 && !v.IsRecord() {
			w.enumConstruct(expr.Pos(), typ, v, nil, nil)
		} else {
			w.Code(exprEnumConstructor)
			w.pos(expr)
			w.typ(typ)
			w.typ(w.p.typeOf(expr))
			w.Len(v.Tag())
			w.Len(v.StorageIndex())
		}
		return true
	case *syntax.CallExpr:
		typ, v := w.enumHead(expr.Fun)
		if v == nil {
			return false
		}
		w.enumConstruct(expr.Pos(), typ, v, expr.ArgList, nil)
		return true
	case *syntax.CompositeLit:
		if expr.Type == nil {
			return false
		}
		typ, v := w.enumHead(expr.Type)
		if v == nil {
			return false
		}
		args := make([]syntax.Expr, len(expr.ElemList))
		indices := make([]int, len(args))
		for i, elem := range expr.ElemList {
			kv := elem.(*syntax.KeyValueExpr)
			args[i] = kv.Value
			name := kv.Key.(*syntax.Name).Value
			for j := 0; j < v.NumFields(); j++ {
				if v.Field(j).Name() == name {
					indices[i] = j
					break
				}
			}
		}
		w.enumConstruct(expr.Pos(), typ, v, args, indices)
		return true
	}
	return false
}

func (w *writer) enumConstruct(pos syntax.Pos, typ types2.Type, variant *types2.EnumVariant, args []syntax.Expr, indices []int) {
	w.Code(exprEnumConstruct)
	w.pos(pos)
	w.typ(typ)
	w.Len(variant.Tag())
	w.Len(variant.StorageIndex())
	w.Len(len(args))
	for i, arg := range args {
		j := i
		if indices != nil {
			j = indices[i]
		}
		w.Len(j)
		w.implicitConvExpr(variant.Field(j).Type(), arg)
	}
}

func (r *reader) enumConstruct() ir.Node {
	pos, typ := r.pos(), r.typ()
	tag, storage := r.Len(), r.Len()
	fields := make([]int, r.Len())
	values := make([]ir.Node, len(fields))
	for i := range fields {
		fields[i] = r.Len()
		values[i] = r.expr()
	}
	return enumValue(pos, typ, tag, storage, fields, values)
}

func enumValue(pos src.XPos, typ *types.Type, tag, storage int, fields []int, values []ir.Node) ir.Node {
	var elements []ir.Node
	// The default discriminant and omitted payload fields already have Go
	// zeros. Avoid materializing explicit zero-valued storage initializers.
	if tag != 0 {
		tagValue := ir.NewBasicLit(pos, types.Types[types.TUINT], constant.MakeUint64(uint64(tag)))
		elements = append(elements, ir.NewStructKeyExpr(pos, typ.Field(0), tagValue))
	}
	if len(values) != 0 {
		payload := typ.Field(storage).Type
		data := make([]ir.Node, len(values))
		for i, value := range values {
			data[i] = ir.NewStructKeyExpr(pos, payload.Field(fields[i]), value)
		}
		payloadValue := ir.NewCompLitExpr(pos, ir.OCOMPLIT, payload, data)
		payloadValue.GonEnumStorage = true
		storageValue := ir.NewStructKeyExpr(pos, typ.Field(storage), typecheck.Expr(payloadValue))
		storageValue.GonEnumStorage = true
		elements = append(elements, storageValue)
	}
	return typecheck.Expr(ir.NewCompLitExpr(pos, ir.OCOMPLIT, typ, elements))
}

func (r *reader) enumConstructor() ir.Node {
	r.suppressInlPos++
	pos, typ, sig := r.pos(), r.typ(), r.typ()
	r.suppressInlPos--
	tag, storage := r.Len(), r.Len()
	fn := r.inlClosureFunc(pos, sig, ir.OCLOSURE)
	// The constructor body is generated as typed IR, without a serialized
	// function-body reader of its own. Calls remain ordinary Go function calls.
	fn.Pragma |= ir.Noinline
	fn.DeclareParams(true)
	ir.WithFunc(fn, func() {
		fields := make([]int, sig.NumParams())
		values := make([]ir.Node, len(fields))
		for i := range values {
			fields[i] = i
			values[i] = sig.Param(i).Nname.(*ir.Name)
		}
		fn.Body = []ir.Node{ir.NewReturnStmt(pos, []ir.Node{enumValue(pos, typ, tag, storage, fields, values)})}
		typecheck.Stmts(fn.Body)
	})
	return fn.OClosure
}
