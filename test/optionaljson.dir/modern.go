package main

import (
	"encoding/json"
	"errors"
	"reflect"
)

// Modern form: native optionals, with no wrapper types or pointer adapters.
type Profile struct {
	Name    string?         `json:"name"`
	Age     int?            `json:"age"`
	Score   float64?        `json:"score"`
	Active  bool?           `json:"active"`
	Role    string?         `json:"role"`
	Home    Address?        `json:"home"`
	Stamp   Stamp?          `json:"stamp"`
	Label   Label?          `json:"label"`
	Aliases []string?       `json:"aliases"`
	Extra   map[string]int? `json:"extra"`
	Note    string?         `json:"note,omitempty"`
	Count   int?            `json:"count,omitzero"`
	Limit   int?            `json:"limit,string"`
	Manager (*Person)?      `json:"manager"`
}

func lift[T any](value T) T? { return value }

func optional[T any](value opt[T]) T? {
	if !value.ok {
		return nil
	}
	return value.value
}

func unoptional[T any](value T?) opt[T] {
	return switch value {
	case nil => none[T]()
	case payload? => some(payload)
	}
}

func build(s spec) Profile {
	p := Profile{
		Name: optional(s.name), Age: optional(s.age), Score: optional(s.score), Active: optional(s.active),
		Role: optional(s.role), Home: optional(s.home), Stamp: optional(s.stamp), Label: optional(s.label),
		Note: optional(s.note), Count: optional(s.count), Limit: optional(s.limit), Manager: optional(s.manager),
	}
	if s.aliases != nil {
		p.Aliases = make([]string?, len(s.aliases))
		for i, alias := range s.aliases {
			p.Aliases[i] = optional(alias)
		}
	}
	if s.extra != nil {
		p.Extra = make(map[string]int?)
		for key, value := range s.extra {
			p.Extra[key] = optional(value)
		}
	}
	return p
}

func (p Profile) view() spec {
	s := spec{
		name: unoptional(p.Name), age: unoptional(p.Age), score: unoptional(p.Score), active: unoptional(p.Active),
		role: unoptional(p.Role), home: unoptional(p.Home), stamp: unoptional(p.Stamp), label: unoptional(p.Label),
		note: unoptional(p.Note), count: unoptional(p.Count), limit: unoptional(p.Limit), manager: unoptional(p.Manager),
	}
	if p.Aliases != nil {
		s.aliases = make([]opt[string], len(p.Aliases))
		for i, alias := range p.Aliases {
			s.aliases[i] = unoptional(alias)
		}
	}
	if p.Extra != nil {
		s.extra = make(map[string]opt[int])
		for key, value := range p.Extra {
			s.extra[key] = unoptional(value)
		}
	}
	return s
}

func (p Profile) nameValue() any { return p.Name }

// A present nil payload is rejected by encoding/json itself.
func marshalProfile(p Profile) ([]byte, error) { return json.Marshal(p) }

func topLevel[T any](value opt[T]) any { return optional(value) }

func decodeTop(input string, start opt[int]) (opt[int], error) {
	value := optional(start)
	err := json.Unmarshal([]byte(input), &value)
	return unoptional(value), err
}

func decodeItems(input string) ([]opt[int], error) {
	var items []int?
	err := json.Unmarshal([]byte(input), &items)
	result := make([]opt[int], len(items))
	for i, item := range items {
		result[i] = unoptional(item)
	}
	return result, err
}

func encodeItems(items []opt[int]) ([]byte, error) {
	optionals := make([]int?, len(items))
	for i, item := range items {
		optionals[i] = optional(item)
	}
	return json.Marshal(optionals)
}

// Properties without a pointer equivalent are checked only in the modern form.
func init() {
	checkReflection()
	checkUnsupported()
}

func checkReflection() {
	value := build(filled).Age
	check(reflect.IsOptional(reflect.TypeOf(value)) && reflect.IsOptional(reflect.TypeFor[Profile]().Field(0).Type), "profile fields are native optionals")
	check(reflect.TypeFor[Profile]().Field(3).Tag.Get("json") == "active", "field tags are ordinary")
}

func checkUnsupported() {
	// Nested optionals would make null ambiguous.
	nested := lift(lift(1))
	_, err := json.Marshal(nested)
	var unsupportedType *json.UnsupportedTypeError
	check(errors.As(err, &unsupportedType), "nested optional marshal is unsupported")
	var absentNested (int?)?
	_, err = json.Marshal(absentNested)
	check(errors.As(err, &unsupportedType), "absent nested optional marshal is unsupported")
	for _, input := range []string{`1`, `null`} {
		check(json.Unmarshal([]byte(input), &absentNested) != nil, "nested optional unmarshal is unsupported")
	}
	// Optional map keys are not representable.
	_, err = json.Marshal(map[int?]string{})
	check(errors.As(err, &unsupportedType), "optional map keys are unsupported")
	// Present payloads that encode as null are rejected, never silently lost.
	var unsupportedValue *json.UnsupportedValueError
	var nilPointer *Person
	var nilSlice []int
	var nilMap map[string]int
	var nilInterface any
	for _, value := range []any{lift(nilPointer), lift(nilSlice), lift(nilMap), lift(nilInterface), lift(nullStamp{})} {
		_, err := json.Marshal(value)
		check(errors.As(err, &unsupportedValue), "present null payload is rejected")
	}
	// A present empty value is not null.
	data, err := json.Marshal(lift([]int{}))
	check(err == nil && string(data) == "[]", "present empty slice")
	// Type errors leave an absent optional absent.
	var absent int?
	check(json.Unmarshal([]byte(`"x"`), &absent) != nil, "type error on absent optional")
	check(unoptional(absent) == none[int](), "absent optional unchanged by a type error")
}

type nullStamp struct{}

func (nullStamp) MarshalJSON() ([]byte, error) { return []byte("null"), nil }
