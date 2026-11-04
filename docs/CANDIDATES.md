# Candidate projects for ShipSmoke — evidence-based research

Status: research only. **No outreach has been sent**, no issues or PRs have been
filed on any repository listed here, and nothing below claims that any of them
use or endorse ShipSmoke.

Evidence dates: 2026-11-01 (first pass, via GitHub pages/API) and 2026-11-02
(re-verification during the v1.0.4 reliability work). Each row says **how** the
claim was established, because the first pass contained errors and honesty about
method is the point of this file.

## What makes a project a good fit

1. Ships prebuilt archives (zip / tar.gz / standalone binary) for several OSes.
2. Users install from GitHub Releases directly, so a bad archive reaches people.
3. Their CI tests the **source build**, not the **published archive** — the gap
   ShipSmoke fills.
4. Small enough that one maintainer will read one message.
5. A release flow simple enough that adding one job is not a rewrite.

Criteria 4 and 5 are the ones the original list got wrong.

## Corrections to the original eight-project table

Re-checked 2026-11-02 against each project's own release page:

| Project | The table said | Actually | How established |
|---|---|---|---|
| jqlang/jq | "no checksums" | ships `sha256sum.txt` **and** `jq-attestation.json` | release page |
| dundee/gdu | "2 archives, no checksums" | ~30 archives (28 `.tgz` + 2 `.exe.zip`) **with** `sha256sums.txt.asc` | release page |
| BurntSushi/ripgrep | "checksums" | one `.sha256` file per artifact, no single manifest | release page |
| junegunn/fzf | "16 archives, sha256 list" | correct; plus 8 `.deb`, asset counter 28 | release page |
| caddyserver/caddy | "standard GoReleaser flow" | GoReleaser **plus** cosign signatures and per-arch SBOMs | release page |

Two of those five rows were materially wrong. Treat the rest of this file as
"verify before quoting", including its own numbers.

## Why the big names are the wrong next target

fzf, caddy, ripgrep, bat, yazi, zoxide and jq are high-visibility projects whose
maintainers field many proposals and, in several cases, already run serious
release automation (GoReleaser, cosign, SBOM, attestations). For them ShipSmoke's
whole pitch reduces to one narrow sentence — "also *run* the artifact, don't only
*sign* it" — which is true but is a harder conversation to have with a stranger
than the tool deserves. They stay here as **visibility references**, not as the
next-phase target list.

## Recommended trial candidates (small/medium, gap verified first-hand)

### 1. [termide/termide](https://github.com/termide/termide) — Rust terminal IDE, 183 stars

- Latest release `0.48.0`, published 2026-10-31.
- Assets confirmed by direct download (HTTP 200 on
  `releases/download/0.48.0/termide-0.48.0-x86_64-pc-windows-msvc.zip`,
  `…-x86_64-unknown-linux-gnu.tar.gz`, `…-aarch64-apple-darwin.tar.gz`).
- **Gap, read from their own `release.yml`:** the AUR and Homebrew update steps
  do `curl -sL ".../releases/download/${VERSION}/termide-…" | sha256sum` and write
  `sha256sums=(…)` into package metadata; the built executable is only `cp`'d into
  `package/` and `Compress-Archive`'d. **Bytes are counted, behaviour is never
  exercised.** Their `ci.yml` runs a multi-OS matrix, but from source.
- No checksums manifest on the release itself.
- **Already trialled locally (Tier 0, see ADOPTION.md):** the released v1.0.3
  binary extracted and ran their real `termide.exe`; a truncated copy failed with
  exit 1. Only candidate with executed proof in hand.

### 2. [mlange-42/git-graph](https://github.com/mlange-42/git-graph) — Rust git graph CLI, 998 stars

- Latest release `v0.8.3`, 2026-10-29. Default branch `master`.
- Assets confirmed by direct download (HTTP 200 on
  `git-graph-v0.8.3-windows-amd64.tar.gz` and the linux equivalent — note the
  **Windows** artifact is a `.tar.gz`, which ShipSmoke handles, but the config
  must not assume `.zip`).
- **Gap, verified mechanically 2026-11-02:** their `release.yml` (91 lines)
  contains **zero** occurrences of `releases/download` and **zero** of
  `sha256`/`checksum`; `tests.yml` (50 lines) runs on `ubuntu-latest` only. So
  nothing anywhere downloads a published archive, and no checksum manifest is
  produced at all.
- Cleanest gap of the three: the missing layer is entirely absent, not partial.

### 3. [mobile-next/mobilecli](https://github.com/mobile-next/mobilecli) — Go device/simulator CLI, 310 stars

- Latest release `1.0.22`, 2026-10-31. Assets confirmed by direct download (HTTP
  200 on the windows/linux/macos `-amd64`/`-arm64` zips; all archives, no tar.gz).
- **Gap, verified mechanically 2026-11-02:** `build.yml` (395 lines) contains
  **eight** `download-artifact` steps and **zero** `releases/download`. Their
  smoke tests run `./mobilecli-linux-amd64 --version` against **CI artifacts**,
  before a release exists.
- This is the most persuasive shape of the argument: the maintainers already
  believe in running a smoke test; the only change is *what the test points at*.
- Caveat: org-backed, busy (runs on `self-hosted` too), so a proposal has to be
  short and specific.

## Not recommended for the first round

| Project | Why not now |
|---|---|
| sharkdp/bat, sxyazi/yazi, ajeetdsouza/zoxide | real gaps, but large maintainer queues; "no checksums" claims were not re-verified here |
| junegunn/fzf, caddyserver/caddy, jqlang/jq, dundee/gdu | adjacent supply-chain tooling already present; pitch narrows to "run, not sign" |
| mobile-adjacent partial adopters (motty, contexere, uteke) | each already does part of it (`gh release download` + checksum + a partial run); the ask becomes "extend what you do", which is a different, longer conversation |
| unixwin/niubash | already built the whole thing (`gh release download` → extract → run). Not a candidate: **evidence the pattern is reachable**, and a fair precedent to cite |

## Honest caveats

- Star counts above were read from each repository page on 2026-11-02; they move.
- "Gap verified" means: the workflow files fetched on 2026-11-02 contain no step
  that downloads and runs a published archive. It does **not** prove no such
  check exists elsewhere (a self-hosted runner script, a manual release habit).
  Before any approach, re-read the current workflow files.
- Release cadence was taken from the latest release page; a project may ship
  again before this list is used.

## Outreach policy

No issues, PRs, emails, or social posts about ShipSmoke have been sent to any of
these projects, and none may be sent without explicit maintainer approval.

1. Fix shipped defects first. Do not invite a stranger to install a tool whose
   released binary crashes on a documented flag.
2. One project at a time, termide first.
3. Channel per their own CONTRIBUTING policy; if unsolicited PRs are discouraged,
   then no PR.
4. Content order: their release matrix → the specific gap quoted from their
   workflow → the local evidence → an offer to wire it up. Authorship disclosed.
5. Accept "no" once, permanently. No follow-up, no re-raise elsewhere.
6. Never open a PR to manufacture a footprint: that is fabricated adoption.
7. Record anything that happens in `docs/ADOPTION.md` under the right tier. A
   trial **we** ran on their artifacts is Tier 0 and is never called adoption.
