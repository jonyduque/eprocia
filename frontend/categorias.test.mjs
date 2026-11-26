// Ajustes do dono, rodada 2 (2026-11-24): categorias das Configurações
// colapsáveis, com o estado lembrado em localStorage.
// Rodar com: node --test frontend/categorias.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { CATEGORIAS, CHAVE_CATEGORIAS, gravarCategorias, lerCategorias } from './src/lib/categorias.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8').replace(/\r\n/g, '\n');
const settings = ler('src', 'components', 'Settings.svelte');
const design = ler('..', 'DESIGN.md');

const TODAS_EXPANDIDAS = { monitoramento: false, processamento: false, sistema: false, atualizacoes: false };

function memoria(inicial = {}) {
  const dados = { ...inicial };
  return {
    dados,
    getItem: (k) => (k in dados ? dados[k] : null),
    setItem: (k, v) => { dados[k] = String(v); },
  };
}

function estilo(texto) {
  const i = texto.indexOf('<style>');
  return texto.slice(i, texto.indexOf('</style>', i));
}

function regra(css, seletor) {
  const i = css.indexOf(seletor + ' {');
  assert.ok(i >= 0, `${seletor} ausente`);
  return css.slice(i, css.indexOf('}', i));
}

test('categorias: as quatro do painel, nessa ordem', () => {
  assert.deepEqual([...CATEGORIAS], ['monitoramento', 'processamento', 'sistema', 'atualizacoes']);
  assert.equal(CHAVE_CATEGORIAS, 'eprocia.categorias');
});

test('lerCategorias: sem nada salvo, todas expandidas', () => {
  assert.deepEqual(lerCategorias(memoria()), TODAS_EXPANDIDAS);
  assert.deepEqual(lerCategorias(null), TODAS_EXPANDIDAS);
});

test('lerCategorias: devolve o que foi salvo e ignora o que não é booleano', () => {
  const s = memoria({ [CHAVE_CATEGORIAS]: JSON.stringify({ processamento: true, sistema: 'sim', outra: true }) });
  assert.deepEqual(lerCategorias(s), { ...TODAS_EXPANDIDAS, processamento: true });
});

test('lerCategorias: o valor salvo antes de Atualizações, sem a chave nova, continua válido', () => {
  const s = memoria({ [CHAVE_CATEGORIAS]: JSON.stringify({ monitoramento: true, processamento: false, sistema: true }) });
  assert.deepEqual(lerCategorias(s), { monitoramento: true, processamento: false, sistema: true, atualizacoes: false });
});

test('lerCategorias: valor ilegível ou storage que lança, todas expandidas', () => {
  assert.deepEqual(lerCategorias(memoria({ [CHAVE_CATEGORIAS]: '{quebrado' })), TODAS_EXPANDIDAS);
  assert.deepEqual(lerCategorias(memoria({ [CHAVE_CATEGORIAS]: '[true]' })), TODAS_EXPANDIDAS);
  const lanca = { getItem: () => { throw new Error('SecurityError'); } };
  assert.deepEqual(lerCategorias(lanca), TODAS_EXPANDIDAS);
});

test('gravarCategorias: grava só as quatro chaves e não lança sem storage', () => {
  const s = memoria();
  assert.equal(gravarCategorias(s, { ...TODAS_EXPANDIDAS, sistema: true, outra: true }), true);
  assert.deepEqual(JSON.parse(s.dados[CHAVE_CATEGORIAS]), { ...TODAS_EXPANDIDAS, sistema: true });
  assert.deepEqual(lerCategorias(s), { ...TODAS_EXPANDIDAS, sistema: true });
  assert.equal(gravarCategorias(null, TODAS_EXPANDIDAS), false);
  const lanca = { setItem: () => { throw new Error('QuotaExceededError'); } };
  assert.equal(gravarCategorias(lanca, TODAS_EXPANDIDAS), false);
});

for (const [chave, rotulo] of [['monitoramento', 'Monitoramento'], ['processamento', 'Processamento'], ['sistema', 'Sistema'], ['atualizacoes', 'Atualizações']]) {
  test(`${rotulo}: cabeçalho é um botão que recolhe o conteúdo`, () => {
    const i = settings.indexOf(`aria-controls="grupo-${chave}"`);
    assert.ok(i >= 0, `aria-controls de ${chave} ausente`);
    const ini = settings.lastIndexOf('<h3', i);
    const cab = settings.slice(ini, settings.indexOf('</h3>', i) + 5);
    assert.match(cab, /^<h3 class="rotulo-grupo">\s*<button\s/);
    assert.match(cab, /type="button"/);
    assert.match(cab, /class="cabecalho-grupo"/);
    assert.match(cab, new RegExp(`aria-expanded=\\{!recolhidas\\.${chave}\\}`));
    assert.match(cab, new RegExp(`on:click=\\{\\(\\) => alternar\\('${chave}'\\)\\}`));
    assert.match(cab, /<span class="chevron-grupo" aria-hidden="true"><Icone icone=\{ChevronDown\} tamanho=\{14\} \/><\/span>/);
    assert.match(cab, new RegExp(`>\\s*${rotulo}\\s*</button>`));
    // O conteúdo fica montado e some com hidden: fora do foco e da árvore de acessibilidade.
    assert.match(settings, new RegExp(`<div class="conteudo-grupo" class:animar=\\{animar\\.${chave}\\} id="grupo-${chave}" hidden=\\{recolhidas\\.${chave}\\}>`));
  });
}

test('categorias: estado lido do storage e gravado a cada troca', () => {
  assert.match(settings, /import ChevronDown from 'lucide-svelte\/icons\/chevron-down'/);
  assert.match(settings, /let recolhidas = lerCategorias\(armazenamento\(\)\)/);
  const f = settings.slice(settings.indexOf('function alternar('), settings.indexOf('}', settings.indexOf('gravarCategorias(')) + 1);
  assert.match(f, /recolhidas = \{ \.\.\.recolhidas, \[chave\]: !recolhidas\[chave\] \}/);
  assert.match(f, /gravarCategorias\(armazenamento\(\), recolhidas\)/);
});

test('categorias: seta gira e o conteúdo entra com fade curto, zerado em movimento reduzido', () => {
  const css = estilo(settings);
  assert.match(regra(css, '.chevron-grupo'), /transition:\s*transform var\(--dur-fast\) var\(--ease-out\)/);
  assert.match(regra(css, '.cabecalho-grupo[aria-expanded="false"] .chevron-grupo'), /transform:\s*rotate\(-90deg\)/);
  // --dur-fast vale 0ms com prefers-reduced-motion (style.css), que o WebView2
  // liga quando os efeitos de animação do Windows estão desligados.
  assert.match(regra(css, '.conteudo-grupo.animar'), /animation:\s*entrar-grupo var\(--dur-fast\) var\(--ease-out\)/);
  assert.match(css, /@keyframes entrar-grupo/);
  assert.match(regra(css, '.cabecalho-grupo:focus-visible'), /outline:\s*2px solid var\(--focus-ring\)/);
  // O botão herda a aparência do rótulo da seção.
  const cab = regra(css, '.cabecalho-grupo');
  for (const p of [/font:\s*inherit/, /text-transform:\s*inherit/, /letter-spacing:\s*inherit/, /color:\s*inherit/]) assert.match(cab, p);
});

test('DESIGN.md descreve as categorias colapsáveis', () => {
  assert.match(design, /eprocia\.categorias/);
  assert.match(design, /aria-expanded/);
  assert.match(design, /ChevronDown/);
});
