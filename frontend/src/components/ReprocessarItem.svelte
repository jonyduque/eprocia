<script>
  import { onMount } from 'svelte';
  import { OPCOES_OCR, OPCOES_LIMPEZA } from '../lib/atividade.js';

  export let id = '';
  // Valores iniciais: os da última execução do item, ou os da config atual.
  export let ocrInicial = 'normal';
  export let limpezaInicial = 'basic';
  // Recebe (ocr, limpeza); devolve uma Promise que rejeita se a fila recusar.
  export let aoConfirmar = async () => {};
  export let aoCancelar = () => {};

  let ocr = ocrInicial;
  let limpeza = limpezaInicial;
  let enviando = false;
  let primeiroCampo;

  onMount(() => primeiroCampo?.focus());

  async function confirmar() {
    if (enviando) return;
    enviando = true;
    try {
      await aoConfirmar(ocr, limpeza);
    } finally {
      enviando = false;
    }
  }

  function tecla(e) {
    if (e.key === 'Escape') {
      e.preventDefault();
      aoCancelar();
    }
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<form class="painel-reprocessar" {id} aria-label="Reprocessar com outras configurações" on:submit|preventDefault={confirmar} on:keydown={tecla}>
  <label>
    <span>OCR</span>
    <select bind:value={ocr} bind:this={primeiroCampo}>
      {#each OPCOES_OCR as o}
        <option value={o.valor}>{o.rotulo}</option>
      {/each}
    </select>
  </label>
  <label>
    <span>Higienização</span>
    <select bind:value={limpeza}>
      {#each OPCOES_LIMPEZA as o}
        <option value={o.valor}>{o.rotulo}</option>
      {/each}
    </select>
  </label>
  <div class="botoes">
    <button type="button" class="secundario" on:click={aoCancelar}>Cancelar</button>
    <button type="submit" class="primario" disabled={enviando}>Reprocessar</button>
  </div>
  <p class="nota">Vale só para este arquivo. O TXT anterior será substituído.</p>
</form>

<style>
  .painel-reprocessar {
    flex-basis: 100%;
    margin-left: 30px;
    padding: 8px;
    border-left: 2px solid var(--accent);
    border-radius: 4px;
    background-color: var(--bg-main);
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 8px 12px;
    font-size: var(--fs-meta);
  }

  label {
    display: flex;
    flex-direction: column;
    gap: 2px;
    color: var(--text-secondary);
    min-width: 120px;
    flex: 1 1 120px;
  }

  select {
    padding: 4px 8px;
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    background-color: var(--surface);
    color: var(--text-primary);
    font: inherit;
    font-size: var(--fs-base);
    cursor: pointer;
  }

  .botoes {
    display: flex;
    gap: 6px;
    margin-left: auto;
  }

  button {
    font: inherit;
    font-size: var(--fs-meta);
    font-weight: 600;
    padding: 5px 10px;
    border-radius: 4px;
    cursor: pointer;
    white-space: nowrap;
    transition: background-color var(--dur-fast) var(--ease-out), border-color var(--dur-fast) var(--ease-out);
  }

  .primario {
    border: none;
    background-color: var(--accent-strong);
    color: var(--on-accent);
  }

  .primario:hover {
    background-color: var(--accent-strong-hover);
  }

  .primario:disabled {
    opacity: 0.6;
    cursor: default;
  }

  .secundario {
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-primary);
  }

  .secundario:hover {
    background-color: var(--surface-hover);
  }

  .nota {
    flex-basis: 100%;
    margin: 0;
    color: var(--text-secondary);
  }

  @container (max-width: 420px) {
    .painel-reprocessar {
      margin-left: 0;
    }
  }
</style>
