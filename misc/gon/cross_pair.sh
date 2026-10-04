#!/usr/bin/env bash
# Run portable legacy/modern pairs for all nine Gon features on GOOS/GOARCH.
# The unmodified baseline executes each legacy program on the same target;
# the public Gon launcher executes legacy, modern and non-inlined modern.
# Other architectures run through binfmt (for example QEMU); wasm uses each
# toolchain's own lib/wasm exec wrapper. No target toolchain is required.
#
# usage: GON_BASELINE_GO=/absolute/path/to/go misc/gon/cross_pair.sh GOOS GOARCH
set -euo pipefail

if [ "$#" -ne 2 ]; then
	echo "usage: GON_BASELINE_GO=/absolute/path/to/go $0 GOOS GOARCH" >&2
	exit 2
fi
goos=$1
goarch=$2
root=$(cd "$(dirname "$0")/../.." && pwd)
gon="$root/gon/bin/gon"
baseline=${GON_BASELINE_GO:-}
if [[ "$baseline" != /* ]] || [ ! -x "$baseline" ]; then
	echo "GON_BASELINE_GO must name an executable unmodified Go 1.27+ toolchain (absolute path)" >&2
	exit 2
fi
if [ ! -x "$gon" ]; then
	echo "public Gon tools are missing; run python3 misc/gon/build.py first" >&2
	exit 2
fi

# Keep toolchain selection local to these commands, including the baseline.
unset GOROOT GOTOOLDIR GOOS GOARCH GOFLAGS
export GOTOOLCHAIN=local GOENV=off GOWORK=off CGO_ENABLED=0 GON_ROOT="$root"
baseline_root=$("$baseline" env GOROOT)
baseline_version=$("$baseline" version)
if "$baseline" capabilities --json >/dev/null 2>&1 ||
	[ "$baseline_root" = "$root" ] || [[ "$baseline_version" == *devel* ]] ||
	[[ ! "$baseline_version" =~ go1\.([0-9]+)(\.[0-9]+)?[[:space:]] ]] ||
	[ "${BASH_REMATCH[1]:-0}" -lt 27 ]; then
	echo "baseline must be unmodified released Go 1.27+: $baseline_version" >&2
	exit 2
fi
echo "baseline: $baseline_version; target: $goos/$goarch"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/gon-cross-pairs.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

run() {
	local tool=$1 wrapper_root=$2 output=$3
	shift 3
	if ! GOOS=$goos GOARCH=$goarch GOFLAGS= PATH="$wrapper_root/lib/wasm:$PATH" \
		"$tool" run "$@" >"$output" 2>&1; then
		cat "$output" >&2
		echo "failed: $tool run on $goos/$goarch" >&2
		exit 1
	fi
}

pair() {
	local feature=$1 legacy=$2 modern=$3
	shift 3
	local mode
	run "$baseline" "$baseline_root" "$tmp/want" "$@" "$legacy"
	for mode in legacy modern no-inline; do
		case "$mode" in
			legacy) run "$gon" "$root" "$tmp/got" "$@" "$legacy" ;;
			modern) run "$gon" "$root" "$tmp/got" "$@" "$modern" ;;
			no-inline) run "$gon" "$root" "$tmp/got" -gcflags=-l "$@" "$modern" ;;
		esac
		if ! cmp -s "$tmp/want" "$tmp/got"; then
			echo "$feature $mode on $goos/$goarch differs from the baseline legacy program:" >&2
			diff -u "$tmp/want" "$tmp/got" >&2 || true
			exit 1
		fi
		echo "ok: $feature $mode on $goos/$goarch"
	done
}

for feature in errorhandling conditional lambda nullsafety; do
	dir="$root/test/$feature.dir"
	pair "$feature" "$dir/legacy.go" "$dir/modern.go" "$dir/common.go"
done
fixtures="$root/misc/gon/analysisfixtures"
pair "enums/matching/Option/Result" "$fixtures/alternatives_legacy.go" "$fixtures/alternatives_modern.go"
pair namedarguments "$fixtures/namedarguments_legacy.go" "$fixtures/namedarguments_modern.go"
dir="$root/test/optionsyntax.dir"
pair optional-syntax "$dir/legacy.go" "$dir/modern.go" "$dir/common.go"
pair optional-boundaries "$dir/boundaries_legacy.go" "$dir/boundaries_modern.go"
