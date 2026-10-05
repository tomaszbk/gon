package main

import "featuretest/lib"

type Derived lib.Role

func importedDerivedRoundTrip(input string) string {
	return Derived.Parse(input).String()
}

func importedParser() func(string) lib.Role                { return lib.Alias.Parse }
func importedGenericParser() func(string) lib.Generic[int] { return lib.Generic[int].Parse }
func importedString(role lib.Role) string                  { return lib.Alias.String(role) }
func importedDecode(role *lib.Alias, b []byte) error {
	return (*lib.Role).UnmarshalText(role, b)
}
func importedDescription(role lib.Role) string {
	return switch role {
	case lib.Role.Unknown(text) => "unknown:" + text
	case lib.Role.Teacher => "teacher"
	case lib.Role.Student => "student"
	}
}
