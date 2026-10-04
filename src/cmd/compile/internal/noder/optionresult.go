package noder

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/compile/internal/types2"
	"cmd/internal/src"
	"fmt"
)

func propagationVariable(pkg *types2.Package, info *types2.Info, serial *int, pos syntax.Pos, typ types2.Type) (*syntax.Name, *syntax.Name) {
	*serial++
	name := fmt.Sprintf("#alternative%d", *serial)
	obj := types2.NewVar(pos, pkg, name, typ)
	def, use := syntax.NewName(pos, name), syntax.NewName(pos, name)
	info.Defs[def], info.Uses[use] = obj, obj
	tv := syntax.TypeAndValue{Type: typ}
	tv.SetIsValue()
	tv.SetAddressable()
	tv.SetAssignable()
	def.SetTypeInfo(tv)
	use.SetTypeInfo(tv)
	return def, use
}

func prepareOptionPropagation(pkg *types2.Package, info *types2.Info, serial *int, n *syntax.OptionalExpr, sig *types2.Signature) {
	if n.Body != nil {
		return
	}
	pos := n.Pos()
	def, use := propagationVariable(pkg, info, serial, pos, sig.Results().At(0).Type())
	decl := &syntax.VarDecl{NameList: []*syntax.Name{def}}
	decl.SetPos(pos)
	stmt := &syntax.DeclStmt{DeclList: []syntax.Decl{decl}}
	stmt.SetPos(pos)
	ret := &syntax.ReturnStmt{Results: use}
	ret.SetPos(pos)
	block := &syntax.BlockStmt{Rbrace: pos, List: []syntax.Stmt{stmt, ret}}
	block.SetPos(pos)
	n.Body = block
}

func prepareResultPropagation(pkg *types2.Package, info *types2.Info, serial *int, n *syntax.ErrorExpr, sig *types2.Signature) {
	if n.Body != nil {
		return
	}
	pos := n.Pos()
	input := n.X.GetTypeInfo().Type
	failure := types2.EnumStorageOf(input).Lookup("Err", nil)
	def, use := propagationVariable(pkg, info, serial, pos, failure.Field(0).Type())
	n.Err = def
	destination := sig.Results().At(0).Type()
	constructed := &syntax.EnumConstructExpr{Variant: 1, ArgList: []syntax.Expr{use}}
	constructed.SetPos(pos)
	tv := syntax.TypeAndValue{Type: destination}
	tv.SetIsValue()
	constructed.SetTypeInfo(tv)
	ret := &syntax.ReturnStmt{Results: constructed}
	ret.SetPos(pos)
	block := &syntax.BlockStmt{Rbrace: pos, List: []syntax.Stmt{ret}}
	block.SetPos(pos)
	n.Body = block
	n.SynthesizedHandler = true
}

func enumPayload(pos src.XPos, value ir.Node, storage, field int) ir.Node {
	intermediate := typecheck.DotField(pos, value, storage)
	intermediate.GonEnumStorage = true
	return typecheck.DotField(pos, intermediate, field)
}

func enumTagTest(pos src.XPos, value ir.Node, tag int, op ir.Op) ir.Node {
	actual := typecheck.DotField(pos, value, 0)
	constant := typecheck.Conv(ir.NewInt(pos, int64(tag)), actual.Type())
	return typecheck.DefaultLit(typecheck.Expr(ir.NewBinaryExpr(pos, op, actual, constant)), types.Types[types.TBOOL])
}

func (w *writer) optionExpr(e *syntax.OptionalExpr) {
	variant := types2.EnumStorageOf(w.p.typeOf(e.X)).Lookup("$present", nil)
	w.Code(exprOption)
	w.pos(e)
	w.Len(variant.Tag())
	w.Len(variant.StorageIndex())
	w.expr(e.X)
	w.openScope(e.Pos())
	w.blockStmt(e.Body)
	w.closeScope(e.Body.Rbrace)
}

func (r *reader) optionExpr() ir.Node {
	pos := r.pos()
	tag, storage := r.Len(), r.Len()
	var body ir.Nodes
	value := r.tempCopy(pos, r.expr(), &body)
	r.openScope()
	handler := r.blockStmt()
	markPropagationTemporaries(handler)
	r.closeScope()
	body.Append(typecheck.Stmt(ir.NewIfStmt(pos, enumTagTest(pos, value, tag, ir.ONE), handler, nil)))
	return nilInlineValue(pos, body, enumPayload(pos, value, storage, 0))
}

func (w *writer) resultErrorExpr(e *syntax.ErrorExpr) {
	desc := types2.EnumStorageOf(w.p.typeOf(e.X))
	failure, success := desc.Lookup("Err", nil), desc.Lookup("Ok", nil)
	w.Code(exprResultError)
	w.pos(e)
	w.Bool(e.SynthesizedHandler)
	w.Len(failure.Tag())
	w.Len(failure.StorageIndex())
	w.Len(success.StorageIndex())
	w.expr(e.X)
	w.openScope(e.Pos())
	w.assign(e.Err)
	w.blockStmt(e.Body)
	w.closeScope(e.Body.Rbrace)
}

func (r *reader) resultErrorExpr() ir.Node {
	pos := r.pos()
	synthesized := r.Bool()
	tag, failure, success := r.Len(), r.Len(), r.Len()
	var body ir.Nodes
	value := r.tempCopy(pos, r.expr(), &body)
	r.openScope()
	bound, def := r.assign()
	assign := ir.NewAssignStmt(pos, bound, enumPayload(pos, value, failure, 0))
	assign.GonBinding = true
	if def {
		assign.Def = true
		name := bound.(*ir.Name)
		name.Defn = assign
		decl := ir.NewDecl(pos, ir.ODCL, name)
		decl.GonBinding = true
		assign.PtrInit().Append(decl)
	}
	handler := ir.Nodes{typecheck.Stmt(assign)}
	handler.Append(r.blockStmt()...)
	if synthesized {
		markPropagationTemporaries(handler)
	}
	r.closeScope()
	body.Append(typecheck.Stmt(ir.NewIfStmt(pos, enumTagTest(pos, value, tag, ir.OEQ), handler, nil)))
	return nilInlineValue(pos, body, enumPayload(pos, value, success, 0))
}

func (w *writer) optionConversion(dst, srcType types2.Type, pos syntax.Pos) {
	value := syntax.NewName(pos, "#optionPayload")
	tv := syntax.TypeAndValue{Type: srcType}
	tv.SetIsValue()
	value.SetTypeInfo(tv)
	w.nilConversion(dst, value)
}

func (w *writer) optionCoalesce(e *syntax.Operation) {
	source := w.p.typeOf(e.X)
	variant := types2.EnumStorageOf(source).Lookup("$present", nil)
	w.Code(exprOptionCoalesce)
	w.pos(e)
	w.typ(w.p.typeOf(e))
	w.Len(variant.Tag())
	w.Len(variant.StorageIndex())
	w.expr(e.X)
	w.optionConversion(w.p.typeOf(e), variant.Field(0).Type(), e.Pos())
	w.implicitConvExpr(w.p.typeOf(e), e.Y)
}

func (r *reader) optionCoalesce() ir.Node {
	pos := r.pos()
	typ := r.typ()
	tag, storage := r.Len(), r.Len()
	var body ir.Nodes
	value := r.tempCopy(pos, r.expr(), &body)
	result := r.temp(pos, typ)
	body.Append(typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, result)))
	previous := r.nilSafety
	r.nilSafety = &nilSafetyContext{value: enumPayload(pos, value, storage, 0)}
	converted := r.expr()
	r.nilSafety = previous
	fallback := r.expr()
	body.Append(typecheck.Stmt(ir.NewIfStmt(pos, enumTagTest(pos, value, tag, ir.OEQ), []ir.Node{typecheck.Stmt(ir.NewAssignStmt(pos, result, converted))}, []ir.Node{typecheck.Stmt(ir.NewAssignStmt(pos, result, fallback))})))
	return nilInline(pos, body, result)
}

func (w *writer) optionSafeNav(e *syntax.SafeNavExpr, statement bool) {
	w.Code(exprOptionSafeNav)
	w.pos(e)
	w.Bool(statement)
	var wrapped types2.Type
	if !statement {
		typ := w.p.typeOf(e)
		wrapped = types2.NewOptional(w.p.typeOf(e.X))
		some := types2.EnumStorageOf(wrapped).Lookup("$present", nil)
		w.typ(typ)
		w.typ(wrapped)
		w.Len(some.Tag())
		w.Len(some.StorageIndex())
		w.optionConversion(typ, wrapped, e.Pos())
	}
	w.expr(e.X)
	if !statement {
		w.optionConversion(w.p.typeOf(e), wrapped, e.Pos())
	}
}

func (r *reader) optionSafeNav() ir.Node {
	pos := r.pos()
	statement := r.Bool()
	var typ, wrapped *types.Type
	var tag, storage int
	if !statement {
		typ = r.typ()
		wrapped = r.typ()
		tag, storage = r.Len(), r.Len()
	}
	var body ir.Nodes
	previous := r.nilSafety
	ctx := &nilSafetyContext{current: &body}
	r.nilSafety = ctx
	var result *ir.Name
	if !statement {
		result = r.temp(pos, typ)
		ctx.value = typecheck.Expr(ir.NewZero(pos, wrapped))
		zeroConverted := r.expr()
		body.Append(typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, result)), typecheck.Stmt(ir.NewAssignStmt(pos, result, zeroConverted)))
		ctx.value = nil
	}
	value := r.expr()
	if statement {
		ctx.current.Append(typecheck.Stmt(value))
	} else {
		ctx.value = r.tempCopy(pos, enumValue(pos, wrapped, tag, storage, []int{0}, []ir.Node{value}), ctx.current)
		converted := r.expr()
		ctx.current.Append(typecheck.Stmt(ir.NewAssignStmt(pos, result, converted)))
	}
	r.nilSafety = previous
	return nilInline(pos, body, result)
}

func (w *writer) optionGuard(e *syntax.NilGuardExpr) {
	some := types2.EnumStorageOf(w.p.typeOf(e.X)).Lookup("$present", nil)
	w.Code(exprOptionGuard)
	w.pos(e)
	w.Len(some.Tag())
	w.Len(some.StorageIndex())
	w.expr(e.X)
}

func (r *reader) optionGuard() ir.Node {
	pos := r.pos()
	tag, storage := r.Len(), r.Len()
	ctx := r.nilSafety
	assert(ctx != nil)
	value := r.tempCopy(pos, r.expr(), ctx.current)
	guard := ir.NewIfStmt(pos, enumTagTest(pos, value, tag, ir.OEQ), nil, nil)
	guard.SetTypecheck(1)
	ctx.current.Append(guard)
	ctx.current = &guard.Body
	return r.tempCopy(pos, enumPayload(pos, value, storage, 0), ctx.current)
}

func (w *writer) optionCoalesceAssign(e *syntax.AssignStmt) {
	source := w.p.typeOf(e.Lhs)
	some := types2.EnumStorageOf(source).Lookup("$present", nil)
	w.Code(stmtOptionCoalesceAssign)
	w.pos(e)
	w.Len(some.Tag())
	w.Len(some.StorageIndex())
	w.expr(e.Lhs)
	w.implicitConvExpr(some.Field(0).Type(), e.Rhs)
}

func (r *reader) optionCoalesceAssign() ir.Node {
	pos := r.pos()
	tag, storage := r.Len(), r.Len()
	lhs, rhs := r.expr(), r.expr()
	var body ir.Nodes
	if lhs.Op() == ir.OINDEXMAP {
		index := lhs.(*ir.IndexExpr)
		index.X = r.tempCopy(pos, index.X, &body)
		index.Index = r.tempCopy(pos, index.Index, &body)
	} else {
		pointer := r.tempCopy(pos, typecheck.Expr(typecheck.NodAddrAt(pos, lhs)), &body)
		lhs = typecheck.Expr(ir.NewStarExpr(pos, pointer))
	}
	read := ir.Copy(lhs)
	replacement := enumValue(pos, lhs.Type(), tag, storage, []int{0}, []ir.Node{rhs})
	body.Append(typecheck.Stmt(ir.NewIfStmt(pos, enumTagTest(pos, read, tag, ir.ONE), []ir.Node{typecheck.Stmt(ir.NewAssignStmt(pos, lhs, replacement))}, nil)))
	result := ir.NewBlockStmt(pos, body)
	result.GonLowering = true
	return result
}
