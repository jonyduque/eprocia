<script>
  import Icone from './Icone.svelte';
  import ChevronDown from 'lucide-svelte/icons/chevron-down';
  import EyeOff from 'lucide-svelte/icons/eye-off';
  import FolderOpen from 'lucide-svelte/icons/folder-open';
  import FolderSearch from 'lucide-svelte/icons/folder-search';
  import LoaderCircle from 'lucide-svelte/icons/loader-circle';
  import Power from 'lucide-svelte/icons/power';
  import ScanEye from 'lucide-svelte/icons/scan-eye';
  import ScanText from 'lucide-svelte/icons/scan-text';
  import Sparkles from 'lucide-svelte/icons/sparkles';
  import Atualizacoes from './Atualizacoes.svelte';
  import Salvo from './Salvo.svelte';
  import Segmentado from './Segmentado.svelte';
  import { onDestroy, tick } from 'svelte';
  import { criarAnunciador } from '../lib/regiaoViva.js';
  import { criarConfirmacao } from '../lib/aviso.js';
  import { OPCOES_OCR, OPCOES_LIMPEZA, descricaoOpcao } from '../lib/atividade.js';
  import { estadoMonitoramento, reticenciasMeio } from '../lib/monitoramento.js';
  import { itensLog } from '../lib/log.js';
  import { ABERTURA_INICIAL, aberturaAtualizacoes, gravarCategorias, lerCategorias } from '../lib/categorias.js';
  import { armazenamento } from '../lib/tema.js';
  import {
    AJUDA_FORCAR_OCR_CONFIG,
    AJUDA_FORCAR_OCR_SEM_OCR,
    ERRO_ABRIR_LOGS,
    ROTULO_FORCAR_OCR_CONFIG
  } from '../lib/textos.js';
  import { AbrirPastaLogs, SaveConfig, SelectDirectory } from '../wailsjs/go/main/App.js';

  // O App lê a config uma vez e a compartilha por bind; null enquanto carrega.
  export let config = null;
  export let erroConfig = '';
  // Chamado quando SaveConfig falha: o App relê a config gravada, para o
  // rodapé e o formulário não exibirem valores que não foram salvos.
  export let aoFalharSalvar = () => {};
  // Erros vão para o aviso do pé do painel (App), fora da área rolável, para
  // não cobrir os controles; o sucesso é o Check ao lado do controle.
  export let avisar = () => {};
  // App.LogAtivo(), lido uma vez pelo App: sem log, nada sobre ele aparece aqui.
  export let logAtivo = false;
  // Estado da atualização (App): sem atualizador ativo o bloco não aparece.
  export let estadoAtualizacao = null;
  export let filaOcupada = false;

  const PADRAO = {
    monitored_path: '',
    ocr_mode: 'normal',
    cleaning_level: 'standard',
    forcar_ocr: false,
    auto_start: false,
    watchdog_enabled: true,
    omitir_tipos: []
  };

  // Categorias recolhidas (true) por chave, lembradas entre sessões. A entrada
  // animada só vale depois de um clique (ou da abertura automática de
  // Atualizações fora do primeiro estado), não na abertura do painel.
  // preferidas é o que o usuário escolheu e se grava; recolhidas é o que está
  // na tela, que a abertura automática de Atualizações muda só em memória.
  let preferidas = lerCategorias(armazenamento());
  let recolhidas = { ...preferidas };
  let animar = {};

  function alternar(chave) {
    const valor = !recolhidas[chave];
    recolhidas = { ...recolhidas, [chave]: valor };
    preferidas = { ...preferidas, [chave]: valor };
    animar = { ...animar, [chave]: true };
    gravarCategorias(armazenamento(), preferidas);
  }

  // Versão nova disponível abre Atualizações uma vez por versão (lib/categorias.js).
  let aberturaAtual = ABERTURA_INICIAL;
  $: verAberturaAtualizacoes(estadoAtualizacao);

  function verAberturaAtualizacoes(estado) {
    const r = aberturaAtualizacoes(estado, aberturaAtual);
    aberturaAtual = r.memoria;
    if (!r.abrir || !recolhidas.atualizacoes) return;
    recolhidas = { ...recolhidas, atualizacoes: false };
    animar = { ...animar, atualizacoes: r.animar };
  }

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

  // O estado (Monitorando/Pausado) aparece só no rodapé (App), não aqui.
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
    autostart: 'Iniciar com o Windows',
    forcar: ROTULO_FORCAR_OCR_CONFIG
  };

  async function abrirPastaLogs() {
    try {
      await AbrirPastaLogs();
    } catch (err) {
      avisar(ERRO_ABRIR_LOGS + (err?.message ?? err), 'error');
    }
  }

  let confirmado = null;
  const confirmacao = criarConfirmacao((c) => { confirmado = c; });
  onDestroy(confirmacao.limpar);

  // Esvazia antes para que o mesmo texto seja anunciado de novo.
  let anuncio = '';
  const anunciar = criarAnunciador((t) => (anuncio = t), tick);
</script>

<!-- O título "Configurações" fica na faixa do botão de recolher (App). -->
<div class="settings-card">
  {#if loading}
    <div class="loading-state">
      <span class="spinner" aria-hidden="true"><Icone icone={LoaderCircle} tamanho={20} cor="var(--accent)" /></span>
      <span>Carregando configurações...</span>
    </div>
  {:else}
    <div class="grupo">
      <h3 class="rotulo-grupo">
        <button
          type="button"
          class="cabecalho-grupo"
          aria-expanded={!recolhidas.monitoramento}
          aria-controls="grupo-monitoramento"
          on:click={() => alternar('monitoramento')}
        >
          <span class="chevron-grupo" aria-hidden="true"><Icone icone={ChevronDown} tamanho={14} /></span>
          Monitoramento
        </button>
      </h3>

      <div class="conteudo-grupo" class:animar={animar.monitoramento} id="grupo-monitoramento" hidden={recolhidas.monitoramento}>
        <div class="form-group">
          <div class="toggle-group">
            <label for="watchdog-toggle">
              <span class="icone-rotulo"><Icone icone={FolderSearch} tamanho={14} /></span>
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
          {#if semPasta}
            <p class="help-text" id="ajuda-monitoramento">Escolha uma pasta para monitorar</p>
          {/if}
        </div>

        <!-- Sem rótulo visível: o nome do grupo e do botão vem do aria-label. -->
        <div class="path-input-group" role="group" aria-label="Pasta monitorada">
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
              aria-label="Alterar pasta monitorada"
              aria-describedby="monitored-path"
              on:click={handleSelectDirectory}
            >
              Alterar
            </button>
          {/if}
          <Salvo campo="pasta" {confirmado} />
        </div>
      </div>
    </div>

    <div class="grupo">
      <h3 class="rotulo-grupo">
        <button
          type="button"
          class="cabecalho-grupo"
          aria-expanded={!recolhidas.processamento}
          aria-controls="grupo-processamento"
          on:click={() => alternar('processamento')}
        >
          <span class="chevron-grupo" aria-hidden="true"><Icone icone={ChevronDown} tamanho={14} /></span>
          Processamento
        </button>
      </h3>

      <div class="conteudo-grupo" class:animar={animar.processamento} id="grupo-processamento" hidden={recolhidas.processamento}>
        <div class="form-group">
          <Segmentado
            id="ocr-mode"
            icone={ScanText}
            legenda="Processamento de Imagens (OCR)"
            idAjuda="ajuda-ocr"
            opcoes={OPCOES_OCR}
            bind:valor={config.ocr_mode}
            bind:previa={previaOcr}
            aoMudar={() => handleSave('ocr')}
          >
            <svelte:fragment slot="legenda"><Salvo campo="ocr" {confirmado} /></svelte:fragment>
          </Segmentado>
          <p class="help-text ajuda-pilha" id="ajuda-ocr"><span>{descricaoOpcao(OPCOES_OCR, previaOcr ?? config.ocr_mode)}</span>{#each OPCOES_OCR as o (o.valor)}<span class="reserva" aria-hidden="true">{o.descricao}</span>{/each}</p>
        </div>

        <!-- Com o OCR desligado o interruptor fica desabilitado e o valor salvo não
             muda (voltando a um modo com OCR, reaparece como estava); a ajuda diz por quê. -->
        <div class="form-group">
          <div class="toggle-group">
            <label for="forcar-ocr">
              <span class="icone-rotulo"><Icone icone={ScanEye} tamanho={14} /></span>
              <span class="rotulo-texto">{ROTULO_FORCAR_OCR_CONFIG}</span>
              <Salvo campo="forcar" {confirmado} />
            </label>
            <label class="switch">
              <input
                id="forcar-ocr"
                type="checkbox"
                role="switch"
                bind:checked={config.forcar_ocr}
                disabled={config.ocr_mode === 'none'}
                aria-describedby="ajuda-forcar-ocr"
                on:change={() => handleSave('forcar')}
              />
              <span class="slider" aria-hidden="true"></span>
            </label>
          </div>
          <p class="help-text" id="ajuda-forcar-ocr">{config.ocr_mode === 'none' ? AJUDA_FORCAR_OCR_SEM_OCR : AJUDA_FORCAR_OCR_CONFIG}</p>
        </div>

        <div class="form-group">
          <Segmentado
            id="cleaning-level"
            icone={Sparkles}
            legenda="Higienização de texto"
            idAjuda="ajuda-limpeza"
            opcoes={OPCOES_LIMPEZA}
            bind:valor={config.cleaning_level}
            bind:previa={previaLimpeza}
            aoMudar={() => handleSave('limpeza')}
          >
            <svelte:fragment slot="legenda"><Salvo campo="limpeza" {confirmado} /></svelte:fragment>
          </Segmentado>
          <p class="help-text ajuda-pilha" id="ajuda-limpeza"><span>{descricaoOpcao(OPCOES_LIMPEZA, previaLimpeza ?? config.cleaning_level)}</span>{#each OPCOES_LIMPEZA as o (o.valor)}<span class="reserva" aria-hidden="true">{o.descricao}</span>{/each}</p>
        </div>

        <div class="form-group">
          <label for="omitir-tipos" class="rotulo-campo">
            <span class="icone-rotulo"><Icone icone={EyeOff} tamanho={14} /></span>
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
    </div>

    <div class="grupo">
      <h3 class="rotulo-grupo">
        <button
          type="button"
          class="cabecalho-grupo"
          aria-expanded={!recolhidas.sistema}
          aria-controls="grupo-sistema"
          on:click={() => alternar('sistema')}
        >
          <span class="chevron-grupo" aria-hidden="true"><Icone icone={ChevronDown} tamanho={14} /></span>
          Sistema
        </button>
      </h3>

      <div class="conteudo-grupo" class:animar={animar.sistema} id="grupo-sistema" hidden={recolhidas.sistema}>
        <div class="form-group toggle-group">
          <label for="autostart-toggle">
            <span class="icone-rotulo"><Icone icone={Power} tamanho={14} /></span>
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

        {#if logAtivo}
          <div class="form-group">
            {#each itensLog(logAtivo) as item}
              {#if item.tipo === 'botao'}
                <button type="button" class="btn-primary btn-icone-texto btn-log" on:click={abrirPastaLogs}>
                  <Icone icone={FolderOpen} tamanho={12} />
                  {item.texto}
                </button>
              {:else}
                <p class="help-text">{item.texto}</p>
              {/if}
            {/each}
          </div>
        {/if}
      </div>
    </div>

    {#if estadoAtualizacao?.ativo}
      <div class="grupo" data-bloco="atualizacoes">
        <h3 class="rotulo-grupo">
          <button
            type="button"
            class="cabecalho-grupo"
            aria-expanded={!recolhidas.atualizacoes}
            aria-controls="grupo-atualizacoes"
            on:click={() => alternar('atualizacoes')}
          >
            <span class="chevron-grupo" aria-hidden="true"><Icone icone={ChevronDown} tamanho={14} /></span>
            Atualizações
          </button>
        </h3>

        <div class="conteudo-grupo" class:animar={animar.atualizacoes} id="grupo-atualizacoes" hidden={recolhidas.atualizacoes}>
          <Atualizacoes estado={estadoAtualizacao} {filaOcupada} {avisar} />
        </div>
      </div>
    {/if}
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

  /* Cabeçalho da categoria: botão com a aparência do rótulo da seção, na
     largura toda, que recolhe e expande o conteúdo. */
  .cabecalho-grupo {
    display: flex;
    align-items: center;
    gap: 4px;
    width: 100%;
    padding: 2px 0;
    margin: -2px 0;
    border: none;
    border-radius: 4px;
    background: none;
    font: inherit;
    text-transform: inherit;
    letter-spacing: inherit;
    color: inherit;
    text-align: left;
    cursor: pointer;
  }

  .cabecalho-grupo:hover {
    color: var(--text-primary);
  }

  .cabecalho-grupo:focus-visible {
    outline: 2px solid var(--focus-ring);
    outline-offset: 2px;
  }

  .chevron-grupo {
    display: inline-flex;
    flex-shrink: 0;
    transition: transform var(--dur-fast) var(--ease-out);
  }

  .cabecalho-grupo[aria-expanded="false"] .chevron-grupo {
    transform: rotate(-90deg);
  }

  .conteudo-grupo {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  /* display: flex acima venceria o atributo hidden. */
  .conteudo-grupo[hidden] {
    display: none;
  }

  /* Só depois de um clique (class:animar), não na abertura do painel; com
     prefers-reduced-motion (ou os efeitos de animação do Windows desligados)
     --dur-fast é 0ms e o conteúdo aparece direto. Recolher não anima. */
  .conteudo-grupo.animar {
    animation: entrar-grupo var(--dur-fast) var(--ease-out);
  }

  @keyframes entrar-grupo {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
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
    gap: 6px;
    font-size: var(--fs-base);
    font-weight: 500;
    color: var(--text-primary);
  }

  /* Ícone da configuração (DESIGN.md §4), o mesmo no painel e na linha. */
  .icone-rotulo {
    display: inline-flex;
    flex-shrink: 0;
    color: var(--text-secondary);
  }

  .toggle-group label:not(.switch) {
    display: flex;
    align-items: center;
    gap: 6px;
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

  /* Ajuda de um segmentado: a descrição ativa e, por baixo, todas as outras
     na mesma célula, invisíveis. A célula fica com a altura da mais longa, e o
     hover (que troca a descrição) não muda a altura do painel: com a rolagem
     no fim, a mudança movia o controle para fora do ponteiro e o hover
     alternava sem parar. */
  .ajuda-pilha {
    display: grid;
  }

  .ajuda-pilha > span {
    grid-area: 1 / 1;
  }

  .reserva {
    visibility: hidden;
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

  /* No .form-group (coluna flex): não estica e, a 280px, quebra em vez de cortar. */
  .btn-log {
    align-self: flex-start;
    width: fit-content;
    max-width: 100%;
    white-space: normal;
    text-align: left;
  }

  /* Rótulo longo quebra a linha em vez de empurrar o switch. */
  .rotulo-texto {
    min-width: 0;
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
    display: inline-flex;
    animation: spin var(--dur-spin) linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
