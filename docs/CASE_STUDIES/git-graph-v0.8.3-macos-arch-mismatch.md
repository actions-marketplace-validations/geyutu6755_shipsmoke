# Case study: git-graph v0.8.3 — the `macos-amd64` archive contains an arm64 binary

Status: **independently verified, NOT reported upstream.** No issue, PR, or
message has been sent to git-graph's maintainers; the English draft below is
held here pending maintainer-consent approval. This is a Tier 0 trial under
[../ADOPTION.md](../ADOPTION.md) — git-graph's maintainers have not adopted
ShipSmoke and are not aware of this document.

## Summary

`git-bahn/git-graph` release v0.8.3 publishes
`git-graph-v0.8.3-macos-amd64.tar.gz`, an archive **named** for macOS Intel
(amd64). Its payload is actually a **single-architecture arm64 Mach-O**
binary. Intel-Mac users who download that asset get a binary their machine
cannot execute. The linux and windows archives match their names (ELF
amd64 / PE amd64) and work as advertised.

## Evidence (all re-verified 2026-11-06, this round)

### The release

- Repository: https://github.com/git-bahn/git-graph
- Tag: `v0.8.3` (latest release at the time of writing; published 2026-10-29)

### Assets, sizes, SHA-256 (downloaded 2026-11-06)

| Asset | Size (bytes) | SHA-256 |
|---|---|---|
| `git-graph-v0.8.3-linux-amd64.tar.gz` | 1 339 344 | `f2f83fce6458f716e2d80dab572444e69abcbebc14ce0a1f70d532ce87ac6729` |
| `git-graph-v0.8.3-macos-amd64.tar.gz` | 1 156 111 | `a86bf7ad8ea408bd62eb3c87f5f7d7c4c5b01d235fb36e5c302468fed67fcc9c` |
| `git-graph-v0.8.3-windows-amd64.tar.gz` | 1 443 599 | `95fd0fdc726fef4ff8bf8a61f85345b66e73e72c52d9e119c45661dd801b5e14` |

### Header identification (two independent methods)

`file(1)` and a direct Mach-O header parse (magic `0xFEEDFACF`, little-endian,
read with Python `struct`) agree:

- `macos-amd64.tar.gz` payload `git-graph`:
  `Mach-O 64-bit arm64 executable, cputype=0x100000c, filetype=2 (MH_EXECUTE),
  21 load commands, single-arch (NOT a universal/fat binary)`.
- `linux-amd64.tar.gz` payload `git-graph`:
  `ELF 64-bit LSB pie executable, x86-64, e_machine=62` — matches its name.
- `windows-amd64.tar.gz` payload `git-graph.exe`:
  `PE32+ executable, machine=0x8664 (amd64)` — matches its name.

The macos archive is not a Universal Binary; it is one arm64 slice under an
amd64 name.

### Execution checks

- windows/amd64 archive on a windows-2025 runner (shipsmoke v1.0.4 trial,
  [run 42130759840](https://github.com/geyutu6755/shipsmoke-integration/actions/runs/42130759840)):
  extracted, executed, `[PASS] git-graph.exe --version (79ms)` — PASS.
- linux/amd64 archive on an ubuntu-24.04 runner (same run):
  `[PASS] git-graph --version (1ms)` — PASS.
- macos arm64-named-binary check: shipsmoke's platform gate recorded the
  mismatch honestly (`[SKIP] … targets [darwin/amd64] but host is darwin/arm64`,
  gate exit 3 — **recorded as SKIP, not PASS**), and a second pass declaring
  the platform the binary actually is (darwin/arm64) executed it natively:
  `[PASS] git-graph --version (16ms)` — proving the payload works on arm64
  and is simply named wrong for amd64.
- On a Windows host, the macos archive is NOT EXECUTABLE (foreign binary) —
  recorded as NOT TESTED for execution, per platform rules.

### Scope: not a one-off

`git-graph-v0.8.2-macos-amd64.tar.gz` (1 154 674 bytes, downloaded
2026-11-06) contains the same arm64 Mach-O (`cputype 0x100000c`). The
mismatch is systematic in the release packaging, not a v0.8.3 accident.

### Upstream context

- git-graph's latest release remains v0.8.3 (no fixed version exists).
- Upstream issue
  [#74 "Support for macos-arm64 platforms"](https://github.com/git-bahn/git-graph/issues/74)
  (open, created 2023-08-10) shows the maintainer knows arm64 support is
  missing — the deeper problem our evidence adds is that the asset **named**
  amd64 has been serving arm64 binaries.

### How this was found

The archives were verified with ShipSmoke v1.0.4 (a release-verification
tool — it downloads an asset, extracts it under safety limits, identifies the
entry binary from its ELF/PE/Mach-O header, and gates the header against the
host platform). ShipSmoke's header-based detection refused to trust the
`macos-amd64` filename, which is what surfaced the mismatch. The header
findings above were then re-confirmed with `file(1)` and an independent
Python header parse, without relying on ShipSmoke.

## Draft report (English, held pending maintainer-contact approval)

> **Title:** `macos-amd64` release archive contains an arm64 binary
>
> **Version:** v0.8.3 (also reproduced with v0.8.2)
>
> **Asset:** `git-graph-v0.8.3-macos-amd64.tar.gz`
> SHA-256 `a86bf7ad8ea408bd62eb3c87f5f7d7c4c5b01d235fb36e5c302468fed67fcc9c`
>
> **Expected:** an archive named `macos-amd64` should contain an x86-64
> (amd64) Mach-O binary.
>
> **Actual:** the payload is a single-architecture **arm64** Mach-O
> (`cputype 0x100000c`, not a universal binary). Verified with `file(1)`
> (`Mach-O 64-bit arm64 executable`) and an independent parse of the Mach-O
> header. Execution on an arm64 Mac works; on Intel Macs the binary fails
> with `Bad CPU type in executable`.
>
> **Reproduction:** download the asset, `tar -xzf`, run `file git-graph`.
>
> **Note:** upstream issue #74 tracks adding macos-arm64 support; the arm64
> binary appears to already exist under the `amd64` name, so this may be as
> simple as renaming/re-splitting the asset (or emitting a universal binary).
>
> **Disclosure:** the mismatch was surfaced while verifying the release with
> ShipSmoke (a release-verification tool we maintain, which gates binaries by
> their ELF/PE/Mach-O header rather than by filename). Reproduction requires
> only `tar` and `file`; ShipSmoke is not needed.

## Tier discipline

- Trial tier: **0** (local/CI trial on a third party's published artifacts).
- Nothing has been sent to git-graph's maintainers; the draft above is the
  complete, ready-to-file report awaiting user authorization.
- No third-party binaries are redistributed from this project's releases.
