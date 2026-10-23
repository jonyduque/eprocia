<script>
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { fly, fade } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import { criarAviso, duracaoMovimento, DUR_FAST, DUR_BASE } from '../lib/aviso.js';
  import { GetConfig, SaveConfig, SelectDirectory } from '../wailsjs/go/main/App.js';

  const dispatch = createEventDispatcher();

  let config = {
    monitored_path: '',
    ocr_mode: 'normal',
    cleaning_level: 'basic',
    auto_start: false,
    watchdog_enabled: true
  };

  const AJUDA_OCR = {
    none: 'Páginas só com imagem ficam sem texto.',
    fast: 'Modelo leve: mais rápido, menos preciso.',
    normal: 'Equilíbrio entre velocidade e precisão.',
    best: 'Modelo mais preciso, porém mais lento.'
  };

  const AJUDA_LIMPEZA = {
    none: 'Preserva o texto bruto extraído.',
    basic: 'Corrige espaçamento e quebras redundantes.',
    aggressive: 'Básica + remove cabeçalhos e rodapés repetidos.'
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

  const aviso = criarAviso((a) => {
    statusMessage = a ? a.texto : '';
    statusType = a ? a.tipo : '';
  });

  onDestroy(aviso.limpar);

  function showStatus(msg, type) {
    aviso.mostrar(msg, type);
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

      <div class="form-group toggle-group">
        <label for="watchdog-toggle">Monitoramento ativo</label>
        <label class="switch">
          <input
            id="watchdog-toggle"
            type="checkbox"
            bind:checked={config.watchdog_enabled}
            on:change={handleSave}
          />
          <span class="slider" aria-hidden="true"></span>
        </label>
      </div>

      <!-- Monitored Path Section -->
      <div class="form-group">
        <label for="monitored-path">Pasta Monitorada</label>
        <div class="path-input-group">
          <input
            id="monitored-path"
            type="text"
            readonly
            value={config.monitored_path || 'Nenhuma pasta (inativo)'}
            title={config.monitored_path || 'Nenhuma pasta monitorada'}
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
      </div>

      <!-- OCR Settings Dropdown -->
      <div class="form-group">
        <label for="ocr-mode">Processamento de Imagens (OCR)</label>
        <select
          id="ocr-mode"
          bind:value={config.ocr_mode}
          on:change={handleSave}
        >
          <option value="none">Desativado</option>
          <option value="fast">Rápido</option>
          <option value="normal">Normal</option>
          <option value="best">Preciso</option>
        </select>
        <p class="help-text">{AJUDA_OCR[config.ocr_mode] ?? ''}</p>
      </div>

      <!-- Cleaning Level -->
      <div class="form-group">
        <label for="cleaning-level">Higienização de texto</label>
        <select
          id="cleaning-level"
          bind:value={config.cleaning_level}
          on:change={handleSave}
        >
          <option value="none">Nenhuma</option>
          <option value="basic">Básica</option>
          <option value="aggressive">Agressiva</option>
        </select>
        <p class="help-text">{AJUDA_LIMPEZA[config.cleaning_level] ?? ''}</p>
      </div>

      <!-- Auto Start Toggle -->
      <div class="form-group toggle-group">
        <label for="autostart-toggle">Iniciar com o Windows</label>
        <label class="switch">
          <input
            id="autostart-toggle"
            type="checkbox"
            bind:checked={config.auto_start}
            on:change={handleSave}
          />
          <span class="slider" aria-hidden="true"></span>
        </label>
      </div>
    </div>
  {/if}

  {#if statusMessage}
    <div
      class="toast-status {statusType}"
      in:fly={{ y: 8, duration: duracaoMovimento(DUR_BASE), easing: cubicOut }}
      out:fade={{ duration: duracaoMovimento(DUR_FAST) }}
    >
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
    box-shadow: var(--shadow-sm);
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
    gap: 12px;
    min-height: 32px;
  }

  .toggle-group label:not(.switch) {
    cursor: pointer;
    min-width: 0;
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
    min-width: 0; /* Prevents overflow inside flexible layouts */
    padding: 6px 8px;
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    background-color: var(--bg-main);
    color: var(--text-primary);
    font-size: 12px;
    font-family: inherit;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  input[type="text"].inactive {
    color: var(--text-secondary);
    font-style: italic;
  }

  select {
    padding: 6px 10px;
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    background-color: var(--bg-main);
    color: var(--text-primary);
    font-size: 12px;
    cursor: pointer;
    font-family: inherit;
    width: 100%;
    min-width: 0;
  }

  button {
    font-family: inherit;
    font-size: 12px;
    font-weight: 500;
    padding: 6px 10px;
    border-radius: 4px;
    cursor: pointer;
    border: none;
    transition: background-color var(--dur-fast) var(--ease-out), transform var(--dur-fast) var(--ease-out);
    flex-shrink: 0; /* Keep buttons from shrinking */
  }

  .btn-primary {
    background-color: var(--accent-strong);
    color: var(--on-accent);
  }

  .btn-primary:hover {
    background-color: var(--accent-strong-hover);
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
    background-color: var(--error-soft);
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
    background-color: var(--border-strong);
    transition: background-color var(--dur-base) var(--ease-out);
    border-radius: 20px;
  }

  /* O input fica invisível (0x0), então o anel vai no trilho. */
  .switch input:focus-visible + .slider {
    outline: 2px solid var(--focus-ring);
    outline-offset: 2px;
  }

  .slider:before {
    position: absolute;
    content: "";
    height: 14px;
    width: 14px;
    left: 3px;
    bottom: 3px;
    background-color: var(--on-accent);
    transition: transform var(--dur-base) var(--ease-out);
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
    background-color: var(--surface);
    border-top: 1px solid currentColor;
  }

  .toast-status.success {
    color: var(--success-strong);
  }

  .toast-status.error {
    color: var(--error-strong);
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
