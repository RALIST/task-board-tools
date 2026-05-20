<script lang="ts">
  import { untrack } from 'svelte';
  import { marked } from 'marked';
  import DOMPurify from 'dompurify';
  import { Events } from '@wailsio/runtime';
  import {
    addAttachments,
    assignAgent,
    cancelRun,
    closeTask as apiCloseTask,
    editTask,
    editTaskBody,
    errorString,
    getTask,
    groomTask,
    listAttachments,
    listRuns,
    openAttachment,
    pickAttachmentFiles,
    removeAttachments,
    renameTask,
    resumeAgent,
    reviewTask,
    runAgent,
    submitReview,
    type AgentName,
    type Attachment,
    type EditTaskInput,
    type TaskDetail,
  } from '$lib/api';
  import { pushToast } from '$lib/stores/toast';
  import {
    deriveEffectiveRunStatus,
    runsForTask,
    selectedRunID,
    setRunsForTask,
    upsertRun,
    type Run,
  } from '$lib/stores/runs';
  import { consumeGroomSuggestion, groomSuggestedFor } from '$lib/stores/groomSuggestion';
  import { defaultAgent as defaultAgentPreference, preferencesStore } from '$lib/stores/preferences';
  import { autoGroomStore } from '$lib/stores/autoGroom';
  import { triageForTask } from '$lib/stores/triage';
  import { board } from '$lib/stores/board';
  import { epicProgress } from '$lib/filtering';
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
  // TB-235: ReviewRef is the branch/PR/commit/worktree reviewers inspect.
  // Required to enter code-review; surfaced as an editable input next to
  // Module/Tags so the autosave path persists it identically.
  let formReviewRef = $state('');

  // Autosave state machine for metadata.
  //
  // - `userTouchedMetadata` flips on the first user keystroke after a load
  //   or successful save and gates the watcher refresh from clobbering an
  //   in-progress draft. We don't use `metadataDirty` for this because an
  //   external CLI edit can make dirty true without user input — and in
  //   that case we *do* want to absorb the new disk values.
  // - `metaSaveTimer` is the debounce handle; cleared on every form change.
  // - `metaSaving` is true while a CLI `tb edit` is in flight.
  // - `metaPendingResave` coalesces edits that arrive during an in-flight
  //   save: the finally-hook re-runs save once the inner call resolves.
  // - `metaSaveError` carries the last failure for the inline retry chip;
  //   cleared on next successful save or task switch.
  // - `metaRecentlySaved` is a transient flag for the "Saved" indicator.
  const AUTOSAVE_DEBOUNCE_MS = 600;
  const SAVED_INDICATOR_MS = 1800;
  let userTouchedMetadata = $state(false);
  let metaSaveTimer: ReturnType<typeof setTimeout> | null = null;
  let metaSaving = $state(false);
  let metaPendingResave = false;
  let metaSaveError = $state<string | null>(null);
  let metaRecentlySaved = $state(false);
  let metaSavedFadeTimer: ReturnType<typeof setTimeout> | null = null;
  // Latched after a successful backend save; converted into
  // `metaRecentlySaved` once metadataDirty drops back to false (which
  // is when the watcher refresh has caught up). Per AC #4 we must
  // not flash "Saved" before disk + watcher reconcile.
  let metaPendingSavedIndicator = $state(false);

  // Editor state.
  let editMode = $state(false);
  let bodyDraft = $state(''); // current editor contents (full file, header included)
  let bodyDirty = $state(false);
  // Body autosave mirrors the metadata state machine. See above.
  let bodySaveTimer: ReturnType<typeof setTimeout> | null = null;
  let bodySaving = $state(false);
  let bodyPendingResave = false;
  let bodySaveError = $state<string | null>(null);
  let bodyRecentlySaved = $state(false);
  let bodySavedFadeTimer: ReturnType<typeof setTimeout> | null = null;
  let bodyPendingSavedIndicator = $state(false);

  // Archive button two-step confirm.
  let archivePrompt = $state(false);
  let archiveTimer: ReturnType<typeof setTimeout> | null = null;

  // Cancel-run two-step confirm (same UX pattern as Archive).
  let cancelPrompt = $state(false);
  let cancelTimer: ReturnType<typeof setTimeout> | null = null;

  let agentSaving = $state(false);
  // True after the user explicitly picked "(none)" on the current task. We
  // need this to suppress the default-agent fallback in `displayedAgent` —
  // otherwise clearing the agent silently snaps back to the configured
  // default. Resets on every taskId change.
  let userClearedAgent = $state(false);
  let runStarting = $state(false);
  let groomStarting = $state(false);
  let reviewStarting = $state(false);
  let submittingReview = $state(false);
  let resumeStarting = $state(false);
  let clearingNeedsUser = $state(false);
  let groomReasons = $state<string[]>([]);
  let groomHighlight = $state(false);
  let groomHighlightTimer: ReturnType<typeof setTimeout> | null = null;

  // Past-run subscription. runsStore is keyed by run_id; we project the
  // task's slice via runsForTask. Hydrated on every taskId change.
  let runs: Run[] = $state([]);
  let runsUnsub: (() => void) | null = null;

  // Attachment list state. Hydrated on taskId change and refreshed via the
  // watcher's board:reloaded event after add/remove. The drawer only ever
  // *reads* attachments directly; mutations go through `tb attach` via
  // BoardService.
  let attachments = $state<Attachment[]>([]);
  let attachmentsLoading = $state(false);
  // Monotonic request token for refreshAttachments: defends against a
  // newer load (e.g. board:reloaded after a task switch) being overtaken
  // by a stale older promise resolving second.
  let attachmentsReqSeq = 0;
  let attachmentsBusy = $state(false);
  // Two-click confirm state for attachment removal. Declared up here so the
  // task-switch $effect can reset it cleanly — otherwise an armed row on
  // task A whose name matches an attachment on task B would let a single
  // click on B's × bypass the confirm.
  let attachmentRemovePending = $state<string | null>(null);
  let attachmentRemoveTimer: ReturnType<typeof setTimeout> | null = null;

  // Inline title rename state. Reset on task switch (below) so an in-flight
  // draft never leaks across tasks.
  let renaming = $state(false);
  let renameDraft = $state('');
  let renameSaving = $state(false);
  let renameInput = $state<HTMLInputElement | null>(null);

  $effect(() => {
    const id = taskId;
    userClearedAgent = false;
    // Reset the autosave state machine for the new task. The actual flush
    // of any pending save from the *previous* task happens in this effect's
    // teardown function below — putting it inside the tracked body would
    // make the effect subscribe to `detail` / `metadataDirty` / `bodyDirty`
    // (read via the flush helpers) and loop indefinitely.
    if (metaSaveTimer) { clearTimeout(metaSaveTimer); metaSaveTimer = null; }
    if (metaSavedFadeTimer) { clearTimeout(metaSavedFadeTimer); metaSavedFadeTimer = null; }
    if (bodySaveTimer) { clearTimeout(bodySaveTimer); bodySaveTimer = null; }
    if (bodySavedFadeTimer) { clearTimeout(bodySavedFadeTimer); bodySavedFadeTimer = null; }
    metaSaving = false;
    metaPendingResave = false;
    metaSaveError = null;
    metaRecentlySaved = false;
    metaPendingSavedIndicator = false;
    userTouchedMetadata = false;
    bodySaving = false;
    bodyPendingResave = false;
    bodySaveError = null;
    bodyRecentlySaved = false;
    bodyPendingSavedIndicator = false;
    // Disarm any in-flight remove confirmation from the previous task —
    // otherwise switching to a new task whose attachment shares a name with
    // the armed row would let a single click bypass the two-click confirm
    // and silently remove the wrong task's attachment.
    if (attachmentRemoveTimer) { clearTimeout(attachmentRemoveTimer); attachmentRemoveTimer = null; }
    attachmentRemovePending = null;
    // Cancel any in-flight rename when switching tasks.
    renaming = false;
    renameDraft = '';
    renameSaving = false;
    if (!id) {
      detail = null; err = null; loading = false;
      editMode = false; bodyDirty = false; archivePrompt = false;
      cancelPrompt = false; runs = [];
      attachments = []; attachmentsLoading = false; attachmentsBusy = false;
      runsUnsub?.(); runsUnsub = null;
      return;
    }
    loading = true;
    err = null;
    // Clear stale attachment rows from the previous task before requesting
    // fresh data — otherwise the "no rows yet" spinner gate stays hidden
    // because old rows are still in `attachments`, and the user sees the
    // previous task's attachments while the new task loads.
    attachments = [];
    let cancelled = false;

    const fetchOnce = () => {
      getTask(id)
        .then((d) => {
          if (cancelled || taskId !== id) return;
          detail = d;
          loading = false;
          // Reset form to the freshly-loaded values UNLESS the user is
          // mid-edit (touched a field since the last save). Without this
          // guard, the watcher's board:reloaded that follows our own
          // autosave races user keystrokes and silently clobbers them.
          // The flag clears on task switch and after a successful save.
          if (!userTouchedMetadata) {
            formPriority = d.metadata.priority ?? '';
            formType = d.metadata.type ?? '';
            formSize = d.metadata.size ?? '';
            formModule = d.metadata.module ?? '';
            formTags = (d.metadata.tags ?? []).join(', ');
            formReviewRef = d.metadata.reviewRef ?? '';
          }
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

    // Hydrate attachments. Refreshes inside fetchOnce via board:reloaded
    // events when the user attaches or removes files.
    refreshAttachments(id, cancelled);

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
    const offTask = Events.On(`task:updated:${id}`, () => {
      fetchOnce();
      refreshAttachments(id, cancelled);
    });
    const offBoard = Events.On('board:reloaded', () => {
      fetchOnce();
      refreshAttachments(id, cancelled);
    });
    // Drag-and-drop bracket events from gui/main.go. attach:dropping disables
    // the Add Files and Remove buttons while `tb attach` runs (concurrent tb
    // mutations are serialised by .board.lock, but the GUI deserves a
    // feedback signal). attach:dropped re-enables them; the watcher's
    // board:reloaded refresh fires shortly after.
    const onDropEvent = (raw: unknown): { taskId?: string } | undefined => {
      if (raw && typeof raw === 'object') {
        if ('data' in raw && raw.data != null) return raw.data as { taskId?: string };
        return raw as { taskId?: string };
      }
      return undefined;
    };
    const offDropping = Events.On('attach:dropping', (ev: unknown) => {
      const payload = onDropEvent(ev);
      if (payload?.taskId && payload.taskId !== id) return;
      attachmentsBusy = true;
    });
    const offDropped = Events.On('attach:dropped', (ev: unknown) => {
      const payload = onDropEvent(ev);
      if (payload?.taskId && payload.taskId !== id) return;
      attachmentsBusy = false;
    });

    return () => {
      // Flush any pending autosave for the task we're leaving.
      // Belt-and-suspenders: Svelte 5 teardowns don't auto-untrack, and
      // the flush helpers read `detail`/`metadataDirty`/`bodyDirty`. An
      // earlier version of this effect grew its dep set through these
      // reads and OOM'd; wrap them in `untrack` so a future refactor
      // can't reintroduce that loop. Fire-and-forget — toasts surface
      // errors on the runaway save's promise.
      untrack(() => {
        flushPendingMetadataNow();
        flushPendingBodyNow();
      });
      cancelled = true;
      try { offTask(); } catch { /* ignore */ }
      try { offBoard(); } catch { /* ignore */ }
      try { offDropping(); } catch { /* ignore */ }
      try { offDropped(); } catch { /* ignore */ }
      runsUnsub?.();
      runsUnsub = null;
    };
  });

  // TB-130: task-level agentStatus is the source of truth for whether
  // Resume is available — the selected run might be an older one the
  // user is browsing while the latest interrupted run sits at the top.
  let taskAgentStatus = $derived(detail?.metadata.agentStatus ?? '');
  let taskAgentResumable = $derived(detail?.metadata.agentResumable ?? false);
  // Stale-run override: if the JSONL run history reports a run as still
  // `running`/`queued` but the task's AgentStatus has been moved to a
  // terminal value, the run-store is stale (a `finished` JSONL write
  // was lost, or AgentStatus was updated outside `recordTerminal` —
  // agent self-edit, manual `tb edit`, daemon killed mid-write). Without
  // this, the drawer pill and run history show RUNNING forever while
  // the kanban card correctly displays the terminal indicator. The
  // override is a pure derivation; JSONL is not mutated here.
  let effectiveRuns = $derived(
    runs.map((r) => ({ ...r, status: deriveEffectiveRunStatus(r.status, taskAgentStatus) })),
  );
  // Selected run lookup for the status pill source-of-truth (per F4.3 the
  // pill comes from the live Run record, not from currentTask.agentStatus
  // which lags by one tb edit).
  let selectedRun = $derived(effectiveRuns.find((r) => r.runId === $selectedRunID) ?? null);
  let liveStatus = $derived(selectedRun?.status ?? '');
  // Task-scoped busy check: ANY run for this task in queued/running
  // gates Run/Groom/Resume. Selected-run-scoped (liveStatus) was unsound
  // — the user could view a terminal older run while a fresh one was in
  // flight and the gate would mis-report idle.
  let taskHasActiveRun = $derived(
    effectiveRuns.some((r) => r.status === 'queued' || r.status === 'running'),
  );
  let runBusy = $derived(taskHasActiveRun || runStarting || groomStarting || resumeStarting || reviewStarting);
  // TB-197/198: status-aware affordances. Submit-to-review is offered on
  // in-progress tasks (the typical "I'm done, please review" flow);
  // Run-review on code-review tasks (kick a reviewer agent at the linked
  // implementation).
  let canSubmitReview = $derived(detail?.metadata.status === 'in-progress');
  let canRunReview = $derived(detail?.metadata.status === 'code-review');
  let needsUser = $derived(taskAgentStatus === 'needs-user');
  let showResume = $derived(taskAgentResumable);
  let canResume = $derived(showResume && !runBusy && !needsUser);
  let resumeSourceStatus = $derived(taskAgentStatus || 'previous');
  let resumeLabel = $derived(resumeStarting ? 'Resuming...' : `Resume ${resumeSourceStatus} run`);
  let resumeDisabledTitle = $derived(runBusy
    ? 'Another run is in progress for this task'
    : `Resume the captured ${resumeSourceStatus} session`);
  // TB-182: parse the ## User Attention section out of the task body so the
  // drawer can surface the ask near the agent controls. Returns the raw
  // section body (Markdown) or null when the section isn't present.
  let userAttentionBody = $derived(needsUser ? extractUserAttention(detail?.body ?? '') : null);
  let groomEmphasized = $derived(groomReasons.length > 0 || groomHighlight);
  // Auto-groom drawer status row (TB-175): only render anything when
  // auto-groom is enabled. Three mutually exclusive states:
  //   1. needs-default-agent → actionable error copy.
  //   2. settling → countdown so the user knows the run is queued, just
  //      waiting for the settle window to finish.
  //   3. otherwise → small "Auto-grooming on" informational badge.
  let autoGroomEnabledForDrawer = $derived($preferencesStore.autoGroomEnabled);
  let drawerAutoGroomNeedsDefault = $derived(
    autoGroomEnabledForDrawer && $preferencesStore.defaultAgent === 'none',
  );
  let drawerSettleSkipReason = $derived(
    detail ? ($autoGroomStore.lastSkipReasons[detail.metadata.id] ?? '') : '',
  );
  let drawerSettleEligibleMs = $derived(
    detail ? ($autoGroomStore.settleEligibleAtMs[detail.metadata.id] ?? 0) : 0,
  );
  let drawerInSettle = $derived(
    autoGroomEnabledForDrawer && drawerSettleSkipReason === 'settle' && drawerSettleEligibleMs > 0,
  );
  // Coarse "minutes remaining" so the drawer doesn't need a per-second
  // interval just to render guidance. Recomputed whenever the store or
  // current time changes via $derived; close-to-zero rounds up to 1 so
  // we never display "in 0 minutes".
  let now = $state(Date.now());
  $effect(() => {
    if (!drawerInSettle) return;
    const id = setInterval(() => { now = Date.now(); }, 15000);
    return () => clearInterval(id);
  });
  let drawerSettleMinutesRemaining = $derived.by<number>(() => {
    if (!drawerInSettle) return 0;
    const diffMs = drawerSettleEligibleMs - now;
    if (diffMs <= 0) return 0;
    return Math.max(1, Math.ceil(diffMs / 60000));
  });
  // Epic progress derived from the live board store (mirrors Card.svelte).
  // Drives the "Progress" row in the Details rail; gated on the `epic` tag
  // so non-epic tasks never see it.
  let drawerIsEpic = $derived((detail?.metadata.tags ?? []).includes('epic'));
  let drawerProgress = $derived(
    drawerIsEpic && detail ? epicProgress($board, detail.metadata.id) : null,
  );

  let persistedAgent = $derived((detail?.metadata.agent ?? '') as AgentName);

  // TB-237: per-action attribution. Each row is an action that has at
  // least one of (agent, status) recorded — missing actions render
  // nothing (no placeholder rows). Mode order matches the kanban flow.
  type PerActionRow = { mode: string; label: string; agent: string; status: string };
  let perActionAttributions = $derived.by<PerActionRow[]>(() => {
    const md = detail?.metadata;
    if (!md) return [];
    const rows: PerActionRow[] = [
      { mode: 'groom', label: 'Groomed', agent: md.groomedBy ?? '', status: md.groomStatus ?? '' },
      { mode: 'implement', label: 'Implemented', agent: md.implementedBy ?? '', status: md.implementStatus ?? '' },
      { mode: 'review', label: 'Reviewed', agent: md.reviewedBy ?? '', status: md.reviewStatus ?? '' },
    ];
    return rows.filter((r) => r.agent !== '' || r.status !== '');
  });
  // The dropdown falls back to the configured default agent only when the
  // task has no agent set AND the user hasn't explicitly cleared the agent
  // on this task — otherwise picking "(none)" would silently snap back to
  // the default.
  let displayedAgent = $derived(
    persistedAgent
      || (userClearedAgent || $defaultAgentPreference === 'none'
        ? ''
        : $defaultAgentPreference),
  );

  $effect(() => {
    const id = taskId;
    if (!id) {
      groomReasons = [];
      groomHighlight = false;
      return;
    }
    const offTriage = triageForTask(id).subscribe((reasons) => {
      groomReasons = reasons;
    });
    const offSuggest = groomSuggestedFor.subscribe(() => {
      if (!consumeGroomSuggestion(id)) return;
      groomHighlight = true;
      if (groomHighlightTimer) clearTimeout(groomHighlightTimer);
      groomHighlightTimer = setTimeout(() => {
        groomHighlight = false;
        groomHighlightTimer = null;
      }, 2400);
    });
    return () => {
      offTriage();
      offSuggest();
      if (groomHighlightTimer) {
        clearTimeout(groomHighlightTimer);
        groomHighlightTimer = null;
      }
    };
  });

  async function onAgentChange(ev: Event) {
    if (!detail) return;
    const id = detail.metadata.id;
    const target = (((ev.currentTarget as HTMLSelectElement).value as AgentName) || 'none') as AgentName;
    // Explicit "(none)" suppresses the default-agent fallback for this task.
    // Any other pick re-allows it (since the user committed to a real agent).
    userClearedAgent = target === 'none';
    const prev = (detail.metadata.agent ?? '') as AgentName;
    if (target === prev || (target === 'none' && prev === '')) return;
    agentSaving = true;
    try {
      await assignAgent(id, target);
      pushToast(target === 'none' ? `Cleared agent for ${id}` : `Assigned ${target} to ${id}`, 'success');
    } catch (e) {
      pushToast(`Assign failed: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      agentSaving = false;
    }
  }

  // If the dropdown's value comes from the config default rather than the
  // task's stored agent, persist it before kicking off a run. The backend
  // RunAgent/GroomTask read agent from the task file, so the fallback has
  // to be written first or it would fail with ErrNoAgent.
  // `'none'` is never produced by the CLI parser for Task.Agent — the CLI
  // treats it as a clear sentinel — so casting `displayedAgent` (which can
  // be the AgentName union plus the 'claude'|'codex' branches of
  // DefaultAgent) to AgentName is safe here.
  async function ensureAgentPersisted(): Promise<AgentName | null> {
    if (!detail) return null;
    const target = displayedAgent as AgentName;
    if (!target) return null;
    if (persistedAgent === target) return target;
    try {
      await assignAgent(detail.metadata.id, target);
    } catch (e) {
      pushToast(`Assign failed: ${e instanceof Error ? e.message : String(e)}`);
      return null;
    }
    return target;
  }

  async function onRunClick() {
    if (!detail) return;
    const id = detail.metadata.id;
    runStarting = true;
    try {
      const agentName = await ensureAgentPersisted();
      // Null means either the buttons were clicked while disabled (no agent
      // displayed) or the assign step failed and already pushed its own
      // toast — either way, don't proceed to runAgent/groomTask.
      if (!agentName) return;
      const runId = await runAgent(id);
      // Optimistically insert a queued row so the UI is responsive even
      // before the Wails event arrives (avoids a flicker).
      upsertRun({
        runId,
        taskId: id,
        agent: agentName,
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

  async function onResumeClick() {
    if (!detail || !canResume) return;
    const id = detail.metadata.id;
    resumeStarting = true;
    try {
      const agentName = (detail.metadata.agent ?? '').toLowerCase() as AgentName;
      const runId = await resumeAgent(id);
      // Optimistic queued row — the JSONL queued event will carry
      // resumed_from / resumed_from_run so the Wails event soon
      // refreshes this row with the chip.
      upsertRun({
        runId,
        taskId: id,
        agent: agentName || 'claude',
        mode: 'resume',
        status: 'queued',
        queuedAt: new Date().toISOString(),
      });
      selectedRunID.set(runId);
      pushToast(`Resumed ${id}`, 'success');
    } catch (e) {
      pushToast(`Resume failed: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      resumeStarting = false;
    }
  }

  // TB-182: clear the needs-user marker once the user has responded.
  // Sends `agentStatus: "none"` to the CLI which drops the AgentStatus
  // line entirely so manual Run/Groom and the daemon can pick the task
  // up again.
  async function onClearNeedsUserClick() {
    if (!detail || clearingNeedsUser) return;
    const id = detail.metadata.id;
    clearingNeedsUser = true;
    try {
      await editTask(id, { agentStatus: 'none' } as EditTaskInput);
      pushToast(`Cleared user-attention status on ${id}`, 'success');
    } catch (e) {
      pushToast(`Clear failed: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      clearingNeedsUser = false;
    }
  }

  async function onReviewClick() {
    if (!detail) return;
    const id = detail.metadata.id;
    reviewStarting = true;
    try {
      const agentName = await ensureAgentPersisted();
      if (!agentName) return;
      const runId = await reviewTask(id);
      upsertRun({
        runId,
        taskId: id,
        agent: agentName,
        mode: 'review',
        status: 'queued',
        queuedAt: new Date().toISOString(),
      });
      selectedRunID.set(runId);
      pushToast(`Started review for ${id}`, 'success');
    } catch (e) {
      pushToast(`Review failed: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      reviewStarting = false;
    }
  }

  async function onSubmitReviewClick() {
    if (!detail) return;
    const id = detail.metadata.id;
    submittingReview = true;
    try {
      await submitReview(id);
      pushToast(`Submitted ${id} to code review`, 'success');
    } catch (e) {
      pushToast(`Submit failed: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      submittingReview = false;
    }
  }

  async function onGroomClick() {
    if (!detail) return;
    const id = detail.metadata.id;
    groomStarting = true;
    try {
      const agentName = await ensureAgentPersisted();
      // Null means either the buttons were clicked while disabled (no agent
      // displayed) or the assign step failed and already pushed its own
      // toast — either way, don't proceed to runAgent/groomTask.
      if (!agentName) return;
      const runId = await groomTask(id);
      upsertRun({
        runId,
        taskId: id,
        agent: agentName,
        mode: 'groom',
        status: 'queued',
        queuedAt: new Date().toISOString(),
      });
      selectedRunID.set(runId);
      groomHighlight = false;
      pushToast(`Started grooming ${id}`, 'success');
    } catch (e) {
      pushToast(`Groom failed: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      groomStarting = false;
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
      normalizeTags(formTags) !== normalizeTags((detail.metadata.tags ?? []).join(', ')) ||
      (formReviewRef || '').trim() !== (detail.metadata.reviewRef ?? '').trim()
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

    // ReviewRef CAN be cleared from the GUI — the CLI accepts the literal
    // `none` sentinel for `--review-ref` and translates it into a metadata
    // line removal. So a blank → set transition forwards `none`, and a
    // non-empty diff forwards the trimmed value.
    const nextRef = (formReviewRef || '').trim();
    const prevRef = (m.reviewRef ?? '').trim();
    if (nextRef !== prevRef) {
      payload.reviewRef = nextRef === '' ? 'none' : nextRef;
    }
    return { payload, droppedClears };
  }

  // Form-watcher effect: every keystroke that makes the form diverge from
  // disk schedules a single debounced save. Coalesces — the timer is reset
  // on every change, so only the *last* edit before the idle window fires.
  // Stays inert until `detail` is loaded (initial form reset triggers this
  // effect but `metadataDirty` is false then).
  $effect(() => {
    // Track every form field as a reactive dep — this is the trigger we
    // want; everything else (timer assignment, indicator reset, helper
    // dispatch) runs through `untrack` so it doesn't widen the dep set.
    void formPriority; void formType; void formSize; void formModule; void formTags; void formReviewRef;
    if (!detail) return;
    if (!metadataDirty) return;
    untrack(() => {
      userTouchedMetadata = true;
      // Clear the "Saved" indicator the moment new edits start.
      if (metaRecentlySaved) {
        metaRecentlySaved = false;
        if (metaSavedFadeTimer) { clearTimeout(metaSavedFadeTimer); metaSavedFadeTimer = null; }
      }
      if (metaSaveTimer) clearTimeout(metaSaveTimer);
      metaSaveTimer = setTimeout(() => {
        metaSaveTimer = null;
        void runMetadataSave();
      }, AUTOSAVE_DEBOUNCE_MS);
    });
  });

  // Promote `metaPendingSavedIndicator` (latched after a successful save)
  // into the visible "Saved" chip once the watcher refresh has caught up
  // — i.e. metadataDirty is false and no save is in flight. Without this
  // gate we'd flash "Saved" while detail.metadata still held the old
  // pre-save values, lying about persistence. State writes go through
  // `untrack` so the cleared latch doesn't itself re-trigger the effect.
  $effect(() => {
    if (metaPendingSavedIndicator && !metadataDirty && !metaSaving) {
      untrack(() => {
        metaPendingSavedIndicator = false;
        metaRecentlySaved = true;
        if (metaSavedFadeTimer) clearTimeout(metaSavedFadeTimer);
        metaSavedFadeTimer = setTimeout(() => {
          metaRecentlySaved = false;
          metaSavedFadeTimer = null;
        }, SAVED_INDICATOR_MS);
      });
    }
  });

  $effect(() => {
    if (bodyPendingSavedIndicator && !bodyDirty && !bodySaving) {
      untrack(() => {
        bodyPendingSavedIndicator = false;
        bodyRecentlySaved = true;
        if (bodySavedFadeTimer) clearTimeout(bodySavedFadeTimer);
        bodySavedFadeTimer = setTimeout(() => {
          bodyRecentlySaved = false;
          bodySavedFadeTimer = null;
        }, SAVED_INDICATOR_MS);
      });
    }
  });

  async function runMetadataSave() {
    if (!detail) return;
    // Serialize: if a save is in flight, mark the queue and let the
    // in-flight call's finally hook pick up the latest form values once
    // it resolves. Avoids overlapping `tb edit` invocations — the CLI
    // serializes via .board.lock anyway, but back-to-back calls each
    // append a Log line, and we'd race the watcher refresh.
    if (metaSaving) {
      metaPendingResave = true;
      return;
    }
    if (!metadataDirty) return;
    const id = detail.metadata.id;
    const { payload, droppedClears } = diffPayload();

    if (Object.keys(payload).length === 0) {
      // The only diff was a field clear `tb edit` can't represent
      // (empty string = "skip" in the CLI). Surface and snap back, so
      // the drawer stops claiming a value the file doesn't carry.
      if (droppedClears.length > 0) {
        pushToast(`Clearing ${droppedClears.join(', ')} from the GUI isn't supported (CLI has no clear flag).`, 'info');
        resetFormFromDetail();
        userTouchedMetadata = false;
        metaSaveError = null;
      }
      return;
    }

    metaSaving = true;
    metaSaveError = null;
    try {
      await editTask(id, payload);
      // Guard against task-switch contamination: if the drawer's task
      // changed during the await, the post-success state writes belong
      // to a different task. Setting `metaPendingSavedIndicator` or
      // clearing `userTouchedMetadata` on the new task would flash a
      // bogus "Saved" chip or clobber the new task's draft. Surface
      // a toast for the user (the save itself did persist to disk) and
      // skip the state writes.
      if (!detail || detail.metadata.id !== id) {
        pushToast(`Saved ${id}`, 'success');
        return;
      }
      // Clear user-touched FIRST so the watcher's board:reloaded that
      // follows can refill the form from the freshly-saved detail.
      userTouchedMetadata = false;
      if (droppedClears.length > 0) {
        // Partial success: payload applied, but a blanked-out field
        // couldn't be cleared. Reset that field back to disk value.
        pushToast(`Saved ${id}; ${droppedClears.join(', ')} couldn't be cleared from the GUI.`, 'info');
        resetFormFromDetail();
      }
      // Defer the "Saved" indicator until metadataDirty reconciles —
      // an effect below promotes this flag once the watcher refresh
      // makes form values equal detail.metadata again.
      metaPendingSavedIndicator = true;
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      // Same task-switch guard for the error path — surfacing an error
      // on the new task would be misleading.
      if (!detail || detail.metadata.id !== id) {
        pushToast(`Save failed for ${id}: ${msg}`);
        return;
      }
      // Leave the form intact so the user can retry or correct.
      // userTouchedMetadata stays true so the next watcher refresh
      // can't overwrite the draft.
      metaSaveError = msg;
      pushToast(`Save failed: ${metaSaveError}`);
    } finally {
      metaSaving = false;
      // If edits arrived during the in-flight save, run again with the
      // latest form values. Skipped on error so we don't hammer.
      // resetting pendingResave happens regardless so a future task's
      // first save isn't perturbed by a stale flag.
      //
      // Note: the resave's diff is computed against detail.metadata,
      // which still lags the just-completed save until the watcher
      // refresh catches up — so payload may redundantly re-include
      // fields the first save already persisted. The CLI is idempotent
      // under .board.lock so this is harmless, just a slightly fatter
      // call.
      const wasQueued = metaPendingResave;
      metaPendingResave = false;
      if (wasQueued && !metaSaveError && detail && detail.metadata.id === id) {
        void runMetadataSave();
      }
    }
  }

  // Fire any pending debounced save synchronously. Used on task switch
  // and drawer close. Returns immediately — the in-flight call routes
  // its result through pushToast as usual.
  function flushPendingMetadataNow() {
    if (metaSaveTimer) {
      clearTimeout(metaSaveTimer);
      metaSaveTimer = null;
    }
    if (!detail || !metadataDirty || metaSaving) return;
    void runMetadataSave();
  }

  function resetFormFromDetail() {
    if (!detail) return;
    formPriority = detail.metadata.priority ?? '';
    formType = detail.metadata.type ?? '';
    formSize = detail.metadata.size ?? '';
    formModule = detail.metadata.module ?? '';
    formTags = (detail.metadata.tags ?? []).join(', ');
    formReviewRef = detail.metadata.reviewRef ?? '';
  }

  function onRetryMetadataSave() {
    metaSaveError = null;
    void runMetadataSave();
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

  // BodyEditor reports dirty status on every keystroke. We debounce
  // through the body autosave timer and flush on close/task switch.
  function onBodyDirtyChange(d: boolean) {
    bodyDirty = d;
    if (!detail) return;
    if (d) {
      if (bodyRecentlySaved) {
        bodyRecentlySaved = false;
        if (bodySavedFadeTimer) { clearTimeout(bodySavedFadeTimer); bodySavedFadeTimer = null; }
      }
      scheduleBodySave();
    } else {
      // User typed back to disk content — cancel any queued save.
      if (bodySaveTimer) { clearTimeout(bodySaveTimer); bodySaveTimer = null; }
    }
  }

  function scheduleBodySave() {
    if (bodySaveTimer) clearTimeout(bodySaveTimer);
    bodySaveTimer = setTimeout(() => {
      bodySaveTimer = null;
      void runBodySave();
    }, AUTOSAVE_DEBOUNCE_MS);
  }

  async function runBodySave() {
    if (!detail) return;
    if (bodySaving) {
      bodyPendingResave = true;
      return;
    }
    if (!bodyDirty) return;
    const id = detail.metadata.id;
    const draft = bodyDraft;
    bodySaving = true;
    bodySaveError = null;
    try {
      await editTaskBody(id, draft);
      // Task-switch guard: see runMetadataSave for the same pattern.
      // Without this, the new task's body editor would see a flashed
      // "Saved" chip and (worse) `bodyDirty = false` would mask any
      // unflushed draft on the new task.
      if (!detail || detail.metadata.id !== id) {
        pushToast(`Body saved for ${id}`, 'success');
        return;
      }
      // Optimistically clear dirty — onDirtyChange will reassert it if
      // the user typed further during the save. The watcher refresh that
      // follows will update detail.body; if `bodyDraft` still matches,
      // dirty stays false. Defer the "Saved" indicator until that
      // reconciliation happens (see the bodyPendingSavedIndicator effect).
      bodyDirty = false;
      bodyPendingSavedIndicator = true;
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      if (!detail || detail.metadata.id !== id) {
        pushToast(`Body save failed for ${id}: ${msg}`);
        return;
      }
      bodySaveError = msg;
      pushToast(`Body save failed: ${bodySaveError}`);
    } finally {
      bodySaving = false;
      const wasQueued = bodyPendingResave;
      bodyPendingResave = false;
      if (wasQueued && !bodySaveError && detail && detail.metadata.id === id) {
        void runBodySave();
      }
    }
  }

  function flushPendingBodyNow() {
    if (bodySaveTimer) {
      clearTimeout(bodySaveTimer);
      bodySaveTimer = null;
    }
    if (!detail || !bodyDirty || bodySaving) return;
    void runBodySave();
  }

  function onRetryBodySave() {
    bodySaveError = null;
    void runBodySave();
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
    // Cancel any queued autosave so the just-restored disk content
    // isn't immediately re-saved.
    if (bodySaveTimer) { clearTimeout(bodySaveTimer); bodySaveTimer = null; }
    bodyPendingResave = false;
    bodyDraft = detail.body;
    bodyDirty = false;
    bodySaveError = null;
    editMode = false;
  }

  // ---- Title rename ----

  function beginRename() {
    if (!detail || renaming || renameSaving) return;
    renameDraft = detail.metadata.title ?? '';
    renaming = true;
    queueMicrotask(() => {
      renameInput?.focus();
      renameInput?.select();
    });
  }

  function cancelRename() {
    if (renameSaving) return;
    renaming = false;
    renameDraft = '';
  }

  async function commitRename() {
    if (!detail || renameSaving) return;
    const next = renameDraft.trim();
    if (next === '') {
      pushToast('Title cannot be empty', 'info');
      return;
    }
    const current = (detail.metadata.title ?? '').trim();
    if (next === current) {
      // No-op rename — close the editor without a CLI round-trip.
      renaming = false;
      return;
    }
    renameSaving = true;
    try {
      await renameTask(detail.metadata.id, next);
      pushToast(`Renamed ${detail.metadata.id}`, 'success');
      renaming = false;
      // detail will refresh via task:updated / board:reloaded.
    } catch (e) {
      pushToast(`Rename failed: ${e instanceof Error ? e.message : String(e)}`);
      // Keep the editor open with the draft so the user can fix and retry.
    } finally {
      renameSaving = false;
    }
  }

  function onTitleKeydown(ev: KeyboardEvent) {
    // Enter is the button's native activation key; we only intercept F2
    // so power users have the familiar OS-level "rename" shortcut.
    if (renaming) return;
    if (ev.key === 'F2') {
      ev.preventDefault();
      beginRename();
    }
  }

  function onRenameInputKeydown(ev: KeyboardEvent) {
    if (ev.key === 'Enter') {
      ev.preventDefault();
      void commitRename();
    } else if (ev.key === 'Escape') {
      ev.preventDefault();
      ev.stopPropagation();
      cancelRename();
    }
  }

  function refreshAttachments(id: string, cancelled: boolean) {
    const my = ++attachmentsReqSeq;
    attachmentsLoading = true;
    listAttachments(id)
      .then((list) => {
        if (cancelled || taskId !== id || my !== attachmentsReqSeq) return;
        attachments = list;
        attachmentsLoading = false;
      })
      .catch((e) => {
        if (cancelled || taskId !== id || my !== attachmentsReqSeq) return;
        attachments = [];
        attachmentsLoading = false;
        pushToast(`Could not list attachments: ${errorString(e)}`);
      });
  }

  async function onAddAttachments() {
    if (!detail || attachmentsBusy) return;
    const id = detail.metadata.id;
    let picked: string[] = [];
    try {
      picked = await pickAttachmentFiles();
    } catch (e) {
      pushToast(`File picker failed: ${errorString(e)}`);
      return;
    }
    if (picked.length === 0) return;
    attachmentsBusy = true;
    try {
      await addAttachments(id, picked);
      pushToast(`Attached ${picked.length} file(s) to ${id}`, 'success');
      // Refresh is driven by the watcher's board:reloaded; the drawer
      // already listens for it. Don't fetchOnce here — that violates
      // TB-104's "no duplicate refresh" criterion.
    } catch (e) {
      pushToast(`Attach failed: ${errorString(e)}`);
    } finally {
      attachmentsBusy = false;
    }
  }

  // Two-click confirm for attachment removal — mirrors archivePrompt/
  // cancelPrompt. A misclick on the X used to be irrecoverable from the UI;
  // now the first click arms the button for ~4s and the second click commits.
  // (state declared at the top of the script — see attachmentRemovePending /
  // attachmentRemoveTimer.)

  function onRemoveAttachment(name: string) {
    if (!detail || attachmentsBusy) return;
    if (attachmentRemovePending !== name) {
      attachmentRemovePending = name;
      if (attachmentRemoveTimer) clearTimeout(attachmentRemoveTimer);
      attachmentRemoveTimer = setTimeout(() => {
        attachmentRemovePending = null;
        attachmentRemoveTimer = null;
      }, 4000);
      return;
    }
    if (attachmentRemoveTimer) { clearTimeout(attachmentRemoveTimer); attachmentRemoveTimer = null; }
    attachmentRemovePending = null;
    void doRemoveAttachment(name);
  }

  async function doRemoveAttachment(name: string) {
    if (!detail) return;
    const id = detail.metadata.id;
    attachmentsBusy = true;
    try {
      await removeAttachments(id, [name]);
      pushToast(`Removed ${name} from ${id}`, 'info');
    } catch (e) {
      pushToast(`Remove failed: ${errorString(e)}`);
    } finally {
      attachmentsBusy = false;
    }
  }

  async function onOpenAttachment(name: string) {
    if (!detail) return;
    try {
      await openAttachment(detail.metadata.id, name);
    } catch (e) {
      pushToast(`Open failed: ${errorString(e)}`);
    }
  }

  // Use IEC binary labels (KiB/MiB/GiB) since the divisors are 1024-based.
  // Mixing 1024-based math with KB/MB/GB labels is a common bug that mismatches
  // what users see in tools like Finder/Explorer.
  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GiB`;
  }

  function onKeydown(ev: KeyboardEvent) {
    if (!taskId) return;
    if (ev.key === 'Escape') {
      // While renaming, Escape cancels the draft (handled on the input);
      // the drawer must not also close.
      if (renaming) return;
      ev.preventDefault();
      tryClose();
    } else if (editMode && (ev.metaKey || ev.ctrlKey) && ev.key.toLowerCase() === 's') {
      // Cmd/Ctrl+S now flushes the pending autosave rather than acting
      // as the primary save mechanism. Useful for power users who want
      // to skip the debounce window before navigating away.
      ev.preventDefault();
      flushPendingBodyNow();
    }
  }

  function tryClose() {
    // With autosave, "unsaved" only means a pending debounce or
    // in-flight save. The window.confirm dialog the explicit-save
    // version showed is gone — flush synchronously and close. Errors
    // surface via toast, so the user still sees them.
    flushPendingMetadataNow();
    flushPendingBodyNow();
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

  // extractUserAttention returns the contents of the `## User Attention`
  // section from a task body, or null when the section isn't present.
  // Mirrors the CLI's section parsing (cli/edit.go) — boundaries are the
  // next `## ` heading or end-of-file.
  function extractUserAttention(body: string): string | null {
    if (!body) return null;
    const lines = body.split('\n');
    let start = -1;
    for (let i = 0; i < lines.length; i++) {
      if (lines[i].trim() === '## User Attention') {
        start = i + 1;
        break;
      }
    }
    if (start === -1) return null;
    let end = lines.length;
    for (let i = start; i < lines.length; i++) {
      if (lines[i].startsWith('## ')) {
        end = i;
        break;
      }
    }
    const body2 = lines.slice(start, end).join('\n').trim();
    return body2 || null;
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
    <section
      class="surface"
      data-file-drop-target
      data-task-id={taskId}>
      <header class="surface-head">
        <div class="title-block">
          {#if detail}
            <span class="id">{detail.metadata.id}</span>
            {#if renaming}
              <div class="title-edit">
                <input
                  class="title-input"
                  type="text"
                  bind:value={renameDraft}
                  bind:this={renameInput}
                  onkeydown={onRenameInputKeydown}
                  disabled={renameSaving}
                  aria-label="Task title" />
                <div class="title-edit-actions">
                  <button
                    class="primary compact"
                    type="button"
                    onclick={() => void commitRename()}
                    disabled={renameSaving || renameDraft.trim() === ''}>
                    {renameSaving ? 'Saving…' : 'Save'}
                  </button>
                  <button
                    class="ghost compact"
                    type="button"
                    onclick={cancelRename}
                    disabled={renameSaving}>Cancel</button>
                </div>
              </div>
            {:else}
              <div class="title-row">
                <h2
                  title="Double-click to rename"
                  ondblclick={beginRename}>{detail.metadata.title}</h2>
                <button
                  class="rename-btn"
                  type="button"
                  aria-label="Rename task"
                  title="Rename (Enter / F2)"
                  onkeydown={onTitleKeydown}
                  onclick={beginRename}>Rename</button>
              </div>
            {/if}
          {:else}
            <span class="id">{taskId}</span>
            <h2>Loading…</h2>
          {/if}
        </div>
        <button class="close" type="button" onclick={tryClose} aria-label="Close" title="Close (Esc)">×</button>
      </header>

      {#if loading && !detail}
        <p class="hint pad">Loading…</p>
      {:else if err}
        <p class="err pad">{err}</p>
      {:else if detail}
        <div class="grid">
          <div class="main">
            <section class="body-section">
              <div class="section-head">
                <h3>Description</h3>
                <div class="toolbar-actions">
                  {#if editMode}
                    <span
                      class="autosave-status"
                      role="status"
                      aria-live="polite"
                      data-state={bodySaveError
                        ? 'error'
                        : (bodySaving
                          ? 'saving'
                          : (bodyDirty
                            ? 'pending'
                            : (bodyRecentlySaved ? 'saved' : 'idle')))}>
                      {#if bodySaveError}
                        Save failed
                      {:else if bodySaving}
                        Saving…
                      {:else if bodyDirty}
                        Unsaved
                      {:else if bodyRecentlySaved}
                        Saved
                      {:else}
                        &nbsp;
                      {/if}
                    </span>
                    {#if bodySaveError}
                      <button class="ghost compact" type="button" onclick={onRetryBodySave}>Retry</button>
                    {/if}
                    <button class="ghost compact" type="button" onclick={discardBody}>Discard</button>
                  {:else}
                    <button class="ghost compact" type="button" onclick={enterEdit}>Edit</button>
                  {/if}
                </div>
              </div>
              {#if editMode}
                <BodyEditor
                  bind:value={bodyDraft}
                  originalBody={detail.body}
                  onDirtyChange={onBodyDirtyChange}
                />
              {:else}
                <article class="body markdown">
                  <!-- HTML is DOMPurify-sanitized in renderMarkdown(). -->
                  <!-- eslint-disable-next-line svelte/no-at-html-tags -->
                  {@html renderMarkdown(stripFrontmatter(detail.body))}
                </article>
              {/if}
            </section>

            <section class="attachments-section">
              <div class="section-head">
                <h3>Attachments</h3>
                <button
                  class="ghost compact"
                  type="button"
                  disabled={attachmentsBusy}
                  onclick={onAddAttachments}>
                  {attachmentsBusy ? 'Working…' : 'Add files…'}
                </button>
              </div>
              {#if attachmentsLoading && attachments.length === 0}
                <p class="hint">Loading attachments…</p>
              {:else if attachments.length === 0}
                <p class="hint">No attachments. Add files via the button above or drag-and-drop files onto this drawer.</p>
              {:else}
                <ul class="attachment-list" aria-label="Attachments">
                  {#each attachments as a (a.name)}
                    <li>
                      <button
                        class="att-name"
                        type="button"
                        title="Open in default application"
                        aria-label={`Open ${a.name} in default application`}
                        onclick={() => onOpenAttachment(a.name)}>
                        {a.name}
                      </button>
                      <span class="att-size" title={`${a.size.toLocaleString()} bytes`}>{formatSize(a.size)}</span>
                      <button
                        class="att-remove"
                        class:armed={attachmentRemovePending === a.name}
                        type="button"
                        disabled={attachmentsBusy}
                        aria-label={attachmentRemovePending === a.name
                          ? `Click again to remove ${a.name}`
                          : `Remove ${a.name}`}
                        title={attachmentRemovePending === a.name
                          ? 'Click again to remove'
                          : 'Remove attachment'}
                        onclick={() => onRemoveAttachment(a.name)}>{attachmentRemovePending === a.name ? '!' : '×'}</button>
                    </li>
                  {/each}
                </ul>
              {/if}
            </section>
          </div>

          <aside class="rail">
            <section class="rail-section details-section">
              <div class="section-head">
                <h3>Details</h3>
                <div class="autosave-cluster">
                  <span
                    class="autosave-status"
                    role="status"
                    aria-live="polite"
                    data-state={metaSaveError
                      ? 'error'
                      : (metaSaving
                        ? 'saving'
                        : (metadataDirty
                          ? 'pending'
                          : (metaRecentlySaved ? 'saved' : 'idle')))}>
                    {#if metaSaveError}
                      Save failed
                    {:else if metaSaving}
                      Saving…
                    {:else if metadataDirty}
                      Unsaved
                    {:else if metaRecentlySaved}
                      Saved
                    {:else}
                      &nbsp;
                    {/if}
                  </span>
                  {#if metaSaveError}
                    <button class="ghost compact" type="button" onclick={onRetryMetadataSave}>Retry</button>
                  {/if}
                </div>
              </div>

              <dl class="readonly-meta">
                <dt>Status</dt>
                <dd>{detail.metadata.status || '—'}</dd>
                {#if drawerIsEpic && drawerProgress}
                  <dt>Progress</dt>
                  <dd class="epic-progress-cell">
                    <span class="epic-progress-label">
                      {drawerProgress.done}/{drawerProgress.total}
                      {#if drawerProgress.total > 0}
                        <span class="epic-progress-percent">({drawerProgress.percent}%)</span>
                      {:else}
                        <span class="epic-progress-empty">no children yet</span>
                      {/if}
                    </span>
                    <span
                      class="epic-progress-bar"
                      class:complete={drawerProgress.total > 0 && drawerProgress.done === drawerProgress.total}
                      class:empty={drawerProgress.total === 0}
                      aria-hidden="true">
                      <span
                        class="epic-progress-fill"
                        style={`width: ${drawerProgress.total === 0 ? 0 : drawerProgress.percent}%`}></span>
                    </span>
                  </dd>
                {/if}
                {#if detail.metadata.parent}
                  <dt>Parent</dt>
                  <dd>{detail.metadata.parent}</dd>
                {/if}
                {#if detail.metadata.branch}
                  <dt>Branch</dt>
                  <dd class="mono">{detail.metadata.branch}</dd>
                {/if}
              </dl>

              <div class="field">
                <span class="field-label">Priority</span>
                <select bind:value={formPriority}>
                  <option value=""></option>
                  <option>P0</option><option>P1</option><option>P2</option><option>P3</option>
                </select>
              </div>
              <div class="field">
                <span class="field-label">Type</span>
                <select bind:value={formType}>
                  <option value=""></option>
                  <option value="bug">bug</option>
                  <option value="feature">feature</option>
                  <option value="tech-debt">tech-debt</option>
                  <option value="improvement">improvement</option>
                  <option value="spike">spike</option>
                </select>
              </div>
              <div class="field">
                <span class="field-label">Size</span>
                <select bind:value={formSize}>
                  <option value=""></option>
                  <option>S</option><option>M</option><option>L</option><option>XL</option>
                </select>
              </div>
              <div class="field">
                <span class="field-label">Module</span>
                <input bind:value={formModule} placeholder="module" />
              </div>
              <div class="field">
                <span class="field-label">Tags</span>
                <input bind:value={formTags} placeholder="comma,separated" />
              </div>
              <div class="field">
                <span class="field-label">ReviewRef</span>
                <input
                  bind:value={formReviewRef}
                  placeholder="branch / PR URL / commit / worktree"
                  aria-describedby="reviewref-help"
                  spellcheck="false" />
                <span id="reviewref-help" class="field-help">Required to enter code-review.</span>
              </div>
            </section>

            <section class="rail-section agent-section">
              <div class="section-head">
                <h3>Agent</h3>
                {#if needsUser}
                  <span class="pill pill-needs-user" title="Task is waiting for user input — see User Attention panel">needs-user</span>
                {:else if liveStatus}
                  <span class={`pill pill-${liveStatus}`}>{liveStatus}</span>
                {/if}
              </div>
              {#if needsUser}
                <div class="user-attention-panel" role="note" aria-label="User attention required">
                  <div class="user-attention-head">
                    <span class="user-attention-badge">User attention required</span>
                    <span class="user-attention-hint">The previous agent run stopped because it needs a decision.</span>
                  </div>
                  {#if userAttentionBody}
                    <article class="user-attention-body markdown">
                      <!-- HTML is DOMPurify-sanitized in renderMarkdown(). -->
                      <!-- eslint-disable-next-line svelte/no-at-html-tags -->
                      {@html renderMarkdown(userAttentionBody)}
                    </article>
                  {:else}
                    <p class="hint">
                      No <code>## User Attention</code> section in the task body.
                      Check the task file or run history for the agent's ask.
                    </p>
                  {/if}
                  <div class="user-attention-resolve">
                    <p>
                      After you answer in the task body (or grant the ask),
                      click <strong>Clear status</strong> below to re-enable
                      Run and Groom. (Equivalent CLI:
                      <code>tb edit {detail.metadata.id} --agent-status none</code>.)
                    </p>
                    <button
                      class="primary compact"
                      type="button"
                      disabled={clearingNeedsUser}
                      onclick={onClearNeedsUserClick}>
                      {clearingNeedsUser ? 'Clearing…' : 'Clear status'}
                    </button>
                  </div>
                </div>
              {/if}
              <div class="field">
                <span class="field-label">Assigned</span>
                <select
                  aria-label="Agent"
                  disabled={agentSaving}
                  value={displayedAgent}
                  onchange={onAgentChange}>
                  <option value="">(none)</option>
                  <option value="claude">claude</option>
                  <option value="codex">codex</option>
                </select>
              </div>
              {#if autoGroomEnabledForDrawer}
                <div class="auto-groom-status">
                  {#if drawerAutoGroomNeedsDefault}
                    <p class="auto-groom-needs-default" role="alert">
                      Auto-groom is enabled but no default agent is set. Choose claude or codex
                      in Settings.
                    </p>
                  {:else if drawerInSettle}
                    <p class="auto-groom-settle-row">
                      Auto-groom will run in ~{drawerSettleMinutesRemaining}m. Editing or
                      attaching files resets the window.
                    </p>
                  {:else}
                    <p class="auto-groom-on">Auto-grooming on</p>
                  {/if}
                </div>
              {/if}
              <div class="agent-buttons">
                <button
                  class="primary compact"
                  type="button"
                  disabled={!displayedAgent || runBusy || needsUser}
                  title={needsUser
                    ? 'Task needs user input — clear AgentStatus before running again'
                    : (!displayedAgent ? 'Select an agent first' : (liveStatus === 'running' ? 'Already running' : 'Start a fresh conversation'))}
                  onclick={onRunClick}>
                  {runStarting ? 'Starting…' : 'Run'}
                </button>
                {#if showResume}
                  <button
                    class="primary compact resume"
                    type="button"
                    disabled={!canResume}
                    title={canResume
                      ? `Resume the captured ${resumeSourceStatus} session`
                      : resumeDisabledTitle}
                    onclick={onResumeClick}>
                    {resumeLabel}
                  </button>
                {/if}
                <button
                  class:emphasized={groomEmphasized && !runBusy && !needsUser}
                  class="secondary compact"
                  type="button"
                  disabled={!displayedAgent || runBusy || needsUser}
                  title={needsUser
                    ? 'Task needs user input — clear AgentStatus before grooming again'
                    : (groomReasons.length > 0 ? `Needs grooming: ${groomReasons.join(', ')}` : (!displayedAgent ? 'Select an agent first' : ''))}
                  onclick={onGroomClick}>
                  {groomStarting ? 'Grooming…' : 'Groom'}
                </button>
                {#if canRunReview}
                  <button
                    class="secondary compact"
                    type="button"
                    disabled={!displayedAgent || runBusy || needsUser}
                    title={needsUser
                      ? 'Task needs user input — clear AgentStatus before reviewing'
                      : (!displayedAgent ? 'Select an agent first' : 'Run a reviewer agent against the linked Review Target')}
                    onclick={onReviewClick}>
                    {reviewStarting ? 'Reviewing…' : 'Review'}
                  </button>
                {/if}
                {#if canSubmitReview}
                  <button
                    class="secondary compact"
                    type="button"
                    disabled={submittingReview}
                    title="Move this task to code-review for human / agent review"
                    onclick={onSubmitReviewClick}>
                    {submittingReview ? 'Submitting…' : 'Submit for review'}
                  </button>
                {/if}
                {#if liveStatus === 'running' || liveStatus === 'queued'}
                  <button class="danger compact" type="button" onclick={startCancel}>
                    {cancelPrompt ? 'Click again to cancel' : 'Cancel'}
                  </button>
                {/if}
              </div>

              {#if perActionAttributions.length > 0}
                <div class="rail-subhead">Per action</div>
                <ul class="per-action-list" aria-label="Per-action attribution">
                  {#each perActionAttributions as row (row.mode)}
                    <li>
                      <span class="mode">{row.label}</span>
                      <span class="agent">{row.agent || '—'}</span>
                      {#if row.status}
                        <span class={`pill pill-${row.status}`}>{row.status}</span>
                      {/if}
                    </li>
                  {/each}
                </ul>
              {/if}

              {#if effectiveRuns.length > 0}
                <div class="rail-subhead">Run history</div>
                <ul class="run-list" aria-label="Past runs">
                  {#each effectiveRuns as r (r.runId)}
                    <li>
                      <button
                        type="button"
                        class:active={$selectedRunID === r.runId}
                        onclick={() => pickRun(r.runId)}>
                        <span class="when">{fmtRelative(r.startedAt || r.queuedAt)}</span>
                        <span class="mode">{r.mode || 'implement'}</span>
                        <span class={`pill pill-${r.status || 'idle'}`}>{r.status || 'idle'}</span>
                        <span class="agent">{r.agent}</span>
                        {#if r.resumedFromRun}
                          <span
                            class="chip resumed-from"
                            title={`Resumed from run ${r.resumedFromRun}`}>
                            ↻ {r.resumedFromRun}
                          </span>
                        {/if}
                      </button>
                    </li>
                  {/each}
                </ul>
              {/if}

              <div class="run-log-wrap">
                <AgentRunLog runId={$selectedRunID} taskId={detail.metadata.id} />
              </div>
            </section>

            <section class="rail-section danger-section">
              <div class="section-head">
                <h3>Danger zone</h3>
              </div>
              <button class="danger full" type="button" onclick={startArchive}>
                {archivePrompt ? 'Click again to archive' : 'Archive'}
              </button>
            </section>
          </aside>
        </div>
      {/if}
    </section>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    display: flex;
    justify-content: center;
    align-items: stretch;
    padding: 16px;
    box-sizing: border-box;
    z-index: 50;
    backdrop-filter: blur(2px);
  }
  :global(html.platform-mac) .backdrop {
    padding-top: var(--mac-titlebar-height);
  }
  .surface {
    background: var(--bg-elev);
    width: 100%;
    max-width: 1600px;
    height: 100%;
    overflow: hidden;
    border-radius: 10px;
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.55);
    border: 1px solid rgba(255, 255, 255, 0.06);
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    position: relative;
  }
  /* Whole-drawer file drop affordance. The Wails runtime tags the matched
     [data-file-drop-target] element with .file-drop-target-active while the
     OS drag is hovering, so styling it here makes the entire drawer body a
     visible target — see TB-125. The ::after layer is an inset accent
     border drawn over the drawer chrome without nudging layout. */
  .surface:global(.file-drop-target-active)::after {
    content: '';
    position: absolute;
    inset: 0;
    border-radius: inherit;
    pointer-events: none;
    border: 2px dashed var(--accent);
    box-shadow: inset 0 0 0 1px rgba(74, 141, 248, 0.35),
                0 0 0 1px rgba(74, 141, 248, 0.2);
    background: rgba(74, 141, 248, 0.06);
    z-index: 1;
  }
  .surface-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    padding: 16px 24px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);
    background: rgba(255, 255, 255, 0.02);
  }
  .title-block { min-width: 0; flex: 1 1 auto; }
  .id {
    display: inline-block;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 11px;
    color: var(--fg-dim);
    margin-bottom: 4px;
    letter-spacing: 0.03em;
  }
  .surface-head h2 {
    margin: 0;
    font-size: 20px;
    line-height: 1.3;
    font-weight: 600;
    word-break: break-word;
    border-radius: 4px;
    flex: 1 1 auto;
    min-width: 0;
  }
  .title-row {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
  }
  .rename-btn {
    flex: 0 0 auto;
    background: rgba(255, 255, 255, 0.06);
    border: 1px solid rgba(255, 255, 255, 0.12);
    color: var(--fg-dim);
    font: inherit;
    font-size: 11px;
    padding: 3px 9px;
    border-radius: 4px;
    cursor: pointer;
  }
  .rename-btn:hover { background: rgba(255, 255, 255, 0.10); color: var(--fg); }
  .rename-btn:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
  .title-edit {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .title-input {
    background: rgba(0, 0, 0, 0.30);
    border: 1px solid var(--accent);
    color: var(--fg);
    border-radius: 5px;
    padding: 6px 10px;
    font: inherit;
    font-size: 20px;
    line-height: 1.3;
    font-weight: 600;
    width: 100%;
    box-sizing: border-box;
  }
  .title-input:focus { outline: none; box-shadow: 0 0 0 2px rgba(74, 141, 248, 0.25); }
  .title-input:disabled { opacity: 0.7; }
  .title-edit-actions {
    display: flex;
    gap: 6px;
  }
  .close {
    background: none;
    border: 0;
    font-size: 24px;
    line-height: 1;
    cursor: pointer;
    padding: 4px 10px;
    color: var(--fg-dim);
    border-radius: 6px;
    flex: 0 0 auto;
  }
  .close:hover { background: rgba(255, 255, 255, 0.06); color: var(--fg); }

  .grid {
    flex: 1 1 auto;
    display: grid;
    grid-template-columns: minmax(0, 2fr) minmax(300px, 1fr);
    gap: 0;
    overflow: hidden;
  }
  .main {
    overflow-y: auto;
    padding: 20px 24px 28px;
    display: flex;
    flex-direction: column;
    gap: 18px;
    min-width: 0;
  }
  .rail {
    overflow-y: auto;
    padding: 20px 22px 28px;
    border-left: 1px solid rgba(255, 255, 255, 0.06);
    background: rgba(0, 0, 0, 0.12);
    display: flex;
    flex-direction: column;
    gap: 16px;
    min-width: 0;
  }

  .pad { padding: 20px 24px; }

  .section-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin-bottom: 8px;
  }
  .section-head h3 {
    margin: 0;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--fg-dim);
    font-weight: 600;
  }
  .toolbar-actions { display: flex; gap: 6px; align-items: center; }

  .autosave-cluster {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .autosave-status {
    font-size: 10px;
    font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    padding: 2px 6px;
    border-radius: 3px;
    line-height: 1.4;
    min-width: 56px;
    text-align: center;
    color: var(--fg-dim);
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid transparent;
  }
  .autosave-status[data-state='pending'] { color: var(--p1); background: rgba(255, 184, 108, 0.12); }
  .autosave-status[data-state='saving'] { color: var(--p2); background: rgba(74, 141, 248, 0.14); }
  .autosave-status[data-state='saved'] { color: #50c878; background: rgba(80, 200, 120, 0.14); }
  .autosave-status[data-state='error'] { color: var(--p0); background: rgba(255, 90, 82, 0.16); border-color: rgba(255, 90, 82, 0.36); }
  .autosave-status[data-state='idle'] { visibility: hidden; }
  .rail-subhead {
    margin-top: 6px;
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--fg-dim);
    font-weight: 600;
  }

  .rail-section {
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .field-label {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--fg-dim);
    font-weight: 600;
  }
  .field-help {
    font-size: 10px;
    color: var(--fg-dim);
    font-style: italic;
    margin-top: 2px;
  }
  .rail input,
  .rail select {
    background: rgba(0, 0, 0, 0.25);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--fg);
    border-radius: 4px;
    padding: 6px 8px;
    font: inherit;
    font-size: 12px;
    width: 100%;
    box-sizing: border-box;
  }
  .rail input:focus,
  .rail select:focus {
    outline: none;
    border-color: var(--accent);
  }

  .readonly-meta {
    display: grid;
    grid-template-columns: 70px 1fr;
    column-gap: 10px;
    row-gap: 4px;
    margin: 0 0 4px;
    font-size: 12px;
  }
  .readonly-meta dt {
    color: var(--fg-dim);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-size: 10px;
    margin: 0;
    align-self: center;
    font-weight: 600;
  }
  .readonly-meta dd { margin: 0; word-break: break-word; }
  .readonly-meta dd.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 11px; }

  .epic-progress-cell {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .epic-progress-cell .epic-progress-label {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-variant-numeric: tabular-nums;
    font-size: 12px;
    color: var(--fg);
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .epic-progress-cell .epic-progress-percent { color: var(--fg-dim); font-size: 11px; }
  .epic-progress-cell .epic-progress-empty {
    color: var(--fg-dim);
    font-size: 11px;
    font-style: italic;
  }
  .epic-progress-cell .epic-progress-bar {
    display: block;
    width: 100%;
    height: 4px;
    background: rgba(255, 255, 255, 0.06);
    border-radius: 2px;
    overflow: hidden;
  }
  .epic-progress-cell .epic-progress-fill {
    display: block;
    height: 100%;
    background: var(--accent);
    border-radius: 2px;
    transition: width 160ms ease;
  }
  .epic-progress-cell .epic-progress-bar.complete .epic-progress-fill {
    background: #50c878;
  }
  .epic-progress-cell .epic-progress-bar.empty .epic-progress-fill {
    background: transparent;
  }

  .primary {
    background: var(--accent);
    color: white;
    border: 0;
    border-radius: 5px;
    padding: 6px 14px;
    cursor: pointer;
    font-weight: 600;
    font: inherit;
    font-size: 12px;
  }
  .primary:disabled { opacity: 0.4; cursor: not-allowed; }
  .primary.compact { padding: 4px 10px; font-size: 11px; }

  .secondary {
    background: rgba(255, 255, 255, 0.06);
    color: var(--fg);
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 5px;
    padding: 6px 14px;
    cursor: pointer;
    font-weight: 600;
    font: inherit;
    font-size: 12px;
  }
  .secondary:hover { background: rgba(255, 255, 255, 0.10); }
  .secondary:disabled { opacity: 0.4; cursor: not-allowed; }
  .secondary.compact { padding: 4px 10px; font-size: 11px; }
  .secondary.emphasized {
    border-color: rgba(255, 184, 108, 0.62);
    box-shadow: 0 0 0 2px rgba(255, 184, 108, 0.18);
    color: var(--p1);
  }

  .ghost {
    background: transparent;
    border: 1px solid rgba(255, 255, 255, 0.12);
    color: var(--fg);
    border-radius: 5px;
    padding: 6px 14px;
    cursor: pointer;
    font: inherit;
    font-size: 12px;
  }
  .ghost:hover { background: rgba(255, 255, 255, 0.06); }
  .ghost.compact { padding: 4px 10px; font-size: 11px; }

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
  .danger.compact { padding: 4px 10px; font-size: 11px; }
  .danger.full { width: 100%; padding: 8px 14px; }

  .body-section { display: flex; flex-direction: column; gap: 8px; }
  .body {
    background: transparent;
    padding: 4px 0 0;
    border-radius: 0;
    margin: 0;
    font-size: 14px;
    line-height: 1.65;
    overflow-x: auto;
  }
  .markdown :global(h1),
  .markdown :global(h2),
  .markdown :global(h3) {
    margin: 16px 0 6px;
    font-weight: 600;
    line-height: 1.3;
  }
  .markdown :global(h1) { font-size: 18px; }
  .markdown :global(h2) { font-size: 13px; color: var(--fg-dim); text-transform: uppercase; letter-spacing: 0.06em; margin-top: 22px; }
  .markdown :global(h3) { font-size: 13px; }
  .markdown :global(p) { margin: 0 0 10px; }
  .markdown :global(ul), .markdown :global(ol) { margin: 0 0 10px; padding-left: 20px; }
  .markdown :global(li) { margin: 2px 0; }
  .markdown :global(li input[type='checkbox']) { margin-right: 6px; pointer-events: none; }
  .markdown :global(code) { background: rgba(255, 255, 255, 0.06); padding: 1px 5px; border-radius: 3px; font-family: ui-monospace, monospace; font-size: 12px; }
  .markdown :global(pre) { background: rgba(0, 0, 0, 0.25); padding: 10px 12px; border-radius: 4px; overflow-x: auto; margin: 8px 0; }
  .markdown :global(pre code) { background: none; padding: 0; }
  .markdown :global(a) { color: var(--accent); }
  .markdown :global(strong) { color: var(--fg); }
  .markdown :global(blockquote) { border-left: 3px solid rgba(255, 255, 255, 0.1); padding-left: 10px; margin: 8px 0; color: var(--fg-dim); }

  .agent-buttons {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }
  .auto-groom-status {
    margin: 6px 0 8px;
  }
  .auto-groom-status p {
    margin: 0;
    padding: 7px 10px;
    border-radius: 5px;
    font-size: 11.5px;
    line-height: 1.4;
  }
  .auto-groom-needs-default {
    border: 1px solid rgba(244, 191, 79, 0.45);
    background: rgba(244, 191, 79, 0.08);
    color: #f4bf4f;
  }
  .auto-groom-settle-row {
    border: 1px solid rgba(255, 255, 255, 0.1);
    background: rgba(255, 255, 255, 0.04);
    color: var(--fg-dim);
  }
  .auto-groom-on {
    border: 1px solid rgba(74, 141, 248, 0.32);
    background: rgba(74, 141, 248, 0.1);
    color: var(--accent);
    font-weight: 500;
    display: inline-block;
    padding: 4px 9px;
    font-size: 10.5px;
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
  .pill-interrupted { background: rgba(245, 158, 11, 0.22); color: #f59e0b; }
  .pill-lost { background: rgba(236, 72, 153, 0.18); color: #ec4899; }
  .pill-needs-user { background: rgba(168, 85, 247, 0.22); color: #a855f7; }
  .pill-idle { background: rgba(110, 118, 134, 0.10); color: var(--fg-dim); }

  .user-attention-panel {
    margin-top: 4px;
    padding: 10px 12px;
    border-radius: 6px;
    border: 1px solid rgba(168, 85, 247, 0.42);
    background: rgba(168, 85, 247, 0.10);
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .user-attention-head {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .user-attention-badge {
    font-size: 10px;
    font-weight: 700;
    color: #a855f7;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .user-attention-hint {
    font-size: 11px;
    color: var(--fg-dim);
  }
  .user-attention-body {
    font-size: 12px;
    line-height: 1.55;
    color: var(--fg);
    background: rgba(0, 0, 0, 0.18);
    padding: 8px 10px;
    border-radius: 4px;
    overflow-x: auto;
  }
  .user-attention-resolve {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .user-attention-resolve p {
    font-size: 11px;
    color: var(--fg-dim);
    margin: 0;
  }
  .user-attention-resolve code {
    background: rgba(255, 255, 255, 0.06);
    padding: 1px 5px;
    border-radius: 3px;
    font-family: ui-monospace, monospace;
    font-size: 11px;
  }
  .user-attention-resolve button {
    align-self: flex-start;
  }

  .chip.resumed-from {
    font-size: 10px;
    font-weight: 500;
    padding: 1px 6px;
    border-radius: 4px;
    background: rgba(245, 158, 11, 0.14);
    color: #f59e0b;
    letter-spacing: 0.02em;
    text-transform: none;
  }

  .agent-buttons .resume {
    background: rgba(245, 158, 11, 0.16);
    color: #f59e0b;
    border-color: rgba(245, 158, 11, 0.45);
  }
  .agent-buttons .resume:hover {
    background: rgba(245, 158, 11, 0.24);
  }

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
    gap: 6px;
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
    flex-wrap: wrap;
  }
  .run-list button:hover { background: rgba(255, 255, 255, 0.04); }
  .run-list button.active { background: rgba(74, 141, 248, 0.12); }
  .run-list .when { color: var(--fg-dim); font-family: ui-monospace, monospace; }
  .run-list .mode {
    color: var(--p1);
    border: 1px solid rgba(255, 184, 108, 0.24);
    border-radius: 3px;
    padding: 0 5px;
    font-family: ui-monospace, monospace;
  }
  .run-list .agent { color: var(--fg-dim); }

  /* TB-237: per-action attribution list. Mirrors .run-list visually but
     rows are plain <span>s, not buttons, because the per-mode pair is
     read-only. */
  .per-action-list {
    list-style: none;
    margin: 0 0 8px 0;
    padding: 0;
    border-top: 1px solid rgba(255, 255, 255, 0.05);
  }
  .per-action-list li {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 6px;
    font-size: 11px;
    flex-wrap: wrap;
  }
  .per-action-list .mode {
    color: var(--p1);
    border: 1px solid rgba(255, 184, 108, 0.24);
    border-radius: 3px;
    padding: 0 5px;
    font-family: ui-monospace, monospace;
  }
  .per-action-list .agent { color: var(--fg-dim); }

  .run-log-wrap {
    min-height: 0;
  }

  .hint { color: var(--fg-dim); font-size: 12px; }
  .err { color: var(--p0); font-size: 12px; }

  .attachments-section {
    padding: 12px 14px;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .attachment-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .attachment-list li {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 6px;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.02);
  }
  .attachment-list li:hover {
    background: rgba(255, 255, 255, 0.05);
  }
  .att-name {
    flex: 1 1 auto;
    text-align: left;
    background: none;
    border: 0;
    color: var(--accent);
    cursor: pointer;
    font: inherit;
    font-size: 12px;
    font-family: ui-monospace, monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding: 0;
  }
  .att-name:hover { text-decoration: underline; }
  .att-size {
    color: var(--fg-dim);
    font-size: 11px;
    font-family: ui-monospace, monospace;
    flex: 0 0 auto;
  }
  .att-remove {
    flex: 0 0 auto;
    background: none;
    border: 0;
    color: var(--fg-dim);
    cursor: pointer;
    font: inherit;
    font-size: 16px;
    line-height: 1;
    padding: 0 4px;
    border-radius: 3px;
  }
  .att-remove:hover {
    color: var(--p0);
    background: rgba(255, 90, 82, 0.1);
  }
  .att-remove.armed {
    color: #fff;
    background: var(--p0);
    font-weight: 700;
  }
  .att-remove.armed:hover { background: var(--p0); }
  .att-remove:disabled { opacity: 0.4; cursor: not-allowed; }

  /* Narrow viewport: stack the rail below the main content so all controls
     remain reachable without horizontal overflow. */
  @media (max-width: 960px) {
    .backdrop { padding: 0; }
    .surface { border-radius: 0; border: 0; }
    .grid {
      grid-template-columns: 1fr;
      grid-template-rows: auto auto;
      overflow-y: auto;
    }
    .main, .rail {
      overflow-y: visible;
    }
    .rail {
      border-left: 0;
      border-top: 1px solid rgba(255, 255, 255, 0.06);
    }
  }
</style>
