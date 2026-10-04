// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ast

import "go/token"

// An EnumType declares a closed set of alternatives. Enum is the position
// of the contextual identifier "enum", which remains an ordinary Go name
// outside a type declaration followed immediately by its body.
type EnumType struct {
	Enum     token.Pos
	Lbrace   token.Pos
	Variants []*EnumVariant
	Rbrace   token.Pos
}

// An EnumVariant declares one alternative. A nil Payload denotes a unit
// variant; otherwise Record distinguishes named fields from positional types.
// Default is the position of the optional "default" keyword.
type EnumVariant struct {
	Doc     *CommentGroup
	Default token.Pos
	Name    *Ident
	Payload *FieldList
	Record  bool
	Comment *CommentGroup
}

// A MatchExpr selects one value by matching Tag against its arms. The same
// structure occurs in MatchStmt, whose arms contain Body instead of Value.
type MatchExpr struct {
	Switch token.Pos
	Tag    Expr
	Lbrace token.Pos
	Arms   []*MatchArm
	Rbrace token.Pos
}

// A MatchStmt selects an ordinary statement block by matching a value.
type MatchStmt struct{ Match *MatchExpr }

// A MatchArm has either a Value (expression match) or a Body (statement
// match). A nil Pattern denotes a default arm. Bindings belong to the arm,
// including its optional Guard, rather than to the enclosing switch scope.
type MatchArm struct {
	Case    token.Pos
	Pattern *MatchPattern
	Guard   Expr
	Arrow   token.Pos
	Value   Expr
	Body    *BlockStmt
}

// A MatchPattern is structural syntax, never an executable expression.
// Value holds a literal, a binding name, or a qualified alternative name.
// Valid parentheses delimit positional Args, and valid braces delimit Fields.
// Rest marks an explicit "..." in a record pattern. Empty argument or field
// lists are distinguished from unit alternatives by their delimiters.
type MatchPattern struct {
	Inner    *MatchPattern // grouped pattern, or payload pattern when Question is valid
	Question token.Pos
	Value    Expr
	Lparen   token.Pos
	Args     []*MatchPattern
	Rparen   token.Pos
	Lbrace   token.Pos
	Fields   []*MatchField
	Rest     token.Pos
	Rbrace   token.Pos
}

// A MatchField associates a record payload's field with its pattern.
type MatchField struct {
	Name    *Ident
	Colon   token.Pos
	Pattern *MatchPattern
}

func (x *EnumType) Pos() token.Pos { return x.Enum }
func (x *EnumType) End() token.Pos { return x.Rbrace + 1 }
func (*EnumType) exprNode()        {}
func (x *EnumVariant) Pos() token.Pos {
	if x.Default.IsValid() {
		return x.Default
	}
	return x.Name.Pos()
}
func (x *EnumVariant) End() token.Pos {
	if x.Payload != nil {
		return x.Payload.End()
	}
	return x.Name.End()
}
func (x *MatchExpr) Pos() token.Pos { return x.Switch }
func (x *MatchExpr) End() token.Pos { return x.Rbrace + 1 }
func (*MatchExpr) exprNode()        {}
func (x *MatchStmt) Pos() token.Pos { return x.Match.Pos() }
func (x *MatchStmt) End() token.Pos { return x.Match.End() }
func (*MatchStmt) stmtNode()        {}
func (x *MatchArm) Pos() token.Pos  { return x.Case }
func (x *MatchArm) End() token.Pos {
	if x.Body != nil {
		return x.Body.End()
	}
	if x.Value != nil {
		return x.Value.End()
	}
	return x.Arrow + 2
}
func (x *MatchPattern) Pos() token.Pos {
	if x.Inner != nil {
		if x.Lparen.IsValid() {
			return x.Lparen
		}
		return x.Inner.Pos()
	}
	return x.Value.Pos()
}
func (x *MatchPattern) End() token.Pos {
	if x.Question.IsValid() {
		return x.Question + 1
	}
	if x.Rbrace.IsValid() {
		return x.Rbrace + 1
	}
	if x.Rparen.IsValid() {
		return x.Rparen + 1
	}
	return x.Value.End()
}
func (x *MatchField) Pos() token.Pos { return x.Name.Pos() }
func (x *MatchField) End() token.Pos { return x.Pattern.End() }

// An OptionalExpr represents postfix "?". In a type position it abbreviates
// canonical Option[X]; in a value position it propagates None and yields the
// Some payload, evaluating its operand once.
type OptionalExpr struct {
	X        Expr
	Question token.Pos
}

func (x *OptionalExpr) Pos() token.Pos { return x.X.Pos() }
func (x *OptionalExpr) End() token.Pos { return x.Question + 1 }
func (*OptionalExpr) exprNode()        {}

// A ContextualVariantExpr constructs a canonical Option or Result variant using
// the type expected by its surrounding expression. Name is None, Some, Ok, or
// Err. None has no parentheses; the other variants take one payload expression.
type ContextualVariantExpr struct {
	Dot            token.Pos
	Name           *Ident
	Lparen, Rparen token.Pos
	Args           []Expr
}

func (x *ContextualVariantExpr) Pos() token.Pos { return x.Dot }
func (x *ContextualVariantExpr) End() token.Pos {
	if x.Rparen.IsValid() {
		return x.Rparen + 1
	}
	return x.Name.End()
}
func (*ContextualVariantExpr) exprNode() {}
