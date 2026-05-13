package main

import (
	"context"
	"embed"
	"log"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"

	tbapp "tools/tb-gui/app"
	"tools/tb-gui/internal/daemon"
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

	agentService := tbapp.NewAgentService(tbapp.AgentServiceOptions{
		Board:   boardService,
		Emitter: emitter,
	})

	// Build the daemon BEFORE the watcher so we can tee watcher events
	// into both the Wails app bus and the daemon's sink. The settings
	// service (below) gets a BoardActivator hook that drives daemon
	// Activate/Deactivate on OpenBoard.
	settingsForPrefs := tbapp.NewSettingsService(tbapp.SettingsOptions{Logger: logger})
	maxWorkers := settingsForPrefs.GetMaxWorkers()
	recovery := tbapp.NewRecoveryService(boardService, agentService, daemon.PidAliveForRecovery, logger)
	d := daemon.New(daemon.Options{
		Board:      &boardAdapter{b: boardService},
		Agent:      &agentAdapter{s: agentService},
		Recovery:   recovery,
		Logger:     logger,
		MaxWorkers: maxWorkers,
	})

	// Watcher emits to both the Wails app and the daemon sink (TB-58).
	sink := daemon.NewEventSink(d, logger)
	tee := daemon.TeeEmitter{A: emitter, B: sink}
	w := watcher.New(teeShim{tee: tee}, logger)

	settingsService := tbapp.NewSettingsService(tbapp.SettingsOptions{
		Logger:    logger,
		Board:     boardService,
		Watcher:   w,
		Activator: d,
	})

	app := application.New(application.Options{
		Name:        "tb-gui",
		Description: "Task Board Tools GUI — kanban over markdown tasks",
		LogLevel:    slog.LevelInfo,
		Services: []application.Service{
			application.NewService(boardService),
			application.NewService(settingsService),
			application.NewService(agentService),
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
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	appRef = app

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "tb-gui",
		Width: 1280, Height: 800,
		MinWidth: 720, MinHeight: 480,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
	})

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
