// F-AC-05 (mudanças de estado anunciadas) e F-AC-06 (rótulo e retorno do filtro).
// Rodar com: node --test frontend/anuncios.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarAviso } from './src/lib/aviso.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (nome) => readFileSync(join(raiz, 'src', 'components', nome), 'utf8').replace(/\r\n/g, '\n');
const settings = ler('Settings.svelte');
const lista = ler('ProcessList.svelte');

function relogio() {
  let agora = 0;
  let prox = 1;
  const pend = new Map();
  return {
    setTimeout(fn, ms) { const id = prox++; pend.set(id, { fn, em: agora + ms }); return id; },
    clearTimeout(id) { pend.delete(id); },
    avancar(ms) {
      agora += ms;
      for (const [id, t] of [...pend].sort((a, b) => a[1].em - b[1].em)) {
        if (t.em <= agora) { pend.delete(id); t.fn(); }
      }
    },
    get pendentes() { return pend.size; },
  };
}

test('aviso de erro não some sozinho e fecha pelo botão', () => {
  const r = relogio();
  let atual = null;
  const aviso = criarAviso((m) => { atual = m; }, 3000, r);
  aviso.mostrar('falhou', 'error');
  assert.equal(r.pendentes, 0);
  r.avancar(60000);
  assert.deepEqual(atual, { texto: 'falhou', tipo: 'error' });
  aviso.fechar();
  assert.equal(atual, null);
});

test('sucesso depois de erro volta a sumir em 3 s; erro depois de sucesso cancela o prazo', () => {
  const r = relogio();
  let atual = null;
  const aviso = criarAviso((m) => { atual = m; }, 3000, r);
  aviso.mostrar('falhou', 'error');
  aviso.mostrar('salvo', 'success');
  r.avancar(3000);
  assert.equal(atual, null);
  aviso.mostrar('salvo', 'success');
  r.avancar(1000);
  aviso.mostrar('falhou', 'error');
  assert.equal(r.pendentes, 0);
  r.avancar(5000);
  assert.deepEqual(atual, { texto: 'falhou', tipo: 'error' });
});

test('aviso em regiões vivas permanentes: status para sucesso, alert para erro e atenção', () => {
  const aviso = ler('Aviso.svelte');
  assert.match(aviso, /role="status" aria-live="polite"/);
  assert.match(aviso, /role="alert"/);
  for (const papel of ['role="status"', 'role="alert"']) {
    const iRegiao = aviso.indexOf(papel);
    const iIf = aviso.indexOf('{#if aviso', iRegiao);
    assert.ok(iRegiao >= 0 && iIf > iRegiao, 'a região viva precisa existir antes do {#if}, para o leitor de tela detectar a mudança');
    const antes = aviso.slice(0, iRegiao);
    assert.equal((antes.match(/\{#if\b/g) || []).length, (antes.match(/\{\/if\}/g) || []).length, 'a região viva não pode estar dentro de um {#if}');
  }
  assert.match(aviso, /aria-label="Fechar aviso"/);
  assert.match(aviso, /on:click=\{aoFechar\}/);
  // A confirmação de Settings também é anunciada, sem toast.
  assert.match(settings, /class="sr-only" role="status" aria-live="polite"/);
});

test('barra de progresso exposta como progressbar com valores', () => {
  const i = lista.indexOf('class="progress-bar-container"');
  assert.ok(i >= 0);
  const tag = lista.slice(lista.lastIndexOf('<', i), lista.indexOf('>', i) + 1);
  assert.match(tag, /role="progressbar"/);
  assert.match(tag, /aria-valuemin="0"/);
  assert.match(tag, /aria-valuemax="100"/);
  assert.match(tag, /aria-valuenow=\{Math\.round\(/);
  assert.match(tag, /aria-label=/);
});

test('ícones de status ocultos e acompanhados de texto para leitor de tela', () => {
  const i = lista.indexOf('<div class="status-col">');
  const col = lista.slice(i, lista.indexOf('</div>\n\n', i));
  // Icone.svelte sempre põe aria-hidden (conferido em icones.test.mjs).
  const icones = col.match(/<Icone\b[^>]*>/g) ?? [];
  assert.ok(icones.length >= 6);
  assert.doesNotMatch(col, /<svg/);
  assert.match(col, /spinner-small"[^>]*aria-hidden="true"/);
  assert.match(col, /class="sr-only"/);
  for (const rotulo of ['Concluído', 'Em processamento', 'Erro', 'Aguardando', 'Pausado', 'Cancelado']) {
    assert.ok(lista.includes(`'${rotulo}'`), `rótulo ${rotulo} ausente`);
  }
  assert.match(lista, /\.sr-only\s*\{[^}]*clip/);
});

test('filtro com nome acessível e retorno próprio quando nada corresponde', () => {
  const i = lista.indexOf('class="search-input"');
  const tag = lista.slice(lista.lastIndexOf('<input', i), lista.indexOf('/>', i));
  assert.match(tag, /type="search"/);
  assert.match(tag, /aria-label="Filtrar fila por nome ou mensagem"/);
  assert.match(lista, /role="status"[^>]*>\s*Nenhum item corresponde ao filtro/);
  assert.match(lista, /Nenhum processo no histórico\./);
});
