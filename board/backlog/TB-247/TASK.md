# TB-247: Triage TB-205 knip first-pass findings

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Module:** gui-frontend
**Tags:** dead-code,frontend,lint
**Branch:** —
**Parent:** TB-205

## Goal

Triage the unused exports and unused dependencies reported by the
initial `npm run deadcode` (knip) pass landed in TB-205. For each, either
remove the dead code/dep, wire it up where the API was always meant to
be used, or document why it must remain exported.

## Context

TB-205 set up `knip` for `gui/frontend` and committed `knip.config.js`
with intentionally narrow ignores. The first pass reports the items
below. They are real findings, not false positives.

### Unused dependencies (`gui/frontend/package.json`)

- `@types/dompurify` — `dompurify@^3` ships its own types; this
  `@types/*` package is likely redundant. Verify and remove if so.
- `codemirror` (meta-package) — we import only `@codemirror/commands`,
  `@codemirror/lang-markdown`, `@codemirror/state`, `@codemirror/view`
  directly. Confirm the meta package is unused and drop it.
- `@types/marked` — `marked@^18` ships its own types. Likely redundant.

### ESLint disable to remove

- `gui/frontend/src/lib/components/FilterBar.svelte` has an inline
  `eslint-disable-next-line svelte/prefer-writable-derived` over the
  `let f = $state(...) + $effect(...)` pair. The autofix rewrites it to
  `let f = $derived(...)` (Svelte 5.25+ writable $derived). It was
  intentionally deferred from TB-205 because a reactivity rewrite is a
  behavior change, not a setup blocker. Validate behaviour (toggle a
  filter chip, then trigger an external `filter.set` and confirm the
  chip resets) and either land the autofix or document why $state +
  $effect is the right call here.

### Unused exports (`gui/frontend/src/`)

- `src/lib/api.ts`:
  - `pullNext` — added with the TB-239 pull mechanics; the GUI does not
    expose a Pull action yet. Either wire it (kanban "Pull next" button)
    or drop it until the UI is ready.
  - `setReviewTarget`, `setReviewerNotes`, `setReviewFindings`,
    `failReview` — review-workflow API. Decide whether the GUI is
    intended to drive these (then wire) or whether `tb review …` is the
    only entrypoint (then drop).
  - `regenerate` — historically called from the Tools menu; verify and
    remove if no longer used.
  - `isAlreadyInitializedError`, `getBoardInfo` — initial-board-setup
    helpers. Confirm whether the Init flow still references them.
- `src/lib/stores/board.ts: totalTaskCount` — derived store; if not used
  by any header/badge, remove.
- `src/lib/stores/runs.ts: runsStore` — confirm whether this is
  intentional public re-export or leftover from refactor.

## Acceptance Criteria

- [ ] Each unused dependency above is either removed or has a short
  comment in `package.json`-adjacent docs explaining why it must stay.
- [ ] Each unused export above is either wired up to a real caller,
  removed, or annotated in a way `knip` recognises (e.g. added to
  `knip.config.js` ignores with a justifying comment).
- [ ] `npm run deadcode` returns zero findings (or only documented
  ignores), and existing verification (`npm run check`, `npm test`,
  `npm run lint`) continues to pass.

## Related Tasks

- **TB-205** — Setup ESLint and dead-code check for frontend (parent;
  this task's findings come from the TB-205 first-pass output).

## Attachments

## Log

- 2026-05-19: Created from TB-205 first-pass knip output.
