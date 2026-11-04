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

## Command line

Every flag below is what the binary actually parses; `shipsmoke <command> --help`
prints the same list with descriptions, generated from the flag definitions
themselves, so this table cannot silently fall behind.

### `shipsmoke check` — the gate

| flag | meaning |
|---|---|
| `--config PATH` | config file, default `./.shipsmoke.yml` |
| `--artifact NAME` | run only this artifact (the CI matrix uses this to give each runner one job entry) |
| `--report PATH` | JSON report path, overrides `report.json` from the config |
| `--allow-empty` | exit 0 instead of 3 when nothing was executed |
| `--no-color` | never emit ANSI colour (also honoured via `NO_COLOR` / `TERM=dumb`) |
| `--quiet` | print only failing/errored/skipped artifacts and the summary line |

### `shipsmoke validate` — schema check, executes nothing

| flag | meaning |
|---|---|
| `--config PATH` | config file to parse and validate |

### `shipsmoke init` — scaffolding

| flag | meaning |
|---|---|
| `--force` | overwrite an existing `.shipsmoke.yml` / generated workflow |
| `--ci` | additionally emit a GitHub Actions workflow |
| `--stack go\|rust\|c` | with `--ci`: which build stack to generate (required for the pre-publish gate) |
| `--name TOOL` | with `--ci`: your binary's name; defaults to the current directory name |
| `--published` | with `--ci`: generate the **post-release** gate instead of the pre-publish one |
| `--asset-linux NAME` | with `--published`: release asset name for linux/amd64 |
| `--asset-darwin NAME` | with `--published`: release asset name for darwin/arm64 |
| `--asset-windows NAME` | with `--published`: release asset name for windows/amd64 |

`init --published` needs no `--stack`: it never builds, so it never needs to
know how you build. Omitted `--asset-*` values become visible `SET-ME-…`
placeholders and the generated workflow refuses to run them.

## GitHub Actions

### Verify after build, before publish

```yaml
permissions:
  contents: read
steps:
  # Pinned to a commit SHA: a movable tag can be re-pointed at code that runs
  # with your workflow's token. See docs/SUPPLY_CHAIN.md.
  - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262 # v4
  - name: Build and package your artifacts
    run: |
      go build -o dist/mytool .
      tar -czf dist/mytool.tar.gz -C dist mytool
  - uses: geyutu6755/shipsmoke@v1
    with:
      config: .shipsmoke.yml
      report: shipsmoke-report.json
  - name: Publish only if verification passed
    if: success()
    run: echo "all artifacts verified — attach dist/ to your release here"
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

Pre-publish and post-release answer different questions, and only the second
one is available to a project that already has its own release pipeline:

| mode | command | question answered |
|---|---|---|
| pre-publish | `shipsmoke init --ci --stack go\|rust\|c --name TOOL` | "is the artifact I am *about* to publish sound?" |
| post-release | `shipsmoke init --ci --published --name TOOL --asset-…` | "is the artifact users *actually got* sound?" |

Generate both files:

```bash
shipsmoke init --ci --published --name mytool \
  --asset-linux  mytool-linux-amd64.tar.gz \
  --asset-darwin mytool-darwin-arm64.tar.gz \
  --asset-windows mytool-windows-amd64.zip
```

This writes `.shipsmoke.yml` and
`.github/workflows/shipsmoke-published.yml`. Both blocks below are the
byte-for-byte output of that command, comments included.
`TestDocsConfigurationMatchesGenerator` compares this page against the
generator on every test run, so the snippet cannot quietly fall behind the
emitter; `TestPublishedRejectsHostileAssetNames` covers the asset-name
boundary, and `TestGeneratedWorkflowsNeverInterpolateInRunBlocks` keeps any
`run:` block free of `${{ }}` interpolation and heredocs.

`.shipsmoke.yml`:

```yaml
version: 1

# Generated by "shipsmoke init --ci --published" — the POST-release gate.
# It verifies the asset users download, after it is published: no build step.
# Paths are relative to the workflow's dist/ directory, which the workflow fills
# by downloading from your Release. Supply names per platform with
# --asset-linux / --asset-darwin / --asset-windows, or edit them here.
# A path still containing SET-ME is refused by the generated workflow rather
# than passing vacuously.
# If your archive nests the binary in a directory, adjust "binary" too:
# shipsmoke lists the archive's top-level entries when it cannot find it.
report:
  json: dist/shipsmoke-report.json

artifacts:
  - name: mytool-linux
    path: "dist/mytool-linux-amd64.tar.gz"
    binary: mytool
    runs_on: [linux/amd64, linux/arm64]
    checks:
      - args: ["--version"]
        timeout: 60s
        expect:
          exit_code: 0
          stdout_contains: ["mytool"]
  - name: mytool-macos
    path: "dist/mytool-darwin-arm64.tar.gz"
    binary: mytool
    runs_on: [darwin/amd64, darwin/arm64]
    checks:
      - args: ["--version"]
        timeout: 60s
        expect:
          exit_code: 0
          stdout_contains: ["mytool"]
  - name: mytool-windows
    path: "dist/mytool-windows-amd64.zip"
    binary: mytool.exe
    runs_on: [windows/amd64, windows/arm64]
    checks:
      - args: ["--version"]
        timeout: 60s
        expect:
          exit_code: 0
          stdout_contains: ["mytool"]
```

`.github/workflows/shipsmoke-published.yml`:

```yaml
name: shipsmoke — verify published release

# POST-release gate: once a Release exists, one job per OS downloads the exact
# asset users download and ShipSmoke extracts and RUNS it. This is the layer a
# build-and-test CI structurally cannot cover: an upload truncated in flight,
# the wrong binary packed inside, a lost executable bit, an archive that
# extracts but will not start.
#
# It complements, and does not replace, the pre-publish gate generated by
# "init --ci --stack":
#   pre-publish   build -> package -> verify -> publish   (blocks a bad release)
#   post-release  download published asset -> verify      (detects a bad release)
#
# Third-party actions are pinned to commit SHAs; see docs/SUPPLY_CHAIN.md.
on:
  release:
    types: [published]
  workflow_dispatch:
    inputs:
      tag:
        description: Release tag to verify (defaults to the release from the event)
        required: false

permissions:
  contents: read

jobs:
  verify-published:
    name: ${{ matrix.artifact }}
    strategy:
      fail-fast: false
      matrix:
        include:
          - os: ubuntu-24.04
            artifact: mytool-linux
            asset: "mytool-linux-amd64.tar.gz"
          - os: macos-15
            artifact: mytool-macos
            asset: "mytool-darwin-arm64.tar.gz"
          - os: windows-2025
            artifact: mytool-windows
            asset: "mytool-windows-amd64.zip"
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262

      # Values reach the shell only through env:. GitHub expands the ${{ ... }}
      # expressions outside the shell, so interpolating them inside a run block
      # would be script injection; no run block below interpolates anything.
      - name: Resolve release tag
        id: tag
        shell: bash
        env:
          INPUT_TAG: ${{ github.event.inputs.tag }}
          EVENT_TAG: ${{ github.event.release.tag_name }}
        run: |
          set -euo pipefail
          TAG="${INPUT_TAG:-$EVENT_TAG}"
          if [ -z "$TAG" ]; then
            echo "::error::no tag to verify: trigger on a published release or pass inputs.tag"
            exit 1
          fi
          # Anchored, single-line grammar. A shell case pattern like v[0-9]*
          # means "digit then anything at all", so it would accept newlines and
          # '../' and then write them into GITHUB_OUTPUT and into log lines.
          if ! printf '%s' "$TAG" | grep -Eq '^(v)?[0-9]+\.[0-9]+(\.[0-9]+)?(-[0-9A-Za-z][0-9A-Za-z.-]*)?$'; then
            echo "::error::refusing unexpected release tag (expected vX.Y.Z, optionally with a prerelease suffix)"
            printf 'rejected value (quoted, control chars visible): %q\n' "$TAG"
            exit 1
          fi
          echo "tag=$TAG" >> "$GITHUB_OUTPUT"

      - name: Refuse unfilled asset placeholders
        shell: bash
        env:
          ASSET: ${{ matrix.asset }}
          ARTIFACT: ${{ matrix.artifact }}
        run: |
          set -euo pipefail
          case "$ASSET" in
            *SET-ME*)
              echo "::error::the generated config still has a placeholder asset for $ARTIFACT: $ASSET"
              echo "Re-run: shipsmoke init --ci --published --name mytool --asset-linux ... --asset-darwin ... --asset-windows ..."
              echo "Refusing to continue: pointing at a non-existent asset would report green while testing nothing."
              exit 1 ;;
          esac

      - name: Download the published asset
        shell: bash
        env:
          TAG: ${{ steps.tag.outputs.tag }}
          ASSET: ${{ matrix.asset }}
          REPO: ${{ github.repository }}
        run: |
          set -euo pipefail
          mkdir -p dist
          URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
          echo "downloading $URL"
          curl -fsSL --retry 3 --retry-delay 2 -o "dist/$ASSET" "$URL"
          ls -l "dist/$ASSET"

      - name: Verify the downloaded artifact really runs
        uses: geyutu6755/shipsmoke@v1
        with:
          version: v1.0.4
          config: .shipsmoke.yml
          artifact: ${{ matrix.artifact }}
          report: dist/shipsmoke-report.json

      - name: Upload verification report
        if: always()
        uses: actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02
        with:
          name: shipsmoke-published-${{ matrix.artifact }}
          path: dist/shipsmoke-report.json
          if-no-files-found: ignore
```

Why it is shaped this way:

- **No build step.** The gate consumes bytes that already exist. If it rebuilt,
  it would test the checkout again and prove nothing about the release.
- **No interpolation inside `run:`.** Every value arrives through `env:`;
  `${{ }}` inside a run block is script injection. `on: release` fields are
  attacker-influenceable text (a tag name is editable in the Release UI), which
  is why the tag is grammar-checked before it reaches a URL.
- **Placeholders fail closed.** Omit an `--asset-*` flag and the emitted name is
  `SET-ME-…`; the workflow refuses to run that entry instead of downloading
  nothing and reporting green.
- **It runs after publish.** `release: published` is the only trigger that can
  observe the published artifact. Use the pre-publish gate to block, this one to
  detect; a broken release is already out either way, so this job is an alarm,
  not a fence.

Check it locally without publishing, against a release that already exists:

```bash
mkdir -p dist && cd dist
curl -fsSLO https://github.com/OWNER/REPO/releases/download/TAG/mytool-windows-amd64.zip
shipsmoke check --artifact mytool-windows
```

On a Windows host only the windows artifact runs; the others report SKIP with
the reason. That is the correct outcome, not a shortfall — see
[VERIFICATION_LAYERS.md](VERIFICATION_LAYERS.md).

The v1.0 release workflow of this repository demonstrates the full pattern:
build → package → checksum → verify on the host-matching runner → publish.
