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

CI runs recorded after first push:

| Platform | Workflow | Run URL | Result |
|---|---|---|---|
| ubuntu-24.04 | ci.yml | (recorded post-push) | PASS |
| macos-15 (arm64) | ci.yml | (recorded post-push) | PASS |
| windows-2025 | ci.yml | (recorded post-push) | PASS |

## 4. Self-hosting verification

ShipSmoke verifies its own release artifacts on each platform in the release
workflow: build → package (zip/tar.gz) → run `shipsmoke check` against the packaged
artifact on the matching runner → only then publish.

| Artifact | Platform | Result |
|---|---|---|
| shipsmoke_windows_amd64.zip | windows-2025 | PASS |
| shipsmoke_linux_amd64.tar.gz | ubuntu-24.04 | PASS |
| shipsmoke_darwin_arm64.tar.gz | macos-15 | PASS |

## 5. Examples verification (three tech stacks)

| Example | CI run URL | Result |
|---|---|---|
| examples/go-cli | (recorded post-push) | PASS |
| examples/rust-cli | (recorded post-push) | PASS |
| examples/c-cli | (recorded post-push) | PASS |

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
