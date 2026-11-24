<script>
  import { fly } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import { duracaoMovimento, DUR_FAST } from '../lib/aviso.js';
  import { OPCOES_LIMPEZA, rotuloOpcao } from '../lib/atividade.js';
  import { reducaoLimpeza, formatarDuracao, formatarBytes, linhasAtencao } from '../lib/relatorio.js';
  import {
    ROTULO_PAINEL_RELATORIO,
    TITULO_PAGINAS,
    TITULO_ATENCAO,
    ROTULO_TEXTO_PDF,
    ROTULO_LIDA_OCR,
    ROTULO_EM_BRANCO,
    ROTULO_SEM_OCR,
    ROTULO_COM_ERRO,
    ROTULO_OCR_FALHOU_COM_RESERVA,
    ROTULO_TEMPO_TOTAL,
    ROTULO_TEMPO_OCR,
    NOTA_TEMPO_OCR,
    ROTULO_ESTIMATIVA_PAGINA,
    ROTULO_LIMPEZA,
    ROTULO_PAGINAS_REPETIDAS,
    ROTULO_TIPOS_OMITIDOS,
    textoReducao
  } from '../lib/textos.js';

  // QueueItem.relatorio (models.RelatorioZip) de um item concluído.
  export let relatorio = null;
  export let id = '';

  const ROTULO_CASO = {
    sem_ocr: ROTULO_SEM_OCR,
    com_erro: ROTULO_COM_ERRO,
    ocr_falhou_com_reserva: ROTULO_OCR_FALHOU_COM_RESERVA
  };

  $: p = relatorio?.paginas ?? {};
  $: tempo = relatorio?.tempo ?? {};
  $: ocr = relatorio?.ocr ?? {};
  $: limpeza = relatorio?.limpeza ?? {};
  $: tiposOmitidos = relatorio?.documentos?.tipos_omitidos ?? [];
  $: reducao = reducaoLimpeza(relatorio);
  $: atencao = linhasAtencao(relatorio);
  $: categorias = [
    { rotulo: ROTULO_TEXTO_PDF, n: p.lida_nativa || 0 },
    { rotulo: ROTULO_LIDA_OCR, n: p.lida_ocr || 0 },
    { rotulo: ROTULO_EM_BRANCO, n: p.vazia || 0 },
    { rotulo: ROTULO_SEM_OCR, n: p.sem_ocr || 0, aviso: true },
    { rotulo: ROTULO_COM_ERRO, n: p.com_erro || 0, aviso: true }
  ];
</script>

<div class="painel-relatorio" {id} role="region" aria-label={ROTULO_PAINEL_RELATORIO} in:fly={{ y: -4, duration: duracaoMovimento(DUR_FAST), easing: cubicOut }}>
  <section>
    <h3>{TITULO_PAGINAS}</h3>
    <dl class="grade font-tabular">
      {#each categorias as c}
        <dt>{c.rotulo}</dt>
        <dd class:aviso={c.aviso && c.n > 0}>{c.n}</dd>
      {/each}
      {#if p.ocr_falhou_com_reserva > 0}
        <dt>{ROTULO_OCR_FALHOU_COM_RESERVA}</dt>
        <dd class="aviso">{p.ocr_falhou_com_reserva}</dd>
      {/if}
      {#if p.duplicadas > 0}
        <dt>{ROTULO_PAGINAS_REPETIDAS}</dt>
        <dd>{p.duplicadas}</dd>
      {/if}
    </dl>
  </section>

  <section>
    <dl class="grade font-tabular">
      <dt>{ROTULO_TEMPO_TOTAL}</dt>
      <dd>{formatarDuracao(tempo.total_ms)}</dd>
      {#if tempo.ocr_trabalho_ms > 0}
        <dt>{ROTULO_TEMPO_OCR}</dt>
        <dd>{formatarDuracao(tempo.ocr_trabalho_ms)} <span class="nota">{NOTA_TEMPO_OCR}</span></dd>
      {/if}
      {#if ocr.pagina_estimada_ms > 0}
        <dt>{ROTULO_ESTIMATIVA_PAGINA}</dt>
        <dd>{formatarDuracao(ocr.pagina_estimada_ms)}</dd>
      {/if}
      <dt>{ROTULO_LIMPEZA}</dt>
      <dd>
        {rotuloOpcao(OPCOES_LIMPEZA, limpeza.nivel)}{#if reducao !== null}: {formatarBytes(limpeza.bytes_antes)} → {formatarBytes(limpeza.bytes_depois)} ({textoReducao(reducao)}){/if}
      </dd>
      {#if tiposOmitidos.length > 0}
        <dt>{ROTULO_TIPOS_OMITIDOS}</dt>
        <dd class="texto">{tiposOmitidos.join(', ')}</dd>
      {/if}
    </dl>
  </section>

  {#if atencao.length > 0}
    <section>
      <h3>{TITULO_ATENCAO}</h3>
      <ul class="atencao">
        {#each atencao as d}
          <li>
            <span class="doc-nome" title={d.nome}>{d.nome}</span>
            {#each d.casos as c}
              <span class="caso">{ROTULO_CASO[c.tipo]}: <span class="font-tabular">{c.paginas}</span></span>
            {/each}
          </li>
        {/each}
      </ul>
    </section>
  {/if}
</div>

<style>
  .painel-relatorio {
    flex-basis: 100%;
    margin-left: 30px;
    padding: 8px;
    border-left: 2px solid var(--accent);
    border-radius: 4px;
    background-color: var(--bg-main);
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-size: var(--fs-meta);
    color: var(--text-primary);
    min-width: 0;
    user-select: text;
    -webkit-user-select: text;
  }

  section {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }

  h3 {
    margin: 0;
    font-size: var(--fs-meta);
    font-weight: 600;
    color: var(--text-secondary);
  }

  .grade {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: 2px 12px;
    margin: 0;
  }

  dt {
    color: var(--text-secondary);
  }

  dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  dd.aviso {
    color: var(--warning-strong);
    font-weight: 600;
  }

  .nota {
    display: block;
    color: var(--text-secondary);
  }

  .atencao {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 220px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .atencao li {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .doc-nome {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .caso {
    padding-left: 12px;
    color: var(--warning-strong);
    overflow-wrap: anywhere;
  }

  .font-tabular {
    font-variant-numeric: tabular-nums;
  }

  @container (max-width: 420px) {
    .painel-relatorio {
      margin-left: 0;
    }

    .grade {
      grid-template-columns: minmax(0, 1fr);
    }

    dd {
      padding-left: 8px;
    }
  }
</style>
