# Candidate projects for ShipSmoke — evidence-based research

Status: research only. No outreach has been sent to any of these projects,
no issues/PRs have been filed on their repositories, and nothing here claims
any of them use or endorse ShipSmoke. Date of evidence: 2026-11-01 (GitHub
API, latest release of each repository).

## What makes a project a good fit

1. Ships prebuilt binaries/archives for multiple platforms (the more manual
   packaging steps, the more the archive contents can drift from the build).
2. Has users who install from GitHub Releases directly (not only via
   package managers).
3. Verifies checksums but (visibly) does not run the packaged artifact on
   target OSes before/after publishing — the gap ShipSmoke fills.
4. Has CI where a three-platform gate can slot in.

## Candidate list (evidence: latest release asset listing, GitHub API)

| Project | Language | Latest tag | Archives | checksums.txt | Notes / fit rationale |
|---|---|---|---|---|---|
| [junegunn/fzf](https://github.com/junegunn/fzf) | Go | v0.74.5 | 16 | yes (sha256 list) | 16 hand-packaged tar.gz for huge platform matrix incl. freebsd/android; huge installer-from-release user base. Any mis-packaged archive reaches users directly. Maintainer already automates distribution (winget workflow present). |
| [caddyserver/caddy](https://github.com/caddyserver/caddy) | Go | v2.11.7 | 18 | yes | High-security-visibility server; 18 archives; standard GoReleaser-style flow. A post-publish re-download + run gate matches its verify culture. |
| [BurntSushi/ripgrep](https://github.com/BurntSushi/ripgrep) | Rust | 15.2.0 | 13 | yes | Large Rust release matrix; cargo-driven packaging; releases consumed directly by many users. |
| [sharkdp/bat](https://github.com/sharkdp/bat) | Rust | v0.26.1 | 13 + debs | no checksums manifest observed | Same shape as ripgrep; no checksum manifest in latest release — download-integrity layer would be step one, run-verification step two. |
| [sxyazi/yazi](https://github.com/sxyazi/yazi) | Rust | v26.9.1 | 11 (+debs) | no checksums manifest observed | Very active file manager with per-target zip/tar mix; direct-release installers prominent in README. |
| [ajeetdsouza/zoxide](https://github.com/ajeetdsouza/zoxide) | Rust | v0.10.0 | 12 | no checksums manifest observed | Popular shell addon installed from releases via install scripts. |
| [jqlang/jq](https://github.com/jqlang/jq) | C | jq-1.8.2 | 2 (plus raw binaries) | no | Ships raw standalone binaries — exactly the "standalone binary" artifact class ShipSmoke verifies. C toolchain demonstrates language-independence. |
| [dundee/gdu](https://github.com/dundee/gdu) | Go | v5.38.0 | 2 | no | Smaller matrix, single-maintainer — low-effort adoption candidate via `init --ci`. |

## Honest caveats

- "No checksums manifest observed" reflects only the latest release's asset
  list via the API at the date above; some projects verify differently
  (sigstore, attestations, CI-internal checks).
- Several of these projects have sophisticated release automation already;
  ShipSmoke's proposition to them is specifically the *run the artifact on
  real OSes* layer, not integrity tooling they may already have.
- No ranking implies criticism: all eight ship high-quality releases. The
  list exists to focus outreach (when approved) where the fit is real.

## Outreach policy

No issues, PRs, emails, or social posts about ShipSmoke have been sent to
any of these projects. First contact, if ever made, should be: one short,
personalized message per project, leading with the project's own release
matrix, offering the integration repo as a live example, and accepting a
"no" gracefully.
