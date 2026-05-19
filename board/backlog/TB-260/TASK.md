# TB-260: Ability to edit agent prompts

**Type:** feature
**Priority:** P2
**Size:** L
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** agents,prompt,settings
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —

## Goal

Allow users to edit and reset the GUI agent prompt templates used for implement, groom, and review runs, while preserving embedded defaults and a mode-keyed path that can support future prompt-backed run modes without another storage migration.

## Context

Agent prompt defaults are currently embedded in `gui/internal/agent/prompts/implement.md`, `gui/internal/agent/prompts/groom.md`, and `gui/internal/agent/prompts/review.md`, with constants, placeholder rendering, and mode decorators in `gui/internal/agent/runner.go`. Fresh runs are assembled in `gui/app/agent_run.go`, where implement starts from `PromptImplement` and groom/review swap templates through decorators. User preferences already persist through `gui/app/preferences.go`, `gui/app/settings_service.go`, `gui/frontend/src/lib/stores/preferences.ts`, and `gui/frontend/src/lib/components/SettingsPanel.svelte`.

`docs/FEATURES.md` currently lists a built-in prompt editor as an explicit non-goal. This task intentionally changes that product stance, so the implementation must update that documentation instead of leaving the contradiction behind.

## Constraints

- Store prompt overrides as local GUI preferences, not in task markdown, `BOARD.md`, or board configuration.
- Keep embedded prompt files as the reset/default source of truth and as the fallback when no override exists.
- Preserve the current placeholder contract: `{{TASK_ID}}`, `{{TASK_TITLE}}`, and `{{TASK_BODY}}` render exactly as they do today; unknown placeholders remain literal unless a separate task expands the renderer.
- Prompt edits affect only future runs. Already-running runs, persisted historical logs, and resume continuation behavior must not be rewritten.
- Resume prompt editing is out of scope unless resume is explicitly registered as a user-editable prompt mode in the same registry.
- The UI may warn that changing prompts can bypass project workflow guidance, but it should not block valid custom prompt text beyond empty/whitespace validation for saved overrides.

## Related Tasks

- **TB-65** — `prompts/groom.md` template and `agent.PromptGroom` embed (prerequisite prompt surface).
- **TB-198** — Review-mode prompt and findings flow (prerequisite prompt surface).
- **TB-238** — Recent implement prompt contract around `ReviewRef` before review submit (safety context for preserving defaults/reset behavior).

## Acceptance Criteria

- [ ] A backend prompt registry centralizes the user-editable modes (`implement`, `groom`, `review`) with embedded default text, display labels, and the supported placeholder list; adding a future prompt-backed mode requires registering it in one place and does not require changing the preferences JSON shape.
- [ ] `preferences.json` persists prompt overrides keyed by mode, normalizes missing/corrupt/unknown values safely, omits or clears overrides when the user resets a mode, and falls back to the embedded default when no override is present.
- [ ] `SettingsService` exposes typed get/list/save/reset prompt APIs, the Wails/TypeScript wrappers and `preferencesStore` load/save them, and existing settings preferences (`max_workers`, timeout, default agent, CLI path) continue to round-trip unchanged.
- [ ] New implement, groom, and review runs render from the saved override for their mode when present and from the embedded default when absent; mode-specific safety tests still prove groom uses the grooming contract and review uses the review-findings contract after default reset.
- [ ] The Settings panel includes an Agent Prompts editor with mode selection, editable prompt text, placeholder/default visibility, dirty-state save/reset controls, and toast/error behavior consistent with the existing settings UI.
- [ ] Manual test note: in the GUI, edit the groom prompt to include an obvious sentinel instruction, run Groom on a disposable task, confirm the launched run receives the edited prompt, then reset groom and confirm the next run uses the embedded default again.
- [ ] Documentation updates remove or revise the `docs/FEATURES.md` built-in-prompt-editor non-goal and document where prompt overrides live, how reset works, and that custom prompts affect future runs only.
- [ ] Verification covers backend preference/prompt selection tests, frontend store/panel tests, regenerated Wails bindings if service methods change, `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited priority=P2, type=feature, size=L, module=gui, tags=agents,prompt,settings, goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=success
- 2026-05-19: Edited agentstatus=success, groomed-by=codex, groom-status=success

