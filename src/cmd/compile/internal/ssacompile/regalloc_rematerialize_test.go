package ssacompile

import (
	"testing"

	"cmd/compile/internal/ssa/block"
	"cmd/compile/internal/ssa/ssaop"
	"cmd/compile/internal/types"
)

func TestRegallocDeadRematerializations(t *testing.T) {
	// Both stored constants are live across the loop in the original SSA.
	// Shuffle can rematerialize them on the no-store edge solely to match
	// the merge's register state. Once strict SSA is restored, these copies
	// have no uses and must not remain in the generated fast path.
	c := testConfigARM64(t)
	intType := c.config.Types.Int64
	ptrType := intType.PtrTo()
	f := c.Fun("entry",
		Bloc("entry",
			Valu("mem", ssaop.OpInitMem, types.TypeMem, 0, nil),
			Valu("ptr", ssaop.OpArg, ptrType, 0, c.Temp(ptrType)),
			Valu("zero", ssaop.OpARM64MOVDconst, intType, 0, nil),
			Valu("one", ssaop.OpARM64MOVDconst, intType, 1, nil),
			Valu("payload", ssaop.OpARM64MOVDconst, intType, 7, nil),
			Goto("loop")),
		Bloc("loop",
			Valu("loopmem", ssaop.OpPhi, types.TypeMem, 0, nil, "mem", "merged"),
			Valu("index", ssaop.OpPhi, intType, 0, nil, "zero", "next"),
			Valu("bound", ssaop.OpARM64CMPconst, types.TypeFlags, 64, nil, "index"),
			ctrl{block.BlockARM64LT, "bound", []string{"check", "exit"}}),
		Bloc("check",
			Valu("tag", ssaop.OpARM64MOVDload, intType, 0, nil, "ptr", "loopmem"),
			Valu("present", ssaop.OpARM64CMPconst, types.TypeFlags, 1, nil, "tag"),
			ctrl{block.BlockARM64NE, "present", []string{"store", "skip"}}),
		Bloc("store",
			Valu("stored", ssaop.OpARM64STP, types.TypeMem, 0, nil, "ptr", "one", "payload", "loopmem"),
			Goto("merge")),
		Bloc("skip", Goto("merge")),
		Bloc("merge",
			Valu("merged", ssaop.OpPhi, types.TypeMem, 0, nil, "stored", "loopmem"),
			Valu("next", ssaop.OpARM64ADDconst, intType, 1, nil, "index"),
			Goto("loop")),
		Bloc("exit", Exit("loopmem")))
	CheckFunc(f.f)
	regalloc(f.f)
	CheckFunc(f.f)
	for _, b := range f.f.Blocks {
		for _, v := range b.Values {
			if v.Uses == 0 && v.Rematerializeable() && v.Removeable() {
				t.Errorf("unused rematerialization in %s: %s", b, v.LongString())
			}
		}
	}
	// Useful rematerializations, the conditional store, and the flag controls
	// must survive the cleanup.
	stores := 0
	for _, v := range f.blocks["store"].Values {
		if v.Op == ssaop.OpARM64STP {
			stores++
			if v.Args[1].AuxInt != 1 || v.Args[2].AuxInt != 7 {
				t.Fatalf("store operands changed: %s", v.LongString())
			}
		}
	}
	if stores != 1 || f.blocks["check"].Controls[0].Op != ssaop.OpARM64CMPconst {
		t.Fatal("cleanup removed the store or its control")
	}
}
