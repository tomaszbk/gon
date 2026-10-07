//go:build !compiler_bootstrap

package main

import (
	"go/ast"
	"go/token"
)

// walkGonExpr supplies structural traversal for the Gon nodes that are absent
// from the upstream AST used to bootstrap cgo.
func (f *File) walkGonExpr(x any, context astContext, visit func(*File, any, astContext)) bool {
	switch n := x.(type) {
	case *ast.EnumType:
		for _, variant := range n.Variants {
			if variant.Payload != nil {
				f.walk(variant.Payload, ctxType, visit)
			}
			if variant.Value != nil {
				f.walk(&variant.Value, ctxExpr, visit)
			}
		}
	case *ast.PatternTestExpr:
		f.walk(&n.X, ctxExpr, visit)
		f.walkMatchPattern(n.Pattern, visit)
	case *ast.MatchExpr:
		f.walk(&n.Tag, ctxExpr, visit)
		for _, arm := range n.Arms {
			for _, pattern := range arm.Patterns {
				f.walkMatchPattern(pattern, visit)
			}
			if arm.Guard != nil {
				f.walk(&arm.Guard, ctxExpr, visit)
			}
			if arm.Value != nil {
				f.walk(&arm.Value, ctxExpr, visit)
			}
			if arm.Body != nil {
				f.walk(arm.Body, ctxStmt, visit)
			}
		}
	case *ast.MatchStmt:
		f.walk(n.Match, ctxStmt, visit)
	case *ast.InterpolatedStringExpr:
		for _,part:=range n.Parts {if part.Expr!=nil {f.walk(&part.Expr,ctxExpr,visit)}}
	case *ast.OptionalExpr:
		f.walk(&n.X, context, visit)
	case *ast.LambdaExpr:
		// Parameters are names, not type expressions or C references.
		if n.Body != nil {
			f.walk(&n.Body, ctxExpr, visit)
		}
		if n.Block != nil {
			f.walk(n.Block, ctxStmt, visit)
		}
	case *ast.NilGuardExpr:
		f.walk(&n.X, ctxExpr, visit)
	case *ast.SafeNavExpr:
		f.walk(&n.X, ctxExpr, visit)
	default:
		return false
	}
	return true
}

// Pattern heads may refer to C constants or payload types; bindings and
// record labels are names and must not be mistaken for executable calls.
func (f *File) walkMatchPattern(pattern *ast.MatchPattern, visit func(*File, any, astContext)) {
	if pattern.Inner != nil {
		f.walkMatchPattern(pattern.Inner, visit)
	}
	if _, binding := pattern.Value.(*ast.Ident); !binding && pattern.Value != nil {
		f.walk(&pattern.Value, ctxExpr, visit)
	}
	for _, arg := range pattern.Args {
		f.walkMatchPattern(arg, visit)
	}
	for _, field := range pattern.Fields {
		f.walkMatchPattern(field.Pattern, visit)
	}
}

func needsGonTargetType(x ast.Expr) bool {
	switch x := ast.Unparen(x).(type) {
	case *ast.LambdaExpr, *ast.SafeNavExpr, *ast.MatchExpr:
		return true
	case *ast.BinaryExpr:
		return x.Op == token.COALESCE
	}
	return false
}
