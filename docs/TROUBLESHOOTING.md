# Troubleshooting

Real failure modes, what the report looks like, and what to do.

## `artifact file not found: dist/…`

The path in `path:` does not exist relative to where you ran `shipsmoke
check`. Check the working directory, or run with `--config` pointing at the
right file. In CI, make sure the build step actually produced the artifact
before the verify step.

## `entry binary "mytool" not found in the artifact; top-level contents: …`

The `binary:` path does not match the archive layout. The report lists the
archive's top-level entries. Common causes:

- Archive wraps everything in a folder: use `binary: mytool-1.0/mytool`.
- Windows binaries: include the `.exe` (`binary: mytool.exe`).
- Standalone file: `binary:` must be the artifact's own file name.

## `platform mismatch: … binary header says linux/amd64; host is windows/amd64`

The artifact contains a binary for another OS. ShipSmoke detected this from
the real ELF/PE/Mach-O header. Verify each artifact on a runner that matches
its platform — see the matrix recipe in the README. If the binary is
something you can rebuild, rebuild for the platform you are verifying on.

## `declaration mismatch: runs_on says [X] but the binary header identifies Y`

Your config's `runs_on` does not include the platform the binary actually is.
Fix the config (or the build). Listing several platforms is fine as long as
one matches — e.g. windows/arm64 emulation for an amd64 binary.

## `archive could not be extracted safely: … path traversal`

The archive contains an entry that tries to escape the extraction directory
(`../…`, absolute paths, drive letters). ShipSmoke refuses the whole archive.
If this is your own artifact, inspect how it was built — a traversal entry is
never innocent.

## `archive entry "…" is a symbolic link: not supported, refusing to create links`

The archive contains symlinks/hard links/devices. ShipSmoke rejects them
entirely. If your tar contains symlinks (e.g. from a Unix build), package the
real files instead.

## `archive rejected by safety limits: …`

The archive hit `safety` limits: too many entries, a file larger than
`max_file_size`, more than `max_total_size` uncompressed, or an absurd
compression ratio (probable decompression bomb). If the artifact is
legitimately large, raise the limits in `safety:` — they exist to protect the
machine running the check.

## `expected exit code 0, got N`

The program ran and exited with an unexpected code. The report shows captured
stdout/stderr. If the non-zero code is the *expected* behavior of a healthy
program (e.g. `--help` exiting 2 on some tools), declare it:
`expect: { exit_code: 2 }`.

## `timed out after 60s (process tree killed)`

The program hung. ShipSmoke killed the whole process tree and failed the
check. Options: fix the program; raise `timeout:` for slow-but-healthy
programs (e.g. `timeout: 2m`); or add the flag that makes it exit.

## `could not start the entry binary: format not executable on this platform`

The binary is not runnable on this OS/arch — or it lost its executable bit on
Unix. If your packaging step builds a tar.gz on Windows, the exec bit is lost;
build the tar.gz on Unix or add it before packaging.

## Exit code 3 — "nothing was executed"

Every artifact was skipped: your `runs_on` lists do not include the current
platform. In a per-platform CI matrix that means a config mistake on this
runner — check the platform spec spelling (`linux/amd64`, not `linux-x64`).
If you genuinely want a no-op run, pass `--allow-empty`.

## CI: shipsmoke check passes locally but fails in Actions

- Different platform: the artifact verified locally may be for another OS —
  check the SKIP lines in both reports.
- Different working directory: use absolute or workflow-relative paths, or
  `working-directory:` on the step.
- Missing environment variables: default `env: minimal` does not pass parent
  variables through; add them via `env_extra`.

## Secrets in reports

Values of secret-shaped variables are masked with `***REDACTED***` before
anything is written. If you see one unmasked, the value likely reached the
report through program output under a different name — report it as a bug.
