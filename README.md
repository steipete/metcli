# metcli
cli to get your data out of Meta. very WIP

Download the [0.1.0 release](https://github.com/steipete/metcli/releases/tag/v0.1.0)
for macOS, Linux, or Windows (Intel/AMD 64-bit or ARM64). Each archive contains
`metcli`, `ig-cookies`, and `ig-profile`; verify it against the release's
`checksums.txt` before extracting the binaries into a directory on your PATH.
Run `metcli --version` to check the installed version.

macOS binaries require macOS 12 or newer. These GoReleaser cross-builds are not
Developer ID signed or notarized; Gatekeeper may block downloaded binaries.
You can also install from source:

```sh
go install github.com/steipete/metcli/cmd/metcli@v0.1.0
go install github.com/steipete/metcli/cmd/ig-cookies@v0.1.0
go install github.com/steipete/metcli/cmd/ig-profile@v0.1.0
```

Build from source with Go 1.25.0 or newer. The preferred toolchain is Go
1.25.14, selected by `go.mod`; CI checks both versions.

```sh
make test build
go build -o bin/ ./cmd/...
python3 scripts/smoke.py bin
```

The smoke check requires Python 3 and exercises the built CLIs with a temporary,
synthetic Chrome cookie database. It does not use your browser profile or Keychain.

See [RELEASING.md](RELEASING.md) for the release procedure.
