import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { OPCOES_OCR, OPCOES_LIMPEZA, descricaoOpcao } from './src/lib/atividade.js';
import { moverFoco } from './src/lib/menu.js';

// F-AC-14: na barra recolhida, OCR e higienização abrem um menu com as 4
// opções e a descrição de cada uma, em vez de ciclar a cada clique.

const ler = (rel) => readFileSync(new URL(rel, import.meta.url), 'utf8').replace(/\r\n/g, '\n');
const caminhoMenu = new URL('./src/components/MenuNivel.svelte', import.meta.url);

test('moverFoco: setas dão a volta, Home e End vão às pontas', () => {
  assert.equal(moverFoco(0, 'ArrowDown', 4), 1);
  assert.equal(moverFoco(3, 'ArrowDown', 4), 0);
  assert.equal(moverFoco(0, 'ArrowUp', 4), 3);
  assert.equal(moverFoco(2, 'Home', 4), 0);
  assert.equal(moverFoco(1, 'End', 4), 3);
  assert.equal(moverFoco(1, 'a', 4), null);
});

test('cada opção traz a descrição que o painel aberto mostra', () => {
  for (const o of [...OPCOES_OCR, ...OPCOES_LIMPEZA]) assert.ok(o.descricao, o.valor);
  assert.equal(descricaoOpcao(OPCOES_OCR, 'normal'), 'Equilíbrio entre velocidade e precisão.');
  assert.equal(descricaoOpcao(OPCOES_LIMPEZA, 'basic'), 'Remove espaços e linhas vazias');
  assert.equal(descricaoOpcao(OPCOES_LIMPEZA, 'x'), '');
  const settings = ler('./src/components/Settings.svelte');
  assert.match(settings, /descricaoOpcao\(OPCOES_OCR/);
  assert.match(settings, /descricaoOpcao\(OPCOES_LIMPEZA/);
  assert.doesNotMatch(settings, /Equilíbrio entre velocidade/, 'descrição duplicada no Settings');
});

test('MenuNivel: botão com aria-haspopup e menu de rádios na camada superior', () => {
  assert.ok(existsSync(caminhoMenu), 'MenuNivel.svelte ausente');
  const m = ler('./src/components/MenuNivel.svelte');
  assert.match(m, /aria-haspopup="menu"/);
  assert.match(m, /aria-expanded=\{aberto\}/);
  assert.match(m, /popover="manual"[^>]*role="menu"|role="menu"[^>]*popover="manual"/);
  assert.match(m, /role="menuitemradio"/);
  assert.match(m, /aria-checked=\{o\.valor === valor\}/);
  assert.match(m, /\{#each opcoes as o/);
  assert.match(m, /o\.descricao/);
  assert.match(m, /\{@attach /, 'attachment cuida de abrir, fechar ao clicar fora e devolver o foco');
  assert.match(m, /showPopover\(\)/);
  assert.match(m, /pointerdown/);
  assert.match(m, /anchor-name/);
  assert.match(m, /position-anchor/);
});

test('MenuNivel: Esc fecha e devolve o foco; setas movem; foco na opção atual', () => {
  const m = ler('./src/components/MenuNivel.svelte');
  assert.match(m, /'Escape'/);
  assert.match(m, /botao\?*\.focus\(\)/);
  assert.match(m, /moverFoco\(/);
  assert.match(m, /'Tab'/);
});

test('MenuNivel: entra com fly de 4px em --dur-fast e mostra o nome por 1,5 s', () => {
  const m = ler('./src/components/MenuNivel.svelte');
  assert.match(m, /in:fly=\{\{ x: -4, duration: duracaoMovimento\(DUR_FAST\)/);
  assert.match(m, /criarConfirmacao\(/);
  assert.doesNotMatch(m, /criarConfirmacao\([^)]*,\s*\d/, 'usa o 1,5 s padrão de criarConfirmacao');
});

test('App: sem ciclo; os dois menus com ícone e opções', () => {
  const app = ler('./src/App.svelte');
  assert.doesNotMatch(app, /ciclarOcr|ciclarLimpeza|proximaOpcao/);
  assert.match(app, /<MenuNivel[^>]*opcoes=\{OPCOES_OCR\}[^>]*icone=\{ScanText\}|<MenuNivel[^>]*icone=\{ScanText\}[^>]*opcoes=\{OPCOES_OCR\}/);
  assert.match(app, /<MenuNivel[^>]*opcoes=\{OPCOES_LIMPEZA\}[^>]*icone=\{Sparkles\}|<MenuNivel[^>]*icone=\{Sparkles\}[^>]*opcoes=\{OPCOES_LIMPEZA\}/);
});

test('DESIGN.md descreve o menu da barra recolhida', () => {
  const design = ler('../DESIGN.md');
  assert.match(design, /barra recolhida[^\n]*menu|menu[^\n]*barra recolhida/i);
  assert.match(design, /MenuNivel\.svelte/);
});
