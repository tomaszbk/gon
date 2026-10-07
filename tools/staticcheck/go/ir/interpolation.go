// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"
)

// interpolation lowers a checked expression when the optional
// types.Info.Interpolations map was not requested by the caller.
func (b *builder) interpolation(fn *Function, e *ast.InterpolatedStringExpr) Value {
	var sprintf *types.Func
	for _, imported := range syntheticPackage(fn).Pkg.Imports() {
		if imported.Path() == "fmt" {
			sprintf, _ = imported.Scope().Lookup("Sprintf").(*types.Func)
			break
		}
	}
	if sprintf == nil {
		panic("checked interpolation has no fmt.Sprintf")
	}
	var format strings.Builder
	var values []Value
	sig := sprintf.Type().(*types.Signature)
	varargsType := sig.Params().At(1).Type().(*types.Slice)
	for _, part := range e.Parts {
		if part.Expr == nil {
			text := part.Text
			if e.Quote == '`' {
				text = strings.ReplaceAll(text, "\r", "")
			} else {
				var escaped strings.Builder
				for i := 0; i < len(text); i++ {
					if text[i] == '\\' && i+1 < len(text) {
						i++
						if text[i] != '$' {
							escaped.WriteByte('\\')
						}
					}
					escaped.WriteByte(text[i])
				}
				var err error
				text, err = strconv.Unquote("\"" + escaped.String() + "\"")
				if err != nil {
					panic("invalid checked interpolation text: " + err.Error())
				}
			}
			format.WriteString(strings.ReplaceAll(text, "%", "%%"))
			continue
		}
		verb := part.Format
		if verb == "" {
			verb = "%v"
		}
		format.WriteString(strings.ReplaceAll(verb, "[1]", "["+strconv.Itoa(len(values)+1)+"]"))
		// Materialize each operand before evaluating the next one.
		values = append(values, emitConv(fn, b.expr(fn, part.Expr), varargsType.Elem(), part.Expr))
	}
	args := []Value{stringConst(format.String(), e)}
	if len(values) == 0 {
		args = append(args, zeroConst(varargsType, e))
	} else {
		array := emitNew(fn, types.NewArray(varargsType.Elem(), int64(len(values))), e, "interpolation operands")
		for i, value := range values {
			addr := &IndexAddr{X: array, Index: intConst(int64(i), e)}
			addr.setType(types.NewPointer(varargsType.Elem()))
			fn.emit(addr, e)
			emitStore(fn, addr, value, e)
		}
		slice := &Slice{X: array}
		slice.setType(varargsType)
		args = append(args, fn.emit(slice, e))
	}
	call := &Call{Call: CallCommon{Value: fn.Prog.FuncValue(sprintf), Args: args}}
	call.setType(types.Typ[types.String])
	return emitCall(fn, call, e)
}
