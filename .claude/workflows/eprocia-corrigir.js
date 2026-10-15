export const meta = {
  name: 'eprocia-corrigir',
  description: 'Corrige achados aprovados da revisão do eprocIA: um implementador e um revisor novos por achado, gates e um commit por id',
  whenToUse: 'Chamado pelo comando /corrigir com args { data, relatorio, ids, config }',
  phases: [
    { title: 'Preparação', detail: 'branch, leitura dos achados e pré-requisitos' },
    { title: 'Pré-requisitos', detail: 'Passo 0 do toolchain e migração para Svelte 5' },
    { title: 'Correção', detail: 'implementador → gate → revisor → commit, por achado' },
    { title: 'Final', detail: 'revisão da branch e relatório' },
  ],
}

// args: { data: 'AAAA-MM-DD', relatorio: 'docs/revisao/AAAA-MM-DD-relatorio.md', ids: ['B-ES-01', ...],
//         config: <.claude/review.json>, atribuicao: '<linhas de atribuição de commit da sessão>' }
const { data, relatorio, ids, config } = args ?? {}
if (!/^\d{4}-\d{2}-\d{2}$/.test(data ?? '')) throw new Error('args.data deve ser AAAA-MM-DD')
if (!relatorio) throw new Error('args.relatorio é obrigatório')
if (!Array.isArray(ids) || !ids.length) throw new Error('args.ids deve ser uma lista de ids do relatório')
if (!config?.correcao) throw new Error('args.config deve ser o conteúdo de .claude/review.json')

const TENTATIVAS = config.correcao.tentativas_max ?? 3
const branch = `${config.correcao.branch_prefixo ?? 'revisao/'}${data}`
// Commit attribution lines come from the calling session (they change per session).
const ATRIBUICAO = typeof args.atribuicao === 'string' ? args.atribuicao.trim() : ''

const PREPARO = {
  type: 'object',
  properties: {
    ok: { type: 'boolean' },
    motivo: { type: 'string' },
    base: { type: 'string' },
    nao_rastreados: { type: 'array', items: { type: 'string' } },
    passo0_feito: { type: 'boolean' },
    svelte5_feito: { type: 'boolean' },
    achados: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          titulo: { type: 'string' },
          arquivo: { type: 'string' },
          linha: { type: 'integer' },
          gravidade: { type: 'string' },
          evidencia: { type: 'string' },
          proposta: { type: 'string' },
          custo: { type: 'string' },
          ganho_esperado: { type: 'string' },
        },
        required: ['id', 'titulo', 'arquivo', 'evidencia', 'proposta'],
      },
    },
    ids_ausentes: { type: 'array', items: { type: 'string' } },
  },
  required: ['ok', 'base', 'nao_rastreados', 'passo0_feito', 'svelte5_feito', 'achados', 'ids_ausentes'],
}

const IMPLEMENTACAO = {
  type: 'object',
  properties: {
    arquivos: { type: 'array', items: { type: 'string' } },
    teste: { type: 'string' },
    comandos: { type: 'string' },
    escopo_ampliado: { type: 'string' },
    observacoes: { type: 'string' },
  },
  required: ['arquivos', 'teste', 'comandos'],
}

const GATE = {
  type: 'object',
  properties: {
    passou: { type: 'boolean' },
    resumo: { type: 'string' },
    benchstat: { type: 'string' },
    capturas: { type: 'array', items: { type: 'string' } },
  },
  required: ['passou', 'resumo'],
}

const REVISAO = {
  type: 'object',
  properties: { aprovado: { type: 'boolean' }, motivo: { type: 'string' } },
  required: ['aprovado', 'motivo'],
}

const COMMIT = {
  type: 'object',
  properties: { ok: { type: 'boolean' }, sha: { type: 'string' }, erro: { type: 'string' } },
  required: ['ok'],
}

async function comPapel(papel, prompt, opts) {
  try {
    return await agent(prompt, { ...opts, agentType: papel })
  } catch (err) {
    log(`agente ${papel} indisponível (${err.message}); usando o agente padrão com .claude/agents/${papel}.md`)
    return agent(`Antes de tudo, leia e siga à risca .claude/agents/${papel}.md (seu papel).\n\n${prompt}`, opts)
  }
}

const camadaDe = (id) => (id.startsWith('F-') ? 'frontend' : 'backend')
const lenteDe = (id) => id.split('-')[1]

// ---------------------------------------------------------------- Preparação
phase('Preparação')
const prep = await agent(
  `Prepare a correção de achados do eprocIA.\n` +
    `1. Rode git status --porcelain. Se houver arquivos rastreados modificados, pare com ok=false e motivo (não descarte nada do usuário).\n` +
    `2. Garanta a branch ${branch}: se existir, git switch ${branch}; senão, git switch -c ${branch} a partir do HEAD atual. Nunca trabalhe na main. Nunca faça push.\n` +
    `3. base = git rev-parse HEAD (depois do switch). nao_rastreados = caminhos de git status --porcelain com "??" (para não apagá-los depois).\n` +
    `4. Leia ${relatorio} e devolva, para cada id em ${JSON.stringify(ids)}, os campos da seção do achado. Ids que não existem vão em ids_ausentes.\n` +
    `5. passo0_feito = o go.mod declara go 1.27 ou maior E "tool golang.org/x/perf/cmd/benchstat" E go version -m "$(go env GOPATH)/bin/wails.exe" mostra golang.org/x/tools acima de v0.30.0.\n` +
    `6. svelte5_feito = frontend/package.json depende de svelte ^5 e de lucide-svelte e svelte-check.\n` +
    `7. Escreva docs/revisao/estado.json: {"relatorio":"${relatorio}","fase":"correcao","ids":{<cada id pedido>:"pendente"}} preservando outros ids que já estejam no arquivo.`,
  { label: 'preparar', phase: 'Preparação', schema: PREPARO, effort: 'low' },
)
if (!prep?.ok) return { erro: prep?.motivo ?? 'preparação falhou' }
if (prep.ids_ausentes.length) log(`ids fora do relatório, ignorados: ${prep.ids_ausentes.join(', ')}`)

const status = Object.fromEntries(prep.ids_ausentes.map((id) => [id, 'pulado']))
const protegidos = JSON.stringify(prep.nao_rastreados)

// executarTarefa runs implementer → gate → reviewer, up to TENTATIVAS times,
// then commits. On final failure it discards only this task's changes.
async function executarTarefa({ chave, rotulo, pedido, gate, mensagem, faseProgresso }) {
  let feedback = ''
  for (let tentativa = 1; tentativa <= TENTATIVAS; tentativa++) {
    const impl = await comPapel(
      'implementador',
      `${pedido}${feedback ? `\n\nTENTATIVA ${tentativa} de ${TENTATIVAS}. Motivo da reprovação anterior (corrija exatamente isto):\n${feedback}` : ''}`,
      { label: `implementar:${rotulo}#${tentativa}`, phase: faseProgresso, schema: IMPLEMENTACAO },
    )
    if (!impl) { feedback = 'o implementador não devolveu resultado'; continue }

    const g = await agent(
      `Gate da tarefa ${rotulo} do eprocIA (não edite código; só rode e relate).\n${gate}\n` +
        `Arquivos alterados segundo o implementador: ${impl.arquivos.join(', ')}.\n` +
        `Devolva passou=true só se todas as etapas passarem; em resumo, a última linha decisiva de cada etapa.`,
      { label: `gate:${rotulo}#${tentativa}`, phase: faseProgresso, schema: GATE },
    )
    if (!g?.passou) { feedback = `gate reprovado: ${g?.resumo ?? 'sem resposta'}`; continue }

    const rev = await comPapel(
      'revisor-correcao',
      `Revise o diff não commitado da tarefa ${rotulo}.\nPedido original:\n${pedido}\n\n` +
        `Retorno do implementador: ${JSON.stringify(impl)}\nResultado do gate: ${JSON.stringify(g)}`,
      { label: `revisar:${rotulo}#${tentativa}`, phase: faseProgresso, schema: REVISAO },
    )
    if (!rev?.aprovado) { feedback = `revisor reprovou: ${rev?.motivo ?? 'sem resposta'}`; continue }

    const c = await agent(
      `Faça o commit da tarefa ${rotulo} do eprocIA na branch ${branch} (nunca push).\n` +
        `Adicione pelo nome só os arquivos da tarefa (git status mostra quais; inclua go.mod e frontend/src/wailsjs se o build os regenerou). ` +
        `Nunca adicione docs/revisao/estado.json nem os caminhos ${protegidos}. Nunca use git add -A ou git add .\n` +
        `Mensagem (use um heredoc):\n${mensagem}\n\n${ATRIBUICAO}\n` +
        `Depois do commit, em docs/revisao/estado.json marque ids["${chave}"]="feito" (se a chave existir) sem commitar o estado.`,
      { label: `commit:${rotulo}`, phase: faseProgresso, schema: COMMIT, effort: 'low' },
    )
    if (c?.ok) return { feito: true, sha: c.sha, benchstat: g.benchstat, capturas: g.capturas }
    feedback = `commit falhou: ${c?.erro ?? 'sem resposta'}`
  }

  await agent(
    `A tarefa ${rotulo} do eprocIA falhou ${TENTATIVAS} vezes. Descarte só as mudanças dela:\n` +
      `- git restore --staged --worktree nos arquivos rastreados modificados (git status);\n` +
      `- apague arquivos não rastreados novos, exceto estes, que já existiam: ${protegidos} e docs/revisao/estado.json.\n` +
      `Não use git reset --hard nem git clean. Depois marque ids["${chave}"]="bloqueado" em docs/revisao/estado.json (se a chave existir).`,
    { label: `descartar:${rotulo}`, phase: faseProgresso, effort: 'low' },
  )
  return { feito: false, motivo: feedback }
}

const GATE_BUILD = 'Rode pwsh scripts/build/gate.ps1 (skill eprocia-build). Se eprocia.exe estiver aberto e o script pedir para fechá-lo, devolva passou=false com esse motivo; não encerre processos que você não iniciou.'
const GATE_VISUAL = `Rode também o gate visual da skill eprocia-visual: capturas em ${config.visual.saida}/${data}/depois (só os estados afetados, com --estados), cd frontend && npx svelte-check sem erros e npm run build sem warnings novos. Liste os PNGs em capturas.`
const GATE_BENCH = 'Rode o benchmark da skill eprocia-bench: compare o commit base (git stash não; use git worktree add em diretório temporário para o HEAD) com a árvore atual, -count=10, e cole a tabela do go tool benchstat em benchstat. Aceite só com p < 0,05 na métrica-alvo e nenhuma outra métrica pior que 5%; remova o worktree temporário.'
const GATE_SEM_REGRESSAO = 'Rode o benchmark da skill eprocia-bench (antes = HEAD via git worktree temporário, depois = árvore atual, -count=10) e reprove se alguma métrica piorar mais de 5% com p < 0,05. Cole a tabela em benchstat; remova o worktree.'

// ---------------------------------------------------------------- Pré-requisitos
phase('Pré-requisitos')
if (!prep.passo0_feito) {
  const r = await executarTarefa({
    chave: 'passo0',
    rotulo: 'passo0',
    faseProgresso: 'Pré-requisitos',
    pedido:
      'Execute o Passo 0 da skill eprocia-build: (1) se o Wails CLI ainda tiver x/tools v0.30.0, rode pwsh scripts/build/recompilar-wails.ps1; ' +
      '(2) go mod edit -go=1.27; go get -tool golang.org/x/perf/cmd/benchstat@latest; go mod tidy; (3) wails build -o eprocia-passo0.exe e apague esse exe. ' +
      'Não há teste novo a escrever: o teste é o build e a suíte existente. Atualize o comando do benchstat na skill eprocia-bench para go tool benchstat.',
    gate: GATE_BUILD,
    mensagem: 'chore(build): Go 1.27 e benchstat como tool',
  })
  if (!r.feito) return { erro: `Passo 0 falhou: ${r.motivo}. Nenhuma correção foi feita.`, status }
}

const achados = [...prep.achados].sort((a, b) => {
  const ordem = config.correcao.ordem
  return ordem.indexOf(lenteDe(a.id)) - ordem.indexOf(lenteDe(b.id))
})

if (!prep.svelte5_feito && achados.some((a) => camadaDe(a.id) === 'frontend')) {
  const r = await executarTarefa({
    chave: 'svelte5',
    rotulo: 'svelte5',
    faseProgresso: 'Pré-requisitos',
    pedido:
      'Migre o frontend (frontend/) para Svelte 5 e Vite atuais, com @sveltejs/vite-plugin-svelte compatível, e adicione lucide-svelte e svelte-check como dependências. ' +
      'Use versões estáveis publicadas há pelo menos duas semanas. Converta main.js para mount() do Svelte 5; o restante pode seguir em modo de compatibilidade, ' +
      'mas sem warnings de depreciação no build. Adicione o script "check": "svelte-check" no package.json. O comportamento deve ser idêntico: ' +
      `antes de mudar, capture node scripts/visual/capturar.mjs --saida ${config.visual.saida}/${data}/antes-migracao e, depois, compare com a mesma matriz em depois-migracao (as imagens devem coincidir salvo diferenças de antialiasing). ` +
      'Não troque ícones nem estilos nesta tarefa.',
    gate: `${GATE_BUILD}\n${GATE_VISUAL}`,
    mensagem: 'chore(frontend): migra para Svelte 5 e Vite atual',
  })
  if (!r.feito) {
    log(`migração para Svelte 5 bloqueada: ${r.motivo}; achados de frontend serão pulados`)
    for (const a of achados) if (camadaDe(a.id) === 'frontend') status[a.id] = 'pulado'
  }
}

// ---------------------------------------------------------------- Correção
// Sequential on purpose: every task edits the same working tree and commits on top of the previous one.
phase('Correção')
const detalhes = {}
for (const a of achados) {
  if (status[a.id]) continue
  const camada = camadaDe(a.id)
  const lente = lenteDe(a.id)
  let gate = GATE_BUILD
  if (lente === 'EF') gate += `\n${GATE_BENCH}`
  if (lente === 'ES') gate += `\n${GATE_SEM_REGRESSAO}`
  if (camada === 'frontend') gate += `\n${GATE_VISUAL}`

  const r = await executarTarefa({
    chave: a.id,
    rotulo: a.id,
    faseProgresso: 'Correção',
    pedido:
      `Achado aprovado ${a.id} (${camada}, lente ${lente}): ${a.titulo}\n` +
      `Arquivo: ${a.arquivo}${a.linha ? `:${a.linha}` : ''}\nEvidência: ${a.evidencia}\nProposta: ${a.proposta}\n` +
      `Custo estimado: ${a.custo ?? '?'}. Ganho esperado: ${a.ganho_esperado ?? '?'}.\n` +
      (lente === 'EF' ? 'Antes de mudar, grave a linha de base do benchmark (skill eprocia-bench) num arquivo temporário fora do repositório.\n' : '') +
      (camada === 'frontend' ? 'Siga DESIGN.md (tokens de movimento --dur-fast 150ms, --dur-base 220ms, --ease-out cubic-bezier(.2,.8,.2,1), prefers-reduced-motion; ícones lucide-svelte). Atualize DESIGN.md se a regra visual mudar.\n' : ''),
    gate,
    mensagem: `fix(${camada}): ${a.titulo} [${a.id}]`,
  })
  status[a.id] = r.feito ? 'feito' : 'bloqueado'
  detalhes[a.id] = r
  log(`${a.id}: ${status[a.id]}`)
}

// ---------------------------------------------------------------- Final
phase('Final')
const revisaoFinal = await agent(
  `Revise a branch ${branch} inteira do eprocIA: git log --oneline ${prep.base}..HEAD e git diff ${prep.base}..HEAD. Read-only.\n` +
    `Procure: interação ruim entre correções, regressão, código morto deixado para trás, mensagens fora do pt-BR, arquivos sensíveis commitados (zip, pdf, saídas .txt). ` +
    `Rode go vet ./... e go test ./... -count=1. Devolva achados em texto curto, um por linha, com arquivo:linha; ou "nenhum problema".`,
  { label: 'revisao-final', phase: 'Final' },
)

await agent(
  `Atualize ${relatorio} com o resultado da correção de ${data} e faça um commit só dele.\n` +
    `Status por id (troque a linha "Status: pendente" de cada seção): ${JSON.stringify(status)}.\n` +
    `Detalhes por id (sha, benchstat, capturas, motivo de bloqueio): ${JSON.stringify(detalhes)}.\n` +
    `Em cada achado feito, inclua o sha do commit, a tabela do benchstat (se houver) e links relativos para as capturas antes/depois (se houver). ` +
    `Acrescente uma seção "Revisão final da branch" com:\n${revisaoFinal ?? '(sem resposta)'}\n` +
    `Atualize docs/revisao/estado.json com fase "concluida" e os status acima. ` +
    `Commit (git add ${relatorio} e as capturas em ${config.visual.saida}/${data}/ pelo caminho; nunca git add -A): "docs(revisao): resultado da correção ${data}" com as linhas:\n${ATRIBUICAO}\nNunca faça push.`,
  { label: 'relatorio-final', phase: 'Final', effort: 'low' },
)

return { branch, status, revisaoFinal }
