# TB-286: Show readable GUI error toasts

**Type:** bug
**Priority:** P2
**Size:** S
**Agent:** codex
**AgentStatus:** success
**Module:** gui-frontend
**Tags:** gui,frontend,ux,error-handling
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —

## Goal

Replace raw Wails/CLI error payloads in GUI toasts with concise, human-readable messages while preserving the actionable CLI hint that tells the user how to recover.

**Context:** The attached screenshot shows a drag/drop move failure dumping a structured runtime error object into the toast: nested `message`, `cause`, `Kind`, `Op`, `Args`, and `Stderr` JSON are visible to the user. The observed path is backlog-to-Ready drag/drop, where `gui/frontend/src/routes/+page.svelte` calls `readyTask()` and pushes a `Move failed:` toast using `errorString(err)` on rejection. `gui/frontend/src/lib/api.ts` currently centralizes error formatting in `errorString()`, while `gui/internal/cli/mutations.go` wraps CLI failures such as `tb ready <ID>` as typed `MutationError` values with useful stderr.

**Constraints / non-goals:** Keep the CLI as the source of truth for validation; do not add a separate frontend pre-check for whether a task can enter Ready. Do not change CLI command behavior or board mutation semantics. Preserve useful CLI recovery text such as `Fix with ...` and `tb triage`, but strip transport/debug envelopes (`RuntimeError`, JSON object fields, args arrays, nested cause dumps) from user-facing toast copy. Scope this task to GUI error text shown in toasts, not a full notification redesign.

**Related Tasks:** TB-235 is the closest existing contract: GUI validation toasts should preserve actionable CLI hints instead of hiding them behind generic copy. TB-285 was only the example task in the screenshot; this task should fix the generic toast formatting path.

## Acceptance Criteria

- [ ] `errorString()` (or an equivalent centralized helper) formats `Error`, plain string, Wails runtime-error-like objects, and nested Go/CLI mutation-error payloads into readable text without returning raw JSON, `[object Object]`, `RuntimeError`, `Kind`, `Op`, `Args`, or `Cause` fields.
- [ ] A failed backlog-to-Ready drag/drop still reverts the optimistic move and shows a concise toast such as `Move failed: TB-285 is not ready - needs grooming. Fix with ...` rather than the structured JSON object from the screenshot.
- [ ] Actionable CLI stderr remains visible when it is the useful user message, including commands or hints such as `tb edit <ID>` and `tb triage`; the GUI must not replace validation details with a generic `Move failed` only message.
- [ ] Existing toast producers continue to work for simple `Error` and string rejections; no success/info toast behavior changes.
- [ ] Frontend tests cover the formatter for the screenshot-shaped runtime error payload and at least one caller path that pushes a toast from a rejected board mutation.
- [ ] `cd gui/frontend && npm run check` passes.
- [ ] `cd gui/frontend && npm test -- --run` passes.
- [ ] Manual test note: in `cd gui && wails3 dev`, drag an ungroomed backlog task to Ready, confirm the card returns to Backlog, and confirm the toast is concise/readable with no JSON envelope.

## Attachments

- Снимок экрана 2026-05-19 в 23.53.20.png

## Log

- 2026-05-19: Created
- 2026-05-19: Attached Снимок экрана 2026-05-19 в 23.53.20.png
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited type=bug, size=S, module=gui-frontend, tags=gui,frontend,ux,error-handling, title=Show readable GUI error toasts, goal
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited goal
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited agentstatus=success
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success

