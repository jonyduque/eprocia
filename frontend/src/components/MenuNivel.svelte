<script>
  import Icone from './Icone.svelte';
  import Check from 'lucide-svelte/icons/check';
  import { fade, fly } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import { DUR_FAST, criarConfirmacao, duracaoMovimento } from '../lib/aviso.js';
  import { nivelOpcao, rotuloOpcao } from '../lib/atividade.js';
  import { moverFoco } from '../lib/menu.js';

  // Botão da barra recolhida que abre, à direita, o menu com as opções e a
  // descrição de cada uma (OPCOES_OCR, OPCOES_LIMPEZA).
  export let id = '';
  export let legenda = '';
  export let icone = null;
  export let opcoes = [];
  export let valor = '';
  export let desabilitado = false;
  // Grava a opção escolhida; devolve true quando salvou.
  export let aoEscolher = async () => true;

  let aberto = false;
  let botao;
  // { campo: rótulo, n } por 1,5 s ao lado do botão depois de salvar.
  let escolhido = null;
  const confirmacao = criarConfirmacao((c) => (escolhido = c));

  $: rotulo = rotuloOpcao(opcoes, valor);
  $: ancora = `--menu-${id}`;
  $: if (desabilitado) aberto = false;

  function abrir() {
    if (desabilitado) return;
    confirmacao.limpar();
    escolhido = null;
    aberto = true;
  }

  function fechar(devolverFoco) {
    if (!aberto) return;
    aberto = false;
    if (devolverFoco) botao?.focus();
  }

  async function escolher(v) {
    fechar(true);
    if (v === valor) return;
    if (await aoEscolher(v)) confirmacao.mostrar(rotuloOpcao(opcoes, v));
  }

  function teclarBotao(e) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      abrir();
    }
  }

  // Os dois ficam na camada superior: a coluna recolhida tem overflow: hidden.
  function camadaSuperior(el) {
    el.showPopover();
    return () => {
      if (el.matches(':popover-open')) el.hidePopover();
    };
  }

  // Abre focando a opção atual; fecha com clique fora, Esc (devolvendo o foco
  // ao botão) ou Tab (o foco segue adiante).
  function comportamentoMenu(el) {
    const sair = camadaSuperior(el);
    const itens = () => [...el.querySelectorAll('[role="menuitemradio"]')];
    (itens().find((b) => b.getAttribute('aria-checked') === 'true') ?? itens()[0])?.focus();

    const aoApontar = (e) => {
      if (!el.contains(e.target) && !botao?.contains(e.target)) fechar(false);
    };
    const aoTeclar = (e) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        fechar(true);
        return;
      }
      if (e.key === 'Tab') {
        fechar(false);
        return;
      }
      const lista = itens();
      const prox = moverFoco(Math.max(0, lista.indexOf(document.activeElement)), e.key, lista.length);
      if (prox !== null) {
        e.preventDefault();
        lista[prox].focus();
      }
    };

    document.addEventListener('pointerdown', aoApontar, true);
    el.addEventListener('keydown', aoTeclar);
    return () => {
      document.removeEventListener('pointerdown', aoApontar, true);
      el.removeEventListener('keydown', aoTeclar);
      sair();
    };
  }
</script>

<button
  bind:this={botao}
  type="button"
  class="nivel-btn"
  class:aberto
  style:anchor-name={ancora}
  on:click={() => (aberto ? fechar(true) : abrir())}
  on:keydown={teclarBotao}
  disabled={desabilitado}
  title="{legenda}: {rotulo}"
  aria-label="{legenda}: {rotulo}"
  aria-haspopup="menu"
  aria-expanded={aberto}
  aria-controls={aberto ? `menu-${id}` : undefined}
>
  <Icone {icone} tamanho={14} />
  <span class="nivel" aria-hidden="true">
    {#each opcoes.slice(1) as _, i}
      <span class="nivel-ponto" class:cheio={i < nivelOpcao(opcoes, valor)}></span>
    {/each}
  </span>
</button>

{#if aberto}
  <div
    id="menu-{id}"
    class="menu"
    popover="manual"
    role="menu"
    aria-label={legenda}
    style:position-anchor={ancora}
    {@attach comportamentoMenu}
    in:fly={{ x: -4, duration: duracaoMovimento(DUR_FAST), easing: cubicOut }}
  >
    <div class="titulo" aria-hidden="true"><Icone {icone} tamanho={12} />{legenda}</div>
    {#each opcoes as o (o.valor)}
      <button
        type="button"
        class="opcao"
        class:selecionada={o.valor === valor}
        role="menuitemradio"
        tabindex="-1"
        aria-checked={o.valor === valor}
        aria-label={o.rotulo}
        aria-describedby="desc-{id}-{o.valor}"
        on:click={() => escolher(o.valor)}
      >
        <span class="marca">{#if o.valor === valor}<Icone icone={Check} tamanho={12} />{/if}</span>
        <span class="texto">
          <span class="rotulo">{o.rotulo}</span>
          <span class="descricao" id="desc-{id}-{o.valor}">{o.descricao}</span>
        </span>
      </button>
    {/each}
  </div>
{/if}

{#if escolhido}
  <span
    class="escolhido"
    popover="manual"
    aria-hidden="true"
    style:position-anchor={ancora}
    {@attach camadaSuperior}
    in:fade={{ duration: duracaoMovimento(DUR_FAST) }}
    out:fade={{ duration: duracaoMovimento(DUR_FAST) }}
  >{escolhido.campo}</span>
{/if}

<style>
  /* Mesmo visual de .collapsed-tool-btn (App.svelte), com os pontos de nível
     embaixo do ícone. */
  .nivel-btn {
    background-color: var(--surface-hover);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    width: 32px;
    height: 36px;
    padding: 0;
    border-radius: 4px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 3px;
    cursor: pointer;
  }

  .nivel-btn:hover,
  .nivel-btn.aberto {
    color: var(--accent);
    border-color: var(--accent);
  }

  .nivel-btn:disabled {
    cursor: default;
    opacity: 0.6;
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

  /* Popover ancorado à direita do botão; o UA centraliza com inset/margin. */
  .menu,
  .escolhido {
    position: fixed;
    inset: auto;
    margin: 0 0 0 8px;
    left: anchor(right);
    background-color: var(--surface);
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    box-shadow: var(--shadow-sm);
    color: var(--text-primary);
  }

  .menu {
    top: anchor(top);
    position-try-fallbacks: flip-block;
    width: 232px;
    padding: 4px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .titulo {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 8px 2px;
    font-size: var(--fs-meta);
    font-weight: 500;
    color: var(--text-secondary);
  }

  .opcao {
    display: flex;
    align-items: flex-start;
    gap: 6px;
    width: 100%;
    padding: 6px 8px;
    border: none;
    border-radius: 4px;
    background-color: transparent;
    color: var(--text-primary);
    text-align: left;
    cursor: pointer;
  }

  .opcao:hover {
    background-color: var(--surface-hover);
  }

  .opcao:focus-visible {
    outline: 2px solid var(--focus-ring);
    outline-offset: -2px;
  }

  .marca {
    display: inline-flex;
    flex: none;
    width: 12px;
    padding-top: 2px;
    color: var(--accent-text);
  }

  .texto {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .rotulo {
    font-size: var(--fs-base);
    font-weight: 500;
  }

  .selecionada .rotulo {
    color: var(--accent-text);
    font-weight: 600;
  }

  .descricao {
    font-size: var(--fs-meta);
    color: var(--text-secondary);
    line-height: 1.35;
  }

  .escolhido {
    top: anchor(center);
    transform: translateY(-50%);
    padding: 2px 6px;
    font-size: var(--fs-meta);
    white-space: nowrap;
    pointer-events: none;
  }

  /* Alto contraste do Windows: o fundo vira Canvas e os pontos sumiriam.
     Vazio = desligado/vazio; Highlight = ligado/cheio. */
  @media (forced-colors: active) {
    .nivel-ponto {
      forced-color-adjust: none;
      border: 1px solid CanvasText;
      background-color: Canvas;
    }

    .nivel-ponto.cheio {
      background-color: Highlight;
      border-color: Highlight;
    }

    .selecionada .rotulo {
      color: Highlight;
    }
  }
</style>
