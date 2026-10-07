package noder

import (
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types2"
)

// prepareBangPropagation gives postfix ! its implicit handler. The type
// checker has already validated the enclosing function. A final result of
// exactly error receives the error after zeroing the other results; a test
// function reports the error through its first parameter's Fatal method and
// returns zero values. Fatal does not return, but the return keeps the control
// flow well-formed.
//
// The synthesized nodes carry types, object identities and the position of !,
// so shadowing cannot change their meaning and Fatal reports the line of !.
func prepareBangPropagation(pkg *types2.Package, info *types2.Info, serial *int, n *syntax.ErrorExpr, sig *types2.Signature) {
	if n.Body != nil {
		return
	}
	pos := n.Pos()
	errorType := types2.Universe.Lookup("error").Type()
	results := sig.Results()
	last := results.Len() - 1
	n.SynthesizedHandler = true
	failure := n.Context
	if failure == nil {
		def, use := propagationVariable(pkg, info, serial, pos, errorType)
		n.Err = def
		failure = use
	}
	if last >= 0 && types2.Identical(results.At(last).Type(), errorType) {
		n.Body = propagationBlock(pkg, info, serial, pos, results, failure)
	} else {
		n.Body = propagationBlock(pkg, info, serial, pos, results, nil, fatalStatement(pkg, info, pos, sig, failure))
	}
	// The expression is now in the synthesized handler. Keep a single tree
	// occurrence so nested propagation and rangefunc visit it only once.
	n.Context = nil
}

// propagationBlock returns the handler "{ var zeros...; stmts...; return
// zeros..., last }". With a nil last, every result is a zero value.
func propagationBlock(pkg *types2.Package, info *types2.Info, serial *int, pos syntax.Pos, results *types2.Tuple, last syntax.Expr, stmts ...syntax.Stmt) *syntax.BlockStmt {
	block := &syntax.BlockStmt{Rbrace: pos}
	block.SetPos(pos)
	var values []syntax.Expr
	for i := 0; i < results.Len(); i++ {
		if i == results.Len()-1 && last != nil {
			values = append(values, last)
			continue
		}
		def, use := propagationVariable(pkg, info, serial, pos, results.At(i).Type())
		decl := &syntax.VarDecl{NameList: []*syntax.Name{def}}
		decl.SetPos(pos)
		stmt := &syntax.DeclStmt{DeclList: []syntax.Decl{decl}}
		stmt.SetPos(pos)
		block.List = append(block.List, stmt)
		values = append(values, use)
	}
	block.List = append(block.List, stmts...)
	var result syntax.Expr
	switch len(values) {
	case 0:
	case 1:
		result = values[0]
	default:
		list := &syntax.ListExpr{ElemList: values}
		list.SetPos(pos)
		result = list
	}
	ret := &syntax.ReturnStmt{Results: result}
	ret.SetPos(pos)
	block.List = append(block.List, ret)
	return block
}

// fatalStatement builds "param.Fatal(err)" for the first parameter of sig,
// which the checker has verified to be *testing.T, *testing.B, *testing.F or
// testing.TB. The method selection is recorded as the checker would, so the
// ordinary call lowering handles promoted methods and interface methods.
func fatalStatement(pkg *types2.Package, info *types2.Info, pos syntax.Pos, sig *types2.Signature, err syntax.Expr) syntax.Stmt {
	param := sig.Params().At(0)
	selection, ok := types2.LookupSelection(param.Type(), true, pkg, "Fatal")
	if !ok || selection.Kind() != types2.MethodVal {
		panic("test propagation: missing Fatal method")
	}
	method := selection.Obj().(*types2.Func)
	methodSig := method.Type().(*types2.Signature)

	recv := syntax.NewName(pos, param.Name())
	info.Uses[recv] = param
	tv := syntax.TypeAndValue{Type: param.Type()}
	tv.SetIsValue()
	tv.SetAddressable()
	tv.SetAssignable()
	recv.SetTypeInfo(tv)

	fun := &syntax.SelectorExpr{X: recv, Sel: syntax.NewName(pos, "Fatal")}
	fun.SetPos(pos)
	info.Selections[fun] = &selection
	info.Uses[fun.Sel] = method
	funType := types2.NewSignatureType(nil, nil, nil, methodSig.Params(), methodSig.Results(), methodSig.Variadic())
	tv = syntax.TypeAndValue{Type: funType}
	tv.SetIsValue()
	fun.SetTypeInfo(tv)

	call := &syntax.CallExpr{Fun: fun, ArgList: []syntax.Expr{err}}
	call.SetPos(pos)
	tv = syntax.TypeAndValue{Type: types2.NewTuple()}
	tv.SetIsVoid()
	call.SetTypeInfo(tv)

	stmt := &syntax.ExprStmt{X: call}
	stmt.SetPos(pos)
	return stmt
}
