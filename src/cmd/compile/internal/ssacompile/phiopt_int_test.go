package ssacompile

import (
	"fmt"
	"testing"

	"cmd/compile/internal/ssa/ssaop"
	"cmd/compile/internal/types"
)

// Integer flags often survive an inlined branch containing additional control
// flow. Their comparisons should reuse that branch's condition, just as boolean
// Phis do, without removing the control flow that computes the other payloads.
func TestPhioptIntegerNestedBranches(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			c := testConfigArch(t, arch)
			for _, width := range []struct {
				name string
				typ  *types.Type
				con  ssaop.Op
				eq   ssaop.Op
				neq  ssaop.Op
			}{
				{"int8", c.config.Types.Int8, ssaop.OpConst8, ssaop.OpEq8, ssaop.OpNeq8},
				{"uint16", c.config.Types.UInt16, ssaop.OpConst16, ssaop.OpEq16, ssaop.OpNeq16},
				{"int32", c.config.Types.Int32, ssaop.OpConst32, ssaop.OpEq32, ssaop.OpNeq32},
				{"uint64", c.config.Types.UInt64, ssaop.OpConst64, ssaop.OpEq64, ssaop.OpNeq64},
			} {
				for _, shape := range []string{"nested", "direct-true", "direct-false"} {
					for _, trueTag := range []int64{0, 1} {
						for _, compareTag := range []int64{0, 1} {
							for _, equal := range []bool{false, true} {
								name := fmt.Sprintf("%s/%s/true=%d/compare=%d/equal=%t", width.name, shape, trueTag, compareTag, equal)
								t.Run(name, func(t *testing.T) {
									op := width.neq
									if equal {
										op = width.eq
									}
									fun := integerFlagPhi(c, width.typ, width.con, op, shape, trueTag, 1-trueTag, compareTag)
									CheckFunc(fun.f)
									phiopt(fun.f)
									CheckFunc(fun.f)
									if fun.values["flag"].Op == ssaop.OpPhi {
										t.Fatal("integer flag Phi was not converted")
									}
									opt(fun.f)
									CheckFunc(fun.f)
									got := fun.blocks["merge"].Controls[0]
									for got.Op == ssaop.OpCopy {
										got = got.Args[0]
									}
									if got != fun.values["condition"] {
										t.Fatalf("comparison = %s, want original condition", got.LongString())
									}
									// Generic rewriting removes Not controls by swapping the
									// successors. Check both the reused condition and its sense.
									wantTrue := "yes"
									if (trueTag == compareTag) != equal {
										wantTrue = "no"
									}
									if fun.blocks["merge"].Succs[0].B != fun.blocks[wantTrue] {
										t.Fatal("comparison has the wrong branch sense")
									}
								})
							}
						}
					}
				}
			}
		})
	}
}

func TestPhioptIntegerRejectsUnsafeFlags(t *testing.T) {
	c := testConfig(t)
	for _, tc := range []struct {
		name              string
		shape             string
		trueTag, falseTag int64
	}{
		{"other-constant", "nested", 2, 0},
		{"negative-constant", "nested", -1, 0},
		{"same-constant", "nested", 1, 1},
		{"shared-successor", "shared", 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fun := integerFlagPhi(c, c.config.Types.UInt64, ssaop.OpConst64, ssaop.OpEq64, tc.shape, tc.trueTag, tc.falseTag, 1)
			CheckFunc(fun.f)
			phiopt(fun.f)
			CheckFunc(fun.f)
			if got := fun.values["flag"]; got.Op != ssaop.OpPhi {
				t.Fatalf("unsafe flag was converted: %s", got.LongString())
			}
		})
	}
}

// integerFlagPhi builds nested branches with different predecessor orders. In
// the shared shape, the false predecessor can also be reached from the true
// branch, so the flag must not be replaced with the outer condition.
func integerFlagPhi(c *Conf, typ *types.Type, con, compare ssaop.Op, shape string, trueTag, falseTag, compareTag int64) fun {
	left, right := "left", "right"
	if shape == "direct-true" {
		left = "merge"
	}
	if shape == "direct-false" {
		right = "merge"
	}
	blocs := []bloc{Bloc("entry",
		Valu("mem", ssaop.OpInitMem, types.TypeMem, 0, nil),
		Valu("condition", ssaop.OpArg, c.config.Types.Bool, 0, c.Temp(c.config.Types.Bool)),
		Valu("inner", ssaop.OpArg, c.config.Types.Bool, 1, c.Temp(c.config.Types.Bool)),
		Valu("trueTag", con, typ, trueTag, nil),
		Valu("falseTag", con, typ, falseTag, nil),
		Valu("compareTag", con, typ, compareTag, nil),
		If("condition", left, right))}
	args := []string{"trueTag", "falseTag"}
	switch shape {
	case "nested":
		blocs = append(blocs,
			Bloc("left", If("inner", "left-join", "left-extra")),
			Bloc("left-extra", Goto("left-join")),
			Bloc("left-join", Goto("merge")),
			Bloc("right", Goto("merge")))
	case "direct-true":
		blocs = append(blocs,
			Bloc("right", If("inner", "right-join", "right-extra")),
			Bloc("right-extra", Goto("right-join")),
			Bloc("right-join", Goto("merge")))
	case "direct-false":
		args = []string{"falseTag", "trueTag"}
		blocs = append(blocs,
			Bloc("left", If("inner", "left-join", "left-extra")),
			Bloc("left-extra", Goto("left-join")),
			Bloc("left-join", Goto("merge")))
	case "shared":
		blocs = append(blocs,
			Bloc("left", If("inner", "merge", "shared")),
			Bloc("right", Goto("shared")),
			Bloc("shared", Goto("merge")))
	default:
		c.tb.Fatalf("unknown shape %q", shape)
	}
	blocs = append(blocs,
		Bloc("merge",
			Valu("flag", ssaop.OpPhi, typ, 0, nil, args...),
			Valu("comparison", compare, c.config.Types.Bool, 0, nil, "flag", "compareTag"),
			If("comparison", "yes", "no")),
		Bloc("yes", Exit("mem")),
		Bloc("no", Exit("mem")))
	return c.Fun("entry", blocs...)
}
