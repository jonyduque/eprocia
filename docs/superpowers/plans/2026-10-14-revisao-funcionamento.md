# Revisão de Funcionamento — Plano de Implementação

> **Para agentes:** executar tarefa a tarefa (TDD: teste que falha → implementação mínima → teste verde → commit). Passos usam `- [ ]`.

**Goal:** Corrigir os 19 defeitos de funcionamento levantados em `revisao-funcionamento-eprocia.md` (fases 1–3).

**Architecture:** Mudanças cirúrgicas nos pacotes existentes (`etl`, `ocr`, `watchdog`, `app.go`, `main.go`, frontend). Dois pacotes novos pequenos: `internal/autostart` e helper `hideWindow` no `ocr`.

**Tech Stack:** Go 1.2x, Wails v2.12, `golang.org/x/sys/windows/registry`, Svelte 4.

**Branch:** `fix/revisao-funcionamento`. Commit-base com o trabalho já validado (concorrência/UUID/overlay) antes das fases.

**Decisões:**
- `CleanText("aggressive")` continua juntando linhas em uma só (design de economia de tokens, coberto por teste existente); corrige-se apenas a **perda de dados**: só remove linha inteira de rodapé (`Página N de M`) ou que *comece* com "Documento assinado"/"Assinatura eletrônica".
- Filtro de ZIP do Eproc fica em `etl.HasProcessableFiles`, chamado no callback do watcher em `app.go` (o pacote `watchdog` permanece genérico e seus testes seguem válidos). Entrada manual (drag/botão) **não** é filtrada: erro visível é desejável.
- Auto-start via `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` com flag `--minimized` (inicia oculto na bandeja).

---

## Fase 1 — Corretude do resultado

### Task 1: Ordenação por número do documento + Capa primeiro
**Files:** Modify `internal/models/models.go` (+`DocNum int`), `internal/etl/sorter.go`; Test `internal/etl/etl_test.go`.
- [ ] Teste: `Evento 1 - COMP3`, `INIC1`, `PROC2` → ordem `INIC1, PROC2, COMP3`; `Capa do processo.pdf` primeiro; arquivo sem padrão por último; `DocNum` extraído de `Evento 12 - PROC_VIRTUAL_3.pdf` = 3.
- [ ] Rodar: `go test ./internal/etl -run 'TestSort|TestParseFileInfo'` → FAIL.
- [ ] Implementar: `docRegex = (\d+)\.[A-Za-z0-9]+$` aplicado ao resto do nome após `Evento N`; `Capa do processo.pdf` → `EventNum=0, DocNum=0`; sem número → `DocNum=math.MaxInt32`. `SortProcessFiles`: Evento → DocNum → `CompareNatural`.
- [ ] Rodar → PASS. Commit `fix(etl): ordena documentos pelo número do documento e coloca a capa primeiro`.

### Task 2: Limpeza agressiva sem perda de conteúdo
**Files:** Modify `internal/etl/formatter.go`; Test `internal/etl/formatter_test.go`.
- [ ] Teste: linha `conforme a página inicial dos autos` **preservada** no modo agressivo; `Página 12 de 35`, `Pág. 3/10`, `Documento assinado eletronicamente por X` removidas; linha de texto que apenas *menciona* "assinatura eletrônica" preservada.
- [ ] Implementar regexes de linha inteira (`^(?i)p[áa]g(ina|\.)?\s*\d+(\s*(de|/)\s*\d+)?$`) e `HasPrefix` para assinaturas.
- [ ] PASS. Commit `fix(etl): limpeza agressiva remove apenas rodapés e assinaturas de linha inteira`.

### Task 3: Escape XML de atributos
**Files:** Modify `internal/etl/formatter.go`; Test `formatter_test.go`.
- [ ] Teste: `nome="A & B \"x\".pdf"` → `A &amp; B &quot;x&quot;.pdf`.
- [ ] Implementar `escapeAttr` (`& < > "`) usado em `<documento nome=...>`. Commit.

---

## Fase 2 — Estabilidade

### Task 4: Ocultar janelas de console dos subprocessos
**Files:** Create `internal/ocr/proc_windows.go` (`//go:build windows`), `internal/ocr/proc_other.go` (`//go:build !windows`); Modify `internal/ocr/tesseract.go`.
- [ ] `hideWindow(cmd)` define `SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}`; aplicar às 3 invocações. Teste em `proc_windows_test.go` verifica os flags. Commit.

### Task 5: Recuperação de panics
**Files:** Modify `internal/etl/pdf_parser.go`, `internal/etl/pipeline.go`; Test `pdf_parser_test.go`.
- [ ] Teste: `ParsePages` com `ocrRunner` que dá `panic` → retorna página com fallback, sem derrubar o processo.
- [ ] Implementar `safeNativeText`/`safeHasImages` (recover por página), `recover` em `runOCRTask` e na goroutine de documento (vira página de erro `[ERRO ... panic]`).
- [ ] Commit.

### Task 6: Contadores consistentes com documento em erro
**Files:** Modify `internal/etl/pipeline.go`.
- [ ] `defer` único que incrementa `processedDocs` e reporta (exceto cancelamento); no erro de parse, somar páginas restantes (`docPageCounts - concluídas`). Commit.

### Task 7: Instância única
**Files:** Modify `main.go`.
- [ ] `SingleInstanceLock{UniqueId: "eprocia-8f3c1a52", OnSecondInstanceLaunch: → app.Show()}`. Commit.

### Task 8: `InitBinaries` com retry + locks na fila
**Files:** Modify `internal/ocr/tesseract.go`, `internal/etl/queue.go`; Test `tesseract_test.go`.
- [ ] Trocar `sync.Once` por mutex + flag de sucesso (erro **não** é cacheado). `broadcast` lê `isPaused/isWorking/wailsCtx` sob `RLock`; `SetContext` sob `Lock`.
- [ ] Validar com `go test -race ./internal/...`. Commit.

---

## Fase 3 — Fluxo e UX

### Task 9: Filtrar ZIPs que não são do Eproc
**Files:** Modify `internal/etl/zip_reader.go`, `app.go`; Test `etl_test.go`.
- [ ] Teste `HasProcessableFiles`: ZIP com PDF → true; só `.txt` → false; arquivo inválido → false.
- [ ] Callback do watcher só enfileira se `HasProcessableFiles`. Commit.

### Task 10: Deduplicação no watcher
**Files:** Modify `internal/watchdog/watchdog.go`; Test `watchdog_test.go`.
- [ ] Teste: vários `Write` no mesmo arquivo → callback **1×**. Reescrita com tamanho/mtime diferentes → notifica de novo.
- [ ] Implementar `inFlight` (1 goroutine por caminho) e `lastNotified[path]` por fingerprint (size+mtime). Commit.

### Task 11: Pré-scan com progresso, cancelável e sem cópia em disco
**Files:** Modify `internal/etl/pipeline.go`; Test `etl_test.go`.
- [ ] Contar páginas lendo a entrada para memória (limite 200 MB, senão estima 1) em vez de copiar todos os PDFs para `%TEMP%`; checar `ctx` e emitir `Analisando documentos (i/N)`. Remove `docTempFiles`/safety net (cada documento extrai o próprio temporário já existente).
- [ ] Teste: `ProcessZip` com contexto cancelado antes do pré-scan retorna `context.Canceled` rápido. Commit.

### Task 12: Throttle de progresso
**Files:** Modify `internal/etl/pipeline.go`, `internal/etl/queue.go`.
- [ ] `reportProgress` emite no máximo a cada 150 ms (CAS em `lastEmit`); `force` no fim de cada documento e no 100%. Remover `log.Printf` por callback em `queue.onProgress`. Commit.

### Task 13: Estado real do watchdog na UI
**Files:** Modify `app.go`, `frontend/src/App.svelte`; regenerar bindings (`wails generate module`).
- [ ] `App.GetWatcherStatus()` + evento `watcher:status {active,path,error}` emitido em `startup`/`SaveConfig`; rodapé mostra "Monitorando: …" ou o erro. Commit.

### Task 14: Auto-start do Windows
**Files:** Create `internal/autostart/autostart_windows.go`, `autostart_other.go`, `autostart_test.go`; Modify `app.go`, `main.go` (`StartHidden` com `--minimized`).
- [ ] Teste com chave temporária `HKCU\Software\eprocIA-test`. `SaveConfig` e `startup` sincronizam o valor. Commit.

---

## Verificação final
- [ ] `go build ./...`, `go vet ./...`, `go test ./...`, `go test -race ./internal/...`
- [ ] `cd frontend && npm run build`; `wails build` (smoke).
- [ ] Atualizar `CLAUDE.md`. Commit e merge/PR a critério do usuário.
