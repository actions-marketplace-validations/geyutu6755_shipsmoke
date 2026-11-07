# Third-party integration template

Copy-paste starting point for verifying **another project's** release
artifacts with ShipSmoke on real runners. This is the shape proven by
`shipsmoke-integration` (teapot) and the git-graph trial
(`.github/workflows/gitgraph-trial.yml`).

## What to change

1. `asset:` — the release asset names of the project you verify.
2. `binary:` — the entry binary path inside each archive.
3. `runs_on:` — the platforms each archive targets.
4. `stdout_contains:` — something the binary actually prints.

## Minimal pre-publish gate (build → package → verify → publish)

```yaml
name: shipsmoke
on:
  push:
    branches: [main]
  pull_request:
permissions:
  contents: read
jobs:
  verify:
    name: Verify release artifact (${{ "{{ matrix.os }}" }})
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-24.04, macos-15, windows-2025]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262 # v4
      - name: Build and package (adapt to the project's real build)
        shell: bash
        run: |
          set -euo pipefail
          EXE=""
          [ "$RUNNER_OS" = "Windows" ] && EXE=".exe"
          mkdir -p dist
          go build -o "dist/tool$EXE" .
          if [ "$RUNNER_OS" = "Windows" ]; then
            powershell -NoProfile -Command "Compress-Archive -Path dist/tool.exe -DestinationPath dist/tool.zip -Force"
          else
            tar -czf dist/tool.tar.gz -C dist tool
          fi
      - uses: geyutu6755/shipsmoke@v1
        with:
          config: .shipsmoke.yml
          report: dist/shipsmoke-report.json
          version: v1.0.4
      - name: Publish only if verification passed
        if: success()
        run: echo "attach dist/ to the release here"
```

Or generate it: `shipsmoke init --ci --stack go|rust|c --name <tool>`.

## Post-release gate (download the published asset and run it)

`shipsmoke init --ci --published --name <tool> --asset-linux <name>
--asset-darwin <name> --asset-windows <name>` generates a workflow that
fires on `release: published` (and accepts `workflow_dispatch` with
`inputs.tag`). Commit it **before** the release is published: the event
fires at release creation, not at asset-upload completion.

## Honest-outcome notes

- Each platform runs its own artifact; others appear as SKIP with reasons.
- A macos-15 runner is arm64: darwin/amd64 archives are refused by the
  platform gate (recorded as SKIP, never as PASS) and need an explicit
  Rosetta step for direct execution evidence.
- A failed verification fails the job (exit 1); nothing executed exits 3;
  config errors exit 2.
