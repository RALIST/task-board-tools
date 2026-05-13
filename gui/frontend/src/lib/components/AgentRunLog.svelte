<script lang="ts">
  import { Events } from '@wailsio/runtime';
  import { onMount } from 'svelte';
  import { getRunLog, isRunLogNotFoundError } from '$lib/api';
  import { runById } from '$lib/stores/runs';

  interface Props {
    /** Run identifier whose log to render. When null, the pane is empty. */
    runId: string | null;
    /** Task ID for GetRunLog path resolution. Required for terminal runs;
     * passed in (rather than read from the store) so the fetch isn't blocked
     * on the runsStore having hydrated the matching Run record. */
    taskId: string | null;
  }

  let { runId, taskId }: Props = $props();

  /** Buffered live log lines; rebuilt when runId changes. */
  let lines = $state<string[]>([]);
  /** Static log fetched from disk for terminal runs. */
  let fileText = $state('');
  let fileError = $state<string | null>(null);
  let pre: HTMLElement | undefined = $state();
  let stickyBottom = $state(true);

  // Track the most recent runId-keyed Run record from the store. Status
  // drives whether we subscribe to live events or render the file.
  let runStore = $derived(runById(runId));
  let run = $derived($runStore);

  /** Run is in flight when status is queued or running. The drawer's status
   * pill follows the same rule. */
  let isLive = $derived(run?.status === 'running' || run?.status === 'queued');

  /** Active run subscription. Re-created whenever runId flips. */
  $effect(() => {
    if (!runId) {
      lines = [];
      fileText = '';
      fileError = null;
      return;
    }

    // Reset buffer when switching runs so old lines don't bleed into the
    // new view.
    lines = [];

    if (!isLive) {
      // Terminal run — fetch the static log file. Prefer the prop taskId
      // (which the drawer always knows) over run?.taskId so the fetch
      // doesn't race the runsStore hydration.
      const effectiveTaskId = taskId || run?.taskId || '';
      if (!effectiveTaskId) {
        fileText = '';
        fileError = null;
        return;
      }
      fileText = '';
      fileError = null;
      let cancelled = false;
      getRunLog(effectiveTaskId, runId)
        .then((text) => {
          if (cancelled) return;
          fileText = text;
        })
        .catch((e) => {
          if (cancelled) return;
          if (isRunLogNotFoundError(e)) {
            fileText = '';
            fileError = null;
          } else {
            fileError = e instanceof Error ? e.message : String(e);
          }
        });
      return () => { cancelled = true; };
    }

    // Live run — subscribe to agent:run-log. Filter by run_id so two
    // concurrent runs (M5+) don't interleave.
    fileText = '';
    fileError = null;
    const off = Events.On('agent:run-log', (ev: { data: unknown[] }) => {
      const p = ev?.data?.[0];
      if (!p || typeof p !== 'object') return;
      const payload = p as { run_id?: string; line?: string; stream?: string };
      if (payload.run_id !== runId) return;
      const text = stripAnsi(String(payload.line ?? ''));
      // Functional update so Svelte's reactivity proxy sees the new array.
      lines = [...lines, text];
      // Auto-scroll on the next tick once the DOM has the new line.
      queueMicrotask(scrollIfSticky);
    });
    return () => {
      try { off(); } catch { /* ignore */ }
    };
  });

  function scrollIfSticky() {
    if (!pre || !stickyBottom) return;
    pre.scrollTop = pre.scrollHeight;
  }

  function onScroll() {
    if (!pre) return;
    const atBottom = pre.scrollTop + pre.clientHeight >= pre.scrollHeight - 8;
    stickyBottom = atBottom;
  }

  onMount(() => {
    // First mount: scroll to bottom so newly opened drawers don't show the
    // top of a long static log.
    queueMicrotask(scrollIfSticky);
  });

  /** Strip ANSI CSI sequences from a single line. Cheap regex — agents may
   * emit coloured output and we don't want control codes in the DOM. */
  function stripAnsi(s: string): string {
    // ESC [ ... letter — common SGR sequences. Also handle OSC (\x1b ]...).
    return s.replace(/\x1b\[[0-9;?]*[A-Za-z]/g, '').replace(/\x1b\][^\x07]*\x07/g, '');
  }

  let displayedText = $derived(isLive ? lines.join('\n') : fileText);
  let statusLabel = $derived(run?.status ?? '');
</script>

<section class="run-log">
  <header class="run-log-head">
    <span class="label">Run log</span>
    {#if run}
      <span class={`pill pill-${statusLabel || 'idle'}`}>{statusLabel || 'idle'}</span>
      {#if run.exitCode !== 0 && (statusLabel === 'failed' || statusLabel === 'cancelled')}
        <span class="exit-code">exit {run.exitCode}</span>
      {/if}
    {/if}
  </header>

  {#if !runId}
    <p class="hint">No run selected.</p>
  {:else if fileError}
    <p class="err">{fileError}</p>
  {:else}
    <pre bind:this={pre} onscroll={onScroll}>{displayedText}</pre>
  {/if}
</section>

<style>
  .run-log {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-height: 0;
  }
  .run-log-head {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--fg-dim);
    font-weight: 600;
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
  .exit-code {
    font-size: 10px;
    color: var(--fg-dim);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  }
  pre {
    background: rgba(0, 0, 0, 0.30);
    color: var(--fg);
    border-radius: 5px;
    padding: 10px 12px;
    margin: 0;
    font-size: 11.5px;
    line-height: 1.45;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    max-height: 260px;
    min-height: 60px;
    overflow-y: auto;
    white-space: pre-wrap;
    word-break: break-word;
  }
  .hint {
    color: var(--fg-dim);
    font-size: 11px;
    margin: 4px 0 0;
  }
  .err {
    color: var(--p0);
    font-size: 11px;
    margin: 4px 0 0;
  }
</style>
