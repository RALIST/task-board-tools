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
  const directOpenSequences = new Map<string, number>();

  async function commit(seq: number, projectRoot: string): Promise<boolean> {
    if (seq !== sequence) return false;
    if (committedSequence === seq) return true;
    committedSequence = seq;
    committedRoot = projectRoot;
    openingSequence = 0;
    openingTarget = '';
    await onCommitted(projectRoot);
    return true;
  }

  async function activate(projectRoot: string): Promise<void> {
    if (onActivated) await onActivated(projectRoot);
  }

  return {
    async open(projectRoot: string): Promise<void> {
      const seq = ++sequence;
      openingSequence = seq;
      committedSequence = 0;
      openingTarget = projectRoot;
      directOpenSequences.set(projectRoot, seq);
      try {
        await openBoard(projectRoot);
      } catch (err) {
        if (seq === sequence) {
          openingSequence = 0;
          openingTarget = '';
        }
        directOpenSequences.delete(projectRoot);
        throw err;
      }
      if (await commit(seq, projectRoot)) {
        directOpenSequences.delete(projectRoot);
      }
    },

    async handleOpenedEvent(projectRoot: string, activeRoot: string | null = ''): Promise<boolean> {
      if (activeRoot === null) return false;
      const root = projectRoot || activeRoot;
      if (!root) return false;
      if (activeRoot && root !== activeRoot) return false;
      if (openingSequence !== 0 && root !== openingTarget) return false;
      const directSeq = directOpenSequences.get(root);
      if (directSeq !== undefined && directSeq < sequence) {
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
      const committed = await commit(seq, root);
      if (committed) {
        if (directSeq === seq) directOpenSequences.delete(root);
        await activate(root);
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
