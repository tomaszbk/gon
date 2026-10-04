package ssacompile

import (
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/ssa/block"
	"cmd/compile/internal/ssa/ssaop"
	"cmd/compile/internal/types"
)

// conditionalLoadConstant recognizes a constant established on both inputs
// of a memory merge. For example, after `if *p != 1 { *p = 1 }`, another load
// of *p is 1: the store establishes it on one edge and the comparison on the
// other. This also avoids a redundant Option presence check after ??=.
//
// This is a memory fact, not just a control-flow fact. Unknown or potentially
// aliasing writes stop the search. No stores are introduced or moved.
func conditionalLoadConstant(v *ssa.Value) *ssa.Value {
	if v.Op != ssaop.OpLoad || !v.Type.IsInteger() {
		return nil
	}
	mem := v.MemoryArg()
	if mem.Op != ssaop.OpPhi || len(mem.Args) != 2 || len(mem.Block.Preds) != 2 {
		return nil
	}
	var result *ssa.Value
	for i, m := range mem.Args {
		c := constantLoadOnEdge(v, m, mem.Block.Preds[i])
		if c == nil || result != nil && (c.Op != result.Op || c.AuxInt != result.AuxInt) {
			return nil
		}
		result = c
	}
	return result
}

func constantLoadOnEdge(load, mem *ssa.Value, edge ssa.Edge) *ssa.Value {
	// The equality edge of a predecessor comparison can establish the value
	// of a load, but only for that comparison's exact memory snapshot.
	var guarded, constant *ssa.Value
	if p := edge.B; p.Kind == block.BlockIf {
		control := p.Controls[0]
		equality := false
		switch control.Op {
		case ssaop.OpEq8, ssaop.OpEq16, ssaop.OpEq32, ssaop.OpEq64:
			equality = edge.I == 0
		case ssaop.OpNeq8, ssaop.OpNeq16, ssaop.OpNeq32, ssaop.OpNeq64:
			equality = edge.I == 1
		}
		if equality {
			a, b := control.Args[0], control.Args[1]
			if a.IsGenericIntConst() {
				a, b = b, a
			}
			if a.Op == ssaop.OpLoad && b.IsGenericIntConst() && a.Type.Size() == load.Type.Size() && b.Type.Size() == load.Type.Size() && sameConditionalLoadPtr(a.Args[0], load.Args[0]) {
				guarded, constant = a, b
			}
		}
	}
	// Keep compile-time work bounded and decline loops or long store chains.
	for range 32 {
		if guarded != nil && mem == guarded.MemoryArg() {
			return constant
		}
		switch mem.Op {
		case ssaop.OpCopy, ssaop.OpVarDef, ssaop.OpVarLive:
			mem = mem.MemoryArg()
		case ssaop.OpStore:
			ptr, value := mem.Args[0], mem.Args[1]
			stored := mem.Aux.(*types.Type)
			if sameConditionalLoadPtr(ptr, load.Args[0]) && stored.IsInteger() && stored.Size() == load.Type.Size() && value.IsGenericIntConst() {
				return value
			}
			if !conditionalLoadDisjoint(ptr, stored, load.Args[0], load.Type) {
				return nil
			}
			mem = mem.MemoryArg()
		default:
			return nil
		}
	}
	return nil
}

// Copy and zero-offset pointer wrappers do not change an address. The prove
// pass can introduce Copies when removing nil checks, after the preceding CSE.
func conditionalLoadPtr(v *ssa.Value) *ssa.Value {
	for v.Op == ssaop.OpCopy || v.Op == ssaop.OpOffPtr && v.AuxInt == 0 {
		v = v.Args[0]
	}
	return v
}

func sameConditionalLoadPtr(a, b *ssa.Value) bool {
	a, b = conditionalLoadPtr(a), conditionalLoadPtr(b)
	if ssa.IsSamePtr(a, b) {
		return true
	}
	if a.Op != b.Op {
		return false
	}
	switch a.Op {
	case ssaop.OpOffPtr:
		return a.AuxInt == b.AuxInt && sameConditionalLoadPtr(a.Args[0], b.Args[0])
	case ssaop.OpAddPtr:
		return conditionalLoadPtr(a.Args[1]) == conditionalLoadPtr(b.Args[1]) && sameConditionalLoadPtr(a.Args[0], b.Args[0])
	}
	return false
}

func conditionalLoadDisjoint(a *ssa.Value, at *types.Type, b *ssa.Value, bt *types.Type) bool {
	baseOffset := func(v *ssa.Value) (*ssa.Value, int64) {
		v = conditionalLoadPtr(v)
		var offset int64
		for v.Op == ssaop.OpOffPtr {
			offset += v.AuxInt
			v = conditionalLoadPtr(v.Args[0])
		}
		return v, offset
	}
	ab, ao := baseOffset(a)
	bb, bo := baseOffset(b)
	if sameConditionalLoadPtr(ab, bb) {
		return !ssa.Overlap(ao, at.Size(), bo, bt.Size())
	}
	return ssa.Disjoint(conditionalLoadPtr(a), at, conditionalLoadPtr(b), bt)
}
