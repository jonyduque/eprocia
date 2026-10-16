// SessionStart: one line with Go version, Wails CLI compatibility, running
// eprocia.exe and benchmark corpus status.
import { execFileSync } from 'node:child_process';
import { executar } from './lib.mjs';

// x/tools v0.30.0 (shipped with Wails CLI v2.12.0) breaks with Go 1.27.
export function xtoolsCompativel(versao) {
  const m = /^v0\.(\d+)\./.exec(versao ?? '');
  if (!m) return null;
  return Number(m[1]) >= 31;
}

export function montarLinha({ goVersao, wails, xtools, pid, benchDir }) {
  const partes = [`Go ${goVersao ?? '?'}`];
  const compat = xtoolsCompativel(xtools);
  if (compat === null && wails) partes.push(`Wails CLI em ${wails} (versão do x/tools não lida)`);
  else if (compat === null) partes.push('Wails CLI: não encontrado');
  else if (compat) partes.push(`Wails CLI ok (x/tools ${xtools})`);
  else partes.push(`Wails CLI incompatível com Go 1.27 (x/tools ${xtools}): use GOTOOLCHAIN=go1.25.5 wails build ou rode scripts/build/recompilar-wails.ps1`);
  partes.push(pid ? `eprocia.exe rodando (PID ${pid})` : 'eprocia.exe parado');
  partes.push(benchDir ? `EPROCIA_BENCH_DIR=${benchDir}` : 'EPROCIA_BENCH_DIR não definido (bench real será pulado)');
  return `[eprocIA] ${partes.join(' · ')}`;
}

// A cold `go version -m` on the 36 MB wails.exe can take over 1.5 s at session
// start, so each call gets 4 s (the hook timeout in settings.json is 20 s).
function sh(cmd, args) {
  try {
    return execFileSync(cmd, args, { encoding: 'utf8', timeout: 4000, stdio: ['ignore', 'pipe', 'ignore'] });
  } catch {
    return '';
  }
}

function coletar() {
  const goVersao = /go(\d+\.\d+(\.\d+)?)/.exec(sh('go', ['env', 'GOVERSION']))?.[1];
  const onde = sh(process.platform === 'win32' ? 'where' : 'which', ['wails']).split(/\r?\n/)[0].trim();
  const xtools = onde ? /golang\.org\/x\/tools\s+(v\S+)/.exec(sh('go', ['version', '-m', onde]))?.[1] : undefined;
  const lista = process.platform === 'win32' ? sh('tasklist', ['/FI', 'IMAGENAME eq eprocia.exe', '/FO', 'CSV', '/NH']) : '';
  const pid = /"eprocia\.exe","(\d+)"/i.exec(lista)?.[1];
  return { goVersao, wails: onde || undefined, xtools, pid, benchDir: process.env.EPROCIA_BENCH_DIR };
}

export function avaliar(_evento, ctx = {}) {
  return { codigo: 0, contexto: montarLinha(ctx.info ?? coletar()) };
}

await executar(import.meta.url, 'SessionStart', avaliar);
