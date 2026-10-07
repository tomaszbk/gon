package ast

import "go/token"

// InterpolatedStringExpr is a string whose embedded expressions are formatted
// by fmt.Sprintf. Quote is '"' for interpreted text or '`' for raw text.
type InterpolatedStringExpr struct {
	Dollar token.Pos
	Quote  byte
	Parts  []*InterpolationPart
	Rquote token.Pos
}

func (x *InterpolatedStringExpr) Pos() token.Pos { return x.Dollar }
func (x *InterpolatedStringExpr) End() token.Pos { return x.Rquote + 1 }
func (*InterpolatedStringExpr) exprNode()        {}

// InterpolationPart is either raw source Text or an Expr with an optional fmt
// Format. Text retains escapes; Start includes the ${ for an expression part.
type InterpolationPart struct {
	Start, EndPos token.Pos
	Text          string
	Expr          Expr
	Format        string
}

func (x *InterpolationPart) Pos() token.Pos { return x.Start }
func (x *InterpolationPart) End() token.Pos { return x.EndPos }
