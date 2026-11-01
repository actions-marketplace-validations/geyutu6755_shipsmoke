# Tutorial: verify a C tool from zero to three-platform CI

Every step below corresponds 1:1 to what runs in
[example-c-cli's workflow](https://github.com/geyutu6755/shipsmoke/blob/main/.github/workflows/example-c-cli.yml),
executed on real ubuntu-24.04, macos-15 (arm64) and windows-2025 runners on
every push to the shipsmoke repository
([latest run](https://github.com/geyutu6755/shipsmoke/actions/workflows/example-c-cli.yml)).

## 0. Prerequisites

- A C compiler: `cc` (Linux/macOS), `gcc` (Windows runners ship MinGW gcc)
- shipsmoke on PATH (from [Releases](https://github.com/geyutu6755/shipsmoke/releases), or `go install github.com/geyutu6755/shipsmoke/cmd/shipsmoke@latest`)

## 1. A minimal tool

`ckit.c`:

```c
#include <stdio.h>
#include <string.h>

int main(int argc, char **argv) {
    if (argc > 1 && strcmp(argv[1], "--version") == 0) {
        printf("ckit version 0.1.0\n");
        return 0;
    }
    printf("usage: ckit --version\n");
    return 0;
}
```

## 2. Build, package, verify locally

Linux/macOS:

```bash
mkdir -p dist
cc -O2 -o dist/ckit ckit.c
tar -czf dist/ckit.tar.gz -C dist ckit
```

Windows (MinGW gcc):

```powershell
gcc -O2 -o dist\ckit.exe ckit.c
Compress-Archive -Path dist\ckit.exe -DestinationPath dist\ckit.zip
```

`.shipsmoke.yml`:

```yaml
version: 1
artifacts:
  - name: ckit-unix
    path: dist/ckit.tar.gz
    binary: ckit
    runs_on: [linux/amd64, darwin/amd64, darwin/arm64]
    checks:
      - args: ["--version"]
        expect: { exit_code: 0, stdout_contains: ["ckit version 0.1.0"] }

  - name: ckit-windows
    path: dist/ckit.zip
    binary: ckit.exe
    runs_on: [windows/amd64]
    checks:
      - args: ["--version"]
        expect: { exit_code: 0, stdout_contains: ["ckit version 0.1.0"] }
```

```bash
shipsmoke check
```

shipsmoke identifies the compiled binary from its real ELF/PE/Mach-O header —
no toolchain integration needed; C artifacts are verified exactly like Go or
Rust ones.

## 3. CI

```bash
shipsmoke init --ci --stack c --name ckit
```

The generated workflow compiles with `cc`/`gcc` per platform, packages, and
runs the shipsmoke gate — the same shape verified in example-c-cli's CI.
Commit and push.

## 4. Troubleshooting

- `cannot start the entry binary: format not executable on this platform` →
  you packaged the binary for another OS, or the exec bit was lost (Windows
  packagers drop Unix modes; build tar.gz artifacts on Unix or set the bit
  before packaging).
- Unix permissions: if you must build tar.gz on Windows, add the exec bit on
  the extracting side is impossible — rebuild on a unix runner instead
  (the generated CI does exactly that).
- Everything else: [TROUBLESHOOTING.md](TROUBLESHOOTING.md).
