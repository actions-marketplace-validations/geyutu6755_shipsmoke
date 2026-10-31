# Security Policy

## Reporting a vulnerability

Please report security issues by opening a
[security advisory](https://github.com/geyutu6755/shipsmoke/security/advisories/new)
or, if that is unavailable, an issue marked **SECURITY** with minimal
reproduction detail. Do not include exploit payloads in public issues.

You can expect a first response within 7 days. Fixes are released as patch
versions and noted in CHANGELOG.md.

## Security model, in one paragraph

ShipSmoke extracts archives (with strict path, size, entry-type, and ratio
limits) and **executes programs**. Everything shipsmoke itself does is
hardened — argv-array process launch (never a shell), double Zip Slip
validation, streaming extraction limits, explicit rejection of symlinks,
hard links, devices, and FIFOs, hard timeout with process-tree kill, capped
output capture, and redaction of secret-shaped environment values from
reports — but the program being verified runs with your workflow's
permissions. **ShipSmoke is not an operating-system sandbox.**

## What this means in practice

- **Verify your own build artifacts.** That is the intended, documented use:
  the artifact was produced by your build and is about to be published.
- **Untrusted third-party artifacts**: run them only on ephemeral CI runners
  that hold no deployment secrets. Do not verify untrusted artifacts on your
  laptop or on runners that have release keys.
- ShipSmoke's default `env: minimal` child environment deliberately excludes
  most parent environment variables and all secret-shaped ones
  (`*TOKEN*`, `*SECRET*`, `*PASSWORD*`, `*KEY*`, `*CREDENTIAL*`).
  `env: inherit` still removes secret-shaped variables and prints a warning.
- Reports (terminal, JSON, GitHub job summary) are passed through a redactor
  for values of secret-shaped environment variables before being written.

## Known boundaries (documented, accepted for v1.0)

- No syscall sandboxing (no seccomp/Job Objects/VM isolation).
- No network egress restriction on the verified binary.
- Header-based detection cannot classify packed/obfuscated executables; it
  reports "unknown" rather than guessing.
- Symbolic links and hard links inside artifacts are refused, never recreated.

Full analysis: [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md).
