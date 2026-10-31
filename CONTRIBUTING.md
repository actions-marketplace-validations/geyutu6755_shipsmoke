# Contributing to ShipSmoke

Thank you for helping make release artifacts more trustworthy. This document
covers the essentials; the project values small, well-tested changes.

## Development setup

Requirements: Go 1.26+ (any platform), git. No other toolchain needed — test
fixtures are cross-compiled with the local Go toolchain inside the tests.

```bash
git clone https://github.com/geyutu6755/shipsmoke
cd shipsmoke
go build ./cmd/shipsmoke
go test ./...
```

## Before opening a PR

1. `go test ./...` passes on your platform.
2. `go vet ./...` is clean and `gofmt -l .` prints nothing.
3. New behavior has tests — including failure paths. Bug fixes need a test
   that reproduces the bug.
4. Security-relevant changes (extraction, execution, env handling) update
   `docs/THREAT_MODEL.md`.
5. Config-facing changes update `docs/CONFIGURATION.md` and the
   `shipsmoke init` example.

## Code layout

```
cmd/shipsmoke/       entrypoint
internal/config/     schema, strict parse, validation
internal/artifact/   format detection, safe extraction, entry location
internal/runner/     process execution, timeout, env construction
internal/engine/     orchestration (config → artifacts → checks → report)
internal/report/     terminal / JSON / GitHub summary rendering, redaction
internal/cli/        commands and exit codes
internal/testfixture cross-compiled fixture binaries for tests
```

Layering rule: `cli → engine → {config, artifact, runner, report}`. No layer
imports upward; `report` does not execute anything.

## Design principles

- Fail honestly: SKIP is not PASS; "unknown" is not a guess.
- No shell in the exec path, ever.
- Every failure message says what happened, where, and what to do next.
- No new third-party dependencies without discussion (the project's only
  dependency today is `gopkg.in/yaml.v3`).

## Commit style

Short imperative subject (`runner: kill process tree on timeout`), blank
line, then motivation and evidence (commands run, results). Reference issues
as `Fixes #N`.

## Reporting bugs

See [ISSUE_TEMPLATE/bug_report.md](.github/ISSUE_TEMPLATE/bug_report.md) —
include the terminal or JSON report (redacted) and `shipsmoke version`
output.

## Security issues

Follow [SECURITY.md](SECURITY.md) — please do not open public issues for
vulnerabilities.
