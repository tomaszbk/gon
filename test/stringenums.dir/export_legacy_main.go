package main

import "featuretest/lib"

type Derived lib.Role

func importedDerivedRoundTrip(input string) string {
	return lib.Role(Derived(lib.ParseRole(input))).String()
}

func importedParser() func(string) lib.Role                { return lib.ParseRole }
func importedGenericParser() func(string) lib.Generic[int] { return lib.ParseGeneric[int] }
func importedString(role lib.Role) string                  { return lib.Alias.String(role) }
func importedDecode(role *lib.Alias, b []byte) error {
	return (*lib.Role).UnmarshalText(role, b)
}
func importedDescription(role lib.Role) string { return lib.Describe(role) }
