# Limitations

Honest boundaries of ShipSmoke v1.0.0. Nothing here is hidden behind a
flag — these are scope decisions with reasons.

## Artifact formats

| Format | Status | Notes |
|---|---|---|
| ZIP | supported | including archives written with backslash separators (PowerShell Compress-Archive), which are normalized like Windows Explorer does |
| tar.gz / tgz | supported | plain uncompressed `.tar` is rejected with an explicit error |
| Standalone executables | supported | ELF, PE, Mach-O, and shebang scripts |
| ELF | identified | GOOS-equivalent assumed linux |
| PE | identified | windows |
| Mach-O | identified | darwin |
| `.deb`, `.rpm`, `.msi`, `.dmg`, `.pkg`, `.apk`, `.snap`, `.flatpak` | not supported (roadmap) | package-manager integration is a different problem domain |
| AppImage | not supported | depends on FUSE and runtime conventions |
| Container images | not supported | out of scope; use cosign/trivy |
| WASM | not supported | no host execution path |

## Detection

- Identification uses real file headers, never filenames. A binary whose
  header cannot be parsed is reported as **unknown** and the report says so;
  it still runs when the platform gate matches.
- Shebang scripts run, but their platform cannot be determined from the
  shebang; the report states this instead of guessing.
- Packed/obfuscated executables may defeat header parsing; ShipSmoke prefers
  "unknown" over a wrong answer.

## Execution

- The verified program runs with the invoking user's permissions. ShipSmoke
  isolates the working directory, the environment, and the process lifetime —
  **it is not a syscall sandbox** (no seccomp, no Job Objects, no VM).
- No network egress control on the verified program.
- stdout/stderr capture is capped at 1 MiB per stream; a program that floods
  output is still terminated by its timeout, and the report notes truncation.
- On timeout the process tree is killed (process group on Unix, `taskkill /T`
  on Windows). Grandchildren that detach and re-parent may still survive on
  Windows; on Unix the process-group kill covers them.
- Execution coverage of the six release artifacts (v1.0.2 release pipeline):
  linux/amd64, darwin/arm64, windows/amd64 run **natively** on their matching
  runners; linux/arm64 runs **in a QEMU arm64 container** (ubuntu:24.04);
  darwin/amd64 runs **under Rosetta 2**; windows/arm64 is **build-only** —
  no execution path exists on an x64 GitHub-hosted runner, and the release
  job summary records it as such rather than claiming a run.

## Archives

- Symbolic links and hard links inside artifacts are rejected, not recreated
  (they are a common archive-attack vector; a real release rarely needs them).
- Devices and FIFOs are rejected.
- Extraction limits are configurable but always on: entry count, per-file
  size, total size, compression ratio.
- **Documented leniency (Go standard library behavior):** for a tar.gz whose
  tar payload is structurally complete, corrupted bytes *inside file
  contents* do not fail extraction — the tar reader stops at the zero end
  blocks and the gzip member CRC is never reached. Truncated archives, broken
  gzip headers and structurally broken tars ARE rejected. This is why the
  checksum verification of a downloaded artifact (cosign/attestations or a
  published checksums.txt) remains the tool for content integrity, and why
  the shipsmoke action verifies SHA-256 before executing anything.

## Environment

- `env: minimal` provides PATH (extraction dir first), HOME, LANG/LC_ALL/TZ,
  TMPDIR, and Windows runtime essentials (SystemRoot, TEMP, PATHEXT, …).
  A program needing exotic variables needs `env_extra` or `env: inherit`.
- `env: inherit` never passes variables whose names look secret-shaped
  (`*TOKEN*`, `*SECRET*`, `*PASSWORD*`, `*KEY*`, `*CREDENTIAL*`).

## CI integration

- The composite action downloads a shipsmoke release or uses `binary-path`;
  it does not build from source (keep the quality gate dependency-free).
- Post-release re-download verification is a documented recipe rather than
  deep action support in v1.0; the build-then-publish gate is fully
  implemented.

## Platform notes

- Windows: file modes are not emulated; executable-ness is the PE header's
  business. macOS runners on GitHub are arm64 (Apple silicon); darwin/amd64
  binaries are verified only if you add an Intel runner or cross-run locally.
