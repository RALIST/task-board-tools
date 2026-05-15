# TB-151: TB-93/GUI: watcher attach() lacks mutex for concurrent Switch invocations

**Type:** bug
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,watcher,concurrency
**Branch:** —
**Parent:** TB-93

## Goal

gui/internal/watcher/watcher.go:202-224 - addExistingFolderTaskWatches is called during attach() without holding w.mu. watchDirs is the same map later mutated under w.mu by addWatchDir. attach itself swaps w.watchDirs only after the loop completes (line 187), and nothing prevents two goroutines from calling attach concurrently (e.g., a user clicks 'open board' twice). The old w.fsw is still receiving events during the second attach's setup; if the first attach's pump goroutine delivers an event that lands on handle -> reconcileDirWatches -> addWatchDir, that grabs w.mu and operates on w.watchDirs while the second attach is concurrently populating its own fresh watchDirs map (unlocked). Likely safe in practice but worth a guard. Fix: add a separate attachMu sync.Mutex taken at top of attach(), distinct from per-event w.mu. Source: GUI backend review finding #3.

## Acceptance Criteria

- [x] `Watcher` carries a dedicated `attachMu sync.Mutex`, taken at the top of `attach`, that serialises full attach setup. Two concurrent `Switch`/`Start` invocations now run their fsw.Add loops sequentially; the live event pump on `w.mu` remains untouched.
- [x] `attachMu` is distinct from `w.mu` so an in-flight event handler taking `w.mu` to call `addWatchDir` does not block on a slow attach.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

