// Copyright (c) 2026 Gon contributors.
// Licensed under the Apache License, Version 2.0.

package function

import (
	"fmt"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"
)

// Payload identities preserve argument-specific nilness across package
// boundaries. A generic optional identity function may be called with both nil
// and nonnil payloads; joining its return-field annotation would lose that
// relationship. These facts describe only identities proved for every present
// return, while other functions retain NilAway's annotation inference.
type gonPayloadFact struct{ Identities []gonPayloadIdentity }

func (*gonPayloadFact) AFact() {}

type gonPayloadIdentity struct {
	Result     int
	OutputPath []int
	Parameter  int // SSA argument index, including a method receiver
	InputPath  []int
}

func (f *gonFlow) exportPayloadFacts() {
	f.payloadFacts = make(map[*types.Func]*gonPayloadFact)
	for _, fn := range f.functions {
		object := f.object(fn)
		if object == nil || object.Pkg() != f.pass.Pkg || !object.Exported() || len(fn.TypeArgs()) > 0 {
			continue
		}
		fact := new(gonPayloadFact)
		context := &gonContext{bindings: make(map[ssa.Value]gonBinding)}
		for result := range fn.Signature.Results().Len() {
			for _, payload := range gonPayloads(fn.Signature.Results().At(result).Type(), nil) {
				var identity *gonPayloadIdentity
				valid := true
				for _, block := range fn.Blocks {
					if !f.reachable(block, context) {
						continue
					}
					for index, ins := range block.Instrs {
						ret, ok := ins.(*ssa.Return)
						if !ok || result >= len(ret.Results) {
							continue
						}
						point := gonPoint{block, index}
						if !f.present(ret.Results[result], payload.path, point, context) {
							continue
						}
						parameter, path, ok := f.parameterProjection(ret.Results[result], payload.path, point, context, make(map[gonTraceKey]bool))
						if !ok || parameter.Parent() != fn {
							valid = false
							break
						}
						candidate := &gonPayloadIdentity{Result: result, OutputPath: slices.Clone(payload.path), Parameter: slices.Index(fn.Params, parameter), InputPath: path}
						if identity != nil && (identity.Parameter != candidate.Parameter || !slices.Equal(identity.InputPath, candidate.InputPath)) {
							valid = false
							break
						}
						identity = candidate
					}
					if !valid {
						break
					}
				}
				if valid && identity != nil {
					fact.Identities = append(fact.Identities, *identity)
				}
			}
		}
		if len(fact.Identities) > 0 {
			f.payloadFacts[object] = fact
			f.pass.ExportObjectFact(object, fact)
		}
	}
}

func (f *gonFlow) parameterProjection(value ssa.Value, path []int, point gonPoint, context *gonContext, active map[gonTraceKey]bool) (*ssa.Parameter, []int, bool) {
	key := gonTraceKey{value, fmt.Sprint(path), point.block, point.index, context}
	if active[key] {
		return nil, nil, false
	}
	active[key] = true
	defer delete(active, key)
	if binding, ok := context.bindings[value]; ok {
		return f.parameterProjection(binding.value, path, binding.point, binding.context, active)
	}
	recur := func(value ssa.Value, path []int) (*ssa.Parameter, []int, bool) {
		return f.parameterProjection(value, path, point, context, active)
	}
	merge := func(stores []gonStored) (*ssa.Parameter, []int, bool) {
		var parameter *ssa.Parameter
		var inputPath []int
		for _, stored := range stores {
			if stored.unknown {
				return nil, nil, false
			}
			p, projection, ok := f.parameterProjection(stored.value, stored.path, stored.point, stored.context, active)
			if !ok || parameter != nil && (parameter != p || !slices.Equal(inputPath, projection)) {
				return nil, nil, false
			}
			parameter, inputPath = p, projection
		}
		return parameter, inputPath, parameter != nil
	}
	switch value := value.(type) {
	case *ssa.Parameter:
		return value, slices.Clone(path), true
	case *ssa.Field:
		return recur(value.X, append([]int{value.Field}, path...))
	case *ssa.ChangeType:
		return recur(value.X, path)
	case *ssa.Convert:
		// Boxing into an interface changes typed-nil semantics. Identity facts
		// only propagate conversions preserving the same underlying value type.
		if types.Identical(value.Type().Underlying(), value.X.Type().Underlying()) {
			return recur(value.X, path)
		}
	case *ssa.UnOp:
		if value.Op == token.MUL {
			return merge(f.reaching(value.X, path, gonPoint{value.Block(), instructionIndex(value)}, context, nil))
		}
	case *ssa.Phi:
		var stores []gonStored
		for i, edge := range value.Edges {
			pred := value.Block().Preds[i]
			if f.reachable(pred, context) && f.possibleEdge(pred, value.Block(), context) {
				stores = append(stores, gonStored{value: edge, path: path, point: gonPoint{pred, len(pred.Instrs)}, context: context})
			}
		}
		return merge(stores)
	}
	return nil, nil, false
}
