#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [[ "$BRANCH" != "main" && "$BRANCH" != "master" ]]; then
    echo "Error: current branch is '$BRANCH', expected 'main' or 'master'"
    exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
    echo "Error: working tree has uncommitted changes"
    exit 1
fi

DATE=$(date +%Y%m%d)
PATTERN="v${DATE}.v*"
LAST_TAG=$(git tag -l "$PATTERN" | sort -V | tail -1)

if [[ -z "$LAST_TAG" ]]; then
    N=1
else
    LAST_N=$(echo "$LAST_TAG" | sed "s/v${DATE}.v//")
    N=$((LAST_N + 1))
fi

TAG="v${DATE}.v${N}"

if [[ "${1:-}" == "--push" ]]; then
    git tag "$TAG"
    git push origin "$TAG"
    echo "Created and pushed tag: $TAG"
    echo ""
    echo "Build with version injection:"
    echo "  go build -ldflags \"-X github.com/ddd-qce/core.Version=$TAG\" ./..."
else
    echo "Will create tag: $TAG"
    echo "Run with --push to create and push:"
    echo "  ./scripts/tag.sh --push"
fi
