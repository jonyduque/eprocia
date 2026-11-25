// F-VI-06 (linha da fila em uma faixa de até 40px, gap 4px) e F-VI-10
// (escala tipográfica do DESIGN.md e painel sem cabeçalhos redundantes).
// Rodar com: node --test frontend/densidade.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8');
const app = ler('src', 'App.svelte');
const lista = ler('src', 'components', 'ProcessList.svelte');
const settings = ler('src', 'components', 'Settings.svelte');
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

function temRegra(texto, seletor) {
  const esc = seletor.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return new RegExp(`(?:^|[\\s}])${esc}\\s*\\{`).test(texto);
}

const px = (bloco, prop) => {
  const m = bloco.match(new RegExp(`(?:^|[\\s;])${prop}:\\s*([^;]+);`));
  return m ? m[1].trim() : null;
};

// --- F-VI-06 ---------------------------------------------------------------

test('linha da fila tem uma faixa de 36 a 40px', () => {
  const linha = regra(estilo(lista), '.item-row');
  const alt = Number(px(linha, 'min-height')?.replace('px', ''));
  assert.ok(alt >= 36 && alt <= 40, `min-height ${alt}px fora de 36-40px`);
  // padding vertical + conteúdo de 22px (botão de ação) não passa de 40px.
  const [v] = px(linha, 'padding').split(/\s+/).map((x) => Number(x.replace('px', '')));
  assert.ok(v * 2 + 22 <= 40, `padding vertical ${v}px estoura 40px`);
});

test('nome e detalhes ficam lado a lado, não empilhados', () => {
  const info = regra(estilo(lista), '.info-col');
  assert.match(info, /flex-direction:\s*row/);
  assert.match(info, /align-items:\s*center/);
  const nome = regra(estilo(lista), '.file-name');
  assert.match(nome, /min-width:\s*\d+px/, 'o nome precisa de uma largura mínima para não sumir');
});

test('lista com gap 4px e padding 8px', () => {
  const r = regra(estilo(lista), '.items-list');
  assert.match(r, /gap:\s*4px/);
  assert.match(r, /padding:\s*8px;/);
});

test('barra de progresso de 2px colada à borda inferior, sem aumentar a linha', () => {
  const s = estilo(lista);
  const barra = regra(s, '.progress-bar-container');
  assert.match(barra, /height:\s*2px/);
  assert.match(barra, /bottom:\s*0/);
  assert.match(barra, /left:\s*0/);
  assert.match(barra, /right:\s*0/);
  if (temRegra(s, '.item-row.has-progress')) {
    assert.doesNotMatch(regra(s, '.item-row.has-progress'), /padding/);
  }
  assert.doesNotMatch(regra(s, '.item-row'), /padding-bottom/);
});

test('horário em 11px tabular', () => {
  const r = regra(estilo(lista), '.timestamp');
  assert.match(r, /font-variant-numeric:\s*tabular-nums/);
  assert.match(regra(estilo(lista), '.details-row'), /font-size:\s*var\(--fs-meta\)/);
});

test('DESIGN.md descreve a linha de uma faixa', () => {
  const sec = design.slice(design.indexOf('### 5.1'), design.indexOf('### 5.2'));
  assert.match(sec, /uma faixa/);
  assert.match(sec, /2px/);
});

// --- F-VI-10 ---------------------------------------------------------------

test('tokens de tipografia em style.css', () => {
  const root = regra(css, ':root');
  assert.match(root, /--fs-title:\s*16px/);
  assert.match(root, /--fs-section:\s*12px/);
  assert.match(root, /--fs-base:\s*13px/);
  assert.match(root, /--fs-meta:\s*11px/);
});

test('título da fila em 16px semibold', () => {
  const r = regra(estilo(lista), '.header-left h2');
  assert.match(r, /font-size:\s*var\(--fs-title\)/);
  assert.match(r, /font-weight:\s*600/);
});

test('nome do arquivo em 13px/500', () => {
  const r = regra(estilo(lista), '.file-name');
  assert.match(r, /font-size:\s*var\(--fs-base\)/);
  assert.match(r, /font-weight:\s*500/);
});

test('marca da barra de título sem peso 700', () => {
  const r = regra(estilo(app), '.brand-title h1');
  assert.match(r, /font-size:\s*var\(--fs-base\)/);
  assert.doesNotMatch(r, /font-weight:\s*700/);
});

function rotuloSecao(r, nome) {
  assert.match(r, /font-size:\s*var\(--fs-section\)/, nome);
  assert.match(r, /font-weight:\s*600/, nome);
  assert.match(r, /text-transform:\s*uppercase/, nome);
  assert.match(r, /color:\s*var\(--text-secondary\)/, nome);
}

test('Configurações vira seção com rótulo pequeno, sem Importação Manual', () => {
  // F-VI-07: "Configurações" vai para a faixa do botão de recolher.
  const ini = app.indexOf('<div class="sidebar-header-toggle">');
  const cab = app.slice(ini, app.indexOf('</div>', app.indexOf('</h2>', ini)));
  assert.match(cab, /<h2[^>]*>Configurações<\/h2>/);
  rotuloSecao(regra(estilo(app), '.painel-titulo h2'), 'Configurações');
  assert.doesNotMatch(app, /manual-card|Importação Manual/);
});

test('Configurações em três blocos rotulados: Monitoramento, Processamento, Sistema', () => {
  // O rótulo é o texto do botão que recolhe a categoria (depois do chevron).
  const blocos = [...settings.matchAll(/<h3 class="rotulo-grupo">[\s\S]*?<\/span>\s*([^<\s][^<]*?)\s*<\/button>\s*<\/h3>/g)].map((m) => m[1]);
  assert.deepEqual(blocos, ['Monitoramento', 'Processamento', 'Sistema']);
  rotuloSecao(regra(estilo(settings), '.rotulo-grupo'), 'blocos');
  const ordem = ['Monitoramento', 'id="watchdog-toggle"', 'id="monitored-path"', 'Processamento',
    'id="ocr-mode"', 'id="cleaning-level"', 'id="omitir-tipos"', 'Sistema', 'id="autostart-toggle"'];
  const pos = ordem.map((o) => settings.indexOf(o));
  for (let i = 1; i < pos.length; i++) assert.ok(pos[i] > pos[i - 1], `${ordem[i]} fora de ordem`);
});

test('painel com a guia de 12px das barras, 16px entre seções e 12px entre grupos', () => {
  const s = estilo(app);
  const conteudo = regra(s, '.sidebar-content');
  assert.equal(px(conteudo, 'padding'), '12px');
  assert.equal(px(conteudo, 'gap'), '16px');
  assert.match(px(regra(s, '.sidebar-header-toggle'), 'padding'), /^\d+px 12px$/);
  const st = estilo(settings);
  assert.equal(px(regra(st, '.settings-card'), 'gap'), '16px');
  assert.equal(px(regra(st, '.grupo'), 'gap'), '12px');
});

test('DESIGN.md: título do painel na faixa de recolher e guia de 12px', () => {
  const s53 = design.slice(design.indexOf('### 5.3'), design.indexOf('### 5.4'));
  assert.doesNotMatch(s53, /Sem título geral/);
  assert.match(s53, /Monitoramento, Processamento e Sistema/);
  const s6 = design.slice(design.indexOf('## 6.'), design.indexOf('## 7.'));
  assert.match(s6, /painel[^.]*12px/i);
});

test('seção do painel sem moldura de cartão', () => {
  const cfg = regra(estilo(settings), '.settings-card');
  assert.doesNotMatch(cfg, /box-shadow/);
  assert.doesNotMatch(cfg, /(?:^|[\s;])border:\s*1px/);
});

test('painel sem o cabeçalho "Painel de Controle"', () => {
  assert.doesNotMatch(app, /Painel de Controle/);
});

test('componentes sem tamanhos de fonte fora da escala', () => {
  for (const [nome, texto] of [['App', app], ['ProcessList', lista], ['Settings', settings]]) {
    const s = estilo(texto);
    for (const m of s.matchAll(/font-size:\s*([^;]+);/g)) {
      assert.match(m[1].trim(), /^var\(--fs-(title|section|base|meta)\)$/, `${nome}: font-size ${m[1]}`);
    }
  }
});

test('DESIGN.md registra os tokens de tipografia', () => {
  const sec = design.slice(design.indexOf('## 3. Tipografia'), design.indexOf('## 4.'));
  for (const t of ['--fs-title', '--fs-section', '--fs-base', '--fs-meta']) {
    assert.ok(sec.includes(t), `DESIGN.md §3 sem ${t}`);
  }
});
