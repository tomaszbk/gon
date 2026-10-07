package lib

import (
	"fmt"
	"reflect"
	"strings"
)

type Unit enum {
	Green
	default Red
	Blue
}

type Alias = Unit
type Single enum {
	default Only
}
type Flag[T any] enum {
	Ready
	default Idle
	Stopped
}
type FlagAlias[T any] = Flag[T]

func UnitDefault() Unit           { return Unit.Red }
func UnitAlternate() Unit         { return Unit.Green }
func SingleDefault() Single       { return Single.Only }
func DefaultFlag[T any]() Flag[T] { return Flag[T].Idle }
func ReadyFlag[T any]() Flag[T]   { return Flag[T].Ready }

func UnitMetadata() string {
	t := reflect.TypeFor[Unit]()
	if !reflect.IsEnum(t) || t.Kind() != reflect.Struct || t.NumField() != 0 {
		panic("private unit enum storage")
	}
	variants := reflect.EnumVariants(t)
	if len(variants) != 3 || !variants[1].Default {
		panic("default declaration order")
	}
	names := make([]string, len(variants))
	for i, variant := range variants {
		names[i] = variant.Name
		if len(variant.Fields) != 0 || variant.Record || variant.Default != (i == 1) {
			panic("unit variant metadata")
		}
	}
	variants[0].Name = "changed"
	if reflect.EnumVariants(t)[0].Name != "Green" {
		panic("detached unit metadata")
	}
	var zero Unit
	if reflect.EnumValueVariant(reflect.ValueOf(zero)).Name != "Red" || fmt.Sprint(zero) != "lib.Unit.Red" {
		panic("default metadata")
	}
	if reflect.EnumValueVariant(reflect.ValueOf(Unit.Green)).Name != "Green" || !reflect.IsEnum(reflect.TypeFor[Flag[[]byte]]()) {
		panic("small enum discriminator metadata")
	}
	if reflect.EnumValueVariant(reflect.ValueOf(ReadyFlag[[]byte]())).Name != "Ready" {
		panic("generic enum discriminator metadata")
	}
	return strings.Join(names, ",")
}
