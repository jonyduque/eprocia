// F-AC-03, F-AC-08 e F-AC-09: pasta monitorada sem lixeira, vocabulário único
// (Monitorando / Pausado / Sem pasta) com os mesmos ícones no painel, no
// rodapé e na barra recolhida, e o primeiro uso resolvido onde o olho está.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  ROTULO_MONITORAMENTO,
  estadoMonitoramento,
  textoMonitoramento,
  truncarMeio
} from './src/lib/monitoramento.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8').replace(/\r\n/g, '\n');
const app = ler('src', 'App.svelte');
const settings = ler('src', 'components', 'Settings.svelte');
const lista = ler('src', 'components', 'ProcessList.svelte');
const design = ler('..', 'DESIGN.md');

function bloco(texto, ini, fim) {
  const i = texto.indexOf(ini);
  assert.ok(i >= 0, `${ini} ausente`);
  return texto.slice(i, texto.indexOf(fim, i) + fim.length);
}

function tag(texto, marca) {
  const i = texto.indexOf(marca);
  assert.ok(i >= 0, `${marca} ausente`);
  const ini = texto.lastIndexOf('<', i);
  return texto.slice(ini, texto.indexOf('>', i) + 1);
}

const pasta = 'C:\\Usuarios\\exemplo\\Downloads\\Processos';

test('estado do monitoramento: monitorando, pausado ou sem pasta', () => {
  assert.equal(estadoMonitoramento({ monitored_path: pasta, watchdog_enabled: true }), 'monitorando');
  assert.equal(estadoMonitoramento({ monitored_path: pasta, watchdog_enabled: false }), 'pausado');
  // Sem pasta não há o que monitorar, mesmo com o interruptor gravado ligado.
  assert.equal(estadoMonitoramento({ monitored_path: '', watchdog_enabled: true }), 'sem_pasta');
  assert.equal(estadoMonitoramento({ monitored_path: '', watchdog_enabled: false }), 'sem_pasta');
  assert.equal(estadoMonitoramento(null), 'sem_pasta');
  assert.deepEqual(ROTULO_MONITORAMENTO, { monitorando: 'Monitorando', pausado: 'Pausado', sem_pasta: 'Sem pasta' });
});

test('texto do monitoramento usa o mesmo vocabulário', () => {
  assert.equal(textoMonitoramento({ monitored_path: pasta, watchdog_enabled: true }), `Monitorando: ${pasta}`);
  assert.equal(textoMonitoramento({ monitored_path: pasta, watchdog_enabled: false }), `Pausado: ${pasta}`);
  assert.equal(textoMonitoramento({ monitored_path: '', watchdog_enabled: true }), 'Sem pasta');
});

test('truncarMeio põe as reticências no meio e preserva o fim do caminho', () => {
  const porTamanho = (n) => (s) => s.length <= n;
  assert.equal(truncarMeio(pasta, porTamanho(100)), pasta);
  assert.equal(truncarMeio('', porTamanho(5)), '');
  const curto = truncarMeio(pasta, porTamanho(24));
  assert.ok(curto.length <= 24, curto);
  assert.equal(curto.length, 24, 'aproveita o espaço todo');
  assert.match(curto, /^C:\\Usu.*….*\\Processos$/);
  assert.equal((curto.match(/…/g) || []).length, 1);
  // Mais do fim que do começo: o nome da pasta é o que diferencia.
  const [ini, fim] = curto.split('…');
  assert.ok(fim.length > ini.length, curto);
  // Também aceita um número de caracteres.
  assert.equal(truncarMeio(pasta, 24), curto);
  // Espaço para nada: só as reticências.
  assert.equal(truncarMeio(pasta, porTamanho(1)), '…');
});

test('F-AC-03: pasta sem lixeira, como texto, e Alterar/Escolher pasta', () => {
  assert.doesNotMatch(settings, /Trash2|trash-2|handleClearDirectory|Desativar monitoramento|btn-danger-icon/);
  assert.doesNotMatch(settings, /readonly/);
  assert.doesNotMatch(settings, /Nenhuma pasta \(inativo\)|Monitoramento ativo/);
  const caminho = tag(settings, 'use:reticenciasMeio=');
  assert.match(caminho, /id="monitored-path"/);
  assert.match(caminho, /^<span/);
  assert.match(caminho, /title=\{config\.monitored_path/);
  assert.match(caminho, /use:reticenciasMeio=/);
  assert.match(settings, /import \{[^}]*reticenciasMeio[^}]*\} from '\.\.\/lib\/monitoramento\.js'/);
  assert.match(settings, /Escolher pasta\.\.\./);
  assert.match(settings, /import FolderOpen from 'lucide-svelte\/icons\/folder-open'/);
  assert.match(settings, />\s*Alterar\s*</);
});

test('F-AC-03: sem pasta, interruptor desligado e desabilitado com ajuda ligada', () => {
  const sw = tag(settings, 'id="watchdog-toggle"');
  assert.match(sw, /role="switch"/);
  assert.match(sw, /disabled=\{semPasta\}/);
  assert.match(sw, /checked=\{estado === 'monitorando'\}/);
  assert.doesNotMatch(sw, /bind:checked/);
  assert.match(sw, /aria-describedby=\{semPasta \? 'ajuda-monitoramento' : undefined\}/);
  assert.match(settings, /id="ajuda-monitoramento"[^>]*>Escolha uma pasta para monitorar</);
});

test('F-AC-03/F-AC-09: painel mostra o estado com o ícone e o rótulo únicos', () => {
  for (const f of [settings, app]) {
    assert.match(f, /import FolderSearch from 'lucide-svelte\/icons\/folder-search'/);
    assert.match(f, /import CirclePause from 'lucide-svelte\/icons\/circle-pause'/);
    assert.match(f, /import FolderX from 'lucide-svelte\/icons\/folder-x'/);
    assert.match(f, /ROTULO_MONITORAMENTO\[estado\]/);
  }
});

test('F-AC-09: rodapé com botão de ícone, alvo 24px e rótulo fixo', () => {
  const footer = bloco(app, '<footer>', '</footer>');
  assert.doesNotMatch(footer, /status-dot/);
  const btn = tag(footer, 'class="monitor-btn"');
  assert.match(btn, /aria-label="Monitoramento"/);
  assert.match(btn, /aria-pressed=\{estado === 'monitorando'\}/);
  assert.match(btn, /on:click=\{alternarMonitoramento\}/);
  assert.match(footer, /<Icone icone=\{ICONE_MONITORAMENTO\[estado\]\} tamanho=\{12\} \/>/);
  const css = app.slice(app.indexOf('<style>'));
  const regra = css.slice(css.indexOf('.monitor-btn {'), css.indexOf('}', css.indexOf('.monitor-btn {')));
  assert.match(regra, /width:\s*24px/);
  assert.match(regra, /height:\s*24px/);
  assert.match(css, /\.monitor-btn:hover\s*\{[^}]*--surface-hover/);
  assert.doesNotMatch(app, /\.status-dot|\.collapsed-dot/);
});

test('F-AC-09: barra recolhida com ícone de 16px, rótulo fixo e estado descrito', () => {
  const ind = tag(app, 'class="collapsed-indicator"');
  assert.match(ind, /aria-label="Monitoramento"/);
  assert.match(ind, /aria-pressed=\{estado === 'monitorando'\}/);
  assert.match(ind, /aria-describedby="estado-monitor-recolhido"/);
  const i = app.indexOf('class="collapsed-indicator"');
  const corpo = app.slice(i, app.indexOf('</button>', i));
  assert.match(corpo, /<Icone icone=\{ICONE_MONITORAMENTO\[estado\]\} \/>/);
  assert.match(app, /id="estado-monitor-recolhido"[^>]*>\{textoMonitor\}</);
});

test('F-AC-08: rodapé sem pasta oferece o botão de texto, sem mandar ao painel', () => {
  const footer = bloco(app, '<footer>', '</footer>');
  assert.doesNotMatch(app, /Configure uma pasta no painel/);
  const btn = tag(footer, 'class="status-link"');
  assert.match(btn, /type="button"/);
  assert.match(btn, /on:click=\{alternarMonitoramento\}/);
  assert.match(footer, /class="status-link"[^>]*>Escolher pasta para monitorar</);
  assert.match(footer, /Monitorando: <strong>/);
  assert.match(footer, /Pausado: <strong>/);
});

test('F-AC-08: estado vazio da fila tem o botão de selecionar ZIP', () => {
  assert.doesNotMatch(lista, /no painel ao lado/);
  assert.match(lista, /export let aoSelecionarZip = /);
  const i = lista.indexOf('Nenhum processo no histórico');
  const vazio = lista.slice(i, lista.indexOf('{:else}', i));
  assert.match(vazio, /class="manual-select-btn"[^>]*on:click=\{aoSelecionarZip\}|on:click=\{aoSelecionarZip\}[^>]*class="manual-select-btn"/);
  assert.match(vazio, /<Icone icone=\{Upload\} tamanho=\{14\} \/>/);
  assert.match(vazio, /Selecionar arquivos ZIP\.\.\./);
  assert.match(vazio, /ou arraste os ZIPs para a janela/);
  assert.match(lista, /import Upload from 'lucide-svelte\/icons\/upload'/);
  assert.match(app, /<ProcessList[^>]*aoSelecionarZip=\{triggerManualSelect\}/);
});

test('DESIGN.md registra o vocabulário, os ícones e as reticências no meio', () => {
  for (const t of ['Monitorando', 'Pausado', 'Sem pasta', 'FolderSearch', 'FolderX', 'reticências no meio']) {
    assert.ok(design.includes(t), t);
  }
  assert.doesNotMatch(design, /O ponto da barra recolhida/);
});
