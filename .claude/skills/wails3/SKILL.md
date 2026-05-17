---
name: "wails3"
description: "Wails v3 documentation and Writer Studio conventions"
---

# Wails 3

This skill covers the Wails v3 contract Writer Studio relies on. Detailed reference docs live at
`~/projects/books/wails/docs/src/content/docs/`. Source code: `~/projects/books/wails/v3/`.

## What Wails 3 actually contributes

Wails is a Go ↔ JS bridge. It does three things:

1. **Auto-binds Go services to JS.** Public methods of registered structs are callable from the frontend through
   generated TypeScript bindings.
2. **Provides an event bus.** Backend emits events; frontend listens via the runtime; specific windows can be targeted
   or all broadcast.
3. **Owns the application lifecycle.** Startup → service init → event loop → shutdown, with hookable lifecycle
   callbacks.

Everything else (windows, menus, dialogs, drag-drop, file associations) is a feature on top of these three primitives.

## Service contract — the whole thing

A Wails service is a Go struct registered with `application.NewService(...)`. Public methods on the struct become async
callable functions in JS.

```go
type BookController struct {
domain *book.Service
log    *logger.Logger
}

// REQUIRED — labels the service for logs/debug.
func (s *BookController) ServiceName() string {
return "BookController"
}

// OPTIONAL — runs once at app startup, in registration order.
// Return error to abort app startup.
func (s *BookController) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error {
return nil
}

// OPTIONAL — runs once at app shutdown, in REVERSE registration order.
// Return error to log a warning (does not block shutdown).
func (s *BookController) ServiceShutdown() error {
return nil
}
```

Registration order in `WailsServices()` decides startup order. After changes to method signatures or adding new public
methods: `wails3 generate bindings -clean -ts`.

**Constraints:**

- Only **exported** (PascalCase) methods are bound.
- Services are **singletons** — one instance per service type.
- Services are **shared across all windows** — protect mutable state with `sync.RWMutex`.
- `ServiceStartup` must finish quickly (<2 s). Long-running init goes into a goroutine that listens to `ctx.Done()`.
- `ServiceShutdown` must finish quickly (<1 s). The OS may force-kill a slow shutdown.

## Event bus — emit and listen

**Go side:**

```go
// Broadcast to all windows.
app.Event.Emit("book:opened", payload)

// Target a specific window.
window.EmitEvent("notification", "Hello from Go!")

// Listen to a custom event in Go.
cleanup := app.Event.On("frontend-action", func(e *application.CustomEvent) { /* ... */ })
defer cleanup()
```

**JS side:**

```javascript
import { Events } from '@wailsio/runtime'

const off = Events.On('book:opened', (data) => { /* ... */ })
Events.Once('initialization-complete', (data) => { /* ... */ })
off() // unsubscribe
```

**Critical timing:** events emitted before `events.Common.WindowRuntimeReady` are **lost** — the JS runtime hasn't
loaded its handlers yet. If a service emits during `ServiceStartup`, the frontend will not see it. Either gate the emit
on `WindowRuntimeReady`, or design the flow so the frontend pulls initial state via a method call instead.

In Writer Studio, prefer the domain `EventEmitter` interface (`internal/events/`) over `app.Event.Emit` directly. Only
`menu` and `contextmenu` controllers (`api/menu.go`, `api/contextmenu_handlers.go`) emit through the Wails bus directly,
because there is no domain that owns "user clicked a menu item".

## Lifecycle hooks beyond services

`application.Options` accepts four optional callbacks for app-wide control:

| Hook                                             | When                                                     | Cancellable?                   | Use for                                                 |
|--------------------------------------------------|----------------------------------------------------------|--------------------------------|---------------------------------------------------------|
| `ShouldQuit`                                     | Before quitting (Cmd+Q, last window close, `app.Quit()`) | Yes — return `false` to cancel | Confirm save / unsaved-changes prompts                  |
| `OnShutdown`                                     | After `ShouldQuit` returns true                          | No                             | App-level cleanup; runs before `ServiceShutdown`        |
| `PostShutdown`                                   | After all shutdown completes                             | No                             | Final flushing; app instance is unusable here           |
| `RegisterHook(events.Common.WindowClosing, ...)` | Per-window close request                                 | Yes — `e.Cancel()`             | Hide window instead of closing; per-window save prompts |

Lifecycle order: `ShouldQuit` → `OnShutdown` → `ServiceShutdown` (reverse registration order) → `PostShutdown` → process
exit.

## Common events

Use `events.Common.*` (cross-platform) by default — Wails maps platform-native events into them. Reach for
`events.Mac.*` / `events.Windows.*` / `events.Linux.*` only for platform-specific behaviour not present in `Common`.

Frequently needed:

| Event                                           | Purpose                                                 |
|-------------------------------------------------|---------------------------------------------------------|
| `events.Common.ApplicationStarted`              | App init complete                                       |
| `events.Common.WindowRuntimeReady`              | Frontend JS runtime ready — safe to emit                |
| `events.Common.WindowClosing`                   | Window close requested (cancellable via `RegisterHook`) |
| `events.Common.WindowFocus` / `WindowLostFocus` | Active window changed                                   |
| `events.Common.ThemeChanged`                    | OS theme toggled (light/dark)                           |
| `events.Common.WindowFilesDropped`              | Native OS file drop on window                           |

`WindowDidMove` and `WindowDidResize` are debounced (50 ms) — they will not fire on every pixel.

## Where to look in the docs

Paths are relative to `~/projects/books/wails/docs/src/content/docs/`.

| Topic                                                          | Path                                                                                                                                                                     |
|----------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Service contract, lifecycle                                    | `features/bindings/services.mdx`, `concepts/lifecycle.mdx`                                                                                                               |
| Method binding, types, model rules                             | `features/bindings/methods.mdx`, `features/bindings/models.mdx`, `features/bindings/best-practices.mdx`, `features/bindings/enums.mdx`, `features/bindings/advanced.mdx` |
| Events API, common events, window hooks                        | `reference/events.mdx`                                                                                                                                                   |
| Application options, app-level hooks                           | `reference/application.mdx`                                                                                                                                              |
| Window API                                                     | `reference/window.mdx`                                                                                                                                                   |
| Menus (app menu, context menu, system tray)                    | `reference/menu.mdx`, `features/menus/*.mdx`                                                                                                                             |
| Dialogs (file, message, custom)                                | `reference/dialogs.mdx`, `features/dialogs/*.mdx`                                                                                                                        |
| Drag and drop                                                  | `features/drag-and-drop/*.mdx`                                                                                                                                           |
| CLI (`wails3 dev`, `wails3 build`, `wails3 generate bindings`) | `reference/cli.mdx`, `concepts/build-system.mdx`                                                                                                                         |
| Auto-updater                                                   | `reference/updater.mdx`                                                                                                                                                  |
| Architecture (single-binary bundling, asset server)            | `concepts/architecture.mdx`, `concepts/bridge.mdx`                                                                                                                       |

## Writer Studio additions

These are conventions Wails does **not** require but Writer Studio adopted:

- **Per-project SQLite services.** Created in `OnProjectOpened()`, torn down in `OnProjectClosing()`. Their controllers
  funnel through `getDomain()` helpers that return `project.ErrNotOpen` between sessions. See hard rule 5 in
  `coding-standards-go`.
- **Domain `EventEmitter`.** Most events go through `internal/events/EventEmitter`, not `app.Event.Emit`. Late-bind the
  real emitter in `Container.SetApp()`.
- **Event names: `domain:action-kebab-case`.** See hard rule 6 in `coding-standards-go`.
- **Service file location.** Wails services live in `api/{name}.go`. Constructors are exported (`NewBookAdapter`) but
  the type is `BookController` — historical naming inconsistency; prefer `New{Name}Controller` for new services.
