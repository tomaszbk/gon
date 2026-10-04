ARG BOOTSTRAP_IMAGE=golang:1.27.1-bookworm
FROM ${BOOTSTRAP_IMAGE} AS build

RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/gon
COPY . .

# The Docker context has no Git metadata. Preserve the compiler's underlying
# Go development version, rather than claiming Gon 2.27 is already released.
ARG GON_REVISION=local
RUN python3 -c 'from pathlib import Path; import re, sys; version = re.search(r"(?m)^const Version = (\d+)", Path("src/internal/goversion/goversion.go").read_text())[1]; Path("VERSION").write_text(f"go1.{version}-devel_gon_{sys.argv[1]}\n")' "$GON_REVISION"

ENV GOTOOLCHAIN=local GOENV=off GOFLAGS=""
RUN cd src && env -u GOROOT GOROOT_BOOTSTRAP=/usr/local/go ./make.bash
RUN python3 misc/gon/build.py \
    && GON_BASELINE_GO=/usr/local/go/bin/go python3 misc/gon/test_readme.py

FROM debian:bookworm-slim

# This is a build image: include cgo's C/C++ toolchain and module fetching.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential ca-certificates git pkg-config tzdata \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /opt/gon/bin /opt/gon/bin
COPY --from=build /opt/gon/gon/bin /opt/gon/gon/bin
COPY --from=build /opt/gon/pkg/tool /opt/gon/pkg/tool
COPY --from=build /opt/gon/pkg/include /opt/gon/pkg/include
COPY --from=build /opt/gon/src /opt/gon/src
COPY --from=build /opt/gon/lib /opt/gon/lib
COPY --from=build /opt/gon/api /opt/gon/api
COPY --from=build /opt/gon/LICENSE /opt/gon/PATENTS /opt/gon/VERSION /opt/gon/go.env /opt/gon/
COPY --from=build /opt/gon/tools/x-tools/LICENSE /usr/share/doc/gon/x-tools-LICENSE
COPY --from=build /opt/gon/tools/staticcheck/LICENSE /usr/share/doc/gon/staticcheck-LICENSE

ARG GON_REVISION=local
LABEL org.opencontainers.image.title="Gon" \
      org.opencontainers.image.description="Gon development compiler and gonpls, with cgo support" \
      org.opencontainers.image.source="https://github.com/tomaszbk/gon" \
      org.opencontainers.image.licenses="BSD-3-Clause" \
      org.opencontainers.image.revision="$GON_REVISION"

# Only public launchers are exposed. They select private tools for their own
# child processes, without globally shadowing a separately installed Go.
ENV PATH="/opt/gon/gon/bin:${PATH}" GOPATH=/go GOTOOLCHAIN=local
WORKDIR /workspace
CMD ["gon", "version"]
