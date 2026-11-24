import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { AJUDA_FORCAR_OCR_SEM_OCR } from './src/lib/textos.js';

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
  it('interruptor ligado a config.forcar_ocr, como o do autostart', () => {
    assert.ok(caixa, 'falta o input ligado a config.forcar_ocr');
    assert.match(caixa, /type="checkbox"/);
    assert.match(caixa, /role="switch"/);
    assert.match(caixa, /bind:checked=\{config\.forcar_ocr\}/);
    // Rótulo à esquerda (label for) e o switch à direita, numa .toggle-group.
    const grupo = settings.match(/<div class="toggle-group">\s*<label for="forcar-ocr">[\s\S]*?<\/div>/)?.[0] ?? '';
    assert.ok(grupo, 'falta a .toggle-group com o rótulo à esquerda');
    assert.match(grupo, /\{ROTULO_FORCAR_OCR_CONFIG\}[\s\S]*<label class="switch">[\s\S]*id="forcar-ocr"[\s\S]*<span class="slider" aria-hidden="true"><\/span>/);
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
    assert.match(settings, /\bAJUDA_FORCAR_OCR_CONFIG\b/);
    assert.match(caixa, /aria-describedby="ajuda-forcar-ocr"/);
  });
  it('com OCR desligado a ajuda diz por que não tem efeito, no mesmo id', () => {
    assert.match(
      settings,
      /<p class="help-text" id="ajuda-forcar-ocr">\{config\.ocr_mode === 'none' \? AJUDA_FORCAR_OCR_SEM_OCR : AJUDA_FORCAR_OCR_CONFIG\}<\/p>/
    );
  });
});

describe('AJUDA_FORCAR_OCR_SEM_OCR', () => {
  it('texto exato', () => {
    assert.equal(AJUDA_FORCAR_OCR_SEM_OCR, 'Sem efeito com o OCR desligado; escolha um modo de OCR para usar.');
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
    assert.match(reprocessar, /\bAJUDA_FORCAR_OCR_REPROCESSAR\b/);
  });
  it('com OCR desligado a ajuda diz por que não tem efeito, no mesmo id', () => {
    assert.match(caixa, /aria-describedby="\{id\}-ajuda-forcar"/);
    assert.match(
      reprocessar,
      /id="\{id\}-ajuda-forcar">\{ocr === 'none' \? AJUDA_FORCAR_OCR_SEM_OCR : AJUDA_FORCAR_OCR_REPROCESSAR\}<\/p>/
    );
  });
  it('no Reprocessar fica a caixa nativa, sem switch', () => {
    assert.doesNotMatch(caixa, /role="switch"/);
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
