// F-VI-03 (linha com erro se destaca pela borda, um só ponto vermelho),
// F-VI-09 (nome com prioridade, contadores com ícones e números abreviados) e
// F-DF-02 (linhas fora da tela saem do layout com content-visibility).
// Rodar com: node --test frontend/linha-fila.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { abreviarContagem as abreviar, nomeExibido, dicaContadores } from './src/lib/fila.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8').replace(/\r\n/g, '\n');
const lista = ler('src', 'components', 'ProcessList.svelte');
const css = ler('src', 'assets', 'css', 'style.css');
const design = ler('..', 'DESIGN.md');

function estilo(texto) {
  const i = texto.indexOf('<style>');
  return texto.slice(i, texto.indexOf('</style>', i));
}

function regra(texto, seletor) {
  const esc = seletor.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const m = texto.match(new RegExp(`(?:^|[\\s},])${esc}\\s*[,{]([^}]*)\\}`));
  assert.ok(m, `${seletor} ausente`);
  return m[1];
}

function botao(texto, acao) {
  const i = texto.indexOf(`data-acao="${acao}"`);
  assert.ok(i >= 0, `botão ${acao} ausente`);
  return texto.slice(texto.lastIndexOf('<button', i), texto.indexOf('</button>', i));
}

// --- F-VI-03 ---------------------------------------------------------------

test('--error-line derivado de --error a 30%, como --warning-line', () => {
  assert.match(css, /--error-line:\s*color-mix\(in srgb, var\(--error\) 30%, transparent\);/);
  assert.match(design, /--error-line/);
});

test('linha com erro tem borda --error-line, inclusive no hover; em processamento, --accent-line', () => {
  const s = estilo(lista);
  assert.match(regra(s, '.item-row.error'), /border-color:\s*var\(--error-line\)/);
  assert.match(s, /\.item-row\.error:hover\s*\{[^}]*border-color:\s*var\(--error-line\)/);
  assert.match(regra(s, '.item-row.has-progress'), /border-color:\s*var\(--accent-line\)/);
  assert.doesNotMatch(regra(s, '.item-row.error'), /background/);
});

test('detalhes do erro: Info em --text-secondary, vermelho só aberto', () => {
  const b = botao(lista, 'detalhes');
  assert.match(b, /<Icone icone=\{Info\}/);
  assert.doesNotMatch(lista, /CircleQuestionMark/);
  assert.match(lista, /import Info from 'lucide-svelte\/icons\/info'/);
  const s = estilo(lista);
  assert.doesNotMatch(s, /\.btn-detalhes\s*\{[^}]*color:\s*var\(--error\)/);
  assert.match(s, /\.btn-detalhes\[aria-expanded='true'\][^{]*\{[^}]*(?:^|[^-])color:\s*var\(--error\)/);
});

// --- F-VI-09 ---------------------------------------------------------------

test('números abreviados em pt-BR', () => {
  // Intl separa número e unidade com espaço inseparável: o selo não quebra.
  const abreviarContagem = (n) => abreviar(n).replace(/\u00a0/g, ' ');
  assert.equal(abreviarContagem(24), '24');
  assert.equal(abreviarContagem(999), '999');
  assert.equal(abreviarContagem(1234), '1,2 mil');
  assert.equal(abreviarContagem(98450), '98 mil');
  assert.equal(abreviarContagem(1234567), '1,2 mi');
  assert.equal(abreviarContagem(undefined), '0');
});

test('dica dos contadores traz os valores exatos', () => {
  assert.equal(
    dicaContadores({ status: 'processing', processed_docs: 24, total_docs: 57, processed_pages: 1234, total_pages: 2000 }),
    'Documentos: 24 de 57; páginas: 1.234 de 2.000'
  );
  assert.equal(
    dicaContadores({ status: 'success', docs_count: 57, pages_count: 142, estimated_tokens: 98450 }),
    '57 documentos, 142 páginas, ~98.450 tokens (estimativa local: 1 token a cada 4 caracteres)'
  );
  assert.equal(dicaContadores({ status: 'success', docs_count: 1, pages_count: 1 }), '1 documento, 1 página');
});

test('nome exibido sem a extensão .zip', () => {
  assert.equal(nomeExibido('5001234-56.2024.8.21.0001.zip'), '5001234-56.2024.8.21.0001');
  assert.equal(nomeExibido('processo.ZIP'), 'processo');
  assert.equal(nomeExibido('sem extensao'), 'sem extensao');
  assert.equal(nomeExibido(undefined), '');
  assert.match(lista, /<span class="file-name" title=\{item\.path\}>\{nomeExibido\(item\.name\)\}<\/span>/);
});

test('nome tem prioridade: os detalhes encolhem antes', () => {
  const s = estilo(lista);
  const nome = regra(s, '.file-name');
  const det = regra(s, '.details-row');
  // Base 0: os detalhes só recebem o espaço que sobra depois do nome inteiro.
  assert.match(nome, /flex:\s*0 1 auto/);
  assert.match(det, /flex:\s*1 1 0;/);
  assert.match(nome, /min-width:\s*\d+px/);
  // O que não cabe some inteiro (quebra para uma linha recortada), sem fatiar.
  assert.match(det, /flex-wrap:\s*wrap/);
  assert.match(det, /flex-direction:\s*row-reverse/);
  assert.match(det, /height:\s*\d+px/);
  assert.match(det, /overflow:\s*hidden/);
  // Fila da direita: não lidos, selo, mensagem e, por último, o horário.
  const ordem = (sel) => Number(regra(s, sel).match(/order:\s*(-?\d+)/)?.[1]);
  assert.ok(ordem('.details-row > .nao-lidos') < ordem('.details-row > .meta-badge'));
  assert.ok(ordem('.details-row > .meta-badge') < 0);
  assert.ok(ordem('.timestamp') > 0);
  // Sentinela de largura nula abre cada linha: selo que não cabe quebra inteiro.
  assert.ok(ordem('.details-row::before') < ordem('.details-row > .nao-lidos'));
  assert.match(regra(s, '.details-row::before'), /height:\s*18px/, 'sem altura a primeira linha fica com 0px');
  assert.match(regra(s, '.details-row > .meta-badge'), /flex:\s*none/);
  // Mensagens truncam (base 0) e saem inteiras abaixo da largura mínima.
  const msg = regra(s, '.details-row > .error-message');
  assert.match(msg, /flex:\s*1 1 0;/);
  assert.match(msg, /min-width:\s*\d+px/);
  assert.match(msg, /max-width:\s*max-content/, 'sem teto a mensagem afasta o horário');
  // Fila estreita: tokens saem antes de documentos e páginas.
  assert.match(s, /@container \(max-width: \d+px\) \{\s*\.contador\.tokens \{\s*display: none;/);
  assert.match(lista, /<span class="contador tokens"><Icone icone=\{Coins\}/);
});

test('contadores com ícones, números abreviados, nomes em .sr-only e valor exato na dica', () => {
  for (const i of ['files', 'file-text', 'coins']) {
    assert.match(lista, new RegExp(`from 'lucide-svelte/icons/${i}'`));
  }
  const ini = lista.indexOf('<span class="details-row">');
  const linha = lista.slice(ini, lista.indexOf('</span>\n              {#if expansivel}', ini));
  assert.doesNotMatch(linha, /Docs:|Págs:|docs \||pág\b/);
  assert.match(linha, /<Icone icone=\{Files\} tamanho=\{12\}/);
  assert.match(linha, /<Icone icone=\{FileText\} tamanho=\{12\}/);
  assert.match(linha, /<Icone icone=\{Coins\} tamanho=\{12\}/);
  assert.match(linha, /<span class="sr-only">documentos<\/span>/);
  assert.match(linha, /<span class="sr-only">páginas<\/span>/);
  assert.match(linha, /<span class="sr-only">tokens estimados<\/span>/);
  assert.match(linha, /abreviarContagem\(item\.estimated_tokens\)/);
  assert.match(linha, /title=\{dicaContadores\(item\)\}/);
  assert.match(regra(estilo(lista), '.contador'), /font-variant-numeric:\s*tabular-nums/);
});

test('mensagem de progresso só aparece antes dos contadores', () => {
  const ini = lista.indexOf("{:else if estado === 'processando'}");
  const bloco = lista.slice(ini, lista.indexOf("{:else if estado === 'pausado'}", ini));
  assert.match(bloco, /\{#if !item\.total_docs\}\s*<span class="progress-message">/);
});

test('"Aguardando na fila..." sem itálico; .nao-lidos não encolhe', () => {
  const s = estilo(lista);
  const pend = s.match(/\n  \.pending-message\s*\{([^}]*)\}/);
  assert.ok(pend, '.pending-message ausente');
  assert.doesNotMatch(pend[1], /italic/);
  assert.match(regra(s, '.details-row > .nao-lidos'), /flex:\s*none/);
});

test('DESIGN.md §5.1 descreve contadores com ícones e nome com prioridade', () => {
  const sec = design.slice(design.indexOf('### 5.1'), design.indexOf('### 5.2'));
  assert.match(sec, /Files/);
  assert.match(sec, /FileText/);
  assert.match(sec, /abreviad/);
  assert.match(sec, /sem a extensão/);
});

// --- F-DF-02 ---------------------------------------------------------------

test('linha da fila com content-visibility: auto e tamanho intrínseco lembrado', () => {
  const r = regra(estilo(lista), '.item-row');
  assert.match(r, /content-visibility:\s*auto/);
  assert.match(r, /contain-intrinsic-size:\s*auto 36px/);
});

test('anel de foco do nome fica dentro da linha (contenção de pintura recorta o que sai)', () => {
  const r = regra(estilo(lista), '.info-btn:focus-visible');
  const off = Number(r.match(/outline-offset:\s*(-?\d+)px/)?.[1]);
  assert.ok(off <= -2, `outline-offset ${off}px sai da linha`);
});
