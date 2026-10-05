// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
)

// opt describes a nullable column value independently of its representation.
// The legacy form uses pointers and the modern form native optionals.
type opt[T any] struct {
	value T
	ok    bool
}

func some[T any](value T) opt[T] { return opt[T]{value, true} }
func none[T any]() opt[T]        { return opt[T]{} }

func (o opt[T]) String() string {
	if !o.ok {
		return "-"
	}
	return fmt.Sprintf("%+v", o.value)
}

// row holds every nullable column of the test table.
type row struct {
	label  opt[string]
	age    opt[int]
	score  opt[float64]
	active opt[bool]
	data   opt[[]byte]
	seen   opt[time.Time]
	role   opt[Role]
}

func (r row) String() string {
	seen := "-"
	if r.seen.ok {
		seen = r.seen.value.UTC().Format(time.RFC3339Nano)
	}
	role := "-"
	if r.role.ok {
		role = roleText(r.role.value)
	}
	return fmt.Sprintf("label=%v age=%v score=%v active=%v data=%v seen=%s role=%s", r.label, r.age, r.score, r.active, r.data, seen, role)
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

const insert = "INSERT INTO gon_optionals (id, label, age, score, active, data, seen, role) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)"
const selection = "SELECT label, age, score, active, data, seen, role FROM gon_optionals WHERE id = $1"

func store(exec func(query string, args ...any) (sql.Result, error), id int, r row) {
	_, err := exec(insert, id, arg(r.label), arg(r.age), arg(r.score), arg(r.active), arg(r.data), arg(r.seen), arg(r.role))
	must(err)
}

func sameRow(got, want row) bool { return got.String() == want.String() }

func exercise(name, dsn string) {
	db, err := sql.Open(name, dsn)
	must(err)
	defer db.Close()
	// Keep temporary tables and prepared statements on their owning connection.
	db.SetMaxOpenConns(1)
	must(db.Ping())
	_, err = db.Exec("CREATE TEMP TABLE gon_optionals (id integer PRIMARY KEY, label text, age integer, score double precision, active boolean, data bytea, seen timestamptz, role text)")
	must(err)

	instant := time.Unix(1700000000, 123000000)
	rows := []row{
		{},
		{some(""), some(0), some(0.0), some(false), some([]byte{}), some(time.Unix(0, 0)), some(role(""))},
		{some("text"), some(42), some(2.5), some(true), some([]byte("raw")), some(instant), some(role("teacher"))},
		{label: some("only"), age: some(-7), role: some(role("future"))},
	}
	for id, want := range rows {
		store(db.Exec, id, want)
		got, err := scanRow(func(dest ...any) error { return db.QueryRow(selection, id).Scan(dest...) }, row{})
		must(err)
		check(sameRow(got, want), fmt.Sprintf("round trip %d: got %v; want %v", id, got, want))
		// NULL clears values that were already present, and values replace them.
		stale := rows[2]
		got, err = scanRow(func(dest ...any) error { return db.QueryRow(selection, id).Scan(dest...) }, stale)
		must(err)
		check(sameRow(got, want), fmt.Sprintf("replacement %d: got %v; want %v", id, got, want))
	}

	// SQL NULL arguments are visible to the server.
	var nulls int
	must(db.QueryRow("SELECT count(*) FROM gon_optionals WHERE label IS NULL AND age IS NULL AND data IS NULL AND role IS NULL").Scan(&nulls))
	check(nulls == 1, "absent optionals are SQL NULL")
	var zeroes int
	must(db.QueryRow("SELECT count(*) FROM gon_optionals WHERE label = '' AND age = 0 AND active = false AND length(data) = 0 AND role = ''").Scan(&zeroes))
	check(zeroes == 1, "present zero values are not SQL NULL")

	// Typed parameters, named arguments, prepared statements and transactions.
	var text opt[string]
	text, err = scanInto(none[string](), func(dest any) error { return db.QueryRow("SELECT $1::text", arg(some("x"))).Scan(dest) })
	must(err)
	check(text == some("x"), "optional parameter round trip")
	text, err = scanInto(some("stale"), func(dest any) error { return db.QueryRow("SELECT $1::text", arg(none[string]())).Scan(dest) })
	must(err)
	check(text == none[string](), "absent parameter is NULL")
	text, err = scanInto(none[string](), func(dest any) error {
		return db.QueryRow("SELECT $1::text", sql.Named("", arg(some("named")))).Scan(dest)
	})
	must(err)
	check(text == some("named"), "wrapped named optional argument")
	statement, err := db.Prepare("SELECT $1::integer")
	must(err)
	var number opt[int]
	number, err = scanInto(none[int](), func(dest any) error { return statement.QueryRow(arg(some(5))).Scan(dest) })
	must(err)
	check(number == some(5), "prepared optional conversion")
	number, err = scanInto(some(9), func(dest any) error { return statement.QueryRow(arg(none[int]())).Scan(dest) })
	must(err)
	check(number == none[int](), "prepared absent conversion")
	must(statement.Close())
	tx, err := db.Begin()
	must(err)
	store(tx.Exec, 10, rows[2])
	got, err := scanRow(func(dest ...any) error { return tx.QueryRow(selection, 10).Scan(dest...) }, row{})
	must(err)
	check(sameRow(got, rows[2]), "transaction optional conversion")
	must(tx.Commit())

	// Failed conversions are errors and no row is mistaken for NULL.
	for _, query := range []string{"SELECT 'abc'::text", "SELECT true", "SELECT now()"} {
		_, err = scanInto(none[int](), func(dest any) error { return db.QueryRow(query).Scan(dest) })
		check(err != nil, "invalid scan returns an error: "+query)
	}
	check(failedScanKeepsValue(db), "failed conversion keeps its destination")

	var rowsRead []string
	cursor, err := db.Query("SELECT label FROM gon_optionals ORDER BY id")
	must(err)
	for cursor.Next() {
		value, err := scanOne(cursor.Scan)
		must(err)
		rowsRead = append(rowsRead, value.String())
	}
	must(cursor.Err())
	must(cursor.Close())
	check(strings.Join(rowsRead, ",") == "-,,text,only,text", "Rows.Scan over every row: "+strings.Join(rowsRead, ","))
	fmt.Println(name + " PASS")
}

func main() {
	dsn := os.Getenv("GON_SQL_TEST_DSN")
	check(dsn != "", "GON_SQL_TEST_DSN is required")
	for _, name := range []string{"pgx", "postgres"} {
		exercise(name, dsn)
	}
}
