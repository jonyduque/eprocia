// Histórico da fila ao reabrir (spec 2026-11-18-historico): estado interrompido,
// TXT e origem ausentes, data da linha, painel sem detalhes, interruptor e aviso.
// Rodar com: node --test frontend/historico.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, 'src', ...p), 'utf8').replace(/\r\n/g, '\n');
const lista = ler('components', 'ProcessList.svelte');
const atividade = ler('components', 'AtividadeItem.svelte');
const settings = ler('components', 'Settings.svelte');
const app = ler('App.svelte');

function marcacao(texto) {
  const i = texto.indexOf('</script>');
  return texto.slice(i, texto.indexOf('<style>', i));
}

// Todos os <button ...>...</button> que contêm a marca.
function botoesCom(texto, marca) {
  const r = [];
  let i = texto.indexOf(marca);
  while (i >= 0) {
    r.push(texto.slice(texto.lastIndexOf('<button', i), texto.indexOf('</button>', i)));
    i = texto.indexOf(marca, i + 1);
  }
  return r;
}

const ml = marcacao(lista);

test('ProcessList: ícone e rótulo do interrompido', () => {
  const i = ml.indexOf('<div class="status-col">');
  const col = ml.slice(i, ml.indexOf('</div>', i));
  assert.match(col, /\{:else if estado === 'interrompido'\}\s*<Icone icone=\{\w+\}[^>]*cor="var\(--warning\)"/);
  assert.match(lista, /interrompido: HISTORICO\.interrompido|interrompido: 'Interrompido'/);
  const j = ml.indexOf('<span class="details-row">');
  const det = ml.slice(j, ml.indexOf('</span>\n              {#if expansivel}', j));
  assert.match(det, /\{:else if estado === 'interrompido'\}[\s\S]*?\{HISTORICO\.interrompido\}/);
});

test('ProcessList: Processar de novo no interrompido', () => {
  const i = ml.indexOf("{:else if item.status === 'interrupted'}");
  assert.ok(i >= 0, 'falta o ramo interrupted nas ações');
  const [b] = botoesCom(ml.slice(i), 'data-acao="processar"');
  assert.ok(b, 'falta o botão processar');
  assert.match(b, /data-parada="reprocessar"/);
  assert.match(b, /tabindex=\{tabParada\(item\.id, 'reprocessar', paradaAtiva\)\}/);
  assert.match(b, /aria-disabled=\{[^}]*item\.source_missing[^}]*\}/);
  assert.match(b, /aria-disabled=\{[^}]*entradaBloqueada[^}]*\}/);
  assert.match(b, /aria-label=\{rotuloAcao\(HISTORICO\.processarDeNovo, item\.name\)\}/);
  assert.match(b, /HISTORICO\.origemAusente/);
  assert.match(b, /ATUALIZACAO\.soltarDepois/);
  assert.match(lista, /ReprocessQueueItem\(item\.id, config\?\.ocr_mode[^,]*, config\?\.cleaning_level[^,]*, !!config\?\.forcar_ocr\)/);
  const f = lista.slice(lista.indexOf('async function processarDeNovo('));
  const corpo = f.slice(0, f.indexOf('\n  }\n'));
  const guarda = corpo.search(/if \([^)]*entradaBloqueada[^)]*item\.source_missing[^)]*\) return;|if \([^)]*item\.source_missing[^)]*entradaBloqueada[^)]*\) return;/);
  assert.ok(guarda >= 0 && guarda < corpo.indexOf('ReprocessQueueItem('), 'clique bloqueado não pode chamar a fila');
});

test('ProcessList: Reprocessar de sucesso e erro barra a origem ausente', () => {
  const bs = botoesCom(ml, 'data-acao="reprocessar"');
  assert.equal(bs.length, 2);
  for (const b of bs) {
    assert.match(b, /aria-disabled=\{entradaBloqueada \|\| !!item\.source_missing\}/);
    assert.match(b, /item\.source_missing \? HISTORICO\.origemAusente/);
  }
  const f = lista.slice(lista.indexOf('function alternarReprocessar('));
  assert.match(f.slice(0, f.indexOf('\n  }\n')), /source_missing/);
});

test('ProcessList: TXT ausente na linha e no Copiar', () => {
  assert.match(ml, /\{#if item\.output_missing\}[\s\S]*?\{HISTORICO\.txtAusente\}/);
  const [b] = botoesCom(ml, 'data-acao="copiar"');
  assert.match(b, /aria-disabled=\{!!item\.output_missing\}/);
  assert.match(b, /item\.output_missing \? HISTORICO\.txtAusente/);
  // aria-disabled, e não disabled: o botão segue no foco itinerante da linha.
  assert.match(b, /data-parada="copiar"/);
  assert.doesNotMatch(b, /\sdisabled=/);
  assert.match(b, /on:click=\{\(\) => \{ if \(copiavel\(item\)\) handleCopy\(item\); \}\}/);
});

test('ProcessList: data, Limpar concluídos, painel e restaurado', () => {
  assert.match(ml, /\{formatarQuando\(item, agora\)\}/);
  assert.doesNotMatch(lista, /formatarHora/);
  const [lc] = botoesCom(ml, 'on:click={handleLimparConcluidos}');
  assert.match(lc, /title=\{HISTORICO\.tituloLimparConcluidos\}/);
  const f = lista.slice(lista.indexOf('function temAtividade('));
  assert.match(f.slice(0, f.indexOf('\n  }\n')), /'interrompido'/);
  assert.match(ml, /<AtividadeItem[^>]*restaurado=\{item\.restaurado\}/);
  assert.match(lista, /import \{[^}]*\bHISTORICO\b[^}]*\} from '\.\.\/lib\/textos\.js'/);
});

test('AtividadeItem: item restaurado sem instantâneo mostra a frase fixa', () => {
  assert.match(atividade, /export let restaurado = false/);
  assert.match(
    atividade,
    /\{:else if atividade === 'nenhuma'\}\s*\{#if restaurado\}\s*<p class="vazio">\{HISTORICO\.semDetalhes\}<\/p>\s*\{:else\}\s*<p class="vazio">Sem registro dos documentos: este item não chegou a ser processado\.<\/p>\s*\{\/if\}/
  );
  assert.match(atividade, /import \{[^}]*\bHISTORICO\b[^}]*\} from '\.\.\/lib\/textos\.js'/);
});

test('Settings: interruptor do histórico', () => {
  const ms = marcacao(settings);
  const i = ms.indexOf('bind:checked={config.guardar_historico}');
  assert.ok(i >= 0, 'falta o interruptor');
  const input = ms.slice(ms.lastIndexOf('<input', i), ms.indexOf('/>', i));
  assert.match(input, /role="switch"/);
  assert.match(input, /id="guardar-historico"/);
  assert.match(input, /aria-describedby="ajuda-guardar-historico"/);
  assert.match(input, /on:change=\{\(\) => handleSave\('historico'\)\}/);
  assert.match(ms, /<label for="guardar-historico">[\s\S]*?\{HISTORICO\.interruptor\}[\s\S]*?<Salvo campo="historico" \{confirmado\} \/>/);
  assert.match(ms, /<p class="help-text" id="ajuda-guardar-historico">\{HISTORICO\.fraseInterruptor\}<\/p>/);
  const padrao = settings.slice(settings.indexOf('const PADRAO = {'));
  assert.match(padrao.slice(0, padrao.indexOf('};')), /guardar_historico: false/);
  assert.match(settings, /historico: HISTORICO\.interruptor/);
});

test('App: historico:falha vira aviso', () => {
  assert.match(app, /EventsOn\('historico:falha', \(\) => aviso\.mostrar\(HISTORICO\.falhaGravar, 'aviso'\)\)/);
  assert.match(app, /import \{[^}]*\bHISTORICO\b[^}]*\} from '\.\/lib\/textos\.js'/);
});
