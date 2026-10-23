// F-VI-07 (formulário no painel estreito), F-VI-08 (tema do sistema sem
// flash) e F-VI-09 (cores só por tokens).
// Rodar com: node --test frontend/tema.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { TEMAS, proximoTema, temaEfetivo, lerPreferencia } from './src/lib/tema.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8');
const app = ler('src', 'App.svelte');
const settings = ler('src', 'components', 'Settings.svelte');
const lista = ler('src', 'components', 'ProcessList.svelte');
const css = ler('src', 'assets', 'css', 'style.css');
const html = ler('index.html');

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

// --- F-VI-08 ---------------------------------------------------------------

test('tema tem três estados, padrão sistema', () => {
  assert.deepEqual(TEMAS, ['system', 'light', 'dark']);
  assert.equal(proximoTema('system'), 'light');
  assert.equal(proximoTema('light'), 'dark');
  assert.equal(proximoTema('dark'), 'system');
  assert.equal(lerPreferencia({ getItem: () => null }), 'system');
  assert.equal(lerPreferencia({ getItem: () => 'qualquer' }), 'system');
  assert.equal(lerPreferencia({ getItem: () => 'light' }), 'light');
  assert.equal(temaEfetivo('system', true), 'dark');
  assert.equal(temaEfetivo('system', false), 'light');
  assert.equal(temaEfetivo('light', true), 'light');
  assert.equal(temaEfetivo('dark', false), 'dark');
});

function rodarScriptInicial(salvo, sistemaEscuro) {
  const blocos = [...html.matchAll(/<script(?![^>]*\bsrc=)(?![^>]*type="module")[^>]*>([\s\S]*?)<\/script>/g)];
  assert.equal(blocos.length, 1, 'index.html precisa de um script inline clássico');
  const posInline = html.indexOf(blocos[0][0]);
  assert.ok(posInline < html.indexOf('type="module"'), 'o script do tema roda antes do bundle');
  const classes = new Set();
  const documento = {
    documentElement: {
      classList: {
        add: (c) => classes.add(c),
        remove: (c) => classes.delete(c),
        toggle: (c, on) => (on ? classes.add(c) : classes.delete(c)),
      },
      style: {},
    },
  };
  const ambiente = {
    localStorage: { getItem: () => salvo },
    matchMedia: (q) => ({ matches: /dark/.test(q) ? sistemaEscuro : !sistemaEscuro }),
    document: documento,
  };
  new Function('window', 'localStorage', 'matchMedia', 'document', blocos[0][1])(
    ambiente, ambiente.localStorage, ambiente.matchMedia, documento);
  return classes.has('light');
}

test('index.html aplica o tema antes da montagem', () => {
  assert.equal(rodarScriptInicial(null, false), true, 'sistema claro sem preferência');
  assert.equal(rodarScriptInicial(null, true), false, 'sistema escuro sem preferência');
  assert.equal(rodarScriptInicial('light', true), true);
  assert.equal(rodarScriptInicial('dark', false), false);
  assert.equal(rodarScriptInicial('system', false), true);
});

test('App segue prefers-color-scheme e escuta mudanças', () => {
  assert.match(ler('src', 'lib', 'tema.js'), /CONSULTA_ESCURO = '\(prefers-color-scheme: dark\)'/);
  assert.match(app, /matchMedia\(CONSULTA_ESCURO\)/);
  assert.match(app, /addEventListener\('change'/);
  assert.match(app, /removeEventListener\('change'/);
  assert.match(app, /proximoTema/);
});

test('janela nativa não usa mais o fundo claro fixo', () => {
  const main = readFileSync(join(raiz, '..', 'main.go'), 'utf8');
  assert.doesNotMatch(main, /R:\s*248,\s*G:\s*249,\s*B:\s*250/);
});

// --- F-VI-09 ---------------------------------------------------------------

test('componentes não têm cor solta', () => {
  for (const [nome, texto] of [['App', app], ['Settings', settings], ['ProcessList', lista]]) {
    const soltas = texto.match(/#[0-9a-fA-F]{3,8}\b|\brgba?\(|\bhsla?\(|:\s*(white|black)\b/g);
    assert.equal(soltas, null, `${nome}: ${soltas}`);
  }
});

test('overlay e fundos tonais vêm de tokens por tema', () => {
  const escuro = regra(css, ':root');
  const claro = regra(css, ':root.light');
  for (const token of ['--overlay', '--glass', '--glass-border', '--on-accent']) {
    assert.match(escuro, new RegExp(`${token}:`), `${token} no tema escuro`);
    assert.match(claro, new RegExp(`${token}:`), `${token} no tema claro`);
  }
  assert.match(css, /--accent-soft:\s*color-mix\(in srgb, var\(--accent\) 12%, transparent\)/);
  assert.match(regra(estilo(app), '.drag-overlay'), /var\(--overlay\)/);
});

// --- F-VI-07 ---------------------------------------------------------------

test('interruptores com rótulo à esquerda e linha de 32px', () => {
  const grupo = regra(estilo(settings), '.toggle-group');
  assert.doesNotMatch(grupo, /justify-content:\s*center/);
  assert.match(grupo, /justify-content:\s*space-between/);
  assert.match(grupo, /min-height:\s*32px/);
  for (const id of ['watchdog-toggle', 'autostart-toggle']) {
    const rotulo = settings.indexOf(`for="${id}"`);
    const chave = settings.indexOf(`id="${id}"`);
    assert.ok(rotulo >= 0 && rotulo < chave, `rótulo de ${id} antes do switch`);
  }
});

test('opções curtas com texto de ajuda abaixo do select', () => {
  const opcoes = [...settings.matchAll(/<option value="[^"]+">([^<]+)<\/option>/g)].map((m) => m[1]);
  assert.ok(opcoes.length >= 7);
  for (const o of opcoes) assert.ok(o.length <= 12, `opção longa: ${o}`);
  for (const o of ['Nenhuma', 'Básica', 'Agressiva']) assert.ok(opcoes.includes(o), o);
  assert.ok((settings.match(/class="help-text"/g) || []).length >= 2);
  assert.match(regra(estilo(settings), '.help-text'), /font-size:\s*11px/);
});

test('pasta monitorada com title e reticências', () => {
  const i = settings.indexOf('id="monitored-path"');
  const input = settings.slice(settings.lastIndexOf('<input', i), settings.indexOf('/>', i));
  assert.match(input, /title=\{/);
  assert.match(regra(estilo(settings), 'input[type="text"]'), /text-overflow:\s*ellipsis/);
});
