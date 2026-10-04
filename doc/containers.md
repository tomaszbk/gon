# Gon containers

The development toolchain image is `ghcr.io/tomaszbk/gon:dev`, linked to the
[Gon repository](https://github.com/tomaszbk/gon). It includes `gon`, `gonpls`,
the standard library, Git, CA certificates and the C/C++ tools needed by cgo.
The public PATH exposes only `gon` and `gonpls`; private Go-compatible tools
remain inside `/opt/gon`.

Published platforms are Linux `amd64` and `arm64`. These container builds do
not establish validation evidence for `riscv64` or `wasm`.

## Try the toolchain

```sh
docker run --rm ghcr.io/tomaszbk/gon:dev gon version
docker run --rm ghcr.io/tomaszbk/gon:dev gon capabilities --json
docker run --rm -v "$PWD:/workspace" ghcr.io/tomaszbk/gon:dev gon test ./...
```

Run the last command from your application module. Keep the existing `.go`
files, `go.mod`, `go.sum` and any `go.work`. Gon selects its compiler itself;
no extra language configuration is needed.

## Build an application for the cloud

Use Gon in the build stage, then copy the executable into a small runtime
image. This example assumes an application at `./cmd/server` that listens on
port 8080 and can build with cgo disabled:

```dockerfile
FROM ghcr.io/tomaszbk/gon:dev AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN gon mod download
COPY . .
RUN CGO_ENABLED=0 gon build -trimpath -o /out/server ./cmd/server

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /out/server /server
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/server"]
```

If your project has no `go.sum`, copy only `go.mod` before `gon mod download`.
Workspace projects must also copy the workspace files and local modules before
downloading dependencies. An application requiring cgo needs a runtime image
with its shared libraries instead of `scratch`.

Build for the platform your cloud provider runs, then push to its accepted
registry. Replace the sample registry and image name with your own:

```sh
docker build --platform linux/amd64 -t YOUR_REGISTRY/your-app:VERSION .
docker push YOUR_REGISTRY/your-app:VERSION
```

Use `linux/arm64` for an ARM service. The application image contains your
executable and runtime data; deployment does not need a Gon compiler.

## Tags and publication

`dev` follows the latest successfully tested `master` commit. It is a moving
development tag, not a stable Gon 2.27 release. For reproducible application
builds, use the corresponding `sha-FULL_COMMIT_SHA` tag or image digest.

The [container workflow](../.github/workflows/container.yml) builds both
architectures on standard GitHub-hosted Linux runners. The Docker build runs
the existing nine-feature legacy/modern examples, with unmodified Go 1.27.1
also executing the legacy program. The installed image then builds and runs
both variants, checks gonpls/semantic commands and compiles a cgo program.
Only passing images are pushed; `dev` advances after both platforms pass.
Pull requests build and test without publishing. Publication uses the
repository's `GITHUB_TOKEN`, with no personal-token secret required.

GitHub makes [public packages free](https://docs.github.com/en/packages/learn-github-packages/introduction-to-github-packages#about-billing-for-github-packages),
and [standard runners are free for public repositories](https://docs.github.com/en/billing/concepts/product-billing/github-actions).
The cost of running your application in a cloud provider is separate.

GitHub initially creates container packages as private, even for a public
repository. After the first publication, open the
[package settings](https://github.com/users/tomaszbk/packages/container/gon/settings)
and set the package visibility to **Public**. Public GHCR images can then be
pulled without credentials. See
[GitHub's visibility documentation](https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility).

## Build locally

From this repository root:

```sh
docker build --build-arg GON_REVISION="$(git rev-parse HEAD)" -t gon:local .
python3 misc/gon/test_container.py gon:local
```

The build context excludes local instructions/design documents, Git metadata
and generated host binaries. Only the compiled distribution is copied into
the final stage.
