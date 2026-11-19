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
const avisoComp = ler('src', 'components', 'Aviso.svelte');
const menuNivel = ler('src', 'components', 'MenuNivel.svelte');
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

test('avisos com texto colorido sobre superfície neutra', () => {
  const s = estilo(avisoComp);
  assert.match(regra(s, '.aviso'), /background-color:\s*var\(--surface\)/);
  const ok = regra(s, '.aviso.sucesso');
  const erro = regra(s, '.aviso.erro');
  // Atenção: texto neutro, o âmbar fica no ícone e na borda.
  assert.match(regra(s, '.aviso.atencao'), /color:\s*var\(--text-primary\)/);
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
  assert.match(regra(estilo(ler('src', 'components', 'Segmentado.svelte')), '.faixa'), /border:\s*1px solid var\(--border-strong\)/);
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

// --- Revisão 2026-11-10: F-AC-02 (texto colorido de 11px por seletor) -------

const atividadeComp = ler('src', 'components', 'AtividadeItem.svelte');
const design = ler('..', 'DESIGN.md');

// Fundo tonal (color-mix com transparent) composto sobre um fundo opaco.
function sobre(t, nome, base) {
  const m = (t[nome] ?? '').match(/^color-mix\(in srgb, var\((--[\w-]+)\) (\d+)%, transparent\)$/);
  if (!m) return hex(t, nome);
  const p = Number(m[2]) / 100;
  const [a, b] = [hex(t, m[1]), hex(t, base)].map((h) => parseInt(h.slice(1), 16));
  const canal = (s) => Math.round(((a >> s) & 255) * p + ((b >> s) & 255) * (1 - p));
  return '#' + [16, 8, 0].map((s) => canal(s).toString(16).padStart(2, '0')).join('');
}

function corTexto(r, seletor) {
  const m = r.match(/(?:^|[^-])color:\s*var\((--[\w-]+)\)/);
  assert.ok(m, `${seletor} sem color: var(...)`);
  return m[1];
}

// [componente, seletor, fundos opacos sob o elemento]
const TEXTOS = [
  [lista, '.meta-badge.success', ['--surface', '--surface-hover']],
  [lista, '.queue-chip', ['--surface']],
  [lista, '.paused-message', ['--surface', '--surface-hover']],
  [lista, '.error-message', ['--surface', '--surface-hover']],
  [lista, '.progress-message', ['--surface', '--surface-hover']],
  [lista, '.nao-lidos', ['--surface', '--surface-hover']],
  [atividadeComp, '.chip.ok', ['--bg-main']],
  [atividadeComp, '.chip.ocr', ['--bg-main']],
  [atividadeComp, '.chip.erro', ['--bg-main']],
  [atividadeComp, '.chip.aviso', ['--bg-main']],
  [atividadeComp, '.btn-ocr', ['--bg-main']],
  [atividadeComp, '.doc-ocr', ['--bg-main', '--accent-faint']],
  [atividadeComp, '.doc-erro', ['--bg-main']],
  [atividadeComp, '.doc-erro.aviso', ['--bg-main']],
  [app, '.pulse-indicator', ['--surface']],
];

test('texto colorido de 11px passa de 4,5:1 nos dois temas, com hover e fundo tonal', () => {
  for (const [comp, seletor, fundos] of TEXTOS) {
    // Regra cujo seletor começa a linha (".details-row > .nao-lidos" não conta).
    const esc = seletor.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const m = estilo(comp).match(new RegExp(`\\n\\s*${esc}\\s*\\{([^}]*)\\}`));
    assert.ok(m, `${seletor} ausente`);
    const r = m[1];
    const frente = corTexto(r, seletor);
    assert.match(frente, /-(strong|text)$/, `${seletor}: texto em ${frente}; use a variante -strong/-text`);
    const tonal = r.match(/background-color:\s*var\((--[\w-]+)\)/)?.[1];
    for (const [tema, t] of TEMAS) {
      for (const f of fundos) {
        const base = f === '--accent-faint' ? sobre(t, f, '--bg-main') : hex(t, f);
        const tt = { ...t, '--base-tmp': base };
        const fundo = tonal ? sobre(tt, tonal, '--base-tmp') : base;
        const c = contraste(hex(t, frente), fundo);
        assert.ok(c >= 4.5, `${tema}: ${seletor} (${frente} sobre ${tonal ?? f} em ${f}) = ${c.toFixed(2)}:1`);
      }
    }
  }
});

// Mesmo o --success-faint fica em 4,27:1 sob o hover claro: o selo usa o fundo da linha.
test('selo verde da fila sobre fundo neutro, com contorno', () => {
  const r = regra(estilo(lista), '.meta-badge.success');
  assert.doesNotMatch(r, /background(-color)?:/);
  assert.match(r, /border:\s*1px solid currentColor/);
});

test('--warning-strong registrado no style.css e no DESIGN.md', () => {
  assert.equal(hex(claro, '--warning-strong').toUpperCase(), '#9A5B00');
  for (const [tema, t] of TEMAS) {
    for (const fundo of ['--surface', '--bg-main', '--surface-hover']) exige(t, tema, '--warning-strong', fundo, 4.5);
  }
  const sec = design.slice(design.indexOf('## 2. Cores e Temas'), design.indexOf('## 3.'));
  assert.match(sec, /--warning-strong/);
  assert.match(sec, /#9A5B00/i);
});

// --- Revisão 2026-11-10: F-AC-10 (modo de alto contraste do Windows) --------

function forcado(texto, nome) {
  const folha = estilo(texto);
  const i = folha.indexOf('@media (forced-colors: active)');
  assert.ok(i >= 0, `${nome}: sem @media (forced-colors: active)`);
  let n = 0;
  for (let j = folha.indexOf('{', i); j < folha.length; j++) {
    if (folha[j] === '{') n++;
    else if (folha[j] === '}' && --n === 0) return folha.slice(i, j + 1);
  }
  assert.fail(`${nome}: bloco forced-colors sem fechamento`);
}

function regraForcada(bloco, seletor) {
  const esc = seletor.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const m = bloco.match(new RegExp(`(?:^|[\\s,}])${esc}\\s*(?:,[^{]*)?\\{([^}]*)\\}`));
  assert.ok(m, `forced-colors: ${seletor} ausente`);
  return m[1];
}

test('alto contraste: interruptor com contorno, bolinha e trilho ligado em Highlight', () => {
  const b = forcado(settings, 'Settings');
  assert.match(regraForcada(b, '.slider'), /border:\s*1px solid ButtonText/);
  assert.match(regraForcada(b, '.slider:before'), /background-color:\s*ButtonText/);
  const ligado = regraForcada(b, 'input:checked + .slider');
  assert.match(ligado, /forced-color-adjust:\s*none/);
  assert.match(ligado, /background-color:\s*Highlight/);
  assert.match(regraForcada(b, 'input:checked + .slider:before'), /background-color:\s*HighlightText/);
  assert.match(regraForcada(b, '.switch input:focus-visible + .slider'), /outline-color:\s*CanvasText/);
});

// F-AC-09: o monitoramento virou ícone (traço em currentColor, que o modo de
// alto contraste já pinta); ficam os pontos de nível.
test('alto contraste: pontos de nível com contorno e Highlight quando cheios', () => {
  const b = forcado(menuNivel, 'MenuNivel');
  const r = regraForcada(b, '.nivel-ponto');
  assert.match(r, /forced-color-adjust:\s*none/);
  assert.match(r, /border:\s*1px solid CanvasText/);
  assert.match(regraForcada(b, '.nivel-ponto.cheio'), /background-color:\s*Highlight/);
  assert.doesNotMatch(app, /status-dot|collapsed-dot/);
});

test('alto contraste: barras e vagas com trilho contornado e preenchimento Highlight', () => {
  const l = forcado(lista, 'ProcessList');
  assert.match(regraForcada(l, '.progress-bar-container'), /border-top:\s*1px solid CanvasText/);
  const barra = regraForcada(l, '.progress-bar');
  assert.match(barra, /forced-color-adjust:\s*none/);
  assert.match(barra, /background-color:\s*Highlight/);

  const a = forcado(atividadeComp, 'AtividadeItem');
  for (const s of ['.vaga', '.doc-barra']) assert.match(regraForcada(a, s), /border:\s*1px solid CanvasText/, s);
  for (const s of ['.vaga.cheia', '.doc-barra span']) {
    const r = regraForcada(a, s);
    assert.match(r, /background-color:\s*Highlight/, s);
  }
  assert.match(regraForcada(a, '.vaga'), /forced-color-adjust:\s*none/);
  assert.match(regraForcada(a, '.doc-barra span'), /forced-color-adjust:\s*none/);
});

test('DESIGN.md descreve o modo de alto contraste', () => {
  assert.match(design, /forced-colors/);
});
