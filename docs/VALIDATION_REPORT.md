# ShipSmoke — Validation Report

Living document. Every claim here is backed by a command output or a CI run URL.

## 1. Local verification (Windows 11 x64, Go 1.27.3)

| Command | Result | Date |
|---|---|---|
| `go build ./...` | OK | 2026-11-01 |
| `go test ./...` | OK — see §2 counts | 2026-11-01 |
| `go vet ./...` | clean | 2026-11-01 |
| `gofmt -l .` | empty | 2026-11-01 |
| `govulncheck ./...` | no known vulnerabilities | 2026-11-01 |
| Fuzz: extractors (tar.gz, zip), 30 s each | no crashes | 2026-11-01 |

## 2. Automated test matrix results

Filled during P6 — final state:

| Gate | Result |
|---|---|
| A1 positive scenarios (P-01…P-08) | PASS (integration + unit) |
| A2 failure scenarios (F-01…F-12) | PASS (integration + unit) |
| A3 tooling gates | PASS |

## 3. Cross-platform CI evidence (real GitHub-hosted runners)

CI runs recorded after the first push. Each `CI` run contains one
`Test (os)` job per platform — the full suite (unit, integration, fuzz seeds,
fuzz 30 s, gofmt, go vet, govulncheck, CLI self-check) — plus one
`Self-verify artifact (os)` job per platform (packaging shipsmoke and
verifying the package with shipsmoke itself).

| Platform | Workflow / Job | Run URL | Result |
|---|---|---|---|
| ubuntu-24.04 | Test + Self-verify | https://github.com/geyutu6755/shipsmoke/actions/runs/41225707854 | PASS |
| macos-15 (arm64) | Test + Self-verify | https://github.com/geyutu6755/shipsmoke/actions/runs/41225707854 | PASS |
| windows-2025 | Test + Self-verify | https://github.com/geyutu6755/shipsmoke/actions/runs/41225707854 | PASS |
| all three (repeat after docs) | full CI | https://github.com/geyutu6755/shipsmoke/actions/runs/41226190065 | PASS |

Notable real bugs that only surfaced through this cross-platform CI, all fixed
and re-verified:

1. `kill_unix.go` unused import — invisible on Windows builds (file excluded
   by build tag); caught by linux/macos vet.
2. `go build -o` does not append `.exe` on Windows — self-verify packaging
   failed on windows-2025 until binary names were made explicit.
3. `....//evil.txt` is contained on Unix (literal directory named `....`) —
   test expectation corrected to a true traversal (`....//../evil.txt`).
4. Zip entries without the exec bit (mode 0) produce Unix binaries that
   cannot run after extraction — kept as a genuine FAIL (real packaging
   defect), fixtures corrected.
5. **GO-2026-4869** (archive/tar unbounded allocation for old GNU sparse
   entries), reachable from `ExtractTarGz` via `tar.Reader.Next` — caught by
   the govulncheck gate; fixed by requiring go >= 1.26.2 in go.mod (the
   extractor also rejects non-regular tar entry types before reading).

## 4. Self-hosting verification

The `Self-verify artifact` jobs in the CI run above package shipsmoke on each
real runner (zip on Windows, tar.gz on Linux/macOS) and verify the package
with shipsmoke itself:

| Artifact | Platform (job) | Result |
|---|---|---|
| shipsmoke_windows_amd64.zip | windows-2025 | PASS |
| shipsmoke_linux_amd64.tar.gz | ubuntu-24.04 | PASS |
| shipsmoke_darwin_arm64.tar.gz | macos-15 | PASS |

The release workflow repeats this gate per target before publishing
(host-matching targets execute; cross targets are skipped with an explicit
log line — never faked).

## 5. Examples verification (three tech stacks)

Each example workflow builds shipsmoke from the checkout, builds and packages
the example tool with its native toolchain, and runs `shipsmoke check` — on
ubuntu-24.04, macos-15 (arm64), and windows-2025:

| Example | CI run URL | Result |
|---|---|---|
| examples/go-cli | https://github.com/geyutu6755/shipsmoke/actions/runs/41226190152 | PASS |
| examples/rust-cli | https://github.com/geyutu6755/shipsmoke/actions/runs/41226190149 | PASS |
| examples/c-cli | https://github.com/geyutu6755/shipsmoke/actions/runs/41226190111 | PASS |

## 6. Release verification

| Item | Evidence |
|---|---|
| v1.0.0 tag pushed | commit SHA recorded post-push |
| Release published | https://github.com/geyutu6755/shipsmoke/releases/tag/v1.0.0 |
| 6 platform binaries + checksums.txt attached | release assets |
| checksums.txt verified against binaries | sha256 -c output recorded post-release |
| Release binary downloaded and run on matching OS | recorded post-release |

## 7. Final acceptance conclusion

(recorded at P9)
