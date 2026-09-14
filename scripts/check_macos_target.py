"""Require every Darwin release binary to match the documented macOS 12 minimum."""

import re
import subprocess
import sys


def check_target(load_commands):
    targets = []
    for block in re.split(r"(?m)^Load command \d+\s*$", load_commands)[1:]:
        command = re.search(r"(?m)^\s*cmd (\S+)\s*$", block)
        if command is None:
            continue
        if command[1] == "LC_BUILD_VERSION":
            platform = re.search(r"(?m)^\s*platform (\S+)\s*$", block)
            if platform is None or platform[1] not in ("1", "MACOS"):
                raise ValueError("expected a macOS platform")
            field = "minos"
        elif command[1] == "LC_VERSION_MIN_MACOSX":
            field = "version"
        else:
            continue
        match = re.search(rf"(?m)^\s*{field} (\d+\.\d+(?:\.\d+)?)\s*$", block)
        if match is None:
            raise ValueError("missing macOS deployment target")
        version = tuple(map(int, match[1].split(".")))
        version += (0,) * (3 - len(version))
        if version != (12, 0, 0):
            raise ValueError(f"macOS {match[1]} differs from documented minimum 12.0")
        targets.append(match[1])
    if not targets:
        raise ValueError("no macOS deployment target found")
    return targets


def main():
    if len(sys.argv) != 3:
        raise SystemExit("usage: check_macos_target.py OS BINARY")
    if sys.argv[1] != "darwin":
        return
    try:
        result = subprocess.run(
            ["otool", "-arch", "all", "-l", sys.argv[2]],
            check=True, capture_output=True, text=True,
        )
        targets = check_target(result.stdout)
    except (OSError, subprocess.CalledProcessError, ValueError) as error:
        raise SystemExit(f"macOS release gate failed: {error}") from error
    print(f"{sys.argv[2]}: macOS minos {', '.join(targets)} == 12.0")


if __name__ == "__main__":
    main()
