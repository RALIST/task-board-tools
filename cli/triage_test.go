package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTriageJSONOutput(t *testing.T) {
	cases := []struct {
		name      string
		tasks     []triageTestTask
		wantEmpty bool
		wantIDs   []string
		want      map[string]triageJSONExpectation
	}{
		{
			name:      "empty backlog emits empty array",
			wantEmpty: true,
		},
		{
			name: "emits task metadata and grooming reasons",
			tasks: []triageTestTask{
				{
					id:       "TB-2",
					title:    "Ungroomed Generated Task",
					priority: "P2",
					size:     "M",
					module:   "",
					tags:     "scan, needs-grooming",
					goal:     "(to be filled)",
					accept:   "- [ ] (to be filled)",
					extra:    "\nCreated by `tb scan`.\n",
				},
				{
					id:       "TB-1",
					title:    "Missing Acceptance",
					priority: "P1",
					size:     "S",
					module:   "cli",
					tags:     "quick-win",
					goal:     "Make the command scriptable.",
					accept:   "- [ ] (to be filled)",
				},
			},
			wantIDs: []string{"TB-1", "TB-2"},
			want: map[string]triageJSONExpectation{
				"TB-1": {
					title:    "Missing Acceptance",
					priority: "P1",
					module:   "cli",
					tags:     []string{"quick-win"},
					reasons:  []string{"no acceptance criteria"},
				},
				"TB-2": {
					title:    "Ungroomed Generated Task",
					priority: "P2",
					module:   "",
					tags:     []string{"scan", "needs-grooming"},
					reasons:  []string{"no module", "no goal", "no acceptance criteria", "auto-created by scan"},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			boardDir := newCommandTestBoard(t)
			for _, task := range tc.tasks {
				writeTriageTestTask(t, boardDir, task)
			}

			out := captureStdout(t, func() {
				cmdTriage([]string{"--json"})
			})

			if tc.wantEmpty {
				if out != "[]\n" {
					t.Fatalf("empty JSON output = %q, want []\\n", out)
				}
				return
			}
			if strings.Contains(out, "Found ") || strings.Contains(out, "No tasks") {
				t.Fatalf("JSON output included prose:\n%s", out)
			}

			var got []struct {
				ID       string   `json:"id"`
				Title    string   `json:"title"`
				Priority string   `json:"priority"`
				Module   string   `json:"module"`
				Tags     []string `json:"tags"`
				Reasons  []string `json:"reasons"`
			}
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("unmarshal triage JSON: %v\n%s", err, out)
			}

			gotIDs := make([]string, 0, len(got))
			for _, item := range got {
				gotIDs = append(gotIDs, item.ID)
				want := tc.want[item.ID]
				if item.Title != want.title || item.Priority != want.priority || item.Module != want.module {
					t.Fatalf("%s metadata = %+v, want title=%q priority=%q module=%q", item.ID, item, want.title, want.priority, want.module)
				}
				if !reflect.DeepEqual(item.Tags, want.tags) {
					t.Fatalf("%s tags = %#v, want %#v", item.ID, item.Tags, want.tags)
				}
				if !reflect.DeepEqual(item.Reasons, want.reasons) {
					t.Fatalf("%s reasons = %#v, want %#v", item.ID, item.Reasons, want.reasons)
				}
			}
			if !reflect.DeepEqual(gotIDs, tc.wantIDs) {
				t.Fatalf("task order = %#v, want %#v", gotIDs, tc.wantIDs)
			}
		})
	}
}

type triageJSONExpectation struct {
	title    string
	priority string
	module   string
	tags     []string
	reasons  []string
}

type triageTestTask struct {
	id       string
	title    string
	priority string
	size     string
	module   string
	tags     string
	goal     string
	accept   string
	extra    string
}

func writeTriageTestTask(t *testing.T, boardDir string, task triageTestTask) {
	t.Helper()

	var b strings.Builder
	b.WriteString("# " + task.id + ": " + task.title + "\n\n")
	b.WriteString("**Type:** bug\n")
	if task.priority != "" {
		b.WriteString("**Priority:** " + task.priority + "\n")
	}
	if task.size != "" {
		b.WriteString("**Size:** " + task.size + "\n")
	}
	if task.module != "" {
		b.WriteString("**Module:** " + task.module + "\n")
	}
	if task.tags != "" {
		b.WriteString("**Tags:** " + task.tags + "\n")
	}
	b.WriteString("**Branch:** -\n\n")
	b.WriteString("## Goal\n\n")
	b.WriteString(task.goal + "\n\n")
	b.WriteString("## Acceptance Criteria\n\n")
	b.WriteString(task.accept + "\n\n")
	b.WriteString("## Log\n\n- 2026-05-14: Created\n")
	if task.extra != "" {
		b.WriteString(task.extra)
	}

	path := filepath.Join(boardDir, "backlog", task.id+".md")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
