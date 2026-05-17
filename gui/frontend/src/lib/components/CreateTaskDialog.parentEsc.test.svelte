<script lang="ts">
  import CreateTaskDialog from './CreateTaskDialog.svelte';

  interface Props {
    onClose: () => void;
    onCreated?: (id: string) => void;
    confirmFn?: () => boolean;
  }

  let { onClose, onCreated, confirmFn }: Props = $props();

  let open = $state(true);
  let dirty = $state(false);
  let confirmCalls = $state(0);

  export function getDirty() {
    return dirty;
  }

  export function getOpen() {
    return open;
  }

  export function getConfirmCalls() {
    return confirmCalls;
  }

  function tryCloseCreate() {
    if (dirty) {
      confirmCalls += 1;
      const ok = confirmFn ? confirmFn() : window.confirm('Discard this unsaved task?');
      if (!ok) return;
    }
    open = false;
    onClose();
  }

  function onGlobalKeydown(ev: KeyboardEvent) {
    if (!open) return;
    if (ev.key === 'Escape') {
      ev.preventDefault();
      tryCloseCreate();
    }
  }
</script>

<svelte:window onkeydown={onGlobalKeydown} />

<CreateTaskDialog
  {open}
  {onClose}
  {onCreated}
  bind:dirty />
