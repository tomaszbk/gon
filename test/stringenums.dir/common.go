package main

import (
	"encoding"
	"encoding/json"
	"fmt"
	"strings"
)

var (
	_ fmt.Stringer             = *new(Role)
	_ encoding.TextMarshaler   = *new(Role)
	_ encoding.TextUnmarshaler = (*Role)(nil)
	_ fmt.Stringer             = *new(Alias)
	_ encoding.TextMarshaler   = *new(Alias)
	_ encoding.TextUnmarshaler = (*Alias)(nil)
)

type loginRequest struct {
	Role Role `json:"role"`
}

type localHost[T any] struct{}

func check(ok bool, description string) {
	if !ok {
		panic(description)
	}
}

func main() {
	var zero Role
	check(zero.String() == "" && describe(zero) == "unknown:", "zero is Unknown(empty)")
	parse := parser()
	for _, input := range []string{"", "teacher", "student", "future", "Teacher", "\x00\n\t\"\\", "é世🌎", "invalid:\xff"} {
		role := parse(input)
		check(role.String() == input, "parser preserves text")
		check(formatRole(role) == input, "String method expression")
		check(fmt.Sprintf("%s", role) == input, "Stringer interface")
		text, err := role.MarshalText()
		check(err == nil && string(text) == input, "MarshalText preserves text")
		if len(text) != 0 {
			text[0] ^= 0xff
			check(role.String() == input, "MarshalText returns independent bytes")
		}
		var decoded Alias
		data := []byte(input)
		check(decodeText(&decoded, data) == nil && decoded == role, "UnmarshalText round trip")
		if len(data) != 0 {
			data[0] ^= 0xff
			check(decoded.String() == input, "UnmarshalText retains independent text")
		}
		wire, err := json.Marshal(loginRequest{Role: role})
		check(err == nil, "JSON marshal")
		want, err := json.Marshal(struct {
			Role string `json:"role"`
		}{input})
		check(err == nil && string(wire) == string(want), "JSON matches ordinary string")
		var original struct {
			Role string `json:"role"`
		}
		check(json.Unmarshal(want, &original) == nil, "string JSON baseline")
		var output loginRequest
		check(json.Unmarshal(wire, &output) == nil, "JSON unmarshal")
		check(output.Role == parse(original.Role), "JSON text round trip, including UTF-8 replacement")
		fmt.Printf("%q:%s:%s\n", input, describe(role), wire)
	}
	check(describe(teacher()) == "teacher" && describe(student()) == "student", "known matching")
	check(describe(unknown("teacher")) == "unknown:teacher", "explicit fallback keeps variant identity")
	check(unknown("teacher") != teacher(), "fallback differs from known variant")
	var marshaler encoding.TextMarshaler = teacher()
	encoded, err := marshaler.MarshalText()
	check(err == nil && string(encoded) == "teacher", "TextMarshaler interface")
	role := student()
	var unmarshaler encoding.TextUnmarshaler = &role
	check(unmarshaler.UnmarshalText([]byte("future")) == nil && role == unknown("future"), "TextUnmarshaler interface")
	for _, input := range []string{`null`, `{"other":1}`, `{"role":null}`} {
		value := loginRequest{Role: teacher()}
		check(json.Unmarshal([]byte(input), &value) == nil && value.Role == teacher(), "JSON null or missing preserves existing enum")
	}
	for _, input := range []string{`0`, `true`, `{}`, `[]`} {
		role := teacher()
		check(json.Unmarshal([]byte(input), &role) != nil && role == teacher(), "non-string JSON fails without modifying enum")
		value := loginRequest{Role: student()}
		check(json.Unmarshal([]byte(`{"role":`+input+`}`), &value) != nil && value.Role == student(), "non-string JSON field preserves value")
	}
	role = teacher()
	check(json.Unmarshal([]byte(`null`), &role) == nil && role == teacher(), "direct JSON null preserves value")
	check(json.Unmarshal([]byte(`"\ud800"`), &role) == nil && role.String() == "\ufffd", "JSON surrogate replacement")
	check(json.Unmarshal([]byte(`"student"`), &role) == nil && role == student(), "known JSON decoding")
	check(json.Unmarshal([]byte(`"future"`), &role) == nil && role == unknown("future"), "unknown JSON decoding")
	check(strings.Contains(string(mustJSON(unknown("teacher"))), "teacher"), "fallback encoding")
	checkGeneric()
	checkCalls()
	for _, input := range []string{"", "ready", "future"} {
		check(localText(input) == input, "local string enum methods")
	}
	for _, input := range []string{"", "known", "future", "\x00\n\xff"} {
		check(localGeneric[int](input) == input, "local string enum in generic int function")
		check(localGeneric[string](input) == input, "local string enum in generic string function")
		check((localHost[int]{}).text(input) == input, "local string enum in generic int method")
		check((localHost[string]{}).text(input) == input, "local string enum in generic string method")
	}
	fmt.Println("string enums: PASS")
}

func checkCalls() {
	var order []string
	parseFactory := func() func(string) Role {
		order = append(order, "function")
		return parser()
	}
	input := func() string {
		order = append(order, "argument")
		return "teacher"
	}
	value := parseFactory()(input())
	check(value == teacher() && strings.Join(order, ",") == "function,argument", "parser function and argument evaluated once in order")
	format := value.String
	value = student()
	check(format() == "teacher", "method value captures value receiver")
	receiver := func() *Role {
		order = append(order, "receiver")
		return &value
	}
	bytes := func() []byte {
		order = append(order, "bytes")
		return []byte("future")
	}
	order = nil
	check(receiver().UnmarshalText(bytes()) == nil && value == unknown("future"), "method decoder")
	check(strings.Join(order, ",") == "receiver,bytes", "receiver and bytes evaluated once in order")
	order = nil
	value = teacher()
	func() {
		defer receiver().UnmarshalText(bytes())
		check(value == teacher() && strings.Join(order, ",") == "receiver,bytes", "defer evaluates receiver and argument eagerly")
		value = student()
	}()
	check(value == unknown("future") && strings.Join(order, ",") == "receiver,bytes", "defer executes decoder once on captured pointer")
	order = nil
	value = teacher()
	done := make(chan struct{})
	invoke := func(decode func([]byte) error, input []byte) {
		defer close(done)
		if err := decode(input); err != nil {
			panic(err)
		}
	}
	go invoke(receiver().UnmarshalText, bytes())
	<-done
	check(value == unknown("future") && strings.Join(order, ",") == "receiver,bytes", "goroutine invokes captured decoder with eagerly evaluated bytes once")
}

func checkGeneric() {
	parse := genericParser()
	format := GenericAlias.String
	for _, input := range []string{"", "ready", "future"} {
		value := parse(input)
		check(value.String() == input && format(value) == input, "generic string enum methods")
		wire, err := json.Marshal(value)
		check(err == nil, "generic JSON marshal")
		var decoded Generic[int]
		check(json.Unmarshal(wire, &decoded) == nil && decoded == value, "generic JSON round trip")
		var other Generic[string]
		check(other.UnmarshalText([]byte(input)) == nil && other.String() == input, "independent generic instantiation")
		var formatter fmt.Stringer = value
		check(formatter.String() == input, "generic Stringer interface")
		fmt.Printf("generic:%q:%s\n", input, wire)
	}
}

func mustJSON(value Role) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
