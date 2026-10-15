// PreToolUse (Bash): keeps real process data (ZIPs, PDFs, processing outputs,
// the benchmark corpus) out of git.
import { execFileSync } from 'node:child_process';
import { executar, normalizar, RAIZ } from './lib.mjs';

// .txt files are processing outputs unless they live in harness/doc folders.
const TXT_PERMITIDO = /^(docs|scripts|\.claude)\//;

export function arquivoSensivel(caminho, benchDir) {
  const n = normalizar(caminho);
  if (!n) return false;
  if (benchDir) {
    const b = normalizar(benchDir).replace(/\/$/, '');
    if (n === b || n.startsWith(b + '/')) return true;
  }
  if (/\.(zip|pdf)$/.test(n)) return true;
  if (n.endsWith('.txt') && !TXT_PERMITIDO.test(n)) return true;
  return false;
}

// tokens splits one shell segment into words, honouring single/double quotes.
function tokens(segmento) {
  const out = [];
  const re = /"([^"]*)"|'([^']*)'|(\S+)/g;
  let m;
  while ((m = re.exec(segmento))) out.push(m[1] ?? m[2] ?? m[3]);
  return out;
}

const AMPLOS = new Set(['-a', '-A', '--all', '.', ':/', '-u', '--update']);

export function avaliar(evento, ctx = {}) {
  const cmd = evento?.tool_input?.command;
  if (typeof cmd !== 'string' || !/\bgit\b/.test(cmd)) return { codigo: 0 };
  const benchDir = ctx.benchDir ?? process.env.EPROCIA_BENCH_DIR;

  for (const segmento of cmd.split(/&&|\|\||;|\|/)) {
    const t = tokens(segmento.trim());
    const i = t.indexOf('git');
    if (i < 0) continue;
    const sub = t[i + 1];
    if (sub !== 'add' && sub !== 'commit') continue;
    const args = t.slice(i + 2);

    const diretos = args.filter((a) => !a.startsWith('-') && a !== '.' && a !== ':/');
    const ruins = sub === 'add' ? diretos.filter((a) => arquivoSensivel(a, benchDir)) : [];
    if (ruins.length) {
      return { codigo: 2, stderr: `Bloqueado: dados reais não entram no git (${ruins.join(', ')}). Adicione ao .gitignore se for saída local.` };
    }

    const amplo = args.some((a) => AMPLOS.has(a)) || (sub === 'commit' && args.some((a) => /^-[a-z]*a/.test(a)));
    if (amplo) {
      const candidatos = (ctx.naoIgnorados ?? listarNaoIgnorados)().filter((a) => arquivoSensivel(a, benchDir));
      if (candidatos.length) {
        return { codigo: 2, stderr: `Bloqueado: \`git ${sub}\` amplo incluiria dados reais: ${candidatos.slice(0, 5).join(', ')}. Adicione arquivos pelo nome ou ignore-os.` };
      }
    }
  }
  return { codigo: 0 };
}

// listarNaoIgnorados returns modified and untracked, non-ignored paths.
function listarNaoIgnorados() {
  try {
    const saida = execFileSync('git', ['status', '--porcelain', '-uall'], { cwd: RAIZ, encoding: 'utf8', timeout: 800 });
    return saida.split('\n').filter(Boolean).map((l) => l.slice(3).replace(/^"|"$/g, '').split(' -> ').pop());
  } catch {
    return [];
  }
}

await executar(import.meta.url, 'PreToolUse', avaliar);
