package types2

import (
	"cmd/compile/internal/syntax"
	"go/constant"
	. "internal/types/errors"
	"strconv"
	"strings"
)

// interpolationText interprets Go escapes while preserving the additional \$.
func interpolationText(text string, quote byte) (string, error) {
	if quote == '`' {
		return strings.ReplaceAll(text, "\r", ""), nil
	}
	var escaped strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' && i+1 < len(text) {
			i++
			if text[i] != '$' {
				escaped.WriteByte('\\')
			}
			escaped.WriteByte(text[i])
		} else {
			escaped.WriteByte(text[i])
		}
	}
	return strconv.Unquote("\"" + escaped.String() + "\"")
}

func validInterpolationFormat(format string) bool {
	if len(format) < 2 || format[0] != '%' || !strings.ContainsRune("vTtbcdoOxXUeEfFgGspq", rune(format[len(format)-1])) {
		return false
	}
	i := 1
	for i < len(format) && strings.ContainsRune("#+- 0", rune(format[i])) {
		i++
	}
	for i < len(format) && format[i] >= '0' && format[i] <= '9' {
		i++
	}
	if i < len(format) && format[i] == '.' {
		i++
		for i < len(format) && format[i] >= '0' && format[i] <= '9' {
			i++
		}
	}
	// A single explicit first-operand index is valid; every other index would
	// reference an operand outside this interpolation.
	if strings.HasPrefix(format[i:], "[1]") {
		i += 3
	}
	return i == len(format)-1 && strings.ContainsRune("vTtbcdoOxXUeEfFgGspq", rune(format[i]))
}

func (check *Checker) interpolatedString(x *operand, e *syntax.InterpolatedStringExpr) {
	var sprintf *Func
	for _, imported := range check.imports {
		if imported.imported.Path() == "fmt" && imported.Pos().FileBase() == e.Pos().FileBase() {
			sprintf, _ = imported.imported.Scope().Lookup("Sprintf").(*Func)
			check.usedPkgNames[imported] = true
			break
		}
	}
	valid := true
	if sprintf == nil {
		check.error(e, InvalidInterpolation, "string interpolation requires an explicit import of \"fmt\" in this file")
		valid = false
	}
	var format strings.Builder
	args := []syntax.Expr{}
	for _, part := range e.Parts {
		if part.Expr == nil {
			text, err := interpolationText(part.Text, e.Quote)
			if err != nil {
				check.errorf(part, InvalidInterpolation, "invalid interpolated string escape: %s", err)
				valid = false
			}
			format.WriteString(strings.ReplaceAll(text, "%", "%%"))
			continue
		}
		verb := part.Format
		if verb == "" {
			verb = "%v"
		}
		if !validInterpolationFormat(verb) {
			check.error(part, InvalidInterpolation, "interpolation format must be one fmt verb consuming one operand (without * or %%)")
			valid = false
		}
		format.WriteString(strings.ReplaceAll(verb, "[1]", "["+strconv.Itoa(len(args)+1)+"]"))
		var arg operand
		check.expr(nil, &arg, part.Expr)
		check.assignment(&arg, &emptyInterface, "string interpolation")
		if !arg.isValid() {
			valid = false
		}
		args = append(args, part.Expr)
	}
	x.expr = e
	x.mode_, x.typ_ = value, Typ[String]
	if !valid {
		x.invalidate()
		return
	}
	fun := syntax.NewName(e.Pos(), "Sprintf")
	check.recordUse(fun, sprintf)
	check.recordTypeAndValue(fun, value, sprintf.Type(), nil)
	literal := &syntax.BasicLit{Kind: syntax.StringLit, Value: strconv.Quote(format.String())}
	literal.SetPos(e.Pos())
	check.recordTypeAndValue(literal, constant_, Typ[UntypedString], constant.MakeString(format.String()))
	call := &syntax.CallExpr{Fun: fun, ArgList: append([]syntax.Expr{literal}, args...)}
	call.SetPos(e.Pos())
	check.recordTypeAndValue(call, value, Typ[String], nil)
	check.hasCallOrRecv = true
	e.Lowered = call
}
