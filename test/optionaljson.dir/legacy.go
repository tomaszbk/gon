package main

import "encoding/json"

// Legacy form: absence is a nil pointer and presence a non-nil pointer.
type Profile struct {
	Name    *string         `json:"name"`
	Age     *int            `json:"age"`
	Score   *float64        `json:"score"`
	Active  *bool           `json:"active"`
	Role    *string         `json:"role"`
	Home    *Address        `json:"home"`
	Stamp   *Stamp          `json:"stamp"`
	Label   *Label          `json:"label"`
	Aliases []*string       `json:"aliases"`
	Extra   map[string]*int `json:"extra"`
	Note    *string         `json:"note,omitempty"`
	Count   *int            `json:"count,omitzero"`
	Limit   *int            `json:"limit,string"`
	Manager **Person        `json:"manager"`
}

func pointer[T any](value opt[T]) *T {
	if !value.ok {
		return nil
	}
	return &value.value
}

func unpointer[T any](value *T) opt[T] {
	if value == nil {
		return none[T]()
	}
	return some(*value)
}

func build(s spec) Profile {
	p := Profile{
		Name: pointer(s.name), Age: pointer(s.age), Score: pointer(s.score), Active: pointer(s.active),
		Role: pointer(s.role), Home: pointer(s.home), Stamp: pointer(s.stamp), Label: pointer(s.label),
		Note: pointer(s.note), Count: pointer(s.count), Limit: pointer(s.limit), Manager: pointer(s.manager),
	}
	if s.aliases != nil {
		p.Aliases = make([]*string, len(s.aliases))
		for i, alias := range s.aliases {
			p.Aliases[i] = pointer(alias)
		}
	}
	if s.extra != nil {
		p.Extra = make(map[string]*int)
		for key, value := range s.extra {
			p.Extra[key] = pointer(value)
		}
	}
	return p
}

func (p Profile) view() spec {
	s := spec{
		name: unpointer(p.Name), age: unpointer(p.Age), score: unpointer(p.Score), active: unpointer(p.Active),
		role: unpointer(p.Role), home: unpointer(p.Home), stamp: unpointer(p.Stamp), label: unpointer(p.Label),
		note: unpointer(p.Note), count: unpointer(p.Count), limit: unpointer(p.Limit), manager: unpointer(p.Manager),
	}
	if p.Aliases != nil {
		s.aliases = make([]opt[string], len(p.Aliases))
		for i, alias := range p.Aliases {
			s.aliases[i] = unpointer(alias)
		}
	}
	if p.Extra != nil {
		s.extra = make(map[string]opt[int])
		for key, value := range p.Extra {
			s.extra[key] = unpointer(value)
		}
	}
	return s
}

func (p Profile) nameValue() any { return p.Name }

// The wire format of an optional never contains a present null: a present
// pointer to a nil pointer is rejected explicitly in the legacy form.
func marshalProfile(p Profile) ([]byte, error) {
	if p.Manager != nil && *p.Manager == nil {
		return nil, errPresentNil
	}
	return json.Marshal(p)
}

var errPresentNil = json.Unmarshal([]byte("["), new(int))

func topLevel[T any](value opt[T]) any { return pointer(value) }

func decodeTop(input string, start opt[int]) (opt[int], error) {
	p := pointer(start)
	err := json.Unmarshal([]byte(input), &p)
	return unpointer(p), err
}

func decodeItems(input string) ([]opt[int], error) {
	var items []*int
	err := json.Unmarshal([]byte(input), &items)
	result := make([]opt[int], len(items))
	for i, item := range items {
		result[i] = unpointer(item)
	}
	return result, err
}

func encodeItems(items []opt[int]) ([]byte, error) {
	pointers := make([]*int, len(items))
	for i, item := range items {
		pointers[i] = pointer(item)
	}
	return json.Marshal(pointers)
}
