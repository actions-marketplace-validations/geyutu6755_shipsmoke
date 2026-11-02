# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased] — v1.0.4 candidate (branch `fix/v1.0.4-reliability`, not released)

### Fixed

- **`shipsmoke check --quiet` no longer crashes.** It panicked with a nil-pointer
  dereference in the released v1.0.3 binary: the quiet path rendered the full
  report into `io.Discard` with a `nil` redactor, and `Redactor.Apply`
  dereferenced it. The useless render is gone, and `Apply` now treats a nil
  receiver as "nothing to redact" so the class cannot return. Reproduced against
  the official v1.0.3 asset and covered by regression tests for the PASS, FAIL,
  ERROR, SKIP, NO-CHECKS, JSON-report and exit-code paths.
- **`shipsmoke <command> --help` prints usage.** For `check`, `validate` and
  `init` it previously printed `shipsmoke check: flag: help requested`, because
  the flag set wrote its usage to `io.Discard` while `flag.ErrHelp` was reported
  as an error. Help now goes to stdout and exits 0; genuinely invalid flags still
  print the error plus usage to stderr and exit 2 — the two cases are no longer
  conflated. The top-level help and README both told users to run this command.
- **Signal handling is per-run, not per-check.** `withSignalContext()` registered
  `signal.Notify` and a goroutine for every single check and never released them
  on normal completion, leaking one goroutine plus one live notifier per check,
  and making Ctrl+C abort only the check in flight. One handler is now installed
  for the whole run and explicitly torn down; a Unix-only test delivers a real
  SIGINT and asserts the remaining checks do not carry on.
- **The `env: inherit` warning is routable.** It wrote to the raw process
  `os.Stderr`, escaping any caller that injected its own writers, so it could not
  be asserted in tests. It now honours `Options.Stderr` (falling back to
  `os.Stderr`).
- **Checksums-manifest lookup in the action's fetch step is literal.** The asset
  name was interpolated into a `grep -E` pattern, where its dots match any byte:
  a manifest line for `shipsmoke_linux_amd64XtarXgz` satisfied the lookup for
  `shipsmoke_linux_amd64.tar.gz`. The digest is now taken from the line whose
  name field equals the asset byte-for-byte. New test proves the pre-fix script
  accepted and extracted the lookalike (exit 0) and the fixed one refuses (exit 2).

### Security

- `action.yml` — the composite action that **outside callers execute** — was the
  one file in this repository still using a movable third-party ref
  (`actions/upload-artifact@v4`). It is now pinned to the same commit SHA the
  rest of the repo uses, and `scripts/check-action-pins.bash` runs in CI to fail
  on any movable third-party ref, so this cannot regress silently. This closes a
  hole in the v1.0.3 entry below, which claimed all third-party Actions were
  pinned; literally that covered the workflows and missed the action itself.

### Added

- **`shipsmoke init --ci --published`** — a real, generated **post-release**
  gate: one job per OS downloads the asset users actually get from the Release
  and ShipSmoke runs it, triggered by `release: published` and also dispatchable
  against any existing tag. Per-platform asset names come from
  `--asset-linux/--asset-darwin/--asset-windows`; anything left unset becomes a
  visible `SET-ME-` placeholder that the workflow **refuses to run** rather than
  passing vacuously. This is a different question from the pre-publish gate —
  one blocks a bad release, the other detects a released one — and it is the
  only shape usable by a project that already has its own release pipeline.
- `docs/CONFIGURATION.md` gains a full command-line reference (previously no CLI
  flags were documented anywhere) and the post-release section is replaced with
  the generator's actual output instead of an unrunnable sketch.
- New regression tests across `cli`, `engine`, `cigen` and `report`, including a
  check that no generated workflow interpolates `${{ }}` inside a `run:` block
  or uses a heredoc — the two patterns behind the v1.0.1 fake-green incident.
- CI now runs `go test -race` on Linux. Disclosed rather than glossed: the race
  detector had **never** run for any release. It requires cgo, so it cannot run
  on a machine without a C toolchain, and no CI job invoked it either — the
  concurrent runner and engine had therefore never been race-checked anywhere.

### Removed

- `fetched-ci.yml` at the repository root: a leftover copy of the integration
  harness's workflow from the v1.0.2 heredoc debugging. Unreferenced, not in
  `.github/workflows/` so never executed, and it built a `teapot` target that
  does not exist in this repository.

### Corrected (documentation honesty)

- README's headline sample was labelled "real output from the self-verification
  CI run" while showing `shipsmoke v1.0.0` and artifacts that run does not
  produce. Replaced with captured output of the released v1.0.3 binary and an
  accurate caption; `README.zh-CN.md` matched.
- `shipsmoke-integration` described as "independent": it is a separate repository
  owned by the same maintainer, i.e. Tier 2 under `docs/ADOPTION.md`, not
  third-party adoption. Stated plainly in both READMEs, and a Tier 0 (local trial
  on someone else's artifacts) is now defined and recorded.
- Tier 2 "all green runs since v1.0.2" replaced with what the run list shows:
  latest run green on all three OSes, earlier failures while the harness itself
  was being fixed.
- `docs/CANDIDATES.md` re-verified: the rows for jq (has `sha256sum.txt` plus
  attestations) and gdu (~30 archives with `sha256sums.txt.asc`) were wrong, and
  ripgrep's checksums are per-artifact files, not a manifest. The list is
  retargeted from large projects to three small/medium ones with the gap read
  from their own workflow files.
- `docs/VALIDATION_REPORT.md` said 7 release assets; v1.0.3 publishes 9.
- `docs/DEVELOPMENT_STATUS.md` said there were no known open issues while a
  crash was shipped; it now lists the register.
- The v1.0.3 claim that `init --ci` was "end-to-end verified" overstated: only
  unit tests covered it then. The generated post-release template is exercised by
  CI on this branch, and the distinction is recorded in the status doc.

### Fixed in review of this branch's own changes

The first cut of `init --ci --published` was reviewed adversarially before
committing, and the review found three defects in code written on this branch.
Recording them here rather than pretending the first version was right:

- **Generated workflow could be corrupted by its own input.** `--asset-*` values
  were written into `.shipsmoke.yml` and the workflow unquoted and unvalidated.
  A value containing a newline closes the mapping and injects keys into a
  workflow that then runs with the caller's token; benign values like
  `*name.tar.gz`, a space or a `#` corrupt the emitted document outright. Asset
  names are now grammar-checked (`ValidateAsset`, rejecting spaces, slashes,
  quotes, newlines, `..`) and emitted as quoted scalars, and
  `TestPublishedRejectsHostileAssetNames` covers the hostile cases.
- **The tag "grammar check" was not a validator.** A POSIX `case` pattern
  `v[0-9]*` means *digit, then anything*, so `v1.0../../x`, spaces and newlines
  all passed, and the value was then appended to `GITHUB_OUTPUT` and echoed into
  log lines. Replaced by an anchored `grep -Eq` on a single line, with the
  rejected value printed through `printf %q` so control characters are visible
  instead of executed as log markup.
- **A format verb swallowed an argument.** The template contained shell text
  `printf '%s' "$TAG"`; inside Go's `Sprintf` that is a positional verb, so the
  generated workflow came out with an action ref pasted in place of the shell's
  `%s`. It is escaped now, and the new documentation test would have caught it
  either way — which is the reason it exists.

### Added (review round)

- `TestDocsConfigurationMatchesGenerator` compares `docs/CONFIGURATION.md`
  against the generator's bytes (comments ignored) on every test run, so the
  page cannot drift from the emitter. The doc blocks are now the generator's
  actual output, comments included, replacing the earlier "with comments
  elided" reproduction that had already diverged.
- Pinning gate now also scans `README.md`, `README.zh-CN.md` and `docs/*.md` for
  `uses:` lines: a snippet in documentation is an instruction someone pastes.
  Three snippets still showing `actions/checkout@v4` / `setup-go@v5` were
  corrected; verified the gate fails when such a ref is reintroduced.
- `init --ci --published --stack …` is now refused instead of silently dropping
  `--stack`, and passing `--asset-*` without `--published` warns rather than
  ignoring the flag.

### Fixed (reliability details found in the same review)

- `scripts/fetch-shipsmoke.sh`: a manifest whose last line has no trailing
  newline was skipped entirely, so the final asset would be reported as missing.
  Also, an uppercase digest in a valid manifest would have mismatched the
  lowercase output of `sha256sum` and refused a good file; digests are now
  normalised (`tr`, not `${var,,}`, because macOS runners still ship bash 3.2).
- The goroutine-leak test sampled `runtime.NumGoroutine` through a helper that
  called its own baseline inside its poll loop, so it returned on the first
  sample and never actually settled. Baseline is measured once, a warm-up run
  absorbs lazy init, and samples must agree twice before being compared.
- The SIGINT test asserted only "not success" and waited on a fixed sleep that
  could fire before the child process existed. It now waits for evidence the
  fixture is running, asserts that no check reported PASS, and bounds elapsed
  time — which is what distinguishes the fixed behaviour (seconds) from the old
  one (the remaining 30s checks still running).

### Changed (deliberate behaviour differences)

Two user-visible changes are intended, not accidents, and callers should know:

- `shipsmoke <command> --help` now writes to **stdout and exits 0** instead of
  exiting 2. Anything that treated the broken `--help` exit as an error will now
  see success.
- Ctrl+C aborts the **whole run**, not just the check that was in flight. A run
  interrupted mid-way reports failure rather than a partial pass, so CI cannot
  read an interrupted verification as a green gate.

`engine.Options` gained a `Stderr` field (additive; unset means `os.Stderr`), and
the removed `usageError` helper was unexported, so nothing outside this module
was affected.

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
