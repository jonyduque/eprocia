// RNF01: o app roda offline. Nenhum recurso do frontend pode vir da rede, e a
// tipografia precisa usar fontes que existem no Windows sem download.
// Rodar com: node --test frontend/offline.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = dirname(fileURLToPath(import.meta.url));

function listar(dir) {
  return readdirSync(dir).flatMap((nome) => {
    const p = join(dir, nome);
    if (nome === 'wailsjs') return [];
    if (statSync(p).isDirectory()) return listar(p);
    return /\.(svelte|css|js|html)$/.test(nome) ? [p] : [];
  });
}

test('nenhum recurso externo no frontend', () => {
  const arquivos = [join(raiz, 'index.html'), ...listar(join(raiz, 'src'))];
  const externos = [];
  for (const arq of arquivos) {
    const texto = readFileSync(arq, 'utf8');
    for (const m of texto.matchAll(/(?:https?:)?\/\/[a-z0-9.-]+\.[a-z]{2,}[^\s"')]*/gi)) {
      // xmlns do SVG é só identificador, não gera requisição.
      if (m[0].startsWith('http://www.w3.org/')) continue;
      externos.push(`${arq}: ${m[0]}`);
    }
  }
  assert.deepEqual(externos, []);
});

test('pilha tipográfica usa só fontes do sistema', () => {
  const css = readFileSync(join(raiz, 'src', 'assets', 'css', 'style.css'), 'utf8');
  const sans = css.match(/--font-sans:\s*([^;]+);/);
  assert.ok(sans, '--font-sans não encontrado');
  assert.match(sans[1], /^"Segoe UI Variable"/);
  assert.doesNotMatch(sans[1], /Inter/);
  assert.doesNotMatch(css, /--font-mono|JetBrains/);
});
