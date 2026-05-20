package main

import (
	"context"
	"embed"
	"log"
	"log/slog"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	tbapp "tools/tb-gui/app"
	"tools/tb-gui/internal/daemon"
	"tools/tb-gui/internal/shell"
	"tools/tb-gui/internal/watcher"
)

//go:embed all:frontend/dist
var assets embed.FS

// singleInstanceKey is the per-user XOR key for the single-instance message bus.
// Any non-zero 32-byte value works; we use a constant so the message format is
// stable across builds without leaking anything sensitive.
var singleInstanceKey = [32]byte{
	0x74, 0x62, 0x2d, 0x67, 0x75, 0x69, 0x2d, 0x73, // "tb-gui-s"
	0x69, 0x6e, 0x67, 0x6c, 0x65, 0x2d, 0x69, 0x6e, // "ingle-in"
	0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x2d, 0x6b, // "stance-k"
	0x65, 0x79, 0x2d, 0x76, 0x31, 0x2e, 0x30, 0x2e, // "ey-v1.0."
}

func main() {
	var window *application.WebviewWindow

	logger := slog.Default()
	boardService := tbapp.NewBoardService()

	// emitterShim adapts *application.App's event bus to watcher.Emitter
	// and AgentService.Emitter. Constructed after the app exists so we can
	// capture it by reference.
	var appRef *application.App
	emitter := emitterShim{getApp: func() *application.App { return appRef }}
	var settingsService *tbapp.SettingsService

	agentService := tbapp.NewAgentService(tbapp.AgentServiceOptions{
		Board:   boardService,
		Emitter: emitter,
		TimeoutProvider: func() time.Duration {
			if settingsService == nil {
				return time.Duration(tbapp.AgentTimeoutMinutesDefault) * time.Minute
			}
			return time.Duration(settingsService.GetAgentTimeoutMinutes()) * time.Minute
		},
	})

	// Build the daemon BEFORE the watcher so we can tee watcher events
	// into both the Wails app bus and the daemon's sink. The settings
	// service (below) gets a BoardActivator hook that drives daemon
	// Activate/Deactivate on OpenBoard.
	settingsForPrefs := tbapp.NewSettingsService(tbapp.SettingsOptions{Logger: logger})
	maxWorkers := settingsForPrefs.GetMaxWorkers()
	recovery := tbapp.NewRecoveryService(boardService, agentService, daemon.PidAliveForRecovery, logger)
	stageReconciler := tbapp.NewStageReconciler(boardService, logger)
	d := daemon.New(daemon.Options{
		Board:                   &boardAdapter{b: boardService},
		Agent:                   &agentAdapter{s: agentService},
		Recovery:                recovery,
		Reconciler:              stageReconciler,
		Logger:                  logger,
		MaxWorkers:              maxWorkers,
		DisablePeriodicRecovery: !settingsForPrefs.GetPeriodicRecoveryEnabled(),
	})

	// Auto-groom coordinator (TB-174). Activates parallel to the daemon
	// via the composite activator below; its sink is tee'd into the
	// watcher emitter alongside the daemon's sink so board:reloaded /
	// task:updated:<id> events drive incremental scans.
	autoGroom := tbapp.NewAutoGroomCoordinator(tbapp.AutoGroomCoordinatorOptions{
		Board:    boardService,
		Agent:    agentService,
		Settings: nil, // wired below after settingsService is constructed
		Emitter:  emitter,
		Logger:   logger,
	})

	// Auto-implement coordinator (TB-179). Parallel to AutoGroomCoordinator
	// but scoped to the ready column and implement-mode runs. Watches the
	// same watcher events.
	autoImplement := tbapp.NewAutoImplementCoordinator(tbapp.AutoImplementCoordinatorOptions{
		Board:        boardService,
		Agent:        agentService,
		Settings:     nil, // wired below after settingsService is constructed
		Emitter:      emitter,
		Logger:       logger,
		WorkerBudget: d,
	})

	// Watcher emits to the Wails app, the daemon sink, the board sink, the
	// auto-groom coordinator sink, and the auto-implement coordinator sink
	// (TB-58 + TB-174 + TB-179). Right-associative fan-out keeps the
	// existing TeeEmitter contract unchanged.
	sink := daemon.NewEventSink(d, logger)
	boardSink := tbapp.NewBoardWatcherSink(boardService)
	tee := daemon.TeeEmitter{A: emitter, B: daemon.TeeEmitter{A: sink, B: daemon.TeeEmitter{A: boardSink, B: daemon.TeeEmitter{A: autoGroom, B: autoImplement}}}}
	w := watcher.New(teeShim{tee: tee}, logger)

	settingsService = tbapp.NewSettingsService(tbapp.SettingsOptions{
		Logger:    logger,
		Board:     boardService,
		Watcher:   w,
		Activator: &boardActivator{daemon: d, agent: agentService, autoGroom: autoGroom, autoImplement: autoImplement},
	})
	// Late-bind the SettingsService so both coordinators can read
	// preferences on every scan.
	autoGroom.SetSettings(settingsService)
	autoImplement.SetSettings(settingsService)

	// Per-agent quota usage — independent of any individual run, refreshed on
	// a timer and on demand from the header widget (TB-107).
	usageService := tbapp.NewUsageService(tbapp.UsageServiceOptions{
		Emitter:     emitter,
		Logger:      logger,
		ProjectRoot: settingsService.GetProjectRoot,
	})

	// Refresh the per-agent quota chip the moment a board opens. Without
	// this, the seed runs before OpenBoard has set ProjectRoot, so claude
	// falls into the "no project open" stub branch and the header chip
	// shows "unknown" until the 5-minute ticker fires or the user clicks ↻.
	settingsService.SetBoardOpenedHook(func(ctx context.Context) {
		usageService.RefreshAgentUsage(ctx)
	})

	app := application.New(application.Options{
		Name:        "Task Board Tools",
		Description: "Task Board Tools GUI — kanban over markdown tasks",
		LogLevel:    slog.LevelInfo,
		Services: []application.Service{
			application.NewService(boardService),
			application.NewService(settingsService),
			application.NewService(agentService),
			application.NewService(usageService),
			application.NewService(autoGroom),
			application.NewService(autoImplement),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:      "com.taskboard.tbgui",
			EncryptionKey: singleInstanceKey,
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if window == nil {
					return
				}
				window.Restore()
				window.Focus()
				slog.Info("second instance launched", "args", data.Args, "cwd", data.WorkingDir)
			},
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: !shell.TraySupported(),
		},
	})
	appRef = app

	// Auto-groom coordinator listens for terminal groom runs so it can
	// re-check triage and promote successfully groomed tasks to `ready`
	// via `tb ready` (TB-174). The payload shape mirrors what
	// AgentService.recordTerminal emits.
	app.Event.On("agent:run-finished", func(ev *application.CustomEvent) {
		if payload, ok := ev.Data.(map[string]any); ok {
			autoGroom.OnAgentRunFinished(payload)
			stageReconciler.OnAgentRunFinished(payload)
		}
	})

	shellController, err := shell.NewController(shell.Options{
		App:      app,
		Board:    boardService,
		Settings: settingsService,
		Logger:   logger,
	})
	if err != nil {
		log.Fatal(err)
	}
	app.Menu.Set(shellController.ApplicationMenu())

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Task Board Tools",
		Width: 1280, Height: 800,
		MinWidth: 720, MinHeight: 480,
		EnableFileDrop: true,
		Mac: application.MacWindow{
			// 0 (not the visible 50px topbar height) so Wails' native
			// performWindowDragWithEvent: handler doesn't capture mouse-downs in
			// the content-area drag region. The native title-bar/toolbar strip
			// still handles drag in its own area (height depends on toolbar
			// style / macOS version); below that the frontend's
			// --wails-draggable: drag runtime drives the move. This restores
			// standard macOS titlebar double-click zoom semantics (TB-236) —
			// paired with onTopbarDblClick in routes/+page.svelte.
			InvisibleTitleBarHeight: 0,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour:   application.NewRGB(27, 38, 54),
		URL:                "/",
		UseApplicationMenu: true,
	})

	// Route file drops onto elements with data-file-drop-target into the
	// shared `tb attach` path. The webview's runtime tags drop targets via
	// the data-file-drop-target attribute; here we read data-task-id off
	// that element and forward to BoardService. The GUI never writes
	// attachment files itself — everything goes through `tb`.
	window.OnWindowEvent(events.Common.WindowFilesDropped, func(ev *application.WindowEvent) {
		details := ev.Context().DropTargetDetails()
		files := ev.Context().DroppedFiles()
		if details == nil || len(files) == 0 {
			return
		}
		taskID := details.Attributes["data-task-id"]
		if taskID == "" {
			app.Event.Emit("attach:dropped", map[string]any{
				"ok":    false,
				"error": "drop target has no task id",
			})
			return
		}
		// Signal in-flight so the drawer can disable Add / Remove and show a
		// hint while `tb attach` runs. Paired with attach:dropped (success or
		// failure) that re-enables the controls. Concurrent tb mutations are
		// serialised by .board.lock but the GUI gives no other feedback during
		// the drop.
		app.Event.Emit("attach:dropping", map[string]any{
			"taskId": taskID,
			"count":  len(files),
		})
		err := boardService.AddAttachments(context.Background(), taskID, files)
		payload := map[string]any{
			"taskId": taskID,
			"count":  len(files),
		}
		if err != nil {
			payload["ok"] = false
			payload["error"] = err.Error()
		} else {
			payload["ok"] = true
		}
		app.Event.Emit("attach:dropped", payload)
	})
	shellController.AttachWindow(window)
	shellController.InstallTray()

	// Run the watcher loop now; SettingsService.OpenBoard will call Switch
	// to point it at a board. Events received before Switch are dropped
	// (no active subscription), but the goroutine itself is ready.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = w.Run(ctx) }()

	// Start the daemon's watcher-event reader. The daemon itself stays
	// inert until SettingsService.OpenBoard calls Activate via the
	// BoardActivator hook.
	sink.Start(ctx)
	defer sink.Close()
	defer func() {
		if err := d.Close(); err != nil {
			slog.Warn("daemon: shutdown error", "err", err)
		}
	}()

	// Background periodic refresh of per-agent quota usage. Stops when ctx
	// is cancelled at shutdown.
	usageService.Start(ctx)
	defer usageService.Close()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// emitterShim implements watcher.Emitter by deferring to the live app's event
// bus. Using a getter avoids a chicken-and-egg between New(options{...}) and
// the watcher.New call that needs the app reference.
type emitterShim struct {
	getApp func() *application.App
}

func (e emitterShim) Emit(name string, data ...any) {
	if a := e.getApp(); a != nil {
		a.Event.Emit(name, data...)
	}
}
