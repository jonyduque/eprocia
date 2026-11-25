// F-AN-02 (hover e pressionado iguais em todo botão), F-VI-05 (botões da
// janela no canto, como no Windows 11), F-AN-07 (recolher o painel sem salto
// do botão nem troca seca do chevron) e F-AN-12 (troca de tema numa transição só).
// Rodar com: node --test frontend/botoes.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { trocarTema } from './src/lib/tema.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8').replace(/\r\n/g, '\n');
const app = ler('src', 'App.svelte');
const css = ler('src', 'assets', 'css', 'style.css');
const design = ler('..', 'DESIGN.md');
const componentes = Object.fromEntries(
  readdirSync(join(raiz, 'src', 'components'))
    .filter((f) => f.endsWith('.svelte'))
    .map((f) => [f, ler('src', 'components', f)]),
);
componentes['App.svelte'] = app;

function estilo(texto) {
  const i = texto.indexOf('<style>');
  return i < 0 ? '' : texto.slice(i, texto.indexOf('</style>', i));
}

function regra(texto, seletor) {
  const esc = seletor.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const m = texto.match(new RegExp(`(?:^|[\\s}])${esc}\\s*\\{([^}]*)\\}`));
  assert.ok(m, `${seletor} ausente`);
  return m[1];
}

function semRegra(texto, seletor) {
  const esc = seletor.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return !new RegExp(`(?:^|[\\s},])${esc}\\s*[,{]`).test(texto);
}

function funcao(texto, nome) {
  const i = texto.indexOf(`function ${nome}(`);
  assert.ok(i >= 0, `${nome} ausente`);
  return texto.slice(i, texto.indexOf('\n  }\n', i));
}

function tag(texto, marca) {
  const i = texto.indexOf(marca);
  assert.ok(i >= 0, `${marca} ausente`);
  const ini = texto.lastIndexOf('<', i);
  return texto.slice(ini, texto.indexOf('>', i) + 1);
}

// --- F-AN-02 ---------------------------------------------------------------

test('todo botão transiciona fundo, borda e cor pelo mesmo token', () => {
  const r = regra(css, ':where(button)');
  assert.match(r, /transition:\s*background-color var\(--dur-fast\) var\(--ease-out\), border-color var\(--dur-fast\) var\(--ease-out\), color var\(--dur-fast\) var\(--ease-out\)/);
});

test('componentes não repetem a transição de hover dos botões', () => {
  const classes = new Set();
  for (const fonte of Object.values(componentes)) {
    for (const m of fonte.matchAll(/<button\b[^>]*?\sclass="([^"]+)"/g)) {
      for (const c of m[1].split(/\s+/)) classes.add(c);
    }
  }
  assert.ok(classes.has('btn-pause') && classes.has('titlebar-btn'));
  for (const [nome, fonte] of Object.entries(componentes)) {
    const s = estilo(fonte);
    for (const [, sel, decl] of s.matchAll(/([^{}]+)\{([^}]*)\}/g)) {
      const t = decl.match(/transition:\s*([^;]+);/);
      if (!t) continue;
      const props = t[1].split(',').map((p) => p.trim().split(/\s+/)[0]);
      const soHover = props.every((p) => ['background-color', 'border-color', 'color'].includes(p));
      const ehBotao = sel.split(',').some((x) => {
        const s1 = x.trim();
        return s1 === 'button' || [...s1.matchAll(/\.([\w-]+)/g)].some(([, c]) => classes.has(c));
      });
      assert.ok(!(soHover && ehBotao), `${nome}: ${sel.trim()} repete a transição global dos botões`);
    }
  }
});

test('pressionado é o mesmo fundo --border em todo botão neutro, sem scale', () => {
  const m = css.match(/([^{}]*:active[^{}]*)\{([^}]*)\}/);
  assert.ok(m, 'regra :active global ausente');
  assert.match(m[1], /button/);
  assert.match(m[1], /:disabled/, 'desabilitado não reage');
  assert.match(m[1], /\.info-btn/, 'a linha expansível fica de fora');
  assert.match(m[2], /var\(--border\)/);
  for (const [nome, fonte] of Object.entries(componentes)) {
    assert.doesNotMatch(estilo(fonte), /:active\s*\{[^}]*scale/, `${nome}: pressionado com scale`);
  }
});

test('DESIGN.md §7 traz a tabela de estados dos botões', () => {
  const s7 = design.slice(design.indexOf('## 7. Movimento'));
  assert.match(s7, /\| *Estado *\|/);
  assert.match(s7, /Pressionado/);
  assert.match(s7, /:where\(button\)/);
});

// --- F-VI-05 ---------------------------------------------------------------

test('botões da janela 44 px de largura na altura da barra, sem raio nem intervalo', () => {
  const s = estilo(app);
  assert.match(regra(s, '.custom-titlebar'), /padding:\s*0 0 0 12px/, 'o Fechar encosta na borda direita');
  const acoes = regra(s, '.titlebar-actions');
  assert.match(acoes, /gap:\s*0/);
  assert.match(acoes, /align-self:\s*stretch/);
  const janela = regra(s, '.btn-janela');
  assert.match(janela, /width:\s*44px/);
  assert.match(janela, /height:\s*100%/);
  assert.doesNotMatch(janela, /border-radius/, 'sem raio');
  assert.doesNotMatch(regra(s, '.titlebar-btn'), /border-radius/, 'sem raio herdado');
  for (const marca of ['on:click={WindowMinimise}', 'on:click={alternarMaximizar}', 'on:click={handleCloseWindow}']) {
    assert.match(tag(app, marca), /btn-janela/, marca);
  }
  assert.match(regra(s, '.titlebar-btn.btn-close:hover'), /background-color:\s*var\(--error\)[^}]*color:\s*var\(--on-accent\)|color:\s*var\(--on-accent\)[^}]*background-color:\s*var\(--error\)/);
  assert.match(regra(s, '.titlebar-btn:hover'), /background-color:\s*var\(--surface-hover\)/);
});

test('botão de tema segue 28 px e fica separado dos da janela por um divisor', () => {
  const s = estilo(app);
  assert.match(tag(app, 'on:click={toggleTheme}'), /btn-tema/);
  const tema = regra(s, '.btn-tema');
  assert.match(tema, /width:\s*28px/);
  assert.match(tema, /height:\s*28px/);
  assert.match(tema, /border-radius:\s*4px/);
  const ini = app.indexOf('on:click={toggleTheme}');
  const div = app.indexOf('class="titlebar-divisor"', ini);
  assert.ok(div > ini && div < app.indexOf('on:click={WindowMinimise}'), 'divisor entre tema e minimizar');
  assert.match(tag(app, 'class="titlebar-divisor"'), /aria-hidden="true"/);
  const d = regra(s, '.titlebar-divisor');
  assert.match(d, /width:\s*1px/);
  assert.match(d, /background-color:\s*var\(--border\)/);
});

// --- F-AN-07 ---------------------------------------------------------------

test('recolher não muda o alinhamento do cabeçalho no mesmo quadro da largura', () => {
  const s = estilo(app);
  assert.ok(semRegra(s, '.sidebar.collapsed .sidebar-header-toggle'), 'regra que salta o botão ainda existe');
  assert.match(regra(s, '.sidebar-header-toggle'), /justify-content:\s*flex-end/);
  assert.match(regra(s, '.painel-titulo'), /margin-right:\s*auto/);
});

test('um chevron só, que gira ao recolher', () => {
  assert.doesNotMatch(app, /ChevronRight/);
  const ini = app.indexOf('class="btn-toggle-sidebar"');
  const botao = app.slice(ini, app.indexOf('</button>', ini));
  assert.doesNotMatch(botao, /\{#if/);
  assert.match(botao, /class="chevron-painel"/);
  assert.match(botao, /<Icone icone=\{ChevronLeft\}/);
  const s = estilo(app);
  assert.match(regra(s, '.chevron-painel'), /transition:\s*transform var\(--dur-fast\) var\(--ease-out\)/);
  assert.match(regra(s, '.sidebar.collapsed .chevron-painel'), /transform:\s*rotate\(180deg\)/);
});

// --- F-AN-12 ---------------------------------------------------------------

function documentoFalso() {
  const chamadas = [];
  const classes = new Set();
  let terminar;
  const finished = new Promise((r) => { terminar = r; });
  return {
    chamadas,
    classes,
    terminar: () => terminar(),
    documentElement: { classList: { add: (c) => classes.add(c), remove: (c) => classes.delete(c) } },
    startViewTransition(fn) {
      chamadas.push(fn);
      const feito = Promise.resolve().then(fn);
      return { finished, updateCallbackDone: feito, ready: feito };
    },
  };
}

test('trocarTema usa a View Transitions API quando há movimento', async () => {
  const doc = documentoFalso();
  let aplicado = 0;
  await trocarTema(doc, () => {
    aplicado++;
    assert.ok(doc.classes.has('trocando-tema'), 'transições próprias desligadas durante a troca');
  }, 220);
  assert.equal(doc.chamadas.length, 1);
  assert.equal(aplicado, 1);
  assert.ok(doc.classes.has('trocando-tema'), 'segue desligado até o fim do instantâneo');
  doc.terminar();
  await new Promise((r) => setTimeout(r, 0));
  assert.ok(!doc.classes.has('trocando-tema'));
});

test(':root.trocando-tema desliga as transições próprias', () => {
  assert.match(regra(css, ':root.trocando-tema *'), /transition:\s*none !important/);
});

test('trocarTema aplica direto sem movimento ou sem a API', async () => {
  const doc = documentoFalso();
  let aplicado = 0;
  await trocarTema(doc, () => { aplicado++; }, 0);
  assert.equal(doc.chamadas.length, 0, 'prefers-reduced-motion não anima');
  assert.equal(aplicado, 1);
  await trocarTema({}, () => { aplicado++; }, 220);
  assert.equal(aplicado, 2, 'WebView sem startViewTransition só aplica');
});

test('trocarTema não deixa a troca presa se a transição falhar', async () => {
  let aplicado = 0;
  const doc = documentoFalso();
  doc.startViewTransition = () => { throw new Error('indisponível'); };
  const erro = console.error;
  console.error = () => {};
  try {
    await trocarTema(doc, () => { aplicado++; }, 220);
  } finally {
    console.error = erro;
  }
  assert.equal(aplicado, 1);
  assert.ok(!doc.classes.has('trocando-tema'));
});

test('botão de tema e mudança do sistema passam pela mesma transição', () => {
  for (const nome of ['toggleTheme', 'mudouTemaDoSistema']) {
    const f = funcao(app, nome);
    assert.match(f, /trocarTema\(document, async \(\) => \{/, nome);
    assert.match(f, /await tick\(\)/, nome);
    assert.match(f, /duracaoMovimento\(DUR_BASE\)/, nome);
  }
  assert.match(funcao(app, 'mudouTemaDoSistema'), /sistemaEscuro = e\.matches/);
});

test('o instantâneo da troca dura --dur-base e não bloqueia o ponteiro', () => {
  const m = css.match(/::view-transition-old\(root\),\s*::view-transition-new\(root\)\s*\{([^}]*)\}/);
  assert.ok(m, '::view-transition-old/new(root) ausente');
  assert.match(m[1], /animation-duration:\s*var\(--dur-base\)/);
  assert.match(m[1], /animation-timing-function:\s*var\(--ease-out\)/);
  assert.match(regra(css, '::view-transition'), /pointer-events:\s*none/);
});

test('superfícies não animam fundo nem borda na troca de tema', () => {
  assert.doesNotMatch(regra(css, 'body'), /transition/);
  const superficies = {
    'App.svelte': ['.custom-titlebar', '.sidebar', 'footer'],
    'ProcessList.svelte': ['.list-card'],
  };
  for (const [nome, sels] of Object.entries(superficies)) {
    const s = estilo(componentes[nome]);
    for (const sel of sels) {
      assert.doesNotMatch(regra(s, sel), /var\(--dur-base\)/, `${nome}: ${sel}`);
    }
  }
  assert.match(regra(estilo(app), '.sidebar'), /transition:\s*width var\(--dur-fast\) var\(--ease-out\);/);
});

test('DESIGN.md §7 descreve a troca de tema pela View Transitions API', () => {
  const s7 = design.slice(design.indexOf('## 7. Movimento'));
  assert.match(s7, /startViewTransition/);
  assert.match(s7, /::view-transition/);
});
