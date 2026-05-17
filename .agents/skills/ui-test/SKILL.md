---
name: ui-test
description: Test real UI — native app via computer-use MCP
---

# Desktop UI Testing

Visually test the desktop app. Take screenshots, analyze with vision, interact via clicks and
keyboard, verify behavior.

## Test Scenario

Use the scenario supplied in the user's prompt. If the user only asks to explore, inspect the app and report the most important visible UI behavior and issues.

**Prefer computer-use MCP** for interactive and exploratory testing — it drives the real native app, handles
Retina scaling automatically, and can test native menus and OS-level behavior. Use Playwright for scripted
regression tests in CI.

## Native App Testing (computer-use MCP)

The computer-use MCP provides screenshot, mouse, keyboard, and scroll tools that control the actual native
app on macOS.

### Setup

#### Step 1: Load tools

```
ToolSearch({ query: "computer-use", max_results: 30 })
```

Load ALL computer-use tools in a single ToolSearch call — the keyword matches every tool name.

#### Step 2: Request access

```
mcp__computer-use__request_access({
  apps: [app-to-test],
  reason: "Testing UI"
})
```

#### Step 3: Launch the app

Either bring the already-running app to front:

```
mcp__computer-use__open_application({ app: "app-to-test" })
```

Or start it via `wails3 dev` in a background Bash command first, wait ~8 seconds, then open it.

#### Step 4: Take initial screenshot

```
mcp__computer-use__screenshot()
```

Verify the app is visible and loaded. If the app shows "Loading ...", wait and retry.

### Core Tools

| Tool               | Use for                                                                                                                 |
|--------------------|-------------------------------------------------------------------------------------------------------------------------|
| `screenshot`       | Capture current screen state — all click coordinates are relative to the latest screenshot                              |
| `zoom`             | Inspect small UI elements (buttons, icons, labels) — returns high-res crop, but clicks still use full-screenshot coords |
| `left_click`       | Click at `[x, y]` coordinates from the screenshot                                                                       |
| `right_click`      | Open context menus                                                                   |
| `double_click`     | Select words in the editor                                                                                              |
| `type`             | Type text into focused element — works with Cyrillic and all Unicode                                                    |
| `key`              | Keyboard shortcuts, e.g. `"cmd+shift+p"`, `"escape"`, `"return"`, `"tab"`                                               |
| `scroll`           | Scroll at coordinates: `{ coordinate: [x,y], scroll_direction: "down", scroll_amount: 3 }`                              |
| `computer_batch`   | Chain multiple actions in one call — eliminates round-trips                                                             |
| `open_application` | Bring app to front / launch it                                                                                          |
| `wait`             | Pause between actions when the UI needs time to update                                                                  |

### Interaction Patterns

#### Click + verify

```
1. screenshot()                          — see current state
2. left_click({ coordinate: [x, y] })   — interact
3. screenshot()                          — verify result
```

#### Batch actions (fast path)

When you can predict a sequence, batch it:

```
computer_batch({ actions: [
  { action: "left_click", coordinate: [x, y] },
  { action: "wait", duration: 0.5 },
  { action: "type", text: "Chapter title" },
  { action: "key", text: "Return" },
  { action: "screenshot" }
]})
```

#### Inspect small elements

When you need to read small text or identify icon buttons:

```
1. screenshot()                                        — get full screen
2. zoom({ region: [x0, y0, x1, y1] })                 — zoom into area of interest
3. left_click({ coordinate: [x, y] })                  — click using FULL screenshot coords (not zoom coords)
```

**IMPORTANT:** `zoom` is read-only for inspection. Click coordinates always reference the full screenshot.

#### Type Cyrillic text

The `type` tool handles Cyrillic natively — no clipboard workaround needed:

```
type({ text: "Глава первая" })
```

### Workflow

#### For each test action:

1. **Screenshot** — see current state, identify targets
2. **Zoom** if needed — inspect small elements for precise coordinates
3. **Act** — click, type, press keys (prefer keyboard shortcuts when available)
4. **Screenshot** — verify the result
5. **Report** — pass/fail with expected vs actual

#### Retry policy:

- If a click misses, use `zoom` to find the exact target and retry
- If still failing after 2 attempts, try an alternative path (keyboard shortcut, command palette, Tab)
- Max 3 attempts per interaction before marking as blocked and moving on

### Rules

- Always screenshot BEFORE and AFTER each interaction
- Use `zoom` for small targets — don't guess coordinates
- Use keyboard shortcuts over clicking when possible
- Use `computer_batch` for predictable multi-step sequences
- Max 3 retry attempts per interaction before moving on
- Do not create/delete user projects unless the test explicitly requires it
- Report issues clearly: expected vs. actual behavior
- Take a final screenshot at the end of every test to capture end state
