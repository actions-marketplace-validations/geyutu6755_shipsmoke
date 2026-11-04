## ShipSmoke v1.0.4

**Test what users actually download.**

Reliability release: every defect below was reproduced against the released
v1.0.3 binary before it was fixed. Merged from
[PR #1](https://github.com/geyutu6755/shipsmoke/pull/1) with full history
(merge commit `91c0e62`).

### Fixed (reproduced on v1.0.3 first)

- **`shipsmoke check --quiet` crashed** with a nil-pointer dereference in the
  report redactor. Regression tests cover PASS/FAIL/ERROR/SKIP/NO-CHECKS,
  the JSON report and exit codes.
- **`shipsmoke <command> --help` printed `flag: help requested`** for check,
  validate and init. Help now goes to stdout with exit 0; invalid flags still
  print to stderr with exit 2. (Behaviour change: `--help` is now success.)
- **A signal handler and goroutine leaked per check**, and Ctrl+C aborted
  only the check in flight. One handler per run now, with explicit teardown;
  Ctrl+C aborts the whole run and an interrupted run reports failure.
- **The checksums-manifest lookup in the action's fetch step could be fooled**
  by a lookalike asset line (the asset name was interpolated into a regex).
  The lookup is now a byte-for-byte name comparison, with a regression test
  proving the pre-fix script accepted a lookalike.
- The `env: inherit` warning is now routable through the caller's stderr.

### Security

- `action.yml` — the file outside callers execute — still pinned
  `actions/upload-artifact@v4`; it is now SHA-pinned and
  `scripts/check-action-pins.bash` fails CI on any movable third-party ref.
- Generated workflows are adversarially validated: asset names are
  grammar-checked (newlines, traversal, YAML-key injection rejected) and
  quoted; the tag grammar is a real validator, not a shell `case` pattern.
- CI now runs `go test -race` on Linux — disclosed plainly: the race detector
  had never run for any prior release. `govulncheck` and fuzz gates kept green.

### Added

- **`shipsmoke init --ci --published`** — a generated **post-release** gate
  (`release: published` trigger): downloads the exact asset users get and
  runs it; unset asset names become visible `SET-ME-` placeholders that the
  workflow refuses to run rather than passing vacuously. Verified locally
  against real published assets: good archive PASS, truncated archive FAIL.
- `docs/CONFIGURATION.md` gained a full command-line reference; a test
  compares the documented snippet against the generator's bytes.
- `docs/VERIFICATION_LAYERS.md` documents the three verification layers
  (download checksum / archive integrity / real-run) per failure class.

### Verification of this release

- Merge-commit CI green on all five workflows, including `-race` (Linux),
  fuzz seeds, `govulncheck`, the fetch-script suite (30 cases) and the
  independent `uses: ./` end-to-end run with injection regression —
  [CI run](https://github.com/geyutu6755/shipsmoke/actions/runs/41893741772)
  and the [release run](https://github.com/geyutu6755/shipsmoke/actions/runs/41886310724)
  (7/7 jobs after a macOS-runner re-run; windows/arm64 remains build-only
  and says so in the job summary).
- Released `windows/amd64` asset: SHA-256 matches `checksums.txt`, binary
  self-reports `v1.0.4 commit 91c0e62`; `go install ...@v1.0.4` verified.

### Downloads

| File | Platform |
|---|---|
| `shipsmoke_linux_amd64.tar.gz` | Linux x86-64 |
| `shipsmoke_linux_arm64.tar.gz` | Linux ARM64 |
| `shipsmoke_darwin_amd64.tar.gz` | macOS Intel |
| `shipsmoke_darwin_arm64.tar.gz` | macOS Apple Silicon |
| `shipsmoke_windows_amd64.zip` | Windows x86-64 |
| `shipsmoke_windows_arm64.zip` | Windows ARM64 |
| `checksums.txt` | SHA-256 checksums |
