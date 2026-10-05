package main

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"uuid"
)

type maybe[T any] struct {
	value   T
	present bool
}

func some[T any](value T) maybe[T] { return maybe[T]{value, true} }
func none[T any]() maybe[T]        { return maybe[T]{} }

// Assessment preserves an application's custom JSON decoding, including old
// documents. It deliberately has no SQL Scanner method.
type Assessment struct {
	Corrections []string `json:"corrections"`
}

func (a *Assessment) UnmarshalJSON(data []byte) error {
	var stored struct {
		Corrections []string `json:"corrections"`
		Errors      []string `json:"errors"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	a.Corrections = stored.Corrections
	if a.Corrections == nil {
		a.Corrections = append([]string{}, stored.Errors...)
	}
	return nil
}

// A custom decoder that mutates before failing must never mutate the original
// SQL destination: the adapter/native fallback decodes a fresh temporary.
type Mutating struct {
	Text  string   `json:"text"`
	Items []string `json:"items"`
}

func (m *Mutating) UnmarshalJSON(data []byte) error {
	m.Text = "mutated before decoding"
	m.Items = []string{"mutated before decoding"}
	var stored struct {
		Text  string   `json:"text"`
		Items []string `json:"items"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	if stored.Text == "reject" {
		return errors.New("custom JSON rejection")
	}
	m.Text, m.Items = stored.Text, stored.Items
	return nil
}

// Explicit SQL protocols win over JSON decoding, including JSON-looking text
// and "null". UnmarshalJSON panics if a fallback incorrectly bypasses Scan.
type Explicit struct {
	Text  string
	Calls int
}

func (e *Explicit) Scan(src any) error {
	e.Calls++
	switch src := src.(type) {
	case string:
		e.Text = "scan:" + src
	case []byte:
		e.Text = "scan:" + string(src)
	default:
		return fmt.Errorf("explicit Scanner rejects %T", src)
	}
	return nil
}

func (*Explicit) UnmarshalJSON([]byte) error { panic("explicit Scanner was bypassed") }

type record struct {
	Labels     []string
	IDs        []uuid.UUID
	Counts     map[string]int
	Fixed      [2]int
	Assessment Assessment
	Explicit   Explicit
}

var firstID = uuid.MustParse("46cd2740-6081-4289-a659-03b61ebb92f7")
var secondID = uuid.MustParse("97b6a4e0-5323-43e9-82c8-88110d6686d6")

var labels = []string{"first", "quote\" slash\\\n雪", "", "NULL"}
var legacyCorrections = []string{"legacy", `escaped \`}

func populatedRow() []driver.Value {
	encoded, err := json.Marshal(labels)
	require(err)
	assessment, err := json.Marshal(map[string]any{"errors": legacyCorrections})
	require(err)
	return []driver.Value{
		encoded,
		`["46cd2740-6081-4289-a659-03b61ebb92f7","97b6a4e0-5323-43e9-82c8-88110d6686d6"]`,
		[]byte(`{"correct":2,"wrong":0}`),
		"[0,7]",
		assessment,
		[]byte(`{"custom":"protocol"}`),
	}
}

func emptyRow() []driver.Value {
	return []driver.Value{"[]", []byte("[]"), "{}", []byte("[0,0]"), `{"corrections":[]}`, "null"}
}

type store struct {
	columns     []string
	rows        [][]driver.Value
	direct      bool
	columnCalls int
	open        int
}

func (s *store) single(value driver.Value) {
	s.columns = []string{"value"}
	s.rows = [][]driver.Value{{value}}
}

type fakeDriver struct{ state *store }

func (d fakeDriver) Open(string) (driver.Conn, error) { return fakeConn{d.state}, nil }

type fakeConn struct{ state *store }

func (c fakeConn) Prepare(string) (driver.Stmt, error) { return fakeStmt{c.state}, nil }
func (fakeConn) Close() error                          { return nil }
func (fakeConn) Begin() (driver.Tx, error)             { return nil, errors.New("no transactions") }

type fakeStmt struct{ state *store }

func (fakeStmt) Close() error  { return nil }
func (fakeStmt) NumInput() int { return 0 }
func (fakeStmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, errors.New("no exec")
}
func (s fakeStmt) Query([]driver.Value) (driver.Rows, error) {
	// Every query owns driver buffers which become invalid when rows close.
	data := make([][]driver.Value, len(s.state.rows))
	for i, row := range s.state.rows {
		data[i] = append([]driver.Value(nil), row...)
		for j, value := range data[i] {
			if bytes, ok := value.([]byte); ok {
				data[i][j] = append([]byte(nil), bytes...)
			}
		}
	}
	s.state.open++
	rows := &fakeRows{state: s.state, columns: append([]string{}, s.state.columns...), rows: data, position: -1}
	if s.state.direct {
		return &columnRows{rows}, nil
	}
	return rows, nil
}

type fakeRows struct {
	state    *store
	columns  []string
	rows     [][]driver.Value
	position int
	closed   bool
}

func (r *fakeRows) Columns() []string { return r.columns }
func (r *fakeRows) Close() error {
	if !r.closed {
		r.closed = true
		r.state.open--
		for _, row := range r.rows {
			for _, value := range row {
				if data, ok := value.([]byte); ok {
					for i := range data {
						data[i] = 'X'
					}
				}
			}
		}
	}
	return nil
}
func (r *fakeRows) advance() error {
	r.position++
	if r.position >= len(r.rows) {
		return io.EOF
	}
	return nil
}
func (r *fakeRows) Next(dest []driver.Value) error {
	if err := r.advance(); err != nil {
		return err
	}
	copy(dest, r.rows[r.position])
	return nil
}

type columnRows struct{ *fakeRows }

func (*columnRows) Next([]driver.Value) error { panic("direct column driver must use NextRow") }
func (r *columnRows) NextRow() error          { return r.advance() }
func (r *columnRows) ScanColumn(ctx driver.ScanContext, index int, dest any) error {
	r.state.columnCalls++
	if scanner, ok := dest.(sql.Scanner); ok {
		return scanner.Scan(r.rows[r.position][index])
	}
	return sql.ConvertAssign(ctx, dest, r.rows[r.position][index])
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
	for _, mode := range []string{"values", "column-scanner"} {
		state := &store{direct: mode == "column-scanner"}
		sql.Register(mode, fakeDriver{state})
		db, err := sql.Open(mode, "")
		require(err)
		db.SetMaxOpenConns(1)
		verifyRecords(db, state)
		verifyOptionals(db, state)
		verifyFailures(db, state)
		verifyOrdinaryScan(db, state)
		check(state.open == 0, "every query closes its rows")
		check(!state.direct || state.columnCalls > 0, "direct column scanner ran")
		require(db.Close())
		fmt.Println(mode, "JSON columns/order/NULL/errors/protocols/lifetime: PASS")
	}
}

func verifyRecords(db *sql.DB, state *store) {
	state.columns = []string{"labels", "ids", "counts", "fixed", "assessment", "explicit"}
	state.rows = [][]driver.Value{populatedRow(), emptyRow()}
	items, err := collectRecords(db)
	require(err)
	check(len(items) == 2, "two collected JSON rows")
	check(reflect.DeepEqual(items[0].Labels, labels), "array order and escaped characters survive driver Close")
	check(reflect.DeepEqual(items[0].IDs, []uuid.UUID{firstID, secondID}), "UUID array order")
	check(reflect.DeepEqual(items[0].Counts, map[string]int{"correct": 2, "wrong": 0}), "JSON map")
	check(items[0].Fixed == [2]int{0, 7} && items[1].Fixed == [2]int{}, "fixed arrays preserve zero and nonzero elements")
	check(reflect.DeepEqual(items[0].Assessment.Corrections, legacyCorrections), "custom JSON assessment remains valid after driver Close")
	check(items[0].Explicit.Text == `scan:{"custom":"protocol"}` && items[0].Explicit.Calls == 1, "explicit Scanner keeps precedence")
	check(items[1].Labels != nil && len(items[1].Labels) == 0 && items[1].IDs != nil && len(items[1].IDs) == 0, "empty JSON arrays remain non-nil")
	check(items[1].Counts != nil && len(items[1].Counts) == 0 && items[1].Assessment.Corrections != nil, "empty map and assessment")
	check(items[1].Explicit.Text == "scan:null", "Scanner receives JSON null as text")
	state.rows = [][]driver.Value{populatedRow()}
	var single record
	require(scanRecord(db, &single))
	check(reflect.DeepEqual(single, items[0]), "Row.Scan equals collected JSON row")
	state.rows = nil
	empty, err := collectRecords(db)
	require(err)
	check(empty != nil && len(empty) == 0, "empty collection stays non-nil")
	fmt.Println("composites: ordered arrays, UUIDs, map, custom assessment, empty values, Scanner precedence")
}

func verifyOptional[T any](db *sql.DB, state *store, name string, source driver.Value, want maybe[T]) {
	for _, structScan := range []bool{false, true} {
		state.single(source)
		var zero T
		for _, start := range []maybe[T]{none[T](), some(zero)} {
			got, err := scanOptional(db, start, structScan)
			require(err)
			check(reflect.DeepEqual(got, want), fmt.Sprintf("%s (struct=%v): got %#v; want %#v", name, structScan, got, want))
		}
	}
}

func verifyOptionals(db *sql.DB, state *store) {
	verifyOptional(db, state, "SQL NULL struct", nil, none[Assessment]())
	verifyOptional(db, state, "JSON null struct", []byte(" \n null \t "), none[Assessment]())
	verifyOptional(db, state, "SQL NULL slice", nil, none[[]string]())
	verifyOptional(db, state, "JSON null slice", "null", none[[]string]())
	verifyOptional(db, state, "SQL NULL map", nil, none[map[string]int]())
	verifyOptional(db, state, "JSON null map", []byte("null"), none[map[string]int]())
	verifyOptional(db, state, "SQL NULL array", nil, none[[2]int]())
	verifyOptional(db, state, "JSON null array", "null", none[[2]int]())
	verifyOptional(db, state, "present empty slice", []byte("[]"), some([]string{}))
	verifyOptional(db, state, "present ordered slice", populatedRow()[0], some(labels))
	verifyOptional(db, state, "present UUID slice", populatedRow()[1], some([]uuid.UUID{firstID, secondID}))
	verifyOptional(db, state, "present map", populatedRow()[2], some(map[string]int{"correct": 2, "wrong": 0}))
	verifyOptional(db, state, "present assessment", `{"corrections":["now"]}`, some(Assessment{[]string{"now"}}))
	verifyOptional(db, state, "present array", "[3,4]", some([2]int{3, 4}))
	verifyOptional(db, state, "explicit Scanner JSON null", "null", some(Explicit{Text: "scan:null", Calls: 1}))
	verifyOptional(db, state, "explicit Scanner SQL NULL", nil, none[Explicit]())
	fmt.Println("optionals: SQL NULL/JSON null absent; empty/zero payloads and Scanner text present")
}

func verifyFailures(db *sql.DB, state *store) {
	kept := Mutating{Text: "kept", Items: []string{"original"}}
	for _, source := range []driver.Value{`{"text":"reject"}`, []byte(`{"text":42}`), "{", true} {
		state.single(source)
		value := kept
		err := scanMutation(db, &value)
		check(err != nil && reflect.DeepEqual(value, kept), fmt.Sprintf("failed composite decode mutates destination: %#v, %v", value, err))
		for _, structScan := range []bool{false, true} {
			for _, start := range []maybe[Mutating]{none[Mutating](), some(kept)} {
				got, err := scanOptional(db, start, structScan)
				check(err != nil && reflect.DeepEqual(got, start), "failed optional custom JSON decode preserves presence and payload")
			}
		}
	}
	state.single(`{"text":"fresh","items":["valid"]}`)
	value := kept
	require(scanMutation(db, &value))
	check(reflect.DeepEqual(value, Mutating{"fresh", []string{"valid"}}), "custom JSON decoder can succeed")
	fmt.Println("errors: malformed/type/custom failures preserve composite and optional destinations")
}

func verifyOrdinaryScan(db *sql.DB, state *store) {
	for _, destination := range []any{new([]string), new([]uuid.UUID), new(map[string]int), new(Assessment), new([2]int), new(Mutating)} {
		state.single("[]")
		check(db.QueryRow("rows").Scan(destination) != nil, fmt.Sprintf("ordinary Scan must still reject JSON into %T", destination))
	}
	fmt.Println("legacy Scan: plain Go composite destinations still reject JSON")
}
