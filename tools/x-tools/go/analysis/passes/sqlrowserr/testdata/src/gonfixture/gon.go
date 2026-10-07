package gon

import "database/sql"

func missingErrBang(db *sql.DB) error {
	rows := db.Query("")! // want `sql.Rows "rows" is used in Next loop at line [0-9]+ without final check of rows.Err\(\)`
	defer rows.Close()
	for rows.Next() {
		println(rows.Scan())
	}
	return nil
}

func missingErrOr(db *sql.DB) error {
	rows := db.QueryContext(nil, "") or err { // want `sql.Rows "rows" is used in Next loop at line [0-9]+ without final check of rows.Err\(\)`
		return err
	}
	defer rows.Close()
	for rows.Next() {
		println(rows.Scan())
	}
	return nil
}

func missingErrParenthesized(tx *sql.Tx) error {
	rows := (tx.Query(""))! // want `sql.Rows "rows" is used in Next loop at line [0-9]+ without final check of rows.Err\(\)`
	for rows.Next() {
		rows.Scan()
	}
	return nil
}

func missingErrVar(stmt *sql.Stmt) error {
	var rows = stmt.Query("")! // want `sql.Rows "rows" is used in Next loop at line [0-9]+ without final check of rows.Err\(\)`
	for rows.Next() {
		rows.Scan()
	}
	return nil
}

func okErrBang(db *sql.DB) error {
	rows := db.Query("")!
	defer rows.Close()
	for rows.Next() {
		println(rows.Scan())
	}
	rows.Err()!
	return nil
}

func okErrOr(db *sql.DB) error {
	rows := db.Query("") or err {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		println(rows.Scan())
	}
	rows.Err() or err {
		return err
	}
	return nil
}

func okNoLoop(db *sql.DB) error {
	rows := db.Query("")!
	defer rows.Close()
	return nil
}

func okEscapes(db *sql.DB) (*sql.Rows, error) {
	rows := db.Query("")!
	for rows.Next() {
		rows.Scan()
	}
	return rows, nil
}

// A handler on an unrelated call is not a query.
func okOtherCall(db *sql.DB) error {
	res := db.Exec("")!
	println(res)
	return nil
}
