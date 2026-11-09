import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const settings = readFileSync(new URL('./src/components/Settings.svelte', import.meta.url), 'utf8');
const app = readFileSync(new URL('./src/App.svelte', import.meta.url), 'utf8');
const lista = readFileSync(new URL('./src/components/ProcessList.svelte', import.meta.url), 'utf8');

describe('higienização em Settings.svelte', () => {
  const select = settings.match(/<select\s+id="cleaning-level"[\s\S]*?<\/select>/)?.[0] ?? '';
  it('select tem as 4 opções na ordem', () => {
    const opcoes = [...select.matchAll(/<option value="(\w+)">([^<]+)<\/option>/g)].map((m) => [m[1], m[2]]);
    assert.deepEqual(opcoes, [['none', 'Nenhuma'], ['basic', 'Básica'], ['standard', 'Padrão'], ['aggressive', 'Máxima']]);
  });
  it('padrão é standard', () => {
    assert.match(settings, /cleaning_level: 'standard'/);
  });
  it('ajuda de cada nível', () => {
    const ajuda = settings.match(/const AJUDA_LIMPEZA = \{[\s\S]*?\};/)?.[0] ?? '';
    assert.match(ajuda, /none: 'Texto como extraído'/);
    assert.match(ajuda, /basic: 'Remove espaços e linhas vazias'/);
    assert.match(ajuda, /standard: 'Remove cabeçalhos, rodapés e duplicatas; resume assinaturas'/);
    assert.match(ajuda, /aggressive: 'Padrão \+ ruído de OCR e parágrafos contínuos'/);
  });
});

describe('padrão fora das Configurações', () => {
  it('lista de processos usa standard como reserva', () => {
    assert.doesNotMatch(lista, /cleaning_level \|\| 'basic'/);
  });
  it('indicador da barra recolhida cobre os 4 níveis', () => {
    assert.match(app, /OPCOES_LIMPEZA\.slice\(1\)/);
  });
});
