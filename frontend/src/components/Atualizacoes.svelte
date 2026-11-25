<script>
  // Conteúdo do bloco "Atualizações" das Configurações (o cabeçalho colapsável
  // fica no Settings). O estado vem do App (EstadoAtualizacao e o evento
  // atualizacao:estado); a consulta à rede só acontece pelo clique em Procurar.
  import { tick } from 'svelte';
  import Icone from './Icone.svelte';
  import Download from 'lucide-svelte/icons/download';
  import ExternalLink from 'lucide-svelte/icons/external-link';
  import LoaderCircle from 'lucide-svelte/icons/loader-circle';
  import RefreshCw from 'lucide-svelte/icons/refresh-cw';
  import RotateCw from 'lucide-svelte/icons/rotate-cw';
  import X from 'lucide-svelte/icons/x';
  import { acoes, fraseModo, mostraProgresso, textoFase } from '../lib/atualizacao.js';
  import { ATUALIZACAO as T } from '../lib/textos.js';
  import {
    AbrirNovidades,
    AbrirPaginaVersoes,
    CancelarAtualizacao,
    InstalarAtualizacao,
    ReiniciarAtualizacao,
    VerificarAtualizacao
  } from '../wailsjs/go/main/App.js';

  export let estado = null;
  // Algum item pendente ou em processamento: muda os rótulos de Atualizar e Reiniciar.
  export let filaOcupada = false;
  // O mesmo avisar(texto, tipo) do Settings: erro de um binding vai para o pé do painel.
  export let avisar = () => {};

  const FASES_EM_CURSO = ['verificando', 'conferindo', 'instalando', 'reiniciando', 'abrindo_instalador'];

  $: texto = textoFase(estado);
  $: botoes = acoes(estado, filaOcupada);
  $: progresso = mostraProgresso(estado);
  $: frase = fraseModo(estado);
  $: emCurso = FASES_EM_CURSO.includes(estado?.fase);
  $: erro = estado?.fase === 'erro';
  $: pct = Math.max(0, Math.min(100, Math.round(estado?.progresso || 0)));

  // O texto da fase é anunciado uma vez por fase (não a cada % do download).
  let faseAnunciada = null;
  let anuncio = '';
  $: if ((estado?.fase ?? null) !== faseAnunciada) {
    faseAnunciada = estado?.fase ?? null;
    anuncio = texto;
  }

  async function executar(acao) {
    try {
      await acao();
    } catch (err) {
      const msg = String(err?.message ?? err);
      // A fase erro com a mesma frase já aparece no bloco: o aviso a repetiria.
      await tick();
      if (estado?.fase === 'erro' && estado.mensagem === msg) return;
      avisar(msg, 'error');
    }
  }

  function procurar() {
    return executar(() => VerificarAtualizacao());
  }

  function atualizar() {
    return executar(() => InstalarAtualizacao());
  }

  function cancelar() {
    return executar(() => CancelarAtualizacao());
  }

  function verNovidades() {
    return executar(() => AbrirNovidades());
  }

  function abrirPaginaVersoes() {
    return executar(() => AbrirPaginaVersoes());
  }

  function reiniciar() {
    return executar(() => ReiniciarAtualizacao());
  }
</script>

<div class="atualizacoes">
  {#if estado?.atual}
    <p class="help-text">{T.versao(estado.atual)}</p>
  {/if}

  {#if texto}
    <p class="fase" class:erro>
      {#if emCurso}
        <span class="spinner" aria-hidden="true"><Icone icone={LoaderCircle} tamanho={12} cor="var(--accent)" /></span>
      {/if}
      <span>{texto}</span>
    </p>
  {/if}

  {#if progresso}
    <div
      class="trilho"
      role="progressbar"
      aria-valuemin="0"
      aria-valuemax="100"
      aria-valuenow={pct}
      aria-label={T.titulo}
    >
      <div class="barra" style:--p={pct / 100}></div>
    </div>
  {/if}

  {#if frase}
    <p class="help-text">{frase}</p>
  {/if}

  {#if botoes.procurar || botoes.atualizar || botoes.reiniciar || botoes.cancelar || botoes.novidades || botoes.paginaVersoes}
    <div class="botoes">
      {#if botoes.atualizar}
        <button type="button" class="primario" on:click={atualizar}>
          <Icone icone={Download} tamanho={12} />
          {botoes.atualizar}
        </button>
      {/if}
      {#if botoes.reiniciar}
        <button type="button" class="primario" on:click={reiniciar}>
          <Icone icone={RotateCw} tamanho={12} />
          {botoes.reiniciar}
        </button>
      {/if}
      {#if botoes.procurar}
        <button type="button" class="secundario" on:click={procurar}>
          <Icone icone={RefreshCw} tamanho={12} />
          {T.procurar}
        </button>
      {/if}
      {#if botoes.novidades}
        <button type="button" class="secundario" on:click={verNovidades}>
          <Icone icone={ExternalLink} tamanho={12} />
          {T.novidades}
        </button>
      {/if}
      {#if botoes.paginaVersoes}
        <button type="button" class="secundario" on:click={abrirPaginaVersoes}>
          <Icone icone={ExternalLink} tamanho={12} />
          {T.paginaVersoes}
        </button>
      {/if}
      {#if botoes.cancelar}
        <button type="button" class="secundario" on:click={cancelar}>
          <Icone icone={X} tamanho={12} />
          {T.cancelar}
        </button>
      {/if}
    </div>
  {/if}

  <!-- Sempre montada: o leitor de tela só anuncia mudanças numa região que
       já existia antes da mensagem. -->
  <div class="sr-only" role="status" aria-live="polite">{anuncio}</div>
</div>

<style>
  .atualizacoes {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .help-text {
    font-size: var(--fs-meta);
    color: var(--text-secondary);
    margin: 0;
  }

  .fase {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0;
    font-size: var(--fs-base);
    color: var(--text-primary);
  }

  .fase.erro {
    color: var(--error-strong);
  }

  .spinner {
    display: inline-flex;
    flex-shrink: 0;
    animation: spin var(--dur-spin) linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* Mesma barra da linha da fila: trilho --track, preenchimento --processing. */
  .trilho {
    height: 4px;
    border-radius: 2px;
    background-color: var(--track);
    overflow: hidden;
  }

  .barra {
    width: 100%;
    height: 100%;
    background-color: var(--processing);
    transform: scaleX(var(--p));
    transform-origin: left center;
    transition: transform var(--dur-base) var(--ease-out);
  }

  /* Alto contraste do Windows: o fundo vira Canvas e a barra sumiria. */
  @media (forced-colors: active) {
    .trilho {
      border: 1px solid CanvasText;
    }

    .barra {
      forced-color-adjust: none;
      background-color: Highlight;
    }
  }

  /* A 280px os botões quebram a linha em vez de cortar o rótulo. */
  .botoes {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  button {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    max-width: 100%;
    font-family: inherit;
    font-size: var(--fs-base);
    font-weight: 500;
    padding: 6px 10px;
    border-radius: 4px;
    cursor: pointer;
    text-align: left;
    white-space: normal;
  }

  .primario {
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

  .secundario {
    border: 1px solid var(--border-strong);
    background: transparent;
    color: var(--text-primary);
  }

  .secundario:hover {
    background-color: var(--surface-hover);
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
</style>
