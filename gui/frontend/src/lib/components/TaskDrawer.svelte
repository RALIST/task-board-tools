<script lang="ts">
  import { marked } from 'marked';
  import DOMPurify from 'dompurify';
  import { Events } from '@wailsio/runtime';
  import {
    assignAgent,
    cancelRun,
    closeTask as apiCloseTask,
    editTask,
    editTaskBody,
    getTask,
    listRuns,
    runAgent,
    type AgentName,
    type EditTaskInput,
    type Task,
    type TaskDetail,
  } from '$lib/api';
  import { pushToast } from '$lib/stores/toast';
  import {
    runsForTask,
    selectedRunID,
    setRunsForTask,
    upsertRun,
    type Run,
  } from '$lib/stores/runs';
  import BodyEditor from './BodyEditor.svelte';
  import AgentRunLog from './AgentRunLog.svelte';

  interface Props {
    taskId: string | null;
    onClose?: () => void;
  }

  let { taskId, onClose }: Props = $props();

  let detail = $state<TaskDetail | null>(null);
  let loading = $state(false);
  let err = $state<string | null>(null);

  // Editable metadata fields. Initialised from `detail` whenever the task
  // changes; tracked separately so we can highlight dirty state and submit
  // only the diff.
  let formPriority = $state('');
  let formType = $state('');
  let formSize = $state('');
  let formModule = $state('');
  let formTags = $state('');
  let saving = $state(false);

  // Editor state.
  let editMode = $state(false);
  let bodyDraft = $state(''); // current editor contents (full file, header included)
  let bodyDirty = $state(false);

  // Archive button two-step confirm.
  let archivePrompt = $state(false);
  let archiveTimer: ReturnType<typeof setTimeout> | null = null;

  // Cancel-run two-step confirm (same UX pattern as Archive).
  let cancelPrompt = $state(false);
  let cancelTimer: ReturnType<typeof setTimeout> | null = null;

  // Agent dropdown's current selection — derived from detail.metadata.agent
  // but stored separately so the user can pick "claude" without immediately
  // firing AssignAgent until the dropdown commits.
  let formAgent = $state<AgentName>('');
  let agentSaving = $state(false);
  let runStarting = $state(false);

  // Past-run subscription. runsStore is keyed by run_id; we project the
  // task's slice via runsForTask. Hydrated on every taskId change.
  let runs: Run[] = $state([]);
  let runsUnsub: (() => void) | null = null;

  $effect(() => {
    const id = taskId;
    if (!id) {
      detail = null; err = null; loading = false;
      editMode = false; bodyDirty = false; archivePrompt = false;
      cancelPrompt = false; runs = [];
      runsUnsub?.(); runsUnsub = null;
      return;
    }
    loading = true;
    err = null;
    let cancelled = false;

    const fetchOnce = () => {
      getTask(id)
        .then((d) => {
          if (cancelled || taskId !== id) return;
          detail = d;
          loading = false;
          // Reset form to the freshly-loaded values.
          formPriority = d.metadata.priority ?? '';
          formType = d.metadata.type ?? '';
          formSize = d.metadata.size ?? '';
          formModule = d.metadata.module ?? '';
          formTags = (d.metadata.tags ?? []).join(', ');
          formAgent = (d.metadata.agent ?? '') as AgentName;
          // Don't replace bodyDraft while the editor is open — preserve the
          // user's in-progress buffer. If they Discard, we'll snap it back
          // to d.body via the Discard handler.
          if (!editMode) bodyDraft = d.body;
        })
        .catch((e) => {
          if (cancelled || taskId !== id) return;
          err = e instanceof Error ? e.message : String(e);
          loading = false;
        });
    };

    // Hydrate past runs from disk and subscribe to live store updates.
    listRuns(id).then((list) => {
      if (cancelled || taskId !== id) return;
      setRunsForTask(id, list as Run[]);
    }).catch(() => { /* empty list is fine */ });

    const taskRunsStore = runsForTask(id);
    runsUnsub?.();
    runsUnsub = taskRunsStore.subscribe((list) => {
      if (taskId !== id) return;
      runs = list;
      // Default selectedRunID to the most recent run if none is selected.
      if ($selectedRunID == null || !list.find((r) => r.runId === $selectedRunID)) {
        selectedRunID.set(list[0]?.runId ?? null);
      }
    });

    fetchOnce();
    // Subscribe to BOTH event shapes:
    //  - task:updated:<id> fires when the CLI does a direct Write (rare,
    //    since both CLI and GUI write atomically via temp+rename).
    //  - board:reloaded fires for atomic writes (Create/Rename) and is
    //    the dominant refresh signal in practice.
    const offTask = Events.On(`task:updated:${id}`, () => fetchOnce());
    const offBoard = Events.On('board:reloaded', () => fetchOnce());

    return () => {
      cancelled = true;
      try { offTask(); } catch { /* ignore */ }
      try { offBoard(); } catch { /* ignore */ }
      runsUnsub?.();
      runsUnsub = null;
    };
  });

  // Selected run lookup for the status pill source-of-truth (per F4.3 the
  // pill comes from the live Run record, not from currentTask.agentStatus
  // which lags by one tb edit).
  let selectedRun = $derived(runs.find((r) => r.runId === $selectedRunID) ?? null);
  let liveStatus = $derived(selectedRun?.status ?? '');

  async function onAgentChange() {
    if (!detail) return;
    const id = detail.metadata.id;
    const target = (formAgent || 'none') as AgentName;
    const prev = (detail.metadata.agent ?? '') as AgentName;
    if (target === prev || (target === 'none' && prev === '')) return;
    agentSaving = true;
    try {
      await assignAgent(id, target);
      pushToast(target === 'none' ? `Cleared agent for ${id}` : `Assigned ${target} to ${id}`, 'success');
    } catch (e) {
      pushToast(`Assign failed: ${e instanceof Error ? e.message : String(e)}`);
      // Snap dropdown back to disk state on failure.
      formAgent = (detail.metadata.agent ?? '') as AgentName;
    } finally {
      agentSaving = false;
    }
  }

  async function onRunClick() {
    if (!detail) return;
    const id = detail.metadata.id;
    runStarting = true;
    try {
      const runId = await runAgent(id);
      // Optimistically insert a queued row so the UI is responsive even
      // before the Wails event arrives (avoids a flicker).
      upsertRun({
        runId,
        taskId: id,
        agent: detail.metadata.agent ?? '',
        mode: 'implement',
        status: 'queued',
        queuedAt: new Date().toISOString(),
      });
      selectedRunID.set(runId);
      pushToast(`Started run on ${id}`, 'success');
    } catch (e) {
      pushToast(`Run failed: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      runStarting = false;
    }
  }

  function startCancel() {
    if (!detail) return;
    if (!cancelPrompt) {
      cancelPrompt = true;
      if (cancelTimer) clearTimeout(cancelTimer);
      cancelTimer = setTimeout(() => { cancelPrompt = false; }, 4000);
      return;
    }
    if (cancelTimer) { clearTimeout(cancelTimer); cancelTimer = null; }
    void doCancel();
  }

  async function doCancel() {
    if (!detail) return;
    const id = detail.metadata.id;
    try {
      await cancelRun(id);
      pushToast(`Cancelled run on ${id}`, 'info');
    } catch (e) {
      pushToast(`Cancel failed: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      cancelPrompt = false;
    }
  }

  function pickRun(runID: string) {
    selectedRunID.set(runID);
  }

  function fmtRelative(iso: string): string {
    if (!iso) return '—';
    const t = Date.parse(iso);
    if (!t) return iso;
    const delta = Date.now() - t;
    const m = Math.round(delta / 60_000);
    if (m < 1) return 'just now';
    if (m < 60) return `${m}m ago`;
    const h = Math.round(m / 60);
    if (h < 24) return `${h}h ago`;
    const d = Math.round(h / 24);
    return `${d}d ago`;
  }

  let metadataDirty = $derived(
    detail !== null && (
      (formPriority || '') !== (detail.metadata.priority ?? '') ||
      (formType || '') !== (detail.metadata.type ?? '') ||
      (formSize || '') !== (detail.metadata.size ?? '') ||
      (formModule || '') !== (detail.metadata.module ?? '') ||
      normalizeTags(formTags) !== normalizeTags((detail.metadata.tags ?? []).join(', '))
    ),
  );

  function normalizeTags(s: string): string {
    return s.split(',').map((t) => t.trim()).filter(Boolean).sort().join(',');
  }

  function diffPayload(): { payload: EditTaskInput; droppedClears: string[] } {
    if (!detail) return { payload: {} as EditTaskInput, droppedClears: [] };
    const m = detail.metadata;
    const payload: EditTaskInput = {} as EditTaskInput;
    const droppedClears: string[] = [];

    // The CLI's `tb edit` treats empty-string args as "skip this field" — it
    // has no clear-field syntax. So if the user blanks Module/Tags (was
    // non-empty), we can't actually clear them; ignore the diff for that
    // field and warn via toast.
    function include(name: keyof EditTaskInput, label: string, next: string, prev: string) {
      if (next === prev) return;
      if (next === '' && prev !== '') {
        droppedClears.push(label);
        return;
      }
      (payload as unknown as Record<string, string>)[name as string] = next;
    }
    include('priority', 'priority', formPriority, m.priority ?? '');
    include('type', 'type', formType, m.type ?? '');
    include('size', 'size', formSize, m.size ?? '');
    include('module', 'module', formModule, m.module ?? '');

    const nextTags = normalizeTags(formTags);
    const prevTags = normalizeTags((m.tags ?? []).join(', '));
    if (nextTags !== prevTags) {
      if (nextTags === '' && prevTags !== '') {
        droppedClears.push('tags');
      } else {
        payload.tags = formTags.split(',').map((t) => t.trim()).filter(Boolean).join(',');
      }
    }
    return { payload, droppedClears };
  }

  async function saveMetadata() {
    if (!detail || !metadataDirty || saving) return;
    saving = true;
    const id = detail.metadata.id;
    const { payload, droppedClears } = diffPayload();

    if (Object.keys(payload).length === 0) {
      // The only diff was a field clear we can't represent — surface that
      // rather than silently no-op'ing.
      if (droppedClears.length > 0) {
        pushToast(`Clearing ${droppedClears.join(', ')} from the GUI isn't supported (CLI has no clear flag).`, 'info');
        resetForm();
      }
      saving = false;
      return;
    }

    try {
      await editTask(id, payload);
      if (droppedClears.length > 0) {
        pushToast(`Saved ${id}; ${droppedClears.join(', ')} couldn't be cleared from the GUI.`, 'info');
        resetForm();
      } else {
        pushToast(`Saved ${id}`, 'success');
      }
    } catch (e) {
      pushToast(`Save failed: ${e instanceof Error ? e.message : String(e)}`);
      resetForm();
    } finally {
      saving = false;
    }
  }

  function resetForm() {
    if (!detail) return;
    formPriority = detail.metadata.priority ?? '';
    formType = detail.metadata.type ?? '';
    formSize = detail.metadata.size ?? '';
    formModule = detail.metadata.module ?? '';
    formTags = (detail.metadata.tags ?? []).join(', ');
  }

  function startArchive() {
    if (!detail) return;
    if (!archivePrompt) {
      archivePrompt = true;
      if (archiveTimer) clearTimeout(archiveTimer);
      archiveTimer = setTimeout(() => { archivePrompt = false; }, 4000);
      return;
    }
    if (archiveTimer) { clearTimeout(archiveTimer); archiveTimer = null; }
    void doArchive();
  }

  async function doArchive() {
    if (!detail) return;
    const id = detail.metadata.id;
    try {
      await apiCloseTask(id);
      pushToast(`Archived ${id}`, 'success');
      onClose?.();
    } catch (e) {
      pushToast(`Archive failed: ${e instanceof Error ? e.message : String(e)}`);
      archivePrompt = false;
    }
  }

  async function saveBody() {
    if (!detail) return;
    try {
      await editTaskBody(detail.metadata.id, bodyDraft);
      pushToast(`Body saved`, 'success');
      bodyDirty = false;
      editMode = false;
      // detail will refresh via task:updated event
    } catch (e) {
      pushToast(`Body save failed: ${e instanceof Error ? e.message : String(e)}`);
    }
  }

  async function enterEdit() {
    if (!detail) return;
    // Pull the latest body from disk before mounting the editor so the
    // buffer matches what EditTaskBody will compare against. Avoids the
    // race where a recent metadata edit mutated the file but the watcher
    // event hadn't refreshed our `detail.body` yet.
    try {
      const d = await getTask(detail.metadata.id);
      detail = d;
      bodyDraft = d.body;
    } catch {
      bodyDraft = detail.body;
    }
    bodyDirty = false;
    editMode = true;
  }

  function discardBody() {
    if (!detail) return;
    bodyDraft = detail.body;
    bodyDirty = false;
    editMode = false;
  }

  function onKeydown(ev: KeyboardEvent) {
    if (!taskId) return;
    if (ev.key === 'Escape') {
      ev.preventDefault();
      tryClose();
    } else if (editMode && (ev.metaKey || ev.ctrlKey) && ev.key.toLowerCase() === 's') {
      ev.preventDefault();
      void saveBody();
    }
  }

  function tryClose() {
    if (editMode && bodyDirty) {
      const ok = window.confirm('You have unsaved body edits. Discard them?');
      if (!ok) return;
    }
    onClose?.();
  }

  function onBackdropClick(ev: MouseEvent) {
    if (ev.target === ev.currentTarget) tryClose();
  }

  marked.setOptions({ gfm: true, breaks: false });

  function renderMarkdown(src: string): string {
    try {
      const raw = marked.parse(src, { async: false }) as string;
      // Sanitize: task bodies are user-authored and may contain raw HTML
      // (`<img onerror=…>`, `<script>`, etc.). Stripping disallowed tags here
      // prevents an untrusted task from executing JS in the GUI's privileged
      // Wails context. KEEP_CONTENT means HTML text inside disallowed tags is
      // still shown as escaped text.
      return DOMPurify.sanitize(raw, { USE_PROFILES: { html: true } });
    } catch {
      return `<pre>${escapeHtml(src)}</pre>`;
    }
  }
  function escapeHtml(s: string): string {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function stripFrontmatter(body: string): string {
    const lines = body.split('\n');
    for (let i = 0; i < Math.min(lines.length, 30); i++) {
      if (i > 0 && lines[i].trim().startsWith('## ')) {
        return lines.slice(i).join('\n');
      }
    }
    return body;
  }

  function meta(t: Task): Array<[string, string]> {
    const rows: Array<[string, string]> = [];
    if (t.status) rows.push(['Status', t.status]);
    if (t.branch) rows.push(['Branch', t.branch]);
    if (t.parent) rows.push(['Parent', t.parent]);
    if (t.agent) rows.push(['Agent', t.agent + (t.agentStatus ? ` (${t.agentStatus})` : '')]);
    return rows;
  }
</script>

<svelte:window onkeydown={onKeydown} />

{#if taskId}
  <div
    class="backdrop"
    role="dialog"
    aria-modal="true"
    aria-label="Task detail"
    tabindex="-1"
    onclick={onBackdropClick}
    onkeydown={() => {}}>
    <aside class="drawer">
      <header>
        {#if detail}
          <div>
            <span class="id">{detail.metadata.id}</span>
            <h2>{detail.metadata.title}</h2>
          </div>
        {:else}
          <h2>{taskId}</h2>
        {/if}
        <button class="close" type="button" onclick={tryClose} aria-label="Close">×</button>
      </header>

      {#if loading && !detail}
        <p class="hint">Loading…</p>
      {:else if err}
        <p class="err">{err}</p>
      {:else if detail}
        <section class="meta-edit">
          <div class="meta-row">
            <label>
              <span>Priority</span>
              <select bind:value={formPriority}>
                <option value=""></option>
                <option>P0</option><option>P1</option><option>P2</option><option>P3</option>
              </select>
            </label>
            <label>
              <span>Type</span>
              <select bind:value={formType}>
                <option value=""></option>
                <option value="bug">bug</option>
                <option value="feature">feature</option>
                <option value="tech-debt">tech-debt</option>
                <option value="improvement">improvement</option>
                <option value="spike">spike</option>
              </select>
            </label>
            <label>
              <span>Size</span>
              <select bind:value={formSize}>
                <option value=""></option>
                <option>S</option><option>M</option><option>L</option><option>XL</option>
              </select>
            </label>
          </div>
          <div class="meta-row">
            <label class="grow">
              <span>Module</span>
              <input bind:value={formModule} placeholder="module" />
            </label>
            <label class="grow">
              <span>Tags</span>
              <input bind:value={formTags} placeholder="comma,separated" />
            </label>
          </div>
          <div class="row save-row">
            <button class="primary" type="button" onclick={saveMetadata} disabled={!metadataDirty || saving}>
              {saving ? 'Saving…' : (metadataDirty ? 'Save' : 'Saved')}
            </button>
          </div>
        </section>

        {#if meta(detail.metadata).length > 0}
          <dl class="meta">
            {#each meta(detail.metadata) as [k, v]}
              <dt>{k}</dt><dd>{v}</dd>
            {/each}
          </dl>
        {/if}

        <section class="agent-section">
          <header class="agent-head">
            <h3>Agent</h3>
            {#if liveStatus}
              <span class={`pill pill-${liveStatus}`}>{liveStatus}</span>
            {/if}
          </header>
          <div class="agent-controls">
            <label class="agent-dropdown">
              <span>Assigned</span>
              <select
                aria-label="Agent"
                disabled={agentSaving}
                bind:value={formAgent}
                onchange={onAgentChange}>
                <option value="">(none)</option>
                <option value="claude">claude</option>
                <option value="codex">codex</option>
              </select>
            </label>
            <div class="agent-buttons">
              <button
                class="primary"
                type="button"
                disabled={!formAgent || runStarting || liveStatus === 'queued' || liveStatus === 'running'}
                title={!formAgent ? 'Assign an agent first' : (liveStatus === 'running' ? 'Already running' : '')}
                onclick={onRunClick}>
                {runStarting ? 'Starting…' : 'Run agent'}
              </button>
              {#if liveStatus === 'running'}
                <button class="danger" type="button" onclick={startCancel}>
                  {cancelPrompt ? 'Click again to cancel' : 'Cancel'}
                </button>
              {/if}
            </div>
          </div>

          {#if runs.length > 0}
            <ul class="run-list" aria-label="Past runs">
              {#each runs as r}
                <li>
                  <button
                    type="button"
                    class:active={$selectedRunID === r.runId}
                    onclick={() => pickRun(r.runId)}>
                    <span class="when">{fmtRelative(r.startedAt || r.queuedAt)}</span>
                    <span class={`pill pill-${r.status || 'idle'}`}>{r.status || 'idle'}</span>
                    <span class="agent">{r.agent}</span>
                  </button>
                </li>
              {/each}
            </ul>
          {/if}

          <AgentRunLog runId={$selectedRunID} taskId={detail.metadata.id} />
        </section>

        <section class="body-section">
          <div class="body-toolbar">
            <h3>Body</h3>
            <div class="toolbar-actions">
              {#if editMode}
                <button class="ghost" type="button" onclick={discardBody}>Discard</button>
                <button class="primary" type="button" onclick={saveBody} disabled={!bodyDirty}>Save body</button>
              {:else}
                <button class="ghost" type="button" onclick={enterEdit}>Edit</button>
              {/if}
            </div>
          </div>
          {#if editMode}
            <BodyEditor
              bind:value={bodyDraft}
              originalBody={detail.body}
              onDirtyChange={(d) => bodyDirty = d}
            />
          {:else}
            <article class="body markdown">
              {@html renderMarkdown(stripFrontmatter(detail.body))}
            </article>
          {/if}
        </section>

        <footer class="drawer-footer">
          <button class="danger" type="button" onclick={startArchive}>
            {archivePrompt ? 'Click again to archive' : 'Archive'}
          </button>
        </footer>
      {/if}
    </aside>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.45);
    display: flex;
    justify-content: flex-end;
    z-index: 50;
  }
  .drawer {
    background: var(--bg-elev);
    width: min(720px, 96vw);
    height: 100%;
    overflow-y: auto;
    box-shadow: -8px 0 32px rgba(0, 0, 0, 0.45);
    padding: 20px 22px;
    box-sizing: border-box;
    border-left: 1px solid rgba(255, 255, 255, 0.06);
  }
  header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 18px;
  }
  .id {
    display: inline-block;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 11px;
    color: var(--fg-dim);
    margin-bottom: 4px;
  }
  header h2 {
    margin: 0;
    font-size: 18px;
    line-height: 1.3;
    font-weight: 600;
  }
  .close {
    background: none;
    border: 0;
    font-size: 22px;
    line-height: 1;
    cursor: pointer;
    padding: 4px 8px;
    color: var(--fg-dim);
    border-radius: 4px;
  }
  .close:hover { background: rgba(255, 255, 255, 0.06); color: var(--fg); }

  .meta-edit {
    background: rgba(255, 255, 255, 0.03);
    padding: 12px;
    border-radius: 6px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 14px;
  }
  .meta-row {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 8px;
  }
  .meta-edit label {
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 11px;
    color: var(--fg-dim);
  }
  .meta-edit input,
  .meta-edit select {
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--fg);
    border-radius: 4px;
    padding: 5px 7px;
    font: inherit;
    font-size: 12px;
  }
  .save-row { justify-content: flex-end; display: flex; }
  .primary {
    background: var(--accent);
    color: white;
    border: 0;
    border-radius: 5px;
    padding: 5px 14px;
    cursor: pointer;
    font-weight: 600;
    font: inherit;
    font-size: 12px;
  }
  .primary:disabled { opacity: 0.4; cursor: not-allowed; }
  .ghost {
    background: transparent;
    border: 1px solid rgba(255, 255, 255, 0.12);
    color: var(--fg);
    border-radius: 5px;
    padding: 5px 14px;
    cursor: pointer;
    font: inherit;
    font-size: 12px;
  }
  .ghost:hover { background: rgba(255, 255, 255, 0.06); }

  .meta {
    display: grid;
    grid-template-columns: 96px 1fr;
    column-gap: 14px;
    row-gap: 6px;
    margin: 0 0 18px;
    padding: 12px 14px;
    background: rgba(255, 255, 255, 0.03);
    border-radius: 6px;
    font-size: 12px;
  }
  .meta dt {
    color: var(--fg-dim);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-size: 10px;
    margin: 0;
    align-self: center;
  }
  .meta dd { margin: 0; word-break: break-word; }

  .body-section { display: flex; flex-direction: column; gap: 8px; }
  .body-toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  .body-toolbar h3 {
    margin: 0;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--fg-dim);
    font-weight: 600;
  }
  .toolbar-actions { display: flex; gap: 6px; }

  .body {
    background: rgba(255, 255, 255, 0.03);
    padding: 12px 16px;
    border-radius: 6px;
    margin: 0;
    font-size: 13px;
    line-height: 1.6;
    overflow-x: auto;
  }
  .markdown :global(h1),
  .markdown :global(h2),
  .markdown :global(h3) {
    margin: 14px 0 6px;
    font-weight: 600;
    line-height: 1.3;
  }
  .markdown :global(h1) { font-size: 16px; }
  .markdown :global(h2) { font-size: 14px; color: var(--fg-dim); text-transform: uppercase; letter-spacing: 0.06em; }
  .markdown :global(h3) { font-size: 13px; }
  .markdown :global(p) { margin: 0 0 8px; }
  .markdown :global(ul), .markdown :global(ol) { margin: 0 0 8px; padding-left: 20px; }
  .markdown :global(li) { margin: 2px 0; }
  .markdown :global(li input[type='checkbox']) { margin-right: 6px; pointer-events: none; }
  .markdown :global(code) { background: rgba(255, 255, 255, 0.06); padding: 1px 5px; border-radius: 3px; font-family: ui-monospace, monospace; font-size: 12px; }
  .markdown :global(pre) { background: rgba(0, 0, 0, 0.25); padding: 10px 12px; border-radius: 4px; overflow-x: auto; margin: 8px 0; }
  .markdown :global(pre code) { background: none; padding: 0; }
  .markdown :global(a) { color: var(--accent); }
  .markdown :global(strong) { color: var(--fg); }
  .markdown :global(blockquote) { border-left: 3px solid rgba(255, 255, 255, 0.1); padding-left: 10px; margin: 8px 0; color: var(--fg-dim); }

  .drawer-footer {
    margin-top: 22px;
    padding-top: 14px;
    border-top: 1px solid rgba(255, 255, 255, 0.06);
    display: flex;
    justify-content: flex-end;
  }
  .danger {
    background: rgba(255, 90, 82, 0.12);
    color: var(--p0);
    border: 1px solid rgba(255, 90, 82, 0.3);
    border-radius: 5px;
    padding: 6px 14px;
    cursor: pointer;
    font: inherit;
    font-size: 12px;
  }
  .danger:hover { background: rgba(255, 90, 82, 0.2); }

  .agent-section {
    margin: 18px 0;
    padding: 12px;
    background: rgba(255, 255, 255, 0.03);
    border-radius: 6px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .agent-head {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .agent-head h3 {
    margin: 0;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--fg-dim);
    font-weight: 600;
  }
  .agent-controls {
    display: flex;
    align-items: flex-end;
    gap: 8px;
    flex-wrap: wrap;
  }
  .agent-dropdown {
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 11px;
    color: var(--fg-dim);
  }
  .agent-dropdown select {
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--fg);
    border-radius: 4px;
    padding: 5px 7px;
    font: inherit;
    font-size: 12px;
  }
  .agent-buttons {
    display: flex;
    gap: 6px;
    margin-left: auto;
  }
  .pill {
    font-size: 10px;
    font-weight: 700;
    padding: 1px 6px;
    border-radius: 4px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .pill-queued { background: rgba(255, 184, 108, 0.18); color: var(--p1); }
  .pill-running { background: rgba(74, 141, 248, 0.18); color: var(--p2); }
  .pill-success { background: rgba(80, 200, 120, 0.18); color: #50c878; }
  .pill-failed { background: rgba(255, 90, 82, 0.18); color: var(--p0); }
  .pill-cancelled { background: rgba(110, 118, 134, 0.18); color: var(--p3); }
  .pill-idle { background: rgba(110, 118, 134, 0.10); color: var(--fg-dim); }
  .run-list {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 132px;
    overflow-y: auto;
    border-top: 1px solid rgba(255, 255, 255, 0.05);
  }
  .run-list li { display: block; }
  .run-list button {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 4px 6px;
    background: none;
    border: 0;
    color: inherit;
    text-align: left;
    cursor: pointer;
    font: inherit;
    font-size: 11px;
    border-radius: 3px;
  }
  .run-list button:hover { background: rgba(255, 255, 255, 0.04); }
  .run-list button.active { background: rgba(74, 141, 248, 0.12); }
  .run-list .when { color: var(--fg-dim); font-family: ui-monospace, monospace; }
  .run-list .agent { color: var(--fg-dim); }

  .hint { color: var(--fg-dim); font-size: 12px; }
  .err { color: var(--p0); font-size: 12px; }
</style>
