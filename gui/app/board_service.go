// Package app holds Wails services exposed to the frontend.
package app

import (
	"context"
	"errors"
	"strings"
	"sync"

	"tools/tb-gui/internal/cli"
)

// Task mirrors the JSON contract emitted by `tb ls --json` (see
// cli/json_output.go in the CLI module). The JSON tags drive both the Wails
// binding generator and the on-the-wire shape.
type Task struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Priority    string   `json:"priority"`
	Size        string   `json:"size"`
	Module      string   `json:"module"`
	Tags        []string `json:"tags"`
	Branch      string   `json:"branch"`
	Parent      string   `json:"parent"`
	Status      string   `json:"status"`
	FilePath    string   `json:"filePath"`
	Agent       string   `json:"agent"`
	AgentStatus string   `json:"agentStatus"`
}

// BoardSnapshot is the read-only view the frontend renders as a kanban.
// Tasks are pre-bucketed server-side so the frontend doesn't have to know
// about the status taxonomy.
type BoardSnapshot struct {
	Backlog    []Task `json:"backlog"`
	InProgress []Task `json:"inProgress"`
	Done       []Task `json:"done"`
}

// TaskDetail is what GetTask returns: full metadata plus the raw markdown
// body (the frontend's TaskDrawer renders it).
type TaskDetail struct {
	Metadata Task   `json:"metadata"`
	Body     string `json:"body"`
}

// ErrNoBoard is returned by service methods when no project root has been
// selected yet. The frontend treats this as "show the folder picker", not
// "fail loudly".
var ErrNoBoard = errors.New("no board selected")

// ErrNotFound is returned by GetTask when the requested ID isn't on disk.
var ErrNotFound = errors.New("task not found")

// BoardService exposes read-only board operations to the frontend.
//
// The active CLI client is swappable so SettingsService can re-target the
// service when the user picks a new board, without rebuilding the service or
// restarting the daemon.
type BoardService struct {
	mu     sync.RWMutex
	client *cli.Client
}

// NewBoardService returns a service with no client attached. The caller (a
// SettingsService at startup) must call setClient before LoadBoard/GetTask
// return useful data.
func NewBoardService() *BoardService { return &BoardService{} }

// ServiceName satisfies the Wails service contract.
func (b *BoardService) ServiceName() string { return "BoardService" }

// setClient atomically swaps the active CLI client. Passing nil clears it
// (LoadBoard then returns ErrNoBoard).
func (b *BoardService) setClient(c *cli.Client) {
	b.mu.Lock()
	b.client = c
	b.mu.Unlock()
}

// LoadBoard returns the active task set (backlog + in-progress + done),
// pre-bucketed by status. Archive tasks are intentionally excluded — they
// surface in M3 behind a filter toggle.
func (b *BoardService) LoadBoard(ctx context.Context) (BoardSnapshot, error) {
	c := b.snapshot()
	if c == nil {
		return BoardSnapshot{}, ErrNoBoard
	}

	var tasks []Task
	if err := c.RunJSON(ctx, &tasks, "ls", "--json", "--status", "active"); err != nil {
		return BoardSnapshot{}, err
	}

	snap := BoardSnapshot{
		Backlog:    make([]Task, 0),
		InProgress: make([]Task, 0),
		Done:       make([]Task, 0),
	}
	for _, t := range tasks {
		switch t.Status {
		case "backlog":
			snap.Backlog = append(snap.Backlog, t)
		case "in-progress":
			snap.InProgress = append(snap.InProgress, t)
		case "done":
			snap.Done = append(snap.Done, t)
		}
	}
	return snap, nil
}

// GetTask returns metadata + body for a single task. Returns ErrNotFound when
// the ID resolves to nothing.
func (b *BoardService) GetTask(ctx context.Context, id string) (TaskDetail, error) {
	c := b.snapshot()
	if c == nil {
		return TaskDetail{}, ErrNoBoard
	}

	var detail TaskDetail
	if err := c.RunJSON(ctx, &detail, "show", id, "--json"); err != nil {
		if isNotFoundErr(err) {
			return TaskDetail{}, ErrNotFound
		}
		return TaskDetail{}, err
	}
	return detail, nil
}

func (b *BoardService) snapshot() *cli.Client {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.client
}

// isNotFoundErr inspects an *ExitError's stderr for the CLI's not-found
// signature. The CLI returns exit code 1 with stderr `error: task FOO not
// found in any directory ...`.
func isNotFoundErr(err error) bool {
	var exit *cli.ExitError
	if !errors.As(err, &exit) {
		return false
	}
	return strings.Contains(exit.Stderr, "not found in any directory")
}
