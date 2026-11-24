// Ajuste do dono (2026-11-24): caixas com borda colorida de um lado só não
// arredondam os cantos daquele lado (a curva interna incomodava).
// Rodar com: node --test frontend/bordas.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8').replace(/\r\n/g, '\n');
const app = ler('src', 'App.svelte');

// B: regra com borda grossa (ou colorida) de um lado só não arredonda os cantos
// daquele lado, para a curva interna não aparecer.
function regras(css) {
  const out = [];
  const re = /([^{}]+)\{([^{}]*)\}/g;
  let m;
  while ((m = re.exec(css))) out.push({ sel: m[1].trim(), corpo: m[2] });
  return out;
}

function componentes() {
  const dir = join(raiz, 'src', 'components');
  const arqs = readdirSync(dir).filter((f) => f.endsWith('.svelte')).map((f) => ['components/' + f, ler('src', 'components', f)]);
  arqs.push(['App.svelte', app]);
  arqs.push(['assets/css/style.css', '<style>' + ler('src', 'assets', 'css', 'style.css') + '</style>']);
  return arqs;
}

test('B: caixas com borda lateral colorida têm os cantos daquele lado retos', () => {
  let achadas = 0;
  for (const [nome, fonte] of componentes()) {
    const css = fonte.slice(fonte.indexOf('<style>'));
    for (const { sel, corpo } of regras(css)) {
      const lado = corpo.match(/border-(left|right|inline-start|inline-end):\s*(\d+)px solid ([^;]+);/);
      if (!lado) continue;
      const grossa = Number(lado[2]) > 1 || !/var\(--border(-strong)?\)/.test(lado[3]);
      if (!grossa) continue;
      const raio = corpo.match(/border-radius:\s*([^;]+);/);
      if (!raio) continue;
      achadas++;
      const v = raio[1].trim().split(/\s+/);
      const [tl, tr, br, bl] = v.length === 4 ? v : v.length === 3 ? [v[0], v[1], v[2], v[1]] : v.length === 2 ? [v[0], v[1], v[0], v[1]] : [v[0], v[0], v[0], v[0]];
      const esquerda = lado[1] === 'left' || lado[1] === 'inline-start';
      const cantos = esquerda ? [tl, bl] : [tr, br];
      for (const c of cantos) assert.equal(c, '0', `${nome} ${sel}: canto do lado da borda colorida arredondado (${raio[1]})`);
    }
  }
  assert.ok(achadas >= 4, `esperava as 4 caixas conhecidas, achou ${achadas}`);
});
