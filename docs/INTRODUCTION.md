# ShipSmoke — an introduction for open-source maintainers

*Test what users actually download.*

## The problem, in one paragraph

Your release pipeline builds binaries for six platforms, tars and zips them,
publishes them with a checksums file, and everything is green. What nobody
checked is whether the **archive the user downloads actually contains the
binary, whether it extracts, and whether it runs** — on the OS it claims to
support. Packaging bugs are silent by nature: the build succeeded, the
upload succeeded, the checksums match, and the user on ARM Linux gets
"cannot execute binary file". Real projects have shipped wrong-architecture
archives this way — including, embarrassingly, this project's own v1.0.0 and
v1.0.1, which is exactly why the tool now catches it in CI before publish
(see the [v1.0.2 release notes](https://github.com/geyutu6755/shipsmoke/releases/tag/v1.0.2)).

## What ShipSmoke does

A single static binary + one composite GitHub Action that:

1. **Download integrity** — fetches a release asset and verifies its SHA-256
   against the release's `checksums.txt` before anything executes.
2. **Archive integrity** — extracts it safely (Zip Slip protection, bomb
   limits, symlink rejection) and verifies the gzip CRC fully.
3. **Real-run verification** — locates the entry binary from its real
   ELF/PE/Mach-O header, runs your configured checks (`--version`, exit
   codes, stdout/stderr matching), and fails the job — blocking your publish
   step — if the artifact doesn't work on that OS.

Runs on `ubuntu-24.04`, `macos-15` (arm64) and `windows-2025` GitHub-hosted
runners. Each artifact is verified on a runner that genuinely matches its
target — no emulated platform names.

## What it is not

- Not a build tool. It does not replace GoReleaser/Cargo/npm; it verifies
  their output.
- Not a sandbox. The verified program runs with your workflow's permissions;
  verify your *own* artifacts on ephemeral runners.
- Not an integrity library. For cryptographic signing use cosign or GitHub
  artifact attestations (ShipSmoke's own releases carry attestations); the
  action's checksum gate is integrity-to-manifest, not authenticity.
- No cloud, no account, no paid tier. One Go dependency (a YAML parser).

## Honest limitations

- Supports zip, tar.gz, and standalone binaries. Not deb/rpm/msi/dmg.
- Symlinks inside artifacts are rejected, not recreated.
- windows/arm64 artifacts are built but cannot be *executed* on x64 hosted
  runners — the release pipeline records this as build-only rather than
  pretending.
- tar.gz content corruption is caught via the gzip CRC since v1.0.3, but
  trailing junk after a complete member is tolerated (documented).

## Try it

- One-minute quickstart and a real broken-release demo: [README](https://github.com/geyutu6755/shipsmoke#readme)
- Tutorials: [Go](TUTORIAL_GO.md) · [Rust](TUTORIAL_RUST.md) · [C](TUTORIAL_C.md)
- Independent three-OS integration harness: [shipsmoke-integration](https://github.com/geyutu6755/shipsmoke-integration)
- Release quality gate in CI: `shipsmoke init --ci --stack go --name mytool`

MIT licensed. Free forever, no accounts.
