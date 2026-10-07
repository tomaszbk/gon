package noder

import (
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types2"
)

// prepareBangPropagation gives postfix ! its implicit return handler. The
// checker has validated that the enclosing function returns exactly error
// last. Failure returns the error after zeroing the other results.
// Synthesized nodes carry types, object identities and the operator position.
func prepareBangPropagation(pkg *types2.Package, info *types2.Info, serial *int, n *syntax.ErrorExpr, sig *types2.Signature) {
	if n.Body != nil {
		return
	}
	pos := n.Pos()
	errorType := types2.Universe.Lookup("error").Type()
	results := sig.Results()
	n.SynthesizedHandler = true
	failure := n.Context
	if failure == nil {
		def, use := propagationVariable(pkg, info, serial, pos, errorType)
		n.Err = def
		failure = use
	}
	n.Body = propagationBlock(pkg, info, serial, pos, results, failure)
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
