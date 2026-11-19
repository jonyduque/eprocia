<script>
  import Icone from './Icone.svelte';
  import CirclePause from 'lucide-svelte/icons/circle-pause';
  import FolderOpen from 'lucide-svelte/icons/folder-open';
  import FolderSearch from 'lucide-svelte/icons/folder-search';
  import FolderX from 'lucide-svelte/icons/folder-x';
  import Salvo from './Salvo.svelte';
  import Segmentado from './Segmentado.svelte';
  import { onDestroy, tick } from 'svelte';
  import { criarConfirmacao } from '../lib/aviso.js';
  import { OPCOES_OCR, OPCOES_LIMPEZA } from '../lib/atividade.js';
  import { ROTULO_MONITORAMENTO, estadoMonitoramento, reticenciasMeio } from '../lib/monitoramento.js';
  import { SaveConfig, SelectDirectory } from '../wailsjs/go/main/App.js';

  // O App lê a config uma vez e a compartilha por bind; null enquanto carrega.
  export let config = null;
  export let erroConfig = '';
  // Chamado quando SaveConfig falha: o App relê a config gravada, para o
  // rodapé e o formulário não exibirem valores que não foram salvos.
  export let aoFalharSalvar = () => {};
  // Erros vão para o aviso do pé do painel (App), fora da área rolável, para
  // não cobrir os controles; o sucesso é o Check ao lado do controle.
  export let avisar = () => {};

  const PADRAO = {
    monitored_path: '',
    ocr_mode: 'normal',
    cleaning_level: 'standard',
    auto_start: false,
    watchdog_enabled: true,
    omitir_tipos: []
  };

  const AJUDA_OCR = {
    none: 'Páginas só com imagem ficam sem texto.',
    fast: 'Modelo leve: mais rápido, menos preciso.',
    normal: 'Equilíbrio entre velocidade e precisão.',
    best: 'Modelo mais preciso, porém mais lento.'
  };

  const AJUDA_LIMPEZA = {
    none: 'Texto como extraído',
    basic: 'Remove espaços e linhas vazias',
    standard: 'Remove cabeçalhos, rodapés e duplicatas; resume assinaturas',
    aggressive: 'Padrão + ruído de OCR e parágrafos contínuos'
  };

  // Opção sob o ponteiro em cada controle: a ajuda a descreve antes de gravar.
  let previaOcr = null;
  let previaLimpeza = null;

  // Códigos de tipo (EXTR, COMP...) em maiúsculas, sem espaços nem repetição.
  function normalizarTipos(texto) {
    const codigos = String(texto ?? '')
      .split(',')
      .map((c) => c.replace(/\s+/g, '').toUpperCase())
      .filter(Boolean);
    return [...new Set(codigos)];
  }

  // O campo guarda o texto digitado; só vira lista (e é salvo) ao confirmar.
  let omitirTexto = '';
  let omitirCarregado = false;
  $: if (config && !omitirCarregado) {
    omitirTexto = (config.omitir_tipos ?? []).join(', ');
    omitirCarregado = true;
  }

  async function handleSalvarOmitir() {
    config.omitir_tipos = normalizarTipos(omitirTexto);
    omitirTexto = config.omitir_tipos.join(', ');
    await handleSave('omitir');
  }

  // Sem a config salva, o formulário segue editável com os padrões.
  $: if (!config && erroConfig) {
    config = { ...PADRAO };
    avisar(erroConfig, 'error');
  }
  $: loading = !config;

  const ICONE_MONITORAMENTO = { monitorando: FolderSearch, pausado: CirclePause, sem_pasta: FolderX };
  $: estado = estadoMonitoramento(config);
  $: semPasta = estado === 'sem_pasta';

  // Escolher a primeira pasta liga o monitoramento, como no rodapé; trocar de
  // pasta mantém o estado do interruptor.
  async function handleSelectDirectory() {
    try {
      const selected = await SelectDirectory();
      if (selected) {
        if (!config.monitored_path) config.watchdog_enabled = true;
        config.monitored_path = selected;
        await handleSave('pasta');
      }
    } catch (err) {
      avisar('Erro ao selecionar pasta: ' + (err?.message ?? err), 'error');
    }
  }

  async function handleToggleWatchdog(e) {
    config.watchdog_enabled = e.currentTarget.checked;
    await handleSave('watchdog');
  }

  async function handleSave(campo) {
    try {
      await SaveConfig(config);
      config = config;
      confirmacao.mostrar(campo);
      anunciar(ROTULO_CAMPO[campo] + ': salvo');
    } catch (err) {
      // Falha parcial ("configurações salvas, mas...") já se explica sozinha.
      const msg = String(err?.message ?? err);
      const parcial = /^configurações salvas/i.test(msg);
      avisar(parcial ? msg[0].toUpperCase() + msg.slice(1) : 'Erro ao salvar configurações: ' + msg, 'error');
      await aoFalharSalvar();
      // O campo de tipos relê a config gravada, não o texto que não foi salvo.
      omitirCarregado = false;
    }
  }

  const ROTULO_CAMPO = {
    watchdog: 'Monitoramento',
    pasta: 'Pasta monitorada',
    ocr: 'OCR',
    limpeza: 'Higienização',
    omitir: 'Tipos omitidos',
    autostart: 'Iniciar com o Windows'
  };

  let confirmado = null;
  const confirmacao = criarConfirmacao((c) => { confirmado = c; });
  onDestroy(confirmacao.limpar);

  // Esvazia antes para que o mesmo texto seja anunciado de novo.
  let anuncio = '';
  async function anunciar(texto) {
    anuncio = '';
    await tick();
    anuncio = texto;
  }
</script>

<!-- O título "Configurações" fica na faixa do botão de recolher (App). -->
<div class="settings-card">
  {#if loading}
    <div class="loading-state">
      <div class="spinner"></div>
      <span>Carregando configurações...</span>
    </div>
  {:else}
    <div class="grupo">
      <h3 class="rotulo-grupo">Monitoramento</h3>

      <div class="form-group">
        <div class="toggle-group">
          <label for="watchdog-toggle">
            Monitorar a pasta
            <Salvo campo="watchdog" {confirmado} />
          </label>
          <label class="switch">
            <input
              id="watchdog-toggle"
              type="checkbox"
              role="switch"
              checked={estado === 'monitorando'}
              disabled={semPasta}
              aria-describedby={semPasta ? 'ajuda-monitoramento' : undefined}
              on:change={handleToggleWatchdog}
            />
            <span class="slider" aria-hidden="true"></span>
          </label>
        </div>
        <p class="help-text estado-monitor" class:pausado={estado === 'pausado'}>
          <Icone icone={ICONE_MONITORAMENTO[estado]} tamanho={12} />
          <span>{ROTULO_MONITORAMENTO[estado]}</span>
        </p>
        {#if semPasta}
          <p class="help-text" id="ajuda-monitoramento">Escolha uma pasta para monitorar</p>
        {/if}
      </div>

      <div class="form-group">
        <span class="rotulo-campo" id="rotulo-pasta">
          Pasta monitorada
          <Salvo campo="pasta" {confirmado} />
        </span>
        <div class="path-input-group">
          {#if semPasta}
            <span id="monitored-path" class="caminho vazio">Nenhuma pasta escolhida</span>
            <button type="button" class="btn-primary btn-icone-texto" on:click={handleSelectDirectory}>
              <Icone icone={FolderOpen} tamanho={12} />
              Escolher pasta...
            </button>
          {:else}
            <span id="monitored-path" class="caminho" title={config.monitored_path} use:reticenciasMeio={config.monitored_path}></span>
            <button
              type="button"
              class="btn-primary"
              aria-describedby="rotulo-pasta monitored-path"
              on:click={handleSelectDirectory}
            >
              Alterar
            </button>
          {/if}
        </div>
      </div>
    </div>

    <div class="grupo">
      <h3 class="rotulo-grupo">Processamento</h3>

      <div class="form-group">
        <Segmentado
          id="ocr-mode"
          legenda="Processamento de Imagens (OCR)"
          idAjuda="ajuda-ocr"
          opcoes={OPCOES_OCR}
          bind:valor={config.ocr_mode}
          bind:previa={previaOcr}
          aoMudar={() => handleSave('ocr')}
        >
          <svelte:fragment slot="legenda"><Salvo campo="ocr" {confirmado} /></svelte:fragment>
        </Segmentado>
        <p class="help-text" id="ajuda-ocr">{AJUDA_OCR[previaOcr ?? config.ocr_mode] ?? ''}</p>
      </div>

      <div class="form-group">
        <Segmentado
          id="cleaning-level"
          legenda="Higienização de texto"
          idAjuda="ajuda-limpeza"
          opcoes={OPCOES_LIMPEZA}
          bind:valor={config.cleaning_level}
          bind:previa={previaLimpeza}
          aoMudar={() => handleSave('limpeza')}
        >
          <svelte:fragment slot="legenda"><Salvo campo="limpeza" {confirmado} /></svelte:fragment>
        </Segmentado>
        <p class="help-text" id="ajuda-limpeza">{AJUDA_LIMPEZA[previaLimpeza ?? config.cleaning_level] ?? ''}</p>
      </div>

      <div class="form-group">
        <label for="omitir-tipos">
          Omitir documentos dos tipos
          <Salvo campo="omitir" {confirmado} />
        </label>
        <input
          id="omitir-tipos"
          type="text"
          aria-describedby="ajuda-omitir"
          placeholder="EXTR, COMP"
          autocomplete="off"
          spellcheck="false"
          bind:value={omitirTexto}
          on:change={handleSalvarOmitir}
        />
        <p class="help-text" id="ajuda-omitir">Ex.: EXTR, COMP. O documento vira um aviso no texto e não é lido.</p>
      </div>
    </div>

    <div class="grupo">
      <h3 class="rotulo-grupo">Sistema</h3>

      <div class="form-group toggle-group">
        <label for="autostart-toggle">
          Iniciar com o Windows
          <Salvo campo="autostart" {confirmado} />
        </label>
        <label class="switch">
          <input
            id="autostart-toggle"
            type="checkbox"
            role="switch"
            bind:checked={config.auto_start}
            on:change={() => handleSave('autostart')}
          />
          <span class="slider" aria-hidden="true"></span>
        </label>
      </div>
    </div>
  {/if}

  <!-- Sempre montada: o leitor de tela só anuncia mudanças numa região que
       já existia antes da mensagem. -->
  <div class="sr-only" role="status" aria-live="polite">{anuncio}</div>
</div>

<style>
  /* Seção do painel, sem moldura de cartão. flex: none: a 600px de altura
     quem rola é .sidebar-content (App), em vez de a seção encolher e cortar. */
  .settings-card {
    flex: none;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .grupo {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .rotulo-grupo {
    font-size: var(--fs-section);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin: 0;
    color: var(--text-secondary);
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .toggle-group {
    display: flex;
    flex-direction: row;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    min-height: 32px;
  }

  .rotulo-campo {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: var(--fs-base);
    font-weight: 500;
    color: var(--text-primary);
  }

  .estado-monitor {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .estado-monitor.pausado {
    color: var(--warning-strong);
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
    align-items: center;
    gap: 6px;
    min-height: 30px;
  }

  /* Texto, não campo: sem contorno nem foco; o caminho inteiro fica no title. */
  .caminho {
    flex: 1;
    min-width: 0;
    font-size: var(--fs-base);
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .caminho.vazio {
    color: var(--text-secondary);
    font-style: italic;
  }

  .btn-icone-texto {
    display: flex;
    align-items: center;
    gap: 4px;
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

  /* Com 1px o anel encosta no fundo do botão, da mesma cor (--accent-strong). */
  .btn-primary:focus-visible {
    outline-offset: 2px;
  }

  .btn-primary:active {
    transform: scale(0.98);
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

  /* Sem pasta: desligado e desabilitado (a ajuda abaixo diz o que fazer). */
  .switch input:disabled + .slider {
    cursor: default;
    opacity: 0.6;
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

  /* Alto contraste do Windows: o trilho vira Canvas; contorno e bolinha mostram o estado. */
  @media (forced-colors: active) {
    .slider {
      border: 1px solid ButtonText;
    }

    .slider:before {
      forced-color-adjust: none;
      background-color: ButtonText;
      left: 2px;
      bottom: 2px;
    }

    input:checked + .slider {
      forced-color-adjust: none;
      background-color: Highlight;
      border-color: Highlight;
    }

    input:checked + .slider:before {
      background-color: HighlightText;
    }

    .switch input:focus-visible + .slider {
      outline-color: CanvasText;
    }
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
