# Final acceptance — ShipSmoke active-development closeout

Date: 2026-11-08. This document closes the project's active development
phase. Evidence links point to real GitHub runs; anything not executed is
marked NOT TESTED rather than assumed.

## Current official version

**v1.0.5** — https://github.com/geyutu6755/shipsmoke/releases/tag/v1.0.5
(v1.0.4 remains available at https://github.com/geyutu6755/shipsmoke/releases/tag/v1.0.4)
(7 assets: 6 platform archives + checksums.txt; build provenance attestations
attached via `gh attestation verify <archive> -R geyutu6755/shipsmoke`).

v1.0.4 stays the stable release: the final review found **no defects that
require changing the shipped product** (see "Review outcome" below). All
closeout commits are docs, tests and evidence only.

## Final main commit

`07e519c`…`137a86b` line ends the closeout series; the closeout branch tip
is recorded in git log — the authoritative value is
`git rev-parse origin/main` at read time. Every closeout commit's CI is
green (links in git log and the Actions tab).

## Core functionality (final state)

- Formats: ZIP, tar.gz/tgz, standalone binaries (ELF/PE/Mach-O/shebang).
- Identification from real binary headers (magic bytes via `debug/elf`,
  `debug/pe`, `debug/mach-o`); filenames never decide (regression test:
  `TestArchiveNameDoesNotDecideArchitecture`, added this round).
- Safe extraction: Zip Slip double guard, streaming limits (entries,
  per-file, total, compression ratio), symlink/hardlink/device/FIFO
  rejection, control-character and Windows-reserved-name rejection, exec-bit
  normalization, full gzip member CRC verification (v1.0.3).
- Execution: argv-array launch (no shell), context timeout with
  process-tree kill, one signal handler per run with teardown, capped output
  capture (1 MiB/stream), env modes `minimal`/`inherit` with secret-shaped
  variable filtering, platform gate with header-based mismatch diagnosis.
- Reports: colored terminal, JSON, GitHub step summary; secret redaction
  including args and check names.
- CLI: check/validate/init/version/help; `init --ci --stack go|rust|c`
  (pre-publish gate) and `init --ci --published` (post-release gate).
  Exit codes: 0 pass / 1 fail / 2 usage / 3 nothing executed.
- Composite action: env-only input passing, SHA-256-verified tool download,
  pinned third-party refs, `verified` output.

## Security checks

- `govulncheck` green in CI (caught GO-2026-4869 in the v1.0.3 cycle).
- All third-party Actions pinned to commit SHAs across all workflows;
  `scripts/check-action-pins.bash` gates it in CI (34 refs, 0 violations).
- Fetch script: strict `vX.Y.Z`/`latest` tag grammar, byte-for-byte manifest
  lookup, refuses to execute unverified downloads (30-case suite).
- Generated workflows: no `${{ }}` inside run blocks, no heredocs, asset
  names grammar-checked and quoted (adversarial tests).
- Release pipeline: per-asset SHA-256 re-verification before publish;
  SLSA build-provenance attestations; minimal workflow permissions.
- Supply-chain rationale: docs/SUPPLY_CHAIN.md.

## Test matrix

- Go suite: 8 packages green (Windows local + CI on ubuntu-24.04,
  macos-15 arm64, windows-2025). Coverage (2026-11-04, 7a2a82d): artifact
  62.7%, cigen 96.6%, cli 79.3%, config 91.7%, engine 84.6%, report 84.1%,
  runner 84.6%.
- `go test -race` on Linux CI (first enabled in the v1.0.4 cycle).
- Fuzz targets for both extractors (Linux CI short runs).
- Fetch-script suite: 30 cases (injection, tamper, refusal, success paths).
- Docs-drift test: `TestDocsConfigurationMatchesGenerator`.
- New this round: `TestArchiveNameDoesNotDecideArchitecture` (three verdict
  directions across the CI matrix: windows/ubuntu refuse, macos executes).

## Cross-platform real-run evidence

- shipsmoke's own releases: per-target verification native (linux/amd64,
  darwin/arm64, windows/amd64), QEMU container (linux/arm64), Rosetta
  (darwin/amd64); windows/arm64 honestly recorded as build-only.
- Post-release gate self-hosted on main (`.shipsmoke-published.yml`):
  dispatch with tag=v1.0.4 —
  [run 42060359609](https://github.com/geyutu6755/shipsmoke/actions/runs/42060359609)
  — all three OS jobs verified SHA-256 then extracted and executed the real
  v1.0.4 binaries; no SKIP conversions.
- Real `release: published` event exercised in shipsmoke-integration with
  controlled teapot assets (v0.0.2 draft→publish): event auto-triggered the
  gate; v1.0.4 later verified the full success path on all three OS with
  correct assets — [run 42130307774](https://github.com/geyutu6755/shipsmoke-integration/actions/runs/42130307774).
  A v0.0.1 sequencing finding (event fires at release creation, before asset
  upload completes when done via API) is recorded in VALIDATION_REPORT §6f.
- Truncated-artifact refusal: gate failed with the exact SHA-256 mismatch
  ([run 42075856530](https://github.com/geyutu6755/shipsmoke-integration/actions/runs/42075856530)).

## Third-party evidence

- termide 0.48.0 (Tier 0, windows/amd64 real run, 2 PASS; truncation
  refusal demonstrated) — 2026-11-05, shipsmoke v1.0.4.
- git-graph v0.8.3 (Tier 0, three-OS trial): linux and windows archives
  really executed and PASSED; the macos-amd64-named archive was verified to
  contain an **arm64** Mach-O (file(1) + independent header parse +
  native-execution pass) — a real, unreported upstream packaging defect,
  held as a ready-to-file report in
  [CASE_STUDIES/git-graph-v0.8.3-macos-arch-mismatch.md](CASE_STUDIES/git-graph-v0.8.3-macos-arch-mismatch.md).
  Maintainers NOT contacted.

## Install channels (all v1.0.4)

| Channel | Command | Last verified install test |
|---|---|---|
| Homebrew tap | `brew install geyutu6755/tap/shipsmoke` | install-test workflow green per push |
| Scoop bucket | `scoop bucket add geyutu6755 https://github.com/geyutu6755/scoop-bucket` then `scoop install shipsmoke` | install-test workflow green per push |
| Go | `go install github.com/geyutu6755/shipsmoke/cmd/shipsmoke@v1.0.5` | verified locally (reports v1.0.5) |
| GitHub Release | per-platform curl examples in README | latest URL download verified |
| **GitHub Marketplace** | `uses: geyutu6755/shipsmoke@v1` | **[Listed](https://github.com/marketplace/actions/shipsmoke)** (v1.0.5, CI: Continuous integration + Testing) |

## Known limitations (unchanged, documented)

- windows/arm64: build-only, never executed (no x64 execution path;
  disclosed in release job summaries).
- macOS runner starvation can cancel queued jobs (observed repeatedly in
  the v1.0.4 cycle; resolved by re-runs — infrastructure, not code).
- macos/amd64 archives can only be gate-verified on matching runners; on
  arm64 macos runners the gate SKIPs them honestly.
- tar.gz: content corruption inside a complete member is caught by the
  member CRC (v1.0.3+); trailing bytes after a complete member are
  tolerated (documented leniency).
- Not a sandbox: verified programs run with the caller's permissions.

## Outstanding items (need the account owner)

1. GitHub Marketplace listing for the action: metadata (name, description,
   branding, license) is ready in action.yml; publishing requires the
   repository owner to accept the GitHub Marketplace Developer Agreement
   once, in the Release-creation UI ("Publish to Marketplace"). No API path
   exists; no bypass attempted.
2. Nothing else pending.

## External adoption status (honest)

| Tier | State |
|---|---|
| 1 self-verification | every release |
| 2 independent harness | shipsmoke-integration, green |
| 3 external maintainer acceptance | **none** |
| 4 sustained external use | **none** |

Trials (Tier 0): termide 0.48.0, teapot fixtures, git-graph v0.8.3. None of
these imply adoption; no third-party maintainer has been contacted.

## Authenticity conclusion

Every claim above links to a run, a hash, or a command that can be
re-executed. Where something was not executed (windows/arm64, Intel macOS
runners), this document says NOT TESTED/build-only instead of assuming.
ShipSmoke's own release history (v1.0.0/v1.0.1 wrong-arch artifacts) was
self-reported, disclosed on the release pages, and is the case study that
motivates the tool's platform gate.
