package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"uuid"
)

// opt describes an optional value independently of its representation. The
// legacy form uses pointers and the modern form native optionals.
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

// Stamp has explicit database/sql protocols that must keep precedence.
type Stamp struct {
	Text  string
	scans int
}

func (s Stamp) Value() (driver.Value, error) { return "stamp:" + s.Text, nil }
func (s *Stamp) Scan(src any) error {
	s.scans++
	if src == "fail" {
		return errors.New("stamp scan failure")
	}
	s.Text = "scanned:" + fmt.Sprint(src)
	return nil
}

// The driver never imports reflect, Gon metadata or the application's types.
// It records the values it receives and returns rows chosen by the scenario.
type store struct {
	args        []driver.Value
	row         []driver.Value
	checks      int
	commits     int
	rollbacks   int
	acceptAny   bool
	columnScan  bool
	columnCalls int
}

type testDriver struct {
	state   *store
	context bool
}

func (d testDriver) Open(string) (driver.Conn, error) {
	c := &testConn{d.state}
	if d.context {
		return &contextConn{c}, nil
	}
	return c, nil
}

type testConn struct{ state *store }

func (c *testConn) Prepare(query string) (driver.Stmt, error) {
	return &testStmt{c.state, query}, nil
}
func (c *testConn) Close() error              { return nil }
func (c *testConn) Begin() (driver.Tx, error) { return testTx{c.state}, nil }
func (c *testConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	return c.state.exec(args)
}
func (c *testConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	return c.state.query(query, args)
}

type contextConn struct{ *testConn }

func (c *contextConn) CheckNamedValue(value *driver.NamedValue) error {
	c.state.checks++
	if c.state.acceptAny {
		converted, err := checkDriverValue(value.Value)
		value.Value = converted
		return err
	}
	return driver.ErrSkip // Preserve the driver's decision to use common conversion.
}
func (c *contextConn) ExecContext(_ context.Context, _ string, args []driver.NamedValue) (driver.Result, error) {
	return c.state.exec(values(args))
}
func (c *contextConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.state.query(query, values(args))
}

func values(args []driver.NamedValue) []driver.Value {
	result := make([]driver.Value, len(args))
	for i, arg := range args {
		result[i] = arg.Value
	}
	return result
}

type testStmt struct {
	state *store
	query string
}

func (*testStmt) Close() error  { return nil }
func (*testStmt) NumInput() int { return -1 }
func (*testStmt) ColumnConverter(int) driver.ValueConverter {
	return driver.DefaultParameterConverter
}
func (s *testStmt) Exec(args []driver.Value) (driver.Result, error) { return s.state.exec(args) }
func (s *testStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.state.query(s.query, args)
}

type testTx struct{ state *store }

func (t testTx) Commit() error   { t.state.commits++; return nil }
func (t testTx) Rollback() error { t.state.rollbacks++; return nil }

func (s *store) exec(args []driver.Value) (driver.Result, error) {
	for _, arg := range args {
		switch arg.(type) {
		case nil, int64, float64, bool, []byte, string, time.Time:
		default:
			return nil, fmt.Errorf("driver received non-driver value %T", arg)
		}
	}
	s.args = append([]driver.Value(nil), args...)
	return driver.RowsAffected(int64(len(args))), nil
}

func (s *store) query(query string, args []driver.Value) (driver.Rows, error) {
	var row []driver.Value
	switch query {
	case "saved":
		row = s.args
	case "argument":
		row = args
	case "row":
		row = s.row
	default:
		return nil, errors.New("unknown query")
	}
	// The driver reuses and clobbers its byte buffers when rows are closed.
	row = append([]driver.Value(nil), row...)
	for i, value := range row {
		if data, ok := value.([]byte); ok {
			row[i] = append([]byte(nil), data...)
		}
	}
	rows := &testRows{row: row}
	if s.columnScan {
		return &columnRows{rows, s}, nil
	}
	return rows, nil
}

type testRows struct {
	row  []driver.Value
	done bool
}

func (r *testRows) Columns() []string {
	names := make([]string, len(r.row))
	for i := range names {
		names[i] = fmt.Sprintf("c%d", i)
	}
	return names
}
func (r *testRows) Close() error {
	for _, value := range r.row {
		if data, ok := value.([]byte); ok {
			for i := range data {
				data[i] = 'X'
			}
		}
	}
	return nil
}
func (r *testRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	copy(dest, r.row)
	return nil
}

type columnRows struct {
	*testRows
	state *store
}

func (r *columnRows) NextRow() error {
	if r.done {
		return io.EOF
	}
	r.done = true
	return nil
}
func (r *columnRows) ScanColumn(ctx driver.ScanContext, index int, dest any) error {
	r.state.columnCalls++
	checkColumnDestination(dest)
	if scanner, ok := dest.(sql.Scanner); ok {
		return scanner.Scan(r.row[index])
	}
	return sql.ConvertAssign(ctx, dest, r.row[index])
}

func check(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func require(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	for _, mode := range []string{"legacy-driver", "context-skip", "context-accept", "column-scanner"} {
		state := new(store)
		usesContext := mode != "legacy-driver"
		state.acceptAny = mode == "context-accept" || mode == "column-scanner"
		state.columnScan = mode == "column-scanner"
		sql.Register(mode, testDriver{state, usesContext})
		db, err := sql.Open(mode, "")
		require(err)
		db.SetMaxOpenConns(1)
		verify(db, state, usesContext)
		require(db.Close())
		fmt.Println(mode, "arguments/scans/NULL/prepared/transaction: PASS")
	}
}

// saved reports what the driver received for args, in a representation that
// does not depend on how the application spelled the optional values.
func saved(db *sql.DB, state *store, args ...any) string {
	_, err := db.Exec("save", args...)
	require(err)
	var parts []string
	for _, arg := range state.args {
		parts = append(parts, fmt.Sprintf("%T=%v", arg, arg))
	}
	return strings.Join(parts, " ")
}

func verify(db *sql.DB, state *store, usesContext bool) {
	when := time.Unix(1234, 0).UTC()
	id := uuid.MustParse("46cd2740-6081-4289-a659-03b61ebb92f7")
	for _, test := range []struct {
		name string
		arg  any
		want string
	}{
		{"absent int", arg(none[int]()), "<nil>=<nil>"},
		{"zero int", arg(some(0)), "int64=0"},
		{"int", arg(some(7)), "int64=7"},
		{"absent float", arg(none[float64]()), "<nil>=<nil>"},
		{"zero float", arg(some(0.0)), "float64=0"},
		{"empty string", arg(some("")), "string="},
		{"string", arg(some("text")), "string=text"},
		{"absent string", arg(none[string]()), "<nil>=<nil>"},
		{"false", arg(some(false)), "bool=false"},
		{"empty bytes", arg(some([]byte{})), "[]uint8=[]"},
		{"bytes", arg(some([]byte("ab"))), "[]uint8=[97 98]"},
		{"absent bytes", arg(none[[]byte]()), "<nil>=<nil>"},
		{"time", arg(some(when)), "time.Time=" + when.String()},
		{"absent time", arg(none[time.Time]()), "<nil>=<nil>"},
		{"uuid", arg(some(id)), "string=" + id.String()},
		{"zero uuid", arg(some(uuid.UUID{})), "string=00000000-0000-0000-0000-000000000000"},
		{"absent uuid", arg(none[uuid.UUID]()), "<nil>=<nil>"},
		{"role", arg(some(role("teacher"))), "string=teacher"},
		{"unknown role", arg(some(role("future"))), "string=future"},
		{"empty role", arg(some(role(""))), "string="},
		{"absent role", arg(none[Role]()), "<nil>=<nil>"},
		{"valuer", arg(some(Stamp{Text: "v"})), "string=stamp:v"},
		{"absent valuer", arg(none[Stamp]()), "<nil>=<nil>"},
	} {
		got := saved(db, state, test.arg)
		check(got == test.want, test.name+": driver received "+got+"; want "+test.want)
		fmt.Printf("%s: %s\n", test.name, got)
	}
	got := saved(db, state, arg(some(1)), arg(none[int]()), arg(some("x")), "plain", nil)
	check(got == "int64=1 <nil>=<nil> string=x string=plain <nil>=<nil>", "mixed arguments: "+got)
	fmt.Println("mixed:", got)

	// Every statement form converts arguments identically.
	statement, err := db.Prepare("argument")
	require(err)
	_, err = statement.Exec(arg(some(3)), arg(none[string]()))
	require(err)
	check(describeArgs(state) == "int64=3 <nil>=<nil>", "prepared Exec: "+describeArgs(state))
	var prepared opt[int]
	prepared, err = scanInto(none[int](), func(dest any) error { return statement.QueryRow(arg(some(8))).Scan(dest) })
	require(err)
	check(prepared == some(8), "prepared query round trip: "+prepared.String())
	require(statement.Close())
	tx, err := db.BeginTx(context.Background(), nil)
	require(err)
	_, err = tx.Exec("save", arg(some(0)), arg(none[float64]()))
	require(err)
	check(describeArgs(state) == "int64=0 <nil>=<nil>", "transaction Exec: "+describeArgs(state))
	var inTx opt[float64]
	inTx, err = scanInto(some(9.0), func(dest any) error { return tx.QueryRow("argument", arg(none[float64]())).Scan(dest) })
	require(err)
	check(inTx == none[float64](), "transaction NULL round trip: "+inTx.String())
	require(tx.Commit())
	tx, err = db.Begin()
	require(err)
	require(tx.Rollback())
	check(state.commits == 1 && state.rollbacks == 1, "transaction lifecycle")
	if usesContext {
		_, err = db.ExecContext(context.Background(), "save", sql.Named("a", arg(some(5))), sql.Named("b", arg(none[int]())))
		require(err)
		check(describeArgs(state) == "int64=5 <nil>=<nil>", "named arguments: "+describeArgs(state))
		check(state.checks > 0, "driver argument checker ran")
	}

	checkScans(db, state, when, id)
	checkProtocols(db, state)
	checkModern(db, state)
	if state.columnScan {
		check(state.columnCalls > 0, "direct driver ScanColumn executed")
	}
}

func describeArgs(state *store) string {
	var parts []string
	for _, arg := range state.args {
		parts = append(parts, fmt.Sprintf("%T=%v", arg, arg))
	}
	return strings.Join(parts, " ")
}

// scanned reads one column holding src into a destination of type T that
// starts out as start.
func scanned[T any](db *sql.DB, state *store, start opt[T], src driver.Value) (opt[T], error) {
	state.row = []driver.Value{src}
	return scanInto(start, func(dest any) error { return db.QueryRow("row").Scan(dest) })
}

func checkScan[T any](db *sql.DB, state *store, name string, src driver.Value, want opt[T]) {
	var zero T
	for _, start := range []opt[T]{none[T](), some(zero)} {
		got, err := scanned(db, state, start, src)
		check(err == nil, name+": scan: "+fmt.Sprint(err))
		check(fmt.Sprint(got) == fmt.Sprint(want), fmt.Sprintf("%s: got %v; want %v", name, got, want))
	}
	fmt.Printf("scan %s: %v\n", name, want)
}

func checkScans(db *sql.DB, state *store, when time.Time, id uuid.UUID) {
	// NULL is absence, even for a destination that held a value.
	checkScan(db, state, "NULL int", nil, none[int]())
	checkScan(db, state, "NULL float", nil, none[float64]())
	checkScan(db, state, "NULL string", nil, none[string]())
	checkScan(db, state, "NULL bool", nil, none[bool]())
	checkScan(db, state, "NULL bytes", nil, none[[]byte]())
	checkScan(db, state, "NULL time", nil, none[time.Time]())
	checkScan(db, state, "NULL uuid", nil, none[uuid.UUID]())
	checkScan(db, state, "NULL role", nil, none[Role]())
	// Zero values are present.
	checkScan(db, state, "zero int", int64(0), some(0))
	checkScan(db, state, "zero float", float64(0), some(0.0))
	checkScan(db, state, "empty string", "", some(""))
	checkScan(db, state, "false", false, some(false))
	checkScan(db, state, "empty role", "", some(role("")))
	// The normal conversion rules apply to the payload.
	checkScan(db, state, "int", int64(42), some(42))
	checkScan(db, state, "int from text", "17", some(17))
	checkScan(db, state, "int from bytes", []byte("18"), some(18))
	checkScan(db, state, "int64", int64(5), some(int64(5)))
	checkScan(db, state, "float", float64(2.5), some(2.5))
	checkScan(db, state, "float from text", "0.25", some(0.25))
	checkScan(db, state, "string", "text", some("text"))
	checkScan(db, state, "string from bytes", []byte("bytes"), some("bytes"))
	checkScan(db, state, "string from int", int64(9), some("9"))
	checkScan(db, state, "bool", true, some(true))
	checkScan(db, state, "bytes", []byte("raw"), some([]byte("raw")))
	checkScan(db, state, "time", when, some(when))
	checkScan(db, state, "uuid", id.String(), some(id))
	checkScan(db, state, "role", "teacher", some(role("teacher")))
	checkScan(db, state, "unknown role", "future", some(role("future")))
	checkScan(db, state, "role from bytes", []byte("student"), some(role("student")))
	// Failed conversions are errors for every starting state.
	for _, src := range []driver.Value{"abc", true, 1.5, when, []byte("x")} {
		_, err := scanned(db, state, none[int](), src)
		check(err != nil, fmt.Sprintf("int rejects %T", src))
		_, err = scanned(db, state, some(5), src)
		check(err != nil, fmt.Sprintf("present int rejects %T", src))
	}
	_, err := scanned(db, state, none[uuid.UUID](), "not a uuid")
	check(err != nil, "uuid rejects malformed text")
	_, err = scanned(db, state, none[time.Time](), "text")
	check(err != nil, "time rejects text")
	fmt.Println("scan errors: rejected")
	// Rows.Scan reads several optionals in one row.
	state.row = []driver.Value{int64(1), nil, "x", nil}
	rows, err := db.Query("row")
	require(err)
	check(rows.Next(), "Rows.Next")
	first, second, third, fourth := scanTuple(rows)
	check(first+" "+second+" "+third+" "+fourth == "1 - x -", "Rows.Scan: "+first+" "+second+" "+third+" "+fourth)
	check(!rows.Next(), "Rows EOF")
	require(rows.Err())
	require(rows.Close())
	fmt.Println("Rows.Scan:", first, second, third, fourth)
}

func checkProtocols(db *sql.DB, state *store) {
	// An explicit Scanner payload is called with each non-NULL source and
	// never with NULL, which is absence.
	for _, test := range []struct {
		src  driver.Value
		want string
	}{
		{nil, "-"},
		{int64(4), "scanned:4/1"},
		{"text", "scanned:text/1"},
		{"", "scanned:/1"},
	} {
		got, err := scanned(db, state, none[Stamp](), test.src)
		require(err)
		check(stampText(got) == test.want, fmt.Sprintf("Scanner payload %v: %s", test.src, stampText(got)))
		fmt.Printf("Scanner %v: %s\n", test.src, stampText(got))
	}
	_, err := scanned(db, state, none[Stamp](), "fail")
	check(err != nil, "Scanner payload error")
}

func stampText(value opt[Stamp]) string {
	if !value.ok {
		return "-"
	}
	return fmt.Sprintf("%s/%d", value.value.Text, value.value.scans)
}
