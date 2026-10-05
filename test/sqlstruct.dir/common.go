package main

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// course repeats no column list or Scan destination order: the modern fixture
// maps it straight from the result columns, the legacy fixture lists them.
// Role is declared by each fixture. The embedded struct has an unexported type
// whose exported field is promoted, as encoding/json would.
type timestamps struct {
	CreatedAt time.Time
}

type course struct {
	ID           int
	Title        string
	Description  sql.NullString
	Role         Role
	LLMContext   string `sql:"llm_context"`
	MaxAttempts  int
	StudentCount int `sql:"students"` // the query aliases the column
	timestamps
}

const courseColumns = "id,title,description,role,llm_context,max_attempts,students,created_at"

func describe(c course) string {
	description := "NULL"
	if c.Description.Valid {
		description = fmt.Sprintf("%q", c.Description.String)
	}
	return fmt.Sprintf("course %d %s desc=%s role=%s ctx=%s attempts=%d students=%d created=%s",
		c.ID, c.Title, description, c.Role.String(), c.LLMContext, c.MaxAttempts, c.StudentCount,
		c.CreatedAt.UTC().Format(time.RFC3339))
}

var epoch = time.Unix(86400, 0)

var (
	algebra    = []driver.Value{int64(1), "Algebra", "Intro", "teacher", "ctx-1", int64(3), int64(7), epoch}
	geometry   = []driver.Value{int64(2), "Geometry", nil, "assistant", "ctx-2", int64(5), int64(9), epoch.Add(time.Hour)}
	algebraID  = "46cd2740-6081-4289-a659-03b61ebb92f7"
	geometryID = "97b6a4e0-5323-43e9-82c8-88110d6686d6"
)

type result struct {
	columns []string
	rows    [][]driver.Value
	failAt  int // Next returns an error instead of this row
}

func results() map[string]result {
	cols := strings.Split(courseColumns, ",")
	return map[string]result{
		"courses": {cols, [][]driver.Value{algebra, geometry}, -1},
		"empty":   {cols, nil, -1},
		"one":     {cols, [][]driver.Value{geometry}, -1},
		"failing": {cols, [][]driver.Value{algebra, geometry, algebra}, 1},
		"extra":   {append(append([]string{}, cols...), "cnt"), [][]driver.Value{append(append([]driver.Value{}, algebra...), int64(1))}, -1},
		"titles":  {[]string{"title"}, [][]driver.Value{{"Algebra"}, {"Geometry"}}, -1},
		"ids":     {[]string{"id"}, [][]driver.Value{{algebraID}, {geometryID}}, -1},
		"wide":    {[]string{"id", "title"}, [][]driver.Value{{int64(1), "Algebra"}}, -1},
	}
}

// The driver counts the rows that were opened and closed.
type fakeDriver struct{ open *int }

func (d fakeDriver) Open(string) (driver.Conn, error) { return fakeConn{d.open}, nil }

type fakeConn struct{ open *int }

func (c fakeConn) Prepare(query string) (driver.Stmt, error) { return fakeStmt{c.open, query}, nil }
func (fakeConn) Close() error                                { return nil }
func (fakeConn) Begin() (driver.Tx, error)                   { return nil, errors.New("no transactions") }

type fakeStmt struct {
	open  *int
	query string
}

func (fakeStmt) Close() error  { return nil }
func (fakeStmt) NumInput() int { return -1 }
func (fakeStmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, errors.New("no exec")
}
func (s fakeStmt) Query([]driver.Value) (driver.Rows, error) {
	if s.query == "broken" {
		return nil, errors.New("driver: query refused")
	}
	res, ok := results()[s.query]
	if !ok {
		return nil, fmt.Errorf("driver: unknown query %q", s.query)
	}
	*s.open++
	return &fakeRows{result: res, open: s.open, pos: -1}, nil
}

type fakeRows struct {
	result
	open   *int
	pos    int
	closed bool
}

func (r *fakeRows) Columns() []string { return r.columns }
func (r *fakeRows) Close() error {
	if !r.closed {
		r.closed = true
		*r.open--
	}
	return nil
}
func (r *fakeRows) Next(dest []driver.Value) error {
	r.pos++
	if r.pos == r.failAt {
		return errors.New("driver: connection reset")
	}
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.pos])
	return nil
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
	open := new(int)
	sql.Register("sqlstruct", fakeDriver{open})
	db, err := sql.Open("sqlstruct", "")
	require(err)
	db.SetMaxOpenConns(1)

	courses, err := listCourses(db, "courses")
	require(err)
	for _, c := range courses {
		fmt.Println(describe(c))
	}

	streamed, err := streamCourses(db, "courses")
	require(err)
	check(len(streamed) == 2 && describe(streamed[1]) == describe(courses[1]), "streamed rows equal collected rows")
	fmt.Println("streamed:", len(streamed))

	empty, err := listCourses(db, "empty")
	require(err)
	fmt.Println("empty:", len(empty), empty == nil)

	pointers, err := listCoursePointers(db, "courses")
	require(err)
	fmt.Println("pointers:", len(pointers), pointers[0] != pointers[1], describe(*pointers[1]))

	titles, err := listTitles(db, "titles")
	require(err)
	fmt.Println("titles:", titles)

	ids, err := listIDs(db, "ids")
	require(err)
	fmt.Println("ids:", len(ids), ids[0], ids[1] != ids[0])

	only, err := onlyCourse(db, "one")
	require(err)
	fmt.Println("one:", describe(only))
	_, err = onlyCourse(db, "empty")
	fmt.Println("one without rows:", errors.Is(err, sql.ErrNoRows))
	_, err = onlyCourse(db, "courses")
	fmt.Println("one with two rows:", tooMany(err))

	first, err := firstCourse(db, "courses")
	require(err)
	fmt.Println("first:", describe(first))
	_, err = firstCourse(db, "empty")
	fmt.Println("first without rows:", errors.Is(err, sql.ErrNoRows))
	_, err = firstCourse(db, "broken")
	fmt.Println("first with query error:", err)

	failed, err := listCourses(db, "failing")
	fmt.Println("iteration error:", failed == nil, err)

	// An extra column is an error in both styles; the legacy Scan notices the
	// destination count and the strict struct mapping notices the column.
	_, err = listCourses(db, "extra")
	fmt.Println("extra column:", err != nil)
	_, err = firstCourse(db, "extra")
	fmt.Println("extra column row:", err != nil)
	_, err = listTitles(db, "wide")
	fmt.Println("two columns for one value:", err != nil)

	fmt.Println("all rows closed:", *open == 0)
	check(*open == 0, "every query closed its rows")
}
