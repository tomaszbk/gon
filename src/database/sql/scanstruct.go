// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sql

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ErrTooManyRows is returned by [CollectOne] when the result contains more
// than one row. It is the counterpart of [ErrNoRows].
var ErrTooManyRows = errors.New("sql: more than one row in result")

// ScanStruct copies the columns of the current row into the struct pointed to
// by dest. It is the struct counterpart of [Rows.Scan]: it has the same
// preconditions and failure modes, including that [Rows.Next] must have been
// called, and every column value goes through the same conversion as Scan, so
// [Scanner] fields, [Null] fields, pointers, native string enums, [time.Time],
// [*uuid.UUID] and other destination types behave exactly as they do there.
// Composite fields (structs, maps, arrays and slices other than byte slices)
// also accept JSON string or []byte columns when the ordinary conversion is
// unsupported. JSON decoding uses a fresh value, so an error leaves the field
// unchanged. Explicit Scanners keep precedence. JSON null is absence for a
// native optional composite field; ordinary Scan into Go composites retains
// its existing conversion rules.
//
// dest must be a non-nil pointer to a struct. Columns are matched to fields by
// name, using the column names reported by [Rows.Columns]:
//
//   - A field whose tag is `sql:"name"` receives the column called exactly
//     name (case-sensitive). `sql:"-"` excludes the field. A tagged field is
//     known only by its tag; the tag value is the whole column name and
//     options after a comma are not supported (and are reported as an error).
//     A tag match always takes precedence over a match by field name.
//   - Otherwise an exported field matches the column whose name equals the
//     field name when both are compared case-insensitively with underscores
//     removed: created_at matches CreatedAt, llm_context matches LLMContext,
//     id matches ID and student_count matches StudentCount.
//   - Fields of embedded (anonymous) struct types are flattened as if they
//     were declared in the outer struct, following the rules of encoding/json:
//     among fields that match one column, the one at the shallowest embedding
//     depth wins, and an unresolved tie at the same depth is an error that is
//     reported only if the column is present in the result. An embedded
//     struct with a `sql:"name"` tag is an ordinary field instead, and so are
//     embedded types that are time.Time, implement [Scanner], or are native
//     enums or optionals. An embedded pointer to a struct is allocated when
//     any column maps into it, whether or not the value is NULL; embedded
//     pointers to unexported struct types cannot be set and are ignored.
//   - Unexported fields are ignored.
//
// The mapping is strict. Every column must map to exactly one field, and no
// two columns may map to the same field, otherwise ScanStruct returns an error
// that names the column and the destination type before anything is stored.
// This catches typos and missing aliases such as an unaliased count(*).
// Duplicate column names in the result are an error. Fields without a matching
// column keep their current value.
//
// The column-to-field mapping is computed once per result set and reused by
// later calls to ScanStruct, [Collect] and [CollectOne] on the same [Rows]; the
// per-type field index is cached for the life of the process.
//
// As with [Rows.Scan], a [RawBytes] field is valid only until the next call to
// [Rows.Next], [Rows.NextResultSet] or [Rows.Close].
func (rs *Rows) ScanStruct(dest any) error {
	return rs.scanStruct(dest, "Rows.ScanStruct", true)
}

// ScanStruct copies the columns of the matched row into the struct pointed to
// by dest. See [Rows.ScanStruct] for the mapping rules. Like [Row.Scan], it
// returns the deferred query error if there is one, uses the first row and
// discards the rest, returns [ErrNoRows] if no row matches, and closes the
// underlying [Rows]. A struct with a [RawBytes] field is an error.
func (r *Row) ScanStruct(dest any) error {
	if r.err != nil {
		return r.err
	}
	defer r.rows.Close()
	if !r.rows.Next() {
		if err := r.rows.Err(); err != nil {
			return err
		}
		return ErrNoRows
	}
	if err := r.rows.scanStruct(dest, "Row.ScanStruct", false); err != nil {
		return err
	}
	// Make sure the query can be processed to completion with no errors.
	return r.rows.Close()
}

// Collect scans all remaining rows of rows into a slice of T and closes rows
// before returning. It reports the error from [Rows.Err], and returns a
// non-nil empty slice, not nil, when there are no rows. On error the slice is
// nil. Only the current result set is read.
//
// If T is a struct type, or a pointer to a struct type whose elements are
// allocated, each row is mapped with the rules of [Rows.ScanStruct], and
// mapping errors are reported the same way. Any other T receives exactly one
// column through the ordinary conversion of [Rows.Scan], so Collect[string],
// Collect[uuid.UUID], Collect[*int] and Collect[sql.Null[int]] read a single
// column and report an error if the query returns more.
//
// A struct type is also read as a single column when database/sql already
// converts it: time.Time, a native enum or optional, and any struct type
// whose pointer implements [Scanner], such as [NullString] and [Null]. That
// rule applies to a struct that implements Scanner by embedding a type with a
// Scan method, too; call [Rows.ScanStruct] in a loop to map such a type.
//
// T may not be [RawBytes], and a struct mapped by Collect may not have
// [RawBytes] fields, because the memory would be reused by the next row.
// A nil rows is reported as an error.
func Collect[T any](rows *Rows) ([]T, error) {
	if rows == nil {
		return nil, errors.New("sql: Collect called with nil Rows")
	}
	defer rows.Close()
	reader := newRowReader[T]("Collect")
	out := []T{}
	for rows.Next() {
		var zero T
		out = append(out, zero)
		if err := reader.scan(rows, &out[len(out)-1]); err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// CollectOne scans the only row of rows into a value of T and closes rows
// before returning. T is mapped as described for [Collect]. It returns
// [ErrNoRows] if there are no rows and [ErrTooManyRows] if there is more than
// one, and reports an error from [Rows.Err] encountered while reading. The
// value is the zero T on error. Only the current result set is read.
//
// Use it for statements that return exactly one row, such as an INSERT with
// RETURNING; use [Row.ScanStruct] or [Row.Scan] when a first row of many is
// acceptable.
func CollectOne[T any](rows *Rows) (T, error) {
	var zero T
	if rows == nil {
		return zero, errors.New("sql: CollectOne called with nil Rows")
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return zero, err
		}
		return zero, ErrNoRows
	}
	var value T
	if err := newRowReader[T]("CollectOne").scan(rows, &value); err != nil {
		return zero, err
	}
	if rows.Next() {
		return zero, ErrTooManyRows
	}
	if err := rows.Err(); err != nil {
		return zero, err
	}
	return value, nil
}

type rowMode uint8

const (
	rowScalar        rowMode = iota // one column through Rows.Scan
	rowStruct                       // T is a struct
	rowStructPointer                // T is a pointer to a struct; allocate
)

// rowReader scans one row into a *T for Collect and CollectOne.
type rowReader[T any] struct {
	op      string
	mode    rowMode
	typ     reflect.Type // T
	elem    reflect.Type // the struct for rowStructPointer
	checked bool         // the scalar column count was validated
}

func newRowReader[T any](op string) *rowReader[T] {
	typ := reflect.TypeFor[T]()
	r := &rowReader[T]{op: op, typ: typ}
	switch {
	case isStructTarget(typ):
		r.mode = rowStruct
	case typ.Kind() == reflect.Pointer && isStructTarget(typ.Elem()):
		r.mode = rowStructPointer
		r.elem = typ.Elem()
	}
	return r
}

func (r *rowReader[T]) scan(rows *Rows, dst *T) error {
	switch r.mode {
	case rowStruct:
		return rows.scanStruct(dst, r.op, false)
	case rowStructPointer:
		value := reflect.New(r.elem)
		if err := rows.scanStruct(value.Interface(), r.op, false); err != nil {
			return err
		}
		reflect.ValueOf(dst).Elem().Set(value)
		return nil
	}
	if !r.checked {
		if isRawBytesType(r.typ) {
			return fmt.Errorf("sql: RawBytes isn't allowed on %s", r.op)
		}
		if rows.numCols != 1 {
			return fmt.Errorf("sql: %s into %s: query returned %d columns, want exactly 1", r.op, r.typ, rows.numCols)
		}
		r.checked = true
	}
	return rows.Scan(dst)
}

var (
	scannerType  = reflect.TypeFor[Scanner]()
	timeType     = reflect.TypeFor[time.Time]()
	rawBytesType = reflect.TypeFor[RawBytes]()
)

func isRawBytesType(t reflect.Type) bool {
	return t == rawBytesType || (t.Kind() == reflect.Pointer && t.Elem() == rawBytesType)
}

// isStructTarget reports whether t is mapped column by column instead of
// receiving a single column. Types that already have a database/sql
// conversion keep it.
func isStructTarget(t reflect.Type) bool {
	if t.Kind() != reflect.Struct || t == timeType || reflect.IsEnum(t) || reflect.IsOptional(t) {
		return false
	}
	return !t.Implements(scannerType) && !reflect.PointerTo(t).Implements(scannerType)
}

// structField is one settable leaf field of a destination struct, reached
// through the embedded structs in index.
type structField struct {
	index []int
	path  string       // Go field path for messages, such as Base.ID
	typ   reflect.Type // field type
	depth int          // number of embedded structs crossed
	tag   string       // exact column name from `sql:"name"`, or empty
	norm  string       // normalized field name, empty when tagged
	ptrs  bool         // the path crosses an embedded pointer
}

// addr returns a pointer to the field, allocating embedded pointers.
// root is an addressable struct value.
func (f *structField) addr(root reflect.Value) any {
	if len(f.index) == 1 {
		return root.Field(f.index[0]).Addr().Interface()
	}
	v := root
	last := len(f.index) - 1
	for n, i := range f.index {
		v = v.Field(i)
		if n == last {
			break
		}
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
	}
	return v.Addr().Interface()
}

// structMatch is the field that wins for a name, or the fields that tie.
type structMatch struct {
	field *structField
	tied  []*structField // more than one candidate at the shallowest depth
}

// structType is the cached, immutable field index of a struct type.
type structType struct {
	typ    reflect.Type
	err    error // invalid tag found while indexing
	byTag  map[string]structMatch
	byName map[string]structMatch
}

var structTypes sync.Map // reflect.Type -> *structType

func structTypeOf(t reflect.Type) *structType {
	if st, ok := structTypes.Load(t); ok {
		return st.(*structType)
	}
	st, _ := structTypes.LoadOrStore(t, buildStructType(t))
	return st.(*structType)
}

func buildStructType(t reflect.Type) *structType {
	st := &structType{typ: t}
	var all []*structField
	var walk func(t reflect.Type, index []int, path string, stack []reflect.Type, ptrs bool)
	walk = func(t reflect.Type, index []int, path string, stack []reflect.Type, ptrs bool) {
		stack = append(stack, t)
		for i := range t.NumField() {
			sf := t.Field(i)
			tag := sf.Tag.Get("sql")
			if tag == "-" {
				continue
			}
			if strings.Contains(tag, ",") {
				if st.err == nil {
					st.err = fmt.Errorf("field %s%s has invalid tag sql:%q: options after a comma are not supported", path, sf.Name, tag)
				}
				continue
			}
			ft, embeddedPtr := sf.Type, false
			if sf.Anonymous && ft.Kind() == reflect.Pointer {
				ft, embeddedPtr = ft.Elem(), true
			}
			if sf.Anonymous && tag == "" && isStructTarget(ft) {
				// Flatten. Unexported embedded structs still promote their
				// exported fields, but an unexported pointer cannot be set.
				if (embeddedPtr && !sf.IsExported()) || slices.Contains(stack, ft) {
					continue
				}
				walk(ft, append(slices.Clone(index), i), path+sf.Name+".", stack, ptrs || embeddedPtr)
				continue
			}
			if !sf.IsExported() {
				continue
			}
			f := &structField{
				index: append(slices.Clone(index), i),
				path:  path + sf.Name,
				typ:   sf.Type,
				depth: len(index),
				tag:   tag,
				ptrs:  ptrs,
			}
			if tag == "" {
				f.norm = normalizeColumnName(sf.Name)
			}
			all = append(all, f)
		}
	}
	walk(t, nil, "", nil, false)

	tagged := map[string][]*structField{}
	named := map[string][]*structField{}
	for _, f := range all {
		if f.tag != "" {
			tagged[f.tag] = append(tagged[f.tag], f)
		} else {
			named[f.norm] = append(named[f.norm], f)
		}
	}
	st.byTag = resolveStructFields(tagged)
	st.byName = resolveStructFields(named)
	return st
}

// resolveStructFields applies the shallowest-depth rule to each name.
func resolveStructFields(groups map[string][]*structField) map[string]structMatch {
	out := make(map[string]structMatch, len(groups))
	for name, group := range groups {
		shallow := group[0].depth
		for _, f := range group {
			shallow = min(shallow, f.depth)
		}
		var winners []*structField
		for _, f := range group {
			if f.depth == shallow {
				winners = append(winners, f)
			}
		}
		if len(winners) == 1 {
			out[name] = structMatch{field: winners[0]}
		} else {
			out[name] = structMatch{tied: winners}
		}
	}
	return out
}

// normalizeColumnName lower-cases s and removes underscores.
func normalizeColumnName(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '_' {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}

// structPlan is the column-to-field mapping of one result set. It is cached
// in the Rows, which already require that Scan not be called concurrently.
type structPlan struct {
	typ      reflect.Type
	fields   []*structField // by column
	dests    []any          // reused by every row
	rawBytes bool           // some field is RawBytes or *RawBytes
}

func (p *structPlan) fill(root reflect.Value) []any {
	for i, f := range p.fields {
		dest := f.addr(root)
		if jsonColumnType(f.typ) {
			dest = jsonColumnScanner{dest}
		}
		p.dests[i] = dest
	}
	return p.dests
}

func (rs *Rows) scanStruct(dest any, op string, streaming bool) error {
	root, err := structDestination(dest, op)
	if err != nil {
		return err
	}
	plan, err := rs.structPlanFor(root.Type(), op)
	if err != nil {
		return err
	}
	if plan.rawBytes && !streaming {
		return fmt.Errorf("sql: RawBytes isn't allowed on %s", op)
	}
	dests := plan.fill(root)
	err = rs.Scan(dests...)
	clear(dests) // do not retain the row after returning
	return err
}

func structDestination(dest any, op string) (reflect.Value, error) {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return reflect.Value{}, fmt.Errorf("sql: %s destination must be a non-nil pointer to a struct, not %T", op, dest)
	}
	if t := v.Type().Elem(); t.Kind() != reflect.Struct || reflect.IsEnum(t) || reflect.IsOptional(t) {
		return reflect.Value{}, fmt.Errorf("sql: %s destination must be a non-nil pointer to a struct, not %T", op, dest)
	}
	return v.Elem(), nil
}

// structPlanFor returns the cached plan for typ or builds one with the
// preconditions of Scan, so closed rows and a missing Next report the same
// way.
func (rs *Rows) structPlanFor(typ reflect.Type, op string) (*structPlan, error) {
	if p := rs.scanPlan; p != nil && p.typ == typ {
		return p, nil
	}
	if rs.closemuScanHold {
		return nil, fmt.Errorf("sql: %s called without calling Next (closemuScanHold)", op)
	}
	var cols []string
	err := func() error {
		rs.closemu.RLock()
		defer rs.closemu.RUnlock()
		if rs.lasterr != nil && rs.lasterr != io.EOF {
			return rs.lasterr
		}
		if rs.closed {
			return rs.lasterrOrErrLocked(errRowsClosed)
		}
		if !rs.nextCalled {
			return fmt.Errorf("sql: %s called without calling Next", op)
		}
		withLock(rs.dc, func() { cols = rs.rowsi.Columns() })
		return nil
	}()
	if err != nil {
		return nil, err
	}
	plan, err := newStructPlan(typ, cols, op)
	if err != nil {
		return nil, err
	}
	rs.scanPlan = plan
	return plan, nil
}

// manyColumns is the result width above which duplicate detection uses maps
// instead of comparing against the earlier columns.
const manyColumns = 32

func newStructPlan(typ reflect.Type, cols []string, op string) (*structPlan, error) {
	st := structTypeOf(typ)
	if st.err != nil {
		return nil, fmt.Errorf("sql: %s: %s: %w", op, typ, st.err)
	}
	plan := &structPlan{
		typ:    typ,
		fields: make([]*structField, len(cols)),
		dests:  make([]any, len(cols)),
	}
	var seenCols map[string]int
	var seenFields map[*structField]int
	if len(cols) > manyColumns {
		seenCols = make(map[string]int, len(cols))
		seenFields = make(map[*structField]int, len(cols))
	}
	for i, col := range cols {
		if j := earlierIndex(cols[:i], seenCols, col); j >= 0 {
			return nil, fmt.Errorf("sql: %s: duplicate column name %q at indexes %d and %d", op, col, j, i)
		}
		m, ok := st.byTag[col]
		if !ok {
			m, ok = st.byName[normalizeColumnName(col)]
		}
		switch {
		case !ok:
			return nil, fmt.Errorf("sql: %s: column %q has no matching field in %s", op, col, typ)
		case m.field == nil:
			paths := make([]string, len(m.tied))
			for k, f := range m.tied {
				paths[k] = f.path
			}
			return nil, fmt.Errorf("sql: %s: column %q is ambiguous in %s: it matches fields %s", op, col, typ, strings.Join(paths, ", "))
		}
		if j := earlierIndex(plan.fields[:i], seenFields, m.field); j >= 0 {
			return nil, fmt.Errorf("sql: %s: columns %q and %q both map to field %s of %s", op, cols[j], col, m.field.path, typ)
		}
		plan.fields[i] = m.field
		plan.rawBytes = plan.rawBytes || isRawBytesType(m.field.typ)
		if seenCols != nil {
			seenCols[col] = i
			seenFields[m.field] = i
		}
	}
	return plan, nil
}

// earlierIndex returns the index of v among the earlier values, or -1. For
// narrow results it compares directly, otherwise it consults seen.
func earlierIndex[K comparable](earlier []K, seen map[K]int, v K) int {
	if seen == nil {
		return slices.Index(earlier, v)
	}
	if j, ok := seen[v]; ok {
		return j
	}
	return -1
}
