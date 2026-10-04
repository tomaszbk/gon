// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inspector

// This file defines func typeOf(ast.Node) nodeMask.
//
// The initial map-based implementation was too slow;
// see https://go-review.googlesource.com/c/tools/+/135655/1/go/ast/inspector/inspector.go#196

import (
	"go/ast"
)

const (
	nArrayType = iota
	nAssignStmt
	nBadDecl
	nBadExpr
	nBadStmt
	nBasicLit
	nBinaryExpr
	nBlockStmt
	nBranchStmt
	nCallExpr
	nCaseClause
	nChanType
	nCommClause
	nComment
	nCommentGroup
	nCompositeLit
	nDeclStmt
	nDeferStmt
	nEllipsis
	nEmptyStmt
	nExprStmt
	nField
	nFieldList
	nFile
	nForStmt
	nFuncDecl
	nFuncLit
	nFuncType
	nGenDecl
	nGoStmt
	nIdent
	nIfStmt
	nImportSpec
	nIncDecStmt
	nIndexExpr
	nIndexListExpr
	nInterfaceType
	nKeyValueExpr
	nLabeledStmt
	nMapType
	nPackage
	nParenExpr
	nRangeStmt
	nReturnStmt
	nSelectStmt
	nSelectorExpr
	nSendStmt
	nSliceExpr
	nStarExpr
	nStructType
	nSwitchStmt
	nTypeAssertExpr
	nTypeSpec
	nTypeSwitchStmt
	nUnaryExpr
	nValueSpec
	nErrorExpr
	nCondExpr
	nLambdaExpr
	nNilGuardExpr
	nSafeNavExpr
	nEnumType
	nEnumVariant
	nMatchExpr
	nMatchStmt
	nMatchArm
	nMatchPattern
	nMatchField
	nOptionalExpr
	nContextualVariantExpr
)

// typeOf returns a distinct single-bit value that represents the type of n.
//
// Various implementations were benchmarked with BenchmarkNewInspector:
//
//	                                                                GOGC=off
//	- type switch					4.9-5.5ms	2.1ms
//	- binary search over a sorted list of types	5.5-5.9ms	2.5ms
//	- linear scan, frequency-ordered list		5.9-6.1ms	2.7ms
//	- linear scan, unordered list			6.4ms		2.7ms
//	- hash table					6.5ms		3.1ms
//
// A perfect hash seemed like overkill.
//
// The compiler's switch statement is the clear winner
// as it produces a binary tree in code,
// with constant conditions and good branch prediction.
// (Sadly it is the most verbose in source code.)
// Binary search suffered from poor branch prediction.
func typeOf(n ast.Node) nodeMask {
	// Fast path: nearly half of all nodes are identifiers.
	if _, ok := n.(*ast.Ident); ok {
		return nodeBit(nIdent)
	}

	// These cases include all nodes encountered by ast.Inspect.
	switch n.(type) {
	case *ast.ContextualVariantExpr:
		return nodeBit(nContextualVariantExpr)
	case *ast.OptionalExpr:
		return nodeBit(nOptionalExpr)

	case *ast.EnumType:
		return nodeBit(nEnumType)

	case *ast.EnumVariant:
		return nodeBit(nEnumVariant)

	case *ast.MatchExpr:
		return nodeBit(nMatchExpr)

	case *ast.MatchStmt:
		return nodeBit(nMatchStmt)

	case *ast.MatchArm:
		return nodeBit(nMatchArm)

	case *ast.MatchPattern:
		return nodeBit(nMatchPattern)

	case *ast.MatchField:
		return nodeBit(nMatchField)

	case *ast.ArrayType:
		return nodeBit(nArrayType)
	case *ast.AssignStmt:
		return nodeBit(nAssignStmt)
	case *ast.BadDecl:
		return nodeBit(nBadDecl)
	case *ast.BadExpr:
		return nodeBit(nBadExpr)
	case *ast.BadStmt:
		return nodeBit(nBadStmt)
	case *ast.BasicLit:
		return nodeBit(nBasicLit)
	case *ast.BinaryExpr:
		return nodeBit(nBinaryExpr)
	case *ast.BlockStmt:
		return nodeBit(nBlockStmt)
	case *ast.BranchStmt:
		return nodeBit(nBranchStmt)
	case *ast.ErrorExpr:
		return nodeBit(nErrorExpr)
	case *ast.CondExpr:
		return nodeBit(nCondExpr)
	case *ast.LambdaExpr:
		return nodeBit(nLambdaExpr)
	case *ast.NilGuardExpr:
		return nodeBit(nNilGuardExpr)
	case *ast.SafeNavExpr:
		return nodeBit(nSafeNavExpr)
	case *ast.CallExpr:
		return nodeBit(nCallExpr)
	case *ast.CaseClause:
		return nodeBit(nCaseClause)
	case *ast.ChanType:
		return nodeBit(nChanType)
	case *ast.CommClause:
		return nodeBit(nCommClause)
	case *ast.Comment:
		return nodeBit(nComment)
	case *ast.CommentGroup:
		return nodeBit(nCommentGroup)
	case *ast.CompositeLit:
		return nodeBit(nCompositeLit)
	case *ast.DeclStmt:
		return nodeBit(nDeclStmt)
	case *ast.DeferStmt:
		return nodeBit(nDeferStmt)
	case *ast.Ellipsis:
		return nodeBit(nEllipsis)
	case *ast.EmptyStmt:
		return nodeBit(nEmptyStmt)
	case *ast.ExprStmt:
		return nodeBit(nExprStmt)
	case *ast.Field:
		return nodeBit(nField)
	case *ast.FieldList:
		return nodeBit(nFieldList)
	case *ast.File:
		return nodeBit(nFile)
	case *ast.ForStmt:
		return nodeBit(nForStmt)
	case *ast.FuncDecl:
		return nodeBit(nFuncDecl)
	case *ast.FuncLit:
		return nodeBit(nFuncLit)
	case *ast.FuncType:
		return nodeBit(nFuncType)
	case *ast.GenDecl:
		return nodeBit(nGenDecl)
	case *ast.GoStmt:
		return nodeBit(nGoStmt)
	case *ast.Ident:
		return nodeBit(nIdent)
	case *ast.IfStmt:
		return nodeBit(nIfStmt)
	case *ast.ImportSpec:
		return nodeBit(nImportSpec)
	case *ast.IncDecStmt:
		return nodeBit(nIncDecStmt)
	case *ast.IndexExpr:
		return nodeBit(nIndexExpr)
	case *ast.IndexListExpr:
		return nodeBit(nIndexListExpr)
	case *ast.InterfaceType:
		return nodeBit(nInterfaceType)
	case *ast.KeyValueExpr:
		return nodeBit(nKeyValueExpr)
	case *ast.LabeledStmt:
		return nodeBit(nLabeledStmt)
	case *ast.MapType:
		return nodeBit(nMapType)
	case *ast.Package:
		return nodeBit(nPackage)
	case *ast.ParenExpr:
		return nodeBit(nParenExpr)
	case *ast.RangeStmt:
		return nodeBit(nRangeStmt)
	case *ast.ReturnStmt:
		return nodeBit(nReturnStmt)
	case *ast.SelectStmt:
		return nodeBit(nSelectStmt)
	case *ast.SelectorExpr:
		return nodeBit(nSelectorExpr)
	case *ast.SendStmt:
		return nodeBit(nSendStmt)
	case *ast.SliceExpr:
		return nodeBit(nSliceExpr)
	case *ast.StarExpr:
		return nodeBit(nStarExpr)
	case *ast.StructType:
		return nodeBit(nStructType)
	case *ast.SwitchStmt:
		return nodeBit(nSwitchStmt)
	case *ast.TypeAssertExpr:
		return nodeBit(nTypeAssertExpr)
	case *ast.TypeSpec:
		return nodeBit(nTypeSpec)
	case *ast.TypeSwitchStmt:
		return nodeBit(nTypeSwitchStmt)
	case *ast.UnaryExpr:
		return nodeBit(nUnaryExpr)
	case *ast.ValueSpec:
		return nodeBit(nValueSpec)
	}
	return nodeMask{}
}

func maskOf(nodes []ast.Node) nodeMask {
	if len(nodes) == 0 {
		return nodeMask{^uint64(0), ^uint64(0)} // match all node types
	}
	var mask nodeMask
	for _, n := range nodes {
		mask = mask.union(typeOf(n))
	}
	return mask
}

// nodeMask keeps distinct filters for both Go and additive Gon nodes. Two
// words avoid aliasing node kinds once the inventory exceeds 64 types.
type nodeMask struct{ lo, hi uint64 }

func nodeBit(bit uint) nodeMask {
	if bit < 64 {
		return nodeMask{lo: 1 << bit}
	}
	if bit < 128 {
		return nodeMask{hi: 1 << (bit - 64)}
	}
	panic("inspector node inventory exceeds 128 types")
}

func (m nodeMask) union(n nodeMask) nodeMask {
	return nodeMask{m.lo | n.lo, m.hi | n.hi}
}

func (m nodeMask) intersects(n nodeMask) bool {
	return m.lo&n.lo != 0 || m.hi&n.hi != 0
}
