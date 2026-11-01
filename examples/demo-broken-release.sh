#!/usr/bin/env bash
# demo-broken-release.sh — a short, reproducible demonstration that a broken
# release artifact is detected by shipsmoke and blocks the "publish" step.
#
# The demo builds a tiny CLI for THIS platform, packages it, verifies the
# good artifact, then damages the package the way a broken upload would
# (truncation). The verify step must fail; the "publish" step never runs.
#
# Usage: bash demo-broken-release.sh
# Needs: go; shipsmoke binary on PATH or via $SHIPSMOKE
set -euo pipefail

SHIPSMOKE_BIN="${SHIPSMOKE:-shipsmoke}"

cd "$(mktemp -d)"
echo "demo dir: $PWD"

cat > main.go <<'GOEOF'
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("demo version 1.0.0")
	}
}
GOEOF

cat > go.mod <<'GOEOF'
module demo

go 1.26
GOEOF

EXE=""
case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
  mingw*|msys*|cygwin*) EXE=".exe" ;;
esac

echo "== 1. build and package for this platform =="
mkdir -p dist
go build -o "dist/demo$EXE" .
if [ -n "$EXE" ]; then
  powershell -NoProfile -Command "Compress-Archive -Path dist/demo.exe -DestinationPath dist/demo.zip -Force"
  ARCHIVE="dist/demo.zip"; ENTRY="demo.exe"; TARGET="windows/amd64"
else
  tar -czf dist/demo.tar.gz -C dist demo
  ARCHIVE="dist/demo.tar.gz"; ENTRY="demo"; TARGET="$(go env GOOS)/$(go env GOARCH)"
fi

printf 'version: 1\nartifacts:\n  - name: demo\n    path: %s\n    binary: %s\n    runs_on: [%s]\n    checks:\n      - args: ["--version"]\n        expect: { exit_code: 0, stdout_contains: ["demo version 1.0.0"] }\n' \
  "$ARCHIVE" "$ENTRY" "$TARGET" > .shipsmoke.yml

echo
echo "== 2. verify the GOOD artifact =="
"$SHIPSMOKE_BIN" check

echo
echo "== 3. break the artifact the way a broken upload would (truncate) =="
SIZE=$(wc -c < "$ARCHIVE" | tr -d ' ')
head -c $((SIZE / 2)) "$ARCHIVE" > "$ARCHIVE.part" && mv "$ARCHIVE.part" "$ARCHIVE"
echo "truncated to $((SIZE / 2)) of $SIZE bytes"

echo
echo "== 4. verify the BROKEN artifact: the gate must fail =="
set +e
"$SHIPSMOKE_BIN" check
CODE=$?
set -e

echo
if [ "$CODE" -eq 0 ]; then
  echo "DEMO FAILED: a truncated artifact passed the gate?!"; exit 1
fi
echo "== 5. publish step would be skipped (verify exited $CODE) =="
echo "In CI, a 'Publish' step with 'if: success()' never runs after this."
