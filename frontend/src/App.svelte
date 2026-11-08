<script>
  import Icone from './components/Icone.svelte';
  import ChevronLeft from 'lucide-svelte/icons/chevron-left';
  import ChevronRight from 'lucide-svelte/icons/chevron-right';
  import Copy from 'lucide-svelte/icons/copy';
  import FileArchive from 'lucide-svelte/icons/file-archive';
  import FilePlusCorner from 'lucide-svelte/icons/file-plus-corner';
  import Minus from 'lucide-svelte/icons/minus';
  import Monitor from 'lucide-svelte/icons/monitor';
  import Moon from 'lucide-svelte/icons/moon';
  import ScanText from 'lucide-svelte/icons/scan-text';
  import Sparkles from 'lucide-svelte/icons/sparkles';
  import Square from 'lucide-svelte/icons/square';
  import Sun from 'lucide-svelte/icons/sun';
  import Upload from 'lucide-svelte/icons/upload';
  import X from 'lucide-svelte/icons/x';
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
    SetTheme,
    SaveConfig,
    SelectDirectory
  } from './wailsjs/go/main/App.js';
  import Settings from './components/Settings.svelte';
  import ProcessList from './components/ProcessList.svelte';
  import { contarAtivos, rotuloFila } from './lib/estado.js';
  import { criarFilaPorQuadro } from './lib/quadro.js';
  import { fade, fly } from 'svelte/transition';
  import { LARGURA_MIN_PAINEL, criarArrasto, larguraMaxPainel, limitarPainel, teclaDivisor } from './lib/divisor.js';
  import { DUR_BASE, DUR_FAST, criarAviso, duracaoMovimento } from './lib/aviso.js';
  import { OPCOES_LIMPEZA, OPCOES_OCR, nivelOpcao, proximaOpcao, rotuloOpcao } from './lib/atividade.js';
  import { cubicOut } from 'svelte/easing';
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
  // Atividade por documento de cada item (queue:atividade), por id.
  let atividades = {};
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
      EventsOn('queue:update', (data) => {
        filaQuadro.lista(data);
        podarAtividades(data);
      })
    );

    unsubscribers.push(
      EventsOn('queue:atividade', (inst) => {
        if (inst?.item_id) atividades = { ...atividades, [inst.item_id]: inst };
      })
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
    aviso.limpar();
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

  // Item removido ou de volta à fila (reprocessar) não mostra a atividade antiga.
  function podarAtividades(lista) {
    const vivos = new Set((lista || []).filter((i) => i.status !== 'pending').map((i) => i.id));
    const ids = Object.keys(atividades);
    if (ids.every((id) => vivos.has(id))) return;
    atividades = Object.fromEntries(Object.entries(atividades).filter(([id]) => vivos.has(id)));
  }

  // Mudanças rápidas da config fora do formulário (painel recolhido e rodapé).
  // O Settings recebe a config nova pelo bind; o aviso aparece aqui porque o
  // formulário pode estar recolhido.
  let avisoRapido = null;
  const aviso = criarAviso((a) => { avisoRapido = a; });
  let anuncio = '';

  async function salvarRapido(mudanca, texto) {
    if (!config) return;
    const novo = { ...config, ...mudanca };
    try {
      await SaveConfig(novo);
      config = novo;
      anuncio = texto;
    } catch (err) {
      const msg = String(err?.message ?? err);
      // "configurações salvas, mas..." já se explica sozinha.
      aviso.mostrar(/^configurações salvas/i.test(msg) ? msg[0].toUpperCase() + msg.slice(1) : 'Erro ao salvar: ' + msg, 'error');
      await refreshConfig();
    }
  }

  function ciclarOcr() {
    const prox = proximaOpcao(OPCOES_OCR, config?.ocr_mode);
    salvarRapido({ ocr_mode: prox.valor }, `OCR: ${prox.rotulo}`);
  }

  function ciclarLimpeza() {
    const prox = proximaOpcao(OPCOES_LIMPEZA, config?.cleaning_level);
    salvarRapido({ cleaning_level: prox.valor }, `Higienização: ${prox.rotulo}`);
  }

  // Sem pasta configurada, ligar o monitoramento começa pela escolha da pasta.
  async function alternarMonitoramento() {
    if (!config) return;
    if (!config.monitored_path) {
      try {
        const pasta = await SelectDirectory();
        if (pasta) await salvarRapido({ monitored_path: pasta, watchdog_enabled: true }, 'Monitoramento ativado');
      } catch (err) {
        aviso.mostrar('Erro ao selecionar pasta: ' + (err?.message ?? err), 'error');
      }
      return;
    }
    const ligar = !config.watchdog_enabled;
    await salvarRapido({ watchdog_enabled: ligar }, ligar ? 'Monitoramento ativado' : 'Monitoramento pausado');
  }

  $: rotuloOcr = rotuloOpcao(OPCOES_OCR, config?.ocr_mode);
  $: rotuloLimpeza = rotuloOpcao(OPCOES_LIMPEZA, config?.cleaning_level);
  $: acaoMonitor = !config?.monitored_path
    ? 'Escolher pasta e ativar o monitoramento'
    : (config.watchdog_enabled ? 'Pausar o monitoramento' : 'Ativar o monitoramento');

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
        <Icone icone={FileArchive} tamanho={48} cor="var(--accent)" />
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
          <Icone icone={Monitor} tamanho={14} />
        {:else if temaPref === 'light'}
          <Icone icone={Sun} tamanho={14} />
        {:else}
          <Icone icone={Moon} tamanho={14} />
        {/if}
      </button>

      <!-- Minimize Window -->
      <button class="titlebar-btn" on:click={WindowMinimise} title="Minimizar" aria-label="Minimizar">
        <Icone icone={Minus} tamanho={14} />
      </button>

      <!-- Maximize/Restore Window -->
      <button
        class="titlebar-btn"
        on:click={alternarMaximizar}
        title={maximizada ? 'Restaurar' : 'Maximizar'}
        aria-label={maximizada ? 'Restaurar' : 'Maximizar'}
      >
        {#if maximizada}
          <Icone icone={Copy} tamanho={14} />
        {:else}
          <Icone icone={Square} tamanho={14} />
        {/if}
      </button>

      <!-- Close Window (Hides to Tray) -->
      <button class="titlebar-btn btn-close" on:click={handleCloseWindow} title="Fechar (Minimiza para a bandeja)" aria-label="Fechar (minimiza para a bandeja)">
        <Icone icone={X} tamanho={14} />
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
            <Icone icone={ChevronRight} />
          {:else}
            <Icone icone={ChevronLeft} />
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
            <Icone icone={FilePlusCorner} tamanho={12} />
            <h3>Importação Manual</h3>
          </div>

          <button
            type="button"
            class="manual-select-btn"
            on:click={triggerManualSelect}
          >
            <Icone icone={Upload} tamanho={14} />
            <span>Selecionar arquivos ZIP...</span>
          </button>
        </div>
      </div>

      {#if sidebarCollapsed}
        <!-- Collapsed Sidebar Vertical Tools -->
        <div class="collapsed-tools" in:fade={{ duration: duracaoMovimento(DUR_FAST), delay: duracaoMovimento(DUR_FAST) }}>
          <button
            type="button"
            class="collapsed-indicator"
            class:active={monitorando}
            class:inactive={!monitorando}
            on:click={alternarMonitoramento}
            title="{textoMonitoramento}. Clique para {acaoMonitor.toLowerCase()}"
            aria-label="{textoMonitoramento}. {acaoMonitor}"
            aria-pressed={monitorando}
            disabled={!config}
          >
            <span class="collapsed-dot" aria-hidden="true"></span>
          </button>

          <button class="collapsed-tool-btn" on:click={triggerManualSelect} title="Processar ZIP Manualmente" aria-label="Processar ZIP manualmente">
            <Icone icone={Upload} />
          </button>

          <button
            type="button"
            class="collapsed-tool-btn nivel-btn"
            on:click={ciclarOcr}
            disabled={!config}
            title="OCR: {rotuloOcr}. Clique para {rotuloOpcao(OPCOES_OCR, proximaOpcao(OPCOES_OCR, config?.ocr_mode).valor)}"
            aria-label="OCR: {rotuloOcr}. Mudar para {rotuloOpcao(OPCOES_OCR, proximaOpcao(OPCOES_OCR, config?.ocr_mode).valor)}"
          >
            <Icone icone={ScanText} tamanho={14} />
            <span class="nivel" aria-hidden="true">
              {#each OPCOES_OCR.slice(1) as _, i}
                <span class="nivel-ponto" class:cheio={i < nivelOpcao(OPCOES_OCR, config?.ocr_mode)}></span>
              {/each}
            </span>
          </button>

          <button
            type="button"
            class="collapsed-tool-btn nivel-btn"
            on:click={ciclarLimpeza}
            disabled={!config}
            title="Higienização: {rotuloLimpeza}. Clique para {rotuloOpcao(OPCOES_LIMPEZA, proximaOpcao(OPCOES_LIMPEZA, config?.cleaning_level).valor)}"
            aria-label="Higienização: {rotuloLimpeza}. Mudar para {rotuloOpcao(OPCOES_LIMPEZA, proximaOpcao(OPCOES_LIMPEZA, config?.cleaning_level).valor)}"
          >
            <Icone icone={Sparkles} tamanho={14} />
            <span class="nivel" aria-hidden="true">
              {#each OPCOES_LIMPEZA.slice(1) as _, i}
                <span class="nivel-ponto" class:cheio={i < nivelOpcao(OPCOES_LIMPEZA, config?.cleaning_level)}></span>
              {/each}
            </span>
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
      <ProcessList bind:items={items} {isPaused} {atividades} {config} />
    </section>
  </main>

  <!-- Footer Status Bar -->
  <footer>
    <div class="status-item status-main">
      <button
        type="button"
        class="status-dot-btn"
        on:click={alternarMonitoramento}
        disabled={!config}
        title={acaoMonitor}
        aria-label={acaoMonitor}
        aria-pressed={monitorando}
      >
        <span class="status-dot" class:active={monitorando} class:inactive={!monitorando}></span>
      </button>
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

  <!-- Sempre montadas: o leitor de tela só anuncia mudanças numa região que
       já existia antes da mensagem. -->
  <div class="sr-only" role="status" aria-live="polite">{anuncio}</div>
  <div class="aviso-rapido-regiao" role="alert" aria-atomic="true">
    {#if avisoRapido}
      <div
        class="aviso-rapido"
        in:fly={{ y: 8, duration: duracaoMovimento(DUR_BASE), easing: cubicOut }}
        out:fade={{ duration: duracaoMovimento(DUR_FAST) }}
      >
        <span class="aviso-rapido-texto">{avisoRapido.texto}</span>
        <button class="aviso-rapido-fechar" on:click={aviso.fechar} aria-label="Fechar aviso" title="Fechar aviso">
          <Icone icone={X} tamanho={12} />
        </button>
      </div>
    {/if}
  </div>
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
    background-color: var(--overlay);
    z-index: 1000;
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none;
  }

  .overlay-box {
    border: 2px dashed var(--accent);
    border-radius: 6px;
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
    font-size: var(--fs-title);
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
    background-color: var(--surface);
    border-bottom: 1px solid var(--border);
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
    font-size: var(--fs-base);
    font-weight: 600;
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
    font-size: var(--fs-meta);
    color: var(--accent);
    font-weight: 600;
    background-color: var(--accent-soft);
    padding: 2px 8px;
    border-radius: 4px;
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
    background-color: var(--surface);
    border-right: 1px solid var(--border);
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

  /* Só o botão de recolher: o painel não repete um título acima das seções. */
  .sidebar-header-toggle {
    display: flex;
    justify-content: flex-end;
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

  /* Botão de 24px em volta do ponto de 6px: alvo fácil sem mudar o visual. */
  .collapsed-indicator {
    width: 24px;
    height: 24px;
    padding: 0;
    border: none;
    background: transparent;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: background-color var(--dur-fast) var(--ease-out);
  }

  .collapsed-indicator:hover {
    background-color: var(--surface-hover);
  }

  .collapsed-indicator:hover .collapsed-dot {
    transform: scale(1.4);
  }

  .collapsed-indicator:disabled,
  .collapsed-tool-btn:disabled,
  .status-dot-btn:disabled {
    cursor: default;
    opacity: 0.6;
  }

  .collapsed-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    transition: transform var(--dur-fast) var(--ease-out), background-color var(--dur-fast) var(--ease-out);
  }

  /* Ícone com os pontos de nível embaixo: o nível muda a cada clique. */
  .nivel-btn {
    flex-direction: column;
    gap: 3px;
    height: 36px;
  }

  .nivel {
    display: flex;
    gap: 2px;
  }

  .nivel-ponto {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background-color: var(--border-strong);
    transition: background-color var(--dur-fast) var(--ease-out);
  }

  .nivel-ponto.cheio {
    background-color: var(--accent);
  }

  .collapsed-indicator.active .collapsed-dot {
    background-color: var(--success);
  }

  .collapsed-indicator.inactive .collapsed-dot {
    background-color: var(--warning);
  }

  .collapsed-tool-btn {
    background-color: var(--surface-hover);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    width: 32px;
    height: 32px;
    border-radius: 4px;
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
  /* Seção do painel: divisor de 1px no lugar da moldura de cartão. */
  .manual-card {
    border-top: 1px solid var(--border);
    padding-top: 8px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    transition: border-color var(--dur-base) var(--ease-out);
  }

  .manual-card .card-header {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--text-secondary);
  }

  .manual-card h3 {
    font-size: var(--fs-section);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--text-secondary);
    margin: 0;
  }

  .manual-select-btn {
    background-color: var(--bg-main);
    border: 1px solid var(--border);
    color: var(--text-primary);
    font-size: var(--fs-base);
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
    background-color: var(--surface);
    border-top: 1px solid var(--border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 12px;
    font-size: var(--fs-meta);
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

  .status-dot-btn {
    flex-shrink: 0;
    width: 18px;
    height: 18px;
    margin: 0 -6px;
    padding: 0;
    border: none;
    border-radius: 50%;
    background: transparent;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
  }

  .status-dot-btn:hover {
    background-color: var(--surface-hover);
  }

  .status-dot-btn:hover .status-dot {
    transform: scale(1.4);
  }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
    transition: transform var(--dur-fast) var(--ease-out), background-color var(--dur-fast) var(--ease-out);
  }

  .aviso-rapido-regiao {
    position: absolute;
    left: 50%;
    bottom: 34px;
    transform: translateX(-50%);
    z-index: 900;
    max-width: min(480px, calc(100% - 32px));
  }

  .aviso-rapido {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    font-size: var(--fs-meta);
    font-weight: 500;
    color: var(--error-strong);
    background-color: var(--surface);
    border: 1px solid currentColor;
    border-radius: 4px;
    box-shadow: var(--shadow-overlay);
  }

  .aviso-rapido-texto {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .aviso-rapido-fechar {
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
  }

  .aviso-rapido-fechar:hover {
    background-color: var(--error-soft);
  }

  .status-dot.active {
    background-color: var(--success);
  }

  .status-dot.inactive {
    background-color: var(--warning);
  }

  .status-text {
    font-size: var(--fs-meta);
  }

  .font-tabular {
    font-variant-numeric: tabular-nums;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
