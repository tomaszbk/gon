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

// gonErrorHandlingCases are the diagnostics of postfix ! and "or name { }".
// They mirror the messages of types2 and go/types.
var gonErrorHandlingCases = []gonCase{
	{"error handling requires a function or method call",
		"The legacy error-return protocol requires a function call. Canonical Result values also support ! and or handlers.",
		"Call a function returning error last, or use a canonical Result value with an explicit handler or compatible Result return type."},
	{"error handling is only permitted inside a function",
		"! and or handlers need an enclosing function, for example not in a package-level var.",
		"Move the call into a function, such as init or a helper returning error."},
	{"error handling requires a final result of type error",
		"The called function's last result must have exactly type error (an alias is accepted). " +
			"Named interfaces, concrete error types, type parameters and non-final errors do not qualify.",
		"Keep the explicit form: v, err := f(); if err != nil { ... }."},
	{"error propagation requires an enclosing function with a final result of type error, exactly one Result, or, in a _test.go file, a first named parameter of type *testing.T, *testing.B, *testing.F or testing.TB",
		"Postfix ! on a Go error tuple returns from the nearest enclosing function literal, lambda or declaration. That function must return error last, " +
			"return exactly one Result, or be a test function: in a _test.go file, a function that does not qualify otherwise and whose first parameter is named " +
			"(not blank) with type *testing.T, *testing.B, *testing.F or testing.TB reports the failure with Fatal, at the line of the !, and then returns zero values. " +
			"Method receivers are not parameters.",
		"Handle the error locally with 'or err { ... }', or make the function a test function: name its first parameter and use the standard testing type. " +
			"Adding an error or Result result changes the function's contract; review its callers before choosing that design alternative."},
	{"error propagation into a Result requires error to be assignable to its error type",
		"Inside a function returning exactly one Result[T, E], ! on a Go error tuple fails as Result[T, E].Err(err), converting the error to E. " +
			"The error type must therefore be assignable to E, for example error, any or an interface that error implements.",
		"Return Result[T, error], or handle the error with 'or err { return .Err(convert(err)) }'."},
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
					{"safe navigation requires a pointer or interface", "?. guards pointer or interface operands; ?( guards a function value. Canonical Option checks Some/None and wraps the final result in Option.", "Use the guard corresponding to the operand, or explicit nil checks."},
					{"absence propagation requires an enclosing function returning exactly one Option", "Postfix ? returns None from the nearest compatible function. Some(nil) and Some(zero) remain present.", "Return exactly Option[U], or handle None with ?? or exhaustive matching."},
					{"mixed Option and nil navigation requires an explicit boundary", "The first implementation keeps Option presence and Go nil guards in separate chains.", "Extract or coalesce the Option payload, then start a separate nil-navigation chain."},
				}
				if doc := filepath.Join(root, "design", "null-safety", "README.md"); gonExists(doc) {
					ex.References = append(ex.References, doc)
				}
			}
			if c.name == "InvalidMatch" {
				ex.Cases = []gonCase{
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
			if c.name == "InvalidErrorHandling" {
				ex.Cases = append(append([]gonCase(nil), gonErrorHandlingCases...),
					gonCase{"Result propagation requires exactly one enclosing Result with an assignable error type, a final result of type error, or, in a _test.go file, a first named parameter of type *testing.T, *testing.B, *testing.F or testing.TB",
						"Result ! propagates Err by its variant, including Err(nil), to the nearest function returning exactly one Result with an assignable error type, " +
							"to a function whose last result is error when the payload is assignable to error, or, in a test function (see the previous case), to Fatal.",
						"Return exactly Result[U,F] with the source error assignable to F, return error last, name a first *testing.T, *testing.B, *testing.F or testing.TB parameter in a _test.go file, or handle it explicitly with or problem { ... }."},
					gonCase{"Result propagation into a final result of type error requires an error type assignable to error",
						"Inside a function returning error last, ! on a Result returns the payload as the error, with zero values for the other results. " +
							"A nil payload is still a failure: it is returned as errors.ErrNilResult, never as a nil error. " +
							"The payload type must be assignable to error; a typed nil pointer payload stays a non-nil error, as in Go.",
						"Use a Result whose error type is error or implements it, or handle the payload with 'or problem { return ..., convert(problem) }'."},
					gonCase{"Result error handler must terminate", "Result has one success payload even when its type is struct{}.", "End every handler path with return, panic, or another terminating statement."})
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
