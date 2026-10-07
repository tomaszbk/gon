// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import (
	"go/ast"
	"go/scanner"
	"go/token"
)

// parseDeclaredType keeps enum contextual and preserves the ordinary inserted
// semicolon after an identifier at a newline. It is used only for the type
// immediately following a declaration name and its optional type parameters.
func (p *parser) parseDeclaredType(allowEnum bool) ast.Expr {
	if !allowEnum || p.tok != token.IDENT || p.lit != "enum" {
		return p.parseType()
	}
	name := p.parseIdent()
	var stringPos token.Pos
	if p.tok == token.IDENT && p.lit == "string" {
		stringPos = p.pos
		p.next()
	}
	if p.tok == token.LBRACE {
		x := p.parseEnumType(name.Pos())
		x.String = stringPos
		return x
	}
	if stringPos.IsValid() {
		p.errorExpected(p.pos, "'{'")
	}
	typ := p.parseTypeName(name)
	if p.tok == token.LBRACK {
		typ = p.parseTypeInstance(typ)
	}
	return typ
}

func (p *parser) parseEnumType(pos token.Pos) *ast.EnumType {
	x := &ast.EnumType{Enum: pos, Lbrace: p.expect(token.LBRACE)}
	for p.tok != token.RBRACE && p.tok != token.EOF {
		if p.tok == token.SEMICOLON {
			p.next()
			continue
		}
		v := &ast.EnumVariant{Doc: p.leadComment}
		if p.tok == token.DEFAULT {
			v.Default = p.pos
			p.next()
		}
		v.Name = p.parseIdent()
		switch p.tok {
		case token.LPAREN:
			v.Payload = &ast.FieldList{Opening: p.pos}
			p.next()
			for p.tok != token.RPAREN && p.tok != token.EOF {
				v.Payload.List = append(v.Payload.List, &ast.Field{Type: p.parseType()})
				if !p.atComma("enum payload", token.RPAREN) {
					break
				}
				p.next()
			}
			v.Payload.Closing = p.expectClosing(token.RPAREN, "enum payload")
		case token.LBRACE:
			v.Record = true
			v.Payload = &ast.FieldList{Opening: p.pos}
			p.next()
			for p.tok != token.RBRACE && p.tok != token.EOF {
				if p.tok == token.SEMICOLON {
					p.next()
					continue
				}
				field := &ast.Field{Doc: p.leadComment, Names: []*ast.Ident{p.parseIdent()}}
				for p.tok == token.COMMA {
					p.next()
					field.Names = append(field.Names, p.parseIdent())
				}
				field.Type = p.parseType()
				field.Comment = p.expectSemi()
				v.Payload.List = append(v.Payload.List, field)
			}
			v.Payload.Closing = p.expect(token.RBRACE)
		}
		if p.tok == token.ASSIGN {
			p.next()
			v.Value = p.parseRhs()
		}
		v.Comment = p.expectSemi()
		x.Variants = append(x.Variants, v)
	}
	x.Rbrace = p.expect(token.RBRACE)
	return x
}

// matchClauseAhead distinguishes the first arrow arm from an ordinary Go
// case before parsing it. Only delimiter-balanced tokens are examined, so
// calls, function bodies, literals, and selectors in existing cases retain
// their ordinary grammar. Rescanning a scanner's file is supported: repeated
// line positions are ignored by token.File, and ParseFile deduplicates errors.
func (p *parser) matchClauseAhead() bool {
	if p.tok != token.CASE && p.tok != token.DEFAULT {
		return false
	}
	s := p.scanner
	depth := 0
	for {
		_, tok, _ := s.Scan()
		switch tok {
		case token.LPAREN, token.LBRACK, token.LBRACE:
			depth++
		case token.RPAREN, token.RBRACK, token.RBRACE:
			if depth == 0 {
				return false
			}
			depth--
		case token.FATARROW:
			if depth == 0 {
				return true
			}
		case token.COLON, token.CASE, token.DEFAULT, token.EOF:
			if depth == 0 {
				return false
			}
		}
	}
}

func (p *parser) parseMatchExpr() *ast.MatchExpr {
	defer decNestLev(incNestLev(p))
	x := &ast.MatchExpr{Switch: p.expect(token.SWITCH)}
	level := p.exprLev
	p.exprLev = -1
	x.Tag = p.parseRhs()
	p.exprLev = level
	x.Lbrace = p.expect(token.LBRACE)
	p.parseMatchArms(x, false)
	return x
}

func (p *parser) parseMatchArms(x *ast.MatchExpr, statement bool) {
	for p.tok == token.CASE || p.tok == token.DEFAULT {
		a := &ast.MatchArm{Case: p.pos}
		isCase := p.tok == token.CASE
		p.next()
		if isCase {
			a.Patterns = append(a.Patterns, p.parseMatchPattern())
			for p.tok == token.COMMA {
				p.next()
				a.Patterns = append(a.Patterns, p.parseMatchPattern())
			}
		}
		if p.tok == token.IF {
			p.next()
			level := p.exprLev
			p.exprLev = -1
			guard := p.matchGuard
			p.matchGuard = true
			a.Guard = p.parseRhs()
			p.matchGuard = guard
			p.exprLev = level
		}
		a.Arrow = p.expect(token.FATARROW)
		if statement {
			level := p.exprLev
			p.exprLev = 0
			a.Body = p.parseBlockStmt()
			p.exprLev = level
		} else {
			a.Value = p.parseRhs()
		}
		p.expectSemi()
		x.Arms = append(x.Arms, a)
	}
	x.Rbrace = p.expect(token.RBRACE)
}

func (p *parser) parseMatchPattern() *ast.MatchPattern {
	defer decNestLev(incNestLev(p))
	x := &ast.MatchPattern{}
	switch p.tok {
	case token.LPAREN:
		x.Lparen = p.pos
		p.next()
		x.Inner = p.parseMatchPattern()
		x.Rparen = p.expectClosing(token.RPAREN, "grouped pattern")
	case token.IDENT:
		x.Value = p.parseIdent()
		for p.tok == token.PERIOD || p.tok == token.LBRACK {
			if p.tok == token.PERIOD {
				p.next()
				x.Value = p.parseSelector(x.Value)
			} else {
				x.Value = p.parseTypeInstance(x.Value)
			}
		}
	case token.INT, token.FLOAT, token.IMAG, token.CHAR, token.STRING:
		x.Value = &ast.BasicLit{ValuePos: p.pos, ValueEnd: p.end(), Kind: p.tok, Value: p.lit}
		p.next()
	case token.ADD, token.SUB:
		op, pos := p.tok, p.pos
		p.next()
		if p.tok != token.INT && p.tok != token.FLOAT && p.tok != token.IMAG {
			p.errorExpected(p.pos, "numeric literal")
		}
		lit := &ast.BasicLit{ValuePos: p.pos, ValueEnd: p.end(), Kind: p.tok, Value: p.lit}
		p.next()
		x.Value = &ast.UnaryExpr{OpPos: pos, Op: op, X: lit}
	default:
		pos := p.pos
		p.errorExpected(pos, "match pattern")
		if p.tok != token.EOF {
			p.next()
		}
		x.Value = &ast.BadExpr{From: pos, To: p.pos}
		return x
	}
	switch p.tok {
	case token.LPAREN:
		x.Lparen = p.pos
		p.next()
		for p.tok != token.RPAREN && p.tok != token.EOF {
			x.Args = append(x.Args, p.parseMatchPattern())
			if !p.atComma("positional pattern", token.RPAREN) {
				break
			}
			p.next()
		}
		x.Rparen = p.expectClosing(token.RPAREN, "positional pattern")
	case token.LBRACE:
		if p.exprLev < 0 && !p.patternTestRecordAhead() {
			break
		}
		x.Lbrace = p.pos
		p.next()
		for p.tok != token.RBRACE && p.tok != token.EOF {
			if p.tok == token.ELLIPSIS {
				x.Rest = p.pos
				p.next()
				if p.tok == token.COMMA {
					p.next()
				}
				break
			}
			f := &ast.MatchField{Name: p.parseIdent(), Colon: p.expect(token.COLON)}
			f.Pattern = p.parseMatchPattern()
			x.Fields = append(x.Fields, f)
			if !p.atComma("record pattern", token.RBRACE) {
				break
			}
			p.next()
		}
		x.Rbrace = p.expectClosing(token.RBRACE, "record pattern")
	}
	if p.tok == token.QUESTION {
		x = &ast.MatchPattern{Inner: x, Question: p.pos}
		p.next()
	}
	return x
}

func (p *parser) patternTestRecordAhead() bool {
	source := p.src[p.file.Offset(p.pos):]
	file := token.NewFileSet().AddFile("pattern-lookahead", -1, len(source))
	var scan scanner.Scanner
	scan.Init(file, source, nil, 0)
	_, first, _ := scan.Scan()
	if first != token.LBRACE {
		return false
	}
	depth := 1
	for depth > 0 {
		_, tok, _ := scan.Scan()
		switch tok {
		case token.LBRACE:
			depth++
		case token.RBRACE:
			depth--
		case token.EOF:
			return false
		}
	}
	_, following, literal := scan.Scan()
	return following == token.LBRACE || following.Precedence() > 0 || following == token.FATARROW || following == token.COMMA || following == token.RPAREN || following == token.QUESTION || following == token.IDENT && literal == "is"
}
