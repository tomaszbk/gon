#!/usr/bin/env python3
"""Expose only gon and gonpls; never overwrite an existing command.

With --project-skill, copy the gon agent skill into a repository that adopts
Gon instead. The skill is project-scoped and never installed globally.
"""
import argparse
import filecmp
import os
from pathlib import Path
import shlex
import shutil

ROOT = Path(__file__).resolve().parents[2]
SKILL = ROOT / ".agents" / "skills" / "gon"


def configure_path(destination):
    """Expose public commands in future shells without exposing private go."""
    if str(destination) in os.environ.get("PATH", "").split(os.pathsep):
        return
    if os.name == "nt":
        import winreg
        with winreg.CreateKey(winreg.HKEY_CURRENT_USER, "Environment") as key:
            try:
                current, kind = winreg.QueryValueEx(key, "Path")
            except FileNotFoundError:
                current, kind = "", winreg.REG_EXPAND_SZ
            entries = current.split(os.pathsep)
            if str(destination).casefold() not in [os.path.expandvars(p).casefold() for p in entries]:
                winreg.SetValueEx(key, "Path", 0, kind, str(destination) + (os.pathsep + current if current else ""))
        # Tell Explorer and other applications to refresh their environment.
        import ctypes
        from ctypes import wintypes
        broadcast = ctypes.windll.user32.SendMessageTimeoutW
        broadcast.argtypes = [wintypes.HWND, wintypes.UINT, wintypes.WPARAM,
                              wintypes.LPCWSTR, wintypes.UINT, wintypes.UINT,
                              ctypes.POINTER(ctypes.c_size_t)]
        broadcast.restype = wintypes.LPARAM
        result = ctypes.c_size_t()
        broadcast(
            0xFFFF, 0x001A, 0, "Environment", 0x0002, 5000, ctypes.byref(result))
        print("Added public commands to your user PATH. Open a new terminal and restart your editor.")
        return
    shell = Path(os.environ.get("SHELL", "/bin/sh")).name
    home = Path.home()
    if shell == "zsh":
        profiles = [Path(os.environ.get("ZDOTDIR") or home) / ".zshrc"]
    elif shell == "bash":
        login = next((home / name for name in (".bash_profile", ".bash_login", ".profile")
                      if (home / name).exists()), home / ".bash_profile")
        profiles = [home / ".bashrc", login]
    elif shell == "fish":
        base = Path(os.environ.get("XDG_CONFIG_HOME") or home / ".config")
        profiles = [base / "fish" / "conf.d" / "gon.fish"]
    elif shell in ("sh", "dash", "ksh"):
        profiles = [home / ".profile"]
    else:
        raise SystemExit(f"Unsupported shell {shell!r}; use --no-modify-path and add {destination} to PATH.")
    # Quote literal paths, including spaces and shell metacharacters. No user
    # content is executed while installing, and repeated installs append once.
    if shell == "fish":
        quoted = "'" + str(destination).replace("\\", "\\\\").replace("'", "\\'") + "'"
        body = f"if not contains -- {quoted} $PATH\n    set -gx PATH {quoted} $PATH\nend\n"
    else:
        body = (f'case ":${{PATH-}}:" in\n'
                f"    *{shlex.quote(':' + str(destination) + ':')}*) ;;\n"
                f'    *) export PATH={shlex.quote(str(destination))}:"${{PATH-}}" ;;\n'
                'esac\n')
    block = "\n# Gon public commands (private Go tools are not added to PATH).\n" + body
    for profile in profiles:
        original = profile.read_text() if profile.exists() else ""
        if block in original:
            continue
        profile.parent.mkdir(parents=True, exist_ok=True)
        with profile.open("a") as stream:
            stream.write(block)
        print(f"Added public commands to PATH in {profile}")
    print("Open a new terminal and restart your editor to load the updated PATH.")


def same_tree(a, b):
    compare = filecmp.dircmp(a, b)
    if compare.left_only or compare.right_only or compare.funny_files:
        return False
    _, mismatch, errors = filecmp.cmpfiles(a, b, compare.common_files, shallow=False)
    return not mismatch and not errors and all(
        same_tree(Path(a) / d, Path(b) / d) for d in compare.common_dirs)


def install_skill(repo):
    repo = repo.expanduser().absolute()
    if not repo.is_dir():
        raise SystemExit(f"{repo} is not a directory")
    target = repo / ".agents" / "skills" / "gon"
    if target.exists() or target.is_symlink():
        if target.is_dir() and not target.is_symlink() and same_tree(SKILL, target):
            print(f"{target} is up to date")
            return
        raise SystemExit(f"Refusing to overwrite {target}; remove it to install this toolchain's skill")
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copytree(SKILL, target)
    print(f"Copied the gon skill to {target}; commit it to opt this repository into Gon")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, default=Path.home() / ".local" / "bin")
    parser.add_argument("--no-modify-path", action="store_true",
                        help="install commands without updating your shell/user PATH")
    parser.add_argument("--project-skill", type=Path, metavar="REPO",
                        help="copy the agent skill into REPO/.agents/skills/gon and exit")
    args = parser.parse_args()
    if args.project_skill:
        install_skill(args.project_skill)
        return
    destination = args.bin_dir.expanduser().absolute()
    suffix = ".exe" if os.name == "nt" else ""
    links = []
    for name in ("gon", "gonpls"):
        source = ROOT / "gon" / "bin" / (name + suffix)
        target = destination / (name + suffix)
        if not source.is_file():
            raise SystemExit("Build first: python3 misc/gon/build.py")
        if target.exists() or target.is_symlink():
            if target.is_symlink() and target.resolve() == source.resolve():
                continue
            # Repair an earlier Gon installation after its checkout was moved.
            # Other dangling links and existing commands remain protected.
            if (target.is_symlink() and not target.exists() and
                    Path(os.readlink(target)).parts[-3:] == ("gon", "bin", name + suffix)):
                links.append((source, target))
                continue
            raise SystemExit(f"Refusing to overwrite {target}; choose another --bin-dir")
        links.append((source, target))
    destination.mkdir(parents=True, exist_ok=True)
    for source, target in links:
        if target.is_symlink():
            target.unlink()
        target.symlink_to(source)
        print(f"Installed {target} -> {source}")
    print(f"Public commands are in {destination}; keep this toolchain directory in place.")
    if args.no_modify_path and str(destination) not in os.environ.get("PATH", "").split(os.pathsep):
        print(f"Add {destination} to PATH. Do not add the toolchain's private bin directory.")
    elif not args.no_modify_path:
        configure_path(destination)


if __name__ == "__main__":
    main()
