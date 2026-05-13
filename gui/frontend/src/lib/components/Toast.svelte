<script lang="ts">
  import { dismissToast, toasts } from '$lib/stores/toast';
</script>

<aside class="toasts" aria-live="polite">
  {#each $toasts as t (t.id)}
    <button
      class="toast"
      class:err={t.kind === 'error'}
      class:ok={t.kind === 'success'}
      class:info={t.kind === 'info'}
      type="button"
      onclick={() => dismissToast(t.id)}
      aria-label={`Dismiss ${t.kind} toast`}>
      {t.message}
    </button>
  {/each}
</aside>

<style>
  .toasts {
    position: fixed;
    bottom: 16px;
    right: 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 100;
    pointer-events: none;
  }
  .toast {
    pointer-events: auto;
    background: var(--bg-elev);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--fg);
    padding: 8px 12px;
    border-radius: 6px;
    max-width: 380px;
    text-align: left;
    cursor: pointer;
    font: inherit;
    font-size: 12px;
    box-shadow: 0 6px 22px rgba(0, 0, 0, 0.4);
  }
  .toast:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
  .toast.err { border-color: rgba(255, 90, 82, 0.4); }
  .toast.ok { border-color: rgba(80, 200, 120, 0.4); }
  .toast.info { border-color: rgba(74, 141, 248, 0.4); }
</style>
