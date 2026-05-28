<script>
  import { OpenDirectory } from '../wailsjs/go/main/App.js';

  export let items = [];

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

  function clearHistory() {
    items = [];
  }
</script>

<div class="list-card">
  <div class="list-header">
    <div class="header-left">
      <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-list"><line x1="8" x2="21" y1="6" y2="6"/><line x1="8" x2="21" y1="12" y2="12"/><line x1="8" x2="21" y1="18" y2="18"/><line x1="3" x2="3.01" y1="6" y2="6"/><line x1="3" x2="3.01" y1="12" y2="12"/><line x1="3" x2="3.01" y1="18" y2="18"/></svg>
      <h2>Fila de Processamento</h2>
    </div>
    <div class="header-actions">
      <input 
        type="text" 
        placeholder="Filtrar processos..." 
        bind:value={filterText}
        class="search-input"
      />
      {#if items.length > 0}
        <button class="btn-clear" on:click={clearHistory} title="Limpar Histórico">
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
        <p class="sub-text">Baixe arquivos ZIP na pasta monitorada ou selecione manualmente para iniciar.</p>
      </div>
    {:else}
      <div class="items-list">
        {#each filteredItems as item (item.id)}
          <div class="item-row {item.status}" class:has-progress={item.status === 'processing'}>
            <!-- Status Icon -->
            <div class="status-col">
              {#if item.status === 'processing'}
                <div class="spinner-small"></div>
              {:else if item.status === 'success'}
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#28A745" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-check-circle"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
              {:else if item.status === 'error'}
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#DC3545" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-alert-triangle"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><line x1="12" x2="12" y1="9" y2="13"/><line x1="12" x2="12.01" y1="17" y2="17"/></svg>
              {/if}
            </div>

            <!-- Name and Info -->
            <div class="info-col">
              <span class="file-name" title={item.path}>{item.name}</span>
              <div class="details-row">
                <span class="timestamp">{formatTime(item.timestamp)}</span>
                {#if item.status === 'success'}
                  <span class="meta-badge success">
                    {item.docsCount} docs | {item.pagesCount} pág
                  </span>
                {:else if item.status === 'processing'}
                  <span class="progress-message">{item.message || 'Iniciando extração...'}</span>
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
              {#if item.status === 'success' && item.outputPath}
                <button 
                  class="btn-action" 
                  on:click={() => handleOpenDir(item.outputPath)} 
                  title="Abrir pasta de destino e selecionar TXT"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-folder-open"><path d="m6 14 1.45-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.55 6a2 2 0 0 1-1.94 1.5H4a2 2 0 0 1-2-2V5c0-1.1.9-2 2-2h3.93a2 2 0 0 1 1.66.9l.82 1.2a2 2 0 0 0 1.66.9H18a2 2 0 0 1 2 2v2"/></svg>
                </button>
              {:else if item.status === 'error'}
                <div class="error-tooltip-container" title={item.message}>
                  <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-help-circle"><circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><line x1="12" x2="12.01" y1="17" y2="17"/></svg>
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
    gap: 8px;
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
    width: 140px;
    transition: width 0.2s;
  }

  .search-input:focus {
    width: 180px;
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
    height: 40px; /* 40px line height as requested */
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
  }

  .timestamp {
    font-variant-numeric: tabular-nums;
  }

  .meta-badge {
    padding: 1px 6px;
    border-radius: 3px;
    font-size: 10px;
    font-weight: 600;
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
    width: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
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
    color: var(--accent);
    background-color: rgba(0, 86, 179, 0.15);
  }

  .error-tooltip-container {
    color: var(--error);
    cursor: help;
  }

  .spinner-small {
    width: 14px;
    height: 14px;
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
