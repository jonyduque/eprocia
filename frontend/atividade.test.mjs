import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { rotuloEstadoDoc, LEITURA_NAO_LIDO, OPCOES_OCR, OPCOES_LIMPEZA, nivelOpcao, rotuloOpcao, ordenarDocs, resumoAtividade, prepararDocs, LIMITE_AGUARDANDO, segmentos } from './src/lib/atividade.js';

describe('opções', () => {
  it('higienização tem quatro níveis: Nenhuma, Básica, Padrão, Máxima', () => {
    assert.deepEqual(OPCOES_LIMPEZA.map((o) => o.valor), ['none', 'basic', 'standard', 'aggressive']);
    assert.equal(rotuloOpcao(OPCOES_LIMPEZA, 'standard'), 'Padrão');
    assert.equal(rotuloOpcao(OPCOES_LIMPEZA, 'aggressive'), 'Máxima');
    assert.equal(nivelOpcao(OPCOES_LIMPEZA, 'aggressive'), 3);
  });
  it('nível e rótulo', () => {
    assert.equal(nivelOpcao(OPCOES_OCR, 'best'), 3);
    assert.equal(nivelOpcao(OPCOES_OCR, 'x'), 0);
    assert.equal(rotuloOpcao(OPCOES_LIMPEZA, 'basic'), 'Básica');
    assert.equal(nivelOpcao(OPCOES_LIMPEZA, 'standard'), 2);
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
    assert.deepEqual(resumoAtividade({ docs }), { ativos: 3, emOcr: 2, aguardando: 1, concluidos: 1, erros: 1, paginasOcr: 4, naoLidos: 0, semOCR: 0 });
  });
  it('prepararDocs conta igual a resumoAtividade', () => {
    const docs = ['sem_ocr', 'senha', 'ilegivel', 'prazo', 'ocr_falhou', 'sem_texto'].map((leitura, i) => ({ nome: 'n' + i, estado: 'erro', leitura }));
    docs.push({ nome: 'e', estado: 'erro' }, { nome: 'o1', estado: 'ocr', ocr: 3 }, { nome: 'o2', estado: 'ocr', ocr: 2 },
      { nome: 'l', estado: 'lendo' }, { nome: 'c', estado: 'concluido', leitura: 'lido' });
    for (let i = 0; i < LIMITE_AGUARDANDO + 5; i++) docs.push({ nome: 'a' + i, estado: 'aguardando' });
    const inst = { docs };
    assert.deepEqual(prepararDocs(inst).resumo, resumoAtividade(inst));
    assert.deepEqual(prepararDocs(inst, { expandirAguardando: true, mostrarConcluidos: true }).resumo, resumoAtividade(inst));
  });
  it('segmentos do medidor', () => {
    assert.deepEqual(segmentos(2, 4), [true, true, false, false]);
    assert.equal(segmentos(10, 32).length, 16);
    assert.equal(segmentos(16, 32).filter(Boolean).length, 8);
    assert.deepEqual(segmentos(0, 0), [false]);
  });
});

describe('rotuloEstadoDoc (leitor de tela)', () => {
  it('documento não lido anuncia "Não lido", nunca "Erro"', () => {
    for (const leitura of LEITURA_NAO_LIDO) {
      assert.equal(rotuloEstadoDoc({ estado: 'erro', leitura }), 'Não lido');
    }
  });
  it('erro de verdade, outros estados e estado desconhecido', () => {
    assert.equal(rotuloEstadoDoc({ estado: 'erro' }), 'Erro');
    assert.equal(rotuloEstadoDoc({ estado: 'concluido' }), 'Concluído');
    assert.equal(rotuloEstadoDoc({ estado: 'ocr' }), 'Em OCR');
    assert.equal(rotuloEstadoDoc({ estado: 'xyz' }), 'xyz');
  });
  it('AtividadeItem usa rotuloEstadoDoc(d) no sr-only e não o ROTULO local', () => {
    const src = readFileSync(new URL('./src/components/AtividadeItem.svelte', import.meta.url), 'utf8');
    assert.match(src, /<span class="sr-only">\{rotuloEstadoDoc\(d\)\}:<\/span>/);
    assert.doesNotMatch(src, /ROTULO\[d\.estado\]/);
  });
});
