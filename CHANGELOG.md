# Changelog

## Unreleased

- Security: select Go 1.27.3 for source builds and Go 1.26.10 for macOS 12-compatible releases to fix GO-2026-6809, preserving the Go 1.25.0 language minimum.
- Move macOS CI and signed release verification to Xcode 27, preserve macOS 12 with patched Go 1.26.9, test current Go 1.27.2, and verify all Intel CLIs under Rosetta.
- Add safe, non-publishing release dry runs through the updated shared workflow.

## 0.1.1 - 2026-09-14

**Highlights:** macOS downloads are signed and notarized.

- Release binaries for macOS are now Developer ID signed and notarized, so direct downloads pass Gatekeeper.
- Fix release-note publication and verify the published body against the finalized changelog.

## 0.1.0 - 2026-09-13

**Highlights:** First release.

- Publish macOS, Linux, and Windows archives containing `metcli`, `ig-cookies`, and `ig-profile`, with SHA-256 checksums and `--version` output.
- Prefer Go 1.25.14 while retaining Go 1.25.0 compatibility; verify both toolchains and synthetic cookie export in CI.
- Update the SQLite cookie-reader dependency to v1.58.0, retaining its required libc v1.75.6.
