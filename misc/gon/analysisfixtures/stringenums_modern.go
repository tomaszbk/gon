package main

type Role enum string {
	default Unknown(string)
	Teacher = "teacher"
	Student = "student"
}

type Alias = Role

type Generic[T any] enum string {
	default Other(string)
	Ready = "ready"
}

type GenericAlias = Generic[int]

type localHost[T any] struct{}

func genericParser() func(string) Generic[int] { return GenericAlias.Parse }

func localText(input string) string {
	type Local enum string {
		default Other(string)
		Ready = "ready"
	}
	value := Local.Parse(input)
	bytes, err := value.MarshalText()
	if err != nil {
		panic(err)
	}
	var decoded Local
	if decoded.UnmarshalText(bytes) != nil || decoded != value {
		panic("local text interfaces")
	}
	return decoded.String()
}

func localGeneric[T any](input string) string {
	type Local enum string {
		default Unknown(string)
		Known = "known"
	}
	type Explicit[U any] enum string {
		default Unknown(string)
		Known = "known"
	}
	inner := Explicit[int].Parse(input)
	formatInner := Explicit[int].String
	innerData, innerErr := inner.MarshalText()
	var other Explicit[string]
	decodeInner := other.UnmarshalText
	if innerErr != nil || inner.String() != input || formatInner(inner) != input || decodeInner(innerData) != nil || other.String() != input {
		panic("explicit local generic text methods")
	}
	parse := Local.Parse
	text := Local.String
	marshal := Local.MarshalText
	decode := (*Local).UnmarshalText
	value := parse(input)
	boundString := value.String
	boundMarshal := value.MarshalText
	boundDecode := value.UnmarshalText
	closure := func(input string) string { return Local.Parse(input).String() }
	var formatter interface{ String() string } = value
	if value.String() != input || text(value) != input || closure(input) != input || formatter.String() != input {
		panic("local generic String")
	}
	data, err := value.MarshalText()
	expressionData, expressionErr := marshal(value)
	boundData, boundErr := boundMarshal()
	if err != nil || expressionErr != nil || boundErr != nil || string(data) != input || string(expressionData) != input || string(boundData) != input {
		panic("local generic MarshalText")
	}
	var decoded Local
	if decoded.UnmarshalText(data) != nil || decoded != value || decode(&decoded, data) != nil || decoded != value {
		panic("local generic UnmarshalText")
	}
	if boundDecode([]byte("changed")) != nil || value.String() != "changed" || boundString() != input {
		panic("local generic captured method values")
	}
	return text(decoded)
}

func (localHost[T]) text(input string) string {
	type Local enum string {
		default Unknown(string)
		Known = "known"
	}
	value := Local.Parse(input)
	format := Local.String
	bound := value.String
	closure := func() string { return bound() }
	data, err := value.MarshalText()
	var decoded Local
	decode := (*Local).UnmarshalText
	if err != nil || decode(&decoded, data) != nil || decoded != value || closure() != input {
		panic("local enum in generic method")
	}
	return format(decoded)
}

func description(role Role) string {
	return switch role {
	case Role.Unknown(text) => "unknown:" + text
	case Role.Teacher => "known:teacher"
	case Role.Student => "known:student"
	}
}

func main() {
	parser := Alias.Parse
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
