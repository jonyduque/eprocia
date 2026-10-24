// F-AN-02 (arraste do divisor 1:1 com o cursor), F-AN-03 (tokens de movimento
// e prefers-reduced-motion) e F-AN-04 (overlay de soltar com fade).
// Rodar com: node --test frontend/movimento.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarArrasto } from './src/lib/divisor.js';
import { criarMovimentoFila } from './src/lib/fila.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8').replace(/\r\n/g, '\n');
const app = ler('src', 'App.svelte');
const lista = ler('src', 'components', 'ProcessList.svelte');
const settings = ler('src', 'components', 'Settings.svelte');
const css = ler('src', 'assets', 'css', 'style.css');
const design = ler('..', 'DESIGN.md');

function estilo(texto) {
  const i = texto.indexOf('<style>');
  return texto.slice(i, texto.indexOf('</style>', i));
}

function tag(texto, marca) {
  const i = texto.indexOf(marca);
  assert.ok(i >= 0, `${marca} ausente`);
  const ini = texto.lastIndexOf('<', i);
  return texto.slice(ini, texto.indexOf('>', i) + 1);
}

function regra(texto, seletor) {
  const i = texto.indexOf(`${seletor} {`);
  assert.ok(i >= 0, `${seletor} ausente`);
  return texto.slice(i, texto.indexOf('}', i));
}

function corpo(nome) {
  const i = app.indexOf(`function ${nome}(`);
  assert.ok(i >= 0, `${nome} ausente`);
  return app.slice(i, app.indexOf('\n  }\n', i));
}

function quadros() {
  const pend = new Map();
  let n = 0;
  return {
    requestAnimationFrame(fn) { pend.set(++n, fn); return n; },
    cancelAnimationFrame(id) { pend.delete(id); },
    rodar() { const fns = [...pend.values()]; pend.clear(); fns.forEach((f) => f()); },
    get pendentes() { return pend.size; },
  };
}

// --- F-AN-02 ---------------------------------------------------------------

test('arrasto aplica só a última posição de cada quadro', () => {
  const q = quadros();
  const aplicadas = [];
  const arrasto = criarArrasto((x) => aplicadas.push(x), q);
  for (let x = 200; x < 600; x += 7) arrasto.mover(x);
  assert.equal(aplicadas.length, 0);
  assert.equal(q.pendentes, 1);
  q.rodar();
  assert.deepEqual(aplicadas, [599]);
  arrasto.mover(10);
  q.rodar();
  assert.deepEqual(aplicadas, [599, 10]);
});

test('soltar aplica a posição pendente na hora e cancela o quadro', () => {
  const q = quadros();
  const aplicadas = [];
  const arrasto = criarArrasto((x) => aplicadas.push(x), q);
  arrasto.mover(333);
  arrasto.soltar();
  assert.deepEqual(aplicadas, [333]);
  assert.equal(q.pendentes, 0);
  arrasto.soltar();
  assert.deepEqual(aplicadas, [333], 'soltar sem movimento pendente não reaplica');
});

test('divisor usa Pointer Events com captura e encerra em cancel, perda de captura e blur', () => {
  const d = tag(app, 'class="resize-handle"');
  assert.match(d, /on:pointerdown=\{startResizing\}/);
  assert.match(d, /on:pointermove=\{handlePointerMove\}/);
  assert.match(d, /on:pointerup=\{stopResizing\}/);
  assert.match(d, /on:pointercancel=\{stopResizing\}/);
  assert.match(d, /on:lostpointercapture=\{stopResizing\}/);
  assert.doesNotMatch(app, /mousemove|mouseup|on:mousedown/);
  assert.match(corpo('startResizing'), /setPointerCapture\(e\.pointerId\)/);
  assert.match(corpo('startResizing'), /addEventListener\('blur', stopResizing\)/);
  assert.match(corpo('stopResizing'), /removeEventListener\('blur', stopResizing\)/);
  assert.match(corpo('handlePointerMove'), /arrasto\.mover\(e\.clientX\)/);
  assert.match(app, /criarArrasto\(\(x\) => \{\s*sidebarWidth = limitarPainel\(x, larguraJanela\)/);
});

test('durante o arrasto a largura não anima e o cursor fica estável na janela toda', () => {
  assert.match(tag(app, 'class="sidebar"'), /class:resizing=\{isResizing\}/);
  assert.match(estilo(app), /\.sidebar\.resizing\s*\{[^}]*transition:\s*none/);
  assert.match(corpo('startResizing'), /classList\.add\('redimensionando'\)/);
  assert.match(corpo('stopResizing'), /classList\.remove\('redimensionando'\)/);
  assert.match(css, /:root\.redimensionando,\s*:root\.redimensionando \*\s*\{[^}]*cursor:\s*col-resize !important;[^}]*user-select:\s*none/);
});

test('alça captura 8px e desenha uma linha de 2px', () => {
  const s = estilo(app);
  const alca = regra(s, '.resize-handle');
  assert.match(alca, /width:\s*8px/);
  assert.match(alca, /right:\s*-4px/);
  assert.match(alca, /touch-action:\s*none/);
  assert.match(regra(s, '.resize-handle::before'), /width:\s*2px/);
});

// --- F-AN-03 ---------------------------------------------------------------

const folhas = { 'App.svelte': estilo(app), 'ProcessList.svelte': estilo(lista), 'Settings.svelte': estilo(settings), 'style.css': css };

test('transições e animações usam tokens, sem durações nem curvas soltas', () => {
  for (const [nome, folha] of Object.entries(folhas)) {
    const decls = folha.match(/(?:^|[;{\s])(?:transition|animation)\s*:[^;]+;/g) || [];
    for (const d of decls) {
      assert.doesNotMatch(d, /\b\d*\.?\d+m?s\b/, `${nome}: duração literal em ${d.trim()}`);
      assert.doesNotMatch(d, /cubic-bezier|(?<![-\w])ease(?:-in-out|-in|-out)?\b/, `${nome}: curva solta em ${d.trim()}`);
      if (/^\s*[;{]?\s*transition/.test(d) && !/none/.test(d)) {
        assert.match(d, /transition:\s*[a-z-]+\s+var\(/, `${nome}: transição sem propriedade em ${d.trim()}`);
      }
    }
  }
});

test('style.css define os tokens e os zera em prefers-reduced-motion', () => {
  assert.match(css, /--dur-fast:\s*150ms/);
  assert.match(css, /--dur-base:\s*220ms/);
  assert.match(css, /--ease-out:\s*cubic-bezier\(\.2, \.8, \.2, 1\)/);
  assert.match(css, /--dur-spin:\s*800ms/);
  const i = css.indexOf('@media (prefers-reduced-motion: reduce)');
  assert.ok(i >= 0);
  const bloco = css.slice(i, css.indexOf('\n}\n', i));
  assert.match(bloco, /--dur-fast:\s*0ms/);
  assert.match(bloco, /--dur-base:\s*0ms/);
  assert.match(bloco, /--dur-spin:\s*2400ms/, 'spinner mais lento, mas ainda indica atividade');
});

test('spinners giram pelo token', () => {
  for (const f of [lista, settings]) {
    assert.match(estilo(f), /animation:\s*spin var\(--dur-spin\) linear infinite/);
  }
});

test('pulso anima só transform/opacity num pseudo-elemento e para em reduced-motion', () => {
  const s = estilo(app);
  const i = s.indexOf('@keyframes pulse');
  const kf = s.slice(i, s.indexOf('\n  }\n', i));
  assert.doesNotMatch(kf, /box-shadow/);
  assert.match(kf, /transform/);
  assert.match(kf, /opacity/);
  assert.match(s, /\.pulse-dot::after\s*\{[^}]*animation:\s*pulse/);
  assert.doesNotMatch(regra(s, '.pulse-dot'), /animation/);
  const m = s.indexOf('@media (prefers-reduced-motion: reduce)');
  assert.ok(m >= 0, 'App sem regra de reduced-motion');
  const bloco = s.slice(m, s.indexOf('\n  }\n', s.indexOf('}', m)) + 4);
  assert.match(bloco, /\.pulse-dot::after[^{]*\{[^}]*animation:\s*none/);
});

// F-VI-05 tirou o deslocamento do hover; nada a trocar em reduced-motion.
test('hover da linha não desloca', () => {
  for (const m of estilo(lista).matchAll(/\.item-row:hover\s*\{([^}]*)\}/g)) {
    assert.doesNotMatch(m[1], /transform/);
  }
});

test('transições do Svelte passam por duracaoMovimento', () => {
  for (const [nome, f] of [['App', app], ['ProcessList', lista], ['Settings', settings]]) {
    const usos = f.match(/(?:transition|in|out):(?:fly|fade)(?:\|global)?=\{\{[^}]*\}\}/g) || [];
    for (const u of usos) assert.match(u, /duration:\s*(?:[^,?]*\?\s*)?duracaoMovimento\(DUR_(FAST|BASE)\)/, `${nome}: ${u}`);
  }
});

test('DESIGN.md tem a seção Movimento com tokens e reduced-motion', () => {
  const i = design.indexOf('## 7. Movimento');
  assert.ok(i >= 0, 'seção Movimento ausente');
  const sec = design.slice(i);
  for (const t of ['--dur-fast', '--dur-base', '--ease-out', '--dur-spin', 'prefers-reduced-motion', 'duracaoMovimento']) {
    assert.ok(sec.includes(t), `${t} ausente da seção Movimento`);
  }
});

// --- F-AN-04 ---------------------------------------------------------------

test('overlay de soltar entra e sai com fade e a caixa cresce de 0.98', () => {
  assert.match(app, /import \{ fade \} from 'svelte\/transition'/);
  assert.match(tag(app, 'class="drag-overlay"'), /transition:fade=\{\{ duration: duracaoMovimento\(DUR_FAST\) \}\}/);
  const s = estilo(app);
  assert.match(regra(s, '.overlay-box'), /animation:\s*zoomIn var\(--dur-fast\) var\(--ease-out\)/);
  assert.match(s, /@keyframes zoomIn\s*\{\s*from\s*\{\s*transform:\s*scale\(0\.98\)/);
});

// --- F-AN-05 ---------------------------------------------------------------

test('movimento da fila separa item novo, removido e mudança de filtro', () => {
  const mov = criarMovimentoFila();
  const a = { id: 'a', name: 'a.zip' };
  const b = { id: 'b', name: 'b.zip' };
  const c = { id: 'c', name: 'c.zip' };

  mov.atualizar([a, b], '');
  mov.atualizar([a, b, c], '');
  assert.equal(mov.novo('c'), true, 'item que chegou pelo queue:update entra animado');
  assert.equal(mov.novo('a'), false);
  assert.equal(mov.porFiltro(), false);

  mov.atualizar([a, b, c], 'b');
  assert.equal(mov.porFiltro(), true, 'digitar no filtro desliga o deslocamento');
  assert.equal(mov.novo('c'), false, 'item que reaparece pelo filtro não é novo');
  assert.equal(mov.removido('a'), false, 'item escondido pelo filtro não foi removido');

  mov.atualizar([a, b, c], '');
  assert.equal(mov.porFiltro(), true);
  assert.equal(mov.novo('a'), false, 'limpar o filtro não anima as linhas que voltam');

  mov.atualizar([a, c], '');
  assert.equal(mov.porFiltro(), false);
  assert.equal(mov.removido('b'), true, 'item removido da fila sai com fade');
  assert.equal(mov.novo('a'), false);

  mov.atualizar([{ ...a, progress: 0.5 }, c], '');
  assert.equal(mov.novo('a'), false, 'atualização de progresso não reanima a linha');
});

test('linha da fila: só entrada com fly, saída com fade e flip, controlados por criarMovimentoFila', () => {
  const linha = tag(lista, 'class="item-row {item.status}"');
  assert.doesNotMatch(linha, /transition:fly/);
  assert.match(linha, /in:fly\|global=\{\{ y: 8, duration: mov\.novo\(item\.id\) \? duracaoMovimento\(DUR_BASE\) : 0, easing: cubicOut \}\}/);
  assert.match(linha, /out:fade=\{\{ duration: mov\.removido\(item\.id\) \? duracaoMovimento\(DUR_FAST\) : 0 \}\}/);
  assert.match(linha, /animate:flip=\{\{ duration: mov\.porFiltro\(\) \? 0 : duracaoMovimento\(DUR_FAST\), easing: cubicOut \}\}/);
  assert.match(lista, /import \{ flip \} from 'svelte\/animate'/);
  assert.match(lista, /const mov = criarMovimentoFila\(\)/);
  assert.match(lista, /\$: mov\.atualizar\(items, filterText\)/);
});

// --- F-AN-06 ---------------------------------------------------------------

test('conteúdo do painel fica montado, inerte e some com fade ao recolher', () => {
  const conteudo = tag(app, 'class="sidebar-content"');
  assert.match(conteudo, /inert=\{sidebarCollapsed\}/);
  assert.doesNotMatch(conteudo, /hidden=/, 'hidden troca o conteúdo no mesmo quadro');
  const s = estilo(app);
  assert.doesNotMatch(s, /\.sidebar-content\[hidden\]/);
  assert.match(regra(s, '.sidebar.collapsed'), /overflow:\s*hidden/, 'conteúdo de 280px não vaza na coluna de 48px');
  const aberto = regra(s, '.sidebar-content');
  assert.match(aberto, /transition:\s*opacity var\(--dur-fast\) var\(--ease-out\) var\(--dur-fast\)/, 'ao expandir, aparece depois da largura');
  const fechado = regra(s, '.sidebar.collapsed .sidebar-content');
  assert.match(fechado, /position:\s*absolute/);
  assert.match(fechado, /width:\s*var\(--largura-painel\)/, 'não se espreme enquanto a coluna encolhe');
  assert.match(fechado, /opacity:\s*0/);
  assert.match(fechado, /visibility:\s*hidden/);
  assert.match(fechado, /transition:\s*opacity var\(--dur-fast\) var\(--ease-out\), visibility var\(--dur-fast\) linear/);
  assert.match(tag(app, 'class="sidebar"'), /style:--largura-painel="\{larguraPainel\}px"/);
});

test('título do painel some com fade e as ferramentas recolhidas entram depois da largura', () => {
  const ini = app.indexOf('<div class="sidebar-header-toggle">');
  const cab = app.slice(ini, app.indexOf('<button', ini));
  assert.doesNotMatch(cab, /\{#if/, 'título não pode ser desmontado de repente');
  const s = estilo(app);
  const titulo = regra(s, '.sidebar.collapsed .sidebar-title');
  assert.match(titulo, /opacity:\s*0/);
  assert.match(titulo, /visibility:\s*hidden/);
  assert.match(regra(s, '.sidebar-title'), /transition:\s*opacity var\(--dur-fast\) var\(--ease-out\) var\(--dur-fast\)/);
  assert.match(tag(app, 'class="collapsed-tools"'), /in:fade=\{\{ duration: duracaoMovimento\(DUR_FAST\), delay: duracaoMovimento\(DUR_FAST\) \}\}/);
});

test('DESIGN.md descreve o movimento da fila e do painel recolhível', () => {
  const sec = design.slice(design.indexOf('## 7. Movimento'));
  assert.match(sec, /animate:flip/);
  assert.match(sec, /filtro/);
  assert.match(sec, /Recolher\/expandir o painel/);
});
