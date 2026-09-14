"""Print the finalized changelog section and download links for a release tag."""

from pathlib import Path
import re
import sys


def main():
    tag = sys.argv[1]
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", tag):
        raise SystemExit("expected a vMAJOR.MINOR.PATCH tag")
    version = tag[1:]
    source = Path("internal/version/version.go").read_text()
    if f'const Version = "{version}"' not in source:
        raise SystemExit("tag does not match internal/version/version.go")
    changelog = Path("CHANGELOG.md").read_text()
    section = re.search(
        rf"^## {re.escape(version)} - \d{{4}}-\d{{2}}-\d{{2}}\n(.*?)(?=^## |\Z)",
        changelog,
        re.MULTILINE | re.DOTALL,
    )
    if section is None or not section[1].strip():
        raise SystemExit("missing finalized changelog section")
    # The shared workflow publishes the exact dated section, including its heading.
    sys.stdout.write(section[0])


if __name__ == "__main__":
    main()
