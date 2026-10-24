<script>
  import Icone from './Icone.svelte';
  import Archive from 'lucide-svelte/icons/archive';
  import Check from 'lucide-svelte/icons/check';
  import CircleAlert from 'lucide-svelte/icons/circle-alert';
  import CircleCheck from 'lucide-svelte/icons/circle-check';
  import CirclePause from 'lucide-svelte/icons/circle-pause';
  import CircleQuestionMark from 'lucide-svelte/icons/circle-question-mark';
  import CircleSlash from 'lucide-svelte/icons/circle-slash';
  import Clock from 'lucide-svelte/icons/clock';
  import Copy from 'lucide-svelte/icons/copy';
  import FolderOpen from 'lucide-svelte/icons/folder-open';
  import List from 'lucide-svelte/icons/list';
  import ListX from 'lucide-svelte/icons/list-x';
  import LoaderCircle from 'lucide-svelte/icons/loader-circle';
  import Pause from 'lucide-svelte/icons/pause';
  import Play from 'lucide-svelte/icons/play';
  import Trash2 from 'lucide-svelte/icons/trash-2';
  import X from 'lucide-svelte/icons/x';
  import {
    OpenDirectory,
    CopyOutput,
    CancelQueueItem,
    RemoveQueueItem,
    PauseQueue,
    ResumeQueue,
    ClearQueue
  } from '../wailsjs/go/main/App.js';
  import { onDestroy, tick } from 'svelte';
  import { fly, fade } from 'svelte/transition';
  import { flip } from 'svelte/animate';
  import { cubicOut } from 'svelte/easing';
  import { estadoItem, avisoFila } from '../lib/estado.js';
  import { criarAviso, duracaoMovimento, DUR_FAST, DUR_BASE } from '../lib/aviso.js';
  import { formatarHora, filtrarItens, alvoFoco, escolherAcao, criarMovimentoFila } from '../lib/fila.js';

  export let items = [];
  export let isPaused = false;

  let filterText = '';

  const ROTULO_ESTADO = {
    processando: 'Em processamento',
    pausado: 'Pausado',
    aguardando: 'Aguardando',
    sucesso: 'Concluído',
    erro: 'Erro',
    cancelado: 'Cancelado'
  };

  $: aviso = avisoFila(items, isPaused);

  $: filteredItems = filtrarItens(items, filterText);

  const mov = criarMovimentoFila();
  $: mov.atualizar(items, filterText);

  let detalhesAbertos = {};
  function alternarDetalhes(id) {
    detalhesAbertos = { ...detalhesAbertos, [id]: !detalhesAbertos[id] };
  }

  // Cancelar/Remover/Limpar Fila somem com a linha (ou trocam de botão) quando
  // o queue:update chega; sem isto o foco cai no body.
  let tituloEl;
  let listaEl;
  let focoPendente = null;

  function guardarFoco(id, evento) {
    const botao = evento?.currentTarget ?? null;
    focoPendente = {
      id,
      botao,
      acao: botao?.dataset?.acao ?? null,
      idsAntes: filteredItems.map((i) => i.id),
    };
  }

  function linhaDe(id) {
    if (!listaEl) return null;
    for (const linha of listaEl.querySelectorAll('.item-row[data-id]')) {
      if (linha.dataset.id === String(id)) return linha;
    }
    return null;
  }

  async function devolverFoco() {
    await tick();
    const p = focoPendente;
    if (!p) return;
    const ids = filteredItems.map((i) => i.id);
    const sumiu = p.id == null || !ids.includes(p.id);
    if (!sumiu && p.botao?.isConnected) return;
    focoPendente = null;
    const ativo = document.activeElement;
    // O usuário já levou o foco para outro lugar: não puxar de volta.
    if (ativo && ativo !== document.body && ativo !== p.botao && ativo.isConnected) return;
    const alvo = alvoFoco(p.idsAntes, p.id, ids);
    const linha = alvo == null ? null : linhaDe(alvo);
    if (linha) {
      const botoes = [...linha.querySelectorAll('.actions-col button[data-acao]')];
      const acao = escolherAcao(botoes.map((b) => b.dataset.acao), p.acao);
      const botao = botoes.find((b) => b.dataset.acao === acao);
      if (botao) {
        botao.focus();
        return;
      }
    }
    tituloEl?.focus();
  }

  $: if (focoPendente) devolverFoco(filteredItems);

  async function handleOpenDir(path) {
    try {
      await OpenDirectory(path);
    } catch (err) {
      console.error('Erro ao abrir pasta:', err);
    }
  }

  let copiadoId = null;
  let copiadoTimer;
  let erroCopia = '';

  const avisoCopia = criarAviso((a) => {
    erroCopia = a ? a.texto : '';
  });

  onDestroy(() => {
    avisoCopia.limpar();
    clearTimeout(copiadoTimer);
  });

  async function handleCopy(item) {
    try {
      await CopyOutput(item.output_path);
      copiadoId = item.id;
      clearTimeout(copiadoTimer);
      copiadoTimer = setTimeout(() => { copiadoId = null; }, 1500);
    } catch (err) {
      console.error('Erro ao copiar o TXT:', err);
      avisoCopia.mostrar('Erro ao copiar o TXT: ' + (err?.message ?? err), 'error');
    }
  }

  async function handlePause() {
    try {
      await PauseQueue();
    } catch (err) {
      console.error('Erro ao pausar:', err);
    }
  }

  async function handleResume() {
    try {
      await ResumeQueue();
    } catch (err) {
      console.error('Erro ao retomar:', err);
    }
  }

  async function handleCancel(id, evento) {
    guardarFoco(id, evento);
    try {
      await CancelQueueItem(id);
    } catch (err) {
      focoPendente = null;
      console.error('Erro ao cancelar:', err);
    }
  }

  async function handleRemove(id, evento) {
    guardarFoco(id, evento);
    try {
      await RemoveQueueItem(id);
    } catch (err) {
      focoPendente = null;
      console.error('Erro ao remover:', err);
    }
  }

  async function handleClear(evento) {
    guardarFoco(null, evento);
    try {
      await ClearQueue();
    } catch (err) {
      focoPendente = null;
      console.error('Erro ao limpar fila:', err);
    }
  }
</script>

<div class="list-card">
  <div class="list-header">
    <div class="header-left">
      <Icone icone={List} />
      <h2 title="Fila de Processamento" tabindex="-1" bind:this={tituloEl}>Fila de Processamento</h2>
    </div>
    <div class="header-actions">
      <input
        type="search"
        placeholder="Filtrar..."
        aria-label="Filtrar fila por nome ou mensagem"
        bind:value={filterText}
        class="search-input"
      />
      {#if items.length > 0}
        {#if isPaused}
          <button class="btn-pause resume" on:click={handleResume} title="Retomar processamento da fila" aria-label="Retomar">
            <Icone icone={Play} tamanho={12} />
            <span class="rotulo-acao">Retomar</span>
          </button>
        {:else}
          <button class="btn-pause" on:click={handlePause} title="Pausar fila de processamento" aria-label="Pausar">
            <Icone icone={Pause} tamanho={12} />
            <span class="rotulo-acao">Pausar</span>
          </button>
        {/if}
        <button class="btn-clear" on:click={handleClear} title="Limpar todos os itens da fila" aria-label="Limpar Fila">
          <Icone icone={ListX} tamanho={12} />
          <span class="rotulo-acao">Limpar Fila</span>
        </button>
      {/if}
    </div>
  </div>

  {#if aviso}
    <div class="queue-notice">
      <span class="queue-chip" role="status" title={aviso}>
        <Icone icone={CirclePause} tamanho={12} />
        <span class="queue-chip-text">{aviso}</span>
      </span>
    </div>
  {/if}

  <div class="items-container">
    {#if items.length > 0 && filteredItems.length === 0}
      <div class="empty-state">
        <p role="status">Nenhum item corresponde ao filtro</p>
      </div>
    {:else if filteredItems.length === 0}
      <div class="empty-state">
        <Icone icone={Archive} tamanho={32} />
        <p>Nenhum processo no histórico.</p>
        <p class="sub-text">Arraste arquivos ZIP em qualquer lugar ou clique em "Selecionar" no painel ao lado.</p>
      </div>
    {:else}
      <div class="items-list" bind:this={listaEl}>
        {#each filteredItems as item (item.id)}
          {@const estado = estadoItem(item.status, isPaused)}
          <div
            class="item-row {item.status}"
            class:has-progress={estado === 'processando'}
            data-id={item.id}
            in:fly|global={{ y: 8, duration: mov.novo(item.id) ? duracaoMovimento(DUR_BASE) : 0, easing: cubicOut }}
            out:fade={{ duration: mov.removido(item.id) ? duracaoMovimento(DUR_FAST) : 0 }}
            animate:flip={{ duration: mov.porFiltro() ? 0 : duracaoMovimento(DUR_FAST), easing: cubicOut }}
          >
            <!-- Status Icon -->
            <div class="status-col">
              {#if estado === 'processando'}
                <span class="spinner-small" aria-hidden="true"><Icone icone={LoaderCircle} cor="var(--accent)" /></span>
              {:else if estado === 'pausado'}
                <Icone icone={CirclePause} cor="var(--warning)" />
              {:else if estado === 'aguardando'}
                <Icone icone={Clock} cor="var(--text-secondary)" />
              {:else if estado === 'sucesso'}
                <Icone icone={CircleCheck} cor="var(--success)" />
              {:else if estado === 'erro'}
                <Icone icone={CircleAlert} cor="var(--error)" />
              {:else if estado === 'cancelado'}
                <Icone icone={CircleSlash} cor="var(--text-secondary)" />
              {/if}
              <span class="sr-only">{ROTULO_ESTADO[estado] ?? ''}</span>
            </div>

            <!-- Name and Info -->
            <div class="info-col">
              <span class="file-name" title={item.path}>{item.name}</span>
              <div class="details-row">
                <span class="timestamp">{formatarHora(item.timestamp)}</span>
                {#if estado === 'sucesso'}
                  <span class="meta-badge success">
                    {item.docs_count} docs | {item.pages_count} pág
                  </span>
                {:else if estado === 'processando'}
                  <span class="progress-message">{item.message || 'Processando...'}</span>
                  <span class="meta-badge processing">
                    Docs: {item.processed_docs || 0}/{item.total_docs || 0} | Págs: {item.processed_pages || 0}/{item.total_pages || 0}
                  </span>
                {:else if estado === 'pausado'}
                  <span class="paused-message">Pausado: começa quando a fila for retomada</span>
                {:else if estado === 'aguardando'}
                  <span class="pending-message">Aguardando na fila...</span>
                {:else if estado === 'cancelado'}
                  <span class="cancelled-message">Cancelado</span>
                {:else if estado === 'erro'}
                  <span class="error-message truncate">{item.message || 'Erro no processamento'}</span>
                {/if}
              </div>
            </div>

            <!-- Inline Progress (only when processing) -->
            {#if estado === 'processando'}
              <div
                class="progress-bar-container"
                role="progressbar"
                aria-valuemin="0"
                aria-valuemax="100"
                aria-valuenow={Math.round((item.progress || 0) * 100)}
                aria-label="Progresso de {item.name}"
              >
                <div class="progress-bar" style="width: {item.progress * 100}%"></div>
              </div>
            {/if}

            <!-- Actions -->
            <div class="actions-col">
              {#if item.status === 'processing'}
                <button
                  class="btn-action btn-cancel"
                  data-acao="cancelar"
                  on:click={(e) => handleCancel(item.id, e)}
                  title="Cancelar processamento ativo"
                >
                  <Icone icone={X} tamanho={14} />
                </button>
              {:else if item.status === 'pending'}
                <button
                  class="btn-action btn-cancel"
                  data-acao="cancelar"
                  on:click={(e) => handleCancel(item.id, e)}
                  title="Remover da fila"
                >
                  <Icone icone={X} tamanho={14} />
                </button>
              {:else if item.status === 'success' && item.output_path}
                <div class="success-actions">
                  <button
                    class="btn-action"
                    data-acao="copiar"
                    on:click={() => handleCopy(item)}
                    title={copiadoId === item.id ? 'Copiado!' : 'Copiar o TXT para colar na IA'}
                    aria-label="Copiar o TXT"
                  >
                    {#if copiadoId === item.id}
                      <Icone icone={Check} tamanho={14} />
                    {:else}
                      <Icone icone={Copy} tamanho={14} />
                    {/if}
                  </button>
                  <button
                    class="btn-action"
                    data-acao="abrir"
                    on:click={() => handleOpenDir(item.output_path)}
                    title="Abrir pasta contendo o TXT"
                  >
                    <Icone icone={FolderOpen} tamanho={14} />
                  </button>
                  <button
                    class="btn-action btn-remove"
                    data-acao="remover"
                    on:click={(e) => handleRemove(item.id, e)}
                    title="Remover da lista"
                  >
                    <Icone icone={Trash2} tamanho={14} />
                  </button>
                </div>
              {:else}
                <!-- Error or Cancelled -->
                <div class="error-actions">
                  {#if item.status === 'error'}
                    <button
                      type="button"
                      class="btn-action btn-detalhes"
                      data-acao="detalhes"
                      on:click={() => alternarDetalhes(item.id)}
                      aria-label="Ver detalhes do erro"
                      aria-expanded={!!detalhesAbertos[item.id]}
                      aria-controls="erro-detalhe-{item.id}"
                      title={detalhesAbertos[item.id] ? 'Ocultar detalhes do erro' : 'Ver detalhes do erro'}
                    >
                      <Icone icone={CircleQuestionMark} tamanho={14} />
                    </button>
                  {/if}
                  <button
                    class="btn-action btn-remove"
                    data-acao="remover"
                    on:click={(e) => handleRemove(item.id, e)}
                    title="Remover da lista"
                  >
                    <Icone icone={Trash2} tamanho={14} />
                  </button>
                </div>
              {/if}
            </div>

            {#if item.status === 'error' && detalhesAbertos[item.id]}
              <div
                class="erro-detalhe"
                id="erro-detalhe-{item.id}"
                in:fade={{ duration: duracaoMovimento(DUR_FAST) }}
              >
                <p class="erro-detalhe-texto">{item.message || 'Erro no processamento'}</p>
                <p class="erro-detalhe-caminho">Arquivo: {item.path}</p>
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Sempre montada: o leitor de tela só anuncia mudanças numa região que
       já existia antes da mensagem. -->
  <div class="aviso-regiao" role="alert" aria-atomic="true">
    {#if erroCopia}
      <div
        class="aviso-erro"
        in:fly={{ y: 8, duration: duracaoMovimento(DUR_BASE), easing: cubicOut }}
        out:fade={{ duration: duracaoMovimento(DUR_FAST) }}
      >
        <span class="aviso-texto">{erroCopia}</span>
        <button class="aviso-fechar" on:click={avisoCopia.fechar} aria-label="Fechar aviso" title="Fechar aviso">
          <Icone icone={X} tamanho={12} />
        </button>
      </div>
    {/if}
  </div>
</div>

<style>
  .aviso-erro {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 8px 12px;
    font-size: var(--fs-meta);
    font-weight: 500;
    color: var(--error-strong);
    background-color: var(--surface);
    border-top: 1px solid currentColor;
  }
  .aviso-texto {
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
    background-color: var(--error-soft);
  }
  .aviso-fechar:focus-visible {
    outline: 2px solid currentColor;
    outline-offset: 1px;
  }
  .list-card {
    background-color: var(--surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    container-type: inline-size;
    box-shadow: var(--shadow-sm);
    transition: background-color var(--dur-base) var(--ease-out), border-color var(--dur-base) var(--ease-out), box-shadow var(--dur-base) var(--ease-out);
  }

  .list-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border);
    background-color: var(--surface);
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
    min-width: 0;
    color: var(--accent);
  }

  .header-left :global(svg) {
    flex-shrink: 0;
  }

  .queue-notice {
    display: flex;
    padding: 6px 12px 0;
    min-width: 0;
  }

  .queue-chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 1px 8px;
    border-radius: 4px;
    font-size: var(--fs-meta);
    font-weight: 500;
    color: var(--warning);
    background-color: var(--warning-soft);
    border: 1px solid var(--warning-line);
    white-space: nowrap;
    min-width: 0;
    overflow: hidden;
  }

  .queue-chip :global(svg) {
    flex-shrink: 0;
  }

  .queue-chip-text {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .header-left h2 {
    font-size: var(--fs-title);
    font-weight: 600;
    margin: 0;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }

  .header-actions {
    display: flex;
    gap: 6px;
    align-items: center;
    flex-shrink: 0;
  }

  .search-input {
    padding: 4px 8px;
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    background-color: var(--surface);
    color: var(--text-primary);
    font-size: var(--fs-meta);
    flex: 0 1 140px;
    width: 140px;
    min-width: 80px;
    transition: border-color var(--dur-fast) var(--ease-out);
  }

  .search-input:focus {
    border-color: var(--accent);
  }

  .btn-clear {
    background-color: transparent;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: var(--fs-meta);
    padding: 4px 8px;
    border-radius: 4px;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 4px;
    white-space: nowrap;
  }

  .btn-clear:hover {
    background-color: var(--error-soft);
    color: var(--error);
    border-color: var(--error);
  }

  .btn-pause {
    background-color: var(--surface-hover);
    border: 1px solid var(--border);
    color: var(--text-primary);
    font-size: var(--fs-meta);
    padding: 4px 8px;
    border-radius: 4px;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 4px;
    white-space: nowrap;
  }

  /* Fila estreita: os botões ficam só com o ícone; o nome segue no aria-label. */
  @container (max-width: 420px) {
    .rotulo-acao {
      display: none;
    }
  }

  .btn-pause:hover {
    border-color: var(--accent);
    background-color: var(--accent-faint);
  }

  .btn-pause.resume {
    background-color: var(--success-faint);
    border-color: var(--success);
    color: var(--success-strong);
  }

  .btn-pause.resume:hover {
    background-color: var(--success-soft);
  }

  .items-container {
    flex: 1;
    overflow-y: auto;
    background-color: transparent;
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    padding: 48px 12px;
    gap: 8px;
    color: var(--text-secondary);
    text-align: center;
  }

  .empty-state p {
    margin: 0;
    font-size: var(--fs-base);
    font-weight: 500;
  }

  .empty-state .sub-text {
    font-size: var(--fs-meta);
    max-width: 250px;
  }

  .items-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 8px;
  }

  .item-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    min-height: 36px;
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 4px 8px;
    position: relative;
    gap: 8px;
    background-color: var(--surface);
    transition: background-color var(--dur-fast) var(--ease-out),
                border-color var(--dur-fast) var(--ease-out);
  }

  .item-row:hover {
    background-color: var(--surface-hover);
    border-color: var(--border-strong);
  }

  .status-col {
    width: 20px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .info-col {
    flex: 1;
    display: flex;
    flex-direction: row;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .file-name {
    flex: 1 1 auto;
    min-width: 96px;
    font-size: var(--fs-base);
    font-weight: 500;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Mensagem e contadores à direita do nome, na mesma faixa. */
  .details-row {
    flex: 0 1 auto;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    font-size: var(--fs-meta);
    color: var(--text-secondary);
    min-width: 0;
    overflow: hidden;
  }

  .details-row > span {
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .timestamp {
    font-variant-numeric: tabular-nums;
    flex-shrink: 0;
  }

  .meta-badge {
    padding: 0 6px;
    border-radius: 3px;
    font-size: var(--fs-meta);
    font-weight: 600;
    flex: 0 1 auto;
  }

  .meta-badge.success {
    background-color: var(--success-soft);
    color: var(--success);
  }

  .meta-badge.processing {
    background-color: var(--accent-soft);
    color: var(--accent-text);
    border: 1px solid var(--accent-line);
    font-weight: 500;
  }

  /* A mensagem cede espaço antes dos contadores. */
  .progress-message {
    flex: 0 100 auto;
    color: var(--accent);
  }

  @container (max-width: 480px) {
    .progress-message {
      display: none;
    }
  }

  /* Fila no mínimo: os contadores valem mais que o horário e o "Aguardando". */
  @container (max-width: 420px) {
    .timestamp,
    .pending-message {
      display: none;
    }

    .file-name {
      min-width: 64px;
    }
  }

  .paused-message {
    color: var(--warning);
  }

  .pending-message {
    color: var(--text-secondary);
    font-style: italic;
  }

  .cancelled-message {
    color: var(--text-secondary);
  }

  .error-message {
    color: var(--error);
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

  .progress-bar-container {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    height: 2px;
    background-color: var(--track);
    overflow: hidden;
  }

  .progress-bar {
    height: 100%;
    background-color: var(--processing);
    transition: width var(--dur-base) var(--ease-out);
  }

  /* Três botões de 22px (copiar, abrir, remover) sem invadir os contadores. */
  .actions-col {
    width: 74px;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 4px;
    flex-shrink: 0;
  }

  .success-actions, .error-actions {
    display: flex;
    gap: 4px;
    align-items: center;
  }

  .btn-action {
    background: transparent;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 4px;
    border-radius: 4px;
    transition: color var(--dur-fast) var(--ease-out), background-color var(--dur-fast) var(--ease-out);
  }

  .btn-action:hover {
    color: var(--text-primary);
    background-color: var(--surface-hover);
  }

  .btn-cancel:hover {
    color: var(--error);
    background-color: var(--error-soft);
  }

  .btn-remove:hover {
    color: var(--error);
    background-color: var(--error-soft);
  }

  .btn-detalhes {
    color: var(--error);
  }

  .btn-detalhes[aria-expanded='true'] {
    background-color: var(--error-soft);
  }

  .erro-detalhe {
    flex-basis: 100%;
    margin-left: 30px;
    padding: 6px 8px;
    border-left: 2px solid var(--error);
    border-radius: 4px;
    background-color: var(--error-soft);
    font-size: var(--fs-meta);
    color: var(--text-primary);
    user-select: text;
    -webkit-user-select: text;
    cursor: text;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .erro-detalhe p {
    margin: 0;
  }

  .erro-detalhe-caminho {
    margin-top: 2px;
    color: var(--text-secondary);
  }

  .spinner-small {
    display: inline-flex;
    animation: spin var(--dur-spin) linear infinite;
  }

  .truncate {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
