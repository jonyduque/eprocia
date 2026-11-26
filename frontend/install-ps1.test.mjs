// install.ps1: instalar por cima apaga as sobras da atualizacao pelo app e o
// arquivo continua compativel com o PowerShell 5.1 (sem BOM, sem bytes 0x91-0x94).
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
  const ini = texto.indexOf(`function ${nome} {`);
  assert.ok(ini >= 0, `function ${nome} nao encontrada`);
  const fim = texto.indexOf('\nfunction ', ini + 1);
  return texto.slice(ini, fim < 0 ? undefined : fim);
}

test('Install-App apaga eprocia.anterior.exe e eprocia.falhou.exe depois do Move-Item do exe novo', () => {
  const corpo = corpoDe('Install-App');
  const mover = corpo.indexOf('Move-Item -LiteralPath $tmp ');
  assert.ok(mover >= 0, 'Move-Item -LiteralPath $tmp ausente');
  for (const nome of ['eprocia.anterior.exe', 'eprocia.falhou.exe']) {
    const re = new RegExp(`Remove-Item[^\\n]*Join-Path \\$installDir '${nome.replace(/\./g, '\\.')}'[^\\n]*-Force[^\\n]*-ErrorAction SilentlyContinue`);
    const m = re.exec(corpo);
    assert.ok(m, `Remove-Item de ${nome} ausente`);
    assert.ok(m.index > mover, `${nome} deve ser apagado depois do Move-Item`);
  }
});

test('Assert-Closed $exe continua antes do download', () => {
  const corpo = corpoDe('Install-App');
  const fechado = corpo.indexOf('Assert-Closed $exe');
  const download = corpo.indexOf('Save-ReleaseExe');
  assert.ok(fechado >= 0 && download > fechado);
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
