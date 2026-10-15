// Shared helpers for the eprocIA review harness hooks.
// Hooks never block because of their own failures: bad input means exit 0.
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

export const RAIZ = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');

// lerEvento reads the hook event JSON from stdin; null when absent or invalid.
export async function lerEvento() {
  let dados = '';
  for await (const parte of process.stdin) dados += parte;
  try {
    return JSON.parse(dados);
  } catch {
    return null;
  }
}

// normalizar turns any Windows/POSIX path into a lowercase, forward-slash path
// relative to the repository root (or the absolute form when outside it).
export function normalizar(p, raiz = RAIZ) {
  if (!p) return '';
  // Git Bash style /c/Users/... becomes c:/Users/...
  let s = String(p).replace(/\\/g, '/').replace(/^\/([a-z])\//i, '$1:/');
  const r = raiz.replace(/\\/g, '/').replace(/\/$/, '');
  if (/^[a-z]:\//i.test(s) || s.startsWith('/')) {
    if (s.toLowerCase().startsWith(r.toLowerCase() + '/')) s = s.slice(r.length + 1);
  }
  return s.replace(/^\.\//, '').toLowerCase();
}

// caminhoDoEvento extracts the edited file path from Edit/Write/MultiEdit/NotebookEdit.
export function caminhoDoEvento(evento) {
  const ti = evento?.tool_input;
  return ti?.file_path ?? ti?.notebook_path ?? ti?.path ?? '';
}

// responder applies an avaliar() result: exit 2 to block, JSON context otherwise.
export function responder(resultado, nomeEvento) {
  if (!resultado) process.exit(0);
  if (resultado.codigo === 2) {
    process.stderr.write(resultado.stderr + '\n');
    process.exit(2);
  }
  if (resultado.contexto) {
    process.stdout.write(JSON.stringify({
      hookSpecificOutput: { hookEventName: nomeEvento, additionalContext: resultado.contexto },
    }));
  }
  process.exit(0);
}

export function ehScriptPrincipal(metaUrl) {
  return Boolean(process.argv[1]) && metaUrl === pathToFileURL(process.argv[1]).href;
}

// executar wires a hook's avaliar() to stdin/stdout, swallowing its own errors.
export async function executar(metaUrl, nomeEvento, avaliar, criarCtx = () => ({})) {
  if (!ehScriptPrincipal(metaUrl)) return;
  try {
    const evento = await lerEvento();
    if (!evento) process.exit(0);
    responder(await avaliar(evento, await criarCtx(evento)), nomeEvento);
  } catch (err) {
    process.stderr.write(`[hook] erro interno ignorado: ${err.message}\n`);
    process.exit(0);
  }
}
