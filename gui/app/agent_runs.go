package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"time"

	"tools/tb-gui/internal/agent"
)

// Run is the rolled-up view of a single agent run, derived from a task's
// JSONL run history. Sorted-by-StartedAt descending, the GUI's drawer
// renders these as the "Past runs" list.
//
// Times are RFC3339 strings on the wire so they survive Wails serialization
// unchanged; LogPath is absolute so the frontend doesn't need to know about
// boardDir.
//
// SessionID/ResumedFrom/ResumedFromRun are TB-130 additions surfacing the
// agent-side resume linkage to the UI. SessionID is the agent CLI's
// conversation id (claude/codex); ResumedFrom is the parent session id
// when this run was kicked off via ResumeAgent; ResumedFromRun is the
// parent RunID (the chip the UI displays — internal session ids never
// reach the user).
type Run struct {
	RunID          string `json:"runId"`
	TaskID         string `json:"taskId"`
	Agent          string `json:"agent"`
	Mode           string `json:"mode"`
	QueuedAt       string `json:"queuedAt"`
	StartedAt      string `json:"startedAt"`
	FinishedAt     string `json:"finishedAt"`
	Status         string `json:"status"`
	ExitCode       int    `json:"exitCode"`
	LogPath        string `json:"logPath"`
	Detached       bool   `json:"detached,omitempty"`
	SessionID      string `json:"sessionId,omitempty"`
	ResumedFrom    string `json:"resumedFrom,omitempty"`
	ResumedFromRun string `json:"resumedFromRun,omitempty"`
}

// ErrRunLogNotFound is returned by GetRunLog when the on-disk file is
// missing (run hasn't started yet, or the log was rotated away). The
// frontend renders an empty pane rather than a crash.
var ErrRunLogNotFound = errors.New("run log not found")

// ListRuns reads the task's resolved agent state JSONL, groups events by
// `run_id`, and returns one Run per group, sorted by StartedAt desc. A
// queued-but-never-started run sorts by QueuedAt.
//
// Tolerant by design: a trailing partial JSON line (writer flushed mid-
// write before fsync) is dropped with a warning rather than failing the
// whole call. A missing JSONL file returns [], not nil — the first-time
// view of a task should render an empty list cleanly, not an error.
func (s *AgentService) ListRuns(ctx context.Context, id string) ([]Run, error) {
	if s.board == nil {
		return nil, ErrNoBoard
	}
	boardDir, err := s.board.resolveBoardDir(ctx)
	if err != nil {
		return nil, err
	}

	path := agent.StatePath(boardDir, id)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Run{}, nil
		}
		return nil, fmt.Errorf("ListRuns: open %s: %w", path, err)
	}
	defer f.Close()

	grouped := make(map[string]*Run)
	sc := bufio.NewScanner(f)
	// Bigger buffer for long lines (agent stdout can include tool-use
	// payloads). Mirrors the runner's scanner sizing.
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev agent.Event
		if err := json.Unmarshal(line, &ev); err != nil {
			slog.Warn("ListRuns: skip malformed JSONL line", "task", id, "err", err)
			continue
		}
		r, ok := grouped[ev.RunID]
		if !ok {
			r = &Run{
				RunID:   ev.RunID,
				TaskID:  ev.TaskID,
				LogPath: agent.LogPath(boardDir, id, ev.RunID),
			}
			grouped[ev.RunID] = r
		}
		// Backfill TaskID if it was missing on the first event we saw.
		if r.TaskID == "" {
			r.TaskID = ev.TaskID
		}
		switch ev.Event {
		case agent.EvQueued:
			r.QueuedAt = ev.TS
			r.Agent = ev.Agent
			r.Mode = ev.Mode
			if ev.ResumedFrom != "" {
				r.ResumedFrom = ev.ResumedFrom
			}
			if ev.ResumedFromRun != "" {
				r.ResumedFromRun = ev.ResumedFromRun
			}
			if r.Status == "" {
				r.Status = "queued"
			}
		case agent.EvStarted:
			r.StartedAt = ev.TS
			if ev.Mode != "" {
				r.Mode = ev.Mode
			}
			if r.Status == "queued" || r.Status == "" {
				r.Status = "running"
			}
		case agent.EvSession:
			if ev.SessionID != "" {
				r.SessionID = ev.SessionID
			}
		case agent.EvFinished:
			r.FinishedAt = ev.TS
			if ev.Mode != "" {
				r.Mode = ev.Mode
			}
			r.Status = string(ev.Status)
			r.ExitCode = ev.ExitCode
		}
	}
	if err := sc.Err(); err != nil {
		// bufio.ErrTooLong shouldn't fire (we set 1MiB cap), but if it
		// does we still return what we've parsed.
		if !errors.Is(err, bufio.ErrTooLong) {
			slog.Warn("ListRuns: scanner error after parse", "task", id, "err", err)
		}
	}

	out := make([]Run, 0, len(grouped))
	for _, r := range grouped {
		if (r.Status == "queued" || r.Status == "running") && !s.hasActiveRunID(id, r.RunID) {
			r.Detached = true
		}
		out = append(out, *r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return sortKey(out[i]).After(sortKey(out[j]))
	})
	return out, nil
}

// sortKey returns the time to sort a run by: StartedAt when present,
// otherwise QueuedAt (so a queued-but-never-started run still sorts
// sensibly). Malformed timestamps fall back to the zero time and end up at
// the bottom of the list.
func sortKey(r Run) time.Time {
	if r.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339, r.StartedAt); err == nil {
			return t
		}
	}
	if r.QueuedAt != "" {
		if t, err := time.Parse(time.RFC3339, r.QueuedAt); err == nil {
			return t
		}
	}
	return time.Time{}
}

// GetRunLog returns the full text of the resolved per-run log file. Returns
// ErrRunLogNotFound for a missing file (the frontend renders an empty pane).
//
// Takes both IDs explicitly — without taskID the service would need an
// in-memory runID→taskID index, which adds a coordination burden the
// frontend can pay trivially (it already knows both IDs).
func (s *AgentService) GetRunLog(ctx context.Context, taskID, runID string) (string, error) {
	if s.board == nil {
		return "", ErrNoBoard
	}
	boardDir, err := s.board.resolveBoardDir(ctx)
	if err != nil {
		return "", err
	}
	path := agent.LogPath(boardDir, taskID, runID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrRunLogNotFound
		}
		return "", fmt.Errorf("GetRunLog: open %s: %w", path, err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("GetRunLog: read %s: %w", path, err)
	}
	return string(b), nil
}
