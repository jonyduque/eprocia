// F-VI-06 (revisão 2026-11-10): barra de rolagem visível e sombra que indica
// conteúdo rolado na fila e no painel.
// Rodar com: node --test frontend/rolagem.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { estadoRolagem, rolagem } from './src/lib/rolagem.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8').replace(/\r\n/g, '\n');
const app = ler('src', 'App.svelte');
const lista = ler('src', 'components', 'ProcessList.svelte');
const css = ler('src', 'assets', 'css', 'style.css');
const design = ler('..', 'DESIGN.md');

function estilo(texto) {
  const i = texto.indexOf('<style>');
  return texto.slice(i, texto.indexOf('</style>', i));
}

function regra(texto, seletor) {
  const esc = seletor.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const m = texto.match(new RegExp(`(?:^|[\\s}])${esc}\\s*\\{([^}]*)\\}`));
  assert.ok(m, `${seletor} ausente`);
  return m[1];
}

test('polegar da barra de rolagem em --border-strong, 8px com 2px transparentes', () => {
  assert.match(regra(css, '::-webkit-scrollbar'), /width:\s*8px/);
  const polegar = regra(css, '::-webkit-scrollbar-thumb');
  assert.match(polegar, /background-color:\s*var\(--border-strong\)/);
  assert.match(polegar, /border:\s*2px solid transparent/);
  assert.match(polegar, /background-clip:\s*padding-box/);
  assert.match(regra(css, '::-webkit-scrollbar-thumb:hover'), /background-color:\s*var\(--text-secondary\)/);
});

test('estadoRolagem diz se há conteúdo acima e abaixo da área visível', () => {
  assert.deepEqual(estadoRolagem({ scrollTop: 0, scrollHeight: 100, clientHeight: 100 }), { acima: false, abaixo: false });
  assert.deepEqual(estadoRolagem({ scrollTop: 0, scrollHeight: 300, clientHeight: 100 }), { acima: false, abaixo: true });
  assert.deepEqual(estadoRolagem({ scrollTop: 50, scrollHeight: 300, clientHeight: 100 }), { acima: true, abaixo: true });
  // Zoom fracionário deixa meio pixel sobrando no fim.
  assert.deepEqual(estadoRolagem({ scrollTop: 199.5, scrollHeight: 300, clientHeight: 100 }), { acima: true, abaixo: false });
});

test('ação rolagem avisa só quando o estado muda e limpa tudo no destroy', () => {
  const observados = [];
  const desligados = [];
  globalThis.ResizeObserver = class {
    constructor(fn) { this.fn = fn; observados.push(this); }
    observe() {}
    disconnect() { desligados.push('resize'); }
  };
  globalThis.MutationObserver = class {
    constructor(fn) { this.fn = fn; }
    observe() {}
    disconnect() { desligados.push('mutation'); }
  };
  const ouvintes = {};
  const node = {
    scrollTop: 0, scrollHeight: 300, clientHeight: 100, children: [],
    addEventListener(t, f, o) { ouvintes[t] = f; assert.equal(o?.passive, true); },
    removeEventListener(t) { delete ouvintes[t]; },
  };
  const vistos = [];
  const acao = rolagem(node, (e) => vistos.push(e));
  assert.deepEqual(vistos, [{ acima: false, abaixo: true }]);
  ouvintes.scroll();
  assert.equal(vistos.length, 1, 'sem mudança, sem aviso');
  node.scrollTop = 40;
  ouvintes.scroll();
  assert.deepEqual(vistos.at(-1), { acima: true, abaixo: true });
  node.scrollHeight = 140;
  node.scrollTop = 40;
  observados[0].fn();
  assert.deepEqual(vistos.at(-1), { acima: true, abaixo: false });
  desligados.length = 0;
  acao.destroy();
  assert.equal(ouvintes.scroll, undefined);
  assert.deepEqual(desligados.sort(), ['mutation', 'resize']);
  delete globalThis.ResizeObserver;
  delete globalThis.MutationObserver;
});

test('cabeçalho da fila ganha --shadow-sm só com a lista rolada', () => {
  assert.match(lista, /class="items-container"[^>]*use:rolagem=/);
  assert.match(lista, /class="list-header"[^>]*class:rolado=/);
  const s = estilo(lista);
  assert.doesNotMatch(regra(s, '.list-header'), /box-shadow:\s*var/);
  assert.match(regra(s, '.list-header.rolado'), /box-shadow:\s*var\(--shadow-sm\)/);
});

test('painel ganha --shadow-sm sobre o pé só com conteúdo abaixo', () => {
  assert.match(app, /class="sidebar-content"[^>]*use:rolagem=/);
  assert.match(app, /class="sidebar-content"[^>]*class:mais-abaixo=/);
  const s = estilo(app);
  assert.doesNotMatch(regra(s, '.sidebar-content'), /box-shadow:\s*var/);
  assert.match(regra(s, '.sidebar-content.mais-abaixo'), /box-shadow:\s*var\(--shadow-sm\)/);
});

test('DESIGN.md registra a sombra de rolagem e a barra de rolagem', () => {
  const i = design.indexOf('### 2.4');
  const sec = design.slice(i, design.indexOf('\n## ', i));
  assert.match(sec, /--shadow-sm[^\n]*rolad/);
  assert.match(sec, /barra de rolagem/i);
});
