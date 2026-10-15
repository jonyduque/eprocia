# Harness de revisão do eprocIA — Design

Data: 2026-10-15
Status: aprovado em conversa (seções 1–5), aguardando revisão do documento

## 1. Objetivo

Montar, versionado dentro do repositório, um harness do Claude Code (instruções, skills, agents, hooks, workflows e comandos) que conduza uma revisão geral do eprocIA e, depois, a correção dos achados que o usuário aprovar.

Prioridades da revisão, em ordem:

1. **Eficiência** — processar ZIPs consumindo menos CPU, memória, I/O e processos filhos, e/ou em menos tempo.
2. **Estabilidade** — eliminar travamentos, vazamentos, erros engolidos e pânicos.
3. **Modernização** — aproveitar recursos do Go 1.26/1.27 quando trouxerem ganho real ou reduzirem a chance de erro.
4. **Recursos novos** — oportunidades baratas e coerentes com o PRD.
5. **Frontend** — evoluir o visual seguindo o DESIGN.md (sobriedade, densidade), suavizar animações, trocar ícones por um conjunto coerente, criar ícone próprio do aplicativo e corrigir o comportamento ao redimensionar a janela e o divisor interno.

### Critérios de sucesso

- `/revisar` produz um relatório priorizado, em que cada achado tem id estável, evidência verificável, proposta, custo e ganho esperado.
- `/corrigir <ids>` implementa apenas os achados aprovados, um commit por achado, cada um passando pelos gates de build, benchmark e/ou visual.
- Nenhuma melhoria de eficiência é aceita sem medição estatisticamente significativa.
- Nenhum dado pessoal (ZIPs, PDFs ou saídas reais) entra no git.

### Decisões tomadas com o usuário

| Tema | Decisão |
|---|---|
| Modo de operação | Achados → usuário aprova → correção |
| Direção visual | Evoluir o DESIGN.md (sóbrio e denso, modernizado) |
| Corpus de benchmark | ZIPs reais locais (`EPROCIA_BENCH_DIR`, nunca versionados) + fixtures sintéticas versionadas |
| Toolchain | Liberar Go 1.27: recompilar o Wails CLI com `x/tools` novo e subir o `go.mod` |
| Stack do frontend | Migração obrigatória para Svelte 5 + Vite atual, com `lucide-svelte` para ícones |
| Arquitetura do harness | Autossuficiente no repositório; não depende de skills ou workflows de `~/.claude` |

## 2. Contexto

- Backend Go com ~2,5 mil linhas de produção (`app.go`, `main.go`, `internal/{etl,ocr,config,watchdog,models}`) e ~1,1 mil de testes.
- Frontend Svelte 4 com ~1,9 mil linhas (`App.svelte`, `components/ProcessList.svelte`, `components/Settings.svelte`, `assets/css/style.css`).
- Janela `Frameless` com barra de título própria, divisor interno redimensionável e `HideWindowOnClose` com ícone na bandeja.
- Armadilhas conhecidas, que a revisão de 2026-10-14 corrigiu e que o harness deve documentar:
  - `github.com/dslipak/pdf` entra em loop infinito quando o content stream não é um stream único legível (`/Contents` em array, ausente ou corrompido) e dá `panic` em filtros não suportados.
  - O `getlantern/systray` no Windows exige dados ICO; um PNG falha com `Unable to set icon`.
  - Processos de console (`tesseract.exe`, `pdftoppm.exe`) abertos por um app GUI precisam de `CREATE_NO_WINDOW`, ou cada página faz piscar uma janela.
  - Páginas com texto desenhado como vetor (iLovePDF) não têm texto nativo nem imagens grandes e precisam ir para o OCR.
  - O Wails CLI v2.12.0 compilado com `x/tools v0.30.0` falha com Go 1.27 (`package "bytes" without types was imported`).
- Ferramentas na máquina: Go 1.27.1, Node 24, ImageMagick 7, Python 3.13, gh, Chrome. Inkscape e rsvg-convert não estão instalados; o SVG é rasterizado pelo próprio ImageMagick.

## 3. Estrutura

```
CLAUDE.md                         seção "Harness de revisão" + comandos atualizados
.claude/
  settings.json                   hooks e permissões (versionado)
  settings.local.json             continua local
  review.json                     partições, lentes, limites, tamanhos de tela, corpus
  hooks/
    proteger-gerados.mjs
    proteger-dados.mjs
    guarda-bench.mjs
    go-check.mjs
    front-check.mjs
    ambiente.mjs
  workflows/
    eprocia-revisao.js
    eprocia-corrigir.js
  agents/
    revisor-go.md
    revisor-frontend.md
    cetico.md
    implementador.md
    revisor-correcao.md
  skills/
    eprocia-build/SKILL.md
    eprocia-bench/SKILL.md
    eprocia-visual/SKILL.md
    eprocia-icone/SKILL.md
  commands/
    revisar.md
    corrigir.md
  statusline.mjs                  branch, fase da revisão em curso, ids pendentes/feitos
scripts/
  bench/                          gerador de fixtures sintéticas + benchmarks Go
  visual/                         script Playwright de screenshots (executado via npx)
  icone/                          SVG mestre -> .ico/.png (ImageMagick)
docs/revisao/
  AAAA-MM-DD-relatorio.md         saída da revisão, atualizada pela correção
```

### 3.1 Responsabilidades

- **Workflows** orquestram as fases de forma determinística.
- **Agents** fixam papel, ferramentas e restrições:
  - `revisor-go` e `revisor-frontend` são read-only;
  - `cetico` é read-only e pode rodar testes e comandos de medição;
  - `implementador` edita, mas não toca arquivos gerados ou embutidos;
  - `revisor-correcao` é read-only.
- **Skills** guardam o conhecimento operacional de cada tarefa recorrente (build, benchmark, captura visual, ícone).
- **Commands** são a interface com o usuário: `/revisar [backend|frontend|tudo]` e `/corrigir <ids>`.
- **Statusline** (`.claude/statusline.mjs`, configurada em `settings.json`) mostra a branch e, quando houver relatório, quantos ids estão `feito`, `bloqueado` e pendentes. O estado vem de `docs/revisao/estado.json`, que os workflows atualizam ao fim de cada fase.

## 4. Hooks

Scripts em Node (já obrigatório por causa do frontend), cada um com orçamento de menos de 1 s. Recebem o JSON do evento via stdin. Bloqueios saem com código 2 e mensagem em pt-BR no stderr.

| Hook | Evento / matcher | Comportamento |
|---|---|---|
| `proteger-gerados` | PreToolUse `Edit\|Write\|MultiEdit\|NotebookEdit` | Bloqueia caminhos em `frontend/src/wailsjs/**`, `frontend/dist/**`, `internal/ocr/embed/**` e `build/bin/**`. |
| `proteger-dados` | PreToolUse `Bash` | Bloqueia `git add`/`git commit` que inclua `*.zip`, `*.pdf`, saídas `.txt` de processamento ou qualquer caminho dentro de `EPROCIA_BENCH_DIR`. Também bloqueia `git add -A`/`git add .` quando houver arquivos desses tipos não ignorados. |
| `guarda-bench` | PreToolUse `Bash` | Não bloqueia; devolve aviso quando `go test -bench` roda com `-count` ausente ou menor que 6. |
| `go-check` | PostToolUse `Edit\|Write\|MultiEdit` em `*.go` | Roda `gofmt -l` no arquivo editado e `go vet` no pacote dele, e devolve a saída como contexto para o Claude. Não reformata arquivos que não foram tocados. |
| `front-check` | PostToolUse `Edit\|Write\|MultiEdit` em `frontend/src/**/*.{svelte,js,ts,css}` | Roda `svelte-check` com saída limitada ao arquivo. Antes da migração para Svelte 5 (sem `svelte-check` instalado), apenas informa que está inativo. |
| `ambiente` | SessionStart | Emite uma linha: versão do Go, compatibilidade do Wails CLI (versão de `x/tools` embutida via `go version -m`), `eprocia.exe` em execução (PID) e se `EPROCIA_BENCH_DIR` está definido. |

Testes completos, benchmarks e build ficam fora dos hooks, porque são caros demais para rodar a cada edição; são gates dos workflows (seção 6). Não há hook de Stop.

## 5. Revisão — `eprocia-revisao.js`

Entrada: `args = { escopo: "backend" | "frontend" | "tudo" }`. Arquivos, lentes e parâmetros vêm de `.claude/review.json`.

### 5.1 Fase 1 — Lentes

Um agente read-only por lente, em paralelo, cobrindo a camada inteira. O código cabe em contexto, então não há partição lente × arquivo. Cada agente responde a uma única pergunta e devolve achados num esquema fixo.

**Backend** (`revisor-go`):

| id | lente | pergunta |
|---|---|---|
| EF | eficiência | Onde se gasta CPU, RAM, I/O ou processos sem necessidade? (processo por página no OCR, PDFs temporários, alocações da biblioteca de PDF, leitura duplicada do ZIP, eventos de progresso em excesso) |
| ES | estabilidade | Onde pode travar, vazar recurso, engolir erro ou entrar em pânico numa goroutine sem `recover`? |
| GM | go-moderno | O que o Go 1.26/1.27 resolve melhor ou com menos chance de erro? (perfil `goroutineleak`, modernizadores do `go fix`, `sync.WaitGroup.Go`, iteradores, `new(expr)`) |
| TE | testes | Que teste não consegue falhar? Que caminho de falha nunca é exercitado? |
| RN | recursos | Que recurso novo cabe no PRD com custo baixo? |

**Frontend** (`revisor-frontend`):

| id | lente | pergunta |
|---|---|---|
| VI | visual | Onde o visual se afasta do DESIGN.md ou parece datado? (ícones, tipografia, hierarquia, estados) |
| AN | animação | Que transição é brusca, longa demais ou ignora `prefers-reduced-motion`? |
| RS | layout/resize | O que quebra ao redimensionar a janela ou o divisor interno? (limites mínimo e máximo, overflow, bordas de resize da janela frameless, persistência do tamanho do painel) |
| AC | acessibilidade | Foco visível, navegação por teclado, contraste, `aria-*` |
| DF | desempenho-front | Re-renderizações desnecessárias, listeners que vazam, eventos Wails não desinscritos, trabalho síncrono pesado na UI |

Esquema de cada achado:

```json
{
  "lente": "EF",
  "titulo": "string curta",
  "arquivo": "caminho/relativo",
  "linha": 123,
  "gravidade": "alta | media | baixa",
  "evidencia": "o que foi observado, com trecho, comando ou medição",
  "proposta": "o que mudar",
  "custo": "P | M | G",
  "ganho_esperado": "string (para EF: métrica e ordem de grandeza)"
}
```

### 5.2 Fase 2 — Captura visual (somente nos escopos frontend e tudo)

Um agente usa a skill `eprocia-visual`:

1. Sobe só o frontend (`npm run dev`, Vite), sem o backend Go. Assim nenhum dado real aparece nas capturas.
2. Roda `scripts/visual/capturar.mjs` via `npx playwright` (efêmero, sem entrar no `package.json`). Antes da carga, um `addInitScript` injeta stubs de `window.go.main.App` e `window.runtime` com dados fictícios por estado (`scripts/visual/estados.mjs`). As capturas cobrem:
   - os tamanhos 800×600, 1024×768, 1440×900 e 1920×1080;
   - os temas claro e escuro;
   - os estados vazio, fila com itens, processamento, erro e configurações abertas;
   - o divisor interno arrastado até o mínimo e o máximo.
3. Grava em `docs/revisao/capturas/AAAA-MM-DD/antes/` e passa os caminhos para as lentes RS e VI como evidência complementar.
4. Encerra o servidor do Vite.

Limitação aceita: o Playwright renderiza o frontend no Chromium, não no WebView2 da janela frameless. Bordas de resize da janela nativa são verificadas pelo código (opções do Wails e CSS de `--wails-draggable`) e por um teste manual guiado descrito no relatório.

### 5.3 Fase 3 — Refutação

Um `cetico` por lente recebe todos os achados daquela lente e tenta derrubar cada um: lê o código, roda o teste ou mede. Um achado sobrevive somente com evidência concreta (`arquivo:linha` conferido, teste que reproduz, medição ou captura). O cético devolve `mantido | derrubado | rebaixado`, com justificativa.

### 5.4 Fase 4 — Síntese

Um agente final:

- remove duplicatas entre lentes;
- ordena por `gravidade × ganho ÷ custo`;
- atribui ids estáveis `<camada>-<lente>-<nn>` (ex.: `B-EF-03`, `F-RS-01`);
- escreve `docs/revisao/AAAA-MM-DD-relatorio.md`, com uma tabela-resumo e uma seção por achado (evidência, proposta, custo, ganho, veredito do cético) e uma lista "derrubados" para auditoria.

### 5.5 Custo

| Escopo | Agentes aproximados |
|---|---|
| backend | 11 (5 lentes + 5 céticos + 1 síntese) |
| frontend | 12 (5 lentes + 1 visual + 5 céticos + 1 síntese) |
| tudo | 22 |

Acima da diretriz padrão de 10 agentes, por pedido explícito de revisão geral. Rodadas seguintes podem usar um escopo só.

## 6. Gates

### 6.1 Benchmark — skill `eprocia-bench`

**Fixtures sintéticas**, geradas por `scripts/bench/gerar` (Go) e versionadas em `scripts/bench/fixtures/`. Os PDFs são montados com o mesmo construtor de xref usado nos testes:

- PDF com texto nativo;
- PDF escaneado (imagem grande + página sem texto);
- PDF de texto vetorial;
- PDF com `/Contents` em array;
- HTML;
- ZIP misto com cerca de 50 documentos, com nomes no padrão `Evento N - TIPO<doc>.pdf`.

**Corpus real**: todos os `*.zip` em `EPROCIA_BENCH_DIR`. Se a variável estiver ausente, essa parte é pulada sem erro. Nada desse diretório é copiado para dentro do repositório.

**Benchmarks**: `go test ./scripts/bench -bench . -count=10 -benchmem`, em duas variantes:

- `OCR=none`, que mede o custo do Go;
- `OCR=fast`, que mede o pipeline completo.

Métricas coletadas:

- `ns/op`, `B/op` e `allocs/op` (padrão);
- pico de RSS, pela métrica `/memory/classes/total:bytes` amostrada;
- número de processos filhos criados (contador no runner de OCR, exposto para teste);
- pausa total do GC.

**Comparação**: `benchstat` declarado como ferramenta do módulo (`go get -tool golang.org/x/perf/cmd/benchstat`) e executado com `go tool benchstat antes.txt depois.txt`.

**Aceite de um achado de eficiência**:

- melhora com `p < 0,05` na métrica-alvo;
- nenhuma outra métrica coletada piora mais que 5%.

Achados de estabilidade exigem apenas ausência de regressão acima de 5%.

### 6.2 Build — skill `eprocia-build`

**Passo 0** (pré-requisito único do fluxo de correção):

1. Recompilar o Wails CLI v2.12.0 com `golang.org/x/tools` atual: o módulo sai do cache para um diretório temporário, recebe `go get golang.org/x/tools@latest` e é instalado com `go install ./cmd/wails`.
2. Validar com `wails build` sob Go 1.27.1.
3. Subir o `go.mod` para `go 1.27` e rodar `go mod tidy`.

Se qualquer etapa falhar, o fluxo para antes de começar qualquer correção.

**Gate** (cada tarefa de correção):

- `go vet ./...`
- `go test ./... -race -count=1`
- `wails build` sem erro
- smoke test: o exe sobe, responde por 5 s e é encerrado pelo PID que o próprio script iniciou

### 6.3 Visual — skill `eprocia-visual`

- Captura "depois" com a mesma matriz da Fase 2, em `docs/revisao/capturas/AAAA-MM-DD/depois/`.
- `npx svelte-check` sem erros.
- `npm run build` sem warnings novos em relação à linha de base.
- O relatório recebe os pares antes/depois de cada achado visual. A aprovação é do usuário, olhando as imagens.

### 6.4 Ícone — skill `eprocia-icone`

- SVG mestre em `build/icon.svg`.
- `scripts/icone/gerar.ps1` usa o ImageMagick para gerar:
  - `build/windows/icon.ico` com 16, 24, 32, 48, 64 e 256 px;
  - `build/appicon.png` com 1024 px.
- O `icon.go` continua embutindo `build/windows/icon.ico`, então bandeja e executável usam a mesma arte.
- O conceito do ícone é escolhido pelo usuário entre 3 ou 4 propostas apresentadas no momento da tarefa.

## 7. Correção — `eprocia-corrigir.js`

Entrada: `args = { relatorio: "docs/revisao/AAAA-MM-DD-relatorio.md", ids: ["B-ES-01", ...] }`.

### 7.1 Preparação

- Cria a branch `revisao/AAAA-MM-DD` a partir do HEAD atual, ou reutiliza a branch se já existir. Nunca trabalha na `main` e nunca faz push.
- Pré-requisitos, uma vez, cada um com commit próprio e passando pelos gates:
  1. Passo 0 do toolchain (6.2).
  2. Migração para Svelte 5 + Vite atual + `lucide-svelte` + `svelte-check`, quando algum id for de frontend. Comportamento idêntico, verificado pelas capturas "antes" contra "depois da migração".

### 7.2 Ordem das tarefas

Estabilidade (ES), depois eficiência (EF), depois go-moderno e testes (GM, TE), depois frontend (RS, DF, AC, AN, VI), depois recursos (RN). Dentro de cada grupo, a ordem do relatório.

### 7.3 Loop por tarefa

1. **`implementador`**, um agente novo, sem contexto da conversa. Recebe só o achado e o trecho relevante.
   - Escreve um teste que falha ou, para EF, um benchmark de linha de base.
   - Implementa a correção.
   - Só pode editar os arquivos citados pelo achado e seus testes; ampliar o escopo exige justificativa no retorno.
2. **Gate** conforme o tipo: build (todos), benchmark (EF, e ES sem regressão), visual (frontend).
3. **`revisor-correcao`**, um agente novo, verifica:
   - aderência ao achado;
   - qualidade (escopo, padrão do repositório, logs e mensagens em pt-BR, sem comentários excessivos).
4. Em reprovação, devolve ao implementador com o motivo, até 3 tentativas. Na terceira falha, descarta o diff da tarefa e marca o achado como `bloqueado`.
5. Em aprovação, faz um commit por achado, com mensagem `fix(<camada>): <título> [<id>]` mais as linhas de atribuição.

### 7.4 Final

- Revisão da branch inteira por um agente novo.
- Atualiza o relatório com status por id (`feito | bloqueado | pulado`), medições benchstat e links das capturas.
- Para e apresenta o resultado; merge e push ficam com o usuário.

## 8. Documentação

- **CLAUDE.md**:
  - nova seção "Harness de revisão", com comandos, gates, toolchain e as armadilhas da seção 2;
  - seção "Commands" atualizada para Go 1.27 e o Wails CLI recompilado;
  - menção aos hooks e ao que eles bloqueiam.
- **DESIGN.md**, atualizado pela revisão de frontend:
  - ícones via `lucide-svelte` (substitui a menção a Lucide React);
  - tokens de movimento: `--dur-fast: 150ms`, `--dur-base: 220ms`, `--ease-out: cubic-bezier(.2,.8,.2,1)`, e o comportamento sob `prefers-reduced-motion`;
  - regras de layout responsivo e de limites do divisor;
  - correção da menção a "Wails v3" (o projeto usa Wails v2).
- **.gitignore**: `docs/revisao/capturas/**/tmp`, arquivos de benchmark temporários e qualquer saída real.

## 9. Testes do próprio harness

- **Hooks**: cada script tem um teste em `node --test` (`.claude/hooks/*.test.mjs`) cobrindo casos de bloqueio e de passagem com JSON de evento simulado.
- **Workflows**: uma execução de fumaça de `/revisar backend` com a lente `TE` apenas (args `lentes: ["TE"]`) para validar esquema, refutação e escrita do relatório antes da rodada completa.
- **Skills de script** (bench, visual, ícone): cada script tem uma execução de validação documentada na skill.

## 10. Fora de escopo

- Implementar os achados durante a montagem do harness; isso acontece só via `/corrigir`, depois da aprovação.
- Instalar dependências globais além da recompilação do Wails CLI.
- CI remoto (GitHub Actions); o harness roda localmente.
- Trocar a biblioteca de PDF; se a revisão recomendar, vira achado com custo G.
