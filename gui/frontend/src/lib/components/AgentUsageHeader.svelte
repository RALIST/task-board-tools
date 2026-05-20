<script lang="ts">
  // Header widget for per-agent global quota (TB-107).
  //
  // Renders one chip per agent surfaced by UsageService.GetAgentUsage.
  // When an agent reports Available=false the chip says "unknown" and the
  // tooltip explains why; the rest of the header keeps working.
  //
  // The refresh button calls UsageService.RefreshAgentUsage; while in
  // flight the button is disabled and shows an ellipsis so the user knows
  // the action took effect.

  import { usageStore, refresh, type AgentUsage } from '$lib/stores/usage';

  let refreshing = $state(false);
  let lastError = $state('');

  async function onRefreshClick() {
    if (refreshing) return;
    refreshing = true;
    lastError = '';
    try {
      await refresh();
    } catch (err) {
      lastError = err instanceof Error ? err.message : String(err);
    } finally {
      refreshing = false;
    }
  }

  function formatPercent(pct: number | null | undefined): string {
    if (pct == null || !Number.isFinite(pct)) return '—';
    if (pct >= 99.95) return '100%';
    return `${pct.toFixed(pct < 10 ? 1 : 0)}%`;
  }

  function severity(pct: number | null | undefined): 'ok' | 'warn' | 'high' | 'none' {
    if (pct == null || !Number.isFinite(pct)) return 'none';
    if (pct >= 80) return 'high';
    if (pct >= 50) return 'warn';
    return 'ok';
  }

  function chipTitle(u: AgentUsage): string {
    const lines: string[] = [`${u.agent}`];
    if (u.plan) lines.push(`plan: ${u.plan}`);
    if (u.primary?.usedPercent != null) {
      const lbl = u.primary.windowLabel || 'short';
      lines.push(`${lbl}: ${formatPercent(u.primary.usedPercent)}`);
    }
    if (u.secondary?.usedPercent != null) {
      const lbl = u.secondary.windowLabel || 'long';
      lines.push(`${lbl}: ${formatPercent(u.secondary.usedPercent)}`);
    }
    if (!u.available && u.reason) lines.push(u.reason);
    if (u.lastUpdated) lines.push(`updated: ${u.lastUpdated}`);
    return lines.join('\n');
  }

  function dominantSeverity(u: AgentUsage): 'ok' | 'warn' | 'high' | 'none' {
    if (!u.available) return 'none';
    const sevPrimary = severity(u.primary?.usedPercent);
    const sevSecondary = severity(u.secondary?.usedPercent);
    if (sevPrimary === 'high' || sevSecondary === 'high') return 'high';
    if (sevPrimary === 'warn' || sevSecondary === 'warn') return 'warn';
    if (sevPrimary === 'ok' || sevSecondary === 'ok') return 'ok';
    return 'none';
  }
</script>

<div class="usage" aria-label="Agent quota usage">
  {#each $usageStore as u (u.agent)}
    {@const sev = dominantSeverity(u)}
    <span class="chip sev-{sev}" class:unavailable={!u.available} title={chipTitle(u)} aria-label={chipTitle(u)}>
      <span class="name">{u.agent}</span>
      {#if u.available}
        {#if u.primary?.usedPercent != null}
          <span class="pct">{formatPercent(u.primary.usedPercent)}</span>
        {/if}
        {#if u.secondary?.usedPercent != null}
          <span class="sep" aria-hidden="true">·</span>
          <span class="pct secondary">{formatPercent(u.secondary.usedPercent)}</span>
        {/if}
      {:else}
        <span class="pct unknown">unknown</span>
      {/if}
    </span>
  {/each}
  <button
    class="refresh"
    type="button"
    onclick={onRefreshClick}
    disabled={refreshing}
    title={lastError || 'Refresh agent quota'}
    aria-label="Refresh agent quota">
    {refreshing ? '…' : '↻'}
  </button>
</div>

<style>
  .usage {
    display: flex;
    align-items: center;
    gap: 6px;
    -webkit-app-region: no-drag;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 3px 8px;
    border-radius: 999px;
    background: rgba(255, 255, 255, 0.06);
    border: 1px solid rgba(255, 255, 255, 0.1);
    font-size: 11px;
    line-height: 1.4;
    color: var(--fg);
    white-space: nowrap;
  }
  .chip.unavailable {
    color: var(--fg-dim);
    border-style: dashed;
  }
  .chip .name {
    font-weight: 600;
    text-transform: lowercase;
    letter-spacing: 0.02em;
  }
  .chip .pct {
    font-variant-numeric: tabular-nums;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  }
  .chip .pct.secondary {
    color: var(--fg-dim);
  }
  .chip .sep {
    color: var(--fg-dim);
    opacity: 0.7;
  }
  .chip .pct.unknown {
    font-style: italic;
    color: var(--fg-dim);
    font-variant-numeric: tabular-nums;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  }
  .chip.sev-warn {
    border-color: rgba(255, 184, 108, 0.6);
    background: rgba(255, 184, 108, 0.12);
  }
  .chip.sev-high {
    border-color: rgba(255, 90, 82, 0.7);
    background: rgba(255, 90, 82, 0.15);
    color: #fff;
  }
  .refresh {
    background: rgba(255, 255, 255, 0.06);
    border: 1px solid rgba(255, 255, 255, 0.12);
    color: var(--fg);
    border-radius: 999px;
    width: 22px;
    height: 22px;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    line-height: 1;
    padding: 0;
  }
  .refresh:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.12);
  }
  .refresh:disabled {
    cursor: progress;
    opacity: 0.6;
  }
</style>
