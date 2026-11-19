<script>
  import Icone from './Icone.svelte';
  import CircleAlert from 'lucide-svelte/icons/circle-alert';
  import CircleCheck from 'lucide-svelte/icons/circle-check';
  import X from 'lucide-svelte/icons/x';
  import { fly, fade } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import { duracaoMovimento, DUR_FAST, DUR_BASE } from '../lib/aviso.js';

  // { texto, tipo } de criarAviso, com tipo 'success', 'error' ou 'aviso'
  // (atenção); null sem aviso.
  export let aviso = null;
  export let aoFechar = () => {};
  // Flutuante fica sobre o conteúdo, com sombra; senão ocupa o pé de quem o usa.
  export let flutuante = false;

  $: sucesso = aviso?.tipo === 'success';
  $: classe = aviso?.tipo === 'aviso' ? 'atencao' : sucesso ? 'sucesso' : 'erro';
</script>

<!-- Regiões sempre montadas: o leitor de tela só anuncia mudanças numa
     região que já existia antes da mensagem. Sucesso é educado; erro e
     atenção interrompem. -->
<div role="status" aria-live="polite" aria-atomic="true">
  {#if aviso && sucesso}
    <div
      class="aviso sucesso"
      class:flutuante
      in:fly={{ y: 8, duration: duracaoMovimento(DUR_BASE), easing: cubicOut }}
      out:fade={{ duration: duracaoMovimento(DUR_FAST) }}
    >
      <span class="aviso-icone"><Icone icone={CircleCheck} tamanho={12} /></span>
      <span class="aviso-texto">{aviso.texto}</span>
      <button class="aviso-fechar" on:click={aoFechar} aria-label="Fechar aviso" title="Fechar aviso">
        <Icone icone={X} tamanho={12} />
      </button>
    </div>
  {/if}
</div>
<div role="alert" aria-atomic="true">
  {#if aviso && !sucesso}
    <div
      class="aviso {classe}"
      class:flutuante
      in:fly={{ y: 8, duration: duracaoMovimento(DUR_BASE), easing: cubicOut }}
      out:fade={{ duration: duracaoMovimento(DUR_FAST) }}
    >
      <span class="aviso-icone"><Icone icone={CircleAlert} tamanho={12} /></span>
      <span class="aviso-texto">{aviso.texto}</span>
      <button class="aviso-fechar" on:click={aoFechar} aria-label="Fechar aviso" title="Fechar aviso">
        <Icone icone={X} tamanho={12} />
      </button>
    </div>
  {/if}
</div>

<style>
  .aviso {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0 8px 8px;
    padding: 4px 4px 4px 8px;
    font-size: var(--fs-meta);
    font-weight: 500;
    background-color: var(--surface);
    border: 1px solid currentColor;
    border-radius: 4px;
  }

  .aviso.flutuante {
    margin: 0;
    box-shadow: var(--shadow-sm);
  }

  .aviso.sucesso {
    color: var(--success-strong);
  }

  .aviso.erro {
    color: var(--error-strong);
  }

  /* Texto neutro: --warning não chega a 4,5:1 como texto pequeno no claro. */
  .aviso.atencao {
    color: var(--text-primary);
    background-color: var(--warning-soft);
    border-color: var(--warning-line);
  }

  .aviso.atencao .aviso-icone {
    color: var(--warning);
  }

  .aviso-icone {
    flex: none;
    display: inline-flex;
  }

  .aviso-texto {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .aviso-fechar {
    flex: none;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    padding: 0;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: inherit;
    cursor: pointer;
    transition: background-color var(--dur-fast) var(--ease-out);
  }

  .aviso-fechar:hover {
    background-color: var(--surface-hover);
  }

  .aviso.erro .aviso-fechar:hover {
    background-color: var(--error-soft);
  }

  .aviso.atencao .aviso-fechar:hover {
    background-color: var(--warning-line);
  }

  .aviso-fechar:focus-visible {
    outline: 2px solid currentColor;
    outline-offset: 1px;
  }
</style>
