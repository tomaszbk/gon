package main

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
)

// Modern form: nullable columns use native optionals, with no adapters.
type Role enum string {
	default Unknown(string)
	Teacher = "teacher"
	Student = "student"
}

func role(text string) Role { return Role.Parse(text) }

func optional[T any](value opt[T]) T? {
	if !value.ok {
		return nil
	}
	return value.value
}

func unoptional[T any](value T?) opt[T] {
	return switch value {
	case nil => none[T]()
	case payload? => some(payload)
	}
}

// arg passes the native optional.
func arg[T any](value opt[T]) any { return optional(value) }

// scanInto scans into a native optional that starts out as start.
func scanInto[T any](start opt[T], scan func(dest any) error) (opt[T], error) {
	value := optional(start)
	err := scan(&value)
	return unoptional(value), err
}

func scanTuple(rows *sql.Rows) (string, string, string, string) {
	var a int?
	var b float64?
	var c string?
	var d bool?
	require(rows.Scan(&a, &b, &c, &d))
	return unoptional(a).String(), unoptional(b).String(), unoptional(c).String(), unoptional(d).String()
}

// A driver accepting arbitrary arguments must never see a native optional:
// database/sql hands it the payload or nil. It converts the payload itself.
func checkDriverValue(value any) (driver.Value, error) {
	if value != nil && reflect.IsOptional(reflect.TypeOf(value)) {
		return nil, fmt.Errorf("driver hook received native optional %T", value)
	}
	return driver.DefaultParameterConverter.ConvertValue(value)
}

// Direct column decoders receive a standard Scanner for optional destinations.
func checkColumnDestination(dest any) {
	if _, ok := dest.(sql.Scanner); ok {
		return
	}
	typ := reflect.TypeOf(dest)
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	check(typ == nil || !reflect.IsOptional(typ), "column decoder receives standard Scanner for native destination")
}

// Properties that a pointer destination does not share are modern-only.
func checkModern(db *sql.DB, state *store) {
	// A failed conversion leaves the optional unchanged.
	present := optional(some(5))
	state.row = []driver.Value{"abc"}
	check(db.QueryRow("row").Scan(&present) != nil, "conversion error")
	check(unoptional(present) == some(5), "failed conversion keeps a present optional")
	var absent int?
	check(db.QueryRow("row").Scan(&absent) != nil, "conversion error on absence")
	check(unoptional(absent) == none[int](), "failed conversion keeps an absent optional")
	var role Role? = Role.Teacher
	state.row = []driver.Value{int64(3)}
	check(db.QueryRow("row").Scan(&role) != nil, "string enum rejects numbers")
	check(unoptional(role) == some(Role.Teacher), "failed enum conversion keeps its value")
	// Nested optionals are an error in both directions.
	var nested (int?)?
	state.row = []driver.Value{int64(1)}
	check(db.QueryRow("row").Scan(&nested) != nil, "nested optional destination")
	state.row = []driver.Value{nil}
	check(db.QueryRow("row").Scan(&nested) != nil, "nested optional destination with NULL")
	var inner int? = 1
	var nestedArgument (int?)? = inner
	_, err := db.Exec("save", nestedArgument)
	check(err != nil, "nested optional argument")
	// Pointers to optionals follow the ordinary pointer rules.
	state.row = []driver.Value{int64(6)}
	var pointer *int?
	require(db.QueryRow("row").Scan(&pointer))
	check(pointer != nil && unoptional(*pointer) == some(6), "pointer to optional destination")
	state.row = []driver.Value{nil}
	require(db.QueryRow("row").Scan(&pointer))
	check(pointer == nil, "NULL clears a pointer to an optional")
	present = optional(some(2))
	_, err = db.Exec("save", &present)
	require(err)
	check(describeArgs(state) == "int64=2", "pointer to optional argument")
	var nilPointer *int?
	_, err = db.Exec("save", nilPointer)
	require(err)
	check(describeArgs(state) == "<nil>=<nil>", "nil pointer to optional argument")
	// sql.Null keeps its ordinary behavior.
	state.row = []driver.Value{nil}
	var nullable sql.Null[int]
	require(db.QueryRow("row").Scan(&nullable))
	check(!nullable.Valid, "sql.Null still records NULL")
}
