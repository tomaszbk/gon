package main

import (
	"encoding"
	"encoding/json"
	"featuretest/lib"
	"fmt"
	"testing"
)

var (
	_ fmt.Stringer             = *new(lib.Alias)
	_ encoding.TextMarshaler   = *new(lib.Role)
	_ encoding.TextUnmarshaler = (*lib.Role)(nil)
)

func main() {
	checkImportedTextDecoding()
	for _, input := range []string{"", "teacher", "student", "future", "\x00\n\"\\"} {
		if importedDerivedRoundTrip(input) != input {
			panic("derived cross-package string enum parser")
		}
		role := importedParser()(input)
		if role != lib.Parser(input) || role.String() != input || importedString(role) != input {
			panic("imported parser or method metadata")
		}
		encoded, err := json.Marshal(role)
		if err != nil {
			panic(err)
		}
		var decoded lib.Alias
		if err := json.Unmarshal(encoded, &decoded); err != nil || decoded != role {
			panic("imported JSON methods")
		}
		if err := importedDecode(&decoded, []byte("student")); err != nil || decoded.String() != "student" {
			panic("imported pointer method expression")
		}
		fmt.Printf("%q:%s:%s\n", input, importedDescription(role), encoded)
	}
	for _, input := range []string{"", "ready", "future"} {
		value := importedGenericParser()(input)
		var formatter fmt.Stringer = value
		if formatter.String() != input || lib.Generic[int].String(value) != input {
			panic("imported generic method metadata")
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			panic(err)
		}
		var decoded lib.Generic[int]
		if err := json.Unmarshal(encoded, &decoded); err != nil || decoded != value {
			panic("imported generic JSON methods")
		}
		fmt.Printf("generic:%q:%s\n", input, encoded)
	}
	fmt.Println("string enum exports: PASS")
}

func checkImportedTextDecoding() {
	for _, text := range []string{"teacher", "student"} {
		data := []byte(text)
		var value lib.Alias
		allocs := testing.AllocsPerRun(100, func() {
			value = lib.Parser("previous payload")
			if err := importedDecode(&value, data); err != nil {
				panic(err)
			}
		})
		if allocs != 0 || value != lib.Parser(text) {
			panic("imported known UnmarshalText allocations or retained fallback payload")
		}
	}
	var generic lib.Generic[int]
	data := []byte("ready")
	allocs := testing.AllocsPerRun(100, func() {
		generic = importedGenericParser()("previous payload")
		if err := generic.UnmarshalText(data); err != nil {
			panic(err)
		}
	})
	if allocs != 0 || generic != importedGenericParser()("ready") {
		panic("imported generic known UnmarshalText allocations or retained fallback payload")
	}
	data = []byte("future fallback text retained across packages")
	var value lib.Role
	if err := importedDecode(&value, data); err != nil {
		panic(err)
	}
	clear(data)
	if value.String() != "future fallback text retained across packages" {
		panic("imported unknown UnmarshalText owns retained bytes")
	}
}
