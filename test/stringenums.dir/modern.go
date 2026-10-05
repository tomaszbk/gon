package main

import "reflect"

func init() {
	verifyReflection()
	check(Role.Parse(text: "teacher") == Role.Teacher, "generated parser named argument")
	check(Generic[int].Parse(text: "ready") == Generic[int].Ready, "generated generic parser named argument")
	var value Role
	check(value.UnmarshalText(text: []byte("student")) == nil && value == Role.Student, "generated method named argument")
}

func verifyReflection() {
	type Plain struct{ Text string }
	type Ordinary enum {
		default Empty
		Value(string)
	}
	type Metadata enum string {
		default Other(string)
		Empty = ""
		Special = ":\x00\n\"\xff"
	}
	check(!reflect.IsEnum(reflect.TypeFor[Plain]()) && !reflect.IsStringEnum(reflect.TypeFor[Plain]()), "ordinary Go struct reflection")
	check(reflect.IsEnum(reflect.TypeFor[Ordinary]()) && !reflect.IsStringEnum(reflect.TypeFor[Ordinary]()), "ordinary enum reflection")
	check(!reflect.IsStringEnum(nil) && !reflect.IsStringEnum(reflect.TypeFor[string]()), "non-enum string reflection")
	for _, typ := range []reflect.Type{reflect.TypeFor[Role](), reflect.TypeFor[Alias](), reflect.TypeFor[Generic[int]](), reflect.TypeFor[Generic[string]](), reflect.TypeFor[Metadata]()} {
		check(reflect.IsEnum(typ) && reflect.IsStringEnum(typ), "string enum reflection identity")
		check(typ.NumField() == 0, "string enum backing fields hidden")
		variants := reflect.EnumVariants(typ)
		check(variants[0].Default && len(variants[0].Fields) == 1 && variants[0].Fields[0].Type == reflect.TypeFor[string](), "fallback metadata")
		_, known := variants[0].StringValue()
		check(!known, "fallback has no fixed spelling")
		variants[0].Name = "changed"
		check(reflect.EnumVariants(typ)[0].Name != "changed", "metadata detached")
	}
	variants := reflect.EnumVariants(reflect.TypeFor[Role]())
	for i, spelling := range []string{"teacher", "student"} {
		text, known := variants[i+1].StringValue()
		check(known && text == spelling, "known string metadata")
	}
	variants = reflect.EnumVariants(reflect.TypeFor[Metadata]())
	for i, spelling := range []string{"", ":\x00\n\"\xff"} {
		text, known := variants[i+1].StringValue()
		check(known && text == spelling, "empty and escaped metadata")
		value := Metadata.Parse(spelling)
		check(value.String() == spelling && reflect.EnumValueVariant(reflect.ValueOf(value)).Name == variants[i+1].Name, "metadata and parser agree")
	}
	var zero Metadata
	check(reflect.ValueOf(zero).IsZero() && !reflect.ValueOf(Metadata.Empty).IsZero(), "known empty spelling differs from fallback zero")
	value := reflect.ValueOf(Role.Unknown("future"))
	check(reflect.EnumValueVariant(value).Name == "Unknown" && reflect.EnumValuePayload(value, 0).String() == "future", "fallback reflection payload")
	check(!reflect.EnumValuePayload(value, 0).CanSet() && !reflect.EnumValuePayload(value, 0).CanAddr(), "fallback reflection payload detached")
}

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

func genericParser() func(string) Generic[int] { return GenericAlias.Parse }

func localText(input string) string {
	type Local enum string {
		default Other(string)
		Ready = "ready"
	}
	value := Local.Parse(input)
	data, err := value.MarshalText()
	if err != nil {
		panic(err)
	}
	var decoded Local
	if err := decoded.UnmarshalText(data); err != nil || decoded != value {
		panic("local enum round trip")
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

func teacher() Role                          { return Role.Teacher }
func student() Role                          { return Role.Student }
func unknown(text string) Role               { return Role.Unknown(text) }
func parser() func(string) Role              { return Alias.Parse }
func formatRole(role Role) string            { return Role.String(role) }
func decodeText(role *Alias, b []byte) error { return (*Alias).UnmarshalText(role, b) }

func describe(role Role) string {
	return switch role {
	case Role.Unknown(text) => "unknown:" + text
	case Role.Teacher => "teacher"
	case Role.Student => "student"
	}
}
