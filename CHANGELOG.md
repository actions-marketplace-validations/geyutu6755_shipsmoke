# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [1.0.1] - 2026-11-01

### Fixed

- `go install github.com/geyutu6755/shipsmoke/cmd/shipsmoke@vX.Y.Z` now
  reports the installed module version instead of `0.0.0-dev` (version is
  read from build info when ldflags are absent).

## [1.0.0] - 2026-11-01

First stable release.

### Added

- `shipsmoke check` — the release quality gate: safe extraction, binary
  identification from real ELF/PE/Mach-O headers, configurable checks
  (exit code, stdout/stderr contains / not-contains / equals / regex),
  per-check timeouts with process-tree kill, explicit PASS/FAIL/SKIP/ERROR.
- `shipsmoke validate` — strict config validation (unknown keys are errors).
- `shipsmoke init` — generates a commented example config.
- `shipsmoke version` — build information.
- `.shipsmoke.yml` schema v1: multiple artifacts, per-artifact platform
  gating (`runs_on`), environment modes (`minimal`/`inherit` with secret
  filtering), extraction safety limits, JSON report path.
- Reporting: colored terminal report, machine-readable JSON report, GitHub
  Actions job summary, secret redaction, repro commands on failures.
- Exit-code contract: 0 pass, 1 fail, 2 usage/config, 3 nothing executed
  (fail-closed; `--allow-empty` overrides).
- GitHub Actions composite action (`geyutu6755/shipsmoke@v1`) with report
  upload and job summary.
- Platform mismatch diagnosis from binary headers (declared vs actual vs
  host); honest "unknown" when a binary cannot be identified.
- Extraction hardening: Zip Slip double validation, streaming limits
  (entries, per-file, total, compression ratio), rejection of symlinks,
  hard links, devices, FIFOs, control characters, Windows-reserved names;
  normalization of backslash zip entries (PowerShell Compress-Archive
  quirk) with traversal still rejected.
- Tests: unit, integration (fixture binaries cross-compiled in-test),
  fuzzing targets for both extractors, full CI matrix on ubuntu-24.04,
  macos-15 (arm64), windows-2025; govulncheck gate.
- Examples for Go, Rust, and C toolchains, each verified in CI.
- Documentation: README (EN/中文), configuration reference, system design,
  threat model, acceptance plan, limitations, troubleshooting.

### Security

See [SECURITY.md](SECURITY.md) and [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md).

[1.0.0]: https://github.com/geyutu6755/shipsmoke/releases/tag/v1.0.0
