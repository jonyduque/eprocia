// F-AC-09 (foco não se perde ao remover/cancelar/limpar), F-AC-10 (mensagem
// de erro completa acessível por teclado) e F-DF-03 (hora e filtro sem
// trabalho repetido por linha).
// Rodar com: node --test frontend/fila.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { formatarHora, filtrarItens, alvoFoco, escolherAcao, textoConfirmarLimpeza, contarConcluidos, rotuloAcao } from './src/lib/fila.js';
import { alvoAoFecharReprocessar } from './src/lib/reprocessar.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, 'src', ...p), 'utf8').replace(/\r\n/g, '\n');
const lista = ler('components', 'ProcessList.svelte');
const app = ler('App.svelte');

function tag(texto, marca) {
  const i = texto.indexOf(marca);
  assert.ok(i >= 0, `${marca} ausente`);
  const ini = texto.lastIndexOf('<', i);
  return texto.slice(ini, texto.indexOf('>', i) + 1);
}

function estilo(texto) {
  const i = texto.indexOf('<style>');
  return texto.slice(i, texto.indexOf('</style>', i));
}

function regra(css, seletor) {
  const i = css.indexOf(`${seletor} {`);
  assert.ok(i >= 0, `regra ${seletor} ausente`);
  return css.slice(i, css.indexOf('}', i));
}

// --- F-DF-03 ---------------------------------------------------------------

test('formatarHora usa pt-BR com segundos e tolera vazio/inválido', () => {
  const ts = new Date(2026, 9, 23, 9, 5, 7).getTime();
  assert.equal(formatarHora(ts), '09:05:07');
  assert.equal(formatarHora(0), '');
  assert.equal(formatarHora(undefined), '');
  assert.equal(formatarHora('não é data'), '');
});

test('formatarHora reaproveita um Intl.DateTimeFormat criado no módulo', () => {
  const fonte = ler('lib', 'fila.js');
  const corpo = fonte.slice(fonte.indexOf('export function formatarHora'));
  assert.doesNotMatch(corpo.slice(0, corpo.indexOf('\n}')), /DateTimeFormat|toLocale/);
  assert.match(fonte, /^const \w+ = new Intl\.DateTimeFormat\('pt-BR'/m);
});

test('filtrarItens devolve a mesma lista com filtro vazio', () => {
  const items = [{ name: 'a.zip' }];
  assert.equal(filtrarItens(items, ''), items);
  assert.equal(filtrarItens(items, undefined), items);
});

test('filtrarItens compara nome e mensagem sem diferenciar maiúsculas', () => {
  const items = [
    { name: 'Processo-1.zip', message: '' },
    { name: 'outro.zip', message: 'Falha no PDF' },
    { name: 'x.zip' },
  ];
  assert.deepEqual(filtrarItens(items, 'PROCESSO').map((i) => i.name), ['Processo-1.zip']);
  assert.deepEqual(filtrarItens(items, 'pdf').map((i) => i.name), ['outro.zip']);
});

test('filtrarItens converte o filtro para minúsculas uma vez só', () => {
  let chamadas = 0;
  const filtro = { toLowerCase() { chamadas++; return 'z'; } };
  filtrarItens([{ name: 'a' }, { name: 'b' }, { name: 'c' }], filtro);
  assert.equal(chamadas, 1);
});

test('ProcessList usa formatarHora e filtrarItens', () => {
  assert.match(lista, /\{formatarHora\(item\.timestamp\)\}/);
  assert.match(lista, /\$: filteredItems = filtrarItens\(items, filterText\)/);
  assert.doesNotMatch(lista, /toLocaleTimeString|function formatTime/);
});

// --- F-AC-09 ---------------------------------------------------------------

test('alvoFoco: linha que continua recebe o foco de volta', () => {
  assert.equal(alvoFoco(['a', 'b', 'c'], 'b', ['a', 'b', 'c']), 'b');
});

test('alvoFoco: linha removida passa para a seguinte, depois a anterior', () => {
  assert.equal(alvoFoco(['a', 'b', 'c'], 'b', ['a', 'c']), 'c');
  assert.equal(alvoFoco(['a', 'b', 'c'], 'c', ['a', 'b']), 'b');
  assert.equal(alvoFoco(['a', 'b', 'c', 'd'], 'b', ['a', 'd']), 'd');
});

test('alvoFoco: sem vizinhos (ou limpar fila) devolve null', () => {
  assert.equal(alvoFoco(['a'], 'a', []), null);
  assert.equal(alvoFoco(['a', 'b'], null, []), null);
});

test('escolherAcao prefere a mesma ação, depois remover/cancelar', () => {
  assert.equal(escolherAcao(['copiar', 'abrir', 'remover'], 'remover'), 'remover');
  assert.equal(escolherAcao(['remover'], 'cancelar'), 'remover');
  assert.equal(escolherAcao(['cancelar'], 'remover'), 'cancelar');
  assert.equal(escolherAcao(['detalhes', 'remover'], 'copiar'), 'remover');
  assert.equal(escolherAcao([], 'remover'), null);
});

test('título da lista recebe foco programático', () => {
  assert.match(tag(lista, '<h2'), /tabindex="-1"/);
  assert.match(tag(lista, '<h2'), /bind:this=\{tituloEl\}/);
});

test('linhas e botões de ação são localizáveis para devolver o foco', () => {
  assert.match(tag(lista, 'class="item-row'), /data-id=\{item\.id\}/);
  for (const acao of ['cancelar', 'remover', 'copiar', 'abrir', 'detalhes']) {
    assert.match(lista, new RegExp(`data-acao="${acao}"`), `data-acao="${acao}" ausente`);
  }
});

test('Cancelar, Remover e as limpezas registram para onde o foco volta', () => {
  assert.match(lista, /on:click=\{\(e\) => handleCancel\(item\.id, e\)\}/);
  assert.match(lista, /on:click=\{\(e\) => handleRemove\(item\.id, e\)\}/);
  assert.match(lista, /on:click=\{handleLimparConcluidos\}/);
  assert.match(lista, /on:click=\{handleClear\}/);
  for (const nome of ['handleCancel', 'handleRemove', 'handleClear', 'handleLimparConcluidos']) {
    const i = lista.indexOf(`async function ${nome}`);
    assert.ok(i >= 0, `${nome} ausente`);
    const corpo = lista.slice(i, lista.indexOf('\n  }\n', i));
    assert.match(corpo, /guardarFoco\(/, `${nome} não guarda o foco`);
  }
  assert.match(lista, /alvoFoco\(/);
  assert.match(lista, /tick\(\)/);
});

// --- F-AC-06 ---------------------------------------------------------------

function funcao(texto, nome) {
  const i = texto.indexOf(`function ${nome}`);
  assert.ok(i >= 0, `${nome} ausente`);
  return texto.slice(i, texto.indexOf('\n  }\n', i));
}

test('textoConfirmarLimpeza diz quantos cancela e quantos remove', () => {
  const it = (status, id) => ({ id, status });
  assert.equal(
    textoConfirmarLimpeza(['processing', 'pending', 'success', 'error', 'cancelled'].map(it)),
    'Cancelar 1 em andamento e remover 5 itens?',
  );
  assert.equal(textoConfirmarLimpeza(['success', 'error'].map(it)), 'Remover 2 itens da lista?');
  assert.equal(textoConfirmarLimpeza(['success'].map(it)), 'Remover 1 item da lista?');
  assert.equal(textoConfirmarLimpeza(['processing'].map(it)), 'Cancelar 1 em andamento e remover 1 item?');
});

test('contarConcluidos conta sucesso, erro e cancelado', () => {
  const itens = ['processing', 'pending', 'success', 'error', 'cancelled'].map((status, id) => ({ id, status }));
  assert.equal(contarConcluidos(itens), 3);
  assert.equal(contarConcluidos([]), 0);
});

test('Limpar concluídos remove sem confirmar e Limpar tudo pede confirmação em linha', () => {
  const concl = tag(lista, 'aria-label="Limpar concluídos"');
  assert.match(concl, /on:click=\{handleLimparConcluidos\}/);
  assert.match(funcao(lista, 'handleLimparConcluidos'), /await ClearFinishedQueue\(\)/);
  assert.match(lista, /\{#if concluidos > 0\}/);

  const tudo = tag(lista, 'aria-label="Limpar tudo"');
  assert.match(tudo, /on:click=\{pedirLimpeza\}/);
  assert.match(tudo, /bind:this=\{limparTudoEl\}/);
  assert.match(tudo, /aria-expanded=\{confirmandoLimpeza\}/);
  assert.doesNotMatch(funcao(lista, 'pedirLimpeza'), /ClearQueue/);
  assert.match(funcao(lista, 'handleClear'), /await ClearQueue\(\)/);
  assert.doesNotMatch(lista, /aria-label="Limpar Fila"/);

  const faixa = tag(lista, 'class="queue-notice confirmar-limpeza"');
  assert.match(faixa, /role="group"/);
  assert.match(faixa, /aria-labelledby="confirmar-limpeza-texto"/);
  assert.match(faixa, /on:keydown=\{teclaConfirmacao\}/);
  assert.match(faixa, /transition:fade=\{\{ duration: duracaoMovimento\(DUR_FAST\) \}\}/);
  const i = lista.indexOf('class="queue-notice confirmar-limpeza"');
  const bloco = lista.slice(i, lista.indexOf('{/if}', i));
  assert.match(bloco, /id="confirmar-limpeza-texto"[^>]*>\{textoConfirmarLimpeza\(items\)\}/);
  assert.match(bloco, /on:click=\{cancelarLimpeza\}[^>]*>Cancelar<\/button>/);
  assert.match(bloco, /on:click=\{handleClear\}[^>]*>Limpar tudo<\/button>/);
  assert.doesNotMatch(lista, /setTimeout\([^)]*confirmandoLimpeza/, 'a confirmação não expira');

  assert.match(funcao(lista, 'teclaConfirmacao'), /e\.key === 'Escape'[\s\S]*cancelarLimpeza\(\)/);
  assert.match(funcao(lista, 'cancelarLimpeza'), /confirmandoLimpeza = false[\s\S]*await tick\(\)[\s\S]*limparTudoEl\?\.focus\(\)/);
  assert.match(funcao(lista, 'pedirLimpeza'), /confirmandoLimpeza = true[\s\S]*await tick\(\)[\s\S]*\.focus\(\)/);
  assert.match(funcao(lista, 'handleClear'), /confirmandoLimpeza = false/);
});

// --- F-AC-12 ---------------------------------------------------------------

test('o X do item que aguarda só cancela e diz isso; a linha cancelada oferece Remover da lista', () => {
  assert.doesNotMatch(lista, /Remover da fila/);
  const i = lista.indexOf("{:else if item.status === 'pending'}");
  assert.ok(i >= 0);
  const botao = lista.slice(i, lista.indexOf('</button>', i));
  assert.match(botao, /use:dica=\{'Cancelar'\}/);
  assert.match(botao, /aria-label=\{rotuloAcao\('Cancelar', item\.name\)\}/);
  assert.match(botao, /handleCancel\(item\.id, e\)/);
  const j = lista.indexOf("{#if item.status === 'processing'}");
  assert.match(lista.slice(j, lista.indexOf('</button>', j)), /aria-label=\{rotuloAcao\('Cancelar', item\.name\)\}/);
  const k = lista.indexOf('<!-- Error or Cancelled -->');
  const resto = lista.slice(k, lista.indexOf('</div>', k));
  assert.match(resto, /data-acao="reprocessar"/);
  assert.match(resto, /data-acao="remover"[\s\S]*aria-label=\{rotuloAcao\('Remover da lista', item\.name\)\}/);
});

// --- F-AC-10 ---------------------------------------------------------------

test('detalhe do erro é um botão com aria-label e aria-expanded', () => {
  assert.doesNotMatch(lista, /class="error-tooltip-container"/);
  const i = lista.indexOf('data-acao="detalhes"');
  assert.ok(i >= 0, 'botão de detalhes ausente');
  const botao = lista.slice(lista.lastIndexOf('<', i), lista.indexOf('</button>', i));
  assert.match(botao, /^<button/);
  assert.match(botao, /type="button"/);
  assert.match(botao, /aria-label=\{rotuloAcao\('Ver detalhes do erro', item\.name\)\}/);
  assert.match(botao, /aria-expanded=/);
  assert.match(botao, /aria-controls=/);
});

test('linha expandida mostra a mensagem completa e o caminho, selecionáveis', () => {
  const i = lista.indexOf('class="erro-detalhe"');
  assert.ok(i >= 0, 'linha de detalhe ausente');
  const bloco = lista.slice(i, lista.indexOf('{/if}', i));
  assert.match(bloco, /\{item\.message/);
  assert.match(bloco, /\{item\.path\}/);
  const css = regra(estilo(lista), '.erro-detalhe');
  assert.match(css, /user-select:\s*text/);
  assert.match(css, /overflow-wrap:\s*anywhere/);
});

// O indicador virou botão de ligar/desligar: o estado vai no nome acessível e
// em aria-pressed, e a mudança é anunciada numa região viva sempre montada.
test('indicador da barra recolhida é botão com o estado no nome e aria-pressed', () => {
  const ind = tag(app, 'class="collapsed-indicator"');
  assert.match(ind, /type="button"/);
  // F-AC-09: rótulo fixo; o estado vai em aria-pressed e na descrição.
  assert.match(ind, /aria-label="Monitoramento"/);
  assert.match(ind, /aria-pressed=\{estado === 'monitorando'\}/);
  assert.match(ind, /aria-describedby="estado-monitor-recolhido"/);
  assert.match(ind, /on:click=\{alternarMonitoramento\}/);
  assert.match(app, /<div class="sr-only" role="status" aria-live="polite">\{anuncio\}<\/div>/);
  assert.match(app, /\.sr-only\s*\{[^}]*clip/);
});

// --- F-AC-04 ---------------------------------------------------------------

test('alvoAoFecharReprocessar volta a quem abriu, senão ao botão da linha', () => {
  const linha = { isConnected: true, nome: 'linha' };
  const painel = { isConnected: true, nome: 'painel' };
  assert.equal(alvoAoFecharReprocessar(painel, linha), painel);
  assert.equal(alvoAoFecharReprocessar({ isConnected: false }, linha), linha);
  assert.equal(alvoAoFecharReprocessar(null, linha), linha);
  assert.equal(alvoAoFecharReprocessar(null, null), null);
});

test('Reprocessar guarda quem abriu (linha ou painel) e devolve o foco ao fechar ou confirmar', () => {
  // Botão da linha e "Reprocessar com OCR" do painel passam o evento.
  const botoes = [...lista.matchAll(/data-acao="reprocessar"\n[\s\S]*?on:click=\{([^}]*)\}/g)].map((m) => m[1]);
  assert.equal(botoes.length, 2);
  for (const b of botoes) assert.match(b, /\(e\) => alternarReprocessar\(item\.id, e\)/);
  assert.match(lista, /aoReprocessarComOCR=\{\(e\) => reprocessarComOCR\(item\.id, e\)\}/);
  assert.match(funcao(lista, 'reprocessarComOCR'), /origemReprocessar = \{ \.\.\.origemReprocessar, \[id\]: evento\?\.currentTarget/);
  assert.match(funcao(lista, 'alternarReprocessar'), /origemReprocessar/);
  // Cancelar e Esc (ReprocessarItem chama aoCancelar nos dois) fecham pelo mesmo caminho.
  assert.match(lista, /aoCancelar=\{\(\) => fecharReprocessar\(item\.id\)\}/);
  const fechar = funcao(lista, 'fecharReprocessar');
  assert.match(fechar, /await tick\(\)[\s\S]*alvoAoFecharReprocessar\(origem, botaoReprocessarDa\(id\)\)[\s\S]*\.focus\(\)/);
  // Confirmar: o item volta para a fila e a linha troca de botões; o foco vai ao Cancelar.
  const conf = funcao(lista, 'confirmarReprocessar');
  assert.match(conf, /guardarFoco\(item\.id, \{ currentTarget: botao \}, 'cancelar'\)/);
  assert.match(funcao(lista, 'guardarFoco'), /acao: preferida \?\? botao\?\.dataset\?\.acao \?\? null/);
  const rep = ler('components', 'ReprocessarItem.svelte');
  assert.match(rep, /e\.key === 'Escape'[\s\S]*aoCancelar\(\)/);
  assert.match(rep, /class="secundario" on:click=\{aoCancelar\}/);
});

// --- F-AC-16 ---------------------------------------------------------------

test('rotuloAcao põe o nome do arquivo no nome acessível', () => {
  assert.equal(rotuloAcao('Remover da lista', '5000000-12.2026.zip'), 'Remover da lista: 5000000-12.2026');
  assert.equal(rotuloAcao('Cancelar', ''), 'Cancelar');
  assert.equal(rotuloAcao('Cancelar', undefined), 'Cancelar');
});

test('botões de ícone da linha têm aria-label com o arquivo e dica própria, sem title', () => {
  const ini = lista.indexOf('<div class="actions-col">');
  const fim = lista.indexOf('{#if item.status === \'error\' && detalhesAbertos[item.id]}');
  const trecho = lista.slice(ini, fim);
  // A tag termina num > sozinho na linha (o => do on:click não conta).
  const botoes = trecho.match(/<button\b[\s\S]*?\n\s*>/g) ?? [];
  assert.equal(botoes.length, 9, 'cancelar x2, reprocessar x2, copiar, abrir, remover x2, detalhes');
  for (const b of botoes) {
    assert.match(b, /aria-label=\{rotuloAcao\(/, b);
    assert.match(b, /item\.name\)\}/, b);
    assert.match(b, /use:dica=\{/, b);
    assert.doesNotMatch(b, /\btitle=/, b);
  }
  assert.match(lista, /import \{ dica \} from '\.\.\/lib\/dica\.js'/);
});

test('a fila é uma lista: role list e listitem', () => {
  assert.match(tag(lista, 'class="items-list"'), /role="list"/);
  assert.match(tag(lista, 'class="item-row {item.status}"'), /role="listitem"/);
});

test('erro e detalhe de cada documento aparecem inteiros, com quebra e selecionáveis', () => {
  const ativ = ler('components', 'AtividadeItem.svelte');
  const li = tag(ativ, 'class="doc {d.mais');
  assert.doesNotMatch(li, /d\.erro\}`|d\.detalhe\}`/, 'erro e detalhe não ficam só no title');
  assert.match(ativ, /<span class="doc-erro"[^>]*>\{d\.erro\}\{#if naoLido\(d\) && d\.detalhe\}<span class="doc-detalhe">\{d\.detalhe\}<\/span>\{\/if\}<\/span>/);
  const css = regra(estilo(ativ), '.doc-erro');
  assert.match(css, /white-space:\s*normal/);
  assert.match(css, /overflow-wrap:\s*anywhere/);
  assert.match(css, /user-select:\s*text/);
  assert.doesNotMatch(css, /text-overflow/);
  assert.match(regra(estilo(ativ), '.doc'), /flex-wrap:\s*wrap/);
});
