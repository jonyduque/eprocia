// F-AN-01: toast de status com entrada/saída simétricas e timer único.
// Rodar com: node --test frontend/aviso.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarAviso, duracaoMovimento, DUR_FAST, DUR_BASE } from './src/lib/aviso.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const settings = readFileSync(join(raiz, 'src', 'components', 'Settings.svelte'), 'utf8');

function relogio() {
  let agora = 0;
  let prox = 1;
  const pend = new Map();
  return {
    setTimeout(fn, ms) { const id = prox++; pend.set(id, { fn, em: agora + ms }); return id; },
    clearTimeout(id) { pend.delete(id); },
    avancar(ms) {
      agora += ms;
      for (const [id, t] of [...pend].sort((a, b) => a[1].em - b[1].em)) {
        if (t.em <= agora) { pend.delete(id); t.fn(); }
      }
    },
    get pendentes() { return pend.size; },
  };
}

test('mensagem nova não é apagada pelo timer da anterior', () => {
  const r = relogio();
  let atual = null;
  const aviso = criarAviso((m) => { atual = m; }, 3000, r);
  aviso.mostrar('primeira', 'success');
  r.avancar(2000);
  aviso.mostrar('segunda', 'success');
  r.avancar(1500);
  assert.deepEqual(atual, { texto: 'segunda', tipo: 'success' });
  assert.equal(r.pendentes, 1);
  r.avancar(1500);
  assert.equal(atual, null);
  assert.equal(r.pendentes, 0);
});

test('limpar no desmonte cancela o timer pendente', () => {
  const r = relogio();
  let chamadas = 0;
  const aviso = criarAviso(() => { chamadas++; }, 3000, r);
  aviso.mostrar('x', 'success');
  aviso.limpar();
  assert.equal(r.pendentes, 0);
  r.avancar(5000);
  assert.equal(chamadas, 1);
});

test('durações seguem os tokens e zeram com reduced-motion', () => {
  assert.equal(DUR_FAST, 150);
  assert.equal(DUR_BASE, 220);
  assert.equal(duracaoMovimento(DUR_BASE, { matchMedia: () => ({ matches: false }) }), 220);
  assert.equal(duracaoMovimento(DUR_BASE, { matchMedia: () => ({ matches: true }) }), 0);
  assert.equal(duracaoMovimento(DUR_FAST, {}), 150);
});

test('Settings usa fly na entrada, fade na saída e limpa o timer no desmonte', () => {
  assert.match(settings, /in:fly=\{\{[^}]*y:\s*8[^}]*\}\}/);
  assert.match(settings, /out:fade=\{\{[^}]*\}\}/);
  assert.match(settings, /onDestroy\(/);
  assert.doesNotMatch(settings, /setTimeout\(/);
  assert.doesNotMatch(settings, /slideUp/);
});
