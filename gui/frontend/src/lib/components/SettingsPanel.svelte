<script lang="ts">
  import { Dialogs } from '@wailsio/runtime';
  import { get } from 'svelte/store';
  import { preferencesStore, type DefaultAgent, type PreferencesState } from '$lib/stores/preferences';
  import { pushToast } from '$lib/stores/toast';
  import {
    EnableClaudeUsageTap,
    DisableClaudeUsageTap,
    GetClaudeUsageTap,
  } from '../../../bindings/tools/tb-gui/app/settingsservice.js';
  import { refresh as refreshUsage } from '$lib/stores/usage';

  interface ClaudeUsageTapStatus {
    enabled: boolean;
    scriptPath: string;
    settingsPath: string;
    usagePath: string;
    reason?: string;
  }

  interface Props {
    open: boolean;
    onClose: () => void;
  }

  type EditablePreferences = Omit<PreferencesState, 'loaded'>;

  let { open, onClose }: Props = $props();

  let maxWorkersInput = $state('1');
  let agentTimeoutInput = $state('30');
  let defaultAgentInput = $state<DefaultAgent>('none');
  let cliPathInput = $state('');
  let periodicRecoveryInput = $state(true);
  let autoGroomInput = $state(false);
  let autoGroomSettleInput = $state('5');
  let autoImplementInput = $state(false);
  let autoImplementQueryInput = $state('');
  let autoImplementQueryError = $state('');
  // Plain `let` (not $state) so the increment-in-effect doesn't
  // self-trigger the effect via Svelte's reactive dependency tracking.
  let autoImplementQueryValidateToken = 0;
  let saving = $state(false);
  let opened = $state(false);
  let seededLoaded = $state(false);
  let tapStatus = $state<ClaudeUsageTapStatus | null>(null);
  let tapBusy = $state(false);
  let baseline = $state<EditablePreferences>({
    maxWorkers: 1,
    agentTimeoutMinutes: 30,
    defaultAgent: 'none',
    cliPath: '',
    periodicRecoveryEnabled: true,
    autoGroomEnabled: false,
    autoGroomSettleMinutes: 5,
    autoImplementEnabled: false,
    autoImplementQuery: '',
  });

  let nextMaxWorkers = $derived(clampNumber(maxWorkersInput, 1, 4, baseline.maxWorkers));
  let nextAgentTimeout = $derived(
    clampNumber(agentTimeoutInput, 1, 240, baseline.agentTimeoutMinutes),
  );
  let nextCLIPath = $derived(cliPathInput.trim());
  let nextAutoGroomSettle = $derived(
    clampNumber(autoGroomSettleInput, 0, 60, baseline.autoGroomSettleMinutes),
  );
  let nextAutoImplementQuery = $derived(autoImplementQueryInput.trim());
  let dirty = $derived(
    nextMaxWorkers !== baseline.maxWorkers ||
      nextAgentTimeout !== baseline.agentTimeoutMinutes ||
      defaultAgentInput !== baseline.defaultAgent ||
      nextCLIPath !== baseline.cliPath ||
      periodicRecoveryInput !== baseline.periodicRecoveryEnabled ||
      autoGroomInput !== baseline.autoGroomEnabled ||
      nextAutoGroomSettle !== baseline.autoGroomSettleMinutes ||
      autoImplementInput !== baseline.autoImplementEnabled ||
      nextAutoImplementQuery !== baseline.autoImplementQuery,
  );
  let autoGroomNeedsDefaultAgent = $derived(
    autoGroomInput && defaultAgentInput === 'none',
  );
  let autoImplementNeedsDefaultAgent = $derived(
    autoImplementInput && defaultAgentInput === 'none',
  );
  let autoImplementNeedsQuery = $derived(
    autoImplementInput && nextAutoImplementQuery === '',
  );

  // Debounced parser validation through the backend. Triggered on each
  // query input change; result lands in autoImplementQueryError. The
  // request token guards against out-of-order responses (a fresh edit
  // landing after a stale validation resolves).
  $effect(() => {
    const query = nextAutoImplementQuery;
    if (query === '') {
      autoImplementQueryError = '';
      return;
    }
    autoImplementQueryValidateToken += 1;
    const token = autoImplementQueryValidateToken;
    const handle = setTimeout(async () => {
      const err = await preferencesStore.validateAutoImplementQuery(query);
      if (token !== autoImplementQueryValidateToken) return;
      autoImplementQueryError = err ?? '';
    }, 250);
    return () => clearTimeout(handle);
  });

  $effect(() => {
    const prefs = $preferencesStore;
    if (!open) {
      opened = false;
      seededLoaded = false;
      return;
    }
    if (!prefs.loaded) {
      void preferencesStore.load().catch(() => {});
    }
    if (!opened || (prefs.loaded && !seededLoaded)) {
      resetFromPreferences(prefs);
      opened = true;
      seededLoaded = prefs.loaded;
      void refreshTapStatus();
    }
  });

  async function refreshTapStatus() {
    try {
      tapStatus = (await GetClaudeUsageTap()) as ClaudeUsageTapStatus;
    } catch (err) {
      pushToast(`Could not read claude tap status: ${errorMessage(err)}`);
      tapStatus = null;
    }
  }

  async function toggleTap() {
    if (tapBusy || tapStatus == null) return;
    tapBusy = true;
    try {
      const next = tapStatus.enabled
        ? ((await DisableClaudeUsageTap()) as ClaudeUsageTapStatus)
        : ((await EnableClaudeUsageTap()) as ClaudeUsageTapStatus);
      tapStatus = next;
      void refreshUsage();
      pushToast(
        next.enabled
          ? 'Claude usage tap enabled — run claude once to populate data'
          : 'Claude usage tap disabled',
        'success',
      );
    } catch (err) {
      pushToast(`Tap toggle failed: ${errorMessage(err)}`);
    } finally {
      tapBusy = false;
    }
  }

  function errorMessage(err: unknown): string {
    if (err instanceof Error) return err.message;
    return String(err);
  }

  async function save() {
    if (!dirty || saving) return;
    snapMaxWorkers();
    snapAgentTimeout();
    snapAutoGroomSettle();

    const failures: string[] = [];
    saving = true;
    try {
      if (nextMaxWorkers !== baseline.maxWorkers) {
        try {
          await preferencesStore.setMaxWorkers(nextMaxWorkers);
        } catch {
          failures.push('max workers');
        }
      }
      if (nextAgentTimeout !== baseline.agentTimeoutMinutes) {
        try {
          await preferencesStore.setAgentTimeoutMinutes(nextAgentTimeout);
        } catch {
          failures.push('agent timeout');
        }
      }
      if (defaultAgentInput !== baseline.defaultAgent) {
        try {
          await preferencesStore.setDefaultAgent(defaultAgentInput);
        } catch {
          failures.push('default agent');
        }
      }
      if (nextCLIPath !== baseline.cliPath) {
        try {
          await preferencesStore.setCLIPath(nextCLIPath);
        } catch {
          failures.push('CLI path');
        }
      }
      if (periodicRecoveryInput !== baseline.periodicRecoveryEnabled) {
        try {
          await preferencesStore.setPeriodicRecoveryEnabled(periodicRecoveryInput);
        } catch {
          failures.push('periodic recovery');
        }
      }
      if (autoGroomInput !== baseline.autoGroomEnabled) {
        try {
          await preferencesStore.setAutoGroomEnabled(autoGroomInput);
        } catch {
          failures.push('auto-groom');
        }
      }
      if (nextAutoGroomSettle !== baseline.autoGroomSettleMinutes) {
        try {
          await preferencesStore.setAutoGroomSettleMinutes(nextAutoGroomSettle);
        } catch {
          failures.push('auto-groom settle window');
        }
      }
      // Auto-implement: persist the query first (a freshly typed query
      // must be on disk before the enable validator reads it), then the
      // enable toggle. Validation errors land in `failures` so the
      // toast still surfaces them and the baseline doesn't drift.
      if (nextAutoImplementQuery !== baseline.autoImplementQuery) {
        try {
          await preferencesStore.setAutoImplementQuery(nextAutoImplementQuery);
        } catch {
          failures.push('auto-implement query');
        }
      }
      if (autoImplementInput !== baseline.autoImplementEnabled) {
        try {
          await preferencesStore.setAutoImplementEnabled(autoImplementInput);
        } catch {
          failures.push('auto-implement');
        }
      }

      const current = get(preferencesStore);
      baseline = toEditable(current);
      if (failures.length === 0) {
        resetFromPreferences(current);
        pushToast('Settings saved', 'success');
      }
    } finally {
      saving = false;
    }
  }

  async function browseCLIPath() {
    try {
      const picked = await Dialogs.OpenFile({
        Title: 'Choose tb binary',
        Message: 'Choose the tb CLI binary',
        ButtonText: 'Choose',
        CanChooseFiles: true,
        CanChooseDirectories: false,
        AllowsMultipleSelection: false,
      });
      const path = Array.isArray(picked) ? picked[0] : picked;
      if (path) cliPathInput = path;
    } catch (err) {
      pushToast(`File picker failed: ${err instanceof Error ? err.message : String(err)}`);
    }
  }

  function resetFromPreferences(prefs: PreferencesState) {
    baseline = toEditable(prefs);
    maxWorkersInput = String(baseline.maxWorkers);
    agentTimeoutInput = String(baseline.agentTimeoutMinutes);
    defaultAgentInput = baseline.defaultAgent;
    cliPathInput = baseline.cliPath;
    periodicRecoveryInput = baseline.periodicRecoveryEnabled;
    autoGroomInput = baseline.autoGroomEnabled;
    autoGroomSettleInput = String(baseline.autoGroomSettleMinutes);
    autoImplementInput = baseline.autoImplementEnabled;
    autoImplementQueryInput = baseline.autoImplementQuery;
    autoImplementQueryError = '';
  }

  function toEditable(prefs: PreferencesState): EditablePreferences {
    return {
      maxWorkers: prefs.maxWorkers,
      agentTimeoutMinutes: prefs.agentTimeoutMinutes,
      defaultAgent: prefs.defaultAgent,
      cliPath: prefs.cliPath,
      periodicRecoveryEnabled: prefs.periodicRecoveryEnabled,
      autoGroomEnabled: prefs.autoGroomEnabled,
      autoGroomSettleMinutes: prefs.autoGroomSettleMinutes,
      autoImplementEnabled: prefs.autoImplementEnabled,
      autoImplementQuery: prefs.autoImplementQuery,
    };
  }

  function snapMaxWorkers() {
    maxWorkersInput = String(nextMaxWorkers);
  }

  function snapAgentTimeout() {
    agentTimeoutInput = String(nextAgentTimeout);
  }

  function snapAutoGroomSettle() {
    autoGroomSettleInput = String(nextAutoGroomSettle);
  }

  function clampNumber(raw: string, min: number, max: number, fallback: number): number {
    const n = Number(raw);
    if (!Number.isFinite(n)) return fallback;
    if (n < min) return min;
    if (n > max) return max;
    return Math.trunc(n);
  }

  function onBackdropClick(ev: MouseEvent) {
    if (ev.target === ev.currentTarget) onClose();
  }
</script>

{#if open}
  <div
    class="backdrop"
    role="dialog"
    aria-modal="true"
    aria-label="Settings"
    tabindex="-1"
    onclick={onBackdropClick}
    onkeydown={() => {}}>
    <aside class="panel">
      <header>
        <div>
          <span class="eyebrow">Preferences</span>
          <h2>Settings</h2>
        </div>
        <button class="close" type="button" onclick={onClose} aria-label="Close">×</button>
      </header>

      <section class="form">
        <label class="field">
          <span>Max workers</span>
          <input
            type="number"
            min="1"
            max="4"
            step="1"
            bind:value={maxWorkersInput}
            onblur={snapMaxWorkers} />
          <small>1-4</small>
        </label>

        <label class="field">
          <span>Agent timeout</span>
          <input
            type="number"
            min="1"
            max="240"
            step="1"
            bind:value={agentTimeoutInput}
            onblur={snapAgentTimeout} />
          <small>minutes, 1-240</small>
        </label>

        <label class="field">
          <span>Default agent</span>
          <select bind:value={defaultAgentInput}>
            <option value="none">none</option>
            <option value="claude">claude</option>
            <option value="codex">codex</option>
          </select>
        </label>

        <label class="field cli-path">
          <span>CLI path</span>
          <div class="path-row">
            <input bind:value={cliPathInput} placeholder="PATH lookup" />
            <button class="secondary" type="button" onclick={browseCLIPath}>Browse…</button>
          </div>
        </label>

        <div class="field tap">
          <span>Claude usage tap</span>
          <div class="tap-row">
            <button
              class="secondary"
              type="button"
              disabled={tapBusy || tapStatus == null}
              onclick={toggleTap}>
              {#if tapStatus?.enabled}
                Disable tap
              {:else}
                Enable tap
              {/if}
            </button>
            <small class="tap-status">
              {#if tapStatus == null}
                checking…
              {:else if tapStatus.enabled}
                installed at {tapStatus.scriptPath}
              {:else}
                {tapStatus.reason || 'not installed'}
              {/if}
            </small>
          </div>
          <small class="tap-help">
            Installs a project-local statusline hook in <code>.claude/settings.local.json</code>
            so the header can read claude's <code>/usage</code> data. Run <code>claude</code> once
            after enabling to populate the value.
          </small>
        </div>

        <label class="field checkbox-field">
          <span>Periodic recovery</span>
          <input
            type="checkbox"
            bind:checked={periodicRecoveryInput} />
          <small>Reconcile stale running agent rows while the app stays open.</small>
        </label>

        <label class="field checkbox-field">
          <span>Enable auto groom</span>
          <input
            type="checkbox"
            bind:checked={autoGroomInput} />
          <small>
            When on, the GUI starts a grooming run for backlog tasks that need triage,
            after a settle window. Requires a default agent.
          </small>
        </label>

        <label class="field" class:disabled-field={!autoGroomInput}>
          <span>Auto-groom settle window</span>
          <input
            type="number"
            min="0"
            max="60"
            step="1"
            disabled={!autoGroomInput}
            bind:value={autoGroomSettleInput}
            onblur={snapAutoGroomSettle} />
          <small>
            Minutes auto-groom waits after a task is created or edited so attachments and
            follow-up notes can land. 0 = no delay.
          </small>
        </label>

        {#if autoGroomNeedsDefaultAgent}
          <p class="inline-warning" role="alert">
            Set a default agent before automation can run.
          </p>
        {/if}

        <label class="field checkbox-field">
          <span>Enable auto implement</span>
          <input
            type="checkbox"
            data-testid="auto-implement-toggle"
            bind:checked={autoImplementInput} />
          <small>
            When on, the GUI pulls matching <code>ready</code> tasks into in-progress and
            starts implementation runs. Requires a default agent and a non-empty filter.
          </small>
        </label>

        <label class="field" class:disabled-field={!autoImplementInput}>
          <span>Auto-implement filter</span>
          <input
            type="text"
            spellcheck="false"
            placeholder="bug, S size, gui"
            data-testid="auto-implement-query"
            disabled={!autoImplementInput}
            bind:value={autoImplementQueryInput} />
          <small>
            Comma-separated terms: type, priority (P0–P3), size, module, tag:name,
            agent:claude, parent:TB-N. Bare words match the task title or module.
          </small>
        </label>

        {#if autoImplementNeedsDefaultAgent}
          <p class="inline-warning" role="alert">
            Set a default agent before auto-implement can run.
          </p>
        {/if}
        {#if autoImplementNeedsQuery}
          <p class="inline-warning" role="alert">
            Auto-implement needs a non-empty filter to know which tasks to pick.
          </p>
        {/if}
        {#if autoImplementQueryError && !autoImplementNeedsQuery}
          <p class="inline-warning" role="alert">
            Filter is invalid: {autoImplementQueryError}
          </p>
        {/if}
      </section>

      <footer>
        <button class="ghost" type="button" onclick={onClose}>Cancel</button>
        <button class="primary" type="button" disabled={!dirty || saving} onclick={save}>
          {saving ? 'Saving…' : 'Save'}
        </button>
      </footer>
    </aside>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 55;
    display: flex;
    justify-content: flex-end;
    background: rgba(0, 0, 0, 0.45);
  }
  .panel {
    width: min(440px, 96vw);
    height: 100%;
    box-sizing: border-box;
    padding: 20px 22px;
    overflow-y: auto;
    background: var(--bg-elev);
    border-left: 1px solid rgba(255, 255, 255, 0.06);
    box-shadow: -8px 0 32px rgba(0, 0, 0, 0.45);
    display: flex;
    flex-direction: column;
    gap: 18px;
  }
  :global(html.platform-mac) .panel {
    padding-top: calc(20px + var(--mac-titlebar-height));
  }
  header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }
  .eyebrow {
    display: inline-block;
    margin-bottom: 4px;
    color: var(--fg-dim);
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  h2 {
    margin: 0;
    font-size: 18px;
    line-height: 1.3;
    font-weight: 600;
  }
  .close {
    background: none;
    border: 0;
    border-radius: 4px;
    color: var(--fg-dim);
    cursor: pointer;
    font-size: 22px;
    line-height: 1;
    padding: 4px 8px;
  }
  .close:hover { background: rgba(255, 255, 255, 0.06); color: var(--fg); }
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 12px;
  }
  .field span,
  .field small {
    color: var(--fg-dim);
  }
  .field small { font-size: 11px; }
  .field input,
  .field select {
    box-sizing: border-box;
    width: 100%;
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--fg);
    border-radius: 5px;
    padding: 7px 9px;
    font: inherit;
  }
  .field input:focus,
  .field select:focus {
    outline: 2px solid rgba(74, 141, 248, 0.45);
    outline-offset: 1px;
  }
  .checkbox-field input {
    width: auto;
    align-self: flex-start;
  }
  .disabled-field {
    opacity: 0.55;
  }
  .inline-warning {
    margin: -2px 0 0;
    padding: 8px 10px;
    border: 1px solid rgba(244, 191, 79, 0.45);
    border-radius: 5px;
    background: rgba(244, 191, 79, 0.08);
    color: #f4bf4f;
    font-size: 11.5px;
    line-height: 1.4;
  }
  .path-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px;
  }
  .tap-row {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .tap-status {
    color: var(--fg-dim);
    font-size: 11px;
    word-break: break-all;
  }
  .tap-help {
    color: var(--fg-dim);
    font-size: 11px;
    line-height: 1.5;
  }
  .tap-help code {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 10.5px;
    background: rgba(255, 255, 255, 0.05);
    padding: 1px 4px;
    border-radius: 3px;
  }
  footer {
    margin-top: auto;
    padding-top: 14px;
    border-top: 1px solid rgba(255, 255, 255, 0.06);
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
  .primary,
  .secondary,
  .ghost {
    border-radius: 5px;
    cursor: pointer;
    font: inherit;
    font-size: 12px;
    padding: 6px 14px;
  }
  .primary {
    background: var(--accent);
    border: 0;
    color: white;
    font-weight: 600;
  }
  .primary:disabled { opacity: 0.4; cursor: not-allowed; }
  .secondary,
  .ghost {
    background: rgba(255, 255, 255, 0.06);
    border: 1px solid rgba(255, 255, 255, 0.12);
    color: var(--fg);
  }
  .ghost {
    background: transparent;
  }
  .secondary:hover,
  .ghost:hover {
    background: rgba(255, 255, 255, 0.1);
  }
</style>
