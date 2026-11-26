// PDF avulso (spec 2026-11-18-pdf-avulso-design.md): ícone de tipo na linha,
// nome do item de vários PDFs e a opção de converter PDFs da pasta monitorada.
// Rodar com: node --test frontend/pdf-avulso.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { nomeExibido } from './src/lib/fila.js';
import { AJUDA_MONITORAR_PDF, ROTULO_MONITORAR_PDF, TIPO_ENTRADA } from './src/lib/textos.js';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8').replace(/\r\n/g, '\n');
const lista = ler('src', 'components', 'ProcessList.svelte');
const settings = ler('src', 'components', 'Settings.svelte');

test('textos do PDF avulso', () => {
  assert.equal(ROTULO_MONITORAR_PDF, 'Também converter PDFs novos nesta pasta');
  assert.equal(AJUDA_MONITORAR_PDF, 'Cada PDF que aparecer na pasta ganha um TXT ao lado, inclusive boletos e outros PDFs que não são de processo');
  assert.deepEqual(TIPO_ENTRADA, { zip: 'ZIP', pdf: 'PDF' });
});

test('linha da fila: ícone por tipo antes do nome, com o tipo em sr-only', () => {
  assert.match(lista, /import File from 'lucide-svelte\/icons\/file'/);
  assert.match(lista, /import FileArchive from 'lucide-svelte\/icons\/file-archive'/);
  assert.match(lista, /const ICONE_TIPO = \{ zip: FileArchive, pdf: File \}/);
  assert.match(lista, /\{@const tipo = item\.kind === 'pdf' \? 'pdf' : 'zip'\}/);
  assert.match(lista, /import \{[^}]*\bTIPO_ENTRADA\b[^}]*\} from '\.\.\/lib\/textos\.js'/);
  const i = lista.indexOf('class="info-col"');
  assert.ok(i >= 0, 'info-col ausente');
  const nome = lista.indexOf('class="file-name"', i);
  const icone = lista.indexOf('<Icone icone={ICONE_TIPO[tipo]} tamanho={12}', i);
  assert.ok(icone > i && icone < nome, 'ícone de tipo fora de info-col ou depois do nome');
  const trecho = lista.slice(icone, nome);
  assert.match(trecho, /cor="var\(--text-secondary\)"/);
  assert.match(trecho, /<span class="sr-only">\{TIPO_ENTRADA\[tipo\]\}<\/span>/);
});

test('nome de item de vários PDFs volta igual', () => {
  assert.equal(nomeExibido('5000000-00.2026.8.26.0001 (3 PDFs)'), '5000000-00.2026.8.26.0001 (3 PDFs)');
  assert.equal(nomeExibido('5000000-00.2026.8.26.0001.zip'), '5000000-00.2026.8.26.0001');
});

test('Configurações: interruptor de PDFs na pasta depois do grupo da pasta monitorada', () => {
  assert.match(settings, /const PADRAO = \{[^}]*monitorar_pdf: false/);
  assert.match(settings, /import FileSearch from 'lucide-svelte\/icons\/file-search'/);
  const id = settings.indexOf('id="monitorar-pdf"');
  assert.ok(id >= 0, 'input monitorar-pdf ausente');
  // O on:change tem "=>", então o fim da tag é o "/>" e não o primeiro ">".
  const inp = settings.slice(settings.lastIndexOf('<input', id), settings.indexOf('/>', id) + 2);
  assert.match(inp, /type="checkbox"/);
  assert.match(inp, /role="switch"/);
  assert.match(inp, /bind:checked=\{config\.monitorar_pdf\}/);
  assert.match(inp, /aria-describedby="ajuda-monitorar-pdf"/);
  assert.match(inp, /on:change=\{\(\) => handleSave\('monitorarPdf'\)\}/);
  assert.doesNotMatch(inp, /disabled/, 'o interruptor fica habilitado com o monitoramento desligado');
  const rotulo = settings.match(/<label for="monitorar-pdf">[\s\S]*?<\/label>/)?.[0];
  assert.ok(rotulo, 'rótulo ausente');
  assert.match(rotulo, /<Icone icone=\{FileSearch\} tamanho=\{14\} \/>/);
  assert.match(rotulo, /\{ROTULO_MONITORAR_PDF\}/);
  assert.match(rotulo, /<Salvo campo="monitorarPdf" \{confirmado\} \/>/);
  assert.match(settings, /<p class="help-text" id="ajuda-monitorar-pdf">\{AJUDA_MONITORAR_PDF\}<\/p>/);
  assert.match(settings, /monitorarPdf: ROTULO_MONITORAR_PDF/);
  const pasta = settings.indexOf('aria-label="Pasta monitorada"');
  assert.ok(pasta >= 0 && settings.indexOf('id="monitorar-pdf"') > pasta, 'interruptor antes do grupo da pasta');
  assert.ok(settings.indexOf('id="monitorar-pdf"') < settings.indexOf('id="grupo-processamento"'), 'fora do Monitoramento');
});
