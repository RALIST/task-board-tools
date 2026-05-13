<script lang="ts">
  import { marked } from 'marked';
  import { Events } from '@wailsio/runtime';
  import { getTask, type Task, type TaskDetail } from '$lib/api';

  interface Props {
    taskId: string | null;
    onClose?: () => void;
  }

  let { taskId, onClose }: Props = $props();

  let detail = $state<TaskDetail | null>(null);
  let loading = $state(false);
  let err = $state<string | null>(null);

  // Fetch + subscribe to task:updated:<id> while the drawer is open. The
  // effect re-runs whenever taskId changes; the cleanup tears down the
  // previous subscription.
  $effect(() => {
    const id = taskId;
    if (!id) {
      detail = null;
      err = null;
      loading = false;
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
        })
        .catch((e) => {
          if (cancelled || taskId !== id) return;
          err = e instanceof Error ? e.message : String(e);
          loading = false;
        });
    };

    fetchOnce();
    const off = Events.On(`task:updated:${id}`, () => {
      // Re-fetch on every event; the body may have changed, not just
      // metadata. We don't show a spinner — replace silently to avoid
      // flicker.
      fetchOnce();
    });

    return () => {
      cancelled = true;
      try { off(); } catch { /* ignore */ }
    };
  });

  function onKeydown(ev: KeyboardEvent) {
    if (taskId && ev.key === 'Escape') {
      ev.preventDefault();
      onClose?.();
    }
  }

  function onBackdropClick(ev: MouseEvent) {
    if (ev.target === ev.currentTarget) onClose?.();
  }

  function meta(t: Task): Array<[string, string]> {
    const rows: Array<[string, string]> = [];
    if (t.type) rows.push(['Type', t.type]);
    if (t.priority) rows.push(['Priority', t.priority]);
    if (t.size) rows.push(['Size', t.size]);
    if (t.module) rows.push(['Module', t.module]);
    if (t.status) rows.push(['Status', t.status]);
    if (t.tags?.length) rows.push(['Tags', t.tags.join(', ')]);
    if (t.branch) rows.push(['Branch', t.branch]);
    if (t.parent) rows.push(['Parent', t.parent]);
    if (t.agent) rows.push(['Agent', t.agent + (t.agentStatus ? ` (${t.agentStatus})` : '')]);
    return rows;
  }

  // marked: we sanitize trust because the body content comes from the
  // user's own filesystem via tb. Disable mangle/headerIds (not needed for
  // a drawer view) and enable GFM (task lists, tables) for board-style
  // markdown.
  marked.setOptions({ gfm: true, breaks: false });

  function renderMarkdown(src: string): string {
    try {
      return marked.parse(src, { async: false }) as string;
    } catch {
      // Fall back to escaped pre — never crash the drawer over a parse error.
      return `<pre>${escapeHtml(src)}</pre>`;
    }
  }

  function escapeHtml(s: string): string {
    return s
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }

  // Strip the markdown header + metadata block (first ~15 lines) when
  // rendering the body — those duplicate the right rail.
  function stripFrontmatter(body: string): string {
    const lines = body.split('\n');
    // Find the first blank line after the metadata block (look ahead at
    // most 20 lines).
    let i = 0;
    for (; i < Math.min(lines.length, 20); i++) {
      const l = lines[i].trim();
      if (i > 0 && l === '') {
        // Look for the next non-metadata line.
        let j = i + 1;
        while (j < lines.length && lines[j].trim() === '') j++;
        if (j >= lines.length) break;
        const peek = lines[j].trim();
        if (peek.startsWith('## ') || peek.startsWith('# ')) {
          // The header line "# TB-N: ..." comes first; stop AFTER metadata,
          // before the first `## Goal` (or similar) section.
          if (peek.startsWith('# ')) continue;
          return lines.slice(j).join('\n');
        }
      }
    }
    return body;
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
        <button class="close" type="button" onclick={onClose} aria-label="Close">×</button>
      </header>

      {#if loading && !detail}
        <p class="hint">Loading…</p>
      {:else if err}
        <p class="err">{err}</p>
      {:else if detail}
        <dl class="meta">
          {#each meta(detail.metadata) as [k, v]}
            <dt>{k}</dt><dd>{v}</dd>
          {/each}
        </dl>
        <article class="body markdown">
          {@html renderMarkdown(stripFrontmatter(detail.body))}
        </article>
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
    width: min(640px, 95vw);
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
  .close:focus-visible { outline: 2px solid var(--accent); }

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

  .body {
    background: rgba(255, 255, 255, 0.03);
    padding: 12px 16px;
    border-radius: 6px;
    margin: 0;
    font-size: 13px;
    line-height: 1.6;
    overflow-x: auto;
  }
  /* Bare-minimum markdown styling — enough to read tasks, not enough to be
   * a content rendering library. M3 can iterate. */
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
  .markdown :global(ul),
  .markdown :global(ol) {
    margin: 0 0 8px;
    padding-left: 20px;
  }
  .markdown :global(li) { margin: 2px 0; }
  .markdown :global(li input[type="checkbox"]) {
    margin-right: 6px;
    pointer-events: none; /* read-only */
  }
  .markdown :global(code) {
    background: rgba(255, 255, 255, 0.06);
    padding: 1px 5px;
    border-radius: 3px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 12px;
  }
  .markdown :global(pre) {
    background: rgba(0, 0, 0, 0.25);
    padding: 10px 12px;
    border-radius: 4px;
    overflow-x: auto;
    margin: 8px 0;
  }
  .markdown :global(pre code) {
    background: none;
    padding: 0;
    border-radius: 0;
  }
  .markdown :global(a) { color: var(--accent); }
  .markdown :global(strong) { color: var(--fg); }
  .markdown :global(blockquote) {
    border-left: 3px solid rgba(255, 255, 255, 0.1);
    padding-left: 10px;
    margin: 8px 0;
    color: var(--fg-dim);
  }
  .hint { color: var(--fg-dim); font-size: 12px; }
  .err { color: var(--p0); font-size: 12px; }
</style>
