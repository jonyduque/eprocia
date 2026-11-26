# Processar PDF avulso: plano de implementação

> **Para agentes:** sub-skill obrigatória: use superpowers:subagent-driven-development (recomendado) ou superpowers:executing-plans para implementar este plano tarefa por tarefa. Os passos usam checkbox (`- [ ]`) para acompanhamento. Regras comuns a todo implementador e revisor (worktree, gate, TDD, commit, relatório): `docs/superpowers/plans/2026-11-26-pdf-historico-regras.md`.

**Goal:** Aceitar PDFs soltos (arrastados, escolhidos no diálogo ou, com a opção ligada, vindos da pasta monitorada) e convertê-los pelo mesmo caminho dos documentos de um ZIP, num TXT que nunca sobrescreve arquivo alheio.

**Architecture:** Um pacote folha novo, `internal/saida`, guarda a regra única do nome de saída (`Base`, `NomeLote`, `GravarSemSobrescrever`, `DaEntrada`, `SaidaDoItem`) e é importado por `etl`, `watchdog` e `main` (e, depois, pelo histórico). `models.ProcessFile` ganha `Abrir`/`Tamanho`/`Avulso`, para o pipeline ler entrada de ZIP e PDF solto pela mesma rotina (`extrairPDF`, que copia para memória ou para um temporário próprio: o caminho do usuário nunca vira `fontePDF.caminho`). `LerEntrada` substitui `ReadZipFiles` na entrada do pipeline; `ProcessarEntrada(Entrada)` é o método novo, e `ProcessZip`/`ProcessZipComAtividade` viram invólucros de uma linha. A fila junta os PDFs de uma chamada num item (`Kind`, `Paths`, `saidaAnterior`), e o watchdog passa a receber a lista de extensões.

**Tech Stack:** Go 1.27, Wails v2.13.0, `fsnotify`, `golang.org/x/sys/windows` (já no `go.mod`, só em teste aqui), Svelte 5 em sintaxe legada, `lucide-svelte` 1.0.1, `node --test`, Playwright das capturas (`scripts/visual/capturar.mjs`). Nenhuma dependência nova.

**Spec:** `docs/superpowers/specs/2026-11-18-pdf-avulso-design.md` (decisões p1 a p3, e1 a e5 e q1 na seção 10). Formato do `.txt`: `PRD.md` §6. Visual: `DESIGN.md`.

## Global Constraints

- Regras de ambiente, gate, commit e relatório: `docs/superpowers/plans/2026-11-26-pdf-historico-regras.md`. Este plano roda antes do plano do histórico (`docs/superpowers/plans/2026-11-26-historico.md`), na mesma branch; os nomes de `internal/saida`, `QueueItem.saidaAnterior` e `Entrada.SaidaAnterior` são contrato com ele e não mudam.
- **O original do usuário nunca é alterado, apagado nem passado como `fontePDF.caminho`**, inclusive com panic, cancelamento, prazo do OCR e caminho do Poppler. Só temporários `eprocia_pdf_*.pdf` são apagados.
- Extensões aceitas: `.zip` e `.pdf`, sem diferença de caixa. O resto é recusado no Go (`AddFilesComOrigem`), não só no frontend.
- Nome do TXT de item de PDF: `<nome>.txt`; ocupado, `<nome>_2.txt` ... `<nome>_999.txt`; depois disso, erro do item `não há nome livre para o TXT`. "Ocupado" é existir qualquer arquivo com esse nome ou, só para a base, existir `<nome>.zip` na mesma pasta (e5). Escolha e criação numa só operação (`O_CREATE|O_EXCL`).
- Vários PDFs (p3, e2, e3): base = número CNJ (`NNNNNNN-DD.AAAA.J.TR.OOOO`) comum a todos os nomes, senão o nome do primeiro PDF na ordem por evento; pasta = a do primeiro PDF nessa ordem. Com um PDF só, `Base` dele.
- ZIP mantém o comportamento atual: grava `<nome>.txt` e sobrescreve o próprio TXT.
- Reprocessar (e1) e PDF editado na pasta monitorada (q1) gravam por cima do TXT que o item gerou, se ele ainda existir e passar em `saida.SaidaDoItem`; senão escolhem nome livre de novo.
- `OmitirTipos` não vale para PDF avulso.
- Marcadores e erros que vão ao `.txt` ou ao painel passam por `textoErroSaida`: nenhum caminho, stderr ou dump. Texto novo do avulso: `[ERRO ao abrir PDF: <texto>]` (o de ZIP continua `[ERRO ao extrair PDF do ZIP: <texto>]`).
- `MonitorarPDF` (`json:"monitorar_pdf"`) nasce `false`, inclusive no overlay de `LoadFromFile`.
- Textos de interface exatos: botão "Selecionar arquivos..."; vazio "ou arraste ZIPs do Eproc ou PDFs para a janela"; recusa "Arraste arquivos ZIP do Eproc ou PDF"; atenção "N arquivos ignorados: só ZIP do Eproc ou PDF"; interruptor "Também converter PDFs novos nesta pasta" com a ajuda "Cada PDF que aparecer na pasta ganha um TXT ao lado, inclusive boletos e outros PDFs que não são de processo"; filtros do diálogo "ZIP do Eproc e PDF (*.zip;*.pdf)", "Somente ZIP (*.zip)", "Somente PDF (*.pdf)"; nome de item de vários PDFs `<nome do TXT sem extensão> (N PDFs)`.
- Dados de teste só sintéticos (`5000000-00.2026.8.26.0001`, `FULANO DE TAL`, `C:\Users\fulano`); nada de PDF real.
- `frontend/src/wailsjs/` só muda por `wails generate module`. `go.mod`, `go.sum` e `frontend/package.json` não mudam.

## Divergências da spec com o código atual

A spec foi lida sobre a v0.4.2; o código de referência é a `main` com a v0.5.0.

- **Linhas.** `extrairPDF` está em `pipeline.go:209-251`, `tipoOmitido` em `:255-260` (aplicado em `:405-408`), `ReadZipFiles` é chamado em `:343`, o marcador de extração em `:732`, o HTML abre em `:778` e a gravação está em `:838-851`. O campo `bytes` da linha "documento concluído" (`:593`) também lê `ZipFileRef.UncompressedSize64` e passa a ler `f.Tamanho`. O relógio do formatter está em `formatter.go:496`, dentro de `GerarSaida` (`GenerateConsolidatedOutput` hoje a embrulha).
- **Fila.** A entrada real é `AddFilesComOrigem(paths, origem)` (`queue.go:245-288`); `AddFiles` só a chama com origem "manual". O campo `processar` está em `queue.go:100` e é chamado em `:746`; `Reprocessar` zera `OutputPath` em `:372`; `copiarItem` está em `:306-331`. Os fakes de `processar` (atribuições `q.processar = func(...)`, `procFalso.processar` e o fake de `queue_fila_test.go:234`, e o tipo `processarFunc` de `queue_eventos_test.go:44`) estão em `queue_test.go`, `queue_eventos_test.go`, `queue_fila_test.go`, `queue_nao_lidos_test.go`, `queue_notificacao_test.go`, `queue_relatorio_test.go`, `queue_reprocessar_test.go`, `queue_retida_test.go` e `queue_telemetria_test.go`. Todos mudam de assinatura na Task 5. A lista completa sai de `grep -n "processar" internal/etl/queue_*_test.go`.
- **`Reprocessar` move o item para o topo** (`queue.go:378-380`). A spec pede que `ReprocessarMonitorado` deixe o item "na mesma posição": o plano segue a spec para `ReprocessarMonitorado` e não mexe no `Reprocessar`.
- **Arraste.** O soltar chama `AddDroppedFiles` (origem "arrastada", `App.svelte:225`), não `AddFilesToQueue`; a seleção manual (`App.svelte:407-425`) chama `SelectZipFiles` e `AddFilesToQueue`. Não há botão no painel (`App.svelte:599` da spec): o único botão de selecionar é o do vazio, `ProcessList.svelte:598-602` (prop `aoSelecionarZip`, mantida com esse nome para não mexer em testes alheios).
- **Ícones.** `Icone.svelte` recebe um componente do lucide (DESIGN §4); "ganhar o ícone de documento" vira importar `File` e `FileArchive` de `lucide-svelte/icons/`. A linha não tem hoje ícone de tipo: ele entra antes do nome, a 12 px.
- **Watcher.** Além de `aplicarMonitor` (`app.go:351`), `app_atualizacao.go:99` (arranque depois da atualização) chama `watcher.Start`; os dois passam a mandar as extensões. Os testes do watchdog chamam `Start(dir)` e `pendingZips(dir, desde, ignorar)`: mudam pela assinatura nova.
- **Log e telemetria** (`internal/registro`, `internal/telemetria`) não existiam na spec. O atributo `zip` das linhas do pipeline e da fila passa a levar o nome do primeiro PDF num item de PDF; a etapa de erro de `LerEntrada` continua `EtapaAbrirZip` (o enum da telemetria não muda); `ExecucaoZip.BytesZip` de item de PDF é a soma dos tamanhos dos PDFs.
- **`NomeLote` e a ordem por evento.** `internal/saida` é folha e não pode importar o ordenador de `etl`. Contrato: `NomeLote(pdfs)` recebe a lista já na ordem por evento (é assim que `QueueItem.Paths` é guardado) e toma o primeiro elemento como "primeiro na ordem". Quem chama ordena com `etl.ordenarPorEvento`; para a ordem ser a mesma na fila e no pipeline com nomes iguais vindos de pastas diferentes, `SortProcessFiles` passa a ser estável (`slices.SortStableFunc`).
- **Testes de frontend.** `avisos.test.mjs:160-171` fixa `separarZips` e os textos antigos, e `monitoramento.test.mjs:151-152` fixa os textos do vazio; `scripts/visual/estados.test.mjs:7-8` lista os campos de `QueueItem` e `:121` exige nomes `.zip`. Todos são atualizados na Task 8.
- **Textos novos de interface** vão em `frontend/src/lib/textos.js`, como manda o `CLAUDE.md` desde o relatório de processamento.
- **`lerSaidaParaCopiar`**: o teste atual (`app_saida_test.go:27-30`) monta itens sem `Kind` nem `Path`; com `SaidaDoItem` eles precisam de `Kind: "zip"` e `Path` (Task 7).
- **Frase do arquivo trocado no meio da leitura (decidido).** A spec diz, em 2.1, que entrada maior que o tamanho declarado no avulso dá "o arquivo mudou durante a leitura". Em 4, diz que a segunda leitura "com tamanho diferente" termina com "falha de leitura ou gravação em disco". Decisão (ruling 1 do orquestrador, 2026-11-26): qualquer diferença de tamanho (maior, menor ou zerado) dá "o arquivo mudou durante a leitura"; arquivo apagado ou travado dá "falha de leitura ou gravação em disco" (`errArquivoMudou`, Task 4).
- **Overlay do arraste** (`App.svelte:519`, "Solte o arquivo ZIP em qualquer lugar para processar"): a spec não o cita. Por decisão do orquestrador, ele passa a dizer "Solte arquivos ZIP ou PDF em qualquer lugar para processar" (Task 8).
- **Anonimizador de log** (`scripts/anonimizar-log/anonimizar.go:38-57`): só liga o radical ao `.txt` quando o nome termina em `.zip`. Num item de PDF, o atributo `zip` é `<nome>.pdf` ou o nome do item com ` (N PDFs)`, e o TXT pode ser `<nome>_N.txt`. A Task 10 cobre esses casos.

## Review Focus

- **Lote cujo CNJ comum coincide com o processo baixado em ZIP na mesma pasta.** O caso mais comum: `5000000-00.2026.8.26.0001.zip` e o `.txt` dele já estão em Downloads, e o usuário solta três peças avulsas do mesmo processo. Esperado: o lote grava `<CNJ>_2.txt` e o TXT do ZIP fica intacto (mesmo SHA-256). Teste na Task 3 (`TestProcessarEntrada_LoteNaoTomaOTxtDoZip`).
- **O mesmo PDF duas vezes numa chamada** (seleção que repete o arquivo, ou o mesmo caminho com outra caixa). Esperado: um documento só, retorno 1. Teste na Task 5 (`TestAddFiles_MesmoPDFRepetidoNaChamada`).
- **Pasta com nome terminado em `.pdf` no arraste.** Esperado: nada trava; num item de um PDF, erro do item "arquivo não encontrado"; num lote, o documento fica `ilegivel` e os outros seguem. Teste na Task 2 (`TestLerEntrada_DiretorioComNomePDF`).
- **Gravação que falha** (pasta só de leitura, disco cheio, pasta apagada). Esperado: erro do item, nenhum `<nome>_N.txt` vazio ou parcial deixado. Teste na Task 1 (`TestGravarSemSobrescrever_FalhaNaoDeixaArquivo`).
- **Dois soltares seguidos de peças do mesmo processo.** Dois itens com o mesmo nome provisório. Esperado: o segundo grava `<CNJ>_2.txt` e cada linha termina com o nome do seu TXT ("<CNJ> (2 PDFs)" e "<CNJ>_2 (2 PDFs)"). Teste na Task 5 (`TestProcessItem_NomeTrocadoPeloTXT`).

---

### Task 1: pacote internal/saida

**Files:**
- Create: `internal/saida/saida.go`
- Test: `internal/saida/saida_test.go`

**Interfaces:**
- Produces (pacote `saida`, importa só a biblioteca padrão):
  - `func Base(entrada string) string`: `<pasta>\<nome sem a última extensão>.txt`.
  - `func NomeLote(pdfs []string) string`: base do TXT de um item de PDFs, com `pdfs` já na ordem por evento; `""` para lista vazia.
  - `func GravarSemSobrescrever(base, conteudo string) (string, error)`: devolve o caminho criado.
  - `func Gravar(caminho, conteudo string) error`: gravação direta (`O_WRONLY|O_CREATE|O_TRUNC`, 0644), a de hoje do ZIP.
  - `func DaEntrada(entrada, saida string) bool`.
  - `func SaidaDoItem(kind, path string, paths []string, saida string) bool`.
  - `var ErrSemNomeLivre = errors.New("não há nome livre para o TXT")`.

- [ ] **Passo 1: testes** (os de nome são puros, com `C:\Users\fulano\Downloads`; os de gravação usam `t.TempDir()`).
  - `TestBase`: `C:\Users\fulano\Downloads\processo.zip`, `processo.ZIP`, `processo.pdf`, `processo.PDF` dão `...\processo.txt`; `a.b.pdf` dá `...\a.b.txt`.
  - `TestNomeLote`: três nomes `5000000-00.2026.8.26.0001 - Evento 3 - INIC1.pdf`, `... - Evento 12 - SENT1.pdf`, `... - Evento 7 - LAUDO1.pdf` (em `dir1`) dão `dir1\5000000-00.2026.8.26.0001.txt`; com um deles trocado para `5000000-00.2026.8.26.0002 ...` dá `Base(pdfs[0])`; sem número em nenhum, `Base(pdfs[0])`; um PDF só com CNJ no nome dá `Base` dele (não o CNJ); PDFs em `dir1` e `dir2`, com o primeiro em `dir2`, dão o TXT em `dir2` (e3); lista vazia dá `""`.
  - `TestGravarSemSobrescrever_NomeLivre` (grava `processo.txt`, conteúdo conferido), `_BaseOcupadaGrava2` (com `processo.txt` existente grava `processo_2.txt`), `_2e3OcupadosGrava4`, `_ZipAoLadoGrava2` (só `processo.zip` na pasta, sem `processo.txt`: grava `processo_2.txt`), `_TudoOcupadoDaErro` (base e `_2` a `_999` existentes: `errors.Is(err, ErrSemNomeLivre)` e nenhum `_1000`), `_NaoTocaNoExistente` (SHA-256 de `processo.txt` igual antes e depois).
  - `TestGravarSemSobrescrever_FalhaNaoDeixaArquivo` (Review Focus): base numa pasta inexistente devolve erro e não cria nada; e, trocando a escrita por uma que falha (variável de pacote `escrever = (*os.File).WriteString`, trocada no teste), o arquivo criado é removido e o erro volta.
  - `TestDaEntrada`: para `entrada = C:\Users\fulano\Downloads\processo.pdf`, aceita `...\processo.txt`, `...\processo_7.txt`, `...\PROCESSO.TXT`, `...\processo_999.txt`; recusa `D:\outra\processo.txt`, `processo_1.txt`, `processo_02.txt`, `processo_1000.txt`, `processo_abc.txt`, `outro.txt` e `""`.
  - `TestSaidaDoItem`: `("zip", ...\processo.zip, nil, ...\processo.txt)` verdadeiro e com `processo_2.txt` falso; `("pdf", ...\processo.pdf, [esse], processo.txt)` e `processo_3.txt` verdadeiros; `("pdf", ...\processo.pdf, nil, processo_2.txt)` verdadeiro (sem `paths`, vale o `path`); item de três PDFs com CNJ comum aceita `<CNJ>.txt` e `<CNJ>_2.txt` na pasta do primeiro e recusa `<nome do primeiro PDF>.txt`; saída vazia falsa; `kind` diferente de `"zip"` e `"pdf"` falso.
- [ ] **Passo 2: rodar** `go test ./internal/saida -v`: falham (esqueleto com `panic("falta")`).
- [ ] **Passo 3: implementar.**
  - CNJ: `regexp.MustCompile(`\d{7}-\d{2}\.\d{4}\.\d\.\d{2}\.\d{4}`)`, primeira ocorrência no `filepath.Base` de cada nome; comum só se todos tiverem e forem iguais.
  - `GravarSemSobrescrever`: candidatos `base`, depois `<nome>_2.txt` a `<nome>_999.txt`; a base conta como ocupada também se `<nome>.zip` existir (`os.Stat`); cada candidato é aberto com `O_WRONLY|O_CREATE|O_EXCL`, e `fs.ErrExist` passa ao próximo; outro erro volta na hora. Falha de escrita ou de `Close` remove o arquivo criado.
  - `DaEntrada(entrada, s)` = mesma pasta (`strings.EqualFold` de `filepath.Clean(filepath.Dir(...))`) e nome `<radical>.txt` ou `<radical>_N.txt` sem caixa, N decimal de 2 a 999 sem zero à esquerda. Uma função interna `daBase(base, s string) bool` faz a conferência contra uma base; `SaidaDoItem` com `kind` `"pdf"` segue a regra: `len(paths) > 1` usa `daBase(NomeLote(paths), s)`, senão `DaEntrada(path, s)`. ZIP compara `filepath.Clean` de `s` e `Base(path)` com `strings.EqualFold`.
- [ ] **Passo 4: rodar** os testes e o gate Go (regras).
- [ ] **Passo 5: commit** `feat(saida): regra única do nome do TXT de saída`.

---

### Task 2: ProcessFile com Abrir e Tamanho, ParseNome e LerEntrada

**Files:**
- Modify: `internal/models/models.go:24-32` (`ProcessFile`)
- Modify: `internal/etl/sorter.go:37-79` (`ParseFileInfo`, `SortProcessFiles`)
- Create: `internal/etl/entrada.go`
- Test: `internal/etl/sorter_nome_test.go`, `internal/etl/entrada_test.go`

**Interfaces:**
- Produces:
  - `models.ProcessFile` ganha `Abrir func() (io.ReadCloser, error)`, `Tamanho int64` e `Avulso bool`, todos com `json:"-"`. `ZipFileRef` fica.
  - `func ParseNome(nome string) models.ProcessFile` (preenche `Name = nome`, `DocName = filepath.Base(nome)`, `EventNum`, `DocNum`, `Extension`); `ParseFileInfo(f)` vira `ParseNome(f.Name)` mais `ZipFileRef: f`, `Abrir: f.Open`, `Tamanho: int64(f.UncompressedSize64)`.
  - `func ordenarPorEvento(caminhos []string) []string` (em `sorter.go`): cópia da lista na ordem de `SortProcessFiles` sobre `ParseNome(filepath.Base(c))`, estável.
  - `func LerEntrada(caminhos []string) ([]models.ProcessFile, io.Closer, error)`.
  - `var ErrTipoNaoSuportado = errors.New("tipo de arquivo não suportado")`.
  - `type erroEntrada struct{ frase string; err error }` com `Error()` = `frase` e `Unwrap()` = `err`; frases `"arquivo não encontrado"`, `"arquivo vazio"` e, para falha ao abrir, `textoErroSaida(err)`.

- [ ] **Passo 1: testes.**
  - `TestParseNome`: `5000000-00.2026.8.26.0001 - Evento 12 - SENT1.pdf` dá evento 12, documento 1, `DocName` igual ao nome, `Extension` `.pdf`; `anexo.pdf` dá `noEventNum` e `noDocNum`; `Capa do processo.pdf` dá evento 0; `C:\Users\fulano\x\Evento 3 - INIC2.PDF` dá `DocName` `Evento 3 - INIC2.PDF` e `Extension` `.pdf`.
  - `TestSortProcessFiles_EstavelComNomesIguais`: dois `ProcessFile` com o mesmo `DocName` e `Name` diferentes ficam na ordem de entrada, nas duas ordens.
  - `TestOrdenarPorEvento`: `[... Evento 12 ..., anexo.pdf, ... Evento 3 ...]` volta `[Evento 3, Evento 12, anexo]`, e a lista de entrada não muda.
  - `TestLerEntrada_ExtensaoNaoSuportada` (`.txt`, `.docx`), `_MisturaZipEPDF`, `_ListaVazia` e `_DoisZips`: `errors.Is(err, ErrTipoNaoSuportado)`.
  - `TestLerEntrada_Zip`: um `.ZIP` gravado com `gravarZip` devolve as entradas e um `Closer` que é `*zip.ReadCloser`; cada `ProcessFile` tem `Abrir` e `Tamanho` iguais aos da entrada.
  - `TestLerEntrada_PDFs`: três PDFs (um `.PDF`) devolvem três `ProcessFile` com `Avulso`, `Name` = `DocName` = nome base, `Tamanho` = tamanho em disco, `Abrir` lendo os mesmos bytes, na ordem recebida; o `Closer` fecha sem erro.
  - `TestLerEntrada_UmPDFInexistente`: erro `"arquivo não encontrado"`, `errors.Is(err, fs.ErrNotExist)`, sem a pasta no texto. `_UmPDFVazio`: `"arquivo vazio"`.
  - `TestLerEntrada_DiretorioComNomePDF` (Review Focus): uma pasta `x.pdf` sozinha dá `"arquivo não encontrado"`; junto de um PDF bom, fica na lista com `Tamanho` 0 e sem erro.
  - `TestLerEntrada_UmPDFTravado`: arquivo aberto no teste com `windows.CreateFile(..., GENERIC_READ, 0 /* dwShareMode */, ...)` dá erro com texto exato `"falha de leitura ou gravação em disco"` e sem caminho.
  - `TestLerEntrada_VariosComUmInexistente`: o ausente fica na lista (`Tamanho` 0), os outros normais, sem erro. `_VariosTodosInexistentes`: erro `"arquivo não encontrado"`.
- [ ] **Passo 2: rodar** `go test ./internal/etl -run "TestParseNome|TestSortProcessFiles_Estavel|TestOrdenarPorEvento|TestLerEntrada" -v`: falham.
- [ ] **Passo 3: implementar.** `LerEntrada`: classifica pela extensão (`strings.EqualFold`); um `.zip` chama `ReadZipFiles`; uma lista só de `.pdf` confere cada arquivo com `os.Stat` (diretório conta como não encontrado), uma abertura de prova (`os.Open` e `Close`) e o tamanho. Com um PDF só, a primeira falha vira o erro do item; com vários, o erro só sai se nenhum PDF passar nas três conferências (vale a frase do primeiro na lista); os que falharam ficam na lista e terminam no worker pelo caminho de hoje. `Abrir` de cada PDF é um `os.Open` do caminho, `Closer` é um `io.NopCloser`-equivalente que não faz nada. Sem conferir `%PDF`.
- [ ] **Passo 4: rodar** os testes e o gate Go (`TestReadZipFiles` e os da ordenação seguem passando).
- [ ] **Passo 5: commit** `feat(etl): entrada de PDF avulso com Abrir e Tamanho`.

---

### Task 3: ProcessarEntrada no pipeline, gravação pela saida e relógio do formatter

**Files:**
- Modify: `internal/etl/pipeline.go:207-251` (`extrairPDF`), `:304-337` (`ProcessZip`, `ProcessZipComAtividade`), `:339-874` (`processarZip`, que vira `processarEntrada`)
- Modify: `internal/etl/formatter.go:496`
- Test: `internal/etl/pdf_avulso_test.go`

**Interfaces:**
- Consumes: `saida.Base`, `saida.NomeLote`, `saida.Gravar`, `saida.GravarSemSobrescrever`, `saida.SaidaDoItem` (Task 1); `LerEntrada`, `ordenarPorEvento`, `ProcessFile.Abrir/Tamanho/Avulso`, `ErrTipoNaoSuportado` (Task 2).
- Produces:
  - `type Entrada struct { Kind string; Caminhos []string; SaidaAnterior string }` (em `pipeline.go`; `Kind` `"zip"` ou `"pdf"`, vazio vale o que `LerEntrada` reconhecer).
  - `func (p *Pipeline) ProcessarEntrada(ctx context.Context, e Entrada, cfg *models.Config, onProgress func(progress float64, message string, processedDocs, totalDocs, processedPages, totalPages int), atv *Atividade) (string, int, int, int, error)`.
  - `ProcessZip` e `ProcessZipComAtividade` mantêm a assinatura e chamam `ProcessarEntrada` com `Entrada{Kind: "zip", Caminhos: []string{zipPath}}`.
  - `var agoraFormatter = time.Now` (em `formatter.go`).

- [ ] **Passo 1: testes** (todos com `isolarTemp`, `OmitirTipos` vazio salvo o da omissão, `agoraFormatter` fixo em 26/11/2026 10:00:00 e restaurado no `t.Cleanup`; nome base `5000000-00.2026.8.26.0001 - Evento 12 - SENT1.pdf`).
  - `TestFormatter_RelogioFixo`: dois `GerarSaida` do mesmo mapa dão bytes iguais e contêm `Data de Consolidação: 26/11/2026 10:00:00`.
  - `TestProcessarEntrada_EquivaleAoZip`: para cada PDF, o TXT do avulso é igual, byte a byte, ao de um ZIP (`gravarZipComo`) com só aquele PDF, com o mesmo nome, na raiz. Casos: `scripts/bench/fixtures/nativo.pdf`, `vetorial.pdf` e `escaneado.pdf` (OCR real; pula com `-short`), `pdfVersao20(objetosTexto())`, `pdfEmEnvelope(buildPDF(objetosTexto()))`, `pdfTruncado()` (tabela inconsistente), `pdfAESV2(objetosTexto(), "")` e `pdfAESV2(objetosTexto(), "segredo")` (senha); os quatro gerados usam `trocarPoppler(t, ocr.InfoPDF, ocr.TextoPDF)` e pulam com `-short`.
  - `TestProcessarEntrada_VariosPDFsEquivaleAoZip`: `... Evento 12 - SENT1.pdf`, `anexo.pdf` (sem evento), `... Evento 3 - INIC1.pdf` e `... Evento 7 - LAUDO1.pdf` protegido por senha, passados nessa ordem (fora da ordem), dão o mesmo TXT de um ZIP com os quatro; o protegido aparece como `[documento não lido: protegido por senha; consulte o PDF original]` e o cabeçalho diz `Documentos não lidos: 1`.
  - `TestProcessarEntrada_OmissaoNaoValeParaAvulso`: com `OmitirTipos: ["PROC"]`, o avulso `... Evento 1 - PROC1.pdf` tem o texto do PDF no TXT e nenhuma chave `omitido`; o mesmo PDF num ZIP continua omitido.
  - `TestProcessarEntrada_MarcadorDoAvulso`: lote de três com um apagado depois de `LerEntrada` (pelo gancho `aoFimDaPreContagem`, que apaga o arquivo) termina com esse documento em `Leitura` `ilegivel` (via `atv.Instantaneo()`), o TXT com `[ERRO ao abrir PDF: falha de leitura ou gravação em disco]` e sem a pasta do teste; um de 0 bytes num lote termina `ilegivel` com `[ERRO ao analisar PDF: `; num ZIP com uma entrada `Store` cujo dado foi corrompido depois de gravado (CRC errado), o marcador continua `[ERRO ao extrair PDF do ZIP: `.
  - `TestProcessarEntrada_NomeDaSaida`: um PDF com `processo.txt` já na pasta grava `processo_2.txt` e devolve esse caminho, com `processo.txt` intacto (SHA-256); lote com PDFs em `dir1` e `dir2`, o primeiro por evento em `dir2`, grava em `dir2`.
  - `TestProcessarEntrada_LoteNaoTomaOTxtDoZip` (Review Focus): pasta com `<CNJ>.zip` e `<CNJ>.txt`; lote de três PDFs do mesmo CNJ grava `<CNJ>_2.txt` e o `<CNJ>.txt` fica com o mesmo SHA-256.
  - `TestProcessarEntrada_SaidaAnterior` (e1): com `processo.txt` alheio e `SaidaAnterior` = `processo_2.txt` existente, grava por cima de `processo_2.txt` e não cria `processo_3.txt`; com `SaidaAnterior` apagada, escolhe nome livre (`processo_2.txt` de novo); com `SaidaAnterior` em outra pasta (`D:` sintético ou outro `t.TempDir()`) ou com outro nome (`outro.txt`), ela é ignorada e fica intacta, e a gravação escolhe nome livre.
  - `TestProcessarEntrada_ZipSobrescreveOProprioTxt` (guarda de regressão): ZIP com `processo.txt` antigo grava por cima dele, no mesmo caminho, sem `_2`.
  - `TestProcessarEntrada_KindIncompativel`: `Entrada{Kind: "zip", Caminhos: [x.pdf]}` e `Kind: "pdf"` com um `.zip` dão `errors.Is(err, ErrTipoNaoSuportado)` e etapa `EtapaAbrirZip` (`errors.AsType[ErroEtapa]`), sem TXT.
- [ ] **Passo 2: rodar** `go test ./internal/etl -run "TestFormatter_RelogioFixo|TestProcessarEntrada" -v`: falham (esqueleto de `ProcessarEntrada` com `panic("falta")`).
- [ ] **Passo 3: implementar.**
  - `processarEntrada` recebe a `Entrada` e começa por `LerEntrada(e.Caminhos)`. O tipo reconhecido é `"zip"` quando o `Closer` é `*zip.ReadCloser`, senão `"pdf"`. Com `e.Kind != ""` diferente do reconhecido, fecha o `Closer` e devolve `comEtapa(EtapaAbrirZip, ErrTipoNaoSuportado)` antes de ler qualquer documento. `rc.RegisterDecompressor` só quando o `Closer` for `*zip.ReadCloser`; a mensagem `no PDF or HTML documents found in ZIP` só para ZIP; erros de `LerEntrada` com `EtapaAbrirZip`. Os atributos `zip` das linhas de log usam `filepath.Base(e.Caminhos[0])`; `bytes` do "zip iniciado" soma os tamanhos.
  - `extrairPDF` e o caminho do HTML usam `f.Abrir()` e `f.Tamanho`; `encerrar` registra `bytes` de `f.Tamanho`.
  - `tiposOmitidos[i]` fica `""` quando `f.Avulso`. O marcador de falha de extração é `[ERRO ao abrir PDF: %s]` quando `f.Avulso`.
  - Gravação: ZIP em `saida.Gravar(saida.Base(e.Caminhos[0]), texto)`; PDF em `saida.Gravar(e.SaidaAnterior, texto)` quando `e.SaidaAnterior != ""`, `saida.SaidaDoItem("pdf", ordenados[0], ordenados, e.SaidaAnterior)` e o arquivo existe; senão `saida.GravarSemSobrescrever(saida.NomeLote(ordenados), texto)`, com `ordenados = ordenarPorEvento(e.Caminhos)`. Erros com `EtapaGravar`.
  - `formatter.go:496` lê `agoraFormatter()`.
- [ ] **Passo 4: rodar** os testes, `go test -race ./internal/etl` sem `-short` e o gate Go. As cerca de 40 chamadas a `ProcessZip`/`ProcessZipComAtividade` nos testes e em `scripts/bench` não mudam.
- [ ] **Passo 5: commit** `feat(etl): pipeline lê PDF avulso e grava sem sobrescrever`.

---

### Task 4: original preservado, temporários e arquivo trocado no meio

**Files:**
- Modify: `internal/etl/pipeline.go` (`extrairPDF`, a parte do avulso)
- Test: `internal/etl/pdf_avulso_original_test.go`

**Interfaces:**
- Consumes: `ProcessarEntrada`, `Entrada` (Task 3); ganchos de teste existentes `contarPaginasPDF`, `executarOCR`, `executarOCRPaginas`, `aoFimDaPreContagem`, `trocarPoppler`, `isolarTemp`, `semTemporarios`, `pdfAcimaDoTeto`.
- Produces: `var errArquivoMudou = errors.New("o arquivo mudou durante a leitura")` (ver a divergência da frase: é a única linha a trocar se o dono decidir outra).

- [ ] **Passo 1: testes.**
  - `TestProcessarEntrada_OriginalPreservado`: subtestes `sucesso` (nativo), `erro` (`pdfTruncado()` sem Poppler: documento ilegível), `cancelamento` (ctx cancelado no `onProgress` da primeira página), `panic` (`contarPaginasPDF` que entra em panic), `prazo` (`executarOCR` e `executarOCRPaginas` devolvendo `ocr.ErrPrazo`, com `buildPDF(objetosDigitalizado(t))`, que a biblioteca abre e manda ao OCR, e OCR ligado), `poppler` (`pdfEmEnvelope` com `trocarPoppler` real; pula com `-short`) e `acima_do_teto` (`pdfAcimaDoTeto`, que passa pelo temporário em disco). Em cada um: o original existe e tem o mesmo SHA-256 de antes, e `semTemporarios(t, tmp)` com `TMP` e `TEMP` apontando para o `t.TempDir()` de `isolarTemp`.
  - `TestProcessarEntrada_ArquivoTrocadoNoMeio`: num lote de dois PDFs, o gancho `aoFimDaPreContagem` troca o primeiro por um maior, por um menor ou por um de 0 bytes, ou o apaga. Maior, menor e 0 bytes: documento `ilegivel` com `[ERRO ao abrir PDF: o arquivo mudou durante a leitura]`; apagado: `[ERRO ao abrir PDF: falha de leitura ou gravação em disco]`. Nenhum caminho do teste no TXT; o outro PDF lido. Os três casos também com o PDF acima do teto (temporário em disco), e nenhum temporário sobra.
  - `TestProcessarEntrada_ArquivoTravadoNoLote`: um dos dois PDFs travado com `windows.CreateFile` (`dwShareMode` 0) depois de `LerEntrada` (pelo gancho) termina `ilegivel` com `falha de leitura ou gravação em disco`; o outro lido.
- [ ] **Passo 2: rodar** `go test ./internal/etl -run "TestProcessarEntrada_(OriginalPreservado|ArquivoTrocadoNoMeio|ArquivoTravadoNoLote)" -v`: os de tamanho falham (hoje sai `unexpected EOF` ou o PDF truncado); os de preservação podem já passar depois da Task 3, e nesse caso são guardas de regressão: quebre de propósito numa cópia local (`fontePDF{caminho: <caminho do usuário>}` no avulso, por exemplo) e registre no relatório o que cada um pegou.
- [ ] **Passo 3: implementar.** Só para `f.Avulso`: na leitura para memória, `io.ErrUnexpectedEOF`, `io.EOF` (arquivo que encolheu para 0 bytes) e o byte além de `f.Tamanho` viram `errArquivoMudou`; na cópia para o temporário, `n != f.Tamanho` remove o temporário e devolve `errArquivoMudou`. ZIP não muda (o `archive/zip` confere tamanho e CRC, e a frase "entrada do ZIP maior que o tamanho declarado" fica).
- [ ] **Passo 4: rodar** os testes, `go test -race ./internal/etl` sem `-short` e o gate Go.
- [ ] **Passo 5: commit** `fix(etl): PDF avulso trocado durante a leitura e original preservado`.

---

### Task 5: fila com itens de PDF

**Files:**
- Modify: `internal/etl/queue.go:23-67` (`QueueItem`), `:99-100` (`processar`), `:243-288` (`AddFilesComOrigem`), `:306-331` (`copiarItem`), `:333-391` (`Reprocessar`), `:626-792` (`processItem`)
- Modify (fakes de `processar`): `internal/etl/queue_test.go`, `queue_eventos_test.go` (inclui o tipo `processarFunc`, linha 44), `queue_fila_test.go` (`procFalso.processar`, linha 43, e o fake da linha 234), `queue_nao_lidos_test.go`, `queue_notificacao_test.go`, `queue_relatorio_test.go`, `queue_reprocessar_test.go`, `queue_retida_test.go`, `queue_telemetria_test.go`. Confira com `grep -n "processar" internal/etl/queue_*_test.go` que nenhum ficou de fora.
- Test: `internal/etl/queue_pdf_test.go`

**Interfaces:**
- Consumes: `Entrada`, `ProcessarEntrada` (Task 3); `ordenarPorEvento` (Task 2); `saida.NomeLote` (Task 1).
- Produces:
  - `QueueItem.Kind string` (`json:"kind"`, `"zip"` ou `"pdf"`), `QueueItem.Paths []string` (`json:"paths,omitempty"`, na ordem por evento, vazio para ZIP), `QueueItem.saidaAnterior string` (não exportado). `Path` do item de PDF é `Paths[0]`.
  - `processar func(ctx context.Context, e Entrada, cfg *models.Config, onProgress func(float64, string, int, int, int, int)) (string, int, int, int, error)`.
  - `func (q *Queue) ReprocessarMonitorado(path string) bool`.
  - `func nomeItemPDF(base string, n int) string`: `base` com ` (N PDFs)` quando `n > 1`.

- [ ] **Passo 1: testes** (fila com `processar` falso, `Reter` quando o teste precisa do item `pending`).
  - `TestAddFiles_RecusaExtensoes`: `a.txt`, `b.exe`, `c.docx` dão 0 e nenhum item; `d.pdf` e `E.PDF` entram (um item de dois PDFs, retorno 2).
  - `TestAddFiles_DoisZipsETresPDFs`: uma chamada com dois ZIPs e três PDFs devolve 5 e cria três itens: dois `Kind` `"zip"` com `Paths` nil e um `Kind` `"pdf"` com os três em `Paths` na ordem por evento e `Path` = `Paths[0]`.
  - `TestAddFiles_PDFJaPendenteSaiDoItemNovo`: com `a.pdf` num item pendente, `[a.pdf, b.pdf]` devolve 1 e cria um item só com `b.pdf`; `[A.PDF]` (outra caixa) devolve 0 e não cria item.
  - `TestAddFiles_MesmoPDFRepetidoNaChamada` (Review Focus): `[a.pdf, a.pdf, A.pdf]` devolve 1, `Paths` com um caminho.
  - `TestAddFiles_NomeProvisorio`: três PDFs com CNJ comum dão `Name` `5000000-00.2026.8.26.0001 (3 PDFs)`; um `processo.pdf` dá `processo`.
  - `TestCopiarItem_KindEPaths`: a cópia (`GetItems` e o `queue:update`) leva `Kind` e `Paths`; mudar `Paths` da cópia não muda o item; o JSON tem `"kind"` e `"paths"`.
  - `TestProcessItem_EntradaPassadaAoProcessar`: item ZIP chega como `Entrada{Kind: "zip", Caminhos: [path]}`; item de PDF como `Kind` `"pdf"` e `Caminhos` = `Paths`; sem reprocessamento, `SaidaAnterior` vazia.
  - `TestProcessItem_NomeTrocadoPeloTXT` (com Review Focus): item de um PDF cujo `processar` devolve `<dir>\processo_2.txt` termina com `Name` `processo_2`, e o último `queue:update` já traz esse nome; item de três PDFs devolvendo `<CNJ>.txt` termina `<CNJ> (3 PDFs)`; dois itens de dois PDFs do mesmo lote, cujos fakes devolvem `<CNJ>.txt` e `<CNJ>_2.txt`, terminam `<CNJ> (2 PDFs)` e `<CNJ>_2 (2 PDFs)`; item ZIP mantém o `Name` de antes.
  - `TestReprocessar_PassaSaidaAnterior` (e1): depois de um sucesso com `OutputPath` X, `Reprocessar` deixa `OutputPath` vazio na cópia enviada e o `processar` falso recebe `SaidaAnterior` X.
  - `TestReprocessar_CanceladoMantemSaidaAnterior` (e1): depois de um sucesso com `OutputPath` X, um `Reprocessar` cancelado no meio e um segundo `Reprocessar` fazem o `processar` falso receber `SaidaAnterior` X nas duas execuções. Depois de um sucesso, um terceiro `Reprocessar` recebe o `OutputPath` novo.
  - `TestReprocessar_PDFComOrigemApagada`: item de dois PDFs com o segundo apagado devolve `arquivo de origem não encontrado: <nome base do PDF>` e o item fica como estava (`success`, mesmo `OutputPath`, mesma posição).
  - `TestReprocessarMonitorado_ItemConcluidoDeUmPDF` (q1): item `success` de um PDF; `ReprocessarMonitorado(<mesmo caminho em maiúsculas>)` devolve `true`, o item fica `pending` no mesmo índice, nenhum item novo, e o `processar` falso recebe `SaidaAnterior` = `OutputPath` de antes e a config global (sem `ajuste`).
  - `TestReprocessarMonitorado_DevolveFalso`: sem item; item `error`; item `cancelled`; o PDF dentro de um item de vários PDFs `success`; e item ZIP `success` com o mesmo `Path`: `false` e `GetItems` igual antes e depois.
- [ ] **Passo 2: rodar** `go test ./internal/etl -run "TestAddFiles_|TestCopiarItem_KindEPaths|TestProcessItem_|TestReprocessar_|TestReprocessarMonitorado_" -v`: falham.
- [ ] **Passo 3: implementar.**
  - `AddFilesComOrigem`: descarta extensão fora de `.zip`/`.pdf` com `slog.Info("arquivo recusado: extensão não aceita", "arquivo", <nome base>)`; cada ZIP como hoje, com `Kind` `"zip"`; os PDFs, sem repetidos na chamada e sem os que já estão em `Paths` de item `pending`/`processing` (`strings.EqualFold`), ordenados por `ordenarPorEvento`, viram um item `Kind` `"pdf"` com `Name` = `nomeItemPDF(<radical de saida.NomeLote(paths)>, len(paths))`, entrando depois dos ZIPs da mesma chamada (fica no topo). Devolve ZIPs que entraram mais PDFs do item novo.
  - `copiarItem` copia `Kind` e `slices.Clone(Paths)`.
  - `processItem` monta a `Entrada` (`Kind` vazio vale `"zip"`, para itens de teste antigos; `Caminhos` = `Paths` ou `[Path]`; `SaidaAnterior` lida sob `q.mu`, sem zerar); `item.saidaAnterior` só é zerada no sucesso (o novo `OutputPath` assume o papel); o padrão de `processar` chama `q.pipeline.ProcessarEntrada(ctx, e, cfg, onProgress, atv)`; no sucesso de item `"pdf"`, `Name` = `nomeItemPDF(<radical de outputPath>, len(Paths))` antes do broadcast; `bytesZip` soma os `os.Stat` de `Caminhos`.
  - `Reprocessar`: para `Kind` `"pdf"`, confere todos os `Paths` com `os.Stat` antes de mexer no item; grava `item.saidaAnterior = item.OutputPath` antes de zerar `OutputPath`, só quando `OutputPath != ""`: um item cancelado ou com erro mantém a `saidaAnterior` que já tinha. A zeragem vira uma função `reabrirLocked(item *QueueItem)`, que começa por `pararEnvioPendente(item)` e faz essa cópia condicional da `saidaAnterior`. `ReprocessarMonitorado` também a usa, que não move o item nem cria `ajuste` (`item.ajuste = nil`, `OcrMode`/`CleaningLevel`/`ForcarOCR` da execução os corrige) e chama `startProcessing` depois do broadcast.
  - Fakes antigos: trocar `zipPath string` por `e Entrada` e o uso de `zipPath` por `e.Caminhos[0]`, sem mudar o que cada teste confere.
- [ ] **Passo 4: rodar** os testes, `go test -race ./internal/etl` sem `-short` e o gate Go.
- [ ] **Passo 5: commit** `feat(fila): itens de PDF, saída anterior e reprocessamento do PDF monitorado`.

---

### Task 6: MonitorarPDF na config e no watchdog

**Files:**
- Modify: `internal/models/models.go:6-22` (`Config`), `internal/config/config.go:40-50` (`DefaultConfig`), `:92-103` (`configOverlay`), `:105-` (`LoadFromFile`)
- Modify: `internal/watchdog/watchdog.go:17-112` (`Watcher`, `Start`), `:145-173` (`watchLoop`), `:176-219` (`handleNewFile`); `internal/watchdog/pendentes.go:15-54`
- Modify: `app.go:103-107` (callback do watcher), `:342-360` (`aplicarMonitor`); `app_atualizacao.go:99`
- Modify (chamadas antigas): `internal/watchdog/watchdog_test.go`, `falhas_test.go`, `pendentes_test.go`
- Test: `internal/config/config_monitorar_pdf_test.go`, `internal/watchdog/pdf_test.go`, `app_monitorado_test.go`

**Interfaces:**
- Consumes: `saida.Base`, `saida.DaEntrada` (Task 1); `Queue.ReprocessarMonitorado`, `Queue.AddFilesComOrigem` (Task 5).
- Produces:
  - `models.Config.MonitorarPDF bool` (`json:"monitorar_pdf"`).
  - `func watchdog.Extensoes(monitorarPDF bool) []string`: `[".zip"]` ou `[".zip", ".pdf"]`.
  - `func (w *Watcher) Start(path string, extensoes []string) error` (nil ou vazio vale `[".zip"]`).
  - `func pendingZips(dir string, since time.Time, extensoes []string, ignorar func(string, time.Time) bool) ([]string, error)`.
  - `func jaConvertido(caminho string, mod time.Time) bool` (em `pendentes.go`).
  - Em `main`: `type filaMonitorada interface { ReprocessarMonitorado(string) bool; AddFilesComOrigem([]string, string) int }` e `func encaminharMonitorado(f filaMonitorada, caminho string)`.

- [ ] **Passo 1: testes.**
  - `TestLoadFromFile_MonitorarPDFAusenteCarregaFalse` (JSON antigo sem a chave), `TestDefaultConfig_MonitorarPDFDesligado` e `TestSaveLoad_MonitorarPDFLigado` (ida e volta com `true`).
  - `TestJaConvertido`: ZIP com `<nome>.txt` igual ou mais novo é convertido, mais velho não; PDF com `<nome>_2.txt` mais novo é convertido; PDF com `<nome>.txt` mais novo e sem `<nome>.zip` é convertido; com `<nome>.zip` ao lado, `<nome>.txt` não conta (é do ZIP) e o PDF não é convertido; `<nome>_2.txt` mais velho que o PDF não conta. Datas com `os.Chtimes`.
  - `TestPendingZips_PDF`: com `[".zip"]` nenhum PDF sai; com `[".zip", ".pdf"]` sai o PDF pendente, não sai o convertido por `_2`, sai o que só tem `<nome>.txt` do ZIP homônimo, não sai `eprocia_pdf_123.pdf`; ordem do mais antigo ao mais novo, ZIPs e PDFs juntos.
  - `TestWatcher_PDFIgnoradoComOpcaoDesligada`: com `Start(dir, Extensoes(false))`, um PDF novo não chama o callback (espera de 1,5 s) e um ZIP chama.
  - `TestWatcher_PDFsSeparados` (e4): com a opção ligada, dois PDFs gravados com 1 s de diferença chamam o callback duas vezes, um caminho por vez; `eprocia_pdf_1.pdf` não chama.
  - `TestHandleNewFile_PDFJaConvertido`: PDF com TXT mais novo não chama o callback (dois `handleNewFile` seguidos); ZIP nas mesmas condições chama; PDF salvo de novo depois do TXT (mtime mais novo) chama.
  - `TestWatcher_StartMesmasExtensoes`: `Start(dir, [".zip"])` duas vezes mantém o mesmo `w.watcher` (ponteiro igual); `Start(dir, [".zip", ".pdf"])` troca o ponteiro e a varredura inicial roda de novo (um ZIP pendente chama o callback outra vez).
  - `TestEncaminharMonitorado` (em `main`, fila falsa): com `ReprocessarMonitorado` verdadeiro, `AddFilesComOrigem` não é chamado; com falso, é chamado uma vez com `[caminho]` e `"monitorada"`, depois de `ReprocessarMonitorado`.
- [ ] **Passo 2: rodar** `go test ./internal/config ./internal/watchdog . -run "MonitorarPDF|TestJaConvertido|TestPendingZips_PDF|TestWatcher_|TestHandleNewFile_PDF|TestEncaminharMonitorado" -v`: falham.
- [ ] **Passo 3: implementar.**
  - Config: campo, padrão `false` em `DefaultConfig`, `*bool` no overlay.
  - `Start`: guarda as extensões normalizadas (minúsculas, sem repetição) e só é no-op com a mesma pasta e o mesmo conjunto; a varredura inicial chama `pendingZips(path, ..., extensoes, w.falhas.Ignorar)`; o log de pendentes diz "arquivos pendentes".
  - `jaConvertido`: ZIP pela regra de hoje (`saida.Base`); PDF lendo a pasta uma vez (`os.ReadDir`) e aceitando um `.txt` com `saida.DaEntrada` e mtime igual ou mais novo, salvo o `saida.Base` exato quando existe `<nome>.zip`. `pendingZips` passa a usá-la e ignora PDFs com prefixo `eprocia_pdf_` (sem caixa).
  - `watchLoop` filtra pelas extensões do `Start` (passadas por argumento, como `watcher` e `done`) e ignora `eprocia_pdf_*`. `handleNewFile`, depois de o tamanho estabilizar e o arquivo abrir, descarta o PDF com `jaConvertido` verdadeiro (`slog.Debug`).
  - `app.go`: o callback de `NewApp` chama `encaminharMonitorado(app.queue, filePath)` (com `app.queue` não nil); `aplicarMonitor` e `app_atualizacao.go:99` passam `watchdog.Extensoes(cfg.MonitorarPDF)`.
  - Testes antigos: `Start(dir)` vira `Start(dir, nil)`; `pendingZips(dir, desde, f)` vira `pendingZips(dir, desde, []string{".zip"}, f)`.
- [ ] **Passo 4: rodar** os testes e o gate Go.
- [ ] **Passo 5: commit** `feat(watchdog): converter PDFs novos da pasta monitorada, desligado por padrão`.

---

### Task 7: SelectFiles, cópia pela saida do item e bindings

**Files:**
- Modify: `app.go:418-435` (`lerSaidaParaCopiar`), depois de `:463-477` (novo `SelectFiles`)
- Modify: `app_saida_test.go:27-30` (itens com `Kind` e `Path`)
- Generated: `frontend/src/wailsjs/go/main/App.js`, `App.d.ts`, `frontend/src/wailsjs/go/models.ts` (por `wails generate module`)
- Test: `app_saida_test.go`, `app_selecao_test.go`

**Interfaces:**
- Consumes: `saida.SaidaDoItem` (Task 1); `QueueItem.Kind/Paths` (Task 5).
- Produces:
  - `func (a *App) SelectFiles() ([]string, error)`: `runtime.OpenMultipleFilesDialog` com título `Selecionar arquivos ZIP do Eproc ou PDF` e `filtrosSelecao()`.
  - `func filtrosSelecao() []runtime.FileFilter`.
  - Binding JS `SelectFiles()`; `etl.QueueItem` com `kind` e `paths` em `models.ts`.

- [ ] **Passo 1: testes.**
  - `TestFiltrosSelecao`: exatamente `{"ZIP do Eproc e PDF (*.zip;*.pdf)", "*.zip;*.pdf"}`, `{"Somente ZIP (*.zip)", "*.zip"}`, `{"Somente PDF (*.pdf)", "*.pdf"}`, nessa ordem.
  - `TestLerSaidaParaCopiar_SaidaDoItem`: item `pdf` de três PDFs com CNJ comum e `OutputPath` `<CNJ>.txt` é copiado; item `pdf` de um PDF com `processo_2.txt` é copiado; item `zip` `a.zip` com `OutputPath` `b.txt` (não é a saída dele) é recusado; item `success` sem `Kind` é recusado.
  - `TestLerSaidaParaCopiar_SoAceitaTxtDeItemConcluido` (existente): os itens ganham `Kind: "zip"` e `Path` `processo.zip`/`segredo.zip`; as asserções não mudam.
- [ ] **Passo 2: rodar** `go test . -run "TestFiltrosSelecao|TestLerSaidaParaCopiar" -v`: falham.
- [ ] **Passo 3: implementar.** `lerSaidaParaCopiar` só aceita o item `success` cujo `OutputPath` bate com o caminho pedido (como hoje) e passa em `saida.SaidaDoItem(it.Kind, it.Path, it.Paths, it.OutputPath)`. `SelectZipFiles` e `SelectZipFile` ficam. Depois, `wails generate module` (regras: CRLF e o que entra no commit).
- [ ] **Passo 4: rodar** os testes e o gate Go; conferir em `models.ts` os campos `kind` e `paths` de `QueueItem` e, em `App.d.ts`, `SelectFiles():Promise<Array<string>>`.
- [ ] **Passo 5: commit** `feat(app): seleção de ZIP e PDF e cópia conferida pela saída do item`.

---

### Task 8: frontend (soltar, selecionar, linha, Configurações) e capturas

**Files:**
- Modify: `frontend/src/lib/aviso.js:68-80`, `frontend/src/lib/textos.js`
- Modify: `frontend/src/App.svelte:32-35,52` (imports), `:213-236` (`OnFileDrop`), `:407-425` (`triggerManualSelect`), `:519` (texto do overlay)
- Modify: `frontend/src/components/ProcessList.svelte:1-20` (imports), `:596-602` (vazio), `:657` (nome)
- Modify: `frontend/src/components/Settings.svelte:46-54` (`PADRAO`), bloco da pasta monitorada (`:190-235`)
- Modify (testes que fixam o antigo): `frontend/avisos.test.mjs:160-171`, `frontend/monitoramento.test.mjs:151-152`
- Modify: `scripts/visual/estados.mjs`, `scripts/visual/estados.test.mjs`
- Create: `frontend/pdf-avulso.test.mjs`; `docs/revisao/capturas/2026-11-26-pdf-avulso/antes/` e `depois/` (PNGs)
- Modify: `DESIGN.md` §4 e §5.1 (ícone de tipo na linha), `PRD.md` RF01, RF02 e §5 passo 5

**Interfaces:**
- Consumes: binding `SelectFiles`, `AddDroppedFiles`, `AddFilesToQueue`; `QueueItem.kind`, `QueueItem.name` já com "(N PDFs)" (Tasks 5 e 7); `Config.monitorar_pdf` (Task 6).
- Produces:
  - `export function separarEntradas(caminhos)` → `{ aceitos: string[], ignorados: number }` (`separarZips` sai).
  - `export function avisoSoltura(aceitos, ignorados)`: `null` sem ignorados; sem aceitos, `{ texto: 'Arraste arquivos ZIP do Eproc ou PDF. Ignorados: N', tipo: 'error' }`; com aceitos, `{ texto: '1 arquivo ignorado: só ZIP do Eproc ou PDF' | 'N arquivos ignorados: só ZIP do Eproc ou PDF', tipo: 'aviso' }`.
  - Em `textos.js`: `ROTULO_MONITORAR_PDF = 'Também converter PDFs novos nesta pasta'`, `AJUDA_MONITORAR_PDF = 'Cada PDF que aparecer na pasta ganha um TXT ao lado, inclusive boletos e outros PDFs que não são de processo'`, `TIPO_ENTRADA = { zip: 'ZIP', pdf: 'PDF' }` (texto `.sr-only` do ícone).
  - Estado de captura `fila-pdf` em `scripts/visual/estados.mjs`.

- [ ] **Passo 1: capturas "antes".** Antes de mexer no frontend (o HEAD não mudou `frontend/` nem `scripts/visual/` desde a `main`; confira com `git diff --stat main HEAD -- frontend scripts/visual`), rode `node scripts/visual/capturar.mjs --saida docs/revisao/capturas/2026-11-26-pdf-avulso/antes --estados vazio,fila,configuracoes` e confira `[visual] N capturas em ...` e o Vite encerrado.
- [ ] **Passo 2: testes.**
  - `avisos.test.mjs`: `separarEntradas(['a.ZIP', 'b.pdf', 'c.zip', 'd.txt', 'e.PDF', 'f.docx'])` dá `{ aceitos: ['a.ZIP', 'b.pdf', 'c.zip', 'e.PDF'], ignorados: 2 }`; `separarEntradas(null)` dá `{ aceitos: [], ignorados: 0 }`; `avisoSoltura` com os três textos acima; no `App.svelte`, o bloco de `OnFileDrop` usa `separarEntradas(paths)`, chama `AddDroppedFiles(` uma vez só e `textoJaNaFila(aceitos.length)`; o `App.svelte` não cita `SelectZipFiles` nem `separarZips`, `triggerManualSelect` chama `SelectFiles()` e o overlay diz `Solte arquivos ZIP ou PDF em qualquer lugar para processar` (não `Solte o arquivo ZIP`). Hoje nenhum teste de `frontend/` fixa o texto antigo do overlay; se aparecer um, atualize-o.
  - `monitoramento.test.mjs:151-152`: o vazio tem `Selecionar arquivos...` (e não `Selecionar arquivos ZIP`) e `ou arraste ZIPs do Eproc ou PDFs para a janela`.
  - `pdf-avulso.test.mjs`: `ProcessList.svelte` importa `File` de `lucide-svelte/icons/file` e `FileArchive` de `lucide-svelte/icons/file-archive`, tem `const ICONE_TIPO = { zip: FileArchive, pdf: File }` e usa `<Icone icone={ICONE_TIPO[tipo]} tamanho={12}` dentro de `info-col`, antes de `file-name`, com `{TIPO_ENTRADA[tipo]}` em `.sr-only` (o formato `ICONE_TIPO[tipo]` é o que `icones.test.mjs` aceita para mapa); `nomeExibido('5000000-00.2026.8.26.0001 (3 PDFs)')` volta igual; `Settings.svelte` tem `monitorar_pdf: false` em `PADRAO`, um `input` `id="monitorar-pdf"` `type="checkbox"` `role="switch"` com `bind:checked={config.monitorar_pdf}`, `aria-describedby="ajuda-monitorar-pdf"`, `on:change={() => handleSave('monitorarPdf')}`, rótulo `{ROTULO_MONITORAR_PDF}` com `<Icone icone={FileSearch} tamanho={14} />` e a ajuda `{AJUDA_MONITORAR_PDF}`, depois do grupo da pasta monitorada.
  - `estados.test.mjs`: `'kind'` em `CAMPOS`; `fila-pdf` entra na lista dos estados novos, com nomes aceitos por `/^5000000-00\.2026\.4\.00\.\d{4}(\.zip| \(\d+ PDFs\))?$/` e caminhos só em `C:\Usuarios\exemplo`.
- [ ] **Passo 3: rodar** `node --test frontend/*.test.mjs scripts/visual/estados.test.mjs` (Git Bash): os novos falham.
- [ ] **Passo 4: implementar.**
  - `aviso.js` e `App.svelte` como nas Interfaces; o texto do overlay em `App.svelte:519` troca para `Solte arquivos ZIP ou PDF em qualquer lugar para processar`. `OnFileDrop` manda todos os aceitos numa chamada de `AddDroppedFiles` (é o que junta os PDFs num item).
  - `ProcessList.svelte`: `{@const tipo = item.kind === 'pdf' ? 'pdf' : 'zip'}` na linha; ícone em `--text-secondary`; textos do vazio.
  - `Settings.svelte`: o interruptor fica habilitado mesmo com o monitoramento desligado (o valor é só salvo) e usa o `Salvo` com `campo="monitorarPdf"`, como os outros.
  - `estados.mjs`: todo `item()` ganha `kind: 'zip'`; estado `fila-pdf` com um ZIP concluído, um PDF avulso concluído (`kind: 'pdf'`, `name` `processo(n)`, `paths` de um) e um item de três PDFs concluído (`name` `${processo(n)} (3 PDFs)`, `paths` de três), todos em `C:\Usuarios\exemplo`.
  - `DESIGN.md`: §4 ganha "Tipo da entrada: `FileArchive` (ZIP) e `File` (PDF), `12px`, `--text-secondary`, antes do nome" e `FileSearch` (converter PDFs da pasta) nas Configurações; §5.1 descreve o ícone de tipo e o nome do item de PDF. `PRD.md`: RF01 "ZIP do Eproc ou PDF avulso (vários PDFs de uma vez viram um TXT, por evento)"; RF02 ganha "e, se a opção estiver ligada, PDFs novos"; §5 passo 5 cita a regra do `_2` para PDF.
- [ ] **Passo 5: rodar** `node --test frontend/*.test.mjs` e `node --test .claude/hooks/hooks.test.mjs .claude/statusline.test.mjs .claude/workflows/workflows.test.mjs scripts/visual/estados.test.mjs`, `npx svelte-check` e `npm run build` em `frontend` (sem aviso novo), e o gate Go.
- [ ] **Passo 6: capturas "depois".** `node scripts/visual/capturar.mjs --saida docs/revisao/capturas/2026-11-26-pdf-avulso/depois --estados vazio,fila,fila-pdf,configuracoes`. No relatório, os pares antes/depois de `vazio` (textos novos), `configuracoes` (interruptor novo) e `fila` (ícone de tipo), e as capturas novas de `fila-pdf`; quem aprova é o dono, pelas imagens.
- [ ] **Passo 7: commit** `feat(frontend): soltar e selecionar PDF, ícone por tipo e opção de PDFs na pasta`, com os PNGs, `DESIGN.md` e `PRD.md`.

---

### Task 9: README e CLAUDE.md

**Files:**
- Modify: `README.md` (linhas 7, 43, 49-50, 99-118, 188, 233 e a tabela de pastas em 339-347)
- Modify: `CLAUDE.md` ("What this is", "Processing pipeline", "Config", "Frontend")

**Interfaces:**
- Consumes: o comportamento entregue nas Tasks 1 a 8.

- [ ] **Passo 1: README** (para o usuário): entra PDF avulso no resumo e na lista de recursos (arrastar ou selecionar um ou vários PDFs; vários viram um TXT por evento; nome `processo.txt` ou `processo_2.txt`, nunca por cima de arquivo alheio, salvo o reprocessamento do próprio item); o diagrama e "O `.txt` sai com o mesmo nome do ZIP" ganham o caso do PDF; a tabela de Configurações ganha "Também converter PDFs novos nesta pasta" (desligado; converte qualquer PDF que aparecer, inclusive boletos); `internal/saida/` na tabela de pastas. Tipos omitidos não valem para PDF avulso.
- [ ] **Passo 2: CLAUDE.md** (para quem programa), curto: a entrada do pipeline é `LerEntrada` e `ProcessarEntrada(Entrada)` (`ProcessZip`/`ProcessZipComAtividade` são invólucros); `ProcessFile.Abrir/Tamanho/Avulso`, e o caminho do usuário nunca vira `fontePDF.caminho`; `internal/saida` é a regra única do nome do TXT (ZIP sobrescreve o próprio; PDF por `GravarSemSobrescrever` com `_N` e o ZIP homônimo, `SaidaDoItem` para cópia e histórico, `NomeLote` recebe a lista na ordem por evento); `QueueItem.Kind/Paths/saidaAnterior` e `ReprocessarMonitorado`; o watchdog recebe as extensões em `Start` e descarta PDF já convertido (`jaConvertido`); `MonitorarPDF` na config.
- [ ] **Passo 3: rodar** `node --test frontend/*.test.mjs` (alguns testes leem `README.md` e `DESIGN.md`) e conferir os links.
- [ ] **Passo 4: commit** `docs: PDF avulso no README e no CLAUDE.md` (o `README.md` entra só nesta tarefa).

---

### Task 10: anonimizador de log com o TXT de item de PDF

**Files:**
- Modify: `scripts/anonimizar-log/anonimizar.go:38-57` (`rotulos.zip`)
- Test: `scripts/anonimizar-log/anonimizar_test.go`

**Interfaces:**
- Consumes: o atributo `zip` das linhas do pipeline e da fila num item de PDF, que leva o nome do primeiro PDF ou o `Name` do item com ` (N PDFs)` (Tasks 3 e 5), e o TXT `<radical>_N.txt` (Task 1).

- [ ] **Passo 1: testes** (linhas JSON sintéticas, com `5000000-00.2026.8.26.0001` e `C:\Users\fulano`).
  - `TestAnonimizar_TxtDePDF`: com `zip` = `5000000-00.2026.8.26.0001 - Evento 12 - SENT1.pdf`, `...\5000000-00.2026.8.26.0001 - Evento 12 - SENT1.txt` num erro de gravação vira `...\zip-1.txt`, e o radical delimitado também é trocado, como no ZIP.
  - `TestAnonimizar_NomeDeItemComContagem`: com `zip` = `5000000-00.2026.8.26.0001 (3 PDFs)`, o sufixo ` (3 PDFs)` sai antes de tirar o radical. `5000000-00.2026.8.26.0001.txt` vira `zip-1.txt`, e o nome do item inteiro vira `zip-1`.
  - `TestAnonimizar_TxtComSufixoN`: para o radical `processo-exemplo` (de `.zip` ou `.pdf`), `processo-exemplo_2.txt` e `processo-exemplo_999.txt` viram `zip-1_2.txt` e `zip-1_999.txt`. `processo-exemplo_1.txt` e `processo-exemplo_1000.txt` não viram `zip-1_N.txt`.
  - Os testes atuais do ZIP seguem iguais.
- [ ] **Passo 2: rodar** `go test ./scripts/anonimizar-log -v`: os novos falham.
- [ ] **Passo 3: implementar.** Em `rotulos.zip`:
  - antes de tirar a extensão, remover o sufixo ` (N PDFs)` (regex `` ` \(\d+ PDFs\)$` ``);
  - tratar `.pdf` como `.zip` (sem diferença de caixa);
  - um nome sem extensão (o do item) também gera o radical.
  
  Para cada radical, `<radical>_N.txt`, com N de 2 a 999, vira `<rótulo>_N.txt`. Pode ser pelo `Replacer` (998 entradas por radical) ou por uma regex por radical aplicada antes dele. Escolha a que mantiver o tempo de `anonimizar_test.go` e explique no relatório.
- [ ] **Passo 4: rodar** os testes e o gate Go.
- [ ] **Passo 5: commit** `fix(anonimizar-log): TXT de item de PDF`.
