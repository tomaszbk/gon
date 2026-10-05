// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sql

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"
	"uuid"
)

type gonSQLOptRole enum string {
	default Unknown(string)
	Teacher = "teacher"
	Student = "student"
}

// gonSQLOptScanner records every source its pointer-receiver Scan receives.
type gonSQLOptScanner struct {
	text  string
	scans int
}

var errGonSQLOptScan = errors.New("scan failure")

func (s *gonSQLOptScanner) Scan(src any) error {
	s.scans++
	if src == "fail" {
		return errGonSQLOptScan
	}
	s.text = fmt.Sprint(src)
	return nil
}

type gonSQLOptValuer struct{ n int }

func (v gonSQLOptValuer) Value() (driver.Value, error) { return int64(v.n * 2), nil }

func gonSQLOptSome[T any](value T) T? { return value }

func gonSQLOptAbsent(value any) bool {
	return !reflect.OptionalValuePresent(reflect.ValueOf(value).Elem())
}

func TestGonOptionalScan(t *testing.T) {
	when := time.Unix(1234, 0)
	id := uuid.MustParse("46cd2740-6081-4289-a659-03b61ebb92f7")
	for _, test := range []struct {
		name string
		src  any
		dest any
		want any
	}{
		{"int64", int64(7), new(int?), gonSQLOptSome(7)},
		{"zero int64", int64(0), new(int?), gonSQLOptSome(0)},
		{"string to int", "42", new(int?), gonSQLOptSome(42)},
		{"[]byte to int64", []byte("5"), new(int64?), gonSQLOptSome(int64(5))},
		{"float64", 1.5, new(float64?), gonSQLOptSome(1.5)},
		{"zero float64", 0.0, new(float64?), gonSQLOptSome(0.0)},
		{"string", "text", new(string?), gonSQLOptSome("text")},
		{"empty string", "", new(string?), gonSQLOptSome("")},
		{"bytes", []byte("bytes"), new(string?), gonSQLOptSome("bytes")},
		{"bytes payload", []byte("bytes"), new(([]byte)?), gonSQLOptSome([]byte("bytes"))},
		{"bool", true, new(bool?), gonSQLOptSome(true)},
		{"false", false, new(bool?), gonSQLOptSome(false)},
		{"time", when, new(time.Time?), gonSQLOptSome(when)},
		{"uuid", id.String(), new(uuid.UUID?), gonSQLOptSome(id)},
		{"any", int64(3), new(any?), gonSQLOptSome(any(int64(3)))},
		{"enum", "teacher", new(gonSQLOptRole?), gonSQLOptSome(gonSQLOptRole.Teacher)},
		{"unknown enum", "future", new(gonSQLOptRole?), gonSQLOptSome(gonSQLOptRole.Unknown("future"))},
		{"empty enum", "", new(gonSQLOptRole?), gonSQLOptSome(gonSQLOptRole.Unknown(""))},
		{"enum bytes", []byte("student"), new(gonSQLOptRole?), gonSQLOptSome(gonSQLOptRole.Student)},
	} {
		if err := convertAssign(test.dest, test.src); err != nil {
			t.Errorf("%s: %v", test.name, err)
			continue
		}
		got := reflect.ValueOf(test.dest).Elem()
		if !reflect.DeepEqual(got.Interface(), reflect.ValueOf(test.want).Interface()) {
			t.Errorf("%s: got %v; want %v", test.name, got.Interface(), test.want)
		}
	}
	// A pointer payload is allocated by the ordinary pointer rules.
	var pointer (*int)?
	if err := convertAssign(&pointer, int64(9)); err != nil {
		t.Fatal(err)
	}
	if got := reflect.ValueOf(&pointer).Elem(); !reflect.OptionalValuePresent(got) || *reflect.OptionalValuePayload(got).Interface().(*int) != 9 {
		t.Errorf("pointer payload = %v", pointer)
	}
}

func TestGonOptionalScanNull(t *testing.T) {
	number := gonSQLOptSome(5)
	text := gonSQLOptSome("x")
	role := gonSQLOptSome(gonSQLOptRole.Teacher)
	zero := gonSQLOptSome(0)
	for _, dest := range []any{&number, &text, &role, &zero} {
		if err := convertAssign(dest, nil); err != nil {
			t.Fatal(err)
		}
		if !gonSQLOptAbsent(dest) {
			t.Errorf("%T: NULL did not make the optional absent", dest)
		}
	}
	// NULL on an absent optional stays absent.
	var absent float64?
	if err := convertAssign(&absent, nil); err != nil || !gonSQLOptAbsent(&absent) {
		t.Errorf("NULL on absent = %v, %v", absent, err)
	}
	// A pointer to an optional keeps the ordinary pointer rule: NULL is nil.
	pointer := &number
	number = gonSQLOptSome(3)
	if err := convertAssign(&pointer, int64(8)); err != nil || pointer == nil || *pointer != gonSQLOptSome(8) {
		t.Errorf("pointer to optional = %v, %v", pointer, err)
	}
	if err := convertAssign(&pointer, nil); err != nil || pointer != nil {
		t.Errorf("NULL into pointer to optional = %v, %v", pointer, err)
	}
	var nilDest *int?
	if err := convertAssign(nilDest, int64(1)); !errors.Is(err, errNilPtr) {
		t.Errorf("nil destination error = %v", err)
	}
	if err := convertAssign(gonSQLOptSome(1), int64(1)); err == nil {
		t.Error("non-pointer destination accepted")
	}
}

func TestGonOptionalScanErrorLeavesDestination(t *testing.T) {
	number := gonSQLOptSome(5)
	var absent int?
	var role gonSQLOptRole? = gonSQLOptRole.Teacher
	var overflow int8?
	for _, test := range []struct {
		name string
		src  any
		dest any
		want any
	}{
		{"not a number", "abc", &number, gonSQLOptSome(5)},
		{"not a number into absent", "abc", &absent, (int?)(nil)},
		{"unsupported source", true, &number, gonSQLOptSome(5)},
		{"overflow", int64(300), &overflow, (int8?)(nil)},
		{"enum rejects numbers", int64(1), &role, gonSQLOptSome(gonSQLOptRole.Teacher)},
		{"unsupported struct", struct{}{}, &number, gonSQLOptSome(5)},
	} {
		if err := convertAssign(test.dest, test.src); err == nil {
			t.Errorf("%s: no error", test.name)
		}
		if got := reflect.ValueOf(test.dest).Elem().Interface(); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: destination changed to %v; want %v", test.name, got, test.want)
		}
	}
}

func TestGonOptionalScanNested(t *testing.T) {
	var nested (int?)?
	for _, src := range []any{int64(1), "x", nil} {
		if err := convertAssign(&nested, src); err == nil {
			t.Errorf("nested optional accepted %T", src)
		}
		if !gonSQLOptAbsent(&nested) {
			t.Errorf("nested optional changed by %T", src)
		}
	}
}

func TestGonOptionalScanRawBytes(t *testing.T) {
	var raw RawBytes?
	if err := convertAssign(&raw, []byte("x")); err == nil {
		t.Error("RawBytes payload accepted")
	}
	if err := convertAssign(&raw, nil); err == nil {
		t.Error("RawBytes payload accepted for NULL")
	}
}

func TestGonOptionalScanScanner(t *testing.T) {
	// A payload implementing Scanner is called on a temporary and is stored
	// only on success. NULL never reaches it.
	var value gonSQLOptScanner?
	if err := convertAssign(&value, int64(4)); err != nil {
		t.Fatal(err)
	}
	if got := reflect.ValueOf(&value).Elem(); !reflect.OptionalValuePresent(got) {
		t.Fatal("Scanner payload was not stored")
	} else if payload := reflect.OptionalValuePayload(got).Interface().(gonSQLOptScanner); payload.text != "4" || payload.scans != 1 {
		t.Errorf("payload = %+v", payload)
	}
	if err := convertAssign(&value, "fail"); !errors.Is(err, errGonSQLOptScan) {
		t.Errorf("Scanner error = %v", err)
	}
	if got := reflect.OptionalValuePayload(reflect.ValueOf(&value).Elem()).Interface().(gonSQLOptScanner); got.text != "4" || got.scans != 1 {
		t.Errorf("failed Scanner changed the payload: %+v", got)
	}
	if err := convertAssign(&value, nil); err != nil || !gonSQLOptAbsent(&value) {
		t.Errorf("NULL = %v, %v", value, err)
	}
	var absent gonSQLOptScanner?
	if err := convertAssign(&absent, "fail"); !errors.Is(err, errGonSQLOptScan) || !gonSQLOptAbsent(&absent) {
		t.Errorf("failed Scanner on absent = %v, %v", absent, err)
	}
}

func TestGonOptionalScanOrdinaryPayloads(t *testing.T) {
	// sql.Null[T] and pointers keep their ordinary behavior.
	var nullable Null[int]
	if err := nullable.Scan(nil); err != nil || nullable.Valid {
		t.Errorf("Null = %+v, %v", nullable, err)
	}
	var nullString NullString
	if err := convertAssign(&nullString, "x"); err != nil || !nullString.Valid {
		t.Errorf("NullString = %+v, %v", nullString, err)
	}
	var pointer *int
	if err := convertAssign(&pointer, int64(2)); err != nil || *pointer != 2 {
		t.Errorf("pointer = %v, %v", pointer, err)
	}
}

func TestGonOptionalArgument(t *testing.T) {
	var absent int?
	zero := gonSQLOptSome(0)
	var nilPointer *int?
	var nilInner *gonSQLOptRole
	nested := gonSQLOptSome(gonSQLOptSome(1))
	for _, test := range []struct {
		name string
		arg  any
		want any
		err  bool
	}{
		{"absent", absent, nil, false},
		{"zero", zero, 0, false},
		{"string", gonSQLOptSome(""), "", false},
		{"pointer to present", &zero, 0, false},
		{"pointer to absent", &absent, nil, false},
		{"nil pointer", nilPointer, nil, false},
		{"enum", gonSQLOptSome(gonSQLOptRole.Teacher), gonSQLOptRole.Teacher, false},
		{"valuer", gonSQLOptSome(gonSQLOptValuer{3}), gonSQLOptValuer{3}, false},
		{"nil pointer payload", gonSQLOptSome(nilInner), nilInner, false},
		{"nested", nested, nil, true},
	} {
		got, ok, err := optionalArgument(test.arg)
		if !ok || (err != nil) != test.err || (err == nil && !reflect.DeepEqual(got, test.want)) {
			t.Errorf("%s: optionalArgument = %#v, %v, %v; want %#v (error %v)", test.name, got, ok, err, test.want, test.err)
		}
	}
	for _, arg := range []any{1, "x", nil, gonSQLOptRole.Teacher, (*int)(nil), new(string), Null[int]{}} {
		if _, ok, _ := optionalArgument(arg); ok {
			t.Errorf("optionalArgument(%T) selected an ordinary argument", arg)
		}
	}
	if optionalDestination(new(string)) || optionalDestination(nil) || optionalDestination(&gonSQLOptScanner{}) {
		t.Error("ordinary destination selected as optional")
	}
	if !optionalDestination(new(int?)) || !optionalDestination(new(*int?)) {
		t.Error("optional destination not selected")
	}
}

// Statement arguments and results through the fake driver, with and without
// the optional driver interfaces.
func TestGonOptionalFakeDB(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		exec(t, db, "CREATE|t|id=int32,name=nullstring,age=nullint64,score=nullfloat64,uid=nulluuid")
		id := uuid.MustParse("46cd2740-6081-4289-a659-03b61ebb92f7")
		const insert = "INSERT|t|id=?,name=?,age=?,score=?,uid=?"
		var (
			noName  string?
			noAge   int64?
			noScore float64?
			noID    uuid.UUID?
		)
		// Present zero values, written with Exec.
		exec(t, db, insert, 1, gonSQLOptSome(""), gonSQLOptSome(int64(0)), gonSQLOptSome(0.0), gonSQLOptSome(uuid.UUID{}))
		// Absence, written with a prepared statement.
		stmt, err := db.Prepare(insert)
		if err != nil {
			t.Fatal(err)
		}
		defer stmt.Close()
		if _, err := stmt.Exec(2, noName, noAge, noScore, noID); err != nil {
			t.Fatal(err)
		}
		// Pointers to optionals are NULL when nil or absent.
		var nilName *string?
		if _, err := stmt.Exec(3, nilName, &noAge, &noScore, &noID); err != nil {
			t.Fatal(err)
		}
		// Present values, written in a transaction.
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(insert, 4, gonSQLOptSome("alice"), gonSQLOptSome(int64(31)), gonSQLOptSome(2.5), gonSQLOptSome(id)); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}

		type row struct {
			name  string?
			age   int64?
			score float64?
			uid   uuid.UUID?
		}
		want := map[int]row{
			1: {gonSQLOptSome(""), gonSQLOptSome(int64(0)), gonSQLOptSome(0.0), gonSQLOptSome(uuid.UUID{})},
			2: {},
			3: {},
			4: {gonSQLOptSome("alice"), gonSQLOptSome(int64(31)), gonSQLOptSome(2.5), gonSQLOptSome(id)},
		}
		for key, expected := range want {
			var got row
			// Overwrite stale values: NULL must make a present optional absent.
			got = row{gonSQLOptSome("stale"), gonSQLOptSome(int64(-1)), gonSQLOptSome(-1.0), gonSQLOptSome(uuid.UUID{1})}
			if err := db.QueryRow("SELECT|t|name,age,score,uid|id=?", key).Scan(&got.name, &got.age, &got.score, &got.uid); err != nil {
				t.Fatalf("id %d: %v", key, err)
			}
			if !reflect.DeepEqual(got, expected) {
				t.Errorf("id %d: got %+v; want %+v", key, got, expected)
			}
		}
		// An optional argument may also filter rows; absence matches nothing.
		var name string
		if err := db.QueryRow("SELECT|t|name|id=?", gonSQLOptSome(4)).Scan(&name); err != nil || name != "alice" {
			t.Errorf("filter = %q, %v", name, err)
		}
		var noID32 int?
		if err := db.QueryRow("SELECT|t|name|id=?", noID32).Scan(&name); err != ErrNoRows {
			t.Errorf("absent filter = %v", err)
		}
		// Rows.Scan with the same destinations.
		rows, err := db.Query("SELECT|t|id,name|")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		seen := 0
		for rows.Next() {
			var id int
			var name string?
			if err := rows.Scan(&id, &name); err != nil {
				t.Fatal(err)
			}
			if gonSQLOptAbsent(&name) != (id == 2 || id == 3) {
				t.Errorf("id %d: name = %v", id, name)
			}
			seen++
		}
		if err := rows.Err(); err != nil || seen != 4 {
			t.Errorf("rows = %d, %v", seen, err)
		}
	})
}

func TestGonOptionalFakeDBEnumsAndValuers(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		exec(t, db, "CREATE|t|id=int32,role=nullstring,n=nullint64")
		const insert = "INSERT|t|id=?,role=?,n=?"
		var noRole gonSQLOptRole?
		exec(t, db, insert, 1, gonSQLOptSome(gonSQLOptRole.Teacher), gonSQLOptSome(gonSQLOptValuer{21}))
		exec(t, db, insert, 2, noRole, gonSQLOptSome(gonSQLOptValuer{0}))
		exec(t, db, insert, 3, gonSQLOptSome(gonSQLOptRole.Unknown("")), nil)
		for id, want := range map[int]struct {
			role gonSQLOptRole?
			n    int64?
		}{
			1: {gonSQLOptSome(gonSQLOptRole.Teacher), gonSQLOptSome(int64(42))},
			2: {noRole, gonSQLOptSome(int64(0))},
			3: {gonSQLOptSome(gonSQLOptRole.Unknown("")), (int64?)(nil)},
		} {
			var role gonSQLOptRole? = gonSQLOptRole.Student
			var n int64? = 99
			if err := db.QueryRow("SELECT|t|role,n|id=?", id).Scan(&role, &n); err != nil {
				t.Fatal(err)
			}
			if role != want.role || n != want.n {
				t.Errorf("id %d: got %v, %v; want %v, %v", id, role, n, want.role, want.n)
			}
		}
	})
}

// gonSQLOptDriver is a minimal driver in the style of pgx's database/sql
// adapter: its NamedValueChecker accepts any argument unchanged, so
// database/sql must hand it already unwrapped optionals.
type gonSQLOptState struct {
	args    []driver.NamedValue
	value   driver.Value
	skip    bool
	columns bool
	scanned []any
}

type gonSQLOptConnector struct{ state *gonSQLOptState }

func (c gonSQLOptConnector) Connect(context.Context) (driver.Conn, error) {
	return &gonSQLOptConn{c.state}, nil
}
func (c gonSQLOptConnector) Driver() driver.Driver { return gonSQLOptDriver{} }

type gonSQLOptDriver struct{}

func (gonSQLOptDriver) Open(string) (driver.Conn, error) { return nil, errors.New("unused") }

type gonSQLOptConn struct{ state *gonSQLOptState }

func (c *gonSQLOptConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unsupported") }
func (c *gonSQLOptConn) Close() error                        { return nil }
func (c *gonSQLOptConn) Begin() (driver.Tx, error)           { return nil, errors.New("unsupported") }
func (c *gonSQLOptConn) CheckNamedValue(nv *driver.NamedValue) error {
	if c.state.skip {
		return driver.ErrSkip
	}
	return nil
}
func (c *gonSQLOptConn) ExecContext(_ context.Context, _ string, args []driver.NamedValue) (driver.Result, error) {
	c.state.args = append([]driver.NamedValue(nil), args...)
	return driver.RowsAffected(1), nil
}
func (c *gonSQLOptConn) QueryContext(_ context.Context, _ string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.args = append([]driver.NamedValue(nil), args...)
	rows := &gonSQLOptRows{value: c.state.value}
	if c.state.columns {
		return &gonSQLOptColumnRows{rows, c.state}, nil
	}
	return rows, nil
}

type gonSQLOptRows struct {
	value driver.Value
	done  bool
}

func (*gonSQLOptRows) Columns() []string { return []string{"v"} }
func (*gonSQLOptRows) Close() error      { return nil }
func (r *gonSQLOptRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = r.value
	return nil
}

// gonSQLOptColumnRows decodes columns directly into Scanners, as drivers with
// their own type system do.
type gonSQLOptColumnRows struct {
	*gonSQLOptRows
	state *gonSQLOptState
}

func (r *gonSQLOptColumnRows) NextRow() error {
	if r.done {
		return io.EOF
	}
	r.done = true
	return nil
}

func (r *gonSQLOptColumnRows) ScanColumn(_ driver.ScanContext, _ int, dest any) error {
	r.state.scanned = append(r.state.scanned, dest)
	scanner, ok := dest.(Scanner)
	if !ok {
		return fmt.Errorf("column decoder received %T, not a Scanner", dest)
	}
	return scanner.Scan(r.value)
}

func TestGonOptionalNamedValueChecker(t *testing.T) {
	state := new(gonSQLOptState)
	db := OpenDB(gonSQLOptConnector{state})
	defer db.Close()
	var absent int?
	var absentRole gonSQLOptRole?
	id := uuid.MustParse("46cd2740-6081-4289-a659-03b61ebb92f7")
	zero := gonSQLOptSome(0)
	for _, test := range []struct {
		name string
		arg  any
		want any
	}{
		{"absent", absent, nil},
		{"zero", zero, 0},
		{"pointer", &zero, 0},
		{"empty", gonSQLOptSome(""), ""},
		{"false", gonSQLOptSome(false), false},
		{"enum", gonSQLOptSome(gonSQLOptRole.Teacher), "teacher"},
		{"absent enum", absentRole, nil},
		{"uuid payload", gonSQLOptSome(id), id},
		{"valuer", gonSQLOptSome(gonSQLOptValuer{4}), gonSQLOptValuer{4}},
		{"named", Named("n", gonSQLOptSome(7)), 7},
		{"named absent", Named("n", absent), nil},
	} {
		if _, err := db.Exec("exec", test.arg); err != nil {
			t.Errorf("%s: %v", test.name, err)
			continue
		}
		if len(state.args) != 1 || !reflect.DeepEqual(state.args[0].Value, test.want) {
			t.Errorf("%s: driver received %#v; want %#v", test.name, state.args, test.want)
		}
		if _, err := db.Query("query", test.arg); err != nil {
			t.Errorf("%s: Query: %v", test.name, err)
		} else if len(state.args) != 1 || !reflect.DeepEqual(state.args[0].Value, test.want) {
			t.Errorf("%s: Query driver received %#v; want %#v", test.name, state.args, test.want)
		}
	}
	if state.args[0].Name != "n" {
		t.Errorf("name = %q", state.args[0].Name)
	}
	if _, err := db.Exec("exec", gonSQLOptSome(gonSQLOptSome(1))); err == nil {
		t.Error("nested optional argument accepted")
	}

	// Falling back to the default converter produces driver values.
	state.skip = true
	for _, test := range []struct {
		name string
		arg  any
		want any
	}{
		{"absent", absent, nil},
		{"int", gonSQLOptSome(7), int64(7)},
		{"float", gonSQLOptSome(0.5), 0.5},
		{"enum", gonSQLOptSome(gonSQLOptRole.Student), "student"},
		{"valuer", gonSQLOptSome(gonSQLOptValuer{4}), int64(8)},
		{"uuid", gonSQLOptSome(id), id.String()},
		{"pointer", &zero, int64(0)},
	} {
		if _, err := db.Exec("exec", test.arg); err != nil {
			t.Errorf("skip %s: %v", test.name, err)
			continue
		}
		if !reflect.DeepEqual(state.args[0].Value, test.want) {
			t.Errorf("skip %s: driver received %#v; want %#v", test.name, state.args[0].Value, test.want)
		}
	}
}

func TestGonOptionalColumnScanner(t *testing.T) {
	state := &gonSQLOptState{columns: true}
	db := OpenDB(gonSQLOptConnector{state})
	defer db.Close()
	stale := gonSQLOptSome(99)
	for _, test := range []struct {
		name  string
		value driver.Value
		dest  any
		want  any
	}{
		{"present", int64(3), new(int?), gonSQLOptSome(3)},
		{"null", nil, &stale, (int?)(nil)},
		{"enum", "teacher", new(gonSQLOptRole?), gonSQLOptSome(gonSQLOptRole.Teacher)},
		{"string", "", new(string?), gonSQLOptSome("")},
		{"pointer", int64(4), new(*int?), nil},
	} {
		state.value = test.value
		state.scanned = nil
		err := db.QueryRow("query").Scan(test.dest)
		if err != nil {
			t.Errorf("%s: %v", test.name, err)
			continue
		}
		if len(state.scanned) != 1 {
			t.Errorf("%s: ScanColumn called %d times", test.name, len(state.scanned))
		}
		if test.name == "pointer" {
			continue
		}
		if got := reflect.ValueOf(test.dest).Elem().Interface(); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: got %v; want %v", test.name, got, test.want)
		}
	}
	// Explicit Scanners are passed through untouched.
	state.value = int64(5)
	var scanner gonSQLOptScanner
	if err := db.QueryRow("query").Scan(&scanner); err != nil || scanner.text != "5" {
		t.Errorf("explicit Scanner = %+v, %v", scanner, err)
	}
	if _, wrapped := state.scanned[len(state.scanned)-1].(*gonSQLOptScanner); !wrapped {
		t.Errorf("explicit Scanner was wrapped: %T", state.scanned[len(state.scanned)-1])
	}
}
