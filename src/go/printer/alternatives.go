// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package printer

import (
	"go/ast"
	"go/token"
)

func (p *printer) enumType(x *ast.EnumType) {
	p.setPos(x.Enum)
	p.print("enum", blank)
	if x.String.IsValid() {
		p.setPos(x.String)
		p.print("string", blank)
	}
	p.setPos(x.Lbrace)
	p.print(token.LBRACE, indent)
	for _, v := range x.Variants {
		p.linebreak(p.lineFor(v.Pos()), 1, ignore, true)
		p.setComment(v.Doc)
		p.setPos(v.Pos())
		if v.Default.IsValid() {
			p.print(token.DEFAULT, blank)
		}
		p.expr(v.Name)
		if v.Payload != nil {
			if v.Record {
				p.print(blank)
				p.fieldList(v.Payload, true, false)
			} else {
				p.parameters(v.Payload, funcParam)
			}
		}
		if v.Value != nil {
			p.print(blank, token.ASSIGN, blank)
			p.expr(v.Value)
		}
		p.setComment(v.Comment)
	}
	p.print(unindent)
	p.linebreak(p.lineFor(x.Rbrace), 1, ignore, true)
	p.setPos(x.Rbrace)
	p.print(token.RBRACE)
}

func (p *printer) matchExpr(x *ast.MatchExpr) {
	p.setPos(x.Switch)
	p.print(token.SWITCH, blank)
	if x.Tag != nil {
		p.expr(x.Tag)
	}
	p.print(blank)
	p.setPos(x.Lbrace)
	p.print(token.LBRACE)
	for _, a := range x.Arms {
		p.linebreak(p.lineFor(a.Case), 1, ignore, true)
		p.setPos(a.Case)
		if a.Pattern == nil {
			p.print(token.DEFAULT)
		} else {
			p.print(token.CASE, blank)
			p.matchPattern(a.Pattern)
		}
		if a.Guard != nil {
			p.print(blank, token.IF, blank)
			p.expr(a.Guard)
		}
		p.setPos(a.Arrow)
		p.print(blank, token.FATARROW)
		if a.Body != nil {
			p.print(blank)
			p.block(a.Body, 1)
		} else if a.Value != nil {
			if p.lineFor(a.Value.Pos()) > p.lineFor(a.Arrow) {
				p.print(indent, newline)
				p.expr(a.Value)
				p.print(unindent)
			} else {
				p.print(blank)
				p.expr(a.Value)
			}
		}
	}
	p.linebreak(p.lineFor(x.Rbrace), 1, ignore, true)
	p.setPos(x.Rbrace)
	p.print(token.RBRACE)
}

func (p *printer) matchPattern(x *ast.MatchPattern) {
	if x.Inner != nil {
		if x.Lparen.IsValid() {
			p.print(token.LPAREN)
		}
		p.matchPattern(x.Inner)
		if x.Rparen.IsValid() {
			p.print(token.RPAREN)
		}
		if x.Question.IsValid() {
			p.setPos(x.Question)
			p.print(token.QUESTION)
		}
		return
	}
	p.expr(x.Value)
	if x.Lparen.IsValid() {
		p.setPos(x.Lparen)
		p.print(token.LPAREN)
		for i, a := range x.Args {
			if i > 0 {
				p.print(token.COMMA, blank)
			}
			p.matchPattern(a)
		}
		p.setPos(x.Rparen)
		p.print(token.RPAREN)
	}
	if x.Lbrace.IsValid() {
		p.setPos(x.Lbrace)
		p.print(token.LBRACE)
		multiline := p.lineFor(x.Lbrace) < p.lineFor(x.Rbrace)
		if multiline {
			p.print(indent)
		}
		for i, f := range x.Fields {
			if multiline {
				p.linebreak(p.lineFor(f.Pos()), 1, ignore, i == 0)
			} else if i > 0 {
				p.print(blank)
			}
			p.expr(f.Name)
			p.setPos(f.Colon)
			p.print(token.COLON, blank)
			p.matchPattern(f.Pattern)
			if multiline || i+1 < len(x.Fields) || x.Rest.IsValid() {
				p.print(token.COMMA)
			}
		}
		if x.Rest.IsValid() {
			if multiline {
				p.linebreak(p.lineFor(x.Rest), 1, ignore, len(x.Fields) == 0)
			} else if len(x.Fields) > 0 {
				p.print(blank)
			}
			p.setPos(x.Rest)
			p.print(token.ELLIPSIS)
			if multiline {
				p.print(token.COMMA)
			}
		}
		if multiline {
			p.print(unindent)
			p.linebreak(p.lineFor(x.Rbrace), 1, ignore, true)
		}
		p.setPos(x.Rbrace)
		p.print(token.RBRACE)
	}
}
