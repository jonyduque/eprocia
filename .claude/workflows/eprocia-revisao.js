export const meta = {
  name: 'eprocia-revisao',
  description: 'Revisão do eprocIA: lentes em paralelo, captura visual, refutação cética e relatório priorizado',
  whenToUse: 'Chamado pelo comando /revisar com args { escopo, data, config, lentes? }',
  phases: [
    { title: 'Lentes', detail: 'um revisor read-only por lente' },
    { title: 'Visual', detail: 'capturas com dados fictícios (frontend)' },
    { title: 'Refutação', detail: 'um cético por lente tenta derrubar os achados' },
    { title: 'Síntese', detail: 'dedup, ordenação, ids estáveis e relatório' },
  ],
}

// args: { escopo: 'backend'|'frontend'|'tudo', data: 'AAAA-MM-DD', config: <.claude/review.json>, lentes?: ['TE', ...] }
const { escopo, data, config } = args ?? {}
if (!['backend', 'frontend', 'tudo'].includes(escopo)) throw new Error('args.escopo deve ser backend, frontend ou tudo')
if (!/^\d{4}-\d{2}-\d{2}$/.test(data ?? '')) throw new Error('args.data deve ser AAAA-MM-DD')
if (!config?.camadas) throw new Error('args.config deve ser o conteúdo de .claude/review.json')

const filtroLentes = Array.isArray(args.lentes) && args.lentes.length ? new Set(args.lentes) : null
const camadas = escopo === 'tudo' ? ['backend', 'frontend'] : [escopo]
const relatorio = `docs/revisao/${data}-relatorio.md`
const pastaCapturas = `${config.visual.saida}/${data}/antes`

const ACHADOS = {
  type: 'object',
  properties: {
    achados: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          lente: { type: 'string' },
          titulo: { type: 'string' },
          arquivo: { type: 'string' },
          linha: { type: 'integer' },
          gravidade: { enum: ['alta', 'media', 'baixa'] },
          evidencia: { type: 'string' },
          proposta: { type: 'string' },
          custo: { enum: ['P', 'M', 'G'] },
          ganho_esperado: { type: 'string' },
        },
        required: ['lente', 'titulo', 'arquivo', 'linha', 'gravidade', 'evidencia', 'proposta', 'custo', 'ganho_esperado'],
      },
    },
  },
  required: ['achados'],
}

const VEREDITOS = {
  type: 'object',
  properties: {
    vereditos: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          indice: { type: 'integer' },
          veredito: { enum: ['mantido', 'rebaixado', 'derrubado'] },
          gravidade_corrigida: { enum: ['alta', 'media', 'baixa'] },
          justificativa: { type: 'string' },
        },
        required: ['indice', 'veredito', 'justificativa'],
      },
    },
  },
  required: ['vereditos'],
}

const CAPTURAS = {
  type: 'object',
  properties: { ok: { type: 'boolean' }, capturas: { type: 'array', items: { type: 'string' } }, erro: { type: 'string' } },
  required: ['ok', 'capturas'],
}

const SINTESE = {
  type: 'object',
  properties: {
    relatorio: { type: 'string' },
    ids: { type: 'array', items: { type: 'string' } },
    derrubados: { type: 'integer' },
  },
  required: ['relatorio', 'ids', 'derrubados'],
}

// comPapel runs an agent with a project agent type; if the type is not
// registered in this session (agents added after it started), it falls back to
// the default agent told to follow the same definition file.
async function comPapel(papel, prompt, opts) {
  try {
    return await agent(prompt, { ...opts, agentType: papel })
  } catch (err) {
    log(`agente ${papel} indisponível (${err.message}); usando o agente padrão com .claude/agents/${papel}.md`)
    return agent(`Antes de tudo, leia e siga à risca .claude/agents/${papel}.md (seu papel).\n\n${prompt}`, opts)
  }
}

// The visual capture starts at once; only the RS and VI lenses wait for it.
let capturasPromise = Promise.resolve(null)
if (camadas.includes('frontend')) {
  phase('Visual')
  capturasPromise = agent(
    `Use a skill eprocia-visual para capturar o frontend do eprocIA com dados fictícios.\n` +
      `Rode: node scripts/visual/capturar.mjs --saida ${pastaCapturas}\n` +
      `Não rode wails dev. Confirme que o Vite foi encerrado ao fim. ` +
      `Devolva ok=true e a lista de caminhos relativos dos PNGs gerados, ou ok=false com o erro.`,
    { label: 'visual:capturas', phase: 'Visual', schema: CAPTURAS, effort: 'low' },
  ).then((r) => {
    if (!r?.ok) log(`captura visual falhou: ${r?.erro ?? 'sem resposta'}; lentes RS/VI seguem só com o código`)
    return r?.ok ? r.capturas : null
  })
}

const tarefas = []
for (const camada of camadas) {
  const c = config.camadas[camada]
  for (const lente of c.lentes) {
    if (filtroLentes && !filtroLentes.has(lente.id)) continue
    tarefas.push({ camada, prefixo: c.prefixo, papel: c.agente, arquivos: c.arquivos, lente })
  }
}
if (!tarefas.length) throw new Error('nenhuma lente selecionada')
log(`${tarefas.length} lentes: ${tarefas.map((t) => `${t.prefixo}-${t.lente.id}`).join(', ')}`)

const resultados = await pipeline(
  tarefas,
  // Stage 1: one read-only reviewer per lens.
  async (t) => {
    let capturas = null
    if (t.lente.id === 'RS' || t.lente.id === 'VI') capturas = await capturasPromise
    const pistas = t.lente.pistas?.length ? `\nPistas (verifique, não presuma): ${t.lente.pistas.join('; ')}.` : ''
    const imgs = capturas?.length
      ? `\nCapturas disponíveis como evidência complementar (abra as relevantes com Read; o nome indica tamanho-tema-estado):\n${capturas.join('\n')}`
      : ''
    const r = await comPapel(
      t.papel,
      `Lente ${t.lente.id} (${t.lente.nome}) da camada ${t.camada} do eprocIA.\n` +
        `Pergunta: ${t.lente.pergunta}${pistas}\n` +
        `Arquivos da camada (globs): ${t.arquivos.join(', ')}.${imgs}\n` +
        `Devolva os achados no esquema, com lente="${t.lente.id}". Só achados com evidência conferida.`,
      { label: `lente:${t.prefixo}-${t.lente.id}`, phase: 'Lentes', schema: ACHADOS },
    )
    return (r?.achados ?? []).map((a) => ({ ...a, lente: t.lente.id }))
  },
  // Stage 2: one skeptic per lens tries to refute every finding of that lens.
  async (achados, t) => {
    if (!achados.length) return { t, achados: [], vereditos: [] }
    const lista = achados.map((a, i) => `[${i}] ${JSON.stringify(a)}`).join('\n')
    const v = await comPapel(
      'cetico',
      `Achados da lente ${t.prefixo}-${t.lente.id} (${t.lente.nome}) do eprocIA. Tente derrubar cada um.\n${lista}\n` +
        `Devolva um veredito por índice (0..${achados.length - 1}).`,
      { label: `cetico:${t.prefixo}-${t.lente.id}`, phase: 'Refutação', schema: VEREDITOS },
    )
    return { t, achados, vereditos: v?.vereditos ?? [] }
  },
)

// Barrier: the synthesis needs every lens to dedupe and rank across them.
const mantidos = []
const derrubados = []
for (const r of resultados.filter(Boolean)) {
  r.achados.forEach((a, i) => {
    const v = r.vereditos.find((x) => x.indice === i)
    const item = { camada: r.t.camada, prefixo: r.t.prefixo, ...a, veredito: v?.veredito ?? 'derrubado', justificativa: v?.justificativa ?? 'sem veredito do cético' }
    if (v?.veredito === 'rebaixado' && v.gravidade_corrigida) item.gravidade = v.gravidade_corrigida
    ;(item.veredito === 'derrubado' ? derrubados : mantidos).push(item)
  })
}
const falhas = resultados.filter((r) => !r).length
if (falhas) log(`${falhas} lente(s) sem resultado (agente pulado ou com erro)`)
log(`${mantidos.length} achados sobreviveram, ${derrubados.length} derrubados`)

phase('Síntese')
const capturas = await capturasPromise
const sintese = await agent(
  `Você é a síntese da revisão do eprocIA de ${data} (escopo: ${escopo}${filtroLentes ? `, lentes: ${[...filtroLentes].join(',')}` : ''}).\n\n` +
    `ACHADOS QUE SOBREVIVERAM (JSON):\n${JSON.stringify(mantidos, null, 1)}\n\n` +
    `DERRUBADOS (JSON):\n${JSON.stringify(derrubados, null, 1)}\n\n` +
    `Faça:\n` +
    `1. Remova duplicatas entre lentes (mesmo problema no mesmo trecho): fique com a versão de melhor evidência e cite as lentes que o acharam.\n` +
    `2. Ordene por gravidade × ganho ÷ custo (alta=3, media=2, baixa=1; P=1, M=2, G=3; use o ganho_esperado para desempatar).\n` +
    `3. Atribua ids estáveis <prefixo>-<lente>-<nn> (ex.: B-EF-01, F-RS-02), numerando por lente na ordem final, começando em 01.\n` +
    `4. Escreva ${relatorio} em pt-BR (prosa normal), com: título e data; escopo; tabela-resumo (id, título, gravidade, custo, ganho, veredito); ` +
    `uma seção "### <id> — <título>" por achado com arquivo:linha, evidência, proposta, custo, ganho esperado, veredito e justificativa do cético e uma linha "Status: pendente"; ` +
    (capturas?.length ? `uma seção "Capturas" com links relativos para ${pastaCapturas}/; ` : '') +
    (camadas.includes('frontend') ? `uma seção "Teste manual guiado" para o que as capturas não cobrem (bordas de resize da janela frameless, maximizar/restaurar, arrastar a barra de título, divisor nos limites); ` : '') +
    `uma seção "Derrubados" (título, lente, justificativa) para auditoria; e, ao fim, "Como aprovar": /corrigir <ids>.\n` +
    `5. Escreva docs/revisao/estado.json: {"relatorio":"${relatorio}","fase":"aguardando-aprovacao","ids":{<id>:"pendente",...}}.\n` +
    `Se o relatório já existir (rodada anterior do mesmo dia), substitua-o. Não faça commit. Nunca inclua dados reais (nomes de processos reais, caminhos de usuário).\n` +
    `Devolva o caminho do relatório, a lista de ids na ordem e o número de derrubados.`,
  { label: 'sintese', phase: 'Síntese', schema: SINTESE },
)

return { relatorio: sintese?.relatorio ?? relatorio, ids: sintese?.ids ?? [], mantidos: mantidos.length, derrubados: derrubados.length, lentesSemResultado: falhas }
