package scanner

import "go/token"

type interpolationFrame struct {
	quote      rune
	text       bool
	depth      int
	pending    token.Token
	start, end int
}

func (s *Scanner) interpolationToken() (token.Pos, token.Token, string, bool) {
	if len(s.interpolations) == 0 {
		return 0, 0, "", false
	}
	f := s.interpolations[len(s.interpolations)-1]
	if !f.text {
		return 0, 0, "", false
	}
	if f.pending != 0 {
		tok := f.pending
		f.pending = 0
		s.endPosValid = true
		s.endPos = s.file.Pos(f.end)
		if tok == token.INTERPOLATION_OPEN {
			f.text = false
		} else {
			s.interpolations = s.interpolations[:len(s.interpolations)-1]
			s.insertSemi = true
		}
		return s.file.Pos(f.start), tok, "", true
	}
	start := s.offset
	for {
		switch s.ch {
		case eof:
			s.error(start, "interpolated string not terminated")
			f.pending = token.INTERPOLATION_END
			f.start, f.end = s.offset, s.offset
			return s.file.Pos(start), token.INTERPOLATION_TEXT, string(s.src[start:s.offset]), true
		case '\n':
			if f.quote == '"' {
				s.error(s.offset, "newline in interpolated string")
			}
		case '\\':
			if f.quote == '"' {
				s.next()
				if s.ch == '\n' {
					s.error(s.offset, "newline in interpolated string")
				}
				if s.ch >= 0 {
					s.next()
				}
				continue
			}
		case '$':
			if s.peek() == '{' {
				f.start = s.offset
				s.next()
				s.next()
				f.end = s.offset
				f.pending = token.INTERPOLATION_OPEN
				s.endPosValid = true
				s.endPos = s.file.Pos(f.start)
				return s.file.Pos(start), token.INTERPOLATION_TEXT, string(s.src[start:f.start]), true
			}
		}
		if s.ch == f.quote {
			f.start = s.offset
			s.next()
			f.end = s.offset
			f.pending = token.INTERPOLATION_END
			s.endPosValid = true
			s.endPos = s.file.Pos(f.start)
			return s.file.Pos(start), token.INTERPOLATION_TEXT, string(s.src[start:f.start]), true
		}
		s.next()
	}
}

func (s *Scanner) interpolationExprToken() (token.Token, string, bool) {
	if len(s.interpolations) == 0 {
		return 0, "", false
	}
	f := s.interpolations[len(s.interpolations)-1]
	if f.text {
		return 0, "", false
	}
	switch s.ch {
	case '(', '[', '{':
		f.depth++
	case ')', ']':
		f.depth--
	case '}':
		if f.depth > 0 {
			f.depth--
			return 0, "", false
		}
		s.next()
		f.text = true
		return token.INTERPOLATION_CLOSE, "", true
	case ':':
		if f.depth != 0 {
			return 0, "", false
		}
		s.next()
		start := s.offset
		for s.ch >= 0 && s.ch != '}' {
			if s.ch == '\n' && f.quote == '"' {
				s.error(s.offset, "newline in interpolated string")
			}
			s.next()
		}
		return token.INTERPOLATION_FORMAT, string(s.src[start:s.offset]), true
	}
	return 0, "", false
}
