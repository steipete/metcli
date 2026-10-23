# metcli
cli to get your data out of Meta. very WIP

Download the [0.1.1 release](https://github.com/steipete/metcli/releases/tag/v0.1.1)
for macOS, Linux, or Windows (Intel/AMD 64-bit or ARM64). Each archive contains
`metcli`, `ig-cookies`, and `ig-profile`; verify it against the release's
`checksums.txt` before extracting the binaries into a directory on your PATH.
Run `metcli --version` to check the installed version.

macOS binaries require macOS 12 or newer. Starting with 0.1.1, release binaries
are Developer ID signed by Peter Steinberger and notarized by Apple, so direct
downloads pass Gatekeeper. You can also install from source:

```sh
go install github.com/steipete/metcli/cmd/metcli@v0.1.1
go install github.com/steipete/metcli/cmd/ig-cookies@v0.1.1
go install github.com/steipete/metcli/cmd/ig-profile@v0.1.1
```

The source language minimum remains Go 1.25.0; `go.mod` selects patched Go
1.27.3. CI tests Go 1.26.10 and 1.27.3. Release builds and Xcode 27 release checks
use patched Go 1.26.10 to retain macOS 12 support and smoke-test Intel binaries
under Rosetta.

```sh
make test build
go build -o bin/ ./cmd/...
python3 scripts/smoke.py bin
```

The smoke check requires Python 3 and exercises the built CLIs with a temporary,
synthetic Chrome cookie database. It does not use your browser profile or Keychain.

See [RELEASING.md](RELEASING.md) for the release procedure.
