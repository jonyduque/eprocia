<script>
  import { onMount, createEventDispatcher } from 'svelte';
  import { GetConfig, SaveConfig, SelectDirectory } from '../wailsjs/go/main/App.js';

  const dispatch = createEventDispatcher();

  let config = {
    monitored_path: '',
    ocr_enabled: true,
    cleaning_level: 'basic',
    auto_start: false
  };

  let loading = true;
  let statusMessage = '';
  let statusType = ''; // 'success' or 'error'

  onMount(async () => {
    await loadConfig();
  });

  async function loadConfig() {
    try {
      loading = true;
      const loaded = await GetConfig();
      if (loaded) {
        config = { ...loaded };
      }
    } catch (err) {
      showStatus('Erro ao carregar configurações: ' + err.message, 'error');
    } finally {
      loading = false;
    }
  }

  async function handleSelectDirectory() {
    try {
      const selected = await SelectDirectory();
      if (selected) {
        config.monitored_path = selected;
        await handleSave();
      }
    } catch (err) {
      showStatus('Erro ao selecionar pasta: ' + err.message, 'error');
    }
  }

  async function handleClearDirectory() {
    config.monitored_path = '';
    await handleSave();
  }

  async function handleSave() {
    try {
      await SaveConfig(config);
      showStatus('Configurações salvas com sucesso!', 'success');
      dispatch('configChanged', config);
    } catch (err) {
      showStatus('Erro ao salvar configurações: ' + err.message, 'error');
    }
  }

  function showStatus(msg, type) {
    statusMessage = msg;
    statusType = type;
    setTimeout(() => {
      statusMessage = '';
      statusType = '';
    }, 3000);
  }
</script>

<div class="settings-card">
  <div class="card-header">
    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-settings"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.1a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>
    <h2>Configurações</h2>
  </div>

  {#if loading}
    <div class="loading-state">
      <div class="spinner"></div>
      <span>Carregando configurações...</span>
    </div>
  {:else}
    <div class="settings-form">
      <!-- Monitored Path Section -->
      <div class="form-group">
        <label for="monitored-path">Pasta Monitorada (Watchdog)</label>
        <div class="path-input-group">
          <input
            id="monitored-path"
            type="text"
            readonly
            value={config.monitored_path || 'Nenhuma pasta monitorada (inativo)'}
            class:inactive={!config.monitored_path}
          />
          <button type="button" class="btn-primary" on:click={handleSelectDirectory}>
            Alterar
          </button>
          {#if config.monitored_path}
            <button type="button" class="btn-danger-icon" on:click={handleClearDirectory} title="Desativar monitoramento">
              <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-trash-2"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/><line x1="10" x2="10" y1="11" y2="17"/><line x1="14" x2="14" y1="11" y2="17"/></svg>
            </button>
          {/if}
        </div>
        <p class="help-text">O eprocIA processará automaticamente qualquer arquivo ZIP baixado nesta pasta.</p>
      </div>

      <!-- OCR Toggle -->
      <div class="form-group toggle-group">
        <div class="toggle-text">
          <label for="ocr-toggle">OCR Fallback Inteligente</label>
          <p class="help-text">Ativa OCR local (Tesseract) automaticamente em páginas de PDF digitalizadas (imagens).</p>
        </div>
        <label class="switch">
          <input
            id="ocr-toggle"
            type="checkbox"
            bind:checked={config.ocr_enabled}
            on:change={handleSave}
          />
          <span class="slider round"></span>
        </label>
      </div>

      <!-- Cleaning Level -->
      <div class="form-group">
        <label for="cleaning-level">Nível de Higienização de Texto</label>
        <select
          id="cleaning-level"
          bind:value={config.cleaning_level}
          on:change={handleSave}
        >
          <option value="none">Nenhum (Preserva texto bruto)</option>
          <option value="basic">Básico (Corrige espaçamento e quebras redundantes)</option>
          <option value="aggressive">Agressivo (Básico + remove rodapés e cabeçalhos repetitivos)</option>
        </select>
        <p class="help-text">Otimiza o texto extraído para economizar tokens na inteligência artificial.</p>
      </div>

      <!-- Auto Start Toggle -->
      <div class="form-group toggle-group">
        <div class="toggle-text">
          <label for="autostart-toggle">Iniciar com o Windows</label>
          <p class="help-text">Executar o eprocIA minimizado na bandeja do sistema ao iniciar o computador.</p>
        </div>
        <label class="switch">
          <input
            id="autostart-toggle"
            type="checkbox"
            bind:checked={config.auto_start}
            on:change={handleSave}
          />
          <span class="slider round"></span>
        </label>
      </div>
    </div>
  {/if}

  {#if statusMessage}
    <div class="toast-status {statusType}">
      <span>{statusMessage}</span>
    </div>
  {/if}
</div>

<style>
  .settings-card {
    background-color: var(--surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
    position: relative;
    overflow: hidden;
  }

  .card-header {
    display: flex;
    align-items: center;
    gap: 8px;
    border-bottom: 1px solid var(--border);
    padding-bottom: 8px;
    color: var(--accent);
  }

  .card-header h2 {
    font-size: 14px;
    font-weight: 600;
    margin: 0;
    color: var(--text-primary);
  }

  .settings-form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .toggle-group {
    flex-direction: row;
    justify-content: space-between;
    align-items: center;
    padding: 4px 0;
    border-bottom: 1px dashed var(--border);
  }

  .toggle-text {
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-width: 80%;
  }

  label {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .help-text {
    font-size: 11px;
    color: var(--text-secondary);
    margin: 0;
  }

  .path-input-group {
    display: flex;
    gap: 6px;
  }

  input[type="text"] {
    flex: 1;
    padding: 6px 10px;
    border: 1px solid var(--border);
    border-radius: 4px;
    background-color: var(--bg-main);
    color: var(--text-primary);
    font-size: 12px;
    outline: none;
    font-family: inherit;
  }

  input[type="text"].inactive {
    color: var(--text-secondary);
    font-style: italic;
  }

  select {
    padding: 6px 10px;
    border: 1px solid var(--border);
    border-radius: 4px;
    background-color: var(--bg-main);
    color: var(--text-primary);
    font-size: 12px;
    outline: none;
    cursor: pointer;
    font-family: inherit;
  }

  button {
    font-family: inherit;
    font-size: 12px;
    font-weight: 500;
    padding: 6px 12px;
    border-radius: 4px;
    cursor: pointer;
    border: none;
    transition: background-color 0.2s, transform 0.1s;
  }

  .btn-primary {
    background-color: var(--accent);
    color: #ffffff;
  }

  .btn-primary:hover {
    background-color: var(--accent-hover);
  }

  .btn-primary:active {
    transform: scale(0.98);
  }

  .btn-danger-icon {
    background-color: transparent;
    border: 1px solid var(--border);
    color: var(--error);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 6px 8px;
  }

  .btn-danger-icon:hover {
    background-color: rgba(220, 53, 69, 0.1);
    border-color: var(--error);
  }

  /* Switch / Toggle styling */
  .switch {
    position: relative;
    display: inline-block;
    width: 36px;
    height: 20px;
    flex-shrink: 0;
  }

  .switch input {
    opacity: 0;
    width: 0;
    height: 0;
  }

  .slider {
    position: absolute;
    cursor: pointer;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: var(--border);
    transition: .3s;
    border-radius: 20px;
  }

  .slider:before {
    position: absolute;
    content: "";
    height: 14px;
    width: 14px;
    left: 3px;
    bottom: 3px;
    background-color: white;
    transition: .3s;
    border-radius: 50%;
  }

  input:checked + .slider {
    background-color: var(--accent);
  }

  input:checked + .slider:before {
    transform: translateX(16px);
  }

  /* Toast Notification */
  .toast-status {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    padding: 8px;
    text-align: center;
    font-size: 11px;
    font-weight: 500;
    animation: slideUp 0.3s ease-out;
  }

  .toast-status.success {
    background-color: var(--success);
    color: white;
  }

  .toast-status.error {
    background-color: var(--error);
    color: white;
  }

  @keyframes slideUp {
    from { transform: translateY(100%); }
    to { transform: translateY(0); }
  }

  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 24px;
    gap: 8px;
    color: var(--text-secondary);
  }

  .spinner {
    width: 20px;
    height: 20px;
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
