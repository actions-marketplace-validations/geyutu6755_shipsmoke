# External trial plan — git-graph and mobilecli

Status: preparation only. **No contact has been made with either project; no
external issues or PRs exist.** This document exists so that, if a
maintainer-approved opportunity appears, the integration work is already
done and verified against public facts.

Date of evidence: 2026-11-05/06, GitHub API (latest releases), 2026-11-05.

## Candidate 1 — git-bahn/git-graph (Rust, 996★)

- Latest release v0.8.3: three tar.gz archives (`git-graph-v0.8.3-{linux,
  macos,windows}-amd64.tar.gz`), **no checksums manifest observed**.
- Users install from GitHub Releases (the README's install section).
- Fit: smallest possible matrix — exactly the three OSes shipsmoke's CI
  template covers; each archive carries the binary at its root.
- Verified facts about the archives (downloaded and inspected locally,
  2026-11-06): linux/windows archives contain `git-graph` / `git-graph.exe`
  at the archive root; the linux tar entry mode is 0755 (packaged on a
  Unix-like environment — no exec-bit risk).
- Reproducible trial config (would live in a fork or a PR if ever invited):

```yaml
version: 1
artifacts:
  - name: git-graph-linux
    path: dist/git-graph-v0.8.3-linux-amd64.tar.gz
    binary: git-graph
    runs_on: [linux/amd64]
    checks:
      - args: ["--version"]
        expect: { exit_code: 0, stdout_contains: ["git-graph"] }
  - name: git-graph-macos
    path: dist/git-graph-v0.8.3-macos-amd64.tar.gz
    binary: git-graph
    runs_on: [darwin/amd64]
    checks:
      - args: ["--version"]
        expect: { exit_code: 0, stdout_contains: ["git-graph"] }
  - name: git-graph-windows
    path: dist/git-graph-v0.8.3-windows-amd64.tar.gz
    binary: git-graph.exe
    runs_on: [windows/amd64]
    checks:
      - args: ["--version"]
        expect: { exit_code: 0, stdout_contains: ["git-graph"] }
```

(Local spot-check executed on Windows, 2026-11-06: downloaded the real
v0.8.3 windows tar.gz and ran the released shipsmoke v1.0.4 check against
this config — `detected: windows/amd64`, `[PASS] git-graph.exe --version`,
exit 0; the linux artifact correctly reported SKIP with its reason.
Note: all three v0.8.3 assets are tar.gz, not zip as an earlier draft of
this document claimed.)

- Maintenance cost once merged: zero — the workflow runs on their existing
  release automation; no secrets, no infra.

## Candidate 2 — mobile-next/mobilecli (Go, 314★)

- Latest release 1.0.22: six per-arch zips (`mobilecli-1.0.22-{linux,
  macos,windows}-{amd64,arm64}.zip`), **no checksums manifest observed**.
- Fit: Go project → `shipsmoke init --ci --stack go --name mobilecli`
  generates the whole pre-publish gate; the per-arch zip layout matches the
  generated `.shipsmoke.yml` shape (runs_on gating per archive).
- Extra angle: they ship darwin/arm64 and linux/arm64 archives, which the
  template verifies on the arm64-capable macos-15 runner and (for linux) via
  the QEMU path shipsmoke's own release pipeline uses.
- Maintenance cost once merged: zero.

## Chosen focus

**git-graph first**: three archives, exact three-OS match, root-level
binaries, large user base installing from Releases, and the absence of a
checksums manifest means the post-release download gate adds a layer the
project currently lacks entirely. mobilecli second (same arguments, plus the
arm64 story).

## What "trial" means (tier discipline)

Per docs/ADOPTION.md: running shipsmoke against these projects' published
assets locally is **Tier 0** (recorded, not adoption). A fork of theirs
running the workflow would still be Tier 0 unless the maintainers merge it
into their own CI — only that is Tier 3.

## If a maintainer ever expresses interest

1. Send the two-file diff (`.shipsmoke.yml` + one workflow) — nothing else.
2. Offer to open the PR on their behalf or walk them through
   `shipsmoke init --ci --stack rust --name git-graph`.
3. Accept "no" immediately; keep the trial record at Tier 0.
