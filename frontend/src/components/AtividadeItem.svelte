<script>
  import Icone from './Icone.svelte';
  import CircleAlert from 'lucide-svelte/icons/circle-alert';
  import CircleCheck from 'lucide-svelte/icons/circle-check';
  import Clock from 'lucide-svelte/icons/clock';
  import FileText from 'lucide-svelte/icons/file-text';
  import ScanText from 'lucide-svelte/icons/scan-text';
  import { ordenarDocs, resumoAtividade, segmentos, naoLido } from '../lib/atividade.js';

  // Instantâneo de etl.InstantaneoAtividade; null enquanto carrega e
  // 'nenhuma' quando o item foi cancelado antes de começar.
  export let atividade = null;
  export let id = '';
  // false quando o item já terminou: o painel mostra o resultado final.
  export let emAndamento = true;
  // Abre o reprocessamento da linha com OCR ligado (botão "Reprocessar com OCR").
  export let aoReprocessarComOCR = () => {};

  let mostrarConcluidos = false;

  const ROTULO_LEITURA = { ocr: 'via OCR', segundo_leitor: 'segundo leitor' };

  const ROTULO = {
    ocr: 'Em OCR',
    lendo: 'Lendo texto',
    aguardando: 'Aguardando vaga',
    concluido: 'Concluído',
    erro: 'Erro'
  };

  $: docs = ordenarDocs(atividade?.docs);
  $: resumo = resumoAtividade(atividade);
  $: limiteDocs = atividade?.limite_docs || 0;
  $: limiteOcr = atividade?.limite_ocr || 0;
  $: ocrAtivos = atividade?.ocr_ativos || 0;
  $: visiveis = mostrarConcluidos ? docs : docs.filter((d) => d.estado !== 'concluido');

  function pct(d) {
    if (!d.paginas) return d.estado === 'concluido' ? 100 : 0;
    return Math.min(100, Math.round((d.feitas / d.paginas) * 100));
  }
</script>

<div class="painel-atividade" {id} role="region" aria-label="Atividade dos documentos">
  {#if !atividade}
    <p class="vazio">Carregando a atividade...</p>
  {:else if atividade === 'nenhuma'}
    <p class="vazio">Sem registro dos documentos: este item não chegou a ser processado.</p>
  {:else}
    {#if emAndamento}
      <div class="medidores">
        <div class="medidor" title="Documentos processados ao mesmo tempo (limite: núcleos do processador - 1)">
          <span class="medidor-rotulo">Documentos em paralelo</span>
          <span class="vagas" aria-hidden="true">
            {#each segmentos(resumo.ativos, limiteDocs) as cheia}
              <span class="vaga" class:cheia></span>
            {/each}
          </span>
          <span class="medidor-valor font-tabular">{resumo.ativos}/{limiteDocs}</span>
        </div>
        <div class="medidor ocr" title="Processos de OCR (tesseract) rodando agora; cada um pode levar várias páginas">
          <span class="medidor-rotulo">Processos de OCR</span>
          <span class="vagas" aria-hidden="true">
            {#each segmentos(ocrAtivos, limiteOcr) as cheia}
              <span class="vaga" class:cheia></span>
            {/each}
          </span>
          <span class="medidor-valor font-tabular">{ocrAtivos}/{limiteOcr}</span>
        </div>
      </div>
    {/if}

    <div class="resumo font-tabular" role="status">
      {#if emAndamento}
        <span class="chip ativo">{resumo.ativos} em andamento</span>
        {#if resumo.paginasOcr > 0}
          <span class="chip ocr">{resumo.paginasOcr} pág. em OCR</span>
        {/if}
        <span class="chip">{resumo.aguardando} aguardando</span>
      {/if}
      <span class="chip ok">{resumo.concluidos} concluído{resumo.concluidos === 1 ? '' : 's'}</span>
      {#if resumo.naoLidos > 0}
        <span class="chip aviso">{resumo.naoLidos} não lido{resumo.naoLidos === 1 ? '' : 's'}</span>
      {/if}
      {#if resumo.erros > 0}
        <span class="chip erro">{resumo.erros} com erro</span>
      {/if}
      {#if resumo.concluidos > 0}
        <button
          type="button"
          class="alternar"
          aria-pressed={mostrarConcluidos}
          on:click={() => (mostrarConcluidos = !mostrarConcluidos)}
        >
          {mostrarConcluidos ? 'Ocultar concluídos' : 'Mostrar concluídos'}
        </button>
      {/if}
    </div>

    {#if resumo.semOCR > 0 && !emAndamento}
      <div class="acao-ocr">
        <span>{resumo.semOCR === 1 ? 'Um documento digitalizado ficou sem OCR.' : `${resumo.semOCR} documentos digitalizados ficaram sem OCR.`}</span>
        <button type="button" class="btn-ocr" on:click={aoReprocessarComOCR}>Reprocessar com OCR</button>
      </div>
    {/if}

    {#if visiveis.length > 0}
      <ul class="docs">
        {#each visiveis as d (d.i)}
          <li class="doc {d.estado}" class:nao-lido={naoLido(d)} title={naoLido(d) && d.detalhe ? `${d.nome}\n${d.detalhe}` : d.erro ? `${d.nome}\n${d.erro}` : d.nome}>
            <span class="doc-icone">
              {#if d.estado === 'ocr'}
                <span class="pulsa"><Icone icone={ScanText} tamanho={12} cor="var(--processing)" /></span>
              {:else if d.estado === 'lendo'}
                <Icone icone={FileText} tamanho={12} cor="var(--accent)" />
              {:else if d.estado === 'concluido'}
                <Icone icone={CircleCheck} tamanho={12} cor="var(--success)" />
              {:else if naoLido(d)}
                <Icone icone={CircleAlert} tamanho={12} cor="var(--warning)" />
              {:else if d.estado === 'erro'}
                <Icone icone={CircleAlert} tamanho={12} cor="var(--error)" />
              {:else}
                <Icone icone={Clock} tamanho={12} cor="var(--text-secondary)" />
              {/if}
              <span class="sr-only">{ROTULO[d.estado] ?? d.estado}:</span>
            </span>
            <span class="doc-nome">{d.nome}</span>
            {#if d.ocr > 0}
              <span class="doc-ocr font-tabular">OCR · {d.ocr} pág.</span>
            {/if}
            {#if ROTULO_LEITURA[d.leitura]}
              <span class="doc-leitura">{ROTULO_LEITURA[d.leitura]}</span>
            {/if}
            {#if d.estado === 'erro'}
              <span class="doc-erro" class:aviso={naoLido(d)}>{d.erro}</span>
            {:else}
              <span class="doc-barra" aria-hidden="true"><span style="width: {pct(d)}%"></span></span>
              <span class="doc-paginas font-tabular">{d.feitas}/{d.paginas}</span>
            {/if}
          </li>
        {/each}
      </ul>
    {:else}
      <p class="vazio">Todos os documentos foram concluídos.</p>
    {/if}
  {/if}
</div>

<style>
  .painel-atividade {
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
  }

  .vazio {
    margin: 0;
    color: var(--text-secondary);
  }

  .medidores {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 16px;
  }

  .medidor {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }

  .medidor-rotulo {
    color: var(--text-secondary);
    white-space: nowrap;
  }

  .vagas {
    display: inline-flex;
    gap: 2px;
  }

  .vaga {
    width: 8px;
    height: 10px;
    border-radius: 2px;
    background-color: var(--track);
    transition: background-color var(--dur-fast) var(--ease-out);
  }

  .vaga.cheia {
    background-color: var(--accent);
  }

  .medidor.ocr .vaga.cheia {
    background-color: var(--processing);
  }

  .medidor-valor {
    font-weight: 600;
  }

  .resumo {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
  }

  .chip {
    padding: 0 6px;
    border-radius: 3px;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    white-space: nowrap;
  }

  .chip.ativo {
    color: var(--accent-text);
    border-color: var(--accent-line);
    background-color: var(--accent-soft);
  }

  .chip.ocr {
    color: var(--processing);
    border-color: currentColor;
  }

  .chip.ok {
    color: var(--success);
    border-color: currentColor;
  }

  .chip.erro {
    color: var(--error);
    border-color: currentColor;
    background-color: var(--error-soft);
  }

  .chip.aviso {
    color: var(--warning);
    border-color: var(--warning-line);
    background-color: var(--warning-soft);
  }

  .acao-ocr {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 10px;
    color: var(--text-secondary);
  }

  .btn-ocr {
    padding: 1px 8px;
    border-radius: 4px;
    border: 1px solid var(--warning-line);
    background-color: var(--warning-soft);
    color: var(--warning);
    font: inherit;
    font-weight: 500;
    cursor: pointer;
  }

  .btn-ocr:hover {
    border-color: var(--warning);
  }

  .btn-ocr:focus-visible {
    outline: 2px solid var(--warning);
    outline-offset: 1px;
  }

  .alternar {
    margin-left: auto;
    background: transparent;
    border: none;
    padding: 0 4px;
    border-radius: 3px;
    font: inherit;
    color: var(--accent);
    cursor: pointer;
  }

  .alternar:hover {
    text-decoration: underline;
  }

  .docs {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 220px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .doc {
    display: flex;
    align-items: center;
    gap: 6px;
    min-height: 20px;
    padding: 0 4px;
    border-radius: 3px;
    min-width: 0;
  }

  .doc.ocr,
  .doc.lendo {
    background-color: var(--accent-faint);
  }

  .doc-icone {
    display: inline-flex;
    flex-shrink: 0;
  }

  .doc-nome {
    flex: 1 1 auto;
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .doc.concluido .doc-nome,
  .doc.aguardando .doc-nome {
    color: var(--text-secondary);
  }

  .doc-ocr {
    flex-shrink: 0;
    color: var(--processing);
    font-weight: 600;
  }

  .doc-leitura {
    flex-shrink: 0;
    color: var(--text-secondary);
    font-style: italic;
  }

  .doc-erro.aviso {
    color: var(--warning);
  }

  .doc-erro {
    flex: 0 1 50%;
    min-width: 0;
    color: var(--error);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .doc-barra {
    flex: 0 0 56px;
    height: 4px;
    border-radius: 2px;
    background-color: var(--track);
    overflow: hidden;
  }

  .doc-barra span {
    display: block;
    height: 100%;
    background-color: var(--processing);
    transition: width var(--dur-base) var(--ease-out);
  }

  .doc.concluido .doc-barra span {
    background-color: var(--success);
  }

  .doc-paginas {
    flex: 0 0 auto;
    min-width: 40px;
    text-align: right;
    color: var(--text-secondary);
  }

  .pulsa {
    display: inline-flex;
    animation: pulsar var(--dur-pulse) var(--ease-out) infinite alternate;
  }

  @keyframes pulsar {
    from { opacity: 1; }
    to { opacity: 0.35; }
  }

  @media (prefers-reduced-motion: reduce) {
    .pulsa {
      animation: none;
    }
  }

  .font-tabular {
    font-variant-numeric: tabular-nums;
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

  @container (max-width: 420px) {
    .painel-atividade {
      margin-left: 0;
    }

    .doc-barra {
      display: none;
    }
  }
</style>
