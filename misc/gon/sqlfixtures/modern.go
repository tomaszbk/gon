// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// The standard SQL boundary supplies the protocol for native textual enums.
type Role enum string {
	default Unknown(string)
	Teacher = "teacher"
	Student = "student"
}

func parseRole(text string) Role { return Role.Parse(text) }

type Generic[T any] enum string {
	default Unknown(string)
	Ready = "ready"
}

func parseGeneric[T any](text string) Generic[T] { return Generic[T].Parse(text) }
