package syntax

// EnumType describes a closed set of variants. enum remains an identifier
// outside the right-hand side of a defined type declaration.
type EnumType struct {
	Lbrace, Rbrace Pos
	String         Pos // optional contextual "string" representation marker
	Variants       []*EnumVariant
	expr
}

type EnumVariant struct {
	Default        Pos
	Name           *Name
	Payload        []*Field // nil for a unit variant
	Record         bool
	Value          Expr // optional constant string spelling
	Ldelim, Rdelim Pos
	node
}

// EnumConstructExpr is compiler-generated after type checking for checked
// propagation boundaries; it is never parsed from source.
type EnumConstructExpr struct {
	Variant int
	ArgList []Expr
	expr
}

// ContextualVariantExpr constructs a canonical Option or Result using a target.
// Its position is the leading dot.
type ContextualVariantExpr struct {
	Name           *Name
	Lparen, Rparen Pos
	ArgList        []Expr
	expr
}

type OptionalExpr struct {
	X        Expr
	Question Pos
	Body     *BlockStmt // compiler-prepared None return for range-function rewriting
	expr
}

type MatchExpr struct {
	Tag            Expr
	Lbrace, Rbrace Pos
	Arms           []*MatchArm
	expr
}

type MatchStmt struct {
	Match *MatchExpr
	stmt
}

type MatchArm struct {
	Arrow   Pos
	Pattern *MatchPattern // nil for default
	Guard   Expr
	Value   Expr
	Body    *BlockStmt
	node
}

// MatchPattern is intentionally not an expression: its arguments bind or test
// data and must never be evaluated as function calls.
type MatchPattern struct {
	Inner                          *MatchPattern
	Question                       Pos
	Value                          Expr
	Lparen, Rparen, Lbrace, Rbrace Pos
	Args                           []*MatchPattern
	Fields                         []*MatchField
	Rest                           Pos
	node
}

type MatchField struct {
	Name    *Name
	Colon   Pos
	Pattern *MatchPattern
	node
}

func (p *parser) enumDeclType(allow bool) Expr {
	typ := p.typeOrNil()
	if name, ok := typ.(*Name); allow && ok && name.Value == "enum" {
		var stringPos Pos
		if p.tok == _Name && p.lit == "string" {
			stringPos = p.pos()
			p.next()
		}
		if p.tok == _Lbrace {
			x := p.enumType(name.Pos())
			x.String = stringPos
			return x
		}
		if stringPos.IsKnown() {
			p.syntaxError("expecting {")
		}
	}
	return typ
}

func (p *parser) enumType(pos Pos) *EnumType {
	t := &EnumType{}
	t.pos, t.Lbrace = pos, p.pos()
	p.want(_Lbrace)
	for p.tok != _Rbrace && p.tok != _EOF {
		v := &EnumVariant{}
		v.pos = p.pos()
		if p.tok == _Default {
			v.Default = p.pos()
			p.next()
		}
		v.Name = p.name()
		switch p.tok {
		case _Lparen:
			v.Ldelim = p.pos()
			p.next()
			for p.tok != _Rparen && p.tok != _EOF {
				f := &Field{}
				f.pos, f.Type = p.pos(), p.type_()
				v.Payload = append(v.Payload, f)
				if !p.got(_Comma) {
					break
				}
			}
			v.Rdelim = p.pos()
			p.want(_Rparen)
		case _Lbrace:
			v.Record, v.Ldelim = true, p.pos()
			p.next()
			st := &StructType{}
			for p.tok != _Rbrace && p.tok != _EOF {
				p.fieldDecl(st)
				if p.tok != _Rbrace {
					p.want(_Semi)
				}
			}
			v.Payload = st.FieldList
			for i, f := range st.FieldList {
				if f.Name == nil {
					p.errorAt(f.Pos(), "enum payload fields require names")
				}
				if i < len(st.TagList) && st.TagList[i] != nil {
					p.errorAt(st.TagList[i].Pos(), "enum payload fields cannot have tags")
				}
			}
			v.Rdelim = p.pos()
			p.want(_Rbrace)
		}
		if p.got(_Assign) {
			v.Value = p.expr()
		}
		t.Variants = append(t.Variants, v)
		if p.tok != _Rbrace {
			p.want(_Semi)
		}
	}
	t.Rbrace = p.pos()
	p.want(_Rbrace)
	return t
}

func (p *parser) matchExpr() *MatchExpr {
	m := &MatchExpr{}
	m.pos = p.pos()
	p.want(_Switch)
	xnest := p.xnest
	p.xnest = -1
	m.Tag = p.expr()
	p.xnest = xnest
	m.Lbrace = p.pos()
	p.want(_Lbrace)
	for p.tok != _Rbrace && p.tok != _EOF {
		a := &MatchArm{}
		a.pos = p.pos()
		switch p.tok {
		case _Case:
			p.next()
			a.Pattern = p.matchPattern()
		case _Default:
			p.next()
		default:
			p.syntaxError("expected case or default in match expression")
			p.advance(_Case, _Default, _Rbrace)
			continue
		}
		if p.got(_If) {
			a.Guard = p.expr()
		}
		a.Arrow = p.pos()
		p.want(_FatArrow)
		if p.tok == _Lbrace {
			p.syntaxError("match expression arm requires an expression")
			a.Body = p.blockStmt("match arm")
			a.Value = p.badExpr()
		} else {
			a.Value = p.expr()
		}
		m.Arms = append(m.Arms, a)
		if p.tok != _Rbrace {
			p.want(_Semi)
		}
	}
	m.Rbrace = p.pos()
	p.want(_Rbrace)
	return m
}

func (p *parser) matchPattern() *MatchPattern {
	n := &MatchPattern{}
	n.pos = p.pos()
	switch p.tok {
	case _Lparen:
		n.Lparen = p.pos()
		p.next()
		n.Inner = p.matchPattern()
		n.Rparen = p.pos()
		p.want(_Rparen)
	case _Name:
		n.Value = p.name()
		for {
			if p.got(_Dot) {
				sel := &SelectorExpr{X: n.Value, Sel: p.name()}
				sel.pos = n.pos
				n.Value = sel
			} else if p.tok == _Lbrack {
				n.Value = p.typeInstance(n.Value)
			} else {
				break
			}
		}
	case _Literal:
		n.Value = p.oliteral()
	case _Operator:
		// Signed numeric literal patterns keep their ordinary expression shape.
		if p.op == Add || p.op == Sub {
			x := &Operation{Op: p.op}
			x.pos = p.pos()
			p.next()
			x.X = p.oliteral()
			if x.X == nil {
				x.X = p.badExpr()
				p.syntaxError("expected literal in pattern")
			}
			n.Value = x
			break
		}
		fallthrough
	default:
		n.Value = p.badExpr()
		p.syntaxError("expected pattern")
		p.advance(_FatArrow, _Rparen, _Rbrace)
	}
	switch p.tok {
	case _Lparen:
		n.Lparen = p.pos()
		p.next()
		for p.tok != _Rparen && p.tok != _EOF {
			n.Args = append(n.Args, p.matchPattern())
			if !p.got(_Comma) {
				break
			}
		}
		n.Rparen = p.pos()
		p.want(_Rparen)
	case _Lbrace:
		n.Lbrace = p.pos()
		p.next()
		for p.tok != _Rbrace && p.tok != _EOF {
			if p.tok == _DotDotDot {
				n.Rest = p.pos()
				p.next()
				p.got(_Comma)
				break
			}
			f := &MatchField{}
			f.pos = p.pos()
			f.Name = p.name()
			f.Colon = p.pos()
			p.want(_Colon)
			f.Pattern = p.matchPattern()
			n.Fields = append(n.Fields, f)
			if !p.got(_Comma) {
				break
			}
		}
		n.Rbrace = p.pos()
		p.want(_Rbrace)
	}
	if p.tok == _Question {
		q := &MatchPattern{Inner: n, Question: p.pos()}
		q.pos = n.Pos()
		p.next()
		n = q
	}
	return n
}

func (p *parser) matchSwitch(s *SwitchStmt) Stmt {
	var arrow bool
	for _, c := range s.Body {
		arrow = arrow || c.Arrow.IsKnown()
	}
	if !arrow {
		return s
	}
	m := &MatchExpr{Tag: s.Tag, Rbrace: s.Rbrace}
	m.pos = s.Pos()
	if s.Init != nil {
		p.errorAt(s.Init.Pos(), "match cannot have an init statement")
	}
	if s.Tag == nil {
		p.errorAt(s.Pos(), "match requires a value")
	}
	for _, c := range s.Body {
		if !c.Arrow.IsKnown() {
			p.errorAt(c.Pos(), "cannot mix : and => in one switch")
		}
		a := &MatchArm{Pattern: c.Pattern, Guard: c.Guard, Arrow: c.Arrow}
		a.pos = c.Pos()
		if len(c.Body) == 1 {
			a.Body, _ = c.Body[0].(*BlockStmt)
		}
		m.Arms = append(m.Arms, a)
	}
	out := &MatchStmt{Match: m}
	out.pos = s.Pos()
	return out
}

func (p *parser) patternFromExpr(x Expr) *MatchPattern {
	n := &MatchPattern{}
	n.pos = x.Pos()
	switch x := x.(type) {
	case *OptionalExpr:
		n.Inner, n.Question = p.patternFromExpr(x.X), x.Question
	case *ParenExpr:
		n.Inner, n.Lparen, n.Rparen = p.patternFromExpr(x.X), x.Pos(), EndPos(x)
	case *Name, *SelectorExpr, *IndexExpr, *BasicLit, *Operation:
		n.Value = x
	case *CallExpr:
		n.Value, n.Lparen, n.Rparen = x.Fun, x.Pos(), EndPos(x)
		for _, a := range x.ArgList {
			n.Args = append(n.Args, p.patternFromExpr(a))
		}
		if x.HasDots {
			p.errorAt(x.Pos(), "positional patterns cannot use ...")
		}
		for _, name := range x.ArgNames {
			if name != nil {
				p.errorAt(name.Pos(), "positional patterns cannot use named arguments")
			}
		}
	case *CompositeLit:
		n.Value, n.Lbrace, n.Rbrace = x.Type, x.Pos(), x.Rbrace
		for i, a := range x.ElemList {
			if marker, ok := a.(*Name); ok && marker.Value == "..." {
				n.Rest = marker.Pos()
				if i != len(x.ElemList)-1 {
					p.errorAt(marker.Pos(), "pattern rest must be last")
				}
				continue
			}
			kv, ok := a.(*KeyValueExpr)
			if !ok {
				p.errorAt(a.Pos(), "record patterns require field names")
				continue
			}
			name, ok := kv.Key.(*Name)
			if !ok {
				p.errorAt(kv.Key.Pos(), "record pattern field must be a name")
				continue
			}
			f := &MatchField{Name: name, Pattern: p.patternFromExpr(kv.Value)}
			f.pos = kv.Pos()
			n.Fields = append(n.Fields, f)
		}
	default:
		n.Value = p.badExpr()
		p.errorAt(x.Pos(), "invalid match pattern")
	}
	return n
}

func (p *printer) printAlternative(n Node) {
	switch n := n.(type) {
	case *EnumConstructExpr:
		p.print(_Name, "<enum construction>")
	case *EnumType:
		p.print(_Name, "enum", blank)
		if n.String.IsKnown() {
			p.print(_Name, "string", blank)
		}
		p.print(_Lbrace, newline, indent)
		for _, v := range n.Variants {
			p.print(v, _Semi, newline)
		}
		p.print(outdent, _Rbrace)
	case *EnumVariant:
		if n.Default.IsKnown() {
			p.print(_Default, blank)
		}
		p.print(n.Name)
		if n.Record {
			p.print(blank, _Lbrace, newline, indent)
			for _, f := range n.Payload {
				p.print(f.Name, blank, f.Type, _Semi, newline)
			}
			p.print(outdent, _Rbrace)
		} else if n.Ldelim.IsKnown() {
			p.print(_Lparen)
			for i, f := range n.Payload {
				if i > 0 {
					p.print(_Comma, blank)
				}
				p.print(f.Type)
			}
			p.print(_Rparen)
		}
		if n.Value != nil {
			p.print(blank, _Assign, blank, n.Value)
		}
	case *MatchExpr:
		p.print(_Switch, blank, n.Tag, blank, _Lbrace, newline, indent)
		for _, a := range n.Arms {
			p.print(a, _Semi, newline)
		}
		p.print(outdent, _Rbrace)
	case *MatchStmt:
		p.print(n.Match)
	case *MatchArm:
		if n.Pattern == nil {
			p.print(_Default)
		} else {
			p.print(_Case, blank, n.Pattern)
		}
		if n.Guard != nil {
			p.print(blank, _If, blank, n.Guard)
		}
		p.print(blank, _FatArrow, blank)
		if n.Body != nil {
			p.print(n.Body)
		} else {
			p.print(n.Value)
		}
	case *MatchPattern:
		if n.Inner != nil {
			if n.Lparen.IsKnown() {
				p.print(_Lparen)
			}
			p.print(n.Inner)
			if n.Rparen.IsKnown() {
				p.print(_Rparen)
			}
			if n.Question.IsKnown() {
				p.print(_Question)
			}
			return
		}
		p.print(n.Value)
		if n.Lparen.IsKnown() {
			p.print(_Lparen)
			for i, a := range n.Args {
				if i > 0 {
					p.print(_Comma, blank)
				}
				p.print(a)
			}
			p.print(_Rparen)
		} else if n.Lbrace.IsKnown() {
			p.print(_Lbrace)
			for i, f := range n.Fields {
				if i > 0 {
					p.print(_Comma, blank)
				}
				p.print(f)
			}
			if n.Rest.IsKnown() {
				if len(n.Fields) > 0 {
					p.print(_Comma, blank)
				}
				p.print(_DotDotDot)
			}
			p.print(_Rbrace)
		}
	case *MatchField:
		p.print(n.Name, _Colon, blank, n.Pattern)
	}
}
