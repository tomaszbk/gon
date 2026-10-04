package inline

import (
	"testing"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/src"
	"cmd/internal/sys"
)

func init() {
	// Initialize the compiler's type universe for these isolated IR tests.
	types.PtrSize = 8
	types.RegSize = 8
	types.MaxWidth = 1 << 50
	base.Ctxt = &obj.Link{Arch: &obj.LinkArch{Arch: &sys.Arch{Alignment: 1, CanMergeLoads: true}}}
	typecheck.InitUniverse()
}

func gonName(name string, typ *types.Type, class ir.Class) *ir.Name {
	n := ir.NewNameAt(src.NoXPos, types.NewPkg("gon-inline-test", "goninline").Lookup(name), typ)
	n.Class = class
	return n
}

func gonVisitor(fn *ir.Func) *hairyVisitor {
	v := &hairyVisitor{curFunc: fn, budget: 1000, maxBudget: 1000, extraCallCost: inlineExtraCallCost}
	v.do = v.doNode
	return v
}

func gonCost(t *testing.T, fn *ir.Func, node ir.Node) int32 {
	t.Helper()
	v := gonVisitor(fn)
	if v.doNode(node) {
		t.Fatalf("unexpected inlining rejection: %s", v.reason)
	}
	if v.gonLoweringDepth != 0 {
		t.Fatalf("Gon lowering scope leaked: depth %d", v.gonLoweringDepth)
	}
	return v.maxBudget - v.budget
}

func gonBlock(marked bool, body ...ir.Node) *ir.BlockStmt {
	n := ir.NewBlockStmt(src.NoXPos, body)
	n.GonLowering = marked
	return n
}

func TestGonLoweringAdministrativeCost(t *testing.T) {
	for _, generated := range []bool{false, true} {
		for _, expression := range []bool{false, true} {
			name := "AutoTemp"
			if generated {
				name = "GonTemporary"
			}
			if expression {
				name += "/expression"
			} else {
				name += "/block"
			}
			t.Run(name, func(t *testing.T) {
				tmp := gonName("temporary", types.Types[types.TINT], ir.PAUTO)
				tmp.SetAutoTemp(!generated)
				tmp.GonTemporary = generated
				body := []ir.Node{ir.NewDecl(src.NoXPos, ir.ODCL, tmp), ir.NewAssignStmt(src.NoXPos, tmp, ir.NewInt(src.NoXPos, 7))}
				var marked, ordinary ir.Node
				if expression {
					x := ir.NewInlinedCallExpr(src.NoXPos, body, []ir.Node{tmp})
					x.GonLowering = true
					marked = x
					ordinary = ir.NewInlinedCallExpr(src.NoXPos, body, []ir.Node{tmp})
				} else {
					marked = gonBlock(true, append(body, tmp)...)
					ordinary = gonBlock(false, append(body, tmp)...)
				}
				if got := gonCost(t, nil, marked); got != 1 {
					t.Fatalf("generated administration must leave the literal's cost intact: got %d, want 1", got)
				}
				if got := gonCost(t, nil, ordinary); got <= 1 {
					t.Fatalf("unmarked compiler temporary received a Gon discount: cost %d", got)
				}
				if generated && tmp.AutoTemp() {
					t.Fatal("GonTemporary must not change AutoTemp and its scope/debug semantics")
				}
			})
		}
	}
	user := gonName("user", types.Types[types.TINT], ir.PAUTO)
	assignment := ir.NewAssignStmt(src.NoXPos, user, ir.NewInt(src.NoXPos, 7))
	if got, want := gonCost(t, nil, gonBlock(true, assignment)), gonCost(t, nil, assignment); got != want {
		t.Fatalf("source assignment discounted inside Gon lowering: got %d, want %d", got, want)
	}
}

func TestGonLoweringScope(t *testing.T) {
	tmp := gonName("scopeTemporary", types.Types[types.TINT], ir.PAUTO)
	tmp.SetAutoTemp(true)
	inner := ir.NewInlinedCallExpr(src.NoXPos, []ir.Node{ir.NewAssignStmt(src.NoXPos, tmp, ir.NewInt(src.NoXPos, 1))}, []ir.Node{tmp})
	inner.GonLowering = true
	v := gonVisitor(nil)
	if v.doNode(gonBlock(true, inner)) || v.gonLoweringDepth != 0 {
		t.Fatalf("nested lowering failed to restore scope: reason %q, depth %d", v.reason, v.gonLoweringDepth)
	}
	before := v.budget
	outside := ir.NewDecl(src.NoXPos, ir.ODCL, tmp)
	if v.doNode(outside) {
		t.Fatal(v.reason)
	}
	if got, want := before-v.budget, gonCost(t, nil, outside); got != want || got == 0 {
		t.Fatalf("discount escaped into following ordinary IR: got %d, want %d", got, want)
	}
}

func TestGonLoweringOperations(t *testing.T) {
	tmp := gonName("operationTemporary", types.Types[types.TINT], ir.PAUTO)
	tmp.SetAutoTemp(true)
	add := ir.NewBinaryExpr(src.NoXPos, ir.OADD, ir.NewInt(src.NoXPos, 1), ir.NewInt(src.NoXPos, 2))
	if got, want := gonCost(t, nil, gonBlock(true, ir.NewAssignStmt(src.NoXPos, tmp, add))), gonCost(t, nil, add); got != want || got <= 1 {
		t.Fatalf("arithmetic cost changed: got %d, want %d", got, want)
	}
	cond := gonName("condition", types.Types[types.TBOOL], ir.PPARAM)
	guard := ir.NewIfStmt(src.NoXPos, cond,
		[]ir.Node{ir.NewAssignStmt(src.NoXPos, tmp, ir.NewInt(src.NoXPos, 1))},
		[]ir.Node{ir.NewAssignStmt(src.NoXPos, tmp, ir.NewInt(src.NoXPos, 2))})
	if got := gonCost(t, nil, gonBlock(true, guard)); got != 4 {
		t.Fatalf("guard, condition and both branch values must still count: got %d, want 4", got)
	}
	other := gonName("otherTemporary", types.Types[types.TINT], ir.PAUTO)
	other.SetAutoTemp(true)
	rhs := []ir.Node{ir.NewInt(src.NoXPos, 1), ir.NewInt(src.NoXPos, 2)}
	allTemps := ir.NewAssignListStmt(src.NoXPos, ir.OAS2, []ir.Node{tmp, other}, rhs)
	if got := gonCost(t, nil, gonBlock(true, allTemps)); got != 2 {
		t.Fatalf("parallel generated assignment must retain RHS cost: got %d, want 2", got)
	}
	user := gonName("sourceDestination", types.Types[types.TINT], ir.PAUTO)
	mixed := ir.NewAssignListStmt(src.NoXPos, ir.OAS2, []ir.Node{tmp, user}, rhs)
	if got := gonCost(t, nil, gonBlock(true, mixed)); got != 4 {
		t.Fatalf("mixed source assignment must retain assignment and source destination cost: got %d, want 4", got)
	}
}

func TestGonLoweringCallbackCost(t *testing.T) {
	for _, mode := range []string{"immutable", "reassigned", "address-taken", "source-local"} {
		t.Run(mode, func(t *testing.T) {
			sig := types.NewSignature(nil, nil, nil)
			fn := ir.NewFunc(src.NoXPos, src.NoXPos, types.NewPkg("gon-inline-test", "goninline").Lookup("caller"), sig)
			param := gonName("callback", sig, ir.PPARAM)
			tmp := gonName("savedCallback", sig, ir.PAUTO)
			tmp.SetAutoTemp(mode != "source-local")
			tmp.Curfn = fn
			assignment := ir.NewAssignStmt(src.NoXPos, tmp, param)
			tmp.Defn = assignment
			fn.Body = []ir.Node{assignment}
			if mode == "reassigned" {
				fn.Body.Append(ir.NewAssignStmt(src.NoXPos, tmp, param))
			}
			if mode == "address-taken" {
				tmp.SetAddrtaken(true)
			}
			call := ir.NewCallExpr(src.NoXPos, ir.OCALLFUNC, tmp, nil)
			got := gonCost(t, fn, gonBlock(true, call))
			want := int32(1 + inlineExtraCallCost)
			if mode == "immutable" {
				want = 1 + inlineParamCallCost
			} else if mode == "source-local" {
				want++ // The ordinary local's reference remains source complexity.
			}
			if got != want {
				t.Fatalf("callback cost = %d, want %d", got, want)
			}
			if ordinary := gonCost(t, fn, gonBlock(false, call)); ordinary != 2+inlineExtraCallCost {
				t.Fatalf("unmarked callback temporary received Gon parameter-call discount: got %d", ordinary)
			}
		})
	}
}

func TestGonLoweringMultiResultCost(t *testing.T) {
	// Unified IR puts a multi-result call and temporary declarations in the
	// first result's init list. Its ordinary inlining adjustment must not
	// discount those same temporaries again inside a Gon lowering container.
	sig := types.NewSignature(nil, nil, nil)
	callback := gonName("multiCallback", sig, ir.PPARAM)
	call := ir.NewCallExpr(src.NoXPos, ir.OCALLFUNC, callback, nil)
	temps := make([]ir.Node, 4)
	for i := range temps {
		n := gonName("multiTemporary", types.Types[types.TINT], ir.PAUTO)
		n.SetAutoTemp(true)
		temps[i] = n
	}
	inner := ir.NewAssignListStmt(src.NoXPos, ir.OAS2, temps[:2], []ir.Node{call})
	inner.PtrInit().Append(ir.NewDecl(src.NoXPos, ir.ODCL, temps[0].(*ir.Name)), ir.NewDecl(src.NoXPos, ir.ODCL, temps[1].(*ir.Name)))
	first := ir.InitExpr([]ir.Node{inner}, temps[0])
	outer := ir.NewAssignListStmt(src.NoXPos, ir.OAS2, temps[2:], []ir.Node{first, temps[1]})
	// InitExpr's OCONVNOP carrier already has zero cost in ordinary Go.
	if got, want := gonCost(t, nil, gonBlock(true, outer)), gonCost(t, nil, call); got != want {
		t.Fatalf("multi-result lowering changed the call's cost: got %d, want %d", got, want)
	}
	direct := ir.NewAssignListStmt(src.NoXPos, ir.OAS2, temps[2:], []ir.Node{call})
	if got, want := gonCost(t, nil, outer), gonCost(t, nil, direct); got != want {
		t.Fatalf("ordinary unified-IR compensation changed: got %d, want %d", got, want)
	}
}

func TestGonLoweringHardRejections(t *testing.T) {
	for _, marked := range []bool{false, true} {
		for _, op := range []ir.Op{ir.OGO, ir.ODEFER} {
			callback := gonName("deferredCallback", types.NewSignature(nil, nil, nil), ir.PPARAM)
			call := ir.NewCallExpr(src.NoXPos, ir.OCALLFUNC, callback, nil)
			v := gonVisitor(nil)
			if !v.doNode(gonBlock(marked, ir.NewGoDeferStmt(src.NoXPos, op, call))) || v.reason != "unhandled op "+op.String() {
				t.Fatalf("%v (GonLowering=%v) lost its hard rejection: %q", op, marked, v.reason)
			}
			if v.gonLoweringDepth != 0 {
				t.Fatalf("rejection leaked Gon lowering scope: depth %d", v.gonLoweringDepth)
			}
		}
	}
}

func TestGonEnumStorageLiterals(t *testing.T) {
	pkg := types.NewPkg("gon-inline-test", "goninline")
	field := types.NewField(src.NoXPos, pkg.Lookup("value"), types.Types[types.TINT])
	payload := types.NewStruct([]*types.Field{field})
	tag := types.NewField(src.NoXPos, pkg.Lookup("tag"), types.Types[types.TUINT])
	storage := types.NewField(src.NoXPos, pkg.Lookup("storage"), payload)
	backing := types.NewStruct([]*types.Field{tag, storage})
	flatField := types.NewField(src.NoXPos, pkg.Lookup("flatValue"), types.Types[types.TINT])
	flat := types.NewStruct([]*types.Field{tag, flatField})
	for _, useCall := range []bool{false, true} {
		name := "arithmetic"
		var value ir.Node = ir.NewBinaryExpr(src.NoXPos, ir.OADD, ir.NewInt(src.NoXPos, 1), ir.NewInt(src.NoXPos, 2))
		if useCall {
			name = "call"
			callback := gonName("storageCallback", types.NewSignature(nil, nil, nil), ir.PPARAM)
			value = ir.NewCallExpr(src.NoXPos, ir.OCALLFUNC, callback, nil)
		}
		t.Run(name, func(t *testing.T) {
			inner := ir.NewCompLitExpr(src.NoXPos, ir.OSTRUCTLIT, payload,
				[]ir.Node{ir.NewStructKeyExpr(src.NoXPos, field, value)})
			inner.GonEnumStorage = true
			key := ir.NewStructKeyExpr(src.NoXPos, storage, inner)
			key.GonEnumStorage = true
			literal := ir.NewCompLitExpr(src.NoXPos, ir.OSTRUCTLIT, backing, []ir.Node{
				ir.NewStructKeyExpr(src.NoXPos, tag, ir.NewInt(src.NoXPos, 1)), key,
			})
			source := ir.NewCompLitExpr(src.NoXPos, ir.OSTRUCTLIT, flat, []ir.Node{
				ir.NewStructKeyExpr(src.NoXPos, tag, ir.NewInt(src.NoXPos, 1)),
				ir.NewStructKeyExpr(src.NoXPos, flatField, value),
			})
			want := gonCost(t, nil, source)
			if got := gonCost(t, nil, literal); got != want {
				t.Fatalf("storage wrappers changed tag, source field or operand costs: got %d, want %d", got, want)
			}
			if want != gonCost(t, nil, value)+4 {
				t.Fatalf("constructor, tag field/value and payload field must still count: cost %d", want)
			}
			// Generated wrappers may carry an init list with real effects.
			inner.PtrInit().Append(ir.NewAssignStmt(src.NoXPos,
				gonName("storageInitDestination", types.Types[types.TINT], ir.PAUTO), value))
			initCost := gonCost(t, nil, inner.Init()[0])
			if got := gonCost(t, nil, literal); got != want+initCost {
				t.Fatalf("storage wrapper skipped init effects: got %d, want %d", got, want+initCost)
			}
			inner.GonEnumStorage, key.GonEnumStorage = false, false
			if got := gonCost(t, nil, gonBlock(true, literal)); got != want+initCost+2 {
				t.Fatalf("ordinary nested source literal/key received enum-storage discount: got %d, want %d", got, want+initCost+2)
			}
		})
	}
}

func TestGonEnumStorageSelectors(t *testing.T) {
	pkg := types.NewPkg("gon-inline-test", "goninline")
	field := types.NewField(src.NoXPos, pkg.Lookup("selectedValue"), types.Types[types.TINT])
	payload := types.NewStruct([]*types.Field{field})
	storage := types.NewField(src.NoXPos, pkg.Lookup("selectedStorage"), payload)
	backing := types.NewStruct([]*types.Field{storage})
	for _, useCall := range []bool{false, true} {
		name := "parameter"
		var value ir.Node = gonName("selectedEnum", backing, ir.PPARAM)
		if useCall {
			name = "call"
			callback := gonName("selectedCallback", types.NewSignature(nil, nil, nil), ir.PPARAM)
			value = ir.NewCallExpr(src.NoXPos, ir.OCALLFUNC, callback, nil)
		}
		t.Run(name, func(t *testing.T) {
			inner := ir.NewSelectorExpr(src.NoXPos, ir.ODOT, value, storage.Sym)
			inner.GonEnumStorage = true
			selected := ir.NewSelectorExpr(src.NoXPos, ir.ODOT, inner, field.Sym)
			want := gonCost(t, nil, value) + 1 // Source payload selection remains.
			if got := gonCost(t, nil, selected); got != want {
				t.Fatalf("storage selection changed source payload/operand cost: got %d, want %d", got, want)
			}
			inner.GonEnumStorage = false
			if got := gonCost(t, nil, gonBlock(true, selected)); got != want+1 {
				t.Fatalf("ordinary nested source selection received a storage discount: got %d, want %d", got, want+1)
			}
		})
	}
}

func TestGonHandlerBindingCost(t *testing.T) {
	for _, useCall := range []bool{false, true} {
		name := "arithmetic"
		var value ir.Node = ir.NewBinaryExpr(src.NoXPos, ir.OADD, ir.NewInt(src.NoXPos, 1), ir.NewInt(src.NoXPos, 2))
		if useCall {
			name = "call"
			callback := gonName("bindingCallback", types.NewSignature(nil, nil,
				[]*types.Field{types.NewField(src.NoXPos, nil, types.Types[types.TINT])}), ir.PPARAM)
			value = ir.NewCallExpr(src.NoXPos, ir.OCALLFUNC, callback, nil)
		}
		t.Run(name, func(t *testing.T) {
			bound := gonName("failureBinding", types.Types[types.TINT], ir.PAUTO)
			decl := ir.NewDecl(src.NoXPos, ir.ODCL, bound)
			decl.GonBinding = true
			capture := ir.NewAssignStmt(src.NoXPos, bound, value)
			capture.GonBinding = true
			init := ir.NewAssignStmt(src.NoXPos,
				gonName("bindingInitDestination", types.Types[types.TINT], ir.PAUTO), ir.NewInt(src.NoXPos, 3))
			capture.PtrInit().Append(decl, init)
			want := gonCost(t, nil, value) + gonCost(t, nil, init)
			v := gonVisitor(nil)
			if v.doNode(gonBlock(true, capture)) || v.maxBudget-v.budget != want {
				t.Fatalf("implicit binding changed captured RHS/init cost: got %d, want %d, rejection %q", v.maxBudget-v.budget, want, v.reason)
			}
			if !v.usedLocals.Has(bound) {
				t.Fatal("discounted binding was lost from used locals")
			}
			if bound.AutoTemp() || bound.GonTemporary {
				t.Fatal("binding metadata changed name/debug classification or discounted source uses")
			}
			update := ir.NewAssignStmt(src.NoXPos, bound, value)
			if got := gonCost(t, nil, gonBlock(true, capture, bound, update)); got != want+1+gonCost(t, nil, update) {
				t.Fatalf("binding source use or later update discounted: got %d", got)
			}
			if got := gonCost(t, nil, gonBlock(false, capture)); got != want+4 {
				t.Fatalf("binding discount escaped Gon lowering scope: got %d, want %d", got, want+4)
			}
			decl.GonBinding, capture.GonBinding = false, false
			if got := gonCost(t, nil, gonBlock(true, capture)); got != want+4 {
				t.Fatalf("ordinary source declaration/assignment discounted in Gon lowering: got %d, want %d", got, want+4)
			}
		})
	}
}

func TestGonHandlerBindingInitRejections(t *testing.T) {
	for _, op := range []ir.Op{ir.OGO, ir.ODEFER} {
		bound := gonName("rejectedBinding", types.Types[types.TINT], ir.PAUTO)
		capture := ir.NewAssignStmt(src.NoXPos, bound, ir.NewInt(src.NoXPos, 1))
		capture.GonBinding = true
		callback := gonName("bindingInitCallback", types.NewSignature(nil, nil, nil), ir.PPARAM)
		call := ir.NewCallExpr(src.NoXPos, ir.OCALLFUNC, callback, nil)
		capture.PtrInit().Append(ir.NewGoDeferStmt(src.NoXPos, op, call))
		v := gonVisitor(nil)
		if !v.doNode(gonBlock(true, capture)) || v.reason != "unhandled op "+op.String() {
			t.Fatalf("binding initializer skipped %v rejection: %q", op, v.reason)
		}
		if v.gonLoweringDepth != 0 {
			t.Fatalf("binding initializer rejection leaked scope: depth %d", v.gonLoweringDepth)
		}
	}
}
