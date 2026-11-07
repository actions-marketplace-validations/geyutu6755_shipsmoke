# Maintenance guide

Everything a future maintainer (human or agent) needs to safely take over
ShipSmoke. Read [AGENTS.md](../AGENTS.md) first, then this file.

## Repository layout

- Main repo: this one. Channel repos: `geyutu6755/homebrew-tap`
  (`Formula/shipsmoke.rb`), `geyutu6755/scoop-bucket`
  (`bucket/shipsmoke.json`), `geyutu6755/shipsmoke-integration`
  (Tier 2 harness: `ci.yml` teapot gate, `shipsmoke-published.yml`
  post-release gate, `gitgraph-trial.yml` Tier 0 trial).

## Daily commands

```bash
go build ./...                 # build (needs Go >= 1.26.2, see go.mod)
gofmt -l .                     # must print nothing
go vet ./...                   # must be clean
go test ./...                  # full suite (~90s warm)
go test -cover ./...           # per-package coverage
bash scripts/fetch-shipsmoke_test.bash   # action fetch suite (30 cases)
bash scripts/check-action-pins.bash      # third-party Action pinning gate
```

Plain `go build` binaries report a VCS-derived version
(`v1.0.X-0.<date>-<sha>`); that is expected. Release binaries get exact
versions from ldflags.

`go test -race` needs cgo (a C toolchain). Without gcc it errors with
"requires cgo" — not a test failure. Race/fuzz run in Linux CI by design.

## Fixing a bug

1. Reproduce against the **latest released binary** first; if it reproduces,
   it is a shipped defect — record it in AGENTS.md's defect register.
2. Branch `fix/<version>-<topic>`; write the failing test before the fix.
3. `go build && gofmt -l . && go vet ./... && go test ./...` locally.
4. Push; confirm CI green (three OS + race + fuzz + govulncheck + pinning +
   docs-drift); open/merge a PR with history preserved (no squash of public
   commits).
5. Update CHANGELOG under a new `## [x.y.z]` header or Unreleased.

## Releasing a new version

1. Confirm main CI green on the release commit (check the run's head_sha —
   short SHAs in logs can collide; use the full SHA via the API).
2. `git tag -a vX.Y.Z <commit> -m "…"` then `git push origin vX.Y.Z`.
3. `release.yml` cross-builds 6 targets, verifies host-matching targets
   (native / QEMU for linux/arm64 / Rosetta for darwin/amd64; windows/arm64
   is build-only and recorded as such), re-hashes every asset against
   checksums.txt, generates provenance attestations, publishes.
4. If a job is cancelled by macOS runner starvation, re-run
   `rerun-failed-jobs` (legitimate infra retry — distinguish cancelled from
   code-failed by checking the job log for an actual step failure).
5. Verify: release assets = 7; download your platform's archive, check
   SHA-256 against checksums.txt, run `shipsmoke version` (must print the
   new tag and the release commit); `go install …@vX.Y.Z` reports the tag.
6. Update the `v1` moving ref **only after** the release is verified:
   back up the old target (note it in the release/acceptance doc), then
   `PATCH /repos/…/git/refs/tags/v1 {"sha": "<release commit>", "force": true}`.
   Never touch immutable `vX.Y.Z` tags.
7. Update channels (below) and confirm their install-test runs go green.

## Updating channels

- homebrew-tap: bump `version` in `Formula/shipsmoke.rb` and replace the
  four sha256 values with the new release's `checksums.txt` entries
  (darwin_amd64, darwin_arm64, linux_amd64, linux_arm64 — keep position
  mapping).
- scoop-bucket: bump `version` and `url`s in `bucket/shipsmoke.json`
  (windows_amd64, windows_arm64 hashes; JSON must stay valid).
- shipsmoke-integration: bump `version:` (and the resolved-version
  assertion) in `.github/workflows/ci.yml` and
  `.github/workflows/shipsmoke-published.yml`.
- Push each; their `install-test`/`integration` workflows must go green
  before claiming the channel updated.

## Verifying published assets (post-release)

The self-hosted gate: `gh workflow run shipsmoke-published.yml -f tag=vX.Y.Z`
in the main repo, or the `release: published` trigger on the next release.
It downloads the exact assets, checks SHA-256 against `checksums.txt`,
extracts, and runs the binary — one job per OS. Read VALIDATION_REPORT §6f
for the verified evidence pattern.

## Handling security issues

- Report path: SECURITY.md (advisory or a SECURITY-labelled issue).
- Add a negative test with any fix (see corruption_test.go and
  cli_regressions_test.go for the pattern: reproduce first, then fix).
- Update docs/THREAT_MODEL.md if the trust boundary changes.

## CI failure recovery

1. Read the failing **job log**, not just the conclusion.
2. `cancelled` on macOS jobs = runner starvation (seen repeatedly) — rerun
   failed jobs; if it recurs, run one workflow at a time.
3. 403 on log-download redirects = token scope/redirect artifact; use curl
   with the token header (the API redirect forwards headers badly).
4. Never convert a cancelled/failed run into a claim of success.

## Docs that must not drift

- `TestDocsConfigurationMatchesGenerator` fails when
  docs/CONFIGURATION.md's snippet diverges from `internal/cigen` output.
- CHANGELOG: new `[x.y.z]` entries under the header; Unreleased becomes
  formal only at release.
- AGENTS.md defect register: update when defects ship, get fixed, or move.
