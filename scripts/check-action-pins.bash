#!/usr/bin/env bash
# check-action-pins.bash — fail when a third-party GitHub Action is referenced
# by a movable tag instead of a full commit SHA.
#
# Why: a tag like @v4 can be re-pointed by the upstream owner at any time, and
# the workflow runs whatever code is behind it with this repository's token.
# Pinning to the commit SHA is the only lockfile GitHub gives you.
#
# Exemptions, deliberately narrow:
#   * ./                     — this repository's own composite action
#   * geyutu6755/shipsmoke   — first-party action, and its published v1 tag is a
#                              documented contract; the fetch step still verifies
#                              the downloaded binary's SHA-256 against the
#                              release's checksums.txt before executing it.
#
# Usage: bash scripts/check-action-pins.bash [files...]
# Exit:  0 no violations, 1 violations found, 2 nothing to check (suspicious).
set -euo pipefail

FIRST_PARTY="geyutu6755/"
SHA_RE='@[0-9a-f]{40}$'

if [ "$#" -gt 0 ]; then
  FILES=("$@")
else
  FILES=(action.yml)
  while IFS= read -r f; do FILES+=("$f"); done < <(find .github/workflows -maxdepth 1 -name '*.yml' 2>/dev/null | sort)
fi

EXISTING=()
for f in "${FILES[@]}"; do
  [ -f "$f" ] && EXISTING+=("$f")
done
if [ "${#EXISTING[@]}" -eq 0 ]; then
  echo "check-action-pins: no files to scan (wrong working directory?)" >&2
  exit 2
fi

VIOLATIONS=0
CHECKED=0
for f in "${EXISTING[@]}"; do
  # "uses: owner/repo@ref" with an optional leading "- " (steps) and comment.
  while IFS= read -r raw; do
    ref="$(printf '%s' "$raw" |
      sed -e 's/^[[:space:]]*-[[:space:]]*//' -e 's/^[[:space:]]*uses:[[:space:]]*//' -e 's/[[:space:]]#.*$//' -e 's/[[:space:]]*$//' -e "s/^['\"]//" -e "s/['\"]\$//")"
    [ -n "$ref" ] || continue
    case "$ref" in
    ./*) continue ;;                            # local action
    "$FIRST_PARTY"*) continue ;;                # our own action, by contract
    esac
    CHECKED=$((CHECKED + 1))
    if printf '%s' "$ref" | grep -qE "$SHA_RE"; then
      continue
    fi
    lineno="$(grep -n -F -- "$ref" "$f" | head -1 | cut -d: -f1)"
    echo "::error::$f:${lineno:-?}: '$ref' is pinned to a movable ref, not a commit SHA"
    VIOLATIONS=$((VIOLATIONS + 1))
  done < <(grep -E '^[[:space:]]*(-[[:space:]]*)?uses:' "$f" || true)
done

echo "check-action-pins: scanned ${#EXISTING[@]} file(s), $CHECKED third-party reference(s), $VIOLATIONS violation(s)"
if [ "$CHECKED" -eq 0 ]; then
  echo "check-action-pins: WARNING - found no third-party references at all; a checker that checks nothing is not a gate" >&2
fi
[ "$VIOLATIONS" -eq 0 ] || exit 1
exit 0
