#!/usr/bin/env bash
# fetch-shipsmoke.sh — resolve, download, VERIFY and extract a shipsmoke
# release asset for the composite GitHub Action.
#
# Security design:
#   * every input arrives through an environment variable, never through
#     interpolated GitHub Actions expressions, so hostile input cannot break
#     out of quoting (the whole reason this file exists);
#   * the downloaded asset is checksummed against the release's own
#     checksums.txt and is extracted only after the hash matches — a failed
#     download, missing checksum entry, or mismatch aborts before anything
#     is extracted or executed;
#   * version must be "latest" or a full release tag "vX.Y.Z" — mutable
#     moving references like "v1" are rejected here on purpose;
#   * the script itself never executes the downloaded file.
#
# Inputs (environment):
#   SHIPS_VERSION            release tag "vX.Y.Z" or "latest" (default latest)
#   SHIPS_BINARY_PATH        when set and a real file, skip download entirely
#   SHIPS_RELEASE_BASE_URL   override for tests (default: official release URL)
#   SHIPS_RELEASE_LATEST_URL override for tests (default: official latest URL)
#   GITHUB_OUTPUT            step-output file; written only when set
#   RUNNER_TEMP              persistent temp dir; defaults to a mktemp dir
#
# Outputs (GITHUB_OUTPUT):
#   path      absolute path of the shipsmoke binary to execute
#   version   resolved release tag ("user-provided" for binary-path mode)
#   verified  "checksum" | "user-provided"
set -euo pipefail

OWNER_REPO="${SHIPS_REPO:-geyutu6755/shipsmoke}"
BASE_URL="${SHIPS_RELEASE_BASE_URL:-https://github.com/${OWNER_REPO}/releases/download}"
LATEST_URL="${SHIPS_RELEASE_LATEST_URL:-https://github.com/${OWNER_REPO}/releases/latest}"

log() { printf '[shipsmoke-action] %s\n' "$*"; }
die() { printf '::error::%s\n' "$*" >&2; exit 2; }

write_output() {
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    printf '%s=%s\n' "$1" "$2" >>"$GITHUB_OUTPUT"
  fi
}

# --- user-provided binary passthrough (no download, nothing executed here) --
if [ -n "${SHIPS_BINARY_PATH:-}" ]; then
  # Literal path comparison only: the value is never re-parsed by a shell.
  if [ ! -f "$SHIPS_BINARY_PATH" ]; then
    die "binary-path '$SHIPS_BINARY_PATH' does not exist or is not a regular file"
  fi
  write_output path "$SHIPS_BINARY_PATH"
  write_output version "user-provided"
  write_output verified "user-provided"
  log "using user-provided binary: $SHIPS_BINARY_PATH (integrity is the caller's responsibility)"
  exit 0
fi

# --- version pinning ---------------------------------------------------------
# Strict grammar: "latest" or "vX.Y.Z" with X/Y/Z as plain integers without
# leading zeros. Moving refs (v1, v1.0), prerelease/build suffixes
# (v1.0.2-rc1, +build) and padding (v01.02.03) are all rejected here — the
# download URL must be an exact, immutable release asset path.
VERSION="${SHIPS_VERSION:-latest}"
[ -n "$VERSION" ] || VERSION="latest"
is_exact_tag() {
  printf '%s' "$1" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
}
if [ "$VERSION" != "latest" ] && ! is_exact_tag "$VERSION"; then
  die "version '$VERSION' is not accepted: use 'latest' or a full release tag 'vX.Y.Z' (moving refs like 'v1', prerelease suffixes and leading zeros are rejected; pin the exact release for reproducible verification)"
fi

TAG="$VERSION"
if [ "$VERSION" = "latest" ]; then
  TAG="$(curl -fsSL --retry 3 --retry-delay 2 -o /dev/null -w '%{url_effective}' "$LATEST_URL" | sed 's#.*/tag/##')"
  is_exact_tag "$TAG" || die "could not resolve the latest release tag (got '$TAG')"
  log "resolved latest release: $TAG"
fi

# --- platform detection -------------------------------------------------------
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$OS" in
  linux*) OS="linux" ;;
  darwin*) OS="darwin" ;;
  mingw* | msys* | cygwin*) OS="windows" ;;
  *) die "unsupported operating system '$OS'" ;;
esac
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *) die "unsupported architecture '$ARCH'" ;;
esac
EXT="tar.gz"
[ "$OS" = "windows" ] && EXT="zip"
ASSET="shipsmoke_${OS}_${ARCH}.${EXT}"

# --- download asset AND its manifest (fail closed on either) -------------------
DEST="${RUNNER_TEMP:-$(mktemp -d)}/shipsmoke-action"
mkdir -p "$DEST"
curl -fsSL --retry 3 --retry-delay 2 -o "$DEST/$ASSET" "$BASE_URL/$TAG/$ASSET" ||
  die "download failed: $TAG/$ASSET"
curl -fsSL --retry 3 --retry-delay 2 -o "$DEST/checksums.txt" "$BASE_URL/$TAG/checksums.txt" ||
  die "download failed: $TAG/checksums.txt (a release without checksums is refused, not executed)"

# --- verify SHA-256 against the manifest ---------------------------------------
EXPECTED="$(grep -E "^[0-9a-fA-F]{64}[[:space:]][ *]?${ASSET}\$" "$DEST/checksums.txt" | head -1 | awk '{print $1}' || true)"
if [ -z "$EXPECTED" ]; then
  die "$ASSET is not listed in checksums.txt; refusing to execute an unverified file"
fi
case "$EXPECTED" in
  [0-9a-fA-F]*) ;;
  *) die "malformed checksum entry for $ASSET" ;;
esac
hex_of() {
  # Different sha256sum builds decorate output differently (binary-mode
  # markers, path suffixes); extract the 64-hex-digit digest robustly.
  "$@" | grep -Eo '[0-9a-fA-F]{64}' | head -1
}
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(hex_of sha256sum "$DEST/$ASSET")"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(hex_of shasum -a 256 "$DEST/$ASSET")"
else
  die "no SHA-256 tool available (need sha256sum or shasum)"
fi
if [ "$ACTUAL" != "$EXPECTED" ]; then
  die "SHA-256 mismatch for $ASSET: expected $EXPECTED, got $ACTUAL; refusing to execute a tampered or corrupted file"
fi
log "checksum verified: $ASSET = $EXPECTED"

# --- extract (only after verification) ------------------------------------------
OUT="$DEST/bin"
mkdir -p "$OUT"
if [ "$EXT" = "zip" ]; then
  if command -v unzip >/dev/null 2>&1; then
    unzip -q "$DEST/$ASSET" -d "$OUT" || die "extraction of $ASSET failed"
  elif command -v python >/dev/null 2>&1; then
    # zipfile.extractall drops ".." components and absolute prefixes.
    python - "$DEST/$ASSET" "$OUT" <<'PYEOF' || die "extraction of $ASSET failed"
import sys, zipfile
zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])
PYEOF
  else
    die "no zip extraction tool available (need unzip or python)"
  fi
else
  tar -xzf "$DEST/$ASSET" -C "$OUT" || die "extraction of $ASSET failed"
fi
BIN="$OUT/shipsmoke"
[ "$OS" = "windows" ] && BIN="$OUT/shipsmoke.exe"
if [ ! -f "$BIN" ]; then
  die "extracted archive does not contain the shipsmoke binary"
fi
chmod +x "$BIN" 2>/dev/null || true
rm -f "$DEST/$ASSET" "$DEST/checksums.txt"

write_output path "$BIN"
write_output version "$TAG"
write_output verified "checksum"
log "ready: $BIN ($TAG, checksum-verified)"
