package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"

	tbapp "tools/tb-gui/app"
)

const (
	settingsOpenPanelEvent = "settings:open-panel"
	maxRecentMenuEntries   = 10
	appVersion             = "dev"
)

// TraySupported returns whether the current Wails3 alpha ships a desktop tray
// implementation for this platform. Non-desktop builds intentionally no-op.
func TraySupported() bool {
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		return true
	default:
		return false
	}
}

type Controller struct {
	app      *application.App
	board    *tbapp.BoardService
	settings *tbapp.SettingsService
	logger   *slog.Logger

	mu         sync.Mutex
	window     application.Window
	menu       *application.Menu
	recentMenu *application.Menu
	tray       *application.SystemTray

	idleIcon    []byte
	runningIcon []byte
	activeRuns  atomic.Int32
}

type Options struct {
	App      *application.App
	Board    *tbapp.BoardService
	Settings *tbapp.SettingsService
	Logger   *slog.Logger
}

func NewController(opts Options) (*Controller, error) {
	if opts.App == nil {
		return nil, errors.New("shell controller requires an application")
	}
	if opts.Board == nil {
		return nil, errors.New("shell controller requires a board service")
	}
	if opts.Settings == nil {
		return nil, errors.New("shell controller requires a settings service")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	idleIcon, err := trayIconPNG(false)
	if err != nil {
		return nil, fmt.Errorf("build idle tray icon: %w", err)
	}
	runningIcon, err := trayIconPNG(true)
	if err != nil {
		return nil, fmt.Errorf("build running tray icon: %w", err)
	}

	c := &Controller{
		app:         opts.App,
		board:       opts.Board,
		settings:    opts.Settings,
		logger:      logger.With("component", "shell"),
		idleIcon:    idleIcon,
		runningIcon: runningIcon,
	}
	c.installEventHooks()
	return c, nil
}

func (c *Controller) ApplicationMenu() *application.Menu {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.menu != nil {
		return c.menu
	}

	menu := c.app.NewMenu()
	if runtime.GOOS == "darwin" {
		menu.AddRole(application.AppMenu)
	}

	file := menu.AddSubmenu("File")
	file.Add("Open board...").
		SetAccelerator("CmdOrCtrl+o").
		OnClick(func(*application.Context) { c.openBoardDialog() })
	c.recentMenu = file.AddSubmenu("Open Recent")
	c.rebuildRecentMenuLocked()
	file.AddSeparator()
	file.Add("Settings...").
		SetAccelerator("CmdOrCtrl+,").
		OnClick(func(*application.Context) { c.openSettings() })
	file.AddSeparator()
	file.AddRole(application.Quit)

	menu.AddRole(application.EditMenu)

	view := menu.AddSubmenu("View")
	view.Add("Reload board").
		SetAccelerator("CmdOrCtrl+r").
		OnClick(func(*application.Context) { c.reloadBoard() })

	menu.AddRole(application.WindowMenu)

	help := menu.AddSubmenu("Help")
	help.Add("About tb-gui").
		OnClick(func(*application.Context) { c.showAbout() })
	help.Add("Open docs").
		OnClick(func(*application.Context) { c.openDocs() })

	c.menu = menu
	return c.menu
}

func (c *Controller) AttachWindow(window application.Window) {
	c.mu.Lock()
	c.window = window
	tray := c.tray
	c.mu.Unlock()

	if tray != nil && window != nil {
		tray.AttachWindow(window)
	}
}

func (c *Controller) InstallTray() bool {
	if !TraySupported() {
		c.logger.Info("system tray unavailable on this platform", "goos", runtime.GOOS)
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tray != nil {
		return true
	}

	trayMenu := c.app.NewMenu()
	trayMenu.Add("Show window").OnClick(func(*application.Context) { c.showWindow() })
	trayMenu.Add("Settings...").OnClick(func(*application.Context) { c.openSettings() })
	trayMenu.AddSeparator()
	trayMenu.Add("Quit").OnClick(func(*application.Context) { c.app.Quit() })

	tray := c.app.SystemTray.New()
	tray.SetTemplateIcon(c.idleIcon)
	tray.SetMenu(trayMenu)
	tray.OnClick(func() { c.toggleWindow() })
	tray.OnRightClick(func() { tray.ShowMenu() })
	if c.window != nil {
		tray.AttachWindow(c.window)
	}

	c.tray = tray
	return true
}

func (c *Controller) installEventHooks() {
	c.app.Event.On("board:opened", func(*application.CustomEvent) {
		c.refreshRecentMenu()
	})
	c.app.Event.On("recents:changed", func(*application.CustomEvent) {
		c.refreshRecentMenu()
	})
	c.app.Event.On("agent:run-started", func(*application.CustomEvent) {
		c.activeRuns.Add(1)
		c.updateTrayState()
	})
	c.app.Event.On("agent:run-finished", func(*application.CustomEvent) {
		c.decrementActiveRuns()
		c.updateTrayState()
	})
}

func (c *Controller) openBoardDialog() {
	path, err := c.settings.PickBoardDialog()
	if errors.Is(err, tbapp.ErrCancelled) {
		return
	}
	if err != nil {
		c.showError("Open board", err)
		return
	}
	c.openBoard(path)
}

func (c *Controller) openBoard(projectRoot string) {
	if err := c.settings.OpenBoard(c.context(), projectRoot); err != nil {
		c.showError("Open board", err)
		return
	}
	c.refreshRecentMenu()
}

func (c *Controller) reloadBoard() {
	if err := c.board.Regenerate(c.context()); err != nil {
		c.showError("Reload board", err)
		return
	}
	c.app.Event.Emit("board:reloaded")
}

func (c *Controller) openSettings() {
	c.showWindow()
	c.app.Event.Emit(settingsOpenPanelEvent)
}

func (c *Controller) showAbout() {
	project := "task-board-tools"
	if path, ok := findProjectFile("README.md"); ok {
		project = fileURL(path)
	}
	c.showInfo("About tb-gui", fmt.Sprintf(
		"tb-gui\nVersion: %s\nProject: %s\n\nKanban over markdown task boards.",
		appVersion,
		project,
	))
}

func (c *Controller) openDocs() {
	for _, name := range []string{"docs/PROJECT.md", "README.md"} {
		if path, ok := findProjectFile(name); ok {
			if err := c.app.Browser.OpenFile(path); err != nil {
				c.showError("Open docs", err)
			}
			return
		}
	}
	c.showInfo("Open docs", "Could not find docs/PROJECT.md or README.md near this build.")
}

func (c *Controller) refreshRecentMenu() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rebuildRecentMenuLocked()
	c.refreshApplicationMenuLocked()
}

func (c *Controller) rebuildRecentMenuLocked() {
	if c.recentMenu == nil {
		return
	}

	c.recentMenu.Clear()
	recents, err := c.settings.ListRecentBoards()
	if err != nil {
		c.logger.Warn("load recent boards for native menu", "err", err)
		c.recentMenu.Add("Could not load recent boards").SetEnabled(false)
		return
	}
	if len(recents) == 0 {
		c.recentMenu.Add("No Recent Boards").SetEnabled(false)
		return
	}

	if len(recents) > maxRecentMenuEntries {
		recents = recents[:maxRecentMenuEntries]
	}
	for _, recent := range recents {
		recent := recent
		c.recentMenu.Add(recentBoardLabel(recent.ProjectRoot)).
			OnClick(func(*application.Context) { c.openBoard(recent.ProjectRoot) })
	}
}

func (c *Controller) refreshApplicationMenuLocked() {
	if c.menu == nil {
		return
	}
	c.menu.Update()
	c.app.Menu.Set(c.menu)
	if c.window != nil && runtime.GOOS != "darwin" {
		c.window.SetMenu(c.menu)
	}
}

func (c *Controller) updateTrayState() {
	c.mu.Lock()
	tray := c.tray
	running := c.activeRuns.Load() > 0
	c.mu.Unlock()

	if tray == nil {
		return
	}
	if running {
		tray.SetTemplateIcon(c.runningIcon)
		return
	}
	tray.SetTemplateIcon(c.idleIcon)
}

func (c *Controller) decrementActiveRuns() {
	for {
		current := c.activeRuns.Load()
		if current <= 0 {
			// Recovery may emit finished events for stale runs that never
			// started in this process, so keep the tray counter floored.
			return
		}
		if c.activeRuns.CompareAndSwap(current, current-1) {
			return
		}
	}
}

func (c *Controller) toggleWindow() {
	c.mu.Lock()
	window := c.window
	c.mu.Unlock()
	if window == nil {
		return
	}
	if window.IsVisible() {
		window.Hide()
		return
	}
	window.Show().Focus()
}

func (c *Controller) showWindow() {
	c.mu.Lock()
	window := c.window
	c.mu.Unlock()
	if window == nil {
		return
	}
	window.Show().Focus()
}

func (c *Controller) context() context.Context {
	if c.app != nil {
		return c.app.Context()
	}
	return context.Background()
}

func (c *Controller) showError(title string, err error) {
	if err == nil {
		return
	}
	c.logger.Warn(title, "err", err)
	dialog := c.app.Dialog.Error().
		SetTitle(title).
		SetMessage(err.Error())
	c.attachDialog(dialog)
	dialog.Show()
}

func (c *Controller) showInfo(title, message string) {
	dialog := c.app.Dialog.Info().
		SetTitle(title).
		SetMessage(message)
	c.attachDialog(dialog)
	dialog.Show()
}

func (c *Controller) attachDialog(dialog *application.MessageDialog) {
	c.mu.Lock()
	window := c.window
	c.mu.Unlock()
	if window != nil {
		dialog.AttachToWindow(window)
	}
}

func recentBoardLabel(projectRoot string) string {
	name := filepath.Base(projectRoot)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return projectRoot
	}
	return fmt.Sprintf("%s (%s)", name, projectRoot)
}

func findProjectFile(name string) (string, bool) {
	var bases []string
	if wd, err := os.Getwd(); err == nil {
		bases = append(bases, wd, filepath.Dir(wd))
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		bases = append(bases, dir, filepath.Dir(dir), filepath.Join(dir, "..", "Resources"))
	}

	seen := make(map[string]struct{}, len(bases))
	for _, base := range bases {
		if base == "" {
			continue
		}
		abs, err := filepath.Abs(filepath.Join(base, name))
		if err != nil {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		if stat, err := os.Stat(abs); err == nil && !stat.IsDir() {
			return abs, true
		}
	}
	return "", false
}

func fileURL(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String()
}

func trayIconPNG(running bool) ([]byte, error) {
	const size = 44
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	ink := color.RGBA{A: 255}

	drawCircle(img, 22, 22, 15, ink)
	for y := 15; y <= 29; y++ {
		for x := 15; x <= 29; x++ {
			if x >= 19 && x <= 25 && y >= 19 && y <= 25 {
				continue
			}
			if x >= 20 && x <= 24 && y >= 15 && y <= 29 {
				img.SetRGBA(x, y, color.RGBA{})
			}
			if y >= 20 && y <= 24 && x >= 15 && x <= 29 {
				img.SetRGBA(x, y, color.RGBA{})
			}
		}
	}

	if running {
		drawCircle(img, 32, 12, 5, ink)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawCircle(img *image.RGBA, cx, cy, r int, col color.RGBA) {
	rr := r * r
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy <= rr {
				img.SetRGBA(x, y, col)
			}
		}
	}
}
