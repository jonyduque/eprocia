// F-VI-04 (paleta igual ao DESIGN.md) e F-VI-05 (superfícies opacas, sem
// blur, gradiente, sombra grande nem hover com deslocamento).
// Rodar com: node --test frontend/paleta.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8');
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

function tokens(bloco) {
  const t = {};
  for (const m of bloco.matchAll(/(--[\w-]+):\s*([^;]+);/g)) t[m[1]] = m[2].trim();
  return t;
}

const escuro = tokens(regra(css, ':root'));
const claro = { ...escuro, ...tokens(regra(css, ':root.light')) };
const TEMAS = [['escuro', escuro], ['claro', claro]];

function resolve(t, nome) {
  let v = t[nome];
  for (let i = 0; v && i < 5; i++) {
    const ref = v.match(/^var\((--[\w-]+)\)$/);
    if (!ref) break;
    v = t[ref[1]];
  }
  return (v ?? '').toUpperCase();
}

function secao(titulo) {
  const i = design.indexOf(titulo);
  assert.ok(i >= 0, `DESIGN.md sem "${titulo}"`);
  const fim = design.indexOf('\n#', i + titulo.length);
  return design.slice(i, fim < 0 ? undefined : fim);
}

// Primeiro hex depois do rótulo, na seção dada.
function cor(sec, rotulo) {
  const i = sec.indexOf(rotulo);
  assert.ok(i >= 0, `DESIGN.md sem "${rotulo}"`);
  const m = sec.slice(i).match(/#[0-9a-fA-F]{6}/);
  assert.ok(m, `sem hex para "${rotulo}"`);
  return m[0].toUpperCase();
}

function luminancia(h) {
  const n = parseInt(h.slice(1), 16);
  const c = [16, 8, 0].map((s) => {
    const v = ((n >> s) & 255) / 255;
    return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
}

function contraste(a, b) {
  const [x, y] = [luminancia(a), luminancia(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
}

// --- F-VI-04 ---------------------------------------------------------------

test('neutros de cada tema saem do DESIGN.md', () => {
  const pares = [
    ['Fundo Principal', '--bg-main'],
    ['Superfícies', '--surface'],
    ['Texto Primário', '--text-primary'],
    ['Texto Secundário', '--text-secondary'],
    ['Bordas', '--border'],
  ];
  for (const [titulo, t] of [['### 2.1 Tema Claro', claro], ['### 2.2 Tema Escuro', escuro]]) {
    const sec = secao(titulo);
    for (const [rotulo, token] of pares) {
      assert.equal(resolve(t, token), cor(sec, rotulo), `${titulo}: ${token}`);
    }
  }
});

test('cores de destaque saem do DESIGN.md nos dois temas', () => {
  const sec = secao('### 2.3 Cores de Destaque');
  const pares = [
    ['Primária', '--accent-strong'],
    ['Sucesso', '--success'],
    ['Erro', '--error'],
    ['Processamento', '--processing'],
  ];
  for (const [tema, t] of TEMAS) {
    for (const [rotulo, token] of pares) {
      assert.equal(resolve(t, token), cor(sec, rotulo), `${tema}: ${token}`);
    }
  }
});

test('borda visível e diferente do hover nos dois temas', () => {
  for (const [tema, t] of TEMAS) {
    const borda = resolve(t, '--border');
    assert.notEqual(borda, resolve(t, '--surface-hover'), `${tema}: --border igual a --surface-hover`);
    assert.notEqual(borda, resolve(t, '--surface'), `${tema}: --border igual a --surface`);
    assert.notEqual(resolve(t, '--surface-hover'), resolve(t, '--surface'), `${tema}: hover sem diferença`);
  }
});

test('texto secundário e ícones de destaque legíveis nos dois temas', () => {
  for (const [tema, t] of TEMAS) {
    for (const fundo of ['--surface', '--bg-main']) {
      const f = resolve(t, fundo);
      const sec = contraste(resolve(t, '--text-secondary'), f);
      assert.ok(sec >= 4.5, `${tema}: --text-secondary sobre ${fundo} = ${sec.toFixed(2)}:1`);
    }
    // Ícones de estado ficam nas linhas e barras, todas em --surface.
    for (const icone of ['--accent', '--processing', '--success', '--error', '--warning']) {
      const r = contraste(resolve(t, icone), resolve(t, '--surface'));
      assert.ok(r >= 3, `${tema}: ${icone} sobre --surface = ${r.toFixed(2)}:1`);
    }
  }
});

test('DESIGN.md documenta os tokens fora da paleta base', () => {
  const i = design.indexOf('## 2. Cores e Temas');
  const sec = design.slice(i, design.indexOf('\n## 3.', i));
  for (const t of ['--surface-hover', '--warning', '--focus-ring', '--processing', '--accent', '--shadow-sm']) {
    assert.ok(sec.includes(t), `${t} sem documentação na seção 2`);
  }
});

// --- F-VI-05 ---------------------------------------------------------------

test('sem blur, gradiente nem tokens de vidro', () => {
  for (const [nome, texto] of [['App', estilo(app)], ['ProcessList', estilo(lista)], ['style.css', css]]) {
    assert.doesNotMatch(texto, /backdrop-filter/, `${nome}: backdrop-filter`);
    assert.doesNotMatch(texto, /(linear|radial|conic)-gradient\(/, `${nome}: gradiente`);
    assert.doesNotMatch(texto, /--(glass|card-|bg-glass|accent-glow|success-glow|shadow-md|shadow-lg)/, `${nome}: token de vidro/sombra grande`);
  }
});

test('raios de no máximo 6px (círculos à parte)', () => {
  for (const [nome, texto] of [['App', estilo(app)], ['ProcessList', estilo(lista)]]) {
    for (const m of texto.matchAll(/border-radius:\s*([^;]+);/g)) {
      const v = m[1].trim();
      if (v === '50%') continue;
      const px = v.match(/^(\d+)px$/);
      assert.ok(px && Number(px[1]) <= 6, `${nome}: border-radius ${v}`);
    }
  }
});

test('sombras leves: só --shadow-sm, sem brilho colorido', () => {
  for (const [nome, texto] of [['App', estilo(app)], ['ProcessList', estilo(lista)]]) {
    for (const m of texto.matchAll(/box-shadow:\s*([^;]+);/g)) {
      assert.match(m[1].trim(), /^(none|var\(--shadow-(sm|overlay)\))$/, `${nome}: box-shadow ${m[1]}`);
    }
  }
  for (const [tema, t] of TEMAS) {
    for (const token of ['--shadow-sm', '--shadow-overlay']) {
      const m = (t[token] ?? '').match(/^0 (\d+)px (\d+)px/);
      assert.ok(m, `${tema}: ${token} = ${t[token]}`);
      assert.ok(Number(m[1]) <= 4 && Number(m[2]) <= 12, `${tema}: ${token} grande demais (${t[token]})`);
    }
  }
});

test('contêiner da lista opaco, borda 1px, raio até 6px e sombra leve', () => {
  const r = regra(estilo(lista), '.list-card');
  assert.match(r, /background-color:\s*var\(--surface\)/);
  assert.match(r, /border:\s*1px solid var\(--border\)/);
  assert.match(r, /border-radius:\s*[4-6]px/);
  assert.match(r, /box-shadow:\s*var\(--shadow-sm\)/);
});

test('linha opaca, raio 4px, sem sombra e hover só troca fundo e borda', () => {
  const s = estilo(lista);
  const linha = regra(s, '.item-row');
  assert.match(linha, /background-color:\s*var\(--surface\)/);
  assert.match(linha, /border:\s*1px solid var\(--border\)/);
  assert.match(linha, /border-radius:\s*4px/);
  assert.doesNotMatch(linha, /box-shadow/);
  assert.doesNotMatch(linha, /transform/);
  const hover = regra(s, '.item-row:hover');
  assert.match(hover, /background-color:\s*var\(--surface-hover\)/);
  assert.doesNotMatch(hover, /transform|box-shadow/);
  for (const p of ['background-color', 'border-color']) {
    assert.match(linha, new RegExp(`${p} var\\(--dur-fast\\) var\\(--ease-out\\)`));
  }
});

test('barra de progresso sólida em --processing', () => {
  const r = regra(estilo(lista), '.progress-bar');
  assert.match(r, /background-color:\s*var\(--processing\)/);
  assert.doesNotMatch(r, /gradient|box-shadow/);
});

test('cartão manual e barras do app opacos', () => {
  const s = estilo(app);
  assert.match(regra(s, '.manual-card'), /background-color:\s*var\(--surface\)/);
  assert.match(regra(s, '.manual-card'), /border:\s*1px solid var\(--border\)/);
  for (const sel of ['.custom-titlebar', '.sidebar', 'footer']) {
    assert.match(regra(s, sel), /background-color:\s*var\(--surface\)/, sel);
  }
  assert.doesNotMatch(regra(s, '.app-container'), /background-image/);
});
