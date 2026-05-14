#!/bin/bash
# PostToolUse hook for Write|Edit — auto-format, lint, and type-check.
# Usage:
#   Sync mode (default): auto-format + go vet
#   Async mode (arg):    post-edit.sh --svelte-check
#
# Reads JSON from stdin, extracts tool_input.file_path.

set -uo pipefail

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

if [ -z "$FILE_PATH" ] || [ ! -f "$FILE_PATH" ]; then
  exit 0
fi

# --- Async mode: svelte-check only ---
if [ "${1:-}" = "--svelte-check" ]; then
  [[ "$FILE_PATH" != *.svelte ]] && exit 0

  FRONTEND_DIR="$CLAUDE_PROJECT_DIR/frontend"
  REL_PATH="${FILE_PATH#"$FRONTEND_DIR"/}"
  OUTPUT=$(cd "$FRONTEND_DIR" && npx svelte-check --output human-verbose 2>&1)

  if [ $? -ne 0 ] && [ -n "$OUTPUT" ]; then
    ERRORS=$(echo "$OUTPUT" | grep -A2 "^Error:" || echo "$OUTPUT" | tail -30)
    jq -n --arg msg "svelte-check errors after editing $REL_PATH:
$ERRORS" '{"systemMessage": $msg}'
  else
    echo '{"suppressOutput": true}'
  fi
  exit 0
fi

# --- Sync mode: format + vet ---
case "$FILE_PATH" in
  *.go)
    # 1. Format: TODO: make  it not so noisy, cause it makes agent in cycle add imports after formatter strip
      # probably move formatter to agent stop\stop hooks
#    if command -v goimports &>/dev/null; then
#      goimports -w "$FILE_PATH" 2>/dev/null
#    else
#      gofmt -w "$FILE_PATH" 2>/dev/null
#    fi

    # 2. Vet — find nearest go.mod to support nested modules (tools/tb, etc.)
    PKG_DIR=$(dirname "$FILE_PATH")
    MODULE_ROOT="$PKG_DIR"
    while [ "$MODULE_ROOT" != "/" ]; do
      if [ -f "$MODULE_ROOT/go.mod" ]; then
        break
      fi
      MODULE_ROOT=$(dirname "$MODULE_ROOT")
    done
    # No go.mod found — skip vet silently
    if [ ! -f "$MODULE_ROOT/go.mod" ]; then
      exit 0
    fi
    if [ "$PKG_DIR" = "$MODULE_ROOT" ]; then
      REL_PKG="."
    else
      REL_PKG="./${PKG_DIR#"$MODULE_ROOT"/}"
    fi
    VET_OUTPUT=$(cd "$MODULE_ROOT" && go vet "$REL_PKG" 2>&1)
    if [ $? -ne 0 ] && [ -n "$VET_OUTPUT" ]; then
      jq -n --arg reason "go vet errors in $REL_PKG — fix before continuing:
$VET_OUTPUT" '{"decision": "block", "reason": $reason}'
    fi
    ;;
  *.ts|*.svelte|*.js)
    # Skip e2e test files — Playwright tests use different conventions
    case "$FILE_PATH" in
      */e2e/*) ;;
      *)
        # Format only (svelte-check runs separately in async mode)
        FRONTEND_DIR="$CLAUDE_PROJECT_DIR/frontend"
        if [ -f "$FRONTEND_DIR/node_modules/.bin/eslint" ]; then
          "$FRONTEND_DIR/node_modules/.bin/eslint" --fix "$FILE_PATH" 2>/dev/null || true
        fi
        ;;
    esac
    ;;
esac

exit 0
