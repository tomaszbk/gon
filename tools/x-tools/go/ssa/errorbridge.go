package ssa

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
)

// propagationKind says how postfix ! leaves the function being built. The
// checker has already validated the enclosing function, so the signature
// alone selects the kind: a final error result, exactly one Result, or else a
// test function whose first parameter reports the failure with Fatal.
type propagationKind int

const (
	propagateError propagationKind = iota
	propagateResult
	propagateTest
)

func propagationOf(sig *types.Signature) propagationKind {
	results := sig.Results()
	if n := results.Len(); n > 0 && types.Identical(results.At(n-1).Type(), types.Universe.Lookup("error").Type()) {
		return propagateError
	}
	if results.Len() == 1 && types.IsCanonicalResult(results.At(0).Type()) {
		return propagateResult
	}
	return propagateTest
}

// propagateFailure emits the return of postfix ! without a handler body.
// failure is the failed call's error, or the payload of a failed Result when
// fromResult is set.
func (b *builder) propagateFailure(fn *Function, e *ast.ErrorExpr, failure Value, fromResult bool) {
	sig := fn.source.Signature
	results := make([]Value, sig.Results().Len())
	for i := range results {
		results[i] = zeroConst(fn.typ(sig.Results().At(i).Type()))
	}
	ret := &ast.ReturnStmt{Return: e.OpPos}
	switch propagationOf(sig) {
	case propagateError:
		err := failure
		if fromResult {
			err = nilResultError(fn, emitConv(fn, failure, types.Universe.Lookup("error").Type()), e.OpPos)
		}
		results[len(results)-1] = err
	case propagateResult:
		// An error tuple fails as Err(err), converted to the error type.
		result := fn.typ(sig.Results().At(0).Type())
		variant := enumAlternative(result, "Err")
		results[0] = enumValue(fn, result, variant, []Value{emitConv(fn, failure, variant.Field(0).Type())}, e.OpPos)
	case propagateTest:
		b.fatalCall(fn, sig, failure, e.OpPos)
	}
	b.returnValues(fn, ret, results)
}

// nilResultError returns err, or errors.ErrNilResult when err is nil: a
// failed Result is a failure even when its payload is nil.
func nilResultError(fn *Function, err Value, pos token.Pos) Value {
	substitute := fn.newBasicBlock("result.nilerror")
	done := fn.newBasicBlock("result.error")
	// The edge from the nil test enters done first, then the substitute.
	emitIf(fn, emitCompare(fn, token.EQL, err, zeroConst(err.Type()), pos), substitute, done)
	fn.currentBlock = substitute
	var sentinel Value
	if p := fn.Prog.ImportedPackage("errors"); p != nil {
		if g, ok := p.Members["ErrNilResult"].(*Global); ok {
			sentinel = emitLoad(fn, g)
		}
	}
	if sentinel == nil {
		// Without package errors, model an unknown non-nil error.
		text := NewConst(constant.MakeString("failed Result carries a nil error"), types.Typ[types.String])
		sentinel = emitConv(fn, text, err.Type())
	}
	emitJump(fn, done)
	fn.currentBlock = done
	phi := &Phi{Edges: []Value{err, sentinel}, Comment: "nil Result error"}
	phi.pos = pos
	phi.typ = err.Type()
	return done.emit(phi)
}

// fatalCall emits param.Fatal(failure) for the first parameter of sig, which
// the checker has verified to be *testing.T, *testing.B, *testing.F or
// testing.TB.
func (b *builder) fatalCall(fn *Function, sig *types.Signature, failure Value, pos token.Pos) {
	param := sig.Params().At(0)
	selection, ok := types.LookupSelection(param.Type(), true, fn.Pkg.Pkg, "Fatal")
	if !ok || selection.Kind() != types.MethodVal {
		panic("test propagation: missing Fatal method")
	}
	method := selection.Obj().(*types.Func)
	recv := recvType(method)
	wantAddr := isPointer(recv)

	v := Value(emitLoad(fn, fn.lookup(param, false)))
	index := selection.Index()
	v = emitImplicitSelections(fn, v, index[:len(index)-1], pos)
	if !types.IsInterface(v.Type()) && !wantAddr && isPointerCore(v.Type()) {
		v = emitLoad(fn, v)
	}

	var call Call
	call.Call.pos = pos
	if types.IsInterface(recv) {
		call.Call.Value = v
		call.Call.Method = method
	} else {
		call.Call.Value = fn.Prog.objectMethod(method, nil, b)
		call.Call.Args = []Value{v}
	}
	// Fatal(args ...any): pass the failure as the only variadic element.
	slice := method.Type().(*types.Signature).Params().At(0).Type().(*types.Slice)
	elem := slice.Elem()
	array := emitNew(fn, types.NewArray(elem, 1), pos, "varargs")
	index0 := &IndexAddr{X: array, Index: intConst(0)}
	index0.setType(types.NewPointer(elem))
	fn.emit(index0)
	emitStore(fn, index0, emitConv(fn, failure, elem), pos)
	s := &Slice{X: array}
	s.setType(slice)
	call.Call.Args = append(call.Call.Args, fn.emit(s))
	call.setType(types.NewTuple())
	fn.emit(&call)
}
