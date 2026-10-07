//go:build !compiler_bootstrap

package main

import "go/ast"

// The bootstrap cgo is built by an upstream toolchain, whose go/ast has no
// ErrorExpr. It only processes the Go tree, which does not use Gon's error
// handling syntax, so errorexpr_bootstrap.go provides no-op versions.

// walkErrorExpr walks x if it is an error propagation (x!) or a local
// handler (x or err { ... }), reporting whether it was.
func (f *File) walkErrorExpr(x any, visit func(*File, any, astContext)) bool {
	n, ok := x.(*ast.ErrorExpr)
	if !ok {
		return false
	}
	// The expression consumes the final error result of a call. For a C
	// call that is cgo's errno two-result form, exactly as if the call
	// were assigned to two variables.
	f.walk(&n.X, ctxAssign2, visit)
	if n.Err != nil {
		f.walk(n.Err, ctxExpr, visit)
	}
	if n.Body != nil {
		f.walk(n.Body, ctxStmt, visit)
	}
	if n.Context != nil {
		f.walk(&n.Context, ctxExpr, visit)
	}
	return true
}

// findErrorExpr returns the first error propagation or local handler
// expression in x that belongs to the function containing x,
// or nil if there is none. It does not look inside function literals,
// which are their own propagation boundary.
func findErrorExpr(x ast.Expr) ast.Node {
	var found ast.Node
	ast.Inspect(x, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit, *ast.LambdaExpr:
			return false
		case *ast.ErrorExpr, *ast.OptionalExpr:
			found = n
			return false
		}
		return true
	})
	if found == nil {
		return nil // not a typed nil in the interface
	}
	return found
}
