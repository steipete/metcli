# Releasing metcli

`.github/workflows/release-unified.yml` calls the shared
[Go CLI archetype](https://github.com/openclaw/release-workflows) pinned to
v1.11.8 (`fc49859568b3781af7663bee48c003fd987dc08b`). It publishes six platform
archives containing `metcli`, `ig-cookies`, and `ig-profile`, preserving
`metcli_<version>_<os>_<arch>.tar.gz` (Windows: `.zip`) and `checksums.txt`.
Archives also retain `CHANGELOG.md`, `LICENSE`, and `README.md`.

macOS binaries are Developer ID signed by Peter Steinberger (team
`Y5PE65HELJ`) with hardened runtime and notarized by Apple. The shared workflow
verifies the fleet-signed annotated tag, builds without signing credentials, then signs in a
separate job. Independent arm64 and Intel jobs verify the immutable artifacts
before publication. `ASSET-INVENTORY.json`, `SIGNING-MANIFEST.json`, and
`RELEASE-NOTES.md` accompany the checksums as release provenance.

## Repository prerequisites

Keep `main` protected with CI required. Actions workflow permissions must allow
read/write access and creation of pull requests for the closeout stage; each
workflow still declares explicit least-privilege permissions.

Repository secrets map to the archetype as follows:

| Repository secret | Shared workflow secret |
| --- | --- |
| `MACOS_SIGN_P12` | `MACOS_SIGNING_P12` |
| `MACOS_SIGN_P12_PASSWORD` | `MACOS_SIGNING_P12_PASSWORD` |
| `ASC_KEY_ID` | `ASC_KEY_ID` |
| `ASC_ISSUER_ID` | `ASC_ISSUER_ID` |
| `ASC_PRIVATE_KEY` | `ASC_PRIVATE_KEY_P8` |

There is currently no metcli Homebrew formula. If one is added, set
`homebrew-tap: steipete/homebrew-tap`, `homebrew-formula: metcli`, and map
`HOMEBREW_TAP_TOKEN` to `TAP_TOKEN`. The shared handoff passes inventory-derived
asset names and hashes to the tap, waits for its workflow, and verifies the
formula matches those exact assets. The Go module proxy is the source-install
channel; there is no npm, container, or Sparkle publication.

## Prepare

Start with clean, synchronized `main` and create a release branch. Set
`internal/version/version.go`, update the README install examples, and finalize
the current changelog section with the local date and a `**Highlights:**` line.
Keep every entry and credit.

Release builds use security-patched Go 1.26.10, `CGO_ENABLED=0`, and
`MACOSX_DEPLOYMENT_TARGET=12.0`. Build on macOS so every Darwin GoReleaser
post-build hook can inspect `otool -l`; it requires every binary and architecture
to report exactly macOS 12.0. Go currently sets that minimum for pure-Go builds;
the environment variable alone is not proof. If CGO is introduced, also set
`CGO_CFLAGS` and `CGO_LDFLAGS` with `-mmacosx-version-min=12.0` and retain this gate.

Run the local gate on macOS, substituting the intended version:

```sh
export GOTOOLCHAIN=go1.26.10
test -z "$(gofmt -l .)"
go mod tidy
git diff --exit-code -- go.mod go.sum
make test build
go vet ./...
go build -o bin/ ./cmd/...
python3 scripts/smoke.py bin 0.1.2
python3 -m unittest discover -s scripts -p 'test_*.py'
python3 scripts/release-notes.py v0.1.2
actionlint
goreleaser check
goreleaser build --snapshot --clean --parallelism=2
```

Also run build, test, and smoke with `GOTOOLCHAIN=go1.27.3`. Review through P2,
land preparation through a squash-merged PR, and wait for green CI on the exact
resulting `main` commit.

## Publish

Release dispatches default to `dry_run: true`. To build, sign, and independently verify the selected main commit without creating tags or releases, use `gh workflow run release-unified.yml --ref main -f version=0.1.2 -f dry_run=true`. Xcode 27 runners verify both architectures, with Intel execution under Rosetta.

Check local and remote tags and `gh release list --limit 3 --json tagName`
immediately before dispatch; stop if the intended version already exists.
After pushing the verified signed tag, dispatch the caller at the current
protected `main` head:

```sh
gh workflow run release-unified.yml --ref main -f version=0.1.2 -f dry_run=false
```

After the dry run succeeds, create an SSH-signed annotated `v<version>` tag at
the verified release-prep merge commit using the fleet release key. Verify it
with `.github/release-allowed-signers` before pushing. Never move an existing
tag. The workflow requires that signed tag and reuses its frozen source;
publishing still requires the explicit dispatch, not a tag-push trigger. `scripts/release-notes.py` validates
the version constant against the requested tag. The shared workflow publishes
the exact finalized changelog section, including its dated heading.

Watch the exact run through completion. Retry failed jobs once for transient
infrastructure errors; fix root causes otherwise. Retries reuse the immutable
annotated tag and must never move it. A corrected shared-workflow pin can run
from a newer main caller while rebuilding the existing tag’s frozen source.
After publication, verify the existing
release rather than rerunning signing, which produces new timestamped bytes.

## Verify and finish

Read the published release and assets through REST. Download fresh archives
and `checksums.txt` with `curl -fL`, verify SHA-256 and inventory digests, and
compare the release body and `RELEASE-NOTES.md` with
`python3 scripts/release-notes.py v<version>`.

For both macOS architectures, verify every binary with:

```sh
codesign -dvv metcli
codesign --verify --deep --strict --verbose=4 metcli
codesign --verify --strict --check-notarization -R=notarized metcli
spctl -a -vv -t open --context context:primary-signature metcli
python3 scripts/check_macos_target.py darwin metcli
```

Require Peter's identity and Team ID. Bare CLI archives cannot staple a ticket;
the online codesign notarization check proves Apple has the ticket. Record
`spctl` output separately, since bare executable assessment varies by macOS.
`curl` does not normally set quarantine, so explicitly add
`com.apple.quarantine` to the downloaded archive and extracted binaries before
checking `xattr -l` and running native `--version` commands. Never remove
quarantine to make the proof pass. Run the synthetic smoke check on the native
archive; it uses no real browser profile or Keychain.

Verify `GOPROXY=https://proxy.golang.org go list -m github.com/steipete/metcli@v<version>`
and a scratch `go install` of the versioned commands. If a Homebrew formula
exists, verify its bump and `brew upgrade`/`--version` too.

The workflow opens the next empty `## Unreleased` section in a closeout PR.
Review and land that PR with exact-head CI (dispatch CI explicitly if the bot
push did not trigger it). Leave `main` clean at `origin/main`, then remove task
branches and local `bin/` and `dist/` outputs.
