## ShipSmoke v1.0.2

**Test what users actually download.**

Security and usability revision of the release-verification action.

### Security fixes in the GitHub Action

- **No more script injection:** all action inputs reach shell code only via
  environment variables; automated injection-regression tests on Windows,
  Linux and macOS prove hostile input values fail inertly.
- **Downloads are verified before execution:** fetched release assets are
  SHA-256 checked against `checksums.txt`; mismatches abort before any
  extraction or execution. Step output `verified` reports how the binary
  was obtained (`checksum` or `user-provided`).
- **Strict version pinning:** `version:` accepts `latest` or a full
  `vX.Y.Z` tag only. The `v1` moving tag now exists for `uses:` and tracks
  this release.

### Execution coverage of this release's own artifacts

| Artifact | Verification |
|---|---|
| linux/amd64 | native on ubuntu-24.04 |
| linux/arm64 | QEMU arm64 container (ubuntu:24.04) |
| darwin/arm64 | native on macos-15 |
| darwin/amd64 | Rosetta 2 |
| windows/amd64 | native on windows-2025 |
| windows/arm64 | build-only (no execution path on x64 runners; recorded, not faked) |

Every executable mode runs shipsmoke's own check against the packaged
artifact before publishing.

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

## Install

```
go install github.com/geyutu6755/shipsmoke/cmd/shipsmoke@v1.0.2
```

or download the archive for your platform and run `./shipsmoke version`.

See the [README](https://github.com/geyutu6755/shipsmoke#readme) for full
documentation and [SECURITY.md](https://github.com/geyutu6755/shipsmoke/blob/main/SECURITY.md)
for the security model (shipsmoke runs programs; it is not a sandbox).
