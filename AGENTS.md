# AGENTS.md — ShipSmoke maintenance context

Read this before working on ShipSmoke. It records facts that were expensive to
establish and are not obvious from the code. Verified on 2026-11-01 during a
takeover audit; each claim below was reproduced locally unless marked otherwise.

## What this project is

ShipSmoke is a Go CLI + GitHub composite action that verifies a project's
**release artifact as users actually download it**: detect the container from
magic bytes, extract it under explicit security limits, identify the entry
binary from its real header (ELF/PE/Mach-O), gate it against the host platform,
then **execute it** and assert on exit code / stdout / stderr. It is a quality
gate, not a sandbox.

Current release: **v1.0.3** (annotated tag → commit `88cb63e`).
Module: `github.com/geyutu6755/shipsmoke`, single dependency `gopkg.in/yaml.v3`.

## Non-negotiable project rules

- This is a **continuation**, not a rebuild. Never squash, rebase, force-push,
  delete tags, or recreate history. Existing tags `v1`, `v1.0.0`–`v1.0.3` and
  all Releases must be preserved.
- The project's brand is honesty about evidence. Do not write "verified" for
  anything not actually run. Do not describe Tier 1/Tier 2 self-testing as
  adoption (see the tier model in `docs/ADOPTION.md`; Tier 3/4 = none yet).
- Do not file issues/PRs or post about ShipSmoke in third-party repositories
  without explicit maintainer approval.
- Third-party Actions are pinned to full commit SHAs in every workflow **and in
  `action.yml`**. `scripts/check-action-pins.bash` enforces it in CI; a doc
  snippet using `actions/checkout@v4` is a defect, not a style.

## Environment prerequisites

Portable facts about building and testing this project; where your own toolchain
lives is yours to record locally, not here.

- Go **>= 1.26.2** is required (`go.mod` line, and GO-2026-4869 was caught by the
  `govulncheck` gate). A Go too old to satisfy `go 1.26.2` fails the build.
- `go test -race` needs cgo, so it needs a C toolchain. On a machine without
  `gcc`, `-race` errors out with "requires cgo" — that is not a test failure.
  Race and fuzz coverage are Linux-CI-only by construction.
- `gh` is needed for `gh attestation verify`; the rest of the checks use `curl`.
- GitHub renders release asset lists with JavaScript, so confirming an asset
  exists means `curl -o /dev/null -w "%{http_code}"` against its
  `releases/download/...` URL, not scraping the page.
- Keep test output in English: consoles on some Windows setups are not UTF-8 and
  non-ASCII log text gets mangled.

## Commands

```bash
go build ./...                      # build
go test ./...                       # full suite (~90s warm, slower cold)
go test -cover ./...                # per-package coverage
gofmt -l .                          # must print nothing
go vet ./...                        # must be clean
go build -o shipsmoke.exe ./cmd/shipsmoke
bash scripts/fetch-shipsmoke_test.bash    # action fetch-step suite, runs locally under bash
bash scripts/check-action-pins.bash       # third-party action pinning gate
```

Release builds inject version info via ldflags; a plain `go build` instead
falls back to VCS-derived version (`v1.0.4-0.<date>-<sha>`), which is expected
and not a bug.

## Verification status — what is proven versus CI-only

Proven by direct execution (Windows, 2026-11-01/02):

- `go build`, `go vet`, `gofmt -l`, `go test ./...` pass; `go test -cover` gives
  artifact 62.7%, cigen 97.8%, cli 59.6%, config 91.7%, engine 84.6%,
  report 83.6%, runner 84.6%.
- Released asset `shipsmoke_windows_amd64.zip` downloads, matches `checksums.txt`,
  and the extracted binary self-reports `v1.0.3 commit 88cb63e`.
- `examples/demo-broken-release.sh` with the released binary: good artifact PASS,
  truncated zip ERROR → exit 1.
- `scripts/fetch-shipsmoke_test.bash` runs locally (30 cases) and
  `scripts/check-action-pins.bash` passes on the checked-in YAML.
- Tier 0 trial: v1.0.3 extracted and ran termide 0.48.0's real published Windows
  archive; their linux/darwin archives correctly reported SKIP.

Never claim these as verified from a laptop: macOS/Linux execution, `-race`,
fuzzing, `gh attestation verify`, execution of our own darwin/linux assets, and
`windows/arm64` (the release pipeline marks it `build-only` and does not execute
it — see `release.yml`). Anything cross-platform is CI evidence, and CI evidence
means a run link on that commit, not an assumption.

## Defect register

Fixed on `fix/v1.0.4-reliability`, awaiting review — do not re-report these as
open, and do not re-introduce them:

- **P0 shipped crash:** `check --quiet` panicked (nil `*Redactor` reached
  `report.Redactor.Apply` through a render into `io.Discard` that served no
  purpose). The discarded render is gone and `Apply` now tolerates a nil
  receiver. Regression tests cover PASS/FAIL/ERROR/SKIP/NO-CHECKS, the JSON
  report and exit codes. It reproduced with the **released** v1.0.3 binary.
- **P1 broken help:** every `shipsmoke <command> --help` printed
  `flag: help requested` because `fs.SetOutput(io.Discard)` swallowed the usage
  while `flag.ErrHelp` was surfaced as an error. Help now prints to stdout with
  exit 0; real parse errors still print to stderr with exit 2, and a test asserts
  the two are not conflated.
- **P1 unrunnable post-release recipe:** replaced by a generated
  `release: published` gate (`init --ci --published`), and
  `docs/CONFIGURATION.md` carries the generator's real output plus a command-line
  reference that did not exist before.
- **P1 unpinned ref in `action.yml`:** `actions/upload-artifact@v4` was the only
  movable third-party ref in live code — in the file outside callers execute. Now
  SHA-pinned and guarded by `scripts/check-action-pins.bash` in CI.
- **P2 per-check signal leak:** `withSignalContext()` registered a notifier and
  goroutine per check, discarded `cancel`, and never ran `signal.Stop` on normal
  completion. Now one handler per run with explicit teardown.
- **P3 warning routing:** `env: inherit` wrote to raw `os.Stderr`; it now honours
  `Options.Stderr`.
- **P3 manifest lookup in the fetch script:** the asset name was interpolated
  into `grep -E`, so its dots matched any byte and a lookalike manifest line was
  accepted (proved: pre-fix script extracted it, exit 0). Now a literal field
  comparison with a regression test.
- **Cleanup:** root `fetched-ci.yml` removed — dead copy of the harness workflow,
  outside `.github/workflows/` so never executed, building a `teapot` that does
  not exist here.

Still open, by construction rather than oversight:

- `windows/arm64` ships unexecuted (`build-only`); the job summary says so.
- `-race` runs in CI only, and only from this branch onward — before it, no
  release had ever been race-checked (needs cgo; the dev machine has no C
  compiler and CI never invoked it). Fuzzing is Linux-CI-only for the same
  cgo reason.
- The pinning script checks ref **form**, not that a SHA matches its `# vN`
  comment — labels were verified by hand against `git ls-remote --tags`.
- **Tier 3 adoption is zero.** A trial we run on someone else's artifacts is
  Tier 0 (see `docs/ADOPTION.md`) and must never be described as adoption.

## Architecture map

- `cmd/shipsmoke/main.go` — entrypoint, ldflags version + VCS fallback.
- `internal/cli` — command dispatch (`check`/`validate`/`init`/`version`/`help`),
  flags, exit codes.
- `internal/config` — `.shipsmoke.yml` schema v1, strict `KnownFields(true)`
  parsing, defaults (20000 files, 1 GiB total, 512 MiB/file, ratio 1000, 60s).
- `internal/artifact` — `detect.go` magic bytes + ELF/PE/Mach-O identity;
  `extract.go` security limits (Zip Slip, traversal, reserved names, symlink/
  hardlink/device rejection, size/ratio caps, exec-bit normalisation, gzip CRC);
  `locate.go` entry-binary lookup.
- `internal/engine` — orchestration, platform gate, exit codes
  (0 pass / 1 fail / 2 usage / 3 nothing executed).
- `internal/runner` — no-shell argv execution, process-group kill on Unix
  (`kill_unix.go`), `taskkill /F /T` on Windows (`kill_windows.go`), 1 MiB
  output cap, env building with secret-shaped var filtering.
- `internal/report` — JSON + terminal + GitHub step summary, secret redaction.
- `internal/cigen` — two template families: `init --ci --stack` (pre-publish
  build gate for go|rust|c) and `init --ci --published` (post-release
  download-and-run gate, per-platform asset names, `SET-ME-` placeholders that
  the emitted workflow refuses to execute).
- `internal/testfixture` — cross-compiles real fixture binaries.
- `action.yml` + `scripts/fetch-shipsmoke.sh` — composite action; inputs pass via
  env only (never interpolated into `run`), fetch step refuses to execute an
  asset missing from `checksums.txt`.
- OS-specific tests: `unix_test.go` (`!windows`) provides a real `processAlive`,
  so `TestRunTimeoutKillsTree` only runs on Unix; `windows_test.go` stubs it.

Sibling repos: `geyutu6755/shipsmoke-integration` (Tier 2 harness, pins
`shipsmoke@v1` with `version: v1.0.3`), `geyutu6755/homebrew-tap`
(`Formula/shipsmoke.rb` at 1.0.3, hashes match release `checksums.txt`,
`install-test` workflow green), `geyutu6755/scoop-bucket`
(`bucket/shipsmoke.json` at 1.0.3, windows hashes match).

## Release flow

Tag `vX.Y.Z` → `release.yml` cross-builds 6 targets, per-target verification is
native / QEMU container / Rosetta / build-only, `checksums.txt` is assembled and
then independently re-hashed before publishing, provenance attestation is
generated, and the job summary records which targets were not executed. Before
tagging: confirm the CI on the commit is green (including the new
`Dependency pinning` job), confirm `go test ./...` and both bash suites pass
locally, and re-read the "still open" list above so the release notes do not
claim more than was run.
