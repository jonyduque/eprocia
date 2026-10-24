import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { ESTADOS, instalarStubs } from './estados.mjs';

// Fields of etl.QueueItem (internal/etl/queue.go) the UI reads.
const CAMPOS = ['id', 'name', 'path', 'status', 'progress', 'message', 'output_path', 'docs_count', 'pages_count', 'estimated_tokens',
  'processed_docs', 'total_docs', 'processed_pages', 'total_pages', 'timestamp'];
const STATUS = new Set(['pending', 'processing', 'paused', 'cancelled', 'success', 'error']);

describe('estados visuais', () => {
  it('cobre os estados da review.json', () => {
    assert.deepEqual(Object.keys(ESTADOS).sort(), ['configuracoes', 'erro', 'fila', 'processando', 'vazio']);
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
