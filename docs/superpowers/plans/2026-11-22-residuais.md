# Rodada de residuais (2026-11-22): plano de implementação

> **Para agentes:** sub-skill obrigatória: use superpowers:subagent-driven-development (recomendado) ou superpowers:executing-plans para implementar este plano tarefa por tarefa. Os passos usam checkbox (`- [ ]`) para acompanhamento. Regras comuns a todo implementador e revisor: `docs/superpowers/plans/2026-11-22-residuais-regras.md`.

**Goal:** Fechar os residuais de `docs/revisao/2026-11-14-residuais-pdfs-nao-lidos.md` que ainda valem e não mudam o que o usuário vê: asserções e oráculos de teste que faltam, a limpeza da pasta temporária dos testes de classes, uma captura de conferência (P6) e duas correções pequenas de produção (`esperar()` que despacharia duas vezes; regex do `pdfinfo` e teto de memória na saída do Poppler). O que muda texto do `.txt`, UI ou notificação fica na seção "Decisões do dono", sem tarefa.

**Architecture:** Nenhuma mudança de arquitetura. As tarefas 1 a 6 só mexem em testes e em `scripts/visual/capturar.mjs`. A Task 7 muda `internal/etl/pdf_ocr_despacho.go` (zera `d.pendentes` em `esperar()` e renomeia `aguardar`). A Task 8 muda `internal/ocr/poppler.go` (regex `Pages:` sem quebra de linha e teto de bytes no stdout de `rodarPoppler`).

**Tech Stack:** Go 1.27.3, `github.com/dslipak/pdf` v0.0.2, Poppler e tesseract embutidos (`internal/ocr/embed/`), `node --test`, Playwright (só na Task 6, via `scripts/visual/capturar.mjs`). Sem dependência nova.

**Spec:** `docs/revisao/2026-11-14-residuais-pdfs-nao-lidos.md` (ids `M`, `P` e `CP`). Já decidido sobre eles: `docs/revisao/2026-11-18-plano-correcao.md` §5.

## Situação de cada item

Conferido no código da `main` `aa9e066`.

| Id | Classificação | Gravidade / custo | Onde fica |
|----|---------------|-------------------|-----------|
| M5 | precisa de decisão do dono (muda o painel) | média / M | d1 |
| P3 | já resolvido no Go e na lógica do frontend; sobra o rótulo de leitor de tela | baixa / P | d4 |
| P4 | vale (asserção frágil) | baixa / P | Task 2 |
| P6 | vale, só conferência visual | baixa / P | Task 6 |
| P7 | não vale: mapas por id sem poda, inofensivo (plano de 2026-11-18 §5) | - | - |
| P9 | não vale: refatoração sem ganho, 8 usos em teste (§5) | - | - |
| P10 | não vale: produção só usa `WithCancel`; nenhum `WithTimeout` fora de `internal/ocr/prazo.go:37` | - | - |
| P14 (comportamento) | vale: regex e teto de saída. A trava do `initMu` e o log duplicado não valem (só latência na primeira chamada; §5) | baixa / P | Task 8 |
| P18 | precisa de decisão do dono (muda a linha e a notificação) | baixa / P | d2 |
| CP2 | não vale agora: sem página-isca real para medir (§5) | - | - |
| CP9 | precisa de decisão do dono (muda o `.txt`) | baixa / P | d3 |
| CP4, CP5 | condicionais ao CP10 (medição em curso noutra frente) | - | sem tarefa |
| P2 | vale | baixa / P | Task 2 |
| P5 | vale | baixa / P | Task 2 |
| P8 | não vale: sem navegador nos testes, não há alternativa barata à regex (§5) | - | - |
| P11 | já resolvido: `TestRunOCR_FallbackSemPortugues` (`internal/ocr/tesseract_test.go:332`, commit `d9b91f7`) faz o tesseract com `-l por` falhar sem prazo e confere 3 processos e o texto do fallback; RunOCR e RunOCRPaginas passam pelo mesmo `reconhecer` (`tesseract.go:379`, ramo em `:415-427`) | - | - |
| P12 | vale (teste que falta; o `min` de `pdf_ocr_despacho.go:109` fica como defesa) | baixa / P | Task 2 |
| P13 | vale (produção trivial) | baixa / P | Task 7 |
| P14 (testes) | vale | baixa / P | Task 1 |
| P16 | vale | baixa / M | Task 5 |
| CP6 | vale | média / M | Task 3 |
| CP7 | vale (teste que fixa o comportamento atual) | baixa / P | Task 2 |
| CP8 | vale | baixa / P | Task 4 |

## Decisões do dono (sem tarefa até a resposta)

**d1 (M5).** Documento lido com algumas páginas digitalizadas sem OCR (OCR desligado) não aparece no painel nem oferece "Reprocessar com OCR"; hoje o aviso (`AtividadeItem.svelte:110-118`) só conta documentos inteiros `sem_ocr` (`resumo.semOCR`, `frontend/src/lib/atividade.js:53,83`). O conserto passa por `AtividadeDoc` (`internal/etl/atividade.go:18`) com um campo novo, p. ex. `PaginasSemOCR int json:"paginas_sem_ocr,omitempty"`, preenchido onde o pipeline classifica o documento (`pipeline.go`, junto do `docLeitura`), e pelo resumo do frontend.
- (a) Contar esses documentos no mesmo aviso "N documentos digitalizados ficaram sem OCR" e mostrar o mesmo botão. **Recomendada**: um aviso só, e o botão já reprocessa com OCR.
- (b) Linha própria: "N páginas digitalizadas ficaram sem OCR", com o mesmo botão.
- (c) Deixar como está (o marcador dentro do `.txt` é o único sinal).

**d2 (P18).** Falha do ZIP inteiro mostra `err.Error()` cru na linha e na notificação (`internal/etl/queue.go:565-566`, `:522`, `:462-463`; `app.go:155-159`). Pode trazer o caminho do ZIP ou do `.txt` (nome de usuário do Windows) e frase em inglês (`pipeline.go:307`, "no PDF or HTML documents found in ZIP").
- (a) Mesmo texto, com cada caminho trocado pelo nome do arquivo (função em `queue.go` aplicada a `Message` e `Error`; o log segue com o erro inteiro). **Recomendada**: menor mudança, mesmo princípio de `textoErroSaida`.
- (b) Frase fixa por classe via `textoErroSaida` ("falha de leitura ou gravação em disco" etc.) e tradução da frase em inglês.
- (c) Deixar como está.
Com (a) ou (b), o parêntese do CLAUDE.md "(A whole-ZIP failure still puts the raw error...)" sai.

**d3 (CP9).** `HasImages` (`internal/etl/pdf_parser.go:586`) não tem `recover`; numa página de texto curto com XObject de tabela inconsistente, o pânico cai no `recover` de `analisarPagina` (`:356-360`) e a página sai `[ERRO ao ler página: ...]` (documento de uma página fica não lido).
- (a) `recover` dentro de `HasImages`, pânico conta como sem imagens: a página sai com o texto nativo curto e segue para `HasDrawnContent`. **Recomendada**: é a sugestão do residual e nunca piora a página.
- (b) Pânico conta como com imagens: com OCR, a página vai ao OCR com o texto curto de reserva; sem OCR, sai com o texto e a marca `sem_ocr`. Recupera mais se a imagem existir, mas, se o `pdftoppm` também falhar no arquivo, a página ganha `ocr_erro` e sai pior que em (a).
- (c) Deixar como está.
Teste das duas: a página de `paginaComObjetoTrocado` (`pdf_objeto_quebrado_test.go:19`) com stream curto sem `Do` e `/Im1 6 0 R` no `/XObject`.

**d4 (P3).** O Go e a lógica do frontend já usam `Leitura` (`atividade.js:58,87` excluem não lidos de `erros`; `queue.go:581` e `app.go:141` contam por `NaoLidos()`; o ícone de `AtividadeItem.svelte:139` testa `naoLido(d)` antes de `estado`). Sobra o texto só para leitor de tela de `AtividadeItem.svelte:148`, que lê `ROTULO[d.estado]` e anuncia "Erro:" para um documento não lido (o ícone é de aviso).
- (a) Rótulo "Não lido" quando `naoLido(d)`. **Recomendada.**
- (b) Deixar.

**Condicionais ao CP10 (sem tarefa).** CP4 (`topoImagens` em `pdf_estrutura.go` conta imagem fora do MediaBox) e CP5 (`proximaAncora` quadrática e NFD em `pdf_juncao.go`) vivem na junção, que o resultado do CP10 pode reescrever. Entram num plano depois da medição. A Task 3 não acrescenta caso com NFD para não fixar o comportamento do CP5.

## Global Constraints

- Regras de ambiente, gate, commit e relatório: `docs/superpowers/plans/2026-11-22-residuais-regras.md`.
- Nenhuma tarefa muda texto do `.txt`, rótulo, aviso, notificação, valor de `Leitura` ou formato PRD §6. A Task 8 só muda o resultado de um PDF cuja saída do `pdftotext` passe de 64 MiB (hoje: memória sem teto).
- `go.mod`, `go.sum` e `internal/ocr/embed/` não mudam. `frontend/src/wailsjs/` não muda.
- Tarefas 1 a 6 não mudam código de produção (`*.go` fora de `_test.go`, `frontend/src/**`). Se uma delas exigir, pare com `NEEDS_CONTEXT`.
- Linhas de log novas em pt-BR, só com números e textos fixos (sem caminho, sem texto de página).
- Uma tarefa por vez; no máximo 3 agentes ao mesmo tempo na máquina.

## Review Focus

- **Teste que passa sem testar.** Em cada teste novo, o revisor quebra a produção de propósito (numa cópia, sem commit) e confere que o teste falha: Task 2 (P12: trocar `maxPaginasBlocoOCR` por 25; CP7: fazer `conteudoAposOCR` devolver o OCR em branco), Task 3 (voltar `linhaNativaPresente` para 60% de presença), Task 4 (filtro que não casa nada), Task 7 (tirar a linha `d.pendentes = nil`), Task 8 (voltar a regex para `\s+`).
- **Oráculo independente (Task 3).** O oráculo do teste de propriedade não pode chamar `linhaNativaPresente`, `palavrasDoTexto`, `conjuntoDePalavras` nem `normalizarPalavra`.
- **Movimento puro (Task 5).** Os geradores mudam de arquivo sem mudar uma linha: `git diff --color-moved=zebra` ou comparação das funções.
- **Limpeza que apaga o que não devia (Task 5).** O `TestMain` só apaga `filepath.Join(tempOriginal, "eprocia_testes_classes")`, nunca `%TEMP%\eprocIA_bin`.
- **Teto da Task 8.** Saída exatamente no teto passa; um byte acima falha com erro que não é `ErrSenha` nem `ErrPrazo`; o processo não fica pendurado (o escritor descarta, não devolve erro ao `exec`).
- **Renomeação (Task 7).** `aguardar` some do pacote; os dois chamadores (`pdf_parser.go:148`, `pdf_poppler.go:54`) usam o nome novo.

---

### Task 1: Testes unitários de ErrSenha e de cancelamento em rodarPoppler (P14, testes)

**Files:**
- Modify: `internal/ocr/main_test.go` (modo novo do binário de teste)
- Modify: `internal/ocr/poppler_test.go` (dois testes)

**Interfaces:** nenhuma de produção. No `TestMain`, com `EPROCIA_TESTE_SENHA` definida, o binário escreve `Command Line Error: Incorrect password` no stderr e sai com código 1, sem rodar teste.

- [ ] **Passo 1: testes.** Em `poppler_test.go`, no molde de `TestInfoPDF_Prazo` (`:148`: pasta temporária com `os.Args[0]` copiado como `pdfinfo.exe` via `copiarArquivo`):
  - `TestRodarPoppler_SenhaViraErrSenha`: `t.Setenv("EPROCIA_TESTE_SENHA", "1")`, `rodarPoppler(context.Background(), dir, "pdfinfo.exe", prazoInfo, "x.pdf")`. Asserções: `errors.Is(err, ErrSenha)`; `!errors.Is(err, ErrPrazo)`; a mensagem não contém `Incorrect password` (o stderr não vaza quando vira `ErrSenha`).
  - `TestRodarPoppler_CancelamentoNaoEPrazo`: `t.Setenv("EPROCIA_TESTE_DORMIR", "1")`, `ctx, cancel := context.WithCancel(...)`, `time.AfterFunc(200*time.Millisecond, cancel)`, prazo de 1 minuto. Asserções: `errors.Is(err, context.Canceled)`; `!errors.Is(err, ErrPrazo)`; voltou em menos de 5 s.
- [ ] **Passo 2: fase vermelha sem recursão.** Não rode o teste da senha sem o modo no `TestMain`: o binário copiado como `pdfinfo.exe` rodaria a suíte inteira de novo, e ela o copiaria de novo, em cascata. Primeiro acrescente o modo `EPROCIA_TESTE_SENHA` só com `os.Exit(1)` (sem escrever a frase), rode `go test ./internal/ocr -run "TestRodarPoppler_" -v` e veja o teste da senha falhar porque o erro não é `ErrSenha`. O do cancelamento já deve passar (é cobertura que faltava): registre no relatório.
- [ ] **Passo 3: implementar** a frase no stderr do modo `EPROCIA_TESTE_SENHA`, antes do `m.Run()`, ao lado de `EPROCIA_TESTE_DORMIR`.
- [ ] **Passo 4: rodar** os dois testes e o gate (`go vet ./...`, `go test -race ./internal/...`, `go test -race ./internal/ocr` sem `-short`).
- [ ] **Passo 5: commit** `test(ocr): ErrSenha e cancelamento em rodarPoppler sem o binário real [P14]`.

---

### Task 2: Asserções que faltam em internal/etl (P2, P4, P5, P12, CP7)

**Files:**
- Modify: `internal/etl/pipeline_leitura_test.go` (P2)
- Modify: `internal/etl/formatter_nao_lido_test.go` (P4)
- Modify: `internal/etl/queue_nao_lidos_test.go` (P5)
- Modify: `internal/etl/pdf_parser_lote_test.go` (P12)
- Modify: `internal/etl/pdf_ilegivel_test.go` (CP7)

**Interfaces:** nenhuma de produção. Helper renomeado: `esperarStatus` passa a `esperarItemComStatus` (mesma assinatura; único uso em `queue_nao_lidos_test.go:37`).

- [ ] **Passo 1 (P2).** Em `TestPipeline_EntradasDuplicadas_MotivoNaEntradaCerta` (`pipeline_leitura_test.go:41`), troque a asserção da linha 60 por quatro: `docs[0].Leitura == string(LeituraLido)`, `docs[0].Estado == DocConcluido`, `docs[1].Leitura == string(LeituraIlegivel)`, `docs[1].Estado == DocErro`.
- [ ] **Passo 2 (P4).** Em `TestSaida_PaginaSemOCRForaDasRegras` (`formatter_nao_lido_test.go:151`), a asserção (b) (`:177`) compara a página nativa inteira, o que depende de como `refluir` junta as linhas no nível `aggressive`. Troque por contagem das bordas, que não depende do refluxo: `strings.Count(saida, cab) == 6` e `strings.Count(saida, rod) == 6` (2 nativas que não podem perder a borda + 4 `sem_ocr`, que saem inteiras pela asserção (a)). Rode antes de mexer e confirme que a contagem de hoje é 6 nos dois níveis; se não for, pare com `NEEDS_CONTEXT` e o valor visto.
- [ ] **Passo 3 (P5).** Renomeie `esperarStatus` para `esperarItemComStatus` (o pacote já tem `esperarFim`, `esperarOcioso`, `esperarParsePages`; o nome novo diz que espera qualquer item da fila).
- [ ] **Passo 4 (P12).** Novo `TestOCREmLote_RestoPequenoNoFim` em `pdf_parser_lote_test.go`, no molde de `TestOCREmLote_TetoDePaginasPorBloco` (`:114`): `pdfComPaginas(t, 41, nil)`, `comOCREmLote(registroLote(&mu, &tamanhos), 2)`. Asserções: soma 41; maior `<= maxPaginasBlocoOCR`; `slices.Sorted` dos tamanhos igual a `[1 20 20]` (dois blocos cheios de 20 enquanto lê, e a página que sobra num bloco de 1 no `esperar`).
- [ ] **Passo 5 (CP7).** Novo `TestParsePages_IlegivelComOCREmBrancoFicaIlegivel` em `pdf_ilegivel_test.go`, no molde de `TestParsePages_IlegivelComOCRFalhoViraMarcador` (`:115`), com o OCR por página devolvendo `" \n", nil` (bloco de 1 página usa o OCR por página). Asserções: `content == marcadorIlegivel(strings.Repeat("x", 150))`; `ocr == false`; sem `ocr_erro`; `classificarPaginas(pages, false) == LeituraIlegivel` (com a reserva mantida pelo M2, o documento é `ilegivel`, não `sem_texto`: o teste fixa isso).
- [ ] **Passo 6: rodar** `go test ./internal/etl -run "EntradasDuplicadas|PaginaSemOCRForaDasRegras|ContaNaoLidos|RestoPequenoNoFim|IlegivelComOCREmBranco" -v`. Tudo passa (são testes de comportamento atual). Para cada um, quebre a produção numa cópia (ver Review Focus) e registre no relatório a falha vista.
- [ ] **Passo 7: gate** e **commit** `test(etl): asserções de leitura, bordas, resto do lote e reserva ilegível [P2, P4, P5, P12, CP7]`.

---

### Task 3: Oráculo independente no teste de propriedade da junção (CP6)

**Files:**
- Modify: `internal/etl/pdf_juncao_test.go`

**Interfaces:** helpers só de teste, neste arquivo:
- `palavrasOraculo(s string) []string`: corta `s` em sequências de letras e dígitos (`unicode.IsLetter || unicode.IsDigit`), guarda as de 3 runas ou mais, minúsculas e sem acento por `golang.org/x/text/unicode/norm` (NFD, tirando `unicode.Mn`) e recompõe. Não chama nada de `pdf_juncao.go` nem de `lingua.go`.

- [ ] **Passo 1: teste.** Em `TestJuntar_NuncaPerdeLinhaNativa` (`:78`), troque o laço que usa `linhaNativaPresente` (`:101-106`) por três propriedades, sobre o mesmo conjunto `casos`:
  1. **Nenhuma palavra perdida:** toda palavra de `palavrasOraculo(l)` de cada linha nativa está no conjunto `palavrasOraculo(got)`.
  2. **Linha com palavra fora do OCR entra inteira (CP3):** se alguma palavra de `palavrasOraculo(l)` não está em `palavrasOraculo(c.ocr)`, `l` aparece como linha de `got` (`slices.Contains(strings.Split(got, "\n"), l)`).
  3. **Sem duplicata:** para cada linha nativa `l`, o número de linhas de `got` iguais a `l` não passa do número de linhas iguais a `l` no nativo mais no OCR.
  Mantenha a verificação de ordem das linhas do OCR (`:108-119`). Acrescente a `casos` um caso CP3 explícito: OCR `"Valor da causa: R$"`, nativo `"Valor da causa: R$ 1.500,00"`.
- [ ] **Passo 2: rodar** `go test ./internal/etl -run TestJuntar -v`: passa. Depois, numa cópia, troque o `== len(palavras)` de `linhaNativaPresente` (`pdf_juncao.go:103`) por `*10 >= len(palavras)*6` (regra antiga de 60%) e confirme que a propriedade 1 ou 2 falha; desfaça. Registre no relatório.
- [ ] **Passo 3: gate** e **commit** `test(etl): oráculo próprio no teste de propriedade da junção [CP6]`.

---

### Task 4: Guarda de quantidade nos filtros das páginas sintéticas (CP8)

**Files:**
- Modify: `internal/etl/pdf_conteudo_perdido_test.go`

**Interfaces:** helper só de teste `sinteticasOnde(t *testing.T, minimo int, filtro func(paginaSintetica) bool) []paginaSintetica`, que filtra `paginasSinteticas()` e chama `t.Fatalf` se sobrarem menos de `minimo`.

- [ ] **Passo 1: contar.** Antes de mexer, conte quantas páginas cada filtro de hoje seleciona (`ControlesNaoVaoAoOCR` `:44`, `ImagemDescobertaJuntaOCRENativo` `:66`, `IscaTrocaPeloOCR` `:122`, `paginasComNativoEOCR` `:148` e qualquer outro laço sobre `paginasSinteticas()` com filtro no arquivo). Anote no relatório.
- [ ] **Passo 2: teste.** Troque cada laço com filtro por `sinteticasOnde(t, N, ...)`, com `N` = a quantidade contada. `paginasComNativoEOCR` passa a receber `t` e o mínimo. Em `TestConteudoPerdido_IscaTrocaPeloOCR`, tire o laço das palavras-isca (`:136-140`) e a variável `palavras` (`:121`): a igualdade `c != ocrFalsoTexto` já reprova qualquer palavra da isca no conteúdo. `palavrasIsca` continua usada em `pdf_sinteticos_test.go:201`: não a apague.
- [ ] **Passo 3: rodar** `go test ./internal/etl -run TestConteudoPerdido -v`: passa. Numa cópia, troque um filtro por um que não casa nada (`func(paginaSintetica) bool { return false }`) e confirme o `t.Fatalf`; desfaça.
- [ ] **Passo 4: gate** e **commit** `test(etl): filtros das páginas sintéticas com quantidade mínima [CP8]`.

---

### Task 5: Pasta dos testes de classes limpa no TestMain; geradores em arquivo próprio e casos em tabela (P16)

**Files:**
- Modify: `internal/etl/abertura_test.go` (`TestMain`, `:27`)
- Modify: `internal/etl/pdf_classes_test.go`
- Create: `internal/etl/pdf_classes_geradores_test.go`

**Interfaces:** nenhuma de produção. Os geradores (`pdfVersao20` `:135`, `pdfEmEnvelope` `:142`, `pdfTruncado` `:174`, `padSenha` `:184`, `rc4Iterado` `:199`, `md5Iterado` `:215`, `pdfAESV2` `:231`, `pdfAESV2Filtro` `:236`, e `objetosTexto`/`objetosDigitalizado` se o revisor preferir junto) vão sem mudança para `pdf_classes_geradores_test.go`, com os imports que precisarem.

- [ ] **Passo 1: limpeza.** No `TestMain`, troque `os.Exit(m.Run())` por `codigo := m.Run()`, `os.RemoveAll(filepath.Join(tempOriginal, "eprocia_testes_classes"))` e `os.Exit(codigo)`. Comentário em pt-BR: a pasta é fixa durante o processo por causa do cache `binDir`/`popplerDir` do pacote `ocr` (ver `tempClasses`), e sai no fim para não deixar dezenas de MB na máquina. Atualize o comentário de `tempClasses` (`pdf_classes_test.go:35-40`) dizendo que o `TestMain` a apaga.
- [ ] **Passo 2: verificar a limpeza.** Rode `go test ./internal/etl -run TestClasses -v` (sem `-short`) e confira que `%TEMP%\eprocia_testes_classes` não existe depois. Antes da mudança ela existe: registre os dois resultados.
- [ ] **Passo 3: mover os geradores.** Recorte as funções listadas para o arquivo novo, sem mudar nenhuma linha.
- [ ] **Passo 4: tabela.** Junte `TestClasses_Versao20`, `TestClasses_Envelope`, `TestClasses_AESV2SemSenha`, `TestClasses_AESV2ComSenha`, `TestClasses_Truncado` e `TestClasses_TabelaInconsistente` (`:388-468`) numa tabela `TestClasses_PorClasse` com subtestes de mesmo nome (`Versao20`, `Envelope`, ...). Campos: `pdf func(t *testing.T) []byte`, `preparar func(t *testing.T)` (só o da tabela inconsistente, que troca `contarPaginasPDF` com `t.Cleanup`), `leitura Leitura`, `naoLidos int`, `temFrase bool`, `marcador string` (vazio = não confere). As asserções de cada caso ficam as mesmas de hoje, inclusive "o texto cifrado não vaza" no caso com senha. `TestClasses_Versao20Digitalizado`, `TestClasses_GeradoresRecusadosPelaBiblioteca` e `TestClasses_AESV2NaFormaDaNormaAbreNaBiblioteca` ficam como estão.
- [ ] **Passo 5: rodar** `go test ./internal/etl -run TestClasses -v`: os mesmos casos de antes passam (compare a lista de subtestes com a dos testes antigos no relatório).
- [ ] **Passo 6: gate** e **commit** `test(etl): pasta dos testes de classes limpa no fim e casos em tabela [P16]`.

---

### Task 6: Captura do rótulo de não lidos com o divisor no máximo (P6)

**Files:**
- Modify: `scripts/visual/capturar.mjs`
- Create: capturas em `docs/revisao/capturas/2026-11-22/P6/` (só as de `nao-lidos`)

**Interfaces:** o bloco do divisor (`capturar.mjs:178-188`) passa a rodar também para `nome === 'nao-lidos'`, só o `-divisor-maximo` (o mínimo e o recolhido seguem só em `processando`).

Sem TDD: o script não tem teste unitário (o `estados.test.mjs` cobre só os estados). O critério é a captura.

- [ ] **Passo 1: script.** Acrescente a condição; comentário em pt-BR dizendo que o `nao-lidos` ganha o divisor no máximo para conferir o rótulo numa linha estreita.
- [ ] **Passo 2: capturar** `node scripts/visual/capturar.mjs --saida docs/revisao/capturas/2026-11-22/P6 --estados nao-lidos` (leia `.claude/skills/eprocia-visual/SKILL.md` antes; o script sobe o Vite e encerra no fim; confira pelo PID que nada ficou rodando).
- [ ] **Passo 3: conferir** com a ferramenta Read as imagens `800x600-*-nao-lidos-divisor-maximo.png` e `1024x768-*-nao-lidos-divisor-maximo.png`. Registre no relatório, para cada uma: o rótulo "N não lidos" aparece inteiro, aparece cortado, ou some (a linha quebra e esconde a segunda faixa por desenho, `ProcessList.svelte:1300-1325`). Se cortar ou sumir, **não mexa no CSS**: descreva e responda `DONE_WITH_CONCERNS`; vira decisão do dono.
- [ ] **Passo 4: gate** `node --test scripts/visual/estados.test.mjs` e `node --test frontend/*.test.mjs`. **Commit** do script e das capturas `.png` (pelos nomes; `git add docs/revisao/capturas/2026-11-22/P6/*.png` com o caminho explícito da pasta): `chore(visual): captura de não lidos com o divisor no máximo [P6]`.

---

### Task 7: esperar() zera os pendentes; aguardar vira esperarDespachados (P13)

**Files:**
- Modify: `internal/etl/pdf_ocr_despacho.go` (`:101-116`)
- Modify: `internal/etl/pdf_parser.go` (`:148`)
- Modify: `internal/etl/pdf_poppler.go` (`:54`)
- Modify: `internal/etl/pdf_parser_lote_test.go` (teste novo)

**Interfaces:**
- `func (d *despachoOCR) esperarDespachados()` (era `aguardar`): espera os blocos já despachados, sem despachar a sobra. Usada só no cancelamento.
- `func (d *despachoOCR) esperar() map[string]any`: sem mudança de assinatura; depois de despachar a sobra, faz `d.pendentes = nil`, então uma segunda chamada não despacha de novo.

- [ ] **Passo 1: teste.** `TestDespachoOCR_EsperarDuasVezesNaoRedespacha` em `pdf_parser_lote_test.go`: `p := parserDeArquivo(t, pdfComPaginas(t, 3, nil)).comOCREmLote(registroLote(&mu, &tamanhos), 2)`; OCR por página que conta chamadas num `atomic.Int32` e devolve `"ocr"`; `d := p.novoDespachoOCR(context.Background(), 3, porPagina, nil)`; `d.agendar(ocrTask{pageNum: 1})` (fica pendente: `tamBloco` é 2); `d.esperar()` duas vezes. Asserções: OCR por página chamado 1 vez (o bloco de 1 página usa o OCR por página, `runOCRBloco` `:185`); lote nunca chamado; página `"1"` no mapa com `content == "ocr"`.
- [ ] **Passo 2: rodar** `go test ./internal/etl -run TestDespachoOCR_ -v`: falha com 2 chamadas.
- [ ] **Passo 3: implementar** `d.pendentes = nil` dentro do `if` de `esperar()`, depois do laço; renomeie `aguardar` para `esperarDespachados` e os dois chamadores; ajuste os comentários das duas funções (uma diz que despacha a sobra e pode ser chamada de novo sem efeito; a outra, que é a do cancelamento e não despacha a sobra).
- [ ] **Passo 4: rodar** o teste novo e o gate (`go vet ./...`, `go test -race ./internal/...`). Confira com Grep que `aguardar()` não aparece mais em `internal/etl`.
- [ ] **Passo 5: commit** `fix(etl): esperar() não redespacha a sobra; aguardar vira esperarDespachados [P13]`.

---

### Task 8: Regex do pdfinfo sem quebra de linha e teto na saída do Poppler (P14, comportamento)

**Files:**
- Modify: `internal/ocr/poppler.go` (`:21-36`, `:24`, `:50-75`)
- Modify: `internal/ocr/main_test.go` (modo novo do binário de teste)
- Modify: `internal/ocr/poppler_test.go` (testes)

**Interfaces:**
- `reNumPaginas = regexp.MustCompile(`(?m)^Pages:[ \t]+(\d+)[ \t]*$`)`.
- `var maxSaidaPoppler = 64 << 20` (variável para o teste encurtar; comentário: teto do stdout de um processo do Poppler, muito acima de um documento real e abaixo de um uso de memória que derrube o app).
- `var errSaidaGrande = errors.New("saída do Poppler acima do limite")`.
- `limitado` ganha o teto como campo e uma marca de corte: `type limitado struct{ buf bytes.Buffer; max int; cortou bool }`. `Write` guarda até `max` bytes, marca `cortou` quando descarta algo e sempre devolve `len(p), nil` (descartar em vez de devolver erro: o `exec` não fecha o pipe no meio e o processo termina sozinho, dentro do prazo). O stderr usa `max: maxStderr`; o stdout de `rodarPoppler` usa `max: maxSaidaPoppler`.
- Em `rodarPoppler`, sem erro do processo e com `saida.cortou`, devolve `fmt.Errorf("falha no %s: %w (%d bytes)", exe, errSaidaGrande, maxSaidaPoppler)`. Erro do processo continua tendo prioridade (senha, prazo, stderr).
- No `TestMain`: com `EPROCIA_TESTE_JORRAR=<n>`, o binário escreve `n` bytes `x` no stdout e sai com 0.

- [ ] **Passo 1: testes.**
  - `TestReNumPaginas`: tabela com `FindStringSubmatch` direto: `"Pages:          3\n"` dá `"3"`; `"Title: x\nPages: \t 7  \n"` dá `"7"`; `"Pages:\n12\n"` não casa; `"Pages:\r\n12"` (depois do `ReplaceAll` de `\r` que `InfoPDF` faz, vira `"Pages:\n12"`) não casa.
  - `TestRodarPoppler_SaidaNoTetoPassa` e `TestRodarPoppler_SaidaAcimaDoTetoFalha`: pasta temporária com `os.Args[0]` copiado como `pdftotext.exe`; `maxSaidaPoppler` trocado por 1024 com `t.Cleanup`; `EPROCIA_TESTE_JORRAR` = `1024` e `1025`. Asserções: com 1024, sem erro e `len(saida) == 1024`; com 1025, `errors.Is(err, errSaidaGrande)`, `!errors.Is(err, ErrSenha)`, `!errors.Is(err, ErrPrazo)`, e volta em menos de 5 s.
- [ ] **Passo 2: rodar** `go test ./internal/ocr -run "TestReNumPaginas|TestRodarPoppler_Saida" -v`: `"Pages:\n12\n"` casa (falha), e os de teto não compilam (sem `maxSaidaPoppler`). Como na Task 1, o modo `EPROCIA_TESTE_JORRAR` entra no `TestMain` antes de qualquer execução que copie o binário de teste, ou ele roda a suíte em cascata.
- [ ] **Passo 3: implementar** a regex, o campo de teto em `limitado`, o modo `EPROCIA_TESTE_JORRAR` e o erro em `rodarPoppler`.
- [ ] **Passo 4: rodar** os testes novos, os de `poppler_test.go` com o binário real (`TestInfoPDF_ContaPaginas`, `TestTextoPDF_*`) e o gate (`go vet ./...`, `go test -race ./internal/...`, `go test -race ./internal/ocr` sem `-short`).
- [ ] **Passo 5: relatório** com o efeito no pipeline de um PDF acima do teto: `TextoPDF` falha, e `parsePoppler` (`internal/etl/pdf_poppler.go:38-47`) devolve erro com o OCR desligado ou manda todas as páginas ao OCR com ele ligado. Não mude esse caminho.
- [ ] **Passo 6: commit** `fix(ocr): regex do pdfinfo sem quebra de linha e teto na saída do Poppler [P14]`.
