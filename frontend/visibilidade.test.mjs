// F-AN-10: com a janela na bandeja, nenhuma animação nem renderização da fila.
// Rodar com: node --test frontend/visibilidade.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarFilaPorQuadro } from './src/lib/quadro.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, 'src', ...p), 'utf8').replace(/\r\n/g, '\n');
const app = ler('App.svelte');
const css = ler('assets', 'css', 'style.css');

function quadros() {
  const pend = [];
  return {
    agendar(fn) { pend.push(fn); return pend.length; },
    rodar() { const fns = pend.splice(0); fns.forEach((f) => f()); },
    get pendentes() { return pend.length; },
  };
}

test('fila pausada guarda só o último estado e aplica ao retomar', () => {
  const q = quadros();
  let items = [{ id: 'a', n: 0 }];
  let aplicacoes = 0;
  const fila = criarFilaPorQuadro(() => items, (l) => { items = l; aplicacoes++; }, q.agendar);

  fila.pausar();
  fila.lista([{ id: 'a', n: 1 }, { id: 'b', n: 0 }]);
  fila.item({ id: 'a', n: 2 });
  fila.lista([{ id: 'a', n: 3 }, { id: 'b', n: 0 }]);
  fila.item({ id: 'b', n: 4 });
  assert.equal(q.pendentes, 0, 'nenhum quadro agendado com a janela oculta');
  q.rodar();
  assert.equal(aplicacoes, 0);

  fila.retomar();
  assert.equal(q.pendentes, 1, 'um quadro ao reabrir');
  q.rodar();
  assert.equal(aplicacoes, 1);
  assert.deepEqual(items, [{ id: 'a', n: 3 }, { id: 'b', n: 4 }]);
});

test('retomar sem nada pendente não agenda quadro', () => {
  const q = quadros();
  const fila = criarFilaPorQuadro(() => [], () => {}, q.agendar);
  fila.pausar();
  fila.retomar();
  assert.equal(q.pendentes, 0);
});

test('App escuta a visibilidade da janela, marca :root.oculta e pausa a fila', () => {
  assert.match(app, /EventsOn\('janela:visivel'/);
  assert.match(app, /classList\.toggle\('oculta'/);
  assert.match(app, /filaQuadro\.pausar\(\)/);
  assert.match(app, /filaQuadro\.retomar\(\)/);
});

test('atividade recebida com a janela oculta fica guardada até reabrir', () => {
  assert.match(app, /atividadesOcultas/);
});

test(':root.oculta pausa toda animação, pseudo-elementos inclusive', () => {
  assert.match(css, /:root\.oculta \*,\s*:root\.oculta \*::before,\s*:root\.oculta \*::after\s*\{\s*animation-play-state: paused !important;/);
});
