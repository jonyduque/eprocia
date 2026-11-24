import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { criarAnunciador } from './src/lib/regiaoViva.js';

function montar() {
  const escritas = [];
  let liberar;
  const tick = () => new Promise((r) => { liberar = r; });
  const anunciar = criarAnunciador((t) => escritas.push(t), tick);
  return { escritas, anunciar, liberar: () => liberar() };
}

test('duas chamadas no mesmo tick saem juntas, numa escrita só', async () => {
  const m = montar();
  const a = m.anunciar('X: concluído, 2 documentos');
  const b = m.anunciar('2 itens correspondem ao filtro');
  assert.deepEqual(m.escritas, ['']);
  m.liberar();
  await Promise.all([a, b]);
  assert.deepEqual(m.escritas, ['', 'X: concluído, 2 documentos. 2 itens correspondem ao filtro']);
});

test('uma chamada sozinha escreve vazio e depois o texto', async () => {
  const m = montar();
  const p = m.anunciar('olá');
  m.liberar();
  await p;
  assert.deepEqual(m.escritas, ['', 'olá']);
});

test('texto vazio não escreve nada', async () => {
  const m = montar();
  await m.anunciar('');
  assert.deepEqual(m.escritas, []);
});

test('o mesmo texto depois de um ciclo escreve vazio e o texto de novo', async () => {
  const m = montar();
  let p = m.anunciar('igual');
  m.liberar();
  await p;
  p = m.anunciar('igual');
  m.liberar();
  await p;
  assert.deepEqual(m.escritas, ['', 'igual', '', 'igual']);
});

test('as promessas do mesmo tick resolvem só depois da escrita', async () => {
  const m = montar();
  const a = m.anunciar('a');
  const b = m.anunciar('b');
  let feito = false;
  Promise.all([a, b]).then(() => { feito = true; });
  await Promise.resolve();
  assert.equal(feito, false);
  m.liberar();
  await Promise.all([a, b]);
  assert.equal(m.escritas.at(-1), 'a. b');
});

test('nenhum componente define async function anunciar e todos importam criarAnunciador', () => {
  for (const f of ['src/App.svelte', 'src/components/ProcessList.svelte', 'src/components/Settings.svelte']) {
    const s = readFileSync(new URL(f, import.meta.url), 'utf8');
    assert.doesNotMatch(s, /async function anunciar/, f);
    assert.match(s, /import \{ criarAnunciador \} from '[^']*regiaoViva\.js'/, f);
    assert.match(s, /const anunciar = criarAnunciador\(\(t\) => \(anuncio = t\), tick\)/, f);
  }
});

test('um tick que falha não cala a região', async () => {
  const escritas = [];
  let falhar = true;
  const anunciar = criarAnunciador((t) => escritas.push(t), () => (falhar ? Promise.reject(new Error('x')) : Promise.resolve()));
  await anunciar('a').catch(() => {});
  falhar = false;
  await anunciar('b');
  assert.deepEqual(escritas, ['', 'a', '', 'b']);
});
