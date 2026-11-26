// F-VI-11: ícones do lucide-svelte por um componente com traço e tamanho
// padrão, no lugar dos SVGs copiados à mão.
// Rodar com: node --test frontend/icones.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = dirname(fileURLToPath(import.meta.url));
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8');
const arquivos = {
  App: ler('src', 'App.svelte'),
  ProcessList: ler('src', 'components', 'ProcessList.svelte'),
  Settings: ler('src', 'components', 'Settings.svelte')
};
const caminhoIcone = join(raiz, 'src', 'components', 'Icone.svelte');
const design = ler('..', 'DESIGN.md');

function marcacao(texto) {
  const i = texto.indexOf('</script>');
  return texto.slice(i, texto.indexOf('<style>', i));
}

test('Icone.svelte aplica tamanho 16, traço 1.75 e aria-hidden', () => {
  assert.ok(existsSync(caminhoIcone), 'Icone.svelte ausente');
  const c = readFileSync(caminhoIcone, 'utf8');
  assert.match(c, /export let tamanho = 16/);
  assert.match(c, /strokeWidth=\{1\.75\}/);
  assert.match(c, /aria-hidden="true"/);
});

test('nenhum SVG à mão nos componentes, exceto a marca', () => {
  for (const [nome, f] of Object.entries(arquivos)) {
    const svgs = marcacao(f).match(/<svg[^>]*>/g) ?? [];
    const fora = svgs.filter((s) => !s.includes('class="brand-icon"'));
    assert.equal(fora.length, 0, `${nome} ainda tem ${fora.length} SVG(s) inline`);
  }
});

test('ícones importados um a um de lucide-svelte/icons', () => {
  for (const [nome, f] of Object.entries(arquivos)) {
    assert.doesNotMatch(f, /from 'lucide-svelte'/, `${nome}: import do barril inteiro`);
    const usos = marcacao(f).match(/<Icone\b[^>]*>/g) ?? [];
    assert.ok(usos.length > 0, `${nome} sem <Icone>`);
    for (const u of usos) {
      // Um mapa de estado para ícone (ICONE_MONITORAMENTO[estado]) vale se
      // cada valor do mapa for um ícone importado.
      const mapa = u.match(/icone=\{(\w+)\[\w+\]\}/);
      const nomes = mapa
        ? [...f.match(new RegExp(`const ${mapa[1]} = \\{([^}]*)\\}`))[1].matchAll(/:\s*(\w+)/g)].map((x) => x[1])
        : [u.match(/icone=\{(\w+)\}/)?.[1]];
      for (const n of nomes) {
        assert.ok(n, u);
        assert.match(f, new RegExp(`import ${n} from 'lucide-svelte/icons/[a-z0-9-]+'`), `${n} sem import`);
      }
    }
    const semMarca = marcacao(f).replace(/<svg[^>]*class="brand-icon"[\s\S]*?<\/svg>/, '');
    assert.doesNotMatch(semMarca, /stroke-width|strokeWidth/, `${nome} sobrescreve o traço`);
  }
});

test('estados da fila usam o mapeamento do DESIGN', () => {
  const lista = arquivos.ProcessList;
  const i = lista.indexOf('<div class="status-col">');
  const col = lista.slice(i, lista.indexOf('</div>\n\n', i));
  for (const ic of ['LoaderCircle', 'CirclePause', 'Clock', 'CircleCheck', 'CircleAlert', 'CircleSlash']) {
    assert.match(col, new RegExp(`icone=\\{${ic}\\}`), `${ic} ausente no status`);
  }
  assert.match(col, /class="spinner-small"[^>]*aria-hidden="true"[^>]*>\s*<Icone icone=\{LoaderCircle\}/);
  for (const velho of ['alert-triangle', 'check-circle', 'lucide-ban']) {
    assert.doesNotMatch(lista, new RegExp(velho), `${velho} ainda presente`);
  }
});

// F-AC-03: a lixeira do painel (que apagava a pasta) saiu; fica a da fila.
test('lixeira é Trash2 em todo lugar', () => {
  for (const [nome, f] of Object.entries(arquivos)) {
    assert.doesNotMatch(f, /icons\/trash'/, `${nome} usa trash`);
  }
  assert.match(arquivos.ProcessList, /import Trash2 from 'lucide-svelte\/icons\/trash-2'/);
});

test('seletores de svg alcançam o filho via :global', () => {
  for (const [nome, f] of Object.entries(arquivos)) {
    const css = f.slice(f.indexOf('<style>'));
    assert.doesNotMatch(css, /[\w)]\s+svg\s*[{,]/, `${nome}: seletor svg escopado não alcança o Icone`);
  }
});

test('DESIGN.md documenta lucide-svelte, traço 1.75 e os estados', () => {
  const i = design.indexOf('## 4. Ícones');
  const sec = design.slice(i, design.indexOf('\n## ', i + 5));
  assert.doesNotMatch(sec, /Lucide React/);
  assert.match(sec, /lucide-svelte/);
  assert.match(sec, /1\.75/);
  for (const ic of ['CircleCheck', 'LoaderCircle', 'CircleAlert', 'Clock', 'CirclePause', 'CircleSlash', 'Trash2']) {
    assert.match(sec, new RegExp(ic), `${ic} fora do DESIGN`);
  }
});

// F-VI-04: o mesmo ícone identifica a mesma configuração em todo lugar.
const atividade = ler('src', 'components', 'AtividadeItem.svelte');
const reprocessar = ler('src', 'components', 'ReprocessarItem.svelte');
const segmentado = ler('src', 'components', 'Segmentado.svelte');

function importa(f, nome, arquivo) {
  assert.match(f, new RegExp(`import ${nome} from 'lucide-svelte/icons/${arquivo}'`), `${nome} sem import`);
}

test('rótulos do painel com o ícone da configuração em 14px', () => {
  const s = arquivos.Settings;
  const rotulos = [
    ['watchdog-toggle', 'FolderSearch', 'folder-search'],
    ['autostart-toggle', 'Power', 'power'],
    ['omitir-tipos', 'EyeOff', 'eye-off']
  ];
  for (const [id, ic, arq] of rotulos) {
    importa(s, ic, arq);
    const i = s.indexOf(`<label for="${id}"`);
    assert.ok(i >= 0, id);
    const label = s.slice(i, s.indexOf('</label>', i));
    assert.match(label, new RegExp(`<Icone icone=\\{${ic}\\} tamanho=\\{14\\}`), `${ic} fora do rótulo ${id}`);
  }
  // A pasta monitorada não tem rótulo visível (ajuste do dono, 2026-11-24).
  assert.doesNotMatch(s, /id="rotulo-pasta"|import Folder from/);
  importa(s, 'ScanText', 'scan-text');
  importa(s, 'Sparkles', 'sparkles');
  assert.match(s.match(/<Segmentado\s+id="ocr-mode"[^>]*>/)[0], /icone=\{ScanText\}/);
  assert.match(s.match(/<Segmentado\s+id="cleaning-level"[^>]*>/)[0], /icone=\{Sparkles\}/);
  assert.match(estiloDe(s), /\.icone-rotulo\s*\{[^}]*color:\s*var\(--text-secondary\)/);
});

test('Segmentado mostra o ícone opcional na legenda', () => {
  assert.match(segmentado, /export let icone = null/);
  const legenda = segmentado.slice(segmentado.indexOf('<legend'), segmentado.indexOf('</legend>'));
  assert.match(legenda, /\{#if icone\}[\s\S]*<Icone \{icone\}/);
});

test('carregamento do painel usa LoaderCircle, não spinner de borda', () => {
  const s = arquivos.Settings;
  importa(s, 'LoaderCircle', 'loader-circle');
  const i = s.indexOf('class="loading-state"');
  assert.match(s.slice(i, s.indexOf('</div>', i)), /class="spinner"[^>]*aria-hidden="true"[^>]*>\s*<Icone icone=\{LoaderCircle\}/);
  assert.doesNotMatch(estiloDe(s), /border-top-color/);
});

test('filtro da fila com Search 12px e espaço à esquerda', () => {
  const l = arquivos.ProcessList;
  importa(l, 'Search', 'search');
  const i = l.indexOf('class="search-input"');
  const antes = l.slice(l.lastIndexOf('<span class="busca"', i), i);
  assert.match(antes, /<Icone icone=\{Search\} tamanho=\{12\}/);
  assert.match(estiloDe(l), /\.search-input\s*\{[^}]*padding:\s*4px 8px 4px 24px/);
});

test('botões de texto da linha levam ícone', () => {
  importa(reprocessar, 'RotateCcw', 'rotate-ccw');
  assert.match(reprocessar, /<button type="submit"[^>]*>\s*<Icone icone=\{RotateCcw\} tamanho=\{12\}/);
  assert.match(reprocessar, /legenda="OCR"[^>]*icone=\{ScanText\}/);
  assert.match(reprocessar, /legenda="Higienização"[^>]*icone=\{Sparkles\}/);
  importa(atividade, 'Eye', 'eye');
  importa(atividade, 'EyeOff', 'eye-off');
  importa(atividade, 'RotateCcw', 'rotate-ccw');
  const alt = atividade.slice(atividade.indexOf('class="alternar"'), atividade.indexOf('</button>', atividade.indexOf('class="alternar"')));
  assert.match(alt, /<Icone icone=\{mostrarConcluidos \? EyeOff : Eye\} tamanho=\{12\}/);
  assert.match(atividade, /class="btn-ocr"[^>]*>\s*<Icone icone=\{RotateCcw\} tamanho=\{12\}/);
  assert.doesNotMatch(atividade, /OCR ·/);
  const ocr = atividade.slice(atividade.indexOf('class="doc-ocr'), atividade.indexOf('pág.', atividade.indexOf('class="doc-ocr')));
  assert.match(ocr, /<Icone icone=\{ScanText\} tamanho=\{12\}/);
  assert.match(ocr, /class="sr-only">OCR/);
});

test('ícones dos cabeçalhos de seção do App em 14px', () => {
  const m = marcacao(arquivos.App);
  for (const [classe, titulo] of [['painel-titulo', 'Configurações']]) {
    const i = m.indexOf(`class="${classe}"`);
    assert.ok(i >= 0, `${classe} ausente`);
    const bloco = m.slice(i, m.indexOf('</h2>', i));
    assert.ok(bloco.includes(`<h2>${titulo}`), `${titulo} fora de ${classe}`);
    assert.match(bloco, /<Icone icone=\{\w+\} tamanho=\{14\} \/>/, `ícone de ${titulo} fora de 14px`);
  }
  const i = design.indexOf('## 4. Ícones');
  const sec = design.slice(i, design.indexOf('\n## ', i + 5));
  assert.match(sec, /`14px`[^\n]*cabeçalho de seção/);
});

test('DESIGN.md registra o mapa de configurações', () => {
  const i = design.indexOf('## 4. Ícones');
  const sec = design.slice(i, design.indexOf('\n## ', i + 5));
  for (const ic of ['FolderSearch', 'ScanText', 'Sparkles', 'EyeOff', 'Power', 'Search', 'RotateCcw', 'Eye`']) {
    assert.ok(sec.includes(ic), `${ic} fora do DESIGN §4`);
  }
});

function estiloDe(f) {
  return f.slice(f.indexOf('<style>'));
}
