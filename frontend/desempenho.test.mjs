// F-DF-01 (fila atualizada no máximo uma vez por quadro) e F-DF-02 (config
// lida uma vez pelo App e passada ao Settings).
// Rodar com: node --test frontend/desempenho.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarFilaPorQuadro } from './src/lib/quadro.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, 'src', ...p), 'utf8').replace(/\r\n/g, '\n');
const app = ler('App.svelte');
const settings = ler('components', 'Settings.svelte');

function quadros() {
  const pend = [];
  return {
    agendar(fn) { pend.push(fn); return pend.length; },
    rodar() { const fns = pend.splice(0); fns.forEach((f) => f()); },
    get pendentes() { return pend.length; },
  };
}

// --- F-DF-01 ---------------------------------------------------------------

test('várias listas no mesmo quadro aplicam só a última', () => {
  const q = quadros();
  let items = [];
  const aplicadas = [];
  const fila = criarFilaPorQuadro(() => items, (l) => { items = l; aplicadas.push(l); }, q.agendar);

  for (let i = 0; i < 300; i++) fila.lista([{ id: 'a', pagina: i }]);
  assert.equal(aplicadas.length, 0, 'nada aplicado antes do quadro');
  assert.equal(q.pendentes, 1, 'um único quadro agendado');

  q.rodar();
  assert.equal(aplicadas.length, 1);
  assert.deepEqual(items, [{ id: 'a', pagina: 299 }]);
});

test('itens do mesmo quadro se combinam com a lista mais recente', () => {
  const q = quadros();
  let items = [{ id: 'a', n: 0 }, { id: 'b', n: 0 }];
  let aplicacoes = 0;
  const fila = criarFilaPorQuadro(() => items, (l) => { items = l; aplicacoes++; }, q.agendar);

  fila.item({ id: 'a', n: 1 });
  fila.item({ id: 'a', n: 2 });
  fila.item({ id: 'b', n: 5 });
  fila.item({ id: 'x', n: 9 }); // desconhecido: ignorado
  q.rodar();
  assert.equal(aplicacoes, 1);
  assert.deepEqual(items, [{ id: 'a', n: 2 }, { id: 'b', n: 5 }]);

  // Item anterior à lista é descartado; item posterior vale sobre ela.
  fila.item({ id: 'a', n: 3 });
  fila.lista([{ id: 'a', n: 10 }, { id: 'c', n: 0 }]);
  fila.item({ id: 'c', n: 1 });
  q.rodar();
  assert.deepEqual(items, [{ id: 'a', n: 10 }, { id: 'c', n: 1 }]);
});

test('lista nula vira vazia e cancelar descarta o quadro pendente', () => {
  const q = quadros();
  let items = [{ id: 'a' }];
  let cancelado = 0;
  const fila = criarFilaPorQuadro(() => items, (l) => { items = l; }, q.agendar, () => { cancelado++; });
  fila.lista(null);
  q.rodar();
  assert.deepEqual(items, []);

  fila.lista([{ id: 'z' }]);
  fila.cancelar();
  assert.equal(cancelado, 1);
  q.rodar();
  assert.deepEqual(items, []);
});

test('App encaminha queue:update e queue:item pelo agendador por quadro', () => {
  const ini = app.indexOf("EventsOn('queue:update'");
  const fim = app.indexOf("EventsOn('queue:status'");
  assert.ok(ini >= 0 && fim > ini);
  const trecho = app.slice(ini, fim);
  assert.doesNotMatch(trecho, /items\s*=/, 'eventos não podem substituir items direto');
  assert.match(trecho, /filaQuadro\.lista\(/);
  assert.match(trecho, /filaQuadro\.item\(/);
  assert.match(app, /filaQuadro\.cancelar\(\)/, 'onDestroy cancela o quadro pendente');
});

// --- F-DF-02 ---------------------------------------------------------------

test('Settings não lê a config: recebe do App por bind', () => {
  assert.doesNotMatch(settings, /GetConfig/);
  assert.match(settings, /export let config\b/);
  assert.match(app, /<Settings\b[^>]*bind:config/);
  const chamadas = [...app.matchAll(/GetConfig\(\)/g)];
  assert.equal(chamadas.length, 1, 'uma única leitura, no App');
});
