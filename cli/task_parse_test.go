package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTaskFileRequiresValidHeader(t *testing.T) {
	prevPrefix := cfg.Prefix
	cfg.Prefix = "TB"
	t.Cleanup(func() { cfg.Prefix = prevPrefix })

	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "valid header without optional metadata",
			content: strings.Join([]string{
				"# TB-123: Valid task",
				"",
				"## Goal",
				"",
				"Do the work.",
				"",
			}, "\n"),
		},
		{
			name: "missing matching header",
			content: strings.Join([]string{
				"**Type:** bug",
				"**Priority:** P2",
				"",
				"## Goal",
				"",
			}, "\n"),
			wantErr: "missing task header",
		},
		{
			name: "header after metadata scan limit",
			content: strings.Repeat("metadata\n", maxMetadataLines) +
				"# TB-123: Too late\n",
			wantErr: "missing task header",
		},
		{
			name:    "header without colon",
			content: "# TB-123\n\n**Type:** bug\n",
			wantErr: "malformed task header",
		},
		{
			name:    "header with empty title",
			content: "# TB-123:\n\n**Type:** bug\n",
			wantErr: "malformed task header",
		},
		{
			name:    "header with whitespace title",
			content: "# TB-123:    \n\n**Type:** bug\n",
			wantErr: "malformed task header",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "TB-123.md")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatalf("write task: %v", err)
			}

			task, err := parseTaskFile(path)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("parseTaskFile returned error: %v", err)
				}
				if task.ID != "TB-123" || task.Title != "Valid task" {
					t.Fatalf("parsed task = %+v, want TB-123 / Valid task", task)
				}
				return
			}

			if err == nil {
				t.Fatalf("parseTaskFile succeeded, want %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("parseTaskFile error = %q, want substring %q", err, tc.wantErr)
			}
		})
	}
}
