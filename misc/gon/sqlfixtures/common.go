// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"slices"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
)

type Alias = Role
type OverrideRole Role

// User-defined SQL protocols take precedence over automatic text conversion.
func (value OverrideRole) Value() (driver.Value, error) { return "custom-value", nil }
func (value *OverrideRole) Scan(source any) error {
	*value = OverrideRole(parseRole("custom-scan"))
	return nil
}

func scanText(source any) (string, error) {
	switch value := source.(type) {
	case string:
		return value, nil
	case []byte:
		return string(value), nil
	default:
		return "", fmt.Errorf("cannot scan %T into Role", source)
	}
}

func check(ok bool, description string) {
	if !ok {
		panic(description)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func exercise(name, dsn string) {
	db, err := sql.Open(name, dsn)
	must(err)
	defer db.Close()
	// Keep temporary tables and prepared statements on their owning connection.
	db.SetMaxOpenConns(1)
	must(db.Ping())
	_, err = db.Exec("CREATE TEMP TABLE gon_roles (id integer PRIMARY KEY, role text NOT NULL)")
	must(err)
	for id, text := range []string{"teacher", "student", "visitor", "", "é\n:\""} {
		role := parseRole(text)
		_, err = db.Exec("INSERT INTO gon_roles (id, role) VALUES ($1, $2)", id, role)
		must(err)
		var got Role
		must(db.QueryRow("SELECT role FROM gon_roles WHERE id = $1", id).Scan(&got))
		check(got == role && got.String() == text, "direct enum round trip")
	}

	var zero Role
	var zeroText string
	must(db.QueryRow("SELECT $1::text", zero).Scan(&zeroText))
	check(zeroText == "", "zero enum writes empty text")
	alias := Alias(parseRole("teacher"))
	must(db.QueryRow("SELECT $1::text", &alias).Scan(&alias))
	check(alias.String() == "teacher", "enum alias and pointer argument")
	must(db.QueryRow("SELECT $1::text", sql.Named("", parseRole("student"))).Scan(&alias))
	check(alias.String() == "student", "wrapped named argument")
	var generic Generic[int]
	must(db.QueryRow("SELECT $1::text", parseGeneric[int]("ready")).Scan(&generic))
	check(generic.String() == "ready", "generic enum known round trip")
	must(db.QueryRow("SELECT $1::text", parseGeneric[int]("future")).Scan(&generic))
	check(generic.String() == "future", "generic enum unknown round trip")
	var bytes Role
	must(db.QueryRow("SELECT decode('76697369746f72', 'hex')").Scan(&bytes))
	check(bytes.String() == "visitor", "binary source scans through text parser")

	var missing *Role
	var nullable sql.Null[Role]
	must(db.QueryRow("SELECT $1::text", missing).Scan(&nullable))
	check(!nullable.Valid, "nil pointer writes SQL NULL")
	nullable = sql.Null[Role]{V: parseRole("teacher"), Valid: true}
	var copied sql.Null[Role]
	must(db.QueryRow("SELECT $1::text", nullable).Scan(&copied))
	check(copied.Valid && copied.V == nullable.V, "sql.Null present enum round trip")
	nullable.Valid = false
	must(db.QueryRow("SELECT $1::text", nullable).Scan(&copied))
	check(!copied.Valid && copied.V == zero, "sql.Null absence clears payload")
	var pointer *Role
	must(db.QueryRow("SELECT 'teacher'::text").Scan(&pointer))
	check(pointer != nil && pointer.String() == "teacher", "pointer scan allocates present enum")
	must(db.QueryRow("SELECT NULL::text").Scan(&pointer))
	check(pointer == nil, "pointer scan clears SQL NULL")

	unchanged := parseRole("student")
	for _, query := range []string{"SELECT NULL::text", "SELECT 9::bigint", "SELECT true", "SELECT now()"} {
		err = db.QueryRow(query).Scan(&unchanged)
		check(err != nil && unchanged == parseRole("student"), "invalid scan leaves enum unchanged")
	}
	err = db.QueryRow("SELECT 'teacher'::text").Scan((*Role)(nil))
	check(err != nil, "nil scan destination returns an error")

	var overriddenText string
	must(db.QueryRow("SELECT $1::text", OverrideRole(parseRole("teacher"))).Scan(&overriddenText))
	check(overriddenText == "custom-value", "custom Valuer takes precedence")
	var overridden OverrideRole
	must(db.QueryRow("SELECT 'ignored'::text").Scan(&overridden))
	check(Role(overridden).String() == "custom-scan", "custom Scanner takes precedence")

	stmt, err := db.Prepare("SELECT $1::text")
	must(err)
	var prepared Role
	must(stmt.QueryRow(parseRole("student")).Scan(&prepared))
	check(prepared.String() == "student", "prepared enum conversion")
	must(stmt.Close())
	tx, err := db.Begin()
	must(err)
	_, err = tx.Exec("UPDATE gon_roles SET role = $1 WHERE id = 0", parseRole("visitor"))
	must(err)
	var transactional Role
	must(tx.QueryRow("SELECT role FROM gon_roles WHERE id = 0").Scan(&transactional))
	check(transactional.String() == "visitor", "transaction enum conversion")
	must(tx.Commit())

	rows, err := db.Query("SELECT role FROM gon_roles ORDER BY id")
	must(err)
	var observed []string
	for rows.Next() {
		var role Role
		must(rows.Scan(&role))
		observed = append(observed, role.String())
	}
	must(rows.Err())
	must(rows.Close())
	check(slices.Equal(observed, []string{"visitor", "student", "visitor", "", "é\n:\""}), "multiple row scans")
	fmt.Println(name + " PASS")
}

func main() {
	dsn := os.Getenv("GON_SQL_TEST_DSN")
	check(dsn != "", "GON_SQL_TEST_DSN is required")
	for _, name := range []string{"pgx", "postgres"} {
		exercise(name, dsn)
	}
}
