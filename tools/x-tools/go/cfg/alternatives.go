package cfg

import "go/ast"

// match keeps pattern tests, guards and arm bodies on separate paths. Pattern
// syntax does not evaluate arbitrary expressions: heads select variants and
// identifiers bind payload copies. This type-free graph conservatively treats
// every non-default pattern as a test; SSA has the exact typed pattern tree.
func (b *builder) match(e *ast.MatchExpr, stmt ast.Stmt, label *lblock) {
	b.add(e.Tag)
	done := b.newBlock(KindMatchDone, stmt)
	saved := b.targets
	if stmt != nil {
		b.targets = &targets{tail: saved, _break: done}
		if label != nil {
			label._break = done
		}
	}
	for i, arm := range e.Arms {
		body := b.newBlock(KindMatchArm, stmt)
		next := b.newBlock(KindMatchTest, stmt)
		// Exhaustiveness is mandatory in well-typed Gon matches, so the
		// final unguarded arm covers all values surviving earlier tests.
		if arm.Pattern == nil || i == len(e.Arms)-1 && arm.Guard == nil {
			b.jump(body)
		} else {
			// A MatchPattern is not an expression. The tag is the only
			// evaluated operand of a test, and is already evaluated once.
			// Retain the pattern as structural test syntax, never the tag again.
			b.current.Nodes = append(b.current.Nodes, arm.Pattern)
			b.ifelse(body, next)
		}
		b.current = body
		if arm.Guard != nil {
			guarded := b.newBlock(KindMatchArm, stmt)
			b.add(arm.Guard)
			b.ifelse(guarded, next)
			b.current = guarded
		}
		if stmt != nil {
			b.stmt(arm.Body)
		} else {
			b.add(arm.Value)
		}
		b.jump(done)
		b.current = next
	}
	b.jump(done)
	b.targets = saved
	b.current = done
}
