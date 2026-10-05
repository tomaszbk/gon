package main

import "reflect"

func init() {
	typ := reflect.TypeFor[Role]()
	check(typ.Kind() == reflect.Struct && typ.Name() == "Role", "legacy reflection identity")
	check(reflect.ValueOf(Role{}).IsZero(), "legacy reflection zero")
	check(reflect.TypeFor[Generic[int]]().Name() == "Generic[int]", "legacy generic reflection identity")
	check(parseRole("teacher") == teacher(), "legacy parser argument")
	check(parseGeneric[int]("ready").String() == "ready", "legacy generic parser argument")
	var value Role
	check(value.UnmarshalText([]byte("student")) == nil && value == student(), "legacy method argument")
}

type Role struct {
	kind int
	text string
}

type Alias = Role

type Generic[T any] struct {
	kind int
	text string
}

type GenericAlias = Generic[int]

func parseGeneric[T any](text string) Generic[T] {
	if text == "ready" {
		return Generic[T]{kind: 1}
	}
	return Generic[T]{text: text}
}

func (value Generic[T]) String() string {
	if value.kind == 1 {
		return "ready"
	}
	return value.text
}
func (value Generic[T]) MarshalText() ([]byte, error) { return []byte(value.String()), nil }
func (value *Generic[T]) UnmarshalText(data []byte) error {
	*value = parseGeneric[T](string(data))
	return nil
}

func genericParser() func(string) Generic[int] { return parseGeneric[int] }

func localText(input string) string {
	type Local struct{ text string }
	value := Local{input}
	return value.text
}

func localGeneric[T any](input string) string {
	type Local struct{ text string }
	type Explicit[U any] struct{ text string }
	inner := Explicit[int]{input}
	formatInner := func(value Explicit[int]) string { return value.text }
	innerData := []byte(formatInner(inner))
	var other Explicit[string]
	decodeInner := func(data []byte) error { other = Explicit[string]{string(data)}; return nil }
	if formatInner(inner) != input || decodeInner(innerData) != nil || other.text != input {
		panic("explicit local generic text methods")
	}
	parse := func(text string) Local { return Local{text} }
	text := func(value Local) string { return value.text }
	marshal := func(value Local) ([]byte, error) { return []byte(value.text), nil }
	decode := func(value *Local, data []byte) error { *value = parse(string(data)); return nil }
	value := parse(input)
	captured := value
	boundString := func() string { return text(captured) }
	boundMarshal := func() ([]byte, error) { return marshal(captured) }
	boundDecode := func(data []byte) error { return decode(&value, data) }
	closure := func(input string) string { return text(parse(input)) }
	if text(value) != input || closure(input) != input {
		panic("local generic String")
	}
	data, err := marshal(value)
	boundData, boundErr := boundMarshal()
	if err != nil || boundErr != nil || string(data) != input || string(boundData) != input {
		panic("local generic MarshalText")
	}
	var decoded Local
	if decode(&decoded, data) != nil || decoded != value {
		panic("local generic UnmarshalText")
	}
	if boundDecode([]byte("changed")) != nil || text(value) != "changed" || boundString() != input {
		panic("local generic captured method values")
	}
	return text(decoded)
}

func (localHost[T]) text(input string) string {
	type Local struct{ text string }
	value := Local{input}
	format := func(value Local) string { return value.text }
	bound := func() string { return format(value) }
	closure := func() string { return bound() }
	data := []byte(format(value))
	decoded := Local{string(data)}
	if decoded != value || closure() != input {
		panic("local enum in generic method")
	}
	return format(decoded)
}

func parseRole(text string) Role {
	switch text {
	case "teacher":
		return teacher()
	case "student":
		return student()
	default:
		return unknown(text)
	}
}

func (role Role) String() string {
	switch role.kind {
	case 0:
		return role.text
	case 1:
		return "teacher"
	case 2:
		return "student"
	}
	panic("invalid role tag")
}

func (role Role) MarshalText() ([]byte, error) { return []byte(role.String()), nil }
func (role *Role) UnmarshalText(data []byte) error {
	*role = parseRole(string(data))
	return nil
}

func teacher() Role                          { return Role{kind: 1} }
func student() Role                          { return Role{kind: 2} }
func unknown(text string) Role               { return Role{text: text} }
func parser() func(string) Role              { return parseRole }
func formatRole(role Role) string            { return Role.String(role) }
func decodeText(role *Alias, b []byte) error { return (*Alias).UnmarshalText(role, b) }

func describe(role Role) string {
	switch role.kind {
	case 0:
		return "unknown:" + role.text
	case 1:
		return "teacher"
	case 2:
		return "student"
	}
	panic("invalid role tag")
}
