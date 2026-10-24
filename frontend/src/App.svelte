<script>
  import { onMount, onDestroy, tick } from 'svelte';
  import {
    EventsOn,
    WindowMinimise,
    WindowToggleMaximise,
    WindowIsMaximised,
    OnFileDrop,
    OnFileDropOff
  } from './wailsjs/runtime/runtime.js';
  import {
    GetConfig,
    AddFilesToQueue,
    GetQueue,
    SelectZipFiles,
    IsQueuePaused,
    Hide,
    SetTheme
  } from './wailsjs/go/main/App.js';
  import Settings from './components/Settings.svelte';
  import ProcessList from './components/ProcessList.svelte';
  import { contarAtivos, rotuloFila } from './lib/estado.js';
  import { criarFilaPorQuadro } from './lib/quadro.js';
  import { fade } from 'svelte/transition';
  import { LARGURA_MIN_PAINEL, criarArrasto, larguraMaxPainel, limitarPainel, teclaDivisor } from './lib/divisor.js';
  import { DUR_FAST, duracaoMovimento } from './lib/aviso.js';
  import { gravarPainel, lerPainel } from './lib/painel.js';
  import {
    CONSULTA_ESCURO,
    NOME_TEMA,
    armazenamento,
    conciliarTema,
    gravarSalvo,
    lerPreferencia,
    lerSalvo,
    proximoTema,
    temaEfetivo
  } from './lib/tema.js';

  const versao = __APP_VERSION__;

  let items = [];
  // Lido já na inicialização: o bloco reativo abaixo não pode desfazer a
  // classe que o script de index.html aplicou antes da montagem.
  let temaPref = lerPreferencia(armazenamento());
  const consultaTema = window.matchMedia(CONSULTA_ESCURO);
  let sistemaEscuro = consultaTema.matches;
  let config = null;
  let erroConfig = '';
  let isPaused = false;
  let isWorking = false;

  // Lido antes da montagem para o painel já abrir com a largura salva.
  let larguraJanela = window.innerWidth;
  const painelSalvo = lerPainel(armazenamento(), larguraJanela);
  let sidebarWidth = painelSalvo.largura;
  let sidebarCollapsed = painelSalvo.recolhido;
  let isResizing = false;
  let maximizada = false;
  let botaoRecolher;

  // sidebarWidth guarda a escolha do usuário; a largura exibida é recalculada
  // quando a janela muda, e a escolha volta se a janela crescer de novo.
  $: maxPainel = larguraMaxPainel(larguraJanela);
  $: larguraPainel = limitarPainel(sidebarWidth, larguraJanela);

  // Drag and Drop State
  let dragActive = false;

  // Unsubscribe handlers
  let unsubscribers = [];

  const filaQuadro = criarFilaPorQuadro(() => items, (novos) => { items = novos; });

  // Pausar a fila não interrompe o item em curso, que segue ativo.
  $: processingCount = contarAtivos(items);

  $: monitorando = !!(config && config.watchdog_enabled && config.monitored_path);
  $: textoMonitoramento = monitorando
    ? `Monitorando: ${config.monitored_path}`
    : (config && !config.watchdog_enabled && config.monitored_path ? 'Monitoramento pausado' : 'Sem monitoramento ativo');

  $: temaAtual = temaEfetivo(temaPref, sistemaEscuro);
  $: {
    document.documentElement.classList.toggle('light', temaAtual === 'light');
  }

  function mudouTemaDoSistema(e) {
    sistemaEscuro = e.matches;
  }

  onMount(async () => {
    consultaTema.addEventListener('change', mudouTemaDoSistema);
    atualizarMaximizada();

    // Load initial config
    await refreshConfig();
    alinharTema();

    // Load queue state
    try {
      items = await GetQueue() || [];
      isPaused = await IsQueuePaused();
    } catch (err) {
      console.error('Erro ao carregar fila inicial:', err);
    }

    // Register Wails event listeners for queue updates
    unsubscribers.push(
      EventsOn('queue:update', (data) => filaQuadro.lista(data))
    );

    // Progresso chega item a item; id desconhecido é ignorado.
    unsubscribers.push(
      EventsOn('queue:item', (item) => filaQuadro.item(item))
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
    stopResizing();
    consultaTema.removeEventListener('change', mudouTemaDoSistema);
    filaQuadro.cancelar();

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
      erroConfig = 'Erro ao carregar configurações: ' + (err?.message ?? err);
    }
  }

  // O config.json guarda o tema para a janela nativa abrir com o fundo certo.
  function gravarTemaNoBackend(tema) {
    SetTheme(tema).catch((err) => console.error('Erro ao salvar o tema:', err));
  }

  function alinharTema() {
    const r = conciliarTema(lerSalvo(armazenamento()), config?.theme);
    temaPref = r.tema;
    if (r.gravarLocal) gravarSalvo(armazenamento(), r.tema);
    if (r.gravarConfig) gravarTemaNoBackend(r.tema);
  }

  function toggleTheme() {
    temaPref = proximoTema(temaPref);
    gravarSalvo(armazenamento(), temaPref);
    gravarTemaNoBackend(temaPref);
  }

  // O Wails não emite evento de maximizar; o resize da janela cobre o botão,
  // o duplo clique na barra e o encaixe do Windows.
  async function atualizarMaximizada() {
    try {
      maximizada = Boolean(await WindowIsMaximised());
    } catch {
      // Fora do Wails (capturas visuais) o rótulo fica em Maximizar.
    }
  }

  function alternarMaximizar() {
    WindowToggleMaximise();
    atualizarMaximizada();
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

  // O painel começa em x = 0, então a posição do ponteiro é a própria largura.
  const arrasto = criarArrasto((x) => {
    sidebarWidth = limitarPainel(x, larguraJanela);
  });

  // A captura mantém os eventos no divisor mesmo com o ponteiro fora dele;
  // blur cobre o Alt+Tab no meio do arrasto, que não gera pointerup.
  function startResizing(e) {
    if (e.button !== 0) return;
    e.preventDefault();
    isResizing = true;
    e.currentTarget.setPointerCapture(e.pointerId);
    document.documentElement.classList.add('redimensionando');
    window.addEventListener('blur', stopResizing);
  }

  function handlePointerMove(e) {
    if (!isResizing) return;
    arrasto.mover(e.clientX);
  }

  function stopResizing() {
    if (!isResizing) return;
    arrasto.soltar();
    isResizing = false;
    document.documentElement.classList.remove('redimensionando');
    window.removeEventListener('blur', stopResizing);
    salvarPainel();
  }

  async function handleResizeKeydown(e) {
    const acao = teclaDivisor(e.key, larguraPainel, larguraJanela);
    if (!acao) return;
    e.preventDefault();
    if (acao.recolher) {
      toggleSidebar();
      // O divisor some com o painel recolhido; o foco vai para o botão que o reabre.
      await tick();
      botaoRecolher?.focus();
      return;
    }
    sidebarWidth = acao.largura;
    salvarPainel();
  }

  function toggleSidebar() {
    sidebarCollapsed = !sidebarCollapsed;
    salvarPainel();
  }

  function salvarPainel() {
    gravarPainel(armazenamento(), { largura: sidebarWidth, recolhido: sidebarCollapsed });
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

<svelte:window bind:innerWidth={larguraJanela} on:resize={atualizarMaximizada} />

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
    <div class="drag-overlay" transition:fade={{ duration: duracaoMovimento(DUR_FAST) }}>
      <div class="overlay-box">
        <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-file-archive"><path d="M4 22V4c0-.5.2-1 .6-1.4C5 2.2 5.5 2 6 2h8l6 6v12c0 .5-.2 1-.6 1.4-.4.4-.9.6-1.4.6H6c-.5 0-1-.2-1.4-.6C4.2 21 4 20.5 4 20Z"/><path d="M14 2v6h6"/><circle cx="10" cy="14" r="2"/><path d="M16 14h-4"/><path d="M12 12v4"/><path d="M8 14H6"/></svg>
        <p>Solte o arquivo ZIP em qualquer lugar para processar</p>
      </div>
    </div>
  {/if}

  <!-- Custom Design Titlebar (replacing native title bar) -->
  <div class="custom-titlebar" style="--wails-draggable: drag">
    <div class="brand-title">
      <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 256 256" class="brand-icon" aria-hidden="true"><defs><linearGradient id="brand-grad" x1="0" y1="0" x2="1" y2="1"><stop offset="0" class="brand-de"/><stop offset="1" class="brand-ate"/></linearGradient></defs><rect x="4" y="4" width="248" height="248" rx="48" fill="url(#brand-grad)"/><path d="M66 132H190A62 62 0 1 0 172 176" fill="none" class="brand-traco" stroke-width="34" stroke-linecap="round" stroke-linejoin="round"/></svg>
      <h1>eprocIA</h1>
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
      <button
        class="titlebar-btn"
        on:click={toggleTheme}
        title="Tema: {NOME_TEMA[temaPref]} (clique para {NOME_TEMA[proximoTema(temaPref)]})"
        aria-label="Tema: {NOME_TEMA[temaPref]}. Mudar para {NOME_TEMA[proximoTema(temaPref)]}"
      >
        {#if temaPref === 'system'}
          <svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-monitor"><rect width="20" height="14" x="2" y="3" rx="2"/><line x1="8" x2="16" y1="21" y2="21"/><line x1="12" x2="12" y1="17" y2="21"/></svg>
        {:else if temaPref === 'light'}
          <svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-sun"><circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/></svg>
        {:else}
          <svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-moon"><path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/></svg>
        {/if}
      </button>

      <!-- Minimize Window -->
      <button class="titlebar-btn" on:click={WindowMinimise} title="Minimizar" aria-label="Minimizar">
        <svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="5" y1="12" x2="19" y2="12"/></svg>
      </button>

      <!-- Maximize/Restore Window -->
      <button
        class="titlebar-btn"
        on:click={alternarMaximizar}
        title={maximizada ? 'Restaurar' : 'Maximizar'}
        aria-label={maximizada ? 'Restaurar' : 'Maximizar'}
      >
        {#if maximizada}
          <svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-copy"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
        {:else}
          <svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/></svg>
        {/if}
      </button>

      <!-- Close Window (Hides to Tray) -->
      <button class="titlebar-btn btn-close" on:click={handleCloseWindow} title="Fechar (Minimiza para a bandeja)" aria-label="Fechar (minimiza para a bandeja)">
        <svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
      </button>
    </div>
  </div>

  <!-- Main Content Grid -->
  <main class="grid-layout">
    <!-- Sidebar (Collapsible & Resizable) -->
    <aside
      id="sidebar"
      class="sidebar"
      style="width: {sidebarCollapsed ? '48px' : larguraPainel + 'px'}"
      style:--largura-painel="{larguraPainel}px"
      class:collapsed={sidebarCollapsed}
      class:resizing={isResizing}
    >
      <div class="sidebar-header-toggle">
        <span class="sidebar-title">Painel de Controle</span>
        <button
          class="btn-toggle-sidebar"
          bind:this={botaoRecolher}
          on:click={toggleSidebar}
          title={sidebarCollapsed ? 'Expandir painel' : 'Recolher painel'}
          aria-label={sidebarCollapsed ? 'Expandir painel' : 'Recolher painel'}
          aria-expanded={!sidebarCollapsed}
          aria-controls="sidebar"
        >
          {#if sidebarCollapsed}
            <svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"/></svg>
          {:else}
            <svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>
          {/if}
        </button>
      </div>

      <!-- Escondido, não desmontado: o Settings mantém o aviso e a config. -->
      <div class="sidebar-content" inert={sidebarCollapsed}>
        <!-- Settings Component -->
        <Settings bind:config {erroConfig} aoFalharSalvar={refreshConfig} />

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
        </div>
      </div>

      {#if sidebarCollapsed}
        <!-- Collapsed Sidebar Vertical Tools -->
        <div class="collapsed-tools" in:fade={{ duration: duracaoMovimento(DUR_FAST), delay: duracaoMovimento(DUR_FAST) }}>
          <div class="collapsed-indicator" class:active={monitorando} class:inactive={!monitorando} role="status" title={textoMonitoramento}>
            <span class="collapsed-dot" aria-hidden="true"></span>
            <span class="sr-only">{textoMonitoramento}</span>
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
          role="slider"
          aria-label="Redimensionador de Menu"
          aria-orientation="horizontal"
          aria-valuemin={LARGURA_MIN_PAINEL}
          aria-valuemax={maxPainel}
          aria-valuenow={larguraPainel}
          aria-valuetext="{larguraPainel} pixels"
          aria-controls="sidebar"
          tabindex="0"
          on:pointerdown={startResizing}
          on:pointermove={handlePointerMove}
          on:pointerup={stopResizing}
          on:pointercancel={stopResizing}
          on:lostpointercapture={stopResizing}
          on:keydown={handleResizeKeydown}
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
    <div class="status-item status-main">
      <span class="status-dot" class:active={config && config.watchdog_enabled && config.monitored_path} class:inactive={!config || !config.watchdog_enabled || !config.monitored_path}></span>
      {#if config && config.watchdog_enabled && config.monitored_path}
        <span class="status-text truncate" title={config.monitored_path}>
          Pasta monitorada: <strong>{config.monitored_path}</strong>
        </span>
      {:else if config && !config.watchdog_enabled && config.monitored_path}
        <span class="status-text truncate" title={config.monitored_path}>
          Monitoramento pausado (pasta configurada: <strong>{config.monitored_path}</strong>)
        </span>
      {:else}
        <span class="status-text truncate">Monitoramento inativo. Configure uma pasta no painel ao lado.</span>
      {/if}
    </div>
    <div class="status-item status-count font-tabular">
      <span>{rotuloFila(items.length)}</span>
      {#if versao}
        <span class="status-sep" aria-hidden="true"></span>
        <span title="Versão do eprocIA">v{versao}</span>
      {/if}
    </div>
  </footer>
</div>

<style>
  .app-container {
    display: flex;
    flex-direction: column;
    height: 100vh;
    background-color: var(--bg-main);
    background-image: radial-gradient(at 0% 0%, var(--accent-glow) 0px, transparent 50%),
                      radial-gradient(at 100% 100%, var(--success-glow) 0px, transparent 50%);
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
    background-color: var(--overlay);
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
    box-shadow: var(--shadow-overlay);
    animation: zoomIn var(--dur-fast) var(--ease-out);
  }

  .overlay-box p {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  @keyframes zoomIn {
    from { transform: scale(0.98); opacity: 0; }
    to { transform: scale(1); opacity: 1; }
  }

  /* Custom Frameless Titlebar */
  .custom-titlebar {
    height: 38px;
    background-color: var(--glass);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    border-bottom: 1px solid var(--glass-border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 12px;
    flex-shrink: 0;
    user-select: none;
    transition: background-color var(--dur-base) var(--ease-out), border-color var(--dur-base) var(--ease-out);
  }

  .brand-title {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .brand-icon {
    flex: none;
    display: block;
  }
  .brand-de {
    stop-color: var(--brand-from);
  }
  .brand-ate {
    stop-color: var(--brand-to);
  }
  .brand-traco {
    stroke: var(--on-brand);
  }

  .brand-title h1 {
    font-size: 13px;
    font-weight: 700;
    margin: 0;
    line-height: 1.2;
    color: var(--text-primary);
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
    transition: color var(--dur-fast) var(--ease-out), background-color var(--dur-fast) var(--ease-out);
  }

  .titlebar-btn:hover {
    color: var(--text-primary);
    background-color: var(--surface-hover);
  }

  .titlebar-btn.btn-close:hover {
    color: var(--on-accent);
    background-color: var(--error);
  }

  .pulse-indicator {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 10px;
    color: var(--accent);
    font-weight: 600;
    background-color: var(--accent-soft);
    padding: 2px 8px;
    border-radius: 12px;
    margin-right: 8px;
  }

  .pulse-dot {
    position: relative;
    width: 5px;
    height: 5px;
    background-color: var(--accent);
    border-radius: 50%;
  }

  /* Anel num pseudo-elemento: só transform e opacity, sem repintar a cada quadro. */
  .pulse-dot::after {
    content: '';
    position: absolute;
    inset: 0;
    border-radius: 50%;
    background-color: var(--accent-ring);
    animation: pulse var(--dur-pulse) var(--ease-out) infinite;
  }

  @keyframes pulse {
    from {
      transform: scale(1);
      opacity: 0.8;
    }
    to {
      transform: scale(2.6);
      opacity: 0;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .pulse-dot::after {
      animation: none;
      opacity: 0;
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
    background-color: var(--glass);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    border-right: 1px solid var(--glass-border);
    position: relative;
    flex-shrink: 0;
    overflow: visible; /* Needed for resize handle to overlap */
    transition: width var(--dur-fast) var(--ease-out), background-color var(--dur-base) var(--ease-out), border-color var(--dur-base) var(--ease-out);
  }

  /* A largura acompanha o ponteiro 1:1; a transição só vale para recolher e teclado. */
  .sidebar.resizing {
    transition: none;
  }

  .sidebar.collapsed {
    align-items: center;
    width: 48px !important;
    overflow: hidden;
  }

  .sidebar-header-toggle {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 12px;
    border-bottom: 1px solid var(--glass-border);
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
    white-space: nowrap;
    transition: opacity var(--dur-fast) var(--ease-out) var(--dur-fast);
  }

  /* Recolher: o conteúdo some com fade enquanto a coluna encolhe, preso à
     largura anterior e cortado pelo overflow do aside; expandir: só aparece
     depois que a largura terminou (atraso de --dur-fast). */
  .sidebar.collapsed .sidebar-title {
    position: absolute;
    left: 12px;
    opacity: 0;
    visibility: hidden;
    transition: opacity var(--dur-fast) var(--ease-out), visibility var(--dur-fast) linear;
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
    transition: opacity var(--dur-fast) var(--ease-out) var(--dur-fast);
  }
  .sidebar.collapsed .sidebar-content {
    position: absolute;
    top: 38px;
    left: 0;
    bottom: 0;
    width: var(--largura-painel);
    opacity: 0;
    visibility: hidden;
    transition: opacity var(--dur-fast) var(--ease-out), visibility var(--dur-fast) linear;
  }

  /* Resizing Handle */
  /* Captura 8px centrados na borda; só a linha de 2px aparece. */
  .resize-handle {
    position: absolute;
    top: 0;
    right: -4px;
    bottom: 0;
    width: 8px;
    cursor: col-resize;
    z-index: 10;
    touch-action: none;
  }

  .resize-handle::before {
    content: '';
    position: absolute;
    top: 0;
    bottom: 0;
    left: 3px;
    width: 2px;
    transition: background-color var(--dur-fast) var(--ease-out);
  }

  .resize-handle:hover::before,
  .resize-handle.resizing::before,
  .resize-handle:focus-visible::before {
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
    transition: color var(--dur-fast) var(--ease-out), border-color var(--dur-fast) var(--ease-out);
  }

  .collapsed-tool-btn:hover {
    color: var(--accent);
    border-color: var(--accent);
  }

  .content-area {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    padding: 8px;
    background-color: transparent;
  }

  /* Manual process card */
  .manual-card {
    background-color: var(--card-bg);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    border: 1px solid var(--card-border);
    border-radius: 10px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    box-shadow: var(--shadow-sm);
    transition: background-color var(--dur-base) var(--ease-out), border-color var(--dur-base) var(--ease-out);
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
    transition: border-color var(--dur-fast) var(--ease-out), background-color var(--dur-fast) var(--ease-out);
  }

  .manual-select-btn:hover {
    border-color: var(--accent);
    background-color: var(--surface-hover);
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
    background-color: var(--glass);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    border-top: 1px solid var(--glass-border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 12px;
    font-size: 11px;
    color: var(--text-secondary);
    flex-shrink: 0;
    user-select: none;
    transition: background-color var(--dur-base) var(--ease-out), border-color var(--dur-base) var(--ease-out);
  }

  .status-item {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .status-main {
    flex: 1;
    min-width: 0;
  }

  .status-count {
    flex-shrink: 0;
    white-space: nowrap;
    margin-left: 12px;
  }

  .status-sep {
    width: 1px;
    height: 10px;
    background-color: var(--border);
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
