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
	"internal/pkgbits"
)

func stringEnumMethod(obj *types2.Func) *types2.Enum {
	sig := obj.Type().(*types2.Signature)
	if sig.Recv() == nil {
		return nil
	}
	typ := sig.Recv().Type()
	if ptr, ok := typ.(*types2.Pointer); ok {
		typ = ptr.Elem()
	}
	e := types2.EnumOf(typ)
	if e == nil || !e.IsString() {
		return nil
	}
	switch obj.Name() {
	case "String", "MarshalText", "UnmarshalText":
		return e
	}
	return nil
}

func (pw *pkgWriter) stringEnumBody(obj *types2.Func, dict *writerDict) index {
	w := pw.newWriter(pkgbits.SectionBody, pkgbits.SyncFuncBody)
	w.sig, w.dict = obj.Type().(*types2.Signature), dict
	w.declareParams(w.sig)
	w.Bool(true)
	w.Sync(pkgbits.SyncStmts)
	w.Code(stmtStringEnumMethod)
	w.pos(obj)
	w.String(obj.Name())
	w.stringEnumVariants(stringEnumMethod(obj))
	w.Code(stmtEnd)
	w.Sync(pkgbits.SyncStmtsEnd)
	w.pos(obj)
	return w.Flush()
}

func (w *writer) stringEnumVariants(e *types2.Enum) {
	w.Len(e.Default().StorageIndex())
	w.Len(e.NumVariants() - 1)
	for i := 0; i < e.NumVariants(); i++ {
		v := e.Variant(i)
		if text, ok := v.StringValue(); ok {
			w.Len(v.Tag())
			w.Len(v.StorageIndex())
			w.String(text)
		}
	}
}

type stringEnumVariant struct {
	tag, storage int
	text         string
}

func (r *reader) stringEnumVariants() (fallback int, variants []stringEnumVariant) {
	fallback = r.Len()
	variants = make([]stringEnumVariant, r.Len())
	for i := range variants {
		variants[i] = stringEnumVariant{r.Len(), r.Len(), r.String()}
	}
	return
}

func (w *writer) tryStringEnumParse(expr syntax.Expr) bool {
	sel, ok := expr.(*syntax.SelectorExpr)
	if !ok || sel.Sel.Value != "Parse" {
		return false
	}
	tv, ok := w.p.maybeTypeAndValue(sel.X)
	if !ok || !tv.IsType() {
		return false
	}
	e := types2.EnumOf(tv.Type)
	if e == nil || !e.IsString() {
		return false
	}
	w.Code(exprStringEnumParse)
	w.pos(expr)
	w.typ(tv.Type)
	w.typ(w.p.typeOf(expr))
	w.stringEnumVariants(e)
	return true
}

func (r *reader) stringEnumParse() ir.Node {
	r.suppressInlPos++
	pos, typ, sig := r.pos(), r.typ(), r.typ()
	r.suppressInlPos--
	fallback, variants := r.stringEnumVariants()
	params, results := syntheticSig(sig)
	sig = types.NewSignature(nil, params, results)
	fn := r.inlClosureFunc(pos, sig, ir.OCLOSURE)
	// Like positional constructors, this function has typed, synthesized IR.
	fn.Pragma |= ir.Noinline
	fn.DeclareParams(true)
	text := sig.Param(0).Nname.(*ir.Name)
	fn.Body = stringEnumParseBody(fn, pos, typ, text, fallback, variants, nil)
	typecheck.Stmts(fn, fn.Body)
	return fn.OClosure
}

// stringEnumParseBody returns or assigns a complete enum value. Whole-value
// assignment clears inactive payloads and uses ordinary Go write barriers.
// A byte-slice input is converted separately at each use: comparisons can use
// the existing non-copying string conversion, while the fallback copies the
// bytes into its retained string payload.
func stringEnumParseBody(curfunc *ir.Func, pos src.XPos, typ *types.Type, text ir.Node, fallback int, variants []stringEnumVariant, destination ir.Node) ir.Nodes {
	asString := func() ir.Node { return typecheck.Conv(curfunc, text, types.Types[types.TSTRING]) }
	finish := func(value ir.Node) ir.Nodes {
		if destination == nil {
			return ir.Nodes{ir.NewReturnStmt(pos, []ir.Node{value})}
		}
		return ir.Nodes{ir.NewAssignStmt(pos, destination, value), ir.NewReturnStmt(pos, []ir.Node{typecheck.NodNil()})}
	}
	var body ir.Nodes
	for _, v := range variants {
		condition := ir.NewBinaryExpr(pos, ir.OEQ, asString(), ir.NewString(pos, v.text))
		value := enumValue(curfunc, pos, typ, v.tag, v.storage, nil, nil)
		body.Append(ir.NewIfStmt(pos, condition, finish(value), nil))
	}
	body.Append(finish(enumValue(curfunc, pos, typ, 0, fallback, []int{0}, []ir.Node{asString()}))...)
	return body
}

func (r *reader) stringEnumMethodBody() ir.Node {
	pos, method := r.pos(), r.String()
	fallback, variants := r.stringEnumVariants()
	sig := r.curfn.Type()
	recv := sig.Recv().Nname.(*ir.Name)
	if method == "UnmarshalText" {
		typ := recv.Type().Elem()
		input := sig.Param(sig.NumParams() - 1).Nname.(*ir.Name)
		body := stringEnumParseBody(r.curfn, pos, typ, input, fallback, variants, typecheck.Expr(r.curfn, ir.NewStarExpr(pos, recv)))
		return ir.NewBlockStmt(pos, body)
	}
	finish := func(text ir.Node) ir.Nodes {
		results := []ir.Node{text}
		if method == "MarshalText" {
			results = []ir.Node{typecheck.Conv(r.curfn, text, sig.Result(0).Type), typecheck.NodNil()}
		}
		return ir.Nodes{ir.NewReturnStmt(pos, results)}
	}
	var body ir.Nodes
	for _, v := range variants {
		body.Append(ir.NewIfStmt(pos, enumTagTest(r.curfn, pos, recv, v.tag, ir.OEQ), finish(ir.NewString(pos, v.text)), nil))
	}
	body.Append(finish(enumPayload(pos, recv, fallback, 0))...)
	return ir.NewBlockStmt(pos, body)
}
