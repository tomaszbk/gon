package main

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
)

func checkDriverValue(value any) (driver.Value, error) {
	if valuer, ok := value.(driver.Valuer); ok {
		return valuer.Value()
	}
	if _, ok := value.(string); ok || value == nil {
		return value, nil
	}
	return nil, fmt.Errorf("driver hook received %T instead of common string", value)
}
func checkColumnDestination(dest any) {
	if _, ok := dest.(sql.Scanner); ok {
		return
	}
	typ := reflect.TypeOf(dest)
	if typ == nil || typ.Kind() != reflect.Pointer {
		return
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	check(!reflect.IsStringEnum(typ), "column decoder receives standard Scanner for native destination")
}

type Role enum string {
	default Unknown(string)
	Teacher = "teacher"
	Student = "student"
}

type Alias = Role

type Custom Role

type Generic[T any] enum string {
	default Unknown(string)
	Ready = "ready"
}

func teacher() Role                      { return Role.Teacher }
func student() Role                      { return Role.Student }
func unknown(text string) Role           { return Role.Unknown(text) }
func ready[T any]() Generic[T]           { return Generic[T].Ready }
func generic[T any](s string) Generic[T] { return Generic[T].Parse(s) }
func customKnown() Custom                { return Custom.Teacher }

func ordinaryEnum() any {
	type Ordinary enum {
		default Missing
		Value(string)
	}
	return Ordinary.Value("other")
}
