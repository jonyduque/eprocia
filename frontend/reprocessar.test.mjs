import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { aplicarReprocessarComOCR, ESTADO_REPROCESSAR_INICIAL, argumentosLerComOCR } from './src/lib/reprocessar.js';

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
  it('Reprocessar envia o Forçar OCR como quarto argumento do ReprocessQueueItem', () => {
    const f = readFileSync(new URL('./src/components/ProcessList.svelte', import.meta.url), 'utf8');
    const corpo = f.slice(f.indexOf('async function confirmarReprocessar'));
    assert.match(corpo.slice(0, 300), /ReprocessQueueItem\(item\.id, ocr, limpeza, !!forcar\)/);
    const r = readFileSync(new URL('./src/components/ReprocessarItem.svelte', import.meta.url), 'utf8');
    assert.match(r, /aoConfirmar\(ocr, limpeza, forcar\)/);
  });
});

describe('argumentosLerComOCR', () => {
  it('global none dá normal', () => {
    const a = argumentosLerComOCR({ cleaning_level: 'basic' }, { ocr_mode: 'none', cleaning_level: 'standard' });
    assert.equal(a.ocr, 'normal');
  });
  it('global best dá best', () => {
    const a = argumentosLerComOCR({}, { ocr_mode: 'best', cleaning_level: 'standard' });
    assert.equal(a.ocr, 'best');
  });
  it('forcar é sempre false, mesmo com a global ligada ou o item forçado', () => {
    assert.equal(argumentosLerComOCR({}, { ocr_mode: 'fast', forcar_ocr: true }).forcar, false);
    assert.equal(argumentosLerComOCR({ forcar_ocr: true }, { ocr_mode: 'none', forcar_ocr: true }).forcar, false);
  });
  it('mantém o nível de limpeza do item; sem ele, o global', () => {
    assert.equal(argumentosLerComOCR({ cleaning_level: 'aggressive' }, { ocr_mode: 'normal', cleaning_level: 'basic' }).limpeza, 'aggressive');
    assert.equal(argumentosLerComOCR({}, { ocr_mode: 'normal', cleaning_level: 'basic' }).limpeza, 'basic');
  });
  it('não usa o modo de OCR do item (o item ficou sem OCR)', () => {
    assert.equal(argumentosLerComOCR({ ocr_mode: 'none' }, { ocr_mode: 'fast' }).ocr, 'fast');
  });
  it('item comum (sem ajuste) usa o nível em que rodou, mesmo com a global mudada depois', () => {
    const item = { relatorio: { limpeza: { nivel: 'basic' } } };
    assert.equal(argumentosLerComOCR(item, { ocr_mode: 'normal', cleaning_level: 'aggressive' }).limpeza, 'basic');
    // O ajuste do Reprocessar vale mais que o relatório.
    assert.equal(argumentosLerComOCR({ ...item, cleaning_level: 'none' }, { cleaning_level: 'aggressive' }).limpeza, 'none');
  });
  it('config ausente não lança', () => {
    assert.deepEqual(argumentosLerComOCR({}, null), { ocr: 'normal', limpeza: 'standard', forcar: false });
  });
});
