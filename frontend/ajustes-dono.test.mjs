// Ajustes pedidos pelo dono depois das capturas do relatório de processamento
// (2026-11-24): Configurações mais enxutas e título "Fila".
// Rodar com: node --test frontend/ajustes-dono.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8').replace(/\r\n/g, '\n');
const app = ler('src', 'App.svelte');
const settings = ler('src', 'components', 'Settings.svelte');
const lista = ler('src', 'components', 'ProcessList.svelte');
const design = ler('..', 'DESIGN.md');

function marcacao(texto) {
  const i = texto.indexOf('</script>');
  return texto.slice(i, texto.indexOf('<style>'));
}

test('A1: Configurações sem a linha de estado abaixo de "Monitorar a pasta"', () => {
  assert.doesNotMatch(settings, /estado-monitor/);
  assert.doesNotMatch(settings, /ROTULO_MONITORAMENTO/);
  assert.doesNotMatch(settings, /ICONE_MONITORAMENTO/);
  // O rodapé continua mostrando o estado.
  const footer = app.slice(app.indexOf('<footer>'), app.indexOf('</footer>'));
  assert.match(footer, /ROTULO_MONITORAMENTO\[estado\]/);
});

test('A2: sem o rótulo "Pasta monitorada"; caminho e Alterar com nome acessível', () => {
  const m = marcacao(settings);
  // Só como nome acessível, nunca como texto visível.
  assert.doesNotMatch(m.replace(/aria-label="[^"]*"/g, ''), /Pasta monitorada/i);
  assert.doesNotMatch(m, /rotulo-pasta/);
  assert.doesNotMatch(settings, /import Folder from/);
  const grupo = m.slice(m.indexOf('<div class="path-input-group"'), m.indexOf('>', m.indexOf('<div class="path-input-group"')) + 1);
  assert.match(grupo, /role="group"/);
  assert.match(grupo, /aria-label="Pasta monitorada"/);
  assert.match(m, /aria-label="Alterar pasta monitorada"/);
  assert.match(m, /<Salvo campo="pasta" \{confirmado\} \/>/);
  // O caminho e o botão ficam logo abaixo do interruptor.
  assert.ok(m.indexOf('id="monitored-path"') > m.indexOf('id="watchdog-toggle"'));
});

test('A3: painel sem a seção de importação manual; a fila e a barra recolhida seguem', () => {
  assert.doesNotMatch(app, /manual-card|Importação Manual|manual-select-btn/);
  assert.doesNotMatch(app, /FilePlusCorner/);
  // O botão da fila vazia continua.
  assert.match(app, /<ProcessList\b[^>]*aoSelecionarZip=\{triggerManualSelect\}/);
  assert.match(lista, /class="manual-select-btn"[^>]*on:click=\{aoSelecionarZip\}/);
  assert.doesNotMatch(design, /Importação Manual/);
});

test('Rodada 2: importar ZIPs só pela fila; a barra recolhida não oferece importação', () => {
  const ini = app.indexOf('<div class="collapsed-tools"');
  assert.ok(ini >= 0, 'barra recolhida ausente');
  const barra = app.slice(ini, app.indexOf('{/if}', app.lastIndexOf('<MenuNivel', app.indexOf('</aside>'))));
  assert.doesNotMatch(barra, /triggerManualSelect|Upload|Processar ZIP/i);
  assert.doesNotMatch(app, /collapsed-tool-btn|import Upload from/);
  // Só a fila chama a seleção de ZIPs; o arrastar para a janela continua.
  assert.equal((app.match(/\{triggerManualSelect\}/g) ?? []).length, 1);
  assert.match(app, /<ProcessList\b[^>]*aoSelecionarZip=\{triggerManualSelect\}/);
  assert.match(lista, /class="manual-select-btn"[^>]*on:click=\{aoSelecionarZip\}/);
  assert.match(app, /on:drop=\{handleWindowDrop\}/);
  // O painel lateral é só de configurações.
  assert.match(app, /id="sidebar"[\s\S]*?aria-label="Configurações"/);
  assert.doesNotMatch(app, /Configurações e importação/);
  assert.doesNotMatch(design, /atalho `Upload`|Configurações e importação/);
});

test('C/d7: o título da fila é "Fila"', () => {
  assert.match(lista, /<h2 title="Fila" tabindex="-1" bind:this=\{tituloEl\}>Fila<\/h2>/);
  assert.doesNotMatch(lista, /Fila de Processamento/);
  assert.doesNotMatch(design, /Fila de Processamento/);
});
