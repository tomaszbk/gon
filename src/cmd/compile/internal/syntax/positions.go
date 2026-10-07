// This file implements helper functions for scope position computations.

package syntax

// StartPos returns the start position of n.
func StartPos(n Node) Pos {
	// Cases for nodes which don't need a correction are commented out.
	for m := n; ; {
		switch n := m.(type) {
		case nil:
			panic("nil node")

		// packages
		case *File:
			// file block starts at the beginning of the file
			return MakePos(n.Pos().Base(), 1, 1)

		// declarations
		// case *ImportDecl:
		// case *ConstDecl:
		// case *TypeDecl:
		// case *VarDecl:
		// case *FuncDecl:

		// expressions
		// case *BadExpr:
		// case *Name:
		// case *BasicLit:
		case *CompositeLit:
			if n.Type != nil {
				m = n.Type
				continue
			}
			return n.Pos()
		case *KeyValueExpr:
			m = n.Key
		// case *FuncLit:
		// case *ParenExpr:
		case *OptionalExpr:
			m = n.X
		case *NilGuardExpr:
			m = n.X
		case *SafeNavExpr:
			m = n.X
		case *SelectorExpr:
			m = n.X
		case *IndexExpr:
			m = n.X
		// case *SliceExpr:
		case *AssertExpr:
			m = n.X
		case *TypeSwitchGuard:
			if n.Lhs != nil {
				m = n.Lhs
				continue
			}
			m = n.X
		case *Operation:
			if n.Y != nil {
				m = n.X
				continue
			}
			return n.Pos()
		case *ErrorExpr:
			m = n.X
		// case *CondExpr:
		case *CallExpr:
			m = n.Fun
		case *ListExpr:
			if len(n.ElemList) > 0 {
				m = n.ElemList[0]
				continue
			}
			return n.Pos()
		// types
		// case *ArrayType:
		// case *SliceType:
		// case *DotsType:
		// case *StructType:
		// case *Field:
		// case *InterfaceType:
		// case *FuncType:
		// case *MapType:
		// case *ChanType:

		// statements
		// case *EmptyStmt:
		// case *LabeledStmt:
		// case *BlockStmt:
		// case *ExprStmt:
		case *SendStmt:
			m = n.Chan
		// case *DeclStmt:
		case *AssignStmt:
			m = n.Lhs
		// case *BranchStmt:
		// case *CallStmt:
		// case *ReturnStmt:
		// case *IfStmt:
		// case *ForStmt:
		// case *SwitchStmt:
		// case *SelectStmt:

		// helper nodes
		case *RangeClause:
			if n.Lhs != nil {
				m = n.Lhs
				continue
			}
			m = n.X
		// case *CaseClause:
		// case *CommClause:

		default:
			return n.Pos()
		}
	}
}

// EndPos returns the approximate end position of n in the source.
// For some nodes (*Name, *BasicLit) it returns the position immediately
// following the node; for others (*BlockStmt, *SwitchStmt, etc.) it
// returns the position of the closing '}'; and for some (*ParenExpr)
// the returned position is the end position of the last enclosed
// expression.
// Thus, EndPos should not be used for exact demarcation of the
// end of a node in the source; it is mostly useful to determine
// scope ranges where there is some leeway.
func EndPos(n Node) Pos {
	for m := n; ; {
		switch n := m.(type) {
		case *InterpolatedStringExpr:
			return n.Rquote
		case *InterpolationPart:
			return n.End
		case nil:
			panic("nil node")

		// packages
		case *File:
			return n.EOF

		// declarations
		case *ImportDecl:
			m = n.Path
		case *ConstDecl:
			if n.Values != nil {
				m = n.Values
				continue
			}
			if n.Type != nil {
				m = n.Type
				continue
			}
			if l := len(n.NameList); l > 0 {
				m = n.NameList[l-1]
				continue
			}
			return n.Pos()
		case *TypeDecl:
			m = n.Type
		case *VarDecl:
			if n.Values != nil {
				m = n.Values
				continue
			}
			if n.Type != nil {
				m = n.Type
				continue
			}
			if l := len(n.NameList); l > 0 {
				m = n.NameList[l-1]
				continue
			}
			return n.Pos()
		case *FuncDecl:
			if n.Body != nil {
				m = n.Body
				continue
			}
			m = n.Type

		case *OptionalExpr:
			return n.Question
		case *EnumType:
			return n.Rbrace
		case *EnumVariant:
			if n.Value != nil {
				m = n.Value
				continue
			}
			if n.Rdelim.IsKnown() {
				return n.Rdelim
			}
			m = n.Name
		case *MatchExpr:
			return n.Rbrace
		case *MatchStmt:
			m = n.Match
		case *MatchArm:
			if n.Body != nil {
				m = n.Body
			} else if n.Value != nil {
				m = n.Value
			} else {
				return n.Arrow
			}
		case *MatchPattern:
			if n.Question.IsKnown() {
				return n.Question
			}
			if n.Inner != nil && !n.Rparen.IsKnown() {
				m = n.Inner
				continue
			}
			if n.Rbrace.IsKnown() {
				return n.Rbrace
			}
			if n.Rparen.IsKnown() {
				return n.Rparen
			}
			if n.Value != nil {
				m = n.Value
			} else {
				return n.Pos()
			}
		case *MatchField:
			m = n.Pattern
		case *PatternTestExpr:
			m = n.Pattern

		// expressions
		case *BadExpr:
			return n.Pos()
		case *Name:
			p := n.Pos()
			return MakePos(p.Base(), p.Line(), p.Col()+uint(len(n.Value)))
		case *BasicLit:
			p := n.Pos()
			return MakePos(p.Base(), p.Line(), p.Col()+uint(len(n.Value)))
		case *CompositeLit:
			return n.Rbrace
		case *KeyValueExpr:
			m = n.Value
		case *FuncLit:
			m = n.Body
		case *LambdaExpr:
			if n.Block != nil {
				m = n.Block
			} else {
				m = n.Body
			}
		case *NilGuardExpr:
			return n.Question
		case *SafeNavExpr:
			m = n.X
		case *ParenExpr:
			m = n.X
		case *SelectorExpr:
			m = n.Sel
		case *IndexExpr:
			m = n.Index
		case *SliceExpr:
			for i := len(n.Index) - 1; i >= 0; i-- {
				if x := n.Index[i]; x != nil {
					m = x
					continue
				}
			}
			m = n.X
		case *AssertExpr:
			m = n.Type
		case *TypeSwitchGuard:
			m = n.X
		case *Operation:
			if n.Y != nil {
				m = n.Y
				continue
			}
			m = n.X
		case *ErrorExpr:
			if n.Context != nil {
				m = n.Context
				continue
			}
			if n.Body != nil {
				m = n.Body
				continue
			}
			p := n.Pos()
			return MakePos(p.Base(), p.Line(), p.Col()+1)
		case *CondExpr:
			return n.Rbrace

		case *CallExpr:
			if l := lastExpr(n.ArgList); l != nil {
				m = l
				continue
			}
			m = n.Fun
		case *ListExpr:
			if l := lastExpr(n.ElemList); l != nil {
				m = l
				continue
			}
			return n.Pos()

		// types
		case *ArrayType:
			m = n.Elem
		case *SliceType:
			m = n.Elem
		case *DotsType:
			m = n.Elem
		case *StructType:
			if l := lastField(n.FieldList); l != nil {
				m = l
				continue
			}
			return n.Pos()
			// TODO(gri) need to take TagList into account
		case *Field:
			if n.Type != nil {
				m = n.Type
				continue
			}
			m = n.Name
		case *InterfaceType:
			if l := lastField(n.MethodList); l != nil {
				m = l
				continue
			}
			return n.Pos()
		case *FuncType:
			if l := lastField(n.ResultList); l != nil {
				m = l
				continue
			}
			if l := lastField(n.ParamList); l != nil {
				m = l
				continue
			}
			return n.Pos()
		case *MapType:
			m = n.Value
		case *ChanType:
			m = n.Elem

		// statements
		case *EmptyStmt:
			return n.Pos()
		case *LabeledStmt:
			m = n.Stmt
		case *BlockStmt:
			return n.Rbrace
		case *ExprStmt:
			m = n.X
		case *SendStmt:
			m = n.Value
		case *DeclStmt:
			if l := lastDecl(n.DeclList); l != nil {
				m = l
				continue
			}
			return n.Pos()
		case *AssignStmt:
			m = n.Rhs
			if m == nil {
				p := EndPos(n.Lhs)
				return MakePos(p.Base(), p.Line(), p.Col()+2)
			}
		case *BranchStmt:
			if n.Label != nil {
				m = n.Label
				continue
			}
			return n.Pos()
		case *CallStmt:
			m = n.Call
		case *ReturnStmt:
			if n.Results != nil {
				m = n.Results
				continue
			}
			return n.Pos()
		case *IfStmt:
			if n.Else != nil {
				m = n.Else
				continue
			}
			m = n.Then
		case *ForStmt:
			m = n.Body
		case *SwitchStmt:
			return n.Rbrace
		case *SelectStmt:
			return n.Rbrace

		// helper nodes
		case *RangeClause:
			m = n.X
		case *CaseClause:
			if l := lastStmt(n.Body); l != nil {
				m = l
				continue
			}
			return n.Colon
		case *CommClause:
			if l := lastStmt(n.Body); l != nil {
				m = l
				continue
			}
			return n.Colon

		default:
			return n.Pos()
		}
	}
}

func lastDecl(list []Decl) Decl {
	if l := len(list); l > 0 {
		return list[l-1]
	}
	return nil
}

func lastExpr(list []Expr) Expr {
	if l := len(list); l > 0 {
		return list[l-1]
	}
	return nil
}

func lastStmt(list []Stmt) Stmt {
	if l := len(list); l > 0 {
		return list[l-1]
	}
	return nil
}

func lastField(list []*Field) *Field {
	if l := len(list); l > 0 {
		return list[l-1]
	}
	return nil
}
