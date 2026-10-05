package misc

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/protocol/command"
	. "golang.org/x/tools/gopls/internal/test/integration"
)

// gonResultFiles exercises the predeclared Result type, a generic Gon enum
// whose type name has no package, in method signatures: value and pointer
// receivers, interface methods, generic receivers, an optional Result, an
// alias, and uses from a second package and a test file.
const gonResultFiles = `
-- go.mod --
module example.com/res

go 1.27
-- svc/svc.go --
package svc

import (
	"context"
	"errors"
	"fmt"
)

type Course struct{ Name string }

type Holder struct {
	R Result[int, error]
	N int
}

func NewHolder() Holder {
	h := Holder{}
	var zero Result[int, error]
	h.R = zero
	return h
}

type Role enum string {
	default Unknown(string)
	Teacher = "teacher"
}

func (s *Server) Roles() Result[[]Role, error] { return .Ok(nil) }

type Server struct{ courses []Course }

// ListCourses returns the courses.
func (s *Server) ListCourses(ctx context.Context) Result[[]Course, error] {
	return .Ok(s.courses)
}

func (s Server) ValueRecv(n int) Result[int, error] {
	if n < 0 {
		return .Err(errors.New("negative"))
	}
	return .Ok(n)
}

func (s *Server) listOptional(ctx context.Context) Result[Course, error]? {
	return nil
}

func (s *Server) takesResult(r Result[int, error]) Result[int, error] {
	n := r or err {
		return .Err(err)
	}
	return .Ok(n + 1)
}

type Store interface {
	Load(id int) Result[Course, error]
	Save(c Course) Result[struct{}, error]
}

type memStore struct{}

func (memStore) Load(id int) Result[Course, error]     { return .Ok(Course{}) }
func (memStore) Save(c Course) Result[struct{}, error] { return .Ok(struct{}{}) }

type Box[T any] struct{ v T }

func (b Box[T]) Get() Result[T, error] { return .Ok(b.v) }

type Loaded = Result[Course, error]

type Loader interface{ Load(id int) Loaded }

// IntLoader has the method name of Store but a different Result instantiation.
type IntLoader interface{ Load(id int) Result[int, error] }

type aliased struct{}

func (aliased) Load(id int) Loaded { return .Ok(Course{}) }

func Use(s *Server, st Store) Result[int, error] {
	c := st.Load(1)!
	_ = c
	n := s.ValueRecv(2)!
	return .Ok(n)
}

func Describe(r Result[int, error]) string {
	return switch r {
	case Result[int, error].Ok(n) => fmt.Sprint(n)
	case Result[int, error].Err(err) => err.Error()
	}
}

func TestLike(r *Result[int, error]) {}

func scale(width, height int) int { return width * height }

func Sized() int {
	v := scale(height: 2, width: 3)
	return v
}

func Reported(r Result[int, error]) Result[int, error] {
	n := r or err {
		return .Err(err)
	}
	return .Ok(n + 1)
}

func Fallback(err error) int {
	switch err {
	case nil:
		return 0
	}
	return 1
}
-- app/app.go --
package app

import (
	"context"

	"example.com/res/svc"
)

type remote struct{}

func (remote) Load(id int) Result[svc.Course, error]     { return .Ok(svc.Course{}) }
func (remote) Save(c svc.Course) Result[struct{}, error] { return .Ok(struct{}{}) }

var _ svc.Store = remote{}

func Run(ctx context.Context, s *svc.Server) Result[int, error] {
	courses := s.ListCourses(ctx)!
	return .Ok(len(courses))
}
-- svc/svc_test.go --
package svc

import (
	"context"
	"testing"
)

func TestResultPointer(r *Result[int, error]) {}

func BenchmarkResultPointer(e *error) {}

func TestUse(t *testing.T) {
	s := &Server{}
	r := s.ListCourses(context.Background())
	_ = r
	if got := Use(s, memStore{}); got != Result[int, error].Ok(2) {
		t.Fatal(got)
	}
}
`

var gonSweepWord = regexp.MustCompile(`[A-Za-z_][A-Za-z_0-9]*|[!?.]|=>`)

// TestGonResultMethodsSweep drives language-server requests at every
// word of files that use methods returning the predeclared Result, which
// has no package, to check that no operation assumes that every named type
// has a package. Request errors are acceptable; panics are not.
func TestGonResultMethodsSweep(t *testing.T) {
	WithOptions(
		Settings{"codelenses": map[string]bool{"test": true}},
	).Run(t, gonResultFiles, func(t *testing.T, env *Env) {
		for _, name := range []string{"svc/svc.go", "app/app.go", "svc/svc_test.go"} {
			env.OpenFile(name)
		}
		env.AfterChange()
		ctx := env.Ctx
		srv := env.Editor.Server
		for _, name := range []string{"svc/svc.go", "app/app.go", "svc/svc_test.go"} {
			uri := env.Sandbox.Workdir.URI(name)
			doc := protocol.TextDocumentIdentifier{URI: uri}
			content := env.BufferText(name)
			_, _ = srv.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{TextDocument: doc})
			_, _ = srv.SemanticTokensFull(ctx, &protocol.SemanticTokensParams{TextDocument: doc})
			_, _ = srv.FoldingRange(ctx, &protocol.FoldingRangeParams{TextDocument: doc})
			_, _ = srv.CodeLens(ctx, &protocol.CodeLensParams{TextDocument: doc})
			_, _ = srv.InlayHint(ctx, &protocol.InlayHintParams{TextDocument: doc, Range: protocol.Range{End: protocol.Position{Line: 10000}}})
			codeAction(ctx, srv, doc, protocol.Range{End: protocol.Position{Line: 10000}})
			for lineno, line := range strings.Split(content, "\n") {
				for _, m := range gonSweepWord.FindAllStringIndex(line, -1) {
					pos := protocol.Position{Line: uint32(lineno), Character: uint32(m[0])}
					at := protocol.TextDocumentPositionParams{TextDocument: doc, Position: pos}
					_, _ = srv.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: at})
					_, _ = srv.Definition(ctx, &protocol.DefinitionParams{TextDocumentPositionParams: at})
					_, _ = srv.TypeDefinition(ctx, &protocol.TypeDefinitionParams{TextDocumentPositionParams: at})
					_, _ = srv.Implementation(ctx, &protocol.ImplementationParams{TextDocumentPositionParams: at})
					_, _ = srv.References(ctx, &protocol.ReferenceParams{TextDocumentPositionParams: at, Context: protocol.ReferenceContext{IncludeDeclaration: true}})
					_, _ = srv.DocumentHighlight(ctx, &protocol.DocumentHighlightParams{TextDocumentPositionParams: at})
					_, _ = srv.SignatureHelp(ctx, &protocol.SignatureHelpParams{TextDocumentPositionParams: at})
					_, _ = srv.Completion(ctx, &protocol.CompletionParams{TextDocumentPositionParams: at})
					_, _ = srv.PrepareRename(ctx, &protocol.PrepareRenameParams{TextDocumentPositionParams: at})
					_, _ = srv.Rename(ctx, &protocol.RenameParams{TextDocumentPositionParams: at, NewName: "Renamed"})
					if items, _ := srv.PrepareCallHierarchy(ctx, &protocol.CallHierarchyPrepareParams{TextDocumentPositionParams: at}); len(items) > 0 {
						_, _ = srv.IncomingCalls(ctx, &protocol.CallHierarchyIncomingCallsParams{Item: items[0]})
						_, _ = srv.OutgoingCalls(ctx, &protocol.CallHierarchyOutgoingCallsParams{Item: items[0]})
					}
					if items, _ := srv.PrepareTypeHierarchy(ctx, &protocol.TypeHierarchyPrepareParams{TextDocumentPositionParams: at}); len(items) > 0 {
						_, _ = srv.Supertypes(ctx, &protocol.TypeHierarchySupertypesParams{Item: items[0]})
						_, _ = srv.Subtypes(ctx, &protocol.TypeHierarchySubtypesParams{Item: items[0]})
					}
					rng := protocol.Range{Start: pos, End: protocol.Position{Line: pos.Line, Character: uint32(m[1])}}
					codeAction(ctx, srv, doc, rng)
				}
			}
		}
		_, _ = srv.Symbol(ctx, &protocol.WorkspaceSymbolParams{Query: "Load"})
		_, _ = srv.Symbol(ctx, &protocol.WorkspaceSymbolParams{Query: "Result"})
		_, _ = srv.Symbol(ctx, &protocol.WorkspaceSymbolParams{Query: ""})
	})
}

// codeAction requests the code actions for a range and resolves each one,
// which computes its edit.
func codeAction(ctx context.Context, srv protocol.Server, doc protocol.TextDocumentIdentifier, rng protocol.Range) {
	actions, _ := srv.CodeAction(ctx, &protocol.CodeActionParams{TextDocument: doc, Range: rng})
	for _, action := range actions {
		if action.Edit == nil && action.Command == nil {
			_, _ = srv.ResolveCodeAction(ctx, &action)
		}
	}
}

// TestGonResultAliasImplementInterface checks that adding methods to an alias
// of the predeclared Result, whose type name has no package, is refused
// instead of dereferencing a missing package.
func TestGonResultAliasImplementInterface(t *testing.T) {
	const files = `
-- go.mod --
module example.com/alias

go 1.27
-- a.go --
package a

type Loaded = Result[int, error]

type Local struct{}
`
	Run(t, files, func(t *testing.T, env *Env) {
		env.OpenFile("a.go")
		env.AfterChange()

		implement := func(name string) error {
			args, err := command.MarshalArgs(command.ImplementInterfaceArgs{
				Location: env.RegexpSearch("a.go", name),
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = env.Editor.Server.ExecuteCommand(env.Ctx, &protocol.ExecuteCommandParams{
				Command:   command.ImplementInterface.String(),
				Arguments: args,
				InteractiveParams: protocol.InteractiveParams{
					FormAnswers: []protocol.FormAnswer{{ID: "interface", Value: "error"}},
				},
			})
			return err
		}

		err := implement("Loaded")
		if err == nil || !strings.Contains(err.Error(), "predeclared type Result") {
			t.Errorf("implement_interface on an alias of Result: got error %v, want one mentioning the predeclared type Result", err)
		}

		// A package-level type of the package is still accepted.
		env.Await(NoDiagnostics())
		if err := implement("Local"); err != nil {
			t.Errorf("implement_interface on a local type failed: %v", err)
		}
	})
}

// TestGonResultMethodsImplementations checks that interface satisfaction by
// method sets distinguishes instantiations of Result, within one package and
// across packages (where the method sets come from separate indexes).
func TestGonResultMethodsImplementations(t *testing.T) {
	Run(t, gonResultFiles, func(t *testing.T, env *Env) {
		env.OpenFile("svc/svc.go")
		env.AfterChange()

		// describe returns "file:text" for each location, sorted.
		describe := func(locs []protocol.Location) []string {
			var got []string
			for _, loc := range locs {
				name := env.Sandbox.Workdir.URIToPath(loc.URI)
				mapper := protocol.NewMapper(loc.URI, []byte(env.FileContent(name)))
				start, end, err := mapper.RangeOffsets(loc.Range)
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, name+":"+string(mapper.Content[start:end]))
			}
			slices.Sort(got)
			return got
		}
		check := func(what string, locs []protocol.Location, want ...string) {
			t.Helper()
			if got := describe(locs); !slices.Equal(got, want) {
				t.Errorf("%s: got %q, want %q", what, got, want)
			}
		}

		// Store.Load is implemented by the methods of memStore and, in
		// another package, remote.
		check("Store.Load", env.Implementations(env.RegexpSearch("svc/svc.go", `(Load)\(id int\) Result\[Course`)),
			"app/app.go:Load", "svc/svc.go:Load")
		// Loader spells the result type through an alias; the
		// interface Store satisfies it too.
		check("Loader", env.Implementations(env.RegexpSearch("svc/svc.go", `type (Loader) interface`)),
			"app/app.go:remote", "svc/svc.go:Store", "svc/svc.go:aliased", "svc/svc.go:memStore")
		// Types: Store (Load and Save) is implemented by memStore and remote.
		check("Store", env.Implementations(env.RegexpSearch("svc/svc.go", `type (Store) interface`)),
			"app/app.go:remote", "svc/svc.go:memStore")
		// A different instantiation of Result is a different method type.
		check("IntLoader", env.Implementations(env.RegexpSearch("svc/svc.go", `type (IntLoader) interface`)))

		// References to a method that returns Result span packages and tests.
		refs := env.References(env.RegexpSearch("svc/svc.go", `func \(s \*Server\) (ListCourses)`))
		check("ListCourses", refs, "app/app.go:ListCourses", "svc/svc.go:ListCourses", "svc/svc_test.go:ListCourses")
	})
}
