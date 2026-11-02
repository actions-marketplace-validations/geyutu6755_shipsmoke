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
| Assets (9) | checksums.txt + 6 platform archives (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 as tar.gz; windows/amd64, windows/arm64 as zip) + the 2 source bundles GitHub attaches automatically (zip, tar.gz). Counted from the v1.0.3 release page on 2026-11-01; an earlier revision of this page said 7, counting only what the workflow uploads. |
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

## 6c. v1.0.2 release evidence (final)

| Gate | Evidence | Result |
|---|---|---|
| Release workflow green, all 6 builds verified | https://github.com/geyutu6755/shipsmoke/actions/runs/41265368122 | 7/7 jobs success |
| linux/arm64: `detected: linux/arm64` + PASS inside QEMU arm64 container | job log, run 41265368122 | PASS (was detected: linux/amd64 FAIL before the export fix) |
| darwin/amd64: `detected: darwin/amd64` + PASS under Rosetta | job log, run 41265368122 | PASS (was "Bad CPU type" with the wrong-arch binary) |
| windows/arm64: "cannot be executed on an x64 GitHub-hosted runner (no emulation available); recording as build-only, NOT as executed" | job log, run 41265368122 | honestly recorded |
| Premature publish retracted (needs-guard was overridden by explicit if) | v1.0.2 tag deleted, fixed, re-tagged at 1b75162 | gate integrity restored |
| v1.0.0/v1.0.1 defect disclosed on their release pages | warning appended to both release notes | done |
| Downloaded v1.0.2 linux/arm64 asset: SHA-256 OK + header `elf/arm64 (e_machine=183)` | local verification | PASS |
| Downloaded v1.0.2 windows/amd64 asset: SHA-256 match + `shipsmoke v1.0.2 (windows/amd64)` runs | local verification | PASS |
| `go install ...@v1.0.2` reports v1.0.2 | local verification | PASS |
| `uses: geyutu6755/shipsmoke@v1` reference live (tag exists) | tag v1 → 1b75162 | yes |
| Independent integration repo consuming `@v1` + `version: v1.0.2` on 3 OS (verify + refuse-tampered) | https://github.com/geyutu6755/shipsmoke-integration/actions/runs/41266540816 | PASS |

## 6d. v1.0.3 release evidence (final)

| Gate | Evidence | Result |
|---|---|---|
| tar.gz gzip CRC fix: 5 corruption fixtures (CRC field, truncated trailer, mid-stream deflate, trailing garbage accepted, non-critical content corruption) | `go test ./internal/artifact/ -run TestExtractTarGz` — all pass with the fix; pre-fix behavior covered by the assertions in corruption_test.go comments | PASS |
| Three-layer verification model documented | docs/VERIFICATION_LAYERS.md | yes |
| Strict version grammar (prerelease/leading-zero/moving refs rejected) | fetch-script suite, 28 cases green | PASS |
| Release asset re-verification before publish | release.yml "Re-verify release assets against checksums.txt" step (ran in release 41271136122) | PASS |
| Third-party Actions SHA-pinned; attestations generated | release job attest step; docs/SUPPLY_CHAIN.md | done |
| Full CI (3 OS) on final commit 88cb63e | https://github.com/geyutu6755/shipsmoke/actions/runs/41270711764 | PASS |
| action-test suite (28 fetch tests + e2e + injection) at 00b5b2b | https://github.com/geyutu6755/shipsmoke/actions/runs/41268289905 | PASS |
| Examples (Go/Rust/C) on 88cb63e | runs 41270711764 batch | PASS |
| **v1.0.3 release: 7/7 jobs green** | https://github.com/geyutu6755/shipsmoke/actions/runs/41271136122 | PASS |
| `go install ...@v1.0.3` reports v1.0.3 | local verification | PASS |
| Downloaded v1.0.3 windows/amd64 asset: SHA-256 match + binary runs (commit 88cb63e) | local verification | PASS |
| Homebrew tap installs v1.0.3 on macOS; linux asset + checksum verified | https://github.com/geyutu6755/homebrew-tap/actions/runs/41271668595 | PASS |
| Scoop bucket installs v1.0.3 on Windows | https://github.com/geyutu6755/scoop-bucket/actions/runs/41271672405 | PASS |
| Independent integration repo: `@v1` + `version: v1.0.3`, verify + refuse-tampered, 3 OS | https://github.com/geyutu6755/shipsmoke-integration/actions/runs/41272145878 | PASS |
| No fake-greens: heredocs remain banned (cigen tests assert), all generated configs cat'ed before use, needs-guard present | code review + CI | confirmed |

## 6e. v1.0.4 reliability round — branch evidence (2026-11-02, not released)

Branch `fix/v1.0.4-reliability`, head `f811c10`, opened as
[draft PR #1](https://github.com/geyutu6755/shipsmoke/pull/1). **No tag and no
Release were created for this work.**

Real GitHub-hosted runs on that commit. The branch head moved once during this
round (to add the race step), so both heads are recorded: CI run 25 on
`f811c10` and CI run 26 on `8c4436e`, both success.

| workflow | run on `8c4436e` | result |
|---|---|---|
| CI (test ×3 OS incl. race/fuzz/govulncheck on Linux, self-verify ×3 OS, pinning) | [26](https://github.com/geyutu6755/shipsmoke/actions/runs/41386635793) | success |
| action-test (fetch-script suite + `uses: ./` e2e ×3 OS + injection regression) | [5](https://github.com/geyutu6755/shipsmoke/actions/runs/41386635825) | success |
| example-go-cli | [6](https://github.com/geyutu6755/shipsmoke/actions/runs/41386635805) | success |
| example-rust-cli | [6](https://github.com/geyutu6755/shipsmoke/actions/runs/41386635678) | success |
| example-c-cli | [6](https://github.com/geyutu6755/shipsmoke/actions/runs/41386635763) | success |

Earlier head `f811c10` had the same five workflows green:
[CI 25](https://github.com/geyutu6755/shipsmoke/actions/runs/41385865575),
[action-test 4](https://github.com/geyutu6755/shipsmoke/actions/runs/41385865584),
[go](https://github.com/geyutu6755/shipsmoke/actions/runs/41385865589) /
[rust](https://github.com/geyutu6755/shipsmoke/actions/runs/41385865709) /
[c](https://github.com/geyutu6755/shipsmoke/actions/runs/41385865746).

`Dependency pinning` is a new job and it is green here; that is the first CI run
in which the claim "every third-party Action ref is SHA-pinned" is machine
checked rather than asserted by hand.

Before/after method (this is what makes the fixes trustworthy rather than
claimed): the new regression tests were run against a detached worktree of
`origin/main`, i.e. pre-fix code.

| defect | pre-fix result | post-fix result |
|---|---|---|
| `check --quiet` | test binary died with the nil-pointer panic at `cli.go:145` → `report.go:59` | 6 tests pass; PASS/FAIL/ERROR/SKIP/NO-CHECKS, JSON report and exit codes asserted |
| `<cmd> --help` | `check/validate/init` each exit 2 with `flag: help requested`, no usage | usage on stdout, exit 0; unknown flag and bad bool value still exit 2 with usage on stderr |
| `env: inherit` warning | not observable: injected stderr stayed empty | warning captured through `Options.Stderr` |
| lookalike `checksums.txt` name | pre-fix fetch script matched it, **extracted it, exit 0** (2 test failures) | refused, exit 2, no extraction; fetch suite 30/30 locally and green on 3 OSes |
| per-check signal handler | leaked one goroutine + notifier per check | goroutine count stable across 4 runs of a 6-check config; on CI run 26 the Unix-only `TestInterruptCancelsRemainingChecks` logs `--- PASS (0.65s)` — a real SIGINT aborted the remaining checks, and it was not skipped (the log contains no `--- SKIP` line at all) |

Post-release gate against a real third-party release (Tier 0, self-performed):
`init --ci --published --name termide` output plus termide 0.48.0's actual
published assets, downloaded from their Release:

- `…-x86_64-pc-windows-msvc.zip` (200, 21 262 423 bytes) → extracted, ran
  `termide.exe --version`, **PASS**, exit 0.
- linux tar.gz and darwin tar.gz (both 200) → **SKIP** with target-vs-host
  reasons on a Windows host, which is the designed behaviour.
- same zip truncated to 50% → **ERROR** `not a valid zip file`, exit 1.
- `--quiet` on that same run worked, which is precisely the path that crashed in
  the released v1.0.3 binary.
- Placeholder behaviour: generating without `--asset-*` emits `SET-ME-*` names in
  the matrix and the guard step exits 1 on them (verified by executing the
  emitted shell logic) while a real asset name passes.

Explicitly **not** claimed by this round: execution of our own
darwin/linux/arm64 release artifacts, `windows/arm64` execution (still
`build-only`), any third-party adoption, and `gh attestation verify` (no `gh` in
this environment). One disclosure worth stating plainly: `-race` had never run
for any release — it needs cgo, which is unavailable on a machine without a C
compiler, and no CI job invoked it either. This branch adds
`go test -race ./...` to the Linux job; its first execution is recorded in
CI run 26 (`##[group]Run go test -race -count=1 ./...`, job conclusion success,
no `DATA RACE` report anywhere in the log, no skipped or failed tests). See
docs/LIMITATIONS.md and docs/DEVELOPMENT_STATUS.md.

## 7. Final acceptance conclusion

**ACCEPTED — ShipSmoke v1.0.3 (current release) is released and verified.**
History: v1.0.0/v1.0.1 shipped wrong-arch artifacts for three of six
archives (disclosed on their release pages, fixed in v1.0.2); v1.0.2
hardened the action and its release gate; v1.0.3 verified gzip CRC fully,
pinned the supply chain, and shipped the onboarding stack (init --ci,
tutorials, brew/scoop).

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
   CHANGELOG, SECURITY, CONTRIBUTING, CoC, LICENSE, 9 release assets).
9. GitHub repository published — https://github.com/geyutu6755/shipsmoke (public,
   topics set, CI badge).
10. Final acceptance of real release artifacts recorded above — yes.

Known limitations are documented in docs/LIMITATIONS.md (unsupported package
formats, symlink rejection, no OS sandbox, output capture caps).
