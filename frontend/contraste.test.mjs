// F-AC-01 (contraste de texto), F-AC-02 (contorno de controles),
// F-AC-03 (foco dos interruptores) e F-AC-04 (foco dos campos).
// Rodar com: node --test frontend/contraste.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8');
const app = ler('src', 'App.svelte');
const settings = ler('src', 'components', 'Settings.svelte');
const lista = ler('src', 'components', 'ProcessList.svelte');
const css = ler('src', 'assets', 'css', 'style.css');

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

function tokens(bloco) {
  const t = {};
  for (const m of bloco.matchAll(/(--[\w-]+):\s*([^;]+);/g)) t[m[1]] = m[2].trim();
  return t;
}

const escuro = tokens(regra(css, ':root'));
const claro = { ...escuro, ...tokens(regra(css, ':root.light')) };
const TEMAS = [['escuro', escuro], ['claro', claro]];

function hex(t, nome) {
  let v = t[nome];
  for (let i = 0; v && i < 5; i++) {
    const ref = v.match(/^var\((--[\w-]+)\)$/);
    if (!ref) break;
    v = t[ref[1]];
  }
  assert.match(v ?? '', /^#[0-9a-fA-F]{6}$/, `${nome} precisa resolver para hex, veio ${v}`);
  return v;
}

function luminancia(h) {
  const n = parseInt(h.slice(1), 16);
  const canais = [16, 8, 0].map((s) => {
    const c = ((n >> s) & 255) / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * canais[0] + 0.7152 * canais[1] + 0.0722 * canais[2];
}

function contraste(a, b) {
  const [x, y] = [luminancia(a), luminancia(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
}

function exige(t, tema, frente, fundo, minimo) {
  const r = contraste(hex(t, frente), hex(t, fundo));
  assert.ok(r >= minimo, `${tema}: ${frente} sobre ${fundo} = ${r.toFixed(2)}:1 (mínimo ${minimo})`);
}

// --- F-AC-01 ---------------------------------------------------------------

test('pares de texto colorido passam de 4,5:1 nos dois temas', () => {
  for (const [tema, t] of TEMAS) {
    exige(t, tema, '--on-accent', '--accent-strong', 4.5);
    exige(t, tema, '--on-accent', '--accent-strong-hover', 4.5);
    for (const fundo of ['--surface', '--bg-main']) {
      exige(t, tema, '--success-strong', fundo, 4.5);
      exige(t, tema, '--error-strong', fundo, 4.5);
    }
  }
});

test('botão primário usa o azul forte', () => {
  const s = estilo(settings);
  assert.match(regra(s, '.btn-primary'), /background-color:\s*var\(--accent-strong\)/);
  assert.match(regra(s, '.btn-primary:hover'), /background-color:\s*var\(--accent-strong-hover\)/);
});

test('toasts com texto colorido sobre superfície neutra', () => {
  const s = estilo(settings);
  assert.match(regra(s, '.toast-status'), /background-color:\s*var\(--surface\)/);
  const ok = regra(s, '.toast-status.success');
  const erro = regra(s, '.toast-status.error');
  assert.match(ok, /color:\s*var\(--success-strong\)/);
  assert.match(erro, /color:\s*var\(--error-strong\)/);
  for (const r of [ok, erro]) {
    assert.doesNotMatch(r, /--on-accent/);
    assert.doesNotMatch(r, /background-color:\s*var\(--(success|error)\)/);
  }
});

test('botão Retomar usa o verde forte no texto', () => {
  assert.match(regra(estilo(lista), '.btn-pause.resume'), /(^|[^-])color:\s*var\(--success-strong\)/);
});

// --- F-AC-02 ---------------------------------------------------------------

test('--border-strong passa de 3:1 contra superfície e fundo', () => {
  for (const [tema, t] of TEMAS) {
    exige(t, tema, '--border-strong', '--surface', 3);
    exige(t, tema, '--border-strong', '--bg-main', 3);
    // A bolinha branca do interruptor desligado fica sobre o trilho.
    exige(t, tema, '--on-accent', '--border-strong', 3);
  }
});

test('trilho do interruptor e bordas de campos usam --border-strong', () => {
  const s = estilo(settings);
  assert.match(regra(s, '.slider'), /background-color:\s*var\(--border-strong\)/);
  assert.match(regra(s, 'input[type="text"]'), /border:\s*1px solid var\(--border-strong\)/);
  assert.match(regra(s, 'select'), /border:\s*1px solid var\(--border-strong\)/);
  assert.match(regra(estilo(lista), '.search-input'), /border:\s*1px solid var\(--border-strong\)/);
});

// --- F-AC-03 ---------------------------------------------------------------

test('interruptor mostra anel de foco no trilho', () => {
  const r = regra(estilo(settings), '.switch input:focus-visible + .slider');
  assert.match(r, /outline:\s*2px solid var\(--focus-ring\)/);
  assert.match(r, /outline-offset:\s*2px/);
});

// --- F-AC-04 ---------------------------------------------------------------

test('regra global de foco visível com --focus-ring', () => {
  const m = css.match(/:where\(([^)]*)\):focus-visible\s*\{([^}]*)\}/);
  assert.ok(m, 'regra global :where(...):focus-visible ausente');
  for (const el of ['button', 'input', 'select', '[tabindex]']) assert.ok(m[1].includes(el), el);
  assert.match(m[2], /outline:\s*2px solid var\(--focus-ring\)/);
  assert.match(m[2], /outline-offset:\s*1px/);
  for (const [tema, t] of TEMAS) {
    exige(t, tema, '--focus-ring', '--surface', 3);
    exige(t, tema, '--focus-ring', '--bg-main', 3);
  }
});

test('nenhum componente remove o outline', () => {
  for (const [nome, texto] of [['App', app], ['Settings', settings], ['ProcessList', lista]]) {
    assert.doesNotMatch(estilo(texto), /outline:\s*(none|0)\b/, nome);
  }
});
