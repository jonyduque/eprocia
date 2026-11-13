import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { rotuloNaoLidos, avisoCopiaNaoLidos } from './src/lib/naoLidos.js';
import { resumoAtividade } from './src/lib/atividade.js';

const fonte = (p) => readFileSync(new URL(p, import.meta.url), 'utf8');

describe('naoLidos', () => {
  it('rótulo da linha', () => {
    assert.equal(rotuloNaoLidos(0), '');
    assert.equal(rotuloNaoLidos(undefined), '');
    assert.equal(rotuloNaoLidos(1), '1 não lido');
    assert.equal(rotuloNaoLidos(3), '3 não lidos');
  });
  it('aviso ao copiar', () => {
    assert.equal(avisoCopiaNaoLidos(0), '');
    assert.equal(avisoCopiaNaoLidos(1), 'Copiado. Atenção: 1 documento não foi lido; o texto está incompleto.');
    assert.equal(avisoCopiaNaoLidos(2), 'Copiado. Atenção: 2 documentos não foram lidos; o texto está incompleto.');
  });
  it('resumo conta não lidos pela leitura, não pelo estado', () => {
    const r = resumoAtividade({ docs: [
      { estado: 'concluido', leitura: 'lido' },
      { estado: 'concluido', leitura: 'ocr' },
      { estado: 'erro', leitura: 'sem_ocr' },
      { estado: 'erro', leitura: 'senha' }
    ] });
    assert.equal(r.naoLidos, 2);
    assert.equal(r.semOCR, 1);
  });
  it('sem_texto conta como não lido, não como sem OCR nem erro', () => {
    const r = resumoAtividade({ docs: [
      { estado: 'concluido', leitura: 'lido' },
      { estado: 'concluido', leitura: 'sem_texto' }
    ] });
    assert.equal(r.naoLidos, 1);
    assert.equal(r.semOCR, 0);
    assert.equal(r.erros, 0);
  });
  it('componentes usam as funções e o botão', () => {
    assert.match(fonte('./src/components/ProcessList.svelte'), /rotuloNaoLidos\(/);
    assert.match(fonte('./src/components/ProcessList.svelte'), /avisoCopiaNaoLidos\(/);
    assert.match(fonte('./src/components/AtividadeItem.svelte'), /Reprocessar com OCR/);
  });
});
