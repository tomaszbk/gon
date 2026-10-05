package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Payloads whose own JSON protocol must be honored in both forms.
type Stamp struct{ Text string }

func (s Stamp) MarshalJSON() ([]byte, error) { return json.Marshal("stamp:" + s.Text) }
func (s *Stamp) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	s.Text = strings.TrimPrefix(text, "stamp:")
	return nil
}

type Label struct{ Text string }

func (l Label) MarshalText() ([]byte, error) { return []byte("label:" + l.Text), nil }
func (l *Label) UnmarshalText(data []byte) error {
	l.Text = strings.TrimPrefix(string(data), "label:")
	return nil
}

type Address struct {
	City string `json:"city"`
	Zip  string `json:"zip"`
}

type Person struct {
	Name string `json:"name"`
}

// opt describes an optional value independently of its representation. The
// legacy form turns it into a pointer and the modern form into a native T?.
type opt[T any] struct {
	value T
	ok    bool
}

func some[T any](value T) opt[T] { return opt[T]{value, true} }
func none[T any]() opt[T]        { return opt[T]{} }

func (o opt[T]) String() string {
	if !o.ok {
		return "-"
	}
	return fmt.Sprintf("%+v", o.value)
}

// spec is the representation independent content of a Profile.
type spec struct {
	name    opt[string]
	age     opt[int]
	score   opt[float64]
	active  opt[bool]
	role    opt[string]
	home    opt[Address]
	stamp   opt[Stamp]
	label   opt[Label]
	aliases []opt[string]
	extra   map[string]opt[int]
	note    opt[string]
	count   opt[int]
	limit   opt[int]
	manager opt[*Person]
}

func describe(s spec) string {
	var aliases []string
	for _, alias := range s.aliases {
		aliases = append(aliases, alias.String())
	}
	var keys []string
	for key := range s.extra {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var extra []string
	for _, key := range keys {
		extra = append(extra, key+"="+s.extra[key].String())
	}
	manager := "-"
	if s.manager.ok {
		manager = "nil"
		if s.manager.value != nil {
			manager = s.manager.value.Name
		}
	}
	return fmt.Sprintf("name=%v age=%v score=%v active=%v role=%v home=%v stamp=%v label=%v aliases=%v/%d extra=%v/%d note=%v count=%v limit=%v manager=%s",
		s.name, s.age, s.score, s.active, s.role, s.home, s.stamp, s.label,
		aliases, len(s.aliases), extra, len(s.extra), s.note, s.count, s.limit, manager)
}

func check(ok bool, description string) {
	if !ok {
		panic(description)
	}
}

func main() {
	checkEncoding()
	checkDecoding()
	checkTypeErrors()
	checkTopLevel()
	checkStreams()
	fmt.Println("optional JSON: PASS")
}

var (
	allAbsent = spec{}
	zeroes    = spec{
		name: some(""), age: some(0), score: some(0.0), active: some(false), role: some(""),
		home: some(Address{}), stamp: some(Stamp{}), label: some(Label{}),
		aliases: []opt[string]{some(""), none[string]()}, extra: map[string]opt[int]{"zero": some(0), "none": none[int]()},
		note: some(""), count: some(0), limit: some(0), manager: some(&Person{}),
	}
	filled = spec{
		name: some("Ada"), age: some(36), score: some(9.5), active: some(true), role: some("teacher"),
		home: some(Address{"Lima", "15001"}), stamp: some(Stamp{"a"}), label: some(Label{"b"}),
		aliases: []opt[string]{some("x"), none[string](), some("z")}, extra: map[string]opt[int]{"k": some(7)},
		note: some("n"), count: some(2), limit: some(40), manager: some(&Person{"Grace"}),
	}
)

func checkEncoding() {
	for _, test := range []struct {
		name string
		spec spec
		wire string
	}{
		{"absent", allAbsent, `{"name":null,"age":null,"score":null,"active":null,"role":null,"home":null,"stamp":null,"label":null,"aliases":null,"extra":null,"limit":null,"manager":null}`},
		{"zero", zeroes, `{"name":"","age":0,"score":0,"active":false,"role":"","home":{"city":"","zip":""},"stamp":"stamp:","label":"label:","aliases":["",null],"extra":{"none":null,"zero":0},"note":"","count":0,"limit":"0","manager":{"name":""}}`},
		{"filled", filled, `{"name":"Ada","age":36,"score":9.5,"active":true,"role":"teacher","home":{"city":"Lima","zip":"15001"},"stamp":"stamp:a","label":"label:b","aliases":["x",null,"z"],"extra":{"k":7},"note":"n","count":2,"limit":"40","manager":{"name":"Grace"}}`},
		{"partial", spec{age: some(0), note: some("x"), manager: some(&Person{"m"})}, `{"name":null,"age":0,"score":null,"active":null,"role":null,"home":null,"stamp":null,"label":null,"aliases":null,"extra":null,"note":"x","limit":null,"manager":{"name":"m"}}`},
	} {
		profile := build(test.spec)
		check(describe(profile.view()) == describe(test.spec), test.name+": representation round trip")
		data, err := marshalProfile(profile)
		check(err == nil, test.name+": marshal")
		check(string(data) == test.wire, test.name+": wire\n"+string(data)+"\n"+test.wire)
		var back Profile
		check(json.Unmarshal(data, &back) == nil, test.name+": unmarshal")
		check(describe(back.view()) == describe(test.spec), test.name+": decoded "+describe(back.view()))
		again, err := marshalProfile(back)
		check(err == nil && bytes.Equal(again, data), test.name+": stable encoding")
		fmt.Printf("%s: %s\n", test.name, data)
	}
	// A present nil payload would decode as absence, so it is rejected.
	_, err := marshalProfile(build(spec{manager: some[*Person](nil)}))
	check(err != nil, "present nil payload is rejected")
	fmt.Println("present nil payload: rejected")
	// Indentation and escaping options see the payload's own encoding.
	indented, err := json.MarshalIndent(build(spec{age: some(1), note: some("x"), stamp: some(Stamp{"s"})}), ">", "  ")
	check(err == nil, "indented profile")
	fmt.Printf("indented: %q\n", indented)
	escaped, err := json.Marshal(build(spec{name: some("<a&b>")}).nameValue())
	check(err == nil && string(escaped) == `"\u003ca\u0026b\u003e"`, "escaped payload "+string(escaped))
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	check(encoder.Encode(build(spec{name: some("<a&b>")}).nameValue()) == nil && buffer.String() == "\"<a&b>\"\n", "unescaped payload")
}

func checkDecoding() {
	for _, test := range []struct {
		name  string
		start spec
		input string
		want  spec
	}{
		{"null clears present values", filled, `{"name":null,"age":null,"score":null,"active":null,"role":null,"home":null,"stamp":null,"label":null,"aliases":null,"extra":null,"note":null,"count":null,"limit":null,"manager":null}`, allAbsent},
		{"missing leaves values", filled, `{}`, filled},
		{"unknown keys are ignored", filled, `{"other":1}`, filled},
		{"null on absent stays absent", allAbsent, `{"name":null,"home":null,"limit":null}`, allAbsent},
		{"present zeroes", allAbsent, `{"name":"","age":0,"score":0,"active":false,"role":"","home":{},"stamp":"stamp:","label":"label:","note":"","count":0,"limit":"0"}`,
			spec{name: some(""), age: some(0), score: some(0.0), active: some(false), role: some(""), home: some(Address{}), stamp: some(Stamp{}), label: some(Label{}), note: some(""), count: some(0), limit: some(0)}},
		{"scalar replacement", filled, `{"name":"Bob","age":0,"score":1,"active":false,"role":"student","limit":"5"}`,
			withScalars(filled, "Bob", 0, 1.0, false, "student", 5)},
		{"payload merges into existing", filled, `{"home":{"city":"Cusco"}}`, withHome(filled, Address{"Cusco", "15001"})},
		{"payload starts from zero", allAbsent, `{"home":{"city":"Cusco"}}`, spec{home: some(Address{"Cusco", ""})}},
		{"payload protocols", allAbsent, `{"stamp":"stamp:q","label":"label:r"}`, spec{stamp: some(Stamp{"q"}), label: some(Label{"r"})}},
		{"elements", allAbsent, `{"aliases":[null,"a",""],"extra":{"k":null,"j":3}}`,
			spec{aliases: []opt[string]{none[string](), some("a"), some("")}, extra: map[string]opt[int]{"k": none[int](), "j": some(3)}}},
		{"quoted null clears", filled, `{"limit":"null"}`, withLimit(filled, none[int]())},
		{"pointer payload", allAbsent, `{"manager":{"name":"m"}}`, spec{manager: some(&Person{"m"})}},
		{"pointer payload null", filled, `{"manager":null}`, withManager(filled, none[*Person]())},
	} {
		profile := build(test.start)
		err := json.Unmarshal([]byte(test.input), &profile)
		check(err == nil, test.name+": unmarshal")
		check(describe(profile.view()) == describe(test.want), test.name+": got "+describe(profile.view()))
		fmt.Printf("%s: %s\n", test.name, describe(profile.view()))
	}
}

func withScalars(s spec, name string, age int, score float64, active bool, role string, limit int) spec {
	s.name, s.age, s.score, s.active, s.role, s.limit = some(name), some(age), some(score), some(active), some(role), some(limit)
	return s
}
func withHome(s spec, home Address) spec    { s.home = some(home); return s }
func withLimit(s spec, limit opt[int]) spec { s.limit = limit; return s }
func withManager(s spec, manager opt[*Person]) spec {
	s.manager = manager
	return s
}

func checkTypeErrors() {
	// A value of the wrong type is reported and the other fields still decode.
	profile := build(filled)
	err := json.Unmarshal([]byte(`{"age":"x","name":"Bob"}`), &profile)
	var typeError *json.UnmarshalTypeError
	check(errors.As(err, &typeError) && typeError.Value == "string" && typeError.Field == "age", "type error is reported for the field")
	view := profile.view()
	check(view.age == some(36), "present value survives a type error")
	check(view.name == some("Bob"), "decoding continues after a type error")
	fmt.Printf("type error: %s\n", describe(view))
	for _, input := range []string{`{"home":5}`, `{"stamp":5}`, `{"aliases":{}}`, `{"limit":5}`, `{"name":["x"]}`, `{"extra":[1]}`, `{"manager":"x"}`, `{"score":true}`} {
		profile := build(filled)
		err := json.Unmarshal([]byte(input), &profile)
		check(err != nil, input+": error")
		check(describe(profile.view()) == describe(filled), input+": unchanged "+describe(profile.view()))
	}
	// Syntax errors and unknown fields use the ordinary decoder paths.
	profile = build(filled)
	check(json.Unmarshal([]byte(`{"age":`), &profile) != nil, "truncated input")
	decoder := json.NewDecoder(strings.NewReader(`{"agee":1}`))
	decoder.DisallowUnknownFields()
	check(decoder.Decode(&profile) != nil, "unknown field is rejected")
	check(describe(profile.view()) == describe(filled), "failed decodes leave the profile unchanged")
}

func checkTopLevel() {
	for _, test := range []struct {
		name  string
		value any
		wire  string
	}{
		{"absent int", topLevel(none[int]()), `null`},
		{"zero int", topLevel(some(0)), `0`},
		{"false", topLevel(some(false)), `false`},
		{"empty string", topLevel(some("")), `""`},
		{"struct", topLevel(some(Address{})), `{"city":"","zip":""}`},
		{"stamp", topLevel(some(Stamp{"t"})), `"stamp:t"`},
		{"label", topLevel(some(Label{"t"})), `"label:t"`},
		{"empty slice", topLevel(some([]int{})), `[]`},
		{"float", topLevel(some(1.5)), `1.5`},
	} {
		data, err := json.Marshal(test.value)
		check(err == nil && string(data) == test.wire, test.name+": top-level "+string(data))
		fmt.Printf("%s: %s\n", test.name, data)
	}
	for _, test := range []struct {
		input string
		start opt[int]
		want  opt[int]
	}{
		{`7`, none[int](), some(7)},
		{`0`, none[int](), some(0)},
		{`null`, some(5), none[int]()},
		{`null`, none[int](), none[int]()},
		{`9`, some(5), some(9)},
	} {
		got, err := decodeTop(test.input, test.start)
		check(err == nil && got == test.want, test.input+": top-level decode "+got.String())
		fmt.Printf("decode %s: %v\n", test.input, got)
	}
	_, err := decodeTop(`"x"`, some(5))
	check(err != nil, "top-level type error")
	got, _ := decodeTop(`"x"`, some(5))
	check(got == some(5), "top-level type error preserves a present value")
	items, err := decodeItems(`[1,null,0,3]`)
	check(err == nil && fmt.Sprint(items) == "[1 - 0 3]", "slice elements "+fmt.Sprint(items))
	wire, err := encodeItems(items)
	check(err == nil && string(wire) == `[1,null,0,3]`, "slice encoding "+string(wire))
	fmt.Printf("items: %v %s\n", items, wire)
}

func checkStreams() {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	for _, profile := range []Profile{build(allAbsent), build(spec{age: some(0)})} {
		check(encoder.Encode(profile) == nil, "encode")
	}
	decoder := json.NewDecoder(&buffer)
	var first, second Profile
	check(decoder.Decode(&first) == nil && decoder.Decode(&second) == nil, "decode stream")
	check(describe(first.view()) == describe(allAbsent) && describe(second.view()) == describe(spec{age: some(0)}), "stream round trip")
	fmt.Println("streams: PASS")
}
