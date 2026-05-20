package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// listMultiFixture describes a single task seeded into the temp board for
// multi-value filter tests. Fields the tests don't filter on get sensible
// defaults so writeMultiFilterTask can produce valid task .md files.
type listMultiFixture struct {
	status      string
	id          string
	title       string
	taskType    string
	priority    string
	size        string
	module      string
	tags        string
	parent      string
	agent       string
	agentStatus string
}

func seedMultiFilterBoard(t *testing.T) string {
	t.Helper()

	boardDir := newCommandTestBoard(t)
	fixtures := []listMultiFixture{
		{status: "backlog", id: "TB-10", title: "Bug in gui router", taskType: "bug", priority: "P0", size: "S", module: "gui", tags: "macos,window", parent: "TB-1", agent: "claude"},
		{status: "backlog", id: "TB-11", title: "Improve cli output", taskType: "improvement", priority: "P1", size: "M", module: "cli", tags: "window", parent: "TB-2", agent: "codex"},
		{status: "backlog", id: "TB-12", title: "Feature in docs", taskType: "feature", priority: "P2", size: "L", module: "docs", tags: "macos", parent: "TB-1", agent: ""},
		{status: "backlog", id: "TB-13", title: "Tech debt elsewhere", taskType: "tech-debt", priority: "P2", size: "XL", module: "infra", tags: "performance", parent: "TB-3", agent: "claude"},
		{status: "backlog", id: "TB-14", title: "Router refactor", taskType: "bug", priority: "P1", size: "S", module: "gui-frontend", tags: "needs-split", parent: "", agent: "codex"},
	}
	for _, f := range fixtures {
		writeMultiFilterTask(t, boardDir, f)
	}
	return boardDir
}

func writeMultiFilterTask(t *testing.T, boardDir string, f listMultiFixture) {
	t.Helper()

	lines := []string{
		"# " + f.id + ": " + f.title,
		"",
		"**Type:** " + f.taskType,
		"**Priority:** " + f.priority,
		"**Size:** " + f.size,
		"**Module:** " + f.module,
		"**Tags:** " + f.tags,
	}
	if f.parent != "" {
		lines = append(lines, "**Parent:** "+f.parent)
	}
	if f.agent != "" {
		lines = append(lines, "**Agent:** "+f.agent)
	}
	if f.agentStatus != "" {
		lines = append(lines, "**AgentStatus:** "+f.agentStatus)
	}
	lines = append(lines,
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"Fixture body.",
		"",
		"## Acceptance Criteria",
		"",
		"- [ ] Exercise filtering.",
		"",
		"## Log",
		"",
		"- 2026-05-20: Created",
		"",
	)

	writeFileForTest(t, filepath.Join(boardDir, f.status, f.id+".md"), strings.Join(lines, "\n"))
}

func runListJSON(t *testing.T, args ...string) []taskJSON {
	t.Helper()

	out := captureStdout(t, func() {
		cmdList(args)
	})
	var got []taskJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal ls JSON: %v\n%s", err, out)
	}
	return got
}

func TestListMultiValueFilters(t *testing.T) {
	// Sort order in expected slices follows listPriorityLess: P0 < P1 < P2 < P3,
	// ties broken by ascending numeric ID. The fixture spread is:
	//   TB-10 P0, TB-11 P1, TB-12 P2, TB-13 P2, TB-14 P1.
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{
			// (a) -T bug,improvement matches both types
			name: "type OR matches both",
			args: []string{"--json", "--status", "backlog", "-T", "bug,improvement"},
			want: []string{"TB-10", "TB-11", "TB-14"},
		},
		{
			// (b) -p P0,P1 matches both priorities
			name: "priority OR matches both",
			args: []string{"--json", "--status", "backlog", "-p", "P0,P1"},
			want: []string{"TB-10", "TB-11", "TB-14"},
		},
		{
			// (c) -m gui,cli — substring + case-insensitive: 'gui' matches
			// 'gui' and 'gui-frontend'; 'Cli' matches 'cli'.
			name: "module OR substring case-insensitive",
			args: []string{"--json", "--status", "backlog", "-m", "GUI,Cli"},
			want: []string{"TB-10", "TB-11", "TB-14"},
		},
		{
			// (d) -s S,M matches both
			name: "size OR matches both",
			args: []string{"--json", "--status", "backlog", "-s", "S,M"},
			want: []string{"TB-10", "TB-11", "TB-14"},
		},
		{
			// (e) -t macos,window matches tasks with either tag
			name: "tag OR matches any task tag",
			args: []string{"--json", "--status", "backlog", "-t", "macos,window"},
			want: []string{"TB-10", "TB-11", "TB-12"},
		},
		{
			// (f) --parent 1,2 — numeric forms normalize, then match parents
			// TB-1 (TB-10, TB-12) and TB-2 (TB-11).
			name: "parent OR with numeric normalization",
			args: []string{"--json", "--status", "backlog", "--parent", "1,2"},
			want: []string{"TB-10", "TB-11", "TB-12"},
		},
		{
			// (g) --agent claude,codex matches both. Result ordering is
			// priority-then-id: P0 (TB-10), P1 (TB-11, TB-14), P2 (TB-13).
			name: "agent OR matches claude or codex",
			args: []string{"--json", "--status", "backlog", "--agent", "claude,codex"},
			want: []string{"TB-10", "TB-11", "TB-14", "TB-13"},
		},
		{
			// (g) --agent none matches unassigned
			name: "agent none matches unassigned",
			args: []string{"--json", "--status", "backlog", "--agent", "none"},
			want: []string{"TB-12"},
		},
		{
			// (h) --search router — title case-insensitive
			name: "search matches title case-insensitively",
			args: []string{"--json", "--status", "backlog", "--search", "ROUTER"},
			want: []string{"TB-10", "TB-14"},
		},
		{
			// (h) --search matches against task id substring
			name: "search matches id substring",
			args: []string{"--json", "--status", "backlog", "--search", "tb-13"},
			want: []string{"TB-13"},
		},
		{
			// (i) multi-flag AND: bug ∩ P1/P2 ∩ gui-prefixed modules ⇒ TB-14 only.
			name: "multi-flag AND",
			args: []string{"--json", "--status", "backlog", "-T", "bug", "-p", "P1,P2", "-m", "gui"},
			want: []string{"TB-14"},
		},
		{
			// (j) empty/whitespace segments are tolerated
			name: "empty whitespace segments tolerated",
			args: []string{"--json", "--status", "backlog", "-T", " bug , ,improvement , "},
			want: []string{"TB-10", "TB-11", "TB-14"},
		},
		{
			// (k) unknown existing-filter values keep current no-match behavior
			name: "unknown type returns no matches",
			args: []string{"--json", "--status", "backlog", "-T", "nonsense"},
			want: nil,
		},
		{
			// Single-value calls preserve identical output for compatibility.
			name: "single value preserved",
			args: []string{"--json", "--status", "backlog", "-T", "bug"},
			want: []string{"TB-10", "TB-14"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_ = seedMultiFilterBoard(t)
			got := runListJSON(t, tc.args...)
			assertTaskIDs(t, got, tc.want)
		})
	}
}

func TestListMultiAgentInvalidValueIsError(t *testing.T) {
	// (k, second half) unknown --agent values fail. The validator lives in
	// buildListFilters so we can exercise it without triggering the
	// flag.ExitOnError os.Exit path used by cmdList.
	_, err := buildListFilters("", "", "", "", "", "", "claude,bogus", "")
	if err == nil {
		t.Fatalf("expected error for invalid agent, got nil")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("expected error to mention bogus, got %v", err)
	}
}

func TestListMultiAgentLowercases(t *testing.T) {
	// Canonicalize to lowercase so case-insensitive matching works against
	// the Task.Agent field (which is also lowercased when written by tb edit).
	f, err := buildListFilters("", "", "", "", "", "", "Claude,CODEX,None", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	want := []string{"claude", "codex", "none"}
	if len(f.agents) != len(want) {
		t.Fatalf("agents: got %v, want %v", f.agents, want)
	}
	for i, v := range f.agents {
		if v != want[i] {
			t.Fatalf("agents[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func TestListMultiSplitCSVHelper(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{in: "", want: nil},
		{in: "   ", want: nil},
		{in: ",,,", want: nil},
		{in: "bug", want: []string{"bug"}},
		{in: "bug,improvement", want: []string{"bug", "improvement"}},
		{in: " bug , improvement ", want: []string{"bug", "improvement"}},
		{in: ",bug,,improvement,", want: []string{"bug", "improvement"}},
	}

	for _, tc := range cases {
		got := splitCSVFilter(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("splitCSVFilter(%q): got %v, want %v", tc.in, got, tc.want)
		}
		for i, v := range got {
			if v != tc.want[i] {
				t.Fatalf("splitCSVFilter(%q)[%d] = %q, want %q", tc.in, i, v, tc.want[i])
			}
		}
	}
}

func TestListMultiSearchCommaNotSplit(t *testing.T) {
	// --search accepts commas literally because commas are valid title text.
	// If the implementation split on commas (like the other multi-value flags)
	// the term "gui,router" would split into "gui" OR "router" and match
	// TB-10 and TB-14. Since no title contains the literal substring
	// "gui,router", the correct result is the empty set.
	_ = seedMultiFilterBoard(t)
	got := runListJSON(t, "--json", "--status", "backlog", "--search", "gui,router")
	if len(got) != 0 {
		t.Fatalf("--search treated comma as separator: got %d matches, want 0\n%+v", len(got), got)
	}
}
