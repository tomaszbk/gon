#!/usr/bin/env python3
"""Regression checks for validation evidence on a stable source snapshot."""

import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location("gon_validate", Path(__file__).with_name("validate.py"))
validate = importlib.util.module_from_spec(spec)
with mock.patch.object(sys, "dont_write_bytecode", True):
    spec.loader.exec_module(validate)


class ValidationSnapshotTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="gon-validation-snapshot-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.source = self.root / "src/value.go"
        self.source.parent.mkdir()
        self.source.write_text("package fixture\nvar Value = 1\n")
        (self.root / ".gitignore").write_text("pkg/\nignored/\nfake-baseline\n")
        (self.root / "README.md").write_text("initial documentation\n")
        inventory = self.root / "misc/gon/features.json"
        inventory.parent.mkdir(parents=True)
        inventory.write_text(json.dumps({"tooling": {"pending": []}}))

        # Every Git operation and file mutation is confined to this fixture.
        self.git_env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
        self.git("init", "-q")
        self.git("add", ".gitignore", "README.md", "src/value.go", "misc/gon/features.json")
        self.git("-c", "user.name=Gon validation tests",
                 "-c", "user.email=gon-tests@example.invalid",
                 "-c", "commit.gpgsign=false",
                 "-c", "core.hooksPath=" + str(self.root / ".git/disabled-hooks"),
                 "commit", "-qm", "validation fixture")

        self.addCleanup(mock.patch.stopall)
        mock.patch.object(validate, "ROOT", self.root).start()
        mock.patch.object(validate, "_source_hashes", {}).start()
        mock.patch.dict(os.environ, self.git_env, clear=True).start()

    def git(self, *args):
        subprocess.run(["git", *args], cwd=self.root, env=self.git_env,
                       check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def test_snapshot_tracks_edits_additions_deletions_and_ignores_evidence(self):
        original = validate.source_snapshot()
        self.assertIn("src/value.go", original["files"])
        self.assertNotIn("README.md", original["files"])
        self.assertEqual(original, validate.source_snapshot())

        (self.root / "README.md").write_text("updated documentation\n")
        for relative in ("design/local.md", "ignored/output", "pkg/gon-validation/tooling/log.json",
                         "misc/gon/validation/evidence.json", "misc/gon/benchmarks/results/run/report.json"):
            path = self.root / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("retained evidence\n")
        self.assertEqual(original, validate.source_snapshot())

        # Equal length and restored mtime must not reuse an obsolete cached hash.
        stamp = self.source.stat()
        self.source.write_text("package fixture\nvar Value = 2\n")
        os.utime(self.source, ns=(stamp.st_atime_ns, stamp.st_mtime_ns))
        edited = validate.source_snapshot()
        self.assertNotEqual(original["sha256"], edited["sha256"])
        self.assertNotEqual(original["files"]["src/value.go"], edited["files"]["src/value.go"])
        self.assertEqual(edited, validate.source_snapshot())

        new_source = self.root / "src/new.go"
        new_source.write_text("package fixture\n")
        added = validate.source_snapshot()
        self.assertIn("src/new.go", added["files"])
        self.assertNotEqual(edited["sha256"], added["sha256"])
        new_source.unlink()
        self.assertEqual(edited, validate.source_snapshot())

        self.source.unlink()
        deleted = validate.source_snapshot()
        self.assertIn("src/value.go", deleted["files"])
        self.assertNotEqual(edited["sha256"], deleted["sha256"])
        self.assertEqual(deleted, validate.source_snapshot())

    def run_profile(self, plan):
        baseline = self.root / "fake-baseline"
        baseline.write_text("#!" + sys.executable + "\nprint('go version go1.27.1 fixture/host')\n")
        baseline.chmod(0o755)
        with mock.patch.object(validate, "checks", return_value=plan), \
             mock.patch.object(validate, "tool_snapshot", return_value={"fixture-tool": "unchanged"}), \
             mock.patch.dict(os.environ, {"GON_BASELINE_GO": str(baseline)}), \
             mock.patch.object(sys, "argv", ["validate.py", "tooling"]), \
             contextlib.redirect_stdout(io.StringIO()):
            status = validate.main()
        directory = self.root / "pkg/gon-validation/tooling"
        summary = json.loads((directory / "summary.json").read_text())
        return status, directory, summary

    def test_source_change_fails_profile_and_does_not_run_remaining_checks(self):
        mutation = [sys.executable, "-c",
                    "from pathlib import Path; Path('src/value.go').write_text('changed source\\n')"]
        following = [sys.executable, "-c",
                     "from pathlib import Path; Path('unexpected-following-check').touch()"]
        status, directory, summary = self.run_profile([
            ("mutating-check", ".", mutation),
            ("following-check", ".", following),
        ])
        self.assertEqual(status, 1)
        self.assertFalse(summary["sourceStable"])
        self.assertNotEqual(summary["sourceSHA256"], summary["finalSourceSHA256"])
        first, second = summary["results"]
        self.assertEqual(first["exitCode"], 0)
        self.assertEqual(first["status"], "fail")
        self.assertIn("changed", first["reason"])
        self.assertEqual(second["status"], "not-run")
        self.assertFalse((self.root / "unexpected-following-check").exists())
        start = json.loads((directory / "source-start.json").read_text())
        end = json.loads((directory / "source-end.json").read_text())
        self.assertEqual(start["sha256"], summary["sourceSHA256"])
        self.assertEqual(end["sha256"], summary["finalSourceSHA256"])

    def test_stable_profile_can_pass_while_recording_ignored_outputs(self):
        output = [sys.executable, "-c",
                  "from pathlib import Path; p=Path('pkg/generated'); p.parent.mkdir(exist_ok=True); p.write_text('output')"]
        status, _, summary = self.run_profile([("stable-check", ".", output)])
        self.assertEqual(status, 0)
        self.assertTrue(summary["sourceStable"])
        self.assertEqual(summary["sourceSHA256"], summary["finalSourceSHA256"])
        self.assertEqual(summary["results"][0]["status"], "pass")
        self.assertEqual(summary["baseline"]["version"], "go version go1.27.1 fixture/host")

    def test_compile_only_does_not_require_events_but_executed_tests_do(self):
        tool = self.root / "pkg/fake-go"
        tool.parent.mkdir()
        tool.write_text("#!" + sys.executable + "\n"
                        "import json, sys\n"
                        "from pathlib import Path\n"
                        "if '-c' in sys.argv:\n"
                        "    Path('pkg/compiled.test').touch()\n"
                        "elif 'FixturePass' in sys.argv:\n"
                        "    print(json.dumps({'Action': 'run', 'Test': 'FixturePass'}))\n")
        tool.chmod(0o755)
        status, _, summary = self.run_profile([
            ("compile-only", ".", [str(tool), "test", "-c", "fixture"]),
            ("executed-tests", ".", [str(tool), "test", "-json", "-run", "FixturePass"]),
        ])
        self.assertEqual(status, 0)
        self.assertTrue((self.root / "pkg/compiled.test").exists())
        self.assertNotIn("testsRun", summary["results"][0])
        self.assertEqual(summary["results"][1]["testsRun"], 1)

        status, _, summary = self.run_profile([
            ("no-matching-tests", ".", [str(tool), "test", "-json", "-run", "MissingTest"]),
        ])
        self.assertEqual(status, 1)
        self.assertEqual(summary["results"][0]["reason"], "no matching tests ran")


if __name__ == "__main__":
    unittest.main()
