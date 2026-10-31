## ShipSmoke v1.0.0

**Test what users actually download.**

Verify that release artifacts actually extract and run on the target operating
system — before your users find out they don't.

## Downloads

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

Download the archive for your platform, extract it, and run:

```
./shipsmoke version
```

Or install with Go:

```
go install github.com/geyutu6755/shipsmoke/cmd/shipsmoke@latest
```

## Quick start

```bash
shipsmoke init          # write an example .shipsmoke.yml
shipsmoke validate      # check the config
shipsmoke check         # verify your artifacts (exit code drives your CI gate)
```

Every binary in this release was packaged and then **verified by shipsmoke
itself on a real GitHub-hosted runner** (zip on Windows, tar.gz on Linux and
macOS) before being published — see the release workflow run for evidence.

See the [README](https://github.com/geyutu6755/shipsmoke#readme) for full
documentation, and [SECURITY.md](https://github.com/geyutu6755/shipsmoke/blob/main/SECURITY.md)
for the security model (shipsmoke runs programs; it is not a sandbox).
