#!/bin/bash
# Codex PostToolUse hook for apply_patch edits.
# Reads Codex hook JSON on stdin, extracts changed paths from tool_input.command,
# then runs focused local checks. Quiet success is intentional: Codex ignores
# plain stdout, and JSON stdout is reserved for feedback.

set -uo pipefail

INPUT=$(cat)
PROJECT_DIR="${CODEX_PROJECT_DIR:-}"
if [ -z "$PROJECT_DIR" ]; then
  PROJECT_DIR=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
fi
PROJECT_DIR="${PROJECT_DIR%/}"
GO_CACHE_DIR="${TMPDIR:-/tmp}/task-board-tools-codex-hook-go-build"
if mkdir -p "$GO_CACHE_DIR" 2>/dev/null; then
  export GOCACHE="$GO_CACHE_DIR"
fi

HOOK_CWD=$(printf '%s' "$INPUT" | jq -r '.cwd // .tool_input.cwd // .tool_input.workdir // empty' 2>/dev/null)
if [ -z "$HOOK_CWD" ]; then
  HOOK_CWD="$PROJECT_DIR"
fi

emit_block() {
  local reason="$1"
  jq -n --arg reason "$reason" '{
    decision: "block",
    reason: $reason,
    continue: false,
    hookSpecificOutput: {
      hookEventName: "PostToolUse",
      additionalContext: $reason
    }
  }'
}

emit_context() {
  local message="$1"
  jq -n --arg message "$message" '{
    systemMessage: $message,
    hookSpecificOutput: {
      hookEventName: "PostToolUse",
      additionalContext: $message
    }
  }'
}

resolve_path() {
  local path="$1"

  [ -z "$path" ] && return 0

  case "$path" in
    /*)
      printf '%s\n' "$path"
      ;;
    *)
      if [ -e "$HOOK_CWD/$path" ] || [ -e "$(dirname "$HOOK_CWD/$path")" ]; then
        printf '%s\n' "$HOOK_CWD/$path"
      else
        printf '%s\n' "$PROJECT_DIR/$path"
      fi
      ;;
  esac
}

collect_paths() {
  printf '%s\n' "$INPUT" | jq -r '
    [
      .tool_input.file_path?,
      .tool_input.path?,
      .tool_input.target_file?
    ]
    | .[]
    | select(type == "string" and length > 0)
  ' 2>/dev/null

  printf '%s\n' "$INPUT" | jq -r '.tool_input.command // empty' 2>/dev/null |
    sed -n \
      -e 's/^\*\*\* Add File: //p' \
      -e 's/^\*\*\* Update File: //p' \
      -e 's/^\*\*\* Delete File: //p' \
      -e 's/^\*\*\* Move to: //p'
}

find_nearest_file() {
  local start_dir="$1"
  local filename="$2"
  local dir="$start_dir"

  while [ "$dir" != "/" ]; do
    if [ -f "$dir/$filename" ]; then
      printf '%s\n' "$dir"
      return 0
    fi
    dir=$(dirname "$dir")
  done

  return 1
}

find_frontend_dir() {
  local file_path="$1"
  find_nearest_file "$(dirname "$file_path")" "package.json" || true
}

run_svelte_check() {
  local file_path="$1"

  [[ "$file_path" != *.svelte ]] && return 0

  local frontend_dir
  frontend_dir=$(find_frontend_dir "$file_path")
  [ -z "$frontend_dir" ] && return 0
  [ -x "$frontend_dir/node_modules/.bin/svelte-check" ] || return 0

  local rel_path output status errors
  rel_path="${file_path#"$frontend_dir"/}"
  output=$(cd "$frontend_dir" && ./node_modules/.bin/svelte-check --output human-verbose 2>&1)
  status=$?

  if [ $status -ne 0 ] && [ -n "$output" ]; then
    errors=$(printf '%s\n' "$output" | grep -A2 "^Error:" || printf '%s\n' "$output" | tail -30)
    emit_context "svelte-check errors after editing $rel_path:
$errors"
    return 2
  fi
}

run_go_vet() {
  local file_path="$1"
  local pkg_dir module_root rel_pkg vet_output status

  gofmt -w "$file_path" 2>/dev/null || true

  pkg_dir=$(dirname "$file_path")
  module_root=$(find_nearest_file "$pkg_dir" "go.mod" || true)
  [ -z "$module_root" ] && return 0

  if [ "$pkg_dir" = "$module_root" ]; then
    rel_pkg="."
  else
    rel_pkg="./${pkg_dir#"$module_root"/}"
  fi

  vet_output=$(cd "$module_root" && go vet "$rel_pkg" 2>&1)
  status=$?

  if [ $status -ne 0 ] && [ -n "$vet_output" ]; then
    emit_block "go vet errors in $rel_pkg; fix before continuing:
$vet_output"
    return 2
  fi
}

run_eslint_fix() {
  local file_path="$1"

  case "$file_path" in
    */e2e/*) return 0 ;;
  esac

  local frontend_dir
  frontend_dir=$(find_frontend_dir "$file_path")
  [ -z "$frontend_dir" ] && return 0

  if [ -f "$frontend_dir/node_modules/.bin/eslint" ]; then
    "$frontend_dir/node_modules/.bin/eslint" --fix "$file_path" 2>/dev/null || true
  fi
}

RAW_PATHS=$(collect_paths | sed '/^[[:space:]]*$/d' | sort -u)
[ -z "$RAW_PATHS" ] && exit 0

while IFS= read -r raw_path; do
  file_path=$(resolve_path "$raw_path")
  [ -f "$file_path" ] || continue

  if [ "${1:-}" = "--svelte-check" ]; then
    run_svelte_check "$file_path" || exit 0
    continue
  fi

  case "$file_path" in
    *.go)
      run_go_vet "$file_path" || exit 0
      ;;
    *.ts|*.svelte|*.js)
      run_eslint_fix "$file_path"
      ;;
  esac
done <<< "$RAW_PATHS"

exit 0
