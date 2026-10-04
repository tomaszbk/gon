package completion

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/gopls/internal/protocol"
)

// namedArgumentLabels offers names from the visible static signature at the
// start of an argument. It returns true when editing an existing label, where
// lexical values are not valid replacements.
func (c *completer) namedArgumentLabels() bool {
	var call *ast.CallExpr
	for _, node := range c.path {
		if candidate, ok := node.(*ast.CallExpr); ok {
			call = candidate
			break
		}
	}
	if call == nil || c.pos <= call.Lparen || c.pos > call.Rparen {
		return false
	}
	typ := c.pkg.TypesInfo().TypeOf(call.Fun)
	if typ == nil {
		return false
	}
	sig, _ := typ.Underlying().(*types.Signature)
	if sig == nil {
		return false
	}
	current := -1
	labelOnly := false
	for i, arg := range call.Args {
		if i < len(call.ArgNames) && call.ArgNames[i] != nil {
			name := call.ArgNames[i]
			if name.Pos() <= c.pos && c.pos <= name.End() {
				current, labelOnly = i, true
				break
			}
		}
		if arg.Pos() <= c.pos && c.pos <= arg.End() {
			if _, ok := arg.(*ast.Ident); !ok || i < len(call.ArgNames) && call.ArgNames[i] != nil {
				return false
			}
			current = i
			break
		}
	}
	if current < 0 {
		// Empty calls are useful completion sites; elsewhere only a direct
		// identifier or existing label is sufficiently unambiguous.
		if len(call.Args) != 0 {
			return false
		}
		current = 0
	}
	// Introducing a named argument before a later positional argument would
	// produce an invalid call. Existing label edits remain available while
	// repairing an incomplete call.
	if !labelOnly {
		for i := current + 1; i < len(call.Args); i++ {
			if i >= len(call.ArgNames) || call.ArgNames[i] == nil {
				return false
			}
		}
	}
	used := make(map[string]bool)
	for i := range call.Args {
		if i == current {
			continue
		}
		if i < len(call.ArgNames) && call.ArgNames[i] != nil {
			used[call.ArgNames[i].Name] = true
		} else if i < current && i < sig.Params().Len() {
			used[sig.Params().At(i).Name()] = true
		}
	}
	for param := range sig.Params().Variables() {
		name := param.Name()
		if name == "" || name == "_" || used[name] {
			continue
		}
		score := c.matcher.Score(name)
		if score == 0 {
			continue
		}
		insert := name + ": "
		if labelOnly {
			insert = name // the colon and value already exist
		}
		c.items = append(c.items, CompletionItem{
			Label: name + ":", InsertText: insert,
			Detail: types.TypeString(param.Type(), c.qual),
			Kind:   protocol.VariableCompletion, Score: highScore * float64(score),
		})
	}
	return labelOnly
}
