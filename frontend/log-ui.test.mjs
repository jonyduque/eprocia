import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { itensLog } from './src/lib/log.js';
import { ROTULO_ABRIR_LOGS, TEXTO_LOG_ATIVO } from './src/lib/textos.js';

const ler = (p) => readFileSync(new URL(p, import.meta.url), 'utf8');
const settings = ler('./src/components/Settings.svelte');
const app = ler('./src/App.svelte');

// Só a marcação: sem <script>, <style> e comentários HTML.
function template(fonte) {
  return fonte
    .replace(/<script[\s\S]*?<\/script>/g, '')
    .replace(/<style[\s\S]*?<\/style>/g, '')
    .replace(/<!--[\s\S]*?-->/g, '');
}

// Separa o que fica dentro de cada {#if logAtivo}...{/if} (com ifs aninhados)
// do que fica fora.
function separarBlocoLog(t) {
  const fora = [];
  const dentro = [];
  let i = 0;
  for (;;) {
    const ini = t.indexOf('{#if logAtivo}', i);
    if (ini < 0) {
      fora.push(t.slice(i));
      break;
    }
    fora.push(t.slice(i, ini));
    const re = /\{#if\b|\{\/if\}/g;
    re.lastIndex = ini + '{#if logAtivo}'.length;
    let nivel = 1;
    let m;
    while (nivel > 0 && (m = re.exec(t))) nivel += m[0] === '{/if}' ? -1 : 1;
    assert.equal(nivel, 0, '{#if logAtivo} sem {/if}');
    dentro.push(t.slice(ini, re.lastIndex));
    i = re.lastIndex;
  }
  return { fora: fora.join(''), dentro: dentro.join('') };
}

const PALAVRA_LOG = /\blogs?\b|telemetria/i;

describe('log na interface só com o log ativo', () => {
  const { fora, dentro } = separarBlocoLog(template(settings));

  it('o detector não pega login nem catálogo', () => {
    assert.doesNotMatch('login catálogo diálogo', PALAVRA_LOG);
    assert.match('pasta de logs', PALAVRA_LOG);
  });
  it('nada sobre log fora de {#if logAtivo}', () => {
    assert.ok(dentro.length > 0, 'falta o bloco {#if logAtivo}');
    assert.doesNotMatch(fora, PALAVRA_LOG);
  });
  it('o bloco usa itensLog e o botão chama AbrirPastaLogs', () => {
    assert.match(dentro, /\{#each itensLog\(logAtivo\) as /);
    assert.match(dentro, /on:click=\{abrirPastaLogs\}/);
    assert.match(settings, /AbrirPastaLogs\(\)/);
  });
  it('Settings recebe logAtivo, falso por padrão', () => {
    assert.match(settings, /export let logAtivo = false;/);
  });
  it('App chama LogAtivo() uma vez e passa {logAtivo} ao Settings', () => {
    assert.equal(app.match(/\bLogAtivo\(\)/g)?.length, 1);
    assert.match(app, /<Settings\b[^>]*\{logAtivo\}/);
  });
});

describe('itensLog', () => {
  it('false: nada', () => {
    assert.deepEqual(itensLog(false), []);
  });
  it('valores falsos não ativam', () => {
    assert.deepEqual(itensLog(undefined), []);
    assert.deepEqual(itensLog(null), []);
  });
  it('true: o botão e o texto', () => {
    assert.deepEqual(itensLog(true), [
      { tipo: 'botao', texto: ROTULO_ABRIR_LOGS },
      { tipo: 'texto', texto: TEXTO_LOG_ATIVO }
    ]);
  });
});
