package noder

import (
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types2"
)

// prepareBangPropagation gives postfix ! its implicit handler. The type
// checker has already validated the enclosing function, so the handler is
// chosen by its signature alone:
//
//   - A final result of exactly error returns the zero values and the error,
//     for an error tuple, or the Result payload converted to error, with
//     the runtime's nil-Result error replacing a nil error.
//   - Exactly one Result returns Err of the error, or of the payload.
//   - Any other function is a test function: its first parameter's Fatal
//     method reports the error or payload, and the function then returns
//     zero values. Fatal does not return; the return keeps control flow
//     well-formed.
//
// The synthesized nodes carry types and object identities and the position of
// the !, so shadowing and user-defined names cannot change their meaning, and
// a Fatal report names the line of the !.
func prepareBangPropagation(pkg *types2.Package, info *types2.Info, serial *int, n *syntax.ErrorExpr, sig *types2.Signature) {
	if n.Body != nil {
		return
	}
	pos := n.Pos()
	errorType := types2.Universe.Lookup("error").Type()
	operand := n.X.GetTypeInfo().Type
	isResult := types2.IsCanonicalResult(operand)
	results := sig.Results()
	last := results.Len() - 1
	n.SynthesizedHandler = true

	var errType types2.Type = errorType
	if isResult {
		errType = types2.EnumStorageOf(operand).Lookup("Err", nil).Field(0).Type()
	}

	switch {
	case last >= 0 && types2.Identical(results.At(last).Type(), errorType):
		def, use := propagationVariable(pkg, info, serial, pos, errType)
		n.Err = def
		if !isResult {
			n.Body = propagationBlock(pkg, info, serial, pos, results, use)
			break
		}
		// The binding holds the payload. The handler converts it to error,
		// and a nil result of that conversion, which is still a failure,
		// becomes the runtime's nil-Result error instead of a nil error.
		errDef, errUse := propagationVariable(pkg, info, serial, pos, errorType)
		convert := &syntax.VarDecl{NameList: []*syntax.Name{errDef}, Values: use}
		convert.SetPos(pos)
		convertStmt := &syntax.DeclStmt{DeclList: []syntax.Decl{convert}}
		convertStmt.SetPos(pos)
		n.Body = propagationBlock(pkg, info, serial, pos, results, errUse,
			convertStmt, nilResultSubstitute(info, pos, errDef.Value, errUse))

	case results.Len() == 1 && types2.IsCanonicalResult(results.At(0).Type()):
		if isResult {
			prepareResultPropagation(pkg, info, serial, n, sig)
			return
		}
		// An error tuple fails as Result.Err(err): the enum construction
		// converts the error to the Result's error type.
		def, use := propagationVariable(pkg, info, serial, pos, errorType)
		n.Err = def
		destination := results.At(0).Type()
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

	default:
		def, use := propagationVariable(pkg, info, serial, pos, errType)
		n.Err = def
		n.Body = propagationBlock(pkg, info, serial, pos, results, nil, fatalStatement(pkg, info, pos, sig, use))
	}
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

// nilResultSubstitute builds "if err == nil { err = runtime.nilResultErr() }":
// a failed Result is a failure even when its payload converts to a nil error.
func nilResultSubstitute(info *types2.Info, pos syntax.Pos, name string, err *syntax.Name) syntax.Stmt {
	errorType := types2.Universe.Lookup("error").Type()
	obj := info.Uses[err]

	use := func() *syntax.Name {
		n := syntax.NewName(pos, name)
		info.Uses[n] = obj
		tv := syntax.TypeAndValue{Type: errorType}
		tv.SetIsValue()
		tv.SetAddressable()
		tv.SetAssignable()
		n.SetTypeInfo(tv)
		return n
	}

	nilName := syntax.NewName(pos, "nil")
	info.Uses[nilName] = types2.Universe.Lookup("nil")
	tv := syntax.TypeAndValue{Type: errorType}
	tv.SetIsValue()
	tv.SetIsNil()
	nilName.SetTypeInfo(tv)

	cond := &syntax.Operation{Op: syntax.Eql, X: use(), Y: nilName}
	cond.SetPos(pos)
	tv = syntax.TypeAndValue{Type: types2.Typ[types2.Bool]}
	tv.SetIsValue()
	cond.SetTypeInfo(tv)

	// runtime.nilResultErr is a helper of the fake runtime package, as used by
	// the range-over-func rewriter.
	helper := types2.NewFunc(pos, runtimeHelpers, "nilResultErr", types2.NewSignatureType(nil, nil, nil, nil, types2.NewTuple(types2.NewParam(pos, runtimeHelpers, "", errorType)), false))
	fun := syntax.NewName(pos, "runtime.nilResultErr")
	info.Uses[fun] = helper
	tv = syntax.TypeAndValue{Type: helper.Type()}
	tv.SetIsValue()
	tv.SetIsRuntimeHelper()
	fun.SetTypeInfo(tv)
	call := &syntax.CallExpr{Fun: fun}
	call.SetPos(pos)
	tv = syntax.TypeAndValue{Type: errorType}
	tv.SetIsValue()
	call.SetTypeInfo(tv)

	assign := &syntax.AssignStmt{Lhs: use(), Rhs: call}
	assign.SetPos(pos)
	then := &syntax.BlockStmt{List: []syntax.Stmt{assign}, Rbrace: pos}
	then.SetPos(pos)
	stmt := &syntax.IfStmt{Cond: cond, Then: then}
	stmt.SetPos(pos)
	return stmt
}

// runtimeHelpers is a fake runtime package: only the name and package of a
// helper function reach the unified IR, which resolves it in package runtime.
var runtimeHelpers = types2.NewPackage("runtime", "runtime")
