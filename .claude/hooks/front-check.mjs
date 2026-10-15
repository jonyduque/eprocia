// PostToolUse (Edit|Write|MultiEdit): svelte-check limited to the edited
// frontend file. Inactive (informs once per call) until svelte-check exists,
// i.e. before the Svelte 5 migration.
import { execFile } from 'node:child_process';
import { existsSync } from 'node:fs';
import path from 'node:path';
import { caminhoDoEvento, executar, normalizar, RAIZ } from './lib.mjs';

const ALVO = /^frontend\/src\/.+\.(svelte|js|ts|css)$/;

export function ehAlvo(caminho) {
  return ALVO.test(caminho) && !caminho.startsWith('frontend/src/wailsjs/');
}

function rodarPadrao(cmd, args) {
  return new Promise((resolve) => {
    execFile(cmd, args, { cwd: path.join(RAIZ, 'frontend'), timeout: 60000, encoding: 'utf8', shell: process.platform === 'win32' },
      (err, stdout, stderr) => resolve({ ok: !err, saida: `${stdout ?? ''}${stderr ?? ''}`.trim() }));
  });
}

export async function avaliar(evento, ctx = {}) {
  const caminho = normalizar(caminhoDoEvento(evento), ctx.raiz);
  if (!ehAlvo(caminho)) return { codigo: 0 };

  const instalado = ctx.instalado ?? existsSync(path.join(RAIZ, 'frontend', 'node_modules', 'svelte-check'));
  if (!instalado) {
    return { codigo: 0, contexto: 'front-check inativo: svelte-check não instalado (chega com a migração para Svelte 5).' };
  }

  const rodar = ctx.rodar ?? rodarPadrao;
  const r = await rodar('npx', ['svelte-check', '--output', 'machine', '--threshold', 'warning']);
  const relativo = caminho.slice('frontend/'.length);
  const linhas = r.saida.split('\n').filter((l) => /\s(ERROR|WARNING)\s/.test(l) && l.replace(/\\/g, '/').toLowerCase().includes(relativo));
  if (!linhas.length) return { codigo: 0 };
  return { codigo: 0, contexto: `svelte-check em ${caminho}:\n${linhas.slice(0, 20).join('\n')}` };
}

await executar(import.meta.url, 'PostToolUse', avaliar);
