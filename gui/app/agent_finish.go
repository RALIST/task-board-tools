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

// queuedRun is the rolled-up view of the latest open queued event the
// daemon needs to replay. ResumedFrom / ResumedFromRun are populated
// when the queued event is a resume run (TB-130) — the daemon uses
// them to rehydrate the parent run's execution context before spawning
// the runner.
type queuedRun struct {
	RunID          string
	Mode           agent.Mode
	ResumedFrom    string // parent session id, if mode == ModeResume
	ResumedFromRun string // parent run id, if mode == ModeResume
	TriageHash     string // auto-groom dedupe fingerprint, if mode == ModeGroom + auto-groom (TB-174)
}

// findQueuedRun scans the task's JSONL run history and returns the
// latest `queued` event that has no subsequent `started` or `finished`
// event. This is the run that the daemon's worker should pick up.
//
// Returns ErrNoQueuedRun when no such entry exists (the AgentStatus is
// queued on the .md but the JSONL has no matching open queued event —
// either a recovery synthesised a finished record or the JSONL is
// missing). The caller can decide whether to synthesise a new run_id.
func findQueuedRun(boardDir, taskID string) (queuedRun, error) {
	path := agent.StatePath(boardDir, taskID)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return queuedRun{Mode: agent.ModeImplement}, ErrNoQueuedRun
		}
		return queuedRun{Mode: agent.ModeImplement}, err
	}
	defer f.Close()

	type state struct {
		hasQueued      bool
		hasStarted     bool
		hasFinished    bool
		mode           agent.Mode
		resumedFrom    string
		resumedFromRun string
		triageHash     string
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
		case agent.EvQueued:
			st.hasQueued = true
			st.mode = parseRunMode(ev.Mode)
			// resume linkage lives on the queued event (TB-138). Capture
			// it here so a daemon replay after a crash between AppendEvent
			// and goroutine spawn can rehydrate the parent session.
			if ev.ResumedFrom != "" {
				st.resumedFrom = ev.ResumedFrom
			}
			if ev.ResumedFromRun != "" {
				st.resumedFromRun = ev.ResumedFromRun
			}
			// auto-groom dedupe fingerprint (TB-174): preserved through
			// daemon pickup so the finished event still carries it.
			if ev.TriageHash != "" {
				st.triageHash = ev.TriageHash
			}
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
		if st.hasQueued && !st.hasFinished && !st.hasStarted {
			return queuedRun{
				RunID:          id,
				Mode:           st.mode,
				ResumedFrom:    st.resumedFrom,
				ResumedFromRun: st.resumedFromRun,
				TriageHash:     st.triageHash,
			}, nil
		}
	}
	return queuedRun{Mode: agent.ModeImplement}, ErrNoQueuedRun
}

// runSessionContext looks up the persisted cwd + env for runID in the
// task's JSONL. Returns ok=false when the run has no session event.
// Used by RunQueuedAgentSync to rehydrate a resume run's execution
// context from the parent run that originated the session id.
func runSessionContext(boardDir, taskID, runID string) (cwd string, env map[string]string, ok bool) {
	path := agent.StatePath(boardDir, taskID)
	f, err := os.Open(path)
	if err != nil {
		return "", nil, false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var ev agent.Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		if ev.RunID != runID || ev.Event != agent.EvSession {
			continue
		}
		if ev.Cwd != "" {
			cwd = ev.Cwd
		}
		if ev.RunEnv != nil {
			env = ev.RunEnv
		}
		ok = true
	}
	return cwd, env, ok
}

func parseRunMode(mode string) agent.Mode {
	switch agent.Mode(mode) {
	case agent.ModeGroom:
		return agent.ModeGroom
	case agent.ModeReview:
		return agent.ModeReview
	case agent.ModeResume:
		return agent.ModeResume
	default:
		return agent.ModeImplement
	}
}

// runModeFor scans the task's JSONL run history and returns the
// originating mode for the given RunID (read from its `queued` event).
// Returns ok=false when the run is missing or has no queued event.
// TB-237 uses this to look up a resume's parent action so the per-mode
// pair update lands on the originating mode.
func runModeFor(boardDir, taskID, runID string) (agent.Mode, bool) {
	path := agent.StatePath(boardDir, taskID)
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var ev agent.Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		if ev.RunID != runID || ev.Event != agent.EvQueued {
			continue
		}
		return parseRunMode(ev.Mode), true
	}
	return "", false
}

// ErrNoQueuedRun is returned by findQueuedRun when no open queued
// event exists for the task — meaning the daemon should not pick it up
// even if the .md says AgentStatus=queued.
var ErrNoQueuedRun = errors.New("no open queued run in JSONL")
