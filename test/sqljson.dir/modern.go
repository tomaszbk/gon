package main

import "database/sql"

// The row's native composite fields replace all per-field SQL JSON adapters.
func scanRecord(db *sql.DB, item *record) error {
	return db.QueryRow("rows").ScanStruct(item)
}

func collectRecords(db *sql.DB) ([]record, error) {
	return sql.Collect[record](db.Query("rows")!)!, nil
}

func scanMutation(db *sql.DB, value *Mutating) error {
	row := struct{ Value Mutating }{Value: *value}
	err := db.QueryRow("rows").ScanStruct(&row)
	*value = row.Value
	return err
}

func scanOptional[T any](db *sql.DB, start maybe[T], structScan bool) (maybe[T], error) {
	var value T?
	if start.present {
		value = start.value
	}
	var err error
	if structScan {
		row := struct{ Value T? }{Value: value}
		err = db.QueryRow("rows").ScanStruct(&row)
		value = row.Value
	} else {
		err = db.QueryRow("rows").Scan(&value)
	}
	decoded := switch value {
	case nil => none[T]()
	case payload? => some(payload)
	}
	return decoded, err
}
