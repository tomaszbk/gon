package main

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

type localHost[T any] struct{}

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
	return Local{input}.text
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
		return Role{kind: 1}
	case "student":
		return Role{kind: 2}
	default:
		return Role{text: text}
	}
}

func (role Role) String() string {
	switch role.kind {
	case 1:
		return "teacher"
	case 2:
		return "student"
	}
	return role.text
}

func (role Role) MarshalText() ([]byte, error) { return []byte(role.String()), nil }
func (role *Role) UnmarshalText(data []byte) error {
	*role = parseRole(string(data))
	return nil
}

func description(role Role) string {
	switch role.kind {
	case 1:
		return "known:teacher"
	case 2:
		return "known:student"
	}
	return "unknown:" + role.text
}

func main() {
	parser := parseRole
	text := Alias.String
	decode := (*Alias).UnmarshalText
	for _, input := range []string{"", "teacher", "student", "future", "\x00\xff"} {
		role := parser(input)
		if role.String() != input || text(role) != input {
			panic("string text")
		}
		var formatter interface{ String() string } = role
		if formatter.String() != input {
			panic("String interface")
		}
		bytes, err := role.MarshalText()
		if err != nil || string(bytes) != input {
			panic("MarshalText")
		}
		var decoded Alias
		if decode(&decoded, bytes) != nil || decoded != role {
			panic("UnmarshalText")
		}
		var zero Role
		if description(zero) != "unknown:" {
			panic("zero")
		}
		if description(parser("teacher")) != "known:teacher" || description(parser("student")) != "known:student" || description(parser("future")) != "unknown:future" {
			panic("matching")
		}
	}
	genericParse := genericParser()
	for _, input := range []string{"", "ready", "future"} {
		value := genericParse(input)
		var formatter interface{ String() string } = value
		if GenericAlias.String(value) != input || formatter.String() != input {
			panic("generic methods")
		}
		bytes, err := value.MarshalText()
		var decoded GenericAlias
		if err != nil || decoded.UnmarshalText(bytes) != nil || decoded != value {
			panic("generic text interfaces")
		}
		var other Generic[string]
		if other.UnmarshalText(bytes) != nil || other.String() != input {
			panic("other instantiation")
		}
		if localText(input) != input {
			panic("local methods")
		}
	}
	for _, input := range []string{"", "known", "future", "\x00\n\xff"} {
		if localGeneric[int](input) != input || localGeneric[string](input) != input || (localHost[int]{}).text(input) != input || (localHost[string]{}).text(input) != input {
			panic("local enum in generic function")
		}
	}
	println("PASS")
}
