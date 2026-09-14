# Changelog

## Unreleased

- Release binaries for macOS are now Developer ID signed and notarized, so direct downloads pass Gatekeeper.
- Fix release-note publication and verify the published body against the finalized changelog.

## 0.1.0 - 2026-09-13

**Highlights:** First release.

- Publish macOS, Linux, and Windows archives containing `metcli`, `ig-cookies`, and `ig-profile`, with SHA-256 checksums and `--version` output.
- Prefer Go 1.25.14 while retaining Go 1.25.0 compatibility; verify both toolchains and synthetic cookie export in CI.
- Update the SQLite cookie-reader dependency to v1.58.0, retaining its required libc v1.75.6.
