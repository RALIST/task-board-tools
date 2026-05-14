# Repository Guidelines

## Project Structure & Module Organization

This repo builds two tools over the same markdown board format. `cli/` contains the `tb` Go CLI (`module tools/tb`) as a single `package main`; tests live beside the command files. `gui/` contains the Wails3 desktop app (`module tools/tb-gui`): exported services are in `gui/app/`, non-exported helpers in `gui/internal/`, and the Svelte 5 frontend in `gui/frontend/src/`. Wails build config and platform assets live under `gui/build/`; frontend static assets live in `gui/frontend/static/`. `docs/` is the product and architecture source of truth. `board/` is this repo's own task board; do not hand-edit generated `board/BOARD.md`.

## Build, Test, and Development Commands

- `cd cli && go build -o tb .` builds the CLI.
- `cd cli && go test ./...` runs CLI tests.
- `cd gui && go test ./...` runs GUI backend tests.
- `cd gui/frontend && npm install` installs frontend dependencies.
- `cd gui/frontend && npm run check` runs Svelte/TypeScript checks.
- `cd gui/frontend && npm test` runs Vitest.
- `cd gui && task dev` or `wails3 dev` starts the desktop app in development mode.
- `cd gui && task build` or `wails3 build` creates a production GUI build.

Run Go commands from `cli/` or `gui/`; the workspace root has `go.work` but is not itself a module.

## Coding Style & Naming Conventions

Format Go with `gofmt`; keep CLI code in the existing single-package style and use lower-case command/helper names such as `cmdCreate`, `moveTask`, and `writeFileAtomic`. Preserve the board invariants from `docs/ARCHITECTURE.md`: structured mutations take `.board.lock`, task writes are atomic, and `BOARD.md` is regenerated. In Svelte/TypeScript, keep components PascalCase, stores lower-case (`runsStore`, `boardStore`), and prefer typed wrappers in `gui/frontend/src/lib/api.ts` over direct binding calls.

## Testing Guidelines

Add table-driven Go tests next to the changed package (`*_test.go`). For GUI agent, watcher, daemon, and filesystem behavior, include integration-style tests when the bug depends on real processes, locks, or file events. Frontend logic tests use Vitest (`*.test.ts`); run `npm run check` before treating UI code as complete.

## Commit & Pull Request Guidelines

Recent commits use short, imperative subjects with a scope when useful: `cli: ship M1 ...`, `gui: ship M5 ...`, `board: groom TB-5 ...`, or `TB-26 atomic next-id writes`. Keep commits focused and avoid staging unrelated board churn. PRs should summarize behavior changes, link task IDs (`TB-26`), list verification commands, and include screenshots or smoke-test notes for GUI changes.
