<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { Events, System } from '@wailsio/runtime';
  import Board from '$lib/components/Board.svelte';
  import CreateTaskDialog from '$lib/components/CreateTaskDialog.svelte';
  import FilterBar from '$lib/components/FilterBar.svelte';
  import InitBoardDialog from '$lib/components/InitBoardDialog.svelte';
  import SettingsPanel from '$lib/components/SettingsPanel.svelte';
  import TaskDrawer from '$lib/components/TaskDrawer.svelte';
  import Toast from '$lib/components/Toast.svelte';
  import {
    errorString,
    getProjectRoot,
    isCancelledError,
    isNoBoardError,
    isNoTbYamlError,
    listRecentBoards,
    moveTask,
    openBoard,
    pickBoardDialog,
    type RecentBoard,
  } from '$lib/api';
  import {
    board,
    loaded,
    loadError,
    optimisticMove,
    patchTask,
    refresh,
    revert,
    setStatusMode,
  } from '$lib/stores/board';
  import { applyFilter, observedEpics } from '$lib/filtering';
  import { filter } from '$lib/stores/filter';
  import { closeTask, openTask, selectedTaskId } from '$lib/stores/selection';
  import { pushToast } from '$lib/stores/toast';
  import { registerAgentEventHandlers } from '$lib/stores/runs';
  import { registerTriageEventHandlers } from '$lib/stores/triage';
  import {
    hydrate as hydrateUsage,
    registerUsageEventHandler,
  } from '$lib/stores/usage';
  import AgentUsageHeader from '$lib/components/AgentUsageHeader.svelte';
  import { preferencesStore } from '$lib/stores/preferences';
  import { shortcutAction } from '$lib/shortcuts';

  let projectRoot = $state('');
  let recents = $state<RecentBoard[]>([]);
  let bootStatus = $state<'loading' | 'ready' | 'pick'>('loading');
  let createOpen = $state(false);
  // Mirrors CreateTaskDialog.dirty so this component's global Escape
  // shortcut applies the same discard guard as the dialog's own close paths.
  let createDirty = $state(false);
  let settingsOpen = $state(false);
  // Folder picked but lacking `.tb.yaml` — fed to InitBoardDialog so the user
  // can confirm `tb init` against it (or cancel back to the previous board).
  let initBoardRoot = $state('');

  const offEvents: Array<() => void> = [];

  // Live-derived filtered view.
  let filtered = $derived(applyFilter($board, $filter));
  let epics = $derived(observedEpics($board));

  onMount(async () => {
    document.documentElement.classList.toggle('platform-mac', System.IsMac());
    void preferencesStore.load().catch(() => {});
    try { projectRoot = await getProjectRoot(); } catch { projectRoot = ''; }
    try { recents = await listRecentBoards(); } catch { recents = []; }

    if (projectRoot) {
      await refresh();
      bootStatus = 'ready';
    } else if (recents.length > 0) {
      try {
        await openBoard(recents[0].projectRoot);
        projectRoot = recents[0].projectRoot;
        await refresh();
        bootStatus = 'ready';
      } catch (err) {
        pushToast(`Could not reopen ${recents[0].projectRoot}: ${errorString(err)}`);
        bootStatus = 'pick';
      }
    } else {
      bootStatus = 'pick';
    }

    offEvents.push(Events.On('board:reloaded', async () => { await refresh(); }));
    offEvents.push(Events.On('board:opened', async (info: any) => {
      const root = info?.data?.projectRoot ?? info?.projectRoot ?? '';
      if (root) projectRoot = root;
      bootStatus = 'ready';
      try { recents = await listRecentBoards(); } catch { recents = []; }
      await refresh();
    }));
    offEvents.push(Events.On('settings:open-panel', () => { settingsOpen = true; }));
    // File-drop result toast. Wails surfaces a single WindowFilesDropped per
    // logical drop; main.go routes it to BoardService and emits this event.
    // We surface the outcome here so cards and the drawer don't both need
    // their own listener.
    offEvents.push(Events.On('attach:dropped', (raw: any) => {
      const data = raw?.data ?? raw ?? {};
      const taskId: string = typeof data.taskId === 'string' ? data.taskId : '';
      const ok = data.ok === true;
      const count = typeof data.count === 'number' ? data.count : 0;
      const error = typeof data.error === 'string' ? data.error : '';
      if (ok) {
        pushToast(taskId ? `Attached ${count} file(s) to ${taskId}` : `Attached ${count} file(s)`, 'success');
      } else {
        pushToast(error ? `Attach failed: ${error}` : 'Attach failed');
      }
    }));
    offEvents.push(Events.On('task:updated', async (raw: any) => {
      const name: string = raw?.name ?? '';
      const id = name.replace(/^task:updated:/, '');
      if (id) await patchTask(id);
    }));

    // Agent run lifecycle — populate runsStore from Wails events so any
    // drawer / log panel re-renders without re-reading the JSONL.
    offEvents.push(
      registerAgentEventHandlers((name, handler) => Events.On(name, handler as any)),
    );
    offEvents.push(
      registerTriageEventHandlers((name, handler) => Events.On(name, handler as any)),
    );
    // Per-agent quota usage (TB-107): seed from backend cache, then live-
    // update on agent-usage:updated events from the periodic refresh loop.
    void hydrateUsage();
    offEvents.push(
      registerUsageEventHandler((name, handler) => Events.On(name, handler as any)),
    );
    window.addEventListener('keydown', onGlobalKeydown);
    offEvents.push(() => window.removeEventListener('keydown', onGlobalKeydown));
  });

  onDestroy(() => {
    for (const off of offEvents) { try { off(); } catch { /* ignore */ } }
  });

  async function pickAndOpen() {
    let path = '';
    try {
      path = await pickBoardDialog();
      if (!path) return;
      await openBoard(path);
      projectRoot = path;
      recents = await listRecentBoards();
      await refresh();
      bootStatus = 'ready';
    } catch (err) {
      if (isCancelledError(err)) return;
      if (isNoTbYamlError(err)) {
        // Hand off to the InitBoardDialog. The previous board (if any)
        // remains active until the user confirms.
        initBoardRoot = path;
        return;
      }
      if (isNoBoardError(err)) return;
      pushToast(errorString(err));
    }
  }

  async function onInitBoardConfirmed() {
    const newRoot = initBoardRoot;
    initBoardRoot = '';
    // OpenBoard already emitted board:opened from the backend, but refresh
    // explicitly so a not-yet-listening watcher still gets us to ready.
    projectRoot = newRoot;
    try { recents = await listRecentBoards(); } catch { /* recents are non-fatal */ }
    await refresh();
    bootStatus = 'ready';
  }

  function onInitBoardCancelled() {
    initBoardRoot = '';
  }

  async function openRecent(r: RecentBoard) {
    try {
      await openBoard(r.projectRoot);
      projectRoot = r.projectRoot;
      recents = await listRecentBoards();
      await refresh();
      bootStatus = 'ready';
    } catch (err) {
      pushToast(`Failed to open ${r.projectRoot}: ${errorString(err)}`);
    }
  }

  function shortPath(p: string): string {
    if (!p) return '';
    const home = '/Users/';
    if (p.startsWith(home)) {
      const rest = p.slice(home.length);
      const i = rest.indexOf('/');
      return i === -1 ? '~' : '~/' + rest.slice(i + 1);
    }
    return p;
  }

  async function onDrop(taskId: string, target: 'backlog' | 'in-progress' | 'code-review' | 'done') {
    const before = optimisticMove(taskId, target);
    try {
      await moveTask(taskId, target);
    } catch (err) {
      revert(before);
      pushToast(`Move failed: ${errorString(err)}`);
    }
  }

  function onShowArchiveChange(show: boolean) {
    setStatusMode(show ? 'all' : 'active');
  }

  function tryCloseCreate() {
    if (createDirty) {
      const ok = window.confirm('Discard this unsaved task?');
      if (!ok) return;
    }
    createOpen = false;
  }

  function onGlobalKeydown(event: KeyboardEvent) {
    const focusedCardId = focusedTaskId();
    const action = shortcutAction(event, {
      createOpen,
      settingsOpen,
      drawerOpen: $selectedTaskId != null,
      focusedCardId,
    });
    if (action === 'none') return;
    event.preventDefault();

    switch (action) {
      case 'open-create':
        createOpen = true;
        break;
      case 'focus-search':
        document.querySelector<HTMLInputElement>('.filter .search')?.focus();
        break;
      case 'close-settings':
        settingsOpen = false;
        break;
      case 'close-create':
        tryCloseCreate();
        break;
      case 'close-drawer':
        closeTask();
        break;
      case 'blur-card':
        (document.activeElement as HTMLElement | null)?.blur?.();
        break;
      case 'open-focused-card':
        if (focusedCardId) openTask(focusedCardId);
        break;
    }
  }

  function focusedTaskId(): string | null {
    const active = document.activeElement;
    if (!(active instanceof HTMLElement)) return null;
    return active.closest<HTMLElement>('[data-task-id]')?.dataset.taskId ?? null;
  }
</script>

<svelte:head>
  <title>tb-gui</title>
</svelte:head>

<main class="shell">
  <header class="topbar">
    <div class="title">
      <h1>tb-gui</h1>
      {#if projectRoot}
        <span class="root" title={projectRoot}>{shortPath(projectRoot)}</span>
      {/if}
    </div>
    <div class="actions">
      <AgentUsageHeader />
      {#if bootStatus === 'ready'}
        <button class="new" onclick={() => (createOpen = true)} title="Create task (n)">+ New</button>
      {/if}
      <button class="pick" onclick={() => (settingsOpen = true)}>Settings</button>
      <button class="pick" onclick={pickAndOpen}>Open board…</button>
    </div>
  </header>

  {#if bootStatus === 'loading'}
    <section class="empty"><p>Loading…</p></section>
  {:else if bootStatus === 'pick'}
    <section class="empty">
      <h2>No board open</h2>
      <p>Pick a folder that contains a <code>.tb.yaml</code> to get started.</p>
      <button class="primary" onclick={pickAndOpen}>Open board…</button>
      {#if recents.length > 0}
        <h3>Recent</h3>
        <ul class="recents">
          {#each recents as r (r.projectRoot)}
            <li>
              <button class="link" onclick={() => openRecent(r)}>{r.projectRoot}</button>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  {:else}
    <FilterBar snapshot={$board} {onShowArchiveChange} />
    <Board snapshot={filtered} showArchive={$filter.showArchive} onSelect={openTask} {onDrop} />
    {#if $loadError}<p class="err">{$loadError}</p>{/if}
    {#if !$loaded}<p class="hint">Loading…</p>{/if}
  {/if}
</main>

<TaskDrawer taskId={$selectedTaskId} onClose={closeTask} />

<CreateTaskDialog
  open={createOpen}
  {epics}
  bind:dirty={createDirty}
  onClose={() => (createOpen = false)}
  onCreated={(id) => openTask(id)} />

<SettingsPanel open={settingsOpen} onClose={() => (settingsOpen = false)} />

<InitBoardDialog
  open={initBoardRoot !== ''}
  projectRoot={initBoardRoot}
  onCancel={onInitBoardCancelled}
  onInitialized={onInitBoardConfirmed} />

<Toast />

<style>
  :global(:root) {
    --bg: rgb(20, 26, 38);
    --bg-elev: rgb(28, 36, 52);
    --bg-card: rgb(34, 44, 64);
    --fg: #e6e6e6;
    --fg-dim: rgba(230, 230, 230, 0.55);
    --accent: #4a8df8;
    --p0: #ff5a52;
    --p1: #ffb86c;
    --p2: #4a8df8;
    --p3: #6e7686;
    --radius: 8px;
    --mac-titlebar-height: 50px;
    --mac-traffic-light-safe-left: 152px;
  }
  :global(body) {
    margin: 0;
    background: var(--bg);
    color: var(--fg);
    font: 13px/1.45 -apple-system, BlinkMacSystemFont, system-ui, sans-serif;
  }
  :global(button) { font: inherit; color: inherit; }

  .shell {
    display: flex;
    flex-direction: column;
    height: 100vh;
    overflow: hidden;
  }
  .topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 12px 18px 8px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);
    background: var(--bg-elev);
    --wails-draggable: drag;
    -webkit-app-region: drag;
  }
  :global(html.platform-mac) .topbar {
    min-height: var(--mac-titlebar-height);
    padding-left: var(--mac-traffic-light-safe-left);
  }
  .actions {
    --wails-draggable: no-drag;
    -webkit-app-region: no-drag;
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
  .topbar h1 { margin: 0; font-size: 14px; font-weight: 600; letter-spacing: 0.02em; }
  .title { display: flex; align-items: baseline; gap: 10px; min-width: 0; }
  .root {
    color: var(--fg-dim);
    font-size: 12px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  :global(html.platform-mac) .actions {
    flex-wrap: wrap;
  }
  .pick {
    background: rgba(255, 255, 255, 0.08);
    border: 1px solid rgba(255, 255, 255, 0.12);
    color: var(--fg);
    border-radius: 6px;
    padding: 5px 12px;
    cursor: pointer;
  }
  .pick:hover { background: rgba(255, 255, 255, 0.14); }
  .new {
    background: var(--accent);
    border: 0;
    color: white;
    border-radius: 6px;
    padding: 5px 14px;
    cursor: pointer;
    font-weight: 600;
  }
  .new:hover { filter: brightness(1.1); }

  .empty {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 40px;
    text-align: center;
  }
  .empty h2 { margin: 0 0 8px; font-size: 18px; }
  .empty p { color: var(--fg-dim); margin: 0 0 16px; }
  .empty code {
    background: rgba(255, 255, 255, 0.08);
    padding: 1px 5px;
    border-radius: 4px;
    font-family: ui-monospace, monospace;
  }
  .empty .primary {
    background: var(--accent);
    border: 0;
    color: white;
    padding: 8px 18px;
    border-radius: 6px;
    cursor: pointer;
    font-weight: 600;
  }
  .empty .primary:hover { filter: brightness(1.1); }
  .empty h3 {
    margin: 24px 0 8px;
    font-size: 12px;
    color: var(--fg-dim);
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
  .empty .recents { list-style: none; padding: 0; margin: 0; max-width: 480px; width: 100%; }
  .empty .recents li { margin: 0 0 4px; }
  .empty .link {
    background: none;
    border: 0;
    color: var(--accent);
    cursor: pointer;
    text-align: left;
    padding: 4px 0;
    font-family: ui-monospace, monospace;
    font-size: 12px;
  }
  .empty .link:hover { text-decoration: underline; }

  .err { padding: 8px 18px; color: var(--p0); font-size: 12px; border-top: 1px solid rgba(255, 90, 82, 0.3); }
  .hint { padding: 8px 18px; color: var(--fg-dim); font-size: 12px; }
</style>
