// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gonmodernize

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// ErrorAnalyzer replaces an explicit error check with Gon error propagation or
// a local handler when the discarded bindings and control flow are equivalent.
var ErrorAnalyzer = &analysis.Analyzer{
	Name: "gonerrors",
	Doc: `replace eligible error checks with Gon propagation or local handlers

The analyzer recognizes fresh call-result declarations followed by an error
check, and error-only declarations in an if initializer. It suggests postfix !
when the handler returns the same error and zero values, or when a test
function (in a _test.go file, with a first named *testing.T, *testing.B,
*testing.F or testing.TB parameter, that does not return error last) only calls
Fatal with the same error. Otherwise it suggests an or
handler to preserve wrapping or other behavior. Handlers must terminate when
the call has success values; error-only handlers may fall through.

It keeps checks that observe partial results, reuse bindings, use the error
after the check, or have unsupported control flow or comments that would be
lost. Only calls with exactly error as their final result are eligible.`,
	Run: runErrors,
}

func runErrors(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if ast.IsGenerated(file) {
			continue
		}
		content, err := pass.ReadFile(pass.Fset.File(file.Pos()).Name())
		if err != nil {
			return nil, err
		}
		// Suppress descendants of an edit: a handler can contain another
		// opportunity, but suggested fixes must not overlap.
		var edits [][2]token.Pos
		var visit func(ast.Node, *types.Signature, bool)
		visit = func(n ast.Node, sig *types.Signature, labels bool) {
			for _, edit := range edits {
				if edit[0] <= n.Pos() && n.End() <= edit[1] {
					return
				}
			}
			switch n := n.(type) {
			case *ast.FuncDecl:
				sig, _ = pass.TypesInfo.TypeOf(n.Name).(*types.Signature)
				labels = errorLabels(n.Body)
			case *ast.FuncLit:
				sig, _ = pass.TypesInfo.TypeOf(n).(*types.Signature)
				labels = errorLabels(n.Body)
			case *ast.LambdaExpr:
				sig, _ = pass.TypesInfo.TypeOf(n).(*types.Signature)
				labels = errorLabels(n.Block)
			case *ast.ErrorExpr:
				if sig != nil && !labels && errorContextFix(pass, file, content, sig, n) {
					edits = append(edits, [2]token.Pos{n.Pos(), n.End()})
				}
			case *ast.BlockStmt:
				if sig != nil && !labels {
					for i, stmt := range n.List {
						if check, ok := stmt.(*ast.IfStmt); ok && check.Init != nil {
							if assign, ok := check.Init.(*ast.AssignStmt); ok && len(assign.Lhs) == 1 {
								if errorFix(pass, file, content, sig, assign, check, check.Pos()) {
									edits = append(edits, [2]token.Pos{check.Pos(), check.End()})
								}
							}
						}
						if assign, ok := stmt.(*ast.AssignStmt); ok && i+1 < len(n.List) {
							if check, ok := n.List[i+1].(*ast.IfStmt); ok && check.Init == nil {
								if errorFix(pass, file, content, sig, assign, check, assign.Pos()) {
									edits = append(edits, [2]token.Pos{assign.Pos(), check.End()})
								}
							}
						}
					}
				}
			}
			for child := range ast.Children(n) {
				visit(child, sig, labels)
			}
		}
		visit(file, nil, false)
	}
	return nil, nil
}

func errorFix(pass *analysis.Pass, file *ast.File, content []byte, sig *types.Signature, assign *ast.AssignStmt, check *ast.IfStmt, start token.Pos) bool {
	if assign.Tok != token.DEFINE || len(assign.Rhs) != 1 || len(assign.Lhs) == 0 || check.Else != nil {
		return false
	}
	call, ok := ast.Unparen(assign.Rhs[0]).(*ast.CallExpr)
	if !ok || pass.TypesInfo.Types[call.Fun].IsType() {
		return false
	}
	// Exactly error (including aliases), not a named error interface or a
	// concrete error implementation. Gon deliberately preserves that boundary.
	result := pass.TypesInfo.TypeOf(call)
	count := 1
	if tuple, ok := result.(*types.Tuple); ok {
		count = tuple.Len()
		if count == 0 {
			return false
		}
		result = tuple.At(count - 1).Type()
	}
	if count != len(assign.Lhs) || !types.Identical(result, types.Universe.Lookup("error").Type()) {
		return false
	}
	var values []types.Object
	var names []string
	for _, expr := range assign.Lhs[:len(assign.Lhs)-1] {
		id, ok := expr.(*ast.Ident)
		if !ok || id.Name != "_" && pass.TypesInfo.Defs[id] == nil {
			// Updating an existing value before the error check can be observed
			// by the handler or a defer. A Gon handler runs before that update.
			return false
		}
		if id.Name != "_" {
			values = append(values, pass.TypesInfo.Defs[id])
		}
		names = append(names, id.Name)
	}
	errID, ok := assign.Lhs[len(assign.Lhs)-1].(*ast.Ident)
	if !ok || errID.Name == "_" || pass.TypesInfo.Defs[errID] == nil {
		return false
	}
	errObj := pass.TypesInfo.Defs[errID]
	cond, ok := ast.Unparen(check.Cond).(*ast.BinaryExpr)
	if !ok || cond.Op != token.NEQ ||
		!(errorObject(pass, cond.X, errObj) && errorObject(pass, cond.Y, types.Universe.Lookup("nil")) ||
			errorObject(pass, cond.Y, errObj) && errorObject(pass, cond.X, types.Universe.Lookup("nil"))) {
		return false
	}
	for id, obj := range pass.TypesInfo.Uses {
		if obj == errObj && !(check.Cond.Pos() <= id.Pos() && id.End() <= check.Cond.End()) &&
			!(check.Body.Pos() <= id.Pos() && id.End() <= check.Body.End()) {
			return false // removing this binding would change another use
		}
		if check.Body.Pos() <= id.Pos() && id.End() <= check.Body.End() {
			for _, value := range values {
				if obj == value {
					return false // the handler observes a partial result
				}
			}
		}
	}
	// Postfix ! returns the error, or reports it with Fatal,
	// exactly as such a handler does.
	propagation := errorPropagation(pass, sig, check.Body, errObj)
	fatal := errorFatalPropagation(pass, file, sig, check.Body, errObj)
	if count > 1 && !errorTerminates(pass, check.Body) && !fatal || errorBranches(check.Body) {
		return false
	}
	// The RHS and handler are copied verbatim. Never discard comments in the
	// declaration or condition, or comments between the two statements.
	if errorComments(file, start, assign.Rhs[0].Pos()) || errorComments(file, assign.Rhs[0].End(), check.Body.Pos()) {
		return false
	}
	tokFile := pass.Fset.File(file.Pos())
	source := func(n ast.Node) string { return string(content[tokFile.Offset(n.Pos()):tokFile.Offset(n.End())]) }
	replacement := source(assign.Rhs[0])
	message := "replace error check with a Gon or handler"
	if propagation && !errorComments(file, check.Body.Pos(), check.Body.End()) && !errorImportedUse(pass, check.Body) ||
		fatal && !errorComments(file, check.Body.Pos(), check.Body.End()) {
		replacement += "!"
		message = "replace error check with Gon ! propagation"
	} else {
		replacement += " or " + errID.Name + " " + source(check.Body)
	}
	if len(names) > 0 {
		// An all-blank short declaration was valid only because err was new.
		// Preserve the discarded success values using an assignment instead.
		op := " := "
		if len(values) == 0 {
			op = " = "
		}
		replacement = strings.Join(names, ", ") + op + replacement
	}
	pass.Report(analysis.Diagnostic{
		Pos:     start,
		End:     check.End(),
		Message: message,
		SuggestedFixes: []analysis.SuggestedFix{{Message: message, TextEdits: []analysis.TextEdit{{
			Pos: start, End: check.End(), NewText: []byte(replacement),
		}}}},
	})
	return true
}

func errorObject(pass *analysis.Pass, expr ast.Expr, obj types.Object) bool {
	id, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == obj
}

// Preserve imports whose only use might be a zero constant or type in the
// removed return. This also covers names from dot imports.
func errorImportedUse(pass *analysis.Pass, body *ast.BlockStmt) bool {
	for id, obj := range pass.TypesInfo.Uses {
		if body.Pos() <= id.Pos() && id.End() <= body.End() {
			if _, ok := obj.(*types.PkgName); ok || obj.Pkg() != nil && obj.Pkg() != pass.Pkg {
				return true
			}
		}
	}
	return false
}

func errorPropagation(pass *analysis.Pass, sig *types.Signature, body *ast.BlockStmt, errObj types.Object) bool {
	if len(body.List) != 1 || sig.Results().Len() == 0 ||
		!types.Identical(sig.Results().At(sig.Results().Len()-1).Type(), types.Universe.Lookup("error").Type()) {
		return false
	}
	ret, ok := body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != sig.Results().Len() || !errorObject(pass, ret.Results[len(ret.Results)-1], errObj) {
		return false
	}
	for i, expr := range ret.Results[:len(ret.Results)-1] {
		if !errorZero(pass, expr, sig.Results().At(i).Type()) {
			return false
		}
	}
	return true
}

// errorFatalPropagation reports whether body is exactly "t.Fatal(err)" for the
// first parameter t of a test function, which Gon's ! calls for the same error.
// A function returning error last keeps its own
// propagation, so its Fatal handler is not equivalent to !.
func errorFatalPropagation(pass *analysis.Pass, file *ast.File, sig *types.Signature, body *ast.BlockStmt, errObj types.Object) bool {
	if len(body.List) != 1 || !strings.HasSuffix(pass.Fset.PositionFor(file.Pos(), false).Filename, "_test.go") {
		return false
	}
	param := types.TestFatalParam(sig)
	if param == nil {
		return false
	}
	stmt, ok := body.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := ast.Unparen(stmt.X).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || len(call.ArgNames) != 0 || call.Ellipsis != token.NoPos || !errorObject(pass, call.Args[0], errObj) {
		return false
	}
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Fatal" || !errorObject(pass, sel.X, param) {
		return false
	}
	method, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	return ok && method.Pkg() != nil && method.Pkg().Path() == "testing"
}

func errorZero(pass *analysis.Pass, expr ast.Expr, target types.Type) bool {
	if errorObject(pass, expr, types.Universe.Lookup("nil")) {
		return true
	}
	// Boxing zero into an interface does not produce a nil interface.
	if _, ok := target.Underlying().(*types.Interface); ok {
		return false
	}
	if value := pass.TypesInfo.Types[expr].Value; value != nil {
		switch value.Kind() {
		case constant.Bool:
			return !constant.BoolVal(value)
		case constant.String:
			return constant.StringVal(value) == ""
		case constant.Int, constant.Float, constant.Complex:
			return constant.Sign(constant.Real(value)) == 0 && constant.Sign(constant.Imag(value)) == 0
		}
	}
	if lit, ok := ast.Unparen(expr).(*ast.CompositeLit); ok && len(lit.Elts) == 0 {
		switch target.Underlying().(type) {
		case *types.Array, *types.Struct:
			return true
		}
	}
	return false
}

func errorTerminates(pass *analysis.Pass, body *ast.BlockStmt) bool {
	if len(body.List) == 0 {
		return false
	}
	switch last := body.List[len(body.List)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.ExprStmt:
		call, ok := ast.Unparen(last.X).(*ast.CallExpr)
		return ok && errorObject(pass, call.Fun, types.Universe.Lookup("panic"))
	}
	return false
}

func errorComments(file *ast.File, start, end token.Pos) bool {
	for _, group := range file.Comments {
		if start <= group.Pos() && group.Pos() < end {
			return true
		}
	}
	return false
}

// Labels and gotos elsewhere in the enclosing function can depend on the
// removed declaration. Nested function bodies have their own control flow.
func errorLabels(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	return errorWalkHazards(body, false)
}

func errorBranches(body *ast.BlockStmt) bool {
	return errorWalkHazards(body, true)
}

func errorWalkHazards(n ast.Node, allBranches bool) bool {
	switch n := n.(type) {
	case *ast.FuncLit, *ast.LambdaExpr:
		return false
	case *ast.LabeledStmt:
		return true
	case *ast.BranchStmt:
		return allBranches || n.Tok == token.GOTO || n.Label != nil
	}
	for child := range ast.Children(n) {
		if errorWalkHazards(child, allBranches) {
			return true
		}
	}
	return false
}

// errorContextFix preserves a handler that returns one transformed error and
// zeros for every success result. Imports mentioned only by a discarded zero
// and comments in the handler require keeping the source block.
func errorContextFix(pass *analysis.Pass, file *ast.File, content []byte, sig *types.Signature, e *ast.ErrorExpr) bool {
	if e.Body == nil || len(e.Body.List) != 1 || sig.Results().Len() == 0 ||
		!types.Identical(sig.Results().At(sig.Results().Len()-1).Type(), types.Universe.Lookup("error").Type()) ||
		errorComments(file, e.Body.Pos(), e.Body.End()) {
		return false
	}
	ret, ok := e.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != sig.Results().Len() {
		return false
	}
	for i, zero := range ret.Results[:len(ret.Results)-1] {
		if !errorZero(pass, zero, sig.Results().At(i).Type()) {
			return false
		}
	}
	context := ret.Results[len(ret.Results)-1]
	for id, obj := range pass.TypesInfo.Uses {
		if e.Body.Pos() <= id.Pos() && id.End() <= e.Body.End() &&
			!(context.Pos() <= id.Pos() && id.End() <= context.End()) {
			if _, ok := obj.(*types.PkgName); ok || obj.Pkg() != nil && obj.Pkg() != pass.Pkg {
				return false
			}
		}
	}
	tokFile := pass.Fset.File(file.Pos())
	// An expression context consumes a complete expression, while a block
	// handler ends at its closing brace. Keep operators and selectors after
	// that brace attached to the success value when replacing the handler.
	text := "(" + string(content[tokFile.Offset(e.Pos()):tokFile.Offset(e.Body.Pos())]) +
		"=> " + string(content[tokFile.Offset(context.Pos()):tokFile.Offset(context.End())]) + ")"
	pass.Report(analysis.Diagnostic{
		Pos: e.Body.Pos(), End: e.Body.End(),
		Message:        "replace error handler with Gon error context",
		SuggestedFixes: []analysis.SuggestedFix{{Message: "Use or =>", TextEdits: []analysis.TextEdit{{Pos: e.Pos(), End: e.End(), NewText: []byte(text)}}}},
	})
	return true
}
