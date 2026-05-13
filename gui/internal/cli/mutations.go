// Mutation wrappers around `tb create/edit/mv/close/regenerate`.
//
// Every wrapper returns a typed error so callers can branch on stable
// categories rather than parsing stderr at the call site. Stderr is
// pattern-matched once here, attached to the returned error, and the raw
// payload is preserved in *MutationError.Stderr.

package cli

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// MutationErrKind enumerates the structured failure shapes a CLI mutation can
// produce. Callers branch on this via errors.As(&me) + me.Kind.
type MutationErrKind int

const (
	// ErrKindUnknown is the fall-through bucket. Inspect Stderr for context.
	ErrKindUnknown MutationErrKind = iota
	// ErrKindBinaryNotFound means exec.LookPath couldn't resolve `tb`.
	ErrKindBinaryNotFound
	// ErrKindBoardNotFound means the CLI couldn't locate `.tb.yaml`.
	ErrKindBoardNotFound
	// ErrKindValidation means the CLI rejected an argument (bad priority,
	// missing task ID, unknown status, etc).
	ErrKindValidation
	// ErrKindTaskNotFound means the requested ID doesn't exist on disk.
	ErrKindTaskNotFound
)

// MutationError wraps a CLI failure with a stable Kind. The underlying
// *ExitError (if any) is preserved on Cause so callers can drill into
// exit codes when needed.
type MutationError struct {
	Kind   MutationErrKind
	Op     string   // command name: "create", "edit", "mv", "close", "regenerate"
	Args   []string // full args passed to tb
	Stderr string
	Cause  error
}

func (e *MutationError) Error() string {
	prefix := fmt.Sprintf("tb %s: ", e.Op)
	switch e.Kind {
	case ErrKindBinaryNotFound:
		return prefix + "tb binary not found"
	case ErrKindBoardNotFound:
		return prefix + "no .tb.yaml found"
	case ErrKindValidation:
		if e.Stderr != "" {
			return prefix + "validation: " + strings.TrimSpace(e.Stderr)
		}
		return prefix + "validation"
	case ErrKindTaskNotFound:
		return prefix + "task not found"
	default:
		if e.Stderr != "" {
			return prefix + strings.TrimSpace(e.Stderr)
		}
		if e.Cause != nil {
			return prefix + e.Cause.Error()
		}
		return prefix + "unknown failure"
	}
}

func (e *MutationError) Unwrap() error { return e.Cause }

// classify maps stderr substrings to a MutationErrKind. The patterns mirror
// the CLI's actual error messages (see cli/board.go, cli/edit.go, etc).
//
// When extending this, prefer specific substrings over generic words —
// "invalid " alone would also match systemic errors like "invalid .next-id
// content" which are not validation failures from the caller's perspective.
func classify(stderr string) MutationErrKind {
	s := strings.ToLower(stderr)
	switch {
	case strings.Contains(s, "not found in any directory"):
		return ErrKindTaskNotFound
	case strings.Contains(s, "board not found"),
		strings.Contains(s, "does not contain .next-id"),
		strings.Contains(s, "tb_board_dir"):
		return ErrKindBoardNotFound
	case strings.Contains(s, "invalid priority"),
		strings.Contains(s, "invalid type"),
		strings.Contains(s, "invalid size"),
		strings.Contains(s, "invalid agent"),
		strings.Contains(s, "invalid agent-status"),
		strings.Contains(s, "title is required"),
		strings.Contains(s, "task id is required"),
		strings.Contains(s, "no changes specified"),
		strings.Contains(s, "unknown status"):
		return ErrKindValidation
	default:
		return ErrKindUnknown
	}
}

// wrapMutation converts a Run() error into a MutationError. Returns nil when
// err is nil.
func wrapMutation(op string, args []string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrBinaryNotFound) {
		return &MutationError{Kind: ErrKindBinaryNotFound, Op: op, Args: args, Cause: err}
	}
	var exit *ExitError
	if errors.As(err, &exit) {
		return &MutationError{
			Kind:   classify(exit.Stderr),
			Op:     op,
			Args:   args,
			Stderr: exit.Stderr,
			Cause:  err,
		}
	}
	return &MutationError{Kind: ErrKindUnknown, Op: op, Args: args, Cause: err}
}

// CreateInput is the shape consumed by Client.Create. Empty fields are
// skipped — the CLI applies its own defaults (type=bug, priority=P2, size=M).
type CreateInput struct {
	Title       string
	Module      string
	Type        string
	Priority    string
	Size        string
	Tags        string // comma-separated
	Description string
	Parent      string
	Epic        bool
}

// createdPathRe matches the CLI's `Created <path>` stdout line. The capture
// group is the path; the ID is extracted from the basename (filename without
// the `.md` suffix). The path may contain spaces — relPath() doesn't escape
// them, so we anchor on the literal `.md` suffix at end-of-line.
var createdPathRe = regexp.MustCompile(`(?m)^Created\s+(.+\.md)\s*$`)

// CreateResult holds the parsed outcome of `tb create`.
type CreateResult struct {
	ID   string // e.g. "TB-42"
	Path string // path as printed by tb (relative to cwd)
}

// Create runs `tb create "title" -m module …` and returns the new task ID,
// parsed from the CLI's `Created <path>` stdout line. Empty Input.Title is
// rejected before exec; everything else passes through.
func (c *Client) Create(ctx context.Context, in CreateInput) (CreateResult, error) {
	if strings.TrimSpace(in.Title) == "" {
		return CreateResult{}, &MutationError{Kind: ErrKindValidation, Op: "create", Stderr: "title is required"}
	}
	args := []string{"create", in.Title}
	if in.Module != "" {
		args = append(args, "-m", in.Module)
	}
	if in.Type != "" {
		args = append(args, "-T", in.Type)
	}
	if in.Priority != "" {
		args = append(args, "-p", in.Priority)
	}
	if in.Size != "" {
		args = append(args, "-s", in.Size)
	}
	if in.Tags != "" {
		args = append(args, "-t", in.Tags)
	}
	if in.Description != "" {
		args = append(args, "-d", in.Description)
	}
	if in.Parent != "" {
		args = append(args, "--parent", in.Parent)
	}
	if in.Epic {
		args = append(args, "--epic")
	}

	stdout, err := c.Run(ctx, args...)
	if err != nil {
		return CreateResult{}, wrapMutation("create", args, err)
	}

	m := createdPathRe.FindStringSubmatch(string(stdout))
	if m == nil {
		return CreateResult{}, &MutationError{
			Kind:   ErrKindUnknown,
			Op:     "create",
			Args:   args,
			Stderr: "could not parse `Created <path>` line from stdout: " + strings.TrimSpace(string(stdout)),
		}
	}
	path := m[1]
	id := idFromPath(path)
	if id == "" {
		return CreateResult{}, &MutationError{
			Kind:   ErrKindUnknown,
			Op:     "create",
			Args:   args,
			Stderr: "could not extract ID from path " + path,
		}
	}
	return CreateResult{ID: id, Path: path}, nil
}

// idFromPath turns "board/backlog/TB-42.md" into "TB-42". Returns "" if the
// basename doesn't match the expected <PREFIX>-<digits>.md shape.
var idBasenameRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9]*-\d+)\.md$`)

func idFromPath(p string) string {
	// strip directories
	base := p
	if i := strings.LastIndexAny(p, "/\\"); i >= 0 {
		base = p[i+1:]
	}
	m := idBasenameRe.FindStringSubmatch(base)
	if m == nil {
		return ""
	}
	return m[1]
}

// EditInput is the shape consumed by Client.Edit. Empty fields are skipped.
// Caller is responsible for passing at least one non-empty field; otherwise
// the CLI rejects the call with "no changes specified" (mapped to
// ErrKindValidation).
type EditInput struct {
	Priority    string
	Type        string
	Size        string
	Module      string
	Tags        string // comma-separated; replaces existing tags
	Agent       string
	AgentStatus string
}

// HasChanges reports whether any field is set.
func (in EditInput) HasChanges() bool {
	return in.Priority != "" || in.Type != "" || in.Size != "" ||
		in.Module != "" || in.Tags != "" || in.Agent != "" || in.AgentStatus != ""
}

// Edit runs `tb edit <id> [flags]`. Returns a MutationError on any failure.
func (c *Client) Edit(ctx context.Context, id string, in EditInput) error {
	if strings.TrimSpace(id) == "" {
		return &MutationError{Kind: ErrKindValidation, Op: "edit", Stderr: "task id is required"}
	}
	if !in.HasChanges() {
		return &MutationError{Kind: ErrKindValidation, Op: "edit", Stderr: "no changes specified"}
	}
	args := []string{"edit", id}
	if in.Priority != "" {
		args = append(args, "-p", in.Priority)
	}
	if in.Type != "" {
		args = append(args, "-T", in.Type)
	}
	if in.Size != "" {
		args = append(args, "-s", in.Size)
	}
	if in.Module != "" {
		args = append(args, "-m", in.Module)
	}
	if in.Tags != "" {
		args = append(args, "-t", in.Tags)
	}
	if in.Agent != "" {
		args = append(args, "-a", in.Agent)
	}
	if in.AgentStatus != "" {
		args = append(args, "--agent-status", in.AgentStatus)
	}
	_, err := c.Run(ctx, args...)
	return wrapMutation("edit", args, err)
}

// Move runs `tb mv <id> <status>`. Status must be one of
// backlog | in-progress | done. The CLI accepts aliases (b, ip, d) but we
// pass through verbatim; callers should normalize before calling.
func (c *Client) Move(ctx context.Context, id, status string) error {
	if strings.TrimSpace(id) == "" {
		return &MutationError{Kind: ErrKindValidation, Op: "mv", Stderr: "task id is required"}
	}
	if strings.TrimSpace(status) == "" {
		return &MutationError{Kind: ErrKindValidation, Op: "mv", Stderr: "status is required"}
	}
	args := []string{"mv", id, status}
	_, err := c.Run(ctx, args...)
	return wrapMutation("mv", args, err)
}

// Close runs `tb close <id>` which archives the task.
func (c *Client) Close(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return &MutationError{Kind: ErrKindValidation, Op: "close", Stderr: "task id is required"}
	}
	args := []string{"close", id}
	_, err := c.Run(ctx, args...)
	return wrapMutation("close", args, err)
}

// Regenerate runs `tb regenerate`. Returns a MutationError on any failure.
func (c *Client) Regenerate(ctx context.Context) error {
	args := []string{"regenerate"}
	_, err := c.Run(ctx, args...)
	return wrapMutation("regenerate", args, err)
}
