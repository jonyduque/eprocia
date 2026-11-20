<script>
  import Icone from './Icone.svelte';
  import Check from 'lucide-svelte/icons/check';
  import { fade } from 'svelte/transition';
  import { duracaoMovimento, DUR_FAST } from '../lib/aviso.js';

  export let campo = '';
  // { campo, n } vindo de criarConfirmacao; n muda a cada save e refaz a entrada.
  export let confirmado = null;
</script>

{#if confirmado?.campo === campo}
  <span class="salvo" out:fade={{ duration: duracaoMovimento(DUR_FAST) }}>
    {#key confirmado.n}
      <span class="salvo-icone" in:fade={{ duration: duracaoMovimento(DUR_FAST) }}><Icone icone={Check} tamanho={12} /></span>
    {/key}
  </span>
{/if}

<style>
  /* Check de confirmação: 12px ao lado do rótulo, sem mexer na linha. */
  .salvo {
    display: inline-flex;
    vertical-align: middle;
    margin-left: 2px;
    color: var(--success);
  }

  .salvo-icone {
    display: inline-flex;
  }
</style>
