// F-AC-07 (botões expõem o estado), F-AC-08 (divisor operável por teclado) e
// F-RS-03 (largura e estado recolhido do painel persistidos).
// Rodar com: node --test frontend/painel.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { LARGURA_MIN_PAINEL, larguraMaxPainel, teclaDivisor } from './src/lib/divisor.js';
import { CHAVE_PAINEL, PAINEL_PADRAO, lerPainel, gravarPainel } from './src/lib/painel.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const app = readFileSync(join(raiz, 'src', 'App.svelte'), 'utf8').replace(/\r\n/g, '\n');

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

function memoria(inicial = {}) {
  const dados = { ...inicial };
  return {
    dados,
    getItem: (k) => (k in dados ? dados[k] : null),
    setItem: (k, v) => { dados[k] = String(v); },
  };
}

const quebrado = {
  getItem() { throw new Error('bloqueado'); },
  setItem() { throw new Error('bloqueado'); },
};

// --- F-RS-03 ---------------------------------------------------------------

test('painel sem nada salvo usa o padrão', () => {
  assert.equal(CHAVE_PAINEL, 'eprocia.sidebar');
  assert.deepEqual(lerPainel(memoria(), 1280), PAINEL_PADRAO);
  assert.deepEqual(lerPainel(null, 1280), PAINEL_PADRAO);
  assert.deepEqual(lerPainel(quebrado, 1280), PAINEL_PADRAO);
});

test('painel salvo volta com a largura presa ao limite da janela', () => {
  const s = memoria();
  assert.equal(gravarPainel(s, { largura: 470, recolhido: true }), true);
  assert.deepEqual(JSON.parse(s.dados[CHAVE_PAINEL]), { largura: 470, recolhido: true });
  assert.deepEqual(lerPainel(s, 1280), { largura: 470, recolhido: true });
  assert.deepEqual(lerPainel(s, 800), { largura: larguraMaxPainel(800), recolhido: true });
  gravarPainel(s, { largura: 100, recolhido: false });
  assert.deepEqual(lerPainel(s, 1280), { largura: LARGURA_MIN_PAINEL, recolhido: false });
});

test('valor salvo inválido cai no padrão campo a campo', () => {
  assert.deepEqual(lerPainel(memoria({ [CHAVE_PAINEL]: '{quebrado' }), 1280), PAINEL_PADRAO);
  assert.deepEqual(
    lerPainel(memoria({ [CHAVE_PAINEL]: '{"largura":"x","recolhido":true}' }), 1280),
    { largura: PAINEL_PADRAO.largura, recolhido: true }
  );
  assert.deepEqual(
    lerPainel(memoria({ [CHAVE_PAINEL]: '{"largura":350,"recolhido":"sim"}' }), 1280),
    { largura: 350, recolhido: false }
  );
});

test('gravar com armazenamento bloqueado não lança', () => {
  assert.equal(gravarPainel(quebrado, { largura: 300, recolhido: false }), false);
  assert.equal(gravarPainel(null, { largura: 300, recolhido: false }), false);
});

test('App lê o painel na montagem e grava ao recolher e ao terminar de arrastar', () => {
  assert.match(app, /lerPainel\(armazenamento\(\),\s*larguraJanela\)/);
  const corpo = (nome) => {
    const i = app.indexOf(`function ${nome}(`);
    assert.ok(i >= 0, `${nome} ausente`);
    return app.slice(i, app.indexOf('\n  }\n', i));
  };
  assert.match(corpo('toggleSidebar'), /salvarPainel\(\)/);
  assert.match(corpo('stopResizing'), /salvarPainel\(\)/);
  assert.match(corpo('salvarPainel'), /gravarPainel\(armazenamento\(\)/);
});

// --- F-AC-08 ---------------------------------------------------------------

test('teclas do divisor: setas, Home, End e Enter', () => {
  assert.deepEqual(teclaDivisor('ArrowLeft', 320, 1280), { largura: 310 });
  assert.deepEqual(teclaDivisor('ArrowRight', 320, 1280), { largura: 330 });
  assert.deepEqual(teclaDivisor('ArrowLeft', LARGURA_MIN_PAINEL, 1280), { largura: LARGURA_MIN_PAINEL });
  assert.deepEqual(teclaDivisor('Home', 400, 1280), { largura: LARGURA_MIN_PAINEL });
  assert.deepEqual(teclaDivisor('End', 300, 1280), { largura: 480 });
  assert.deepEqual(teclaDivisor('End', 300, 800), { largura: larguraMaxPainel(800) });
  assert.deepEqual(teclaDivisor('Enter', 300, 1280), { recolher: true });
  assert.equal(teclaDivisor('a', 300, 1280), null);
  assert.equal(teclaDivisor('Tab', 300, 1280), null);
});

test('divisor previne o padrão, expõe valor em texto e controla o painel', () => {
  const i = app.indexOf('function handleResizeKeydown(');
  const corpo = app.slice(i, app.indexOf('\n  }\n', i));
  assert.match(corpo, /teclaDivisor\(/);
  assert.match(corpo, /e\.preventDefault\(\)/);
  const divisor = tag(app, 'class="resize-handle"');
  assert.match(divisor, /aria-valuetext="\{larguraPainel\} pixels"/);
  assert.match(divisor, /aria-controls="sidebar"/);
  assert.match(tag(app, 'class="sidebar"'), /id="sidebar"/);
  assert.match(estilo(app), /\.resize-handle:focus-visible[^{]*\{[^}]*background-color:\s*var\(--accent\)/);
});

// --- F-AC-07 ---------------------------------------------------------------

test('botão de recolher expõe aria-expanded e aria-controls', () => {
  const b = tag(app, 'class="btn-toggle-sidebar"');
  assert.match(b, /aria-expanded=\{!sidebarCollapsed\}/);
  assert.match(b, /aria-controls="sidebar"/);
  assert.match(b, /aria-label=\{sidebarCollapsed \? 'Expandir painel' : 'Recolher painel'\}/);
});

test('botão de tema diz o tema atual e o próximo', () => {
  const b = tag(app, 'on:click={toggleTheme}');
  assert.match(b, /aria-label="Tema: \{NOME_TEMA\[temaPref\]\}\. Mudar para \{NOME_TEMA\[proximoTema\(temaPref\)\]\}"/);
});

test('maximizar alterna o rótulo para Restaurar pelo estado real da janela', () => {
  assert.match(app, /WindowIsMaximised/);
  const b = tag(app, 'on:click={alternarMaximizar}');
  assert.match(b, /aria-label=\{maximizada \? 'Restaurar' : 'Maximizar'\}/);
  assert.match(b, /title=\{maximizada \? 'Restaurar' : 'Maximizar'\}/);
  assert.match(app, /<svelte:window[^>]*on:resize=\{atualizarMaximizada\}/);
  assert.match(tag(app, 'on:click={WindowMinimise}'), /aria-label="Minimizar"/);
  assert.match(tag(app, 'on:click={handleCloseWindow}'), /aria-label="Fechar \(minimiza para a bandeja\)"/);
});

test('SVGs dos botões da barra de título e do painel são decorativos', () => {
  const ini = app.indexOf('<div class="titlebar-actions"');
  const fim = app.indexOf('<div class="sidebar-content"');
  const trecho = app.slice(ini, fim);
  // Icone.svelte sempre põe aria-hidden (conferido em icones.test.mjs).
  const icones = trecho.match(/<Icone\b[^>]*>/g) ?? [];
  assert.ok(icones.length >= 6);
  assert.doesNotMatch(trecho, /<svg/);
});
