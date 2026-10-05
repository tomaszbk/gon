package main

import (
	"database/sql"
	"errors"
	"uuid"
)

// Legacy code names every column three times: in the struct, in the query and
// in the Scan destinations.
type Role string

func (r Role) String() string { return string(r) }

func scanCourse(row interface{ Scan(...any) error }) (course, error) {
	var item course
	err := row.Scan(&item.ID, &item.Title, &item.Description, &item.Role, &item.LLMContext,
		&item.MaxAttempts, &item.StudentCount, &item.CreatedAt)
	return item, err
}

var errTooMany = errors.New("more than one row")

func tooMany(err error) bool { return errors.Is(err, errTooMany) }

func listCourses(db *sql.DB, query string) ([]course, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	courses := []course{}
	for rows.Next() {
		item, err := scanCourse(rows)
		if err != nil {
			return nil, err
		}
		courses = append(courses, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return courses, nil
}

func streamCourses(db *sql.DB, query string) ([]course, error) {
	return listCourses(db, query)
}

func listCoursePointers(db *sql.DB, query string) ([]*course, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	courses := []*course{}
	for rows.Next() {
		item, err := scanCourse(rows)
		if err != nil {
			return nil, err
		}
		courses = append(courses, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return courses, nil
}

func listTitles(db *sql.DB, query string) ([]string, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	titles := []string{}
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		titles = append(titles, title)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return titles, nil
}

func listIDs(db *sql.DB, query string) ([]uuid.UUID, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func onlyCourse(db *sql.DB, query string) (course, error) {
	rows, err := db.Query(query)
	if err != nil {
		return course{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return course{}, err
		}
		return course{}, sql.ErrNoRows
	}
	item, err := scanCourse(rows)
	if err != nil {
		return course{}, err
	}
	if rows.Next() {
		return course{}, errTooMany
	}
	if err := rows.Err(); err != nil {
		return course{}, err
	}
	return item, nil
}

func firstCourse(db *sql.DB, query string) (course, error) {
	return scanCourse(db.QueryRow(query))
}
