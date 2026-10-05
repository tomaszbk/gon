// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import . "internal/types/errors"

// bindStringEnumParse gives each concrete enum descriptor its own static
// parser. It is a function, rather than a method expression with a receiver.
func bindStringEnumParse(e *Enum, owner Type) {
	if !e.IsString() || e.NumVariants() == 0 || e.defaultIndex < 0 {
		return
	}
	obj := e.Default().Object()
	pos, pkg := obj.Pos(), obj.Pkg()
	if named, ok := Unalias(owner).(*Named); ok {
		pos, pkg = named.Obj().Pos(), named.Obj().Pkg()
	}
	sig := NewSignatureType(nil, nil, nil,
		NewTuple(NewParam(pos, pkg, "text", Typ[String])),
		NewTuple(NewParam(pos, pkg, "", owner)), false)
	e.parse = NewFunc(pos, pkg, "Parse", sig)
}

// addStringEnumMethods installs ordinary, exported methods without introducing
// dependencies on encoding packages or on the spelling of predeclared names.
func (check *Checker) addStringEnumMethods(named *Named) {
	e := EnumOf(named)
	if e == nil || !e.IsString() {
		return
	}
	for _, method := range check.methods[named.Obj()] {
		if method.Name() == "Parse" {
			check.error(method, DuplicateDecl, "string enum method Parse collides with the automatic parser")
		}
	}
	pos, pkg := named.Obj().Pos(), named.Obj().Pkg()
	param := func(name string, typ Type) *Var { return NewParam(pos, pkg, name, typ) }
	errorType := Universe.Lookup("error").Type()
	add := func(name string, pointer bool, params, results *Tuple) {
		var receiver Type = named
		var rparams []*TypeParam
		if params := named.TypeParams(); params.Len() != 0 {
			rparams = make([]*TypeParam, params.Len())
			args := make([]Type, len(rparams))
			for i := range rparams {
				p := params.At(i)
				rparams[i] = check.newTypeParam(NewTypeName(pos, pkg, p.Obj().Name(), nil), Typ[Invalid])
				args[i] = rparams[i]
				check.mono.recordCanon(rparams[i], p)
			}
			smap := makeRenameMap(params.list(), rparams)
			for i, p := range rparams {
				p.bound = check.subst(pos, params.At(i).bound, smap, nil, check.context())
			}
			receiver = check.instance(pos, named, args, nil, check.context())
		}
		if pointer {
			receiver = NewPointer(receiver)
		}
		sig := NewSignatureType(NewParam(pos, pkg, "value", receiver), rparams, nil, params, results, false)
		named.AddMethod(NewFunc(pos, pkg, name, sig))
	}
	add("String", false, nil, NewTuple(param("", Typ[String])))
	add("MarshalText", false, nil, NewTuple(param("", NewSlice(Typ[Byte])), param("", errorType)))
	add("UnmarshalText", true, NewTuple(param("text", NewSlice(Typ[Byte]))), NewTuple(param("", errorType)))
}
