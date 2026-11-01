# Configuration Reference

`shipsmoke` reads `.shipsmoke.yml` (or the path given with `--config`).
Schema version is 1. Unknown keys are errors — typos fail loudly.

```yaml
version: 1

safety:                      # optional; defaults shown
  max_files: 20000           # max entries per archive
  max_total_size: 1073741824 # 1 GiB total uncompressed
  max_file_size: 536870912   # 512 MiB per file
  max_compression_ratio: 1000

report:
  json: ""                   # path for the JSON report (optional)

artifacts:                   # one entry per release package
  - name: mytool-linux-amd64 # optional; defaults to the file basename
    path: dist/mytool-linux-amd64.tar.gz
    format: auto             # auto | zip | tar.gz | binary
    binary: mytool           # entry program, relative to the archive root
    runs_on: [linux/amd64]   # platforms where this artifact is verified
    env: minimal             # minimal (default) | inherit
    env_extra:               # extra variables for the verified program
      MY_FLAG: "1"
    checks:                  # at least one
      - args: ["--version"]
        expect:
          exit_code: 0       # default 0
          stdout_contains: ["mytool version"]
          stdout_not_contains: []
          stderr_contains: []
          stderr_not_contains: []
          stdout_equals: ""
          stderr_equals: ""
          stdout_regex: ""
          stderr_regex: ""
        timeout: 60s
```

## Field reference

### `version`
Must be `1`. Other values are rejected with an explanation.

### `safety` (optional)
Extraction limits, enforced while streaming — an archive cannot exhaust disk
before the limit triggers. Raising them loosens protection; lowering them
tightens it. `max_compression_ratio` compares uncompressed to compressed
bytes per entry (zip) or per archive (tar.gz), with an absolute-size override
so small highly-compressible files are not falsely rejected.

### `report.json`
When set, a machine-readable JSON report is written after every
`shipsmoke check` run. Override on the CLI with `--report PATH`. All string
values are passed through the secret redactor before writing.

### Each artifact

| Field | Required | Meaning |
|---|---|---|
| `name` | no | Identifier used in reports and `--artifact` filtering. Must be unique. Defaults to the basename of `path` without extensions. |
| `path` | yes | Path to the artifact (zip, tar.gz, or standalone binary). |
| `format` | no | `auto` (default) classifies by magic bytes. Explicit values are verified against reality — a mismatch is an error, so a mislabeled artifact cannot sneak through. |
| `binary` | yes | Entry program, relative to the archive root (forward slashes). For a standalone binary, the file's own name. |
| `runs_on` | no | `os/arch` list the artifact is verified on. Current host not in the list → artifact is **SKIP** (never a false pass). Empty = any platform. `any` is the same as empty. |
| `env` | no | `minimal` (default): small env (PATH with the extraction dir first, HOME/TEMP and Windows runtime essentials) + `env_extra`. `inherit`: parent env, minus secret-shaped variables, with a warning. |
| `env_extra` | no | Map of extra environment variables. Keys must be valid identifiers. |
| `checks` | yes | At least one check. |

### Each check

| Field | Required | Meaning |
|---|---|---|
| `args` | yes | Argument array. Passed to the process as-is — **never** through a shell, so no quoting or injection concerns. |
| `expect.exit_code` | no | Default 0. A binary that exits non-zero can still PASS if you declare the code (e.g. `exit_code: 2` for "usage error expected"). |
| `expect.stdout_contains` / `stderr_contains` | no | All listed substrings must appear (after CRLF normalization). |
| `expect.stdout_not_contains` / `stderr_not_contains` | no | None of the substrings may appear. |
| `expect.stdout_equals` / `stderr_equals` | no | Exact match after CRLF normalization. |
| `expect.stdout_regex` / `stderr_regex` | no | Regular expressions evaluated with `s` flag (dot matches newlines). Invalid regexes are config errors. |
| `timeout` | no | Go duration (`30s`, `2m`). Default 60 s, max 1 h. On expiry the process **tree** is killed (process group on Unix, `taskkill /T` on Windows) and the check FAILs. |

## Semantics you can rely on

- **Platform gating.** With `runs_on: [linux/amd64]` on a macOS runner, the
  artifact is SKIP with the reason in the report. A run where nothing was
  executed exits with code 3 so CI cannot go green by accident.
- **Declaration mismatch diagnosis.** If the binary header (e.g. PE/amd64)
  contradicts the host (e.g. linux/amd64) the check FAILs with an explicit
  diagnosis. Declaring several platforms is fine as long as one matches the
  header (windows/arm64 emulation is a legitimate reason to list both).
- **Honest unknowns.** Scripts (shebang) and unidentified binaries run if
  the platform gate matches, and the report says the platform could not be
  determined from the content.
- **Secrets.** Variables whose names look secret-shaped are removed from the
  child environment in both modes, and their values (length ≥ 8) are masked
  in every report.

## GitHub Actions

### Verify after build, before publish

```yaml
permissions:
  contents: read
steps:
  - uses: actions/checkout@v4
  - run: ./build-all.sh        # produce dist/
  - uses: geyutu6755/shipsmoke@v1
    with:
      config: .shipsmoke.yml
      report: shipsmoke-report.json
  - run: ./publish.sh
    if: success()
```

Inputs: `config`, `version` (release of shipsmoke to fetch: `latest` or a
full tag `vX.Y.Z` — moving refs like `v1` are rejected here), `binary-path`
(use an already-built shipsmoke; its integrity is then the caller's
responsibility), `report`, `artifact`, `allow-empty`, `upload-report`.
Outputs: `report-path`, `exit-code`, `verified` (`checksum` or
`user-provided`), `resolved-version`.

Security model: inputs reach the shell only via environment variables (no
`${{ inputs.* }}` interpolation inside `run:` blocks, so input values cannot
execute as script injection). Downloaded assets are SHA-256 verified against
the release's `checksums.txt` before extraction; download failure, a missing
manifest entry, or a hash mismatch aborts before the binary runs. The action
never passes `GITHUB_TOKEN` (or any secret) to the verified binary — the
default minimal environment excludes it. These behaviors are covered by the
`action-test` workflow (injection payloads must fail inertly; tampered
downloads must be refused), which runs on all three operating systems.

Run the matrix so each artifact is verified on a runner that really matches
its target platform — see the README recipe.

### Post-release verification

After publishing, re-download the actual release asset and verify it again
(catches CDN truncation and packaging mistakes):

```yaml
verify-release:
  needs: release
  strategy:
    matrix:
      include:
        - { os: ubuntu-24.04, asset: shipsmoke_linux_amd64.tar.gz }
        - { os: macos-15,     asset: shipsmoke_darwin_arm64.tar.gz }
        - { os: windows-2025, asset: shipsmoke_windows_amd64.zip }
  runs-on: ${{ matrix.os }}
  steps:
    - uses: actions/checkout@v4
    - run: |
        URL="https://github.com/${{ github.repository }}/releases/download/${{ github.ref_name }}/${{ matrix.asset }}"
        curl -fsSL -o artifact "$URL"
        # verify the downloaded bytes (integrity)
        sha256sum artifact
    - uses: geyutu6755/shipsmoke@v1
      with:
        binary-path: ${{ runner.temp }}/shipsmoke # or build from the checkout
    # then run shipsmoke check against the downloaded artifact
```

The v1.0 release workflow of this repository demonstrates the full pattern:
build → package → checksum → verify on the host-matching runner → publish.
