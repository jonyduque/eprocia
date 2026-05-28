<script>
  import { onMount, onDestroy } from 'svelte';
  import { EventsOn } from './wailsjs/runtime/runtime.js';
  import { GetConfig, ProcessFile, SelectZipFile } from './wailsjs/go/main/App.js';
  import Settings from './components/Settings.svelte';
  import ProcessList from './components/ProcessList.svelte';

  let items = [];
  let isDarkMode = true;
  let config = null;
  let processingCount = 0;
  let manualProcessing = false;
  let manualFilePath = '';
  let dragActive = false;

  // Unsubscribe handlers
  let unsubscribers = [];

  onMount(async () => {
    // Detect system theme or storage
    const storedTheme = localStorage.getItem('theme');
    if (storedTheme === 'light') {
      isDarkMode = false;
      document.documentElement.classList.add('light');
    } else {
      isDarkMode = true;
      document.documentElement.classList.remove('light');
    }

    // Load initial config
    await refreshConfig();

    // Register Wails event listeners
    unsubscribers.push(
      EventsOn('process:start', (zipPath) => {
        handleProcessStart(zipPath);
      })
    );

    unsubscribers.push(
      EventsOn('process:progress', (data) => {
        handleProcessProgress(data);
      })
    );

    unsubscribers.push(
      EventsOn('process:success', (data) => {
        handleProcessSuccess(data);
      })
    );

    unsubscribers.push(
      EventsOn('process:error', (data) => {
        handleProcessError(data);
      })
    );
  });

  onDestroy(() => {
    // Unsubscribe from all events
    unsubscribers.forEach(unsub => {
      if (typeof unsub === 'function') unsub();
    });
  });

  async function refreshConfig() {
    try {
      config = await GetConfig();
    } catch (err) {
      console.error('Erro ao ler configuração:', err);
    }
  }

  function handleConfigChanged(event) {
    config = event.detail;
  }

  function toggleTheme() {
    isDarkMode = !isDarkMode;
    if (isDarkMode) {
      document.documentElement.classList.remove('light');
      localStorage.setItem('theme', 'dark');
    } else {
      document.documentElement.classList.add('light');
      localStorage.setItem('theme', 'light');
    }
  }

  // Wails Event Handlers
  function handleProcessStart(zipPath) {
    const fileName = zipPath.split(/[\\/]/).pop();
    
    // Check if it already exists in the list
    const existingIndex = items.findIndex(item => item.path === zipPath);
    
    if (existingIndex !== -1) {
      // Move to top and reset state
      const updatedItem = {
        ...items[existingIndex],
        status: 'processing',
        progress: 0,
        message: 'Iniciando processamento...',
        timestamp: Date.now()
      };
      items = [updatedItem, ...items.filter((_, idx) => idx !== existingIndex)];
    } else {
      const newItem = {
        id: zipPath,
        name: fileName,
        path: zipPath,
        status: 'processing',
        progress: 0,
        message: 'Iniciando processamento...',
        timestamp: Date.now()
      };
      items = [newItem, ...items];
    }
    
    processingCount++;
  }

  function handleProcessProgress(data) {
    // data structure: { file: string, progress: number, message: string }
    items = items.map(item => {
      if (item.path === data.file) {
        return {
          ...item,
          progress: data.progress,
          message: data.message
        };
      }
      return item;
    });
  }

  function handleProcessSuccess(data) {
    // data structure: { file: string, output_path: string, docs_count: number, pages_count: number }
    items = items.map(item => {
      if (item.path === data.file) {
        return {
          ...item,
          status: 'success',
          progress: 1.0,
          outputPath: data.output_path,
          docsCount: data.docs_count,
          pagesCount: data.pages_count,
          message: 'Concluído com sucesso!'
        };
      }
      return item;
    });

    if (processingCount > 0) processingCount--;
  }

  function handleProcessError(data) {
    // data structure: { file: string, error: string }
    items = items.map(item => {
      if (item.path === data.file) {
        return {
          ...item,
          status: 'error',
          progress: 0,
          message: data.error
        };
      }
      return item;
    });

    if (processingCount > 0) processingCount--;
  }

  // Manual Trigger
  async function triggerManualSelect() {
    try {
      const filePath = await SelectZipFile();
      if (filePath) {
        runManualProcess(filePath);
      }
    } catch (err) {
      console.error('Erro ao selecionar arquivo:', err);
    }
  }

  async function runManualProcess(path) {
    try {
      manualFilePath = path;
      manualProcessing = true;
      await ProcessFile(path);
    } catch (err) {
      console.error('Erro ao processar arquivo manual:', err);
    } finally {
      manualProcessing = false;
      manualFilePath = '';
    }
  }

  // Drag and Drop Handlers
  function handleDragOver(e) {
    e.preventDefault();
    dragActive = true;
  }

  function handleDragLeave(e) {
    e.preventDefault();
    dragActive = false;
  }

  function handleDrop(e) {
    e.preventDefault();
    dragActive = false;
    
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      const file = e.dataTransfer.files[0];
      const path = file.path; // Available in WebView2 on Windows
      
      if (path) {
        if (path.toLowerCase().endsWith('.zip')) {
          runManualProcess(path);
        } else {
          alert('Por favor, selecione um arquivo ZIP do eproc.');
        }
      } else {
        alert('Arraste e solte não é suportado pelo seu WebView2. Por favor, use o botão "Selecionar ZIP".');
      }
    }
  }
</script>

<div 
  class="app-container"
  on:dragover={handleDragOver}
  on:dragenter={handleDragOver}
  on:dragleave={handleDragLeave}
  on:drop={handleDrop}
>
  <!-- Drag & Drop overlay cover -->
  {#if dragActive}
    <div class="drag-overlay">
      <div class="overlay-box">
        <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-file-archive"><path d="M4 22V4c0-.5.2-1 .6-1.4C5 2.2 5.5 2 6 2h8l6 6v12c0 .5-.2 1-.6 1.4-.4.4-.9.6-1.4.6H6c-.5 0-1-.2-1.4-.6C4.2 21 4 20.5 4 20Z"/><path d="M14 2v6h6"/><circle cx="10" cy="14" r="2"/><path d="M16 14h-4"/><path d="M12 12v4"/><path d="M8 14H6"/></svg>
        <p>Solte o arquivo ZIP do processo para extrair</p>
      </div>
    </div>
  {/if}

  <!-- Header -->
  <header>
    <div class="brand">
      <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="brand-icon"><path d="m12 3-1.912 5.886H3.887L9.043 12.5 7.13 18.386l5.158-3.746 5.157 3.746-1.912-5.886 5.156-3.614H13.912Z"/></svg>
      <div class="brand-text">
        <h1>eprocIA</h1>
        <span class="version">v1.0 (Local Offline)</span>
      </div>
    </div>
    <div class="header-actions">
      <!-- Active Processes Indicator -->
      {#if processingCount > 0}
        <div class="pulse-indicator">
          <span class="pulse-dot"></span>
          <span>{processingCount} processamento{processingCount > 1 ? 's' : ''} ativo{processingCount > 1 ? 's' : ''}</span>
        </div>
      {/if}

      <!-- Theme Switcher -->
      <button class="theme-toggle" on:click={toggleTheme} title="Alternar tema claro/escuro">
        {#if isDarkMode}
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-sun"><circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/></svg>
        {:else}
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-moon"><path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/></svg>
        {/if}
      </button>
    </div>
  </header>

  <!-- Main Content Grid -->
  <main class="grid-layout">
    <!-- Left Sidebar -->
    <aside class="sidebar">
      <!-- Settings Component -->
      <Settings on:configChanged={handleConfigChanged} />

      <!-- Manual Process Card -->
      <div class="manual-card">
        <div class="card-header">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-file-plus-2"><path d="M4 22V4c0-.5.2-1 .6-1.4C5 2.2 5.5 2 6 2h8l6 6v12c0 .5-.2 1-.6 1.4-.4.4-.9.6-1.4.6H6c-.5 0-1-.2-1.4-.6C4.2 21 4 20.5 4 20Z"/><path d="M14 2v6h6"/><path d="M12 18v-6"/><path d="M9 15h6"/></svg>
          <h3>Processamento Manual</h3>
        </div>
        
        <div class="drag-zone" on:click={triggerManualSelect} class:processing={manualProcessing}>
          {#if manualProcessing}
            <div class="spinner"></div>
            <p class="processing-text">Processando arquivo manual...</p>
            <p class="sub-text truncate" title={manualFilePath}>{manualFilePath.split(/[\\/]/).pop()}</p>
          {:else}
            <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-upload-cloud"><path d="M4 14.899A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.5 8.242"/><path d="M12 12v9"/><path d="m15 15-3-3-3 3"/></svg>
            <p>Arraste o ZIP ou clique para selecionar</p>
            <span class="sub-text">Formatos aceitos: ZIP (padrão Eproc)</span>
          {/if}
        </div>
      </div>
    </aside>

    <!-- Right Process List -->
    <section class="content-area">
      <ProcessList bind:items={items} />
    </section>
  </main>

  <!-- Footer Status Bar -->
  <footer>
    <div class="status-item">
      <span class="status-dot" class:active={config && config.monitored_path} class:inactive={!config || !config.monitored_path}></span>
      {#if config && config.monitored_path}
        <span class="status-text truncate" title={config.monitored_path}>
          Monitorando downloads: <strong>{config.monitored_path}</strong>
        </span>
      {:else}
        <span class="status-text">Watchdog inativo (defina uma pasta monitorada)</span>
      {/if}
    </div>
    <div class="status-item font-tabular">
      <span>Fila: {items.length} total</span>
    </div>
  </footer>
</div>

<style>
  :global(:root) {
    /* Set color theme mapping */
  }

  .app-container {
    display: flex;
    flex-direction: column;
    height: 100vh;
    background-color: var(--bg-main);
    color: var(--text-primary);
    overflow: hidden;
    position: relative;
  }

  /* Drag overlay */
  .drag-overlay {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(11, 15, 25, 0.9);
    z-index: 100;
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none;
  }

  .overlay-box {
    border: 2px dashed var(--accent);
    border-radius: 8px;
    padding: 32px;
    background-color: var(--surface);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    box-shadow: 0 10px 25px rgba(0, 0, 0, 0.4);
    animation: zoomIn 0.15s ease-out;
  }

  .overlay-box p {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  @keyframes zoomIn {
    from { transform: scale(0.95); opacity: 0; }
    to { transform: scale(1); opacity: 1; }
  }

  /* Header */
  header {
    height: 48px;
    background-color: var(--surface);
    border-bottom: 1px solid var(--border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 12px;
    flex-shrink: 0;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .brand-icon {
    color: var(--accent);
  }

  .brand-text {
    display: flex;
    flex-direction: column;
  }

  header h1 {
    font-size: 14px;
    font-weight: 700;
    margin: 0;
    line-height: 1.2;
    color: var(--text-primary);
  }

  .version {
    font-size: 10px;
    color: var(--text-secondary);
    line-height: 1;
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .theme-toggle {
    background-color: transparent;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    cursor: pointer;
    border-radius: 4px;
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.15s, border-color 0.15s, background-color 0.15s;
  }

  .theme-toggle:hover {
    color: var(--text-primary);
    border-color: var(--text-secondary);
    background-color: var(--surface-hover);
  }

  .pulse-indicator {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--accent);
    font-weight: 500;
    background-color: rgba(59, 130, 246, 0.1);
    padding: 3px 8px;
    border-radius: 20px;
  }

  .pulse-dot {
    width: 6px;
    height: 6px;
    background-color: var(--accent);
    border-radius: 50%;
    box-shadow: 0 0 0 0 rgba(59, 130, 246, 0.7);
    animation: pulse 1.5s infinite;
  }

  @keyframes pulse {
    0% {
      transform: scale(0.95);
      box-shadow: 0 0 0 0 rgba(59, 130, 246, 0.7);
    }
    70% {
      transform: scale(1);
      box-shadow: 0 0 0 6px rgba(59, 130, 246, 0);
    }
    100% {
      transform: scale(0.95);
      box-shadow: 0 0 0 0 rgba(59, 130, 246, 0);
    }
  }

  /* Grid layout */
  .grid-layout {
    flex: 1;
    display: grid;
    grid-template-columns: 280px 1fr;
    gap: 8px;
    padding: 8px;
    overflow: hidden;
  }

  .sidebar {
    display: flex;
    flex-direction: column;
    gap: 8px;
    overflow-y: auto;
  }

  .content-area {
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  /* Manual processing card */
  .manual-card {
    background-color: var(--surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  }

  .manual-card .card-header {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--accent);
    border-bottom: 1px solid var(--border);
    padding-bottom: 6px;
  }

  .manual-card h3 {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .drag-zone {
    border: 1px dashed var(--border);
    border-radius: 4px;
    padding: 16px 8px;
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 6px;
    cursor: pointer;
    background-color: var(--bg-main);
    color: var(--text-secondary);
    transition: border-color 0.2s, background-color 0.2s, color 0.2s;
  }

  .drag-zone:hover:not(.processing) {
    border-color: var(--accent);
    background-color: var(--surface-hover);
    color: var(--text-primary);
  }

  .drag-zone.processing {
    cursor: not-allowed;
    background-color: rgba(59, 130, 246, 0.05);
    border-color: var(--accent);
  }

  .drag-zone p {
    font-size: 11px;
    font-weight: 500;
    margin: 0;
  }

  .drag-zone .sub-text {
    font-size: 9px;
    color: var(--text-secondary);
  }

  .processing-text {
    color: var(--accent) !important;
  }

  .truncate {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 100%;
  }

  /* Footer */
  footer {
    height: 28px;
    background-color: var(--surface);
    border-top: 1px solid var(--border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 12px;
    font-size: 11px;
    color: var(--text-secondary);
    flex-shrink: 0;
  }

  .status-item {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }

  .status-dot.active {
    background-color: var(--success);
    box-shadow: 0 0 6px var(--success);
  }

  .status-dot.inactive {
    background-color: var(--warning);
    box-shadow: 0 0 6px var(--warning);
  }

  .status-text {
    font-size: 11px;
  }

  .font-tabular {
    font-variant-numeric: tabular-nums;
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
