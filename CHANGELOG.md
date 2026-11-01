# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [1.0.3] - 2026-11-01

### Fixed

- **tar.gz integrity:** the extractor now drains the full gzip member after
  the tar payload ends, so the CRC32/ISIZE trailer is always verified.
  Corrupted payload bytes, corrupt CRC fields and truncated trailers fail
  extraction; trailing bytes after a complete member remain accepted
  (documented). Five corruption fixtures added; see
  docs/VERIFICATION_LAYERS.md for the three-layer model.

### Security

- All third-party Actions in this repository's workflows pinned to commit
  SHAs; build-provenance attestations (SLSA) generated for every released
  archive (`gh attestation verify`); release job re-verifies every downloaded
  asset against checksums.txt before publishing. Audit: docs/SUPPLY_CHAIN.md.
- Action `version:` grammar hardened: strict `vX.Y.Z` (no moving refs, no
  prerelease suffixes, no leading zeros); 28 fetch-script tests.

### Added

- `shipsmoke init --ci --stack go|rust|c --name <tool>`: generates
  `.shipsmoke.yml` plus a three-platform GitHub workflow (build, package,
  shipsmoke gate) mirroring the verified integration harness; explicit
  stack contract, no build-system inference. End-to-end verified.
- Tutorials: docs/TUTORIAL_GO.md (executed end-to-end), TUTORIAL_RUST.md and
  TUTORIAL_C.md (1:1 with the example workflows that run in CI).
- Distribution: Homebrew tap ([geyutu6755/homebrew-tap]) and Scoop bucket
  ([geyutu6755/scoop-bucket]), both installed-and-tested in their own CI.
- docs: VERIFICATION_LAYERS, SUPPLY_CHAIN, INTRODUCTION, ADOPTION,
  CANDIDATES (evidence-based research; no outreach performed).
- README: real broken-release demo output, one-minute quickstart, failure
  guidance now points at TROUBLESHOOTING/CONFIGURATION.

### Fixed (docs)

- README, the Chinese README and CONFIGURATION.md no longer reference
  placeholder scripts (./build.sh, ./publish.sh); every example is
  copy-pasteable.

[geyutu6755/homebrew-tap]: https://github.com/geyutu6755/homebrew-tap
[geyutu6755/scoop-bucket]: https://github.com/geyutu6755/scoop-bucket

## [1.0.2] - 2026-11-01

### Security

- **Composite action: eliminated script-injection vectors.** All user inputs
  (`version`, `binary-path`, `config`, `artifact`, `report`,
  `allow-empty`) now reach shell code only through environment variables;
  no `${{ inputs.* }}` interpolation remains inside any `run:` block.
  Automated injection-regression tests on all three operating systems
  verify hostile payloads fail inertly.
- **Composite action: release downloads are now integrity-checked.** The
  fetched asset is SHA-256 verified against the release's `checksums.txt`
  before extraction; failed downloads, missing manifest entries, and hash
  mismatches abort before the binary is executed. A step output
  (`verified`) exposes how the executed binary was obtained.
- **Version pinning hardened:** the action's `version` input accepts only
  `latest` or a full `vX.Y.Z` tag; moving refs (`v1`, `v1.0`) and non-tag
  values are rejected with an explanatory error.

### Added

- `scripts/fetch-shipsmoke.sh` + `scripts/fetch-shipsmoke_test.bash`: the
  action's fetch logic is a standalone, environment-parameterized script
  with a 23-case test suite (injection, malformed versions, tampered
  checksums, refusal paths, success path) running in CI on all three OSes.
- `action-test` workflow: fetch-script suite, end-to-end composite-action
  run via `uses: ./` with report assertions, real-download check against a
  published release, and the injection regression.
- Independent integration repository
  ([shipsmoke-integration](https://github.com/geyutu6755/shipsmoke-integration))
  consuming `geyutu6755/shipsmoke@v1` from an outside project on all three
  operating systems.
- Release pipeline execution coverage: linux/arm64 verified in a QEMU
  arm64 container, darwin/amd64 verified under Rosetta 2; windows/arm64 is
  recorded honestly as build-only in the release job summary.

### Fixed

- README/中文文档 referenced `uses: geyutu6755/shipsmoke@v1` before any `v1`
  tag existed; the `v1` tag now tracks the v1.0.2 release.
- Documentation/Release-notes version inconsistencies cleaned up.

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
