package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAllocateID_SkipsExistingFiles verifies that allocateID bumps past any
// ID whose task file already exists in ANY status directory (backlog,
// in-progress, done, archive). This guards against overwriting prior work
// when .next-id gets out of sync with directory state.
func TestAllocateID_SkipsExistingFiles(t *testing.T) {
	prevPrefix := cfg.Prefix
	cfg.Prefix = "WS"
	defer func() { cfg.Prefix = prevPrefix }()

	cases := []struct {
		name       string
		takenDir   string
		takenIDs   []int
		startID    int
		wantID     int
		wantNextID int
	}{
		{
			name:       "no collision returns current id",
			takenDir:   "",
			startID:    10,
			wantID:     10,
			wantNextID: 11,
		},
		{
			name:       "collides in backlog bumps to next",
			takenDir:   "backlog",
			takenIDs:   []int{5},
			startID:    5,
			wantID:     6,
			wantNextID: 7,
		},
		{
			name:       "collides in in-progress bumps to next",
			takenDir:   "in-progress",
			takenIDs:   []int{7},
			startID:    7,
			wantID:     8,
			wantNextID: 9,
		},
		{
			name:       "collides in done bumps to next",
			takenDir:   "done",
			takenIDs:   []int{12},
			startID:    12,
			wantID:     13,
			wantNextID: 14,
		},
		{
			name:       "collides in archive bumps to next",
			takenDir:   "archive",
			takenIDs:   []int{20},
			startID:    20,
			wantID:     21,
			wantNextID: 22,
		},
		{
			name:       "skips a contiguous run of taken ids across dirs",
			takenDir:   "mixed",
			takenIDs:   []int{15, 16, 17},
			startID:    15,
			wantID:     18,
			wantNextID: 19,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			boardDir := t.TempDir()
			// Create all status directories + archive.
			for _, d := range []string{"backlog", "in-progress", "done", "archive"} {
				if err := os.MkdirAll(filepath.Join(boardDir, d), 0755); err != nil {
					t.Fatalf("mkdir %s: %v", d, err)
				}
			}

			// Seed taken files. "mixed" spreads across multiple dirs.
			dirs := []string{"backlog", "in-progress", "done", "archive"}
			for i, id := range tc.takenIDs {
				var dir string
				switch tc.takenDir {
				case "mixed":
					dir = dirs[i%len(dirs)]
				default:
					dir = tc.takenDir
				}
				if dir == "" {
					continue
				}
				path := filepath.Join(boardDir, dir, fmt.Sprintf("%s-%d.md", cfg.Prefix, id))
				if err := os.WriteFile(path, []byte("# existing\n"), 0644); err != nil {
					t.Fatalf("seed %s: %v", path, err)
				}
			}

			// Seed .next-id.
			nextIDPath := filepath.Join(boardDir, ".next-id")
			if err := os.WriteFile(nextIDPath, []byte(fmt.Sprintf("%d\n", tc.startID)), 0644); err != nil {
				t.Fatalf("seed .next-id: %v", err)
			}

			// Capture stderr for warning detection on collision.
			gotID, err := allocateID(boardDir)
			if err != nil {
				t.Fatalf("allocateID: %v", err)
			}
			if gotID != tc.wantID {
				t.Errorf("allocateID returned %d, want %d", gotID, tc.wantID)
			}

			data, err := os.ReadFile(nextIDPath)
			if err != nil {
				t.Fatalf("read .next-id: %v", err)
			}
			gotNext := strings.TrimSpace(string(data))
			wantNext := fmt.Sprintf("%d", tc.wantNextID)
			if gotNext != wantNext {
				t.Errorf(".next-id = %q, want %q", gotNext, wantNext)
			}
		})
	}
}
