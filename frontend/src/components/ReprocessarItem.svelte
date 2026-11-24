<script>
  import { onMount } from 'svelte';
  import { fly } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import { duracaoMovimento, DUR_FAST } from '../lib/aviso.js';
  import { OPCOES_OCR, OPCOES_LIMPEZA } from '../lib/atividade.js';
  import {
    AJUDA_FORCAR_OCR_REPROCESSAR,
    AJUDA_FORCAR_OCR_SEM_OCR,
    ROTULO_FORCAR_OCR_REPROCESSAR
  } from '../lib/textos.js';
  import Segmentado from './Segmentado.svelte';
  import Icone from './Icone.svelte';
  import RotateCcw from 'lucide-svelte/icons/rotate-ccw';
  import ScanText from 'lucide-svelte/icons/scan-text';
  import Sparkles from 'lucide-svelte/icons/sparkles';

  export let id = '';
  // Valores iniciais: os da última execução do item, ou os da config atual.
  export let ocrInicial = 'normal';
  export let limpezaInicial = 'basic';
  // O valor da config global (forcar_ocr); vale só para esta execução.
  export let forcarInicial = false;
  // Recebe (ocr, limpeza, forcar); devolve uma Promise que rejeita se a fila recusar.
  export let aoConfirmar = async () => {};
  export let aoCancelar = () => {};

  let ocr = ocrInicial;
  let limpeza = limpezaInicial;
  let forcar = forcarInicial;
  let enviando = false;
  let primeiroCampo;

  onMount(() => primeiroCampo?.focar());

  async function confirmar() {
    if (enviando) return;
    enviando = true;
    try {
      await aoConfirmar(ocr, limpeza, forcar);
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
<form class="painel-reprocessar" in:fly={{ y: -4, duration: duracaoMovimento(DUR_FAST), easing: cubicOut }} {id} aria-label="Reprocessar com outras configurações" on:submit|preventDefault={confirmar} on:keydown={tecla}>
  <div class="campo">
    <Segmentado nome="{id}-ocr" legenda="OCR" icone={ScanText} discreto opcoes={OPCOES_OCR} bind:valor={ocr} bind:this={primeiroCampo} />
  </div>
  <div class="campo">
    <Segmentado nome="{id}-limpeza" legenda="Higienização" icone={Sparkles} discreto opcoes={OPCOES_LIMPEZA} bind:valor={limpeza} />
  </div>
  <!-- Com OCR desligado fica desabilitada e o valor não muda (o pipeline o ignora). -->
  <div class="campo-forcar">
    <label class="caixa">
      <input
        type="checkbox"
        bind:checked={forcar}
        disabled={ocr === 'none'}
        aria-describedby="{id}-ajuda-forcar"
      />
      <span>{ROTULO_FORCAR_OCR_REPROCESSAR}</span>
    </label>
    <p class="ajuda" id="{id}-ajuda-forcar">{ocr === 'none' ? AJUDA_FORCAR_OCR_SEM_OCR : AJUDA_FORCAR_OCR_REPROCESSAR}</p>
  </div>
  <div class="botoes">
    <button type="button" class="secundario" on:click={aoCancelar}>Cancelar</button>
    <button type="submit" class="primario" disabled={enviando}>
      <Icone icone={RotateCcw} tamanho={12} />
      Reprocessar
    </button>
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

  /* 4 segmentos de rótulo inteiro pedem uns 220px; abaixo disso quebra a linha. */
  .campo {
    min-width: 0;
    flex: 1 1 220px;
  }

  .campo-forcar {
    flex-basis: 100%;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .caixa {
    display: flex;
    align-items: flex-start;
    gap: 6px;
    cursor: pointer;
    color: var(--text-primary);
  }

  .caixa input {
    margin: 1px 0 0;
    flex-shrink: 0;
    accent-color: var(--accent);
  }

  .caixa:has(input:disabled) {
    cursor: default;
    color: var(--text-secondary);
  }

  .ajuda {
    margin: 0 0 0 19px;
    color: var(--text-secondary);
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
  }

  .primario {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    border: none;
    background-color: var(--accent-strong);
    color: var(--on-accent);
  }

  .primario:hover {
    background-color: var(--accent-strong-hover);
  }

  /* Com 1px o anel encosta no fundo do botão, da mesma cor (--accent-strong). */
  .primario:focus-visible {
    outline-offset: 2px;
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
