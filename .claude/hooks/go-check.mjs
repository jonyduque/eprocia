// PostToolUse (Edit|Write|MultiEdit): gofmt -l on the edited Go file and go vet
// on its package; findings go back to Claude as context.
import { execFile } from 'node:child_process';
import path from 'node:path';
import { caminhoDoEvento, executar, normalizar, RAIZ } from './lib.mjs';

function rodarPadrao(cmd, args) {
  return new Promise((resolve) => {
    execFile(cmd, args, { cwd: RAIZ, timeout: 20000, encoding: 'utf8' }, (err, stdout, stderr) => {
      resolve({ ok: !err, saida: `${stdout ?? ''}${stderr ?? ''}`.trim() });
    });
  });
}

export async function avaliar(evento, ctx = {}) {
  const bruto = caminhoDoEvento(evento);
  const caminho = normalizar(bruto, ctx.raiz);
  if (!caminho.endsWith('.go') || caminho.startsWith('frontend/')) return { codigo: 0 };
  const rodar = ctx.rodar ?? rodarPadrao;

  const raiz = ctx.raiz ?? RAIZ;
  const absoluto = path.join(raiz, caminho);
  const pacote = './' + path.relative(raiz, path.dirname(absoluto)).replace(/\\/g, '/');
  const problemas = [];

  const fmt = await rodar('gofmt', ['-l', absoluto]);
  if (fmt.saida) problemas.push(`gofmt: arquivo fora do formato padrão (rode gofmt -w ${caminho}).`);

  const vet = await rodar('go', ['vet', pacote === './' ? '.' : pacote]);
  if (!vet.ok && vet.saida) problemas.push(`go vet ${pacote}:\n${vet.saida.split('\n').slice(0, 20).join('\n')}`);

  return problemas.length ? { codigo: 0, contexto: problemas.join('\n') } : { codigo: 0 };
}

await executar(import.meta.url, 'PostToolUse', avaliar);
