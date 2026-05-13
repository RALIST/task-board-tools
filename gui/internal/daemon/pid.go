package daemon

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// PidAliveForRecovery is the exported pidAlive entry point used by the
// production wiring (gui/main.go). The recovery service in app/ takes
// a function injection so tests can swap in fakes without depending on
// this package; production passes this symbol verbatim.
var PidAliveForRecovery = pidAlive

// pidAlive reports whether the given PID is still alive AND its command
// looks like the expected agent. Used by stale-recovery (TB-60) to
// distinguish "process really did crash" (recover the task) from "an
// unrelated process happens to have the same PID after reuse" (leave the
// task alone — R10 mitigation).
//
// Two-step name check:
//
//  1. `ps -o comm= -p <pid>` → basename match against expectedAgent.
//     macOS truncates `comm` at 16 chars; we still match the basename
//     exactly so `claude` does NOT match `claude2` or `claude-bin`.
//
//  2. `ps -o args= -p <pid>` → tokenise on whitespace, accept if any
//     token's basename exact-matches expectedAgent. This is the
//     node-wrapper fallback: npm-installed `claude` is `#!/usr/bin/env
//     node` shebang, so step 1 returns "node"; step 2 finds
//     "/usr/local/bin/claude" in argv and accepts.
//
// If `ps` is not on PATH we don't fail the recovery — over-recovery is
// worse than under-recovery. We return the liveness result (alive→true,
// dead→false) and let the caller log a warning.
//
// Liveness semantics: signal 0 returns nil for alive, ESRCH for dead,
// EPERM for "alive but not ours" (rare on macOS/Linux for our own
// children but possible after a UID switch — still alive).
func pidAlive(pid int, expectedAgent string) bool {
	if pid <= 0 {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		// ESRCH → dead.
		if errors.Is(err, syscall.ESRCH) {
			return false
		}
		// EPERM → alive but owned by someone else; treat as alive.
		// Anything else: be conservative and treat as alive (the goal
		// is to NOT over-recover when uncertain).
	}

	// At this point the PID is alive; check the command name.
	psPath, lpErr := exec.LookPath("ps")
	if lpErr != nil {
		// No `ps` — return liveness only (don't over-recover).
		return true
	}

	if commMatches(psPath, pid, expectedAgent) {
		return true
	}
	if argsMatches(psPath, pid, expectedAgent) {
		return true
	}
	return false
}

func commMatches(psPath string, pid int, expectedAgent string) bool {
	out, err := exec.Command(psPath, "-o", "comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false
	}
	base := filepath.Base(strings.TrimSpace(string(out)))
	return base == expectedAgent
}

func argsMatches(psPath string, pid int, expectedAgent string) bool {
	out, err := exec.Command(psPath, "-o", "args=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false
	}
	for _, tok := range strings.Fields(string(out)) {
		if filepath.Base(tok) == expectedAgent {
			return true
		}
	}
	return false
}
