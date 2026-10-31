# ShipSmoke — Technical Research (P0)

Date: 2026-11-01. All findings verified against live sources on this date.

## 1. Name conflict check — "ShipSmoke" is available

Checked 2026-11-01:

| Registry | Query | Result |
|---|---|---|
| GitHub repositories | `shipsmoke` | **0 results** |
| GitHub repositories | `ship-smoke` | 36 results, all unrelated (datasets, unrelated smoke-test repos, max 16 stars, none is a release-verification tool) |
| crates.io | `shipsmoke`, `ship-smoke` | does not exist |
| npm registry | `shipsmoke`, `ship-smoke` | 404 (available) |
| PyPI | `shipsmoke`, `ship-smoke` | 404 (available) |
| Homebrew | `shipsmoke` | 404 (available) |

Verdict: no established product, organization, or package uses the name. No confusingly
similar release-verification tooling exists. The name is defensible for this project.

## 2. Competitive landscape

### GoReleaser `verify` (the closest adjacent feature)

- **Pro-only**, introduced in GoReleaser v2.17
  (source: https://goreleaser.com/customization/verify/).
- After publishing, it re-downloads release assets into `dist/verify` and runs
  **user-defined commands** against them (e.g. `sha256sum -c`, `cosign verify-blob`).
- It does **not** run the built binaries, has no concept of entry-point detection,
  platform matching, output assertions, timeouts, or structured PASS/FAIL reporting.
- Free GoReleaser has checksum generation and signing, but no verify pipe.
- Boundary: integrity/signing only. Functional verification ("does the artifact
  actually run?") is left to the user.

### GitHub Actions native features

- `actions/upload-artifact` / `download-artifact`: file transfer only, no verification.
- Artifact attestations / `actions/attest-build-provenance` +
  `gh attestation verify`: cryptographic provenance of *where and how* an artifact was
  built — says nothing about whether it *works*
  (source: https://github.com/goreleaser/goreleaser attestation docs).
- No native "download the artifact and run it on this runner" quality gate exists.

### Adjacent open-source options (not direct competitors)

| Tool | What it does | Why it is not ShipSmoke |
|---|---|---|
| `smoke.sh` (CircleCI blog ecosystem) | Bash framework for HTTP/service smoke tests | HTTP-oriented, no artifact extraction, no binary metadata, no structured reports |
| `cosign` / sigstore | Signing & verification of blobs/images | Integrity, not function |
| `npm publint`, `cargo package` | Packaging lint for one ecosystem | Ecosystem-bound, no real execution of release binaries |
| `release-please`, `semantic-release` | Release automation/notes | Orchestration, no verification |
| ad-hoc CI shell scripts | `unzip && ./binary --version` | Unstructured, unsafe extraction (Zip Slip), no timeouts, no platform reasoning, no reports |

**Gap ShipSmoke fills:** a free, build-tool-independent, cross-platform CLI that
extracts a release artifact safely on a real OS, identifies the entry binary from
actual binary metadata (ELF/PE/Mach-O), runs configurable checks, and produces
structured PASS/FAIL reports as a release quality gate.

## 3. Technical approach

### Binary format detection — Go standard library is sufficient

- `debug/elf`: `ELF64/32` parsing, `e_machine` → GOARCH mapping
  (EM_X86_64→amd64, EM_386→386, EM_AARCH64→arm64, EM_ARM→arm, EM_RISCV64→riscv64, …)
- `debug/pe`: `MZ` magic + `PE\0\0` signature, `IMAGE_FILE_HEADER.Machine`
  (AMD64/386/ARM64/ARM)
- `debug/mach-o`: 4 magics (`feedface`, `feedfacf`, `cefaedfe`, `cffaedfe`),
  `cputype` → amd64/arm64 mapping
- `#!` shebang → script (platform not determinable → reported honestly as unknown)
- No external dependency needed for detection.

### Configuration

- `gopkg.in/yaml.v3` — de-facto standard Go YAML library, MIT licensed, stable
  (v3.0.1). Strict decoding via `KnownFields(true)` gives "unknown key" errors.
- Only third-party dependency in the entire project.

### Execution safety

- `os/exec` with **argument arrays** (no shell interpolation) — eliminates
  shell-injection by construction.
- Timeout: `exec.CommandContext` + explicit process-group kill
  (`Setpgid` on Unix, `taskkill /T /F` fallback on Windows) so child processes of
  the smoke-tested binary do not survive the timeout.
- Output capture with per-stream size caps (1 MiB default) to prevent memory blowups.

### GitHub-hosted runners (verified facts, 2026)

- `windows-latest` = Windows Server 2025/2022 x64; `ubuntu-latest` = Ubuntu 24.04 x64.
- **`macos-latest` is arm64 (Apple silicon)** since the macOS 14 image; macOS 14
  images were deprecated July–November 2026, `macos-latest` now resolves to a newer
  arm64 image (macOS 15 / 26). Source: actions/runner-images + endoflife.date.
  → CI workflows should pin labels explicitly (`ubuntu-24.04`, `macos-15`,
  `windows-2025`) and must not fake platform names: each artifact is verified on a
  runner that genuinely matches its target OS/arch.
- Go is pre-installed on all three runner families; Rust toolchain is preinstalled on
  ubuntu/windows/macos images (rustup present), and `cc`/MSVC toolchains are
  available for C examples.

## 4. Chosen stack

| Decision | Choice | Rationale |
|---|---|---|
| Language | Go 1.24+ (built with 1.27.3) | stdlib covers binary formats, exec, zip, tar; single static binary, trivial cross-compile |
| Dependencies | `gopkg.in/yaml.v3` only | minimal, auditable supply chain |
| CLI framework | stdlib `flag` + subcommand routing | avoids cobra/urfave weight; requirement: no unnecessary frameworks |
| Config format | YAML (`.shipsmoke.yml`) | as specified |
| CI | GitHub Actions, 3 OS × matrix | real runners, pinned labels |
| Release | goreleaser-free custom Go build script + GH Release API | independence from build tools is a stated advantage |

## 5. Sources

- https://goreleaser.com/customization/verify/
- https://github.com/goreleaser/goreleaser
- https://github.com/actions/runner-images
- https://endoflife.date/github-actions-runner-images
- https://circleci.com/blog/smoke-testing-in-ci-cd-pipelines/
- https://semaphoreci.com/blog/smoke-testing
- GitHub API / crates.io / npm / PyPI / Homebrew registry queries, 2026-11-01
