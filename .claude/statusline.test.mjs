import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { formatar, lerEstado } from './statusline.mjs';

describe('statusline', () => {
  const dir = mkdtempSync(path.join(tmpdir(), 'statusline-'));

  it('sem estado mostra só a branch', () => {
    assert.equal(formatar({ branch: 'main', estado: lerEstado(path.join(dir, 'nao-existe.json')) }), 'main');
  });

  it('JSON corrompido é ignorado', () => {
    const f = path.join(dir, 'ruim.json');
    writeFileSync(f, '{ ruim');
    assert.equal(lerEstado(f), null);
  });

  it('conta ids por status', () => {
    const estado = { fase: 'correcao', ids: { a: 'feito', b: 'feito', c: 'bloqueado', d: 'pendente', e: 'pulado' } };
    assert.equal(formatar({ branch: 'revisao/2026-10-15', estado }), 'revisao/2026-10-15 · revisão: correcao · ✓2 ✗1 …1 ↷1');
  });

  it('fase sem ids', () => {
    assert.equal(formatar({ branch: 'x', estado: { fase: 'lentes' } }), 'x · revisão: lentes');
  });
});
