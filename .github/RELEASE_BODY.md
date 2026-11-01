## ShipSmoke v1.0.3

**Test what users actually download.**

Reliability, docs-experience and adoption-readiness release.

### Fixed

- **tar.gz integrity (the headline fix):** the extractor now always verifies
  the full gzip member CRC — corrupted payload bytes, corrupt CRC fields and
  truncated trailers fail extraction instead of passing silently. Trailing
  bytes after a complete member remain accepted (documented leniency).
  Five corruption fixtures cover each failure class; the three verification
  layers (download checksum / archive integrity / real-run) are documented
  in docs/VERIFICATION_LAYERS.md.

### Security

- Third-party Actions pinned to commit SHAs across all workflows.
- Build-provenance attestations for every release archive:
  `gh attestation verify <archive> -R geyutu6755/shipsmoke`
- The release job re-verifies every asset's SHA-256 against checksums.txt
  before attaching it.
- Stricter version grammar in the action's fetch step.

### New

- `shipsmoke init --ci --stack go|rust|c --name <tool>` — generates a
  three-platform build, then package, then verify workflow from the verified
  harness shape. Explicit stacks; no build-system guessing.
- Homebrew tap and Scoop bucket (installed-and-tested in their own CI):
  `brew install geyutu6755/tap/shipsmoke`
  `scoop bucket add geyutu6755 https://github.com/geyutu6755/scoop-bucket`
- Tutorials for Go, Rust and C; real broken-release demo
  (examples/demo-broken-release.sh); failure guidance in reports.

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

Verification coverage of this release: linux/amd64, darwin/arm64,
windows/amd64 native; linux/arm64 in a QEMU container; darwin/amd64 under
Rosetta; windows/arm64 build-only (no x64 execution path — recorded, not
faked).
