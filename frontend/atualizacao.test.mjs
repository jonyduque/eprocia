import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { join, dirname, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ATUALIZACAO as T } from './src/lib/textos.js';
import { textoFase, acoes, mostraProgresso, fraseModo, aceitaEntrada } from './src/lib/atualizacao.js';

const e = (o) => ({ ativo: true, fase: '', versao: '', progresso: 0, mensagem: '', modo: '', motivo: '', atual: '0.4.4', ...o });
const nenhuma = { procurar: false, atualizar: null, reiniciar: null, cancelar: false, novidades: false, paginaVersoes: false };
const A = (o, fila = false) => acoes(e(o), fila);

describe('textos da atualização', () => {
  it('textos exatos', () => {
    assert.equal(T.titulo, 'Atualizações');
    assert.equal(T.versao('1.0.0'), 'Versão 1.0.0');
    assert.equal(T.procurar, 'Procurar atualização');
    assert.equal(T.procurando, 'Procurando...');
    assert.equal(T.emDia('1.0.0'), 'O eprocIA está em dia (1.0.0)');
    assert.equal(T.disponivel('1.0.1'), 'Versão 1.0.1 disponível');
    assert.equal(T.novidades, 'Ver novidades');
    assert.equal(T.atualizar, 'Atualizar');
    assert.equal(T.baixarInstalador, 'Baixar e abrir o instalador');
    assert.equal(T.quandoFilaTerminar, 'Atualizar quando a fila terminar');
    assert.equal(T.cancelar, 'Cancelar');
    assert.equal(T.baixando(42), 'Baixando... 42%');
    assert.equal(T.conferindo, 'Conferindo o download...');
    assert.equal(T.aguardandoFila, 'A atualização começa quando a fila terminar');
    assert.equal(T.abrindoInstalador, 'Abrindo o instalador...');
    assert.equal(T.paginaVersoes, 'Abrir a página de versões no navegador');
    assert.equal(T.instalando, 'Instalando a versão nova...');
    assert.equal(T.instalada('1.0.1'), 'Versão 1.0.1 instalada');
    assert.equal(T.reiniciar, 'Reiniciar agora');
    assert.equal(T.reiniciarQuandoFilaTerminar, 'Reiniciar quando a fila terminar');
    assert.equal(T.aguardandoReinicio, 'O eprocIA reinicia quando a fila terminar');
    assert.equal(
      T.fraseInstalador,
      "O Windows vai pedir permissão de administrador; o eprocIA fecha para a instalação. Na última tela, deixe marcado 'Abrir o eprocIA' para ele voltar sozinho",
    );
    assert.equal(T.incompleta('1.0.1'), 'A versão 1.0.1 foi publicada, mas está incompleta; tente de novo mais tarde');
    assert.equal(
      T.soltarDepois,
      'O eprocIA está sendo atualizado; solte os arquivos de novo depois que ele reabrir',
    );
  });
  it('sem Voltar nem anterior', () => {
    for (const k of Object.keys(T)) {
      assert.doesNotMatch(k, /voltar|anterior/i);
    }
  });
});

describe('atualizacao', () => {
  it('fase vazia só procura', () => {
    assert.deepEqual(A({}), { ...nenhuma, procurar: true });
    assert.equal(textoFase(e({})), '');
  });
  it('verificando', () => {
    assert.deepEqual(A({ fase: 'verificando' }), nenhuma);
    assert.equal(textoFase(e({ fase: 'verificando' })), 'Procurando...');
  });
  it('em_dia', () => {
    assert.equal(textoFase(e({ fase: 'em_dia', versao: '0.4.4' })), 'O eprocIA está em dia (0.4.4)');
    assert.equal(A({ fase: 'em_dia' }).procurar, true);
    assert.equal(A({ fase: 'em_dia' }).atualizar, null);
  });
  it('disponivel trocar', () => {
    const s = { fase: 'disponivel', versao: '0.5.0', modo: 'trocar' };
    assert.equal(textoFase(e(s)), 'Versão 0.5.0 disponível');
    assert.equal(A(s).atualizar, 'Atualizar');
    // A troca dos arquivos não espera a fila: quem espera é o Reiniciar.
    assert.equal(A(s, true).atualizar, 'Atualizar');
    assert.equal(A(s).reiniciar, null);
    assert.equal(fraseModo(e(s)), null);
  });
  it('disponivel instalador', () => {
    const s = { fase: 'disponivel', versao: '0.5.0', modo: 'instalador' };
    assert.equal(A(s).atualizar, 'Baixar e abrir o instalador');
    assert.equal(A(s, true).atualizar, 'Atualizar quando a fila terminar');
    assert.equal(fraseModo(e(s)), T.fraseInstalador);
  });
  it('disponivel so_avisar', () => {
    const s = { fase: 'disponivel', versao: '0.5.0', modo: 'so_avisar', motivo: 'Instalação em pasta protegida' };
    assert.equal(A(s).atualizar, null);
    assert.equal(A(s).novidades, true);
    assert.equal(fraseModo(e(s)), 'Instalação em pasta protegida');
  });
  it('incompleta', () => {
    const s = { fase: 'incompleta', versao: '0.5.0' };
    assert.equal(textoFase(e(s)), 'A versão 0.5.0 foi publicada, mas está incompleta; tente de novo mais tarde');
    assert.equal(A(s).novidades, true);
    assert.equal(A(s).atualizar, null);
  });
  it('baixando', () => {
    const s = { fase: 'baixando', progresso: 30 };
    assert.equal(textoFase(e(s)), 'Baixando... 30%');
    assert.equal(mostraProgresso(e(s)), true);
    assert.equal(A(s).cancelar, true);
    assert.equal(A(s).procurar, false);
  });
  it('conferindo', () => {
    const s = { fase: 'conferindo' };
    assert.equal(textoFase(e(s)), 'Conferindo o download...');
    assert.equal(A(s).cancelar, false);
    assert.equal(mostraProgresso(e(s)), false);
  });
  it('instalando', () => {
    assert.equal(textoFase(e({ fase: 'instalando' })), 'Instalando a versão nova...');
    assert.deepEqual(A({ fase: 'instalando' }), nenhuma);
  });
  it('instalada', () => {
    const s = { fase: 'instalada', versao: '0.5.0' };
    assert.equal(textoFase(e(s)), 'Versão 0.5.0 instalada');
    assert.deepEqual(A(s), { ...nenhuma, reiniciar: 'Reiniciar agora' });
    assert.equal(A(s, true).reiniciar, 'Reiniciar quando a fila terminar');
  });
  it('instalada com mensagem', () => {
    const s = { fase: 'instalada', versao: '0.5.0', mensagem: 'Abre na próxima vez.' };
    assert.equal(textoFase(e(s)), 'Abre na próxima vez.');
    assert.equal(A(s).reiniciar, null);
    assert.equal(A(s).atualizar, null);
  });
  it('aguardando_fila', () => {
    assert.equal(textoFase(e({ fase: 'aguardando_fila', mensagem: 'Espera' })), 'Espera');
    assert.equal(textoFase(e({ fase: 'aguardando_fila', modo: 'trocar' })), 'O eprocIA reinicia quando a fila terminar');
    assert.equal(
      textoFase(e({ fase: 'aguardando_fila', modo: 'instalador' })),
      'A atualização começa quando a fila terminar',
    );
    assert.equal(A({ fase: 'aguardando_fila' }).cancelar, true);
  });
  it('reiniciando e abrindo_instalador', () => {
    assert.equal(textoFase(e({ fase: 'reiniciando', mensagem: 'Reiniciando' })), 'Reiniciando');
    assert.equal(textoFase(e({ fase: 'reiniciando' })), 'Reiniciando...');
    assert.equal(T.reiniciando, 'Reiniciando...');
    assert.equal(textoFase(e({ fase: 'abrindo_instalador' })), 'Abrindo o instalador...');
    assert.equal(textoFase(e({ fase: 'abrindo_instalador', mensagem: 'X' })), 'X');
    assert.deepEqual(A({ fase: 'reiniciando' }), nenhuma);
    assert.deepEqual(A({ fase: 'abrindo_instalador' }), nenhuma);
  });
  it('erro sem modo', () => {
    const s = { fase: 'erro', mensagem: 'Sem conexão' };
    assert.equal(textoFase(e(s)), 'Sem conexão');
    assert.equal(A(s).paginaVersoes, true);
    assert.equal(A(s).atualizar, null);
    assert.equal(A(s).novidades, false);
  });
  it('erro com modo', () => {
    for (const modo of ['trocar', 'instalador']) {
      const s = { fase: 'erro', mensagem: 'Falhou', modo };
      assert.equal(textoFase(e(s)), 'Falhou');
      assert.equal(A(s).novidades, true);
      assert.notEqual(A(s).atualizar, null);
      assert.equal(A(s).paginaVersoes, false);
    }
  });
  it('erro com fila ocupada: só o instalador espera a fila', () => {
    assert.equal(A({ fase: 'erro', mensagem: 'Falhou', modo: 'trocar' }, true).atualizar, 'Atualizar');
    assert.equal(
      A({ fase: 'erro', mensagem: 'Falhou', modo: 'instalador' }, true).atualizar,
      'Atualizar quando a fila terminar',
    );
  });
  it('inativo', () => {
    const s = e({ ativo: false, fase: 'disponivel', modo: 'trocar', versao: '1' });
    assert.deepEqual(acoes(s, false), nenhuma);
    assert.equal(textoFase(s), '');
  });
  it('reiniciar só em instalada', () => {
    for (const fase of ['', 'verificando', 'em_dia', 'disponivel', 'incompleta', 'baixando', 'conferindo', 'instalando',
      'aguardando_fila', 'reiniciando', 'abrindo_instalador', 'erro']) {
      assert.equal(A({ fase, modo: 'trocar' }).reiniciar, null, fase);
    }
  });
  it('aceitaEntrada', () => {
    assert.equal(aceitaEntrada('reiniciando'), false);
    assert.equal(aceitaEntrada('abrindo_instalador'), false);
    for (const f of ['', 'verificando', 'em_dia', 'disponivel', 'incompleta', 'baixando', 'conferindo', 'instalando',
      'instalada', 'aguardando_fila', 'erro']) {
      assert.equal(aceitaEntrada(f), true, f);
    }
  });
});

// Task 17: convenções da interface, lidas do fonte.
const raiz = dirname(fileURLToPath(import.meta.url));
const caminho = (...p) => join(raiz, 'src', ...p);
const lerFonte = (...p) => (existsSync(caminho(...p)) ? readFileSync(caminho(...p), 'utf8').replace(/\r\n/g, '\n') : '');

// Arquivos de frontend/src, fora dos bindings gerados (wailsjs).
function fontes(dir = caminho()) {
  const r = [];
  for (const nome of readdirSync(dir)) {
    const p = join(dir, nome);
    if (statSync(p).isDirectory()) {
      if (nome !== 'wailsjs') r.push(...fontes(p));
    } else if (/\.(svelte|js|ts|css|html)$/.test(nome)) {
      r.push(p);
    }
  }
  return r;
}
const nomeRel = (p) => relative(caminho(), p).split(/[\\/]/).join('/');
const comTexto = (t) => fontes().filter((p) => readFileSync(p, 'utf8').includes(t)).map(nomeRel);

// Tag de abertura a partir de `ini`: até o primeiro > fora de chaves.
function tagEm(src, ini) {
  let prof = 0;
  for (let i = ini; i < src.length; i++) {
    const c = src[i];
    if (c === '{') prof++;
    else if (c === '}') prof--;
    else if (c === '>' && prof === 0) return src.slice(ini, i + 1);
  }
  return src.slice(ini);
}

// Tags <button ...> que contêm o trecho dado.
function botoesCom(src, trecho) {
  const r = [];
  let i = src.indexOf('<button');
  while (i >= 0) {
    const t = tagEm(src, i);
    if (t.includes(trecho)) r.push(t);
    i = src.indexOf('<button', i + 1);
  }
  return r;
}

// Trecho de `assinatura` até a chave (ou parêntese) que fecha o primeiro bloco aberto.
function corpoFuncao(src, assinatura) {
  const ini = src.indexOf(assinatura);
  assert.ok(ini >= 0, `${assinatura} ausente`);
  let prof = 0;
  for (let i = src.indexOf('{', ini); i < src.length; i++) {
    if (src[i] === '{') prof++;
    else if (src[i] === '}' && --prof === 0) return src.slice(ini, i + 1);
  }
  return src.slice(ini);
}

// O binding só é chamado dentro de funções disparadas por on:click, nunca na
// montagem, num bloco reativo nem num callback de evento.
function soPorClique(src, binding) {
  const usos = [...src.matchAll(new RegExp(`\\b${binding}\\(`, 'g'))].map((m) => m.index);
  assert.ok(usos.length > 0, `${binding} não é chamado`);
  for (const i of usos) {
    const m = [...src.slice(0, i).matchAll(/(?:async\s+)?function\s+(\w+)\s*\(/g)].at(-1);
    assert.ok(m, `${binding} fora de função`);
    const corpo = corpoFuncao(src, m[0]);
    assert.ok(src.indexOf(m[0]) + corpo.length > i, `${binding} fora do corpo de ${m[1]}`);
    const nome = m[1];
    assert.match(src, new RegExp(`on:click=\\{(?:${nome}\\}|\\(\\) => ${nome}\\()`), `${nome} não está num on:click`);
    for (const linha of src.split('\n').filter((l) => /^\s*\$:/.test(l))) {
      assert.ok(!linha.includes(nome) && !linha.includes(binding), `${binding} num bloco reativo`);
    }
  }
  for (const gancho of ['onMount(', 'EventsOn(']) {
    let j = src.indexOf(gancho);
    while (j >= 0) {
      assert.ok(!corpoFuncao(src.slice(j), gancho).includes(binding), `${binding} dentro de ${gancho}`);
      j = src.indexOf(gancho, j + 1);
    }
  }
}

describe('interface da atualização', () => {
  const app = lerFonte('App.svelte');
  const lista = lerFonte('components', 'ProcessList.svelte');
  const reprocessar = lerFonte('components', 'ReprocessarItem.svelte');
  const settings = lerFonte('components', 'Settings.svelte');
  const bloco = lerFonte('components', 'Atualizacoes.svelte');

  it('App: soltar arquivos confere aceitaEntrada antes de AddDroppedFiles e avisa', () => {
    const ini = app.indexOf('OnFileDrop(');
    const cb = app.slice(ini, app.indexOf('}, false)', ini));
    const a = cb.indexOf('aceitaEntrada(');
    const add = cb.indexOf('AddDroppedFiles(');
    assert.ok(a >= 0, 'aceitaEntrada ausente no OnFileDrop');
    assert.ok(a < add, 'aceitaEntrada depois de AddDroppedFiles');
    assert.match(cb.slice(a, add), /aviso\.mostrar\([\w.]*soltarDepois/);
  });

  it('App: seleção manual confere antes do diálogo e de novo antes de pôr na fila', () => {
    const f = corpoFuncao(app, 'async function triggerManualSelect');
    const a1 = f.indexOf('aceitaEntrada(');
    const sel = f.indexOf('SelectZipFiles(');
    const add = f.indexOf('AddFilesToQueue(');
    assert.ok(a1 >= 0 && a1 < sel, 'aceitaEntrada antes de SelectZipFiles');
    const a2 = f.indexOf('aceitaEntrada(', sel);
    assert.ok(a2 > sel && a2 < add, 'aceitaEntrada entre SelectZipFiles e AddFilesToQueue');
    assert.match(f, /avisarPainel\([\w.]*soltarDepois/);
  });

  it('App: estado inicial, evento e props', () => {
    assert.match(corpoFuncao(app, 'onMount('), /EstadoAtualizacao\(\)/);
    assert.match(app, /EventsOn\('atualizacao:estado'/);
    assert.match(app, /<ProcessList[^>]*entradaBloqueada=\{!aceitaEntrada\(estadoAtualizacao\.fase\)\}/);
    assert.match(app, /<Settings[^>]*\{estadoAtualizacao\}/);
    assert.match(app, /<Settings[^>]*\{filaOcupada\}/);
  });

  it('ProcessList: os dois Reprocessar da linha bloqueados com a dica da troca', () => {
    const bs = botoesCom(lista, 'data-acao="reprocessar"');
    assert.equal(bs.length, 2);
    for (const b of bs) {
      // aria-disabled, e não disabled: o botão segue no foco itinerante da linha.
      assert.match(b, /aria-disabled=\{entradaBloqueada\}/);
      assert.doesNotMatch(b, /\sdisabled=/);
      assert.match(b, /use:dica=\{entradaBloqueada \? ATUALIZACAO\.soltarDepois : /);
      assert.doesNotMatch(b, /\btitle=/, 'a explicação vai na dica, não no title');
    }
    const f = corpoFuncao(lista, 'function alternarReprocessar(');
    // Só barra abrir: fechar pelo ícone um formulário já aberto continua funcionando.
    assert.match(f, /^function alternarReprocessar\([^)]*\) \{\s*const abrir = !reprocessarAberto\[id\];\s*(?:\/\/[^\n]*\n\s*)?if \(entradaBloqueada && abrir\) return;/);
    assert.doesNotMatch(f, /if \(entradaBloqueada\) return;/);
    assert.match(lista, /\.btn-action\[aria-disabled='true'\] \{[^}]*opacity/);
  });

  it('ProcessList: Ler com OCR bloqueado por aria-disabled e com a dica da troca', () => {
    const [b] = botoesCom(lista, 'class="btn-ler-ocr"');
    assert.ok(b, 'falta o botão btn-ler-ocr');
    assert.match(b, /aria-disabled=\{!!lendoOCR\[item\.id\] \|\| entradaBloqueada\}/);
    assert.match(b, /use:dica=\{entradaBloqueada \? ATUALIZACAO\.soltarDepois : /);
    assert.doesNotMatch(b, /\sdisabled=\{/);
  });

  it('ProcessList: lerComOCR não chama a fila com a entrada bloqueada', () => {
    const f = corpoFuncao(lista, 'async function lerComOCR');
    const b = f.indexOf('entradaBloqueada');
    assert.ok(b >= 0 && b < f.indexOf('ReprocessQueueItem('));
    assert.match(lista, /export let entradaBloqueada = false/);
    assert.match(tagEm(lista, lista.indexOf('<ReprocessarItem')), /\{entradaBloqueada\}/);
  });

  it('ReprocessarItem: Reprocessar desativado com a dica da troca', () => {
    assert.match(reprocessar, /export let entradaBloqueada = false/);
    const [b] = botoesCom(reprocessar, 'type="submit"');
    assert.match(b, /aria-disabled=\{entradaBloqueada\}/);
    assert.doesNotMatch(b, /\sdisabled=\{[^}]*entradaBloqueada/);
    assert.match(b, /use:dica=\{entradaBloqueada \? ATUALIZACAO\.soltarDepois : /);
    assert.match(reprocessar, /\.primario\[aria-disabled='true'\]/);
    assert.match(corpoFuncao(reprocessar, 'async function confirmar'), /entradaBloqueada/);
  });

  it('VerificarAtualizacao e ReiniciarAtualizacao: só no bloco, só por clique', () => {
    assert.deepEqual(comTexto('VerificarAtualizacao'), ['components/Atualizacoes.svelte']);
    assert.deepEqual(comTexto('ReiniciarAtualizacao'), ['components/Atualizacoes.svelte']);
    soPorClique(bloco, 'VerificarAtualizacao');
    soPorClique(bloco, 'ReiniciarAtualizacao');
  });

  it('nenhum endereço de download nem da API no frontend', () => {
    assert.deepEqual(comTexto('releases/download'), []);
    assert.deepEqual(comTexto('api.github.com'), []);
  });

  it('sem voltar para a versão anterior (d5)', () => {
    assert.deepEqual(comTexto('VoltarVersaoAnterior'), []);
    assert.deepEqual(comTexto('Voltar para a versão anterior'), []);
  });

  it('Atualizacoes.svelte: sem cor literal e com os textos de textos.js', () => {
    assert.ok(bloco, 'Atualizacoes.svelte ausente');
    const semVar = bloco.replace(/var\([^)]*\)/g, '');
    assert.doesNotMatch(semVar, /#[0-9a-f]{3,6}\b/i);
    assert.doesNotMatch(semVar, /rgba?\(/);
    for (const t of ['Procurar atualização', 'Ver novidades', 'Reiniciar agora']) assert.ok(!bloco.includes(t), t);
    assert.match(bloco, /export let estado\b/);
    assert.match(bloco, /export let filaOcupada\b/);
    assert.match(bloco, /export let avisar\b/);
    for (const b of ['InstalarAtualizacao', 'CancelarAtualizacao', 'AbrirNovidades', 'AbrirPaginaVersoes']) {
      assert.match(bloco, new RegExp(`\\b${b}\\(`), b);
    }
  });

  it('Atualizacoes.svelte: erro que o bloco já mostra não se repete no aviso', () => {
    const f = corpoFuncao(bloco, 'async function executar');
    const t = f.indexOf('await tick()');
    const cond = f.search(/estado\?\.fase === 'erro' && estado\.mensagem === msg/);
    const av = f.indexOf('avisar(');
    assert.ok(t >= 0 && cond > t && av > cond, 'tick, depois a comparação, depois avisar');
    assert.match(bloco, /import \{[^}]*\btick\b[^}]*\} from 'svelte'/);
  });

  it('Settings: quarta categoria Atualizações no molde das outras', () => {
    const ini = settings.indexOf('data-bloco="atualizacoes"');
    assert.ok(ini >= 0, 'bloco atualizacoes ausente');
    assert.match(settings, /\{#if estadoAtualizacao\?\.ativo\}\s*<div class="grupo" data-bloco="atualizacoes">/);
    const resto = settings.slice(ini);
    const cab = resto.slice(0, resto.indexOf('id="grupo-atualizacoes"'));
    for (const t of ['class="cabecalho-grupo"', 'aria-expanded={!recolhidas.atualizacoes}', 'aria-controls="grupo-atualizacoes"', "alternar('atualizacoes')"]) {
      assert.ok(cab.includes(t), t);
    }
    assert.match(resto, /<div class="conteudo-grupo" class:animar=\{animar\.atualizacoes\} id="grupo-atualizacoes" hidden=\{recolhidas\.atualizacoes\}>\s*<Atualizacoes\b/);
    assert.match(settings, /export let estadoAtualizacao\b/);
    assert.match(settings, /export let filaOcupada\b/);
  });
});
