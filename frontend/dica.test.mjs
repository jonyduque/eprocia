// F-AC-16: dica própria dos botões de ícone da fila (o title nativo demora e
// nunca aparece no foco por teclado).
// Rodar com: node --test frontend/dica.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { posicaoDica, MARGEM_DICA, DISTANCIA_DICA } from './src/lib/dica.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, 'src', ...p), 'utf8').replace(/\r\n/g, '\n');
const fonte = ler('lib', 'dica.js');
const lista = ler('components', 'ProcessList.svelte');

const alvo = { left: 100, right: 120, top: 50, bottom: 70, width: 20, height: 20 };

test('posicaoDica centraliza abaixo do alvo', () => {
  const p = posicaoDica(alvo, 60, 20, 800, 600);
  assert.deepEqual(p, { left: 80, top: 70 + DISTANCIA_DICA });
});

test('posicaoDica sobe quando não cabe abaixo e prende nas bordas', () => {
  const baixo = { ...alvo, top: 580, bottom: 598 };
  assert.equal(posicaoDica(baixo, 60, 20, 800, 600).top, 580 - DISTANCIA_DICA - 20);
  const direita = { ...alvo, left: 780, right: 798 };
  assert.equal(posicaoDica(direita, 100, 20, 800, 600).left, 800 - MARGEM_DICA - 100);
  const esquerda = { ...alvo, left: 0, right: 10 };
  assert.equal(posicaoDica(esquerda, 100, 20, 800, 600).left, MARGEM_DICA);
});

test('dica: um único elemento na camada superior, hover com atraso e foco por teclado sem atraso', () => {
  assert.match(fonte, /popover\s*=\s*'manual'|setAttribute\('popover', 'manual'\)/);
  assert.match(fonte, /showPopover\(\)/);
  assert.match(fonte, /hidePopover\(\)/);
  assert.match(fonte, /'pointerenter'/);
  assert.match(fonte, /setTimeout\(\(\) => mostrar\(no, atual\), DUR_FAST\)/);
  assert.match(fonte, /matches\(':focus-visible'\)/);
  assert.match(fonte, /'Escape'/);
  assert.match(fonte, /duracaoMovimento\(DUR_FAST\)/);
  // Um só elemento para a página inteira, criado na primeira vez.
  assert.equal((fonte.match(/document\.createElement\(/g) ?? []).length, 1);
});

test('estilo da dica segue os tokens e não anima com movimento reduzido', () => {
  const i = lista.indexOf(':global(.dica) {');
  assert.ok(i >= 0, 'regra :global(.dica) ausente');
  const r = lista.slice(i, lista.indexOf('}', i));
  assert.match(r, /font-size:\s*var\(--fs-meta\)/);
  assert.match(r, /background-color:\s*var\(--surface\)/);
  assert.match(r, /border:\s*1px solid var\(--border-strong\)/);
  assert.match(r, /box-shadow:\s*var\(--shadow-sm\)/);
  assert.match(r, /transition:\s*opacity var\(--dur-fast\) var\(--ease-out\)/);
  assert.match(lista, /:global\(\.dica\.visivel\)\s*\{[^}]*opacity:\s*1/);
  assert.match(lista, /prefers-reduced-motion: reduce\)\s*\{[^@]*:global\(\.dica\)\s*\{[^}]*transition:\s*none/);
});
