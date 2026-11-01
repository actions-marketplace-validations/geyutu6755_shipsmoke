# Tutorial: verify a Rust CLI from zero to three-platform CI

Every step below corresponds 1:1 to what runs in
[example-rust-cli's workflow](https://github.com/geyutu6755/shipsmoke/blob/main/.github/workflows/example-rust-cli.yml),
which is executed on real ubuntu-24.04, macos-15 (arm64) and windows-2025
runners on every push to the shipsmoke repository
([latest run](https://github.com/geyutu6755/shipsmoke/actions/workflows/example-rust-cli.yml)).

## 0. Prerequisites

- Rust toolchain (`cargo --version`) — GitHub-hosted runners have it preinstalled
- shipsmoke on PATH (from [Releases](https://github.com/geyutu6755/shipsmoke/releases), or `go install github.com/geyutu6755/shipsmoke/cmd/shipsmoke@latest`)

## 1. A minimal CLI

```bash
cargo new mytool
cd mytool
```

`src/main.rs`:

```rust
fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    match args.first().map(String::as_str) {
        Some("--version") => println!("mytool version 0.1.0"),
        _ => println!("usage: mytool --version"),
    }
}
```

## 2. Build, package, verify locally

```bash
cargo build --release
mkdir -p dist
cp target/release/mytool dist/mytool          # Windows: mytool.exe
tar -czf dist/mytool.tar.gz -C dist mytool    # Windows: Compress-Archive → mytool.zip
```

`.shipsmoke.yml`:

```yaml
version: 1
artifacts:
  - name: mytool-unix
    path: dist/mytool.tar.gz
    binary: mytool
    runs_on: [linux/amd64, darwin/amd64, darwin/arm64]
    checks:
      - args: ["--version"]
        expect: { exit_code: 0, stdout_contains: ["mytool version 0.1.0"] }

  - name: mytool-windows
    path: dist/mytool.zip
    binary: mytool.exe
    runs_on: [windows/amd64]
    checks:
      - args: ["--version"]
        expect: { exit_code: 0, stdout_contains: ["mytool version 0.1.0"] }
```

```bash
shipsmoke check
```

The artifact for the *other* platform family shows as SKIP with its reason;
the matching one runs for real.

## 3. CI

```bash
shipsmoke init --ci --stack rust --name mytool
```

Commit the generated `.shipsmoke.yml` and
`.github/workflows/shipsmoke.yml`, push. The generated workflow
(`cargo build --release` → package → shipsmoke gate) is byte-for-byte the
shape verified in example-rust-cli's CI.

## 4. Troubleshooting

Rust-specific notes:

- The release binary lives in `target/release/` — remember to copy it to
  `dist/` before packaging (the generated workflow does this for you).
- Windows artifacts must keep the `.exe` suffix in both the package and the
  `binary:` field.
- Everything else: [TROUBLESHOOTING.md](TROUBLESHOOTING.md).
