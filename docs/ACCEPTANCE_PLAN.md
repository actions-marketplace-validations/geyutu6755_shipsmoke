# ShipSmoke — Acceptance Plan (V1.0)

Date: 2026-11-01 | Status: Accepted

Every item below must be verified by an actual command run or a real CI run whose
record is linked in docs/VALIDATION_REPORT.md. "Planned" is not "passed".

## A. Automated test matrix (must all pass locally AND in CI on 3 OS)

### A1. Positive scenarios
| ID | Scenario | Verified by |
|---|---|---|
| P-01 | Valid ZIP artifact: extract, run entry, expect exit 0 | engine integration test + CLI run |
| P-02 | Valid tar.gz artifact: extract, run entry, expect exit 0 | engine integration test |
| P-03 | Standalone binary artifact: run directly | engine integration test |
| P-04 | Normal exit (0) recorded as PASS | runner test |
| P-05 | Multiple artifacts in one config all verified | engine integration test |
| P-06 | stdout `contains` matcher passes | runner/engine test |
| P-07 | JSON report well-formed, statuses correct | report test + CLI run + `jq` parse |
| P-08 | Non-zero expected exit code matched (e.g. expect 2, got 2 → PASS) | runner test |

### A2. Failure scenarios
| ID | Scenario | Expected behavior |
|---|---|---|
| F-01 | Artifact file missing | ERROR, clear message, exit ≠ 0 |
| F-02 | Entry binary missing inside archive | FAIL with archive listing, exit 1 |
| F-03 | Platform mismatch (binary header vs declared/host) | FAIL with explicit diagnosis |
| F-04 | Program exits non-zero unexpectedly | FAIL, exit code + output shown |
| F-05 | Unexpected exit code ≠ 0 when 0 expected | FAIL |
| F-06 | Timeout exceeded | FAIL, process tree killed, no orphans |
| F-07 | Invalid config (unknown key, bad regex, bad duration) | ERROR exit 2, actionable message |
| F-08 | Corrupted archive (truncated zip/gzip) | ERROR, structural diagnosis |
| F-09 | Path traversal entry (`../evil`) | extraction aborted, ERROR, nothing written outside |
| F-10 | Malicious symlink entry in tar | rejected explicitly |
| F-11 | Oversized / high-ratio entries (zip bomb) | rejected at limits |
| F-12 | stdout does not match expected | FAIL with diff-ish output |

### A3. Tooling gates
- `go vet ./...` clean.
- `gofmt -l .` empty.
- `go test ./...` green on Windows (local), Linux, macOS (CI).
- `govulncheck ./...` — no known vulnerabilities in deps.
- Fuzz targets (`FuzzExtractZip`, `FuzzExtractTarGz`) run ≥ 60 s without crash
  locally; targets remain in-tree for CI.
- Dependency audit: `go.mod` contains only `gopkg.in/yaml.v3`.

## B. Real cross-platform verification (CI evidence required)

| ID | Requirement | Evidence |
|---|---|---|
| C-01 | Full test suite green on real ubuntu runner | CI run link |
| C-02 | Full test suite green on real windows runner | CI run link |
| C-03 | Full test suite green on real macOS runner (arm64) | CI run link |
| C-04 | ShipSmoke verifies its own release artifacts on each platform (self-hosting) | CI run link |
| C-05 | Example workflows (Go/Rust/C examples) run green in real CI | CI run links |

No simulated platform labels; each job runs on a runner whose OS/arch genuinely
matches the verified artifact.

## C. Deliverable gates

| ID | Requirement |
|---|---|
| D-01 | `shipsmoke version` works from an installed binary (real install path: release download or `go install`) |
| D-02 | Release contains binaries for windows/amd64, windows/arm64, linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 + checksums.txt |
| D-03 | Each released binary: `--version` runs on its target platform (self-verification in release CI) |
| D-04 | README.md (English) complete: install, quickstart, config reference link, CI usage, exit codes, limitations |
| D-05 | README.zh-CN.md (Chinese) complete |
| D-06 | docs/: RESEARCH, PRODUCT_REQUIREMENTS, SYSTEM_DESIGN, THREAT_MODEL, ACCEPTANCE_PLAN, CONFIGURATION, LIMITATIONS, TROUBLESHOOTING, DEVELOPMENT_STATUS, VALIDATION_REPORT |
| D-07 | LICENSE (MIT), CONTRIBUTING.md, CODE_OF_CONDUCT.md, SECURITY.md, CHANGELOG.md present |
| D-08 | action.yml composite action + documented example workflow |
| D-09 | GitHub repo public, description, topics set |
| D-10 | v1.0.0 tag + GitHub Release with notes, binaries, checksums |
| D-11 | No secrets/tokens in repo history (checked before publish) |
| D-12 | examples/go-cli, examples/rust-cli, examples/c-cli each build + are verified by ShipSmoke in their own CI workflow |

## D. Final acceptance conclusion format

docs/VALIDATION_REPORT.md ends with a table mapping every gate above to evidence
(command + result, or CI run URL). The project is "released" only when the GitHub
Release URL is live and D-01…D-12 are checked.
