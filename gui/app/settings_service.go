package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"tools/tb-gui/internal/cli"
)

const (
	recentBoardCap   = 20
	tbConfigFileName = ".tb.yaml"
	recentsFileName  = "recent.json"
	recentsDirName   = "tb-gui"
)

// BoardInfo is the parsed `.tb.yaml` for the currently-open board.
type BoardInfo struct {
	ProjectRoot string `json:"projectRoot"`
	BoardDir    string `json:"boardDir"`
	Prefix      string `json:"prefix"`
	WIPLimit    int    `json:"wipLimit"`
}

// RecentBoard is one entry in recent.json. LastOpened is RFC3339 so JSON stays
// human-readable; the frontend sorts by it.
type RecentBoard struct {
	ProjectRoot string    `json:"projectRoot"`
	Prefix      string    `json:"prefix"`
	LastOpened  time.Time `json:"lastOpened"`
}

// ErrNoTbYaml is returned by OpenBoard when a path doesn't contain `.tb.yaml`.
// The frontend turns this into a non-blocking toast and leaves the previous
// board active (per TB-2 acceptance).
var ErrNoTbYaml = errors.New("path has no .tb.yaml — not a tb project")

// Switcher is the contract SettingsService needs from the watcher. Passed in
// at construction so the service stays test-friendly.
type Switcher interface {
	Switch(boardDir string) error
}

// SettingsService manages project root selection, recent-board persistence,
// and the native folder picker. It also coordinates BoardService and the
// watcher whenever the active board changes.
type SettingsService struct {
	logger *slog.Logger
	board  *BoardService
	wch    Switcher

	mu      sync.RWMutex
	info    BoardInfo
	cliPath string // override for cli.NewClient; empty = PATH

	// recentsPath is the absolute path to recent.json. Configurable for tests.
	recentsPath string
}

// SettingsOptions tunes SettingsService construction.
type SettingsOptions struct {
	Logger      *slog.Logger
	Board       *BoardService
	Watcher     Switcher
	CLIPath     string // override for tests; empty = PATH lookup
	RecentsPath string // override for tests; empty = $XDG_CONFIG_HOME/tb-gui/recent.json
}

// NewSettingsService returns a SettingsService. Until OpenBoard is called the
// service has no active project — GetProjectRoot returns "" and
// ListRecentBoards still works from disk.
func NewSettingsService(opts SettingsOptions) *SettingsService {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	recents := opts.RecentsPath
	if recents == "" {
		recents = defaultRecentsPath()
	}
	return &SettingsService{
		logger:      logger.With("component", "settings"),
		board:       opts.Board,
		wch:         opts.Watcher,
		recentsPath: recents,
		cliPath:     opts.CLIPath,
	}
}

// ServiceName satisfies the Wails service contract.
func (s *SettingsService) ServiceName() string { return "SettingsService" }

// GetProjectRoot returns the active project root, or "" if no board is open.
func (s *SettingsService) GetProjectRoot() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.info.ProjectRoot
}

// GetBoardInfo returns the parsed `.tb.yaml` for the active board. When no
// board is open, returns a zero BoardInfo and ErrNoBoard.
func (s *SettingsService) GetBoardInfo() (BoardInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.info.ProjectRoot == "" {
		return BoardInfo{}, ErrNoBoard
	}
	return s.info, nil
}

// OpenBoard validates the project root, swaps the active CLI client, retargets
// the watcher, and prepends the entry to recent.json.
//
// On any failure the previous board state is left untouched and a typed error
// is returned (`ErrNoTbYaml` for missing config, plain error otherwise). The
// frontend dispatches errors to a toast and keeps the existing UI.
func (s *SettingsService) OpenBoard(ctx context.Context, projectRoot string) error {
	if projectRoot == "" {
		return errors.New("empty project root")
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("resolve abs path: %w", err)
	}

	info, err := readBoardInfo(absRoot)
	if err != nil {
		return err
	}

	// Build a CLI client rooted at the project so `tb` discovers `.tb.yaml`.
	client, err := cli.NewClient(cli.Options{
		BinaryPath: s.cliPath,
		Cwd:        info.ProjectRoot,
		Logger:     s.logger,
	})
	if err != nil {
		return fmt.Errorf("locate tb binary: %w", err)
	}

	if s.wch != nil {
		if err := s.wch.Switch(info.BoardDir); err != nil {
			return fmt.Errorf("watcher.Switch: %w", err)
		}
	}
	if s.board != nil {
		s.board.setClient(client)
	}

	s.mu.Lock()
	s.info = info
	s.mu.Unlock()

	if err := s.rememberBoard(info); err != nil {
		// Non-fatal — the board is open; failed to persist recent list.
		s.logger.Warn("settings: persist recent.json", "err", err)
	}

	// Emit only after state is committed so the frontend sees the new info.
	if app := application.Get(); app != nil {
		app.Event.Emit("board:opened", info)
		app.Event.Emit("board:reloaded")
	}

	_ = ctx // keep the signature ergonomic; future hooks may honour cancel
	return nil
}

// PickBoardDialog opens the native folder picker and returns the selected
// path. Returns ErrCancelled when the user dismisses the dialog.
//
// Does not call OpenBoard — the frontend gets the path and decides how to
// proceed (validate via OpenBoard, fall back, etc).
var ErrCancelled = errors.New("dialog cancelled")

func (s *SettingsService) PickBoardDialog() (string, error) {
	a := application.Get()
	if a == nil {
		return "", errors.New("application not running")
	}
	path, err := a.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(false).
		SetTitle("Open tb board").
		SetMessage("Pick a directory that contains .tb.yaml").
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", ErrCancelled
	}
	return path, nil
}

// ListRecentBoards returns the persisted recent-boards list, most recent
// first. Boards whose project root no longer exists are filtered out so the
// menu never offers a dead link.
func (s *SettingsService) ListRecentBoards() ([]RecentBoard, error) {
	all, err := s.loadRecents()
	if err != nil {
		return nil, err
	}
	out := make([]RecentBoard, 0, len(all))
	for _, r := range all {
		if _, err := os.Stat(filepath.Join(r.ProjectRoot, tbConfigFileName)); err == nil {
			out = append(out, r)
		}
	}
	return out, nil
}

// --- internals ---

func (s *SettingsService) rememberBoard(info BoardInfo) error {
	list, err := s.loadRecents()
	if err != nil {
		return err
	}
	// Dedup by project root.
	filtered := list[:0]
	for _, r := range list {
		if r.ProjectRoot != info.ProjectRoot {
			filtered = append(filtered, r)
		}
	}
	entry := RecentBoard{
		ProjectRoot: info.ProjectRoot,
		Prefix:      info.Prefix,
		LastOpened:  time.Now().UTC(),
	}
	filtered = append([]RecentBoard{entry}, filtered...)
	if len(filtered) > recentBoardCap {
		filtered = filtered[:recentBoardCap]
	}
	// Stable ordering by LastOpened (desc) just in case an external edit
	// introduced an out-of-order entry.
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].LastOpened.After(filtered[j].LastOpened)
	})
	return s.saveRecents(filtered)
}

func (s *SettingsService) loadRecents() ([]RecentBoard, error) {
	b, err := os.ReadFile(s.recentsPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []RecentBoard
	if err := json.Unmarshal(b, &out); err != nil {
		// Corrupt file — log and treat as empty, don't bomb the GUI.
		s.logger.Warn("settings: recent.json malformed; ignoring", "err", err)
		return nil, nil
	}
	return out, nil
}

func (s *SettingsService) saveRecents(list []RecentBoard) error {
	if err := os.MkdirAll(filepath.Dir(s.recentsPath), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	// Atomic-ish write — temp + rename. Same protective pattern the CLI uses
	// for task files, but local to settings so no shared helper is needed.
	tmp := s.recentsPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.recentsPath)
}

// readBoardInfo parses `<projectRoot>/.tb.yaml` and resolves the board dir.
// Returns ErrNoTbYaml if the config file is missing. Mirrors the relevant
// parts of cli/board.go:loadProjectConfig.
func readBoardInfo(projectRoot string) (BoardInfo, error) {
	configPath := filepath.Join(projectRoot, tbConfigFileName)
	data, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return BoardInfo{}, ErrNoTbYaml
	}
	if err != nil {
		return BoardInfo{}, fmt.Errorf("read %s: %w", configPath, err)
	}
	values := parseSimpleYAML(data)

	boardPath := values["board"]
	if boardPath == "" {
		boardPath = "board"
	}
	boardDir := boardPath
	if !filepath.IsAbs(boardDir) {
		boardDir = filepath.Join(projectRoot, boardDir)
	}

	prefix := strings.ToUpper(values["prefix"])
	if prefix == "" {
		prefix = "PR"
	}

	wip := 2
	if wl, ok := values["wip_limit"]; ok {
		if n, err := atoiNonNegative(wl); err == nil && n > 0 {
			wip = n
		}
	}

	return BoardInfo{
		ProjectRoot: projectRoot,
		BoardDir:    boardDir,
		Prefix:      prefix,
		WIPLimit:    wip,
	}, nil
}

// parseSimpleYAML mirrors cli/board.go's minimal "key: value" parser. Keeping
// a small private copy avoids a new shared module just for one helper.
func parseSimpleYAML(data []byte) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		result[key] = val
	}
	return result
}

func atoiNonNegative(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("non-digit in %q", s)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

// defaultRecentsPath returns $XDG_CONFIG_HOME/tb-gui/recent.json, falling back
// to ~/.config/tb-gui/recent.json on macOS/Linux and a per-user fallback in
// the OS-native config dir as a last resort.
func defaultRecentsPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, recentsDirName, recentsFileName)
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", recentsDirName, recentsFileName)
	}
	if cfg, err := os.UserConfigDir(); err == nil {
		return filepath.Join(cfg, recentsDirName, recentsFileName)
	}
	// Last resort — co-locate next to the binary. Better than panicking.
	return filepath.Join(os.TempDir(), recentsDirName, recentsFileName)
}
