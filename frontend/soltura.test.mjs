// F-AN-01: overlay de soltar só para arquivos, contador de dragenter/dragleave
// e confirmação do que foi aceito.
// Rodar com: node --test frontend/soltura.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarSoltura, temArquivos, textoAceitos, textoJaNaFila } from './src/lib/soltura.js';

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

test('textoJaNaFila concorda com o número solto', () => {
  assert.equal(textoJaNaFila(1), 'Esse arquivo já está na fila');
  assert.equal(textoJaNaFila(2), 'Esses arquivos já estão na fila');
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
  // overlay e anúncio usam o número que a fila devolveu, não o que foi solto;
  // o soltar usa AddDroppedFiles, que marca a origem "arrastada" (telemetria)
  assert.match(app, /AddDroppedFiles\(aceitos\)\.then\(\(n\) => \{\s*soltura\.recebido\(n\);/);
  assert.match(app, /anunciar\(n > 0 \? textoAceitos\(n\) : textoJaNaFila\(aceitos\.length\)\)/);
  assert.doesNotMatch(app, /soltura\.recebido\(aceitos\.length\)/);
  // a região viva é esvaziada antes de anunciar: o App escreve pelo anunciador,
  // que escreve '' e só então espera o tick e escreve o texto
  assert.match(app, /const anunciar = criarAnunciador\(\(t\) => \(anuncio = t\), tick\)/);
  const regiao = readFileSync(join(raiz, 'src', 'lib', 'regiaoViva.js'), 'utf8');
  assert.match(regiao, /escrever\(''\);[\s\S]*await proximoTick\(\);[\s\S]*escrever\(junto\)/);
  assert.match(app, /lucide-svelte\/icons\/check'/);
  assert.match(app, /\{#if soltura?Estado\}|\{#if estadoSoltura\}/);
});
