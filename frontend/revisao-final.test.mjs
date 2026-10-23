// Problemas da revisão final da branch 2026-10-15 (itens 1, 3, 4, 6, 7 e 9).
// Rodar com: node --test frontend/revisao-final.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { armazenamento, lerSalvo, lerPreferencia, gravarSalvo, conciliarTema } from './src/lib/tema.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, 'src', ...p), 'utf8').replace(/\r\n/g, '\n');
const app = ler('App.svelte');
const settings = ler('components', 'Settings.svelte');
const lista = ler('components', 'ProcessList.svelte');

function script(texto) {
  return texto.slice(texto.indexOf('<script>'), texto.indexOf('</script>'));
}

function funcao(texto, nome) {
  const i = texto.indexOf(`function ${nome}(`);
  assert.ok(i >= 0, `${nome} ausente`);
  const fim = texto.indexOf('\n  }\n', i);
  return texto.slice(i, fim);
}

// Wails rejeita a promise com uma string, não com Error.
test('erros de binding usam err?.message ?? err', () => {
  for (const [nome, texto] of [['Settings', settings], ['ProcessList', lista], ['App', app]]) {
    assert.doesNotMatch(texto, /[^?]\berr\.message/, `${nome} lê err.message direto`);
  }
  assert.match(funcao(settings, 'handleSave'), /err\?\.message \?\? err/);
  assert.match(funcao(settings, 'handleSelectDirectory'), /err\?\.message \?\? err/);
});

test('falha ao salvar recarrega a config gravada no App', () => {
  const salvar = funcao(settings, 'handleSave');
  const captura = salvar.slice(salvar.indexOf('catch'));
  assert.match(captura, /aoFalharSalvar\(\)/, 'Settings avisa o App da falha');
  assert.match(settings, /export let aoFalharSalvar\b/);
  assert.match(app, /<Settings\b[^>]*aoFalharSalvar=\{refreshConfig\}/);
});

test('Settings fica montado com o painel recolhido', () => {
  const tag = app.indexOf('<Settings');
  const antes = app.slice(0, tag);
  const abertos = (antes.match(/\{#if\b/g) || []).length;
  const fechados = (antes.match(/\{\/if\}/g) || []).length;
  assert.equal(abertos, fechados, 'Settings não pode estar dentro de um {#if}');
  assert.match(app, /class="sidebar-content"[^>]*hidden=\{sidebarCollapsed\}/);
});

test('cópia que falha avisa o usuário', () => {
  const copiar = funcao(lista, 'handleCopy');
  const captura = copiar.slice(copiar.indexOf('catch'));
  assert.match(captura, /avisoCopia\.mostrar\(/);
  assert.match(captura, /err\?\.message \?\? err/);
  assert.match(lista, /role="alert"/);
});

// --- localStorage bloqueado -----------------------------------------------

const bloqueado = {
  get localStorage() { throw new Error('SecurityError'); },
};
const quebrado = {
  getItem() { throw new Error('SecurityError'); },
  setItem() { throw new Error('QuotaExceeded'); },
};

test('tema funciona com o armazenamento bloqueado', () => {
  assert.equal(armazenamento(bloqueado), null);
  assert.equal(lerSalvo(null), null);
  assert.equal(lerSalvo(quebrado), null);
  assert.equal(lerPreferencia(quebrado), 'system');
  assert.equal(gravarSalvo(quebrado, 'dark'), false);
  assert.equal(gravarSalvo(null, 'dark'), false);

  const mem = new Map();
  const ok = { getItem: (k) => mem.get(k) ?? null, setItem: (k, v) => mem.set(k, v) };
  assert.equal(gravarSalvo(ok, 'dark'), true);
  assert.equal(lerSalvo(ok), 'dark');
});

test('App não toca localStorage direto', () => {
  assert.doesNotMatch(script(app), /\blocalStorage\b/);
  assert.match(script(app), /armazenamento\(\)/);
});

// --- tema no config.json (janela nativa) ----------------------------------

test('conciliarTema: localStorage vence; config preenche quando falta', () => {
  assert.deepEqual(conciliarTema('dark', 'system'), { tema: 'dark', gravarConfig: true, gravarLocal: false });
  assert.deepEqual(conciliarTema('light', 'light'), { tema: 'light', gravarConfig: false, gravarLocal: false });
  assert.deepEqual(conciliarTema(null, 'dark'), { tema: 'dark', gravarConfig: false, gravarLocal: true });
  assert.deepEqual(conciliarTema(null, 'system'), { tema: 'system', gravarConfig: false, gravarLocal: false });
  // Config ilegível: não sobrescreve o que o backend tem.
  assert.deepEqual(conciliarTema('dark', undefined), { tema: 'dark', gravarConfig: false, gravarLocal: false });
  assert.deepEqual(conciliarTema(null, 'roxo'), { tema: 'system', gravarConfig: false, gravarLocal: false });
});

test('trocar o tema grava no backend', () => {
  assert.match(funcao(app, 'toggleTheme'), /gravarTemaNoBackend\(temaPref\)/);
  assert.match(funcao(app, 'gravarTemaNoBackend'), /SetTheme\(tema\)/);
  assert.match(app, /conciliarTema\(/);
});

test('App.svelte sem linhas em branco seguidas', () => {
  assert.doesNotMatch(app, /\n[ \t]*\n[ \t]*\n/);
});
