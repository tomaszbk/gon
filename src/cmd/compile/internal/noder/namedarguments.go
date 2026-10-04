package noder

import (
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types2"
)

// namedCallOrder is used only after successful type checking. The public and
// compiler ASTs retain their written order; only evaluated values are reordered.
func namedCallOrder(call *syntax.CallExpr, sig *types2.Signature) []int {
	order := make([]int, len(call.ArgList))
	for i, label := range call.ArgNames {
		order[i] = i
		if label == nil {
			continue
		}
		for j := 0; j < sig.Params().Len(); j++ {
			if sig.Params().At(j).Name() == label.Value {
				order[i] = j
				break
			}
		}
	}
	return order
}
