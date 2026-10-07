package lib

import (
	"fmt"
	"strings"
)

type Unit uint8

const (
	Green Unit = 1
	Red   Unit = 0
	Blue  Unit = 2
)

type Alias = Unit
type Single uint8
type Flag[T any] uint8
type FlagAlias[T any] = Flag[T]

func UnitDefault() Unit           { return Red }
func UnitAlternate() Unit         { return Green }
func SingleDefault() Single       { return 0 }
func DefaultFlag[T any]() Flag[T] { return 0 }
func ReadyFlag[T any]() Flag[T]   { return 1 }

func (v Unit) String() string {
	switch v {
	case Green:
		return "lib.Unit.Green"
	case Red:
		return "lib.Unit.Red"
	case Blue:
		return "lib.Unit.Blue"
	default:
		panic("invalid legacy Unit")
	}
}

// Ordinary Go exposes these variant descriptions through an explicit adapter.
func UnitMetadata() string {
	variants := []struct {
		name  string
		value Unit
	}{{"Green", Green}, {"Red", Red}, {"Blue", Blue}}
	var zero Unit
	names := make([]string, len(variants))
	defaults := 0
	for i, variant := range variants {
		names[i] = variant.name
		if variant.value == zero {
			defaults++
			if i != 1 {
				panic("default declaration order")
			}
		}
	}
	if defaults != 1 || fmt.Sprint(zero) != "lib.Unit.Red" {
		panic("default metadata")
	}
	return strings.Join(names, ",")
}
