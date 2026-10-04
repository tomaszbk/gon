package ssacompile

import (
	"testing"

	"cmd/compile/internal/ssa/block"
	"cmd/compile/internal/ssa/ssaop"
	"cmd/compile/internal/types"
)

type conditionalLoadCase struct {
	bits            int
	equal           bool
	reverseOperands bool
	wrongEdge       bool
	storeTwo        bool
	changedSnapshot bool
	payloadOffset   int64
	tail            string
}

func conditionalLoadFunc(t *testing.T, tc conditionalLoadCase) fun {
	c := testConfig(t)
	typ := c.config.Types.UInt64
	constant, equal, notEqual := ssaop.OpConst64, ssaop.OpEq64, ssaop.OpNeq64
	switch tc.bits {
	case 8:
		typ = c.config.Types.UInt8
		constant, equal, notEqual = ssaop.OpConst8, ssaop.OpEq8, ssaop.OpNeq8
	case 16:
		typ = c.config.Types.UInt16
		constant, equal, notEqual = ssaop.OpConst16, ssaop.OpEq16, ssaop.OpNeq16
	case 32:
		typ = c.config.Types.UInt32
		constant, equal, notEqual = ssaop.OpConst32, ssaop.OpEq32, ssaop.OpNeq32
	}
	comparison := notEqual
	first, second := "store", "merge"
	if tc.equal {
		comparison, first, second = equal, second, first
	}
	if tc.wrongEdge {
		first, second = second, first
	}
	operands := []string{"old", "one"}
	if tc.reverseOperands {
		operands[0], operands[1] = operands[1], operands[0]
	}
	entry := []any{
		Valu("mem", ssaop.OpInitMem, types.TypeMem, 0, nil),
		Valu("ptr", ssaop.OpArg, typ.PtrTo(), 0, nil),
		Valu("alias", ssaop.OpArg, typ.PtrTo(), 0, nil),
		Valu("one", constant, typ, 1, nil),
		Valu("two", constant, typ, 2, nil),
		Valu("old", ssaop.OpLoad, typ, 0, nil, "ptr", "mem"),
		Valu("condition", comparison, c.config.Types.Bool, 0, nil, operands...),
	}
	oldmem := "mem"
	if tc.changedSnapshot {
		entry = append(entry, Valu("changed", ssaop.OpStore, types.TypeMem, 0, typ, "ptr", "two", "mem"))
		oldmem = "changed"
	}
	entry = append(entry, If("condition", first, second))
	stored := "one"
	if tc.storeTwo {
		stored = "two"
	}
	store := []any{
		Valu("savedptr", ssaop.OpCopy, typ.PtrTo(), 0, nil, "ptr"),
		Valu("tagptr", ssaop.OpOffPtr, typ.PtrTo(), 0, nil, "savedptr"),
		Valu("stored", ssaop.OpStore, types.TypeMem, 0, typ, "tagptr", stored, oldmem),
	}
	newmem := "stored"
	switch tc.tail {
	case "payload":
		store = append(store,
			Valu("payloadptr", ssaop.OpOffPtr, typ.PtrTo(), tc.payloadOffset, nil, "savedptr"),
			Valu("tail", ssaop.OpStore, types.TypeMem, 0, typ, "payloadptr", "two", newmem))
		newmem = "tail"
	case "alias":
		store = append(store, Valu("tail", ssaop.OpStore, types.TypeMem, 0, typ, "alias", "two", newmem))
		newmem = "tail"
	case "unknown":
		store = append(store, Valu("tail", ssaop.OpZero, types.TypeMem, typ.Size(), typ, "ptr", newmem))
		newmem = "tail"
	case "partial":
		store = append(store,
			Valu("byte", ssaop.OpConst8, c.config.Types.UInt8, 2, nil),
			Valu("tail", ssaop.OpStore, types.TypeMem, 0, c.config.Types.UInt8, "ptr", "byte", newmem))
		newmem = "tail"
	case "call":
		store = append(store,
			Valu("call", ssaop.OpStaticLECall, types.TypeResultMem, 0, AuxCallLSym("change"), newmem),
			Valu("tail", ssaop.OpSelectN, types.TypeMem, 0, nil, "call"))
		newmem = "tail"
	}
	store = append(store, Goto("merge"))
	return c.Fun("entry",
		Bloc("entry", entry...),
		Bloc("store", store...),
		Bloc("merge",
			Valu("merged", ssaop.OpPhi, types.TypeMem, 0, nil, oldmem, newmem),
			Goto("read")),
		Bloc("read",
			Valu("readptr", ssaop.OpOffPtr, typ.PtrTo(), 0, nil, "ptr"),
			Valu("loaded", ssaop.OpLoad, typ, 0, nil, "readptr", "merged"),
			Valu("same", equal, c.config.Types.Bool, 0, nil, "loaded", "one"),
			If("same", "yes", "no")),
		Bloc("yes", Exit("merged")),
		Bloc("no", Exit("merged")))
}

func TestConditionalLoadConstant(t *testing.T) {
	tests := []struct {
		name string
		conditionalLoadCase
		want bool
	}{
		{"uint64", conditionalLoadCase{}, true},
		{"uint32", conditionalLoadCase{bits: 32}, true},
		{"uint16", conditionalLoadCase{bits: 16}, true},
		{"uint8", conditionalLoadCase{bits: 8}, true},
		{"equal_edge", conditionalLoadCase{equal: true}, true},
		{"reversed_operands", conditionalLoadCase{reverseOperands: true}, true},
		{"copied_pointer_disjoint_payload", conditionalLoadCase{tail: "payload", payloadOffset: 8}, true},
		{"wrong_edge", conditionalLoadCase{wrongEdge: true}, false},
		{"unequal_values", conditionalLoadCase{storeTwo: true}, false},
		{"changed_snapshot", conditionalLoadCase{changedSnapshot: true}, false},
		{"aliasing_store", conditionalLoadCase{tail: "alias"}, false},
		{"partially_overlapping_store", conditionalLoadCase{tail: "payload", payloadOffset: 4}, false},
		{"overwritten_tag", conditionalLoadCase{tail: "payload"}, false},
		{"unknown_memory", conditionalLoadCase{tail: "unknown"}, false},
		{"partial_width_store", conditionalLoadCase{tail: "partial"}, false},
		{"call_after_store", conditionalLoadCase{tail: "call"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fun := conditionalLoadFunc(t, tc.conditionalLoadCase)
			CheckFunc(fun.f)
			got := conditionalLoadConstant(fun.values["loaded"])
			if (got != nil) != tc.want || got != nil && got.AuxInt != 1 {
				t.Fatalf("constant = %v, want known=%v with value 1", got, tc.want)
			}
		})
	}
}

func TestProveConditionalLoad(t *testing.T) {
	fun := conditionalLoadFunc(t, conditionalLoadCase{tail: "payload", payloadOffset: 8})
	CheckFunc(fun.f)
	prove(fun.f)
	CheckFunc(fun.f)
	if fun.blocks["read"].Kind != block.BlockFirst {
		t.Fatalf("redundant load comparison still branches: %s", fun.blocks["read"].LongString())
	}
	// The comparison consumes a constant, so the actual load is now dead.
	if fun.values["loaded"].Uses != 0 {
		t.Fatalf("load still has %d uses", fun.values["loaded"].Uses)
	}
}

func TestConditionalLoadConstantLoop(t *testing.T) {
	c := testConfig(t)
	typ := c.config.Types.UInt64
	fun := c.Fun("entry",
		Bloc("entry",
			Valu("mem", ssaop.OpInitMem, types.TypeMem, 0, nil),
			Valu("ptr", ssaop.OpArg, typ.PtrTo(), 0, nil),
			Valu("condition", ssaop.OpArg, c.config.Types.Bool, 0, nil),
			Valu("one", ssaop.OpConst64, typ, 1, nil),
			Valu("stored", ssaop.OpStore, types.TypeMem, 0, typ, "ptr", "one", "mem"),
			Goto("loop")),
		Bloc("loop",
			Valu("merged", ssaop.OpPhi, types.TypeMem, 0, nil, "stored", "merged"),
			Valu("loaded", ssaop.OpLoad, typ, 0, nil, "ptr", "merged"),
			If("condition", "loop", "exit")),
		Bloc("exit", Exit("merged")))
	CheckFunc(fun.f)
	// A memory backedge is not inspected recursively: the bounded analysis
	// deliberately declines this case instead of relying on cyclic facts.
	if got := conditionalLoadConstant(fun.values["loaded"]); got != nil {
		t.Fatalf("cyclic memory Phi established a constant: %v", got)
	}
}
