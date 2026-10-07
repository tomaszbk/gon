// Copyright (c) 2026 Gon contributors.
// Licensed under the Apache License, Version 2.0.

package function

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"runtime"
	"slices"
	"sort"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/guard"
	"go.uber.org/nilaway/hook"
	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/typeshelper"
	"golang.org/x/exp/typeparams"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"golang.org/x/tools/go/types/typeutil"
)

// Gon expressions can contain control flow, assignments, and early returns.
// The Go AST backpropagator's assumption that evaluating an expression cannot
// change the flow therefore does not apply. Use the maintained SSA builder's
// typed Gon lowering, then feed the same annotation triggers and inference
// engine as the Go frontend. In particular, a phi's edges are alternatives,
// not the multiple components of a Go tuple.
func usesGon(pass *analysishelper.EnhancedPass) bool {
	found := false
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if found {
				return false
			}
			switch n.(type) {
			case *ast.ErrorExpr, *ast.OptionalExpr,
				*ast.EnumType, *ast.MatchExpr, *ast.MatchStmt, *ast.PatternTestExpr,
				*ast.LambdaExpr, *ast.CondExpr, *ast.NilGuardExpr, *ast.SafeNavExpr,
				*ast.InterpolatedStringExpr:
				found = true
			case *ast.Ident:
				if object, ok := pass.TypesInfo.Uses[n.(*ast.Ident)].(*types.Func); ok && object.Pkg() != pass.Pkg {
					fact := new(gonPayloadFact)
					found = pass.ImportObjectFact(object.Origin(), fact) && len(fact.InterfaceParameters) > 0
				}
			case *ast.BinaryExpr:
				found = n.(*ast.BinaryExpr).Op == token.COALESCE
			case *ast.AssignStmt:
				found = n.(*ast.AssignStmt).Tok == token.COALESCE_ASSIGN
			case *ast.CallExpr:
				call := n.(*ast.CallExpr)
				found = len(call.ArgNames) != 0
				if !found {
					if object := typeutil.StaticCallee(pass.TypesInfo, call); object != nil && object.Pkg() != pass.Pkg {
						fact := new(gonPayloadFact)
						found = pass.ImportObjectFact(object.Origin(), fact) && len(fact.InterfaceParameters) > 0
					}
				}
			}
			return !found
		})
	}
	return found
}

type gonPoint struct {
	block *ssa.BasicBlock
	index int
}
type gonBinding struct {
	value   ssa.Value
	point   gonPoint
	context *gonContext
}
type gonContext struct{ bindings map[ssa.Value]gonBinding }
type gonOrigin struct {
	annotation annotation.ProducingAnnotationTrigger
	expr       ast.Expr
}
type gonTraceKey struct {
	value   ssa.Value
	path    string
	block   *ssa.BasicBlock
	index   int
	context *gonContext
}

type gonReachKey struct {
	block   *ssa.BasicBlock
	context *gonContext
}
type gonCallKey struct {
	call    *ssa.CallCommon
	context *gonContext
}

type gonFlow struct {
	pass        *analysishelper.EnhancedPass
	objects     map[*ssa.Function]*types.Func
	expressions map[ssa.Value]ast.Expr
	functions   []*ssa.Function
	triggers    []annotation.FullTrigger
	// Active recursive evaluations, rather than a global visited set: an origin
	// may reach several consumers, and each consumer has different branch guards.
	active         map[gonTraceKey]bool
	constantActive map[gonTraceKey]bool
	callActive     map[*ssa.Function]int
	scalarCache    map[gonTraceKey]constant.Value
	scalarKnown    map[gonTraceKey]bool
	reachableCache map[gonReachKey]bool
	reachableKnown map[gonReachKey]bool
	contexts       map[gonCallKey]*gonContext
	mutationActive map[gonCallKey]bool
	concreteCalls  map[*ssa.CallCommon]*ssa.CallCommon
	payloadFacts   map[*types.Func]*gonPayloadFact
}

func gonTriggers(pass *analysishelper.EnhancedPass) (triggers []annotation.FullTrigger, err error) {
	// Keep any assertions already generated if an unexpected builder/analysis
	// invariant fails. The accumulator reports the error at a real source position.
	var flow *gonFlow
	defer func() {
		if r := recover(); r != nil {
			if flow != nil {
				triggers = flow.triggers
			}
			err = fmt.Errorf("Gon NilAway SSA analysis: %v", r)
		}
	}()
	program := ssa.NewProgram(pass.Fset, ssa.GlobalDebug|ssa.InstantiateGenerics)
	var createImports func(*types.Package)
	createImports = func(pkg *types.Package) {
		if program.Package(pkg) != nil {
			return
		}
		for _, imported := range pkg.Imports() {
			createImports(imported)
		}
		program.CreatePackage(pkg, nil, nil, true)
	}
	for _, imported := range pass.Pkg.Imports() {
		createImports(imported)
	}
	pkg := program.CreatePackage(pass.Pkg, pass.Files, pass.TypesInfo, true)
	pkg.Build()
	flow = &gonFlow{pass: pass, objects: make(map[*ssa.Function]*types.Func),
		expressions: make(map[ssa.Value]ast.Expr), active: make(map[gonTraceKey]bool),
		constantActive: make(map[gonTraceKey]bool), callActive: make(map[*ssa.Function]int), scalarCache: make(map[gonTraceKey]constant.Value), scalarKnown: make(map[gonTraceKey]bool), reachableCache: make(map[gonReachKey]bool), reachableKnown: make(map[gonReachKey]bool), contexts: make(map[gonCallKey]*gonContext), mutationActive: make(map[gonCallKey]bool), concreteCalls: make(map[*ssa.CallCommon]*ssa.CallCommon)}
	for fn := range ssautil.AllFunctions(program) {
		if fn.Pkg != pkg || len(fn.Blocks) == 0 {
			continue
		}
		if fn.Synthetic == "error tree search" {
			// This checked lowering implements errors.AsType matching; analyze
			// the source subject and matched payload at its call, rather than
			// assigning source diagnostics to the generated traversal machinery.
			continue
		}
		if object, ok := fn.Object().(*types.Func); ok && object.Origin().Pkg() != pass.Pkg {
			// Instantiation wrappers for imported functions are implementation
			// artifacts, not source declarations in this analysis package.
			continue
		}
		flow.functions = append(flow.functions, fn)
		if object, ok := fn.Object().(*types.Func); ok {
			flow.objects[fn] = object.Origin()
		} else if fn.Pos().IsValid() {
			// Lambdas and function literals participate in the same parameter
			// constraints as declarations, without mutating shared TypesInfo.
			flow.objects[fn] = types.NewFunc(fn.Pos(), pass.Pkg, fn.Name(), fn.Signature)
		}
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if ref, ok := instruction.(*ssa.DebugRef); ok && ref.X != nil {
					flow.expressions[ref.X] = ref.Expr
				}
			}
		}
	}
	sort.Slice(flow.functions, func(i, j int) bool {
		if flow.functions[i].Pos() != flow.functions[j].Pos() {
			return flow.functions[i].Pos() < flow.functions[j].Pos()
		}
		return flow.functions[i].String() < flow.functions[j].String()
	})
	flow.exportPayloadFacts()
	for _, fn := range flow.functions {
		// Method-expression thunks have a leading receiver parameter while
		// their object has the declared method signature. Their source method
		// is analyzed below; the thunk is only call-lowering machinery.
		if fn.Synthetic != "" && fn.Syntax() == nil {
			continue
		}
		flow.function(fn)
	}
	// SSA can expose several storage loads for one source dereference (notably
	// nested optional pattern bindings). Preserve distinct source sites while
	// coalescing the identical inference constraint at the same site.
	seen := make(map[string]bool)
	unique := flow.triggers[:0]
	for _, trigger := range flow.triggers {
		producer := fmt.Sprint(trigger.Producer.Annotation.Repr())
		if trigger.Producer.Annotation.Kind() == annotation.Always {
			producer = "always"
		}
		key := fmt.Sprint(trigger.Consumer.Expr.Pos(), ":", producer, ":", trigger.Consumer.Annotation.Repr())
		if site := trigger.Producer.Annotation.UnderlyingSite(); site != nil {
			key += ":" + site.String()
		}
		if site := trigger.Consumer.Annotation.UnderlyingSite(); site != nil {
			key += ":" + site.String()
		}
		if !seen[key] {
			seen[key] = true
			unique = append(unique, trigger)
		}
	}
	return unique, nil
}

func (f *gonFlow) expr(value ssa.Value, pos token.Pos) ast.Expr {
	if expr := f.expressions[value]; expr != nil {
		return expr
	}
	if !pos.IsValid() && value != nil {
		pos = value.Pos()
	}
	if !pos.IsValid() {
		pos = f.pass.Files[0].Pos()
	}
	name := "value"
	if value != nil && value.Name() != "" {
		name = value.Name()
	}
	return &ast.Ident{NamePos: pos, Name: name}
}

func (f *gonFlow) object(fn *ssa.Function) *types.Func {
	if object := f.objects[fn]; object != nil {
		return object
	}
	if object, ok := fn.Object().(*types.Func); ok {
		return object.Origin()
	}
	return nil
}

func (f *gonFlow) function(fn *ssa.Function) {
	context := &gonContext{bindings: make(map[ssa.Value]gonBinding)}
	for _, block := range fn.Blocks {
		if !f.reachable(block, context) {
			continue
		}
	instructions:
		for index, instruction := range block.Instrs {
			point := gonPoint{block, index}
			switch ins := instruction.(type) {
			case *ssa.UnOp:
				if ins.Op == token.MUL {
					f.consume(ins.X, nil, point, context, &annotation.PtrLoad{ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{}}, ins.Pos())
				}
			case *ssa.FieldAddr:
				f.consume(ins.X, nil, point, context, &annotation.PtrLoad{ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{}}, ins.Pos())
			case *ssa.IndexAddr:
				if _, ok := ins.X.Type().Underlying().(*types.Pointer); ok {
					f.consume(ins.X, nil, point, context, &annotation.PtrLoad{ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{}}, ins.Pos())
				}
			case *ssa.MapUpdate:
				f.consume(ins.Map, nil, point, context, &annotation.PtrLoad{ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{}}, ins.Pos())
			case *ssa.Slice:
				if _, ok := ins.X.Type().Underlying().(*types.Pointer); ok {
					f.consume(ins.X, nil, point, context, &annotation.PtrLoad{ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{}}, ins.Pos())
				}
			case *ssa.Store:
				f.consume(ins.Addr, nil, point, context, &annotation.PtrLoad{ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{}}, ins.Pos())
				if address, ok := ins.Addr.(*ssa.FieldAddr); ok {
					_, field := gonProjection(address.X.Type().Underlying().(*types.Pointer).Elem(), []int{address.Field})
					if field != nil && types.EnumStorageOf(address.X.Type().Underlying().(*types.Pointer).Elem()) == nil {
						f.consume(ins.Val, nil, point, context, &annotation.FldAssign{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: &annotation.FieldAnnotationKey{FieldDecl: field}}}, ins.Pos())
					}
				}
			case *ssa.Call:
				f.call(&ins.Call, point, context, ins.Pos())
				if f.noReturn(&ins.Call) {
					break instructions
				}
			case *ssa.Go:
				f.call(&ins.Call, point, context, ins.Pos())
			case *ssa.Defer:
				f.call(&ins.Call, point, context, ins.Pos())
			case *ssa.Return:
				object := f.object(fn)
				if object == nil {
					continue
				}
				for i, value := range ins.Results {
					// Nil success components on an error return are partial results,
					// not the successful protocol a checked call consumes.
					if i < len(ins.Results)-1 && typeshelper.FuncIsErrReturning(fn.Signature) && f.definitelyNonNil(ins.Results[len(ins.Results)-1], point, context) {
						continue
					}
					key := annotation.RetKeyFromRetNum(object, i)
					f.consume(value, nil, point, context, &annotation.UseAsReturn{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: key}, RetStmt: &ast.ReturnStmt{Return: ins.Pos()}}, ins.Pos())
					if typeshelper.IsDeep(value.Type()) {
						f.consumeElements(value, point, context, &annotation.UseAsReturnDeep{TriggerIfDeepNonNil: &annotation.TriggerIfDeepNonNil{Ann: key}, RetStmt: &ast.ReturnStmt{Return: ins.Pos()}}, ins.Pos())
					}
					for _, projection := range gonPayloads(value.Type(), nil) {
						if !f.present(value, projection.path, point, context) {
							continue
						}
						key := &annotation.RetFieldAnnotationKey{FuncDecl: object, RetNum: i, FieldDecl: projection.field}
						f.consume(value, projection.path, point, context, &annotation.UseAsFldOfReturn{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: key}}, ins.Pos())
					}
				}
			}
		}
	}
}

func (f *gonFlow) consumeElements(value ssa.Value, point gonPoint, context *gonContext, consumer annotation.ConsumingAnnotationTrigger, pos token.Pos) {
	consumer.SetNeedsGuard(false)
	for _, origin := range f.elementOrigins(value, nil, point, context) {
		f.triggers = append(f.triggers, annotation.FullTrigger{Producer: &annotation.ProduceTrigger{Annotation: origin.annotation, Expr: origin.expr}, Consumer: &annotation.ConsumeTrigger{Annotation: consumer.Copy(), Expr: f.expr(value, pos), Guards: guard.NoGuards(), GuardMatched: true}})
	}
}

func (f *gonFlow) consume(value ssa.Value, path []int, point gonPoint, context *gonContext, consumer annotation.ConsumingAnnotationTrigger, pos token.Pos) {
	consumer.SetNeedsGuard(false)
	for _, origin := range f.origins(value, path, point, context) {
		f.triggers = append(f.triggers, annotation.FullTrigger{
			Producer: &annotation.ProduceTrigger{Annotation: origin.annotation, Expr: origin.expr},
			Consumer: &annotation.ConsumeTrigger{Annotation: consumer.Copy(), Expr: f.expr(value, pos), Guards: guard.NoGuards(), GuardMatched: true},
		})
	}
}

func (f *gonFlow) call(call *ssa.CallCommon, point gonPoint, context *gonContext, pos token.Pos) {
	if call.IsInvoke() {
		f.consume(call.Value, nil, point, context, &annotation.PtrLoad{ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{}}, pos)
	}
	callee := call.StaticCallee()
	if callee == nil && !call.IsInvoke() {
		if _, builtin := call.Value.(*ssa.Builtin); !builtin {
			f.consume(call.Value, nil, point, context, &annotation.PtrLoad{ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{}}, pos)
		}
		return
	}
	object := call.Method
	if callee != nil {
		object = f.object(callee)
	}
	if object == nil {
		return
	}
	for i, value := range call.Args {
		param := i
		var key annotation.Key
		if callee != nil && object.Signature().Recv() != nil {
			if i == 0 {
				key = &annotation.RecvAnnotationKey{FuncDecl: object}
				param = annotation.ReceiverParamIndex
			} else {
				param--
			}
		}
		if key == nil {
			key = annotation.ParamKeyFromArgNum(object, param)
		}
		var consumer annotation.ConsumingAnnotationTrigger = &annotation.ArgPass{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: key}}
		if param == annotation.ReceiverParamIndex {
			consumer = &annotation.RecvPass{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: key}}
		}
		f.consume(value, nil, point, context, consumer, pos)
		if _, isInterface := value.Type().Underlying().(*types.Interface); isInterface {
			deep := &annotation.ArgPassDeep{TriggerIfDeepNonNil: &annotation.TriggerIfDeepNonNil{Ann: key}}
			deep.SetNeedsGuard(false)
			for _, origin := range f.assertedOrigins(value, nil, point, context) {
				f.triggers = append(f.triggers, annotation.FullTrigger{Producer: &annotation.ProduceTrigger{Annotation: origin.annotation, Expr: origin.expr}, Consumer: &annotation.ConsumeTrigger{Annotation: deep.Copy(), Expr: f.expr(value, pos), Guards: guard.NoGuards(), GuardMatched: true}})
			}
		}
		if param != annotation.ReceiverParamIndex && typeshelper.IsDeep(value.Type()) {
			consumer := &annotation.ArgPassDeep{TriggerIfDeepNonNil: &annotation.TriggerIfDeepNonNil{Ann: key}}
			f.consumeElements(value, point, context, consumer, pos)
		}
		for _, projection := range gonPayloads(value.Type(), nil) {
			if !f.present(value, projection.path, point, context) {
				continue
			}
			key := &annotation.ParamFieldAnnotationKey{FuncDecl: object, ParamNum: param, FieldDecl: projection.field}
			f.consume(value, projection.path, point, context, &annotation.ArgFldPass{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: key}}, pos)
		}
	}
}

func (f *gonFlow) origins(value ssa.Value, path []int, point gonPoint, context *gonContext) []gonOrigin {
	if value == nil {
		return nil
	}
	typ, field := gonProjection(value.Type(), path)
	if typ == nil || !gonNilable(typ) {
		return nil
	}
	if len(path) == 0 && f.guarded(value, point, context) {
		return nil
	}
	key := gonTraceKey{value, fmt.Sprint(path), point.block, point.index, context}
	if f.active[key] {
		return nil
	}
	f.active[key] = true
	defer delete(f.active, key)
	if binding, ok := context.bindings[value]; ok {
		return f.origins(binding.value, path, binding.point, binding.context)
	}
	trace := func(v ssa.Value, p []int) []gonOrigin { return f.origins(v, p, point, context) }
	unknown := func() []gonOrigin {
		return []gonOrigin{{&annotation.ProduceTriggerTautology{}, f.expr(value, value.Pos())}}
	}
	switch v := value.(type) {
	case *ssa.Const:
		if v.IsNil() {
			return []gonOrigin{{&annotation.ConstNil{ProduceTriggerTautology: &annotation.ProduceTriggerTautology{}}, f.expr(v, point.block.Parent().Pos())}}
		}
		return nil
	case *ssa.Alloc, *ssa.Global, *ssa.MakeMap, *ssa.MakeChan, *ssa.MakeSlice, *ssa.Function, *ssa.MakeClosure:
		return nil
	case *ssa.Parameter:
		object := f.object(v.Parent())
		if object == nil {
			return unknown()
		}
		index := slices.Index(v.Parent().Params, v)
		if object.Signature().Recv() != nil {
			index--
		}
		var ann annotation.ProducingAnnotationTrigger
		if len(path) > 0 && field != nil {
			if types.EnumStorageOf(v.Type()) != nil {
				ann = &annotation.ParamFldRead{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: &annotation.ParamFieldAnnotationKey{FuncDecl: object, ParamNum: index, FieldDecl: field}}}
			} else {
				ann = &annotation.FldRead{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: &annotation.FieldAnnotationKey{FieldDecl: field}}}
			}
		} else if index < 0 {
			ann = &annotation.MethodRecv{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: &annotation.RecvAnnotationKey{FuncDecl: object}}, VarDecl: object.Signature().Recv()}
		} else {
			ann = annotation.ParamAsProducer(object, object.Signature().Params().At(index))
		}
		return []gonOrigin{{ann, f.expr(v, v.Pos())}}
	case *ssa.FreeVar:
		// Capture addresses are resolved at each closure creation in the outer
		// function, including typed-nil payloads and mutations before invocation.
		var out []gonOrigin
		for _, fn := range f.functions {
			for _, block := range fn.Blocks {
				for i, ins := range block.Instrs {
					closure, ok := ins.(*ssa.MakeClosure)
					if !ok || closure.Fn != v.Parent() {
						continue
					}
					index := slices.Index(v.Parent().FreeVars, v)
					out = append(out, f.origins(closure.Bindings[index], path, gonPoint{block, i}, context)...)
				}
			}
		}
		return out
	case *ssa.Phi:
		var out []gonOrigin
		for i, edge := range v.Edges {
			pred := v.Block().Preds[i]
			if !f.reachable(pred, context) || !f.phiEdgePossible(v, i, point, context) {
				continue
			}
			out = append(out, f.origins(edge, path, gonPoint{pred, len(pred.Instrs)}, context)...)
		}
		return out
	case *ssa.ChangeType:
		return trace(v.X, path)
	case *ssa.Convert:
		return trace(v.X, path)
	case *ssa.ChangeInterface:
		return trace(v.X, path)
	case *ssa.MakeInterface:
		// A boxed typed nil is a nonnil interface. Dereferencing its payload
		// after a type assertion is handled by TypeAssert below.
		return nil
	case *ssa.Slice:
		return trace(v.X, path)
	case *ssa.Field:
		return trace(v.X, append([]int{v.Field}, path...))
	case *ssa.FieldAddr, *ssa.IndexAddr:
		return nil // address formation is a consumption, not a nil result
	case *ssa.UnOp:
		if v.Op == token.MUL {
			return f.load(v.X, path, gonPoint{v.Block(), instructionIndex(v)}, context)
		}
		if v.Op == token.ARROW {
			return f.receiveOrigins(v, path, point, context)
		}
	case *ssa.Extract:
		if call, ok := v.Tuple.(*ssa.Call); ok {
			return f.callOrigins(&call.Call, v.Index, path, point, context, v)
		}
		if assertion, ok := v.Tuple.(*ssa.TypeAssert); ok && v.Index == 0 {
			return f.assertedOrigins(assertion.X, path, point, context)
		}
		if lookup, ok := v.Tuple.(*ssa.Lookup); ok && v.Index == 0 {
			return f.lookupOrigins(lookup, path, point, context)
		}
		if next, ok := v.Tuple.(*ssa.Next); ok && v.Index == 2 {
			if iter, ok := next.Iter.(*ssa.Range); ok {
				return f.elementOrigins(iter.X, path, point, context)
			}
		}
		if recv, ok := v.Tuple.(*ssa.UnOp); ok && recv.Op == token.ARROW && v.Index == 0 {
			return f.receiveOrigins(recv, path, point, context)
		}
	case *ssa.Call:
		return f.callOrigins(&v.Call, 0, path, point, context, v)
	case *ssa.TypeAssert:
		return f.assertedOrigins(v.X, path, point, context)
	case *ssa.Lookup:
		return f.lookupOrigins(v, path, point, context)
	case *ssa.Index:
		return f.elementOrigins(v.X, path, point, context)
	}
	return unknown()
}

func instructionIndex(ins ssa.Instruction) int { return slices.Index(ins.Block().Instrs, ins) }

// A receive from a channel closed locally can yield the element's zero value.
// A successful comma-ok receive excludes that zero while preserving nil values
// that were explicitly sent to the channel.
func (f *gonFlow) receiveOrigins(recv *ssa.UnOp, path []int, point gonPoint, context *gonContext) []gonOrigin {
	out := f.elementOrigins(recv.X, path, point, context)
	checked := false
	if recv.CommaOk {
		for block := point.block; block != nil; block = block.Idom() {
			for _, pred := range block.Preds {
				if !pred.Dominates(point.block) || len(pred.Instrs) == 0 {
					continue
				}
				branch, ok := pred.Instrs[len(pred.Instrs)-1].(*ssa.If)
				if !ok {
					continue
				}
				if okValue, ok := branch.Cond.(*ssa.Extract); ok && okValue.Tuple == recv && okValue.Index == 1 && pred.Succs[0].Dominates(point.block) {
					checked = true
				}
			}
		}
	}
	read := gonPoint{recv.Block(), instructionIndex(recv)}
	if !checked && f.closedBefore(recv.X, read, context) && !f.bufferedBefore(recv.X, read, context) {
		out = append(out, gonOrigin{&annotation.ConstNil{ProduceTriggerTautology: &annotation.ProduceTriggerTautology{}}, f.expr(recv, recv.Pos())})
	}
	return out
}

func (f *gonFlow) closedBefore(channel ssa.Value, point gonPoint, context *gonContext) bool {
	key := gonTraceKey{channel, "closed", point.block, point.index, context}
	if f.active[key] {
		return true
	}
	f.active[key] = true
	defer delete(f.active, key)
	if call, ok := channel.(*ssa.Call); ok {
		callee := call.Call.StaticCallee()
		if callee == nil || len(callee.Blocks) == 0 {
			return true
		}
		child := f.callContext(&call.Call, gonPoint{call.Block(), instructionIndex(call)}, context)
		for _, block := range callee.Blocks {
			for i, ins := range block.Instrs {
				if ret, ok := ins.(*ssa.Return); ok && len(ret.Results) > 0 && f.closedBefore(ret.Results[0], gonPoint{block, i}, child) {
					return true
				}
			}
		}
	}
	if param, ok := channel.(*ssa.Parameter); ok {
		if _, bound := context.bindings[channel]; !bound {
			for _, fn := range f.functions {
				for _, block := range fn.Blocks {
					for i, ins := range block.Instrs {
						if call, ok := ins.(*ssa.Call); ok && call.Call.StaticCallee() == param.Parent() {
							index := slices.Index(param.Parent().Params, param)
							if index < len(call.Call.Args) && f.closedBefore(call.Call.Args[index], gonPoint{block, i}, context) {
								return true
							}
						}
					}
				}
			}
		}
	}
	ancestors := make(map[*ssa.BasicBlock]bool)
	work := []*ssa.BasicBlock{point.block}
	for len(work) > 0 {
		block := work[len(work)-1]
		work = work[:len(work)-1]
		if !ancestors[block] {
			ancestors[block] = true
			work = append(work, block.Preds...)
		}
	}
	for _, block := range point.block.Parent().Blocks {
		if !ancestors[block] {
			continue
		}
		if !f.reachable(block, context) {
			continue
		}
		for i, ins := range block.Instrs {
			if block == point.block && i >= point.index {
				break
			}
			if call, ok := ins.(*ssa.Call); ok {
				if builtin, ok := call.Call.Value.(*ssa.Builtin); ok && builtin.Name() == "close" && len(call.Call.Args) == 1 && f.sameValue(call.Call.Args[0], channel, context) {
					return true
				}
				if f.callMayClose(&call.Call, channel, gonPoint{block, i}, context, make(map[*ssa.Function]bool)) {
					return true
				}
			}
			switch ins := ins.(type) {
			case *ssa.Go:
				if f.channelPassed(&ins.Call, channel, context) {
					return true
				}
			case *ssa.Store:
				if ins.Val == channel {
					if _, field := ins.Addr.(*ssa.FieldAddr); field {
						return true
					}
					if _, global := ins.Addr.(*ssa.Global); global {
						return true
					}
				}
			}

		}
	}
	if binding, ok := context.bindings[channel]; ok {
		return f.closedBefore(binding.value, binding.point, binding.context)
	}
	return false
}

func (f *gonFlow) callMayClose(call *ssa.CallCommon, channel ssa.Value, point gonPoint, context *gonContext, seen map[*ssa.Function]bool) bool {
	if !f.channelPassed(call, channel, context) {
		return false
	}
	if builtin, ok := call.Value.(*ssa.Builtin); ok {
		return builtin.Name() == "close"
	}
	callee := call.StaticCallee()
	if callee == nil || len(callee.Blocks) == 0 || seen[callee] {
		return true
	}
	seen[callee] = true
	defer delete(seen, callee)
	child := f.callContext(call, point, context)
	for _, block := range callee.Blocks {
		for i, ins := range block.Instrs {
			if nested, ok := ins.(*ssa.Call); ok && f.callMayClose(&nested.Call, channel, gonPoint{block, i}, child, seen) {
				return true
			}
		}
	}
	return false
}

func (f *gonFlow) channelPassed(call *ssa.CallCommon, channel ssa.Value, context *gonContext) bool {
	args := slices.Clone(call.Args)
	if closure, ok := call.Value.(*ssa.MakeClosure); ok {
		args = append(args, closure.Bindings...)
	}
	for _, arg := range args {
		if boxed, ok := arg.(*ssa.MakeInterface); ok {
			arg = boxed.X
		}
		a, ap := f.address(arg, context)
		b, bp := f.address(channel, context)
		if a == b && slices.Equal(ap, bp) {
			return true
		}
		if pointer, ok := arg.Type().Underlying().(*types.Pointer); ok {
			if _, chanPointer := pointer.Elem().Underlying().(*types.Chan); chanPointer {
				return true
			}
		}
	}
	return false
}

// In a straight-line local block, a buffered channel retains values sent before
// close. The zero value cannot be received while that known queue is nonempty.
func (f *gonFlow) bufferedBefore(channel ssa.Value, point gonPoint, context *gonContext) bool {
	if binding, ok := context.bindings[channel]; ok {
		_ = binding
		return false // a callee may have already drained the caller's queue
	}
	made, ok := channel.(*ssa.MakeChan)
	if !ok || made.Block() != point.block {
		return false
	}
	capacity := f.scalar(made.Size, nil, point, context)
	if capacity == nil || constant.Sign(capacity) <= 0 {
		return false
	}
	count := 0
	for _, ins := range point.block.Instrs[instructionIndex(made)+1 : point.index] {
		switch ins := ins.(type) {
		case *ssa.Send:
			if ins.Chan == channel {
				count++
			}
		case *ssa.UnOp:
			if ins.Op == token.ARROW && ins.X == channel {
				count--
			}
		case *ssa.Call:
			if slices.Contains(ins.Call.Args, channel) {
				if builtin, ok := ins.Call.Value.(*ssa.Builtin); !ok || builtin.Name() != "close" {
					return false
				}
			}
		case *ssa.Go:
			if slices.Contains(ins.Call.Args, channel) {
				return false
			}
		case *ssa.Defer:
			if slices.Contains(ins.Call.Args, channel) {
				return false
			}
		case *ssa.Select:
			for _, state := range ins.States {
				if state.Chan == channel {
					return false
				}
			}
		case *ssa.MakeClosure:
			if slices.Contains(ins.Bindings, channel) {
				return false
			}
		case *ssa.Store:
			if ins.Val == channel {
				return false
			}
		case *ssa.MakeInterface:
			if ins.X == channel {
				return false
			}
		}
	}
	return count > 0
}

// Pattern tests merge both a binding and a boolean result. The zero binding on
// the false edge cannot reach a consumer dominated by the test's true branch.
// Keep their phi edges correlated instead of treating the two phis independently.
func (f *gonFlow) phiEdgePossible(phi *ssa.Phi, edge int, point gonPoint, context *gonContext) bool {
	block := phi.Block()
	if !f.possibleEdge(block.Preds[edge], block, context) {
		return false
	}
	if len(block.Instrs) == 0 || !block.Dominates(point.block) {
		return true
	}
	branch, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
	if !ok {
		return true
	}
	expected := -1
	for index, successor := range block.Succs {
		if successor.Dominates(point.block) {
			expected = index
			break
		}
	}
	if expected < 0 {
		return true
	}
	child := &gonContext{bindings: make(map[ssa.Value]gonBinding, len(context.bindings))}
	for value, binding := range context.bindings {
		child.bindings[value] = binding
	}
	pred := block.Preds[edge]
	for _, instruction := range block.Instrs {
		if companion, ok := instruction.(*ssa.Phi); ok {
			child.bindings[companion] = gonBinding{companion.Edges[edge], gonPoint{pred, len(pred.Instrs)}, context}
		}
	}
	condition := f.scalar(branch.Cond, nil, gonPoint{block, len(block.Instrs) - 1}, child)
	return condition == nil || condition.Kind() != constant.Bool || constant.BoolVal(condition) == (expected == 0)
}

func (f *gonFlow) callOrigins(call *ssa.CallCommon, result int, path []int, point gonPoint, context *gonContext, value ssa.Value) []gonOrigin {
	if concrete := f.concreteCall(call); concrete != nil {
		call = concrete
	}
	callee := call.StaticCallee()
	if callee != nil && callee.Synthetic == "error tree search" && result == 0 {
		if nilValue, ok := call.Args[0].(*ssa.Const); ok && nilValue.IsNil() {
			return nil
		}
		return f.assertedOrigins(call.Args[0], path, point, context)
	}
	if callee != nil && len(path) > 0 {
		if object := f.object(callee); object != nil {
			fact := f.payloadFacts[object]
			if fact == nil {
				fact = new(gonPayloadFact)
				if !f.pass.ImportObjectFact(object, fact) {
					fact = nil
				}
			}
			if fact != nil {
				for _, identity := range fact.Identities {
					if identity.Result == result && slices.Equal(identity.OutputPath, path) && identity.Parameter < len(call.Args) {
						argument := call.Args[identity.Parameter]
						if len(identity.InputPath) > 0 && !f.present(argument, identity.InputPath, point, context) {
							return nil
						}
						return f.origins(argument, identity.InputPath, point, context)
					}
				}
			}
		}
	}
	if callee != nil && len(path) > 0 {
		if object := f.object(callee); object != nil && object.Pkg() != nil && object.Pkg().Path() == "gon/seq" && filepath.Dir(f.pass.Fset.Position(object.Pos()).Filename) == filepath.Join(runtime.GOROOT(), "src", "gon", "seq") {
			switch object.Name() {
			case "Find", "First", "Last", "At", "Lookup":
				return f.elementOrigins(call.Args[0], nil, point, context)
			}
		}
	}
	if callee != nil && f.callActive[callee] == 0 && len(callee.Blocks) > 0 && f.object(callee) != nil && f.object(callee).Pkg() == f.pass.Pkg {
		f.callActive[callee]++
		defer func() { f.callActive[callee]-- }()
		child := f.callContext(call, point, context)
		var out []gonOrigin
		for _, block := range callee.Blocks {
			if !f.reachable(block, child) {
				continue
			}
			for i, ins := range block.Instrs {
				if ret, ok := ins.(*ssa.Return); ok && result < len(ret.Results) {
					if f.exitsBefore(block, i) {
						continue
					}
					// A successful !/or edge excludes callee returns with nonnil error.
					if result < len(ret.Results)-1 && typeshelper.FuncIsErrReturning(callee.Signature) && f.callSucceeded(call, point) && f.definitelyNonNil(ret.Results[len(ret.Results)-1], gonPoint{block, i}, child) {
						continue
					}
					if len(path) > 0 && !f.present(ret.Results[result], path, gonPoint{block, i}, child) {
						continue
					}
					out = append(out, f.origins(ret.Results[result], path, gonPoint{block, i}, child)...)
				}
			}
		}
		return out
	}
	if callee != nil || call.IsInvoke() {
		object := call.Method
		if callee != nil {
			object = f.object(callee)
		}
		if object != nil {
			var ann annotation.ProducingAnnotationTrigger
			_, field := gonProjection(value.Type(), path)
			if len(path) > 0 && field != nil {
				if types.EnumStorageOf(value.Type()) != nil {
					ann = &annotation.FldReturn{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: &annotation.RetFieldAnnotationKey{FuncDecl: object, RetNum: result, FieldDecl: field}}}
				} else {
					ann = &annotation.FldRead{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: &annotation.FieldAnnotationKey{FieldDecl: field}}}
				}
			} else {
				ann = &annotation.FuncReturn{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: annotation.RetKeyFromRetNum(object, result)}}
			}
			return []gonOrigin{{ann, f.expr(value, value.Pos())}}
		}
	}
	if builtin, ok := call.Value.(*ssa.Builtin); ok && (builtin.Name() == "new" || builtin.Name() == "make" || builtin.Name() == "append") {
		return nil
	}
	return []gonOrigin{{&annotation.ProduceTriggerTautology{}, f.expr(value, value.Pos())}}
}

func (f *gonFlow) assertedOrigins(value ssa.Value, path []int, point gonPoint, context *gonContext) []gonOrigin {
	if binding, ok := context.bindings[value]; ok {
		return f.assertedOrigins(binding.value, path, binding.point, binding.context)
	}
	switch v := value.(type) {
	case *ssa.MakeInterface:
		return f.origins(v.X, path, point, context)
	case *ssa.ChangeInterface:
		return f.assertedOrigins(v.X, path, point, context)
	case *ssa.Phi:
		var out []gonOrigin
		for i, e := range v.Edges {
			pred := v.Block().Preds[i]
			out = append(out, f.assertedOrigins(e, path, gonPoint{pred, len(pred.Instrs)}, context)...)
		}
		return out
	case *ssa.Parameter:
		if object := f.object(v.Parent()); object != nil {
			index := slices.Index(v.Parent().Params, v)
			if object.Signature().Recv() != nil {
				index--
			}
			if index >= 0 {
				return []gonOrigin{{&annotation.FuncParamDeep{TriggerIfDeepNilable: &annotation.TriggerIfDeepNilable{Ann: annotation.ParamKeyFromArgNum(object, index)}}, f.expr(v, v.Pos())}}
			}
			return []gonOrigin{{&annotation.MethodRecvDeep{TriggerIfDeepNilable: &annotation.TriggerIfDeepNilable{Ann: &annotation.RecvAnnotationKey{FuncDecl: object}}, VarDecl: object.Signature().Recv()}, f.expr(v, v.Pos())}}
		}
	case *ssa.Const:
		if v.IsNil() {
			return nil // nil interfaces cannot satisfy a non-interface type assertion
		}
	}
	return []gonOrigin{{&annotation.ProduceTriggerTautology{}, f.expr(value, value.Pos())}}
}

// Container results use deep return annotations across packages and retain
// their actual element producers for local calls, including Go tuples consumed
// by Gon error propagation.
func (f *gonFlow) callElementOrigins(call *ssa.CallCommon, result int, path []int, point gonPoint, context *gonContext, value ssa.Value) []gonOrigin {
	if concrete := f.concreteCall(call); concrete != nil {
		call = concrete
	}
	callee := call.StaticCallee()
	if callee != nil && f.callActive[callee] == 0 && len(callee.Blocks) > 0 && f.object(callee) != nil && f.object(callee).Pkg() == f.pass.Pkg {
		f.callActive[callee]++
		defer func() { f.callActive[callee]-- }()
		child := f.callContext(call, point, context)
		var out []gonOrigin
		for _, block := range callee.Blocks {
			if !f.reachable(block, child) {
				continue
			}
			for i, ins := range block.Instrs {
				if ret, ok := ins.(*ssa.Return); ok && result < len(ret.Results) {
					if result < len(ret.Results)-1 && typeshelper.FuncIsErrReturning(callee.Signature) && f.callSucceeded(call, point) && f.definitelyNonNil(ret.Results[len(ret.Results)-1], gonPoint{block, i}, child) {
						continue
					}
					out = append(out, f.elementOrigins(ret.Results[result], path, gonPoint{block, i}, child)...)
				}
			}
		}
		return out
	}
	object := call.Method
	if callee != nil {
		object = f.object(callee)
	}
	if object != nil {
		return []gonOrigin{{annotation.DeepNilabilityOfFuncRet(object, result), f.expr(value, value.Pos())}}
	}
	return []gonOrigin{{&annotation.ProduceTriggerTautology{}, f.expr(value, value.Pos())}}
}

// Elements and projected payloads share NilAway's deep annotation protocol.
func (f *gonFlow) elementOrigins(value ssa.Value, path []int, point gonPoint, context *gonContext) []gonOrigin {
	if value == nil {
		return nil
	}
	// Loop-carried slices (for example xs = append(xs, p)) contain cyclic
	// SSA phis. Follow each producer once per use while retaining the other
	// incoming edges, rather than recursively expanding the loop forever.
	key := gonTraceKey{value, "elements:" + fmt.Sprint(path), point.block, point.index, context}
	if f.active[key] {
		return nil
	}
	f.active[key] = true
	defer delete(f.active, key)
	if binding, ok := context.bindings[value]; ok {
		return f.elementOrigins(binding.value, path, binding.point, binding.context)
	}
	if slice, ok := value.(*ssa.Slice); ok {
		value = slice.X
	}
	if phi, ok := value.(*ssa.Phi); ok {
		var out []gonOrigin
		for i, edge := range phi.Edges {
			pred := phi.Block().Preds[i]
			if f.possibleEdge(pred, phi.Block(), context) {
				out = append(out, f.elementOrigins(edge, path, gonPoint{pred, len(pred.Instrs)}, context)...)
			}
		}
		return out
	}
	if call, ok := value.(*ssa.Call); ok {
		if builtin, ok := call.Call.Value.(*ssa.Builtin); ok && builtin.Name() == "append" {
			var out []gonOrigin
			for _, arg := range call.Call.Args {
				out = append(out, f.elementOrigins(arg, path, point, context)...)
			}
			return out
		}
		return f.callElementOrigins(&call.Call, 0, path, point, context, value)
	}
	if extract, ok := value.(*ssa.Extract); ok {
		if call, ok := extract.Tuple.(*ssa.Call); ok {
			return f.callElementOrigins(&call.Call, extract.Index, path, point, context, value)
		}
	}
	if loaded, ok := value.(*ssa.UnOp); ok && loaded.Op == token.MUL {
		base, fieldPath := gonAddress(loaded.X)
		if pointer, ok := base.Type().Underlying().(*types.Pointer); ok && len(fieldPath) > 0 && types.EnumStorageOf(pointer.Elem()) == nil {
			_, field := gonProjection(pointer.Elem(), fieldPath)
			if field != nil {
				return []gonOrigin{{annotation.DeepNilabilityOfFld(field), f.expr(value, value.Pos())}}
			}
		}
		if global, ok := loaded.X.(*ssa.Global); ok {
			if object, ok := global.Object().(*types.Var); ok {
				return []gonOrigin{{annotation.DeepNilabilityOfVar(nil, object), f.expr(value, value.Pos())}}
			}
		}
	}
	if field, ok := value.(*ssa.Field); ok && types.EnumStorageOf(field.X.Type()) == nil {
		_, object := gonProjection(field.X.Type(), []int{field.Field})
		if object != nil {
			return []gonOrigin{{annotation.DeepNilabilityOfFld(object), f.expr(value, value.Pos())}}
		}
	}
	if param, ok := value.(*ssa.Parameter); ok {
		if object := f.object(param.Parent()); object != nil {
			index := slices.Index(param.Parent().Params, param)
			if object.Signature().Recv() != nil {
				index--
			}
			if index >= 0 {
				return []gonOrigin{{annotation.DeepNilabilityOfVar(object, object.Signature().Params().At(index)), f.expr(param, param.Pos())}}
			}
			return []gonOrigin{{annotation.DeepNilabilityOfVar(object, object.Signature().Recv()), f.expr(param, param.Pos())}}
		}
	}
	// Locally created arrays/slices and maps retain their actual element
	// producers. Nil containers cannot yield a present gon/seq lookup result.
	if _, ok := value.(*ssa.Const); ok {
		return nil
	}
	base, _ := gonAddress(value)
	var out []gonOrigin
	indices := make(map[int64]bool)
	for _, block := range point.block.Parent().Blocks {
		if !f.reachable(block, context) {
			continue
		}
		for i, instruction := range block.Instrs {
			if block == point.block && i >= point.index {
				break
			}
			switch ins := instruction.(type) {
			case *ssa.Send:
				if ins.Chan == value {
					out = append(out, f.origins(ins.X, path, gonPoint{block, i}, context)...)
				}
			case *ssa.MapUpdate:
				if ins.Map == value {
					out = append(out, f.origins(ins.Value, path, gonPoint{block, i}, context)...)
				}
			case *ssa.Store:
				if address, ok := ins.Addr.(*ssa.IndexAddr); ok {
					addressBase, _ := gonAddress(address.X)
					if addressBase == base {
						out = append(out, f.origins(ins.Val, path, gonPoint{block, i}, context)...)
						if index, ok := address.Index.(*ssa.Const); ok {
							n, _ := constant.Int64Val(index.Value)
							indices[n] = true
						}
					}
				}
			}
		}
	}
	if _, ok := value.(*ssa.MakeMap); ok {
		return out
	}
	if _, ok := value.(*ssa.MakeChan); ok {
		return out
	}
	if alloc, ok := base.(*ssa.Alloc); ok {
		if array, ok := alloc.Type().Underlying().(*types.Pointer).Elem().Underlying().(*types.Array); ok {
			if int64(len(indices)) < array.Len() && gonNilable(array.Elem()) {
				out = append(out, gonOrigin{&annotation.ConstNil{ProduceTriggerTautology: &annotation.ProduceTriggerTautology{}}, f.expr(value, value.Pos())})
			}
			return out
		}
	}
	return []gonOrigin{{&annotation.ProduceTriggerTautology{}, f.expr(value, value.Pos())}}
}

func gonStruct(t types.Type) *types.Struct {
	if optional := types.OptionalStorage(t); optional != nil {
		return optional
	}
	str, _ := t.Underlying().(*types.Struct)
	return str
}
func gonProjection(t types.Type, path []int) (types.Type, *types.Var) {
	var field *types.Var
	for _, index := range path {
		if index < 0 {
			switch array := t.Underlying().(type) {
			case *types.Array:
				t = array.Elem()
				continue
			case *types.Slice:
				t = array.Elem()
				continue
			}
			return nil, nil
		}
		str := gonStruct(t)
		if str == nil || index >= str.NumFields() {
			return nil, nil
		}
		field = str.Field(index)
		t = field.Type()
	}
	return t, field
}

type gonPayload struct {
	path  []int
	field *types.Var
}

func gonPayloads(t types.Type, prefix []int) []gonPayload {
	if types.EnumStorageOf(t) == nil {
		return nil
	}
	var out []gonPayload
	var fields func(types.Type, []int)
	fields = func(t types.Type, path []int) {
		str := gonStruct(t)
		if str == nil {
			return
		}
		for i := range str.NumFields() {
			field := str.Field(i)
			p := append(slices.Clone(path), i)
			if gonNilable(field.Type()) {
				out = append(out, gonPayload{p, field})
			} else if gonStruct(field.Type()) != nil {
				fields(field.Type(), p)
			}
		}
	}
	fields(t, prefix)
	return out
}

// A projection of an optional payload is relevant only to present alternatives.
// Presence deliberately does not imply that a pointer payload is nonnil.
func (f *gonFlow) present(value ssa.Value, path []int, point gonPoint, context *gonContext) bool {
	t := value.Type()
	for i, index := range path {
		if desc := types.EnumStorageOf(t); desc != nil && index > 0 {
			tag := f.scalar(value, append(slices.Clone(path[:i]), 0), point, context)
			if tag != nil {
				for v := range desc.NumVariants() {
					variant := desc.Variant(v)
					if variant.StorageIndex() == index && constant.Compare(tag, token.NEQ, constant.MakeInt64(int64(variant.Tag()))) {
						return false
					}
				}
			}
		}
		str := gonStruct(t)
		if str == nil {
			break
		}
		t = str.Field(index).Type()
	}
	return true
}

// Field paths describe the address of a payload without relying on the compiler's
// physical ABI. EnumStorageOf/OptionalStorage are the checked tooling metadata.
func gonAddress(value ssa.Value) (ssa.Value, []int) {
	switch v := value.(type) {
	case *ssa.IndexAddr:
		if index, ok := v.Index.(*ssa.Const); ok {
			n, ok := constant.Int64Val(index.Value)
			if ok {
				base, path := gonAddress(v.X)
				return base, append(path, -int(n)-1)
			}
		}
	case *ssa.Slice:
		if v.Low == nil {
			return gonAddress(v.X)
		}
	case *ssa.FieldAddr:
		base, path := gonAddress(v.X)
		return base, append(path, v.Field)
	case *ssa.ChangeType:
		return gonAddress(v.X)
	}
	return value, nil
}

type gonStored struct {
	value   ssa.Value
	path    []int
	point   gonPoint
	context *gonContext
	unknown bool
}

func (f *gonFlow) reaching(addr ssa.Value, path []int, point gonPoint, context *gonContext, _ map[*ssa.BasicBlock]bool) []gonStored {
	base, addressPath := gonAddress(addr)
	wanted := append(addressPath, path...)
	resolvedBase, resolvedPath := f.address(addr, context)
	resolvedWanted := append(resolvedPath, path...)
	// Each predecessor is visited once. Recursive traversal of a branching
	// loop enumerates paths exponentially; reaching definitions require a set
	// of stores, not enumeration of the executions reaching those stores.
	seen := make(map[gonPoint]bool)
	work := []gonPoint{point}
	var out []gonStored
	for len(work) > 0 {
		p := work[len(work)-1]
		work = work[:len(work)-1]
		if seen[p] {
			continue
		}
		seen[p] = true
		defined := false
		for i := min(p.index, len(p.block.Instrs)) - 1; i >= 0; i-- {
			if call, ok := p.block.Instrs[i].(*ssa.Call); ok {
				if writes := f.callWrites(&call.Call, addr, path, gonPoint{p.block, i}, context); len(writes) > 0 {
					out = append(out, writes...)
					defined = true
					break
				}
			}
			store, ok := p.block.Instrs[i].(*ssa.Store)
			if !ok {
				continue
			}
			storedBase, storedPath := f.address(store.Addr, context)
			if storedBase != resolvedBase || len(storedPath) > len(resolvedWanted) || !slices.Equal(storedPath, resolvedWanted[:len(storedPath)]) {
				continue
			}
			out = append(out, gonStored{value: store.Val, path: slices.Clone(resolvedWanted[len(storedPath):]), point: gonPoint{p.block, i}, context: context})
			defined = true
			break
		}
		if defined {
			continue
		}
		for _, pred := range p.block.Preds {
			if f.possibleEdge(pred, p.block, context) {
				work = append(work, gonPoint{pred, len(pred.Instrs)})
			}
		}
		if len(p.block.Preds) == 0 {
			if binding, ok := context.bindings[base]; ok {
				out = append(out, f.reaching(binding.value, wanted, binding.point, binding.context, nil)...)
			} else if alloc, ok := base.(*ssa.Alloc); ok {
				typ, _ := gonProjection(alloc.Type().Underlying().(*types.Pointer).Elem(), wanted)
				if typ != nil {
					out = append(out, gonStored{value: ssa.NewConst(nil, typ), point: p, context: context})
				}
			}
		}
	}
	return out
}

func (f *gonFlow) load(addr ssa.Value, path []int, point gonPoint, context *gonContext) []gonOrigin {
	if index, ok := addr.(*ssa.IndexAddr); ok {
		if _, fixed := index.Index.(*ssa.Const); !fixed {
			return f.elementOrigins(index.X, path, point, context)
		}
	}
	var out []gonOrigin
	for _, stored := range f.reaching(addr, path, point, context, make(map[*ssa.BasicBlock]bool)) {
		if stored.unknown {
			out = append(out, gonOrigin{&annotation.ProduceTriggerTautology{}, f.expr(addr, addr.Pos())})
		} else {
			out = append(out, f.origins(stored.value, stored.path, stored.point, stored.context)...)
		}
	}
	if len(out) > 0 {
		return out
	}
	if index, ok := addr.(*ssa.IndexAddr); ok {
		return f.elementOrigins(index.X, path, point, context)
	}
	base, addressPath := gonAddress(addr)
	if free, ok := base.(*ssa.FreeVar); ok {
		for _, fn := range f.functions {
			for _, block := range fn.Blocks {
				for creationIndex, instruction := range block.Instrs {
					closure, ok := instruction.(*ssa.MakeClosure)
					if !ok || closure.Fn != free.Parent() {
						continue
					}
					creation := gonPoint{block, creationIndex}
					index := slices.Index(free.Parent().FreeVars, free)
					capture := closure.Bindings[index]
					projection := append(slices.Clone(addressPath), path...)
					called := false
					var lateAt []gonPoint
					for _, callBlock := range fn.Blocks {
						for i, callInstruction := range callBlock.Instrs {
							var call *ssa.CallCommon
							concurrent := false
							async := false
							deferred := false
							switch ins := callInstruction.(type) {
							case *ssa.Call:
								call = &ins.Call
							case *ssa.Go:
								call, concurrent, async = &ins.Call, true, true
							case *ssa.Defer:
								call, concurrent, deferred = &ins.Call, true, true
							}
							if call == nil || call.Value != closure && !slices.Contains(call.Args, ssa.Value(closure)) {
								continue
							}
							invocation := gonPoint{callBlock, i}
							if !f.afterPoint(invocation, creation) || !f.reachable(callBlock, context) {
								continue
							}
							called = true
							if concurrent {
								lateAt = append(lateAt, invocation)
							}
							if !deferred {
								out = append(out, f.load(capture, projection, invocation, context)...)
							}
							if async {
								// A goroutine can observe an intermediate assignment even
								// when the capture is restored before the caller returns.
								out = append(out, f.futureCaptureWrites(capture, projection, invocation, context)...)
							}
						}
					}
					if !called || len(lateAt) > 0 {
						// Escaped, deferred and concurrent closures can observe later
						// assignments. An earlier return never executed the creation.
						for _, exit := range fn.Blocks {
							for i, ins := range exit.Instrs {
								point := gonPoint{exit, i}
								isExit := false
								switch ins.(type) {
								case *ssa.Return, *ssa.Panic:
									isExit = true
								}
								if !isExit || !f.afterPoint(point, creation) || !f.reachable(exit, context) {
									continue
								}
								if !called || slices.ContainsFunc(lateAt, func(invocation gonPoint) bool { return f.afterPoint(point, invocation) }) {
									out = append(out, f.load(capture, projection, point, context)...)
								}
							}
						}
					}
				}
			}
		}
		return out
	}

	if param, ok := base.(*ssa.Parameter); ok {
		if _, slice := param.Type().Underlying().(*types.Slice); slice {
			return f.elementOrigins(param, path, point, context)
		}
	}
	// Ordinary struct fields use the same field-site annotation as the Go
	// backpropagator, including fields read from imported constructor returns.
	// Missing a reaching local store does not prove that a field can be nil.
	if pointer, ok := base.Type().Underlying().(*types.Pointer); ok && len(addressPath)+len(path) > 0 && types.EnumStorageOf(pointer.Elem()) == nil {
		_, field := gonProjection(pointer.Elem(), append(slices.Clone(addressPath), path...))
		if field != nil {
			return []gonOrigin{{&annotation.FldRead{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: &annotation.FieldAnnotationKey{FieldDecl: field}}}, f.expr(addr, addr.Pos())}}
		}
	}
	if _, ok := base.(*ssa.Alloc); ok {
		return nil
	} // initialized nonnil origins
	if global, ok := base.(*ssa.Global); ok {
		object, _ := global.Object().(*types.Var)
		if object != nil {
			return []gonOrigin{{&annotation.GlobalVarRead{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: &annotation.GlobalVarAnnotationKey{VarDecl: object}}}, f.expr(global, global.Pos())}}
		}
	}
	return []gonOrigin{{&annotation.ProduceTriggerTautology{}, f.expr(addr, addr.Pos())}}
}

func (f *gonFlow) futureCaptureWrites(capture ssa.Value, path []int, invocation gonPoint, context *gonContext) []gonOrigin {
	base, prefix := f.address(capture, context)
	wanted := append(prefix, path...)
	var out []gonOrigin
	for _, block := range invocation.block.Parent().Blocks {
		if !f.reachable(block, context) {
			continue
		}
		for i, instruction := range block.Instrs {
			point := gonPoint{block, i}
			if !f.afterPoint(point, invocation) {
				continue
			}
			if store, ok := instruction.(*ssa.Store); ok {
				storedBase, storedPath := f.address(store.Addr, context)
				if storedBase == base && len(storedPath) <= len(wanted) && slices.Equal(storedPath, wanted[:len(storedPath)]) {
					out = append(out, f.origins(store.Val, wanted[len(storedPath):], point, context)...)
				}
			}
			if call, ok := instruction.(*ssa.Call); ok {
				for _, written := range f.callWrites(&call.Call, capture, path, point, context) {
					if written.unknown {
						out = append(out, gonOrigin{&annotation.ProduceTriggerTautology{}, f.expr(capture, capture.Pos())})
					} else {
						out = append(out, f.origins(written.value, written.path, written.point, written.context)...)
					}
				}
			}
		}
	}
	return out
}

// afterPoint reports whether execution can reach point after the earlier point.
// In particular, error returns before a closure exists are not capture states.
func (f *gonFlow) afterPoint(point, earlier gonPoint) bool {
	if point.block == earlier.block && point.index > earlier.index {
		return true
	}
	seen := make(map[*ssa.BasicBlock]bool)
	work := slices.Clone(point.block.Preds)
	for len(work) > 0 {
		block := work[len(work)-1]
		work = work[:len(work)-1]
		if block == earlier.block {
			return true
		}
		if !seen[block] {
			seen[block] = true
			work = append(work, block.Preds...)
		}
	}
	return false
}

func (f *gonFlow) guarded(value ssa.Value, point gonPoint, context *gonContext) bool {
	for block := point.block; block != nil; block = block.Idom() {
		for _, pred := range block.Preds {
			if !pred.Dominates(point.block) || len(pred.Instrs) == 0 {
				continue
			}
			branch, ok := pred.Instrs[len(pred.Instrs)-1].(*ssa.If)
			if !ok {
				continue
			}
			comparison, ok := branch.Cond.(*ssa.BinOp)
			if !ok || (comparison.Op != token.EQL && comparison.Op != token.NEQ) {
				continue
			}
			other := comparison.X
			nilValue, ok := comparison.Y.(*ssa.Const)
			if !ok || !nilValue.IsNil() {
				other = comparison.Y
				nilValue, ok = comparison.X.(*ssa.Const)
			}
			if !ok || !nilValue.IsNil() || !f.sameValue(other, value, context) {
				continue
			}
			nonnil := 0
			if comparison.Op == token.EQL {
				nonnil = 1
			}
			if pred.Succs[nonnil].Dominates(point.block) {
				return true
			}
		}
	}
	return false
}

func (f *gonFlow) definitelyNonNil(value ssa.Value, point gonPoint, context *gonContext) bool {
	if f.guarded(value, point, context) {
		return true
	}
	if binding, ok := context.bindings[value]; ok {
		return f.definitelyNonNil(binding.value, binding.point, binding.context)
	}
	switch v := value.(type) {
	case *ssa.Alloc, *ssa.MakeMap, *ssa.MakeChan, *ssa.MakeSlice, *ssa.MakeInterface, *ssa.Function, *ssa.MakeClosure:
		return true
	case *ssa.ChangeInterface:
		return f.definitelyNonNil(v.X, point, context)
	case *ssa.Call:
		if call, ok := f.expressions[v].(*ast.CallExpr); ok {
			if producer := hook.AssumeReturn(f.pass, call); producer != nil && producer.Annotation.Kind() == annotation.Never {
				return true
			}
		}
	}
	return false
}

func (f *gonFlow) callSucceeded(call *ssa.CallCommon, point gonPoint) bool {
	for block := point.block; block != nil; block = block.Idom() {
		for _, pred := range block.Preds {
			if !pred.Dominates(point.block) || len(pred.Instrs) == 0 {
				continue
			}
			branch, ok := pred.Instrs[len(pred.Instrs)-1].(*ssa.If)
			if !ok {
				continue
			}
			cmp, ok := branch.Cond.(*ssa.BinOp)
			if !ok {
				continue
			}
			extract, ok := cmp.X.(*ssa.Extract)
			if !ok {
				continue
			}
			c, ok := extract.Tuple.(*ssa.Call)
			if !ok || &c.Call != call {
				continue
			}
			nilValue, ok := cmp.Y.(*ssa.Const)
			if !ok || !nilValue.IsNil() {
				continue
			}
			if extract.Index != call.Signature().Results().Len()-1 {
				continue
			}
			success := 0
			if cmp.Op == token.NEQ {
				success = 1
			} else if cmp.Op != token.EQL {
				continue
			}
			if pred.Succs[success].Dominates(point.block) {
				return true
			}
		}
	}
	return false
}

func (f *gonFlow) reachable(block *ssa.BasicBlock, context *gonContext) (result bool) {
	key := gonReachKey{block, context}
	if f.reachableKnown[key] {
		return f.reachableCache[key]
	}
	defer func() { f.reachableKnown[key] = true; f.reachableCache[key] = result }()
	seen := make(map[*ssa.BasicBlock]bool)
	var visit func(*ssa.BasicBlock) bool
	visit = func(b *ssa.BasicBlock) bool {
		if b.Index == 0 {
			return true
		}
		if seen[b] {
			return false
		}
		seen[b] = true
		for _, pred := range b.Preds {
			if f.possibleEdge(pred, b, context) && visit(pred) {
				return true
			}
		}
		return false
	}
	return visit(block)
}
func (f *gonFlow) possibleEdge(from, to *ssa.BasicBlock, context *gonContext) bool {
	if f.exitsBefore(from, len(from.Instrs)) {
		return false
	}
	if len(from.Instrs) == 0 {
		return true
	}
	branch, ok := from.Instrs[len(from.Instrs)-1].(*ssa.If)
	if !ok {
		return true
	}
	value := f.scalar(branch.Cond, nil, gonPoint{from, len(from.Instrs) - 1}, context)
	if value == nil || value.Kind() != constant.Bool {
		return true
	}
	index := 1
	if constant.BoolVal(value) {
		index = 0
	}
	return from.Succs[index] == to
}

func (f *gonFlow) exitsBefore(block *ssa.BasicBlock, index int) bool {
	for _, instruction := range block.Instrs[:min(index, len(block.Instrs))] {
		if call, ok := instruction.(*ssa.Call); ok && f.noReturn(&call.Call) {
			return true
		}
	}
	return false
}

func (f *gonFlow) noReturn(call *ssa.CallCommon) bool {
	object := call.Method
	if callee := call.StaticCallee(); callee != nil {
		object = f.object(callee)
	}
	if object == nil || object.Pkg() == nil || object.Pkg().Path() != "testing" || object.Signature().Recv() == nil {
		return false
	}
	switch object.Name() {
	case "Fatal", "Fatalf", "FailNow", "Skip", "Skipf", "SkipNow":
		return true
	}
	return false
}

func (f *gonFlow) scalar(value ssa.Value, path []int, point gonPoint, context *gonContext) (result constant.Value) {
	if value == nil {
		return nil
	}
	key := gonTraceKey{value, fmt.Sprint(path), point.block, point.index, context}
	if f.scalarKnown[key] {
		return f.scalarCache[key]
	}
	if f.constantActive[key] {
		return nil
	}
	defer func() { f.scalarCache[key] = result; f.scalarKnown[key] = true }()
	f.constantActive[key] = true
	defer delete(f.constantActive, key)
	if binding, ok := context.bindings[value]; ok {
		return f.scalar(binding.value, path, binding.point, binding.context)
	}
	recur := func(v ssa.Value, p []int) constant.Value { return f.scalar(v, p, point, context) }
	switch v := value.(type) {
	case *ssa.Const:
		if v.IsNil() && len(path) > 0 {
			typ, _ := gonProjection(v.Type(), path)
			if basic, ok := typ.Underlying().(*types.Basic); ok {
				if basic.Info()&types.IsBoolean != 0 {
					return constant.MakeBool(false)
				}
				if basic.Info()&types.IsNumeric != 0 {
					return constant.MakeInt64(0)
				}
			}
		}
		return v.Value
	case *ssa.Field:
		return recur(v.X, append([]int{v.Field}, path...))
	case *ssa.ChangeType:
		return recur(v.X, path)
	case *ssa.Convert:
		return recur(v.X, path)
	case *ssa.UnOp:
		if v.Op == token.MUL {
			var result constant.Value
			stores := f.reaching(v.X, path, gonPoint{v.Block(), instructionIndex(v)}, context, make(map[*ssa.BasicBlock]bool))
			for _, stored := range stores {
				if stored.unknown {
					return nil
				}
				c := f.scalar(stored.value, stored.path, stored.point, stored.context)
				if c == nil {
					return nil
				}
				if result != nil && !constant.Compare(result, token.EQL, c) {
					return nil
				}
				result = c
			}
			return result
		}
		if c := recur(v.X, nil); c != nil && v.Op == token.NOT {
			return constant.UnaryOp(token.NOT, c, 0)
		}
	case *ssa.BinOp:
		x, y := recur(v.X, nil), recur(v.Y, nil)
		if x == nil || y == nil {
			return nil
		}
		switch v.Op {
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
			return constant.MakeBool(constant.Compare(x, v.Op, y))
		}
	case *ssa.Phi:
		var result constant.Value
		for i, edge := range v.Edges {
			pred := v.Block().Preds[i]
			if !f.possibleEdge(pred, v.Block(), context) {
				continue
			}
			c := f.scalar(edge, path, gonPoint{pred, len(pred.Instrs)}, context)
			if c == nil {
				return nil
			}
			if result != nil && !constant.Compare(result, token.EQL, c) {
				return nil
			}
			result = c
		}
		return result
	}
	return nil
}

func gonNilable(t types.Type) bool {
	if parameter, ok := types.Unalias(t).(*types.TypeParam); ok {
		terms, err := typeparams.NormalTerms(parameter)
		if err != nil || len(terms) == 0 {
			return true
		}
		for _, term := range terms {
			if gonNilable(term.Type()) {
				return true
			}
		}
		return false
	}
	if _, ok := t.Underlying().(*types.Signature); ok {
		return true
	}
	return !typeshelper.TypeBarsNilness(t)
}

func (f *gonFlow) callContext(call *ssa.CallCommon, point gonPoint, parent *gonContext) *gonContext {
	key := gonCallKey{call, parent}
	if context := f.contexts[key]; context != nil {
		return context
	}
	child := &gonContext{bindings: make(map[ssa.Value]gonBinding)}
	f.contexts[key] = child
	callee := call.StaticCallee()
	for i, param := range callee.Params {
		if i < len(call.Args) {
			child.bindings[param] = gonBinding{call.Args[i], point, parent}
		}
	}
	if closure, ok := call.Value.(*ssa.MakeClosure); ok {
		for i, free := range callee.FreeVars {
			child.bindings[free] = gonBinding{closure.Bindings[i], point, parent}
		}
	}
	return child
}

// Resolve aliases through call arguments without changing the read point: callee
// stores must be inspected before falling back to the caller's pre-call state.
func (f *gonFlow) address(value ssa.Value, context *gonContext) (ssa.Value, []int) {
	base, path := gonAddress(value)
	if binding, ok := context.bindings[base]; ok {
		parent, prefix := f.address(binding.value, binding.context)
		return parent, append(prefix, path...)
	}
	return base, path
}

func (f *gonFlow) callWrites(call *ssa.CallCommon, addr ssa.Value, path []int, point gonPoint, context *gonContext) []gonStored {
	base, prefix := f.address(addr, context)
	wanted := append(prefix, path...)
	for index, arg := range call.Args {
		boxed := false
		if value, ok := arg.(*ssa.MakeInterface); ok {
			arg = value.X
			boxed = true
		}
		if _, pointer := arg.Type().Underlying().(*types.Pointer); !pointer {
			continue
		}
		argBase, argPath := f.address(arg, context)
		if argBase != base || len(argPath) > len(wanted) || !slices.Equal(argPath, wanted[:len(argPath)]) {
			continue
		}
		key := gonCallKey{call, context}
		callee := call.StaticCallee()
		if boxed || callee == nil || len(callee.Blocks) == 0 || f.mutationActive[key] {
			return []gonStored{{point: point, context: context, unknown: true}}
		}
		f.mutationActive[key] = true
		child := f.callContext(call, point, context)
		var out []gonStored
		for _, block := range callee.Blocks {
			if f.reachable(block, child) {
				for i, ins := range block.Instrs {
					if _, ok := ins.(*ssa.Return); ok {
						out = append(out, f.reaching(callee.Params[index], wanted[len(argPath):], gonPoint{block, i}, child, make(map[*ssa.BasicBlock]bool))...)
					}
				}
			}
		}
		delete(f.mutationActive, key)
		return out
	}
	return nil
}

func (f *gonFlow) sameValue(a, b ssa.Value, context *gonContext) bool {
	if a == b {
		return true
	}
	xaddr, xpath, xpoint, xok := gonRead(a, nil)
	yaddr, ypath, ypoint, yok := gonRead(b, nil)
	if !xok || !yok {
		return false
	}
	xbase, xprefix := f.address(xaddr, context)
	ybase, yprefix := f.address(yaddr, context)
	xpath = append(xprefix, xpath...)
	ypath = append(yprefix, ypath...)
	if xbase != ybase || !slices.Equal(xpath, ypath) {
		return false
	}
	xs := f.reaching(xaddr, xpath[len(xprefix):], xpoint, context, nil)
	ys := f.reaching(yaddr, ypath[len(yprefix):], ypoint, context, nil)
	if len(xs) != len(ys) {
		return false
	}
	for _, left := range xs {
		found := false
		for _, right := range ys {
			if gonSameStoredValue(left.value, right.value) && left.point == right.point && left.context == right.context && left.unknown == right.unknown && slices.Equal(left.path, right.path) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func gonSameStoredValue(a, b ssa.Value) bool {
	if a == b {
		return true
	}
	x, xok := a.(*ssa.Const)
	y, yok := b.(*ssa.Const)
	return xok && yok && x.IsNil() && y.IsNil() && types.Identical(x.Type(), y.Type())
}

// A compiler load may read the whole struct and then project a field, or read
// the field's address directly. Both forms denote the same guarded memory value.
func gonRead(value ssa.Value, path []int) (ssa.Value, []int, gonPoint, bool) {
	switch value := value.(type) {
	case *ssa.Field:
		return gonRead(value.X, append([]int{value.Field}, path...))
	case *ssa.UnOp:
		if value.Op == token.MUL {
			return value.X, path, gonPoint{value.Block(), instructionIndex(value)}, true
		}
	}
	return nil, nil, gonPoint{}, false
}

// Ordinary map indexing can produce the zero value even when every stored
// element is nonnil. Optional Lookup excludes that absent case; Go indexing
// excludes it only after a successful comma-ok check.
func (f *gonFlow) lookupOrigins(lookup *ssa.Lookup, path []int, point gonPoint, context *gonContext) []gonOrigin {
	value := lookup.X
	if param, ok := value.(*ssa.Parameter); ok {
		if _, bound := context.bindings[param]; !bound {
			var out []gonOrigin
			called := false
			for _, fn := range f.functions {
				for _, block := range fn.Blocks {
					for i, ins := range block.Instrs {
						if call, ok := ins.(*ssa.Call); ok && call.Call.StaticCallee() == param.Parent() {
							called = true
							parent := &gonContext{bindings: make(map[ssa.Value]gonBinding)}
							child := f.callContext(&call.Call, gonPoint{block, i}, parent)
							out = append(out, f.lookupOrigins(lookup, path, point, child)...)
						}
					}
				}
			}
			if called {
				return out
			}
		}
	}
	key := f.scalar(lookup.Index, nil, point, context)
	readPoint := gonPoint{lookup.Block(), instructionIndex(lookup)}
	if binding, ok := context.bindings[value]; ok {
		value = binding.value
		point = binding.point
		context = binding.context
		readPoint = point
	}
	present := f.lookupPresent(lookup, point)
	var out []gonOrigin
	if _, local := value.(*ssa.MakeMap); local {
		work := []gonPoint{readPoint}
		seen := make(map[gonPoint]bool)
		for len(work) > 0 {
			p := work[len(work)-1]
			work = work[:len(work)-1]
			if seen[p] {
				continue
			}
			seen[p] = true
			found := false
			for i := min(p.index, len(p.block.Instrs)) - 1; i >= 0; i-- {
				update, ok := p.block.Instrs[i].(*ssa.MapUpdate)
				if !ok || update.Map != value {
					continue
				}
				updated := f.scalar(update.Key, nil, gonPoint{p.block, i}, context)
				if key != nil && updated != nil && !constant.Compare(key, token.EQL, updated) {
					continue
				}
				out = append(out, f.origins(update.Value, path, gonPoint{p.block, i}, context)...)
				if key != nil && updated != nil {
					found = true
					break
				}
			}
			if found {
				continue
			}
			for _, pred := range p.block.Preds {
				if f.possibleEdge(pred, p.block, context) {
					work = append(work, gonPoint{pred, len(pred.Instrs)})
				}
			}
			if len(p.block.Preds) == 0 && !present {
				out = append(out, gonOrigin{&annotation.ConstNil{ProduceTriggerTautology: &annotation.ProduceTriggerTautology{}}, f.expr(lookup, lookup.Pos())})
			}
		}
		return out
	}
	out = f.elementOrigins(value, path, point, context)
	if !present {
		out = append(out, gonOrigin{&annotation.ConstNil{ProduceTriggerTautology: &annotation.ProduceTriggerTautology{}}, f.expr(lookup, lookup.Pos())})
	}
	return out
}
func (f *gonFlow) lookupPresent(lookup *ssa.Lookup, point gonPoint) bool {
	if !lookup.CommaOk {
		return false
	}
	for block := point.block; block != nil; block = block.Idom() {
		for _, pred := range block.Preds {
			if !pred.Dominates(point.block) || len(pred.Instrs) == 0 {
				continue
			}
			branch, ok := pred.Instrs[len(pred.Instrs)-1].(*ssa.If)
			if !ok {
				continue
			}
			extract, ok := branch.Cond.(*ssa.Extract)
			if ok && extract.Tuple == lookup && extract.Index == 1 && pred.Succs[0].Dominates(point.block) {
				return true
			}
		}
	}
	return false
}

// A known boxed receiver permits the same source-sensitive call analysis as a
// direct method call. Unknown interface receivers retain Method annotation facts.
func (f *gonFlow) concreteCall(call *ssa.CallCommon) *ssa.CallCommon {
	if !call.IsInvoke() {
		return nil
	}
	if concrete := f.concreteCalls[call]; concrete != nil {
		return concrete
	}
	boxed, ok := call.Value.(*ssa.MakeInterface)
	if !ok {
		return nil
	}
	method := boxed.Parent().Prog.LookupMethod(boxed.X.Type(), call.Method.Pkg(), call.Method.Name())
	if method == nil {
		return nil
	}
	concrete := &ssa.CallCommon{Value: method, Args: append([]ssa.Value{boxed.X}, call.Args...)}
	f.concreteCalls[call] = concrete
	return concrete
}
