package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
)

// finishCancelled writes the cancel-finish triple (JSONL finished
// {cancelled, reason}, Wails emit, tb edit --agent-status cancelled) via
// the shared finishOnce-gated `recordTerminal` helper. The reason
// distinguishes the source: "user cancelled" from CancelRun and
// "shutdown" from the daemon graceful-shutdown path. Either path may
// arrive first; the second observes a no-op.
func (s *AgentService) finishCancelled(c *cli.Client, ar *activeRun, boardDir, reason string) error {
	s.recordTerminal(c, ar, boardDir, agent.StatusCancelled, reason, -1)
	return nil
}

// findQueuedRunID scans the task's JSONL run history and returns the
// run_id of the latest `queued` event that has no subsequent `started`
// or `finished` event. This is the run that the daemon's worker should
// pick up.
//
// Returns ErrNoQueuedRun when no such entry exists (the AgentStatus is
// queued on the .md but the JSONL has no matching open queued event —
// either a recovery synthesised a finished record or the JSONL is
// missing). The caller can decide whether to synthesise a new run_id.
func findQueuedRunID(boardDir, taskID string) (string, error) {
	path := agent.StatePath(boardDir, taskID)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNoQueuedRun
		}
		return "", err
	}
	defer f.Close()

	type state struct {
		hasStarted  bool
		hasFinished bool
	}
	runs := map[string]*state{}
	order := []string{}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var ev agent.Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		if ev.RunID == "" {
			continue
		}
		st, ok := runs[ev.RunID]
		if !ok {
			st = &state{}
			runs[ev.RunID] = st
			order = append(order, ev.RunID)
		}
		switch ev.Event {
		case agent.EvStarted:
			st.hasStarted = true
		case agent.EvFinished:
			st.hasFinished = true
		}
	}

	// Walk in reverse insertion order so the latest queued-only run wins.
	for i := len(order) - 1; i >= 0; i-- {
		id := order[i]
		st := runs[id]
		if !st.hasFinished && !st.hasStarted {
			return id, nil
		}
	}
	return "", ErrNoQueuedRun
}

// ErrNoQueuedRun is returned by findQueuedRunID when no open queued
// event exists for the task — meaning the daemon should not pick it up
// even if the .md says AgentStatus=queued.
var ErrNoQueuedRun = errors.New("no open queued run in JSONL")
