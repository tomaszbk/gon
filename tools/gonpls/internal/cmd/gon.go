package cmd

// This file implements the command-line tooling that the public gon command
// adds to the Go command: query, refactor, check, explain and capabilities.
// The gon launcher forwards these subcommands to "gonpls gon". Each command
// starts a gonpls session in this process, analyzes the saved files and exits;
// no state is kept between commands. They never parse or type-check Gon with a
// separate implementation. misc/gon/CLI.md specifies the versioned contract.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/gopls/internal/debug"
	goplsversion "golang.org/x/tools/gopls/internal/version"
)

// gonSchemaVersion identifies the JSON contract. Incompatible changes to
// existing fields or their meaning require a new version.
const gonSchemaVersion = 1

// Exit codes shared by every gon tooling command.
const (
	gonExitOK       = 0 // completed; nothing requested failed
	gonExitFindings = 1 // completed with errors, rejected changes or unresolved targets
	gonExitUsage    = 2 // invalid command line
	gonExitInfra    = 3 // the workspace or toolchain could not be used
)

// RunGon runs a gon tooling command and returns its exit status.
func RunGon(ctx context.Context, args []string) int {
	cwd, err := os.Getwd()
	if err == nil {
		// Use canonical paths throughout, as for command-line file arguments,
		// so that workspace roots and file locations agree (e.g. macOS /tmp).
		cwd, err = filepath.EvalSymlinks(cwd)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "gon: %v\n", err)
		return gonExitInfra
	}
	// Route server log events through the client, which prints only errors
	// and warnings, as the other gonpls commands do.
	ctx = debug.WithInstance(ctx, "")
	inv := &gonInvocation{stdout: os.Stdout, stderr: os.Stderr, cwd: cwd, env: os.Environ()}
	return inv.run(ctx, args)
}

// A gonInvocation is one execution of a gon tooling command.
type gonInvocation struct {
	stdout, stderr io.Writer
	cwd            string
	env            []string
}

type gonFlagKind int

const (
	gonBool gonFlagKind = iota
	gonString
	gonInt
)

type gonFlag struct {
	name string
	kind gonFlagKind
	def  string
	help string
}

type gonCommand struct {
	path     string // e.g. "query refs"
	args     string // positional argument synopsis
	summary  string
	detail   string
	flags    []gonFlag
	min, max int  // positional argument bounds; max < 0 means unbounded
	semantic bool // uses the semantic engine
	run      func(r *gonRequest, ctx context.Context) (gonResult, error)
}

var (
	gonJSONFlag   = gonFlag{"json", gonBool, "false", "print the versioned JSON result"}
	gonTagsFlag   = gonFlag{"tags", gonString, "", "comma-separated build tags for the analyzed configuration"}
	gonLimitFlag  = gonFlag{"limit", gonInt, "100", "maximum number of items per target (0 means no limit)"}
	gonOffsetFlag = gonFlag{"offset", gonInt, "0", "number of leading items to skip"}
)

var gonSemanticFlags = []gonFlag{gonJSONFlag, gonTagsFlag}

func gonCommands() []*gonCommand {
	query := func(name, summary, detail string, extra ...gonFlag) *gonCommand {
		return &gonCommand{
			path: "query " + name, args: "<target>...", summary: summary, detail: detail,
			flags: append(append([]gonFlag{}, gonSemanticFlags...), append([]gonFlag{gonLimitFlag, gonOffsetFlag}, extra...)...),
			min:   1, max: -1, semantic: true,
			run: func(r *gonRequest, ctx context.Context) (gonResult, error) { return r.query(ctx, name) },
		}
	}
	return []*gonCommand{
		query("def", "show where targets are declared", "", gonFlag{"doc", gonBool, "false", "include documentation in text output"}),
		query("refs", "list references to targets, including their declarations", ""),
		query("impls", "list implementations of interfaces, or interfaces implemented by types", ""),
		query("type", "show the type of the expression or identifier at targets", ""),
		{
			path: "query symbols", args: "<query>...", summary: "search workspace symbols by fuzzy name",
			detail: "Results are fully qualified and can be used as symbol targets.",
			flags:  append(append([]gonFlag{}, gonSemanticFlags...), gonLimitFlag, gonOffsetFlag),
			min:    1, max: -1, semantic: true,
			run: func(r *gonRequest, ctx context.Context) (gonResult, error) { return r.query(ctx, "symbols") },
		},
		{
			path: "refactor rename", args: "<target> <new-name>", summary: "rename a symbol and all references",
			detail: "Without --dry-run the change is applied after checking that every file still\n" +
				"matches the analyzed revision. Files that were gofmt-clean are formatted.",
			flags: append(append([]gonFlag{}, gonSemanticFlags...),
				gonFlag{"dry-run", gonBool, "false", "print the plan (JSON) or a unified diff without writing files"}),
			min: 2, max: 2, semantic: true,
			run: (*gonRequest).rename,
		},
		{
			path: "refactor optionals", args: "[package|file.go]...",
			summary: "migrate retired optional syntax to native T? with a reviewable diff",
			flags:   append(append([]gonFlag{}, gonSemanticFlags...), gonFlag{"dry-run", gonBool, "true", "preview edits without writing files"}),
			min:     0, max: -1, semantic: true,
			run: (*gonRequest).migrateOptionals,
		},
		{
			path: "refactor apply", args: "<plan.json | ->", summary: "apply a revision-checked refactor plan",
			detail: "The plan is rejected, without writing any file, unless every file still has the\nanalyzed content.",
			flags:  []gonFlag{gonJSONFlag},
			min:    1, max: 1,
			run: (*gonRequest).applyPlanFile,
		},
		{
			path: "check", args: "[packages | files]", summary: "report parse, type and analysis diagnostics without building",
			detail: "Patterns select packages as in 'gon vet' (default \".\"); .go file arguments\n" +
				"report only those files. The result lists what was and was not verified.",
			flags: append(append([]gonFlag{}, gonSemanticFlags...),
				gonFlag{"severity", gonString, "warning", "minimum severity: error, warning, info or hint"},
				gonFlag{"category", gonString, "all", "language, analysis or all"},
				gonFlag{"code", gonString, "", "report only diagnostics with this code"},
				gonFlag{"staticcheck", gonBool, "false", "also run the Staticcheck analyzers"},
				gonFlag{"limit", gonInt, "200", "maximum number of diagnostics (0 means no limit)"},
				gonOffsetFlag),
			min: 0, max: -1, semantic: true,
			run: (*gonRequest).check,
		},
		{
			path: "explain", args: "<code>...", summary: "explain diagnostic codes and analyzers",
			detail: "Codes are the stable names reported by 'gon check', such as InvalidErrorHandling,\n" +
				"UndeclaredName, unusedwrite or SA4006. Numeric type-checker codes are accepted.",
			flags: []gonFlag{gonJSONFlag},
			min:   1, max: -1,
			run: (*gonRequest).explain,
		},
		{
			path: "capabilities", summary: "describe the tooling supported by this toolchain",
			flags: []gonFlag{gonJSONFlag},
			run:   (*gonRequest).capabilities,
		},
	}
}

// gonRequest holds a parsed command line.
type gonRequest struct {
	inv   *gonInvocation
	cmd   *gonCommand
	args  []string
	flags map[string]string

	// State for semantic commands; see gon_engine.go.
	engine *gonEngine
	docs   map[string]*gonDoc
	holds  []func()
}

func (r *gonRequest) bool(name string) bool  { return r.flags[name] == "true" }
func (r *gonRequest) str(name string) string { return r.flags[name] }
func (r *gonRequest) int(name string) int {
	n, _ := strconv.Atoi(r.flags[name]) // validated during parsing
	return n
}

func (inv *gonInvocation) run(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		topic := []string(nil)
		if len(args) > 0 {
			topic = args[1:]
		}
		return inv.help(topic)
	}
	cmd, rest := gonLookup(args)
	if cmd == nil {
		fmt.Fprintf(inv.stderr, "gon: unknown command %q; run 'gon help tooling'\n", strings.Join(args[:min(2, len(args))], " "))
		return gonExitUsage
	}
	r, err := parseGonRequest(inv, cmd, rest)
	if errors.Is(err, errGonHelp) {
		inv.commandHelp(inv.stdout, cmd)
		return gonExitOK
	}
	if err != nil {
		fmt.Fprintf(inv.stderr, "gon %s: %v\n", cmd.path, err)
		inv.commandHelp(inv.stderr, cmd)
		return gonExitUsage
	}
	return r.execute(ctx)
}

func gonLookup(args []string) (*gonCommand, []string) {
	for _, cmd := range gonCommands() {
		words := strings.Fields(cmd.path)
		if len(args) >= len(words) && slicesEqual(args[:len(words)], words) {
			return cmd, args[len(words):]
		}
	}
	return nil, nil
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var errGonHelp = errors.New("help requested")

// parseGonRequest accepts flags before, between and after positional
// arguments, as agents and scripts commonly write them. A "--" argument ends
// flag parsing.
func parseGonRequest(inv *gonInvocation, cmd *gonCommand, args []string) (*gonRequest, error) {
	r := &gonRequest{inv: inv, cmd: cmd, flags: make(map[string]string)}
	specs := make(map[string]gonFlag)
	for _, f := range cmd.flags {
		specs[f.name] = f
		r.flags[f.name] = f.def
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			r.args = append(r.args, args[i+1:]...)
			break
		}
		if len(arg) < 2 || arg[0] != '-' || arg == "-" {
			r.args = append(r.args, arg)
			continue
		}
		name := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
		value, hasValue := "", false
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name, value, hasValue = name[:eq], name[eq+1:], true
		}
		if name == "h" || name == "help" {
			return nil, errGonHelp
		}
		spec, ok := specs[name]
		if !ok {
			return nil, fmt.Errorf("unknown flag -%s", name)
		}
		switch spec.kind {
		case gonBool:
			if !hasValue {
				value = "true"
			}
			b, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("invalid boolean value %q for -%s", value, name)
			}
			value = strconv.FormatBool(b)
		default:
			if !hasValue {
				if i+1 >= len(args) {
					return nil, fmt.Errorf("flag -%s needs a value", name)
				}
				i++
				value = args[i]
			}
			if spec.kind == gonInt {
				if n, err := strconv.Atoi(value); err != nil || n < 0 {
					return nil, fmt.Errorf("invalid value %q for -%s: want a non-negative integer", value, name)
				}
			}
		}
		r.flags[name] = value
	}
	if len(r.args) < cmd.min || cmd.max >= 0 && len(r.args) > cmd.max {
		return nil, fmt.Errorf("wrong number of arguments")
	}
	return r, nil
}

// execute runs the command in this process and prints its result.
func (r *gonRequest) execute(ctx context.Context) int {
	defer r.release()
	result, err := r.cmd.run(r, ctx)
	if err != nil {
		return r.fail(err)
	}
	return r.emit(result)
}

// A gonResult is the successful (possibly partial) outcome of a command.
type gonResult interface {
	base() *gonEnvelope
	text(w io.Writer, r *gonRequest)
	exitCode() int
}

// gonEnvelope holds the fields common to every JSON result.
type gonEnvelope struct {
	SchemaVersion int               `json:"schemaVersion"`
	Operation     string            `json:"operation"`
	OK            bool              `json:"ok"`
	Toolchain     *gonToolchainInfo `json:"toolchain,omitempty"`
	Configuration *gonConfigInfo    `json:"configuration,omitempty"`
	Error         *gonErrorInfo     `json:"error,omitempty"`
}

func (e *gonEnvelope) base() *gonEnvelope { return e }

type gonToolchainInfo struct {
	Root           string `json:"root"`
	LanguageServer string `json:"languageServer"`
	Gonpls         string `json:"gonpls"`
	GoVersion      string `json:"goVersion,omitempty"`
}

type gonConfigInfo struct {
	Workspace   string   `json:"workspace"`
	GOOS        string   `json:"goos"`
	GOARCH      string   `json:"goarch"`
	GOFLAGS     string   `json:"goflags,omitempty"`
	BuildFlags  []string `json:"buildFlags,omitempty"`
	Staticcheck bool     `json:"staticcheck"`
}

type gonErrorInfo struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// A gonError is a failure with a stable kind and exit status.
type gonError struct {
	kind string
	exit int
	msg  string
}

func (e *gonError) Error() string { return e.msg }

func gonErrorf(kind string, exit int, format string, args ...any) error {
	return &gonError{kind: kind, exit: exit, msg: fmt.Sprintf(format, args...)}
}

// Error kinds. Infrastructure kinds use gonExitInfra.
const (
	gonKindUsage     = "usage"
	gonKindNotFound  = "not-found"
	gonKindAmbiguous = "ambiguous"
	gonKindRejected  = "rejected" // an analysis refused the operation (e.g. a rename conflict)
	gonKindStale     = "stale"    // files changed after analysis
	gonKindQuery     = "query-failed"
	gonKindWorkspace = "workspace"
	gonKindInternal  = "internal"
)

func asGonError(err error) *gonError {
	var ge *gonError
	if errors.As(err, &ge) {
		return ge
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &gonError{kind: gonKindInternal, exit: gonExitInfra, msg: err.Error()}
	}
	return &gonError{kind: gonKindQuery, exit: gonExitFindings, msg: err.Error()}
}

func (r *gonRequest) fail(err error) int {
	ge := asGonError(err)
	if r.json() {
		env := r.envelope(r.operation())
		env.OK = false
		env.Error = &gonErrorInfo{Kind: ge.kind, Message: ge.msg}
		r.writeJSON(env)
	} else {
		fmt.Fprintf(r.inv.stderr, "gon %s: %s\n", r.cmd.path, ge.msg)
	}
	return ge.exit
}

func (r *gonRequest) json() bool { return r.bool("json") }

func (r *gonRequest) operation() string { return strings.ReplaceAll(r.cmd.path, " ", ".") }

// envelope returns the common fields for a result of the given operation.
func (r *gonRequest) envelope(op string) *gonEnvelope {
	env := &gonEnvelope{SchemaVersion: gonSchemaVersion, Operation: op, OK: true, Toolchain: gonToolchain(r.inv.env)}
	if r.cmd.semantic {
		env.Configuration = r.configuration()
	}
	return env
}

func (r *gonRequest) emit(result gonResult) int {
	code := result.exitCode()
	env := result.base()
	env.OK = code == gonExitOK
	if r.json() {
		r.writeJSON(result)
	} else {
		result.text(r.inv.stdout, r)
	}
	return code
}

func (r *gonRequest) writeJSON(v any) {
	enc := json.NewEncoder(r.inv.stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	enc.Encode(v) // errors writing stdout are not recoverable here
}

// rel shortens a path for text output when it is inside the working directory.
func (r *gonRequest) rel(path string) string {
	if rel, err := filepath.Rel(r.inv.cwd, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

// abs resolves a command-line path against the invocation's directory.
func (r *gonRequest) abs(path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.inv.cwd, path)
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

func gonGetenv(env []string, name string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if v, ok := strings.CutPrefix(env[i], name+"="); ok {
			return v
		}
	}
	return ""
}

// gonRoot returns the selected toolchain. The launcher sets GOROOT; the
// private executable otherwise lives in GOROOT/pkg/tool/GOOS_GOARCH.
func gonRoot(env []string) string {
	if root := gonGetenv(env, "GOROOT"); root != "" {
		return root
	}
	if exe, err := os.Executable(); err == nil {
		if exe, err := filepath.EvalSymlinks(exe); err == nil {
			root := filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "..", ".."))
			// Elsewhere, as in a test binary, use the toolchain that built this program.
			if _, err := os.Stat(filepath.Join(root, "src", "go.mod")); err == nil {
				return root
			}
		}
	}
	return runtime.GOROOT()
}

func gonToolchain(env []string) *gonToolchainInfo {
	root := gonRoot(env)
	exe := "gonpls"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	return &gonToolchainInfo{
		Root:           root,
		LanguageServer: filepath.Join(root, "gon", "bin", exe),
		Gonpls:         goplsversion.Version(),
		GoVersion:      runtime.Version(),
	}
}

func (inv *gonInvocation) help(topic []string) int {
	if len(topic) > 0 && topic[0] != "tooling" {
		if cmd, _ := gonLookup(topic); cmd != nil {
			inv.commandHelp(inv.stdout, cmd)
			return gonExitOK
		}
		fmt.Fprintf(inv.stderr, "gon help: unknown topic %q\n", strings.Join(topic, " "))
		return gonExitUsage
	}
	fmt.Fprint(inv.stdout, `Gon tooling commands analyze saved files with the gonpls semantic engine.
Build, test, vet and fmt keep their Go-compatible behavior.

Usage:
  gon <command> [arguments] [flags]

Commands:
`)
	for _, cmd := range gonCommands() {
		fmt.Fprintf(inv.stdout, "  %-17s %s\n", cmd.path, cmd.summary)
	}
	fmt.Fprint(inv.stdout, `
Targets are file.go:line:column (1-based, byte columns), file.go:#offset, or
symbols: Name, Type.Method, pkg.Name, ./dir.Name, or import/path.Type.Method.
Flags may appear anywhere; --json prints schema version 1 results.
Exit status: 0 success, 1 findings or rejected operation, 2 usage, 3 infrastructure.
Run 'gon help <command>' for details, e.g. 'gon help query refs'.
`)
	return gonExitOK
}

func (inv *gonInvocation) commandHelp(w io.Writer, cmd *gonCommand) {
	fmt.Fprintf(w, "usage: gon %s", cmd.path)
	if cmd.args != "" {
		fmt.Fprintf(w, " %s", cmd.args)
	}
	fmt.Fprintf(w, " [flags]\n\n%s.\n", strings.ToUpper(cmd.summary[:1])+cmd.summary[1:])
	if cmd.detail != "" {
		fmt.Fprintf(w, "%s\n", cmd.detail)
	}
	if len(cmd.flags) > 0 {
		fmt.Fprintln(w, "\nFlags:")
		flags := append([]gonFlag(nil), cmd.flags...)
		sort.Slice(flags, func(i, j int) bool { return flags[i].name < flags[j].name })
		for _, f := range flags {
			name := f.name
			if f.kind != gonBool {
				name += "=" + map[gonFlagKind]string{gonString: "string", gonInt: "n"}[f.kind]
			}
			def := ""
			if f.def != "" && f.def != "false" && f.def != "0" {
				def = fmt.Sprintf(" (default %s)", f.def)
			}
			fmt.Fprintf(w, "  --%-18s %s%s\n", name, f.help, def)
		}
	}
}
