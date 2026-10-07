# Current Gon extension validation

The Gon 2.27 integration passes targeted checks on macOS arm64 with VS Code
1.140.0, the native Gon toolchain at `/Users/tzbk/Documents/gon`, and the
unmodified Go 1.27.1 baseline. Run from `extension/`:

```sh
VSCODE_EXECUTABLE='/Applications/Visual Studio Code.app/Contents/MacOS/Code' npm run test:syntax
npm run compile
GON_ROOT=/Users/tzbk/Documents/gon \
GO_ERROR_HANDLING_BASELINE=/opt/homebrew/bin/go \
GON_TEST_FEATURES_ONLY=1 \
VSCODE_EXECUTABLE='/Applications/Visual Studio Code.app/Contents/MacOS/Code' npm test
npm run package
```

All exited 0. The actual VS Code TextMate/Oniguruma tokenizer checks ordinary
and generic enums, string enums, native optional types and presence patterns,
contextual `is`, block and one-line `or`, and quoted/raw interpolation. Embedded
expressions cover nested quotes, maps, slice indexes, named calls, conditional
expressions, nested interpolation, comments and fmt verbs. Escaped dollars and
ordinary Go identifiers retain their meanings. Match indentation and packaging
of all eight Gon snippets pass; retired Result constructors and snippets are
absent. TypeScript checking and bundling pass.

The isolated extension host uses the actual language client to verify enum,
optional, error-tuple and named-call hover, definitions, signatures and formatting.
Pattern bindings complete in later `&&` operands, and references inside
interpolation navigate and rename together with their declaration. Multi-pattern
arms, one-line error context and interpolation execute in the modern example;
modern Gon, legacy Gon and unmodified Go produce identical asserted output.

The same focused run executes **Gon: Check Project** with NilAway disabled and
enabled through the `gon.serverSettings` object. Its task arguments omit or
include `--nilaway` accordingly, and both checks complete with exit 0. Server
settings remain an open object; no extra setting schema blocks new gonpls keys.

`npm run package` regenerated `gon-0.1.0.vsix`. Archive inspection confirms the
current interpolation grammar, embedded Gon expression scopes, all eight
snippets, language configuration and bundled client. The package was not
published. This update did not rerun the separate full debugger and test explorer
suite; its earlier macOS evidence remains separate from these current language
and configuration checks.
