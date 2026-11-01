# Tutorial: verify a Go CLI from zero to three-platform CI

Every command in this tutorial has been executed end-to-end; the workflow at
the end is the exact shape running in
[shipsmoke-integration](https://github.com/geyutu6755/shipsmoke-integration/actions).

## 0. Prerequisites

- Go 1.24+ (`go version`)
- shipsmoke — pick one:
  - Download from [Releases](https://github.com/geyutu6755/shipsmoke/releases) and put it on your PATH, or
  - `go install github.com/geyutu6755/shipsmoke/cmd/shipsmoke@latest`

## 1. A minimal CLI (skip if you have one)

```bash
mkdir wordcount && cd wordcount
go mod init example.com/wordcount
```

`main.go`:

```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const version = "1.0.0"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("wordcount version %s\n", version)
		return
	}
	sc := bufio.NewScanner(os.Stdin)
	words := 0
	for sc.Scan() {
		words += len(strings.Fields(sc.Text()))
	}
	fmt.Println(words)
}
```

## 2. Build, package, verify locally

Linux/macOS:

```bash
go build -o dist/wordcount .
tar -czf dist/wordcount.tar.gz -C dist wordcount
```

Windows (PowerShell for the archive; `go build` must produce `wordcount.exe`):

```powershell
go build -o dist\wordcount.exe .
Compress-Archive -Path dist\wordcount.exe -DestinationPath dist\wordcount.zip
```

Write `.shipsmoke.yml` (adjust `runs_on` to wherever you actually test):

```yaml
version: 1
artifacts:
  - name: wordcount-unix
    path: dist/wordcount.tar.gz
    binary: wordcount
    runs_on: [linux/amd64, darwin/amd64, darwin/arm64]
    checks:
      - args: ["--version"]
        expect: { exit_code: 0, stdout_contains: ["wordcount version 1.0.0"] }

  - name: wordcount-windows
    path: dist/wordcount.zip
    binary: wordcount.exe
    runs_on: [windows/amd64]
    checks:
      - args: ["--version"]
        expect: { exit_code: 0, stdout_contains: ["wordcount version 1.0.0"] }
```

Run it:

```console
$ shipsmoke check

  [SKIP] wordcount-unix — artifact targets [linux/amd64, darwin/amd64, darwin/arm64] but host is windows/amd64
  wordcount-windows  format=zip (detected: windows/amd64)
    [PASS] check #1: wordcount.exe --version (149ms)

  summary: 1 passed, 0 failed, 1 skipped, 0 errored  →  PASS
```

*(Real output from a Windows machine. The SKIP line is a feature: artifacts
for other platforms are labeled, never faked. On a unix machine the zip
skips and the tar.gz runs. If NOTHING runs — e.g. you forgot to add the
current platform — shipsmoke exits 3 so CI cannot go green by accident.)*

## 3. CI: three platforms, one gate

Skip the hand-writing entirely:

```bash
shipsmoke init --ci --stack go --name wordcount
```

That generates `.shipsmoke.yml` and
`.github/workflows/shipsmoke.yml` (build → package → shipsmoke gate on
ubuntu-24.04, macos-15, windows-2025). Commit both, push, and the workflow
appears in your Actions tab. Or write it yourself — the hand-written version
lives in the [README](../README.md#use-in-github-actions).

## 4. Blocking a broken release

Add a publish step that depends on verification:

```yaml
      - name: Publish
        if: success()
        run: echo "attach dist/ to your release here"
```

If the artifact is broken — the binary is missing from the archive, the
`--version` output changed, the exit code moved — the verify job fails and
the publish step never runs. See the live demo in
[shipsmoke-integration](https://github.com/geyutu6755/shipsmoke-integration)
(the `refuse-tampered` jobs corrupt an artifact on purpose and must fail).

## 5. Troubleshooting

- `entry binary "wordcount" not found` → your archive layout differs; check
  the top-level listing shipsmoke prints in the failure.
- `platform mismatch … binary header says windows/amd64` → the artifact
  contains a binary for another OS; build per-platform.
- exit code 3 → nothing ran; your `runs_on` lists don't include the current
  platform. Full guide: [TROUBLESHOOTING.md](TROUBLESHOOTING.md).
