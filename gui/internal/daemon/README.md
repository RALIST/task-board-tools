# daemon

In-process worker pool that picks up tasks with
`AgentStatus=queued` + `Agent` set and runs them through
`AgentService.RunQueuedAgentSync`. See `docs/ARCHITECTURE.md` →
"Daemon" for the high-level shape; this README covers operator
concerns.

## Lifecycle

```
New(opts)              -- workers spawned, idle (no IO)
Activate(ctx, dir)     -- recovery → register sink → startup scan
Deactivate()           -- drain workers; clear in-memory active set
Close()                -- cancel ctx; 5s grace; return
```

`Activate` is called from `SettingsService.OpenBoard` via the
`BoardActivator` hook. The watcher event sink is registered during
`main.go` construction (i.e. BEFORE the first Activate) so that a CLI
edit landing between recovery-end and scan-finish is caught.

## Multi-process recovery smoke test

The deterministic Go in-process recovery test
(`app/agent_recovery_test.go::TestRecoverStale_NoFinished_DeadPID_MarksLost`)
stages a board with a synthetic JSONL trail and asserts the recovery
reconciliation. It cannot prove the **parent-process kill** scenario
that R5/R12 describe (kill -9 of the GUI mid-run) because a single Go
binary cannot kill its own parent.

To verify that path manually:

1. Build the GUI: `cd gui && wails3 build`.
2. Open a board with a task assigned to `claude` or `codex`.
3. In a terminal, kick a run: `tb edit TB-1 --agent-status queued`.
4. Once you see the run go `running`, `kill -9` the GUI process from
   another terminal.
5. The board on disk should show `AgentStatus: running` and the
   JSONL should end with a `started` event (no `finished`).
6. Re-launch the GUI. Observe in the log that the recovery scan ran;
   open the task and verify it is now `lost` with reason
   `"stale after restart"`.

For the cancelled carve-out (TB-61):

1. Same setup; start a run.
2. Click Cancel in the drawer.
3. Immediately `kill -9` the GUI **before** the AgentStatus write
   completes (you have ~200ms after the JSONL line is written and the
   Wails event fires).
4. Re-launch. The task should remain `cancelled`, never `lost` or `failed`.

These are documentation steps, not CI gates — they require user
timing that an automated test cannot reliably reproduce.

## Tuning

`max_workers` is persisted at
`$XDG_CONFIG_HOME/tb-gui/preferences.json` and clamped to `[1, 4]` on
read. Bump above 1 only if your machine can absorb multiple
simultaneous agent processes; agents are CPU/IO heavy and can swamp a
laptop quickly.

## Files

- `daemon.go` — Lifecycle + worker pool + active-set dedup.
- `pid.go` — `pidAlive(pid, expectedAgent)` with comm/args fallback.
- `watcher_sink.go` — `EventSink` (`watcher.Emitter` impl) +
  `TeeEmitter` for fan-out to both Wails app and daemon.
