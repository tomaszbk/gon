package noder

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/compile/internal/types2"
)

// prepareErrorPropagation makes the implicit returns of postfix ! explicit
// before rangefunc rewrites loop bodies. This is essential: propagation in an
// iterator loop must return from the source function, not its generated yield
// callback. The synthesized locals have types and object identities, so neither
// shadowing nor a user-defined identifier can affect their meaning.
func prepareErrorPropagation(pkg *types2.Package, info *types2.Info, files []*syntax.File) {
	serial := 0
	var visitFunc func(*syntax.BlockStmt, *types2.Signature)
	visitFunc = func(body *syntax.BlockStmt, sig *types2.Signature) {
		if body == nil {
			return
		}
		syntax.Inspect(body, func(n syntax.Node) bool {
			switch n := n.(type) {
			case *syntax.FuncLit:
				visitFunc(n.Body, n.GetTypeInfo().Type.(*types2.Signature))
				return false
			case *syntax.LambdaExpr:
				lit := n.Lowered
				visitFunc(lit.Body, lit.GetTypeInfo().Type.(*types2.Signature))
				return false
			case *syntax.OptionalExpr:
				if !n.GetTypeInfo().IsType() {
					prepareOptionPropagation(pkg, info, &serial, n, sig)
				}
			case *syntax.ErrorExpr:
				prepareBangPropagation(pkg, info, &serial, n, sig)
			}
			return true
		})
	}
	for _, file := range files {
		syntax.Inspect(file, func(n syntax.Node) bool {
			switch n := n.(type) {
			case *syntax.FuncDecl:
				visitFunc(n.Body, info.Defs[n.Name].Type().(*types2.Signature))
				return false
			case *syntax.FuncLit:
				visitFunc(n.Body, n.GetTypeInfo().Type.(*types2.Signature))
				return false
			case *syntax.LambdaExpr:
				lit := n.Lowered
				visitFunc(lit.Body, lit.GetTypeInfo().Type.(*types2.Signature))
				return false
			}
			return true
		})
	}
}

// errorExpr lowers to ordinary assignments and an if statement, represented as
// an inline expression so the existing ordering pass preserves lazy evaluation.
// InlinedCallExpr is only an IR container here: no function or closure is made.
func (r *reader) errorExpr() ir.Node {
	pos := r.pos()
	synthesized := r.Bool()
	call := r.expr()
	var body ir.Nodes
	var values []ir.Node
	var fields []*types.Field
	if typ := call.Type(); typ.IsFuncArgStruct() {
		fields = typ.Fields()
		as := ir.NewAssignListStmt(pos, ir.OAS2, nil, []ir.Node{call})
		as.Def = true
		for _, field := range fields {
			tmp := r.temp(pos, field.Type)
			tmp.Defn = as
			as.PtrInit().Append(ir.NewDecl(pos, ir.ODCL, tmp))
			as.Lhs.Append(tmp)
			values = append(values, tmp)
		}
		body.Append(typecheck.Stmt(r.curfn, as))
	} else {
		values = []ir.Node{r.tempCopy(pos, call, &body)}
	}
	err := values[len(values)-1]
	values = values[:len(values)-1]
	r.openScope()
	bound, def := r.assign()
	assign := ir.NewAssignStmt(pos, bound, err)
	assign.GonBinding = true
	if def {
		assign.Def = true
		name := bound.(*ir.Name)
		name.Defn = assign
		decl := ir.NewDecl(pos, ir.ODCL, name)
		decl.GonBinding = true
		assign.PtrInit().Append(decl)
	}
	handler := []ir.Node{typecheck.Stmt(r.curfn, assign)}
	handler = append(handler, r.blockStmt()...)
	if synthesized {
		markPropagationTemporaries(handler)
	}
	r.closeScope()
	cond := ir.NewBinaryExpr(pos, ir.ONE, err, ir.NewNilExpr(pos, err.Type()))
	body.Append(typecheck.Stmt(r.curfn, ir.NewIfStmt(pos, cond, handler, nil)))
	res := ir.NewInlinedCallExpr(pos, body, values)
	res.GonLowering = true
	switch len(values) {
	case 0:
	case 1:
		res.SetType(values[0].Type())
	default:
		res.SetType(types.NewSignature(nil, nil, fields[:len(values)]).ResultsTuple())
	}
	res.SetTypecheck(1)
	return res
}

// markPropagationTemporaries changes only inlining metadata. AutoTemp also
// controls DWARF scopes and must retain its original classification.
func markPropagationTemporaries(body ir.Nodes) {
	for _, stmt := range body {
		ir.Visit(stmt, func(node ir.Node) {
			if decl, ok := node.(*ir.Decl); ok {
				decl.X.GonTemporary = true
			}
		})
	}
}
