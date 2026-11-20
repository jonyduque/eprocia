<script>
  import Icone from './Icone.svelte';
  // Escolha entre poucas opções curtas e ordenadas (OPCOES_OCR, OPCOES_LIMPEZA).
  // Rádios nativos dão setas, Home/End e "2 de 4" ao leitor de tela sem ARIA manual.
  export let id = '';
  export let nome = '';
  export let legenda = '';
  export let opcoes = [];
  export let valor = '';
  export let idAjuda = '';
  // Legenda menor, em --text-secondary (painel Reprocessar da linha).
  export let discreto = false;
  // Ícone da configuração (DESIGN.md §4), à esquerda da legenda.
  export let icone = null;
  // Opção sob o ponteiro (null fora da faixa), para a ajuda descrever antes de gravar.
  export let previa = null;
  export let aoMudar = () => {};

  let faixa;

  export function focar() {
    (faixa?.querySelector('input:checked') ?? faixa?.querySelector('input'))?.focus();
  }
</script>

<fieldset {id} class="segmentado" class:discreto aria-describedby={idAjuda || undefined}>
  <legend>
    {#if icone}<span class="icone-legenda"><Icone {icone} tamanho={discreto ? 12 : 14} /></span>{/if}{legenda}<slot name="legenda" />
  </legend>
  <div class="faixa" style:--n={opcoes.length} bind:this={faixa}>
    {#each opcoes as o (o.valor)}
      <label
        class="segmento"
        class:selecionado={valor === o.valor}
        on:pointerenter={() => (previa = o.valor)}
        on:pointerleave={() => (previa = null)}
      >
        <input
          type="radio"
          class="sr-only"
          name={nome || id}
          value={o.valor}
          bind:group={valor}
          on:change={() => aoMudar(o.valor)}
        />
        <span>{o.rotulo}</span>
      </label>
    {/each}
  </div>
</fieldset>

<style>
  .segmentado {
    border: 0;
    margin: 0;
    padding: 0;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  legend {
    padding: 0;
    margin-bottom: 4px;
    font-size: var(--fs-base);
    font-weight: 500;
    color: var(--text-primary);
  }

  .icone-legenda {
    display: inline-flex;
    vertical-align: -2px;
    margin-right: 6px;
    color: var(--text-secondary);
  }

  .discreto legend {
    margin-bottom: 2px;
    font-size: var(--fs-meta);
    font-weight: 400;
    color: var(--text-secondary);
  }

  /* overflow: hidden recorta os cantos dos segmentos; por isso o anel de foco
     fica para dentro do segmento (outline-offset negativo). */
  .faixa {
    display: grid;
    grid-template-columns: repeat(var(--n), minmax(0, 1fr));
    height: 28px;
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    overflow: hidden;
    background-color: var(--bg-main);
  }

  .segmento {
    display: flex;
    align-items: center;
    justify-content: center;
    min-width: 0;
    padding: 0 4px;
    font-size: var(--fs-meta);
    font-weight: 500;
    color: var(--text-primary);
    cursor: pointer;
    user-select: none;
    transition: background-color var(--dur-fast) var(--ease-out), color var(--dur-fast) var(--ease-out);
  }

  .segmento span {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .segmento + .segmento {
    border-left: 1px solid var(--border);
  }

  .segmento:hover {
    background-color: var(--surface-hover);
  }

  .segmento.selecionado {
    background-color: var(--accent-soft);
    color: var(--accent-text);
    font-weight: 600;
  }

  .segmento:has(input:focus-visible) {
    outline: 2px solid var(--focus-ring);
    outline-offset: -2px;
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  @media (forced-colors: active) {
    .segmento.selecionado {
      background-color: Highlight;
      color: HighlightText;
      forced-color-adjust: none;
    }
  }
</style>
