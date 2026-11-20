<script>
  import Icone from './components/Icone.svelte';
  import MenuNivel from './components/MenuNivel.svelte';
  import Check from 'lucide-svelte/icons/check';
  import ChevronLeft from 'lucide-svelte/icons/chevron-left';
  import CirclePause from 'lucide-svelte/icons/circle-pause';
  import Copy from 'lucide-svelte/icons/copy';
  import FileArchive from 'lucide-svelte/icons/file-archive';
  import FilePlusCorner from 'lucide-svelte/icons/file-plus-corner';
  import FolderSearch from 'lucide-svelte/icons/folder-search';
  import FolderX from 'lucide-svelte/icons/folder-x';
  import Minus from 'lucide-svelte/icons/minus';
  import Monitor from 'lucide-svelte/icons/monitor';
  import Moon from 'lucide-svelte/icons/moon';
  import ScanText from 'lucide-svelte/icons/scan-text';
  import SettingsIcon from 'lucide-svelte/icons/settings';
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
    JanelaVisivel,
    Hide,
    SetTheme,
    SaveConfig,
    SelectDirectory
  } from './wailsjs/go/main/App.js';
  import Settings from './components/Settings.svelte';
  import ProcessList from './components/ProcessList.svelte';
  import Aviso from './components/Aviso.svelte';
  import { contarAtivos, rotuloFila } from './lib/estado.js';
  import { criarFilaPorQuadro } from './lib/quadro.js';
  import { fade } from 'svelte/transition';
  import { LARGURA_MIN_PAINEL, criarArrasto, larguraMaxPainel, limitarPainel, teclaDivisor } from './lib/divisor.js';
  import { DUR_BASE, DUR_FAST, avisoSoltura, criarAviso, duracaoMovimento, separarZips } from './lib/aviso.js';
  import { OPCOES_LIMPEZA, OPCOES_OCR, rotuloOpcao } from './lib/atividade.js';
  import { gravarPainel, lerPainel } from './lib/painel.js';
  import { rolagem } from './lib/rolagem.js';
  import { ROTULO_MONITORAMENTO, estadoMonitoramento, textoMonitoramento } from './lib/monitoramento.js';
  import { criarSoltura, textoAceitos, textoJaNaFila } from './lib/soltura.js';
  import {
    CONSULTA_ESCURO,
    NOME_TEMA,
    armazenamento,
    conciliarTema,
    gravarSalvo,
    lerPreferencia,
    lerSalvo,
    proximoTema,
    temaEfetivo,
    trocarTema
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
  let painelMaisAbaixo = false;
  const marcarPainel = (e) => (painelMaisAbaixo = e.abaixo);
  let isResizing = false;
  let maximizada = false;
  let botaoRecolher;

  // sidebarWidth guarda a escolha do usuário; a largura exibida é recalculada
  // quando a janela muda, e a escolha volta se a janela crescer de novo.
  $: maxPainel = larguraMaxPainel(larguraJanela);
  $: larguraPainel = limitarPainel(sidebarWidth, larguraJanela);

  // Overlay de soltar: só para arquivos, e confirma o que foi aceito.
  let estadoSoltura = null;
  const soltura = criarSoltura((e) => { estadoSoltura = e; }, globalThis, DUR_BASE);

  // Unsubscribe handlers
  let unsubscribers = [];

  const filaQuadro = criarFilaPorQuadro(() => items, (novos) => { items = novos; });

  // Pausar a fila não interrompe o item em curso, que segue ativo.
  $: processingCount = contarAtivos(items);

  // Mesmos rótulos e ícones do painel (Settings.svelte).
  const ICONE_MONITORAMENTO = { monitorando: FolderSearch, pausado: CirclePause, sem_pasta: FolderX };
  $: estado = estadoMonitoramento(config);
  $: textoMonitor = textoMonitoramento(config);

  $: temaAtual = temaEfetivo(temaPref, sistemaEscuro);
  $: {
    document.documentElement.classList.toggle('light', temaAtual === 'light');
  }

  function mudouTemaDoSistema(e) {
    trocarTema(document, async () => {
      sistemaEscuro = e.matches;
      await tick();
    }, duracaoMovimento(DUR_BASE));
  }

  onMount(async () => {
    consultaTema.addEventListener('change', mudouTemaDoSistema);

    // Antes de qualquer await: o aviso pode chegar enquanto a consulta volta.
    let visibilidadeAvisada = false;
    unsubscribers.push(
      EventsOn('janela:visivel', (visivel) => {
        visibilidadeAvisada = true;
        mudarVisibilidade(visivel);
      })
    );
    JanelaVisivel().then((visivel) => {
      if (!visibilidadeAvisada) mudarVisibilidade(visivel);
    }).catch((err) => console.warn('Erro ao consultar a visibilidade da janela:', err));
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
        if (janelaOculta) listaOculta = data;
        else podarAtividades(data);
      })
    );

    unsubscribers.push(
      EventsOn('queue:atividade', (inst) => {
        if (!inst?.item_id) return;
        if (janelaOculta) atividadesOcultas[inst.item_id] = inst;
        else atividades = { ...atividades, [inst.item_id]: inst };
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
        const { zips, ignorados } = separarZips(paths);
        // O overlay e o anúncio esperam a fila dizer quantos entraram de fato:
        // um ZIP já pendente ou em processamento é descartado por ela.
        if (zips.length === 0) soltura.recebido(0);
        else {
          AddFilesToQueue(zips).then((n) => {
            soltura.recebido(n);
            anunciar(n > 0 ? textoAceitos(n) : textoJaNaFila(zips.length));
          }).catch((err) => {
            soltura.recebido(0);
            console.error('Erro ao adicionar à fila:', err);
            aviso.mostrar('Erro ao adicionar à fila: ' + (err?.message ?? err), 'error');
          });
        }
        const r = avisoSoltura(zips.length, ignorados);
        if (r) aviso.mostrar(r.texto, r.tipo);
      }, false); // useDropTarget = false for global drop
    } catch (err) {
      console.warn('Wails OnFileDrop not available:', err);
    }
  });

  onDestroy(() => {
    stopResizing();
    aviso.limpar();
    avisoPainel.limpar();
    soltura.limpar();
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

  // Na bandeja a WebView2 segue renderizando (o Windows só esconde a janela):
  // :root.oculta pausa as animações e a fila e a atividade só guardam o último
  // estado, aplicado ao reabrir.
  let janelaOculta = false;
  let atividadesOcultas = {};
  let listaOculta = null;

  function mudarVisibilidade(visivel) {
    janelaOculta = !visivel;
    document.documentElement.classList.toggle('oculta', janelaOculta);
    if (janelaOculta) {
      filaQuadro.pausar();
      return;
    }
    if (Object.keys(atividadesOcultas).length) {
      atividades = { ...atividades, ...atividadesOcultas };
      atividadesOcultas = {};
    }
    if (listaOculta) {
      podarAtividades(listaOculta);
      listaOculta = null;
    }
    filaQuadro.retomar();
  }

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

  // Pé do painel, fora da área rolável: erros do Settings e da importação
  // manual. Com o painel recolhido o pé some, e o aviso vai para o flutuante.
  let avisoPe = null;
  const avisoPainel = criarAviso((a) => { avisoPe = a; });
  function avisarPainel(texto, tipo) {
    (sidebarCollapsed ? aviso : avisoPainel).mostrar(texto, tipo);
  }
  let anuncio = '';

  // Esvazia antes de escrever: o mesmo texto duas vezes seguidas é anunciado de novo.
  async function anunciar(texto) {
    if (!texto) return;
    anuncio = '';
    await tick();
    anuncio = texto;
  }

  async function salvarRapido(mudanca, texto) {
    if (!config) return false;
    const novo = { ...config, ...mudanca };
    try {
      await SaveConfig(novo);
      config = novo;
      await anunciar(texto);
      return true;
    } catch (err) {
      const msg = String(err?.message ?? err);
      // "configurações salvas, mas..." já se explica sozinha.
      aviso.mostrar(/^configurações salvas/i.test(msg) ? msg[0].toUpperCase() + msg.slice(1) : 'Erro ao salvar: ' + msg, 'error');
      await refreshConfig();
      return false;
    }
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

  // Tudo dentro do retorno: com cliques seguidos, cada troca parte do tema
  // que a anterior já aplicou.
  function toggleTheme() {
    trocarTema(document, async () => {
      temaPref = proximoTema(temaPref);
      gravarSalvo(armazenamento(), temaPref);
      gravarTemaNoBackend(temaPref);
      await tick();
    }, duracaoMovimento(DUR_BASE));
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
      avisarPainel('Erro ao selecionar arquivos: ' + (err?.message ?? err), 'error');
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

  // preventDefault só em arrasto de arquivos; texto arrastado segue o padrão.
  function handleDragEnter(e) {
    if (soltura.entrar(e.dataTransfer)) e.preventDefault();
  }

  function handleDragOver(e) {
    if (estadoSoltura) e.preventDefault();
  }

  function handleDragLeave(e) {
    soltura.sair(e.dataTransfer);
  }

  function handleWindowDrop(e) {
    // Sem stopPropagation: o WebView2/Wails precisa do evento para o OnFileDrop.
    soltura.soltar(e.dataTransfer);
  }
</script>

<svelte:window
  bind:innerWidth={larguraJanela}
  on:resize={atualizarMaximizada}
  on:dragover={handleDragOver}
  on:dragenter={handleDragEnter}
  on:dragleave={handleDragLeave}
  on:drop={handleWindowDrop}
/>

<div class="app-container">
  <!-- Full Window Drag & Drop Overlay -->
  {#if estadoSoltura}
    <div class="drag-overlay" transition:fade={{ duration: duracaoMovimento(DUR_FAST) }}>
      <div class="overlay-box">
        {#if estadoSoltura.fase === 'aceito'}
          <Icone icone={Check} tamanho={48} cor="var(--accent)" />
          <p>{estadoSoltura.texto}</p>
        {:else}
          <Icone icone={FileArchive} tamanho={48} cor="var(--accent)" />
          <p>Solte o arquivo ZIP em qualquer lugar para processar</p>
        {/if}
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
        <div class="pulse-indicator" transition:fade={{ duration: duracaoMovimento(DUR_FAST) }}>
          <span class="pulse-dot"></span>
          <span>{processingCount} ativo{processingCount > 1 ? 's' : ''}</span>
        </div>
      {/if}

      <!-- Theme Switcher -->
      <button
        class="titlebar-btn btn-tema"
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

      <span class="titlebar-divisor" aria-hidden="true"></span>

      <!-- Minimize Window -->
      <button class="titlebar-btn btn-janela" on:click={WindowMinimise} title="Minimizar" aria-label="Minimizar">
        <Icone icone={Minus} tamanho={14} />
      </button>

      <!-- Maximize/Restore Window -->
      <button
        class="titlebar-btn btn-janela"
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
      <button class="titlebar-btn btn-janela btn-close" on:click={handleCloseWindow} title="Fechar (Minimiza para a bandeja)" aria-label="Fechar (minimiza para a bandeja)">
        <Icone icone={X} tamanho={14} />
      </button>
    </div>
  </div>

  <!-- Main Content Grid -->
  <div class="grid-layout">
    <!-- Sidebar (Collapsible & Resizable) -->
    <aside
      id="sidebar"
      class="sidebar"
      aria-label="Configurações e importação"
      style="width: {sidebarCollapsed ? '48px' : larguraPainel + 'px'}"
      style:--largura-painel="{larguraPainel}px"
      class:collapsed={sidebarCollapsed}
      class:resizing={isResizing}
    >
      <div class="sidebar-header-toggle">
        <!-- Montado também recolhido: some com fade, como o conteúdo. -->
        <div class="painel-titulo">
          <Icone icone={SettingsIcon} tamanho={14} />
          <h2>Configurações</h2>
        </div>
        <button
          class="btn-toggle-sidebar"
          bind:this={botaoRecolher}
          on:click={toggleSidebar}
          title={sidebarCollapsed ? 'Expandir painel' : 'Recolher painel'}
          aria-label={sidebarCollapsed ? 'Expandir painel' : 'Recolher painel'}
          aria-expanded={!sidebarCollapsed}
          aria-controls="sidebar"
        >
          <span class="chevron-painel"><Icone icone={ChevronLeft} /></span>
        </button>
      </div>

      <!-- Escondido, não desmontado: o Settings mantém o aviso e a config. -->
      <div class="sidebar-content" class:mais-abaixo={painelMaisAbaixo} use:rolagem={marcarPainel} inert={sidebarCollapsed}>
        <!-- Settings Component -->
        <Settings bind:config {erroConfig} aoFalharSalvar={refreshConfig} avisar={avisarPainel} />

        <!-- Manual Process Card -->
        <div class="manual-card">
          <div class="card-header">
            <Icone icone={FilePlusCorner} tamanho={14} />
            <h2>Importação Manual</h2>
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

      <div class="painel-aviso">
        <Aviso aviso={avisoPe} aoFechar={avisoPainel.fechar} />
      </div>

      {#if sidebarCollapsed}
        <!-- Collapsed Sidebar Vertical Tools -->
        <div class="collapsed-tools" in:fade={{ duration: duracaoMovimento(DUR_FAST), delay: duracaoMovimento(DUR_FAST) }}>
          <button
            type="button"
            class="collapsed-indicator"
            class:pausado={estado === 'pausado'}
            on:click={alternarMonitoramento}
            title="{textoMonitor}. Clique para {acaoMonitor.toLowerCase()}"
            aria-label="Monitoramento"
            aria-pressed={estado === 'monitorando'}
            aria-describedby="estado-monitor-recolhido"
            disabled={!config}
          >
            {#key estado}
              <span class="monitor-icone" in:fade={{ duration: duracaoMovimento(DUR_FAST) }}>
                <Icone icone={ICONE_MONITORAMENTO[estado]} />
              </span>
            {/key}
          </button>
          <span id="estado-monitor-recolhido" class="sr-only">{textoMonitor}</span>

          <button class="collapsed-tool-btn" on:click={triggerManualSelect} title="Processar ZIP Manualmente" aria-label="Processar ZIP manualmente">
            <Icone icone={Upload} />
          </button>

          <MenuNivel
            id="ocr"
            legenda="OCR"
            icone={ScanText}
            opcoes={OPCOES_OCR}
            valor={config?.ocr_mode}
            desabilitado={!config}
            aoEscolher={(v) => salvarRapido({ ocr_mode: v }, `OCR: ${rotuloOpcao(OPCOES_OCR, v)}`)}
          />

          <MenuNivel
            id="limpeza"
            legenda="Higienização"
            icone={Sparkles}
            opcoes={OPCOES_LIMPEZA}
            valor={config?.cleaning_level}
            desabilitado={!config}
            aoEscolher={(v) => salvarRapido({ cleaning_level: v }, `Higienização: ${rotuloOpcao(OPCOES_LIMPEZA, v)}`)}
          />
        </div>
      {/if}

      <!-- Drag handle for resizing sidebar -->
      {#if !sidebarCollapsed}
        <div
          class="resize-handle"
          role="slider"
          aria-label="Largura do painel"
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
    <main class="content-area">
      <ProcessList bind:items={items} {isPaused} {atividades} {config} aoSelecionarZip={triggerManualSelect} />
    </main>
  </div>

  <!-- Footer Status Bar -->
  <footer>
    <div class="status-item status-main">
      <button
        type="button"
        class="monitor-btn"
        class:pausado={estado === 'pausado'}
        on:click={alternarMonitoramento}
        disabled={!config}
        title="{textoMonitor}. Clique para {acaoMonitor.toLowerCase()}"
        aria-label="Monitoramento"
        aria-pressed={estado === 'monitorando'}
      >
        {#key estado}
          <span class="monitor-icone" in:fade={{ duration: duracaoMovimento(DUR_FAST) }}>
            <Icone icone={ICONE_MONITORAMENTO[estado]} tamanho={12} />
          </span>
        {/key}
      </button>
      {#if estado === 'monitorando'}
        <span class="status-text truncate" title={config.monitored_path}>
          Monitorando: <strong>{config.monitored_path}</strong>
        </span>
      {:else if estado === 'pausado'}
        <span class="status-text truncate" title={config.monitored_path}>
          Pausado: <strong>{config.monitored_path}</strong>
        </span>
      {:else}
        <span class="status-text">{ROTULO_MONITORAMENTO[estado]}.</span>
        <button type="button" class="status-link" on:click={alternarMonitoramento} disabled={!config}>Escolher pasta para monitorar</button>
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
  <div class="aviso-flutuante">
    <Aviso aviso={avisoRapido} aoFechar={aviso.fechar} flutuante />
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
    /* Sem padding à direita: o Fechar encosta no canto, como no Windows 11. */
    padding: 0 0 0 12px;
    flex-shrink: 0;
    user-select: none;
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
    align-self: stretch;
    gap: 0;
  }

  .titlebar-btn {
    background-color: transparent;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .btn-tema {
    width: 28px;
    height: 28px;
    border-radius: 4px;
  }

  .titlebar-divisor {
    width: 1px;
    height: 16px;
    margin: 0 8px;
    background-color: var(--border);
  }

  /* Minimizar, maximizar e fechar: faixas da altura da barra, coladas entre si. */
  .btn-janela {
    width: 44px;
    height: 100%;
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
    color: var(--accent-text);
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
    transition: width var(--dur-fast) var(--ease-out);
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

  /* Rótulo "Configurações" à esquerda e botão de recolher à direita. */
  /* flex-end: o chevron fica a 12px da borda direita na largura toda e, na
     coluna de 48px, já centrado; nada salta enquanto a largura anima. */
  .sidebar-header-toggle {
    position: relative;
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border);
    height: 38px;
    flex-shrink: 0;
    width: 100%;
  }

  .painel-titulo {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    margin-right: auto;
    color: var(--text-secondary);
    transition: opacity var(--dur-fast) var(--ease-out) var(--dur-fast);
  }

  .painel-titulo h2 {
    font-size: var(--fs-section);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--text-secondary);
    margin: 0;
    white-space: nowrap;
  }

  /* Fora do fluxo, para o chevron centralizar na coluna de 48px. */
  .sidebar.collapsed .painel-titulo {
    position: absolute;
    left: 12px;
    opacity: 0;
    visibility: hidden;
    transition: opacity var(--dur-fast) var(--ease-out), visibility var(--dur-fast) linear;
  }

  .btn-toggle-sidebar {
    flex-shrink: 0;
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

  .chevron-painel {
    display: flex;
    transition: transform var(--dur-fast) var(--ease-out);
  }

  .sidebar.collapsed .chevron-painel {
    transform: rotate(180deg);
  }

  /* 12px: a mesma guia vertical da barra de título, do rodapé e da lista. */
  .sidebar-content {
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    flex: 1;
    overflow-y: auto;
    /* A sombra de "há mais abaixo" precisa ficar por cima do pé do painel. */
    position: relative;
    z-index: 1;
    transition: opacity var(--dur-fast) var(--ease-out) var(--dur-fast), box-shadow var(--dur-fast) var(--ease-out);
  }

  .sidebar-content.mais-abaixo {
    box-shadow: var(--shadow-sm);
  }
  .painel-aviso {
    flex-shrink: 0;
    /* o Aviso já traz 8px de margem lateral: +4px fecham a guia de 12px */
    padding: 0 4px;
  }

  .sidebar.collapsed .painel-aviso {
    display: none;
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

  /* Ícone do estado do monitoramento (o mesmo do rodapé e do painel). */
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
    color: var(--text-secondary);
    cursor: pointer;
  }

  .collapsed-indicator:hover {
    background-color: var(--surface-hover);
    color: var(--text-primary);
  }

  .collapsed-indicator.pausado,
  .monitor-btn.pausado {
    color: var(--warning);
  }

  .monitor-icone {
    display: flex;
  }

  .collapsed-indicator:disabled,
  .collapsed-tool-btn:disabled,
  .monitor-btn:disabled,
  .status-link:disabled {
    cursor: default;
    opacity: 0.6;
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
  /* flex: none: a 600px de altura a área rola em vez de espremer a seção. */
  .manual-card {
    flex: none;
    border-top: 1px solid var(--border);
    padding-top: 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .manual-card .card-header {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--text-secondary);
  }

  .manual-card h2 {
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

  /* Alvo de 24px no rodapé de 26px; a margem negativa alinha o ícone à guia de 12px. */
  .monitor-btn {
    flex-shrink: 0;
    width: 24px;
    height: 24px;
    margin: 0 -4px 0 -6px;
    padding: 0;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: var(--text-secondary);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
  }

  .monitor-btn:hover {
    background-color: var(--surface-hover);
    color: var(--text-primary);
  }

  .status-link {
    flex-shrink: 0;
    padding: 0 4px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: var(--accent-text);
    font-family: inherit;
    font-size: var(--fs-meta);
    text-decoration: underline;
    cursor: pointer;
    white-space: nowrap;
  }

  .status-link:hover {
    background-color: var(--surface-hover);
  }

  .aviso-flutuante {
    position: absolute;
    left: 50%;
    bottom: 34px;
    transform: translateX(-50%);
    z-index: 900;
    width: max-content;
    max-width: min(480px, calc(100% - 32px));
  }

  .status-text {
    font-size: var(--fs-meta);
  }

  /* Alto contraste do Windows: o fundo vira Canvas e o ponto sumiria. */
  @media (forced-colors: active) {
    .pulse-dot {
      forced-color-adjust: none;
      border: 1px solid Highlight;
      background-color: Highlight;
    }
  }

  .font-tabular {
    font-variant-numeric: tabular-nums;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
