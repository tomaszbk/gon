// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"go/ast"
	"go/types"
)

// emitNamedCallArgs evaluates arguments in their written order, after the
// function operand and receiver, and then associates values with parameters.
func (b *builder) emitNamedCallArgs(fn *Function, sig *types.Signature, e *ast.CallExpr, args []Value) []Value {
	params := sig.Params()
	values := make([]Value, params.Len())
	for i, arg := range e.Args {
		index := i
		if name := e.ArgNames[i]; name != nil {
			index = -1
			for j := range params.Len() {
				if params.At(j).Name() == name.Name {
					index = j
					break
				}
			}
		}
		if index < 0 || index >= len(values) || values[index] != nil {
			panic("invalid named argument in well-typed call")
		}
		values[index] = emitConv(fn, b.expr(fn, arg), params.At(index).Type(), arg)
	}
	if sig.Variadic() && values[len(values)-1] == nil {
		values[len(values)-1] = zeroConst(params.At(len(values)-1).Type(), e)
	}
	return append(args, values...)
}
