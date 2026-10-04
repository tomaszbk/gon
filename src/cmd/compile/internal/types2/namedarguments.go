// This file checks named arguments without changing function-type identity.
package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// namedArgumentOrder maps written arguments to static parameter positions.
// A nil result denotes an invalid named call (ordinary calls bypass this helper).
func (check *Checker) namedArgumentOrder(call *syntax.CallExpr, sig *Signature) []int {
	if len(call.ArgNames) != len(call.ArgList) {
		check.error(call, InvalidCall, "invalid argument labels")
		return nil
	}
	order := make([]int, len(call.ArgList))
	used := make([]bool, sig.Params().Len())
	named := false
	for i, label := range call.ArgNames {
		j := i
		if label != nil {
			named = true
			j = -1
			if label.Value != "_" {
				for k := 0; k < sig.Params().Len(); k++ {
					if sig.Params().At(k).Name() == label.Value {
						j = k
						break
					}
				}
			}
			if j < 0 {
				check.errorf(label, InvalidCall, "unknown argument name %s", label.Value)
				return nil
			}
			check.recordUse(label, sig.Params().At(j))
			if sig.Variadic() && j == sig.Params().Len()-1 && (!hasDots(call) || i != len(call.ArgList)-1) {
				check.error(label, InvalidCall, "named variadic argument requires a slice followed by ... as the final argument")
				return nil
			}
		} else if named {
			check.error(call.ArgList[i], InvalidCall, "positional argument after named argument")
			return nil
		}
		if j >= len(used) {
			check.error(call.ArgList[i], WrongArgCount, "too many arguments")
			return nil
		}
		if used[j] {
			check.errorf(call.ArgList[i], InvalidCall, "duplicate argument for parameter %s", sig.Params().At(j).Name())
			return nil
		}
		used[j], order[i] = true, j
	}
	required := len(used)
	if sig.Variadic() && !hasDots(call) {
		required--
	}
	for i := 0; i < required; i++ {
		if !used[i] {
			check.errorf(call, WrongArgCount, "missing argument for parameter %s", sig.Params().At(i).Name())
			return nil
		}
	}
	return order
}

// namedArguments checks values in written order with the associated target types,
// then supplies parameter-ordered operands to ordinary generic inference.
func (check *Checker) namedArguments(call *syntax.CallExpr, sig *Signature, order []int, targetAt func(int) *target) ([]*operand, [][]Type) {
	args := make([]*operand, len(order))
	atargs := make([][]Type, len(order))
	for i, e := range call.ArgList {
		j := order[i]
		values, typeargs := check.genericExprList(func(int) *target { return targetAt(j) }, []syntax.Expr{e})
		if len(values) != 1 {
			check.error(e, InvalidCall, "named call arguments must be single-valued")
			return nil, nil
		}
		args[j] = values[0]
		if len(typeargs) > 0 {
			atargs[j] = typeargs[0]
		}
	}
	return args, atargs
}
