// Package epicorder is the pure ordering-gate used by auto-implement
// candidate selection (TB-267): a child task is eligible for automatic
// implementation only when every same-parent sibling with a lower
// numeric ID is finished. The rule treats task ID order as
// implementation order, which is the working assumption for epics in
// this project.
//
// The package deliberately avoids depending on gui/app or the CLI so
// it can be used from both the daemon coordinator and unit tests
// without dragging in the world.
package epicorder

import (
	"fmt"
	"strconv"
	"strings"
)

// Task is the narrow projection epicorder needs. The caller maps
// from its real Task type. Numeric is parsed from the trailing digits
// of ID (e.g. "TB-177" → 177); ParseNumeric is the canonical helper.
//
// Status uses the canonical kanban directory names: backlog, ready,
// in-progress, code-review, done, archive. Empty status means
// "predecessor expected but cannot be loaded" — the function treats
// that conservatively as a blocking unknown so missing/deleted/
// malformed sibling files cannot silently advance the order.
type Task struct {
	ID          string
	Numeric     int
	Parent      string
	Status      string
	AgentStatus string
	Tags        []string
}

// Result is what EligibleForEpicOrder returns. Eligible is the
// decision; BlockerID and Reason are populated when Eligible is
// false so the daemon can surface "skipped because TB-X is not
// done yet" diagnostics.
type Result struct {
	Eligible  bool
	BlockerID string
	Reason    string
}

// ParseNumeric extracts the numeric component from an ID like "TB-177".
// Returns the int and ok=true on success. Used by callers that need
// the same parsing rule as the package internals.
func ParseNumeric(id string) (int, bool) {
	idx := strings.LastIndex(id, "-")
	// Require: at least one prefix char, the dash, and a non-empty
	// suffix. Reject double-dash forms like "TB--1" by ensuring the
	// character immediately before the last `-` is not itself a `-`.
	if idx <= 0 || idx >= len(id)-1 {
		return 0, false
	}
	if id[idx-1] == '-' {
		return 0, false
	}
	n, err := strconv.Atoi(id[idx+1:])
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// EligibleForEpicOrder reports whether candidate is allowed to be
// auto-implemented given its same-parent siblings. Returns:
//
//   - Eligible=true when there is no parent, no earlier sibling, or
//     every earlier sibling is closed (status=done OR status=archive).
//   - Eligible=false with BlockerID set when an earlier sibling is
//     still on the active board, blocked on user attention
//     (needs-user / interrupted / cancelled), or cannot be loaded
//     (Status==""); also when candidate itself is tagged `epic`.
//
// `siblings` should contain every sibling the caller is aware of —
// including the candidate. The function filters the candidate out and
// considers only those siblings whose Parent matches candidate.Parent
// and whose Numeric is lower than candidate.Numeric.
//
// Archived earlier siblings are treated as closed (board convention:
// archive is for obsolete/superseded/dropped work that should leave
// the active board; the user has explicitly retired the predecessor).
func EligibleForEpicOrder(candidate Task, siblings []Task) Result {
	// Tasks tagged `epic` are parent cards; they are never candidates
	// for auto-implement (the leaf children are).
	if containsTag(candidate.Tags, "epic") {
		return Result{
			Eligible:  false,
			BlockerID: candidate.ID,
			Reason:    fmt.Sprintf("%s is tagged epic and is not an implementation leaf", candidate.ID),
		}
	}

	// No parent → no ordering constraint.
	if candidate.Parent == "" {
		return Result{Eligible: true}
	}

	for _, sib := range siblings {
		if sib.ID == candidate.ID {
			continue
		}
		if sib.Parent != candidate.Parent {
			continue
		}
		if sib.Numeric >= candidate.Numeric {
			continue
		}

		// Unknown/unloadable predecessor → block conservatively.
		if sib.Status == "" {
			return Result{
				Eligible:  false,
				BlockerID: sib.ID,
				Reason:    fmt.Sprintf("earlier sibling %s could not be loaded; refusing to advance", sib.ID),
			}
		}

		// Closed (done or archived) predecessors do not block.
		if sib.Status == "done" || sib.Status == "archive" {
			continue
		}

		// User-attention blockers — explicit human / recovery cursors
		// that should not be skipped just because the task happens to
		// be in a status directory that would otherwise look closed.
		if sib.AgentStatus == "needs-user" ||
			sib.AgentStatus == "interrupted" ||
			sib.AgentStatus == "cancelled" {
			return Result{
				Eligible:  false,
				BlockerID: sib.ID,
				Reason: fmt.Sprintf(
					"earlier sibling %s has AgentStatus=%s (needs user attention)",
					sib.ID, sib.AgentStatus,
				),
			}
		}

		// Active-board predecessor (backlog / ready / in-progress /
		// code-review) — block until it lands in done or archive.
		return Result{
			Eligible:  false,
			BlockerID: sib.ID,
			Reason: fmt.Sprintf(
				"earlier sibling %s is still %s (must be done before %s)",
				sib.ID, sib.Status, candidate.ID,
			),
		}
	}

	return Result{Eligible: true}
}

func containsTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}
