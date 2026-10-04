package noder

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/compile/internal/types2"
	"cmd/internal/src"
)

// nilSafetyContext constructs nested guarded bodies while the ordinary
// expression reader builds the links. Nothing is evaluated by the reader:
// each operand assignment belongs to the body of the preceding guard.
type nilSafetyContext struct {
	current *ir.Nodes
	value   ir.Node
	guards  []*ir.IfStmt
}

func (w *writer) nilConversion(dst types2.Type, x syntax.Expr) {
	previous := w.nilValue
	w.nilValue = syntax.Unparen(x)
	w.implicitConvExpr(dst, x)
	w.nilValue = previous
}

func (w *writer) safeNavExpr(x *syntax.SafeNavExpr, statement bool) {
	if optionChain(x.X) {
		w.optionSafeNav(x, statement)
		return
	}
	w.Code(exprSafeNav)
	w.pos(x)
	w.Bool(statement)
	if !statement {
		w.typ(w.p.typeOf(x))
	}
	w.expr(x.X)
	if !statement {
		w.nilConversion(w.p.typeOf(x), x.X)
	}
}

// nilOperand also guards explicit dereferences on the left of ??, including
// nested dereferences and chains. Unary operators ordinarily end a chain;
// only this operand context extends absence across these dereferences.
func (w *writer) nilOperand(x syntax.Expr) {
	x = syntax.Unparen(x)
	switch x := x.(type) {
	case *syntax.SafeNavExpr:
		w.expr(x.X)
	case *syntax.Operation:
		if x.Op == syntax.Mul && x.Y == nil {
			w.Code(exprUnaryOp)
			w.op(ir.ODEREF)
			w.pos(x)
			w.Code(exprNilGuard)
			w.pos(x.X)
			w.nilOperand(x.X)
			return
		}
		w.expr(x)
	default:
		w.expr(x)
	}
}

func (w *writer) coalesceExpr(x *syntax.Operation) {
	if types2.IsOptional(w.p.typeOf(x.X)) {
		w.optionCoalesce(x)
		return
	}
	w.Code(exprCoalesce)
	w.pos(x)
	w.typ(w.p.typeOf(x))
	w.nilOperand(x.X)
	left := syntax.Unparen(x.X)
	if chain, ok := left.(*syntax.SafeNavExpr); ok {
		left = chain.X
	}
	w.nilConversion(w.p.typeOf(x), left)
	w.implicitConvExpr(w.p.typeOf(x), x.Y)
}

func nilTest(pos src.XPos, x ir.Node, op ir.Op) ir.Node {
	return typecheck.DefaultLit(typecheck.Expr(ir.NewBinaryExpr(pos, op, x, ir.NewNilExpr(pos, x.Type()))), types.Types[types.TBOOL])
}

func (r *reader) nilGuardExpr() ir.Node {
	pos := r.pos()
	x := r.expr()
	assert(r.nilSafety != nil)
	ctx := r.nilSafety
	tmp := r.tempCopy(pos, x, ctx.current)
	guard := ir.NewIfStmt(pos, nilTest(pos, tmp, ir.ONE), nil, nil)
	guard.SetTypecheck(1)
	ctx.current.Append(guard)
	ctx.guards = append(ctx.guards, guard)
	ctx.current = &guard.Body
	return tmp
}

func nilInline(pos src.XPos, body ir.Nodes, result *ir.Name) ir.Node {
	if result == nil {
		return nilInlineValue(pos, body, nil)
	}
	return nilInlineValue(pos, body, result)
}

// nilInlineValue also accepts pure field selections from a saved struct value.
// They need no extra payload temporary and cannot reevaluate the operand.
func nilInlineValue(pos src.XPos, body ir.Nodes, result ir.Node) ir.Node {
	var values []ir.Node
	if result != nil {
		values = []ir.Node{result}
	}
	x := ir.NewInlinedCallExpr(pos, body, values)
	x.GonLowering = true
	if result != nil {
		x.SetType(result.Type())
	}
	x.SetTypecheck(1)
	return x
}

func (r *reader) safeNavExpr() ir.Node {
	pos := r.pos()
	statement := r.Bool()
	var typ *types.Type
	if !statement {
		typ = r.typ()
	}
	var body ir.Nodes
	ctx := &nilSafetyContext{current: &body}
	previous := r.nilSafety
	r.nilSafety = ctx
	x := r.expr()
	var result *ir.Name
	if statement {
		ctx.current.Append(typecheck.Stmt(x))
	} else {
		result = r.temp(pos, typ)
		ctx.value = r.tempCopy(pos, x, ctx.current)
		converted := r.expr()
		ctx.current.Append(typecheck.Stmt(ir.NewAssignStmt(pos, result, converted)))
		body = append(ir.Nodes{typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, result)), typecheck.Stmt(ir.NewAssignStmt(pos, result, ir.NewZero(pos, typ)))}, body...)
	}
	r.nilSafety = previous
	return nilInline(pos, body, result)
}

func (r *reader) coalesceExpr() ir.Node {
	pos := r.pos()
	typ := r.typ()
	result := r.temp(pos, typ)
	body := ir.Nodes{typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, result)), typecheck.Stmt(ir.NewAssignStmt(pos, result, ir.NewZero(pos, typ)))}
	ctx := &nilSafetyContext{current: &body}
	previous := r.nilSafety
	r.nilSafety = ctx
	ctx.value = r.tempCopy(pos, r.expr(), ctx.current)
	if ctx.value.Type().HasNil() {
		guard := ir.NewIfStmt(pos, nilTest(pos, ctx.value, ir.ONE), nil, nil)
		guard.SetTypecheck(1)
		ctx.current.Append(guard)
		ctx.guards = append(ctx.guards, guard)
		ctx.current = &guard.Body
	}
	converted := r.expr()
	ctx.current.Append(typecheck.Stmt(ir.NewAssignStmt(pos, result, converted)))
	r.nilSafety = previous
	rhs := r.expr()
	fallback := typecheck.Stmt(ir.NewAssignStmt(pos, result, rhs))
	if len(ctx.guards) == 1 {
		// A single guard has exactly two outcomes. Use its else branch
		// directly instead of materializing a presence flag and another if.
		ctx.guards[0].Else = ir.Nodes{fallback}
	} else {
		// Nested guards share one lazy fallback. Keep the flag so the RHS
		// remains a single IR subtree and is evaluated only after absence.
		present := r.temp(pos, types.Types[types.TBOOL])
		*ctx.current = append(*ctx.current, typecheck.Stmt(ir.NewAssignStmt(pos, present, ir.NewBool(pos, true))))
		body = append(ir.Nodes{typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, present)), typecheck.Stmt(ir.NewAssignStmt(pos, present, ir.NewBool(pos, false)))}, body...)
		body.Append(typecheck.Stmt(ir.NewIfStmt(pos, typecheck.Expr(ir.NewUnaryExpr(pos, ir.ONOT, present)), []ir.Node{fallback}, nil)))
	}
	return nilInline(pos, body, result)
}

func (r *reader) coalesceAssign() ir.Node {
	pos := r.pos()
	lhs := r.expr()
	rhs := r.expr()
	var body ir.Nodes
	if lhs.Op() == ir.OINDEXMAP {
		index := lhs.(*ir.IndexExpr)
		index.X = r.tempCopy(pos, index.X, &body)
		index.Index = r.tempCopy(pos, index.Index, &body)
	} else {
		pointer := r.tempCopy(pos, typecheck.Expr(typecheck.NodAddrAt(pos, lhs)), &body)
		lhs = typecheck.Expr(ir.NewStarExpr(pos, pointer))
	}
	// Typechecking the store marks map indexes Assigned. Keep its read node
	// separate so the nil test uses mapaccess rather than mapassign and never
	// inserts a key or writes to an already-present entry.
	read := ir.Copy(lhs)
	body.Append(typecheck.Stmt(ir.NewIfStmt(pos, nilTest(pos, read, ir.OEQ), []ir.Node{typecheck.Stmt(ir.NewAssignStmt(pos, lhs, rhs))}, nil)))
	result := ir.NewBlockStmt(pos, body)
	result.GonLowering = true
	return result
}

func optionChain(e syntax.Expr) bool {
	switch e := e.(type) {
	case *syntax.NilGuardExpr:
		return types2.IsOptional(e.X.GetTypeInfo().Type) || optionChain(e.X)
	case *syntax.SelectorExpr:
		return optionChain(e.X)
	case *syntax.CallExpr:
		return optionChain(e.Fun)
	case *syntax.IndexExpr:
		return optionChain(e.X)
	case *syntax.SliceExpr:
		return optionChain(e.X)
	case *syntax.SafeNavExpr:
		return optionChain(e.X)
	}
	return false
}
