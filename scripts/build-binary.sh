#!/usr/bin/env bash
# Build the vivaldi-workspace-mcp binary inside a container and copy it
# to ./bin on the host. No local Go toolchain required.
#
# Uses podman if available, otherwise docker (BuildKit is required for
# `--output`; it is the default builder since Docker 23).
#
# Run from anywhere:
#     scripts/build-binary.sh
#
# Override the engine explicitly:
#     CONTAINER_ENGINE=docker scripts/build-binary.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="$REPO_ROOT/bin"

ENGINE="${CONTAINER_ENGINE:-}"
if [[ -z "$ENGINE" ]]; then
    if command -v podman >/dev/null 2>&1; then
        ENGINE=podman
    elif command -v docker >/dev/null 2>&1; then
        ENGINE=docker
    else
        echo "[build] FAIL: neither podman nor docker found in PATH." >&2
        exit 1
    fi
fi

echo "[build] engine: $ENGINE"
echo "[build] output: $OUT_DIR/vivaldi-workspace-mcp"

mkdir -p "$OUT_DIR"
"$ENGINE" build \
    --target binary \
    --output "type=local,dest=$OUT_DIR" \
    "$REPO_ROOT"

echo "[build] done: $OUT_DIR/vivaldi-workspace-mcp"
