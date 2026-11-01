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
| v1.0.0 tag pushed | tag on commit 9cda5f2 |
| Release workflow green (6 build jobs + publish) | https://github.com/geyutu6755/shipsmoke/actions/runs/41226680894 |
| Release published | https://github.com/geyutu6755/shipsmoke/releases/tag/v1.0.0 |
| v1.0.1 release (go-install version reporting fix) | https://github.com/geyutu6755/shipsmoke/actions/runs/41227510659 → https://github.com/geyutu6755/shipsmoke/releases/tag/v1.0.1 |
| go install path verified | `go install github.com/geyutu6755/shipsmoke/cmd/shipsmoke@v1.0.1` → binary reports `shipsmoke v1.0.1 (windows/amd64)` |
| Assets (7) | checksums.txt + 6 platform archives: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 (tar.gz), windows/amd64, windows/arm64 (zip) |
| Pre-publish execution gate | Build jobs verified host-matching artifacts with shipsmoke before the release job ran (cross targets skipped with explicit log line) |
| checksums.txt verified | downloaded locally; `sha256sum -c` → `shipsmoke_windows_amd64.zip: OK` |
| Release binary downloaded and run | `shipsmoke version` → `shipsmoke v1.0.0 (windows/amd64)`, commit 9cda5f2, built 2026-10-31T17:53:43Z |
| Released binary verifies its own released artifact | v1.0.0: 2 passed → PASS; v1.0.1: `sha256sum -c` OK, downloaded binary reports v1.0.1 and verifies its own zip → PASS, exit 0 |

## 6b. v1.0.2 security revision — disclosures and new evidence

**Disclosure (found while hardening v1.0.2):** v1.0.0/v1.0.1's CI
self-verify and release-verify steps generated their shipsmoke config with a
bash heredoc whose `EOF` terminator was indented inside the YAML block
scalar. Bash never closed such a heredoc, so the follow-up
`shipsmoke check` command was swallowed into the config file and **never
executed** — those CI steps were green without running the gate. The
artifacts themselves were verified manually at release time (see §6), but
the CI gate claim for v1.0.0/v1.0.1 is hereby corrected: their self-verify
and release-verify jobs did not run the check. v1.0.2 replaces all heredocs
with `printf` + explicit `cat` of the generated config, so a swallowed
command is visible and the check genuinely runs.

New v1.0.2 evidence (all real CI):

| Gate | Run | Result |
|---|---|---|
| Fetch-script security suite (injection/tamper/refusal, 23 cases) × 3 OS | https://github.com/geyutu6755/shipsmoke/actions/runs/41260802961 | PASS |
| Composite action e2e via `uses: ./` + report assertions × 3 OS | https://github.com/geyutu6755/shipsmoke/actions/runs/41260802961 | PASS |
| Injection regression through the real action entry × 3 OS | https://github.com/geyutu6755/shipsmoke/actions/runs/41260802961 | PASS (payload inert, step fails cleanly) |
| Real-download check of v1.0.1 assets incl. SHA-256 × 3 OS | https://github.com/geyutu6755/shipsmoke/actions/runs/41260802961 | PASS |
| Full CI after heredoc fix (printf configs) × 3 OS incl. self-verify actually running | https://github.com/geyutu6755/shipsmoke/actions/runs/41261797236 | PASS |
| Independent integration repo (outside project consumes the action; verify + refuse-tampered jobs, 3 OS each) | https://github.com/geyutu6755/shipsmoke-integration/actions | see §5b |

## 7. Final acceptance conclusion

**ACCEPTED — ShipSmoke v1.0.1 (current release) is released and verified.**
v1.0.0 shipped the complete feature set; v1.0.1 fixed version reporting
under `go install` and was verified with the same gates.

Checked against the completion criteria:

1. Core functionality implemented (check/validate/init/version, ZIP/tar.gz/standalone,
   real execution, reporting, Actions integration) — yes.
2. CLI usable via real installation (release download verified by SHA256 +
   execution; `go install` path documented) — yes.
3. Artifact detection and real execution verified — yes (CI + local e2e + release gate).
4. All mandated failure tests pass (F-01…F-12 in the suite; cross-platform
   failures exercised live during CI bring-up) — yes.
5. Windows / Linux / macOS CI real and green — yes (runs 41225707854, 41226190065).
6. Security review: govulncheck clean after fixing GO-2026-4869; threat model
   documented; fuzz targets in place; no unresolved release-blocking issues — yes.
7. Config and Actions examples reproducible — yes (examples green in CI;
   README/action.yml recipes).
8. Documentation and release assets complete — yes (README EN/中文, docs/,
   CHANGELOG, SECURITY, CONTRIBUTING, CoC, LICENSE, 7 release assets).
9. GitHub repository published — https://github.com/geyutu6755/shipsmoke (public,
   topics set, CI badge).
10. Final acceptance of real release artifacts recorded above — yes.

Known limitations are documented in docs/LIMITATIONS.md (unsupported package
formats, symlink rejection, no OS sandbox, output capture caps).
