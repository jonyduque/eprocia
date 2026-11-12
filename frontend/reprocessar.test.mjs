import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { aplicarReprocessarComOCR, ESTADO_REPROCESSAR_INICIAL } from './src/lib/reprocessar.js';

describe('reprocessar com OCR', () => {
  it('painel fechado: abre com OCR normal', () => {
    const r = aplicarReprocessarComOCR(ESTADO_REPROCESSAR_INICIAL);
    assert.equal(r.aberto, true);
    assert.equal(r.ocr, 'normal');
  });
  it('painel já aberto pelo caminho normal: remonta com OCR normal', () => {
    const antes = { aberto: true, ocr: '', geracao: 0 };
    const r = aplicarReprocessarComOCR(antes);
    assert.equal(r.aberto, true);
    assert.equal(r.ocr, 'normal');
    assert.notEqual(r.geracao, antes.geracao, 'a chave precisa mudar para o componente remontar');
  });
  it('segundo clique depois de o usuário trocar o select: muda a chave de novo', () => {
    const primeiro = aplicarReprocessarComOCR(ESTADO_REPROCESSAR_INICIAL);
    const segundo = aplicarReprocessarComOCR(primeiro);
    assert.equal(segundo.ocr, 'normal');
    assert.notEqual(segundo.geracao, primeiro.geracao);
  });
  it('não muta o estado recebido', () => {
    const e = Object.freeze({ aberto: false, ocr: '', geracao: 3 });
    assert.equal(aplicarReprocessarComOCR(e).geracao, 4);
  });
  it('ProcessList usa a função e a chave', () => {
    const f = readFileSync(new URL('./src/components/ProcessList.svelte', import.meta.url), 'utf8');
    assert.match(f, /aplicarReprocessarComOCR\(/);
    assert.match(f, /\{#key /);
  });
});
