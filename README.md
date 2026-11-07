# ShipSmoke

**Test what users actually download.**

[![CI](https://github.com/geyutu6755/shipsmoke/actions/workflows/ci.yml/badge.svg)](https://github.com/geyutu6755/shipsmoke/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/geyutu6755/shipsmoke)](https://github.com/geyutu6755/shipsmoke/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/geyutu6755/shipsmoke.svg)](https://pkg.go.dev/github.com/geyutu6755/shipsmoke)

文档语言：[简体中文](README.zh-CN.md)

ShipSmoke is a free, open-source, cross-platform CLI that verifies your
release artifacts — **ZIP**, **tar.gz**, and standalone binaries — actually
extract, launch, and run on the operating system they target. It is a release
quality gate: a broken artifact fails your CI before your users find out.

```console
$ shipsmoke check

shipsmoke v1.0.3 — verification report
host: windows/amd64   config: .shipsmoke.yml   duration: 822ms

  [SKIP] gokit-unix — artifact targets [linux/amd64, linux/arm64, darwin/amd64, darwin/arm64] but host is windows/amd64
  gokit-windows  format=zip (detected: windows/amd64)
    [PASS] check #1: gokit.exe --version (475ms)

  summary: 1 passed, 0 failed, 1 skipped, 0 errored  →  PASS

json report: dist/shipsmoke-report.json
```

*(Real output of the released v1.0.3 binary verifying
[`examples/go-cli`](examples/go-cli) on a Windows machine. The SKIP is the
point: an artifact that does not target this OS is reported as unverified
rather than counted as a pass. On Linux and macOS runners the same config runs
the tar.gz and skips the zip.)*

## Try it in one minute

```bash
# install (see all options below), then inside any Go CLI project:
mkdir -p dist
go build -o dist/mytool .                              # build
tar -czf dist/mytool.tar.gz -C dist mytool             # package (Linux/macOS)
shipsmoke init --ci --stack go --name mytool           # generate .shipsmoke.yml + 3-platform CI
shipsmoke check                                        # verify the artifact really runs
```

Windows (PowerShell) — the generated config expects `dist/mytool.zip` with
`mytool.exe` inside, so package with:

```powershell
go build -o dist\mytool.exe .
Compress-Archive -Path dist\mytool.exe -DestinationPath dist\mytool.zip
shipsmoke check
```

The generated config asserts `mytool --version` exits 0 and prints `mytool`, so
if your CLI spells its version differently, edit `.shipsmoke.yml` (or run
`shipsmoke check` first and read the failure). Nothing about ShipSmoke assumes a
language — it runs the bytes and checks what comes back.

## See it block a broken release

Reproduce at any time: [`examples/demo-broken-release.sh`](examples/demo-broken-release.sh)
builds a tiny CLI, verifies it, truncates the archive like a broken upload,
and re-verifies — real output from a Windows machine:

```text
== 2. verify the GOOD artifact ==
  demo  format=zip (detected: windows/amd64)
    [PASS] check #1: demo.exe --version (166ms)
  summary: 1 passed, 0 failed, 0 skipped, 0 errored  →  PASS

== 3. break the artifact the way a broken upload would (truncate) ==
truncated to 741624 of 1483249 bytes

== 4. verify the BROKEN artifact: the gate must fail ==
  [ERROR] demo — archive could not be extracted safely: not a readable zip archive: zip: not a valid zip file
  summary: 0 passed, 0 failed, 0 skipped, 1 errored  →  FAIL

== 5. publish step would be skipped (verify exited 1) ==
```

The same gate, driven from a **separate repository** written as an outside
consumer of the action:
[shipsmoke-integration](https://github.com/geyutu6755/shipsmoke-integration/actions)
— three operating systems plus jobs that corrupt an artifact on purpose and
require the gate to fail. It consumes the published `v1` tag rather than this
repo's sources, so it proves the onboarding path works. It is still the same
maintainer's repository, which under the tier model in
[docs/ADOPTION.md](docs/ADOPTION.md) is **Tier 2 — not third-party adoption**.
ShipSmoke has no Tier 3 adopters yet; the README says so plainly rather than
implying otherwise.

## Why ShipSmoke

Checksums tell you an upload was not corrupted in transit. They do not tell
you the binary inside works. ShipSmoke complements tools like GoReleaser
(whose `verify` feature is Pro-only and runs your own commands — it does not
run your binaries), cosign (signing), and GitHub artifact attestations
(provenance):

- **Real execution on a real OS** — extracts the artifact, finds the entry
  binary, and runs configurable checks (exit codes, stdout/stderr matching,
  timeouts) as child processes. No emulated platform names: each check runs on
  a GitHub-hosted runner whose OS/arch genuinely matches the artifact.
- **Binary metadata, not filename guesses** — entry binaries are identified by
  parsing actual ELF / PE / Mach-O headers (via the Go standard library).
- **Safe by construction** — Zip Slip protection, archive size/count/ratio
  limits, symlink/hardlink rejection, argv-array execution (never a shell),
  process-tree kill on timeout, and secret redaction in every report.
- **Independent and free** — no build-tool lock-in, no paid API, no cloud, no
  account. One static binary; its only dependency is a YAML parser.
- **CI-native** — clear exit codes, machine-readable JSON, and a GitHub
  Actions job summary out of the box.

## Install

**Download (recommended):** grab the archive for your platform from
[Releases](https://github.com/geyutu6755/shipsmoke/releases), extract, done.

```bash
# Linux x86-64
curl -fsSL -o shipsmoke.tar.gz \
  https://github.com/geyutu6755/shipsmoke/releases/latest/download/shipsmoke_linux_amd64.tar.gz
tar -xzf shipsmoke.tar.gz && ./shipsmoke version

# Linux ARM64
curl -fsSL -o shipsmoke.tar.gz \
  https://github.com/geyutu6755/shipsmoke/releases/latest/download/shipsmoke_linux_arm64.tar.gz
tar -xzf shipsmoke.tar.gz && ./shipsmoke version

# macOS Apple Silicon
curl -fsSL -o shipsmoke.tar.gz \
  https://github.com/geyutu6755/shipsmoke/releases/latest/download/shipsmoke_darwin_arm64.tar.gz
tar -xzf shipsmoke.tar.gz && ./shipsmoke version

# macOS Intel
curl -fsSL -o shipsmoke.tar.gz \
  https://github.com/geyutu6755/shipsmoke/releases/latest/download/shipsmoke_darwin_amd64.tar.gz
tar -xzf shipsmoke.tar.gz && ./shipsmoke version
```

```powershell
# Windows x86-64 (PowerShell)
curl.exe -fsSL -o shipsmoke.zip `
  https://github.com/geyutu6755/shipsmoke/releases/latest/download/shipsmoke_windows_amd64.zip
Expand-Archive shipsmoke.zip -DestinationPath . && .\shipsmoke.exe version
```

**Package managers:**

```bash
# Homebrew (macOS/Linux)
brew install geyutu6755/tap/shipsmoke

# Scoop (Windows)
scoop bucket add geyutu6755 https://github.com/geyutu6755/scoop-bucket
scoop install shipsmoke

# Go
go install github.com/geyutu6755/shipsmoke/cmd/shipsmoke@latest
```

The tap and bucket are re-installed and tested in CI on every push
([tap runs](https://github.com/geyutu6755/homebrew-tap/actions),
[bucket runs](https://github.com/geyutu6755/scoop-bucket/actions)).

Every released archive is packaged and then verified by ShipSmoke itself on a
real runner before publishing (see the release workflow).

## Quick start

```bash
cd your-project
shipsmoke init        # writes a commented .shipsmoke.yml
shipsmoke validate    # schema + semantic checks, no execution
shipsmoke check       # the quality gate; exit code drives your CI
```

A complete `.shipsmoke.yml`:

```yaml
version: 1

report:
  json: shipsmoke-report.json        # machine-readable report (optional)

artifacts:
  - name: mytool-linux-amd64
    path: dist/mytool-linux-amd64.tar.gz
    format: auto                     # auto | zip | tar.gz | binary
    binary: mytool                   # entry program, relative to archive root
    runs_on: [linux/amd64]           # verify only where it actually runs
    checks:
      - args: ["--version"]
        expect:
          exit_code: 0
          stdout_contains: ["mytool version"]
        timeout: 30s

  - name: mytool-windows-amd64
    path: dist/mytool-windows-amd64.zip
    binary: mytool.exe
    runs_on: [windows/amd64]
    checks:
      - args: ["--version"]
        expect: { exit_code: 0 }
```

On a `linux/amd64` runner the first artifact is extracted and executed; the
second is reported as **SKIP** with the reason. Nothing is ever reported as
passing without having run.

Full schema and all matchers (`stdout_contains`, `stdout_not_contains`,
`equals`, `regex`, `stderr_*`): [docs/CONFIGURATION.md](docs/CONFIGURATION.md).

## Use in GitHub Actions

Generate a complete, pre-verified setup for your project:

```console
$ shipsmoke init --ci --stack go --name mytool
generated files for the go stack (tool "mytool"):
  .shipsmoke.yml          — verification config
  .github/workflows/shipsmoke.yml — three-platform CI: build, package, shipsmoke gate
```

Supported stacks are exactly `go`, `rust`, `c` (ShipSmoke does not infer
build systems). The generated workflow is the same shape proven in
[shipsmoke-integration](https://github.com/geyutu6755/shipsmoke-integration):
build → package (`tar.gz` on Linux/macOS, `zip` on Windows) → `shipsmoke`
verify on ubuntu-24.04, macos-15 and windows-2025.

The hand-written equivalent for a Go CLI — copy-paste ready, with the build
and verify steps inline (no helper scripts):

```yaml
name: shipsmoke
on:
  push:
    branches: [main]
  pull_request:
permissions:
  contents: read
jobs:
  verify:
    name: Verify release artifact (${{ matrix.os }})
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-24.04, macos-15, windows-2025]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262 # v4
      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5
        with:
          go-version: 'stable'
      - name: Build and package
        shell: bash
        run: |
          set -euo pipefail
          EXE=""
          [ "$RUNNER_OS" = "Windows" ] && EXE=".exe"
          mkdir -p dist
          go build -o "dist/mytool$EXE" .
          if [ "$RUNNER_OS" = "Windows" ]; then
            powershell -NoProfile -Command "Compress-Archive -Path dist/mytool.exe -DestinationPath dist/mytool.zip -Force"
          else
            tar -czf dist/mytool.tar.gz -C dist mytool
          fi
      - name: Verify artifact with shipsmoke
        uses: geyutu6755/shipsmoke@v1
        with:
          config: .shipsmoke.yml
          report: dist/shipsmoke-report.json
          version: v1.0.4
      - name: Publish only if verification passed
        if: success()
        run: echo "all artifacts verified — attach dist/ to your release here"
```

A failing check fails the verify step (exit code 1), which blocks anything
that depends on it. The action writes a job summary table and uploads the
JSON report. Full tutorials: [docs/TUTORIAL_GO.md](docs/TUTORIAL_GO.md),
[docs/TUTORIAL_RUST.md](docs/TUTORIAL_RUST.md),
[docs/TUTORIAL_C.md](docs/TUTORIAL_C.md).

**Action security model (v1.0.2+):**

- User inputs reach shell code only through environment variables — no
  `${{ inputs.* }}` is interpolated inside any `run:` block, so hostile
  input values cannot execute as script injection (covered by an automated
  injection-regression test on all three platforms).
- When the action downloads a release, the asset is **SHA-256 verified
  against the release's `checksums.txt` before extraction**; a failed
  download, a missing checksum entry, or a hash mismatch aborts the job
  before the binary is ever executed. `verified: checksum` / `verified:
  user-provided` is exposed as a step output.
- `version:` accepts `latest` or a full tag `vX.Y.Z`; moving refs like `v1`
  are rejected in the action (use them in `uses:` to select the action
  itself, and pin the tool release separately). Pinning both — e.g.
  `uses: geyutu6755/shipsmoke@v1.0.2` with `version: v1.0.2` — gives a fully
  reproducible gate.

Two integration patterns:

1. **Verify after build, before publish** (fully supported) — build in the
   workflow, package, run ShipSmoke, publish only on PASS.
2. **Post-release re-download verification** (documented recipe) — after the
   release is out, download the published asset and run ShipSmoke on it. See
   [docs/CONFIGURATION.md](docs/CONFIGURATION.md#post-release-verification).

The action itself is exercised from an independent repository on all three
operating systems (see
[shipsmoke-integration](https://github.com/geyutu6755/shipsmoke-integration)),
not only by this repo's self-checks.

See [examples/](examples/) for complete, CI-verified Go, Rust, and C projects
using exactly this pattern.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | all executed checks passed |
| 1 | at least one check failed (or an artifact could not be verified) |
| 2 | configuration or usage error (invalid YAML, unknown key, missing config) |
| 3 | nothing was executed — all checks were skipped; CI should not go green (`--allow-empty` overrides) |

Statuses: **PASS** (ran, expectations met), **FAIL** (ran, expectations not
met), **SKIP** (not run — artifact targets another platform; always labeled
with the reason), **ERROR** (infrastructure problem: missing file, unsafe
archive, spawn failure).

## Security

ShipSmoke executes programs. It hardens everything it controls — path
traversal (Zip Slip) defense with a double validation layer, streaming
archive limits (entry count, per-file size, total size, compression ratio),
explicit rejection of symlinks/hardlinks/devices, argv-array process launch
(no shell, no injection), hard timeout with process-tree kill, capped output
capture, and redaction of secret-shaped environment values from all reports —
but the verified binary runs with the permissions of your workflow. **It is
not an operating-system sandbox.** Verify your own build artifacts; for
untrusted third-party artifacts use ephemeral runners without deployment
secrets. Details: [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md),
[SECURITY.md](SECURITY.md).

## Commands

```
shipsmoke check      run verification (the quality gate)
  --config PATH      config path (default ./.shipsmoke.yml)
  --artifact NAME    verify a single named artifact
  --report PATH      JSON report path (overrides config)
  --allow-empty      exit 0 when nothing executed (default exit 3)
  --no-color         disable colored output
  --quiet            failures, skips and the summary only
shipsmoke validate   validate the config without executing anything
  --config PATH      config path (default ./.shipsmoke.yml)
shipsmoke init       write an example .shipsmoke.yml
  --force            overwrite files that already exist
  --ci               also generate a GitHub Actions workflow
  --stack go|rust|c  with --ci: which build stack to emit (required with --ci)
  --name TOOL        with --ci: your binary name (default: current directory)
  --published        with --ci: the post-release gate instead of a build gate
  --asset-linux NAME      with --published: release asset for linux/amd64
  --asset-darwin NAME     with --published: release asset for darwin/arm64
  --asset-windows NAME    with --published: release asset for windows/amd64
shipsmoke version    version, commit, platform
```

`shipsmoke <command> --help` prints this list with each flag's own description,
straight from the code that parses it.

Two CI shapes, deliberately different:

| | what it does | what it proves |
|---|---|---|
| `init --ci --stack …` | build → package → verify → publish | the artifact you are **about** to publish runs |
| `init --ci --published` | download the published asset → run it | the artifact users **actually got** runs |

The first blocks a bad release; the second detects one. A project with an
existing release pipeline can only be served by the second.

## Documentation

- [One-minute quickstart](#try-it-in-one-minute) and [tutorials](docs/TUTORIAL_GO.md) (Go / [Rust](docs/TUTORIAL_RUST.md) / [C](docs/TUTORIAL_C.md))
- [Verification layers](docs/VERIFICATION_LAYERS.md) — download-integrity vs archive-integrity vs real-run
- [Configuration reference](docs/CONFIGURATION.md) — full schema, matchers, CI recipes
- [System design](docs/SYSTEM_DESIGN.md) — architecture and data model
- [Threat model](docs/THREAT_MODEL.md) — what is protected and what is not
- [Limitations](docs/LIMITATIONS.md) — supported formats and honest boundaries
- [Troubleshooting](docs/TROUBLESHOOTING.md) — common failure diagnoses
- [Supply-chain boundaries](docs/SUPPLY_CHAIN.md) — what CI trusts and why
- [Case study](docs/CASE_STUDIES/git-graph-v0.8.3-macos-arch-mismatch.md) — a real macos-amd64/arch-mismatch found in the wild
- [Third-party integration template](docs/THIRD_PARTY_TEMPLATE.md) — copy-paste a gate for another project
- [Trial plan](docs/TRIAL_PLAN.md) · [Final acceptance](docs/FINAL_ACCEPTANCE.md) · [Maintenance guide](docs/MAINTENANCE_GUIDE.md)
- [Maintainer introduction](docs/INTRODUCTION.md) · [Candidate projects](docs/CANDIDATES.md) · [Adoption tracking](docs/ADOPTION.md)
- [Research](docs/RESEARCH.md) — competitive landscape and name check
- [Validation report](docs/VALIDATION_REPORT.md) — evidence per claim, with run links where a link exists
- [Development status](docs/DEVELOPMENT_STATUS.md) — phase-by-phase record

## Development

```bash
go test ./...           # unit + integration (fixture binaries are cross-compiled in-test)
go vet ./...
gofmt -l .
go test -fuzz FuzzExtractZip -fuzztime 30s ./internal/artifact/
go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...
```

Dependencies: `gopkg.in/yaml.v3` only. CI runs the full suite on
`ubuntu-24.04`, `macos-15` (arm64), and `windows-2025`.

## Known limitations

Plain `.tar` (uncompressed), Debian `.deb`, RPM, MSI, DMG, AppImage and
container images are out of scope for v1.0 (see
[docs/LIMITATIONS.md](docs/LIMITATIONS.md)). Symbolic links inside artifacts
are rejected rather than recreated. Script binaries are executed but their
platform cannot be identified from a shebang, and ShipSmoke says so honestly.

## License

[MIT](LICENSE) © 2026 ShipSmoke contributors
