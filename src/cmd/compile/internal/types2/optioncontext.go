// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import "cmd/compile/internal/syntax"

// An implicitly lifted optional result needs a complete lambda result target;
// its body cannot infer that target.
func lambdaNeedsOptionContext(e *syntax.LambdaExpr, sig *Signature) bool {
	return sig.results.Len() == 1 && IsOptional(sig.results.At(0).typ)
}

// optionAssignment performs one implicit lift without changing the recorded
// source type. Its recursive assignment checks only ordinary assignability of
// the payload; it must never invent another optional layer.
func (check *Checker) optionAssignment(x *operand, T Type, context string) bool {
	if check.optionLiftDepth != 0 || x.multiValue || !IsOptional(T) || isGeneric(T) {
		return false
	}
	if ok, _ := x.assignableTo(check, T, nil); ok {
		return false
	}
	if x.typ() != Typ[UntypedNil] {
		payload := OptionalOf(T).Elem()
		check.optionLiftDepth++
		check.assignment(x, payload, context)
		check.optionLiftDepth--
		if !x.isValid() {
			return true
		}
	}
	if check.OptionalConversions != nil {
		check.OptionalConversions[x.expr] = T
	}
	x.mode_, x.typ_, x.val = value, T, nil
	return true
}
