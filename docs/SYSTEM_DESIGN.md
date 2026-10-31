# ShipSmoke — System Design (V1.0)

Version: 1.0 | Status: Accepted | Date: 2026-11-01

## 1. Architecture

Single Go module `github.com/geyutu6755/shipsmoke`, single static binary, no runtime
network access. Layered, dependency-injected design; every layer independently testable.

```
cmd/shipsmoke/main.go        entrypoint; wires flags → cli
internal/cli/                command routing (init/validate/check/version), exit codes
internal/config/             .shipsmoke.yml schema, strict load + validation
internal/artifact/           format detection, safe extraction, binary metadata
internal/runner/             process execution, timeouts, process-tree kill, capture
internal/engine/             orchestration: config → artifacts → checks → results
internal/report/             terminal / JSON / GitHub summary rendering, secret redaction
```

Dependency rule: `cli → engine → {config, artifact, runner, report}`. No layer imports
upward. `report` has no knowledge of execution; it renders data produced by `engine`.

## 2. Data model

```go
type Report struct {
    ShipSmokeVersion string
    ConfigPath       string
    StartedAt        time.Time
    Duration         time.Duration
    HostOS, HostArch string
    Artifacts        []ArtifactResult
    Summary          Summary   // counts per status, overall status
    ExitCode         int
}

type ArtifactResult struct {
    Name, Path, Format string
    Decision           string          // ran | skipped | error
    SkipReason         string          // why skipped (platform mismatch etc.)
    Detected           *BinaryInfo     // OS/arch from actual header, when known
    ExtractedFiles     int
    Checks             []CheckResult
}

type CheckResult struct {
    Name          string          // e.g. "check #1: mytool --version"
    Args          []string
    Status        string          // PASS | FAIL | SKIP | ERROR
    ExitCode      int             // observed
    ExpectedExit  int
    Stdout, Stderr string         // captured, size-capped, redacted
    Duration      time.Duration
    ReproduceCmd  string
    FailureReason string          // human diagnosis
    TimedOut      bool
    Signal        string          // unix signal name if killed by signal
}
```

Overall status = FAIL if any FAIL/ERROR, else PASS if ≥1 executed, else
"NO-CHECKS" (all skipped) — reported explicitly, exit code 3 (unless
`--allow-empty`).

## 3. Configuration schema (v1)

```yaml
version: 1

# Optional extraction safety limits (defaults shown).
safety:
  max_files: 20000            # max entries per archive
  max_total_size: 1073741824  # 1 GiB total uncompressed
  max_file_size: 536870912    # 512 MiB per file
  max_compression_ratio: 1000 # reject zip bombs (per-entry, checked against absolute caps too)

report:
  json: ""                    # path for machine-readable JSON report (optional)

artifacts:
  - name: mytool-linux-amd64
    path: dist/mytool-linux-amd64.tar.gz
    format: auto              # auto | zip | tar.gz | binary
    binary: mytool            # entry binary; path inside archive (or the file itself)
    runs_on: [linux/amd64]    # platforms this artifact targets; "any" allowed
    env: minimal              # minimal | inherit  (default minimal)
    env_extra:                # extra env vars for the child process
      MY_FLAG: "1"
    checks:
      - args: ["--version"]
        expect:
          exit_code: 0        # default 0
          stdout_contains: ["mytool version"]
          stdout_not_contains: []
          stderr_contains: []
          stderr_not_contains: []
          stdout_equals: ""
          stderr_equals: ""
          stdout_regex: ""
          stderr_regex: ""
        timeout: 30s          # Go duration; default 60s
```

Validation rules:
- `version` must be `1`. Unknown top-level keys → error (strict YAML decode).
- Each artifact: `path` required and must exist; `binary` required; `name` defaults
  to file basename; duplicate names → error.
- `runs_on` entries must be `os/arch` (linux/darwin/windows/freebsd/… ×
  amd64/arm64/386/arm/riscv64) or `any`.
- Checks: at least one; `args` array only; timeout parseable; regex must compile.
- `env_extra` keys must be valid environment variable identifiers.

## 4. Detection logic (artifact/binary)

1. Classify container by magic bytes + extension fallback:
   - `PK\x03\x04` → ZIP; gzip(0x1F8B) → decompress → tar header check → tar.gz;
     tar alone (ustar) → rejected in V1.0 (plain tar unsupported; explicit error);
     ELF/PE/Mach-O magics → standalone binary; else unknown → explicit ERROR.
2. Extract to temp dir. Locate `binary` (relative path) inside. Not found → FAIL
   with the archive listing in the diagnosis.
3. Parse the entry binary header (ELF/PE/Mach-O) → `BinaryInfo{Format, OS, Arch,
   Bits, Endian}`. Script/shebang or unknown → `OS=unknown`, message explains that
   platform identity could not be established and why.
4. Platform decision:
   - current host ∈ artifact `runs_on`? no → SKIP (reason states host vs targets).
   - detected binary OS/arch vs host: match → run; mismatch → FAIL (platform
     mismatch, e.g. "artifact targets linux/amd64 but host is windows/amd64; binary
     header says PE/amd64").
   - header undecodable → run anyway if `runs_on` matches host (user said this
     artifact targets this platform), but annotate "binary format not identified".

## 5. Safe extraction (artifact/extract.go)

- All entry names validated before any filesystem write:
  - reject absolute paths, `..` segments, empty names, drive prefixes (`C:`),
    Windows separators/backslash tricks, reserved device names (CON, NUL, COM1…),
    trailing dots/spaces on Windows, and names escaping the destination root after
    `filepath.Clean`+`Join` verification (Zip Slip defense).
- Symbolic/hard links (tar) and zip entries with symlink mode bits → **rejected**
  with explicit error ("unsupported entry type") — never followed, never created.
- tar special types (char/block device, FIFO, directory hardlinks) → rejected.
- Limits enforced *while* streaming (not after): entry count, per-file size,
  cumulative size; zip ratio check = uncompressed/compressed per entry with
  absolute-cap override (a 10 MiB file that compresses to nothing is fine if under
  the absolute per-file cap).
- Extraction writes: regular files only; directory traversal guarded; on Unix,
  restore file mode from tar header masked to 0755/0644 sanity band (no setuid);
  on Windows, read-only attribute cleared for entry binary usability.
- Decompression: gzip with `bufio` streaming; `tar.Reader.Next()`; zip central
  directory walked via `zip.NewReader` with per-entry size checks *before* copy
  using `f.UncompressedSize64`, plus post-copy byte counting (headers can lie).

## 6. Execution engine (runner)

- `exec.CommandContext(ctx, binPath, args...)` — argv array, no shell.
- Child env: `minimal` = PATH (extracted dir first, so adjacent DLLs/rpath resolve),
  HOME, TMPDIR/TEMP/TMP, SystemRoot/COMPUTERNAME/USERPROFILE/windir on Windows,
  LANG/LC_ALL, plus `env_extra`; all variables matching
  `(TOKEN|SECRET|PASSWORD|PASSWD|KEY|CREDENTIAL)` are dropped and logged.
  `inherit` = parent env minus secret-pattern variables (with warning).
- Timeout: context deadline → first `Process.Kill()`, then process-tree kill:
  - Unix: `Setpgid: true`, kill `-pgid` SIGKILL.
  - Windows: `taskkill /F /T /PID` fallback if direct kill leaves descendants.
- Output: stdout/stderr piped through capped buffers (1 MiB per stream; overflow
  marked `[truncated]`).
- Spawn failures (missing binary, permission) → ERROR status with platform-specific
  hint (e.g. "is this a Windows PE running on Linux?").
- Temp dirs under `os.MkdirTemp`, removed in `defer` (best-effort double attempt).

## 7. Reporting & redaction

- Terminal: header (version, host, config) → per-artifact blocks with per-check
  lines `[PASS] mytool --version (12ms)` → summary table → repro commands.
- JSON: exact `Report` struct, stable field names, indented.
- GitHub summary: markdown written to `$GITHUB_STEP_SUMMARY` when set (tables,
  annotations line, status emoji).
- Redaction: collector gathers secret values from env at startup (variables whose
  *name* matches the secret pattern and whose value length ≥ 8), plus their
  URL-encoded/base64-ish forms; every report renderer replaces occurrences with
  `***REDACTED***`. Runs on stdout/stderr text and failure reasons.

## 8. Exit codes

| Code | Meaning |
|---|---|
| 0 | all executed checks passed |
| 1 | at least one check FAIL (or artifact platform mismatch / extraction failure of a targeted artifact) |
| 2 | usage/config error or infrastructure ERROR (invalid YAML, missing file, internal) |
| 3 | nothing executed (all checks skipped) — prevents silent green CI; disable with `--allow-empty` |

## 9. Error handling philosophy

- Every failure path produces: what happened, where, why, what to do next.
- User errors (config, missing files) → clean message, exit 2; no stack traces.
- Internal errors → wrapped with context; stack trace only with `--debug`.
- Unknown config keys are errors (strict decode), preventing silent typos like
  `run_on:` doing nothing.

## 10. Testing strategy

- Unit: config validation (table-driven), detection (fixtures embedded as byte
  headers), extraction (real archives built in-test; malicious archives synthesized
  in-test: Zip Slip paths, symlink entries, oversized entries, high-ratio entries).
- Runner: re-exec test-binary pattern (`TestHelperProcess`) for exit codes, output,
  timeout, signal kill — cross-platform without prebuilt fixtures.
- Engine integration: end-to-end on synthetic artifacts (zip/tgz/binary × pass/fail
  cases) covering all acceptance scenarios in ACCEPTANCE_PLAN.md.
- Fuzz: `FuzzExtract` on the tar/zip extractors (crash + panic targets).
- Cross-platform: full suite runs in GitHub Actions on ubuntu-24.04, macos-15,
  windows-2025; plus Windows-specific case-sensitivity/reserved-name tests.
