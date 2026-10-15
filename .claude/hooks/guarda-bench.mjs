// PreToolUse (Bash): warns (never blocks) when a Go benchmark runs with too few
// samples for benchstat to reach significance.
import { executar } from './lib.mjs';

const MINIMO = 6;

export function avaliar(evento) {
  const cmd = evento?.tool_input?.command;
  if (typeof cmd !== 'string' || !/\bgo\s+test\b/.test(cmd) || !/-(test\.)?bench(=|\s)/.test(cmd)) return { codigo: 0 };
  const m = cmd.match(/-(?:test\.)?count[=\s](\d+)/);
  const count = m ? Number(m[1]) : 1;
  if (count >= MINIMO) return { codigo: 0 };
  return {
    codigo: 0,
    contexto: `Aviso (guarda-bench): benchmark com -count=${count}. Para comparar com benchstat use -count=10 (mínimo ${MINIMO}); com menos amostras o resultado não serve como evidência de ganho.`,
  };
}

await executar(import.meta.url, 'PreToolUse', avaliar);
