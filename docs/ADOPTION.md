# How ShipSmoke adoption is measured

To keep claims honest, adoption is tracked in four separate tiers. Anything
not in tier 3 or 4 must never be described as "used by" anyone.

## Tier 0 — Local trial by the maintainer against someone else's artifacts

Running ShipSmoke against a third party's **published** artifacts on our own
machine, with no contact and no commitment from them.

- Evidence: recorded below, per trial.
- Counts as: proof the tool works on a real-world artifact shape.
- **Never** counts as adoption, and never as "project X uses ShipSmoke".

## Tier 1 — Self-verification (implemented)

ShipSmoke verifies its own release artifacts on real runners before every
publish.

- Evidence: the release workflow's verify steps and job summaries.
- Counts as: engineering quality. **Never** counts as adoption.

## Tier 2 — Independent integration harness (implemented)

A separate repository, written as an outside consumer, exercises the action
end-to-end on all three operating systems, including a deliberate
broken-artifact refusal.

- Evidence: https://github.com/geyutu6755/shipsmoke-integration (Actions runs).
- Counts as: proof the onboarding path works for a stranger's-shaped
  project. **Never** counts as adoption.

## Tier 3 — Third-party independent usage (the real thing)

A repository not owned or controlled by the shipsmoke maintainer commits the
action or the CLI into their own CI, on their own initiative.

- How to detect: external repos referencing `geyutu6755/shipsmoke` in their
  workflow files (GitHub code search), fork activity initiated externally,
  or issues/PRs opened by others.
- How it will be recorded: a table in this file — repository, first commit
  date, link — added only with the maintainer's knowledge (e.g. after they
  comment on an issue or the dependency is publicly visible in their default
  branch).

## Tier 4 — Sustained external use

Tier 3 usage that persists: repeated workflow runs over ≥ 3 months, or an
external maintainer explicitly stating they adopted it (with link).

## Current status

| Tier | State |
|---|---|
| 0 | one trial: termide 0.48.0, verified locally on Windows (see below) |
| 1 | yes — every release |
| 2 | yes — harness exists and its **latest** run is green on all three OSes. Not "all green since v1.0.2": the harness history contains failed runs (including one `verify against shipsmoke v1.0.3` run) while the workflow itself was being fixed. Wording corrected 2026-11-01 after checking the run list. |
| 3 | none yet (honest baseline) |
| 4 | none yet |

## Tier 0 trial record

| date | project | artifact | host | what ran | result |
|---|---|---|---|---|---|
| 2026-11-01 | termide/termide 0.48.0 | `termide-0.48.0-x86_64-pc-windows-msvc.zip` (downloaded from their Release) | windows/amd64, released shipsmoke v1.0.3 | extracted, then executed `termide.exe --version` and `--help` | PASS, 2 checks, exit 0; linux/darwin artifacts reported SKIP with reason |
| 2026-11-01 | same, truncated to 50% | local copy (simulated partial upload — **not** a claim about their release) | windows/amd64 | extraction only | ERROR `not a valid zip file`, exit 1 |

No contact was made with the termide maintainers and nothing here implies they
know about ShipSmoke.

## First external users — concrete plan

1. Make onboarding as close to zero as possible: `shipsmoke init --ci` and
   the three tutorials (done in v1.0.3).
2. Ship the Homebrew tap / Scoop bucket (done in v1.0.3) — installs from the
   channels Rust/Go CLI users already check.
3. Publish the action to the GitHub Marketplace (one manual step per major
   release; see docs/SUPPLY_CHAIN.md) — Marketplace search is where actions
   are discovered.
4. Work the evidence-based candidate list (docs/CANDIDATES.md): when there
   is explicit appetite (e.g. a project asks), help wire it up personally —
   one project, done well, is worth more than any bulk outreach.
5. Post the announcement (docs/INTRODUCTION.md) where the maintainer has a
   genuine presence (own blog/reddit HN with full disclosure of authorship),
   leading with the broken-release demo rather than the tool.
6. Track Tier 3 signals monthly; report honestly in this file.
