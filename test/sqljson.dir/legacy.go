package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"uuid"
)

// Explicit per-field wrappers supply JSON decoding in ordinary Go. All writes
// are staged in a fresh temporary, including custom UnmarshalJSON methods.
type jsonScanner[T any] struct{ destination *T }

func (s jsonScanner[T]) Scan(source any) error {
	var data []byte
	switch source := source.(type) {
	case string:
		data = []byte(source)
	case []byte:
		data = source
	default:
		return fmt.Errorf("JSON Scanner rejects %T", source)
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*s.destination = value
	return nil
}

func destinations(item *record) []any {
	return []any{jsonScanner[[]string]{&item.Labels}, jsonScanner[[]uuid.UUID]{&item.IDs},
		jsonScanner[map[string]int]{&item.Counts}, jsonScanner[[2]int]{&item.Fixed}, jsonScanner[Assessment]{&item.Assessment}, &item.Explicit}
}

func scanRecord(db *sql.DB, item *record) error {
	return db.QueryRow("rows").Scan(destinations(item)...)
}

func collectRecords(db *sql.DB) ([]record, error) {
	rows, err := db.Query("rows")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []record{}
	for rows.Next() {
		var item record
		if err := rows.Scan(destinations(&item)...); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func scanMutation(db *sql.DB, value *Mutating) error {
	return db.QueryRow("rows").Scan(jsonScanner[Mutating]{value})
}

type optionalJSONScanner[T any] struct{ destination *maybe[T] }

func (s optionalJSONScanner[T]) Scan(source any) error {
	if source == nil {
		*s.destination = none[T]()
		return nil
	}
	var value T
	if scanner, ok := any(&value).(sql.Scanner); ok {
		if err := scanner.Scan(source); err != nil {
			return err
		}
	} else {
		var data []byte
		switch source := source.(type) {
		case string:
			data = []byte(source)
		case []byte:
			data = source
		default:
			return fmt.Errorf("optional JSON Scanner rejects %T", source)
		}
		if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
			*s.destination = none[T]()
			return nil
		}
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
	}
	*s.destination = some(value)
	return nil
}

func scanOptional[T any](db *sql.DB, start maybe[T], _ bool) (maybe[T], error) {
	value := start
	err := db.QueryRow("rows").Scan(optionalJSONScanner[T]{&value})
	return value, err
}
