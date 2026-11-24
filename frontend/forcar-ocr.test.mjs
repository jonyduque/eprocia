import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const ler = (p) => readFileSync(new URL(p, import.meta.url), 'utf8');
const settings = ler('./src/components/Settings.svelte');
const reprocessar = ler('./src/components/ReprocessarItem.svelte');
const lista = ler('./src/components/ProcessList.svelte');

// A tag <input ... /> que tem o atributo indicado (o "=>" dos handlers não fecha a tag).
function inputCom(fonte, atributo) {
  const tags = fonte.match(/<input\b[\s\S]*?\/>/g) ?? [];
  return tags.find((t) => new RegExp(atributo).test(t)) ?? '';
}

describe('Forçar OCR nas Configurações', () => {
  const caixa = inputCom(settings, 'config\\.forcar_ocr');
  it('caixa ligada a config.forcar_ocr', () => {
    assert.ok(caixa, 'falta o input ligado a config.forcar_ocr');
    assert.match(caixa, /type="checkbox"/);
    assert.match(caixa, /bind:checked=\{config\.forcar_ocr\}/);
  });
  it('desabilitada com ocr_mode none, sem mexer no valor', () => {
    assert.match(caixa, /disabled=\{config\.ocr_mode === 'none'\}/);
    assert.doesNotMatch(settings, /config\.forcar_ocr\s*=\s*false/);
  });
  it('salva por SaveConfig como os outros campos', () => {
    assert.match(caixa, /on:change=\{\(\) => handleSave\('forcar'\)\}/);
    assert.match(settings, /forcar: ROTULO_FORCAR_OCR_CONFIG/);
  });
  it('rótulo e ajuda vêm de textos.js', () => {
    assert.match(settings, /\{ROTULO_FORCAR_OCR_CONFIG\}/);
    assert.match(settings, /\{AJUDA_FORCAR_OCR_CONFIG\}/);
    assert.match(caixa, /aria-describedby="ajuda-forcar-ocr"/);
  });
});

describe('Forçar OCR no Reprocessar', () => {
  const caixa = inputCom(reprocessar, 'bind:checked=\\{forcar\\}');
  it('valor inicial vem de forcarInicial, falso por padrão', () => {
    assert.match(reprocessar, /export let forcarInicial = false;/);
    assert.match(reprocessar, /let forcar = forcarInicial;/);
  });
  it('caixa desabilitada com ocr none', () => {
    assert.ok(caixa, 'falta o input ligado a forcar');
    assert.match(caixa, /type="checkbox"/);
    assert.match(caixa, /disabled=\{ocr === 'none'\}/);
  });
  it('aoConfirmar recebe três argumentos', () => {
    assert.match(reprocessar, /await aoConfirmar\(ocr, limpeza, forcar\)/);
  });
  it('rótulo e ajuda vêm de textos.js', () => {
    assert.match(reprocessar, /\{ROTULO_FORCAR_OCR_REPROCESSAR\}/);
    assert.match(reprocessar, /\{AJUDA_FORCAR_OCR_REPROCESSAR\}/);
  });
});

describe('ProcessList repassa o Forçar OCR', () => {
  it('passa config.forcar_ocr como forcarInicial', () => {
    assert.match(lista, /forcarInicial=\{!!config\?\.forcar_ocr\}/);
  });
  it('repassa o terceiro argumento ao ReprocessQueueItem', () => {
    assert.match(lista, /aoConfirmar=\{\(ocr, limpeza, forcar\) => confirmarReprocessar\(item, ocr, limpeza, forcar\)\}/);
    assert.match(lista, /async function confirmarReprocessar\(item, ocr, limpeza, forcar\)/);
    assert.match(lista, /ReprocessQueueItem\(item\.id, ocr, limpeza, !!forcar\)/);
  });
});

describe('textos do Forçar OCR e do log vêm de textos.js', () => {
  for (const [nome, f] of [['Settings', settings], ['ReprocessarItem', reprocessar]]) {
    it(nome, () => {
      assert.doesNotMatch(f, /Forçar OCR/);
      assert.doesNotMatch(f, /Abrir pasta de logs/);
      assert.doesNotMatch(f, /telemetria ativos/);
      assert.match(f, /from '\.\.\/lib\/textos\.js'/);
    });
  }
});
