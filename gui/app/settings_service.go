package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

// boardOpenedHookTimeout caps the detached goroutine that fires the
// post-OpenBoard hook (production: UsageService.RefreshAgentUsage). Slightly
// larger than UsageService's own 8s refresh budget so the hook side never
// races the inner cancel.
const boardOpenedHookTimeout = 10 * time.Second

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
// The frontend turns this into the InitBoard prompt so the user can choose
// to initialize a board in that folder. Until they confirm, the previously
// active board is left in place.
var ErrNoTbYaml = errors.New("path has no .tb.yaml — not a tb project")

// ErrInvalidPrefix is returned when InitBoard rejects a task-ID prefix that
// doesn't match the GUI's conservative whitelist (letter-led, 1–10
// alphanumeric). The CLI itself is more permissive, but the prefix becomes
// part of filenames so the GUI keeps the surface tight.
var ErrInvalidPrefix = errors.New("prefix must start with a letter and contain only letters or digits (max 10)")

// ErrInvalidBoardPath is returned when InitBoard rejects a board path that
// is empty, absolute, or tries to escape the project root via "..".
var ErrInvalidBoardPath = errors.New("board path must be a non-empty relative path inside the project root")

// ErrBoardAlreadyInitialized is returned by InitBoard when the project root
// already contains a `.tb.yaml`. The frontend treats this as "open it
// instead" rather than overwriting an existing config with new values.
var ErrBoardAlreadyInitialized = errors.New(".tb.yaml already exists in this folder")

// InitBoardPathDefault and InitPrefixDefault mirror `tb init`'s defaults so
// the InitBoard dialog and the CLI agree on the empty-value behavior.
const (
	InitBoardPathDefault = "board"
	InitPrefixDefault    = "PR"
	initPrefixMaxLen     = 10
)

// initPrefixRe is the conservative whitelist for task-ID prefixes. The
// resulting prefix appears in `<prefix>-<N>` filenames, so the rule mirrors
// what a portable filename safely allows.
var initPrefixRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// Switcher is the contract SettingsService needs from the watcher. Passed in
// at construction so the service stays test-friendly.
type Switcher interface {
	Switch(boardDir string) error
}

// BoardActivator is the post-OpenBoard hook the daemon registers so it
// can run stale-recovery + startup-scan against the freshly attached
// board. SettingsService calls Deactivate first (to drain any previous
// board) and then Activate. Production wiring is *daemon.Daemon.
type BoardActivator interface {
	Activate(ctx context.Context, boardDir string) error
	Deactivate() error
}

// PeriodicRecoveryController is the optional runtime settings hook exposed by
// the daemon. SettingsService persists the preference, then calls this hook
// when present so the running process reflects the new toggle immediately.
type PeriodicRecoveryController interface {
	SetPeriodicRecoveryEnabled(enabled bool)
}

// AutoGroomController is the optional runtime settings hook exposed by
// the auto-groom coordinator (TB-174). SettingsService persists the
// preference, then calls the matching Notify method so a freshly
// flipped toggle (or a freshly chosen default agent) triggers an
// immediate coordinator scan instead of waiting for the next watcher
// event. Wired via the activator at construction time.
type AutoGroomController interface {
	NotifyAutoGroomEnabled()
	NotifyDefaultAgentChanged()
}

// BoardOpenedHook fires once OpenBoard has committed the new board state
// (BoardService rebound, watcher switched, daemon activated, board:opened
// emitted). Production wires it to UsageService.RefreshAgentUsage so the
// per-agent quota chip picks up the new project's claude usage tap instead
// of the seed-time "no project open" snapshot. Runs in a detached goroutine
// — must not block, must tolerate being called with a fresh ctx.
type BoardOpenedHook func(ctx context.Context)

// SettingsService manages project root selection, recent-board persistence,
// and the native folder picker. It also coordinates BoardService and the
// watcher whenever the active board changes.
type SettingsService struct {
	logger    *slog.Logger
	board     *BoardService
	wch       Switcher
	activator BoardActivator

	// boardOpenedHook is fired in a goroutine after every successful
	// OpenBoard. Set late via SetBoardOpenedHook because UsageService
	// (the production wirer) is constructed after SettingsService.
	boardOpenedHook BoardOpenedHook

	// openMu serializes OpenBoard calls so two concurrent invocations
	// cannot interleave: a slow candidate validation from an older call
	// must not resume and commit side effects (watcher swap, daemon
	// activation, recents update, board:opened/board:reloaded emit) on
	// top of a newer board that has already opened (TB-208 review).
	openMu sync.Mutex

	mu      sync.RWMutex
	info    BoardInfo
	cliPath string // test-only fallback for cli.NewClient; empty = PATH

	// recentsPath is the absolute path to recent.json. Configurable for tests.
	recentsPath string
	// prefsPath is the absolute path to preferences.json. Configurable for
	// tests; empty falls back to defaultPreferencesPath.
	prefsPath string
}

// SettingsOptions tunes SettingsService construction.
type SettingsOptions struct {
	Logger      *slog.Logger
	Board       *BoardService
	Watcher     Switcher
	Activator   BoardActivator
	CLIPath     string // override for tests; empty = PATH lookup
	RecentsPath string // override for tests; empty = $XDG_CONFIG_HOME/tb-gui/recent.json
	PrefsPath   string // override for tests; empty = $XDG_CONFIG_HOME/tb-gui/preferences.json
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
		activator:   opts.Activator,
		recentsPath: recents,
		prefsPath:   opts.PrefsPath,
		cliPath:     opts.CLIPath,
	}
}

// ServiceName satisfies the Wails service contract.
func (s *SettingsService) ServiceName() string { return "SettingsService" }

// SetBoardOpenedHook wires a callback fired after every successful OpenBoard.
// Late binding because UsageService (the production hook target) is built
// after SettingsService. Passing nil clears the hook. Safe to call any time.
func (s *SettingsService) SetBoardOpenedHook(hook BoardOpenedHook) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.boardOpenedHook = hook
}

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
	s.openMu.Lock()
	defer s.openMu.Unlock()
	if projectRoot == "" {
		return errors.New("empty project root")
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("resolve abs path: %w", err)
	}

	s.mu.RLock()
	if s.info.ProjectRoot == absRoot {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	info, err := readBoardInfo(absRoot)
	if err != nil {
		return err
	}

	// Build a CLI client rooted at the project so `tb` discovers `.tb.yaml`.
	client, err := cli.NewClient(cli.Options{
		BinaryPath: s.openBoardCLIPath(),
		Cwd:        info.ProjectRoot,
		Logger:     s.logger,
	})
	if err != nil {
		return fmt.Errorf("locate tb binary: %w", err)
	}

	// Validate the candidate board's active scope BEFORE committing the
	// switch. A duplicate-task ID (or any other `tb ls` failure) must
	// abort here, before we touch the watcher, the BoardService client,
	// the daemon, the recent-board list, or emit board:opened/board:reloaded.
	// Otherwise the frontend sees a brief "switched" state followed by a
	// load failure and the daemon scans an inconsistent board (TB-208).
	if err := validateCandidateBoardActive(ctx, client); err != nil {
		return err
	}

	if s.wch != nil {
		if err := s.wch.Switch(info.BoardDir); err != nil {
			return fmt.Errorf("watcher.Switch: %w", err)
		}
	}

	// Drain the previous board before rebinding BoardService. watcher.Switch
	// is the last fallible pre-commit step; keeping BoardService on the old
	// client until after this drain prevents watcher-triggered daemon work from
	// targeting the candidate board during switch teardown.
	if s.activator != nil {
		if err := s.activator.Deactivate(); err != nil {
			s.logger.Warn("daemon: deactivate before reactivate", "err", err)
		}
	}

	if s.board != nil {
		s.board.setClient(client)
		// EditTaskBody needs the absolute board dir for flock + atomic write.
		// SetBoardDir is a no-op if the BoardService doesn't expose direct
		// writes, but for M3 it's required.
		s.board.setBoardDir(info.BoardDir)
	}

	// Daemon activation runs AFTER BoardService + watcher are pointed at
	// the new board so the recovery + scan it triggers reads consistent
	// state.
	if s.activator != nil {
		if err := s.activator.Activate(ctx, info.BoardDir); err != nil {
			s.logger.Warn("daemon: activation failed; board still open", "err", err)
		}
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
		app.Event.Emit("recents:changed")
		app.Event.Emit("board:reloaded")
	}

	// Fire the board-opened hook (production: UsageService.RefreshAgentUsage)
	// in a detached goroutine. Detached so the Wails RPC ctx returning to the
	// frontend doesn't cancel the refresh; bounded with its own short timeout
	// so a stuck filesystem can't leak goroutines on repeated OpenBoard calls.
	s.mu.RLock()
	hook := s.boardOpenedHook
	s.mu.RUnlock()
	if hook != nil {
		go func() {
			hookCtx, cancel := context.WithTimeout(context.Background(), boardOpenedHookTimeout)
			defer cancel()
			hook(hookCtx)
		}()
	}

	_ = ctx // keep the signature ergonomic; future hooks may honour cancel
	return nil
}

// InitBoard creates a fresh tb board under projectRoot — running the same
// on-disk steps as `tb init <projectRoot> --board-path=<boardPath> --prefix=<prefix>`
// — and then commits the switch through OpenBoard so watcher, BoardService,
// daemon, recents, and `board:opened`/`board:reloaded` behave identically to
// opening an existing board.
//
// Validation runs before any file is touched, so a bad prefix or board path
// never leaves a half-written `.tb.yaml`. If the project root already
// contains `.tb.yaml`, InitBoard returns ErrBoardAlreadyInitialized without
// invoking the CLI; the frontend treats that as "just open it" via the
// regular OpenBoard call.
//
// If `tb init` succeeds but the subsequent OpenBoard validation fails (e.g.
// TB-208 candidate-board checks reject the new layout), the on-disk
// artifacts remain — the folder is a valid board — but the previously
// active SettingsService state is left untouched.
func (s *SettingsService) InitBoard(ctx context.Context, projectRoot, boardPath, prefix string) error {
	absRoot, err := normalizeInitProjectRoot(projectRoot)
	if err != nil {
		return err
	}

	bp, err := normalizeInitBoardPath(boardPath)
	if err != nil {
		return err
	}

	pfx, err := normalizeInitPrefix(prefix)
	if err != nil {
		return err
	}

	configPath := filepath.Join(absRoot, tbConfigFileName)
	switch _, statErr := os.Stat(configPath); {
	case statErr == nil:
		return ErrBoardAlreadyInitialized
	case errors.Is(statErr, os.ErrNotExist):
		// expected — we're creating it
	default:
		return fmt.Errorf("stat %s: %w", configPath, statErr)
	}

	client, err := cli.NewClient(cli.Options{
		BinaryPath: s.openBoardCLIPath(),
		Cwd:        absRoot,
		Logger:     s.logger,
	})
	if err != nil {
		return fmt.Errorf("locate tb binary: %w", err)
	}
	if _, err := client.Run(ctx, "init", absRoot, "--board-path="+bp, "--prefix="+pfx); err != nil {
		return fmt.Errorf("tb init: %w", err)
	}

	return s.OpenBoard(ctx, absRoot)
}

// normalizeInitProjectRoot resolves projectRoot to an absolute path and
// verifies it's an existing directory. The CLI would also catch this, but
// failing here yields a friendlier error than a process exit code.
func normalizeInitProjectRoot(projectRoot string) (string, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return "", errors.New("empty project root")
	}
	abs, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve abs path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("project root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project root is not a directory: %s", abs)
	}
	return abs, nil
}

// normalizeInitBoardPath trims, defaults, and validates the requested board
// path. Empty falls back to InitBoardPathDefault to match `tb init`.
func normalizeInitBoardPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return InitBoardPathDefault, nil
	}
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\`) {
		return "", ErrInvalidBoardPath
	}
	clean := filepath.Clean(p)
	if clean == "." || clean == ".." {
		return "", ErrInvalidBoardPath
	}
	if slices.Contains(strings.Split(filepath.ToSlash(clean), "/"), "..") {
		return "", ErrInvalidBoardPath
	}
	return p, nil
}

// normalizeInitPrefix trims, defaults, and validates the prefix against the
// GUI whitelist. Empty falls back to InitPrefixDefault.
func normalizeInitPrefix(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return InitPrefixDefault, nil
	}
	if len(p) > initPrefixMaxLen {
		return "", ErrInvalidPrefix
	}
	if !initPrefixRe.MatchString(p) {
		return "", ErrInvalidPrefix
	}
	return p, nil
}

// SetCLIPath validates and persists the tb binary override. If a board is
// already open, rebuild the BoardService client immediately so the next board
// read uses the new binary without reopening the project or restarting the
// watcher.
func (s *SettingsService) SetCLIPath(path string) error {
	s.mu.RLock()
	info := s.info
	board := s.board
	s.mu.RUnlock()

	client, err := cli.NewClient(cli.Options{
		BinaryPath: s.cliPathForPreference(path),
		Cwd:        info.ProjectRoot,
		Logger:     s.logger,
	})
	if err != nil {
		return fmt.Errorf("locate tb binary: %w", err)
	}

	if err := s.updatePreferences(func(prefs *Preferences) {
		prefs.CLIPath = path
	}); err != nil {
		return err
	}

	if info.ProjectRoot == "" || board == nil {
		return nil
	}
	board.setClient(client)
	return nil
}

// GetClaudeUsageTap returns the on-disk state of the claude usage tap for the
// active board. Safe to call when no board is open — returns Enabled=false
// with a reason.
func (s *SettingsService) GetClaudeUsageTap() ClaudeUsageTapStatus {
	return GetClaudeUsageTapStatus(s.GetProjectRoot())
}

// EnableClaudeUsageTap installs the tap script + settings.local.json patch in
// the active board's .claude directory.
func (s *SettingsService) EnableClaudeUsageTap() (ClaudeUsageTapStatus, error) {
	root := s.GetProjectRoot()
	if root == "" {
		return ClaudeUsageTapStatus{}, ErrNoBoard
	}
	return EnableClaudeUsageTap(root)
}

// DisableClaudeUsageTap removes the tap from the active board.
func (s *SettingsService) DisableClaudeUsageTap() (ClaudeUsageTapStatus, error) {
	root := s.GetProjectRoot()
	if root == "" {
		return ClaudeUsageTapStatus{}, ErrNoBoard
	}
	return DisableClaudeUsageTap(root)
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

func (s *SettingsService) openBoardCLIPath() string {
	if path := s.GetCLIPath(); path != "" {
		return path
	}
	// Preserve SettingsOptions.CLIPath as a test harness override while
	// production follows the persisted preference or PATH lookup.
	return s.cliPath
}

func (s *SettingsService) cliPathForPreference(path string) string {
	if path != "" {
		return path
	}
	return s.cliPath
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

// validateCandidateBoardActive runs `tb ls --json --status active` against
// the candidate board's CLI client and returns an actionable error if the
// board fails the same invariants BoardService.LoadBoardWithMode enforces
// (duplicate task IDs across status directories, etc). Duplicate-task
// failures are reshaped via boardLoadError so the caller sees the same
// "cannot load active board: task X appears in multiple status directories"
// message as the runtime refresh path — never a raw "Binding call failed"
// pass-through.
func validateCandidateBoardActive(ctx context.Context, c *cli.Client) error {
	var tasks []Task
	if err := c.RunJSON(ctx, &tasks, "ls", "--json", "--status", "active"); err != nil {
		return boardLoadError(err, "active")
	}
	return nil
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
