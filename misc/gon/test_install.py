#!/usr/bin/env python3
"""Exercise public installation and persistent PATH in isolated user homes."""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
INSTALL = ROOT / "misc/gon/install.py"


@unittest.skipIf(os.name == "nt", "POSIX shell integration; Windows needs native validation")
class InstallTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="gon-install-")
        self.addCleanup(self.temp.cleanup)
        self.home = Path(self.temp.name).resolve()
        # Do not inherit terminal-session IDs: macOS zsh may otherwise write
        # history asynchronously into a test home after the shell has exited.
        self.env = dict(HOME=str(self.home), SHELL="/bin/zsh", PATH="/usr/bin:/bin", TERM="dumb")

    def install(self, *args, success=True):
        result = subprocess.run([sys.executable, str(INSTALL), *map(str, args)],
                                env=self.env, text=True, capture_output=True)
        self.assertEqual(result.returncode == 0, success, result.stdout + result.stderr)
        return result

    def test_fresh_zsh_and_idempotent_install(self):
        if not shutil.which("zsh"):
            self.skipTest("zsh unavailable")
        profile = self.home / ".zshrc"
        original = "# existing user settings\nexport GON_INSTALL_TEST=preserved\n"
        profile.write_text(original)
        self.install()
        after = profile.read_text()
        self.assertTrue(after.startswith(original))
        self.install()
        self.assertEqual(profile.read_text(), after)
        public = self.home / ".local/bin"
        self.assertEqual(sorted(p.name for p in public.iterdir()), ["gon", "gonpls"])
        result = subprocess.check_output([
            "zsh", "-ic", 'source "$HOME/.zshrc"; command -v gon; command -v gonpls; '
            'gon env GOROOT; print -r -- "$GON_INSTALL_TEST"; print -r -- "$PATH"'],
            env=self.env, text=True)
        self.assertEqual(result.splitlines()[:4], [str(public / "gon"), str(public / "gonpls"), str(ROOT), "preserved"])
        self.assertEqual(result.splitlines()[4].split(os.pathsep).count(str(public)), 1)
        self.assertNotIn(str(ROOT / "bin"), result.splitlines()[4].split(os.pathsep))

    def test_quoted_custom_path_and_go_coexistence(self):
        public = self.home / "public ' tools $(touch SHOULD_NOT_EXIST) &"
        existing = self.home / "existing"
        existing.mkdir()
        (existing / "go").write_text("#!/bin/sh\nprintf 'unchanged-go\\n'\n")
        (existing / "go").chmod(0o755)
        self.env.update(SHELL="/bin/bash", PATH=str(existing) + ":/usr/bin:/bin")
        self.install("--bin-dir", public)
        for name in (".bashrc", ".bash_profile"):
            profile = self.home / name
            command = 'source "$1"; command -v gon; go; gon env GOROOT'
            result = subprocess.check_output(["bash", "-c", command, "test", str(profile)],
                                             cwd=self.home, env=self.env, text=True)
            self.assertEqual(result.splitlines(), [str(public / "gon"), "unchanged-go", str(ROOT)])
        self.assertFalse((self.home / "SHOULD_NOT_EXIST").exists())

    def test_existing_path_and_opt_out(self):
        public = self.home / ".local/bin"
        self.env["PATH"] = str(public) + ":/usr/bin:/bin"
        self.install()
        self.assertFalse((self.home / ".zshrc").exists())
        self.env["PATH"] = "/usr/bin:/bin"
        self.install("--no-modify-path")
        self.assertFalse((self.home / ".zshrc").exists())

    def test_collision_is_non_destructive(self):
        public = self.home / ".local/bin"
        public.mkdir(parents=True)
        occupied = public / "gonpls"
        occupied.write_text("keep existing tool")
        self.install(success=False)
        self.assertEqual(occupied.read_text(), "keep existing tool")
        self.assertFalse((public / "gon").exists())
        self.assertFalse((self.home / ".zshrc").exists())

    def test_shell_locations(self):
        for shell, variable, profile in (
            ("zsh", "ZDOTDIR", "custom/.zshrc"),
            ("fish", "XDG_CONFIG_HOME", "custom/fish/conf.d/gon.fish"),
            ("sh", None, ".profile"),
        ):
            with self.subTest(shell=shell):
                self.env["SHELL"] = "/bin/" + shell
                if variable:
                    self.env[variable] = str(self.home / "custom")
                self.install()
                self.assertTrue((self.home / profile).exists())
                if shell == "sh":
                    result = subprocess.check_output(
                        ["sh", "-c", '. "$HOME/.profile"; command -v gon'], env=self.env, text=True)
                    self.assertEqual(result.strip(), str(self.home / ".local/bin/gon"))

    def test_repair_moved_gon_and_protect_unrelated_links(self):
        public = self.home / ".local/bin"
        public.mkdir(parents=True)
        target = public / "gon"
        target.symlink_to(self.home / "old-checkout/gon/bin/gon")
        self.install()
        self.assertEqual(target.resolve(), ROOT / "gon/bin/gon")
        target.unlink()
        target.symlink_to(self.home / "unrelated/gon")
        self.install(success=False)
        self.assertEqual(os.readlink(target), str(self.home / "unrelated/gon"))

    def test_bash_preserves_existing_login_profile(self):
        self.env["SHELL"] = "/bin/bash"
        profile = self.home / ".profile"
        profile.write_text("# login configuration\n")
        self.install()
        self.assertTrue(profile.read_text().startswith("# login configuration\n"))
        self.assertFalse((self.home / ".bash_profile").exists())


if __name__ == "__main__":
    unittest.main()
