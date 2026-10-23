// F-VI-02 (rodapé) e F-VI-03 (estado pausado).
// Rodar com: node --test frontend/estado.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { estadoItem, contarAtivos, avisoFila, rotuloFila } from './src/lib/estado.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const app = readFileSync(join(raiz, 'src', 'App.svelte'), 'utf8');
const lista = readFileSync(join(raiz, 'src', 'components', 'ProcessList.svelte'), 'utf8');

function bloco(texto, abre, fecha) {
  const i = texto.indexOf(abre);
  assert.ok(i >= 0, `${abre} não encontrado`);
  return texto.slice(i, texto.indexOf(fecha, i) + fecha.length);
}

test('plural do contador da fila', () => {
  assert.equal(rotuloFila(0), '0 itens na fila');
  assert.equal(rotuloFila(1), '1 item na fila');
  assert.equal(rotuloFila(4), '4 itens na fila');
  assert.doesNotMatch(app, /itemns|'ns'/);
});

test('rodapé em uma linha, pasta truncada nos dois ramos e versão do build', () => {
  const footer = bloco(app, '<footer>', '</footer>');
  assert.match(footer, /rotuloFila\(items\.length\)/);
  assert.match(footer, /__APP_VERSION__|versao/);
  const comPasta = [...footer.matchAll(/<span class="status-text truncate" title=\{config\.monitored_path\}>/g)];
  assert.equal(comPasta.length, 2, 'monitorando e pausado precisam de truncate + title');

  const titulo = bloco(app, 'class="custom-titlebar"', '</div>');
  assert.doesNotMatch(titulo, /v1\.0|Offline|titlebar-version/);

  const css = bloco(app, '<style>', '</style>');
  const contador = css.match(/\.status-count\s*\{([^}]*)\}/);
  assert.ok(contador, '.status-count ausente');
  assert.match(contador[1], /flex-shrink:\s*0/);
  assert.match(contador[1], /white-space:\s*nowrap/);
  const principal = css.match(/\.status-main\s*\{([^}]*)\}/);
  assert.ok(principal, '.status-main ausente');
  assert.match(principal[1], /flex:\s*1/);
  assert.match(principal[1], /min-width:\s*0/);
});

test('item em curso continua processando com a fila pausada', () => {
  assert.equal(estadoItem('processing', true), 'processando');
  assert.equal(estadoItem('processing', false), 'processando');
  assert.equal(estadoItem('pending', true), 'pausado');
  assert.equal(estadoItem('pending', false), 'aguardando');
  assert.equal(estadoItem('success', true), 'sucesso');
  assert.equal(estadoItem('error', true), 'erro');
  assert.equal(estadoItem('cancelled', true), 'cancelado');
});

test('aviso da fila pausada e contador de ativos', () => {
  const fila = [{ status: 'processing' }, { status: 'pending' }, { status: 'success' }];
  assert.equal(contarAtivos(fila), 1);
  assert.equal(avisoFila(fila, false), null);
  assert.equal(avisoFila(fila, true), 'Fila pausada: termina o item atual');
  assert.equal(avisoFila([{ status: 'pending' }], true), 'Fila pausada');
});

test('ProcessList e App derivam o estado de um único valor', () => {
  assert.match(lista, /estadoItem\(item\.status, isPaused\)/);
  assert.match(lista, /avisoFila\(items, isPaused\)/);
  assert.doesNotMatch(lista, /\{#if item\.status === 'processing'\}\s*\{#if isPaused\}/);
  assert.match(app, /contarAtivos\(items\)/);
});
