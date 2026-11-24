// F-VI-08 (componente único de aviso, fora da área do formulário), F-AN-06
// (confirmação de cada alteração ao lado do controle) e F-AC-07 (falhas das
// ações da fila e ZIP inválido no mesmo aviso, sem alert()).
// Rodar com: node --test frontend/avisos.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarConfirmacao, separarZips, avisoSoltura } from './src/lib/aviso.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8');
const app = ler('src', 'App.svelte');
const lista = ler('src', 'components', 'ProcessList.svelte');
const settings = ler('src', 'components', 'Settings.svelte');
const caminhoAviso = join(raiz, 'src', 'components', 'Aviso.svelte');

function estilo(texto) {
  const i = texto.indexOf('<style>');
  return i < 0 ? '' : texto.slice(i, texto.indexOf('</style>', i));
}

function funcao(texto, nome) {
  const i = texto.indexOf(`function ${nome}(`);
  assert.ok(i >= 0, `${nome} ausente`);
  const fim = texto.indexOf('\n  }\n', i);
  return texto.slice(i, fim);
}

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

// --- F-VI-08 ---------------------------------------------------------------

test('Aviso.svelte: três tipos com ícone, regiões vivas e fechar', () => {
  assert.ok(existsSync(caminhoAviso), 'Aviso.svelte ausente');
  const c = readFileSync(caminhoAviso, 'utf8');
  assert.match(c, /import CircleCheck from 'lucide-svelte\/icons\/circle-check'/);
  assert.match(c, /import CircleAlert from 'lucide-svelte\/icons\/circle-alert'/);
  assert.match(c, /<Icone icone=\{CircleCheck\} tamanho=\{12\}/);
  assert.match(c, /<Icone icone=\{CircleAlert\} tamanho=\{12\}/);
  assert.match(c, /role="status" aria-live="polite"/);
  assert.match(c, /role="alert"/);
  assert.match(c, /aria-label="Fechar aviso"/);
  assert.match(c, /in:fly=\{\{ y: 8, duration: duracaoMovimento\(DUR_BASE\), easing: cubicOut \}\}/);
  assert.match(c, /out:fade=\{\{ duration: duracaoMovimento\(DUR_FAST\) \}\}/);
  const s = estilo(c);
  assert.match(s, /\.aviso\.atencao\s*\{[^}]*--warning/);
  assert.match(s, /\.aviso\.atencao \.aviso-icone\s*\{[^}]*color:\s*var\(--warning\)/);
  assert.match(s, /\.aviso\.flutuante\s*\{[^}]*box-shadow:\s*var\(--shadow-sm\)/);
});

test('os três pontos usam o Aviso, sem CSS de toast próprio', () => {
  for (const [nome, f] of [['App', app], ['ProcessList', lista], ['Settings', settings]]) {
    assert.doesNotMatch(estilo(f), /\.(toast|aviso-rapido|aviso-erro|aviso-fechar)\b/, `${nome} repete o CSS do aviso`);
  }
  assert.match(app, /import Aviso from '\.\/components\/Aviso\.svelte'/);
  assert.match(lista, /import Aviso from '\.\/Aviso\.svelte'/);
  assert.equal((app.match(/<Aviso\b/g) || []).length, 2, 'App: aviso flutuante e aviso do pé do painel');
  assert.match(lista, /<Aviso\b/);
});

test('aviso do painel fica no pé, fora da área rolável e fora do Settings', () => {
  assert.doesNotMatch(settings, /<Aviso\b|toast-regiao/);
  const cartao = estilo(settings).match(/\.settings-card\s*\{([^}]*)\}/);
  assert.ok(cartao, '.settings-card ausente');
  assert.doesNotMatch(cartao[1], /overflow:\s*hidden/);
  const conteudo = app.indexOf('<div class="sidebar-content"');
  const pe = app.indexOf('class="painel-aviso"');
  assert.ok(conteudo >= 0 && pe > conteudo, 'pé do painel depois de .sidebar-content');
  const fimConteudo = app.lastIndexOf('</div>', pe);
  assert.ok(fimConteudo > app.indexOf('class="manual-card"'), 'o pé do painel não pode estar dentro da área rolável');
  assert.match(app, /<Settings\b[^>]*avisar=\{avisarPainel\}/);
  assert.match(settings, /export let avisar\b/);
  assert.match(estilo(app), /\.sidebar\.collapsed \.painel-aviso\s*\{[^}]*display:\s*none/);
});

// --- F-AN-06 ---------------------------------------------------------------

test('confirmação dura 1,5 s e cada alteração gera um sinal novo', () => {
  const r = relogio();
  let atual = null;
  const conf = criarConfirmacao((c) => { atual = c; }, undefined, r);
  conf.mostrar('ocr');
  const primeiro = atual;
  assert.equal(primeiro.campo, 'ocr');
  r.avancar(1000);
  conf.mostrar('ocr');
  assert.notEqual(atual.n, primeiro.n, 'salvar de novo o mesmo campo precisa de um sinal novo');
  assert.equal(r.pendentes, 1);
  r.avancar(1000);
  assert.equal(atual.campo, 'ocr');
  r.avancar(500);
  assert.equal(atual, null);
  conf.mostrar('limpeza');
  conf.limpar();
  assert.equal(r.pendentes, 0);
});

test('Settings confirma com Check ao lado do controle, sem toast de sucesso', () => {
  assert.doesNotMatch(settings, /'success'/);
  assert.doesNotMatch(settings, /salvas com sucesso/);
  assert.match(settings, /import Salvo from '\.\/Salvo\.svelte'/);
  assert.match(settings, /criarConfirmacao\(/);
  assert.match(settings, /onDestroy\(confirmacao\.limpar\)/);
  const campos = ['watchdog', 'pasta', 'ocr', 'limpeza', 'omitir', 'autostart', 'forcar'];
  for (const campo of campos) {
    assert.match(settings, new RegExp(`<Salvo campo="${campo}" \\{confirmado\\} />`), `${campo} sem confirmação`);
  }
  assert.equal((settings.match(/<Salvo\b/g) || []).length, campos.length);
  // O bloco do Check vive só no componente, não repetido em cada rótulo.
  assert.doesNotMatch(settings, /\{#key confirmado\.n\}|class="salvo/);
  assert.doesNotMatch(estilo(settings), /\.salvo\b/);
});

test('Salvo.svelte: Check de 12px que reaparece a cada confirmação', () => {
  const caminho = join(raiz, 'src', 'components', 'Salvo.svelte');
  assert.ok(existsSync(caminho), 'Salvo.svelte ausente');
  const c = readFileSync(caminho, 'utf8');
  assert.match(c, /export let campo\b/);
  assert.match(c, /export let confirmado\b/);
  assert.match(c, /import Check from 'lucide-svelte\/icons\/check'/);
  assert.match(c, /\{#if confirmado\?\.campo === campo\}/);
  assert.match(c, /\{#key confirmado\.n\}/);
  assert.match(c, /<Icone icone=\{Check\} tamanho=\{12\}/);
  // Externo só sai com fade; o interno entra com fade a cada n. Dois in:fade
  // aninhados multiplicariam as opacidades na primeira exibição.
  assert.match(c, /<span class="salvo" out:fade=\{\{ duration: duracaoMovimento\(DUR_FAST\) \}\}>/);
  assert.match(c, /<span class="salvo-icone" in:fade=\{\{ duration: duracaoMovimento\(DUR_FAST\) \}\}>/);
  assert.equal((c.match(/in:fade/g) || []).length, 1);
  const s = estilo(c);
  assert.match(s, /\.salvo\s*\{[^}]*color:\s*var\(--success\)/);
  assert.match(s, /\.salvo-icone\s*\{/);
});

test('Settings anuncia a confirmação e avisa erros pelo painel', () => {
  // A confirmação também é anunciada, numa região viva sempre montada.
  assert.match(settings, /class="sr-only" role="status" aria-live="polite"/);
  assert.match(funcao(settings, 'handleSave'), /avisar\(/);
});

// --- F-AC-07 ---------------------------------------------------------------

test('soltar arquivos: separa os ZIPs e descreve os ignorados', () => {
  assert.deepEqual(separarZips(['a.ZIP', 'b.pdf', 'c.zip', 'd.txt']), { zips: ['a.ZIP', 'c.zip'], ignorados: 2 });
  assert.deepEqual(separarZips(null), { zips: [], ignorados: 0 });
  assert.equal(avisoSoltura(2, 0), null);
  assert.deepEqual(avisoSoltura(0, 3), { texto: 'Só arquivos ZIP do eproc são aceitos. Ignorados: 3', tipo: 'error' });
  assert.deepEqual(avisoSoltura(1, 1), { texto: '1 arquivo ignorado: só arquivos ZIP do eproc são aceitos.', tipo: 'aviso' });
  assert.deepEqual(avisoSoltura(1, 2), { texto: '2 arquivos ignorados: só arquivos ZIP do eproc são aceitos.', tipo: 'aviso' });
});

test('App não usa alert() e avisa as falhas de soltar e de selecionar ZIP', () => {
  assert.doesNotMatch(app, /\balert\(/);
  assert.match(app, /separarZips\(paths\)/);
  assert.match(app, /avisoSoltura\(/);
  const sel = funcao(app, 'triggerManualSelect');
  assert.match(sel.slice(sel.indexOf('catch')), /avisarPainel\('Erro ao selecionar arquivos: '/);
});

test('ações da fila que falham avisam pelo avisoAcao', () => {
  assert.doesNotMatch(lista, /avisoCopia\b/);
  const casos = {
    handleOpenDir: 'Erro ao abrir a pasta: ',
    handleCopy: 'Erro ao copiar o TXT: ',
    handlePause: 'Erro ao pausar a fila: ',
    handleResume: 'Erro ao retomar a fila: ',
    handleCancel: 'Erro ao cancelar: ',
    handleRemove: 'Erro ao remover: ',
    handleClear: 'Erro ao limpar a fila: ',
    confirmarReprocessar: 'Erro ao reprocessar: ',
  };
  for (const [nome, texto] of Object.entries(casos)) {
    const f = funcao(lista, nome);
    const captura = f.slice(f.indexOf('catch'));
    assert.ok(captura.includes(`avisoAcao.mostrar('${texto}' + (err?.message ?? err), 'error')`), `${nome} não avisa a falha`);
  }
  assert.match(funcao(lista, 'handleCopy'), /avisoAcao\.mostrar\(alerta, 'aviso'\)/);
});
