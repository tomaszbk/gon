# Install Gon 2.27

[Back to Gon](../README.md) · [Examples](gon-examples.md)

Download the toolchain archive for your computer and `gon-0.1.0.vsix` from
[Gon 2.27.0](https://github.com/tomaszbk/gon/releases/tag/v2.27.0).
Prebuilt toolchains are provided for macOS arm64/amd64, Linux arm64/amd64 and
Windows amd64. The compiler also targets riscv64 and wasm; those architectures
do not have standalone host archives in this release.

## Verify and unpack

The release includes `SHA256SUMS`. Verify your download before extracting it:

```sh
# macOS
shasum -a 256 gon-2.27.0-darwin-arm64.tar.gz
# Linux
sha256sum gon-2.27.0-linux-amd64.tar.gz
```

On Windows use `Get-FileHash FILE -Algorithm SHA256`. Compare the result with
the matching line in `SHA256SUMS`. Extract the archive into a permanent location;
keep the complete `gon-2.27.0` directory together.

On macOS/Linux, from that extracted directory:

```sh
python3 misc/gon/install.py
```

The installer exposes `gon` and `gonpls`. Open a new terminal and restart your
editor after installation. Alternatively add only the extracted directory's
`gon/bin` to PATH; this also works on Windows. Keep the private `bin` directory
off PATH so your ordinary Go installation stays available.

```sh
gon version
gon capabilities --json
```

`gon version` reports the underlying Go development provenance. The archive's
`GON_RELEASE.json` identifies the Gon release, source commit, target platform
and compiler hash. Gon 2.27 supports existing Go 1.27+ source, subject to the
documented newline-after-prefix-`!` exception; the upstream development version
does not change that support policy.

## Install the VS Code extension

In VS Code, use **Extensions: Install from VSIX...** and select the downloaded
`gon-0.1.0.vsix`, or run:

```sh
code --install-extension gon-0.1.0.vsix --force
```

Disable the official Go extension **for the Gon workspace**, then reload VS
Code. Gon discovers the installed public tools automatically. Optional path
settings and other editors are covered in the
[editor guide](../misc/gon/README.md#vs-code).
The VSIX is distributed through GitHub Releases; Marketplace installation is
not required.

## Use Gon in your project

From your application's module:

```sh
gon test ./...
gon build ./...
gon vet ./...
```

Keep `.go`, `_test.go`, `go.mod`, `go.sum` and `go.work`. Adopt new syntax
gradually with [Gon by example](gon-examples.md). The packaged
[Gon agent skill](../.agents/skills/gon/SKILL.md) can be copied into your project
with `python3 /path/to/gon-2.27.0/misc/gon/install.py --project-skill /path/to/project`.

For cloud/application builds see [containers](containers.md). Use
`ghcr.io/tomaszbk/gon:v2.27.0` for this release; the `dev` image follows tested
master. Pin a release tag, `sha-FULL_COMMIT_SHA` tag or digest for reproducible
builds.
