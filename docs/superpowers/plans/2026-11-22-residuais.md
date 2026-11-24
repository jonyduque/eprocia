# Rodada de residuais (2026-11-22): plano de implementação

> **Para agentes:** sub-skill obrigatória: use superpowers:subagent-driven-development (recomendado) ou superpowers:executing-plans para implementar este plano tarefa por tarefa. Os passos usam checkbox (`- [ ]`) para acompanhamento. Regras comuns a todo implementador e revisor: `docs/superpowers/plans/2026-11-22-residuais-regras.md`.

**Goal:** Fechar os residuais de `docs/revisao/2026-11-14-residuais-pdfs-nao-lidos.md` que ainda valem e não mudam o que o usuário vê: asserções e oráculos de teste que faltam, a pasta temporária dos testes de classes, duas mudanças no script de capturas (P6, RB3), duas correções pequenas de produção em Go (`esperar()` que despacharia duas vezes; regex do `pdfinfo` e teto de memória na saída do Poppler) e duas no frontend sem mudança de texto (contador único da atividade, RB2; anúncios do mesmo tick que se perdem, RB1). O que muda texto do `.txt`, UI ou notificação fica na seção "Decisões do dono", sem tarefa.

**Architecture:** Nenhuma mudança de arquitetura. Tasks 1 a 5 só mexem em testes; 6 e 7 em `scripts/visual/`. Task 8 muda `internal/etl/pdf_ocr_despacho.go` (zera `d.pendentes` em `esperar()`, renomeia `aguardar`). Task 9 muda `internal/ocr/poppler.go` (regex `Pages:` sem quebra de linha, teto de bytes no stdout de `rodarPoppler`). Task 10 extrai o contador de `frontend/src/lib/atividade.js`. Task 11 cria `frontend/src/lib/regiaoViva.js` e troca as três funções `anunciar` por ele.

**Tech Stack:** Go 1.27.3, `github.com/dslipak/pdf` v0.0.2, Poppler e tesseract embutidos (`internal/ocr/embed/`), Svelte 5 em sintaxe legada, `node --test`, Playwright (só na Task 6, via `scripts/visual/capturar.mjs`). Sem dependência nova (`golang.org/x/text` já é direta em `go.mod:13`).

**Spec:** `docs/revisao/2026-11-14-residuais-pdfs-nao-lidos.md` (ids `M`, `P`, `CP` e `RB`). Já decidido sobre eles: `docs/revisao/2026-11-18-plano-correcao.md` §5.

## Situação de cada item

Conferido no código da `main` `aa9e066`. Os números de P4, P12, P13 e CP7 foram confirmados por sonda numa cópia (revisão adversarial de 2026-11-22).

| Id | Classificação | Gravidade / custo | Onde fica |
|----|---------------|-------------------|-----------|
| M1 | decisão do dono já registrada (muda a aparência de um documento lido; fixada por `TestPipeline_SemOCR_TextoNativoCurtoComImagemContaComoLido`) | - | fora |
| M5 | precisa de decisão do dono (muda o painel) | média / M | d1 |
| M7 (3 notas) | sem ação: prefixo `[ERRO`, filho de processo morto, `readerAt` nulo; o próprio arquivo diz "sem ação pedida" e §5 (:223) só reabre com caso real | - | fora |
| P3 | já resolvido no Go e na lógica do frontend; sobra o rótulo de leitor de tela | baixa / P | d4 |
| P4 | vale (asserção frágil); **desvio de §5** (:221 diz "não vale"), ver nota abaixo | baixa / P | Task 2 |
| P6 | vale, só conferência visual | baixa / P | Task 6 |
| P7 | não vale: mapas por id sem poda, inofensivo (§5) | - | - |
| P9 | não vale: refatoração sem ganho, 8 usos em teste (§5) | - | - |
| P10 | não vale: produção só usa `WithCancel`; nenhum `WithTimeout` fora de `internal/ocr/prazo.go:37` | - | - |
| P14 (comportamento) | vale: regex e teto de saída. A trava do `initMu` e o log duplicado não valem (só latência na primeira chamada; §5) | baixa / P | Task 9 |
| P18 | precisa de decisão do dono (muda a linha e a notificação) | baixa / P | d2 |
| CP2 | não vale agora: sem página-isca real para medir (§5) | - | - |
| CP9 | precisa de decisão do dono (muda o `.txt`) | baixa / P | d3 |
| CP4, CP5 | condicionais ao CP10 (medição em curso noutra frente) | - | sem tarefa |
| P2 | vale | baixa / P | Task 2 |
| P5 | vale | baixa / P | Task 2 |
| P8 | não vale: sem navegador nos testes, não há alternativa barata à regex (§5) | - | - |
| P11 | já resolvido: `TestRunOCR_FallbackSemPortugues` (`internal/ocr/tesseract_test.go:332`, commit `d9b91f7`) faz o tesseract com `-l por` falhar sem prazo e confere 3 processos e o texto do fallback; RunOCR e RunOCRPaginas passam pelo mesmo `reconhecer` (`tesseract.go:379`, ramo em `:415-427`) | - | - |
| P12 | vale (teste que falta; o `min` de `pdf_ocr_despacho.go:109` fica como defesa) | baixa / P | Task 2 |
| P13 | vale (produção trivial) | baixa / P | Task 8 |
| P14 (testes) | vale | baixa / P | Task 1 |
| P16 | vale | baixa / M | Task 5 |
| CP6 | vale | média / M | Task 3 |
| CP7 | vale (teste que fixa o comportamento atual) | baixa / P | Task 2 |
| CP8 | vale | baixa / P | Task 4 |
| RB1 | vale: defeito de leitor de tela (`anunciar()` sem fila em `ProcessList.svelte:97-102`, `App.svelte:279-284`, `Settings.svelte:130-134`); o conserto não muda texto nenhum na tela nem a redação de nenhuma mensagem, só deixa de perder a primeira de duas do mesmo tick | média / P | Task 11 |
| RB2 | vale (limpeza: `ordenarDocs` sem uso em produção; `resumoAtividade` e `prepararDocs` repetem o laço de contagem, `atividade.js:49-63` e `:77-92`) | baixa / P | Task 10 |
| RB3 | vale (configuração morta: `review.json:105` `alto_contraste` não é lido; `capturar.mjs:21` repete a lista) | baixa / P | Task 7 |

**Nota sobre o P4 (desvio de §5).** O plano de 2026-11-18 (§5, :221) pôs o P4 em "não vale: nota sobre o teste, sem defeito". Mantenho como tarefa porque a asserção (b) de `TestSaida_PaginaSemOCRForaDasRegras` compara a página nativa inteira e passa hoje só porque `refluir` não junta essas linhas no nível `aggressive`: uma mudança legítima no refluxo reprovaria o teste sem defeito nenhum na regra que ele protege (bordas das páginas `sem_ocr` fora da contagem). O conserto são duas linhas, a sonda confirmou a contagem (6 cabeçalhos e 6 rodapés nos dois níveis) e cabe na Task 2 sem custo de rodada. Se o dono preferir manter a decisão de §5, o Passo 2 da Task 2 sai sem afetar o resto.

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

**d3 (CP9).** `HasImages` (`internal/etl/pdf_parser.go:586`) não tem `recover`; numa página de texto curto com XObject de tabela inconsistente, o pânico cai no `recover` de `analisarPagina` (`:355-360`) e a página sai `[ERRO ao ler página: ...]` (documento de uma página fica não lido).
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
- Nenhuma tarefa muda texto do `.txt`, rótulo, aviso, notificação, a redação de um anúncio, valor de `Leitura` ou formato PRD §6. A Task 9 só muda o resultado de um PDF cuja saída do `pdftotext` passe de 64 MiB (hoje: memória sem teto). A Task 11 só muda quantos anúncios chegam ao leitor de tela quando dois caem no mesmo tick.
- `go.mod`, `go.sum` e `internal/ocr/embed/` não mudam. `frontend/src/wailsjs/` não muda.
- Tasks 1 a 7 não mudam código de produção (`*.go` fora de `_test.go`, `frontend/src/**`). Se uma delas exigir, pare com `NEEDS_CONTEXT`.
- Linhas de log novas em pt-BR, só com números e textos fixos (sem caminho, sem texto de página).
- Uma tarefa por vez; no máximo 3 agentes ao mesmo tempo na máquina.
- Todo teste que copia o binário de teste (`os.Args[0]`) para imitar um executável define `EPROCIA_TESTE_FILHO=1` junto do modo (ver Task 1). Sem isso, um filho sem modo conhecido roda a suíte inteira e se copia de novo, em cascata.

## Review Focus

- **Teste que passa sem testar.** Em cada teste novo, o revisor quebra a produção de propósito (numa cópia, sem commit) e confere que o teste falha: Task 2 (P12: trocar `maxPaginasBlocoOCR` por 25, o lote vira `[21 20]` e o OCR por página 0; CP7: fazer `conteudoAposOCR` devolver o OCR em branco), Task 3 (voltar `linhaNativaPresente` para 60% de presença; e fazê-la devolver sempre `false`, que insere toda linha nativa), Task 4 (filtro que não casa nada), Task 8 (tirar a linha `d.pendentes = nil`), Task 9 (voltar a regex para `\s+`), Task 10 (contar `erros` diferente num dos laços, se ainda existirem dois), Task 11 (voltar `ProcessList` para o `anunciar` antigo e conferir que o teste de `regiaoViva` não pega, mas o teste de convenção pega).
- **Oráculo independente (Task 3).** O oráculo do teste de propriedade não pode chamar `linhaNativaPresente`, `palavrasDoTexto`, `conjuntoDePalavras` nem `normalizarPalavra`.
- **Movimento puro (Task 5).** Os geradores mudam de arquivo sem mudar uma linha: `git diff --color-moved=zebra` ou comparação das funções.
- **Limpeza que apaga o que não devia (Task 5).** O `TestMain` só apaga a pasta que o próprio processo criou com `os.MkdirTemp` (`eprocia_testes_classes_*`), e só se a criou. Nunca `%TEMP%\eprocIA_bin`, nunca a pasta de outro processo, nunca a pasta fixa antiga `eprocia_testes_classes`.
- **Cascata do binário de teste (Tasks 1 e 9).** `TestMain` sai com código 2 quando `EPROCIA_TESTE_FILHO` vem sem modo conhecido; todo ponto que copia `os.Args[0]` define as duas variáveis.
- **Teto da Task 9.** Saída exatamente no teto passa; um byte acima falha com erro que não é `ErrSenha` nem `ErrPrazo`; o processo não fica pendurado (o escritor descarta, não devolve erro ao `exec`); o stdout devolvido não passa por `TrimSpace`.
- **Renomeação (Task 8).** `aguardar` some do pacote; os dois chamadores (`pdf_parser.go:148`, `pdf_poppler.go:54`) usam o nome novo.
- **Texto igual (Tasks 10 e 11).** Nenhuma string visível ou anunciada muda; `svelte-check`, `npm run build` e todo `frontend/*.test.mjs` passam sem afrouxar regex.

---

### Task 1: Defesa contra cascata do binário de teste; ErrSenha e cancelamento em rodarPoppler (P14, testes)

**Files:**
- Modify: `internal/ocr/main_test.go` (modos novos do binário de teste)
- Modify: `internal/ocr/poppler_test.go` (dois testes; `TestInfoPDF_Prazo` ganha `EPROCIA_TESTE_FILHO`)
- Modify: `internal/ocr/prazo_test.go` (`presos` ganha `EPROCIA_TESTE_FILHO`)

**Interfaces:** nenhuma de produção. No `TestMain`, antes do `m.Run()`:
- `EPROCIA_TESTE_DORMIR` (já existe): dorme 1 minuto e sai com 0.
- `EPROCIA_TESTE_SENHA` (novo): escreve `Command Line Error: Incorrect password` no stderr e sai com 1.
- `EPROCIA_TESTE_FILHO` (novo) definida sem nenhum modo conhecido: escreve `binário de teste chamado como filho sem modo` no stderr e sai com 2, sem rodar teste. É a defesa contra a cascata; vem depois dos modos.

- [ ] **Passo 1: defesa.** Acrescente o ramo `EPROCIA_TESTE_FILHO` ao `TestMain` e `t.Setenv("EPROCIA_TESTE_FILHO", "1")` em `presos` (`prazo_test.go:22`) e em `TestInfoPDF_Prazo` (`poppler_test.go:148`). Rode `go test ./internal/ocr -run "Prazo" -v`: passa como antes (o modo `DORMIR` vem antes da defesa).
- [ ] **Passo 2: testes.** Em `poppler_test.go`, no molde de `TestInfoPDF_Prazo` (pasta temporária com `os.Args[0]` copiado como `pdfinfo.exe` via `copiarArquivo`), sempre com `EPROCIA_TESTE_FILHO=1`:
  - `TestRodarPoppler_SenhaViraErrSenha`: `t.Setenv("EPROCIA_TESTE_SENHA", "1")`, `rodarPoppler(context.Background(), dir, "pdfinfo.exe", prazoInfo, "x.pdf")`. Asserções: `errors.Is(err, ErrSenha)`; `!errors.Is(err, ErrPrazo)`; a mensagem não contém `Incorrect password`.
  - `TestRodarPoppler_CancelamentoNaoEPrazo`: `t.Setenv("EPROCIA_TESTE_DORMIR", "1")`, `ctx, cancel := context.WithCancel(...)`, `time.AfterFunc(200*time.Millisecond, cancel)`, prazo de 1 minuto. Asserções: `errors.Is(err, context.Canceled)`; `!errors.Is(err, ErrPrazo)`; voltou em menos de 5 s.
- [ ] **Passo 3: rodar** `go test ./internal/ocr -run "TestRodarPoppler_" -v`. O da senha falha (o filho sai com 2 pela defesa; o erro não é `ErrSenha`), sem cascata. O do cancelamento já passa (cobertura que faltava): registre no relatório.
- [ ] **Passo 4: implementar** o modo `EPROCIA_TESTE_SENHA`.
- [ ] **Passo 5: rodar** os dois testes e o gate (`go vet ./...`, `go test -race ./internal/...`, `go test -race ./internal/ocr` sem `-short`).
- [ ] **Passo 6: commit** `test(ocr): ErrSenha e cancelamento em rodarPoppler sem o binário real [P14]`.

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
- [ ] **Passo 2 (P4; desvio de §5 justificado na nota do topo).** Em `TestSaida_PaginaSemOCRForaDasRegras` (`formatter_nao_lido_test.go:151`), troque a asserção (b) (`:177`) por `strings.Count(saida, cab) == 6` e `strings.Count(saida, rod) == 6` (2 nativas que não podem perder a borda + 4 `sem_ocr`, que saem inteiras pela asserção (a)). A sonda viu 6 e 6 nos dois níveis; se você vir outro valor, pare com `NEEDS_CONTEXT`.
- [ ] **Passo 3 (P5).** Renomeie `esperarStatus` para `esperarItemComStatus` (o pacote já tem `esperarFim`, `esperarOcioso`, `esperarParsePages`; o nome novo diz que espera qualquer item da fila).
- [ ] **Passo 4 (P12).** Novo `TestOCREmLote_RestoPequenoNoFim` em `pdf_parser_lote_test.go`, no molde de `TestOCREmLote_TetoDePaginasPorBloco` (`:114`): `pdfComPaginas(t, 41, nil)`, `comOCREmLote(registroLote(&mu, &tamanhos), 2)` e um OCR por página que conta chamadas num `atomic.Int32` e devolve `"", nil`. A sobra de 1 página vai ao OCR por página, não ao lote (`runOCRBloco`, `pdf_ocr_despacho.go:184-186`). Asserções: `tamanhos` (do lote) igual a `[20 20]`, em qualquer ordem; OCR por página chamado exatamente 1 vez; soma do lote mais as chamadas por página igual a 41. Não use "maior `<= maxPaginasBlocoOCR`": com a constante trocada, ela compara com o valor já trocado e não pega nada.
- [ ] **Passo 5 (CP7).** Novo `TestParsePages_IlegivelComOCREmBrancoFicaIlegivel` em `pdf_ilegivel_test.go`, no molde de `TestParsePages_IlegivelComOCRFalhoViraMarcador` (`:115`), com o OCR por página devolvendo `" \n", nil` (bloco de 1 página usa o OCR por página). Asserções: `content == marcadorIlegivel(strings.Repeat("x", 150))`; `ocr == false`; sem `ocr_erro`; `classificarPaginas(pages, false) == LeituraIlegivel` (com a reserva mantida pelo M2, o documento é `ilegivel`, não `sem_texto`: o teste fixa isso).
- [ ] **Passo 6: rodar** `go test ./internal/etl -run "EntradasDuplicadas|PaginaSemOCRForaDasRegras|ContaNaoLidos|RestoPequenoNoFim|IlegivelComOCREmBranco" -v`. Tudo passa (são testes de comportamento atual). Para cada um, quebre a produção numa cópia (ver Review Focus) e registre no relatório a falha vista.
- [ ] **Passo 7: gate** e **commit** `test(etl): asserções de leitura, bordas, resto do lote e reserva ilegível [P2, P4, P5, P12, CP7]`.

---

### Task 3: Oráculo independente no teste de propriedade da junção (CP6)

**Files:**
- Modify: `internal/etl/pdf_juncao_test.go`

**Interfaces:** helper só de teste, neste arquivo:
- `palavrasOraculo(s string) []string`: corta `s` em sequências de letras e dígitos (`unicode.IsLetter || unicode.IsDigit`), guarda as de 3 runas ou mais, minúsculas e sem acento por `golang.org/x/text/unicode/norm` (NFD, tirando `unicode.Mn`) e recompõe. Não chama nada de `pdf_juncao.go` nem de `lingua.go`.

- [ ] **Passo 1: teste.** Em `TestJuntar_NuncaPerdeLinhaNativa` (`:78`), troque o laço que usa `linhaNativaPresente` (`:101-106`) por três propriedades, sobre o mesmo conjunto `casos`. Seja `ocrPal = palavrasOraculo(c.ocr)` (como conjunto) e `linhasGot = strings.Split(got, "\n")`:
  1. **Nenhuma palavra perdida:** toda palavra de `palavrasOraculo(l)` de cada linha nativa `l` está no conjunto `palavrasOraculo(got)`.
  2. **Linha com palavra fora do OCR entra inteira (CP3):** se alguma palavra de `palavrasOraculo(l)` não está em `ocrPal`, `l` aparece em `linhasGot`.
  3. **Linha já coberta pelo OCR não entra (recíproca da 2):** se `palavrasOraculo(l)` não é vazio e todas as suas palavras estão em `ocrPal`, o número de vezes que `l` aparece em `linhasGot` não passa do número de vezes que aparece em `linhasDoOCR(c.ocr)` (a linha só pode estar em `got` porque o OCR a trouxe igual). Linhas sem palavra de 3 runas (ex.: `"12"`) ficam fora desta propriedade.
  Mantenha a verificação de ordem das linhas do OCR (`:108-119`). Não acrescente caso CP3 novo: o de `:25` (`"Valor total: R$"` / `"Valor total: R$ 1.500,00"`) já reprova a regra de 60% na propriedade 1 (o `500` se perde).
- [ ] **Passo 2: rodar** `go test ./internal/etl -run TestJuntar -v`: passa. Depois, numa cópia, (i) troque o `== len(palavras)` de `linhaNativaPresente` (`pdf_juncao.go:103`) por `*10 >= len(palavras)*6` (regra antiga de 60%) e confirme que a propriedade 1 ou 2 falha; (ii) faça `linhaNativaPresente` devolver sempre `false` e confirme que a propriedade 3 falha (casos `:19`, `:26`, `:33-35`, em que a linha nativa difere do OCR só em caixa ou acento). Desfaça. Registre no relatório.
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

### Task 5: Pasta dos testes de classes por processo e apagada no fim; geradores em arquivo próprio e casos em tabela (P16)

**Files:**
- Modify: `internal/etl/abertura_test.go` (`TestMain`, `:27`)
- Modify: `internal/etl/pdf_classes_test.go`
- Create: `internal/etl/pdf_classes_geradores_test.go`

**Interfaces:** nenhuma de produção.
- Em `pdf_classes_test.go`: `var dirClasses string` e `var criarDirClasses sync.Once`. `tempClasses(t)` cria a pasta uma vez por processo com `os.MkdirTemp(tempOriginal, "eprocia_testes_classes_*")` (erro: `t.Fatalf`) e aponta `TMP`/`TEMP`/`TMPDIR` para ela como hoje. A limpeza das sobras `eprocia_pdf_*` (`:47-50`) sai: a pasta nasce vazia em cada processo.
- No `TestMain`: `codigo := m.Run()`; se `dirClasses != ""`, `os.RemoveAll(dirClasses)`; `os.Exit(codigo)`. Só apaga a pasta que este processo criou: outra execução (outra worktree, o checkout principal, um medidor em paralelo) tem a sua. A pasta fixa antiga `%TEMP%\eprocia_testes_classes` não é apagada pelo código (um checkout com o código velho pode estar usando); o relatório da tarefa diz ao dono que ela pode ser apagada à mão uma vez.
- Os geradores (`pdfVersao20` `:135`, `pdfEmEnvelope` `:142`, `pdfTruncado` `:174`, `padSenha` `:184`, `rc4Iterado` `:199`, `md5Iterado` `:215`, `pdfAESV2` `:231`, `pdfAESV2Filtro` `:236`) vão sem mudança para `pdf_classes_geradores_test.go`, com os imports que precisarem.

- [ ] **Passo 1: o antes.** Anote no relatório se `%TEMP%\eprocia_testes_classes` existe agora e quais `%TEMP%\eprocia_testes_classes_*` existem (devem ser nenhuma).
- [ ] **Passo 2: pasta por processo e limpeza.** Implemente `dirClasses`, o `sync.Once` em `tempClasses` e o `TestMain`. Atualize o comentário de `tempClasses` (`:35-40`): a pasta continua fixa durante o processo (o cache `binDir`/`popplerDir` do pacote `ocr` guarda o primeiro diretório de extração), mas é própria de cada processo e sai no `TestMain`.
- [ ] **Passo 3: o depois.** Rode `go test ./internal/etl -run TestClasses -v` (sem `-short`) e confira que nenhuma `%TEMP%\eprocia_testes_classes_*` sobrou. Registre.
- [ ] **Passo 4: mover os geradores.** Recorte as funções listadas para o arquivo novo, sem mudar nenhuma linha.
- [ ] **Passo 5: tabela.** Junte `TestClasses_Versao20`, `TestClasses_Envelope`, `TestClasses_AESV2SemSenha`, `TestClasses_AESV2ComSenha`, `TestClasses_Truncado` e `TestClasses_TabelaInconsistente` (`:388-468`) numa tabela `TestClasses_PorClasse` com subtestes de mesmo nome (`Versao20`, `Envelope`, ...). Campos: `pdf func(t *testing.T) []byte`; `preparar func(t *testing.T)` (só o da tabela inconsistente, que troca `contarPaginasPDF` com `t.Cleanup`); `leitura Leitura`; `naoLidos int`; `frase` de um tipo com três valores, `fraseNaoConfere` (Truncado), `fraseNoTxt` (Versao20, Envelope, AESV2SemSenha, TabelaInconsistente) e `fraseFora` (AESV2ComSenha: "o texto cifrado vazou"); `marcador string` (vazio = não confere). Cada caso fica com exatamente as asserções de hoje, nem mais nem menos. `TestClasses_Versao20Digitalizado`, `TestClasses_GeradoresRecusadosPelaBiblioteca` e `TestClasses_AESV2NaFormaDaNormaAbreNaBiblioteca` ficam como estão.
- [ ] **Passo 6: rodar** `go test ./internal/etl -run TestClasses -v`: os mesmos casos de antes passam (compare no relatório a lista de subtestes com a dos testes antigos).
- [ ] **Passo 7: gate** e **commit** `test(etl): pasta dos testes de classes por processo, apagada no fim, e casos em tabela [P16]`.

---

### Task 6: Captura do rótulo de não lidos com o divisor no máximo (P6)

**Files:**
- Modify: `scripts/visual/capturar.mjs`
- Create: só as capturas `*-nao-lidos-divisor-maximo.png` em `docs/revisao/capturas/2026-11-22/P6/`

**Interfaces:** o bloco do divisor (`capturar.mjs:178-188`) passa a rodar também para `nome === 'nao-lidos'`, só o `-divisor-maximo` (o mínimo e o recolhido seguem só em `processando`).

Sem TDD: o script não tem teste unitário (o `estados.test.mjs` cobre só os estados). O critério é a captura. Gate desta tarefa, por exceção às regras (o script não entra no bundle do app): `node --test scripts/visual/estados.test.mjs` e `node --test frontend/*.test.mjs`; sem `svelte-check` nem `npm run build`.

- [ ] **Passo 1: script.** Acrescente a condição; comentário em pt-BR dizendo que o `nao-lidos` ganha o divisor no máximo para conferir o rótulo numa linha estreita.
- [ ] **Passo 2: capturar** `node scripts/visual/capturar.mjs --saida <scratchpad>/P6 --estados nao-lidos` (leia `.claude/skills/eprocia-visual/SKILL.md` antes; o script sobe o Vite e encerra no fim; confira pelo PID que nada ficou rodando). O estado gera ~33 imagens; só as `*-nao-lidos-divisor-maximo.png` (8: 4 tamanhos × 2 temas) vão para `docs/revisao/capturas/2026-11-22/P6/`.
- [ ] **Passo 3: conferir** com a ferramenta Read as imagens `800x600-*-nao-lidos-divisor-maximo.png` e `1024x768-*-nao-lidos-divisor-maximo.png`. Registre no relatório, para cada uma: o rótulo "N não lidos" aparece inteiro, aparece cortado, ou some (a linha quebra e esconde a segunda faixa por desenho, `ProcessList.svelte:1300-1325`). Se cortar ou sumir, **não mexa no CSS**: descreva e responda `DONE_WITH_CONCERNS`; vira decisão do dono.
- [ ] **Passo 4: gate** (acima) e **commit** do script e das 8 capturas, cada `.png` pelo nome: `chore(visual): captura de não lidos com o divisor no máximo [P6]`.

---

### Task 7: capturar.mjs lê a lista de alto contraste do review.json (RB3)

**Files:**
- Modify: `scripts/visual/capturar.mjs` (`:9`, `:21`, `:194`)
- Modify: `scripts/visual/estados.test.mjs`

**Interfaces:** `capturar.mjs` lê `visual.alto_contraste` de `.claude/review.json` (`JSON.parse(readFileSync(path.join(RAIZ, '.claude', 'review.json'), 'utf8'))`) em vez da constante `ALTO_CONTRASTE`; lista ausente vira `[]`.

- [ ] **Passo 1: teste.** Em `estados.test.mjs`, dois testes: (a) toda entrada de `visual.alto_contraste` do `review.json` é uma chave de `ESTADOS`; (b) o fonte de `capturar.mjs` (lido como texto) não tem a lista literal (`assert.doesNotMatch(src, /ALTO_CONTRASTE\s*=\s*\[/)`) e cita `alto_contraste`.
- [ ] **Passo 2: rodar** `node --test scripts/visual/estados.test.mjs`: (b) falha.
- [ ] **Passo 3: implementar** a leitura; ajuste o comentário de `:9`.
- [ ] **Passo 4: rodar** o teste, `node --test frontend/*.test.mjs` e `node --test .claude/hooks/hooks.test.mjs .claude/statusline.test.mjs .claude/workflows/workflows.test.mjs scripts/visual/estados.test.mjs`. Sem `svelte-check`/`npm run build` (mesma exceção da Task 6). Não rode a captura completa: basta `node scripts/visual/capturar.mjs --saida <scratchpad>/rb3 --rapido --estados processando` para ver que a de alto contraste ainda sai (`1024x768-alto-contraste-processando.png`); encerre o que subir.
- [ ] **Passo 5: commit** `chore(visual): lista de alto contraste vem do review.json [RB3]`.

---

### Task 8: esperar() zera os pendentes; aguardar vira esperarDespachados (P13)

**Files:**
- Modify: `internal/etl/pdf_ocr_despacho.go` (`:101-116`)
- Modify: `internal/etl/pdf_parser.go` (`:148`)
- Modify: `internal/etl/pdf_poppler.go` (`:54`)
- Modify: `internal/etl/pdf_parser_lote_test.go` (teste novo)

**Interfaces:**
- `func (d *despachoOCR) esperarDespachados()` (era `aguardar`): espera os blocos já despachados, sem despachar a sobra. Usada só no cancelamento.
- `func (d *despachoOCR) esperar() map[string]any`: sem mudança de assinatura; depois de despachar a sobra, faz `d.pendentes = nil`, então uma segunda chamada não despacha de novo.

- [ ] **Passo 1: teste.** `TestDespachoOCR_EsperarDuasVezesNaoRedespacha` em `pdf_parser_lote_test.go`: `p := parserDeArquivo(t, pdfComPaginas(t, 3, nil)).comOCREmLote(registroLote(&mu, &tamanhos), 2)`; OCR por página que conta chamadas num `atomic.Int32` e devolve `"ocr"`; `d := p.novoDespachoOCR(context.Background(), 3, porPagina, nil)`; `d.agendar(ocrTask{pageNum: 1})` (fica pendente: `tamBloco` é 2); `d.esperar()` duas vezes. Asserções: OCR por página chamado 1 vez (bloco de 1 página usa o OCR por página, `runOCRBloco` `:185`); lote nunca chamado; página `"1"` no mapa com `content == "ocr"`.
- [ ] **Passo 2: rodar** `go test ./internal/etl -run TestDespachoOCR_ -v`: falha com 2 chamadas (confirmado por sonda).
- [ ] **Passo 3: implementar** `d.pendentes = nil` dentro do `if` de `esperar()`, depois do laço; renomeie `aguardar` para `esperarDespachados` e os dois chamadores; ajuste os comentários das duas funções.
- [ ] **Passo 4: rodar** o teste novo e o gate (`go vet ./...`, `go test -race ./internal/...`). Confira com Grep que `aguardar()` não aparece mais em `internal/etl`.
- [ ] **Passo 5: commit** `fix(etl): esperar() não redespacha a sobra; aguardar vira esperarDespachados [P13]`.

---

### Task 9: Regex do pdfinfo sem quebra de linha e teto na saída do Poppler (P14, comportamento)

**Files:**
- Modify: `internal/ocr/poppler.go` (`:21-36`, `:24`, `:50-75`)
- Modify: `internal/ocr/main_test.go` (modo novo do binário de teste)
- Modify: `internal/ocr/poppler_test.go` (testes)

**Interfaces:**
- `reNumPaginas = regexp.MustCompile(`(?m)^Pages:[ \t]+(\d+)[ \t]*$`)`.
- `var maxSaidaPoppler = 64 << 20` (variável para o teste encurtar; comentário: teto do stdout de um processo do Poppler, muito acima de um documento real e abaixo de um uso de memória que derrube o app).
- `var errSaidaGrande = errors.New("saída do Poppler acima do limite")`.
- `limitado` ganha o teto como campo e uma marca de corte: `type limitado struct{ buf bytes.Buffer; max int; cortou bool }`. `Write` guarda até `max` bytes, marca `cortou` quando descarta algo e sempre devolve `len(p), nil` (descartar em vez de devolver erro: com erro no `Write` o `exec` para de ler e o filho trava no pipe até o prazo). `String()` continua com `TrimSpace` e serve só ao stderr. O stderr usa `max: maxStderr`; o stdout de `rodarPoppler` usa `max: maxSaidaPoppler`.
- Em `rodarPoppler`, sem erro do processo e com `saida.cortou`, devolve `fmt.Errorf("falha no %s: %w (%d bytes)", exe, errSaidaGrande, maxSaidaPoppler)`. Sem corte, devolve `saida.buf.String()`, **sem** `TrimSpace` (o `TrimSpace` tiraria o começo da página 1 e os `\f` finais, e `TextoPDF` contaria páginas a menos). Erro do processo continua tendo prioridade (senha, prazo, stderr).
- No `TestMain`: com `EPROCIA_TESTE_JORRAR=<n>`, o binário escreve `n` bytes `x` no stdout e sai com 0. Fica antes da defesa `EPROCIA_TESTE_FILHO` da Task 1.

- [ ] **Passo 1: regex, vermelho e verde.** Escreva só `TestReNumPaginas` (tabela com `FindStringSubmatch` direto: `"Pages:          3\n"` dá `"3"`; `"Title: x\nPages: \t 7  \n"` dá `"7"`; `"Pages:\n12\n"` não casa; `"Pages:\n\n5"` não casa). Rode `go test ./internal/ocr -run TestReNumPaginas -v`: `"Pages:\n12\n"` casa (falha). Troque a regex e rode de novo: passa.
- [ ] **Passo 2: teto, vermelho.** Acrescente o modo `EPROCIA_TESTE_JORRAR` ao `TestMain` e os testes `TestRodarPoppler_SaidaNoTetoPassa` e `TestRodarPoppler_SaidaAcimaDoTetoFalha`: pasta temporária com `os.Args[0]` copiado como `pdftotext.exe`; `EPROCIA_TESTE_FILHO=1`; `maxSaidaPoppler` trocado por 1024 com `t.Cleanup`; `EPROCIA_TESTE_JORRAR` = `1024` e `1025`. Asserções: com 1024, sem erro e `len(saida) == 1024`; com 1025, `errors.Is(err, errSaidaGrande)`, `!errors.Is(err, ErrSenha)`, `!errors.Is(err, ErrPrazo)`, e volta em menos de 5 s. Rode: não compila (sem `maxSaidaPoppler`/`errSaidaGrande`).
- [ ] **Passo 3: implementar** o campo de teto em `limitado`, as duas variáveis e o erro em `rodarPoppler`.
- [ ] **Passo 4: rodar** os testes novos, os de `poppler_test.go` com o binário real (`TestInfoPDF_ContaPaginas`, `TestTextoPDF_*`, inclusive `TestTextoPDF_UltimaPaginaSemTexto`) e o gate (`go vet ./...`, `go test -race ./internal/...`, `go test -race ./internal/ocr` sem `-short`).
- [ ] **Passo 5: relatório** com o efeito no pipeline de um PDF acima do teto: `TextoPDF` falha, e `parsePoppler` (`internal/etl/pdf_poppler.go:38-47`) devolve erro com o OCR desligado ou manda todas as páginas ao OCR com ele ligado. Não mude esse caminho.
- [ ] **Passo 6: commit** `fix(ocr): regex do pdfinfo sem quebra de linha e teto na saída do Poppler [P14]`.

---

### Task 10: Um contador só para o resumo da atividade (RB2)

**Files:**
- Modify: `frontend/src/lib/atividade.js` (`:32-40`, `:49-63`, `:72-92`)
- Modify: `frontend/atividade.test.mjs`

**Interfaces:**
- `function contarDoc(resumo, d)` (interna, não exportada): aplica a um resumo as contagens de um documento (`naoLidos`, `semOCR`, `ativos`, `emOcr`, `aguardando`, `concluidos`, `erros` com a exclusão dos não lidos, `paginasOcr`). `resumoAtividade` e `prepararDocs` passam a usá-la; `prepararDocs` segue limitando os aguardando visíveis pelo valor de `resumo.aguardando` antes do incremento, como hoje (`:89`).
- `ordenarDocs` fica exportada, com comentário dizendo que é o oráculo de ordem dos testes (`atividade.test.mjs`, `desempenho.test.mjs`) e não tem uso em produção.

- [ ] **Passo 1: teste.** Em `atividade.test.mjs`, `prepararDocs(inst).resumo` é igual (`deepEqual`) a `resumoAtividade(inst)` para uma lista com todos os estados, não lidos de cada motivo (`sem_ocr`, `senha`, `ilegivel`, `prazo`, `ocr_falhou`, `sem_texto`) com `estado: 'erro'`, um erro comum, páginas em OCR e `LIMITE_AGUARDANDO + 5` aguardando; também com `{ expandirAguardando: true, mostrarConcluidos: true }`.
- [ ] **Passo 2: rodar** `node --test frontend/atividade.test.mjs`: passa hoje (é a guarda da refatoração; registre). Numa cópia, mude a contagem de `erros` só em `prepararDocs` e confirme que falha; desfaça.
- [ ] **Passo 3: implementar** `contarDoc` e o comentário de `ordenarDocs`.
- [ ] **Passo 4: gate** `node --test frontend/*.test.mjs`, `npx svelte-check` e `npm run build` em `frontend`.
- [ ] **Passo 5: commit** `refactor(frontend): resumo da atividade com um contador só [RB2]`.

---

### Task 11: Anúncios do mesmo tick não se perdem (RB1)

**Files:**
- Create: `frontend/src/lib/regiaoViva.js`
- Create: `frontend/regiaoViva.test.mjs`
- Modify: `frontend/src/components/ProcessList.svelte` (`:94-102`)
- Modify: `frontend/src/App.svelte` (`:276-284`)
- Modify: `frontend/src/components/Settings.svelte` (`:128-134`)

**Interfaces:**
- `export function criarAnunciador(escrever, proximoTick)` devolve `anunciar(texto): Promise<void>`. Texto vazio é ignorado. A primeira chamada de um ciclo guarda o texto, chama `escrever('')`, espera `proximoTick()`, escreve os textos guardados juntados com `'. '` e esvazia a fila. Chamadas que chegam antes da escrita só entram na fila e recebem a mesma promessa. O texto de cada mensagem não muda; só deixam de se perder.
- Nos três componentes: `let anuncio = '';` e `const anunciar = criarAnunciador((t) => (anuncio = t), tick);` no lugar da função `async function anunciar`. As chamadas existentes (`ProcessList.svelte:108,114,124,305`, `App.svelte:196,292`, `Settings.svelte:103`) não mudam. O comentário de cada região viva continua dizendo por que esvazia antes de escrever.

- [ ] **Passo 1: teste.** `frontend/regiaoViva.test.mjs` (node --test, com um `proximoTick` falso que resolve quando o teste mandar):
  - duas chamadas no mesmo tick (`'X: concluído, 2 documentos'` e `'2 itens correspondem ao filtro'`) escrevem `''` e depois `'X: concluído, 2 documentos. 2 itens correspondem ao filtro'`, uma escrita só de texto;
  - uma chamada sozinha escreve `''` e depois o texto igual ao recebido;
  - texto vazio não escreve nada;
  - depois de um ciclo, o mesmo texto de novo volta a escrever `''` e o texto (o leitor repete);
  - as duas promessas do mesmo tick resolvem depois da escrita.
  - Teste de convenção (lendo os três `.svelte` como texto): nenhum define `async function anunciar`; os três importam `criarAnunciador`.
- [ ] **Passo 2: rodar** `node --test frontend/regiaoViva.test.mjs`: falha (módulo inexistente).
- [ ] **Passo 3: implementar** o módulo e trocar as três funções. Confira que `anuncios.test.mjs` e os outros testes por regex continuam passando sem mudar regra; se algum exigir a forma antiga de `anunciar`, ajuste a regex para a forma nova e explique no relatório.
- [ ] **Passo 4: gate** `node --test frontend/*.test.mjs`, `npx svelte-check` e `npm run build` em `frontend`, mais `go vet ./...` (o `main.go` embute `frontend/dist`).
- [ ] **Passo 5: commit** `fix(frontend): anúncios do mesmo tick saem juntos na região viva [RB1]`.
