package ssa

import (
	"go/ast"
	"go/token"
	"go/types"
)

func enumSelector(fn *Function, expr ast.Expr) (types.Type, *types.EnumVariant) {
	sel, ok := ast.Unparen(expr).(*ast.SelectorExpr)
	if !ok {
		return nil, nil
	}
	typ := fn.typeOf(sel.X)
	desc := types.EnumOf(typ)
	if desc == nil {
		return nil, nil
	}
	for i := range desc.NumVariants() {
		v := desc.Variant(i)
		if v.Name() == sel.Sel.Name {
			return typ, v
		}
	}
	return nil, nil
}

func alternativeStruct(t types.Type) *types.Struct {
	if o := types.OptionalStorage(t); o != nil {
		return o
	}
	return t.Underlying().(*types.Struct)
}

func enumField(fn *Function, v Value, index int, pos token.Pos) Value {
	f := &Field{X: v, Field: index}
	f.setType(alternativeStruct(v.Type()).Field(index).Type())
	f.setPos(pos)
	return fn.emit(f)
}
func enumFieldAddr(fn *Function, v Value, index int, pos token.Pos) Value {
	t := alternativeStruct(v.Type().Underlying().(*types.Pointer).Elem()).Field(index).Type()
	f := &FieldAddr{X: v, Field: index}
	f.setType(types.NewPointer(t))
	f.setPos(pos)
	return fn.emit(f)
}
func enumValue(fn *Function, typ types.Type, variant *types.EnumVariant, values []Value, pos token.Pos) Value {
	alloc := emitNew(fn, typ, pos, "enum value")
	tag := enumFieldAddr(fn, alloc, 0, pos)
	emitStore(fn, tag, emitConv(fn, intConst(int64(variant.Tag())), types.Typ[types.Uint]), pos)
	payload := enumFieldAddr(fn, alloc, variant.StorageIndex(), pos)
	for i, value := range values {
		if value != nil {
			addr := enumFieldAddr(fn, payload, i, pos)
			emitStore(fn, addr, emitConv(fn, value, variant.Field(i).Type()), pos)
		}
	}
	return emitLoad(fn, alloc)
}
func (b *builder) enumConstructor(fn *Function, typ types.Type, v *types.EnumVariant, e ast.Expr) Value {
	if v.NumFields() == 0 {
		return enumValue(fn, typ, v, nil, e.Pos())
	}
	sig := fn.typeOf(e).(*types.Signature)
	constructor := fn.Prog.NewFunction(types.TypeString(typ, nil)+"."+v.Name(), sig, "enum constructor")
	constructor.Pkg = fn.Pkg
	constructor.startBody()
	args := make([]Value, sig.Params().Len())
	i := 0
	for param := range sig.Params().Variables() {
		args[i] = constructor.addParamVar(param)
		i++
	}
	value := enumValue(constructor, typ, v, args, e.Pos())
	constructor.emit(&Return{Results: []Value{value}})
	constructor.finishBody()
	return constructor
}
func (b *builder) enumLiteral(fn *Function, e *ast.CompositeLit, typ types.Type, v *types.EnumVariant) Value {
	values := make([]Value, v.NumFields())
	for _, element := range e.Elts {
		kv := element.(*ast.KeyValueExpr)
		name := kv.Key.(*ast.Ident).Name
		for i := range v.NumFields() {
			if v.Field(i).Name() == name {
				values[i] = b.expr(fn, kv.Value)
				break
			}
		}
	}
	return enumValue(fn, typ, v, values, e.Pos())
}

type patternBinding struct {
	object *types.Var
	value  Value
	pos    token.Pos
}

func (b *builder) matchPattern(fn *Function, p *ast.MatchPattern, value Value, next *BasicBlock, bindings *[]patternBinding) {
	if p == nil {
		return
	}
	if p.Inner != nil {
		if !p.Question.IsValid() {
			b.matchPattern(fn, p.Inner, value, next, bindings)
			return
		}
		tag := enumField(fn, value, 0, p.Pos())
		body := fn.newBasicBlock("match.present")
		emitIf(fn, emitCompare(fn, token.EQL, tag, emitConv(fn, intConst(1), tag.Type()), p.Pos()), body, next)
		fn.currentBlock = body
		payload := enumField(fn, enumField(fn, value, 2, p.Pos()), 0, p.Pos())
		b.matchPattern(fn, p.Inner, payload, next, bindings)
		return
	}
	if id, ok := p.Value.(*ast.Ident); ok && id.Name == "nil" && types.IsOptional(value.Type()) {
		tag := enumField(fn, value, 0, p.Pos())
		body := fn.newBasicBlock("match.absent")
		emitIf(fn, emitCompare(fn, token.EQL, tag, emitConv(fn, intConst(0), tag.Type()), p.Pos()), body, next)
		fn.currentBlock = body
		return
	}
	if id, ok := p.Value.(*ast.Ident); ok {
		if id.Name == "_" {
			return
		}
		obj, _ := fn.info.Defs[id].(*types.Var)
		if obj == nil {
			obj, _ = fn.info.Uses[id].(*types.Var)
		}
		if obj != nil {
			*bindings = append(*bindings, patternBinding{obj, value, id.Pos()})
			return
		}
	}
	if typ, variant := enumSelector(fn, p.Value); variant != nil {
		if types.IsInterface(value.Type()) {
			// The pattern tests the dynamic type of an interface subject.
			value = b.interfaceVariant(fn, typ, value, p.Pos(), next)
		}
		tag := enumField(fn, value, 0, p.Pos())
		body := fn.newBasicBlock("match.variant")
		emitIf(fn, emitCompare(fn, token.EQL, tag, emitConv(fn, intConst(int64(variant.Tag())), tag.Type()), p.Pos()), body, next)
		fn.currentBlock = body
		payload := enumField(fn, value, variant.StorageIndex(), p.Pos())
		for i, arg := range p.Args {
			b.matchPattern(fn, arg, enumField(fn, payload, i, arg.Pos()), next, bindings)
		}
		for _, field := range p.Fields {
			for i := range variant.NumFields() {
				if variant.Field(i).Name() == field.Name.Name {
					b.matchPattern(fn, field.Pattern, enumField(fn, payload, i, field.Pos()), next, bindings)
					break
				}
			}
		}
		return
	}
	success := fn.newBasicBlock("match.literal")
	emitIf(fn, emitCompare(fn, token.EQL, value, emitConv(fn, b.expr(fn, p.Value), value.Type()), p.Pos()), success, next)
	fn.currentBlock = success
}
func (b *builder) match(fn *Function, e *ast.MatchExpr, statement bool, label *lblock) Value {
	value := b.expr(fn, e.Tag)
	done := fn.newBasicBlock("match.done")
	var result *Alloc
	if !statement {
		result = emitNew(fn, fn.typeOf(e), e.Pos(), "match result")
	}
	saved := fn.targets
	if statement {
		fn.targets = &targets{tail: saved, _break: done}
		if label != nil {
			label._break = done
		}
	}
	for _, arm := range e.Arms {
		next := fn.newBasicBlock("match.next")
		matched := fn.newBasicBlock("match.arm")
		if len(arm.Patterns) == 0 {
			emitJump(fn, matched)
		} else {
			// The checker declares the common bindings in the first
			// alternative; following alternatives refer to the same objects.
			for node := range ast.Preorder(arm.Patterns[0]) {
				if id, ok := node.(*ast.Ident); ok {
					if obj, ok := fn.info.Defs[id].(*types.Var); ok {
						emitLocalVar(fn, obj)
					}
				}
			}
			for i, pattern := range arm.Patterns {
				failure := next
				if i < len(arm.Patterns)-1 {
					failure = fn.newBasicBlock("match.alternative")
				}
				var bindings []patternBinding
				b.matchPattern(fn, pattern, value, failure, &bindings)
				for _, binding := range bindings {
					emitStore(fn, fn.lookup(binding.object, false), binding.value, binding.pos)
				}
				emitJump(fn, matched)
				fn.currentBlock = failure
			}
		}
		fn.currentBlock = matched
		if arm.Guard != nil {
			body := fn.newBasicBlock("match.guard")
			emitIf(fn, b.expr(fn, arm.Guard), body, next)
			fn.currentBlock = body
		}
		if statement {
			b.stmt(fn, arm.Body)
		} else {
			emitStore(fn, result, emitConv(fn, b.expr(fn, arm.Value), fn.typeOf(e)), arm.Value.Pos())
		}
		emitJump(fn, done)
		fn.currentBlock = next
	}
	// Exhaustive, well-typed values cannot reach this continuation. Keeping a
	// CFG edge matches ordinary switch construction without a synthetic panic.
	emitJump(fn, done)
	fn.targets = saved
	fn.currentBlock = done
	if statement {
		return nil
	}
	return emitLoad(fn, result)
}

// patternTest evaluates the subject once, writes bindings only after the full
// pattern succeeds, and yields a bool. The checker scopes those bindings to
// later top-level && operands and the if body.
func (b *builder) patternTest(fn *Function, e *ast.PatternTestExpr) Value {
	value := b.expr(fn, e.X)
	failed := fn.newBasicBlock("is.false")
	done := fn.newBasicBlock("is.done")
	for node := range ast.Preorder(e.Pattern) {
		if id, ok := node.(*ast.Ident); ok {
			if obj, ok := fn.info.Defs[id].(*types.Var); ok {
				emitLocalVar(fn, obj)
			}
		}
	}
	var bindings []patternBinding
	b.matchPattern(fn, e.Pattern, value, failed, &bindings)
	for _, binding := range bindings {
		emitStore(fn, fn.lookup(binding.object, false), binding.value, binding.pos)
	}
	emitJump(fn, done)
	fn.currentBlock = failed
	emitJump(fn, done)
	fn.currentBlock = done
	typ := types.Default(fn.typ(fn.typeOf(e)))
	phi := &Phi{Edges: []Value{emitConv(fn, vTrue, typ), emitConv(fn, vFalse, typ)}, Comment: "pattern test"}
	phi.typ = typ
	phi.pos = e.Is
	return done.emit(phi)
}

var (
	tError = types.Universe.Lookup("error").Type()
	// Interfaces of the error-tree search performed by errors.As.
	asInterface      = searchInterface("As", []types.Type{tEface}, []types.Type{tBool})
	unwrapInterface  = searchInterface("Unwrap", nil, []types.Type{tError})
	unwrapsInterface = searchInterface("Unwrap", nil, []types.Type{types.NewSlice(tError)})
)

func searchInterface(name string, params, results []types.Type) *types.Interface {
	tuple := func(ts []types.Type) *types.Tuple {
		vars := make([]*types.Var, len(ts))
		for i, t := range ts {
			vars[i] = types.NewVar(token.NoPos, nil, "", t)
		}
		return types.NewTuple(vars...)
	}
	sig := types.NewSignatureType(nil, nil, nil, tuple(params), tuple(results), false)
	return types.NewInterfaceType([]*types.Func{types.NewFunc(token.NoPos, nil, name, sig)}, nil).Complete()
}

// interfaceVariant lowers the head of a variant pattern whose subject has
// interface type. It branches to next unless the subject holds the pattern's
// enum type, and otherwise returns the enum value. A nil interface holds none.
// For the predeclared error the test is the errors.As search of the error tree;
// any other interface uses a plain type assertion.
func (b *builder) interfaceVariant(fn *Function, enum types.Type, subject Value, pos token.Pos, next *BasicBlock) Value {
	var value, ok Value
	if types.Identical(subject.Type(), tError) {
		call := &Call{Call: CallCommon{Value: b.errorSearch(fn, enum, pos), Args: []Value{subject}, pos: pos}}
		call.setType(types.NewTuple(newVar("value", enum), varOk))
		call.setPos(pos)
		tuple := fn.emit(call)
		value, ok = emitExtract(fn, tuple, 0), emitExtract(fn, tuple, 1)
	} else {
		tuple := emitTypeTest(fn, subject, enum, pos)
		value, ok = emitExtract(fn, tuple, 0), emitExtract(fn, tuple, 1)
	}
	body := fn.newBasicBlock("match.interface")
	emitIf(fn, ok, body, next)
	fn.currentBlock = body
	return value
}

// invoke emits an interface method call of the single-method search interface
// iface on x and returns its result.
func invoke(fn *Function, x Value, iface *types.Interface, args []Value, pos token.Pos) Value {
	method := iface.Method(0)
	call := &Call{Call: CallCommon{Value: x, Method: method, Args: args, pos: pos}}
	results := method.Type().(*types.Signature).Results()
	call.setType(results.At(0).Type())
	call.setPos(pos)
	return fn.emit(call)
}

// errorSearch returns a synthetic function func(error) (E, bool) that finds the
// first value of enum type E in an error tree exactly as errors.AsType[E]
// does: err itself, then the errors reached through Unwrap() error and
// Unwrap() []error, depth first, matching a dynamic type E or an As(any) bool
// method that sets its *E argument. Nil errors end a chain and are skipped in
// a multi-error list.
func (b *builder) errorSearch(fn *Function, enum types.Type, pos token.Pos) *Function {
	prog := fn.Prog
	params := types.NewTuple(types.NewVar(pos, nil, "err", tError))
	results := types.NewTuple(types.NewVar(pos, nil, "", enum), types.NewVar(pos, nil, "", tBool))
	f := prog.NewFunction("errors.AsType["+types.TypeString(enum, nil)+"]", types.NewSignatureType(nil, nil, nil, params, results, false), "error tree search")
	f.Pkg = syntheticPackage(fn)
	f.startBody()
	err := f.addParamVar(params.At(0))
	current := emitLocal(f, tError, pos, "err")
	emitStore(f, current, err, pos)
	target := emitNew(f, enum, pos, "target") // handed to As methods
	ret := func(v, ok Value) {
		f.emit(&Return{Results: []Value{v, ok}, pos: pos})
		f.currentBlock = nil
	}
	fail := f.newBasicBlock("search.fail")
	loop := f.newBasicBlock("search.loop")
	emitJump(f, loop)
	f.currentBlock = loop
	e := emitLoad(f, current)
	present := f.newBasicBlock("search.present")
	emitIf(f, emitCompare(f, token.NEQ, e, zeroConst(tError), pos), present, fail)
	f.currentBlock = present

	// A dynamic type of E matches.
	test := emitTypeTest(f, e, enum, pos)
	hit, other := f.newBasicBlock("search.hit"), f.newBasicBlock("search.other")
	emitIf(f, emitExtract(f, test, 1), hit, other)
	f.currentBlock = hit
	ret(emitExtract(f, test, 0), vTrue)

	// An As method may report a match by setting target.
	f.currentBlock = other
	unwrap := f.newBasicBlock("search.unwrap")
	as := emitTypeTest(f, e, asInterface, pos)
	callAs := f.newBasicBlock("search.as")
	emitIf(f, emitExtract(f, as, 1), callAs, unwrap)
	f.currentBlock = callAs
	reported := invoke(f, emitExtract(f, as, 0), asInterface, []Value{emitConv(f, target, tEface)}, pos)
	found := f.newBasicBlock("search.as.found")
	emitIf(f, reported, found, unwrap)
	f.currentBlock = found
	ret(emitLoad(f, target), vTrue)

	// Unwrap() error continues with the single wrapped error.
	f.currentBlock = unwrap
	single := emitTypeTest(f, e, unwrapInterface, pos)
	step, multiple := f.newBasicBlock("search.step"), f.newBasicBlock("search.multiple")
	emitIf(f, emitExtract(f, single, 1), step, multiple)
	f.currentBlock = step
	emitStore(f, current, invoke(f, emitExtract(f, single, 0), unwrapInterface, nil, pos), pos)
	emitJump(f, loop)

	// Unwrap() []error searches every non-nil element in order.
	f.currentBlock = multiple
	list := emitTypeTest(f, e, unwrapsInterface, pos)
	children := f.newBasicBlock("search.children")
	emitIf(f, emitExtract(f, list, 1), children, fail)
	f.currentBlock = children
	elements := invoke(f, emitExtract(f, list, 0), unwrapsInterface, nil, pos)
	var length Call
	length.Call.Value = makeLen(elements.Type())
	length.Call.Args = []Value{elements}
	length.setType(tInt)
	n := f.emit(&length)
	index := emitLocal(f, tInt, pos, "index")
	emitStore(f, index, vZero, pos)
	next, body, advance := f.newBasicBlock("search.next"), f.newBasicBlock("search.child"), f.newBasicBlock("search.advance")
	emitJump(f, next)
	f.currentBlock = next
	i := emitLoad(f, index)
	emitIf(f, emitCompare(f, token.LSS, i, n, pos), body, fail)
	f.currentBlock = body
	address := &IndexAddr{X: elements, Index: i}
	address.setType(types.NewPointer(tError))
	address.setPos(pos)
	child := emitLoad(f, f.emit(address))
	descend := f.newBasicBlock("search.descend")
	emitIf(f, emitCompare(f, token.NEQ, child, zeroConst(tError), pos), descend, advance)
	f.currentBlock = descend
	recurse := &Call{Call: CallCommon{Value: f, Args: []Value{child}, pos: pos}}
	recurse.setType(results)
	recurse.setPos(pos)
	result := f.emit(recurse)
	descended := f.newBasicBlock("search.descended")
	emitIf(f, emitExtract(f, result, 1), descended, advance)
	f.currentBlock = descended
	ret(emitExtract(f, result, 0), vTrue)
	f.currentBlock = advance
	emitStore(f, index, emitArith(f, token.ADD, emitLoad(f, index), vOne, tInt, pos), pos)
	emitJump(f, next)

	f.currentBlock = fail
	ret(zeroConst(enum), vFalse)
	f.finishBody()
	if prog.mode&SanityCheckFunctions != 0 {
		mustSanityCheck(f, nil)
	}
	return f
}

// syntheticPackage returns the package that owns fn, including when fn is an
// instance of a generic function or a function nested in one.
func syntheticPackage(fn *Function) *Package {
	for f := fn; f != nil; f = f.parent {
		if f.Pkg != nil {
			return f.Pkg
		}
		if origin := f.topLevelOrigin; origin != nil && origin.Pkg != nil {
			return origin.Pkg
		}
	}
	return nil
}
