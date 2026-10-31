# ShipSmoke — Threat Model (V1.0)

Date: 2026-11-01 | Status: Accepted

## 1. Scope and trust boundaries

ShipSmoke extracts and **executes software**. Its two operating modes have very
different threat levels:

- **Mode A — verify your own build (primary V1.0 use).** The artifacts were produced
  by the user's own build moments ago and run on a GitHub-hosted runner or dev box.
  Trust level: high (same origin as the operator). This is the documented, intended
  V1.0 workflow.
- **Mode B — verify a downloaded third-party release.** The artifact comes from a
  third party. The extracted binary is **untrusted code**. V1.0 supports this mode
  with explicit extraction defenses, but a hostile binary executing on the host can
  always do whatever the host user can do. ShipSmoke's isolation is the working
  directory and process lifecycle — **it is not a security sandbox**, and the docs
  say so in exactly those words.

Trust boundary diagram:

```
┌──────────────────────────── host / CI runner ────────────────────────────┐
│  CI secrets (GITHUB_TOKEN, signing keys …)                                │
│  ┌──────────────┐   config file   ┌─────────────────────────────────────┐ │
│  │  ShipSmoke   │◄───────────────►│ untrusted artifact (zip/tgz/bin)    │ │
│  │  process     │                 │        │ extracted, executed        │ │
│  └──────────────┘                 │        ▼                            │ │
│         ▲                         │ child process (smoke-tested binary) │ │
│         │ reports (redacted)      └─────────────────────────────────────┘ │
└─────────┼─────────────────────────────────────────────────────────────────┘
          ▼
   report files / GITHUB_STEP_SUMMARY (may be read by others)
```

## 2. Assets to protect

1. CI/developer secrets present in the environment (GITHUB_TOKEN etc.).
2. Host filesystem integrity outside the extraction directory.
3. CI correctness (a broken artifact must never yield a green check).
4. Report integrity (no secret leakage; no false PASS).

## 3. Threats and mitigations

| # | Threat | Vector | Mitigation | Status |
|---|---|---|---|---|
| T1 | Zip Slip / path traversal — archive entry escapes extraction dir (`../../.ssh/authorized_keys`) | malicious zip/tgz | every entry name validated pre-write: cleaned path must stay under dest root; absolute paths, `..` segments, drive letters, backslash tricks, Windows reserved names rejected; tests cover each variant | implemented + tested |
| T2 | Zip bomb — archive expands to disk-filling sizes | malicious zip | streaming limits: max entry count, per-file cap, cumulative cap, per-entry compression-ratio cap; limits enforced during copy, not after; configurable, safe defaults | implemented + tested |
| T3 | Symlink/hardlink attacks — entry is a symlink pointing outside, written then dereferenced by later entries | malicious tar/zip | symlink & hardlink entries rejected outright with explicit error; device/FIFO entries rejected; V1.0 never creates links | implemented + tested |
| T4 | Shell/argument injection — config args interpreted by a shell | malicious or sloppy config | `exec.CommandContext` with argv array; no shell anywhere in the exec path; args are never string-joined | by construction |
| T5 | Secret leakage — child process or reports expose CI tokens | env inheritance; captured output | default `env: minimal` child environment; secret-pattern variables dropped from any inherited env; captured stdout/stderr and all reports pass through redaction of secret values (and common encodings) | implemented + tested |
| T6 | Runaway processes — smoke-tested binary hangs or spawns children | hostile/buggy binary | hard timeout (default 60 s), context kill + process-group kill (Unix) / `taskkill /T` (Windows); stdout/stderr capped at 1 MiB per stream | implemented + tested |
| T7 | Privileged execution of untrusted binary | user runs Mode B on a machine with release keys | documentation: SECURITY.md + README warn explicitly; child env minimized; docs state working-dir isolation is **not** a sandbox; recommendation: run untrusted verification on ephemeral CI runners without deploy secrets | documented; sandbox out of scope V1.0 |
| T8 | False green — check not executed but reported as pass | config mistakes, platform skips | SKIP is a distinct status; zero executed checks → exit code 3 (fails CI by default, `--allow-empty` to override); unsupported platforms labeled, never invented | implemented + tested |
| T9 | Malicious config (`.shipsmoke.yml` from untrusted repo) — huge timeout, env_extra exfiltration, unknown fields smuggled | running against untrusted checkout | strict decode rejects unknown fields; timeouts have sane caps (≤ 1 h enforced); `env: inherit` emits a warning; docs recommend reviewing config like code | implemented + documented |
| T10 | Report tampering / secrets in artifacts uploaded publicly | GITHUB_STEP_SUMMARY, JSON report | redaction before writing; summary contains no raw env dumps; JSON report path is user-controlled | implemented |
| T11 | Memory exhaustion via output flood | binary printing gigabytes | capped capture buffers | implemented + tested |
| T12 | GitHub Actions privilege abuse | workflow misuse | ShipSmoke's own workflows use `permissions: contents: read` (release workflow adds `contents: write` only in the release job); documented recipes use minimal permissions and do not expose the token to the smoke-tested binary | implemented in workflows |

## 4. Explicit non-goals (V1.0)

- Operating-system sandboxing (namespaces, Job Objects, seccomp, VMs). ShipSmoke
  isolates the working directory and the process lifetime; it does not confine a
  malicious binary's syscalls. This is stated in README, SECURITY.md and --help.
- Antivirus / malware classification of artifacts.
- Verifying artifact *provenance* (use cosign / GitHub attestations alongside).
- Network egress restrictions for the child process (the child may make network
  calls; CI runners are ephemeral; documented).

## 5. Residual risks (accepted for V1.0, documented)

1. Mode B execution of a hostile binary is dangerous on non-ephemeral machines —
   mitigated by documentation, minimal env, and process-tree termination.
2. On Windows, path validation cannot fully emulate NTFS case-insensitivity inside
   one volume scan; all crafted traversal names are rejected pre-write, which closes
   the practical vector.
3. Header-based detection cannot identify packed/obfuscated binaries; ShipSmoke
   reports "unknown" honestly rather than guessing.
