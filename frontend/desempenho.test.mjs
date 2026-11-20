// F-DF-01 (fila atualizada no máximo uma vez por quadro) e F-DF-02 (config
// lida uma vez pelo App e passada ao Settings).
// Rodar com: node --test frontend/desempenho.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarFilaPorQuadro } from './src/lib/quadro.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, 'src', ...p), 'utf8').replace(/\r\n/g, '\n');
const app = ler('App.svelte');
const settings = ler('components', 'Settings.svelte');

function quadros() {
  const pend = [];
  return {
    agendar(fn) { pend.push(fn); return pend.length; },
    rodar() { const fns = pend.splice(0); fns.forEach((f) => f()); },
    get pendentes() { return pend.length; },
  };
}

// --- F-DF-01 ---------------------------------------------------------------

test('várias listas no mesmo quadro aplicam só a última', () => {
  const q = quadros();
  let items = [];
  const aplicadas = [];
  const fila = criarFilaPorQuadro(() => items, (l) => { items = l; aplicadas.push(l); }, q.agendar);

  for (let i = 0; i < 300; i++) fila.lista([{ id: 'a', pagina: i }]);
  assert.equal(aplicadas.length, 0, 'nada aplicado antes do quadro');
  assert.equal(q.pendentes, 1, 'um único quadro agendado');

  q.rodar();
  assert.equal(aplicadas.length, 1);
  assert.deepEqual(items, [{ id: 'a', pagina: 299 }]);
});

test('itens do mesmo quadro se combinam com a lista mais recente', () => {
  const q = quadros();
  let items = [{ id: 'a', n: 0 }, { id: 'b', n: 0 }];
  let aplicacoes = 0;
  const fila = criarFilaPorQuadro(() => items, (l) => { items = l; aplicacoes++; }, q.agendar);

  fila.item({ id: 'a', n: 1 });
  fila.item({ id: 'a', n: 2 });
  fila.item({ id: 'b', n: 5 });
  fila.item({ id: 'x', n: 9 }); // desconhecido: ignorado
  q.rodar();
  assert.equal(aplicacoes, 1);
  assert.deepEqual(items, [{ id: 'a', n: 2 }, { id: 'b', n: 5 }]);

  // Item anterior à lista é descartado; item posterior vale sobre ela.
  fila.item({ id: 'a', n: 3 });
  fila.lista([{ id: 'a', n: 10 }, { id: 'c', n: 0 }]);
  fila.item({ id: 'c', n: 1 });
  q.rodar();
  assert.deepEqual(items, [{ id: 'a', n: 10 }, { id: 'c', n: 1 }]);
});

test('lista nula vira vazia e cancelar descarta o quadro pendente', () => {
  const q = quadros();
  let items = [{ id: 'a' }];
  let cancelado = 0;
  const fila = criarFilaPorQuadro(() => items, (l) => { items = l; }, q.agendar, () => { cancelado++; });
  fila.lista(null);
  q.rodar();
  assert.deepEqual(items, []);

  fila.lista([{ id: 'z' }]);
  fila.cancelar();
  assert.equal(cancelado, 1);
  q.rodar();
  assert.deepEqual(items, []);
});

test('App encaminha queue:update e queue:item pelo agendador por quadro', () => {
  const ini = app.indexOf("EventsOn('queue:update'");
  const fim = app.indexOf("EventsOn('queue:status'");
  assert.ok(ini >= 0 && fim > ini);
  const trecho = app.slice(ini, fim);
  assert.doesNotMatch(trecho, /items\s*=/, 'eventos não podem substituir items direto');
  assert.match(trecho, /filaQuadro\.lista\(/);
  assert.match(trecho, /filaQuadro\.item\(/);
  assert.match(app, /filaQuadro\.cancelar\(\)/, 'onDestroy cancela o quadro pendente');
});

// --- F-DF-02 ---------------------------------------------------------------

test('Settings não lê a config: recebe do App por bind', () => {
  assert.doesNotMatch(settings, /GetConfig/);
  assert.match(settings, /export let config\b/);
  assert.match(app, /<Settings\b[^>]*bind:config/);
  const chamadas = [...app.matchAll(/GetConfig\(\)/g)];
  assert.equal(chamadas.length, 1, 'uma única leitura, no App');
});

// --- F-DF-04 ---------------------------------------------------------------

import { prepararDocs, ordenarDocs, resumoAtividade, LIMITE_AGUARDANDO } from './src/lib/atividade.js';

function instantaneo(n, estadoDe) {
  const docs = [];
  for (let i = 0; i < n; i++) docs.push({ nome: `d${i}`, estado: estadoDe(i), paginas: 2, feitas: 0, ocr: 0, leitura: i % 50 === 0 ? 'sem_ocr' : 'lido', detalhe: `det${i}` });
  return { docs };
}

const estado400 = (i) => (i < 30 ? 'concluido' : i < 33 ? 'ocr' : i < 36 ? 'lendo' : i === 36 ? 'erro' : 'aguardando');

test('painel limita os aguardando no DOM e põe a linha "+ N aguardando" antes dos concluídos', () => {
  const inst = instantaneo(400, estado400);
  const { visiveis, resumo, aguardandoOcultos } = prepararDocs(inst);
  assert.equal(LIMITE_AGUARDANDO, 20);
  const docs = visiveis.filter((d) => !d.mais);
  assert.equal(docs.length, 3 + 3 + 1 + LIMITE_AGUARDANDO, 'ativos, erros e só os primeiros aguardando');
  assert.equal(aguardandoOcultos, 363 - LIMITE_AGUARDANDO);
  const mais = visiveis.findIndex((d) => d.mais);
  assert.equal(mais, visiveis.length - 1, 'linha de expansão fecha a lista sem concluídos');
  assert.equal(visiveis[mais].i, -1, 'chave estável');
  assert.equal(visiveis[mais].n, aguardandoOcultos);
  assert.deepEqual(docs.slice(-LIMITE_AGUARDANDO).map((d) => d.i), Array.from({ length: LIMITE_AGUARDANDO }, (_, k) => 37 + k), 'aguardando na ordem do ZIP');
  assert.deepEqual(resumo, resumoAtividade(inst), 'resumo calculado no mesmo laço');
  assert.equal(docs[0].leitura, 'lido');
  assert.equal(docs[0].detalhe, 'det30', 'leitura e detalhe chegam à linha');
});

test('expandir mostra todos os aguardando; com concluídos, a ordem bate com ordenarDocs', () => {
  const inst = instantaneo(400, estado400);
  const tudo = prepararDocs(inst, { mostrarConcluidos: true, expandirAguardando: true });
  assert.equal(tudo.aguardandoOcultos, 0);
  const recolher = tudo.visiveis.filter((d) => d.mais);
  assert.equal(recolher.length, 1, 'linha para recolher de volta');
  assert.equal(recolher[0].expandido, true);
  assert.deepEqual(tudo.visiveis.filter((d) => !d.mais).map((d) => d.i), ordenarDocs(inst.docs).map((d) => d.i));
  const recolhido = prepararDocs(inst, { mostrarConcluidos: true });
  const mais = recolhido.visiveis.findIndex((d) => d.mais);
  assert.equal(recolhido.visiveis[mais + 1].estado, 'concluido', 'concluídos vêm depois da linha de expansão');
  assert.equal(prepararDocs(instantaneo(10, () => 'aguardando')).aguardandoOcultos, 0, 'sem linha quando cabem');
  assert.deepEqual(prepararDocs(null).visiveis, []);
});

test('preparar 400 documentos não clona os ocultos', () => {
  const inst = instantaneo(400, estado400);
  let lidos = 0;
  for (const d of inst.docs) {
    const orig = { ...d };
    Object.defineProperty(d, 'nome', { get() { lidos++; return orig.nome; }, enumerable: true });
  }
  prepararDocs(inst);
  assert.ok(lidos <= 3 + 3 + 1 + LIMITE_AGUARDANDO, `clonou ${lidos} documentos`);
});

test('AtividadeItem usa prepararDocs, sem ordenar nem resumir à parte, e tem a linha de expansão', () => {
  const at = ler('components', 'AtividadeItem.svelte');
  assert.match(at, /prepararDocs\(atividade, \{ mostrarConcluidos, expandirAguardando \}\)/);
  assert.doesNotMatch(at, /ordenarDocs|resumoAtividade|docs\.filter/);
  assert.match(at, /class="mais-aguardando"[^>]*aria-expanded=\{expandirAguardando\}/s);
  assert.match(at, /\+ \{d\.n\} aguardando/);
  assert.match(at, /Mostrar só os primeiros \{LIMITE_AGUARDANDO\}/);
});
