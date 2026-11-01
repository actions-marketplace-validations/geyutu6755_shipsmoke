#!/usr/bin/env bash
# fetch-shipsmoke_test.bash — security and behavior tests for
# scripts/fetch-shipsmoke.sh. Runs on every platform GitHub provides bash
# for (linux, macos, windows via Git Bash).
#
# Coverage:
#   * script-injection payloads in every env input must never execute
#   * malformed versions (moving refs, partial tags, traversal) are rejected
#   * missing checksums.txt / unlisted asset / hash mismatch abort BEFORE
#     extraction or execution
#   * happy path: correct hash → extracted, outputs written
#   * binary-path passthrough: existing file OK, injection payload inert
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FETCH="$SCRIPT_DIR/fetch-shipsmoke.sh"

WORK="$(mktemp -d)"
PWNED="$WORK/pwned-$$.marker"
FAILURES=0
PASS_COUNT=0

cleanup() {
  stop_server
  # Windows locks dirs of recently-killed processes; tolerate failures.
  rm -rf "$WORK" 2>/dev/null || true
}
trap cleanup EXIT

run_case() { # name, expect_exit, env-assignments...
  local name="$1" expect="$2"
  shift 2
  local out_file="$WORK/out-$name.txt"
  local env_file="$WORK/env-$name.txt"
  : >"$out_file"
  : >"$env_file"
  local rc=0
  # /usr/bin/env explicitly: a shadowing `env` earlier on PATH may silently
  # drop assignments (seen on Windows dev machines).
  /usr/bin/env GITHUB_OUTPUT="$env_file" "$@" \
    bash "$FETCH" >"$out_file" 2>&1 || rc=$?
  if [ "$rc" -ne "$expect" ]; then
    echo "FAIL: $name (exit $rc, want $expect)"
    sed 's/^/    | /' "$out_file" | head -10
    FAILURES=$((FAILURES + 1))
    return 1
  fi
  PASS_COUNT=$((PASS_COUNT + 1))
  echo "ok: $name"
  return 0
}

assert_no_pwned() {
  if [ -f "$PWNED" ]; then
    echo "FAIL: injection payload EXECUTED (pwned marker exists)"
    FAILURES=$((FAILURES + 1))
    return 1
  fi
  PASS_COUNT=$((PASS_COUNT + 1))
  return 0
}

assert_output_line() { # file, key, expected-prefix
  local file="$1" key="$2" want="$3"
  if grep -q "^${key}=${want}" "$file"; then
    PASS_COUNT=$((PASS_COUNT + 1))
    return 0
  fi
  echo "FAIL: expected ${key}=${want} in $file"
  cat "$file"
  FAILURES=$((FAILURES + 1))
  return 1
}

# ---------------------------------------------------------------------------
echo "== binary-path mode =="
INJECT='x; touch '"$PWNED"'; #'
run_case binpath-injection 2 \
  SHIPS_BINARY_PATH="$INJECT"
assert_no_pwned
run_case binpath-missing 2 \
  SHIPS_BINARY_PATH="$WORK/does-not-exist-$$"
touch "$WORK/fake-binary-$$"
run_case binpath-ok 0 \
  SHIPS_BINARY_PATH="$WORK/fake-binary-$$"
assert_output_line "$WORK/env-binpath-ok.txt" "path" "$WORK/fake-binary-$$"
assert_output_line "$WORK/env-binpath-ok.txt" "verified" "user-provided"
assert_no_pwned

# ---------------------------------------------------------------------------
echo "== version validation =="
run_case version-moving-ref 2 "SHIPS_VERSION=v1"
run_case version-partial 2 "SHIPS_VERSION=v1.0"
run_case version-no-v 2 "SHIPS_VERSION=1.0.0"
run_case version-traversal 2 "SHIPS_VERSION=../v1.0.0"
run_case version-prerelease 2 "SHIPS_VERSION=v1.0.2-rc1"
run_case version-build-meta 2 "SHIPS_VERSION=v1.0.2+build.5"
run_case version-leading-zero 2 "SHIPS_VERSION=v01.0.2"
run_case version-space-suffix 2 "SHIPS_VERSION=v1.0.2 "
run_case version-ok-shape-then-fail 2 "SHIPS_VERSION=v9.9.9"
run_case version-injection-1 2 "SHIPS_VERSION=v1.0.0\"; touch $PWNED; #"
run_case version-injection-2 2 'SHIPS_VERSION=$(touch '"$PWNED"')'
assert_no_pwned

# ---------------------------------------------------------------------------
echo "== download failure handling =="
run_case download-refused 2 \
  SHIPS_VERSION='v9.9.9' \
  SHIPS_RELEASE_BASE_URL='http://127.0.0.1:1/nope'
assert_no_pwned

# ---------------------------------------------------------------------------
echo "== local release server: tamper and success =="
HOST_OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
HOST_ARCH="$(uname -m)"
case "$HOST_OS" in linux*) P_OS=linux ;; darwin*) P_OS=darwin ;; mingw*|msys*|cygwin*) P_OS=windows ;; esac
case "$HOST_ARCH" in x86_64) P_ARCH=amd64 ;; aarch64|arm64) P_ARCH=arm64 ;; esac
EXT="tar.gz"; [ "$P_OS" = "windows" ] && EXT="zip"
ASSET="shipsmoke_${P_OS}_${P_ARCH}.${EXT}"
TAG="v9.9.99"
SRV_ROOT="$WORK/release/$TAG"
mkdir -p "$SRV_ROOT"

# fake but structurally valid archive
FAKE_BIN="$WORK/shipsmoke"
[ "$P_OS" = "windows" ] && FAKE_BIN="$WORK/shipsmoke.exe"
printf '#!/bin/sh\necho fake-shipsmoke\n' >"$FAKE_BIN"
if [ "$EXT" = "zip" ]; then
  SRC_W="$(cygpath -w "$FAKE_BIN")"
  DST_W="$(cygpath -w "$SRV_ROOT/$ASSET")"
  powershell -NoProfile -Command "Compress-Archive -Path '$SRC_W' -DestinationPath '$DST_W' -Force"
else
  tar -czf "$SRV_ROOT/$ASSET" -C "$WORK" "$(basename "$FAKE_BIN")"
fi
[ -f "$SRV_ROOT/$ASSET" ] || { echo "FAIL: test archive was not created"; exit 1; }

HASH_TOOL="sha256sum"
command -v sha256sum >/dev/null 2>&1 || HASH_TOOL="shasum -a 256"
REAL_HASH="$($HASH_TOOL "$SRV_ROOT/$ASSET" | awk '{print $1}')"

# 1. correct checksums.txt → success
printf '%s  %s\n' "$REAL_HASH" "$ASSET" >"$SRV_ROOT/checksums.txt"
start_server() {
  (cd "$WORK/release" && python -m http.server "$1" >/dev/null 2>&1 &
   echo $! >"$WORK/server.pid") 2>/dev/null ||
  (cd "$WORK/release" && python3 -m http.server "$1" >/dev/null 2>&1 &
   echo $! >"$WORK/server.pid")
  # Ready when the checksums.txt is actually fetchable over HTTP.
  for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
    if curl -fsS -o /dev/null "http://127.0.0.1:$1/$TAG/checksums.txt" 2>/dev/null; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}
PORT="$(( 20000 + RANDOM % 20000 ))"
start_server "$PORT" || { echo "FAIL: could not start test HTTP server"; exit 1; }
stop_server() { [ -f "$WORK/server.pid" ] && kill "$(cat "$WORK/server.pid")" 2>/dev/null || true; }
trap 'stop_server; rm -rf "$WORK"' EXIT

run_case success-checksum-verified 0 \
  SHIPS_VERSION="$TAG" \
  SHIPS_RELEASE_BASE_URL="http://127.0.0.1:$PORT" \
  RUNNER_TEMP="$WORK/runner-temp-success"
assert_output_line "$WORK/env-success-checksum-verified.txt" "version" "$TAG"
assert_output_line "$WORK/env-success-checksum-verified.txt" "verified" "checksum"
FOUND_BIN="$(grep '^path=' "$WORK/env-success-checksum-verified.txt" | cut -d= -f2-)"
[ -f "$FOUND_BIN" ] || { echo "FAIL: extracted binary missing at $FOUND_BIN"; FAILURES=$((FAILURES + 1)); }

# 2. tampered asset (hash in manifest no longer matches) → refuse before extract
rm -rf "$WORK/runner-temp-tamper"
printf 'TAMPERED' >>"$SRV_ROOT/$ASSET"
run_case checksum-mismatch 2 \
  SHIPS_VERSION="$TAG" \
  SHIPS_RELEASE_BASE_URL="http://127.0.0.1:$PORT" \
  RUNNER_TEMP="$WORK/runner-temp-tamper"
if [ -d "$WORK/runner-temp-tamper/shipsmoke-action/bin" ] &&
   ls "$WORK/runner-temp-tamper/shipsmoke-action/bin" 2>/dev/null | grep -q .; then
  echo "FAIL: extraction happened despite checksum mismatch"
  FAILURES=$((FAILURES + 1))
else
  PASS_COUNT=$((PASS_COUNT + 1))
  echo "ok: no extraction on checksum mismatch"
fi

# 3. asset missing from manifest → refuse
rm -rf "$WORK/runner-temp-unlisted"
printf '%s  some-other-asset.zip\n' "$REAL_HASH" >"$SRV_ROOT/checksums.txt"
run_case asset-unlisted 2 \
  SHIPS_VERSION="$TAG" \
  SHIPS_RELEASE_BASE_URL="http://127.0.0.1:$PORT" \
  RUNNER_TEMP="$WORK/runner-temp-unlisted"

# 4. checksums.txt missing entirely (404) → refuse
rm -f "$SRV_ROOT/checksums.txt"
run_case checksums-missing 2 \
  SHIPS_VERSION="$TAG" \
  SHIPS_RELEASE_BASE_URL="http://127.0.0.1:$PORT"

stop_server

# ---------------------------------------------------------------------------
echo
echo "passed: $PASS_COUNT, failed: $FAILURES"
[ "$FAILURES" -eq 0 ] || exit 1
