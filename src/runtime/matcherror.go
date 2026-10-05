// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "unsafe"

// matchErrorAs implements the error-tree search used by compiler-lowered enum
// patterns whose subject has static type error. It is not called by Go
// programs. The compiler converts the subject to the empty interface at the
// call, which preserves its dynamic type and value; every value reached from it
// is an error, because the subject has type error.
//
// typ describes the enum type E and ptrTyp describes *E. It reports a pointer
// to a value of type E found in err's tree, or nil. The search is exactly that
// of errors.AsType[E]: err itself is examined first, then the errors reached
// by repeatedly calling Unwrap() error or Unwrap() []error, depth first. An
// error matches if its dynamic type is E or if it has an As(any) bool method
// that sets the *E it receives and returns true. The returned memory is never
// modified by the runtime after it is returned and must only be read.
//
// The helper lives in the runtime so that it needs no import of package errors,
// which a package using such a pattern may not depend on, and so that standard
// library packages that errors itself depends on can use the construct. It
// does not use panic or recover.
func matchErrorAs(err any, typ, ptrTyp *_type) unsafe.Pointer {
	var target unsafe.Pointer // lazily allocated *E, shared by As methods
	return matchErrorAsTree(err, typ, ptrTyp, &target)
}

func matchErrorAsTree(err any, typ, ptrTyp *_type, target *unsafe.Pointer) unsafe.Pointer {
	for {
		if err == nil {
			return nil
		}
		if e := efaceOf(&err); e._type == typ {
			if typ.IsDirectIface() {
				// The interface word is the value itself.
				p := mallocgc(typ.Size_, typ, true)
				*(*unsafe.Pointer)(p) = e.data
				return p
			}
			return e.data
		}
		if x, ok := err.(interface{ As(any) bool }); ok {
			if *target == nil {
				*target = mallocgc(typ.Size_, typ, true)
			}
			var arg any
			a := efaceOf(&arg)
			a._type, a.data = ptrTyp, *target
			if x.As(arg) {
				return *target
			}
		}
		switch x := err.(type) {
		case interface{ Unwrap() error }:
			err = x.Unwrap()
		case interface{ Unwrap() []error }:
			for _, err := range x.Unwrap() {
				if err == nil {
					continue
				}
				if p := matchErrorAsTree(err, typ, ptrTyp, target); p != nil {
					return p
				}
			}
			return nil
		default:
			return nil
		}
	}
}
