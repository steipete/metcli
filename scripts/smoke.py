"""Exercise built CLIs using only synthetic browser data."""

import json
import os
from pathlib import Path
import sqlite3
import subprocess
import sys
import tempfile


def main():
    binaries = Path(sys.argv[1]).resolve()
    with tempfile.TemporaryDirectory(prefix="metcli-smoke-") as temporary:
        fixture = Path(temporary)
        database = fixture / "Cookies"
        with sqlite3.connect(database) as connection:
            connection.execute("CREATE TABLE meta(key TEXT PRIMARY KEY, value TEXT)")
            connection.execute("INSERT INTO meta VALUES('version', '30')")
            connection.execute(
                "CREATE TABLE cookies(host_key TEXT, name TEXT, path TEXT, "
                "value TEXT, encrypted_value BLOB, expires_utc INTEGER, "
                "is_secure INTEGER, is_httponly INTEGER, samesite INTEGER)"
            )
            connection.executemany(
                "INSERT INTO cookies VALUES(?, ?, '/', ?, X'', 0, 1, 1, 1)",
                [
                    (".instagram.com", "sessionid", "synthetic-session"),
                    (".instagram.com", "csrftoken", "synthetic-csrf"),
                    (".example.com", "sessionid", "excluded-domain"),
                ],
            )

        # The supported override prevents OS credential-store access.
        environment = dict(os.environ, GOOKIE_CHROME_SAFE_STORAGE_PASSWORD="synthetic-only")

        def run(name, *arguments):
            result = subprocess.run(
                [str(binaries / name), *arguments],
                env=environment,
                text=True,
                capture_output=True,
                timeout=15,
                check=True,
            )
            if result.stderr:
                raise RuntimeError(f"{name} emitted unexpected diagnostics: {result.stderr}")
            return result.stdout

        for name in ("metcli", "ig-profile", "ig-cookies"):
            if "Usage:" not in run(name, "--help"):
                raise RuntimeError(f"{name} help is missing usage")

        cookies = json.loads(run("ig-cookies", "--profile", str(database), "--json"))
        values = {cookie["name"]: cookie["value"] for cookie in cookies}
        if len(cookies) != 2 or values != {
            "sessionid": "synthetic-session",
            "csrftoken": "synthetic-csrf",
        }:
            raise RuntimeError("synthetic cookie JSON export differs from the fixture")
        header = run(
            "ig-cookies", "--profile", str(database), "--names", "csrftoken", "--header"
        )
        if header != "Cookie: csrftoken=synthetic-csrf\n":
            raise RuntimeError("cookie name filtering or header export failed")

    print("CLI smoke passed: help, SQLite cookie JSON/header export, domain/name filtering")


if __name__ == "__main__":
    main()
