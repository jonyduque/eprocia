import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { ESTADOS, instalarStubs, listaAltoContraste } from './estados.mjs';

// Fields of etl.QueueItem (internal/etl/queue.go) the UI reads.
const CAMPOS = ['id', 'name', 'path', 'status', 'progress', 'message', 'output_path', 'docs_count', 'pages_count', 'estimated_tokens',
  'processed_docs', 'total_docs', 'processed_pages', 'total_pages', 'timestamp', 'unread_docs'];
const STATUS = new Set(['pending', 'processing', 'paused', 'cancelled', 'success', 'error']);

describe('estados visuais', () => {
  it('cobre os estados da review.json', () => {
    assert.deepEqual(Object.keys(ESTADOS).sort(), ['configuracoes', 'erro', 'fila', 'nao-lidos', 'processando', 'vazio']);
  });

  for (const [nome, e] of Object.entries(ESTADOS)) {
    it(`${nome}: itens com o formato de etl.QueueItem`, () => {
      for (const item of e.fila) {
        for (const c of CAMPOS) assert.ok(c in item, `${nome}: campo ${c} ausente`);
        assert.ok(STATUS.has(item.status), `${nome}: status ${item.status}`);
      }
      assert.equal(new Set(e.fila.map((i) => i.id)).size, e.fila.length, 'ids repetidos');
    });

    it(`${nome}: só dados fictícios`, () => {
      const texto = JSON.stringify(e);
      for (const m of texto.matchAll(/\d{7}-\d{2}\.\d{4}\.\d\.\d{2}\.\d{4}/g)) {
        assert.match(m[0], /^5000000-00\.2026\.4\.00\./, `número de processo não fictício: ${m[0]}`);
      }
      assert.doesNotMatch(texto, /jonyd|Users\\\\[a-z]+\\\\/i, 'caminho de usuário real');
    });
  }

  it('stubs servem o estado e ignoram chamadas desconhecidas', async () => {
    const store = {};
    globalThis.window = globalThis;
    // Node 24 exposes its own localStorage getter; shadow it for the test.
    const original = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
    Object.defineProperty(globalThis, 'localStorage', { value: { setItem: (k, v) => { store[k] = v; } }, configurable: true });
    try {
      instalarStubs({ estado: ESTADOS.fila, tema: 'claro' });
      assert.equal(store.theme, 'light');
      assert.equal((await window.go.main.App.GetQueue()).length, ESTADOS.fila.fila.length);
      assert.equal(await window.go.main.App.PauseQueue(), undefined);
      assert.equal(await window.go.main.App.GetQueueActivity('x'), null);
      assert.equal(typeof window.runtime.EventsOnMultiple('x', () => {}, -1), 'function');
      assert.equal(window.runtime.WindowMinimise(), undefined);
    } finally {
      delete globalThis.go;
      delete globalThis.runtime;
      delete globalThis.localStorage;
      if (original) Object.defineProperty(globalThis, 'localStorage', original);
    }
  });
});

describe('lista de alto contraste', () => {
  const review = JSON.parse(readFileSync(new URL('../../.claude/review.json', import.meta.url), 'utf8'));

  it('toda entrada do review.json é um estado', () => {
    const lista = listaAltoContraste(review);
    assert.ok(lista.length > 0);
    for (const e of lista) assert.ok(e in ESTADOS, `estado desconhecido: ${e}`);
  });

  it('config falso: lista presente, ausente e chave visual ausente', () => {
    assert.deepEqual(listaAltoContraste({ visual: { alto_contraste: ['fila'] } }), ['fila']);
    assert.deepEqual(listaAltoContraste({ visual: {} }), []);
    assert.deepEqual(listaAltoContraste({}), []);
  });

  it('capturar.mjs não tem a lista literal e lê alto_contraste', () => {
    const src = readFileSync(new URL('./capturar.mjs', import.meta.url), 'utf8');
    assert.doesNotMatch(src, /ALTO_CONTRASTE\s*=\s*\[/);
    assert.match(src, /listaAltoContraste/);
    assert.match(src, /review\.json/);
  });
});
