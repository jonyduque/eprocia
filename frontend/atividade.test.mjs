import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { OPCOES_OCR, OPCOES_LIMPEZA, nivelOpcao, proximaOpcao, rotuloOpcao, ordenarDocs, resumoAtividade, segmentos } from './src/lib/atividade.js';

describe('opções', () => {
  it('cicla e volta ao início', () => {
    assert.equal(proximaOpcao(OPCOES_OCR, 'normal').valor, 'best');
    assert.equal(proximaOpcao(OPCOES_OCR, 'best').valor, 'none');
    assert.equal(proximaOpcao(OPCOES_LIMPEZA, 'aggressive').valor, 'none');
    assert.equal(proximaOpcao(OPCOES_LIMPEZA, 'desconhecido').valor, 'none');
  });
  it('nível e rótulo', () => {
    assert.equal(nivelOpcao(OPCOES_OCR, 'best'), 3);
    assert.equal(nivelOpcao(OPCOES_OCR, 'x'), 0);
    assert.equal(rotuloOpcao(OPCOES_LIMPEZA, 'basic'), 'Básica');
  });
});

describe('atividade', () => {
  const docs = [
    { nome: 'a', estado: 'concluido' },
    { nome: 'b', estado: 'aguardando' },
    { nome: 'c', estado: 'ocr', ocr: 3 },
    { nome: 'd', estado: 'lendo' },
    { nome: 'e', estado: 'erro' },
    { nome: 'f', estado: 'ocr', ocr: 1 }
  ];
  it('ordena quem trabalha primeiro, mantendo a ordem do ZIP', () => {
    assert.deepEqual(ordenarDocs(docs).map((d) => d.nome), ['c', 'f', 'd', 'e', 'b', 'a']);
    assert.deepEqual(ordenarDocs(null), []);
  });
  it('resume', () => {
    assert.deepEqual(resumoAtividade({ docs }), { ativos: 3, emOcr: 2, aguardando: 1, concluidos: 1, erros: 1, paginasOcr: 4 });
  });
  it('segmentos do medidor', () => {
    assert.deepEqual(segmentos(2, 4), [true, true, false, false]);
    assert.equal(segmentos(10, 32).length, 16);
    assert.equal(segmentos(16, 32).filter(Boolean).length, 8);
    assert.deepEqual(segmentos(0, 0), [false]);
  });
});
