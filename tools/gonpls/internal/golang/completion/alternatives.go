package completion

import (
	"go/ast"
	"go/types"
)

// contextualVariants uses the target signature, rather than lexical lookup,
// for a leading-dot constructor. Pattern variants stay qualified.
func (c *completer) contextualVariants() bool {
	for _, node := range c.path {
		construction, ok := node.(*ast.ContextualVariantExpr)
		if !ok || c.pos < construction.Dot || c.pos > construction.Name.End() {
			continue
		}
		typ := c.inference.objType
		if !types.IsCanonicalResult(typ) {
			return true
		}
		c.deepState.enabled = false
		enum := types.EnumOf(typ)
		for i := range enum.NumVariants() {
			c.deepState.enqueue(candidate{obj: enum.Variant(i).Object(), enumVariant: enum.Variant(i), score: highScore})
		}
		return true
	}
	return false
}

// matchPatternFields completes record labels using the variant's public
// payload, excluding fields already present in the pattern. Bindings and
// guards continue through normal lexical completion in the arm scope.
func (c *completer) matchPatternFields() bool {
	for _, node := range c.path {
		pattern, ok := node.(*ast.MatchPattern)
		if !ok || !pattern.Lbrace.IsValid() || c.pos <= pattern.Lbrace || c.pos > pattern.Rbrace {
			continue
		}
		var active *ast.MatchField
		for _, field := range pattern.Fields {
			if field.Name.Pos() <= c.pos && c.pos <= field.Name.End() {
				active = field
				break
			}
		}
		if active == nil && len(pattern.Fields) != 0 {
			return false
		}
		head, ok := pattern.Value.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		enum := types.EnumOf(c.pkg.TypesInfo().TypeOf(head.X))
		if enum == nil {
			return false
		}
		variant := enum.Lookup(head.Sel.Name, c.pkg.Types())
		if variant == nil || !variant.IsRecord() {
			return false
		}
		existing := map[string]bool{}
		for _, field := range pattern.Fields {
			if field != active {
				existing[field.Name.Name] = true
			}
		}
		c.deepState.enabled = false
		for i := 0; i < variant.NumFields(); i++ {
			field := variant.Field(i)
			if !existing[field.Name()] && (field.Exported() || field.Pkg() == c.pkg.Types()) {
				c.deepState.enqueue(candidate{obj: field, score: highScore})
			}
		}
		return true
	}
	return false
}

// Existing record braces should be reused, as with ordinary call completion.
func (c *completer) enumRecordContext() (pattern, braces bool) {
	for _, node := range c.path {
		switch node := node.(type) {
		case *ast.MatchPattern:
			if node.Value != nil && node.Value.Pos() <= c.pos && c.pos <= node.Value.End() {
				return true, node.Lbrace.IsValid()
			}
		case *ast.CompositeLit:
			if node.Type != nil && node.Type.Pos() <= c.pos && c.pos <= node.Type.End() {
				return false, node.Lbrace.IsValid() && node.Lbrace != node.Rbrace
			}
		}
	}
	return false, false
}

// A record variant acts as the header of a constructor literal, even though
// its descriptor object is a value rather than an independent named type.
func (c *completer) enumRecordLiteralCandidate(cand *candidate) bool {
	return cand.enumVariant != nil && cand.enumVariant.IsRecord() && c.inference.typeName.compLitType
}
