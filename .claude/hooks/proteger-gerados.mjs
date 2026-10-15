// PreToolUse (Edit|Write|MultiEdit|NotebookEdit): blocks edits to generated or embedded files.
import { caminhoDoEvento, executar, normalizar } from './lib.mjs';

const PROTEGIDOS = [
  ['frontend/src/wailsjs/', 'bindings gerados pelo Wails (rode `wails dev`/`wails build` para regenerar)'],
  ['frontend/dist/', 'build do frontend (gerado por `npm run build`)'],
  ['internal/ocr/embed/', 'binários e tessdata embutidos no executável'],
  ['build/bin/', 'saída do `wails build`'],
];

export function avaliar(evento, ctx = {}) {
  const caminho = normalizar(caminhoDoEvento(evento), ctx.raiz);
  if (!caminho) return { codigo: 0 };
  for (const [prefixo, motivo] of PROTEGIDOS) {
    if (caminho.startsWith(prefixo)) {
      return { codigo: 2, stderr: `Bloqueado: ${caminho} é ${motivo}. Não edite à mão.` };
    }
  }
  return { codigo: 0 };
}

await executar(import.meta.url, 'PreToolUse', avaliar);
