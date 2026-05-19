// Package cli wraps invocations of the `tb` binary.
//
// Every Wails service that mutates or reads board state goes through Client,
// so cross-cutting concerns — binary discovery, exit-code handling, stderr
// capture, JSON decoding, and context cancellation — live in exactly one
// place.
//
// Usage:
//
//	c, err := cli.NewClient(cli.Options{Cwd: projectRoot})
//	if err != nil { /* tb not on PATH; surface to user */ }
//	var tasks []Task
//	if err := c.RunJSON(ctx, &tasks, "ls", "--json", "--status", "active"); err != nil {
//	    var exit *cli.ExitError
//	    if errors.As(err, &exit) { /* exit.Code, exit.Stderr */ }
//	}
//
// Stdout from the CLI is returned to the caller. Stderr is captured and
// surfaced two ways: a bounded copy is attached to ExitError on non-zero exit,
// and the full text is forwarded to the slog logger configured via
// Options.Logger (or slog.Default() if unset).
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
)

// stderrTruncateBytes bounds how much stderr text is attached to ExitError.
// Full stderr is still forwarded to the logger.
const stderrTruncateBytes = 4096

// Options configures Client construction.
type Options struct {
	// BinaryPath, if non-empty, overrides PATH lookup. Useful when the GUI
	// stores an absolute path in user settings.
	BinaryPath string
	// Cwd is the working directory for every invocation. Typically the project
	// root that contains .tb.yaml. Empty = inherit from caller.
	Cwd string
	// Logger receives full stderr output and execution traces. Defaults to
	// slog.Default() when nil.
	Logger *slog.Logger
}

// Client invokes the `tb` CLI. Safe for concurrent use — every call spawns a
// fresh process.
type Client struct {
	binaryPath string
	cwd        string
	logger     *slog.Logger
}

// ExitError reports a non-zero `tb` exit. Stderr is truncated for log safety.
type ExitError struct {
	Args   []string
	Code   int
	Stderr string
}

func (e *ExitError) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("tb %v: exit %d", e.Args, e.Code)
	}
	return fmt.Sprintf("tb %v: exit %d: %s", e.Args, e.Code, e.Stderr)
}

// ErrBinaryNotFound is returned by NewClient when neither the configured path
// nor `exec.LookPath("tb")` resolves to an executable.
var ErrBinaryNotFound = errors.New("tb binary not found (set settings.cli_path or add `tb` to PATH)")

// NewClient resolves the `tb` binary and returns a ready Client.
//
// Resolution order:
//  1. opts.BinaryPath if set — verified via exec.LookPath so a relative path
//     also works.
//  2. exec.LookPath("tb").
//
// When neither resolves, NewClient returns ErrBinaryNotFound.
func NewClient(opts Options) (*Client, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	bin := opts.BinaryPath
	if bin == "" {
		bin = "tb"
	}
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return nil, ErrBinaryNotFound
	}

	return &Client{
		binaryPath: resolved,
		cwd:        opts.Cwd,
		logger:     logger.With("component", "cli", "bin", resolved),
	}, nil
}

// BinaryPath returns the resolved absolute path to the `tb` binary.
func (c *Client) BinaryPath() string { return c.binaryPath }

// Cwd returns the project root the client was constructed with. Empty when
// the client inherits the caller's cwd. Exposed so AgentService can pass
// the same project root to spawned agent processes.
func (c *Client) Cwd() string { return c.cwd }

// Run executes `tb args...` and returns its stdout.
//
// Stderr is forwarded to the logger and (on non-zero exit) attached to an
// *ExitError, which callers can detect via errors.As. Cancelling ctx kills the
// process tree.
func (c *Client) Run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.binaryPath, args...)
	if c.cwd != "" {
		cmd.Dir = c.cwd
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	stderrText := stderr.String()

	if stderrText != "" {
		c.logger.Debug("tb stderr", "args", args, "stderr", stderrText)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return stdout.Bytes(), &ExitError{
				Args:   append([]string(nil), args...),
				Code:   exitErr.ExitCode(),
				Stderr: truncate(stderrText, stderrTruncateBytes),
			}
		}
		return stdout.Bytes(), fmt.Errorf("tb %v: %w", args, err)
	}

	return stdout.Bytes(), nil
}

// RunWithStdin executes `tb args...` with stdin streamed from r. Same error
// shape as Run; used by review-section writers that pipe replacement content
// in via `-`.
func (c *Client) RunWithStdin(ctx context.Context, r io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.binaryPath, args...)
	if c.cwd != "" {
		cmd.Dir = c.cwd
	}
	cmd.Stdin = r

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	stderrText := stderr.String()

	if stderrText != "" {
		c.logger.Debug("tb stderr", "args", args, "stderr", stderrText)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return stdout.Bytes(), &ExitError{
				Args:   append([]string(nil), args...),
				Code:   exitErr.ExitCode(),
				Stderr: truncate(stderrText, stderrTruncateBytes),
			}
		}
		return stdout.Bytes(), fmt.Errorf("tb %v: %w", args, err)
	}

	return stdout.Bytes(), nil
}

// RunJSON executes `tb args...` and decodes its stdout into out.
//
// Returns the same error shape as Run, plus a wrapped json.SyntaxError /
// json.UnmarshalTypeError if the CLI emitted invalid JSON.
func (c *Client) RunJSON(ctx context.Context, out any, args ...string) error {
	stdout, err := c.Run(ctx, args...)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(stdout)) == 0 {
		return fmt.Errorf("tb %v: empty stdout (expected JSON)", args)
	}
	if err := json.Unmarshal(stdout, out); err != nil {
		return fmt.Errorf("tb %v: decode JSON: %w", args, err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// compile-time assertion that *bytes.Buffer satisfies io.Writer (sanity check
// against a stdlib API drift; cost is zero).
var _ io.Writer = (*bytes.Buffer)(nil)
