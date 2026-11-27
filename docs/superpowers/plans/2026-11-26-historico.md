# Histórico da fila ao reabrir: plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** com o interruptor "Guardar a lista da última sessão" ligado, a fila da última sessão com processamento volta ao reabrir o eprocIA, com data e hora na linha, o estado "Interrompido" e as ações de antes.

**Architecture:** pacote folha novo `internal/historico` (formato, validação, `Carregar` e o `Gravador` com debounce e escrita atômica), que não importa `etl`. A conversão para `QueueItem`, a regra da lista gravada (`sessaoComEntrada`), o estado `interrupted` e a reutilização da linha interrompida ficam em `internal/etl` (`queue.go` e `queue_historico.go`). `app_historico.go` liga tudo: carrega antes do watcher, adia o gravador no processo novo da atualização, entrega o gravador ao `atualizacao.Ambiente` e trata o interruptor.

**Tech Stack:** Go 1.27 (Wails v2), Svelte 5 em sintaxe legada, `node --test`.

**Spec:** `docs/superpowers/specs/2026-11-18-historico-design.md` (comportamento). Consome o que a spec `docs/superpowers/specs/2026-11-18-pdf-avulso-design.md` entrega (pacote `internal/saida`, `QueueItem.Kind`/`Paths`/`saidaAnterior`, `Entrada.SaidaAnterior`), executada antes, pelo plano `docs/superpowers/plans/2026-11-26-pdf-avulso.md`.

**Regras comuns dos implementadores:** `docs/superpowers/plans/2026-11-26-pdf-historico-regras.md` (worktree, TDD, gate, commit, relatório). Cada tarefa as segue.

## Global Constraints

- O histórico nasce desligado: `Config.GuardarHistorico bool` (`json:"guardar_historico"`), padrão `false` em `DefaultConfig` e no overlay de `LoadFromFile`. Desligado, nenhuma leitura nem escrita de `historico.json`, e `Restaurar` não é chamado.
- Arquivo: `historico.json` na pasta de dados (`App.pastaDados`, `%APPDATA%\eprocIA`, a mesma de `config.json` e `falhas.json`). UTF-8 sem BOM. Formato `{"versao": 1, "itens": [...]}`, itens do mais novo para o mais antigo.
- Campos gravados por item, com estas chaves exatas: `name`, `path`, `status`, `message`, `output_path`, `docs_count`, `pages_count`, `estimated_tokens`, `unread_docs`, `error`, `timestamp`, `finished_at`, `ocr_mode`, `cleaning_level`, e, no item de PDF, `kind` e `paths`; e `saida_anterior` (omitido quando vazio), acrescentado na revisão final. Não se grava `id`, `restaurado`, progresso, contadores parciais, atividade, ajuste de reprocessamento nem `forcar_ocr`. Grava-se `relatorio` sem `atencao` (omitido quando o item não tem relatório): decisão d3 do dono (2026-11-26), Task 10; a cópia gravada perde só a lista `atencao`, que traz os nomes dos documentos.
- `kind` é sempre gravado, inclusive `"zip"` (sem `omitempty`). A carga só deriva `kind` da extensão do `path` quando o campo vem vazio (arquivo escrito à mão ou de versão anterior do formato).
- Teto de 500 itens (os mais antigos saem). Debounce de 1 s. Nova tentativa de rename uma vez após 200 ms.
- Arquivo corrompido vira `historico.json.corrompido-AAAAMMDD-HHMMSS` (hora local) e a lista abre vazia; `versao` maior que 1 é ignorada e o arquivo não é sobrescrito até a sessão ter um item.
- Status novo `"interrupted"`. Rótulo "Interrompido", ação "Processar de novo".
- Textos exatos da interface: "Guardar a lista da última sessão"; "Ao reabrir, a lista mostra os processos da última sessão. Os nomes dos arquivos ficam guardados neste computador até a sessão seguinte em que algo for processado."; "TXT não encontrado"; "Detalhes por documento só ficam disponíveis durante a sessão em que o item foi processado"; "Não foi possível guardar o histórico"; título do botão "Remover da lista os concluídos, com erro, cancelados e interrompidos".
- Erros do Go com texto exato: "arquivo de origem não encontrado: <nome>" (`Reprocessar`), "O TXT não está mais na pasta; ele pode ter sido movido ou apagado" (`CopyOutput`, sem caminho desde a decisão d9 de 2026-11-26, Task 11; o caminho vai ao log).
- Data na linha: "14:32" hoje, "ontem 14:32", "12/11 14:32" em outro dia do mesmo ano, "12/11/2025 14:32" em outro ano; usa `finished_at` quando houver, senão `timestamp`.
- Linhas de log e mensagens em pt-BR. O pacote `historico` não importa `etl` nem `net`.
- Nenhum teste grava em `%APPDATA%` real: o caminho do histórico é parâmetro e os testes usam `t.TempDir()` (e `t.Setenv("APPDATA", t.TempDir())` onde `SaveConfig` roda).

## Review Focus

1. Caixa diferente entre o caminho guardado e o que o watcher entrega (`c:\users\fulano\downloads\x.zip` contra `C:\Users\fulano\Downloads\x.zip`): a varredura deve reaproveitar a linha interrompida, não criar outra. Teste na Task 4.
2. O mesmo ZIP duas vezes no histórico (uma linha `success` de manhã e uma `interrupted` à tarde): a varredura reaproveita só a interrompida e a concluída fica como está. Teste na Task 4.
3. `historico.json` vazio (0 bytes) ou salvo com BOM pelo Bloco de Notas: o vazio vira `.corrompido-*` e a lista abre vazia; o com BOM é lido normalmente. Teste na Task 1.
4. Primeira gravação numa máquina em que `%APPDATA%\eprocIA` ainda não existe (o usuário nunca salvou a config): o gravador cria a pasta. Teste na Task 1.
5. "Limpar tudo" logo ao abrir, antes de processar qualquer coisa (sessão sem entrada): o arquivo fica com a lista vazia e a sessão anterior não volta na abertura seguinte. Teste na Task 3.
6. Troca de versão de ponta a ponta pelo `Atualizador`: no `Reiniciar`, o `historico.json` já tem a lista; se a troca falhar e for revertida, o que o usuário mexer na espera (remover, limpar) volta a ser gravado; com sucesso, o `shutdown` do processo antigo não regrava. Teste na Task 5.

## Divergências da spec com o código atual

- **Linhas citadas.** A spec cita a `main` da v0.4.2. Na v0.5.0: `Shutdown`/`shutdownComPrazo` em `queue.go:141-164`, `AddFilesComOrigem` em `:245-288` (o `AddFiles` da spec virou wrapper de `AddFilesComOrigem`, que leva a origem para log e telemetria), `copiarItem` em `:306-331`, `Reprocessar` em `:337-391`, `CancelItem` em `:501-526`, `LimparConcluidos` em `:567-582`, `processItem` em `:626-792` (panic em `:633-652`, falha de config em `:700-723`, cancelamento em `:755-759`), `broadcast` em `:796-819` (retorno por `wailsCtx == nil` em `:802`). `OpenDirectory` em `app.go:393-401`, `lerSaidaParaCopiar` em `:420-435`, `ProcessList.svelte` "Limpar concluídos" em `:558-561`, data da linha em `:659`, `CONCLUIDO` em `fila.js:130`. O plano do PDF avulso, executado antes, desloca essas linhas: localize pelo nome da função.
- **Atualização pelo programa (v0.5.0).** O `internal/atualizacao` já tem a interface `Gravador { Fechar() error; Reabrir() error }` em `Ambiente.Gravador` (`atualizador.go:22-38`) e já a chama no passo 4 de `preparar` (`:604-627`) e na reversão (`:670-674`), mas `app_atualizacao.go:173-186` não passa nenhum gravador. O plano cria o adaptador (Task 5). O `Fechar(itensFinais)` do histórico tem outra assinatura, por isso o adaptador chama `Queue.FecharHistorico()`.
- **Processo novo da atualização.** A spec do autoupdate (seção 3.1) diz que o processo com `--apos-atualizar` "carrega config e histórico" no startup e "liga o gravador do histórico" só depois que o antigo sai (`arranqueApos`, `app_atualizacao.go:228-244`). O plano segue isso: `Restaurar` no startup, `SetGravador` em `arranqueApos` depois de `esperarAntigo` e antes de `Soltar`, seguido de um `AgendarHistorico` para não perder uma remoção feita na espera.
- **Segundo `Fechar` (Shutdown depois do fechamento da troca).** A spec não diz o que um `Fechar` num gravador já fechado faz. O plano decide que não grava nada, para que o `Shutdown` do processo antigo nunca regrave por cima do novo (é o motivo do fechamento no passo 4).
- **`Fechar(nil)` vira `FecharEApagar()`.** Com a lista gravada podendo ser vazia, `nil` e `[]Item{}` ficariam com sentidos opostos (apagar e gravar vazio). O plano usa um método explícito para o desligar.
- **Falha de escrita na troca de versão.** O `atualizador` aborta o reinício se `Gravador.Fechar` devolver erro. Pela spec do histórico, a falha de escrita "registra no log ... e segue". O plano faz o gravador ficar fechado mesmo com a escrita falhando, e o adaptador registra no log e devolve `nil`: a troca segue e o processo novo carrega a última escrita que deu certo (pergunta q1 do relatório).
- **Pasta do histórico.** A spec diz "via `config.GetConfigPath`". O app já tem `pastaDados` (o mesmo `%APPDATA%\eprocIA`, trocado por `t.TempDir()` nos testes): o plano usa `filepath.Join(a.pastaDados, "historico.json")`. Com `pastaDados` vazia, o histórico fica desligado e o log avisa.
- **Campos novos desde a spec.** `QueueItem.Relatorio` e `ForcarOCR` não estão no formato original da seção 3.2. Pela decisão d3 (2026-11-26), a Task 10 grava o relatório sem `atencao`, e o item restaurado volta com a linha-resumo e o "Ler com OCR"; `ForcarOCR` não é gravado.
- **Protegido (versão maior).** "Não o sobrescreve até o usuário mexer na fila" precisa de um sinal: `Carregar` devolve `ErrVersaoNova` e o gravador nasce `Protegido`, ignorando listas vazias até a primeira não vazia.
- **Ordem do startup.** A spec pede `Restaurar` antes de `a.watcher.Start` (`app.go:90` da v0.4.2). Hoje o watcher sobe em `iniciarFila` por `ligarMonitor` (`app_atualizacao.go:137-139`); o histórico entra logo antes.
- **RF09 já existe** (relatório). O requisito "Histórico da fila" do `PRD.md` usa o próximo número livre depois do plano do PDF avulso.
- **Contrato do plano do PDF avulso, como ficou depois da revisão dele.** `Reprocessar` sobrescreve `saidaAnterior` só quando `OutputPath != ""` e o zera só no sucesso; `reabrirLocked` (o zeramento comum de `Reprocessar`/`ReprocessarMonitorado`) já inclui `pararEnvioPendente`; `SaidaDoItem` decide por `NomeLote` com `len(paths) > 1` e, nos outros casos, chama `DaEntrada(path)`; `Reprocessar` chama `os.Stat` direto, sem ponto de troca. Este plano cria o ponto de troca `Queue.existe` (Task 2) e o usa também nessa conferência dos `Paths`.
- **Telemetria.** `telemetria.statusZip` aceita só `success`, `error`, `cancelled`; um item interrompido pelo Shutdown chega com status vazio. Fica assim (fora do escopo da spec).

---

### Task 1: Pacote internal/historico

**Files:**
- Create: `internal/historico/historico.go`
- Create: `internal/historico/gravador.go`
- Test: `internal/historico/historico_test.go`, `internal/historico/gravador_test.go`

**Interfaces:**
- Consumes (plano do PDF avulso): `saida.SaidaDoItem(kind, path string, paths []string, saida string) bool`.
- Produces:
  - `const VersaoFormato = 1`, `const Teto = 500`.
  - `type Item struct` com os campos da seção 3.2, nas chaves dos Global Constraints: `Name, Path, Status, Message, OutputPath string; DocsCount, PagesCount, EstimatedTokens, UnreadDocs int; Error string; Timestamp, FinishedAt int64; OcrMode, CleaningLevel string; Kind string (json:"kind", sem omitempty); Paths []string (json:"paths,omitempty")`. `error`, `ocr_mode` e `cleaning_level` sem `omitempty` (o exemplo da spec grava `"error": ""`).
  - `var ErrVersaoNova = errors.New("histórico gravado por uma versão mais nova do eprocIA")`.
  - `func Carregar(arquivo string) ([]Item, error)`.
  - `func Podar(itens []Item, teto int) []Item`.
  - `type Opcoes struct { Protegido bool; AoFalhar func(error); Atraso time.Duration }` (`Atraso` zero vale 1 s).
  - `func NovoGravador(arquivo string, o Opcoes) *Gravador`, com `Agendar(itens []Item)`, `GravarAgora(itens []Item) error`, `Fechar(itens []Item) error`, `FecharEApagar() error`, `Reabrir() error`.

- [ ] **Step 1: Esqueleto.** Crie os tipos e as funções acima com corpo `panic("falta")`, para os testes falharem por asserção.

- [ ] **Step 2: Testes de `Carregar` e `Podar`** (`historico_test.go`, todos com `t.TempDir()` e caminhos fictícios `C:\Users\fulano\Downloads\5000000-00.2026.8.26.0001.zip`):
  - `TestCarregar_TresItens`: um JSON escrito à mão no teste (`os.WriteFile`, `versao: 1`) com três itens (um ZIP `success` com `kind: "zip"`, um PDF único com `output_path` `<nome>_2.txt`, um item de três PDFs com `kind: "pdf"`, `paths` e `output_path` `<CNJ>.txt` na pasta do primeiro PDF): `Carregar` devolve os três, na mesma ordem e com todos os campos. A ida e volta pelo gravador fica no passo 6 (`TestGravador_IdaEVolta`).
  - `TestCarregar_Ausente`: arquivo inexistente devolve `len == 0` e `err == nil`.
  - `TestCarregar_Corrompido`: conteúdo `{"versao":1,"itens":[` devolve vazio e `nil`; `historico.json` some e há exatamente um arquivo que casa `historico.json.corrompido-????????-??????` na pasta. Mesmo resultado com arquivo de 0 bytes (Review Focus 3) e com `versao` ausente ou 0.
  - `TestCarregar_ComBOM`: o JSON de um item válido precedido de `EF BB BF` é carregado (Review Focus 3).
  - `TestCarregar_VersaoNova`: `{"versao":2,"itens":[...]}` devolve vazio e `errors.Is(err, ErrVersaoNova)`; o conteúdo do arquivo não muda (bytes iguais).
  - `TestCarregar_Teto`: 501 itens gravados à mão no JSON (o mais novo primeiro) carregam 500; o último da lista original não está.
  - `TestPodar`: 3 itens com teto 2 devolvem os dois primeiros; teto maior que a lista devolve a lista.
  - `TestCarregar_Validacao`: num arquivo com oito itens, ficam só os válidos: descarta `path` relativo (`processo.zip`), extensão estranha (`.exe`), status `"turbo"`, `output_path` de outra pasta, item de três PDFs com `output_path` `<nome do primeiro PDF>.txt` e item com um dos `paths` relativo; mantém um ZIP `success` sem `kind` (fica com `Kind == "zip"` derivado do `Path`) e um PDF sem `kind` (`Kind == "pdf"`). `output_path` vazio passa. Um `kind` preenchido nunca é trocado pelo derivado.

- [ ] **Step 3: Rodar e ver falhar.** `go test ./internal/historico -run 'TestCarregar|TestPodar' -v` com saída em arquivo. Esperado: FAIL por `panic: falta` (não por compilação).

- [ ] **Step 4: Implementar `Carregar`, `Podar` e a validação** em `historico.go`. Regras: tira o BOM antes do `json.Unmarshal`; corrompido (JSON inválido, vazio, `versao < 1`) renomeia para `arquivo + ".corrompido-" + time.Now().Format("20060102-150405")`, registra `slog.Warn("histórico corrompido; a lista abre vazia", ...)` com o nome do arquivo novo, devolve `nil, nil`; `versao > VersaoFormato` devolve `nil, ErrVersaoNova` sem tocar no arquivo. Validação item a item: `Kind` vazio vira `"pdf"` se a extensão do `Path` for `.pdf` (sem caixa), senão `"zip"`; `Path` e cada `Paths` com `filepath.IsAbs` e extensão `.zip` ou `.pdf` (sem caixa); status em `pending`, `processing`, `success`, `error`, `cancelled`, `interrupted`; `OutputPath` vazio ou `saida.SaidaDoItem(Kind, Path, Paths, OutputPath)`. Depois `Podar(itens, Teto)`.

- [ ] **Step 5: Rodar e ver passar.** Mesmo comando. Esperado: PASS.

- [ ] **Step 6: Testes do `Gravador`** (`gravador_test.go`; `Atraso: 50 * time.Millisecond` nos testes; contador de escritas por um gancho do pacote, por exemplo `var aoGravarTeste func()`, chamado depois de cada rename bem-sucedido):
  - `TestGravador_IdaEVolta`: `GravarAgora` com os três itens de `TestCarregar_TresItens` e `Carregar` devolve `reflect.DeepEqual` a eles, na mesma ordem; o JSON gravado tem `"kind": "zip"` no item de ZIP e `"kind": "pdf"` nos de PDF.
  - `TestGravador_Debounce`: quatro `Agendar` em menos de 50 ms geram uma escrita, e o arquivo tem a lista do último.
  - `TestGravador_CriaPasta`: arquivo em `t.TempDir()/nao/existe/historico.json`; `GravarAgora` cria a pasta e o arquivo (Review Focus 4).
  - `TestGravador_SemBOM`: o arquivo gravado não começa com `EF BB BF` e tem `"versao": 1`.
  - `TestGravador_RenameFalha`: com `renomear` (variável do pacote, padrão `os.Rename`) trocado por uma função que sempre falha, `GravarAgora` devolve erro, chama `AoFalhar` uma vez, o `historico.json` anterior fica com os bytes de antes e não sobra nenhum outro arquivo na pasta. Com o `renomear` falhando só na primeira chamada, a escrita dá certo (a nova tentativa após `esperaRenomear`, variável do pacote, 200 ms no padrão e 1 ms no teste).
  - `TestGravador_Concorrencia` (roda com `-race`): oito goroutines chamam `Agendar` em rajada com listas distintas e uma chama `GravarAgora` em laço; outra goroutine lê o arquivo durante a rajada e cada leitura que existe é JSON válido igual a uma das listas enviadas. No Windows, o `os.Rename` por cima de um arquivo que a leitora tem aberto pode falhar: durante a rajada, o teste aceita erro de `GravarAgora` e chamadas de `AoFalhar` (não as conta como falha), e a leitura que falhar por compartilhamento é só pulada. Depois de juntar as goroutines, `GravarAgora(L)` deixa o arquivo igual a `L`; `Agendar(M)` e uma espera de 3 × `Atraso` deixam igual a `M`.
  - `TestGravador_FecharComTimerPendente`: `Agendar(A)` e logo `Fechar(B)`: o arquivo fica `B` e continua `B` depois de 3 × `Atraso`; `Agendar(C)` e `GravarAgora(C)` depois de `Fechar` não gravam (`GravarAgora` devolve `nil`).
  - `TestGravador_FecharDuasVezes`: `Fechar(A)` e depois `Fechar(B)`: o arquivo fica `A`.
  - `TestGravador_FecharEApagar`: com timer pendente, `FecharEApagar` apaga o arquivo e ele não reaparece depois de 3 × `Atraso`; com o gravador já fechado por `Fechar`, `FecharEApagar` também apaga.
  - `TestGravador_Reabrir`: `Fechar(A)`, `Reabrir()`, `Agendar(B)`: o arquivo fica `B`. `Reabrir` não grava nada por si.
  - `TestGravador_Protegido`: com `Protegido: true` sobre um arquivo `versao: 2`, `Agendar([])`, `GravarAgora([])` e `Fechar([])` não mudam os bytes; `GravarAgora([X])` grava e, a partir daí, `GravarAgora([])` grava a lista vazia.
  - `TestGravador_Teto`: `GravarAgora` com 501 itens grava 500.

- [ ] **Step 7: Rodar e ver falhar.** `go test -race ./internal/historico -run TestGravador -v`. Esperado: FAIL por `panic: falta`.

- [ ] **Step 8: Implementar o `Gravador`** em `gravador.go`. Um `sync.Mutex` serializa escrita, rename e mudança de estado; o timer de `Agendar` grava, sob o mutex, a última lista guardada; o estado `fechado` faz `Agendar` e `GravarAgora` voltarem sem gravar e faz `Fechar` voltar `nil` sem gravar. Escrita: `os.MkdirAll` da pasta, `os.CreateTemp(pasta, "historico-*.tmp")`, JSON indentado de `{versao, itens: Podar(itens, Teto)}`, `Close`, `renomear` por cima, uma nova tentativa após `esperaRenomear`; falhando, apaga o temporário, `slog.Warn("não foi possível gravar o histórico", "erro", err)`, chama `AoFalhar` e devolve o erro. `Fechar` marca `fechado` mesmo quando a escrita falha. Lista nil é gravada como `[]`.

- [ ] **Step 9: Rodar e ver passar.** `go test -race ./internal/historico -v`. Esperado: PASS. Depois o gate Go das regras comuns.

- [ ] **Step 10: Commit.** `git add internal/historico/historico.go internal/historico/gravador.go internal/historico/historico_test.go internal/historico/gravador_test.go`; mensagem `feat(historico): formato, carga e gravador do histórico da fila`.

---

### Task 2: Fila: estado interrompido e FinishedAt

**Files:**
- Modify: `internal/etl/queue.go` (`QueueItem`, `copiarItem`, `Reprocessar`, `CancelItem`, `LimparConcluidos`, `processItem`)
- Test: `internal/etl/queue_interrompido_test.go` (novo); ajuste dos testes antigos de `Reprocessar` que usam caminhos inexistentes (`queue_reprocessar_test.go`, `queue_nao_lidos_test.go`, `queue_relatorio_test.go`, `queue_retida_test.go`, `queue_telemetria_test.go`, e os do plano do PDF avulso que reprocessam PDFs fictícios, se houver)
- Test: `app_test.go:249-263` (`TestReprocessQueueItem_PassaForcarExplicito`, pacote `main`, reprocessa um ZIP que não existe e não alcança `q.existe`)

**Interfaces:**
- Consumes (plano do PDF avulso): `QueueItem.Kind`, `QueueItem.Paths`, `QueueItem.saidaAnterior`, o `processar` que recebe `Entrada`, e a conferência dos `Paths` em `Reprocessar` por `os.Stat` direto, com a mensagem "arquivo de origem não encontrado: <nome do PDF>".
- Produces:
  - Campos de `QueueItem`: `FinishedAt int64 \`json:"finished_at"\``, `OutputMissing bool \`json:"output_missing"\``, `SourceMissing bool \`json:"source_missing"\``, `Restaurado bool \`json:"restaurado"\``. `copiarItem` copia os quatro.
  - Status `"interrupted"` (comentário do campo `Status` atualizado).
  - Campo de `Queue` `existe func(caminho string) bool` (nil = `os.Stat` sem erro), usado por `Reprocessar` para o `Path` do ZIP e para cada um dos `Paths` do PDF: o `os.Stat` direto que o plano do PDF pôs em `Reprocessar` passa a ir por ele.

- [ ] **Step 1: Testes** (`queue_interrompido_test.go`, com `processar`, `emitir` e `carregarConfig` falsos, como os testes de fila existentes):
  - `TestFinishedAt_TodosOsFins`: `FinishedAt > 0` e `>= Timestamp` depois de sucesso, erro, cancelamento pelo usuário (`CancelItem` com o item em `processing`), panic no `processar`, falha de `carregarConfig` (com `q.cfg` nil) e `CancelItem` de item `pending` (fila retida por `Reter`, para ele não começar).
  - `TestReprocessar_ZeraFinishedAt`: depois de `Reprocessar` de um `success`, a cópia enviada tem `FinishedAt == 0`, `OutputMissing == false` e `SourceMissing == false`.
  - `TestShutdown_ItemEmCursoViraInterrompido`: com o `processar` falso bloqueado até `ctx.Done()`, `Shutdown()`; o item fica `"interrupted"`, `Message == "Interrompido"`, `FinishedAt > 0`, nunca `"cancelled"`; e `CancelItem` (sem Shutdown) do mesmo cenário continua dando `"cancelled"`.
  - `TestLimparConcluidos_TiraInterrompido`: com itens `success`, `error`, `cancelled`, `interrupted`, `pending` (fila retida) e `processing` (processar bloqueado), sobram só `pending` e `processing`. Para montar o `interrupted`, ponha o status direto em `q.items` sob `q.mu` (o teste é do pacote).
  - `TestReprocessar_OrigemAusente`: item `interrupted` com `Path` inexistente em `t.TempDir()` devolve erro com "arquivo de origem não encontrado: " seguido de `filepath.Base(Path)`, e `GetItems()` fica igual a antes (status, posição, `OutputPath`).
  - `TestReprocessar_Interrompido`: item `interrupted` com ZIP existente volta a `pending` sem erro.
  - `TestReprocessar_ExisteFalso`: com `q.existe` devolvendo `false` para um dos três `Paths` de um item de PDF, `Reprocessar` devolve "arquivo de origem não encontrado: " + o nome desse PDF, sem tocar no disco (prova que a conferência do PDF passa pelo ponto de troca).

- [ ] **Step 2: Rodar e ver falhar.** `go test -race ./internal/etl -run 'TestFinishedAt|TestReprocessar_|TestShutdown_Item|TestLimparConcluidos_Tira' -v`. Esperado: FAIL nas asserções (crie antes os campos novos vazios, se faltar compilação).

- [ ] **Step 3: Implementar** em `queue.go`: `FinishedAt = time.Now().UnixMilli()` nos seis caminhos de fim; no ramo `ctx.Err() == context.Canceled` de `processItem`, `q.encerrada` (lido sob `q.mu`, já travado ali) leva a `interrupted`/"Interrompido", senão `cancelled`; `LimparConcluidos` tira também `interrupted` (comentário atualizado); `Reprocessar` confere, antes de mudar qualquer campo, `existe(item.Path)` no ZIP e `existe` de cada `Paths` no PDF (trocando o `os.Stat` direto do plano anterior), e zera `FinishedAt`, `OutputMissing`, `SourceMissing`; o sucesso em `processItem` zera `OutputMissing` e `SourceMissing`.

- [ ] **Step 4: Testes antigos.** Os testes que chamam `Reprocessar` com `C:\x\a.zip` e similares passam a falhar pela conferência. Nos construtores de fila desses testes, ponha `q.existe = func(string) bool { return true }`; não mude asserções. Em `app_test.go` (`TestReprocessQueueItem_PassaForcarExplicito`, pacote `main`, que não alcança o campo não exportado), crie o ZIP no `t.TempDir()` com bytes fictícios (`os.WriteFile(zip, []byte("nao e zip"), 0o644)`) antes do `AddFiles`: a execução continua terminando em erro, como o comentário do teste já diz, e o `Reprocessar` passa pela conferência.

- [ ] **Step 5: Rodar e ver passar.** `go test -race ./internal/etl` com saída em arquivo. Esperado: `ok`. Depois o gate Go.

- [ ] **Step 6: Commit.** `git add` de `internal/etl/queue.go`, `internal/etl/queue_interrompido_test.go`, `app_test.go` e dos testes antigos ajustados; mensagem `feat(etl): estado interrompido e hora de término na fila`.

---

### Task 3: Fila: restaurar e gravar o histórico

**Files:**
- Create: `internal/etl/queue_historico.go`
- Modify: `internal/etl/queue.go` (`Queue`, `broadcast`, `shutdownComPrazo`, `AddFilesComOrigem`, `Reprocessar`, e `ReprocessarMonitorado` do plano do PDF avulso)
- Test: `internal/etl/queue_historico_test.go`

**Interfaces:**
- Consumes: Task 1 (`historico.Item`); Task 2 (campos e `interrupted`); plano do PDF avulso (`Kind`, `Paths`, `saidaAnterior`, `ReprocessarMonitorado(path string) bool`).
- Produces (usados pela Task 5):
  - `type GravadorHistorico interface { Agendar(itens []historico.Item); GravarAgora(itens []historico.Item) error; Fechar(itens []historico.Item) error }` (o `*historico.Gravador` a implementa).
  - `func (q *Queue) Restaurar(itens []historico.Item)`.
  - `func (q *Queue) SetGravador(g GravadorHistorico)` (nil solta; não grava).
  - `func (q *Queue) AgendarHistorico()` e `func (q *Queue) GravarHistoricoAgora() error`: mandam a lista gravada atual ao gravador (nada sem gravador).
  - `func (q *Queue) FecharHistorico() error`: `g.Fechar` com a lista gravada, `pending` e `processing` marcados como `interrupted` na lista (não na fila); `nil` sem gravador.
  - Campo de `Queue` `sessaoComEntrada bool`.

- [ ] **Step 1: Testes** (`queue_historico_test.go`; gravador falso que guarda cada lista recebida por método, sob mutex; `processar` e `emitir` falsos):
  - `TestHistorico_BroadcastAgenda`: com gravador, `AddFiles` de um ZIP gera `Agendar`; com `wailsCtx` nil (sem `SetContext`) também; um `onProgress` do `processar` falso (que dispara `queue:item`) não gera `Agendar` novo; sem gravador, nada é chamado e nada quebra.
  - `TestHistorico_ShutdownGravaInterrompido`: item em processamento (processar bloqueado até `ctx.Done()`), `Shutdown()`: o gravador recebeu `Fechar` com o item em `"interrupted"`; nenhuma lista recebida tem `"cancelled"`.
  - `TestHistorico_BroadcastTardioNaoRegrava`: com o `processar` que ignora o contexto e só volta 300 ms depois, `shutdownComPrazo(50 * time.Millisecond)`; depois que o `processItem` termina, o gravador real (`historico.NovoGravador` com `Atraso` de 20 ms em `t.TempDir()`) continua com o arquivo do `Fechar`.
  - `TestRestaurar_Converte`: `Restaurar` com itens `pending`, `processing`, `success` deixa `interrupted`, `interrupted`, `success`, todos com `Restaurado == true`, IDs distintos e não vazios, na mesma ordem, depois dos itens que já estavam na fila; `processar` não é chamado em 200 ms; um só `queue:update` emitido.
  - `TestRestaurar_SaidaAnterior`: item restaurado de PDF `success` com `output_path` gravado, `Reprocessar` dele: o `processar` falso recebe `Entrada.SaidaAnterior` igual a esse `output_path`.
  - `TestListaGravada_SemEntrada`: depois de `Restaurar` de dois itens, `SetGravador` e `RemoveItem` de um: o último `Agendar` tem só o outro.
  - `TestListaGravada_ComEntrada`: depois de `Restaurar` de dois e `AddFiles` de um ZIP novo, o último `Agendar` tem só o novo; `RemoveItem` do novo grava `[]` (não nil), sem os restaurados.
  - `TestListaGravada_SinalLiga`: o sinal `sessaoComEntrada` liga com `Reprocessar` de um item restaurado (e o item perde `Restaurado` e entra na lista gravada), com `Reprocessar` de um item que não foi restaurado, e com `ReprocessarMonitorado` sobre um item restaurado `success` de um PDF só.
  - `TestListaGravada_LimparTudoSemEntrada`: `Restaurar` de dois e `Clear()`: o último `Agendar` é `[]` (Review Focus 5).
  - `TestFecharHistorico_SemGravador`: devolve `nil`.

- [ ] **Step 2: Rodar e ver falhar.** `go test -race ./internal/etl -run 'TestHistorico_|TestRestaurar_|TestListaGravada_|TestFecharHistorico_' -v`. Esperado: FAIL nas asserções (com esqueletos de `panic("falta")` para os métodos novos).

- [ ] **Step 3: Implementar** em `queue_historico.go` as conversões `paraHistorico(*QueueItem) historico.Item` e `deHistorico(historico.Item) *QueueItem` (ID novo com `uuid.NewString()`, `Restaurado: true`, `saidaAnterior = OutputPath` só quando `OutputPath != ""`, como no plano do PDF, `pending`/`processing` viram `interrupted` com `Message` "Interrompido"), `listaGravadaLocked()` (com `sessaoComEntrada`, os itens sem `Restaurado`; sem ele, os com `Restaurado`; sempre slice não nil) e os métodos públicos. Toda chamada ao gravador acontece sob `q.emitMu` (o mesmo de `broadcast`), com a lista copiada sob `q.mu`: assim a ordem das listas que chegam ao gravador é a ordem das mudanças. Em `broadcast`, o `Agendar` vem antes do retorno por `wailsCtx == nil`. `shutdownComPrazo` chama `FecharHistorico` depois da espera (terminada ou estourada) e só então volta. `AddFilesComOrigem` (cada item criado), `Reprocessar` e `ReprocessarMonitorado` ligam `sessaoComEntrada` e tiram `Restaurado` do item.

- [ ] **Step 4: Rodar e ver passar.** `go test -race ./internal/etl` com saída em arquivo. Esperado: `ok`. Depois o gate Go.

- [ ] **Step 5: Commit.** `git add internal/etl/queue_historico.go internal/etl/queue.go internal/etl/queue_historico_test.go`; mensagem `feat(etl): restaurar a fila e gravar a lista da sessão`.

---

### Task 4: Fila: reaproveitar a linha interrompida e marcar arquivos ausentes

**Files:**
- Modify: `internal/etl/queue.go` (`AddFilesComOrigem`)
- Modify: `internal/etl/queue_historico.go`
- Test: `internal/etl/queue_historico_test.go`

**Interfaces:**
- Consumes: Tasks 2 e 3; plano do PDF avulso (`AddFilesComOrigem` agrupando os PDFs de uma chamada num item com `Paths`; `reabrirLocked`, o zeramento comum da linha).
- Produces (usados pela Task 5):
  - `func (q *Queue) ConferirArquivos(existe func(caminho string) bool)`: para cada item com `Restaurado`, `SourceMissing` se `Path` ou algum `Paths` não existe, `OutputMissing` se `OutputPath != ""` e não existe. Copia ID e caminhos sob `q.mu`, faz os `existe` fora dela e, ao aplicar (de novo sob `q.mu`, pelo ID), marca só os itens que ainda existem e ainda têm `Restaurado` (um item que voltou à fila nesse meio-tempo fica sem flag). Um só `broadcast`, e só se havia item restaurado.
  - `func (q *Queue) MarcarSaidaAusente(outputPath string)`: marca `OutputMissing` em todo item com esse `OutputPath` (sem caixa, `filepath.Clean`) e faz `broadcast`.

- [ ] **Step 1: Testes:**
  - `TestAddFiles_ReaproveitaInterrompido`: `Restaurar` de um ZIP `interrupted` (com `output_path`, `error`, `docs_count`, `unread_docs`, `finished_at` preenchidos) entre dois `success`; `AddFilesComOrigem([mesmo Path], "monitorada")` devolve 1, a fila continua com 3 itens, a linha fica na mesma posição com o mesmo ID, `pending`, `Message` "Na fila...", `OutputPath`, `Error`, contadores, `UnreadDocs` e `FinishedAt` zerados, `Timestamp` novo, `Restaurado == false`; o `processar` falso recebe a config global (sem ajuste). Com um item `interrupted` de um PDF só, o mesmo caminho reaproveita a linha e o `processar` recebe `Entrada.SaidaAnterior` igual ao `output_path` de antes.
  - `TestAddFiles_ReaproveitaMantemSaidaAnterior`: item de um PDF só `interrupted` com `OutputPath` vazio e `saidaAnterior` X (montado direto em `q.items` sob `q.mu`, caso do item interrompido no meio de um reprocesso); `AddFilesComOrigem` do mesmo PDF reaproveita a linha e o `processar` falso recebe `Entrada.SaidaAnterior == X`.
  - `TestAddFiles_ReaproveitaSemCaixa`: o mesmo com o `Path` gravado em minúsculas e o novo com maiúsculas (Review Focus 1).
  - `TestAddFiles_SoAInterrompida`: com o mesmo ZIP restaurado duas vezes, `success` acima e `interrupted` abaixo, a `interrupted` é reaproveitada e a `success` fica como estava (Review Focus 2).
  - `TestAddFiles_SuccessCriaNova`: ZIP restaurado só como `success` gera linha nova no topo.
  - `TestAddFiles_PDFsDoLote`: item de três PDFs `interrupted`; `AddFiles` de um deles cria item novo e a linha interrompida não muda; `AddFiles` dos três, em outra ordem e com caixa diferente, reaproveita a linha.
  - `TestConferirArquivos`: três restaurados (TXT e ZIP presentes; TXT ausente; ZIP ausente) e um item novo com caminhos inexistentes: as flags certas nos restaurados e nenhuma no novo; um só `queue:update`. Sem restaurados, nenhum `queue:update`.
  - `TestConferirArquivos_ItemQueVoltouAFila`: com um `existe` falso que, na primeira chamada, faz `AddFilesComOrigem` do ZIP interrompido restaurado (a linha perde `Restaurado`) e devolve `false` para tudo, a linha reaproveitada termina sem `SourceMissing` nem `OutputMissing`.
  - `TestMarcarSaidaAusente`: marca o item com o `OutputPath` em outra caixa e emite `queue:update`.

- [ ] **Step 2: Rodar e ver falhar.** `go test -race ./internal/etl -run 'TestAddFiles_|TestConferirArquivos|TestMarcarSaidaAusente' -v`. Esperado: FAIL.

- [ ] **Step 3: Implementar.** Em `AddFilesComOrigem`, antes de criar o item: ZIP procura linha `interrupted` de `Kind` `zip` com `strings.EqualFold(filepath.Clean(...))` no `Path`; o grupo de PDFs procura `interrupted` de `Kind` `pdf` com o mesmo conjunto de `Paths` (mesmo tamanho, cada caminho casado sem caixa, ordem livre). Achou: zera a linha com o `reabrirLocked` do plano do PDF avulso (o mesmo de `Reprocessar` e `ReprocessarMonitorado`, que já faz `pararEnvioPendente` e só sobrescreve `saidaAnterior` com `OutputPath` não vazio), sem reimplementar; depois `ajuste = nil` e os campos deste plano que o `reabrirLocked` não conhece (`FinishedAt`, `OutputMissing`, `SourceMissing`, `Restaurado`; se a Task 2 ou 3 já os pôs dentro do `reabrirLocked`, não repita). Põe `origem` nova, liga `sessaoComEntrada` e conta os arquivos no retorno. A deduplicação atual contra `pending`/`processing` continua antes.

- [ ] **Step 4: Rodar e ver passar.** `go test -race ./internal/etl`. Esperado: `ok`. Depois o gate Go.

- [ ] **Step 5: Commit.** `git add internal/etl/queue.go internal/etl/queue_historico.go internal/etl/queue_historico_test.go`; mensagem `feat(etl): reaproveitar a linha interrompida e marcar arquivos ausentes`.

---

### Task 5: App: carga no startup, interruptor e troca de versão

**Files:**
- Create: `app_historico.go`
- Test: `app_historico_test.go`
- Modify: `internal/models/models.go` (`Config.GuardarHistorico`), `internal/config/config.go` (`DefaultConfig`, `configOverlay`, `LoadFromFile`), `internal/config/config_test.go`
- Modify: `app.go` (campos de `App`, `SaveConfig`, `CopyOutput`), `app_atualizacao.go` (`iniciarFila`, `iniciarAtualizacao`, `arranqueApos`)
- Generated: `frontend/src/wailsjs/go/models.ts` (pelo `wails generate module`)

**Interfaces:**
- Consumes: Task 1 (`Carregar`, `ErrVersaoNova`, `NovoGravador`, `Opcoes`, `FecharEApagar`, `Reabrir`), Task 3 (`Restaurar`, `SetGravador`, `AgendarHistorico`, `GravarHistoricoAgora`, `FecharHistorico`), Task 4 (`ConferirArquivos`, `MarcarSaidaAusente`); `atualizacao.Gravador`; plano do PDF avulso (`lerSaidaParaCopiar` já com `saida.SaidaDoItem`).
- Produces: evento `"historico:falha"` (sem dados), emitido no máximo uma vez por sessão; `Config.GuardarHistorico` no JSON como `guardar_historico`.
  - Campos de `App`: `historicoMu sync.Mutex`, `historico *historico.Gravador`, `historicoAdiado bool`, `avisoHistorico sync.Once`.
  - `func (a *App) arquivoHistorico() string` (`""` com `pastaDados` vazia).
  - `func (a *App) iniciarHistorico(cfg *models.Config)`, chamada em `iniciarFila` depois de `a.watcher.SetFalhas` e antes de `if a.apos == 0 { a.ligarMonitor(ctx) }`.
  - `func (a *App) ligarHistoricoAdiado()`, chamada em `arranqueApos` depois de `esperarAntigo` e antes de `a.queue.Soltar()`.
  - `func (a *App) aplicarHistorico(ligar bool)`, chamada por `SaveConfig` depois do `config.Save` quando `GuardarHistorico` mudou.
  - `type historicoAtualizacao struct{ a *App }` com `Fechar() error` e `Reabrir() error` (este também reagenda a lista), passado em `Ambiente.Gravador`.

- [ ] **Step 1: Teste de config** (`config_test.go`): `TestLoadFromFile_GuardarHistoricoAusente`: um `config.json` sem a chave carrega `GuardarHistorico == false`; com `"guardar_historico": true`, `true`. `DefaultConfig()` devolve `false`.

- [ ] **Step 2: Testes do app** (`app_historico_test.go`, sobre a montagem `novoAppAtualizacao` de `app_atualizacao_test.go`, com `t.Setenv("APPDATA", t.TempDir())`; o histórico é gravado à mão em `m.app.pastaDados`):
  - `TestHistorico_RestauraAntesDoWatcher`: na pasta monitorada, um ZIP recente (bytes fictícios, sem `.txt`); no histórico, esse ZIP como `processing`; com `GuardarHistorico` ligado, `m.iniciar()`; depois que a varredura entregar o arquivo (espera até o status sair de `interrupted`), a fila tem uma linha só desse ZIP.
  - `TestHistorico_DesligadoNaoLe`: com `GuardarHistorico` desligado e um `historico.json` com um item, `m.iniciar()` deixa a fila vazia; depois de um `AddFilesToQueue`, 1,5 s de espera e `a.shutdown(nil)` chamado no próprio teste, o arquivo tem os mesmos bytes e o mesmo `ModTime`.
  - `TestHistorico_SessaoVaziaNaoApaga`: ligado, dois itens no arquivo; `m.iniciar()`, `a.shutdown(nil)`: `Carregar` devolve os mesmos dois itens.
  - `TestHistorico_VersaoNovaIntacta`: arquivo `versao: 2`; `m.iniciar()` e `a.shutdown(nil)` sem entrada: bytes iguais aos de antes.
  - `TestHistorico_ConfereAusentes`: um restaurado cujo TXT não existe aparece com `OutputMissing` em `GetQueue()` (espera até 2 s).
  - `TestHistorico_Interruptor`: com o histórico desligado e um item na fila, `SaveConfig` com `GuardarHistorico: true` cria o arquivo na hora com esse item; `SaveConfig` com `false` apaga o arquivo, e um `AddFilesToQueue` seguido de 1,5 s não o recria.
  - `TestHistorico_AposAtualizarAdia`: cenário `apos: 4242`, histórico com um item: depois de `m.iniciar()` a fila já tem o item restaurado; `m.app.domPronto(nil)` (o `arranqueApos` só começa pelo `darSinal`); um `RemoveQueueItem` dele antes de liberar `liberaSair` não muda o arquivo; depois de `close(m.f.liberaSair)` (e da trava), o arquivo fica sem o item em até 3 s.
  - `TestHistorico_TrocaFechaEReabre`: `historicoAtualizacao{a}.Fechar()` com um item novo na fila grava a lista e fecha: um `AddFilesToQueue` seguido de 1,5 s não muda o arquivo, e `a.shutdown(nil)` também não; `Reabrir()` sozinho, sem outra ação na fila, deixa o arquivo com a lista atual em até 1,5 s (um `RemoveQueueItem` feito com o gravador fechado chega ao arquivo); outro `AddFilesToQueue` depois volta a gravar. Sem gravador (histórico desligado), `Fechar()` e `Reabrir()` devolvem `nil`.
  - `TestHistorico_TrocaComFalhaSegue`: com o `historico.json` substituído por uma pasta de mesmo nome (o rename falha), `historicoAtualizacao{a}.Fechar()` devolve `nil` (primeira falha); `historicoAtualizacao{a}.Reabrir()` e `a.queue.GravarHistoricoAgora()` devolvem a segunda falha (erro de `GravarHistoricoAgora`); o evento `historico:falha` aparece uma vez só em `m.f.emitidos`.
  - `TestHistorico_TrocaPeloAtualizador` (Review Focus 6; montagem com `GuardarHistorico` ligado e `m.opcoes.Trocador` devolvendo um `trocadorFalso` de `app_atualizacao_test.go:767-786`, estendido com um campo `erro error` devolvido por `Reiniciar` e um gancho `antes func()` chamado no começo de `Reiniciar`; `Sair` da montagem não encerra o processo do teste). Fluxo: `InstalarAtualizacao` até `instalada` (como os testes de reinício que já existem), um item novo na fila já concluído e `ReiniciarAtualizacao()`.
    - No `antes` do `Reiniciar`, `historico.Carregar` do arquivo devolve a lista com o item (gravada no passo 4 da preparação).
    - Com `erro` não nil: o gancho `antes` faz `RemoveQueueItem` do item durante a espera, com o gravador fechado (o arquivo não muda ali). Depois da reversão (estado volta de `reiniciando`), sem nenhuma outra ação na fila, o arquivo fica sem o item em até 1,5 s, o que prova o `AgendarHistorico` do `Reabrir`.
    - Com `erro` nil: depois de `Sair`, `a.shutdown(nil)` não muda os bytes nem o `ModTime` do arquivo, mesmo com um `RemoveQueueItem` feito antes do shutdown.
  - `TestCopyOutput_TXTAusente` (com `a.ctx` não nil, `context.Background()`, e `clipboard` falso): item restaurado `success` cujo TXT não existe; `CopyOutput` devolve exatamente "O TXT não está mais em " + pasta + "; ele pode ter sido movido ou apagado" e o item fica com `OutputMissing`.
  - `TestLerSaidaParaCopiar_SaidaDoItem` (**guarda de regressão**, se o plano do PDF avulso já não tiver um teste com estes casos): recusa `output_path` que não passa em `saida.SaidaDoItem` e aceita o `<CNJ>.txt` de um item de três PDFs.

- [ ] **Step 3: Rodar e ver falhar.** `go test -race . ./internal/config -run 'TestHistorico_|TestCopyOutput_TXTAusente|TestLoadFromFile_GuardarHistorico' -v` com saída em arquivo. Esperado: FAIL nas asserções.

- [ ] **Step 4: Implementar** `Config.GuardarHistorico` e o overlay; `app_historico.go` com as funções das Interfaces. `iniciarHistorico`: sai se desligado ou sem `pastaDados` (com `slog.Warn`); `Carregar`, com `ErrVersaoNova` dando `Protegido: true` e outro erro só no log; `NovoGravador` com `AoFalhar` que emite `historico:falha` por `a.emitirEm(a.ctx, ...)` dentro de `avisoHistorico.Do`; `Restaurar` se houver itens; com `a.apos > 0`, `historicoAdiado = true`, senão `SetGravador`; `go a.queue.ConferirArquivos(existe)`. `aplicarHistorico(true)` cria o gravador, e sem adiamento faz `SetGravador` e `GravarHistoricoAgora`; `aplicarHistorico(false)` faz `FecharEApagar`, `SetGravador(nil)` e zera `a.historico`. O adaptador: `Fechar` chama `a.queue.FecharHistorico()`, registra o erro em `slog.Warn("atualização: o histórico não foi gravado", ...)` e devolve `nil`; `Reabrir` chama `a.historico.Reabrir()` se houver e, em seguida, `a.queue.AgendarHistorico()`, para que um `RemoveItem` ou `Clear` feito nos até 90 s de espera do reinício chegue ao arquivo. `CopyOutput`: com `errors.Is(err, fs.ErrNotExist)`, `MarcarSaidaAusente` e a frase exata.

- [ ] **Step 5: Rodar e ver passar.** Mesmo comando. Esperado: PASS. Depois o gate Go inteiro (`go vet ./...`, `go test -race ./...`), incluindo os testes de `internal/atualizacao` e `app_atualizacao_test.go`.

- [ ] **Step 6: Bindings.** `wails generate module`; confira com `git diff --ignore-cr-at-eol --stat` que `frontend/src/wailsjs/go/models.ts` ganhou `guardar_historico`, `finished_at`, `output_missing`, `source_missing` e `restaurado`; reverta o que for só fim de linha.

- [ ] **Step 7: Commit.** `git add` de `app_historico.go`, `app_historico_test.go`, `app.go`, `app_atualizacao.go`, `internal/models/models.go`, `internal/config/config.go`, `internal/config/config_test.go`, `frontend/src/wailsjs/go/models.ts`; mensagem `feat(app): histórico da fila no startup, no interruptor e na troca de versão`.

---

### Task 6: OpenDirectory abre a pasta quando o arquivo sumiu

**Files:**
- Modify: `app.go:393-401` (`OpenDirectory`) e o struct `App`
- Test: `app_saida_test.go`

**Interfaces:**
- Produces: campo de `App` `explorer func(args ...string) error` (nil = `exec.Command("explorer.exe", args...).Start()`).

- [ ] **Step 1: Testes** (com `explorer` falso que guarda os argumentos):
  - `TestOpenDirectory_ArquivoExiste`: arquivo existente em `t.TempDir()` gera `["/select,", caminho]`.
  - `TestOpenDirectory_ArquivoAusente`: arquivo inexistente numa pasta existente gera `[pasta]`.
  - `TestOpenDirectory_PastaAusente`: arquivo e pasta inexistentes não chamam o `explorer` e devolvem erro com "a pasta não existe mais".
  - `TestOpenDirectory_Pasta`: uma pasta existente gera `[pasta]`.

- [ ] **Step 2: Rodar e ver falhar.** `go test . -run TestOpenDirectory -v`. Esperado: FAIL (`ArquivoAusente` e `PastaAusente`).

- [ ] **Step 3: Implementar.** `Stat` do caminho: arquivo seleciona; pasta abre; ausente, `filepath.Dir` e abre só se ela existir como pasta; senão `fmt.Errorf("a pasta não existe mais: %s", pasta)`.

- [ ] **Step 4: Rodar e ver passar.** Mesmo comando, depois o gate Go. Esperado: PASS.

- [ ] **Step 5: Commit.** `git add app.go app_saida_test.go`; mensagem `fix(app): abrir a pasta do TXT quando o arquivo não existe mais`.

---

### Task 7: Frontend: lógica da linha (data, interrompido, ausentes)

**Files:**
- Modify: `frontend/src/lib/fila.js` (`formatarHora` substituída por `formatarQuando`, `CONCLUIDO`, `copiavel`), `frontend/src/lib/estado.js` (`estadoItem`), `frontend/src/lib/textos.js` (bloco `HISTORICO`)
- Modify: `frontend/src/components/ProcessList.svelte:58` (import) e `:659` (data da linha), só a troca de `formatarHora` por `formatarQuando`, para o build deste commit passar
- Test: `frontend/fila.test.mjs` (incluindo o teste de `:76`, "ProcessList usa formatarHora e filtrarItens"), `frontend/estado.test.mjs`

**Interfaces:**
- Produces:
  - `export function formatarQuando(item, agora = Date.now())` em `fila.js`: usa `item.finished_at` se for número maior que zero, senão `item.timestamp`; devolve `''` para valor ausente ou inválido.
  - `estadoItem('interrupted')` devolve `'interrompido'`.
  - `copiavel(item)` exige também `!item.output_missing`.
  - `export const HISTORICO` em `textos.js`, com as chaves `interrompido` ("Interrompido"), `processarDeNovo` ("Processar de novo"), `txtAusente` ("TXT não encontrado"), `origemAusente` ("O arquivo de origem não foi encontrado; ele pode ter sido movido ou apagado"), `semDetalhes` ("Detalhes por documento só ficam disponíveis durante a sessão em que o item foi processado"), `interruptor` ("Guardar a lista da última sessão"), `fraseInterruptor` (o texto exato dos Global Constraints), `falhaGravar` ("Não foi possível guardar o histórico") e `tituloLimparConcluidos` ("Remover da lista os concluídos, com erro, cancelados e interrompidos").

- [ ] **Step 1: Testes** em `fila.test.mjs` (datas locais com `new Date(ano, mes, dia, h, m).getTime()`, `agora` = 26/11/2026 14:40):
  - `formatarQuando`: 26/11/2026 14:32 dá `'14:32'`; 25/11/2026 14:32 dá `'ontem 14:32'`; 12/11/2026 14:32 dá `'12/11 14:32'`; 12/11/2025 14:32 dá `'12/11/2025 14:32'`; com `agora` = 26/11/2026 00:05, 25/11/2026 23:55 dá `'ontem 23:55'`; com `agora` = 1/1/2026 00:10, 31/12/2025 23:50 dá `'ontem 23:50'` (virada de ano); `finished_at` tem prioridade sobre `timestamp`; `finished_at: 0` usa `timestamp`; `timestamp` `undefined`, `NaN` e `'x'` dão `''`.
  - `contarConcluidos([{status:'interrupted'},{status:'interrupted'}]) === 2`; `acaoDelete('interrupted') === 'remover'`.
  - `copiavel({status:'success', output_path:'C:\\x.txt', output_missing:true}) === false`.
  - `paradasItem` de um `interrupted` dá `['reprocessar', 'remover']` (o "Processar de novo" usa a parada `reprocessar`).
  - Em `estado.test.mjs`: `estadoItem('interrupted', false)` e `estadoItem('interrupted', true)` dão `'interrompido'`.
  - O teste de `fila.test.mjs:76` passa a se chamar "ProcessList usa formatarQuando e filtrarItens" e confere `/\{formatarQuando\(item, agora\)\}/`, `/\$: agora = \(items, Date\.now\(\)\)/` e que `formatarHora` não aparece em `ProcessList.svelte`.

- [ ] **Step 2: Rodar e ver falhar.** `node --test frontend/fila.test.mjs frontend/estado.test.mjs` (Git Bash). Esperado: FAIL.

- [ ] **Step 3: Implementar.** Dois `Intl.DateTimeFormat('pt-BR')` criados uma vez (hora e minuto; dia e mês com dois dígitos) e um terceiro com ano, como o `FORMATO_HORA` atual; "ontem" compara as datas de calendário locais (`agora` menos um dia), não 24 h. Tire `formatarHora` de `fila.js` e, na mesma tarefa, troque o import de `ProcessList.svelte:58` e a linha `:659` por `{formatarQuando(item, agora)}`, com `$: agora = (items, Date.now())` no script (recalculado a cada mudança de `items`, sem timer). Nada mais muda no componente nesta tarefa.

- [ ] **Step 4: Rodar e ver passar.** `node --test frontend/*.test.mjs`, `npx svelte-check` e `npm run build` em `frontend`. Esperado: tudo passa; todo commit precisa passar no build.

- [ ] **Step 5: Commit.** `git add frontend/src/lib/fila.js frontend/src/lib/estado.js frontend/src/lib/textos.js frontend/src/components/ProcessList.svelte frontend/fila.test.mjs frontend/estado.test.mjs`; mensagem `feat(frontend): data, estado interrompido e TXT ausente na lógica da fila`.

---

### Task 8: Frontend: lista, painel, configurações e aviso

**Files:**
- Modify: `frontend/src/components/ProcessList.svelte`, `frontend/src/components/AtividadeItem.svelte`, `frontend/src/components/Settings.svelte`, `frontend/src/App.svelte`
- Test: `frontend/historico.test.mjs` (novo, regex sobre os `.svelte`, no estilo de `linha-fila.test.mjs`)

**Interfaces:**
- Consumes: Task 7 (`formatarQuando`, `estadoItem`, `copiavel`, `HISTORICO`); Task 5 (`guardar_historico`, campos novos de `QueueItem`, evento `historico:falha`); binding `ReprocessQueueItem(id, ocrMode, cleaningLevel, forcarOCR)`.
- Produces: prop `restaurado` de `AtividadeItem.svelte` (booleana, padrão `false`).

- [ ] **Step 1: Testes** (`frontend/historico.test.mjs`):
  - ProcessList: há ramo `estado === 'interrompido'` com um ícone (`Icone icone=`) no bloco de estado e `HISTORICO.interrompido` na linha de detalhes; `ROTULO_ESTADO` tem `interrompido: 'Interrompido'`; há ramo `item.status === 'interrupted'` nas ações com um botão `data-acao="processar"` `data-parada="reprocessar"` que chama `ReprocessQueueItem(item.id, config...ocr_mode, config...cleaning_level, !!config...forcar_ocr)`, com `aria-disabled` que inclui `item.source_missing` e `entradaBloqueada`; o botão Reprocessar dos ramos `success` e erro/cancelado também tem `source_missing` no `aria-disabled` e `HISTORICO.origemAusente` na dica; `item.output_missing` aparece com `HISTORICO.txtAusente`; a linha usa `formatarQuando(item, agora)` e não `formatarHora`; o título de "Limpar concluídos" é `HISTORICO.tituloLimparConcluidos`; `temAtividade` inclui `'interrompido'`; o `AtividadeItem` recebe `restaurado={item.restaurado}`.
  - AtividadeItem: com `atividade === 'nenhuma'` e `restaurado`, mostra `HISTORICO.semDetalhes`; sem `restaurado`, a frase de hoje.
  - Settings: um `role="switch"` com `bind:checked={config.guardar_historico}`, rótulo `HISTORICO.interruptor` e a frase `HISTORICO.fraseInterruptor`; o padrão do objeto de config local tem `guardar_historico: false`.
  - App.svelte: `EventsOn('historico:falha'` mostra `HISTORICO.falhaGravar` pelo `aviso`.

- [ ] **Step 2: Rodar e ver falhar.** `node --test frontend/historico.test.mjs`. Esperado: FAIL.

- [ ] **Step 3: Implementar.** Ícone do interrompido: um ícone do `lucide-svelte` já instalado (por exemplo `circle-pause`) com `cor="var(--warning)"`; confira `frontend/icones.test.mjs` e `icone.test.mjs`, que podem restringir ícones e cores. A data da linha (`formatarQuando(item, agora)`) já veio da Task 7. "Processar de novo" segue o foco e o nome acessível dos outros botões da linha (F10/F11: `rotuloAcao(HISTORICO.processarDeNovo, item.name)`, `tabParada(item.id, 'reprocessar', paradaAtiva)`) e a guarda de `entradaBloqueada`. Clique com `aria-disabled` verdadeiro não faz nada. O botão "Copiar" do `success` com `output_missing` fica com `aria-disabled` e a dica `HISTORICO.txtAusente`; o atalho "c" já respeita `copiavel`.

- [ ] **Step 4: Rodar e ver passar.** Gate frontend das regras comuns: `node --test frontend/*.test.mjs`, `npx svelte-check` e `npm run build` em `frontend`. Esperado: tudo passa, sem aviso novo do build.

- [ ] **Step 5: Commit.** `git add frontend/src/components/ProcessList.svelte frontend/src/components/AtividadeItem.svelte frontend/src/components/Settings.svelte frontend/src/App.svelte frontend/historico.test.mjs`; mensagem `feat(frontend): histórico da fila na lista e nas configurações`.

---

### Task 9: Capturas, DESIGN, PRD, README, CLAUDE.md e ajuda do -Purge

**Files:**
- Modify: `scripts/visual/estados.mjs`, `scripts/visual/estados.test.mjs`, `scripts/visual/capturar.mjs`, `.claude/skills/eprocia-visual/SKILL.md` (contagem de capturas)
- Modify: `DESIGN.md` (seção 5.1 e 5.3), `PRD.md` (requisito novo e privacidade), `README.md` (Configurações e Privacidade), `CLAUDE.md` (Config e Architecture), `install.ps1` (ajuda de `-Purge`, linhas 42-45)
- Create: capturas em `docs/revisao/capturas/2026-11-26-historico/`

**Interfaces:**
- Consumes: Tasks 5, 7 e 8.

- [ ] **Step 1: Teste do estado visual.** Em `estados.test.mjs`, acrescente `'historico'` à lista de estados esperada, `'interrupted'` a `STATUS` e um teste: `ESTADOS.historico.fila` tem um item de hoje, dois da sessão anterior em outro dia (um `success` com `finished_at`, um `success` com `output_missing: true`) e um `interrupted`; todos com nomes `5000000-00.2026.4.00.NNNN`.

- [ ] **Step 2: Rodar e ver falhar.** `node --test scripts/visual/estados.test.mjs`. Esperado: FAIL.

- [ ] **Step 3: Implementar.** Estado `historico` em `estados.mjs`; em `capturar.mjs`, relógio fixo (`page.clock.setFixedTime`) em `BASE_TS` mais duas horas e `timezoneId: 'America/Sao_Paulo'` no contexto, para "hoje", "ontem" e "dd/mm" ficarem estáveis em todas as capturas. A variação de lista estreita do `historico` segue a de `relatorio` (`-fila-660`). Atualize a contagem total na skill.

- [ ] **Step 4: Rodar e capturar.** `node --test .claude/hooks/hooks.test.mjs .claude/statusline.test.mjs .claude/workflows/workflows.test.mjs scripts/visual/estados.test.mjs` passa; `node scripts/visual/capturar.mjs --saida docs/revisao/capturas/2026-11-26-historico --estados historico,fila,configuracoes` termina com `[visual] N capturas em ...`; confira nas imagens dos dois temas e da lista estreita que "ontem 14:32" e "TXT não encontrado" não cortam o nome nem o rótulo de não lidos (F12). Encerre o Vite pelo PID se o script não o fizer.

- [ ] **Step 5: Documentação.**
  - `DESIGN.md`: data na linha (formatos dos Global Constraints), estado Interrompido (ícone, rótulo, "Processar de novo"), "TXT não encontrado", o interruptor em Configurações e a regra de que o monitoramento processa sozinho o ZIP das últimas 24 h sem saída, incluindo o interrompido.
  - `PRD.md`: requisito "Histórico da fila" no próximo número livre (opcional, desligado por padrão, só a última sessão com processamento, até 500 itens, local) e a nota de privacidade da seção 5 da spec.
  - `README.md`: em Configurações, o interruptor; em Privacidade, o que fica guardado, onde e como limpar (X, Limpar tudo, desligar o interruptor, `install.ps1 -Uninstall -Purge`).
  - `CLAUDE.md`: no parágrafo Config, `historico.json` ao lado de `falhas.json`, opcional e desligado por padrão, gravado pelo `historico.Gravador` (debounce de 1 s, temporário e rename) e fechado no `Shutdown` e na troca de versão (adaptador `historicoAtualizacao`), só a última sessão com entrada (`sessaoComEntrada`); no processo `--apos-atualizar`, restaurado no startup e gravado só depois que o antigo sai; `Queue.Restaurar` antes do watcher para a varredura reaproveitar a linha interrompida.
  - `install.ps1`: a ajuda de `-Purge` cita "histórico da fila". Mantenha o arquivo sem BOM e sem caracteres cujos bytes UTF-8 incluam 0x91 a 0x94 (seta, Ó, Ô); "histórico" (ó) pode. Rode `node --test frontend/install-ps1.test.mjs`.

- [ ] **Step 6: Commit.** `git add` dos arquivos listados e das capturas novas (só `.png` em `docs/revisao/capturas/2026-11-26-historico/`); mensagem `docs: histórico da fila no DESIGN, PRD, README, CLAUDE.md e capturas`.

---

### Task 10: Relatório no histórico, sem atencao (condicional)

**Executada: o dono escolheu gravar o relatório só com contagens, sem nomes de documento (decisão d3 de 2026-11-26).**

**Files:**
- Modify: `internal/historico/historico.go` (`Item`), `internal/etl/queue_historico.go` (`paraHistorico`, `deHistorico`)
- Test: `internal/historico/historico_test.go`, `internal/etl/queue_historico_test.go`
- Modify: `scripts/visual/estados.mjs` (um item da sessão anterior com `relatorio` no estado `historico`), `CLAUDE.md`, `README.md` e `PRD.md` (a frase sobre o que fica guardado; no `PRD.md`, a nota de privacidade escrita na Task 9 passa a dizer que o relatório vai para o arquivo só com contagens, sem nomes de documento)

**Interfaces:**
- Consumes: Tasks 1, 3 e 9; `models.RelatorioZip` (`internal/models/relatorio.go:21-30`). O `historico` pode importar `internal/models` (não importa `net` nem `etl`).
- Produces: `Item.Relatorio *models.RelatorioZip` com a chave `json:"relatorio,omitempty"`. A Global Constraint "não se grava `relatorio`" passa a ser "grava-se `relatorio` sem `atencao`".

- [ ] **Step 1: Testes:**
  - `TestGravador_RelatorioIdaEVolta` (historico): um item `success` com um `RelatorioZip` preenchido (documentos, páginas por categoria, tempo, OCR, limpeza, `bytes_txt`, `tokens`) e `Atencao` nil volta igual (`reflect.DeepEqual`) por `GravarAgora` e `Carregar`.
  - `TestParaHistorico_SemAtencao` (etl): um item com `Relatorio.Atencao` de dois documentos (`Nome` `Evento 3 - PET1.pdf`): o JSON gravado no arquivo não contém a chave `"atencao"` nem o texto `PET1`, e o `Relatorio` do item na fila continua com os dois (a cópia gravada é outra struct, não o mesmo ponteiro alterado).
  - `TestRestaurar_ComRelatorio` (etl): o item restaurado tem `Relatorio` não nil com as contagens do arquivo e `Atencao` nil.
  - `TestHistorico_TamanhoPorItem` (historico): 500 itens com relatório completo geram um arquivo de no máximo 1,5 MB (medido: cerca de 1,2 MB, 2,3 KB por item com o JSON indentado).

- [ ] **Step 2: Rodar e ver falhar.** `go test -race ./internal/historico ./internal/etl -run 'Relatorio|SemAtencao|TamanhoPorItem' -v`. Esperado: FAIL.

- [ ] **Step 3: Implementar.** `paraHistorico` copia o `RelatorioZip` por valor e zera `Atencao` na cópia; `deHistorico` passa o ponteiro carregado. Nada muda no frontend: com `relatorio`, o item restaurado já mostra a linha-resumo, o painel e os avisos por `paradasRelatorio` (`fila.js`). Os documentos a conferir (`atencao`) não aparecem no item restaurado.

- [ ] **Step 4: Rodar e ver passar.** Gate Go; `node --test scripts/visual/estados.test.mjs`; captura `--estados historico` nos dois temas, conferindo a linha-resumo no item restaurado.

- [ ] **Step 5: Commit.** `git add` dos arquivos listados e das capturas refeitas; mensagem `feat(historico): guardar o relatório do item sem os nomes de documento`.
