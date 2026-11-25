// Instalador NSIS: inteiro em português, atalho da área de trabalho opcional e
// "Abrir o eprocIA" ao terminar, pelo explorer.exe (decisões d2, d3 e e7).
// Rodar com: node --test frontend/instalador-nsis.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');
const ler = (...p) => readFileSync(join(raiz, ...p), 'utf8').replace(/\r\n/g, '\n');
const nsi = ler('build', 'windows', 'installer', 'project.nsi');
const nsh = ler('build', 'windows', 'installer', 'wails_tools.nsh');
const registro = ler('internal', 'atualizacao', 'registro_windows.go');

const INGLES = /\b(Install|Installing|Installer|Uninstall|requires|required|supported|can't|only|and later|Welcome|Finish)\b/i;

// Linhas sem comentário (# ou ; fora de aspas) e sem espaços nas pontas.
function linhas(texto) {
  return texto.split('\n').map((l) => {
    let aspas = null;
    for (let i = 0; i < l.length; i++) {
      const c = l[i];
      if (aspas) {
        if (c === '$' && l[i + 1] === '\\') { i += 2; continue; }
        if (c === aspas) aspas = null;
      } else if (c === '"' || c === "'" || c === '`') {
        aspas = c;
      } else if (c === '#' || c === ';') {
        return l.slice(0, i).trim();
      }
    }
    return l.trim();
  });
}

// Argumentos de uma linha NSIS: strings entre aspas (sem as aspas) ou palavras.
function argumentos(linha) {
  const re = /"((?:\$\\"|[^"])*)"|'([^']*)'|`([^`]*)`|(\S+)/g;
  const out = [];
  let m;
  while ((m = re.exec(linha))) out.push(m[1] ?? m[2] ?? m[3] ?? m[4]);
  return out;
}

const L = linhas(nsi);
const idx = (re) => L.findIndex((l) => re.test(l));
const idxUltimo = (re) => L.findLastIndex((l) => re.test(l));

// Corpo (linhas) do bloco que começa na linha i até o primeiro fim.
function bloco(i, fim) {
  assert.ok(i >= 0, 'início do bloco ausente');
  const j = L.findIndex((l, k) => k > i && fim.test(l));
  assert.ok(j > i, 'fim do bloco ausente');
  return L.slice(i, j + 1);
}

test('idioma: só PortugueseBR, depois das macros de página', () => {
  const langs = L.filter((l) => /MUI_LANGUAGE\b/.test(l));
  assert.deepEqual(langs, ['!insertmacro MUI_LANGUAGE "PortugueseBR"']);
  assert.doesNotMatch(nsi.replace(/#.*$/gm, ''), /"English"/);
  const iLang = idx(/MUI_LANGUAGE\b/);
  const iPag = idxUltimo(/^!insertmacro\s+MUI_(UN)?PAGE_/);
  assert.ok(iPag >= 0 && iLang > iPag, 'MUI_LANGUAGE antes das páginas');
});

test('mensagens do wails_tools.nsh traduzidas antes do include', () => {
  const nomes = [...nsh.matchAll(/!ifndef\s+(WAILS_\w+)\s*\n\s*!define\s+\1\b/g)].map((m) => m[1]);
  assert.ok(nomes.includes('WAILS_WIN10_REQUIRED'), `lista lida: ${nomes}`);
  assert.ok(nomes.length >= 3, `lista lida: ${nomes}`);
  const iInc = idx(/^!include\s+"wails_tools\.nsh"/);
  assert.ok(iInc >= 0, '!include "wails_tools.nsh" ausente');
  for (const n of nomes) {
    const i = idx(new RegExp(`^!define\\s+${n}\\s+"`));
    assert.ok(i >= 0, `${n} não definido no project.nsi`);
    assert.ok(i < iInc, `${n} definido depois do include`);
  }
});

function textosVisiveis(ls) {
  const out = [];
  for (const l of ls) {
    const a = argumentos(l);
    if (!a.length) continue;
    const [cmd] = a;
    if (cmd === 'MessageBox') out.push(...a.slice(1).filter((x) => !/^(MB_|ID|\/SD)/.test(x)));
    else if (cmd === 'DetailPrint' || cmd === 'Caption' || cmd === 'BrandingText') out.push(...a.slice(1));
    else if (cmd === '!define' && /^(WAILS_|MUI_.*TEXT)/.test(a[1] ?? '')) out.push(...a.slice(2));
    else if (cmd === '!insertmacro' && a[1] === 'MUI_DESCRIPTION_TEXT') out.push(...a.slice(3));
    else if (cmd === 'VIAddVersionKey' && a[1] === 'FileDescription') out.push(...a.slice(2));
    else if (cmd === 'Section' || cmd === 'Section/o') {
      const nome = a.slice(1).find((x) => !x.startsWith('/'));
      if (nome !== undefined && nome.toLowerCase() !== 'uninstall') out.push(nome);
    }
  }
  return out;
}

test('nada em inglês no project.nsi', () => {
  const textos = textosVisiveis(L);
  assert.ok(textos.length >= 6, `poucos textos lidos: ${textos}`);
  for (const t of textos) assert.doesNotMatch(t, INGLES, `texto em inglês: ${t}`);
  const fd = L.find((l) => /^VIAddVersionKey\s+"FileDescription"/.test(l));
  assert.equal(fd, 'VIAddVersionKey "FileDescription" "Instalador do ${INFO_PRODUCTNAME}"');
});

test('nada literal em inglês nas mensagens do wails_tools.nsh', () => {
  const ls = linhas(nsh).filter((l) => /^(MessageBox|DetailPrint)\b/.test(l));
  assert.ok(ls.length >= 2, 'MessageBox/DetailPrint não encontrados');
  for (const t of textosVisiveis(ls)) assert.doesNotMatch(t, INGLES, `texto em inglês: ${t}`);
});

test('seções: programa sempre instalado, atalho da área de trabalho opcional', () => {
  const app = bloco(idx(/^Section\s+"\$\{INFO_PRODUCTNAME\} e atalho no menu Iniciar"\s+SecApp$/), /^SectionEnd$/);
  assert.ok(app.some((l) => /^SectionIn\s+RO\b/.test(l)), 'SecApp sem SectionIn RO');
  assert.ok(!app.some((l) => /^CreateShortCut\s+"\$DESKTOP/i.test(l)), 'SecApp cria o atalho da área de trabalho');
  assert.ok(app.some((l) => l === 'Delete "$DESKTOP\\${INFO_PRODUCTNAME}.lnk"'), 'SecApp sem o Delete do atalho');
  assert.ok(app.some((l) => /^CreateShortcut\s+"\$SMPROGRAMS\\\$\{INFO_PRODUCTNAME\}\.lnk"/i.test(l)), 'atalho do menu Iniciar ausente');
  assert.ok(app.includes('!insertmacro wails.writeUninstaller'), 'writeUninstaller ausente');

  const iDesk = idx(/^Section\b.*"Atalho na área de trabalho"\s+SecDesktop$/);
  assert.ok(iDesk >= 0, 'SecDesktop ausente');
  assert.equal(L[iDesk], 'Section "Atalho na área de trabalho" SecDesktop', 'SecDesktop deve vir marcada (sem /o)');
  const desk = bloco(iDesk, /^SectionEnd$/);
  assert.ok(!desk.some((l) => /^SectionIn\b/.test(l)), 'SecDesktop não pode ser obrigatória');
  assert.ok(desk.some((l) => /^CreateShortCut\s+"\$DESKTOP\\\$\{INFO_PRODUCTNAME\}\.lnk"/i.test(l)), 'SecDesktop sem o atalho');

  const w = idx(/^!insertmacro\s+MUI_PAGE_WELCOME$/);
  const c = idx(/^!insertmacro\s+MUI_PAGE_COMPONENTS$/);
  const d = idx(/^!insertmacro\s+MUI_PAGE_DIRECTORY$/);
  assert.ok(w >= 0 && c > w && d > c, 'MUI_PAGE_COMPONENTS fora de WELCOME..DIRECTORY');
  assert.ok(idx(/^!insertmacro\s+MUI_DESCRIPTION_TEXT\s+\$\{SecApp\}\s+"/) >= 0, 'descrição de SecApp ausente');
  assert.ok(idx(/^!insertmacro\s+MUI_DESCRIPTION_TEXT\s+\$\{SecDesktop\}\s+"/) >= 0, 'descrição de SecDesktop ausente');
});

test('página final: "Abrir o eprocIA" marcada, pelo explorer.exe', () => {
  const fin = idx(/^!insertmacro\s+MUI_PAGE_FINISH$/);
  assert.ok(fin >= 0, 'MUI_PAGE_FINISH ausente');
  const run = idx(/^!define\s+MUI_FINISHPAGE_RUN$/);
  const txt = idx(/^!define\s+MUI_FINISHPAGE_RUN_TEXT\s+"Abrir o eprocIA"$/);
  const fn = idx(/^!define\s+MUI_FINISHPAGE_RUN_FUNCTION\s+\S+$/);
  assert.ok(run >= 0 && run < fin, 'MUI_FINISHPAGE_RUN (sem valor) antes de MUI_PAGE_FINISH');
  assert.ok(txt >= 0 && txt < fin, 'MUI_FINISHPAGE_RUN_TEXT antes de MUI_PAGE_FINISH');
  assert.ok(fn >= 0 && fn < fin, 'MUI_FINISHPAGE_RUN_FUNCTION antes de MUI_PAGE_FINISH');
  assert.doesNotMatch(nsi, /MUI_FINISHPAGE_RUN_NOTCHECKED/);

  const nome = argumentos(L[fn])[2];
  const corpo = bloco(idx(new RegExp(`^Function\\s+${nome.replace(/[.$]/g, '\\$&')}$`)), /^FunctionEnd$/);
  const texto = corpo.join('\n');
  assert.match(texto, /explorer\.exe/);
  assert.ok(texto.includes('$INSTDIR\\${PRODUCT_EXECUTABLE}'), 'função sem o caminho do exe');
  for (const l of corpo) {
    const a = argumentos(l);
    if (!/^Exec(Wait|Shell|ShellWait)?$/.test(a[0] ?? '')) continue;
    const alvo = (a[0].startsWith('ExecShell') ? a[2] : a[1]) ?? '';
    assert.ok(!alvo.replace(/^["'`\s]+/, '').startsWith('$INSTDIR'), `exe aberto direto: ${l}`);
  }
});

test('desinstalador remove os dois atalhos', () => {
  const un = bloco(idx(/^Section\s+"uninstall"$/), /^SectionEnd$/);
  assert.ok(un.includes('Delete "$DESKTOP\\${INFO_PRODUCTNAME}.lnk"'));
  assert.ok(un.includes('Delete "$SMPROGRAMS\\${INFO_PRODUCTNAME}.lnk"'));
});

test('conferência cruzada: empresa + produto formam a chave de chaveNSIS', () => {
  const def = (n) => {
    const m = nsh.match(new RegExp(`!define ${n} "([^"]*)"`));
    assert.ok(m, `wails_tools.nsh sem ${n}`);
    return m[1];
  };
  const chave = registro.match(/chaveNSIS\s*=\s*`([^`]*)`/);
  assert.ok(chave, 'chaveNSIS ausente em registro_windows.go');
  assert.equal(chave[1], `Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\${def('INFO_COMPANYNAME')}${def('INFO_PRODUCTNAME')}`);
});
