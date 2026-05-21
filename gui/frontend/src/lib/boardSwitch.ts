export interface BoardSwitchCoordinator {
  open(projectRoot: string): Promise<void>;
  handleOpenedEvent(projectRoot: string, activeRoot?: string | null): Promise<boolean>;
  isOpening(): boolean;
}

interface BoardSwitchOptions {
  openBoard: (projectRoot: string) => Promise<void>;
  onCommitted: (projectRoot: string) => Promise<void> | void;
  onActivated?: (projectRoot: string) => Promise<void> | void;
}

export function createBoardSwitchCoordinator({
  openBoard,
  onCommitted,
  onActivated,
}: BoardSwitchOptions): BoardSwitchCoordinator {
  let sequence = 0;
  let openingSequence = 0;
  let committedSequence = 0;
  let committedRoot = '';
  let openingTarget = '';
  let deferredCommittedRoot = '';
  let deferredCompletedRoot = '';
  let allowStaleDirectReconcile = false;
  const directOpenSequences = new Map<string, number>();

  async function commit(
    seq: number,
    projectRoot: string,
    beforeCommitted?: (projectRoot: string) => Promise<void> | void,
  ): Promise<boolean> {
    if (seq !== sequence) return false;
    if (committedSequence === seq) return true;
    committedSequence = seq;
    committedRoot = projectRoot;
    openingSequence = 0;
    openingTarget = '';
    deferredCommittedRoot = '';
    deferredCompletedRoot = '';
    allowStaleDirectReconcile = false;
    if (beforeCommitted) await beforeCommitted(projectRoot);
    await onCommitted(projectRoot);
    return true;
  }

  async function activate(projectRoot: string): Promise<void> {
    if (onActivated) await onActivated(projectRoot);
  }

  async function commitActivated(seq: number, projectRoot: string): Promise<boolean> {
    return await commit(seq, projectRoot, activate);
  }

  return {
    async open(projectRoot: string): Promise<void> {
      const seq = ++sequence;
      openingSequence = seq;
      committedSequence = 0;
      openingTarget = projectRoot;
      deferredCommittedRoot = '';
      deferredCompletedRoot = '';
      allowStaleDirectReconcile = false;
      directOpenSequences.set(projectRoot, seq);
      try {
        await openBoard(projectRoot);
      } catch (err) {
        directOpenSequences.delete(projectRoot);
        if (seq === sequence) {
          openingSequence = 0;
          openingTarget = '';
          allowStaleDirectReconcile = true;
          if (deferredCommittedRoot) {
            const root = deferredCommittedRoot;
            await commitActivated(++sequence, root);
            directOpenSequences.delete(root);
          } else if (deferredCompletedRoot) {
            const root = deferredCompletedRoot;
            await commit(++sequence, root);
            directOpenSequences.delete(root);
          }
        }
        throw err;
      }
      const canReconcileDirect =
        seq !== sequence && allowStaleDirectReconcile && directOpenSequences.get(projectRoot) === seq;
      const commitSeq = canReconcileDirect ? ++sequence : seq;
      if (await commit(commitSeq, projectRoot)) {
        directOpenSequences.delete(projectRoot);
      } else if (seq !== sequence && openingSequence !== 0 && directOpenSequences.get(projectRoot) === seq) {
        deferredCompletedRoot = projectRoot;
      }
    },

    async handleOpenedEvent(projectRoot: string, activeRoot: string | null = ''): Promise<boolean> {
      if (activeRoot === null) return false;
      const root = projectRoot || activeRoot;
      if (!root) return false;
      if (activeRoot && root !== activeRoot) return false;
      const directSeq = directOpenSequences.get(root);
      if (openingSequence !== 0 && root !== openingTarget) {
        if (directSeq !== undefined && activeRoot === root) deferredCommittedRoot = root;
        return false;
      }
      if (directSeq !== undefined && directSeq < sequence) {
        if (allowStaleDirectReconcile && activeRoot === root) {
          const committed = await commitActivated(++sequence, root);
          directOpenSequences.delete(root);
          return committed;
        }
        directOpenSequences.delete(root);
        return false;
      }
      if (root === committedRoot) {
        directOpenSequences.delete(root);
        await activate(root);
        return true;
      }
      if (directSeq !== undefined && committedSequence === directSeq) {
        directOpenSequences.delete(root);
        await activate(root);
        return true;
      }
      const seq = openingSequence !== 0 && root === openingTarget ? openingSequence : ++sequence;
      const committed = await commitActivated(seq, root);
      if (committed) {
        if (directSeq === seq) directOpenSequences.delete(root);
      }
      return committed;
    },

    isOpening(): boolean {
      return openingSequence !== 0;
    },
  };
}

export function shouldAcceptBoardReload(visibleRoot: string, activeRoot: string | null): boolean {
  return visibleRoot !== '' && activeRoot !== null && visibleRoot === activeRoot;
}
