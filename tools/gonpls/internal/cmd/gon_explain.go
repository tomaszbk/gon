package cmd

import (
	"context"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/gopls/internal/settings"
	"golang.org/x/tools/gopls/internal/util/safetoken"
)

type gonExplainResult struct {
	gonEnvelope
	Explanations []*gonExplanation `json:"explanations"`
}

// gonExplanation documents a diagnostic code or analyzer of the selected
// toolchain.
type gonExplanation struct {
	Code           string        `json:"code"`
	Kind           string        `json:"kind,omitempty"` // "type-error" or "analyzer"
	Number         *int          `json:"number,omitempty"`
	Summary        string        `json:"summary,omitempty"`
	Documentation  string        `json:"documentation,omitempty"`
	Cases          []gonCase     `json:"cases,omitempty"`
	DefaultEnabled *bool         `json:"defaultEnabled,omitempty"`
	URL            string        `json:"url,omitempty"`
	Source         string        `json:"source,omitempty"`
	References     []string      `json:"references,omitempty"`
	Error          *gonErrorInfo `json:"error,omitempty"`
}

// gonCase explains one message of a Gon diagnostic code.
type gonCase struct {
	Message string `json:"message"`
	Meaning string `json:"meaning"`
	Fix     string `json:"fix"`
}

// gonErrorHandlingCases are diagnostics of postfix !, local handlers and error context.
// They mirror the messages of types2 and go/types.
var gonErrorHandlingCases = []gonCase{
	{"error handling requires a function or method call",
		"The error-return protocol requires a function call.",
		"Call a function returning error last."},
	{"error handling is only permitted inside a function",
		"! and or handlers need an enclosing function, for example not in a package-level var.",
		"Move the call into a function, such as init or a helper returning error."},
	{"error handling requires a final result of type error",
		"The called function's last result must have exactly type error (an alias is accepted). " +
			"Named interfaces, concrete error types, type parameters and non-final errors do not qualify.",
		"Keep the explicit form: v, err := f(); if err != nil { ... }."},
	{"error propagation requires an enclosing function with a final result of type error",
		"Postfix ! and one-line or error context return from the nearest enclosing function literal, lambda or declaration. That function must return error last, including in _test.go files. Failure returns zeros for all other results.",
		"Handle the error locally with 'or err { ... }'. Tests can explicitly call t.Fatal(err) and return in a block handler. Adding an error result changes the function's contract; review its callers before choosing that alternative."},
	{"error context requires an explicit error binding",
		"A one-line error context names the error before transforming it. The transformation runs only on failure.",
		"Write call() or err => expr, where expr is assignable to error; the enclosing function must permit postfix !."},
	{"error handler requires an explicit error binding",
		"An or handler must name the error it receives.",
		"Write 'call() or err { ... }'; the binding may be left unused."},
	{"labels are not permitted in error handlers",
		"Labeled statements inside an or handler are not supported yet.",
		"Move the labeled statement outside the handler or into a function literal."},
	{"branches to labels are not permitted in error handlers",
		"goto and labeled break/continue cannot leave or target an or handler.",
		"Restructure the control flow outside the handler; unlabeled loops inside the handler are allowed."},
	{"error handler for a value-producing call must terminate",
		"When the call has results besides its error, every path through the handler must end in " +
			"return, panic or another terminating statement, because execution cannot resume without those values.",
		"End the handler with a return or panic. Handlers of error-only calls may fall through."},
}

func (r *gonRequest) explain(ctx context.Context) (gonResult, error) {
	root := gonRoot(r.inv.env)
	codes, codesFile, err := gonTypeErrorCodes(root)
	if err != nil {
		return nil, gonErrorf(gonKindWorkspace, gonExitInfra, "reading the toolchain's error codes: %v", err)
	}
	result := &gonExplainResult{}
	for _, arg := range r.args {
		name := strings.TrimSuffix(strings.TrimPrefix(arg, "ErrorCode("), ")")
		ex := &gonExplanation{Code: arg}
		if n, err := strconv.Atoi(name); err == nil {
			for _, c := range codes {
				if c.number == n {
					name = c.name
				}
			}
		}
		if c := gonFindCode(codes, name); c != nil {
			n := c.number
			ex.Code, ex.Kind, ex.Number = c.name, "type-error", &n
			ex.Documentation = c.doc
			ex.Summary = gonFirstSentence(c.doc)
			ex.Source = fmt.Sprintf("%s:%d", codesFile, c.line)
			if c.name == "InvalidLambda" {
				ex.Cases = []gonCase{
					{"lambda requires a function type from context", "A lambda takes its parameter and result types from its use.", "Use a typed declaration, assignment, argument or explicit function-type conversion; remove parentheses around the whole lambda."},
					{"lambda parameters cannot have types; use a function literal", "Only parenthesized identifier parameters are supported.", "Write (x) => body, or use func(x T) R { ... } when spelling types."},
				}
				if doc := filepath.Join(root, "design", "lambda", "README.md"); gonExists(doc) {
					ex.References = append(ex.References, doc)
				}
			}
			if c.name == "InvalidNilSafety" {
				ex.Cases = []gonCase{
					{"cannot be nil; use ?? to provide a value when it is absent", "A safe-navigation chain may stop before producing a result whose type has no nil value.", "Consume the chain with ?? and supply a default."},
					{"safe navigation requires a pointer, interface or function", "?. guards pointer or interface operands; ?( guards a function value. Native optionals guard presence and wrap the final result in an optional.", "Use the guard corresponding to the operand, or explicit nil checks."},
					{"absence propagation requires an enclosing function returning exactly one optional", "Postfix ? returns absence from the nearest compatible function. Present typed nil and zero payloads remain present.", "Return one U? result, or handle absence with ?? or exhaustive matching."},
					{"mixed optional and nil navigation requires an explicit boundary", "Optional presence and Go nil guards need an explicit boundary where their chains mix.", "Extract or coalesce the optional payload, then start a separate nil-navigation chain."},
				}
				if doc := filepath.Join(root, "design", "null-safety", "README.md"); gonExists(doc) {
					ex.References = append(ex.References, doc)
				}
			}
			if c.name == "InvalidMatch" {
				ex.Cases = []gonCase{
					{"match alternatives must bind the same names with identical types", "Every pattern sharing an arm writes the same binding variables. The guard runs once after the first successful alternative.", "Use matching binding names and types across alternatives, or split the arm."},
					{"pattern test bindings require an if condition or a top-level && operand of that condition", "Bindings from x is P are visible to later top-level && operands and the then body, and are absent from else.", "Move the test to the if condition or a top-level && operand, or use a binding-free pattern."},
					{"pattern test must not use a pattern that always matches", "A pattern test must distinguish at least two cases; a wildcard or irrefutable binding does not test anything.", "Use a presence, literal or variant pattern that can fail."},
					{"non-exhaustive match", "Every alternative and nested payload case needs coverage. Guards do not establish complete coverage.", "Add the missing patterns, an irrefutable payload binding, or an explicit default/_ arm."},
					{"record pattern must list every field or explicitly ignore the rest with ...", "Record variants use named fields and explicit rest patterns.", "List the remaining accessible fields or add ... to ignore the remainder."},
					{"unreachable match arm", "An earlier unguarded pattern already covers every value in this arm.", "Remove the arm or make its earlier covering pattern more specific."},
					{"match guard must be boolean", "A guard is an ordinary boolean expression in the pattern bindings' scope.", "Write a boolean test; its result controls whether this arm is selected."},
					{"pattern alternative can never match interface", "On an interface subject a variant pattern is a type test, so its enum type must implement the interface (with value receivers).", "Use an enum type whose value type implements the interface, or match a different subject."},
					{"enum alternatives never cover the interface type", "Variant patterns on an interface subject never prove exhaustiveness; for the predeclared error they search the error tree like errors.As.", "Add a default or case _ arm for every other value, including nil."},
				}
				if doc := filepath.Join(root, "design", "alternatives", "README.md"); gonExists(doc) {
					ex.References = append(ex.References, doc)
				}
			}
			if c.name == "InvalidInterpolation" {
				ex.Cases = []gonCase{
					{`string interpolation requires an explicit import of "fmt" in this file`, "Interpolation formats its operands with fmt.Sprintf; its dependency is resolved from the file's import path even with an alias or a shadowed fmt name.", `Add import "fmt" in this file; the editor offers an import quick fix.`},
					{"interpolation format must be one fmt verb consuming one operand (without * or %%)", "Each ${expression:verb} contributes exactly one operand in written order.", "Use one fmt verb such as %v, %q or %.2f; remove * widths and %% from the verb."},
				}
			}

			if c.name == "InvalidErrorHandling" {
				ex.Cases = append([]gonCase(nil), gonErrorHandlingCases...)
				if doc := filepath.Join(root, "design", "error-handling", "README.md"); gonExists(doc) {
					ex.References = append(ex.References, doc)
				}
			}
		} else if a := gonFindAnalyzer(name); a != nil {
			enabled := a.Enabled(settings.DefaultOptions())
			ex.Code, ex.Kind, ex.DefaultEnabled = a.Analyzer().Name, "analyzer", &enabled
			ex.Documentation = strings.TrimSpace(a.Analyzer().Doc)
			ex.Summary = gonFirstSentence(ex.Documentation)
			ex.URL = a.Analyzer().URL
		} else {
			ex.Error = &gonErrorInfo{Kind: gonKindNotFound,
				Message: fmt.Sprintf("unknown code %q; use a code or analyzer name reported by 'gon check'", arg)}
		}
		result.Explanations = append(result.Explanations, ex)
	}
	result.gonEnvelope = *r.envelope("explain")
	return result, nil
}

func gonExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type gonCodeDoc struct {
	name   string
	number int
	doc    string
	line   int
}

// gonTypeErrorCodes reads the type-checker error codes of the selected
// toolchain, so that explanations always match the compiler in use.
func gonTypeErrorCodes(root string) ([]gonCodeDoc, string, error) {
	path := filepath.Join(root, "src", "internal", "types", "errors", "codes.go")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, path, err
	}
	// Evaluate the constants with the type checker, so that any constant
	// expression, such as Gon's separate range, yields its value.
	conf := types.Config{Error: func(error) {}}
	pkg, _ := conf.Check("errors", fset, []*ast.File{f}, nil)
	var codes []gonCodeDoc
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			for _, id := range vs.Names {
				c, ok := pkg.Scope().Lookup(id.Name).(*types.Const)
				if id.Name == "_" || !ok {
					continue
				}
				value, exact := constant.Int64Val(c.Val())
				if !exact {
					continue
				}
				codes = append(codes, gonCodeDoc{
					name: id.Name, number: int(value),
					doc:  strings.TrimSpace(vs.Doc.Text()),
					line: safetoken.StartPosition(fset, id.Pos()).Line,
				})
			}
		}
	}
	return codes, path, nil
}

func gonFindCode(codes []gonCodeDoc, name string) *gonCodeDoc {
	for i := range codes {
		if codes[i].name == name {
			return &codes[i]
		}
	}
	return nil
}

func gonFindAnalyzer(name string) *settings.Analyzer {
	for _, a := range settings.AllAnalyzers {
		if a.Analyzer().Name == name {
			return a
		}
	}
	return nil
}

func gonFirstSentence(doc string) string {
	para, _, _ := strings.Cut(doc, "\n\n")
	para = strings.Join(strings.Fields(para), " ")
	if i := strings.Index(para, ". "); i >= 0 {
		return para[:i+1]
	}
	return para
}

func (e *gonExplainResult) exitCode() int {
	for _, ex := range e.Explanations {
		if ex.Error != nil {
			return gonExitFindings
		}
	}
	return gonExitOK
}

func (e *gonExplainResult) text(w io.Writer, r *gonRequest) {
	for i, ex := range e.Explanations {
		if i > 0 {
			fmt.Fprintln(w)
		}
		if ex.Error != nil {
			fmt.Fprintf(w, "%s: %s\n", ex.Code, ex.Error.Message)
			continue
		}
		fmt.Fprintf(w, "%s (%s", ex.Code, ex.Kind)
		if ex.Number != nil {
			fmt.Fprintf(w, " %d", *ex.Number)
		}
		fmt.Fprintf(w, ")\n\n%s\n", ex.Documentation)
		for _, c := range ex.Cases {
			fmt.Fprintf(w, "\n%q\n  %s\n  Fix: %s\n", c.Message, c.Meaning, c.Fix)
		}
		for _, ref := range ex.References {
			fmt.Fprintf(w, "\nSee %s\n", ref)
		}
		if ex.URL != "" {
			fmt.Fprintf(w, "\n%s\n", ex.URL)
		}
	}
}
