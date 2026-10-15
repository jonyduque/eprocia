// Static checks for the Workflow scripts: they only run inside Claude Code, so
// here we parse them, validate meta and run them against fake agents.
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const DIR = path.dirname(fileURLToPath(import.meta.url));
const CONFIG = JSON.parse(readFileSync(path.join(DIR, '..', 'review.json'), 'utf8'));
const AsyncFunction = (async () => {}).constructor;

function carregar(nome) {
  const src = readFileSync(path.join(DIR, nome), 'utf8').replace(/\r\n/g, '\n');
  const m = src.match(/^export const meta = (\{[\s\S]*?\n\})\n/);
  assert.ok(m, `${nome}: export const meta ausente no topo`);
  const meta = Function(`return (${m[1]})`)();
  const corpo = src.slice(m[0].length);
  const fn = new AsyncFunction('args', 'agent', 'parallel', 'pipeline', 'phase', 'log', 'budget', 'workflow', corpo);
  return { meta, corpo, fn };
}

// Minimal re-implementations of the Workflow runtime hooks.
async function pipeline(items, ...stages) {
  return Promise.all(items.map(async (item, i) => {
    let r = item;
    try {
      for (const s of stages) r = await s(r, item, i);
      return r;
    } catch {
      return null;
    }
  }));
}

function rodar(nome, args, responder) {
  const { fn } = carregar(nome);
  const chamadas = [];
  const agent = async (prompt, opts = {}) => {
    chamadas.push({ prompt, opts });
    return responder(prompt, opts);
  };
  const parallel = (thunks) => Promise.all(thunks.map((t) => t().catch(() => null)));
  return fn(args, agent, parallel, pipeline, () => {}, () => {}, { total: null }, async () => {})
    .then((resultado) => ({ resultado, chamadas }));
}

for (const nome of ['eprocia-revisao.js', 'eprocia-corrigir.js']) {
  describe(nome, () => {
    it('meta literal com nome, descrição e fases', () => {
      const { meta, corpo } = carregar(nome);
      assert.equal(meta.name, nome.replace('.js', ''));
      assert.ok(meta.description);
      const usadas = new Set([...corpo.matchAll(/phase\('([^']+)'\)|phase: '([^']+)'/g)].map((x) => x[1] ?? x[2]));
      for (const f of usadas) assert.ok(meta.phases.some((p) => p.title === f), `fase sem meta: ${f}`);
    });
    it('não usa APIs proibidas no runtime de workflow', () => {
      const { corpo } = carregar(nome);
      assert.doesNotMatch(corpo, /Date\.now\(|Math\.random\(|new Date\(\)|require\(|import\(/);
    });
  });
}

describe('eprocia-revisao (simulado)', () => {
  const achado = (lente) => ({ lente, titulo: 't', arquivo: 'a.go', linha: 1, gravidade: 'alta', evidencia: 'e', proposta: 'p', custo: 'P', ganho_esperado: 'g' });
  const responder = (prompt, opts) => {
    if (opts.label?.startsWith('lente:')) return { achados: [achado('X'), achado('X')] };
    if (opts.label?.startsWith('cetico:')) {
      return { vereditos: [{ indice: 0, veredito: 'mantido', justificativa: 'ok' }, { indice: 1, veredito: 'derrubado', justificativa: 'não' }] };
    }
    if (opts.label === 'visual:capturas') return { ok: true, capturas: ['docs/revisao/capturas/x.png'] };
    if (opts.label === 'sintese') return { relatorio: 'docs/revisao/2026-10-15-relatorio.md', ids: ['B-TE-01'], derrubados: 1 };
    throw new Error(`agente inesperado: ${opts.label}`);
  };

  it('smoke backend com só a lente TE', async () => {
    const { resultado, chamadas } = await rodar('eprocia-revisao.js', { escopo: 'backend', data: '2026-10-15', config: CONFIG, lentes: ['TE'] }, responder);
    assert.deepEqual(chamadas.map((c) => c.opts.label), ['lente:B-TE', 'cetico:B-TE', 'sintese']);
    assert.equal(resultado.mantidos, 1);
    assert.equal(resultado.derrubados, 1);
    assert.equal(chamadas[0].opts.agentType, 'revisor-go');
    assert.match(chamadas[2].prompt, /docs\/revisao\/2026-10-15-relatorio\.md/);
  });

  it('tudo: 10 lentes, 10 céticos, captura e síntese; RS/VI recebem as capturas', async () => {
    const { chamadas } = await rodar('eprocia-revisao.js', { escopo: 'tudo', data: '2026-10-15', config: CONFIG }, responder);
    const rotulos = chamadas.map((c) => c.opts.label);
    assert.equal(rotulos.filter((l) => l.startsWith('lente:')).length, 10);
    assert.equal(rotulos.filter((l) => l.startsWith('cetico:')).length, 10);
    assert.equal(chamadas.length, 22);
    const rs = chamadas.find((c) => c.opts.label === 'lente:F-RS');
    assert.match(rs.prompt, /capturas\/x\.png/);
    assert.doesNotMatch(chamadas.find((c) => c.opts.label === 'lente:F-AC').prompt, /capturas\/x\.png/);
  });

  it('cético sem veredito derruba o achado', async () => {
    const { resultado } = await rodar('eprocia-revisao.js', { escopo: 'backend', data: '2026-10-15', config: CONFIG, lentes: ['EF'] },
      (p, o) => (o.label?.startsWith('cetico:') ? { vereditos: [] } : responder(p, o)));
    assert.equal(resultado.mantidos, 0);
    assert.equal(resultado.derrubados, 2);
  });

  it('agente de papel indisponível cai no agente padrão com a definição', async () => {
    const { chamadas } = await rodar('eprocia-revisao.js', { escopo: 'backend', data: '2026-10-15', config: CONFIG, lentes: ['TE'] }, (p, o) => {
      if (o.agentType) throw new Error('unknown agent type');
      return responder(p, o);
    });
    const lente = chamadas.filter((c) => c.opts.label === 'lente:B-TE');
    assert.equal(lente.length, 2);
    assert.match(lente[1].prompt, /\.claude\/agents\/revisor-go\.md/);
  });

  it('rejeita args inválidos', async () => {
    await assert.rejects(rodar('eprocia-revisao.js', { escopo: 'x', data: '2026-10-15', config: CONFIG }, responder), /escopo/);
  });
});

describe('eprocia-corrigir (simulado)', () => {
  const base = {
    ok: true, base: 'abc123', nao_rastreados: ['.genkit/'], passo0_feito: true, svelte5_feito: true, ids_ausentes: ['B-XX-09'],
    achados: [
      { id: 'F-RS-01', titulo: 'divisor', arquivo: 'frontend/src/App.svelte', evidencia: 'e', proposta: 'p' },
      { id: 'B-EF-01', titulo: 'ocr em lote', arquivo: 'internal/ocr/tesseract.go', evidencia: 'e', proposta: 'p' },
      { id: 'B-ES-01', titulo: 'goroutine', arquivo: 'internal/etl/queue.go', evidencia: 'e', proposta: 'p' },
    ],
  };
  const args = { data: '2026-10-15', relatorio: 'docs/revisao/2026-10-15-relatorio.md', ids: ['F-RS-01', 'B-EF-01', 'B-ES-01', 'B-XX-09'], config: CONFIG, atribuicao: 'Co-Authored-By: X' };

  function responder(prep, { reprovar = () => false } = {}) {
    return (prompt, o) => {
      const l = o.label ?? '';
      if (l === 'preparar') return prep;
      if (l.startsWith('implementar:')) return { arquivos: ['x'], teste: 't', comandos: 'go test' };
      if (l.startsWith('gate:')) return { passou: true, resumo: 'ok' };
      if (l.startsWith('revisar:')) return { aprovado: !reprovar(l), motivo: 'm' };
      if (l.startsWith('commit:')) return { ok: true, sha: 'deadbee' };
      return 'ok';
    };
  }

  it('ordem ES → EF → frontend, um commit por achado, ausentes pulados', async () => {
    const { resultado, chamadas } = await rodar('eprocia-corrigir.js', args, responder(base));
    const commits = chamadas.filter((c) => c.opts.label?.startsWith('commit:')).map((c) => c.opts.label);
    assert.deepEqual(commits, ['commit:B-ES-01', 'commit:B-EF-01', 'commit:F-RS-01']);
    assert.deepEqual(resultado.status, { 'B-XX-09': 'pulado', 'B-ES-01': 'feito', 'B-EF-01': 'feito', 'F-RS-01': 'feito' });
    const commitEF = chamadas.find((c) => c.opts.label === 'commit:B-EF-01').prompt;
    assert.match(commitEF, /fix\(backend\): ocr em lote \[B-EF-01\]/);
    assert.match(commitEF, /Co-Authored-By: X/);
    assert.match(chamadas.find((c) => c.opts.label === 'gate:B-EF-01#1').prompt, /benchstat/);
    assert.match(chamadas.find((c) => c.opts.label === 'gate:F-RS-01#1').prompt, /svelte-check/);
  });

  it('pré-requisitos: Passo 0 e Svelte 5 antes dos achados', async () => {
    const { chamadas } = await rodar('eprocia-corrigir.js', args, responder({ ...base, passo0_feito: false, svelte5_feito: false }));
    const commits = chamadas.filter((c) => c.opts.label?.startsWith('commit:')).map((c) => c.opts.label);
    assert.deepEqual(commits.slice(0, 2), ['commit:passo0', 'commit:svelte5']);
  });

  it('três reprovações descartam a tarefa e marcam bloqueado', async () => {
    const { resultado, chamadas } = await rodar('eprocia-corrigir.js', args, responder(base, { reprovar: (l) => l.startsWith('revisar:B-ES-01') }));
    assert.equal(resultado.status['B-ES-01'], 'bloqueado');
    assert.equal(chamadas.filter((c) => c.opts.label?.startsWith('implementar:B-ES-01')).length, 3);
    const descarte = chamadas.find((c) => c.opts.label === 'descartar:B-ES-01').prompt;
    assert.match(descarte, /\.genkit\//);
    assert.doesNotMatch(descarte, /git reset --hard(?! nem)/);
    assert.match(chamadas.find((c) => c.opts.label === 'implementar:B-ES-01#2').prompt, /revisor reprovou/);
    assert.equal(resultado.status['B-EF-01'], 'feito');
  });

  it('para sem tocar nada se a preparação falhar', async () => {
    const { resultado, chamadas } = await rodar('eprocia-corrigir.js', args, responder({ ...base, ok: false, motivo: 'árvore suja' }));
    assert.equal(resultado.erro, 'árvore suja');
    assert.equal(chamadas.length, 1);
  });

  it('Passo 0 com falha impede qualquer correção', async () => {
    const { resultado, chamadas } = await rodar('eprocia-corrigir.js', args, responder({ ...base, passo0_feito: false }, { reprovar: (l) => l.startsWith('revisar:passo0') }));
    assert.match(resultado.erro, /Passo 0 falhou/);
    assert.ok(!chamadas.some((c) => c.opts.label?.startsWith('implementar:B-')));
  });
});
