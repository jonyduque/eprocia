<script>
  import { onMount, onDestroy } from 'svelte';
  import { 
    EventsOn, 
    WindowMinimise, 
    WindowToggleMaximise, 
    OnFileDrop, 
    OnFileDropOff 
  } from './wailsjs/runtime/runtime.js';
  import { 
    GetConfig, 
    AddFilesToQueue, 
    GetQueue, 
    SelectZipFiles, 
    IsQueuePaused, 
    Hide 
  } from './wailsjs/go/main/App.js';
  import Settings from './components/Settings.svelte';
  import ProcessList from './components/ProcessList.svelte';

  let items = [];
  let isDarkMode = true;
  let config = null;
  let isPaused = false;
  let isWorking = false;
  
  // Sidebar State
  let sidebarWidth = 320; // Default width increased to fit all settings and buttons nicely
  let sidebarCollapsed = false;
  let isResizing = false;
  
  // Drag and Drop State
  let dragActive = false;

  // Unsubscribe handlers
  let unsubscribers = [];

  // Reactive processing count
  $: processingCount = items.filter(item => item.status === 'processing').length;

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

    // Load queue state
    try {
      items = await GetQueue() || [];
      isPaused = await IsQueuePaused();
    } catch (err) {
      console.error('Erro ao carregar fila inicial:', err);
    }

    // Register Wails event listeners for queue updates
    unsubscribers.push(
      EventsOn('queue:update', (data) => {
        items = data || [];
      })
    );

    unsubscribers.push(
      EventsOn('queue:status', (status) => {
        isPaused = status.isPaused;
        isWorking = status.isWorking;
      })
    );

    // Register Native Drag and Drop for the whole window (global target)
    try {
      OnFileDrop((x, y, paths) => {
        dragActive = false; // Reset visual overlay
        if (paths && paths.length > 0) {
          const zipPaths = paths.filter(path => path.toLowerCase().endsWith('.zip'));
          if (zipPaths.length > 0) {
            AddFilesToQueue(zipPaths);
          } else {
            alert('Por favor, arraste apenas arquivos ZIP do eproc.');
          }
        }
      }, false); // useDropTarget = false for global drop
    } catch (err) {
      console.warn('Wails OnFileDrop not available:', err);
    }
  });

  onDestroy(() => {
    // Unsubscribe from Wails events
    unsubscribers.forEach(unsub => {
      if (typeof unsub === 'function') unsub();
    });
    
    // Unregister drag drop listener
    try {
      OnFileDropOff();
    } catch (e) {}
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

  // Handle window close by hiding
  async function handleCloseWindow() {
    try {
      await Hide();
    } catch (err) {
      console.error('Erro ao ocultar janela:', err);
    }
  }

  // Manual Trigger
  async function triggerManualSelect() {
    try {
      const filePaths = await SelectZipFiles();
      if (filePaths && filePaths.length > 0) {
        await AddFilesToQueue(filePaths);
      }
    } catch (err) {
      console.error('Erro ao selecionar arquivos:', err);
    }
  }



  // Sidebar Resizing Handlers
  function startResizing(e) {
    e.preventDefault();
    isResizing = true;
    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', stopResizing);
  }

  function handleMouseMove(e) {
    if (!isResizing) return;
    const newWidth = e.clientX;
    if (newWidth >= 280 && newWidth <= 480) {
      sidebarWidth = newWidth;
    }
  }

  function stopResizing() {
    isResizing = false;
    window.removeEventListener('mousemove', handleMouseMove);
    window.removeEventListener('mouseup', stopResizing);
  }

  function toggleSidebar() {
    sidebarCollapsed = !sidebarCollapsed;
  }

  // Web Drag and Drop handlers (to show visual overlay when dragging over the window)
  function handleDragOver(e) {
    e.preventDefault();
    dragActive = true;
  }

  function handleDragLeave(e) {
    e.preventDefault();
    if (e.clientX === 0 || e.clientY === 0) {
      dragActive = false;
    }
  }

  function handleWindowDrop(e) {
    // Let the event bubble naturally so WebView2/Wails captures it.
    dragActive = false;
  }
</script>

<div 
  class="app-container"
  role="region"
  aria-label="Aplicativo eprocIA"
  on:dragover={handleDragOver}
  on:dragenter={handleDragOver}
  on:dragleave={handleDragLeave}
  on:drop={handleWindowDrop}
>
  <!-- Full Window Drag & Drop Overlay -->
  {#if dragActive}
    <div class="drag-overlay">
      <div class="overlay-box">
        <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-file-archive"><path d="M4 22V4c0-.5.2-1 .6-1.4C5 2.2 5.5 2 6 2h8l6 6v12c0 .5-.2 1-.6 1.4-.4.4-.9.6-1.4.6H6c-.5 0-1-.2-1.4-.6C4.2 21 4 20.5 4 20Z"/><path d="M14 2v6h6"/><circle cx="10" cy="14" r="2"/><path d="M16 14h-4"/><path d="M12 12v4"/><path d="M8 14H6"/></svg>
        <p>Solte o arquivo ZIP em qualquer lugar para processar</p>
      </div>
    </div>
  {/if}

  <!-- Custom Design Titlebar (replacing native title bar) -->
  <div class="custom-titlebar" style="--wails-draggable: drag">
    <div class="brand-title">
      <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="brand-icon"><path d="m12 3-1.912 5.886H3.887L9.043 12.5 7.13 18.386l5.158-3.746 5.157 3.746-1.912-5.886 5.156-3.614H13.912Z"/></svg>
      <h1>eprocIA</h1>
      <span class="titlebar-version">v1.0 (Offline)</span>
    </div>
    
    <div class="titlebar-actions" style="--wails-draggable: no-drag">
      <!-- Active Processes Indicator -->
      {#if processingCount > 0}
        <div class="pulse-indicator">
          <span class="pulse-dot"></span>
          <span>{processingCount} ativo{processingCount > 1 ? 's' : ''}</span>
        </div>
      {/if}

      <!-- Theme Switcher -->
      <button class="titlebar-btn" on:click={toggleTheme} title="Alternar tema">
        {#if isDarkMode}
          <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-sun"><circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/></svg>
        {:else}
          <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-moon"><path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/></svg>
        {/if}
      </button>

      <!-- Minimize Window -->
      <button class="titlebar-btn" on:click={WindowMinimise} title="Minimizar">
        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="5" y1="12" x2="19" y2="12"/></svg>
      </button>

      <!-- Maximize/Restore Window -->
      <button class="titlebar-btn" on:click={WindowToggleMaximise} title="Maximizar">
        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/></svg>
      </button>

      <!-- Close Window (Hides to Tray) -->
      <button class="titlebar-btn btn-close" on:click={handleCloseWindow} title="Fechar (Minimiza para a bandeja)">
        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
      </button>
    </div>
  </div>

  <!-- Main Content Grid -->
  <main class="grid-layout">
    <!-- Sidebar (Collapsible & Resizable) -->
    <aside 
      class="sidebar" 
      style="width: {sidebarCollapsed ? '48px' : sidebarWidth + 'px'}"
      class:collapsed={sidebarCollapsed}
    >
      <div class="sidebar-header-toggle">
        {#if !sidebarCollapsed}
          <span class="sidebar-title">Painel de Controle</span>
        {/if}
        <button class="btn-toggle-sidebar" on:click={toggleSidebar} title={sidebarCollapsed ? "Expandir menu" : "Recolher menu"}>
          {#if sidebarCollapsed}
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"/></svg>
          {:else}
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>
          {/if}
        </button>
      </div>

      {#if !sidebarCollapsed}
        <div class="sidebar-content">
          <!-- Settings Component -->
          <Settings on:configChanged={handleConfigChanged} />

          <!-- Manual Process Card -->
          <div class="manual-card">
            <div class="card-header">
              <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-file-plus-2"><path d="M4 22V4c0-.5.2-1 .6-1.4C5 2.2 5.5 2 6 2h8l6 6v12c0 .5-.2 1-.6 1.4-.4.4-.9.6-1.4.6H6c-.5 0-1-.2-1.4-.6C4.2 21 4 20.5 4 20Z"/><path d="M14 2v6h6"/><path d="M12 18v-6"/><path d="M9 15h6"/></svg>
              <h3>Importação Manual</h3>
            </div>
            
            <button 
              type="button" 
              class="manual-select-btn" 
              on:click={triggerManualSelect} 
            >
              <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
              <span>Selecionar arquivos ZIP...</span>
            </button>
            <p class="manual-help">Dica: Você também pode arrastar e soltar arquivos ZIP em qualquer lugar desta janela.</p>
          </div>
        </div>
      {:else}
        <!-- Collapsed Sidebar Vertical Tools -->
        <div class="collapsed-tools">
          <div class="collapsed-indicator" class:active={config && config.watchdog_enabled && config.monitored_path} class:inactive={!config || !config.watchdog_enabled || !config.monitored_path} title={config && config.watchdog_enabled && config.monitored_path ? `Monitorando: ${config.monitored_path}` : (config && !config.watchdog_enabled && config.monitored_path ? 'Monitoramento pausado' : 'Sem monitoramento ativo')}>
            <span class="collapsed-dot"></span>
          </div>
          
          <button class="collapsed-tool-btn" on:click={triggerManualSelect} title="Processar ZIP Manualmente">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
          </button>
        </div>
      {/if}

      <!-- Drag handle for resizing sidebar -->
      {#if !sidebarCollapsed}
        <div 
          class="resize-handle" 
          role="separator"
          aria-label="Redimensionador de Menu"
          on:mousedown={startResizing} 
          class:resizing={isResizing}
        ></div>
      {/if}
    </aside>

    <!-- Right Process List Area -->
    <section class="content-area">
      <ProcessList bind:items={items} {isPaused} />
    </section>
  </main>

  <!-- Footer Status Bar -->
  <footer>
    <div class="status-item">
      <span class="status-dot" class:active={config && config.watchdog_enabled && config.monitored_path} class:inactive={!config || !config.watchdog_enabled || !config.monitored_path}></span>
      {#if config && config.watchdog_enabled && config.monitored_path}
        <span class="status-text truncate" title={config.monitored_path}>
          Pasta monitorada: <strong>{config.monitored_path}</strong>
        </span>
      {:else if config && !config.watchdog_enabled && config.monitored_path}
        <span class="status-text">
          Monitoramento pausado (pasta configurada: <strong>{config.monitored_path}</strong>)
        </span>
      {:else}
        <span class="status-text">Monitoramento inativo. Configure uma pasta no painel ao lado.</span>
      {/if}
    </div>
    <div class="status-item font-tabular">
      <span>{items.length} item{items.length !== 1 ? 'ns' : ''} na fila</span>
    </div>
  </footer>
</div>

<style>
  .app-container {
    display: flex;
    flex-direction: column;
    height: 100vh;
    background-color: var(--bg-main);
    color: var(--text-primary);
    overflow: hidden;
    position: relative;
  }

  /* Full Screen Drag Overlay */
  .drag-overlay {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(11, 15, 25, 0.95);
    z-index: 1000;
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

  /* Custom Frameless Titlebar */
  .custom-titlebar {
    height: 38px;
    background-color: var(--surface);
    border-bottom: 1px solid var(--border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 12px;
    flex-shrink: 0;
    user-select: none;
  }

  .brand-title {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .brand-icon {
    color: var(--accent);
  }

  .brand-title h1 {
    font-size: 13px;
    font-weight: 700;
    margin: 0;
    line-height: 1.2;
    color: var(--text-primary);
  }

  .titlebar-version {
    font-size: 10px;
    color: var(--text-secondary);
    background-color: var(--bg-main);
    padding: 1px 6px;
    border-radius: 10px;
    font-weight: 500;
  }

  .titlebar-actions {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .titlebar-btn {
    background-color: transparent;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    border-radius: 4px;
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.15s, background-color 0.15s;
  }

  .titlebar-btn:hover {
    color: var(--text-primary);
    background-color: var(--surface-hover);
  }

  .titlebar-btn.btn-close:hover {
    color: #ffffff;
    background-color: var(--error);
  }

  .pulse-indicator {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 10px;
    color: var(--accent);
    font-weight: 600;
    background-color: rgba(59, 130, 246, 0.12);
    padding: 2px 8px;
    border-radius: 12px;
    margin-right: 8px;
  }

  .pulse-dot {
    width: 5px;
    height: 5px;
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
      box-shadow: 0 0 0 4px rgba(59, 130, 246, 0);
    }
    100% {
      transform: scale(0.95);
      box-shadow: 0 0 0 0 rgba(59, 130, 246, 0);
    }
  }

  /* Grid layout */
  .grid-layout {
    flex: 1;
    display: flex;
    gap: 0;
    overflow: hidden;
  }

  .sidebar {
    display: flex;
    flex-direction: column;
    background-color: var(--surface);
    border-right: 1px solid var(--border);
    position: relative;
    overflow: visible; /* Needed for resize handle to overlap */
    transition: width 0.15s ease-in-out;
  }

  .sidebar.collapsed {
    align-items: center;
    width: 48px !important;
  }

  .sidebar-header-toggle {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border);
    height: 38px;
    flex-shrink: 0;
    width: 100%;
  }

  .sidebar.collapsed .sidebar-header-toggle {
    justify-content: center;
    padding: 8px 0;
  }

  .sidebar-title {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    font-weight: 700;
    color: var(--text-secondary);
  }

  .btn-toggle-sidebar {
    background: transparent;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 4px;
    border-radius: 4px;
  }

  .btn-toggle-sidebar:hover {
    color: var(--text-primary);
    background-color: var(--surface-hover);
  }

  .sidebar-content {
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    flex: 1;
    overflow-y: auto;
  }

  /* Resizing Handle */
  .resize-handle {
    position: absolute;
    top: 0;
    right: -2px;
    bottom: 0;
    width: 4px;
    cursor: col-resize;
    z-index: 10;
    transition: background-color 0.2s;
  }

  .resize-handle:hover, .resize-handle.resizing {
    background-color: var(--accent);
  }

  /* Collapsed tools view */
  .collapsed-tools {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 16px 0;
    flex: 1;
    width: 100%;
  }

  .collapsed-indicator {
    width: 16px;
    height: 16px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .collapsed-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }

  .collapsed-indicator.active .collapsed-dot {
    background-color: var(--success);
    box-shadow: 0 0 6px var(--success);
  }

  .collapsed-indicator.inactive .collapsed-dot {
    background-color: var(--warning);
    box-shadow: 0 0 6px var(--warning);
  }

  .collapsed-tool-btn {
    background-color: var(--surface-hover);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    width: 32px;
    height: 32px;
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: color 0.15s, border-color 0.15s;
  }

  .collapsed-tool-btn:hover {
    color: var(--accent);
    border-color: var(--accent);
  }

  .content-area {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    padding: 8px;
    background-color: var(--bg-main);
  }

  /* Manual process card */
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

  .manual-select-btn {
    background-color: var(--bg-main);
    border: 1px solid var(--border);
    color: var(--text-primary);
    font-size: 12px;
    font-weight: 600;
    width: 100%;
    padding: 10px 8px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    cursor: pointer;
    transition: border-color 0.2s, background-color 0.2s;
  }

  .manual-select-btn:hover:not(.processing) {
    border-color: var(--accent);
    background-color: var(--surface-hover);
  }

  .manual-select-btn.processing {
    cursor: not-allowed;
    background-color: rgba(59, 130, 246, 0.05);
    border-color: var(--accent);
    color: var(--accent);
  }

  .manual-help {
    font-size: 10px;
    color: var(--text-secondary);
    margin: 0;
    line-height: 1.3;
    text-align: center;
  }

  .spinner-tiny {
    width: 12px;
    height: 12px;
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    flex-shrink: 0;
  }

  .truncate {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 100%;
  }

  /* Footer */
  footer {
    height: 26px;
    background-color: var(--surface);
    border-top: 1px solid var(--border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 12px;
    font-size: 11px;
    color: var(--text-secondary);
    flex-shrink: 0;
    user-select: none;
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
    flex-shrink: 0;
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

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
