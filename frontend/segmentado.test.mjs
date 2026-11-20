import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { OPCOES_OCR, OPCOES_LIMPEZA } from './src/lib/atividade.js';

// F-VI-02: OCR e higienização como controle segmentado, sem <select> nativo.

const ler = (rel) => readFileSync(new URL(rel, import.meta.url), 'utf8').replace(/\r\n/g, '\n');
const caminhoSeg = new URL('./src/components/Segmentado.svelte', import.meta.url);

function estilo(texto) {
  const i = texto.indexOf('<style>');
  return texto.slice(i, texto.indexOf('</style>', i));
}

function regra(texto, seletor) {
  const esc = seletor.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const m = texto.match(new RegExp(`(?:^|[\\s}])${esc}\\s*\\{([^}]*)\\}`));
  assert.ok(m, `${seletor} ausente`);
  return m[1];
}

test('Segmentado.svelte: fieldset com legend e rádios nativos ocultos', () => {
  assert.ok(existsSync(caminhoSeg), 'Segmentado.svelte ausente');
  const seg = ler('./src/components/Segmentado.svelte');
  assert.match(seg, /<fieldset[^>]*aria-describedby=\{idAjuda/);
  assert.match(seg, /<legend/);
  assert.match(seg, /\{#each opcoes as o/);
  assert.match(seg, /<input[^>]*type="radio"[^>]*class="sr-only"/);
  assert.match(seg, /bind:group=\{valor\}/);
  assert.doesNotMatch(seg, /role="radiogroup"/, 'fieldset com rádios nativos dispensa ARIA manual');
});

test('Segmentado.svelte: faixa de 28px, contorno forte, selecionado em --accent-soft', () => {
  const s = estilo(ler('./src/components/Segmentado.svelte'));
  const faixa = regra(s, '.faixa');
  assert.match(faixa, /height:\s*28px/);
  assert.match(faixa, /border:\s*1px solid var\(--border-strong\)/);
  assert.match(faixa, /border-radius:\s*4px/);
  assert.match(regra(s, '.segmento + .segmento'), /border-left:\s*1px solid var\(--border\)/);
  const seg = regra(s, '.segmento');
  assert.match(seg, /font-size:\s*var\(--fs-meta\)/);
  assert.match(seg, /transition:\s*background-color var\(--dur-fast\) var\(--ease-out\)/);
  const sel = regra(s, '.segmento.selecionado');
  assert.match(sel, /background-color:\s*var\(--accent-soft\)/);
  assert.match(sel, /color:\s*var\(--accent-text\)/);
  assert.match(sel, /font-weight:\s*600/);
  assert.match(regra(s, '.segmento:has(input:focus-visible)'), /outline:\s*2px solid var\(--focus-ring\)/);
});

test('Settings usa Segmentado com OPCOES_OCR e OPCOES_LIMPEZA, sem select', () => {
  const settings = ler('./src/components/Settings.svelte');
  assert.doesNotMatch(settings, /<select|<option/);
  assert.match(settings, /import \{[^}]*OPCOES_OCR[^}]*\} from '\.\.\/lib\/atividade\.js'/);
  assert.match(settings, /<Segmentado[^>]*id="ocr-mode"[^>]*opcoes=\{OPCOES_OCR\}/);
  assert.match(settings, /<Segmentado[^>]*id="cleaning-level"[^>]*opcoes=\{OPCOES_LIMPEZA\}/);
  assert.match(settings, /bind:valor=\{config\.ocr_mode\}/);
  assert.match(settings, /bind:valor=\{config\.cleaning_level\}/);
});

test('ReprocessarItem usa o mesmo Segmentado, sem select', () => {
  const rep = ler('./src/components/ReprocessarItem.svelte');
  assert.doesNotMatch(rep, /<select|<option/);
  assert.match(rep, /<Segmentado[^>]*opcoes=\{OPCOES_OCR\}[^>]*bind:valor=\{ocr\}/);
  assert.match(rep, /<Segmentado[^>]*opcoes=\{OPCOES_LIMPEZA\}[^>]*bind:valor=\{limpeza\}/);
});

// 'Desativado' saía cortado a 280px (4 segmentos de ~63px); 7 letras cabem.
test('rótulos curtos o bastante para 4 segmentos a 280px', () => {
  for (const o of [...OPCOES_OCR, ...OPCOES_LIMPEZA]) assert.ok(o.rotulo.length <= 7, o.rotulo);
  assert.equal(OPCOES_OCR[0].rotulo, 'Nenhum');
});

test('DESIGN.md §5.3 descreve o controle segmentado', () => {
  const design = ler('../DESIGN.md');
  const sec = design.slice(design.indexOf('### 5.3'), design.indexOf('### 5.4'));
  assert.match(sec, /controle segmentado/i);
  assert.doesNotMatch(sec, /Selects com opções curtas/);
});
