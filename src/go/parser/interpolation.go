package parser

import (
	"go/ast"
	"go/token"
)

func (p *parser) parseInterpolatedString() ast.Expr {
	x := &ast.InterpolatedStringExpr{Dollar: p.pos, Quote: p.lit[0]}
	p.next()
	for p.tok != token.INTERPOLATION_END && p.tok != token.EOF {
		part := &ast.InterpolationPart{Start: p.pos}
		switch p.tok {
		case token.INTERPOLATION_TEXT:
			part.Text = p.lit
			part.EndPos = p.end()
			p.next()
		case token.INTERPOLATION_OPEN:
			p.next()
			outer := p.exprLev
			p.exprLev = 0
			part.Expr = p.parseRhs()
			p.exprLev = outer
			if p.tok == token.INTERPOLATION_FORMAT {
				if p.lit == "" {
					p.errorExpected(p.pos, "non-empty interpolation format")
				}
				part.Format = p.lit
				p.next()
			}
			part.EndPos = p.end()
			p.expect(token.INTERPOLATION_CLOSE)
		default:
			p.errorExpected(p.pos, "interpolated string part")
			p.next()
			continue
		}
		x.Parts = append(x.Parts, part)
	}
	x.Rquote = p.expect(token.INTERPOLATION_END)
	return x
}
