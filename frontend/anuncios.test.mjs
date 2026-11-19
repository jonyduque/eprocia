// F-AC-05 (mudanças de estado anunciadas) e F-AC-06 (rótulo e retorno do filtro).
// Rodar com: node --test frontend/anuncios.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarAviso } from './src/lib/aviso.js';
import { marcosFila, anuncioCopia, anuncioPausa, anuncioFiltro } from './src/lib/anuncios.js';

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
  assert.match(lista, /Nenhum item corresponde ao filtro/);
  assert.match(lista, /Nenhum processo no histórico\./);
});

// Regiões vivas da fila: só marcos, numa região única sempre montada (F-AC-05, F-AC-01).
const atividadeItem = ler('AtividadeItem.svelte');

function foraDeIf(fonte, i) {
  const antes = fonte.slice(0, i);
  return (antes.match(/\{#if\b/g) || []).length === (antes.match(/\{\/if\}/g) || []).length
    && (antes.match(/\{#each\b/g) || []).length === (antes.match(/\{\/each\}/g) || []).length;
}

test('resumo da atividade não é região viva e o botão fica fora dele', () => {
  assert.doesNotMatch(atividadeItem, /role="status"/);
  assert.doesNotMatch(atividadeItem, /aria-live/);
  const i = atividadeItem.indexOf('<div class="resumo');
  const fim = atividadeItem.indexOf('</div>', i);
  assert.ok(i >= 0);
  assert.doesNotMatch(atividadeItem.slice(i, fim), /class="alternar"/);
});

test('ProcessList tem uma única região status, sempre montada, e chip/filtro sem role', () => {
  const regioes = lista.match(/role="status"/g) || [];
  assert.equal(regioes.length, 1);
  const i = lista.indexOf('<div class="sr-only" role="status" aria-live="polite">{anuncio}</div>');
  assert.ok(i >= 0, 'região .sr-only com {anuncio}');
  assert.ok(foraDeIf(lista.slice(lista.indexOf('</script>')), lista.slice(lista.indexOf('</script>')).indexOf('{anuncio}</div>')));
  assert.match(lista, /<span class="queue-chip" title=\{aviso\}>/);
  assert.match(lista, /<p>Nenhum item corresponde ao filtro<\/p>/);
});

test('marcos da fila: fim com documentos e não lidos, erro e mudança de erros', () => {
  let r = marcosFila(null, [{ id: 'a', name: 'p.zip', status: 'processing' }], {});
  assert.deepEqual(r.mensagens, [], 'a primeira leitura não anuncia');
  r = marcosFila(r.estado, [{ id: 'a', name: 'p.zip', status: 'processing' }], {
    a: { docs: [{ estado: 'erro' }, { estado: 'concluido' }] }
  });
  assert.deepEqual(r.mensagens, ['p.zip: 1 documento com erro']);
  r = marcosFila(r.estado, [{ id: 'a', name: 'p.zip', status: 'processing' }], {
    a: { docs: [{ estado: 'erro' }, { estado: 'lendo' }] }
  });
  assert.deepEqual(r.mensagens, [], 'contadores mudam sem marco');
  r = marcosFila(r.estado, [{ id: 'a', name: 'p.zip', status: 'success', docs_count: 2, unread_docs: 2 }], {});
  assert.deepEqual(r.mensagens, ['p.zip: concluído, 2 documentos, 2 documentos não lidos']);
  r = marcosFila(r.estado, [{ id: 'a', name: 'p.zip', status: 'success', docs_count: 2, unread_docs: 2 }], {});
  assert.deepEqual(r.mensagens, [], 'sem mudança, sem anúncio');
  r = marcosFila(r.estado, [{ id: 'b', name: 'q.zip', status: 'processing' }], {});
  r = marcosFila(r.estado, [{ id: 'b', name: 'q.zip', status: 'error' }], {});
  assert.deepEqual(r.mensagens, ['q.zip: erro no processamento']);
  r = marcosFila(r.estado, [{ id: 'c', name: 'r.zip', status: 'processing' }], {});
  r = marcosFila(r.estado, [{ id: 'c', name: 'r.zip', status: 'success', docs_count: 1, unread_docs: 0 }], {});
  assert.deepEqual(r.mensagens, ['r.zip: concluído, 1 documento']);
});

test('anúncios de cópia, pausa e filtro', () => {
  assert.equal(anuncioCopia('p.zip', 0), 'TXT de p.zip copiado');
  assert.equal(anuncioCopia('p.zip', 2), '', 'com não lidos o role=alert já avisa');
  assert.equal(anuncioPausa(false, true, 'Fila pausada'), 'Fila pausada');
  assert.equal(anuncioPausa(true, false, null), 'Fila retomada');
  assert.equal(anuncioPausa(undefined, true, 'Fila pausada'), '', 'estado inicial não anuncia');
  assert.equal(anuncioPausa(true, true, 'Fila pausada'), '');
  assert.equal(anuncioFiltro('', 3), '');
  assert.equal(anuncioFiltro('abc', 0), 'Nenhum item corresponde ao filtro');
  assert.equal(anuncioFiltro('abc', 1), '1 item corresponde ao filtro');
  assert.equal(anuncioFiltro('abc', 4), '4 itens correspondem ao filtro');
  assert.match(lista, /anunciar\(anuncioCopia\(/);
  assert.match(lista, /anuncioPausa\(/);
  assert.match(lista, /anuncioFiltro\(/);
  assert.match(lista, /marcosFila\(/);
});

test('botão Copiar: retorno visual em --success com texto Copiado e entrada animada', () => {
  const i = lista.indexOf('data-acao="copiar"');
  const fim = lista.indexOf('</button>', i);
  const bloco = lista.slice(i, fim);
  assert.match(bloco, /class:copiado=\{copiadoId === item\.id\}/);
  assert.match(bloco, /\{#key /);
  assert.match(bloco, /in:scale=\{\{[^}]*start: 0\.6[^}]*duracaoMovimento\(DUR_FAST\)[^}]*cubicOut/);
  // O texto ocupa o lugar do selo de contadores: a coluna de ações tem largura fixa.
  const linha = lista.slice(lista.indexOf('<span class="details-row">'), lista.indexOf('class="meta-badge success"'));
  assert.match(linha, /\{#if copiadoId === item\.id\}\s*<span class="copiado-texto" in:fade=\{\{ duration: duracaoMovimento\(DUR_FAST\) \}\}>Copiado<\/span>\s*\{:else\}/);
  assert.match(lista, /\.btn-action\.copiado[^{]*\{[^}]*color: var\(--success\)[^}]*background-color: var\(--success-soft\)/);
  assert.match(lista, /\.copiado-texto\s*\{[^}]*color: var\(--success-strong\)[^}]*font-size: var\(--fs-meta\)/);
});
