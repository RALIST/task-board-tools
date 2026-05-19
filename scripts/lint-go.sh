#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LINTER="${GOLANGCI_LINT:-golangci-lint}"

if ! command -v "$LINTER" >/dev/null 2>&1; then
  echo "golangci-lint not found. Install it or set GOLANGCI_LINT=/path/to/golangci-lint." >&2
  exit 127
fi

status=0

for module in cli gui; do
	echo "==> ${module}: ${LINTER} run --config ${ROOT}/.golangci.yml ./..."
	(
		cd "$ROOT/$module"
		"$LINTER" run --config "$ROOT/.golangci.yml" ./...
	) || status=$?
done

exit "$status"
