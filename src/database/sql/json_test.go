// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sql

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
	"uuid"
)

type gonJSONDocument struct {
	Corrections []string `json:"corrections"`
}

type gonJSONMutating struct{ N int }

func (v *gonJSONMutating) UnmarshalJSON([]byte) error {
	v.N = 99
	return errGonSQLOptScan
}

type gonJSONDecimal struct {
	Total int32
	Calls int
}

func (v *gonJSONDecimal) Compose(_ byte, _ bool, _ []byte, exponent int32) error {
	v.Total += exponent
	v.Calls++
	return nil
}

func (*gonJSONDecimal) UnmarshalJSON([]byte) error { panic("decimal Compose protocol was bypassed") }

func TestGonOptionalJSONScan(t *testing.T) {
	for _, src := range []any{`{"corrections":["one","two"]}`, []byte(`{"corrections":["one","two"]}`)} {
		var document gonJSONDocument?
		if err := convertAssign(&document, src); err != nil {
			t.Fatal(err)
		}
		payload := reflect.OptionalValuePayload(reflect.ValueOf(document)).Interface().(gonJSONDocument)
		if !reflect.DeepEqual(payload.Corrections, []string{"one", "two"}) {
			t.Fatalf("document = %+v", payload)
		}
		for _, null := range []any{nil, " \nnull\t", []byte("null")} {
			document = gonJSONDocument{Corrections: []string{"stale"}}
			if err := convertAssign(&document, null); err != nil || !gonSQLOptAbsent(&document) {
				t.Fatalf("null %q = %v, %v", null, document, err)
			}
		}
	}
	var values ([]int)?
	if err := convertAssign(&values, "[]"); err != nil || gonSQLOptAbsent(&values) {
		t.Fatalf("empty array must be present: %v, %v", values, err)
	}
	var mapping (map[string]int)?
	if err := convertAssign(&mapping, `{"zero":0}`); err != nil {
		t.Fatal(err)
	}
	var pointer (*gonJSONDocument)?
	if err := convertAssign(&pointer, `{"corrections":[]}`); err != nil || gonSQLOptAbsent(&pointer) {
		t.Fatalf("pointer payload = %v, %v", pointer, err)
	}
	// Byte payloads retain their raw SQL representation, including text null.
	var raw ([]byte)?
	if err := convertAssign(&raw, []byte("null")); err != nil || gonSQLOptAbsent(&raw) {
		t.Fatalf("raw bytes = %v, %v", raw, err)
	}
	var text string?
	if err := convertAssign(&text, "null"); err != nil || gonSQLOptAbsent(&text) {
		t.Fatalf("text = %v, %v", text, err)
	}
}

func TestGonOptionalJSONScanError(t *testing.T) {
	want := gonJSONDocument{Corrections: []string{"kept"}}
	var value gonJSONDocument? = want
	for _, src := range []any{`{"corrections":[1]}`, `{"corrections":`, true, "null trailing"} {
		if err := convertAssign(&value, src); err == nil {
			t.Fatalf("accepted invalid JSON source %v", src)
		}
		if got := reflect.OptionalValuePayload(reflect.ValueOf(value)).Interface(); !reflect.DeepEqual(got, want) {
			t.Fatalf("failed conversion changed payload to %+v", got)
		}
	}
	var mutating gonJSONMutating? = gonJSONMutating{N: 7}
	if err := convertAssign(&mutating, "{}"); !errors.Is(err, errGonSQLOptScan) {
		t.Fatalf("custom JSON error = %v", err)
	}
	if got := reflect.OptionalValuePayload(reflect.ValueOf(mutating)).Interface().(gonJSONMutating); got.N != 7 {
		t.Fatalf("custom decoder mutated destination: %+v", got)
	}
}

func TestGonOptionalJSONScanReferenceErrors(t *testing.T) {
	t.Run("slice", func(t *testing.T) {
		original := []int{1, 2}
		var value ([]int)? = original
		if err := convertAssign(&value, `[9,"bad"]`); err == nil {
			t.Fatal("invalid slice element accepted")
		}
		payload := reflect.OptionalValuePayload(reflect.ValueOf(value)).Interface().([]int)
		if !reflect.DeepEqual(payload, []int{1, 2}) || !reflect.DeepEqual(original, []int{1, 2}) {
			t.Fatalf("failed JSON changed slice: payload=%v, original=%v", payload, original)
		}
	})
	t.Run("map", func(t *testing.T) {
		original := map[string]int{"kept": 1}
		var value (map[string]int)? = original
		if err := convertAssign(&value, `{"kept":9,"bad":"invalid"}`); err == nil {
			t.Fatal("invalid map element accepted")
		}
		payload := reflect.OptionalValuePayload(reflect.ValueOf(value)).Interface().(map[string]int)
		want := map[string]int{"kept": 1}
		if !reflect.DeepEqual(payload, want) || !reflect.DeepEqual(original, want) {
			t.Fatalf("failed JSON changed map: payload=%v, original=%v", payload, original)
		}
	})
	t.Run("pointer", func(t *testing.T) {
		original := &gonJSONMutating{N: 7}
		var value (*gonJSONMutating)? = original
		if err := convertAssign(&value, "{}"); !errors.Is(err, errGonSQLOptScan) {
			t.Fatalf("custom pointer decoder error = %v", err)
		}
		payload := reflect.OptionalValuePayload(reflect.ValueOf(value)).Interface().(*gonJSONMutating)
		if payload != original || original.N != 7 {
			t.Fatalf("failed JSON changed pointer: payload=%+v, original=%+v", payload, original)
		}
	})
}

func TestGonOptionalJSONScanReplacesPayload(t *testing.T) {
	var document gonJSONDocument? = gonJSONDocument{Corrections: []string{"stale"}}
	if err := convertAssign(&document, "{}"); err != nil {
		t.Fatal(err)
	}
	if gonSQLOptAbsent(&document) || reflect.OptionalValuePayload(reflect.ValueOf(document)).Interface().(gonJSONDocument).Corrections != nil {
		t.Fatalf("JSON object retained stale struct fields: %v", document)
	}
	var mapping (map[string]int)? = map[string]int{"stale": 1}
	if err := convertAssign(&mapping, `{"new":2}`); err != nil {
		t.Fatal(err)
	}
	if got := reflect.OptionalValuePayload(reflect.ValueOf(mapping)).Interface(); !reflect.DeepEqual(got, map[string]int{"new": 2}) {
		t.Fatalf("JSON object retained stale map entries: %v", got)
	}
	original := &gonJSONDocument{Corrections: []string{"stale"}}
	var pointer (*gonJSONDocument)? = original
	if err := convertAssign(&pointer, "{}"); err != nil {
		t.Fatal(err)
	}
	payload := reflect.OptionalValuePayload(reflect.ValueOf(pointer)).Interface().(*gonJSONDocument)
	if payload == original || payload.Corrections != nil || !reflect.DeepEqual(original.Corrections, []string{"stale"}) {
		t.Fatalf("JSON object reused stale pointer: payload=%+v, original=%+v", payload, original)
	}
}

func TestGonJSONScanStruct(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		exec(t, db, "CREATE|documents|id=int32,document=nullstring,items=string,mapping=string")
		for id, document := range []any{nil, "null", `{"corrections":[]}`, `{"corrections":["quoted \"text\"","one,two"]}`} {
			exec(t, db, "INSERT|documents|id=?,document=?,items=?,mapping=?", id, document, `["b","a"]`, `{"n":0}`)
		}
		type row struct {
			ID       int
			Document gonJSONDocument?
			Items    []string
			Mapping  map[string]int
		}
		rows, err := db.Query("SELECT|documents|id,document,items,mapping|")
		if err != nil {
			t.Fatal(err)
		}
		items, err := Collect[row](rows)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 4 {
			t.Fatalf("rows = %d", len(items))
		}
		for _, item := range items {
			if !reflect.DeepEqual(item.Items, []string{"b", "a"}) || item.Mapping["n"] != 0 || gonSQLOptAbsent(&item.Document) != (item.ID < 2) {
				t.Fatalf("row = %+v", item)
			}
		}
	})
}

func TestGonJSONScanStructPreservesCursorConversion(t *testing.T) {
	for _, test := range []struct {
		name string
		scan func(*Rows) (*Rows, error)
	}{
		{"value", func(rows *Rows) (*Rows, error) {
			var value struct{ List Rows }
			err := rows.ScanStruct(&value)
			return &value.List, err
		}},
		{"pointer", func(rows *Rows) (*Rows, error) {
			var value struct{ List *Rows }
			err := rows.ScanStruct(&value)
			return value.List, err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			testDatabase(t, func(t *testing.T, db *DB) {
				populate(t, db, "people")
				exec(t, db, "CREATE|peoplecursor|list=table")
				exec(t, db, "INSERT|peoplecursor|list=people!name!age")
				rows, err := db.QueryContext(t.Context(), "SELECT|peoplecursor|list|")
				if err != nil {
					t.Fatal(err)
				}
				defer rows.Close()
				if !rows.Next() {
					t.Fatal("no parent rows")
				}
				cursor, err := test.scan(rows)
				if err != nil {
					t.Fatal(err)
				}
				if cursor == nil {
					t.Fatal("nil child cursor")
				}
				defer cursor.Close()
				var count int64
				for cursor.Next() {
					var name string
					var age int64
					if err := cursor.Scan(&name, &age); err != nil {
						t.Fatal(err)
					}
					count++
					if age != count {
						t.Fatalf("child age = %d; want %d", age, count)
					}
				}
				if err := cursor.Err(); err != nil || count != 3 {
					t.Fatalf("child cursor rows = %d, error = %v", count, err)
				}
			})
		})
	}
}

func TestGonJSONScanStructPreservesDecimalConversion(t *testing.T) {
	state := &gonSQLOptState{value: decFinite{exponent: 3}}
	db := OpenDB(gonSQLOptConnector{state})
	defer db.Close()
	value := struct{ V gonJSONDecimal }{V: gonJSONDecimal{Total: 7, Calls: 10}}
	if err := db.QueryRow("query").ScanStruct(&value); err != nil {
		t.Fatal(err)
	}
	if value.V.Total != 10 || value.V.Calls != 11 {
		t.Fatalf("decimal conversion lost existing receiver state: %+v", value.V)
	}
	state.value = "{}"
	if err := db.QueryRow("query").ScanStruct(&value); err == nil || value.V.Total != 10 || value.V.Calls != 11 {
		t.Fatalf("JSON bypassed decimal protocol: value=%+v, error=%v", value.V, err)
	}
	var optional gonJSONDecimal? = value.V
	if err := db.QueryRow("query").Scan(&optional); err == nil {
		t.Fatal("native optional JSON bypassed decimal protocol")
	}
	if got := reflect.OptionalValuePayload(reflect.ValueOf(optional)).Interface().(gonJSONDecimal); got != value.V {
		t.Fatalf("failed decimal optional conversion changed payload: %+v", got)
	}
}

func TestGonJSONColumnScanner(t *testing.T) {
	var ids []uuid.UUID
	const id = "46cd2740-6081-4289-a659-03b61ebb92f7"
	if err := (jsonColumnScanner{&ids}).Scan(`["` + id + `"]`); err != nil || len(ids) != 1 || ids[0].String() != id {
		t.Fatalf("uuid array = %v, %v", ids, err)
	}
	value := gonJSONDocument{Corrections: []string{"kept"}}
	if err := (jsonColumnScanner{&value}).Scan(`{"corrections":[1]}`); err == nil || value.Corrections[0] != "kept" {
		t.Fatalf("failed JSON changed destination: %+v, %v", value, err)
	}
	mutating := gonJSONMutating{N: 7}
	if err := (jsonColumnScanner{&mutating}).Scan("{}"); !errors.Is(err, errGonSQLOptScan) || mutating.N != 7 {
		t.Fatalf("custom decoder changed destination: %+v, %v", mutating, err)
	}
	// Ordinary Scan remains unchanged, including unsupported Go composites.
	if err := convertAssign(&value, `{"corrections":[]}`); err == nil {
		t.Fatal("ordinary Go composite Scan accepted JSON")
	}
	for _, typ := range []reflect.Type{reflect.TypeFor[time.Time](), reflect.TypeFor[gonSQLOptScanner](), reflect.TypeFor[gonSQLOptRole](), reflect.TypeFor[gonSQLOptRole?](), reflect.TypeFor[[]byte](), reflect.TypeFor[json.RawMessage](), reflect.TypeFor[Rows](), reflect.TypeFor[*Rows](), reflect.TypeFor[**Rows](), reflect.TypeFor[gonJSONDecimal](), reflect.TypeFor[*gonJSONDecimal](), reflect.TypeFor[**gonJSONDecimal]()} {
		if jsonColumnType(typ) {
			t.Fatalf("standard conversion type selected for JSON: %v", typ)
		}
	}
}
