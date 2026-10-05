package main

import (
	"database/sql"
	"uuid"
)

// The struct is the only place that names the columns.
type Role enum string {
	default Unknown(string)
	Teacher = "teacher"
	Student = "student"
}

func tooMany(err error) bool { return err == sql.ErrTooManyRows }

func listCourses(db *sql.DB, query string) ([]course, error) {
	return sql.Collect[course](db.Query(query)!)!, nil
}

func streamCourses(db *sql.DB, query string) ([]course, error) {
	rows := db.Query(query)!
	defer rows.Close()
	courses := []course{}
	for rows.Next() {
		var item course
		rows.ScanStruct(&item)!
		courses = append(courses, item)
	}
	rows.Err()!
	return courses, nil
}

func listCoursePointers(db *sql.DB, query string) ([]*course, error) {
	return sql.Collect[*course](db.Query(query)!)!, nil
}

func listTitles(db *sql.DB, query string) ([]string, error) {
	return sql.Collect[string](db.Query(query)!)!, nil
}

func listIDs(db *sql.DB, query string) ([]uuid.UUID, error) {
	return sql.Collect[uuid.UUID](db.Query(query)!)!, nil
}

func onlyCourse(db *sql.DB, query string) (course, error) {
	return sql.CollectOne[course](db.Query(query)!)!, nil
}

func firstCourse(db *sql.DB, query string) (course, error) {
	var item course
	db.QueryRow(query).ScanStruct(&item)!
	return item, nil
}
