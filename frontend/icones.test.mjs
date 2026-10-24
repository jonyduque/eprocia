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
      const m = u.match(/icone=\{(\w+)\}/);
      assert.ok(m, u);
      assert.match(f, new RegExp(`import ${m[1]} from 'lucide-svelte/icons/[a-z0-9-]+'`), `${m[1]} sem import`);
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

test('lixeira é Trash2 em todo lugar', () => {
  for (const [nome, f] of Object.entries(arquivos)) {
    assert.doesNotMatch(f, /icons\/trash'/, `${nome} usa trash`);
  }
  assert.match(arquivos.ProcessList, /import Trash2 from 'lucide-svelte\/icons\/trash-2'/);
  assert.match(arquivos.Settings, /import Trash2 from 'lucide-svelte\/icons\/trash-2'/);
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
