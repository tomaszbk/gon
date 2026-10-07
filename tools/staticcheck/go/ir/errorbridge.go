package ir

import (
	"go/ast"
)

// propagateFailure emits the return of postfix ! without a handler body.
// failure is the failed call's error.
func (b *builder) propagateFailure(fn *Function, e *ast.ErrorExpr, failure Value) {
	sig := fn.source.Signature
	results := make([]Value, sig.Results().Len())
	for i := range results {
		results[i] = zeroConst(fn.typ(sig.Results().At(i).Type()), e)
	}
	ret := &ast.ReturnStmt{Return: e.OpPos}
	results[len(results)-1] = failure
	b.returnValues(fn, ret, results)
}
