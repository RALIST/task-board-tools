package epicorder

import "testing"

func task(id string, n int, parent, status, agentStatus string, tags ...string) Task {
	return Task{
		ID:          id,
		Numeric:     n,
		Parent:      parent,
		Status:      status,
		AgentStatus: agentStatus,
		Tags:        tags,
	}
}

func TestNoParent(t *testing.T) {
	cand := task("TB-9", 9, "", "ready", "")
	got := EligibleForEpicOrder(cand, nil)
	if !got.Eligible {
		t.Errorf("no-parent candidate should be eligible: %+v", got)
	}
}

func TestFirstChildIsEligible(t *testing.T) {
	cand := task("TB-178", 178, "TB-177", "ready", "")
	// Only sibling is the candidate itself.
	got := EligibleForEpicOrder(cand, []Task{cand})
	if !got.Eligible {
		t.Errorf("first child of an epic should be eligible: %+v", got)
	}
}

func TestBlockedByEarlierBacklogSibling(t *testing.T) {
	cand := task("TB-200", 200, "TB-177", "ready", "")
	siblings := []Task{
		task("TB-178", 178, "TB-177", "backlog", ""),
		cand,
	}
	got := EligibleForEpicOrder(cand, siblings)
	if got.Eligible {
		t.Errorf("expected block; got %+v", got)
	}
	if got.BlockerID != "TB-178" {
		t.Errorf("expected blocker TB-178, got %q", got.BlockerID)
	}
}

func TestUnblockedWhenAllEarlierAreDone(t *testing.T) {
	cand := task("TB-200", 200, "TB-177", "ready", "")
	siblings := []Task{
		task("TB-178", 178, "TB-177", "done", ""),
		task("TB-179", 179, "TB-177", "done", ""),
		cand,
	}
	got := EligibleForEpicOrder(cand, siblings)
	if !got.Eligible {
		t.Errorf("expected unblocked when all earlier siblings done; got %+v", got)
	}
}

func TestReviewFailedLaterChildStillBlocked(t *testing.T) {
	// A later child carrying `review-failed` does not bypass an
	// unfinished earlier sibling: epic order overrides the priority
	// boost (TB-233).
	cand := task("TB-200", 200, "TB-177", "ready", "", "review-failed")
	siblings := []Task{
		task("TB-178", 178, "TB-177", "backlog", ""),
		cand,
	}
	got := EligibleForEpicOrder(cand, siblings)
	if got.Eligible {
		t.Errorf("review-failed later child should still be blocked: %+v", got)
	}
}

func TestArchivedEarlierSiblingTreatedAsClosed(t *testing.T) {
	cand := task("TB-200", 200, "TB-177", "ready", "")
	siblings := []Task{
		task("TB-178", 178, "TB-177", "archive", ""),
		cand,
	}
	got := EligibleForEpicOrder(cand, siblings)
	if !got.Eligible {
		t.Errorf("archived earlier sibling should unblock per board convention: %+v", got)
	}
}

func TestMissingEarlierSiblingBlocks(t *testing.T) {
	// Sibling entry with Status="" represents an expected predecessor
	// that the caller couldn't load (deleted file, parse error, etc.).
	// Conservative behavior: block with diagnostic.
	cand := task("TB-200", 200, "TB-177", "ready", "")
	siblings := []Task{
		{ID: "TB-178", Numeric: 178, Parent: "TB-177"},
		cand,
	}
	got := EligibleForEpicOrder(cand, siblings)
	if got.Eligible {
		t.Errorf("missing predecessor must block; got %+v", got)
	}
	if got.BlockerID != "TB-178" {
		t.Errorf("expected blocker TB-178, got %q", got.BlockerID)
	}
}

func TestSiblingWithNeedsUserBlocks(t *testing.T) {
	cand := task("TB-200", 200, "TB-177", "ready", "")
	siblings := []Task{
		task("TB-178", 178, "TB-177", "in-progress", "needs-user"),
		cand,
	}
	got := EligibleForEpicOrder(cand, siblings)
	if got.Eligible {
		t.Errorf("needs-user sibling must block: %+v", got)
	}
}

func TestSiblingWithInterruptedBlocks(t *testing.T) {
	cand := task("TB-200", 200, "TB-177", "ready", "")
	// Even if the sibling looks like it's in `done`, an interrupted
	// AgentStatus indicates the run never landed cleanly — block.
	// (In practice interrupted lives in active columns, but the
	// invariant is checked regardless.)
	siblings := []Task{
		task("TB-178", 178, "TB-177", "in-progress", "interrupted"),
		cand,
	}
	got := EligibleForEpicOrder(cand, siblings)
	if got.Eligible {
		t.Errorf("interrupted sibling must block: %+v", got)
	}
}

func TestSiblingWithCancelledBlocks(t *testing.T) {
	cand := task("TB-200", 200, "TB-177", "ready", "")
	siblings := []Task{
		task("TB-178", 178, "TB-177", "in-progress", "cancelled"),
		cand,
	}
	got := EligibleForEpicOrder(cand, siblings)
	if got.Eligible {
		t.Errorf("cancelled sibling must block: %+v", got)
	}
}

func TestEpicCardIsNotACandidate(t *testing.T) {
	// A task tagged `epic` is the parent card, never a leaf candidate.
	cand := task("TB-177", 177, "", "ready", "", "epic")
	got := EligibleForEpicOrder(cand, []Task{cand})
	if got.Eligible {
		t.Errorf("epic-tagged task should not be eligible: %+v", got)
	}
}

func TestUnrelatedSiblingsAreIgnored(t *testing.T) {
	cand := task("TB-200", 200, "TB-177", "ready", "")
	siblings := []Task{
		// Different parent — should not affect candidate.
		task("TB-50", 50, "TB-42", "backlog", ""),
		// Same numeric direction (lower) but different parent — ignore.
		task("TB-100", 100, "TB-9", "in-progress", ""),
		cand,
	}
	got := EligibleForEpicOrder(cand, siblings)
	if !got.Eligible {
		t.Errorf("unrelated siblings should be ignored: %+v", got)
	}
}

func TestHigherIDSiblingsAreIgnored(t *testing.T) {
	cand := task("TB-200", 200, "TB-177", "ready", "")
	siblings := []Task{
		task("TB-300", 300, "TB-177", "backlog", ""), // later sibling — irrelevant
		cand,
	}
	got := EligibleForEpicOrder(cand, siblings)
	if !got.Eligible {
		t.Errorf("higher-id sibling must not block: %+v", got)
	}
}

func TestParseNumeric(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
		ok   bool
	}{
		{"TB-1", 1, true},
		{"TB-177", 177, true},
		{"TB-", 0, false},
		{"177", 0, false},
		{"", 0, false},
		{"PROJ-42", 42, true},
		{"TB--1", 0, false},
	} {
		got, ok := ParseNumeric(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseNumeric(%q) = (%d, %v); want (%d, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
