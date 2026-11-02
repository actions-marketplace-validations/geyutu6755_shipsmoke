# ShipSmoke — Development Status

Living document. Updated at the end of every phase.

## Phase status

| Phase | Scope | Status | Evidence |
|---|---|---|---|
| P0 | Research, requirements, architecture, threat model, acceptance plan | **DONE** | docs/RESEARCH.md, PRODUCT_REQUIREMENTS.md, SYSTEM_DESIGN.md, THREAT_MODEL.md, ACCEPTANCE_PLAN.md; name check completed (available) |
| P1 | Core CLI + config system | DONE | internal/config, internal/cli; `go test ./...` green (Windows) |
| P2 | Safe artifact handling | DONE | internal/artifact (detect/extract/limits); unit + fuzz tests |
| P3 | Execution engine + diagnostics | DONE | internal/runner (timeout, process-tree kill, capture); re-exec tests |
| P4 | Reporting (terminal/JSON/GH summary) + redaction | DONE | internal/report |
| P5 | GitHub Actions integration | DONE | action.yml + .github/workflows/ci.yml + release.yml |
| P6 | Failure tests, cross-platform CI, security review | DONE | CI green on ubuntu-24.04/macos-15/windows-2025; govulncheck clean; see VALIDATION_REPORT.md |
| P7 | Go/Rust/C examples + real integration | DONE | examples/{go,rust,c}-cli with own CI workflows, all green |
| P8 | Docs, packaging, release prep | DONE | README/README.zh-CN/docs/*; build script; checksums |
| P9 | GitHub release + final acceptance | DONE | https://github.com/geyutu6755/shipsmoke/releases/tag/v1.0.0 |

## Environment record

- Dev machine: Windows 11 x64 (win32 10.0.26220)
- Go: 1.27.3 (windows/amd64), SHA256-verified download
- git: 2.55.0.windows.3
- GitHub account: geyutu6755 (token scopes: repo, workflow, gist)
- CI: GitHub Actions (ubuntu-24.04, macos-15 arm64, windows-2025)

## Command log (phase verification)

### P0
- Name check: GitHub API search (0 hits), crates.io/npm/PyPI/Homebrew (all free) — 2026-11-01

### P1
- `go build ./...` — OK
- `go test ./...` — OK (config validation suite)

### P2
- `go test ./internal/artifact/...` — OK (detection, extraction, malicious archive suite)
- `go test -fuzz=FuzzExtractTarGz -fuzztime=30s` — no crashes
- `go test -fuzz=FuzzExtractZip -fuzztime=30s` — no crashes

### P3
- `go test ./internal/runner/...` — OK (exit codes, output capture, timeout kill, env filtering)

### P4
- `go test ./internal/report/...` — OK (redaction, JSON schema, summary)

### P5–P9
- See docs/VALIDATION_REPORT.md for CI run URLs and release evidence.

## Known open issues

Honest list, as of the v1.0.4 reliability round on branch
`fix/v1.0.4-reliability` (2026-11-02). An earlier revision of this page said
"none blocking release" while a shipped crash existed, which is exactly the kind
of claim this project should not make.

### Fixed on the branch, awaiting review and a release

| # | Issue | Severity | Evidence it was real |
|---|---|---|---|
| 1 | `check --quiet` panicked (nil redactor through a discarded render) | **crash, shipped in v1.0.3** | reproduced with the official release asset; stack trace at `report.go:59` |
| 2 | `<command> --help` printed a parse error instead of usage | high (usability) | `shipsmoke check --help` → `flag: help requested`, exit 2 |
| 3 | `action.yml` used movable `actions/upload-artifact@v4` | medium (supply chain) | 1 unpinned ref against 26 SHA-pinned ones |
| 4 | post-release recipe in docs was an unrunnable sketch | high (adoption) | referenced a path nothing created; verification left as a comment |
| 5 | one `signal.Notify` + goroutine leaked per check | medium | `withSignalContext()` discarded `cancel`, called per check |
| 6 | `env: inherit` warning bypassed injected writers | low | wrote to raw `os.Stderr` |
| 7 | fetch script matched the manifest by regex with unescaped dots | low (hardening) | pre-fix script extracted a lookalike-named asset, exit 0 |
| 8 | dead `fetched-ci.yml` at repo root | cleanup | built a `teapot` that does not exist here; never in `.github/workflows/` |

### Still open after this round

- **Windows/macOS/Linux parity is CI-only here.** Fuzzing runs on the Linux job.
  `-race` could not run locally (needs cgo, and there is no C toolchain here) and
  **was not invoked by CI either** until this branch added
  `go test -race ./...` to the Linux job — so no release before v1.0.4 has ever
  been race-checked. Its first real result is this branch's CI run.
- **`windows/arm64` is still published unexecuted** (`build-only` in
  `release.yml`): no x64 GitHub-hosted runner executes it. The job summary says
  so instead of claiming a run.
- **Generated workflows pin `geyutu6755/shipsmoke@v1` by tag** by design (see
  `docs/SUPPLY_CHAIN.md` §3); callers wanting full pinning must use a commit SHA.
- **Tier 3 adoption: still zero.** Nothing on this branch changes that, and the
  local trial against termide's artifacts is Tier 0, not adoption.
- **Pin labels are checked by hand**, not by the new CI script, which verifies
  ref *form* only (`docs/SUPPLY_CHAIN.md` §3.4).

Product-scope limitations (formats, sandbox boundaries) stay in
docs/LIMITATIONS.md.

## v1.0.2 maintenance round (2026-11-01)

- Action hardening: env-var-only inputs, SHA-256-verified downloads, strict
  version pinning; 23-case fetch-script suite + CI injection regression.
- Critical finds fixed: v1.0.0/v1.0.1 wrong-arch artifacts (GOOS/GOARCH not
  exported), heredoc fake-green in CI verify gates, release needs-guard
  override; both premature/wrong publishes retracted and disclosed.
- Coverage: linux/arm64 via QEMU container, darwin/amd64 via Rosetta,
  windows/arm64 build-only (recorded).
- Independent integration repo green on 3 OSes via `@v1` + `version: v1.0.2`.
- Release: https://github.com/geyutu6755/shipsmoke/releases/tag/v1.0.2

## v1.0.3 maintenance round (2026-11-01)

- tar.gz gzip CRC now always verified (payload corruption, corrupt CRC,
  truncated trailers fail; trailing-garbage leniency kept and documented);
  5 corruption fixtures; docs/VERIFICATION_LAYERS.md.
- Strict version grammar in the action fetch step; release job re-verifies
  all assets against checksums.txt; third-party Actions SHA-pinned;
  build-provenance attestations; docs/SUPPLY_CHAIN.md.
- `shipsmoke init --ci --stack go|rust|c` (e2e-verified); Go/Rust/C
  tutorials; README rewritten onboarding (no placeholder scripts); real
  broken-release demo in examples/ and README.
- Homebrew tap + Scoop bucket shipped with CI install-tests (green).
- docs: INTRODUCTION, ADOPTION (tiered honest adoption tracking),
  CANDIDATES (8 evidence-based projects; no outreach performed).

## Final state

- Repository: https://github.com/geyutu6755/shipsmoke (public, MIT)
- Release: https://github.com/geyutu6755/shipsmoke/releases/tag/v1.0.0
- CI: all workflows green on real runners (see VALIDATION_REPORT.md §3–§5)
- Release run: https://github.com/geyutu6755/shipsmoke/actions/runs/41226680894
