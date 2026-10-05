package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"time"
)

// Both the old and context-aware driver APIs receive ordinary string values.
// The driver never imports reflect, Gon metadata, or the application's enum.
type store struct {
	value       driver.Value
	checks      int
	commits     int
	rollbacks   int
	acceptAny   bool
	columnScan  bool
	columnCalls int
}

type textDriver struct {
	state   *store
	context bool
}

func (d textDriver) Open(string) (driver.Conn, error) {
	c := &textConn{d.state}
	if d.context {
		return &contextConn{c}, nil
	}
	return c, nil
}

type textConn struct{ state *store }

func (c *textConn) Prepare(query string) (driver.Stmt, error) {
	return &textStmt{c.state, query}, nil
}
func (c *textConn) Close() error              { return nil }
func (c *textConn) Begin() (driver.Tx, error) { return textTx{c.state}, nil }
func (c *textConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	return c.state.exec(args)
}
func (c *textConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	return c.state.query(query, args)
}

type contextConn struct{ *textConn }

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

type textStmt struct {
	state *store
	query string
}

func (*textStmt) Close() error  { return nil }
func (*textStmt) NumInput() int { return -1 }
func (*textStmt) ColumnConverter(int) driver.ValueConverter {
	return driver.DefaultParameterConverter
}
func (s *textStmt) Exec(args []driver.Value) (driver.Result, error) { return s.state.exec(args) }
func (s *textStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.state.query(s.query, args)
}

type textTx struct{ state *store }

func (t textTx) Commit() error   { t.state.commits++; return nil }
func (t textTx) Rollback() error { t.state.rollbacks++; return nil }

func (s *store) exec(args []driver.Value) (driver.Result, error) {
	if len(args) != 1 {
		return nil, errors.New("expected exactly one parameter")
	}
	if _, ok := args[0].(string); !ok && args[0] != nil {
		return nil, fmt.Errorf("driver received non-text parameter %T", args[0])
	}
	s.value = args[0]
	return driver.RowsAffected(1), nil
}

func (s *store) query(query string, args []driver.Value) (driver.Rows, error) {
	var value driver.Value
	switch query {
	case "saved":
		value = s.value
	case "argument":
		if len(args) != 1 {
			return nil, errors.New("expected query parameter")
		}
		value = args[0]
	case "known":
		value = "teacher"
	case "unknown":
		value = "future-role"
	case "bytes":
		value = []byte("mutable-role")
	case "empty":
		value = ""
	case "null":
		value = nil
	case "integer":
		value = int64(42)
	case "float":
		value = float64(3.5)
	case "boolean":
		value = true
	case "time":
		value = time.Unix(123, 0)
	default:
		return nil, errors.New("unknown query")
	}
	rows := &textRows{value: value}
	if s.columnScan {
		return &columnRows{rows, s}, nil
	}
	return rows, nil
}

type textRows struct {
	value driver.Value
	done  bool
}

func (*textRows) Columns() []string { return []string{"role"} }
func (r *textRows) Close() error {
	if data, ok := r.value.([]byte); ok {
		for i := range data {
			data[i] = 'X'
		}
	}
	return nil
}
func (r *textRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = r.value
	return nil
}

type columnRows struct {
	*textRows
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
	check(index == 0, "column index")
	r.state.columnCalls++
	checkColumnDestination(dest)
	if scanner, ok := dest.(sql.Scanner); ok {
		return scanner.Scan(r.value)
	}
	return sql.ConvertAssign(ctx, dest, r.value)
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

var customScans int

func (Custom) Value() (driver.Value, error) { return "explicit-valuer", nil }
func (c *Custom) Scan(src any) error {
	customScans++
	*c = customKnown()
	return nil // Deliberately accept NULL, unlike the native text boundary.
}

func main() {
	for _, mode := range []string{"legacy-driver", "context-skip", "context-accept", "column-scanner"} {
		state := new(store)
		usesContext := mode != "legacy-driver"
		state.acceptAny = mode == "context-accept" || mode == "column-scanner"
		state.columnScan = mode == "column-scanner"
		sql.Register(mode, textDriver{state, usesContext})
		db, err := sql.Open(mode, "")
		require(err)
		db.SetMaxOpenConns(1)
		verify(db, state, usesContext)
		require(db.Close())
		fmt.Println(mode, "text/unknown/zero/pointers/prepared/transaction: PASS")
	}
}

func verify(db *sql.DB, state *store, usesContext bool) {
	ctx := context.Background()
	for _, role := range []Role{teacher(), student(), unknown("future-role"), unknown("")} {
		result, err := db.ExecContext(ctx, "save", role)
		require(err)
		n, err := result.RowsAffected()
		require(err)
		check(n == 1 && state.value == role.String(), "driver receives string enum's spelling")
		var decoded Alias
		require(db.QueryRowContext(ctx, "saved").Scan(&decoded))
		check(decoded == role, "direct database/sql round trip")
	}
	for _, test := range []struct{ query, want string }{
		{"known", "teacher"}, {"unknown", "future-role"}, {"bytes", "mutable-role"}, {"empty", ""},
	} {
		var role Role
		require(db.QueryRow(test.query).Scan(&role))
		check(role.String() == test.want, "text rows and detached driver byte storage")
	}
	role := teacher()
	_, err := db.Exec("save", &role)
	require(err)
	check(state.value == "teacher", "pointer parameter")
	var nilRole *Role
	_, err = db.Exec("save", nilRole)
	require(err)
	check(state.value == nil, "nil pointer parameter is SQL NULL")
	var pointer *Role
	require(db.QueryRow("unknown").Scan(&pointer))
	check(pointer != nil && pointer.String() == "future-role", "pointer destination allocated")
	require(db.QueryRow("null").Scan(&pointer))
	check(pointer == nil, "pointer destination SQL NULL")
	role = student()
	err = db.QueryRow("null").Scan(&role)
	check(err != nil && role == student(), "SQL NULL cannot replace nonnullable enum")
	var empty Role
	require(db.QueryRow("empty").Scan(&empty))
	check(empty.String() == "", "empty text remains distinct from SQL NULL")
	var g Generic[int]
	_, err = db.Exec("save", ready[int]())
	require(err)
	require(db.QueryRow("saved").Scan(&g))
	check(g == ready[int](), "generic enum parameter and destination")
	var h Generic[string]
	require(db.QueryRow("argument", generic[string]("future-generic")).Scan(&h))
	check(h.String() == "future-generic", "generic enum query parameter")
	statement, err := db.Prepare("argument")
	require(err)
	_, err = statement.Exec(unknown("prepared"))
	require(err)
	check(state.value == "prepared", "prepared parameter conversion")
	require(statement.QueryRow(student()).Scan(&role))
	check(role == student(), "prepared query and scan")
	require(statement.Close())
	tx, err := db.BeginTx(ctx, nil)
	require(err)
	_, err = tx.Exec("save", teacher())
	require(err)
	require(tx.QueryRow("saved").Scan(&role))
	check(role == teacher(), "transaction text boundary")
	require(tx.Commit())
	tx, err = db.Begin()
	require(err)
	require(tx.Rollback())
	check(state.commits == 1 && state.rollbacks == 1, "transaction lifecycle")
	if usesContext {
		_, err = db.ExecContext(ctx, "save", sql.Named("role", student()))
		require(err)
		check(state.value == "student" && state.checks > 0, "named arguments and driver ErrSkip fallback")
	}
	rows, err := db.Query("unknown")
	require(err)
	check(rows.Next(), "Rows.Next")
	require(rows.Scan(&role))
	check(role.String() == "future-role" && !rows.Next(), "Rows.Scan and EOF")
	require(rows.Err())
	require(rows.Close())
	_, err = db.Exec("save", ordinaryEnum())
	check(err != nil, "ordinary enums retain explicit SQL adapters")
	if state.columnScan {
		check(state.columnCalls > 0, "direct driver ScanColumn executed")
	}
	verifyNullable(db, state)
	verifyCustomProtocols(db, state)
	verifyRejectedInputs(db)
}

func verifyCustomProtocols(db *sql.DB, state *store) {
	_, err := db.Exec("save", customKnown())
	require(err)
	check(state.value == "explicit-valuer", "explicit Valuer takes precedence over native text")
	before := customScans
	var custom Custom
	require(db.QueryRow("null").Scan(&custom))
	check(customScans == before+1 && custom == customKnown(), "explicit Scanner receives NULL unaltered")
}

func verifyNullable(db *sql.DB, state *store) {
	var nullable sql.Null[Role]
	require(db.QueryRow("known").Scan(&nullable))
	check(nullable.Valid && nullable.V == teacher(), "sql.Null valid known value")
	_, err := db.Exec("save", nullable)
	require(err)
	check(state.value == "teacher", "sql.Null parameter conversion")
	require(db.QueryRow("null").Scan(&nullable))
	check(!nullable.Valid, "sql.Null records absence")
	_, err = db.Exec("save", nullable)
	require(err)
	check(state.value == nil, "sql.Null absence writes SQL NULL")
	require(db.QueryRow("empty").Scan(&nullable))
	check(nullable.Valid && nullable.V.String() == "", "sql.Null empty text remains present")
}

func verifyRejectedInputs(db *sql.DB) {
	for _, query := range []string{"integer", "float", "boolean", "time"} {
		role := student()
		err := db.QueryRow(query).Scan(&role)
		check(err != nil && role == student(), "non-text input rejected without mutation: "+query)
	}
	var nilRole *Role
	check(db.QueryRow("known").Scan(nilRole) != nil, "nil destination is an error")
	role := teacher()
	check(db.QueryRow("known").Scan(role) != nil, "non-pointer destination is an error")
	fmt.Println("NULL/non-text/nil destination: PASS")
}
