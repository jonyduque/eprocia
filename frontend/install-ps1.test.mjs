// install.ps1: instalar por cima apaga as sobras da atualizacao pelo app, o arquivo
// continua compativel com o PowerShell 5.1 (sem BOM, sem bytes 0x91-0x94) e, desde a
// decisao d6, atualiza a versao instalada no lugar (por usuario ou para todos), pergunta
// antes de encerrar o app aberto e eleva so a acao que precisa.
// Rodar com: node --test frontend/install-ps1.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');
const bytes = readFileSync(join(raiz, 'install.ps1'));
const texto = bytes.toString('utf8').replace(/\r\n/g, '\n');

function corpoDe(nome) {
  let ini = texto.indexOf(`function ${nome}(`);
  if (ini < 0) ini = texto.indexOf(`function ${nome} {`);
  assert.ok(ini >= 0, `function ${nome} nao encontrada`);
  const fim = texto.indexOf('\nfunction ', ini + 1);
  return texto.slice(ini, fim < 0 ? undefined : fim);
}

// Indices em ordem crescente (cada trecho existe e vem depois do anterior).
function emOrdem(corpo, trechos, rotulo) {
  let ultimo = -1;
  for (const t of trechos) {
    const i = typeof t === 'string' ? corpo.indexOf(t, ultimo + 1) : (() => {
      const re = new RegExp(t.source, t.flags.includes('g') ? t.flags : t.flags + 'g');
      re.lastIndex = ultimo + 1;
      const m = re.exec(corpo);
      return m ? m.index : -1;
    })();
    assert.ok(i >= 0, `${rotulo}: ${t} ausente (ou fora de ordem)`);
    ultimo = i;
  }
}

// Bloco { ... } que comeca na primeira chave depois de `ini`.
function blocoDe(corpo, ini) {
  const abre = corpo.indexOf('{', ini);
  let n = 0;
  for (let i = abre; i < corpo.length; i++) {
    if (corpo[i] === '{') n++;
    if (corpo[i] === '}' && --n === 0) return corpo.slice(abre, i + 1);
  }
  return corpo.slice(abre);
}

// Tira comentarios, here-strings e strings, deixando so o codigo (o de dentro de $( )
// numa string de aspas duplas continua, porque roda).
function soCodigo(src) {
  let out = '';
  let i = 0;
  function codigo(ateParenteses) {
    let prof = 0;
    while (i < src.length) {
      const c = src[i];
      const n = src[i + 1];
      if (c === '<' && n === '#') { const j = src.indexOf('#>', i + 2); i = j < 0 ? src.length : j + 2; out += ' '; continue; }
      if (c === '#') { while (i < src.length && src[i] !== '\n') i++; continue; }
      if (c === '@' && (n === "'" || n === '"') && /^[ \t]*\n/.test(src.slice(i + 2, i + 40))) {
        const j = src.indexOf(`\n${n}@`, i + 2);
        i = j < 0 ? src.length : j + 3;
        out += ' ';
        continue;
      }
      if (c === "'") {
        i++;
        while (i < src.length) {
          if (src[i] === "'") { if (src[i + 1] === "'") { i += 2; continue; } i++; break; }
          i++;
        }
        out += ' ';
        continue;
      }
      if (c === '"') { i++; aspasDuplas(); out += ' '; continue; }
      if (c === '`') { out += src.slice(i, i + 2); i += 2; continue; }
      if (ateParenteses) {
        if (c === '(') prof++;
        if (c === ')') { if (prof === 0) { i++; return; } prof--; }
      }
      out += c;
      i++;
    }
  }
  function aspasDuplas() {
    while (i < src.length) {
      const c = src[i];
      if (c === '`') { i += 2; continue; }
      if (c === '"') { if (src[i + 1] === '"') { i += 2; continue; } i++; return; }
      if (c === '$' && src[i + 1] === '(') { i += 2; out += ' '; codigo(true); out += ' '; continue; }
      i++;
    }
  }
  codigo(false);
  return out;
}
const contaExit = (src) => (soCodigo(src).match(/\bexit\b/g) || []).length;

// Modelo do bootstrap: a here-string de aspas simples de Get-ElevationBootstrap.
function modeloBootstrap() {
  const corpo = corpoDe('Get-ElevationBootstrap');
  const m = /@'\n([\s\S]*?)\n'@/.exec(corpo);
  assert.ok(m, 'here-string @\'...\'@ ausente em Get-ElevationBootstrap');
  return m[1];
}
const aspas = (s) => `'${s.replace(/'/g, "''")}'`;
// Get-ElevationBootstrap passa os hashes (hex) em Base64.
const b64 = (hex) => (hex ? Buffer.from(hex, 'hex').toString('base64') : '');
// Mesma montagem de Get-ElevationBootstrap: marcadores trocados por literais e as linhas
// juntadas com ';'.
function montarBootstrap(v) {
  return modeloBootstrap()
    .split('\n').map((l) => l.trim()).filter((l) => l)
    .join(';')
    .replaceAll('_PAI_', aspas(v.pai))
    .replaceAll('_GUID_', aspas(v.guid))
    .replaceAll('_HS_', aspas(b64(v.hs)))
    .replaceAll('_NOME_', aspas(v.nome))
    .replaceAll('_HA_', aspas(b64(v.ha)))
    .replaceAll('_SID_', aspas(v.sid))
    .replaceAll('_ARGS_', v.args.map(aspas).join(','));
}

test('Install-App apaga eprocia.anterior.exe e eprocia.falhou.exe depois da troca do exe novo', () => {
  const corpo = corpoDe('Install-App');
  const mover = corpo.indexOf('Move-ExeWithRetry $tmp ');
  assert.ok(mover >= 0, 'Move-ExeWithRetry $tmp ausente');
  for (const nome of ['eprocia.anterior.exe', 'eprocia.falhou.exe']) {
    const re = new RegExp(`Remove-Item[^\\n]*Join-Path \\$installDir '${nome.replace(/\./g, '\\.')}'[^\\n]*-Force[^\\n]*-ErrorAction SilentlyContinue`);
    const m = re.exec(corpo);
    assert.ok(m, `Remove-Item de ${nome} ausente`);
    assert.ok(m.index > mover, `${nome} deve ser apagado depois da troca`);
  }
});

test('Move-ExeWithRetry repete o Move-Item ate 10 vezes, 200 ms, so em erro de arquivo', () => {
  const corpo = corpoDe('Move-ExeWithRetry');
  assert.match(corpo, /Move-Item -LiteralPath \$Source -Destination \$Destination -Force/);
  assert.match(corpo, /\b10\b/);
  assert.match(corpo, /-Milliseconds 200/);
  assert.match(corpo, /catch \[System\.IO\.IOException\], \[System\.UnauthorizedAccessException\]/);
});

test('install.ps1 nao tem BOM', () => {
  assert.notDeepEqual([...bytes.subarray(0, 3)], [0xef, 0xbb, 0xbf]);
});

test('nenhuma sequencia multibyte UTF-8 contem os bytes 0x91-0x94', () => {
  for (let i = 0; i < bytes.length; i++) {
    if (bytes[i] >= 0x80) {
      assert.ok(!(bytes[i] >= 0x91 && bytes[i] <= 0x94), `byte 0x${bytes[i].toString(16)} no offset ${i}`);
    }
  }
});

test('Assert-Closed nao existe mais', () => {
  assert.ok(!texto.includes('function Assert-Closed'));
});

test('ordem: acao, app aberto, download, encerramento, troca', () => {
  emOrdem(corpoDe('Install-App'), ['Confirm-Install', 'Confirm-Close', 'Save-ReleaseExe', 'Stop-App', 'Move-ExeWithRetry $tmp'], 'Install-App');
  emOrdem(corpoDe('Uninstall-App'), ['Confirm-Uninstall', 'Confirm-Close', 'Stop-App', 'Set-Location'], 'Uninstall-App');
  const asUser = corpoDe('Invoke-AsUser');
  emOrdem(asUser, ['janela oculta', 'Confirm-Install', 'Confirm-Close', 'New-Item -ItemType Directory', 'Register-ScheduledTask'], 'Invoke-AsUser');
  for (const nome of ['Install-App', 'Uninstall-App', 'Invoke-AsUser']) {
    assert.match(corpoDe(nome), /if \(-not \(Confirm-Close [^\n]*\)\) \{ Write-Cancelled; return \}/, nome);
  }
});

test('Invoke-AsUser passa sempre -ClosePids e -CloseAny so com o -Yes do usuario', () => {
  const corpo = corpoDe('Invoke-AsUser');
  // Forma -ClosePids:<valor>: com -File, o powershell.exe 5.1 recusa um '-' solto (sai com 5).
  assert.match(corpo, /\$argv \+= "-ClosePids:\$\(Get-ClosePidText\)"/);
  assert.ok(corpoDe('Invoke-Elevated').includes('"-ClosePids:$(Get-ClosePidText)"'));
  assert.ok(!/'-ClosePids'/.test(texto), "-ClosePids solto, sem ':'");
  assert.match(corpo, /if \(\$Yes -and -not \$Log\) \{ \$argv \+= '-CloseAny' \}/);
});

test('Confirm-Close so pergunta; Get-AppProcesses e Get-OcrChildren leem o Win32_Process', () => {
  const corpo = corpoDe('Confirm-Close');
  for (const s of ['Confirm-Plan', 'Get-OcrChildren', 'closePids', "'Encerrar o eprocIA aberto?'", "'Encerrar'"]) assert.ok(corpo.includes(s), s);
  assert.ok(!corpo.includes('Stop-Process'));
  assert.ok(!corpo.includes("Fail 'O eprocIA est"));
  const proc = corpoDe('Get-AppProcesses');
  for (const s of ['Win32_Process', 'ExecutablePath', 'SessionId', 'ProcessId']) assert.ok(proc.includes(s), s);
  const ocr = corpoDe('Get-OcrChildren');
  for (const s of ['ParentProcessId', 'tesseract.exe', 'pdftoppm.exe', 'pdfinfo.exe', 'pdftotext.exe']) assert.ok(ocr.includes(s), s);
});

test('Stop-App confere sessao e lista antes de encerrar e espera sair', () => {
  const corpo = corpoDe('Stop-App');
  const parar = corpo.indexOf('Stop-Process -Id');
  assert.ok(parar >= 0);
  assert.ok(corpo.indexOf('SessionId') >= 0 && corpo.indexOf('SessionId') < parar);
  assert.ok(corpo.indexOf('closePids') >= 0 && corpo.indexOf('closePids') < parar);
  assert.ok(corpo.indexOf('aberto depois da pergunta') < parar);
  assert.ok(corpo.indexOf('outra sessao') < parar);
  assert.ok(corpo.includes('Wait-Process'));
  assert.ok(corpo.includes('$CloseAny -or ($Yes -and -not $Log)'));
  assert.ok(corpo.includes('nao fechou em 15 s'));
});

test('New-ProtectedDir cria a pasta ja com a ACL e confere vazia e dono', () => {
  const corpo = corpoDe('New-ProtectedDir');
  emOrdem(corpo, ['Test-Path', 'CreateDirectory', /Get-ChildItem[^\n]*-Force/, 'Owner'], 'New-ProtectedDir');
  for (const s of ['SetAccessRuleProtection', 'S-1-5-32-544', 'S-1-5-18', 'ContainerInherit,ObjectInherit']) assert.ok(corpo.includes(s), s);
  assert.ok(!/icacls/i.test(corpo));
  assert.ok(modeloBootstrap().includes('ContainerInherit,ObjectInherit'));
});

test('Remove-StaleElevationDirs pula links, exige dono admin e mais de um dia', () => {
  const corpo = corpoDe('Remove-StaleElevationDirs');
  assert.ok(corpo.includes('-LiteralPath'));
  const rp = corpo.indexOf('ReparsePoint');
  assert.ok(rp >= 0 && rp < corpo.indexOf('Owner') && rp < corpo.indexOf('S-1-5-32-544'));
  for (const s of ['S-1-5-32-544', 'S-1-5-18', 'AddDays(-1)', '[IO.Directory]::Delete(']) assert.ok(corpo.includes(s), s);
  assert.ok(!corpo.includes('Remove-Item'));
  for (const nome of ['Install-Machine', 'Uninstall-Machine']) {
    const c = corpoDe(nome);
    const r = c.indexOf('Remove-StaleElevationDirs');
    assert.ok(r >= 0 && r < c.indexOf('Stop-App'), nome);
  }
});

const piorCaso = {
  pai: 'C:\\' + 'P'.repeat(257),
  guid: 'f'.repeat(32),
  hs: 'AB'.repeat(32),
  nome: 'eprocia-instalador.exe',
  ha: 'CD'.repeat(32),
  sid: 'S-1-5-21-' + '1'.repeat(51),
  args: ['-Uninstall', '-ClosePids:' + Array(8).fill('999999').join(','), '-CloseAny'],
};

test('bootstrap: linha inteira abaixo de 2.000 caracteres no pior caso', () => {
  assert.equal(piorCaso.pai.length, 260);
  assert.equal(piorCaso.sid.length, 60);
  const bootstrap = montarBootstrap(piorCaso);
  const linha = `-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -Command "${bootstrap}"`;
  console.log(`bootstrap pior caso: ${linha.length} caracteres (modelo ${modeloBootstrap().length})`);
  assert.ok(linha.length < 2000, `linha com ${linha.length} caracteres`);
});

test('bootstrap: modelo ASCII, sem aspa dupla, termina em exit $c, sem curinga de limpeza', () => {
  const modelo = modeloBootstrap();
  assert.match(modelo, /^[\x00-\x7F]*$/);
  assert.ok(!modelo.includes('"'));
  assert.match(modelo.trimEnd(), /exit \$c$/);
  assert.ok(!modelo.includes('eprocia-elevar-*'));
  assert.ok(!/\\$/.test(montarBootstrap(piorCaso)));
});

test('bootstrap: cria a pasta protegida, confere, copia, confere o hash, roda, sinaliza e apaga', () => {
  const modelo = modeloBootstrap();
  for (const s of ['ProgramData', 'DirectorySecurity', 'CreateDirectory', 'SetAccessRuleProtection', 'S-1-5-32-544', 'S-1-5-18', '_SID_']) assert.ok(modelo.includes(s), s);
  emOrdem(modelo, ['Test-Path', 'CreateDirectory', /Get-ChildItem[^\n]*-Force/, 'Owner', 'Copy-Item', 'SHA256', '-File', "\\fim'", "\\lido'", 'Remove-Item'], 'bootstrap');
  // $d e a pasta protegida; $p, a do pai: so o 'lido' e as origens das copias ficam no pai.
  assert.match(modelo, /^\$d=\$env:ProgramData\+'\\eprocia-elevar-'\+_GUID_$/m);
  assert.match(modelo, /^\$p=_PAI_$/m);
  assert.match(modelo, /-File \(\$d\+'\\install\.ps1'\)/);
  assert.match(modelo, /-Log \(\$d\+'\\install\.log'\)/);
  assert.match(modelo, /WriteAllText\(\$d\+'\\fim'/);
  assert.match(modelo, /\[IO\.File\]::Exists\(\$p\+'\\lido'\)/);
  assert.match(modelo, /ToBase64String\(\[Security\.Cryptography\.SHA256\]::Create\(\)\.ComputeHash\(\[IO\.File\]::ReadAllBytes\(\$t\)\)\) -cne \$x\[1\]/);
  assert.match(modelo, /& \(\$PSHOME\+'\\powershell\.exe'\)/);
  assert.match(montarBootstrap(piorCaso), /'q6urq6urq6urq6urq6urq6urq6urq6urq6urq6urq6s='/);
  assert.ok(!/-(File|Log) \(\$p/.test(modelo), '-File/-Log na pasta do pai');
  assert.match(modelo, /SetAccessRuleProtection\(1,0\)/);
});

test('Invoke-Elevated: UAC com uma string so, espera fim, le o log da pasta protegida', () => {
  const corpo = corpoDe('Invoke-Elevated');
  for (const s of ['-Verb RunAs', '-PassThru', '-Command', '.Handle', 'WaitForExit', 'ExitCode', "'fim'", "'lido'", '1223',
    'Nada foi alterado', 'Show-Log', '-ClosePids', 'CloseAny', 'finally', 'Get-ElevationBootstrap']) {
    assert.ok(corpo.includes(s), s);
  }
  for (const s of ['-EncodedCommand', '-Wait ', "'-Purge'", "'-DryRun'", "'-NoStart'"]) assert.ok(!corpo.includes(s), s);
  assert.ok(!/-File\b/.test(corpo), '-File em Invoke-Elevated');
  const al = /Start-Process[^\n]*-ArgumentList ([^\n]*)/.exec(corpo);
  assert.ok(al, 'Start-Process -ArgumentList ausente');
  assert.ok(!al[1].startsWith('@('), '-ArgumentList em array');
  assert.ok(al[1].startsWith('"-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -Command `"$bootstrap`""'));
  for (const m of corpo.matchAll(/install\.log/g)) {
    assert.match(corpo.slice(m.index - 30, m.index), /Join-Path \$dst '$/, 'install.log fora da pasta protegida');
  }
});

test('Get-ScriptText usa o texto em execucao com o marcador, guardado no nivel do script', () => {
  assert.ok(corpoDe('Get-ScriptText').includes('eprocia-install: elevacao-v1'));
  assert.match(texto, /^# eprocia-install: elevacao-v1$/m);
  const primeiraFuncao = texto.indexOf('\nfunction ');
  const sb = texto.indexOf('$MyInvocation.MyCommand.ScriptBlock');
  assert.ok(sb >= 0 && sb < primeiraFuncao);
  assert.ok(corpoDe('Get-ScriptText').includes("nao suporta a acao como administrador"));
});

test('Invoke-Main: -Machine vai direto ao filho elevado; machine sem elevacao chama Invoke-Elevated', () => {
  const corpo = corpoDe('Invoke-Main');
  const ini = corpo.indexOf('if ($Machine)');
  assert.ok(ini >= 0);
  assert.ok(ini < corpo.lastIndexOf('Get-Target') && ini < corpo.indexOf('Invoke-AsUser'));
  const bloco = blocoDe(corpo, ini);
  assert.ok(!bloco.includes('Invoke-AsUser'));
  assert.ok(bloco.includes('Install-Machine') && bloco.includes('Uninstall-Machine'));
  for (const s of ['Invoke-Elevated', 'explorer.exe', '-NoDesktop nao vale', 'Confirm-Close']) assert.ok(corpo.includes(s), s);
  const alvo = corpoDe('Get-Target');
  assert.match(alvo, /if \(\$Machine\) \{[^}]*Fail 'Nao ha instalacao para todos os usuarios'/);
});

test('Install-Machine roda o instalador em silencio com /D= por ultimo e nunca abre o app', () => {
  const corpo = corpoDe('Install-Machine');
  assert.match(corpo, /-ArgumentList @\('\/S', "\/D=\$\(\$Target\.Dir\)"\)/);
  const parar = corpo.indexOf('Stop-App');
  assert.ok(parar >= 0 && parar < corpo.indexOf('Start-Process'));
  assert.ok(!corpo.includes('explorer.exe'));
  assert.ok(!corpo.includes('Start-Process -FilePath $Target.Exe'));
  assert.ok(corpo.includes('New-ProtectedDir'));
});

test('Uninstall-Machine roda o uninstall.exe da pasta com /S e _?=', () => {
  const corpo = corpoDe('Uninstall-Machine');
  for (const s of ["Join-Path $Target.Dir 'uninstall.exe'", "'/S'", '_?=']) assert.ok(corpo.includes(s), s);
  assert.ok(!corpo.includes('QuietUninstallString'));
});

test('Get-Release acha o instalador; Save-ReleaseInstaller existe', () => {
  assert.ok(corpoDe('Get-Release').includes('-instalador\\.exe$'));
  assert.ok(corpoDe('Get-Release').includes('Installer'));
  assert.ok(corpoDe('Save-ReleaseInstaller').includes('Installer'));
});

test('Invoke-DryRun mostra UAC e o app aberto sem agir', () => {
  // O fim comum da simulacao (perguntas e "fazer de verdade?") fica em Complete-DryRun, que
  // as duas saidas de Invoke-DryRun chamam.
  const corpo = corpoDe('Invoke-DryRun');
  const fim = corpoDe('Complete-DryRun');
  assert.ok(corpo.includes('UAC'));
  assert.ok(corpo.includes('Terminal elevado: faria aqui mesmo'));
  assert.ok(corpo.includes('Rodaria o instalador em silencio sobre') && corpo.includes('Rodaria o desinstalador de'));
  assert.ok(corpo.includes('-NoDesktop nao vale'));
  assert.equal((corpo.match(/Complete-DryRun /g) || []).length, 2);
  assert.ok(fim.includes('Confirm-Close'));
  assert.ok(corpo.includes('janela oculta'));
  for (const c of [corpo, fim]) {
    for (const s of ['Invoke-Elevated', 'Stop-App', 'Stop-Process', 'Register-ScheduledTask']) assert.ok(!c.includes(s), s);
  }
});

test('param e ajuda: -Machine, -ClosePids, -CloseAny internos; sem "Nunca pede administrador"', () => {
  const param = /\nparam\(([\s\S]*?)\n\)/.exec(texto)[1];
  for (const s of ['[switch]$Machine', '[string]$ClosePids', '[switch]$CloseAny']) assert.ok(param.includes(s), s);
  assert.ok(!param.includes('$Sha256'));
  const ajuda = texto.slice(0, texto.indexOf('#>'));
  for (const p of ['Machine', 'ClosePids', 'CloseAny']) {
    assert.match(ajuda, new RegExp(`\\.PARAMETER ${p}\\n\\s+Interno`), p);
  }
  assert.ok(!ajuda.includes('Nunca pede administrador'));
  assert.match(ajuda, /\.PARAMETER Yes[\s\S]*encerra o eprocIA aberto/);
});

test('soCodigo conta exit so no codigo', () => {
  assert.equal(contaExit("if ($x) { 'a'; exit 1 }"), 1);
  assert.equal(contaExit("'exit'"), 0);
  assert.equal(contaExit('"a $(exit) b"'), 1);
  assert.equal(contaExit("# exit\n<# exit #>\n@'\nexit\n'@\n"), 0);
});

test('install.ps1 tem exatamente 2 exit no codigo', () => {
  assert.equal(contaExit(texto), 2);
});

test('funcoes novas sao so ASCII', () => {
  for (const nome of ['Get-Target', 'Get-AppProcesses', 'Get-OcrChildren', 'Confirm-Close', 'Stop-App', 'New-ProtectedDir',
    'Remove-StaleElevationDirs', 'Get-ElevationBootstrap', 'Invoke-Elevated', 'Install-Machine', 'Uninstall-Machine',
    'Save-ReleaseInstaller', 'Move-ExeWithRetry']) {
    assert.match(corpoDe(nome), /^[\x00-\x7F]*$/, nome);
  }
});
