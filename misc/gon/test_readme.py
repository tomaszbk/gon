#!/usr/bin/env python3
"""Check README and example-guide snippets against executed source fixtures."""

import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
FIXTURES = ROOT / "misc/gon/readme-examples"
DOCUMENTS = (ROOT / "README.md", ROOT / "doc/gon-examples.md")
EXPECTED = "PASS: errors, nil, Option, enums, matching, Result, lambdas, conditionals, named arguments\n"


def snippets(source):
    return dict(re.findall(r"// BEGIN README ([\w-]+)\n(.*?)\n// END README \1", source, re.S))


def normalize(source):
    return source.strip().replace("\r\n", "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", default=os.environ.get("GON_BASELINE_GO"))
    parser.add_argument("--gon", default=str(ROOT / "gon/bin" / ("gon.exe" if os.name == "nt" else "gon")))
    args = parser.parse_args()
    if not args.go or not Path(args.go).is_absolute():
        parser.error("set GON_BASELINE_GO or --go to an absolute unmodified Go executable")
    env = {k: v for k, v in os.environ.items() if not k.startswith(("GO", "GON", "CGO"))}
    env.update(GOENV="off", GOWORK="off", GOTOOLCHAIN="local", GOFLAGS="")
    capabilities = subprocess.run([args.go, "capabilities", "--json"], env=env, text=True,
                                  stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if capabilities.returncode == 0:
        raise SystemExit("the README legacy baseline must not be another Gon launcher")
    baseline = subprocess.check_output([args.go, "env", "-json", "GOVERSION", "GOROOT"], env=env, text=True)
    baseline = json.loads(baseline)
    version = re.fullmatch(r"go1\.(\d+)(?:\.\d+)?", baseline["GOVERSION"])
    if not version or int(version[1]) < 27:
        raise SystemExit("the README legacy baseline must be unmodified Go 1.27+")
    gonroot = subprocess.check_output([args.gon, "env", "GOROOT"], env=env, text=True).strip()
    if Path(gonroot).resolve() == Path(baseline["GOROOT"]).resolve():
        raise SystemExit("baseline and Gon must be separate toolchains")

    sources = {name: (FIXTURES / (name + ".go")).read_text() for name in ("legacy", "modern")}
    examples = {name: snippets(source) for name, source in sources.items()}
    seen = set()
    block_count = 0
    for path in DOCUMENTS:
        document = path.read_text()
        blocks = re.findall(r"<!-- readme-example: ([\w-]+) (legacy|modern) -->\s*```go\n(.*?)\n```", document, re.S)
        if len(re.findall(r"^```go$", document, re.M)) != len(blocks):
            raise SystemExit(f"{path}: each fenced Go example must identify its executed source fixture")
        local_seen = set()
        for name, variant, code in blocks:
            key = (name, variant)
            if key in local_seen or name not in examples[variant]:
                raise SystemExit(f"{path}: duplicate or unknown example: {key!r}")
            if normalize(code) != normalize(examples[variant][name]):
                raise SystemExit(f"{path}: example differs from its executed fixture: {key!r}")
            local_seen.add(key)
            seen.add(key)
        block_count += len(blocks)
    required = {(name, variant) for variant in examples for name in examples[variant]}
    if seen != required:
        raise SystemExit("README examples missing: " + repr(sorted(required - seen)))

    with tempfile.TemporaryDirectory(prefix="gon-readme-") as temporary:
        for variant, command in (("legacy", args.go), ("legacy", args.gon), ("modern", args.gon)):
            folder = Path(temporary) / (variant + ("-go" if command == args.go else "-gon"))
            folder.mkdir()
            (folder / "go.mod").write_text("module example.com/readme\n\ngo 1.27\n")
            shutil.copyfile(FIXTURES / "common.go", folder / "common.go")
            (folder / "impl.go").write_text(sources[variant])
            result = subprocess.run([command, "run", "."], cwd=folder, env=env, text=True,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            if result.returncode or result.stdout != EXPECTED:
                raise SystemExit(f"{variant} with {command}:\n{result.stdout}{result.stderr}")
            print(f"PASS: README {variant} with {command}")
    print(f"PASS: {block_count} documented blocks ({len(seen)} unique snippets) match executed fixtures; baseline {baseline['GOVERSION']}")


if __name__ == "__main__":
    main()
