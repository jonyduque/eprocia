<script>
  import { 
    OpenDirectory, 
    CancelQueueItem, 
    RemoveQueueItem, 
    PauseQueue, 
    ResumeQueue, 
    ClearQueue 
  } from '../wailsjs/go/main/App.js';

  export let items = [];
  export let isPaused = false;

  let filterText = '';

  $: filteredItems = items.filter(item => 
    item.name.toLowerCase().includes(filterText.toLowerCase()) ||
    (item.message && item.message.toLowerCase().includes(filterText.toLowerCase()))
  );

  async function handleOpenDir(path) {
    try {
      await OpenDirectory(path);
    } catch (err) {
      console.error('Erro ao abrir pasta:', err);
    }
  }

  function formatTime(timestamp) {
    if (!timestamp) return '';
    const date = new Date(timestamp);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
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

  async function handleCancel(id) {
    try {
      await CancelQueueItem(id);
    } catch (err) {
      console.error('Erro ao cancelar:', err);
    }
  }

  async function handleRemove(id) {
    try {
      await RemoveQueueItem(id);
    } catch (err) {
      console.error('Erro ao remover:', err);
    }
  }

  async function handleClear() {
    try {
      await ClearQueue();
    } catch (err) {
      console.error('Erro ao limpar fila:', err);
    }
  }
</script>

<div class="list-card">
  <div class="list-header">
    <div class="header-left">
      <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-list"><line x1="8" x2="21" y1="6" y2="6"/><line x1="8" x2="21" y1="12" y2="12"/><line x1="8" x2="21" y1="18" y2="18"/><circle cx="3" cy="6" r="1"/><circle cx="3" cy="12" r="1"/><circle cx="3" cy="18" r="1"/></svg>
      <h2>Fila de Processamento</h2>
    </div>
    <div class="header-actions">
      <input 
        type="text" 
        placeholder="Filtrar..." 
        bind:value={filterText}
        class="search-input"
      />
      {#if items.length > 0}
        {#if isPaused}
          <button class="btn-pause resume" on:click={handleResume} title="Retomar processamento da fila">
            <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="6 3 20 12 6 21 6 3"/></svg>
            <span>Retomar</span>
          </button>
        {:else}
          <button class="btn-pause" on:click={handlePause} title="Pausar fila de processamento">
            <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="14" y="4" width="4" height="16" rx="1"/><rect x="6" y="4" width="4" height="16" rx="1"/></svg>
            <span>Pausar</span>
          </button>
        {/if}
        <button class="btn-clear" on:click={handleClear} title="Limpar todos os itens da fila">
          Limpar Fila
        </button>
      {/if}
    </div>
  </div>

  <div class="items-container">
    {#if filteredItems.length === 0}
      <div class="empty-state">
        <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-archive"><polyline points="21 8 21 21 3 21 3 8"/><rect width="22" height="5" x="1" y="3"/><line x1="10" x2="14" y1="12" y2="12"/></svg>
        <p>Nenhum processo no histórico.</p>
        <p class="sub-text">Arraste arquivos ZIP em qualquer lugar ou clique em "Selecionar" no painel ao lado.</p>
      </div>
    {:else}
      <div class="items-list">
        {#each filteredItems as item (item.id)}
          <div class="item-row {item.status}" class:has-progress={item.status === 'processing'}>
            <!-- Status Icon -->
            <div class="status-col">
              {#if item.status === 'processing'}
                <div class="spinner-small"></div>
              {:else if item.status === 'pending'}
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-clock"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
              {:else if item.status === 'success'}
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--success)" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-check-circle"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
              {:else if item.status === 'error'}
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--error)" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-alert-triangle"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><line x1="12" x2="12" y1="9" y2="13"/><line x1="12" x2="12.01" y1="17" y2="17"/></svg>
              {:else if item.status === 'cancelled'}
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-ban"><circle cx="12" cy="12" r="10"/><path d="m4.93 4.93 14.14 14.14"/></svg>
              {/if}
            </div>

            <!-- Name and Info -->
            <div class="info-col">
              <span class="file-name" title={item.path}>{item.name}</span>
              <div class="details-row">
                <span class="timestamp">{formatTime(item.timestamp)}</span>
                {#if item.status === 'success'}
                  <span class="meta-badge success">
                    {item.docs_count} docs | {item.pages_count} pág
                  </span>
                {:else if item.status === 'processing'}
                  <span class="progress-message">{item.message || 'Processando...'}</span>
                {:else if item.status === 'pending'}
                  <span class="pending-message">Aguardando na fila...</span>
                {:else if item.status === 'cancelled'}
                  <span class="cancelled-message">Cancelado</span>
                {:else if item.status === 'error'}
                  <span class="error-message truncate">{item.message || 'Erro no processamento'}</span>
                {/if}
              </div>
            </div>

            <!-- Inline Progress (only when processing) -->
            {#if item.status === 'processing'}
              <div class="progress-bar-container">
                <div class="progress-bar" style="width: {item.progress * 100}%"></div>
              </div>
            {/if}

            <!-- Actions -->
            <div class="actions-col">
              {#if item.status === 'processing'}
                <button 
                  class="btn-action btn-cancel" 
                  on:click={() => handleCancel(item.id)} 
                  title="Cancelar processamento ativo"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                </button>
              {:else if item.status === 'pending'}
                <button 
                  class="btn-action btn-cancel" 
                  on:click={() => handleCancel(item.id)} 
                  title="Remover da fila"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                </button>
              {:else if item.status === 'success' && item.output_path}
                <div class="success-actions">
                  <button 
                    class="btn-action" 
                    on:click={() => handleOpenDir(item.output_path)} 
                    title="Abrir pasta contendo o TXT"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-folder-open"><path d="m6 14 1.45-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.55 6a2 2 0 0 1-1.94 1.5H4a2 2 0 0 1-2-2V5c0-1.1.9-2 2-2h3.93a2 2 0 0 1 1.66.9l.82 1.2a2 2 0 0 0 1.66.9H18a2 2 0 0 1 2 2v2"/></svg>
                  </button>
                  <button 
                    class="btn-action btn-remove" 
                    on:click={() => handleRemove(item.id)} 
                    title="Remover da lista"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>
                  </button>
                </div>
              {:else}
                <!-- Error or Cancelled -->
                <div class="error-actions">
                  {#if item.status === 'error'}
                    <div class="error-tooltip-container" title={item.message}>
                      <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-help-circle"><circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><line x1="12" x2="12.01" y1="17" y2="17"/></svg>
                    </div>
                  {/if}
                  <button 
                    class="btn-action btn-remove" 
                    on:click={() => handleRemove(item.id)} 
                    title="Remover da lista"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>
                  </button>
                </div>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .list-card {
    background-color: var(--surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  }

  .list-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border);
    background-color: var(--bg-main);
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--accent);
  }

  .header-left h2 {
    font-size: 14px;
    font-weight: 600;
    margin: 0;
    color: var(--text-primary);
  }

  .header-actions {
    display: flex;
    gap: 6px;
    align-items: center;
  }

  .search-input {
    padding: 4px 8px;
    border: 1px solid var(--border);
    border-radius: 4px;
    background-color: var(--surface);
    color: var(--text-primary);
    font-size: 11px;
    outline: none;
    width: 100px;
    transition: width 0.2s;
  }

  .search-input:focus {
    width: 140px;
    border-color: var(--accent);
  }

  .btn-clear {
    background-color: transparent;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: 11px;
    padding: 4px 8px;
    border-radius: 4px;
    cursor: pointer;
  }

  .btn-clear:hover {
    background-color: rgba(220, 53, 69, 0.1);
    color: var(--error);
    border-color: var(--error);
  }

  .btn-pause {
    background-color: var(--surface-hover);
    border: 1px solid var(--border);
    color: var(--text-primary);
    font-size: 11px;
    padding: 4px 8px;
    border-radius: 4px;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .btn-pause:hover {
    border-color: var(--accent);
    background-color: rgba(59, 130, 246, 0.08);
  }

  .btn-pause.resume {
    background-color: rgba(40, 167, 69, 0.08);
    border-color: var(--success);
    color: var(--success);
  }

  .btn-pause.resume:hover {
    background-color: rgba(40, 167, 69, 0.15);
  }

  .items-container {
    flex: 1;
    overflow-y: auto;
    background-color: var(--surface);
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
    font-size: 13px;
    font-weight: 500;
  }

  .empty-state .sub-text {
    font-size: 11px;
    max-width: 250px;
  }

  .items-list {
    display: flex;
    flex-direction: column;
  }

  .item-row {
    display: flex;
    align-items: center;
    height: 40px; /* 40px row height */
    border-bottom: 1px solid var(--border);
    padding: 0 12px;
    position: relative;
    gap: 10px;
    transition: background-color 0.15s;
  }

  .item-row:hover {
    background-color: rgba(255, 255, 255, 0.02);
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
    flex-direction: column;
    justify-content: center;
    min-width: 0;
  }

  .file-name {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .details-row {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 11px;
    color: var(--text-secondary);
    min-width: 0;
  }

  .timestamp {
    font-variant-numeric: tabular-nums;
    flex-shrink: 0;
  }

  .meta-badge {
    padding: 1px 6px;
    border-radius: 3px;
    font-size: 10px;
    font-weight: 600;
    flex-shrink: 0;
  }

  .meta-badge.success {
    background-color: rgba(40, 167, 69, 0.15);
    color: var(--success);
  }

  .progress-message {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--accent);
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

  .progress-bar-container {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    height: 2px;
    background-color: rgba(255, 255, 255, 0.05);
  }

  .progress-bar {
    height: 100%;
    background-color: var(--accent);
    transition: width 0.2s ease-out;
  }

  .actions-col {
    width: 60px;
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
    transition: color 0.15s, background-color 0.15s;
  }

  .btn-action:hover {
    color: var(--text-primary);
    background-color: var(--surface-hover);
  }

  .btn-cancel:hover {
    color: var(--error);
    background-color: rgba(220, 53, 69, 0.1);
  }

  .btn-remove:hover {
    color: var(--error);
    background-color: rgba(220, 53, 69, 0.1);
  }

  .error-tooltip-container {
    color: var(--error);
    cursor: help;
    display: flex;
    align-items: center;
  }

  .spinner-small {
    width: 14px;
    height: 14px;
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
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
