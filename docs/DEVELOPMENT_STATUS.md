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

- None blocking release. See docs/LIMITATIONS.md for product-scope limitations.

## Final state

- Repository: https://github.com/geyutu6755/shipsmoke (public, MIT)
- Release: https://github.com/geyutu6755/shipsmoke/releases/tag/v1.0.0
- CI: all workflows green on real runners (see VALIDATION_REPORT.md §3–§5)
- Release run: https://github.com/geyutu6755/shipsmoke/actions/runs/41226680894
