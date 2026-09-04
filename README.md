# metcli
cli to get your data out of Meta. very WIP

Build from source with Go 1.25.0 or newer. The preferred toolchain is Go
1.25.14, selected by `go.mod`; CI checks both versions.

```sh
make test build
go build -o bin/ ./cmd/...
python3 scripts/smoke.py bin
```

The smoke check requires Python 3 and exercises the built CLIs with a temporary,
synthetic Chrome cookie database. It does not use your browser profile or Keychain.
