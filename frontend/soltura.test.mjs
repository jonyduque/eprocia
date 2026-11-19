// F-AN-01: overlay de soltar só para arquivos, contador de dragenter/dragleave
// e confirmação do que foi aceito.
// Rodar com: node --test frontend/soltura.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarSoltura, temArquivos, textoAceitos } from './src/lib/soltura.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const app = readFileSync(join(raiz, 'src', 'App.svelte'), 'utf8');

function relogioFalso() {
  let agora = 0;
  let seq = 0;
  const timers = new Map();
  return {
    setTimeout(fn, ms) { timers.set(++seq, { fn, em: agora + ms }); return seq; },
    clearTimeout(id) { timers.delete(id); },
    avancar(ms) {
      agora += ms;
      for (const [id, t] of [...timers]) {
        if (t.em <= agora) { timers.delete(id); t.fn(); }
      }
    },
    pendentes: () => timers.size,
  };
}

const arquivos = { types: ['Files'] };
const texto = { types: ['text/plain', 'text/html'] };

test('temArquivos só aceita arrastos com Files', () => {
  assert.equal(temArquivos(arquivos), true);
  assert.equal(temArquivos(texto), false);
  assert.equal(temArquivos(null), false);
  assert.equal(temArquivos({ types: { contains: (t) => t === 'Files' } }), true);
});

test('textoAceitos usa singular e plural', () => {
  assert.equal(textoAceitos(1), '1 arquivo adicionado');
  assert.equal(textoAceitos(3), '3 arquivos adicionados');
});

test('arrasto de texto não liga o overlay', () => {
  let estado = null;
  const s = criarSoltura((e) => { estado = e; }, relogioFalso());
  s.entrar(texto);
  assert.equal(estado, null);
  s.sair(texto);
  assert.equal(estado, null);
});

test('contador: dragleave de um filho não desliga o overlay', () => {
  let estado = null;
  const s = criarSoltura((e) => { estado = e; }, relogioFalso());
  s.entrar(arquivos);
  s.entrar(arquivos); // entra num filho antes do leave do pai
  s.sair(arquivos);
  assert.deepEqual(estado, { fase: 'arrastando' });
  s.sair(arquivos);
  assert.equal(estado, null);
});

test('soltar ZIP: overlay vira confirmação e some depois do prazo', () => {
  let estado = null;
  const r = relogioFalso();
  const s = criarSoltura((e) => { estado = e; }, r, 220);
  s.entrar(arquivos);
  s.soltar(arquivos);
  assert.deepEqual(estado, { fase: 'soltando' });
  s.recebido(2);
  assert.deepEqual(estado, { fase: 'aceito', texto: '2 arquivos adicionados' });
  r.avancar(219);
  assert.equal(estado.fase, 'aceito');
  r.avancar(1);
  assert.equal(estado, null);
  assert.equal(r.pendentes(), 0);
});

test('soltar sem ZIP aceito fecha o overlay na hora', () => {
  let estado = null;
  const s = criarSoltura((e) => { estado = e; }, relogioFalso());
  s.entrar(arquivos);
  s.soltar(arquivos);
  s.recebido(0);
  assert.equal(estado, null);
});

test('soltar sem resposta do Wails não prende o overlay', () => {
  let estado = null;
  const r = relogioFalso();
  const s = criarSoltura((e) => { estado = e; }, r);
  s.entrar(arquivos);
  s.soltar(arquivos);
  r.avancar(1000);
  assert.equal(estado, null);
});

test('novo arrasto durante a confirmação volta ao estado de arrasto', () => {
  let estado = null;
  const r = relogioFalso();
  const s = criarSoltura((e) => { estado = e; }, r);
  s.entrar(arquivos);
  s.soltar(arquivos);
  s.recebido(1);
  s.entrar(arquivos);
  assert.deepEqual(estado, { fase: 'arrastando' });
  r.avancar(5000);
  assert.deepEqual(estado, { fase: 'arrastando' });
});

test('App usa o controlador de soltura, sem clientX/clientY, e anuncia o aceito', () => {
  assert.doesNotMatch(app, /clientX === 0|clientY === 0/);
  assert.match(app, /criarSoltura\(/);
  assert.match(app, /soltura\.recebido\(/);
  // anuncia só depois que a fila aceitou, esvaziando antes (mesmo texto repetido)
  assert.match(app, /AddFilesToQueue\(zips\)\.then\(\(\) => anunciar\(textoAceitos\(/);
  assert.match(app, /anuncio = '';\s*await tick\(\);/);
  assert.match(app, /lucide-svelte\/icons\/check'/);
  assert.match(app, /\{#if soltura?Estado\}|\{#if estadoSoltura\}/);
});
