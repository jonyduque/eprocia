# Harness de revisão do eprocIA — Plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Versionar no repositório o harness do Claude Code (hooks, statusline, agents, skills, scripts, workflows e comandos) que conduz a revisão geral do eprocIA e a correção dos achados aprovados.

**Architecture:** Hooks e statusline são scripts Node sem dependências, com a lógica em funções puras testadas por `node --test`. Os workflows não têm acesso a arquivos, então o comando `/revisar` lê `.claude/review.json` e passa a configuração como `args`; os agentes do workflow fazem toda a leitura e escrita. Os gates (bench, build, visual, ícone) são scripts em `scripts/` descritos pelas skills.

**Tech Stack:** Node 24 (`node:test`), Go 1.27 (`testing.B`, `runtime/metrics`), Playwright via `npx`, ImageMagick 7, Workflow tool do Claude Code.

**Spec:** `docs/superpowers/specs/2026-10-15-harness-revisao-design.md`

## Global Constraints

- Logs, mensagens de bloqueio e textos de interface em pt-BR; documentos e commits em prosa normal.
- Hooks: Node puro, sem `npm install`, orçamento < 1 s, bloqueio = exit 2 com mensagem no stderr.
- Nenhum `*.zip`, `*.pdf` real, saída `.txt` de processamento ou arquivo de `EPROCIA_BENCH_DIR` entra no git.
- Capturas visuais usam só dados fictícios (Vite + stubs), nunca `wails dev`.
- Benchmarks: `-count=10`; aceite com `p < 0,05` e nenhuma outra métrica pior que 5%.
- Ids de achado: `<camada>-<lente>-<nn>` (`B-EF-03`, `F-RS-01`).
- Commit de correção: `fix(<camada>): <título> [<id>]` + linhas de atribuição; nunca push; nunca na `main`.
- Arquivos gerados/embutidos (`frontend/src/wailsjs/**`, `frontend/dist/**`, `internal/ocr/embed/**`, `build/bin/**`) não são editados.
- Montagem do harness não implementa achados (spec §10).

## Review Focus

1. Caminhos Windows com `\` e letra de unidade nos eventos de hook — os hooks devem normalizar antes de casar padrões, ou `proteger-gerados` deixa passar `frontend\src\wailsjs\...`.
2. `git add` com vários caminhos, aspas e `&&` encadeado — `proteger-dados` deve inspecionar cada segmento do comando, não só o primeiro.
3. `docs/revisao/estado.json` ausente ou corrompido — a statusline deve mostrar só a branch, sem quebrar.
4. `EPROCIA_BENCH_DIR` ausente ou vazio — o benchmark real é pulado com `b.Skip`, sem falhar a suíte.
5. Hook recebendo JSON inválido ou sem `tool_input` — sai 0 em silêncio (nunca bloqueia por erro próprio).

Cada item tem teste no task dono (Tasks 1–2 e 4).

---

### Task 1: Hooks com testes e `settings.json`

**Files:**
- Create: `.claude/hooks/lib.mjs` (leitura de stdin, normalização de caminho, helpers de saída)
- Create: `.claude/hooks/{proteger-gerados,proteger-dados,guarda-bench,go-check,front-check,ambiente}.mjs`
- Create: `.claude/hooks/hooks.test.mjs`
- Create: `.claude/settings.json`

**Interfaces:**
- Produces: cada hook exporta uma função pura `avaliar(evento, ctx) -> {codigo: 0|2, stderr?: string, contexto?: string}` e só executa `main()` quando chamado como script (`import.meta.url === pathToFileURL(process.argv[1]).href`).
- `lib.mjs`: `lerEvento(): Promise<object|null>`, `normalizar(p): string` (barras `/`, relativo à raiz do repo, minúsculas para comparação), `responder(resultado, nomeEvento)`.

- [ ] **Step 1: testes** em `hooks.test.mjs`, um `describe` por hook:
  - `proteger-gerados`: bloqueia `frontend/src/wailsjs/go/main/App.js`, `C:\\…\\eprocia\\frontend\\dist\\index.html`, `internal/ocr/embed/x`, `build/bin/eprocia.exe`; deixa `frontend/src/App.svelte`; evento sem `tool_input` sai 0.
  - `proteger-dados`: bloqueia `git add a.zip`, `git add "x/y.PDF"`, `git commit -am x && git add saida.txt`, caminho sob `EPROCIA_BENCH_DIR`; `git add -A` bloqueado quando a lista de não-ignorados (injetada via `ctx.naoIgnorados`) contém `.zip`; deixa `git add README.md` e `git add docs/x.txt` (`.txt` só é bloqueado fora de `docs/`, `scripts/` e `.claude/`, porque as saídas de processamento ficam ao lado dos ZIPs).
  - `guarda-bench`: aviso em `go test -bench .` e `-count=3`; nada com `-count=10` ou sem `-bench`.
  - `go-check`/`front-check`: `avaliar` decide se o arquivo é alvo (`*.go`; `frontend/src/**/*.{svelte,js,ts,css}` fora de `wailsjs`) — a execução de `gofmt`/`svelte-check` é injetada via `ctx.rodar` e testada com stub.
  - `ambiente`: monta a linha a partir de `ctx` (versão do Go, x/tools do wails, PID, bench dir) — compatível quando x/tools ≥ v0.31.
- [ ] **Step 2:** `node --test .claude/hooks/` falha (módulos ausentes).
- [ ] **Step 3:** implementar os hooks e o `settings.json` (hooks + `statusLine`).
- [ ] **Step 4:** `node --test .claude/hooks/` passa; medir tempo de cada hook com evento real (< 1 s).
- [ ] **Step 5:** commit `chore(harness): hooks de proteção e verificação`.

### Task 2: Statusline e `estado.json`

**Files:**
- Create: `.claude/statusline.mjs`, `.claude/statusline.test.mjs`
- Create: `docs/revisao/README.md` (formato do `estado.json`)

**Interfaces:**
- `estado.json`: `{ "relatorio": "docs/revisao/AAAA-MM-DD-relatorio.md", "fase": "lentes|visual|refutacao|sintese|correcao|concluida", "ids": { "B-EF-01": "pendente|feito|bloqueado|pulado" } }`.
- `formatar({branch, estado}) -> string`: `"<branch>"` sem estado; `"<branch> · revisão: <fase> · ✓3 ✗1 …2"` com estado.

- [ ] Testes: sem arquivo, JSON corrompido, contagens corretas. Implementar; commit `chore(harness): statusline`.

### Task 3: `review.json` e agents

**Files:**
- Create: `.claude/review.json` (camadas, arquivos por camada, lentes com id/nome/pergunta, tamanhos de tela, temas, estados, corpus, limites de aceite)
- Create: `.claude/agents/{revisor-go,revisor-frontend,cetico,implementador,revisor-correcao}.md`

- [ ] Agents com frontmatter (`name`, `description`, `tools`); read-only sem `Edit/Write`; `implementador` proibido de tocar gerados. Validar JSON com `node -e`. Commit.

### Task 4: Benchmark (fixtures, gerador, bench, contador de processos)

**Files:**
- Create: `scripts/bench/pdfgen/pdfgen.go` (construtor de PDF com xref, igual ao dos testes)
- Create: `scripts/bench/gerar/main.go` (grava `scripts/bench/fixtures/*`)
- Create: `scripts/bench/bench_test.go` (`BenchmarkSintetico`, `BenchmarkReal`, métricas extras)
- Create: `internal/ocr/metrics.go` + teste (`SpawnedProcesses() int64`; `prepareCmd(cmd)` = `hideConsole` + contador)
- Modify: `internal/ocr/tesseract.go` (3 chamadas `hideConsole` → `prepareCmd`)
- Modify: `go.mod` (`tool golang.org/x/perf/cmd/benchstat`)
- Create: `.claude/skills/eprocia-bench/SKILL.md`

**Interfaces:**
- `pdfgen.Build(objects []string) []byte`; `pdfgen.TextoNativo(n int)`, `Escaneado()`, `Vetorial()`, `ContentsArray()` retornam `[]string` de objetos.
- Bench reporta `b.ReportMetric` para `pico-heap-B`, `procs/op`, `gc-pausa-ns/op`.
- `BenchmarkReal` usa `EPROCIA_BENCH_DIR`; ausente ⇒ `b.Skip`. Variante via `EPROCIA_BENCH_OCR=none|fast` (padrão `none`).

- [ ] Teste de `SpawnedProcesses` (comando `go version`/`cmd /c exit` passado por `prepareCmd` incrementa 1).
- [ ] `go run ./scripts/bench/gerar` gera fixtures determinísticas (rodar 2× ⇒ `git status` limpo).
- [ ] `go test ./scripts/bench -bench . -benchtime=1x -count=1` roda; `BenchmarkReal` pula sem a variável.
- [ ] `go tool benchstat` executa. Commit.

### Task 5: Build (skill + scripts)

**Files:**
- Create: `scripts/build/recompilar-wails.ps1` (Passo 0)
- Create: `scripts/build/gate.ps1` (vet, test -race, wails build, smoke 5 s pelo PID iniciado)
- Create: `.claude/skills/eprocia-build/SKILL.md`

- [ ] Validação: `gate.ps1 -SemBuild` roda vet+test; documentar `-race` exigir CGO/gcc (se ausente, o script avisa e roda sem `-race`). Commit.

### Task 6: Visual (stubs + captura)

**Files:**
- Create: `scripts/visual/estados.mjs` (dados fictícios por estado + `stubScript(estado, tema)`)
- Create: `scripts/visual/capturar.mjs` (sobe Vite, matriz tamanho×tema×estado + divisor min/max, grava PNGs, encerra Vite)
- Create: `scripts/visual/estados.test.mjs`
- Create: `.claude/skills/eprocia-visual/SKILL.md`

- [ ] Teste: cada estado tem itens válidos (campos de `etl.QueueItem`), nenhum nome lembra processo real (padrão `0000000-00.0000.0.00.0000` fictício `5000000-00.2026.0.00.0000`).
- [ ] Validação: `node scripts/visual/capturar.mjs --saida <scratch> --rapido` gera ao menos 1 PNG por estado. Commit.

### Task 7: Ícone

**Files:**
- Create: `scripts/icone/gerar.ps1` (SVG ⇒ ICO 16/24/32/48/64/256 + PNG 1024)
- Create: `.claude/skills/eprocia-icone/SKILL.md`

- [ ] Validação com um SVG temporário no scratchpad (não substitui `build/icon.svg`, que só nasce na tarefa do ícone com o conceito escolhido). `magick identify` lista os 6 tamanhos. Commit.

### Task 8: Workflows e comandos

**Files:**
- Create: `.claude/workflows/eprocia-revisao.js`, `.claude/workflows/eprocia-corrigir.js`
- Create: `.claude/commands/revisar.md`, `.claude/commands/corrigir.md`

**Interfaces:**
- `eprocia-revisao` args: `{ escopo, data: "AAAA-MM-DD", config: <review.json>, lentes?: string[] }` ⇒ retorna `{ relatorio, mantidos, derrubados }`.
- `eprocia-corrigir` args: `{ data, relatorio, ids, config }` ⇒ retorna `{ status: {id: feito|bloqueado|pulado} }`.

- [ ] Sintaxe: `node --check` em cópia `.mjs` de cada workflow. Commit.

### Task 9: Documentação e `.gitignore`

- Modify: `CLAUDE.md` (seção "Harness de revisão", comandos Go 1.27/Wails, hooks, armadilhas)
- Modify: `.gitignore` (`docs/revisao/capturas/**/tmp`, `scripts/bench/*.txt`, `*.prof`)
- [ ] Commit.

### Task 10: Fumaça

- [ ] `node --test .claude/` passa.
- [ ] `/revisar backend` com `lentes: ["TE"]`: valida esquema, refutação e relatório em `docs/revisao/`.
- [ ] Revisão final da branch por agente novo; corrigir o que for apontado.
