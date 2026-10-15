// Status line: branch plus the progress of the review in docs/revisao/estado.json.
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const RAIZ = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

export function lerEstado(arquivo = path.join(RAIZ, 'docs', 'revisao', 'estado.json')) {
  try {
    const e = JSON.parse(readFileSync(arquivo, 'utf8'));
    return e && typeof e === 'object' ? e : null;
  } catch {
    return null;
  }
}

export function formatar({ branch, estado }) {
  const base = branch || '(sem branch)';
  if (!estado) return base;
  const cont = { feito: 0, bloqueado: 0, pendente: 0, pulado: 0 };
  for (const s of Object.values(estado.ids ?? {})) {
    if (s in cont) cont[s]++;
  }
  const partes = [base, `revisão: ${estado.fase ?? '?'}`];
  const total = Object.values(cont).reduce((a, b) => a + b, 0);
  if (total) partes.push(`✓${cont.feito} ✗${cont.bloqueado} …${cont.pendente}${cont.pulado ? ` ↷${cont.pulado}` : ''}`);
  return partes.join(' · ');
}

function branchAtual() {
  try {
    return execFileSync('git', ['branch', '--show-current'], { cwd: RAIZ, encoding: 'utf8', timeout: 500 }).trim();
  } catch {
    return '';
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.stdout.write(formatar({ branch: branchAtual(), estado: lerEstado() }));
}
