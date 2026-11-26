# PDFs não lidos, aviso e segundo leitor: plano de implementação

> **Para agentes:** sub-skill obrigatória: use superpowers:subagent-driven-development (recomendado) ou superpowers:executing-plans para implementar este plano tarefa por tarefa. Os passos usam checkbox (`- [ ]`) para acompanhamento.

**Objetivo:** O item da fila, a notificação, o painel e o `.txt` passam a dizer quantos documentos não foram lidos e por quê; e os PDFs que a `dslipak/pdf` recusa passam a ser lidos pelo Poppler (`pdfinfo` conta as páginas, `pdftotext` extrai o texto, o OCR existente cobre as páginas sem texto).

**Arquitetura:**
- **Motivo por documento.** Um tipo `Leitura` em `internal/etl/leitura.go` classifica cada documento (lido, lido pelo segundo leitor, só OCR, digitalizado sem OCR, senha, ilegível, prazo, OCR falhou). O worker grava o motivo no mapa do próprio documento (chave `nao_lido`) e na `Atividade`.
- **Saída.** `formatter.go` lê a chave `nao_lido`: contagem no `<contexto>`, `NÃO LIDO` no `<summary>` e uma linha em português no corpo. O marcador técnico `[ERRO ...]` continua logo abaixo.
- **Fila e interface.** O item mantém `Status = "success"` e ganha `unread_docs`, lido de `Atividade.NaoLidos()` (a assinatura de `ProcessZipComAtividade` não muda). A linha do item, a notificação de fim e o Copiar usam o contador.
- **Processos externos.** `internal/ocr` ganha prazo por chamada (`ErrPrazo`) e as funções `InfoPDF` e `TextoPDF` (`poppler.go`).
- **Abertura única.** `abrirPDF` (`internal/etl/abertura.go`) tenta a biblioteca e, se ela falhar, o Poppler. A pré-contagem chama essa função e guarda o resultado por índice; o worker não abre de novo.

**Tecnologias:** Go 1.27.3, `github.com/dslipak/pdf` v0.0.2, Poppler 25.07.0 (`pdftoppm.exe` já embutido; `pdfinfo.exe` e `pdftotext.exe` novos), Tesseract embutido, Wails v2.13, Svelte 5 (sintaxe legada).

**Spec:** `docs/superpowers/specs/2026-11-11-pdfs-nao-lidos-design.md` (cópia do `concepts_brief.md`). Leia junto com este plano.

## Restrições globais

- A branch é `pdfs-nao-lidos`, criada a partir da `main` depois do merge do PR #7. Um commit por tarefa, com mensagem em pt-BR no formato `tipo(escopo): resumo` e as linhas de atribuição da sessão.
- Nunca dê push, merge ou release. Só a sessão principal faz isso, e só quando o dono pede.
- Não faça commit de `README.md`, `build/windows/installer/project.nsi`, `install.ps1` nem `.genkit/`: são mudanças locais do dono. Use `git add` com caminhos explícitos, nunca `-A` ou `.`.
- Todo PDF de teste é sintético e gerado no próprio código de teste (`buildPDF` em `pdf_parser_test.go`), sem nome real, número de processo, CPF ou endereço. Placeholders: `FULANO DE TAL`, `5000000-00.2026.8.26.0001`.
- Os ZIPs reais só entram na Tarefa 12, em cópias, e dela só saem contagens.
- No máximo 3 subagentes ao mesmo tempo. Tarefas de implementação rodam uma por vez.
- O item da fila não ganha status novo: continua `"success"`. `falhas.json` não muda de formato nem de regra.
- A saída dos PDFs que a biblioteca abre não muda, exceto pela linha nova do `<contexto>` e, com o OCR desligado, pelas páginas digitalizadas (Tarefa 5).
- A pipeline não muda de ordem nem de concorrência. O mapa `eventos` só recebe chaves de topo na pré-população; cada goroutine escreve só nos mapas do seu documento.
- Logs e textos da interface em pt-BR.
- `internal/ocr/embed/**` é protegida pelo hook `proteger-gerados`. A Tarefa 9 só roda depois de o dono responder "Sim" à decisão d2 no Pinboard.
- Depois de mudar um struct enviado ao frontend (`QueueItem`, `AtividadeDoc`), rode `wails generate module` e reverta o ruído de CRLF que não for do struct. Nunca edite `frontend/src/wailsjs/` à mão.
- Antes de cada commit de Go: `go vet ./...` e `go test -race ./internal/...`. Antes de cada commit de frontend: `cd frontend && npx svelte-check` e `node --test (Get-ChildItem frontend -Filter *.test.mjs).FullName` (pwsh).
- O formato de `gofmt` é conferido com `git show HEAD:<arquivo> | gofmt -d` (o checkout usa `core.autocrlf=true`).

## Foco da revisão

Estas são as entradas que a spec implica e que um usuário vai encontrar. Cada uma ganhou um teste na tarefa dona:

1. **Documento misto com o OCR desligado** (5 páginas, 3 digitalizadas) no nível Padrão: as 3 páginas saem com o marcador `[página digitalizada: o OCR está desativado]`. A regra de linhas repetidas nas bordas e a de páginas duplicadas (H7) não podem apagar nem trocar esses marcadores. Tarefas 2 e 5.
2. **Dois documentos que falham com o mesmo erro** não viram "documento idêntico" um do outro: documento não lido fica fora da pré-passada de duplicatas. Tarefa 2.
3. **Documento em branco** (páginas sem texto e sem imagem) continua contando como lido. Não é "não lido". Tarefa 1.
4. **Falha ao extrair os binários em `%TEMP%`** (ou `pdfinfo.exe` ausente): os PDFs que a biblioteca abre continuam saindo iguais; só os recusados ficam como "arquivo ilegível", com o detalhe técnico no painel. Tarefa 10.
5. **Duas entradas com o mesmo caminho no ZIP**, uma legível e outra não: o motivo fica na entrada certa e o contador mostra 1. Tarefa 1.

---

### Tarefa 1: Motivo de leitura por documento

**Arquivos:**
- Criar: `internal/etl/leitura.go`, `internal/etl/leitura_test.go`
- Modificar: `internal/etl/atividade.go`, `internal/etl/pipeline.go`
- Teste: `internal/etl/atividade_test.go`, `internal/etl/pipeline_leitura_test.go` (novo)

**Interfaces:**
- Produz:

```go
type Leitura string

const (
	LeituraLido          Leitura = "lido"
	LeituraSegundoLeitor Leitura = "segundo_leitor"
	LeituraOCR           Leitura = "ocr"
	LeituraSemOCR        Leitura = "sem_ocr"
	LeituraSenha         Leitura = "senha"
	LeituraIlegivel      Leitura = "ilegivel"
	LeituraPrazo         Leitura = "prazo"
	LeituraOCRFalhou     Leitura = "ocr_falhou"
)

func (l Leitura) NaoLido() bool   // sem_ocr, senha, ilegivel, prazo, ocr_falhou
func (l Leitura) Frase() string   // ver tabela abaixo
func (l Leitura) Acao() string    // "Reprocessar com OCR" só para sem_ocr; "" nos demais
func classificarErro(err error) Leitura
func classificarPaginas(paginas map[string]any, segundoLeitor bool) Leitura
func (a *Atividade) docLeitura(i int, l Leitura, detalhe string) // seguro com a == nil
func (a *Atividade) NaoLidos() int                               // 0 com a == nil
```

- `AtividadeDoc` ganha `Leitura string` (`json:"leitura,omitempty"`) e `Detalhe string` (`json:"detalhe,omitempty"`).
- Chave `nao_lido` (string com a frase) no mapa do documento, o mesmo que já recebe `total_paginas` e `omitido`. A goroutine do documento escreve; o formatter lê depois do `wg.Wait()`.

| `Leitura` | `Frase()` |
|---|---|
| `lido` | (vazia) |
| `segundo_leitor` | `lido pelo segundo leitor` |
| `ocr` | `texto obtido por OCR` |
| `sem_ocr` | `digitalizado; o OCR está desativado` |
| `senha` | `protegido por senha` |
| `ilegivel` | `arquivo ilegível` |
| `prazo` | `prazo de leitura esgotado` |
| `ocr_falhou` | `o OCR falhou em todas as páginas` |

Regras:
- `classificarErro`: `errors.Is(err, pdf.ErrInvalidPassword)` dá `senha`; `errors.Is(err, context.DeadlineExceeded)` dá `prazo`; o resto dá `ilegivel`. As Tarefas 6 e 10 acrescentam `ocr.ErrPrazo` e `ocr.ErrSenha`.
- `classificarPaginas`: uma página "tem texto" quando o `content` não é vazio depois de `TrimSpace`, não começa com `[ERRO` nem com `[texto ilegível na página:` (o texto que `marcadorIlegivel` devolve) e a página não tem `ocr_erro` nem `sem_ocr`. Com pelo menos uma página com texto: `ocr` se todas as páginas com texto têm `"ocr": true`; senão `segundo_leitor` se `segundoLeitor`; senão `lido`. Sem nenhuma página com texto: `sem_ocr` se alguma página tem `sem_ocr`; `prazo` se alguma tem `ocr_prazo`; `ocr_falhou` se alguma tem `ocr_erro`; `ilegivel` se alguma começa com `[ERRO` ou com `[texto ilegível na página:`; senão `lido` (documento em branco).
- No worker (`pipeline.go`), os caminhos que hoje chamam `marcarErro` + `encerrar(err.Error(), ...)` passam por um único ajudante local, por exemplo `naoLido(l Leitura, marcador, detalhe, msgProgresso string)`: mantém `marcarErro(marcador)` como está, grava `nao_lido = l.Frase()`, chama `atv.docLeitura(i, l, detalhe)` e `encerrar(l.Frase(), msgProgresso)`. O `Erro` do painel passa a ser a frase em português; o texto da biblioteca vai para `Detalhe`. O `recover` do worker usa `ilegivel`.
- Depois de `ParsePages` (e do HTML) terminar sem erro, o worker classifica com `classificarPaginas`. Se `NaoLido()`, usa o mesmo ajudante sem trocar as páginas; senão chama `atv.docLeitura(i, l, "")`.
- Nesta tarefa o `.txt` não muda: o formatter ainda ignora `nao_lido`.

- [ ] **Passo 1: Testes que falham** (`leitura_test.go`, em tabela)
  - `TestLeitura_NaoLido`: verdadeiro só para os cinco valores listados.
  - `TestLeitura_FraseEAcao`: as oito frases da tabela, literais; `Acao()` é `"Reprocessar com OCR"` só em `sem_ocr`.
  - `TestClassificarErro`: `fmt.Errorf("falha ao abrir o PDF: %w", pdf.ErrInvalidPassword)` dá `senha`; `context.DeadlineExceeded` embrulhado dá `prazo`; `errors.New("not a PDF file: invalid header")` dá `ilegivel`.
  - `TestClassificarPaginas`: (a) 2 páginas nativas dá `lido`; (b) 2 páginas com `"ocr": true` dá `ocr`; (c) 1 nativa e 1 de OCR dá `lido`; (d) nativas com `segundoLeitor = true` dá `segundo_leitor`; (e) todas com `sem_ocr` dá `sem_ocr`; (f) 1 com texto e 2 com `sem_ocr` dá `lido`; (g) todas com `ocr_erro` dá `ocr_falhou`; (h) todas com `ocr_erro` e uma com `ocr_prazo` dá `prazo`; (i) única página `[ERRO ao ler página 1: x]` dá `ilegivel`, e única página com o texto de `marcadorIlegivel("x")` também; (j) mapa vazio e (k) páginas com `content` vazio dão `lido`.
- [ ] **Passo 2: Testes que falham** (`atividade_test.go`, `pipeline_leitura_test.go`)
  - `TestAtividade_NaoLidos`: 3 documentos, `docLeitura` com `ilegivel`, `ocr` e `lido` dá `NaoLidos() == 1`; com `var a *Atividade`, `NaoLidos() == 0` e `docLeitura` não entra em pânico.
  - `TestPipeline_ClassificaNaoLido`: ZIP (`zipDeTeste`) com um PDF sadio e um arquivo `.pdf` que só tem os bytes `não é PDF`. Depois de `ProcessZipComAtividade`: `atv.NaoLidos() == 1`; o instantâneo do documento ruim tem `Leitura == "ilegivel"`, `Erro == "arquivo ilegível"` e `Detalhe` contendo `invalid header`; o `.txt` ainda contém `[ERRO ao analisar PDF:`.
  - `TestPipeline_EntradasDuplicadas_MotivoNaEntradaCerta`: duas entradas com o mesmo caminho, a primeira sadia e a segunda ilegível; `NaoLidos() == 1` e só o segundo instantâneo tem `Leitura == "ilegivel"`.
- [ ] **Passo 3:** Rode `go test ./internal/etl -run "Leitura|Classificar|NaoLido|EntradasDuplicadas" -v`. Esperado: falha de compilação (`undefined: Leitura`).
- [ ] **Passo 4:** Implemente `leitura.go`, os dois métodos de `Atividade` e a classificação no worker.
- [ ] **Passo 5:** Rode `go vet ./...` e `go test -race ./internal/...`. Esperado: tudo passa, sem mudar teste existente.
- [ ] **Passo 6:** `wails generate module` (o `AtividadeDoc` mudou) e commit: `feat(etl): motivo de leitura por documento`.

### Tarefa 2: O `.txt` avisa

**Arquivos:**
- Modificar: `internal/etl/formatter.go`, `PRD.md` (§6, linhas 73 a 125)
- Teste: `internal/etl/formatter_nao_lido_test.go` (novo); ajuste dos testes que comparam o cabeçalho inteiro

**Interfaces:**
- Consome: chave `nao_lido` do mapa do documento (Tarefa 1); chave `sem_ocr` da página (Tarefa 5; aqui o teste monta o mapa à mão).
- Produz, em `GenerateConsolidatedOutput`:
  - `<contexto>`: linha `Documentos não lidos: N` logo depois de `Total de Documentos:`. Sai sempre, também com `N = 0`.
  - `<summary>`: `- Evento %d - %s: NÃO LIDO` no lugar do rótulo de páginas.
  - Corpo do documento: `[documento não lido: <frase>; consulte o PDF original]` (em HTML, `consulte o arquivo original`). Se o `content` da primeira página começa com `[ERRO`, essa linha sai logo abaixo, como hoje. As demais páginas de um documento não lido não são emitidas.
  - Documento não lido não entra em `duplicadasPorConteudo` nem em `ctxDocumento`.
  - Página com `sem_ocr` (num documento lido) sai com o `content` como está, sem passar por `cleanPage`, e fica fora das linhas repetidas de borda e das duplicatas.

- [ ] **Passo 1: Testes que falham**
  - `TestSaida_ContagemDeNaoLidos`: 3 documentos, 1 com `nao_lido`; a saída contém `Documentos não lidos: 1`. Com nenhum, contém `Documentos não lidos: 0`.
  - `TestSaida_SummaryNaoLido`: a linha do summary do documento termina em `: NÃO LIDO` e não contém `página`.
  - `TestSaida_CorpoNaoLido`: `nao_lido = "arquivo ilegível"` e página 1 `[ERRO ao analisar PDF: x]` dão, nesta ordem, `[documento não lido: arquivo ilegível; consulte o PDF original]` e `[ERRO ao analisar PDF: x]`; `strings.Count(saida, "[ERRO") == 1`.
  - `TestSaida_NaoLidoSemOCR_SoOMarcador`: `nao_lido = "digitalizado; o OCR está desativado"` com 6 páginas `sem_ocr`: o corpo tem só a linha `[documento não lido: ...]`, nenhum `<pag`.
  - `TestSaida_NaoLidosIguaisNaoViramDuplicata`: dois documentos com o mesmo `[ERRO ...]`; a saída não contém `idêntic`.
  - `TestSaida_PaginaSemOCRNaoSomeNaPadrao`: documento lido de 5 páginas, 3 com `sem_ocr` e `content` `[página digitalizada: o OCR está desativado]`, nível `standard`: o marcador aparece 3 vezes.
- [ ] **Passo 2:** Rode `go test ./internal/etl -run TestSaida_ -v`. Esperado: FAIL.
- [ ] **Passo 3:** Implemente no formatter. Ajuste os testes existentes que comparam o `<contexto>` inteiro para incluir a linha nova; `pipeline_saida_test.go:284` e `:292` e `pipeline_test.go:162` devem passar sem mudança.
- [ ] **Passo 4:** Atualize o PRD §6: a linha `Documentos não lidos: [N]` no modelo, um item explicando o campo, `NÃO LIDO` no summary e o marcador do corpo.
- [ ] **Passo 5:** `go vet ./...`, `go test -race ./internal/...` e commit: `feat(etl): o txt informa os documentos não lidos`.

### Tarefa 3: Contador no item, notificação e cópia

**Arquivos:**
- Modificar: `internal/etl/queue.go` (struct `QueueItem`; ramo de sucesso perto da linha 544; `Reprocessar`), `app.go` (`mensagemFim`, linha 129)
- Teste: `internal/etl/queue_test.go`, `app_test.go`

**Interfaces:**
- Consome: `(*Atividade).NaoLidos()`.
- Produz: `QueueItem.UnreadDocs int` com `json:"unread_docs"`. `mensagemFim` com não lidos: `"%s pronto, com %s (%d docs, %d págs)"`, em que `%s` é `1 documento não lido` ou `N documentos não lidos`. Com zero, o texto de hoje.

- [ ] **Passo 1: Testes que falham**
  - `TestQueue_ContaNaoLidos`: fila com a pipeline real (sem o gancho `q.processar`, que não enxerga a `Atividade`) e um ZIP de `zipDeTeste` com 5 PDFs, 2 deles só com os bytes `não é PDF`; o item termina com `Status == "success"`, `Message == "Concluído"` e `UnreadDocs == 2`.
  - `TestQueue_ReprocessarZeraNaoLidos`: depois de `Reprocessar`, o item pendente tem `UnreadDocs == 0`.
  - `TestMensagemFim_NaoLidos`: item de sucesso com `UnreadDocs: 2`, 10 docs e 40 páginas dá `saida.txt pronto, com 2 documentos não lidos (10 docs, 40 págs)`; com `UnreadDocs: 1`, `1 documento não lido`; com 0, o texto atual.
  - `TestAoTerminar_NaoLidosNaoVaoParaFalhas`: item de sucesso com `UnreadDocs: 3` chama `falhas.Esquecer`, não `Registrar`.
- [ ] **Passo 2:** Rode os quatro testes. Esperado: FAIL (campo inexistente).
- [ ] **Passo 3:** Implemente. O contador é lido de `atv.NaoLidos()` em `processItem`, depois de `processar(...)` voltar sem erro (`atv` está no escopo; com o gancho de teste `q.processar` ele fica em 0).
- [ ] **Passo 4:** `wails generate module`, `go vet ./...`, `go test -race ./...` (inclui o pacote raiz) e commit: `feat(fila): contador de documentos não lidos no item e na notificação`.

### Tarefa 4: A interface avisa sem abrir o painel

**Arquivos:**
- Criar: `frontend/src/lib/naoLidos.js`, `frontend/naoLidos.test.mjs`
- Modificar: `frontend/src/lib/atividade.js`, `frontend/src/components/ProcessList.svelte`, `frontend/src/components/AtividadeItem.svelte`, `scripts/visual/estados.mjs`, `scripts/visual/estados.test.mjs`, `frontend/atividade.test.mjs`, `.claude/review.json` (lista `estados` da matriz de capturas, que ganha `nao-lidos`)

**Interfaces:**
- Consome: `item.unread_docs`; `d.leitura`, `d.erro` (frase em português) e `d.detalhe` de cada documento da atividade.
- Produz:
  - `rotuloNaoLidos(n)`: `''` para 0, `'1 não lido'`, `'N não lidos'`.
  - `avisoCopiaNaoLidos(n)`: `''` para 0; `'Copiado. Atenção: 1 documento não foi lido; o texto está incompleto.'`; no plural, `'Copiado. Atenção: N documentos não foram lidos; o texto está incompleto.'`.
  - `resumoAtividade(...)` ganha `naoLidos` (documentos com `leitura` em `sem_ocr`, `senha`, `ilegivel`, `prazo`, `ocr_falhou`) e `semOCR` (só `sem_ocr`).

Comportamento:
- **Linha do item concluído:** ao lado de `N docs | N pág`, o ícone `CircleAlert` (já importado) em `var(--warning)` e o texto de `rotuloNaoLidos`, com `title` "N documentos não foram lidos. Abra o item para ver quais." Nada aparece com zero.
- **Copiar:** depois de copiar com sucesso um item com não lidos, `avisoCopia.mostrar(avisoCopiaNaoLidos(n), 'aviso')`. O aviso some sozinho (3 s, regra atual de `criarAviso`) e usa uma variante de estilo com `--warning`, `--warning-soft` e `--warning-line`. O aviso de erro não muda.
- **Painel:** o chip "N com erro" passa a "N não lidos" quando `naoLidos > 0`. Cada documento não lido mostra `d.erro` e traz `d.detalhe` no `title`. Documentos com `leitura` `ocr` ou `segundo_leitor` ganham um rótulo discreto ("via OCR", "segundo leitor"); eles não aparecem na linha do item.
- **Ação:** com `semOCR > 0` e o item fora de processamento, o painel mostra o botão "Reprocessar com OCR", que abre o `ReprocessarItem` da linha com `ocrInicial = 'normal'`.

- [ ] **Passo 1: Testes que falham** (`naoLidos.test.mjs`, `atividade.test.mjs`): os valores literais acima; `resumoAtividade` com 4 documentos (`lido`, `ocr`, `sem_ocr`, `senha`) dá `naoLidos === 2` e `semOCR === 1`; `ProcessList.svelte` contém `rotuloNaoLidos(` e `avisoCopiaNaoLidos(`; `AtividadeItem.svelte` contém `Reprocessar com OCR`.
- [ ] **Passo 2:** `node --test frontend/naoLidos.test.mjs frontend/atividade.test.mjs`. Esperado: FAIL.
- [ ] **Passo 3:** Implemente a biblioteca e os dois componentes.
- [ ] **Passo 4:** Em `estados.mjs`, acrescente o estado `nao-lidos`: item concluído com `unread_docs: 2` e atividade com documentos `sem_ocr`, `senha` e `ocr` (dados fictícios). Atualize `estados.test.mjs` com o campo novo de `QueueItem`.
- [ ] **Passo 5:** Rode `node --test` nos testes do frontend e do harness, `npx svelte-check` e `npm run build`. Capture `nao-lidos` nos dois temas (skill `eprocia-visual`, `--estados nao-lidos`) e guarde as imagens em `docs/revisao/capturas/2026-11-11/`.
- [ ] **Passo 6:** Commit: `feat(ui): aviso de documentos não lidos na linha, na cópia e no painel`.

### Tarefa 5: Página digitalizada com o OCR desligado

**Arquivos:**
- Modificar: `internal/etl/pdf_parser.go` (`analisarPagina`, linha 457, e o ponto de `ParsePages` que monta o mapa da página)
- Teste: `internal/etl/pdf_sem_ocr_test.go` (novo)

**Interfaces:**
- Produz: página com `"sem_ocr": true` quando o OCR está desligado e a página iria ao OCR se estivesse ligado (texto nativo com menos de 100 caracteres e imagem grande ou conteúdo desenhado; ou texto ilegível). O `content` fica com o texto nativo que houver; se estiver vazio, vira `[página digitalizada: o OCR está desativado]`.
- Consome: `classificarPaginas` (Tarefa 1) e o tratamento de `sem_ocr` no formatter (Tarefa 2).

- [ ] **Passo 1: Testes que falham**
  - `TestSemOCR_PaginaDigitalizada`: `objetosPaginasComImagem(2)` com `ocrEnabled = false`: as 2 páginas têm `sem_ocr == true` e o `content` é o marcador.
  - `TestSemOCR_PaginaNativaNaoMuda`: PDF de texto com `ocrEnabled = false`: nenhuma página tem a chave `sem_ocr`.
  - `TestSemOCR_PaginaEmBrancoNaoEMarcada`: página sem texto, sem imagem e sem desenho: sem a chave, `content` vazio.
  - `TestPipeline_SemOCR_DocumentoNaoLido`: ZIP com `pdfDuasPaginasEscaneadas()` e `OcrMode = "none"`: `NaoLidos() == 1`, `Leitura == "sem_ocr"`, e o `.txt` contém `[documento não lido: digitalizado; o OCR está desativado; consulte o PDF original]` e `Documentos não lidos: 1`.
  - `TestPipeline_SemOCR_DocumentoMisto`: 5 páginas, 3 digitalizadas, nível `standard`: `NaoLidos() == 0` e o marcador de página aparece 3 vezes.
- [ ] **Passo 2:** Rode `go test ./internal/etl -run "SemOCR" -v`. Esperado: FAIL.
- [ ] **Passo 3:** Implemente. A checagem de imagem e de desenho (`HasImages`, `HasDrawnContent`) passa a rodar também com o OCR desligado, só quando o texto tem menos de 100 caracteres.
- [ ] **Passo 4:** `go vet ./...`, `go test -race ./internal/...` e commit: `fix(etl): página digitalizada com OCR desligado deixa de sair vazia`.

### Tarefa 6: Prazo em todo processo externo

**Arquivos:**
- Criar: `internal/ocr/prazo.go`, `internal/ocr/prazo_test.go`, `internal/ocr/main_test.go` (`TestMain`)
- Modificar: `internal/ocr/tesseract.go` (`renderizar`, linha 237; `reconhecer`, linha 330), `internal/etl/leitura.go`, `internal/etl/pdf_parser.go` (`registrarOCR`, `runOCRBloco`)
- Teste: `internal/etl/pdf_parser_lote_test.go`

**Interfaces:**
- Produz, em `internal/ocr`:

```go
var ErrPrazo = errors.New("prazo do processo externo esgotado")

var ( // variáveis para os testes encurtarem
	prazoRenderBase, prazoRenderPorPagina = 30 * time.Second, 10 * time.Second
	prazoOCRBase, prazoOCRPorPagina       = 60 * time.Second, 60 * time.Second
	prazoInfo                             = 20 * time.Second
	prazoTextoBase, prazoTextoPorPagina   = 30 * time.Second, 2 * time.Second
)
```

- Cada chamada externa roda num contexto filho com o prazo `base + porPagina × páginas da chamada` e com `cmd.WaitDelay = 2 * time.Second`. Se o filho expirou e o contexto de quem chamou não, o erro devolvido embrulha `ErrPrazo` (`errors.Is`). Cancelamento pelo usuário continua devolvendo o erro do contexto pai.
- `classificarErro` passa a tratar `ocr.ErrPrazo` como `prazo`. `registrarOCR` grava `"ocr_prazo": true` na página quando o erro é de prazo.
- `runOCRBloco`: um bloco que falha continua sendo refeito página a página. Depois de 2 páginas seguidas com `ErrPrazo`, as restantes do bloco recebem o mesmo erro sem rodar.

- [ ] **Passo 1: Testes que falham**
  - `main_test.go`: `TestMain` que, com `EPROCIA_TESTE_DORMIR` definida, dorme 1 minuto e sai. É o "processo preso".
  - `TestRenderizar_EstouraOPrazo`: `ferramentas` com `pdftoppm = os.Args[0]`, `t.Setenv("EPROCIA_TESTE_DORMIR", "1")` e os prazos de renderização em 200 ms: `renderizar` volta em menos de 5 s com `errors.Is(err, ErrPrazo)`.
  - `TestReconhecer_EstouraOPrazo`: o mesmo para `reconhecer` com `tess = os.Args[0]`.
  - `TestRenderizar_CancelamentoNaoEPrazo`: contexto pai cancelado depois de 100 ms: o erro não é `ErrPrazo`.
  - `TestParsePages_PrazoNoBloco_ParaDepoisDeDuas` (etl): `ocrLote` de teste devolve `ocr.ErrPrazo` e o runner por página também; num bloco de 6 páginas o runner por página é chamado 2 vezes e as 6 páginas têm `ocr_prazo == true`.
  - `TestPipeline_PrazoNaoTravaAFila` (etl): ZIP com um PDF digitalizado cujo OCR devolve `ErrPrazo` e um PDF de texto: `ProcessZipComAtividade` devolve `nil`, o primeiro tem `Leitura == "prazo"` e o segundo `"lido"`.
- [ ] **Passo 2:** Rode `go test ./internal/ocr -run "Prazo|Cancelamento" -v` e os dois testes de etl. Esperado: FAIL.
- [ ] **Passo 3:** Implemente.
- [ ] **Passo 4:** `go vet ./...`, `go test -race ./internal/...` e `go test ./scripts/bench` (OCR real, para confirmar que os prazos padrão não cortam uma página legítima). Commit: `fix(ocr): prazo em todo processo externo`.

### Tarefa 7: Teto de páginas por bloco de OCR

**Arquivos:**
- Modificar: `internal/etl/pdf_parser.go` (cálculo de `tamBloco` e a divisão final com `slices.Chunk`)
- Teste: `internal/etl/pdf_parser_lote_test.go`

**Interfaces:**
- Produz: `const maxPaginasBlocoOCR = 20`. Nenhum bloco entregue ao `ocrLote` tem mais páginas que isso; documentos pequenos continuam com um bloco por worker.

- [ ] **Passo 1: Testes que falham:** `TestOCREmLote_TetoDePaginasPorBloco`: `pdfComPaginas(t, 100, nil)` com `comOCREmLote(lote, 2)`, em que `lote` registra `len(paginas)`: o maior bloco tem no máximo 20 páginas e a soma é 100. `TestOCREmLote_DocumentoPequenoNaoMuda`: 8 páginas e 4 blocos: 4 chamadas de 2 páginas.
- [ ] **Passo 2:** Rode os dois. Esperado: o primeiro falha (blocos de 50).
- [ ] **Passo 3:** Implemente e rode `go test -race ./internal/etl`.
- [ ] **Passo 4:** Commit: `fix(etl): bloco de OCR com teto de 20 páginas`.

### Tarefa 8: Despacho de OCR fora das closures (refatoração pura)

**Arquivos:**
- Modificar: `internal/etl/pdf_parser.go`
- Criar: `internal/etl/pdf_ocr_despacho.go`

**Interfaces:**
- Produz um tipo que carrega o que hoje as closures de `ParsePages` capturam (`concluir`, `registrarOCR`, `runOCRTask`, `runOCRBloco`, `despachar`, o `WaitGroup` e o mapa de páginas):

```go
func (p *PDFParser) novoDespachoOCR(ctx context.Context, numPages int,
	ocrRunner func(context.Context, string, int) (string, error),
	onPageComplete func(pageNum int)) *despachoOCR

func (d *despachoOCR) paginaPronta(pageNum int, pagina map[string]any) // página que não vai ao OCR
func (d *despachoOCR) agendar(task ocrTask)                            // página que vai ao OCR
func (d *despachoOCR) esperar() map[string]any                         // despacha o resto, espera e devolve o mapa
```

- Nenhuma mudança de comportamento. Nenhum teste novo e nenhum teste alterado.

- [ ] **Passo 1:** Rode `go test -race ./internal/etl` e guarde o resultado como linha de base.
- [ ] **Passo 2:** Mova as closures para `despachoOCR` e faça `ParsePages` usar o tipo.
- [ ] **Passo 3:** Rode `go vet ./...` e `go test -race ./internal/...`. Esperado: o mesmo resultado, sem tocar em `_test.go`.
- [ ] **Passo 4:** Rode `go test ./scripts/bench -run '^$' -bench "Sintetico|ParsePDF" -count=10 -benchmem` antes e depois e compare com `go tool benchstat`: nenhuma métrica piora mais que 5% (skill `eprocia-bench`).
- [ ] **Passo 5:** Commit: `refactor(etl): despacho de OCR fora das closures de ParsePages`.

### Tarefa 9: `pdfinfo` e `pdftotext` embutidos

**Pré-requisito:** resposta "Sim, embutir os dois" à decisão d2.

**Arquivos:**
- Criar: `internal/ocr/embed/poppler/pdfinfo.exe`, `internal/ocr/embed/poppler/pdftotext.exe`, `internal/ocr/poppler.go`, `internal/ocr/poppler_test.go`
- Modificar: `internal/ocr/versao_embed.go` (via `go generate`), `internal/ocr/tesseract.go` (preparação só do Poppler), `README.md` (linha de componentes de terceiros; só o trecho novo, sem os trechos locais do dono), `internal/etl/leitura.go`

**Interfaces:**
- Produz:

```go
var ErrSenha = errors.New("PDF protegido por senha")

// InfoPDF devolve o número de páginas lido pelo pdfinfo.
func InfoPDF(ctx context.Context, pdfPath string) (paginas int, err error)

// TextoPDF devolve o texto nativo de cada página (índice 0 = página 1),
// sempre com len == paginas.
func TextoPDF(ctx context.Context, pdfPath string, paginas int) ([]string, error)
```

- `InfoPDF` roda `pdfinfo <caminho>` com `prazoInfo` e lê a linha `Pages:`. `stderr` com `Incorrect password` vira `ErrSenha`. Zero página é erro.
- `TextoPDF` roda `pdftotext -enc UTF-8 -f 1 -l N <caminho> -` com `prazoTextoBase + prazoTextoPorPagina × N` e separa a saída em `\f` (o `pdftotext` põe um `\f` depois de cada página, inclusive a última). Número de páginas diferente de `N` é erro.
- As duas usam uma preparação que só exige os binários do Poppler, sem `tessdata`: precisam funcionar com `OcrMode = "none"`. Os dois comandos passam por `prepareCmd` (sem janela de console) e pelo `PATH` com a pasta do Poppler, como `renderizar`.
- `classificarErro` passa a tratar `ocr.ErrSenha` como `senha`.

Os binários:
- Origem: pacote do Poppler 25.07.0 para Windows de `github.com/oschwartz10612/poppler-windows`, o mesmo build do `pdftoppm.exe` embutido. Baixe para uma pasta nova e vazia; não execute nada de dentro dela.
- Critério de aceite, antes de copiar: `pdfinfo.exe` com 65.024 bytes e SHA-256 `8ec2033f36247092866c757137d8815ab834d228e8322c45a820a12e14002f20`; `pdftotext.exe` com 70.656 bytes e SHA-256 `e66e7ada660a910918dd2974ce6486170a20c2ac73a78b003e9888b97152a82a`; e `pdftoppm.exe` e `poppler.dll` do pacote com o mesmo SHA-256 dos embutidos. Se algum valor divergir, pare e avise.
- Copie os dois arquivos com o shell (`Copy-Item`); as ferramentas Edit e Write são barradas pelo hook nessa pasta. Nenhuma DLL é trocada.

- [ ] **Passo 1:** Confira os quatro SHA-256, copie os dois executáveis e rode `go generate ./internal/ocr`. `go test ./internal/ocr -run TestVersaoEmbutidos_Atualizada` passa com o carimbo novo.
- [ ] **Passo 2: Testes que falham** (`poppler_test.go`; usam os binários reais e são pulados com `-short`)
  - `TestInfoPDF_ContaPaginas`: PDF sintético de 3 páginas dá `3, nil`.
  - `TestTextoPDF_UmTextoPorPagina`: 3 páginas com `PAGINA UM`, `PAGINA DOIS`, `PAGINA TRES`: 3 textos, cada um com a sua frase.
  - `TestTextoPDF_PaginaSemTexto`: página do meio sem texto: `len == 3` e o item do meio vazio depois de `TrimSpace`.
  - `TestInfoPDF_NaoEPDF`: arquivo com bytes aleatórios: erro que não é `ErrSenha` nem `ErrPrazo`.
  - `TestInfoPDF_Prazo`: `pdfinfo` trocado por `os.Args[0]` com `EPROCIA_TESTE_DORMIR`: `errors.Is(err, ErrPrazo)`.
  - `TestPoppler_SemTessdata`: as duas funções funcionam sem que nenhum `tessdata` tenha sido extraído.
- [ ] **Passo 3:** Rode `go test ./internal/ocr -run "InfoPDF|TextoPDF|Poppler" -v`. Esperado: FAIL (`undefined: InfoPDF`).
- [ ] **Passo 4:** Implemente `poppler.go` e a preparação.
- [ ] **Passo 5:** Atualize a linha do Poppler nos componentes de terceiros do README (`pdftoppm`, `pdfinfo`, `pdftotext` e suas DLLs). Faça o commit desse trecho com `git add -p README.md`, deixando de fora os trechos do dono.
- [ ] **Passo 6:** `go vet ./...`, `go test -race ./internal/...` e commit: `feat(ocr): pdfinfo e pdftotext do Poppler embutidos`.

### Tarefa 10: Abertura única e leitura pelo Poppler

**Arquivos:**
- Criar: `internal/etl/abertura.go`, `internal/etl/abertura_test.go`, `internal/etl/pdf_poppler.go`
- Modificar: `internal/etl/pipeline.go` (`contarPaginas`, linha 233; laço de pré-contagem, linhas 305 a 330; montagem do parser no worker), `internal/etl/pdf_parser.go`

**Interfaces:**
- Consome: `ocr.InfoPDF`, `ocr.TextoPDF`, `ocr.ErrSenha`, `ocr.ErrPrazo`, `despachoOCR` (Tarefa 8), `classificarErro`.
- Produz:

```go
type aberturaPDF struct {
	leitor  *pdf.Reader // nil quando o Poppler abriu
	paginas int
	poppler bool
}

var infoPDF, textoPDF = ocr.InfoPDF, ocr.TextoPDF // trocáveis nos testes

func abrirPDF(ctx context.Context, f *fontePDF) (aberturaPDF, error)
func (p *PDFParser) comPoppler(numPaginas int) *PDFParser
```

Regras:
- `abrirPDF` tenta a biblioteca (`contarPaginasPDF`, com o `recover` de hoje). Se ela devolver `pdf.ErrInvalidPassword`, o erro volta sem tentar o Poppler. Em qualquer outra falha, materializa o PDF em disco (`f.materializar()`) e chama `infoPDF`. Se o Poppler também falhar, o erro devolvido embrulha o erro do Poppler (para `errors.Is`) e cita o texto da biblioteca.
- A pré-contagem chama `abrirPDF` e guarda por índice: a contagem (como hoje), `viaPoppler[i]` e `errosAbertura[i]`. Com falha nos dois leitores, o documento continua contando 1 página.
- O worker não abre de novo. Com `errosAbertura[i] != nil`, marca não lido direto: marcador `[ERRO ao analisar PDF: <erro>]` (formato de hoje) e `classificarErro`. Com `viaPoppler[i]`, monta o parser com `comPoppler(contagens[i])`.
- `ParsePages` com `comPoppler` chama `textoPDF` uma vez e trata cada página assim:
  - texto com 100 caracteres ou mais (depois de `TrimSpace`) e legível: página nativa, `"ocr": false`;
  - texto curto ou ilegível (`textoIlegivel`) com o OCR ligado: `agendar` no `despachoOCR`, com o texto curto como reserva;
  - texto curto com o OCR desligado: se houver texto, fica o texto; se estiver vazio, página `sem_ocr` com o marcador da Tarefa 5.
- Se `textoPDF` falhar: com o OCR ligado, todas as páginas vão ao OCR; com o OCR desligado, `ParsePages` devolve o erro e o documento fica não lido.
- O worker passa `segundoLeitor = viaPoppler[i]` a `classificarPaginas`.
- Os PDFs que a biblioteca abre seguem exatamente o caminho de hoje.

- [ ] **Passo 1: Testes que falham** (`abertura_test.go`; `infoPDF` e `textoPDF` trocados por funções de teste, sem binário)
  - `TestAbrirPDF_BibliotecaAbre`: PDF sadio: `poppler == false`, `leitor != nil`, e `infoPDF` não é chamado.
  - `TestAbrirPDF_CaiNoPoppler`: PDF sadio com o cabeçalho trocado para `%PDF-2.0` e `infoPDF` devolvendo 3: `poppler == true`, `paginas == 3`, `leitor == nil`.
  - `TestAbrirPDF_PanicoDaBibliotecaCaiNoPoppler`: `contarPaginasPDF` trocado por uma função que entra em pânico (`loading {8 0}: found {7 0}`): mesmo resultado.
  - `TestAbrirPDF_SenhaNaoTentaOPoppler`: `contarPaginasPDF` devolve `pdf.ErrInvalidPassword`: `infoPDF` não é chamado e `classificarErro(err) == LeituraSenha`.
  - `TestAbrirPDF_OsDoisFalham`: `infoPDF` devolve `ocr.ErrPrazo`: `classificarErro(err) == LeituraPrazo` e `err.Error()` contém `invalid header`.
  - `TestParsePages_Poppler_TextoNativo`: `textoPDF` devolve 2 textos longos: 2 páginas com `"ocr": false` e o runner de OCR não é chamado.
  - `TestParsePages_Poppler_PaginaCurtaVaiAoOCR`: textos `["", <200 caracteres>]` com o OCR ligado: a página 1 vai ao runner e fica `"ocr": true`; a página 2 é nativa.
  - `TestParsePages_Poppler_SemOCR`: textos `["", ""]` com o OCR desligado: 2 páginas `sem_ocr`.
  - `TestParsePages_Poppler_TextoFalhaComOCR`: `textoPDF` devolve erro, OCR ligado, 3 páginas: as 3 vão ao runner.
  - `TestPipeline_Poppler_ProgressoCerto`: ZIP com um PDF `%PDF-2.0` de 7 páginas (`infoPDF` devolve 7): todas as chamadas de `onProgress` têm `totalPages == 7`, a última com `processedPages == 7`.
  - `TestPipeline_Poppler_NaoAbreDuasVezes`: ZIP com um PDF ilegível: `infoPDF` é chamado 1 vez.
  - `TestPipeline_BinariosIndisponiveis`: `infoPDF` devolve `errors.New("falha ao extrair binários")`; ZIP com um PDF sadio e um `%PDF-2.0`: o sadio sai com o mesmo texto de antes, o outro fica `ilegivel` com o `Detalhe` citando a falha.
- [ ] **Passo 2:** Rode `go test ./internal/etl -run "AbrirPDF|Poppler|BinariosIndisponiveis" -v`. Esperado: FAIL.
- [ ] **Passo 3:** Implemente `abertura.go`, `pdf_poppler.go` e as mudanças na pré-contagem e no worker.
- [ ] **Passo 4:** `go vet ./...`, `go test -race ./internal/...` e commit: `feat(etl): Poppler como segundo leitor dos PDFs que a biblioteca recusa`.

### Tarefa 11: Um PDF sintético por classe, de ponta a ponta

**Arquivos:**
- Criar: `internal/etl/pdf_classes_test.go` (geradores e testes; binários reais do Poppler; pulado com `-short`)

**Interfaces:**
- Consome: a pipeline inteira com `ocr.InfoPDF` e `ocr.TextoPDF` reais.
- Produz, só no código de teste:
  - `pdfVersao20(objects []string) []byte`: saída de `buildPDF` com o cabeçalho `%PDF-2.0`.
  - `pdfEmEnvelope(pdf []byte) []byte`: prefixo DER de 52 bytes começando por `30 80 06 09 2A 86 48 86 F7 0D 01 07 02`, o PDF como um OCTET STRING primitivo e os bytes de fechamento.
  - `pdfAESV2(objects []string, senhaUsuario string) []byte`: `/Encrypt` com `/V 4 /R 4`, `/CFM /AESV2` e chave de 128 bits, strings e streams cifrados com AES-CBC (`crypto/md5`, `crypto/rc4` e `crypto/aes` da biblioteca padrão).
  - `pdfTruncado() []byte`: os 64 primeiros bytes de um PDF sadio.

- [ ] **Passo 1: Testes** (`TestClasses_*`, cada um com um ZIP de um documento e `OcrMode = "none"`, salvo indicação)
  - `Versao20`: `Leitura == "segundo_leitor"`, o `.txt` contém a frase sintética do PDF e `Documentos não lidos: 0`.
  - `Envelope`: o mesmo.
  - `AESV2SemSenha` (senha de usuário vazia): o mesmo.
  - `AESV2ComSenha` (senha `segredo`): `Leitura == "senha"` e o `.txt` contém `[documento não lido: protegido por senha; consulte o PDF original]`.
  - `Truncado`: `Leitura == "ilegivel"` e `Documentos não lidos: 1`.
  - `Versao20Digitalizado` (página só com imagem, cabeçalho 2.0): com `OcrMode = "none"`, `Leitura == "sem_ocr"`; com `OcrMode = "fast"` e a imagem de `scripts/bench/fixtures`, `Leitura == "ocr"` e 0 não lidos.
  - `TabelaInconsistente`: não há PDF sintético com esse defeito (a causa dentro dos 4 arquivos não foi investigada). O teste troca `contarPaginasPDF` por uma função que entra em pânico e confere que um PDF sadio de texto sai como `segundo_leitor`.
- [ ] **Passo 2:** Rode `go test ./internal/etl -run TestClasses_ -v`. Esperado: PASS. O que falhar aqui é defeito das Tarefas 9 ou 10: corrija lá, com um teste unitário que reproduza.
- [ ] **Passo 3:** `go test -race ./internal/...` e commit: `test(etl): um PDF sintético por classe recusada pela biblioteca`.

### Tarefa 12: Medição real, documentação e gate final

**Arquivos:**
- Modificar: `CLAUDE.md` (Architecture, OCR e "Known traps"), `README.md` (só se a decisão for tirar a seção "Limitações conhecidas" já nesta branch; por padrão ela sai na release, item t43)

- [ ] **Passo 1: Linha de base.** Com o `eprocia.exe` da `main` (v0.4.0), processe as cópias dos 3 ZIPs de referência com o OCR ligado e guarde os `.txt` numa pasta do scratchpad da sessão. Nada dessa pasta é versionado.
- [ ] **Passo 2: Medição.** Processe as mesmas cópias com o build da branch, com o OCR ligado e depois desligado. Imprima só contagens:
  - OCR ligado: 0 documentos não lidos (hoje 16).
  - OCR desligado: 5 lidos pelo segundo leitor e 11 com "digitalizado; o OCR está desativado", contados no item e no `.txt`.
  - Tempo total por ZIP antes e depois.
- [ ] **Passo 3: Comparação byte a byte.** Um script no scratchpad separa cada `.txt` por `<documento>` e compara os 128 blocos dos PDFs que a biblioteca abre, antes e depois, com o OCR ligado. Esperado: 128 iguais, 0 diferentes. O script imprime só os dois números.
- [ ] **Passo 4: Etapa opcional da spec (cabeçalho `%PDF-2.x`).** Meça, nos 8 arquivos dessa classe, o tempo e o número de caracteres pela biblioteca (com o cabeçalho trocado, em cópia) e pelo Poppler. Relate os números. Não implemente: a decisão é do dono.
- [ ] **Passo 5: `CLAUDE.md`.** Em Architecture: `abrirPDF`, o tipo `Leitura`, a chave `nao_lido`, o campo `unread_docs`, `InfoPDF`/`TextoPDF` e os prazos. Em "Known traps": as quatro classes que a `dslipak/pdf` recusa (PDF 2.0, envelope PKCS#7, tabela de objetos inconsistente, AESV2), que o pânico da terceira acontece em `NumPage()` e que o `pdftotext` põe `\f` depois de cada página, inclusive a última.
- [ ] **Passo 6: Gates.** `go vet ./...`; `go test -race ./...`; `node --test` (frontend e harness); `cd frontend && npx svelte-check && npm run build`; `pwsh scripts/build/gate.ps1`. O gate regrava `build/bin/eprocia.exe`: se o aplicativo estiver aberto a partir dessa pasta, pare e peça ao dono para fechar. Reverta o ruído de CRLF em `wailsjs/`, `go.mod` e `package.json.md5`.
- [ ] **Passo 7:** Commit: `docs: PDFs não lidos e segundo leitor no CLAUDE.md`. Depois, revisão final da branch inteira por um revisor novo, no modelo mais capaz.

---

## Fora deste plano

- **Release v0.4.1** (item t44 do Pinboard): subir `productVersion`, ensaiar com `v0.4.1-rc1`, conferir a atestação, publicar e, então, tirar do README a seção "Limitações conhecidas" e a nota "main x v0.3.0" (t43). As notas da versão, escritas pelo dono, precisam cobrir: o defeito existe desde a v0.2.0; `.txt` sem a linha `Documentos não lidos:` veio de versão antiga, e para refazer basta adicionar o ZIP de novo; o OCR pode errar; sem OCR, digitalizações ficam marcadas como não lidas; fechar outras cópias do aplicativo antes de atualizar (a pasta `%TEMP%/eprocIA_bin` é compartilhada).
- **Reabrir pelo Poppler um PDF que a biblioteca abre mas não consegue ler nenhuma página.** Ele passa a ser avisado como "arquivo ilegível"; a segunda tentativa fica para depois.
- **Tratar o cabeçalho `%PDF-2.x` antes da biblioteca**, o desembrulho do envelope, o botão "Copiar diagnóstico" e o adaptador sobre `pdf.*`: fora, como na spec.

## Decisões do redator, para o dono conferir

A spec não fixa estes pontos. O plano escolheu assim; qualquer um pode mudar antes da execução:

1. **`Documentos não lidos: N` sai sempre, inclusive com 0.** Dá uma confirmação positiva a quem lê e distingue um `.txt` novo de um antigo. Custo: o cabeçalho de todo `.txt` muda (a comparação byte a byte é feita por documento). Depende da decisão d5.
2. **O marcador técnico `[ERRO ...]` continua**, logo abaixo da linha em português. Quem já procura `[ERRO` no `.txt` (como o README ensina) continua achando, e os testes atuais não mudam.
3. **Documento misto com o OCR desligado conta como lido.** As páginas digitalizadas ficam marcadas uma a uma, mas o item não mostra o alerta. A alternativa é contar como não lido qualquer documento com página digitalizada sem OCR.
4. **Documento não lido aparece no painel com o estado de erro que já existe**, e o chip passa de "N com erro" para "N não lidos".
5. **Prazos:** `pdftoppm` 30 s + 10 s por página; `tesseract` 60 s + 60 s por página; `pdfinfo` 20 s; `pdftotext` 30 s + 2 s por página. São folgados de propósito: o objetivo é soltar um processo preso, não apressar um lento.
6. **Depois de 2 páginas seguidas com prazo esgotado, o resto do bloco não é tentado.** Sem isso, um arquivo que trava o `pdftoppm` seguraria a fila por vários minutos por bloco.
7. **Teto de 20 páginas por bloco de OCR** (cerca de 60 MB de PNG por bloco, no pior caso medido de 3 MB por página a 150 dpi).
8. **`pdftotext` no modo padrão** (ordem de leitura), sem `-layout`: menos espaços para o LLM. Uma tabela larga pode sair menos alinhada que pela biblioteca.
9. **No segundo leitor, "menos de 100 caracteres" manda a página ao OCR.** É mais grosseiro que a regra de imagem e vetor do caminho normal; o custo de errar é um OCR desnecessário.
10. **O aviso ao copiar some sozinho em 3 segundos.** A alternativa é ficar até ser fechado, como o aviso de erro.
