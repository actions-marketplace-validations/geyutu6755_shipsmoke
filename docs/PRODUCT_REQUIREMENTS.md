# ShipSmoke — Product Requirements Document (V1.0)

Version: 1.0.0 | Status: Accepted | Date: 2026-11-01

## 1. Positioning

**ShipSmoke — Test what users actually download.**

A free, open-source, cross-platform CLI tool that verifies that a software release
artifact (ZIP, tar.gz, standalone binary) can be extracted, recognized, launched and
exercised on the operating system it targets — before the release goes out.

Target users: GitHub open-source maintainers, CLI tool authors, release engineers,
indie developers.

## 2. Competitive advantages (all mandatory in V1.0)

1. Independent of GoReleaser / npm / Cargo / any build tool.
2. Free; no paid API, no cloud service, no account.
3. Verifies by *real execution on a real operating system* — not filename heuristics,
   not checksum-only, not `--version`-only scripts.
4. Simple YAML configuration; drop into any CI in minutes.
5. Reproducible diagnostics: every failure reports cause, key logs, and the exact
   command to reproduce.
6. Works as an automated release quality gate (exit-code contract).

## 3. Functional requirements

### FR-1 Artifact support
- Formats: ZIP, tar.gz/tgz, standalone executable.
- Verify file existence and structural integrity before extraction.
- Locate the user-specified entry binary inside the artifact.
- Identify the artifact's OS/architecture from actual binary metadata
  (ELF/PE/Mach-O headers) whenever the format allows — never from filenames alone.
- Detect platform/architecture mismatch between declaration and binary reality.
- When reliable identification is impossible, say so explicitly instead of guessing.
- Allow explicit entry-binary relative path; allow multiple artifacts per config.

### FR-2 Real execution
- Extract safely to an isolated temp working directory.
- Locate the entry binary; execute user-configured check commands.
- Capture exit code, stdout, stderr (with size caps).
- Assert expected exit code and optional stdout/stderr conditions.
- Reliable timeout and process-tree termination.
- Explicit diagnosis on abnormal exit (signal, timeout, spawn failure).
- Clean up temp directories after the run.
- Launch via argument arrays only — never shell string concatenation.
- Any check failure → non-zero process exit so CI can block the release.

### FR-3 Configuration (`.shipsmoke.yml`)
- Config version (v1).
- Per-artifact: name, path, optional format, entry binary path, target platforms.
- Per-check: args, expected exit code, optional stdout/stderr matchers
  (contains / not-contains / equals / regex), timeout.
- Global: report output paths, extraction safety limits.
- Strict validation: unknown keys, missing artifacts, illegal values → clear errors.
- `shipsmoke init` generates a commented example.

### FR-4 CLI
- `shipsmoke init` — write example config.
- `shipsmoke validate` — validate config only, no execution.
- `shipsmoke check` — run full verification.
- `shipsmoke version` — version, commit, platform.
- Friendly `--help`, documented exit codes, actionable error messages.

### FR-5 Reporting
- Human terminal report with per-check status.
- Machine-readable JSON report.
- GitHub Actions Job Summary (markdown) when `$GITHUB_STEP_SUMMARY` is set.
- Statuses: PASS / FAIL / SKIP / ERROR.
  - PASS: executed and met expectations.
  - FAIL: executed and did not meet expectations.
  - SKIP: not executed (artifact targets a different platform) — always labeled.
  - ERROR: infrastructure problem (config, extraction failure, spawn failure).
- Report shows verification target and environment (OS/arch, shipsmoke version).
- Failures include reason, key output, and a reproduction command.
- Never report unexecuted checks as passing. Never fake a platform.
- Secrets (tokens/credentials from the environment) are redacted from all reports.

### FR-6 GitHub Actions integration
- A composite action (`action.yml`) usable as `uses: geyutu6755/shipsmoke@v1` with
  documented inputs (config path, version, report paths, fail behavior).
- Works on windows-latest, ubuntu-latest, macos-latest (real runners).
- Workflow recipes for: (a) verify after build, before publish; (b) post-release
  re-download verification (documented; V1.0 fully implements (a)).
- Failure blocks subsequent steps via exit code; report + summary uploaded.

## 4. Non-functional requirements

- NFR-1 Zero-config run against a single binary: `shipsmoke check --binary ./mytool`.
- NFR-2 Cold runtime < 1 s for single-binary checks; artifact checks dominated by
  extraction time.
- NFR-3 Static single binary; no network access at runtime.
- NFR-4 Go standard engineering practices: module boundaries, table-driven tests,
  `go vet` clean, `gofmt` clean.
- NFR-5 All docs in English + a Chinese README (`README.zh-CN.md`).

## 5. Out of scope (V1.0)

- Package-manager formats (deb, rpm, msi, dmg, apk, AppImage).
- Artifact signature verification (cosign integration) — document as roadmap.
- Parallel execution of artifacts (sequential by default for deterministic logs).
- Downloading artifacts from URLs (`source: url` is a roadmap item; V1.0 verifies
  local paths, which covers the build-then-verify and CI-artifact-download flows).
