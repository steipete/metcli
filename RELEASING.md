# Releasing metcli

The tag-triggered GoReleaser workflow is adapted from `steipete/ordercli`.
It publishes six platform archives, each containing all three commands, and
`checksums.txt` to GitHub Releases. The Go module proxy is the source-install
channel. There is no Homebrew, npm, container, or Sparkle publication step.

## Prepare

Start with clean, synchronized `main` and create a release branch. Set
`internal/version/version.go`, update the README install examples, and finalize
the current changelog section with the local date. Keep every entry and credit.
For the first release, use only `**Highlights:** First release.` as the highlight.

Use Go 1.25.14 for release builds. Run the following local gate, plus the build,
test, and smoke steps with `GOTOOLCHAIN=go1.25.0` for minimum-toolchain coverage:

```sh
export GOTOOLCHAIN=go1.25.14
test -z "$(gofmt -l .)"
go mod tidy
git diff --exit-code -- go.mod go.sum
make test build
go vet ./...
go build -o bin/ ./cmd/...
python3 scripts/smoke.py bin 0.1.0
python3 scripts/release-notes.py v0.1.0
actionlint
goreleaser check
goreleaser build --snapshot --clean
```

Substitute the intended version in the commands. Review through P2, commit the
release preparation, and land it through a squash-merged PR. Wait for green CI
on the exact resulting `main` commit before tagging.

## Publish

Check local and remote tags and GitHub Releases immediately before tagging;
stop if the requested version already exists. Create an annotated `v<version>`
tag (signed when Git signing is configured) on the verified commit and push only
that tag. `.github/workflows/release.yml` tests the release and runs GoReleaser.
`scripts/release-notes.py` checks version consistency and supplies the finalized
changelog section, Highlights first, followed by download, checksum, and module
links. The workflow uses the repository's GitHub Actions token.

Watch the exact release run through completion. A manual dispatch accepts an
existing tag for recovery; inspect partial uploads before retrying and never
move a published tag. Fix real failures before retrying. For transient runner or
upload failures, retry once.

## Verify and finish

Read the published release and asset inventory back through the GitHub API.
Download archives and `checksums.txt`, verify SHA-256, and run each installed
command with `--version`. Run the synthetic smoke check on a native archive.
Check both macOS architectures with `codesign -dv --verbose=2`,
`spctl -a -vv -t open --context context:primary-signature`, and `otool -l`.
These cross-builds are not Developer ID signed or notarized (ARM64 receives the
Go linker's ad-hoc signature); report Gatekeeper's result without treating it
as a signing success. Confirm `LC_BUILD_VERSION` matches the README minimum.

Verify `GOPROXY=https://proxy.golang.org go list -m github.com/steipete/metcli@v<version>`
and a scratch `go install` of the versioned commands. Verify the release body
matches the finalized changelog section. Open an empty `## Unreleased` section
above it, review and land that follow-up through a PR, and leave `main` clean at
`origin/main`. Remove task branches and local `bin/` and `dist/` outputs.
