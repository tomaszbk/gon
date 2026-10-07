#!/usr/bin/env python3
"""Execute existing feature pairs using the installed Linux container toolchain."""

import argparse
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[2]
EXPECTED = "PASS: errors, nil, optionals, enums, matching, lambdas, conditionals, named arguments\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("image", help="local or published Gon container image")
    args = parser.parse_args()
    result = subprocess.run([
        "docker", "run", "--rm",
        "--mount", f"type=bind,src={ROOT / 'misc/gon/readme-examples'},dst=/fixtures,readonly",
        args.image, "sh", "-ec", """
            gon version >&2
            gonpls version >&2
            gon capabilities --json >&2
            test "$(gon env GOPROXY)" = https://proxy.golang.org,direct
            test "$(gon env GOSUMDB)" = sum.golang.org
            test "$(gon env GOTOOLCHAIN)" = local
            case ":$PATH:" in
                *:/opt/gon/bin:*) echo 'private toolchain exposed on PATH' >&2; exit 1 ;;
            esac
            mkdir /tmp/app
            cd /tmp/app
            printf 'module example.com/container\n\ngo 1.27\n' > go.mod
            cp /fixtures/common.go common.go
            for variant in legacy modern; do
                cp "/fixtures/$variant.go" impl.go
                CGO_ENABLED=0 gon build -o /tmp/program .
                /tmp/program
            done
            gon check . --json >&2
            # Ensure the installed cgo compiler and headers also work.
            printf 'package main\nimport "C"\nfunc main() {}\n' > impl.go
            rm common.go
            CGO_ENABLED=1 gon build -o /tmp/cgo-program .
            /tmp/cgo-program
        """,
    ], text=True, stdout=subprocess.PIPE)
    if result.returncode or result.stdout != EXPECTED * 2:
        raise SystemExit(f"container smoke test failed ({result.returncode}):\n{result.stdout}")
    print("PASS: installed gon/gonpls, semantic checking, static executable feature pairs and cgo")


if __name__ == "__main__":
    main()
