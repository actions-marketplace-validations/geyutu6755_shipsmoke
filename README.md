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

shipsmoke v1.0.0 — verification report
host: ubuntu-24.04/amd64   config: .shipsmoke.yml   duration: 912ms

  mytool-linux-amd64  format=tar.gz (detected: linux/amd64)
    [PASS] check #1: mytool --version (118ms)
    [PASS] check #2: mytool --help (104ms)
  [SKIP] mytool-windows-amd64 — artifact targets [windows/amd64] but host is linux/amd64

  summary: 2 passed, 0 failed, 1 skipped, 0 errored  →  PASS

json report: shipsmoke-report.json
```

*(Real terminal output from the self-verification CI run.)*

## Try it in one minute

```bash
# install (see all options below), then inside any Go CLI project:
go build -o dist/mytool .                              # build
tar -czf dist/mytool.tar.gz -C dist mytool             # package (zip on Windows)
shipsmoke init --ci --stack go --name mytool           # generate .shipsmoke.yml + 3-platform CI
shipsmoke check                                        # verify the artifact really runs
```

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

The same gate, running from an **independent project** (not this repo's own
tests), on all three operating systems:
[shipsmoke-integration](https://github.com/geyutu6755/shipsmoke-integration/actions)
— including jobs that corrupt an artifact on purpose and require the gate to
fail ([latest run](https://github.com/geyutu6755/shipsmoke-integration/actions/runs/41266540816)).

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
# Linux / macOS example
curl -fsSL -o shipsmoke.tar.gz \
  https://github.com/geyutu6755/shipsmoke/releases/latest/download/shipsmoke_linux_amd64.tar.gz
tar -xzf shipsmoke.tar.gz && ./shipsmoke version
```

**With Go:**

```bash
go install github.com/geyutu6755/shipsmoke/cmd/shipsmoke@latest
```

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
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
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
          version: v1.0.3
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
shipsmoke validate   validate the config without executing anything
shipsmoke init       write an example .shipsmoke.yml
shipsmoke version    version, commit, platform
```

`shipsmoke check --help` documents every flag.

## Documentation

- [Configuration reference](docs/CONFIGURATION.md) — full schema, matchers, CI recipes
- [System design](docs/SYSTEM_DESIGN.md) — architecture and data model
- [Threat model](docs/THREAT_MODEL.md) — what is protected and what is not
- [Limitations](docs/LIMITATIONS.md) — supported formats and honest boundaries
- [Troubleshooting](docs/TROUBLESHOOTING.md) — common failure diagnoses
- [Research](docs/RESEARCH.md) — competitive landscape and name check
- [Validation report](docs/VALIDATION_REPORT.md) — every claim, backed by CI run links
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
