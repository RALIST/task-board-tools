<script lang="ts">
  import CreateTaskDialog from './CreateTaskDialog.svelte';

  interface Props {
    open: boolean;
    onClose: () => void;
    onCreated?: (id: string) => void;
  }

  let { open, onClose, onCreated }: Props = $props();
  let dirty = $state(false);

  export function getDirty() {
    return dirty;
  }
</script>

<CreateTaskDialog {open} {onClose} {onCreated} bind:dirty />
