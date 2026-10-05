package noder

import (
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types"
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

// namedArgumentType returns the type that the i'th argument of a call to a
// function of the given signature is converted to, or nil if it is not known.
// Arguments of a call without a trailing ... are matched to the element type
// of a variadic parameter.
func namedArgumentType(signature *types.Type, i int, dots bool) *types.Type {
	if signature == nil || signature.Kind() != types.TFUNC {
		return nil
	}
	params := signature.Params()
	if i >= len(params) {
		return nil
	}
	typ := params[i].Type
	if params[i].IsDDD() && !dots {
		typ = typ.Elem()
	}
	return typ
}
