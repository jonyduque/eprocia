// Ícone da marca na barra de título: a mesma arte do executável e da bandeja.
// Rodar com: node --test frontend/icone.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = dirname(fileURLToPath(import.meta.url));
const app = readFileSync(join(raiz, 'src', 'App.svelte'), 'utf8');
const css = readFileSync(join(raiz, 'src', 'assets', 'css', 'style.css'), 'utf8');
const pequeno = readFileSync(join(raiz, '..', 'build', 'icon-pequeno.svg'), 'utf8');

function svgDaMarca() {
  const i = app.indexOf('class="brand-icon"');
  assert.ok(i >= 0, 'brand-icon ausente');
  const ini = app.lastIndexOf('<svg', i);
  return app.slice(ini, app.indexOf('</svg>', i) + 6);
}

test('barra de título usa a variante pequena do ícone novo', () => {
  const svg = svgDaMarca();
  assert.doesNotMatch(svg, /m12 3-1\.912 5\.886/, 'estrela provisória ainda presente');
  assert.match(svg, /viewBox="0 0 256 256"/);
  const caminhos = [...pequeno.matchAll(/\sd="([^"]+)"/g)].map((m) => m[1]);
  assert.ok(caminhos.length > 0);
  for (const d of caminhos) assert.ok(svg.includes(`d="${d}"`), `traço ${d} ausente`);
  const fundo = pequeno.match(/<rect[^>]*>/)[0];
  for (const attr of ['x', 'y', 'width', 'height', 'rx']) {
    const v = fundo.match(new RegExp(`\\s${attr}="([^"]+)"`))[1];
    assert.match(svg, new RegExp(`<rect[^>]*\\s${attr}="${v}"`), `rect ${attr}=${v}`);
  }
  assert.match(svg, /width="16" height="16"/);
  assert.match(svg, /aria-hidden="true"/, 'decorativo: o nome vem do h1');
});

test('cores da marca vêm de tokens iguais nos dois temas', () => {
  const svg = svgDaMarca();
  const stops = [...pequeno.matchAll(/stop-color="(#[0-9a-fA-F]{6})"/g)].map((m) => m[1].toUpperCase());
  const raizCss = css.match(/:root\s*\{([^}]*)\}/)[1];
  const de = raizCss.match(/--brand-from:\s*(#[0-9A-Fa-f]{6})/);
  const ate = raizCss.match(/--brand-to:\s*(#[0-9A-Fa-f]{6})/);
  assert.ok(de && ate, 'tokens --brand-from/--brand-to no :root');
  assert.deepEqual([de[1].toUpperCase(), ate[1].toUpperCase()], stops);
  assert.match(raizCss, /--on-brand:\s*#FFFFFF/i);
  const claro = css.match(/:root\.light\s*\{([^}]*)\}/)[1];
  assert.doesNotMatch(claro, /--brand-|--on-brand/, 'a marca não muda com o tema');
  // id próprio, para não colidir com outro gradiente da página.
  assert.match(svg, /id="brand-grad"/);
  assert.match(svg, /url\(#brand-grad\)/);
});
