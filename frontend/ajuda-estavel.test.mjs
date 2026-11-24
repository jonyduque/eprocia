// Ajustes do dono, rodada 2 (2026-11-24): a ajuda de um segmentado não muda de
// altura com o hover. Antes, a descrição sob o ponteiro trocava o texto da
// ajuda; com a rolagem no fim do painel, a altura nova movia o controle para
// fora do ponteiro, o hover voltava e as Configurações "tremiam".
// Rodar com: node --test frontend/ajuda-estavel.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8').replace(/\r\n/g, '\n');
const settings = ler('src', 'components', 'Settings.svelte');

function estilo(texto) {
  const i = texto.indexOf('<style>');
  return texto.slice(i, texto.indexOf('</style>', i));
}

function regra(css, seletor) {
  const i = css.indexOf(seletor + ' {');
  assert.ok(i >= 0, `${seletor} ausente`);
  return css.slice(i, css.indexOf('}', i));
}

// O <p> da ajuda, do início da tag até o fechamento.
function ajuda(id) {
  const i = settings.indexOf(`id="${id}"`);
  assert.ok(i >= 0, `${id} ausente`);
  const ini = settings.lastIndexOf('<p', i);
  return settings.slice(ini, settings.indexOf('</p>', i) + 4);
}

for (const [id, opcoes, previa, valor] of [
  ['ajuda-ocr', 'OPCOES_OCR', 'previaOcr', 'config.ocr_mode'],
  ['ajuda-limpeza', 'OPCOES_LIMPEZA', 'previaLimpeza', 'config.cleaning_level'],
]) {
  test(`${id}: descrição visível e todas as descrições empilhadas na mesma célula`, () => {
    const p = ajuda(id);
    assert.match(p, /class="help-text ajuda-pilha"/);
    // A descrição ativa (a da prévia sob o ponteiro ou a gravada) é a única visível.
    assert.match(p, new RegExp(`<span>\\{descricaoOpcao\\(${opcoes}, ${previa} \\?\\? ${valor.replace('.', '\\.')}\\)\\}</span>`));
    // Cada descrição ocupa a célula, escondida e fora da árvore de acessibilidade:
    // a célula fica com a altura da mais longa, sem número mágico de pixels.
    assert.match(p, new RegExp(`\\{#each ${opcoes} as o \\(o\\.valor\\)\\}<span class="reserva" aria-hidden="true">\\{o\\.descricao\\}</span>\\{/each\\}`));
  });
}

test('pilha da ajuda: grid de uma célula, reservas invisíveis', () => {
  const css = estilo(settings);
  assert.match(regra(css, '.ajuda-pilha'), /display:\s*grid/);
  assert.match(regra(css, '.ajuda-pilha > span'), /grid-area:\s*1\s*\/\s*1/);
  assert.match(regra(css, '.reserva'), /visibility:\s*hidden/);
  assert.doesNotMatch(regra(css, '.ajuda-pilha'), /min-height|height:\s*\d/);
});
