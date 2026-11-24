// F-RS-01 (cabeçalho da fila encolhe em uma linha) e F-RS-02 (limite do
// divisor respeita a largura da janela).
// Rodar com: node --test frontend/layout.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  LARGURA_MIN_PAINEL,
  LARGURA_MAX_PAINEL,
  LARGURA_MIN_FILA,
  larguraMaxPainel,
  limitarPainel
} from './src/lib/divisor.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8');
const app = ler('src', 'App.svelte');
const lista = ler('src', 'components', 'ProcessList.svelte');
const mainGo = readFileSync(join(raiz, '..', 'main.go'), 'utf8');

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

// --- F-RS-02 ---------------------------------------------------------------

test('limite máximo do painel desconta a largura mínima da fila', () => {
  assert.equal(LARGURA_MIN_PAINEL, 280);
  assert.equal(LARGURA_MAX_PAINEL, 480);
  assert.equal(larguraMaxPainel(800), 800 - LARGURA_MIN_FILA);
  assert.ok(larguraMaxPainel(800) < 480);
  assert.equal(larguraMaxPainel(1920), 480);
  // Janela menor que o mínimo somado nunca inverte o intervalo.
  assert.equal(larguraMaxPainel(500), LARGURA_MIN_PAINEL);
});

test('na largura mínima da janela a fila mantém a largura mínima', () => {
  const minJanela = Number(mainGo.match(/MinWidth:\s*(\d+)/)[1]);
  assert.ok(minJanela - larguraMaxPainel(minJanela) >= LARGURA_MIN_FILA);
});

// P6. Medido em 2026-11-22 (Chromium, stubs, janela 800x600, ZIP
// 5000000-00.2026.4.00.0001.zip, rótulo "12 não lidos"): o rótulo só fica
// dentro de .details-row (sem recorte) com a fila em 453 px ou mais; a 451 px
// já é recortado. 453 + 8 px de margem para o gap = 461.
test('a fila mínima mantém visível o rótulo "N não lidos"', () => {
  assert.ok(LARGURA_MIN_FILA >= 461);
  assert.ok(800 - larguraMaxPainel(800) >= 461);
});

test('limitarPainel prende a largura entre o mínimo e o máximo da janela', () => {
  assert.equal(limitarPainel(200, 1024), 280);
  assert.equal(limitarPainel(350, 1024), 350);
  assert.equal(limitarPainel(480, 800), larguraMaxPainel(800));
  assert.equal(limitarPainel(900, 1920), 480);
});

test('App aplica o limite pela largura real da janela', () => {
  assert.match(app, /<svelte:window[^>]*bind:innerWidth=\{larguraJanela\}/);
  assert.match(app, /limitarPainel\(sidebarWidth,\s*larguraJanela\)/);
  assert.doesNotMatch(app, /Math\.min\(480/);
  assert.doesNotMatch(app, /<= 480/);
  assert.match(app, /aria-valuemax=\{maxPainel\}/);
  assert.match(app, /aria-valuenow=\{larguraPainel\}/);
  assert.match(app, /width: \{sidebarCollapsed \? '48px' : larguraPainel \+ 'px'\}/);
});

test('painel não encolhe e a área da fila aceita encolher', () => {
  const css = estilo(app);
  assert.match(regra(css, '.sidebar'), /flex-shrink:\s*0/);
  assert.match(regra(css, '.content-area'), /min-width:\s*0/);
});

// --- F-RS-01 ---------------------------------------------------------------

test('título da fila trunca em uma linha', () => {
  const css = estilo(lista);
  const h2 = regra(css, '.header-left h2');
  assert.match(h2, /white-space:\s*nowrap/);
  assert.match(h2, /overflow:\s*hidden/);
  assert.match(h2, /text-overflow:\s*ellipsis/);
  assert.match(h2, /min-width:\s*0/);
  const esq = regra(css, '.header-left');
  assert.match(esq, /min-width:\s*0/);
  assert.match(esq, /flex:\s*1/);
  assert.match(regra(css, '.header-actions'), /flex-shrink:\s*0/);
});

test('campo Filtrar não cresce no foco', () => {
  const css = estilo(lista);
  const campo = regra(css, '.search-input');
  assert.match(campo, /flex:\s*0 1 140px/);
  assert.match(campo, /min-width:\s*80px/);
  assert.doesNotMatch(regra(css, '.search-input:focus'), /width/);
});

test('em contêiner estreito os botões ficam só com ícone e mantêm nome acessível', () => {
  const css = estilo(lista);
  assert.match(regra(css, '.list-card'), /container-type:\s*inline-size/);
  // Medido em 2026-11-24 (Task 19, rodada 1; Chromium, estado relatorio, claro e
  // escuro, caixa de conteúdo do card de 480 a 700 px): com Pausar, Limpar
  // concluídos e Limpar tudo rotulados, o título inteiro só cabe a partir de 660 px.
  assert.match(css, /@container[^{]*\(max-width:\s*660px\)\s*\{[^}]*\.rotulo-acao\s*\{[^}]*display:\s*none/);
  for (const nome of ['Retomar', 'Pausar', 'Limpar concluídos', 'Limpar tudo']) {
    assert.match(lista, new RegExp(`aria-label="${nome}"`), `${nome} sem aria-label`);
    assert.match(lista, new RegExp(`<span class="rotulo-acao">${nome}</span>`), `${nome} sem rótulo ocultável`);
  }
  assert.match(lista, /class="btn-clear"[^>]*>\s*<Icone icone=\{ListX\}/);
});

// P6. Medido em 2026-11-22 (Chromium, stubs nao-lidos e fila, claro e escuro,
// painel 280, card de 440 a 536 px): o cabeçalho com rótulos e o ícone do
// título só cabe sem corte com card >= 484 px (480 corta). Breakpoint = 484 + 8.
test('o breakpoint dos rótulos do cabeçalho cobre a largura medida de 484px', () => {
  const m = lista.match(/@container\s*\(max-width:\s*(\d+)px\)\s*\{\s*\.rotulo-acao/);
  assert.ok(m, 'regra de .rotulo-acao não encontrada');
  assert.ok(Number(m[1]) >= 484 + 8);
});

// --- F-VI-01 ---------------------------------------------------------------

test('seções do painel não encolhem: a rolagem fica na área do painel', () => {
  const settings = ler('src', 'components', 'Settings.svelte');
  const cfg = regra(estilo(settings), '.settings-card');
  assert.match(cfg, /flex:\s*none/, 'Configurações encolhe e corta os controles a 600px de altura');
  assert.doesNotMatch(cfg, /overflow:\s*hidden/);
  const manual = regra(estilo(app), '.manual-card');
  assert.match(manual, /flex:\s*none/, 'Importação Manual encolhe');
  assert.doesNotMatch(manual, /overflow:\s*hidden/);
  assert.match(regra(estilo(app), '.sidebar-content'), /overflow-y:\s*auto/);
});
