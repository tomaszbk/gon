// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sql

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"
)

type scanCourse struct {
	ID           int
	Title        string
	Description  NullString
	CreatedAt    time.Time
	LLMContext   string
	MaxAttempts  int
	StudentCount int
}

const scanCourseColumns = "id,title,description,created_at,llm_context,max_attempts,student_count"

var scanCourseTime = chrisBirthday

func setupScanCourses(t *testing.T, db *DB) {
	t.Helper()
	exec(t, db, "CREATE|courses|id=int32,title=string,description=nullstring,created_at=datetime,llm_context=string,max_attempts=int32,student_count=int32")
	const insert = "INSERT|courses|id=?,title=?,description=?,created_at=?,llm_context=?,max_attempts=?,student_count=?"
	exec(t, db, insert, 1, "Algebra", "Intro", scanCourseTime, "ctx-1", 3, 7)
	exec(t, db, insert, 2, "Geometry", nil, scanCourseTime.Add(time.Hour), "ctx-2", 5, 9)
}

func wantScanCourse(id int) scanCourse {
	if id == 1 {
		return scanCourse{1, "Algebra", NullString{"Intro", true}, scanCourseTime, "ctx-1", 3, 7}
	}
	return scanCourse{2, "Geometry", NullString{}, scanCourseTime.Add(time.Hour), "ctx-2", 5, 9}
}

func sameCourse(a, b scanCourse) bool {
	return a.ID == b.ID && a.Title == b.Title && a.Description == b.Description &&
		a.CreatedAt.Equal(b.CreatedAt) && a.LLMContext == b.LLMContext &&
		a.MaxAttempts == b.MaxAttempts && a.StudentCount == b.StudentCount
}

func mustQuery(t *testing.T, db *DB, query string, args ...any) *Rows {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("Query(%q): %v", query, err)
	}
	return rows
}

func wantErrorContaining(t *testing.T, err error, parts ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("got nil error; want one containing %q", parts)
	}
	for _, part := range parts {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("error %q does not contain %q", err, part)
		}
	}
}

func TestScanStructMatchesNames(t *testing.T) { testDatabase(t, testScanStructMatchesNames) }
func testScanStructMatchesNames(t *testing.T, db *DB) {
	setupScanCourses(t, db)
	rows := mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|")
	defer rows.Close()
	for id := 1; rows.Next(); id++ {
		var got scanCourse
		if err := rows.ScanStruct(&got); err != nil {
			t.Fatal(err)
		}
		if want := wantScanCourse(id); !sameCourse(got, want) {
			t.Fatalf("row %d = %+v; want %+v", id, got, want)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestScanStructColumnOrderAndSubset(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		// Fewer columns, in a different order than the fields. Fields without a
		// column keep their current values.
		got := scanCourse{ID: 99, Title: "keep", MaxAttempts: 42}
		err := db.QueryRow("SELECT|courses|student_count,llm_context|id=?", 1).ScanStruct(&got)
		if err != nil {
			t.Fatal(err)
		}
		want := scanCourse{ID: 99, Title: "keep", MaxAttempts: 42, StudentCount: 7, LLMContext: "ctx-1"}
		if !sameCourse(got, want) {
			t.Fatalf("got %+v; want %+v", got, want)
		}
	})
}

func TestScanStructTags(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		type tagged struct {
			Heading string `sql:"title"`
			Title   string // would match by name, but the tag wins
			Number  int    `sql:"id"`
			Skipped int    `sql:"-"`
			Context string `sql:"llm_context"`
		}
		got := tagged{Title: "preset", Skipped: 5}
		if err := db.QueryRow("SELECT|courses|id,title,llm_context|id=?", 2).ScanStruct(&got); err != nil {
			t.Fatal(err)
		}
		want := tagged{Heading: "Geometry", Title: "preset", Number: 2, Skipped: 5, Context: "ctx-2"}
		if got != want {
			t.Fatalf("got %+v; want %+v", got, want)
		}

		// A tag is exact and replaces the field name for matching.
		type exact struct {
			Title string `sql:"Title"`
		}
		err := db.QueryRow("SELECT|courses|title|id=?", 1).ScanStruct(new(exact))
		wantErrorContaining(t, err, `column "title"`, "no matching field", "sql.exact")
		type renamed struct {
			Title string `sql:"heading"`
		}
		err = db.QueryRow("SELECT|courses|title|id=?", 1).ScanStruct(new(renamed))
		wantErrorContaining(t, err, `column "title"`, "no matching field")

		// A dash excludes a field entirely, so its column is unmapped.
		type excluded struct {
			ID    int `sql:"-"`
			Title string
		}
		var ex excluded
		if err := db.QueryRow("SELECT|courses|title|id=?", 1).ScanStruct(&ex); err != nil || ex.Title != "Algebra" {
			t.Fatalf("excluded field with no column: %+v, %v", ex, err)
		}
		err = db.QueryRow("SELECT|courses|id,title|id=?", 1).ScanStruct(&ex)
		wantErrorContaining(t, err, `column "id"`, "no matching field")

		// Options are reserved.
		type options struct {
			ID int `sql:"id,pk"`
		}
		err = db.QueryRow("SELECT|courses|id|id=?", 1).ScanStruct(new(options))
		wantErrorContaining(t, err, "sql.options", "field ID", "options after a comma")
	})
}

type scanBase struct {
	ID    int
	Title string
}

type ScanAudit struct {
	CreatedAt   time.Time
	MaxAttempts int
}

type scanLower struct {
	StudentCount int
}

type scanEmbedded struct {
	scanBase // unexported embedded struct promotes its exported fields
	*ScanAudit
	scanLower
	Title       string // depth 0 beats scanBase.Title
	Description NullString
	LLMContext  string
}

func TestScanStructEmbedded(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		var got scanEmbedded
		err := db.QueryRow("SELECT|courses|"+scanCourseColumns+"|id=?", 1).ScanStruct(&got)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != 1 || got.Title != "Algebra" || got.scanBase.Title != "" || got.StudentCount != 7 ||
			got.ScanAudit == nil || got.MaxAttempts != 3 || !got.CreatedAt.Equal(scanCourseTime) ||
			got.Description != (NullString{"Intro", true}) || got.LLMContext != "ctx-1" {
			t.Fatalf("got %+v", got)
		}

		// The embedded pointer is allocated only when a column maps into it.
		var partial scanEmbedded
		err = db.QueryRow("SELECT|courses|id,title|id=?", 2).ScanStruct(&partial)
		if err != nil || partial.ID != 2 || partial.ScanAudit != nil {
			t.Fatalf("partial = %+v, %v", partial, err)
		}
		// An existing embedded pointer is reused.
		audit := &ScanAudit{MaxAttempts: 1}
		partial = scanEmbedded{ScanAudit: audit}
		err = db.QueryRow("SELECT|courses|max_attempts|id=?", 2).ScanStruct(&partial)
		if err != nil || partial.ScanAudit != audit || audit.MaxAttempts != 5 {
			t.Fatalf("reused pointer: %+v, %v", partial, err)
		}
	})
}

type scanLeft struct{ ID, Title int }
type scanRight struct{ Title, Context int }

type scanAmbiguous struct {
	scanLeft
	scanRight
}

type scanTagDepth struct {
	scanBase
	Key int `sql:"id"` // a tag at depth 0 wins over scanBase.ID by name
}

type scanDeepTag struct {
	Name int // by name only, at depth 0
	scanDeepInner
}

type scanDeepInner struct {
	Other int `sql:"name"` // a tag at depth 1 beats Name at depth 0
}

func TestScanStructEmbeddedAmbiguityAndPrecedence(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		// A tie at the same depth fails only for a column that is present.
		var amb scanAmbiguous
		err := db.QueryRow("SELECT|courses|id|id=?", 1).ScanStruct(&amb)
		if err != nil || amb.scanLeft.ID != 1 {
			t.Fatalf("unambiguous column: %+v, %v", amb, err)
		}
		err = db.QueryRow("SELECT|courses|id,title|id=?", 1).ScanStruct(&amb)
		wantErrorContaining(t, err, `column "title"`, "ambiguous", "scanLeft.Title", "scanRight.Title")

		var td scanTagDepth
		if err := db.QueryRow("SELECT|courses|id,title|id=?", 2).ScanStruct(&td); err != nil {
			t.Fatal(err)
		}
		if td.Key != 2 || td.scanBase.ID != 0 || td.scanBase.Title != "Geometry" {
			t.Fatalf("tag precedence: %+v", td)
		}
	})
}

func TestScanStructTagBeatsShallowName(t *testing.T) {
	// Tag matching takes precedence over name matching regardless of depth.
	st := structTypeOf(reflect.TypeFor[scanDeepTag]())
	if st.err != nil {
		t.Fatal(st.err)
	}
	if m := st.byTag["name"]; m.field == nil || m.field.path != "scanDeepInner.Other" {
		t.Fatalf("tag index = %+v", m)
	}
	cols := []string{"name"}
	plan, err := newStructPlan(reflect.TypeFor[scanDeepTag](), cols, "test")
	if err != nil {
		t.Fatal(err)
	}
	if plan.fields[0].path != "scanDeepInner.Other" {
		t.Fatalf("column name mapped to %s; want the tagged scanDeepInner.Other", plan.fields[0].path)
	}
}

func TestScanStructStrictMapping(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		type small struct {
			ID    int
			Title string
		}
		got := small{ID: 5, Title: "before"}
		err := db.QueryRow("SELECT|courses|id,title,max_attempts|id=?", 1).ScanStruct(&got)
		wantErrorContaining(t, err, `column "max_attempts"`, "no matching field", "sql.small")
		if got != (small{5, "before"}) {
			t.Fatalf("a mapping error changed the destination: %+v", got)
		}

		// Duplicate column names in the result.
		err = db.QueryRow("SELECT|courses|title,title|id=?", 1).ScanStruct(&got)
		wantErrorContaining(t, err, "duplicate column", `"title"`)

		// Different columns that normalize to one field.
		exec(t, db, "CREATE|twins|created_at=string,createdat=string")
		exec(t, db, "INSERT|twins|created_at=a,createdat=b")
		type twin struct{ CreatedAt string }
		err = db.QueryRow("SELECT|twins|created_at,createdat|").ScanStruct(new(twin))
		wantErrorContaining(t, err, `"created_at"`, `"createdat"`, "both map to field CreatedAt")

		// Names compare case-insensitively without underscores, both ways.
		type shouty struct{ CREATED_AT, CreatedAT string }
		err = db.QueryRow("SELECT|twins|created_at|").ScanStruct(new(shouty))
		wantErrorContaining(t, err, "ambiguous", "CREATED_AT", "CreatedAT")
	})
}

func TestScanStructDestinationErrors(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		var integer int
		var nilCourse *scanCourse
		for _, dest := range []any{nil, scanCourse{}, nilCourse, &integer, new(*scanCourse)} {
			err := db.QueryRow("SELECT|courses|id|id=?", 1).ScanStruct(dest)
			wantErrorContaining(t, err, "destination must be a non-nil pointer to a struct")
		}
		// Native enums and optionals are not plain structs.
		role := gonSQLRole.Student
		err := db.QueryRow("SELECT|courses|title|id=?", 1).ScanStruct(&role)
		wantErrorContaining(t, err, "destination must be a non-nil pointer to a struct")
		rows := mustQuery(t, db, "SELECT|courses|id|id=?", 1)
		defer rows.Close()
		rows.Next()
		wantErrorContaining(t, rows.ScanStruct(nil), "Rows.ScanStruct destination")
	})
}

func TestRowsScanStructPreconditions(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		var got scanCourse
		rows := mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|")
		wantErrorContaining(t, rows.ScanStruct(&got), "ScanStruct called without calling Next")
		// A failed call must not poison later ones.
		if !rows.Next() {
			t.Fatal("no rows")
		}
		if err := rows.ScanStruct(&got); err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
		}
		if err := rows.ScanStruct(&got); !errors.Is(err, errRowsClosed) {
			t.Fatalf("after the last row: %v; want %v", err, errRowsClosed)
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}

		rows = mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|")
		rows.Next()
		if err := rows.ScanStruct(&got); err != nil { // warm the cached plan
			t.Fatal(err)
		}
		rows.Close()
		if err := rows.ScanStruct(&got); !errors.Is(err, errRowsClosed) {
			t.Fatalf("closed Rows with a cached plan: %v; want %v", err, errRowsClosed)
		}
		rows = mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|")
		rows.Close()
		if err := rows.ScanStruct(&got); !errors.Is(err, errRowsClosed) {
			t.Fatalf("closed Rows: %v; want %v", err, errRowsClosed)
		}
	})
}

func TestScanStructPlanIsPerResultSet(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		rows := mustQuery(t, db, "SELECT|courses|id,title|;SELECT|courses|llm_context,student_count|")
		defer rows.Close()
		type first struct {
			ID    int
			Title string
		}
		type second struct {
			LLMContext   string
			StudentCount int
		}
		var plan *structPlan
		var firsts []first
		for rows.Next() {
			var v first
			if err := rows.ScanStruct(&v); err != nil {
				t.Fatal(err)
			}
			if plan == nil {
				plan = rows.scanPlan
			} else if rows.scanPlan != plan {
				t.Fatal("the column mapping was recomputed for the same result set")
			}
			firsts = append(firsts, v)
		}
		if !reflect.DeepEqual(firsts, []first{{1, "Algebra"}, {2, "Geometry"}}) {
			t.Fatalf("first set = %+v", firsts)
		}
		if !rows.NextResultSet() {
			t.Fatalf("no second result set: %v", rows.Err())
		}
		if rows.scanPlan != nil {
			t.Fatal("NextResultSet kept the mapping of the previous result set")
		}
		wantErrorContaining(t, rows.ScanStruct(new(second)), "ScanStruct called without calling Next")
		var seconds []second
		for rows.Next() {
			var v second
			if err := rows.ScanStruct(&v); err != nil {
				t.Fatal(err)
			}
			seconds = append(seconds, v)
		}
		if !reflect.DeepEqual(seconds, []second{{"ctx-1", 7}, {"ctx-2", 9}}) {
			t.Fatalf("second set = %+v", seconds)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}

		// The same Rows may switch destination types; the mapping follows.
		rows2 := mustQuery(t, db, "SELECT|courses|id,title|")
		defer rows2.Close()
		rows2.Next()
		var f first
		var s struct {
			ID    int32
			Title string
		}
		if err := rows2.ScanStruct(&f); err != nil {
			t.Fatal(err)
		}
		rows2.Next()
		if err := rows2.ScanStruct(&s); err != nil || s.ID != 2 || s.Title != "Geometry" {
			t.Fatalf("second destination type: %+v, %v", s, err)
		}
	})
}

func TestRowScanStruct(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		var got scanCourse
		if err := db.QueryRow("SELECT|courses|"+scanCourseColumns+"|id=?", 2).ScanStruct(&got); err != nil {
			t.Fatal(err)
		}
		if want := wantScanCourse(2); !sameCourse(got, want) {
			t.Fatalf("got %+v; want %+v", got, want)
		}

		// With several matching rows the first is used, like Row.Scan.
		got = scanCourse{}
		if err := db.QueryRow("SELECT|courses|" + scanCourseColumns + "|").ScanStruct(&got); err != nil {
			t.Fatal(err)
		}
		if want := wantScanCourse(1); !sameCourse(got, want) {
			t.Fatalf("first row = %+v; want %+v", got, want)
		}

		// No row.
		err := db.QueryRow("SELECT|courses|"+scanCourseColumns+"|id=?", 99).ScanStruct(&got)
		if err != ErrNoRows {
			t.Fatalf("no rows: %v; want %v", err, ErrNoRows)
		}

		// The query error is propagated.
		err = db.QueryRow("SELECT|nosuchtable|id|").ScanStruct(&got)
		wantErrorContaining(t, err, "nosuchtable")
		r := db.QueryRow("SELECT|nosuchtable|id|")
		if r.Err() == nil || r.ScanStruct(&got) != r.Err() {
			t.Fatalf("deferred query error was not returned: %v", r.Err())
		}

		// The rows are closed, also on mapping errors.
		type tiny struct{ ID int }
		row := db.QueryRow("SELECT|courses|id,title|id=?", 1)
		wantErrorContaining(t, row.ScanStruct(&tiny{}), `column "title"`)
		if !getRowsCursor(row.rows).closed {
			t.Fatal("Row.ScanStruct left the rows open after an error")
		}
		row = db.QueryRow("SELECT|courses|id|id=?", 1)
		if err := row.ScanStruct(&tiny{}); err != nil {
			t.Fatal(err)
		}
		if !getRowsCursor(row.rows).closed {
			t.Fatal("Row.ScanStruct left the rows open")
		}
		if stats := db.Stats(); stats.InUse != 0 {
			t.Fatalf("%d connections still in use", stats.InUse)
		}

		// The same restrictions as Row.Scan apply to RawBytes.
		type raw struct{ Title RawBytes }
		wantErrorContaining(t, db.QueryRow("SELECT|courses|title|id=?", 1).ScanStruct(&raw{}), "RawBytes isn't allowed on Row.ScanStruct")
	})
}

func TestRowsScanStructRawBytes(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		type raw struct {
			Title RawBytes
			ID    int
		}
		rows := mustQuery(t, db, "SELECT|courses|title,id|")
		defer rows.Close()
		rows.Next()
		var v raw
		if err := rows.ScanStruct(&v); err != nil {
			t.Fatal(err)
		}
		if string(v.Title) != "Algebra" || v.ID != 1 {
			t.Fatalf("got %q, %d", v.Title, v.ID)
		}
		// As with Scan, the next call to Next releases the memory.
		if !rows.Next() {
			t.Fatal("no second row")
		}
		if err := rows.ScanStruct(&v); err != nil || string(v.Title) != "Geometry" {
			t.Fatalf("second row: %q, %v", v.Title, err)
		}
	})
}

func TestScanStructNullAndScannerFields(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		exec(t, db, "CREATE|mixed|id=int32,note=nullstring,seen=nulldatetime,ref=nullstring,token=string,count=nullint32,uid=uuid")
		id := uuid.MustParse("46cd2740-6081-4289-a659-03b61ebb92f7")
		const insert = "INSERT|mixed|id=?,note=?,seen=?,ref=?,token=?,count=?,uid=?"
		exec(t, db, insert, 1, "hello", scanCourseTime, "r1", "secret", 4, id)
		exec(t, db, insert, 2, nil, nil, nil, "other", nil, id)

		type mixed struct {
			ID    int
			Note  NullString
			Seen  NullTime
			Ref   *string
			Token scanToken
			Count Null[int32]
			UID   uuid.UUID
		}
		rows := mustQuery(t, db, "SELECT|mixed|id,note,seen,ref,token,count,uid|")
		defer rows.Close()
		var got []mixed
		for rows.Next() {
			var m mixed
			if err := rows.ScanStruct(&m); err != nil {
				t.Fatal(err)
			}
			got = append(got, m)
		}
		if len(got) != 2 {
			t.Fatalf("got %d rows", len(got))
		}
		one, two := got[0], got[1]
		if one.Note != (NullString{"hello", true}) || !one.Seen.Valid || !one.Seen.Time.Equal(scanCourseTime) ||
			one.Ref == nil || *one.Ref != "r1" || one.Token.scans != 1 || one.Token.value != "secret" ||
			one.Count != (Null[int32]{4, true}) || one.UID != id {
			t.Fatalf("row 1 = %+v", one)
		}
		if two.Note.Valid || two.Seen.Valid || two.Ref != nil || two.Token.value != "other" || two.Count.Valid {
			t.Fatalf("row 2 = %+v", two)
		}
	})
}

// scanToken records that its own Scan method was used.
type scanToken struct {
	value string
	scans int
}

func (s *scanToken) Scan(src any) error {
	s.scans++
	switch v := src.(type) {
	case string:
		s.value = v
	case []byte:
		s.value = string(v)
	default:
		return errors.New("scanToken: unsupported source")
	}
	return nil
}

func TestScanStructPropagatesScannerErrors(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		exec(t, db, "CREATE|tokens|id=int32,token=nullstring")
		exec(t, db, "INSERT|tokens|id=?,token=?", 1, nil)
		type row struct {
			ID    int
			Token scanToken
		}
		err := db.QueryRow("SELECT|tokens|id,token|").ScanStruct(new(row))
		wantErrorContaining(t, err, `column index 1, name "token"`, "scanToken: unsupported source")
	})
}

func TestScanStructStringEnums(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		exec(t, db, "CREATE|members|id=int32,role=string,alt=nullstring,maybe=nullstring")
		exec(t, db, "INSERT|members|id=?,role=?,alt=?,maybe=?", 1, "teacher", "student", "future")
		exec(t, db, "INSERT|members|id=?,role=?,alt=?,maybe=?", 2, "other", nil, nil)
		type member struct {
			ID    int
			Role  gonSQLRole
			Alt   *gonSQLRole
			Maybe Null[gonSQLRole]
		}
		members, err := Collect[member](mustQuery(t, db, "SELECT|members|id,role,alt,maybe|"))
		if err != nil {
			t.Fatal(err)
		}
		if len(members) != 2 {
			t.Fatalf("got %d members", len(members))
		}
		first, second := members[0], members[1]
		if first.Role != gonSQLRole.Teacher || first.Alt == nil || *first.Alt != gonSQLRole.Student ||
			!first.Maybe.Valid || first.Maybe.V != gonSQLRole.Unknown("future") {
			t.Fatalf("first = %+v", first)
		}
		if second.Role != gonSQLRole.Unknown("other") || second.Alt != nil || second.Maybe.Valid {
			t.Fatalf("second = %+v", second)
		}

		// A native string enum is one column, not a struct to map.
		roles, err := Collect[gonSQLRole](mustQuery(t, db, "SELECT|members|role|"))
		if err != nil || len(roles) != 2 || roles[0] != gonSQLRole.Teacher || roles[1] != gonSQLRole.Unknown("other") {
			t.Fatalf("Collect[enum] = %v, %v", roles, err)
		}
		one, err := CollectOne[gonSQLGeneric[int]](mustQuery(t, db, "SELECT|members|alt|id=?", 1))
		if err != nil || one != gonSQLGeneric[int].Unknown("student") {
			t.Fatalf("CollectOne[generic enum] = %v, %v", one, err)
		}
	})
}

type gonSQLMaybeInt = int?

// scanEmbedsScanner implements Scanner through the embedded type.
type scanEmbedsScanner struct {
	NullString
	ID int
}

type ScanBase = scanBase

type scanTaggedEmbed struct {
	ScanBase `sql:"base"`
	Title    string
}

type ScanRole = gonSQLRole

type scanCycle struct {
	*scanCycle
	ID int
}

func TestScanStructIndexesSpecialTypes(t *testing.T) {
	for _, test := range []struct {
		typ    reflect.Type
		target bool
	}{
		{reflect.TypeFor[scanCourse](), true},
		{reflect.TypeFor[struct{}](), true},
		{reflect.TypeFor[time.Time](), false},
		{reflect.TypeFor[NullString](), false},
		{reflect.TypeFor[Null[int]](), false},
		{reflect.TypeFor[scanToken](), false},
		{reflect.TypeFor[uuid.UUID](), false},
		{reflect.TypeFor[string](), false},
		{reflect.TypeFor[gonSQLRole](), false},
		{reflect.TypeFor[gonSQLAlias](), false},
		{reflect.TypeFor[gonSQLGeneric[string]](), false},
		{reflect.TypeFor[gonSQLMaybeInt](), false},
		{reflect.TypeFor[*scanCourse](), false}, // Collect unwraps one pointer itself
		{reflect.TypeFor[scanEmbedsScanner](), false},
	} {
		if got := isStructTarget(test.typ); got != test.target {
			t.Errorf("isStructTarget(%s) = %v; want %v", test.typ, got, test.target)
		}
	}
	// Self-referential embedding is bounded.
	st := structTypeOf(reflect.TypeFor[scanCycle]())
	if st.err != nil || st.byName["id"].field == nil || len(st.byName) != 1 {
		t.Fatalf("cycle index = %+v", st)
	}
	if structTypeOf(reflect.TypeFor[scanCycle]()) != st {
		t.Fatal("the type index is not cached")
	}
	// Native enum and optional fields are leaves, even when embedded.
	type withEnum struct {
		ScanRole
		Maybe gonSQLMaybeInt
	}
	enumType := structTypeOf(reflect.TypeFor[withEnum]())
	if enumType.byName["scanrole"].field == nil || enumType.byName["maybe"].field == nil || len(enumType.byName) != 2 {
		t.Fatalf("enum/optional fields were flattened: %+v", enumType)
	}
	// Embedded Scanners and embedded structs with a tag are leaves, not flattened.
	scanner := structTypeOf(reflect.TypeFor[scanEmbedsScanner]())
	if scanner.byName["nullstring"].field == nil || scanner.byName["id"].field == nil || len(scanner.byName) != 2 {
		t.Fatalf("embedded Scanner was flattened: %+v", scanner)
	}
	tagged := structTypeOf(reflect.TypeFor[scanTaggedEmbed]())
	if f := tagged.byTag["base"].field; f == nil || len(tagged.byName) != 1 || tagged.byName["title"].field == nil {
		t.Fatalf("tagged embedded struct was flattened: %+v", tagged)
	}
	// An embedded pointer to an unexported struct cannot be allocated and is ignored.
	type hidden struct {
		*scanBase
		Title string
	}
	if hiddenType := structTypeOf(reflect.TypeFor[hidden]()); len(hiddenType.byName) != 1 || hiddenType.byName["title"].field == nil {
		t.Fatalf("unexported embedded pointer was indexed: %+v", hiddenType)
	}
}

func TestCollectStructs(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		courses, err := Collect[scanCourse](mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|"))
		if err != nil {
			t.Fatal(err)
		}
		if len(courses) != 2 || !sameCourse(courses[0], wantScanCourse(1)) || !sameCourse(courses[1], wantScanCourse(2)) {
			t.Fatalf("Collect = %+v", courses)
		}
		pointers, err := Collect[*scanCourse](mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|"))
		if err != nil {
			t.Fatal(err)
		}
		if len(pointers) != 2 || pointers[0] == pointers[1] || !sameCourse(*pointers[0], wantScanCourse(1)) || !sameCourse(*pointers[1], wantScanCourse(2)) {
			t.Fatalf("Collect pointers = %+v", pointers)
		}
		// Collect only reads the remaining rows.
		rows := mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|")
		rows.Next()
		rest, err := Collect[scanCourse](rows)
		if err != nil || len(rest) != 1 || rest[0].ID != 2 {
			t.Fatalf("remaining rows = %+v, %v", rest, err)
		}
	})
}

func TestCollectEmptyIsNotNil(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		for name, got := range map[string]func() (any, error){
			"struct": func() (any, error) {
				return Collect[scanCourse](mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|id=?", 99))
			},
			"pointer": func() (any, error) {
				return Collect[*scanCourse](mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|id=?", 99))
			},
			"scalar": func() (any, error) {
				return Collect[string](mustQuery(t, db, "SELECT|courses|title|id=?", 99))
			},
		} {
			slice, err := got()
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			v := reflect.ValueOf(slice)
			if v.IsNil() || v.Len() != 0 {
				t.Fatalf("%s: got %#v; want a non-nil empty slice", name, slice)
			}
		}
	})
}

func TestCollectScalars(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		titles, err := Collect[string](mustQuery(t, db, "SELECT|courses|title|"))
		if err != nil || !reflect.DeepEqual(titles, []string{"Algebra", "Geometry"}) {
			t.Fatalf("Collect[string] = %v, %v", titles, err)
		}
		ids, err := Collect[int64](mustQuery(t, db, "SELECT|courses|id|"))
		if err != nil || !reflect.DeepEqual(ids, []int64{1, 2}) {
			t.Fatalf("Collect[int64] = %v, %v", ids, err)
		}
		bytesList, err := Collect[[]byte](mustQuery(t, db, "SELECT|courses|title|"))
		if err != nil || len(bytesList) != 2 || string(bytesList[1]) != "Geometry" {
			t.Fatalf("Collect[[]byte] = %q, %v", bytesList, err)
		}
		anyList, err := Collect[any](mustQuery(t, db, "SELECT|courses|id|"))
		if err != nil || len(anyList) != 2 {
			t.Fatalf("Collect[any] = %v, %v", anyList, err)
		}
		descriptions, err := Collect[NullString](mustQuery(t, db, "SELECT|courses|description|"))
		if err != nil || !reflect.DeepEqual(descriptions, []NullString{{"Intro", true}, {}}) {
			t.Fatalf("Collect[NullString] = %v, %v", descriptions, err)
		}
		nullable, err := Collect[Null[string]](mustQuery(t, db, "SELECT|courses|description|"))
		if err != nil || !reflect.DeepEqual(nullable, []Null[string]{{"Intro", true}, {}}) {
			t.Fatalf("Collect[Null[string]] = %v, %v", nullable, err)
		}
		pointers, err := Collect[*string](mustQuery(t, db, "SELECT|courses|description|"))
		if err != nil || len(pointers) != 2 || pointers[0] == nil || *pointers[0] != "Intro" || pointers[1] != nil {
			t.Fatalf("Collect[*string] = %v, %v", pointers, err)
		}
		times, err := Collect[time.Time](mustQuery(t, db, "SELECT|courses|created_at|"))
		if err != nil || len(times) != 2 || !times[0].Equal(scanCourseTime) {
			t.Fatalf("Collect[time.Time] = %v, %v", times, err)
		}

		exec(t, db, "CREATE|ids|uid=uuid")
		a, b := uuid.MustParse("46cd2740-6081-4289-a659-03b61ebb92f7"), uuid.MustParse("97b6a4e0-5323-43e9-82c8-88110d6686d6")
		exec(t, db, "INSERT|ids|uid=?", a)
		exec(t, db, "INSERT|ids|uid=?", b)
		uuids, err := Collect[uuid.UUID](mustQuery(t, db, "SELECT|ids|uid|"))
		if err != nil || !reflect.DeepEqual(uuids, []uuid.UUID{a, b}) {
			t.Fatalf("Collect[uuid.UUID] = %v, %v", uuids, err)
		}

		// A scalar target needs exactly one column.
		_, err = Collect[string](mustQuery(t, db, "SELECT|courses|id,title|"))
		wantErrorContaining(t, err, "Collect into string", "2 columns, want exactly 1")
		_, err = CollectOne[int](mustQuery(t, db, "SELECT|courses|id,title|id=?", 1))
		wantErrorContaining(t, err, "CollectOne into int", "2 columns")
		// Retained RawBytes memory would be reused by the next row.
		_, err = Collect[RawBytes](mustQuery(t, db, "SELECT|courses|title|"))
		wantErrorContaining(t, err, "RawBytes isn't allowed on Collect")
		type withRaw struct{ Title RawBytes }
		_, err = Collect[withRaw](mustQuery(t, db, "SELECT|courses|title|"))
		wantErrorContaining(t, err, "RawBytes isn't allowed on Collect")
		_, err = CollectOne[*withRaw](mustQuery(t, db, "SELECT|courses|title|id=?", 1))
		wantErrorContaining(t, err, "RawBytes isn't allowed on CollectOne")
	})
}

func TestCollectMappingErrors(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		type small struct{ ID int }
		got, err := Collect[small](mustQuery(t, db, "SELECT|courses|id,title|"))
		wantErrorContaining(t, err, "Collect:", `column "title"`, "sql.small")
		if got != nil {
			t.Fatalf("got %v with an error", got)
		}
		_, err = Collect[*small](mustQuery(t, db, "SELECT|courses|id,title|"))
		wantErrorContaining(t, err, `column "title"`)
		_, err = CollectOne[small](mustQuery(t, db, "SELECT|courses|id,title|id=?", 1))
		wantErrorContaining(t, err, "CollectOne:", `column "title"`)
	})
}

func TestCollectClosesRowsAndReportsErrors(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		type idOnly struct{ ID int }

		rows := mustQuery(t, db, "SELECT|courses|id|")
		cursor := getRowsCursor(rows)
		if _, err := Collect[idOnly](rows); err != nil || !cursor.closed {
			t.Fatalf("success: closed=%v err=%v", cursor.closed, err)
		}

		// A mapping failure closes the rows, too.
		rows = mustQuery(t, db, "SELECT|courses|id,title|")
		cursor = getRowsCursor(rows)
		if _, err := Collect[idOnly](rows); err == nil || !cursor.closed {
			t.Fatalf("mapping failure: closed=%v err=%v", cursor.closed, err)
		}

		// An error while iterating is returned, with no partial result.
		fail := errors.New("fail after one row")
		rows = mustQuery(t, db, "SELECT|courses|id|")
		cursor = getRowsCursor(rows)
		cursor.errPos, cursor.err = 1, fail
		got, err := Collect[idOnly](rows)
		if err != fail || got != nil || !cursor.closed {
			t.Fatalf("iteration error: %v, %v, closed=%v", got, err, cursor.closed)
		}

		// So is the error of closing the rows after the last one.
		rows = mustQuery(t, db, "SELECT|courses|id|")
		cursor = getRowsCursor(rows)
		cursor.closeErr = errors.New("rowsCursor: failed to close")
		if _, err := Collect[idOnly](rows); err != cursor.closeErr {
			t.Fatalf("close error: %v; want %v", err, cursor.closeErr)
		}

		// The count of rows and Err of a nil Rows.
		if _, err := Collect[idOnly](nil); err == nil {
			t.Fatal("Collect(nil) returned no error")
		}
		if _, err := CollectOne[idOnly](nil); err == nil {
			t.Fatal("CollectOne(nil) returned no error")
		}

		// The connection is returned to the pool.
		if stats := db.Stats(); stats.InUse != 0 {
			t.Fatalf("%d connections still in use", stats.InUse)
		}
	})
}

func TestCollectOne(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		got, err := CollectOne[scanCourse](mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|id=?", 2))
		if err != nil || !sameCourse(got, wantScanCourse(2)) {
			t.Fatalf("one row = %+v, %v", got, err)
		}
		pointer, err := CollectOne[*scanCourse](mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|id=?", 1))
		if err != nil || pointer == nil || !sameCourse(*pointer, wantScanCourse(1)) {
			t.Fatalf("one pointer = %+v, %v", pointer, err)
		}
		title, err := CollectOne[string](mustQuery(t, db, "SELECT|courses|title|id=?", 1))
		if err != nil || title != "Algebra" {
			t.Fatalf("one scalar = %q, %v", title, err)
		}

		// No rows.
		rows := mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|id=?", 99)
		cursor := getRowsCursor(rows)
		zero, err := CollectOne[scanCourse](rows)
		if err != ErrNoRows || !sameCourse(zero, scanCourse{}) || !cursor.closed {
			t.Fatalf("no rows = %+v, %v, closed=%v", zero, err, cursor.closed)
		}
		if p, err := CollectOne[*scanCourse](mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|id=?", 99)); err != ErrNoRows || p != nil {
			t.Fatalf("no rows pointer = %v, %v", p, err)
		}

		// More than one row.
		rows = mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|")
		cursor = getRowsCursor(rows)
		zero, err = CollectOne[scanCourse](rows)
		if err != ErrTooManyRows || !sameCourse(zero, scanCourse{}) || !cursor.closed {
			t.Fatalf("two rows = %+v, %v, closed=%v", zero, err, cursor.closed)
		}
		if _, err := CollectOne[string](mustQuery(t, db, "SELECT|courses|title|")); !errors.Is(err, ErrTooManyRows) {
			t.Fatalf("two scalar rows: %v", err)
		}

		// Errors while fetching take precedence over the row count.
		fail := errors.New("fetch failed")
		rows = mustQuery(t, db, "SELECT|courses|"+scanCourseColumns+"|")
		cursor = getRowsCursor(rows)
		cursor.errPos, cursor.err = 0, fail
		if _, err := CollectOne[scanCourse](rows); err != fail {
			t.Fatalf("error on the first row: %v; want %v", err, fail)
		}
		rows = mustQuery(t, db, "SELECT|courses|id|id=?", 1)
		cursor = getRowsCursor(rows)
		cursor.errPos, cursor.err = 1, fail
		if _, err := CollectOne[int](rows); err != fail {
			t.Fatalf("error on the second row: %v; want %v", err, fail)
		}

		// A scan failure of the one row is not a row-count error.
		_, err = CollectOne[int](mustQuery(t, db, "SELECT|courses|title|id=?", 1))
		wantErrorContaining(t, err, "Scan error on column index 0")
		if stats := db.Stats(); stats.InUse != 0 {
			t.Fatalf("%d connections still in use", stats.InUse)
		}
	})
}

func TestCollectWithTransactionAndContext(t *testing.T) {
	testDatabase(t, func(t *testing.T, db *DB) {
		setupScanCourses(t, db)
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		rows, err := tx.Query("SELECT|courses|"+scanCourseColumns+"|id=?", 1)
		if err != nil {
			t.Fatal(err)
		}
		course, err := CollectOne[scanCourse](rows)
		if err != nil || course.ID != 1 {
			t.Fatalf("in transaction: %+v, %v", course, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	})
}

type benchCourse struct {
	ID           int
	Title        string
	Description  NullString
	LLMContext   string
	MaxAttempts  int
	StudentCount int
}

const benchCourseColumns = "id,title,description,llm_context,max_attempts,student_count"

func benchCourses(b *testing.B, rows int) *DB {
	db := newTestDB(b, "")
	exec(b, db, "CREATE|courses|id=int32,title=string,description=nullstring,llm_context=string,max_attempts=int32,student_count=int32")
	for i := range rows {
		exec(b, db, "INSERT|courses|id=?,title=?,description=?,llm_context=?,max_attempts=?,student_count=?", i, "Algebra", "Intro", "context", 3, 7)
	}
	return db
}

func BenchmarkScanStruct(b *testing.B) {
	const rowCount = 100
	db := benchCourses(b, rowCount)
	query := "SELECT|courses|" + benchCourseColumns + "|"
	b.Run("Rows/Scan", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			rows, err := db.Query(query)
			if err != nil {
				b.Fatal(err)
			}
			var items []benchCourse
			for rows.Next() {
				var c benchCourse
				if err := rows.Scan(&c.ID, &c.Title, &c.Description, &c.LLMContext, &c.MaxAttempts, &c.StudentCount); err != nil {
					b.Fatal(err)
				}
				items = append(items, c)
			}
			if err := rows.Err(); err != nil || len(items) != rowCount {
				b.Fatal(err, len(items))
			}
		}
	})
	b.Run("Rows/ScanStruct", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			rows, err := db.Query(query)
			if err != nil {
				b.Fatal(err)
			}
			var items []benchCourse
			for rows.Next() {
				var c benchCourse
				if err := rows.ScanStruct(&c); err != nil {
					b.Fatal(err)
				}
				items = append(items, c)
			}
			if err := rows.Err(); err != nil || len(items) != rowCount {
				b.Fatal(err, len(items))
			}
		}
	})
	b.Run("Collect", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			rows, err := db.Query(query)
			if err != nil {
				b.Fatal(err)
			}
			items, err := Collect[benchCourse](rows)
			if err != nil || len(items) != rowCount {
				b.Fatal(err, len(items))
			}
		}
	})
	b.Run("Row/Scan", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var c benchCourse
			err := db.QueryRow(query).Scan(&c.ID, &c.Title, &c.Description, &c.LLMContext, &c.MaxAttempts, &c.StudentCount)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Row/ScanStruct", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var c benchCourse
			if err := db.QueryRow(query).ScanStruct(&c); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestScanStructWideResults(t *testing.T) {
	// More columns than manyColumns use maps to find duplicates.
	const width = manyColumns + 8
	names := make([]string, width)
	specs := make([]string, width)
	fields := make([]reflect.StructField, width)
	for i := range width {
		names[i] = "c" + strconv.Itoa(i)
		specs[i] = names[i] + "=int32"
		fields[i] = reflect.StructField{Name: "C" + strconv.Itoa(i), Type: reflect.TypeFor[int]()}
	}
	typ := reflect.StructOf(fields)
	testDatabase(t, func(t *testing.T, db *DB) {
		exec(t, db, "CREATE|wide|"+strings.Join(specs, ",")+",c_3=int32")
		args := make([]any, width)
		for i := range args {
			args[i] = i * 10
		}
		exec(t, db, "INSERT|wide|"+strings.Join(names, "=?,")+"=?", args...)
		dest := reflect.New(typ)
		if err := db.QueryRow("SELECT|wide|" + strings.Join(names, ",") + "|").ScanStruct(dest.Interface()); err != nil {
			t.Fatal(err)
		}
		for i := range width {
			if got := dest.Elem().Field(i).Int(); got != int64(i*10) {
				t.Fatalf("field %d = %d", i, got)
			}
		}
		err := db.QueryRow("SELECT|wide|" + strings.Join(names, ",") + ",c3|").ScanStruct(dest.Interface())
		wantErrorContaining(t, err, "duplicate column", `"c3"`, "indexes 3 and "+strconv.Itoa(width))
		// Different spellings of one name map to one field.
		err = db.QueryRow("SELECT|wide|" + strings.Join(names, ",") + ",c_3|").ScanStruct(dest.Interface())
		wantErrorContaining(t, err, `"c3" and "c_3" both map to field C3`)
	})
}
