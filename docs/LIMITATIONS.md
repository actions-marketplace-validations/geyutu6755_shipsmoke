# Limitations

Honest boundaries of ShipSmoke v1.0.3 (and the `fix/v1.0.4-reliability`
branch). Nothing here is hidden behind a flag — these are scope decisions with
reasons.

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
- **gzip member integrity is verified since v1.0.3** (fixed leniency): the
  extractor drains the full gzip member after the tar payload ends, so the
  CRC32/ISIZE trailer is always checked — corrupted payload bytes, corrupt
  CRC fields and truncated trailers now fail extraction. Two accepted
  leniencies remain: (1) trailing bytes *after* a complete gzip member are
  ignored (the payload itself is complete and verified), and (2) archive
  integrity cannot tell whether an executable's content is functionally
  correct — that is layer 3's job. See
  [VERIFICATION_LAYERS.md](VERIFICATION_LAYERS.md) for the three-layer
  model.

## Environment

- `env: minimal` provides PATH (extraction dir first), HOME, LANG/LC_ALL/TZ,
  TMPDIR, and Windows runtime essentials (SystemRoot, TEMP, PATHEXT, …).
  A program needing exotic variables needs `env_extra` or `env: inherit`.
- `env: inherit` never passes variables whose names look secret-shaped
  (`*TOKEN*`, `*SECRET*`, `*PASSWORD*`, `*KEY*`, `*CREDENTIAL*`).

## CI integration

- The composite action downloads a shipsmoke release or uses `binary-path`;
  it does not build from source (keep the quality gate dependency-free).
- Post-release re-download verification is now a **generated workflow**
  (`shipsmoke init --ci --published`), not a hand-copied recipe. What it still
  is *not*: it cannot block a publish (it runs after one, and reports), it needs
  the release asset names supplied per platform, and it verifies that the
  artifact extracts and runs — not that its contents are functionally correct.
  The build-then-publish gate remains the fence; this gate is the alarm.

## Platform notes

- Windows: file modes are not emulated; executable-ness is the PE header's
  business. macOS runners on GitHub are arm64 (Apple silicon); darwin/amd64
  binaries are verified only if you add an Intel runner or cross-run locally.
