<script>
  import Icone from './Icone.svelte';
  import SettingsIcon from 'lucide-svelte/icons/settings';
  import Trash2 from 'lucide-svelte/icons/trash-2';
  import X from 'lucide-svelte/icons/x';
  import { onDestroy } from 'svelte';
  import { fly, fade } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import { criarAviso, duracaoMovimento, DUR_FAST, DUR_BASE } from '../lib/aviso.js';
  import { SaveConfig, SelectDirectory } from '../wailsjs/go/main/App.js';

  // O App lê a config uma vez e a compartilha por bind; null enquanto carrega.
  export let config = null;
  export let erroConfig = '';
  // Chamado quando SaveConfig falha: o App relê a config gravada, para o
  // rodapé e o formulário não exibirem valores que não foram salvos.
  export let aoFalharSalvar = () => {};

  const PADRAO = {
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

  let statusMessage = '';
  let statusType = ''; // 'success' or 'error'

  // Sem a config salva, o formulário segue editável com os padrões.
  $: if (!config && erroConfig) {
    config = { ...PADRAO };
    showStatus(erroConfig, 'error');
  }
  $: loading = !config;

  async function handleSelectDirectory() {
    try {
      const selected = await SelectDirectory();
      if (selected) {
        config.monitored_path = selected;
        await handleSave();
      }
    } catch (err) {
      showStatus('Erro ao selecionar pasta: ' + (err?.message ?? err), 'error');
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
      config = config;
    } catch (err) {
      // Falha parcial ("configurações salvas, mas...") já se explica sozinha.
      const msg = String(err?.message ?? err);
      const parcial = /^configurações salvas/i.test(msg);
      showStatus(parcial ? msg[0].toUpperCase() + msg.slice(1) : 'Erro ao salvar configurações: ' + msg, 'error');
      await aoFalharSalvar();
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
    <Icone icone={SettingsIcon} tamanho={12} />
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
              <Icone icone={Trash2} tamanho={14} />
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

  <!-- Regiões vivas sempre montadas: o leitor de tela só anuncia mudanças
       dentro de uma região que já existia antes da mensagem. -->
  <div class="toast-regiao" role="status" aria-live="polite" aria-atomic="true">
    {#if statusMessage && statusType !== 'error'}
      <div
        class="toast-status {statusType}"
        in:fly={{ y: 8, duration: duracaoMovimento(DUR_BASE), easing: cubicOut }}
        out:fade={{ duration: duracaoMovimento(DUR_FAST) }}
      >
        <span>{statusMessage}</span>
      </div>
    {/if}
  </div>
  <div class="toast-regiao" role="alert" aria-atomic="true">
    {#if statusMessage && statusType === 'error'}
      <div
        class="toast-status error"
        in:fly={{ y: 8, duration: duracaoMovimento(DUR_BASE), easing: cubicOut }}
        out:fade={{ duration: duracaoMovimento(DUR_FAST) }}
      >
        <span class="toast-texto">{statusMessage}</span>
        <button class="toast-fechar" on:click={aviso.fechar} aria-label="Fechar aviso" title="Fechar aviso">
          <Icone icone={X} tamanho={12} />
        </button>
      </div>
    {/if}
  </div>
</div>

<style>
  /* Seção do painel, sem moldura de cartão: o rótulo pequeno já separa. */
  .settings-card {
    padding: 0 0 8px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    position: relative;
    overflow: hidden;
  }

  .card-header {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--text-secondary);
  }

  .card-header h2 {
    font-size: var(--fs-section);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin: 0;
    color: var(--text-secondary);
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
    font-size: var(--fs-base);
    font-weight: 500;
    color: var(--text-primary);
  }

  .help-text {
    font-size: var(--fs-meta);
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
    font-size: var(--fs-base);
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
    font-size: var(--fs-base);
    cursor: pointer;
    font-family: inherit;
    width: 100%;
    min-width: 0;
  }

  button {
    font-family: inherit;
    font-size: var(--fs-base);
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
  .toast-regiao {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
  }

  .toast-status {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 8px;
    text-align: center;
    font-size: var(--fs-meta);
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

  .toast-texto {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .toast-fechar {
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

  .toast-fechar:hover {
    background-color: var(--error-soft);
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
    animation: spin var(--dur-spin) linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
