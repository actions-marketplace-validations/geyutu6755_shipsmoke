# How ShipSmoke adoption is measured

To keep claims honest, adoption is tracked in four separate tiers. Anything
not in tier 3 or 4 must never be described as "used by" anyone.

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
| 1 | yes — every release |
| 2 | yes — all green runs since v1.0.2 |
| 3 | none yet (honest baseline at v1.0.3) |
| 4 | none yet |

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
