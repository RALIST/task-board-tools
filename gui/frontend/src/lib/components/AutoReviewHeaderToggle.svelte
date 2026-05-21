<script lang="ts">
  import { preferencesStore } from '$lib/stores/preferences';

  interface Props {
    onOpenSettings: () => void;
  }

  let { onOpenSettings }: Props = $props();

  let enabled = $derived($preferencesStore.autoReviewEnabled === true);
  let missingDefaultAgent = $derived($preferencesStore.defaultAgent === 'none');
  let busy = $state(false);

  async function toggle() {
    if (busy) return;
    if (missingDefaultAgent && !enabled) {
      onOpenSettings();
      return;
    }
    busy = true;
    try {
      await preferencesStore.setAutoReviewEnabled(!enabled);
    } catch {
      // Store rollback and toast already happen in preferencesStore.
    } finally {
      busy = false;
    }
  }
</script>

<button
  type="button"
  class="auto-review-toggle"
  class:on={enabled}
  class:needs-default={missingDefaultAgent && !enabled}
  aria-pressed={enabled}
  disabled={busy}
  data-testid="auto-review-pill"
  title={missingDefaultAgent && !enabled
    ? 'Auto-review needs a default agent. Click to open Settings.'
    : enabled
      ? 'Auto-review is on. Click to disable.'
      : 'Auto-review is off. Click to enable.'}
  onclick={toggle}>
  <span class="dot" aria-hidden="true"></span>
  Auto-review
</button>

<style>
  .auto-review-toggle {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.12);
    color: var(--fg-dim);
    border-radius: 999px;
    padding: 4px 11px;
    font-size: 11.5px;
    line-height: 1.2;
    letter-spacing: 0.01em;
    cursor: pointer;
    white-space: nowrap;
    transition: background 120ms ease, color 120ms ease, border-color 120ms ease;
  }
  .auto-review-toggle:hover { background: rgba(255, 255, 255, 0.11); }
  .auto-review-toggle:disabled { opacity: 0.55; cursor: progress; }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: rgba(255, 255, 255, 0.2);
    box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.25) inset;
  }
  .auto-review-toggle.on {
    background: rgba(74, 141, 248, 0.18);
    border-color: rgba(74, 141, 248, 0.55);
    color: var(--fg);
  }
  .auto-review-toggle.on .dot {
    background: var(--accent);
    box-shadow: 0 0 0 2px rgba(74, 141, 248, 0.25);
  }
  .auto-review-toggle.needs-default {
    background: rgba(244, 191, 79, 0.13);
    border-color: rgba(244, 191, 79, 0.55);
    color: #f4bf4f;
  }
  .auto-review-toggle.needs-default .dot {
    background: #f4bf4f;
    box-shadow: 0 0 0 2px rgba(244, 191, 79, 0.25);
  }
</style>
