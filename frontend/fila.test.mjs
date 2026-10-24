// F-AC-09 (foco não se perde ao remover/cancelar/limpar), F-AC-10 (mensagem
// de erro completa acessível por teclado) e F-DF-03 (hora e filtro sem
// trabalho repetido por linha).
// Rodar com: node --test frontend/fila.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { formatarHora, filtrarItens, alvoFoco, escolherAcao } from './src/lib/fila.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, 'src', ...p), 'utf8').replace(/\r\n/g, '\n');
const lista = ler('components', 'ProcessList.svelte');
const app = ler('App.svelte');

function tag(texto, marca) {
  const i = texto.indexOf(marca);
  assert.ok(i >= 0, `${marca} ausente`);
  const ini = texto.lastIndexOf('<', i);
  return texto.slice(ini, texto.indexOf('>', i) + 1);
}

function estilo(texto) {
  const i = texto.indexOf('<style>');
  return texto.slice(i, texto.indexOf('</style>', i));
}

function regra(css, seletor) {
  const i = css.indexOf(`${seletor} {`);
  assert.ok(i >= 0, `regra ${seletor} ausente`);
  return css.slice(i, css.indexOf('}', i));
}

// --- F-DF-03 ---------------------------------------------------------------

test('formatarHora usa pt-BR com segundos e tolera vazio/inválido', () => {
  const ts = new Date(2026, 9, 23, 9, 5, 7).getTime();
  assert.equal(formatarHora(ts), '09:05:07');
  assert.equal(formatarHora(0), '');
  assert.equal(formatarHora(undefined), '');
  assert.equal(formatarHora('não é data'), '');
});

test('formatarHora reaproveita um Intl.DateTimeFormat criado no módulo', () => {
  const fonte = ler('lib', 'fila.js');
  const corpo = fonte.slice(fonte.indexOf('export function formatarHora'));
  assert.doesNotMatch(corpo.slice(0, corpo.indexOf('\n}')), /DateTimeFormat|toLocale/);
  assert.match(fonte, /^const \w+ = new Intl\.DateTimeFormat\('pt-BR'/m);
});

test('filtrarItens devolve a mesma lista com filtro vazio', () => {
  const items = [{ name: 'a.zip' }];
  assert.equal(filtrarItens(items, ''), items);
  assert.equal(filtrarItens(items, undefined), items);
});

test('filtrarItens compara nome e mensagem sem diferenciar maiúsculas', () => {
  const items = [
    { name: 'Processo-1.zip', message: '' },
    { name: 'outro.zip', message: 'Falha no PDF' },
    { name: 'x.zip' },
  ];
  assert.deepEqual(filtrarItens(items, 'PROCESSO').map((i) => i.name), ['Processo-1.zip']);
  assert.deepEqual(filtrarItens(items, 'pdf').map((i) => i.name), ['outro.zip']);
});

test('filtrarItens converte o filtro para minúsculas uma vez só', () => {
  let chamadas = 0;
  const filtro = { toLowerCase() { chamadas++; return 'z'; } };
  filtrarItens([{ name: 'a' }, { name: 'b' }, { name: 'c' }], filtro);
  assert.equal(chamadas, 1);
});

test('ProcessList usa formatarHora e filtrarItens', () => {
  assert.match(lista, /\{formatarHora\(item\.timestamp\)\}/);
  assert.match(lista, /\$: filteredItems = filtrarItens\(items, filterText\)/);
  assert.doesNotMatch(lista, /toLocaleTimeString|function formatTime/);
});

// --- F-AC-09 ---------------------------------------------------------------

test('alvoFoco: linha que continua recebe o foco de volta', () => {
  assert.equal(alvoFoco(['a', 'b', 'c'], 'b', ['a', 'b', 'c']), 'b');
});

test('alvoFoco: linha removida passa para a seguinte, depois a anterior', () => {
  assert.equal(alvoFoco(['a', 'b', 'c'], 'b', ['a', 'c']), 'c');
  assert.equal(alvoFoco(['a', 'b', 'c'], 'c', ['a', 'b']), 'b');
  assert.equal(alvoFoco(['a', 'b', 'c', 'd'], 'b', ['a', 'd']), 'd');
});

test('alvoFoco: sem vizinhos (ou limpar fila) devolve null', () => {
  assert.equal(alvoFoco(['a'], 'a', []), null);
  assert.equal(alvoFoco(['a', 'b'], null, []), null);
});

test('escolherAcao prefere a mesma ação, depois remover/cancelar', () => {
  assert.equal(escolherAcao(['copiar', 'abrir', 'remover'], 'remover'), 'remover');
  assert.equal(escolherAcao(['remover'], 'cancelar'), 'remover');
  assert.equal(escolherAcao(['cancelar'], 'remover'), 'cancelar');
  assert.equal(escolherAcao(['detalhes', 'remover'], 'copiar'), 'remover');
  assert.equal(escolherAcao([], 'remover'), null);
});

test('título da lista recebe foco programático', () => {
  assert.match(tag(lista, '<h2'), /tabindex="-1"/);
  assert.match(tag(lista, '<h2'), /bind:this=\{tituloEl\}/);
});

test('linhas e botões de ação são localizáveis para devolver o foco', () => {
  assert.match(tag(lista, 'class="item-row'), /data-id=\{item\.id\}/);
  for (const acao of ['cancelar', 'remover', 'copiar', 'abrir', 'detalhes']) {
    assert.match(lista, new RegExp(`data-acao="${acao}"`), `data-acao="${acao}" ausente`);
  }
});

test('Cancelar, Remover e Limpar Fila registram para onde o foco volta', () => {
  assert.match(lista, /on:click=\{\(e\) => handleCancel\(item\.id, e\)\}/);
  assert.match(lista, /on:click=\{\(e\) => handleRemove\(item\.id, e\)\}/);
  assert.match(lista, /on:click=\{handleClear\}/);
  for (const nome of ['handleCancel', 'handleRemove', 'handleClear']) {
    const i = lista.indexOf(`async function ${nome}`);
    const corpo = lista.slice(i, lista.indexOf('\n  }\n', i));
    assert.match(corpo, /guardarFoco\(/, `${nome} não guarda o foco`);
  }
  assert.match(lista, /alvoFoco\(/);
  assert.match(lista, /tick\(\)/);
});

// --- F-AC-10 ---------------------------------------------------------------

test('detalhe do erro é um botão com aria-label e aria-expanded', () => {
  assert.doesNotMatch(lista, /class="error-tooltip-container"/);
  const i = lista.indexOf('data-acao="detalhes"');
  assert.ok(i >= 0, 'botão de detalhes ausente');
  const botao = lista.slice(lista.lastIndexOf('<', i), lista.indexOf('</button>', i));
  assert.match(botao, /^<button/);
  assert.match(botao, /type="button"/);
  assert.match(botao, /aria-label="Ver detalhes do erro"/);
  assert.match(botao, /aria-expanded=/);
  assert.match(botao, /aria-controls=/);
});

test('linha expandida mostra a mensagem completa e o caminho, selecionáveis', () => {
  const i = lista.indexOf('class="erro-detalhe"');
  assert.ok(i >= 0, 'linha de detalhe ausente');
  const bloco = lista.slice(i, lista.indexOf('{/if}', i));
  assert.match(bloco, /\{item\.message/);
  assert.match(bloco, /\{item\.path\}/);
  const css = regra(estilo(lista), '.erro-detalhe');
  assert.match(css, /user-select:\s*text/);
  assert.match(css, /overflow-wrap:\s*anywhere/);
});

test('indicador da barra recolhida tem role=status e texto .sr-only', () => {
  const ind = tag(app, 'class="collapsed-indicator"');
  assert.match(ind, /role="status"/);
  const i = app.indexOf('class="collapsed-indicator"');
  const bloco = app.slice(i, app.indexOf('</div>', i));
  assert.match(bloco, /<span class="sr-only">\{textoMonitoramento\}<\/span>/);
  assert.match(app, /\.sr-only\s*\{[^}]*clip/);
});
