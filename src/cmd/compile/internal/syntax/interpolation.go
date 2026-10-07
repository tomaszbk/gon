package syntax

// InterpolatedStringExpr retains source text and parsed expressions. Lowered is
// prepared by the checker and deliberately excluded from structural traversal.
type InterpolatedStringExpr struct {
	Quote   byte
	Rquote  Pos
	Parts   []*InterpolationPart
	Lowered *CallExpr
	expr
}

type InterpolationPart struct {
	Text   string
	Expr   Expr
	Format string
	End    Pos
	node
}

type interpolationFrame struct {
	quote     rune
	text      bool
	depth     int
	pending   token
	line, col uint
}

func (s *scanner) interpolationToken() bool {
	if len(s.interpolations) == 0 {
		return false
	}
	f := s.interpolations[len(s.interpolations)-1]
	if !f.text {
		return false
	}
	if f.pending != 0 {
		s.line, s.col, s.tok = f.line, f.col, f.pending
		s.lit = ""
		f.pending = 0
		if s.tok == _InterpOpen {
			f.text = false
		} else {
			s.interpolations = s.interpolations[:len(s.interpolations)-1]
			s.nlsemi = true
		}
		return true
	}
	s.stop()
	s.line, s.col = s.pos()
	s.start()
	for {
		switch s.ch {
		case -1:
			s.errorf("interpolated string not terminated")
			f.pending = _InterpEnd
			f.line, f.col = s.pos()
			s.lit = string(s.segment())
			s.tok = _InterpText
			return true
		case '\n':
			if f.quote == '"' {
				s.errorf("newline in interpolated string")
			}
		case '\\':
			if f.quote == '"' {
				s.nextch()
				if s.ch == '\n' {
					s.errorf("newline in interpolated string")
				}
				if s.ch >= 0 {
					s.nextch()
				}
				continue
			}
		case '$':
			line, col := s.pos()
			s.nextch()
			if s.ch == '{' {
				s.nextch()
				f.pending = _InterpOpen
				f.line, f.col = line, col
				segment := s.segment()
				s.lit = string(segment[:len(segment)-2])
				s.tok = _InterpText
				return true
			}
			continue
		}
		if s.ch == f.quote {
			f.line, f.col = s.pos()
			s.nextch()
			f.pending = _InterpEnd
			segment := s.segment()
			s.lit = string(segment[:len(segment)-1])
			s.tok = _InterpText
			return true
		}
		s.nextch()
	}
}

func (s *scanner) interpolationExprToken() bool {
	if len(s.interpolations) == 0 {
		return false
	}
	f := s.interpolations[len(s.interpolations)-1]
	if f.text {
		return false
	}
	switch s.ch {
	case '(':
		f.depth++
	case '[':
		f.depth++
	case '{':
		f.depth++
	case ')', ']':
		f.depth--
	case '}':
		if f.depth > 0 {
			f.depth--
			return false
		}
		s.nextch()
		s.tok = _InterpClose
		f.text = true
		return true
	case ':':
		if f.depth != 0 {
			return false
		}
		s.nextch()
		for s.ch >= 0 && s.ch != '}' {
			if s.ch == '\n' && f.quote == '"' {
				s.errorf("newline in interpolated string")
			}
			s.nextch()
		}
		s.tok = _InterpFormat
		s.lit = string(s.segment())[1:]
		return true
	}
	return false
}

func (p *parser) interpolatedString() Expr {
	x := &InterpolatedStringExpr{Quote: p.lit[0]}
	x.pos = p.pos()
	p.next()
	for p.tok != _InterpEnd && p.tok != _EOF {
		part := &InterpolationPart{}
		part.pos = p.pos()
		switch p.tok {
		case _InterpText:
			part.Text = p.lit
			part.End = p.pos()
			p.next()
		case _InterpOpen:
			p.next()
			outer := p.xnest
			p.xnest = 0
			part.Expr = p.expr()
			p.xnest = outer
			if p.tok == _InterpFormat {
				if p.lit == "" {
					p.syntaxError("expecting non-empty interpolation format")
				}
				part.Format = p.lit
				p.next()
			}
			part.End = p.pos()
			p.want(_InterpClose)
		default:
			p.syntaxError("expecting interpolated string part")
			p.next()
			continue
		}
		x.Parts = append(x.Parts, part)
	}
	x.Rquote = p.pos()
	p.want(_InterpEnd)
	return x
}

func (s *scanner) nextch() {
	if s.ch == '\n' {
		for _, f := range s.interpolations {
			if f.quote == '"' {
				s.errorf("newline in interpolated string")
				break
			}
		}
	}
	s.source.nextch()
}
