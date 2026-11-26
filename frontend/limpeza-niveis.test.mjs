import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { OPCOES_LIMPEZA } from './src/lib/atividade.js';

const settings = readFileSync(new URL('./src/components/Settings.svelte', import.meta.url), 'utf8');
const app = readFileSync(new URL('./src/App.svelte', import.meta.url), 'utf8');
const lista = readFileSync(new URL('./src/components/ProcessList.svelte', import.meta.url), 'utf8');

describe('higienização em Settings.svelte', () => {
  const controle = settings.match(/<Segmentado\s+id="cleaning-level"[\s\S]*?<\/Segmentado>/)?.[0] ?? '';
  it('controle segmentado tem as 4 opções na ordem', () => {
    assert.match(controle, /opcoes=\{OPCOES_LIMPEZA\}/);
    const opcoes = OPCOES_LIMPEZA.map((o) => [o.valor, o.rotulo]);
    assert.deepEqual(opcoes, [['none', 'Nenhuma'], ['basic', 'Básica'], ['standard', 'Padrão'], ['aggressive', 'Máxima']]);
  });
  it('padrão é standard', () => {
    assert.match(settings, /cleaning_level: 'standard'/);
  });
  it('ajuda de cada nível', () => {
    const ajuda = Object.fromEntries(OPCOES_LIMPEZA.map((o) => [o.valor, o.descricao]));
    assert.equal(ajuda.none, 'Texto como extraído');
    assert.equal(ajuda.basic, 'Remove espaços e linhas vazias');
    assert.equal(ajuda.standard, 'Remove cabeçalhos, rodapés e duplicatas; resume assinaturas');
    assert.equal(ajuda.aggressive, 'Padrão + ruído de OCR e parágrafos contínuos');
    assert.match(settings, /descricaoOpcao\(OPCOES_LIMPEZA, previaLimpeza \?\? config\.cleaning_level\)/);
  });
});

describe('padrão fora das Configurações', () => {
  it('lista de processos usa standard como reserva', () => {
    assert.doesNotMatch(lista, /cleaning_level \|\| 'basic'/);
  });
  it('menu da barra recolhida cobre os 4 níveis', () => {
    assert.match(app, /<MenuNivel[^>]*opcoes=\{OPCOES_LIMPEZA\}/);
  });
});

describe('campo de tipos omitidos', () => {
  it('relê a config gravada quando o salvamento falha', () => {
    assert.match(settings, /await aoFalharSalvar\(\);[\s\S]{0,160}omitirCarregado = false;/);
  });
});
