# Higienização e extração de texto: plano de implementação

> **Para agentes:** sub-skill obrigatória: use superpowers:subagent-driven-development (recomendado) ou superpowers:executing-plans para implementar este plano tarefa por tarefa. Os passos usam checkbox (`- [ ]`) para acompanhamento.

**Objetivo:** Corrigir os quatro defeitos de extração (E1–E4) e trocar a higienização por quatro níveis (`none`, `basic`, `standard`, `aggressive`), mais a opção de omitir tipos de documento.

**Arquitetura:**
- **Extratores.** As correções ficam em `html_parser.go` e `pdf_parser.go`. O texto posicional do PDF ganha um arquivo próprio, `pdf_texto.go`.
- **Regras de higienização.** Cada regra é uma função pura em `internal/etl/higienizacao.go`.
- **Formatter.** `formatter.go` compõe as regras por nível e faz a pré-passada do processo inteiro (H7).
- **H10.** A omissão de tipos entra na pré-população do mapa `eventos` em `pipeline.go`.

**Tecnologias:** Go 1.27.3, `github.com/dslipak/pdf` v0.0.2, `golang.org/x/net/html` e `/html/charset`, Wails v2.13, Svelte 5 (sintaxe legada).

**Spec:** `docs/superpowers/specs/2026-11-08-higienizacao-design.md`. Leia junto com este plano.

## Restrições globais

- A branch é `higienizacao`. Um commit por tarefa, com mensagem em pt-BR no formato `tipo(escopo): resumo` e as linhas de atribuição da sessão.
- Nunca dê push.
- Todo texto de teste é sintético, sem nome real, número de processo, CPF ou endereço. Placeholders: `FULANO DE TAL`, `5000000-00.2026.8.26.0001`.
- Não leia nem cite os `.txt` da pasta Processos, exceto na Tarefa 14, que só imprime contagens.
- A pipeline não muda de ordem nem de concorrência. O mapa `eventos` só recebe chaves novas na pré-população, que roda numa goroutine só.
- `none`, `basic` e `aggressive` continuam aceitos em `config.LimpezaValida`. `aggressive` passa a significar Máxima.
- Tudo que apaga uma linha inteira só age nas bordas da página: as 5 primeiras e as 5 últimas linhas não vazias. A exceção são as regras que não apagam palavra (H1, H2, H8) e H7, que deixa uma referência.
- Logs e textos da interface em pt-BR.
- Depois de mudar um método de `App`, rode `wails generate module`. Nunca edite `frontend/src/wailsjs/` à mão.
- Não faça commit de `README.md`, `build/windows/installer/project.nsi`, `install.ps1` nem `.genkit/`: são mudanças locais do dono.
- Antes de cada commit de Go: `go vet ./...` e `go test -race ./internal/...`. Antes de cada commit de frontend: `cd frontend && npx svelte-check` e `node --test (Get-ChildItem frontend -Filter *.test.mjs).FullName`.

## Foco da revisão

Estas são as entradas que a spec implica e que um usuário vai encontrar. Cada uma ganhou um teste na tarefa dona:

1. **Palavras curtas reais num PDF com kerning** ("casa e campo" com o `e` isolado entre espaços largos) continuam separadas, e valores ("R$ 1.234,56") não ganham espaço. Fica na Tarefa 3.
2. **Extrato bancário:** continua com um lançamento por linha na Máxima (reflow). Fica na Tarefa 11.
3. **Página de tabela numérica legítima** (muitos números, pontuação, `R$`) não conta como ilegível. Fica na Tarefa 4.
4. **Duas páginas que diferem em um dígito** (valor, data) não viram referência de duplicata. Fica na Tarefa 9.
5. **Frase sobre assinatura ou código verificador no meio de uma peça** continua intacta na Padrão e na Máxima. Fica na Tarefa 8.

---

### Tarefa 1: E2, charset do HTML

**Arquivos:**
- Modificar: `internal/etl/html_parser.go`
- Teste: `internal/etl/html_parser_test.go` (novo)

**Interfaces:**
- Produz: `ExtractTextFromHTML(r io.Reader) (string, error)` com a mesma assinatura de hoje, agora decodificando o charset.

- [ ] **Passo 1: Testes que falham**

```go
func TestExtractTextFromHTML_Latin1ComMeta(t *testing.T) {
	src := []byte("<html><head><meta charset=\"iso-8859-1\"></head><body><p>Poder Judici\xe1rio</p></body></html>")
	got, err := ExtractTextFromHTML(bytes.NewReader(src))
	if err != nil || !strings.Contains(got, "Poder Judiciário") || strings.ContainsRune(got, '�') {
		t.Fatalf("got %q, err %v", got, err)
	}
}
func TestExtractTextFromHTML_SemMetaBytesInvalidosViram1252(t *testing.T) {
	src := []byte("<html><body><p>R\xc9U</p></body></html>")
	got, _ := ExtractTextFromHTML(bytes.NewReader(src))
	if !strings.Contains(got, "RÉU") { t.Fatalf("got %q", got) }
}
func TestExtractTextFromHTML_UTF8Intacto(t *testing.T) {
	got, _ := ExtractTextFromHTML(strings.NewReader("<p>Ação — §1º</p>"))
	if !strings.Contains(got, "Ação — §1º") { t.Fatalf("got %q", got) }
}
```

- [ ] **Passo 2:** `go test ./internal/etl -run TestExtractTextFromHTML_ -v` falha nos dois primeiros.
- [ ] **Passo 3:** Ler todo o conteúdo com `io.ReadAll`. Usar `charset.DetermineEncoding(dados, "")`.
  - Se o resultado não for `certain` e `utf8.Valid(dados)` for falso, usar `charmap.Windows1252`.
  - Decodificar e passar ao tokenizer atual.
- [ ] **Passo 4:** Os testes passam, assim como os `TestExtractTextFromHTML*` antigos de `formatter_test.go`.
- [ ] **Passo 5:** Commit `fix(etl): HTML decodificado pelo charset (acentos do eproc)`.

### Tarefa 2: E3, quebra de linha só em elemento de bloco

**Arquivos:**
- Modificar: `internal/etl/html_parser.go`
- Teste: `internal/etl/html_parser_test.go`

- [ ] **Passo 1: Testes que falham**

```go
func TestExtractTextFromHTML_InlineNaMesmaLinha(t *testing.T) {
	got, _ := ExtractTextFromHTML(strings.NewReader(
		"<div><b>AUTOR</b><span>:</span> <span>FULANO DE TAL</span></div><p>Segunda linha</p>"))
	if strings.TrimSpace(got) != "AUTOR: FULANO DE TAL\nSegunda linha" { t.Fatalf("got %q", got) }
}
func TestExtractTextFromHTML_BrELinhaDeTabela(t *testing.T) {
	got, _ := ExtractTextFromHTML(strings.NewReader(
		"<p>um<br>dois</p><table><tr><td>a</td><td>b</td></tr><tr><td>c</td><td>d</td></tr></table>"))
	if strings.TrimSpace(got) != "um\ndois\na b\nc d" { t.Fatalf("got %q", got) }
}
```

- [ ] **Passo 2:** Os testes falham.
- [ ] **Passo 3:** Abrir e fechar `p`, `div`, `br`, `tr`, `li`, `table`, `h1`–`h6`, `section`, `article`, `blockquote`, `pre` e `hr` quebra a linha. Abrir `td` e `th` insere um espaço. O texto inline é concatenado como está. Sequências de quebras viram uma só. Cada linha recebe trim.
  - Se um teste antigo de `formatter_test.go` esperar uma linha por nó, atualize a expectativa para o novo comportamento e cite isso no commit.
- [ ] **Passo 4:** Os testes passam.
- [ ] **Passo 5:** Commit `fix(etl): HTML quebra linha só em elemento de bloco`.

### Tarefa 3: E1, extrator posicional de texto do PDF

**Arquivos:**
- Criar: `internal/etl/pdf_texto.go`
- Modificar: `internal/etl/pdf_parser.go:496-535` (`ExtractNativeText`)
- Teste: `internal/etl/pdf_texto_test.go` (novo). Monte os PDFs com o helper `buildPDF` dos testes do pacote, como em `pdf_parser_lote_test.go`, com fonte `/F1` Helvetica.

**Interfaces:**
- Produz: `extrairTextoPosicional(page pdf.Page) (string, error)`, uma linha por Y e as palavras separadas por espaço.
- `ExtractNativeText` chama primeiro `extrairTextoPosicional`. Se ela der pânico ou devolver texto vazio quando o `GetTextByRow` acha texto, volta ao `GetTextByRow` atual.

- [ ] **Passo 1: Testes que falham.** Cada teste monta uma página com o stream indicado e chama `ExtractNativeText` sobre ela.

| Teste | Stream (dentro de `BT /F1 12 Tf 72 700 Td ... ET`) | Esperado (`strings.TrimSpace`) |
|---|---|---|
| `TestTextoPosicional_TJLetraALetra` | `[(C) -20 (O) -20 (N) -20 (T) -20 (E) -20 (S) -20 (T) -20 (A)] TJ` | `CONTESTA` |
| `TestTextoPosicional_TJEspacoDePalavra` | `[(casa) -280 (e) -280 (campo)] TJ` | `casa e campo` |
| `TestTextoPosicional_TjPorLetraComTd` | `(R) Tj 7 0 Td ($) Tj 7 0 Td (1) Tj 7 0 Td (.) Tj 4 0 Td (2) Tj 7 0 Td (3) Tj 7 0 Td (4) Tj 20 0 Td (OK) Tj` | `R$1.234 OK` |
| `TestTextoPosicional_DuasLinhas` | `(Primeira linha) Tj 0 -14 Td (Segunda linha) Tj` | `Primeira linha\nSegunda linha` |
| `TestTextoPosicional_TextoNormalIgualAoAtual` | o stream de `pdfgen.NativeTextPage` com 3 linhas | as 3 linhas, uma por linha, iguais às de hoje |

- [ ] **Passo 2:** `go test ./internal/etl -run TestTextoPosicional -v`: os três primeiros falham.
- [ ] **Passo 3: Implementar `extrairTextoPosicional`.**
  - Usar `pdf.Interpret` sobre `page.V.Key("Contents")`. Se for array, concatenar os streams na ordem, como `checkContentStream` já conhece.
  - Acompanhar a matriz de texto e de linha: `BT`, `Tf` (tamanho), `TL`, `Td`, `TD`, `Tm`, `T*`, `'`, `"`, `Tj`, `TJ`.
  - Decodificar as strings com o encoder da fonte, como a biblioteca faz: `page.Font(nome).Encoder()`.
  - Largura estimada de um trecho: `runas × 0,5 × tamanho`.
  - Inserir espaço antes de um trecho quando:
    - (a) dentro de um `TJ`, o deslocamento numérico acumulado desde o trecho anterior for ≤ −200; ou
    - (b) entre operadores, na mesma linha (|ΔY| < 0,5 × tamanho), o X novo passar de `fimEstimado + 0,25 × tamanho`.
  - Agrupar por Y, do maior para o menor. Dentro da linha, ordenar por X.
  - Calibre os dois limiares só se um teste falhar, e registre o valor final num comentário.
- [ ] **Passo 4:** Os testes novos passam, assim como `go test ./internal/etl ./scripts/bench -short`.
- [ ] **Passo 5:** Commit `fix(etl): texto do PDF separa palavras pela posição dos glifos`.

### Tarefa 4: E4, texto nativo ilegível vai para o OCR

**Arquivos:**
- Modificar: `internal/etl/pdf_parser.go` (`analisePagina`, `analisarPagina` e o fallback em `registrarOCR`)
- Teste: `internal/etl/pdf_parser_test.go`

**Interfaces:**
- Produz: `textoIlegivel(s string) bool` e `marcadorIlegivel(n int) string`, que devolve `[texto ilegível na página: N caracteres omitidos]` com N em runas.

- [ ] **Passo 1: Testes que falham**

```go
func TestTextoIlegivel(t *testing.T) {
	casos := map[string]bool{
		"Texto jurídico normal com palavras e números 123.": false,
		"R$ 1.234,56 | R$ 2.345,67 | 31/12/2026 | 10,00%":   false, // tabela numérica
		"\x01\x05\x08\x01\x05\x08\x01\x05\x08\x01\x05\x08":   true,
		"#$%&*@#$%&*@ ¨¨ ~~ ^^ ## a":                        true,
		"": false,
	}
	for s, want := range casos { if textoIlegivel(s) != want { t.Errorf("%q: want %v", s, want) } }
}
```

  Também: `TestParsePages_IlegivelSemOCRViraMarcador`, uma página cujo `Tj` tem 150 bytes `\001\005\010`, com OCR desligado, tem conteúdo igual a `marcadorIlegivel(150)`. E `TestParsePages_IlegivelVaiAoOCR`, a mesma página, sem imagem, com o lote fake de `pdf_parser_lote_test.go`, recebe o texto do lote. Se o lote falhar, recebe o marcador.
- [ ] **Passo 2:** Os testes falham.
- [ ] **Passo 3:** `textoIlegivel` é verdadeiro quando há 20 ou mais runas não-espaço e:
  - a razão de letras e dígitos (`unicode.IsLetter || unicode.IsDigit`) sobre as runas não-espaço é menor que 0,4; ou
  - há mais de 10 runas de controle (`unicode.IsControl`, exceto `\n`, `\t`).

  Em `analisarPagina`:
  - se for ilegível e o OCR estiver ligado, `precisaOcr = true`, e o fallback da tarefa de OCR é o marcador;
  - se o OCR estiver desligado, `texto` vira o marcador.
- [ ] **Passo 4:** Os testes passam.
- [ ] **Passo 5:** Commit `fix(etl): página com texto nativo ilegível vai para o OCR`.

### Tarefa 5: Nível `standard` no config

**Arquivos:**
- Modificar: `internal/config/config.go` (`DefaultConfig`, `LimpezaValida`)
- Teste: `internal/config/config_test.go`

- [ ] **Passo 1:** Testes que falham:
  - `TestLimpezaValida_QuatroNiveis`: `none`, `basic`, `standard` e `aggressive` são verdadeiros; `maxima` e `""` são falsos.
  - `TestDefaultConfig`: ajustar para `CleaningLevel == "standard"`.
  - `TestLoad_ConfigAntigoMantemNivel`: um arquivo com `"cleaning_level":"aggressive"` carrega `aggressive`, e um com `"basic"` carrega `basic`.
- [ ] **Passo 2:** Os testes falham.
- [ ] **Passo 3:** Implementar.
- [ ] **Passo 4:** `go test ./internal/config ./internal/etl` passa. Se um teste da fila assume o padrão `basic`, ajuste-o.
- [ ] **Passo 5:** Commit `feat(config): nível de higienização standard, padrão nas instalações novas`.

### Tarefa 6: Regras puras e a Básica (H1, H2)

**Arquivos:**
- Criar: `internal/etl/higienizacao.go`
- Modificar: `internal/etl/formatter.go` (`cleanPage`)
- Teste: `internal/etl/higienizacao_test.go` (novo)

**Interfaces:**
- Produz (em `higienizacao.go`):
  - `const (NivelNenhum = "none"; NivelBasico = "basic"; NivelPadrao = "standard"; NivelMaximo = "aggressive")`;
  - `linhasBasicas(text string) []string`: tira caracteres de controle exceto `\n` e `\t`, troca toda sequência de espaço, tab ou NBSP por um espaço, faz trim e descarta linhas vazias e separadoras;
  - `linhaSeparadora(l string) bool`: verdadeiro para 5 ou mais repetições de `-`, `_`, `=`, `.`, `*` ou `•`, com espaços opcionais entre elas, e nada mais.
  - `type ctxDocumento struct { repetidas map[string]bool; jaVistas map[string]bool; html bool }`: o estado por documento, que as Tarefas 7 e 8 passam a usar. Nesta tarefa, `GenerateConsolidatedOutput` já monta o contexto por documento: `repetidas` vem de `repeatedEdgeLines` e `jaVistas` começa vazio.
- `cleanPage` muda para `cleanPage(text, level string, ctx *ctxDocumento) string`. Para `none`, devolve o texto intacto.

- [ ] **Passo 1:** Testes que falham:
  - `TestLinhasBasicas`: `"  a   b\t\tc  \n\n\x01d  e\n_ _ _ _ _ _\n-----\nItem ........ 12"` vira `["a b c", "d e", "Item ........ 12"]`.
  - `TestCleanPage_BasicaNaoJuntaLinhas`: duas linhas continuam duas.
- [ ] **Passo 2:** Os testes falham.
- [ ] **Passo 3:** Implementar. A Básica é `strings.Join(linhasBasicas(text), "\n")`.
- [ ] **Passo 4:** Os testes novos e os `TestCleanPage*` antigos passam. Atualize as expectativas da Básica que mudarem por H1, por exemplo espaço duplo.
- [ ] **Passo 5:** Commit `feat(etl): Básica colapsa espaços e tira controle e linhas separadoras`.

### Tarefa 7: Padrão, bordas e timbre só na primeira página

**Arquivos:**
- Modificar: `internal/etl/formatter.go` (`edgeLineCount = 5`, `repeatedEdgeLines` a partir de 2 páginas, `cleanPage`)
- Modificar: `internal/etl/higienizacao.go` (`linhaSoSimbolos`)
- Teste: `internal/etl/formatter_test.go`

**Interfaces:**
- Consome: `ctxDocumento` (Tarefa 6).
- Produz:
  - `linhaSoSimbolos(l string) bool`: verdadeiro quando a linha não tem letra nem dígito e não contém `§`, `º`, `ª` ou `%`.
- Regra do timbre: numa página, uma linha de borda que está em `repetidas` é removida se a chave normalizada já está em `jaVistas`. Ao fim da página, todas as chaves de `repetidas` vistas nela entram em `jaVistas`.
- Na Padrão e na Máxima, apagar: os marcadores de página (`pageMarkerRegex`) em qualquer posição; as linhas de `linhaSoSimbolos` em qualquer posição.

- [ ] **Passo 1:** Testes que falham:
  - `TestPadrao_TimbreFicaSoNaPrimeiraPagina`: um documento de 3 páginas, cada uma começando com `ESCRITÓRIO MODELO ADVOGADOS` / `OAB/SP 000.000`. Na Padrão, a pág. 1 mantém as duas linhas e as págs. 2 e 3 não têm nenhuma.
  - `TestPadrao_BordaDe5Linhas`: um rodapé de 5 linhas igual em 3 páginas fica só na pág. 1.
  - `TestPadrao_DocumentoDe2Paginas`: o cabeçalho repetido em 2 páginas sai da pág. 2.
  - `TestPadrao_FraseRepetidaNoMeioFica`: uma frase igual no meio das 3 páginas fica nas 3.
  - `TestPadrao_LinhaSoSimbolos`: `| : "` sai; `§ 1º` e `10%` ficam.
- [ ] **Passo 2:** Os testes falham.
- [ ] **Passo 3:** Implementar. Atualize `TestRepeatedEdgeLines` e `TestGenerateConsolidatedOutput_AggressiveDropsRepeatedHeadersFooters`: agora a primeira ocorrência fica.
- [ ] **Passo 4:** Os testes passam.
- [ ] **Passo 5:** Commit `feat(etl): Padrão remove bordas repetidas e mantém o timbre na primeira página`.

### Tarefa 8: H6, chancelas viram `[assinado: ...]`

**Arquivos:**
- Modificar: `internal/etl/higienizacao.go` (`resumirChancelas`)
- Modificar: `internal/etl/formatter.go` (remover `signaturePrefixes` e `isBoilerplateLine`; `ctxDocumento.html` vem de `filepath.Ext(nome)` em `.html` e `.htm`)
- Teste: `internal/etl/higienizacao_test.go`

**Interfaces:**
- Produz: `resumirChancelas(linhas []string, html bool) []string`. Age só nas 8 últimas linhas da página e, para a família e-SAJ, também nas 5 primeiras. Troca o bloco que casa por um marcador:
  - **HTML:** `[assinado: NOME]`;
  - **PDF:** `[assinado: NOME, CARGO, DATA]`, omitindo as partes ausentes e as vírgulas delas.
- Famílias, com o texto de sistema da spec e regex com grupos para NOME, CARGO e DATA:
  1. eproc HTML: "Documento eletrônico assinado por NOME, CARGO, na forma do artigo 1º, inciso III, da Lei 11.419 ..." até "código CRC X." e o rodapé seguinte `<CNJ> <N> .V<N>`;
  2. e-SAJ: "Este documento é cópia do original, assinado digitalmente por NOME, protocolado em DATA ..." e "Para conferir o original, acesse o site ...";
  3. cartório: "Assinado digitalmente por NOME - CARGO" e "Protocolo nº ... Documento registrado eletronicamente ...". Este último vira o marcador quando está sozinho, ou some quando o outro já deixou o marcador;
  4. "Documento eletrônico ID assinado eletronicamente nos termos do Art.1º §2º inciso III da Lei 11.419/2006 Signatário(a): NOME";
  5. "Este laudo foi assinado eletronicamente" e "Você pode conferir a autenticidade do documento em URL com o código X": somem, sem marcador próprio.
- O bloco pode estar partido em várias linhas. Junte as linhas da janela com espaço antes de aplicar a regex e troque as linhas cobertas pelo marcador.

- [ ] **Passo 1:** Testes que falham:
  - um teste por família, com texto sintético e `FULANO DE TAL`, `Juiz de Direito`, `01/01/2026`;
  - `TestChancela_HTMLSoNome`: o bloco eproc de 5 linhas mais o rodapé CNJ viram exatamente `[assinado: FULANO DE TAL]`;
  - `TestChancela_FraseNoMeioFica`: "o código verificador não confere com o documento assinado por FULANO DE TAL" no meio de uma página de 20 linhas fica intacta;
  - `TestBasica_DocumentoAssinadoFica`: na Básica, "Documento assinado eletronicamente por FULANO" fica (a regra antiga saiu).
- [ ] **Passo 2:** Os testes falham.
- [ ] **Passo 3:** Implementar na Padrão e na Máxima.
- [ ] **Passo 4:** Os testes passam.
- [ ] **Passo 5:** Commit `feat(etl): chancelas de assinatura viram marcador com o assinante`.

### Tarefa 9: H7, duplicadas viram referência

**Arquivos:**
- Modificar: `internal/etl/formatter.go` (`GenerateConsolidatedOutput`: pré-passada)
- Teste: `internal/etl/formatter_test.go`

**Interfaces:**
- Pré-passada na ordem de saída (evento, documento, página): a chave é o `sha256` de `strings.Join(linhasBasicas(conteudo), " ")`, só para páginas com 200 ou mais runas. Guarda a primeira origem `{evento int; doc string; pag int}`, com `doc` igual a `summaryDocName(nome)`.
- Na Padrão e na Máxima:
  - uma página duplicada sai como `[conteúdo idêntico a: evento E, DOC, pág. P]`;
  - se todas as páginas de um documento são duplicadas, e de um mesmo documento de origem, o corpo inteiro vira `[documento idêntico a: evento E, DOC]`.

- [ ] **Passo 1:** Testes que falham:
  - `TestDuplicadas_PaginaViraReferencia`;
  - `TestDuplicadas_DocumentoInteiro`;
  - `TestDuplicadas_UmDigitoDiferenteNaoColapsa`: "Valor R$ 1.234,56" contra "Valor R$ 1.234,57" em páginas de 250 runas;
  - `TestDuplicadas_PaginaCurtaNaoColapsa`: 150 runas;
  - `TestDuplicadas_BasicaNaoColapsa`.
- [ ] **Passo 2:** Os testes falham.
- [ ] **Passo 3:** Implementar.
- [ ] **Passo 4:** Os testes passam, incluindo `TestGenerateConsolidatedOutput_TokensBatemEmTodoTamanho`.
- [ ] **Passo 5:** Commit `feat(etl): páginas e documentos duplicados viram referência`.

### Tarefa 10: H8, formato enxuto em todos os níveis

**Arquivos:**
- Modificar: `internal/etl/formatter.go`
- Modificar: `PRD.md` §6 (exemplo do formato)
- Teste: `internal/etl/formatter_test.go`, `internal/etl/pipeline_saida_test.go`

- Formato novo:
  - nenhuma linha emitida começa com espaço;
  - `<pag n="N">` no lugar de `<pag num="N">`;
  - um documento com uma página só (depois de descartar as vazias) escreve o conteúdo direto dentro de `<documento ...>`, sem `<pag>`;
  - uma página com conteúdo vazio depois da limpeza não emite `<pag>`;
  - `<contexto>` e o resumo também perdem a indentação. O texto do resumo continua igual.

- [ ] **Passo 1:** Atualizar `TestGenerateConsolidatedOutput` para o formato novo. Novo teste `TestFormato_DocUmaPaginaSemPag`: nenhuma linha de saída começa com `" "`, e em todos os níveis aparece `<pag n="2">`.
- [ ] **Passo 2:** Os testes falham.
- [ ] **Passo 3:** Implementar e atualizar o exemplo do PRD §6.
- [ ] **Passo 4:** `go test ./internal/etl ./scripts/bench -short` passa.
- [ ] **Passo 5:** Commit `feat(etl): formato de saída enxuto (sem recuo, pag n, doc de uma página sem pag)`.

### Tarefa 11: Máxima, ruído de OCR, certificado e reflow

**Arquivos:**
- Modificar: `internal/etl/higienizacao.go` (`linhaLetraSolta`, `paginaCertificado`, `refluir`)
- Modificar: `internal/etl/formatter.go` (a Máxima deixa de juntar a página numa linha)
- Teste: `internal/etl/higienizacao_test.go`

**Interfaces:**
- `linhaLetraSolta(l string) bool`: verdadeiro para linhas de 1 a 2 letras, sem dígito, que não são numeral romano (`^[IVXLC]+$`), alínea (`^[a-z]\)$`) nem sigla de UF (as 27).
- `paginaCertificado(linhas []string) bool`: verdadeiro quando a página contém "Termos de Assinatura e Registro Eletrônico", ou pelo menos 3 linhas que começam com `Enviado:`, `Visualizado:`, `Assinado:`, `Aceito:` ou `Nível de segurança`. Na Máxima, a página vira `[certificado de assinatura eletrônica]`.
- `refluir(linhas []string) []string`: junta a linha `i+1` à `i` quando:
  - `i` não termina em `.`, `:`, `;`, `?` ou `!`;
  - `i` tem 40 ou mais runas;
  - nenhuma das duas parece linha de tabela (2 ou mais números com dígito, ou `R$`);
  - `i+1` não começa com marcador de lista (`^([a-z]\)|[IVXLC]+ ?[-–.)]|•|-|\d+[.)])`) e começa com letra minúscula ou dígito.

  Se `i` termina em `-` e a próxima começa com minúscula, junta sem espaço e sem o hífen.

- [ ] **Passo 1:** Testes que falham:
  - `TestLinhaLetraSolta`: `o`, `Ss` e `E` saem; `II`, `a)`, `SP` e `5` ficam;
  - `TestRefluir_Paragrafo`: 3 linhas de um parágrafo viram 1;
  - `TestRefluir_ExtratoUmaLinhaPorLancamento`: `01/01/2026 PIX ENVIADO 150,00 1.000,00` e mais 2 lançamentos continuam 3 linhas;
  - `TestRefluir_ListaNaoJunta`: `a) primeiro item` e `b) segundo item` continuam 2;
  - `TestRefluir_Hifenizacao`: `pala-` e `vra seguinte` viram `palavra seguinte`;
  - `TestMaxima_PaginaCertificado`.
- [ ] **Passo 2:** Os testes falham.
- [ ] **Passo 3:** Implementar. Atualize `TestCleanPage_AggressiveKeepsLegitimateContent`: as quebras entre parágrafos agora ficam.
- [ ] **Passo 4:** Os testes passam.
- [ ] **Passo 5:** Commit `feat(etl): Máxima tira ruído de OCR e reflui parágrafos em vez de juntar a página`.

### Tarefa 12: H10, omitir tipos de documento

**Arquivos:**
- Modificar: `internal/models/*.go` (`Config.OmitirTipos []string \`json:"omitir_tipos"\``)
- Modificar: `internal/config/config.go` (o backfill deixa a lista vazia; o overlay aceita a chave)
- Modificar: `internal/etl/pipeline.go` (pré-população e goroutine)
- Modificar: `internal/etl/formatter.go`
- Modificar: `frontend/src/components/Settings.svelte`
- Teste: `internal/etl/pipeline_test.go`, `internal/etl/formatter_test.go`, `internal/config/config_test.go`

**Interfaces:**
- `tipoDocumento(docName string) string`: tira o prefixo `Evento N - ` (`eventPrefixRegex`), a extensão e os dígitos finais, e devolve em maiúsculas. `"Evento 3 - EXTR2.pdf"` vira `"EXTR"`.
- Na pré-população, quando `slices.Contains(cfg.OmitirTipos, tipoDocumento(f.DocName))`, o mapa do documento ganha `"omitido": tipo` e `"total_paginas": contagens[i]`. A goroutine desse documento chama `encerrar("", ...)` sem abrir o arquivo, e `encerrar` completa as páginas.
- O formatter escreve `[documento omitido: TIPO, N páginas]` como corpo, ou `1 página` quando N é 1. O resumo continua listando o documento.
- Settings: campo de texto "Omitir documentos dos tipos" com códigos separados por vírgula, normalizados para maiúsculas e sem espaços. A ajuda diz: "Ex.: EXTR, COMP. O documento vira um aviso no texto e não é lido."

- [ ] **Passo 1:** Testes que falham:
  - `TestTipoDocumento`, com 4 casos;
  - `TestProcessZip_OmiteTipoSemLer`: um ZIP sintético com `Evento 1 - INIC1.pdf` e `Evento 2 - EXTR1.pdf`, e `OmitirTipos: ["EXTR"]`. A saída contém `[documento omitido: EXTR, 1 página]` e não contém o texto do extrato; o OCR fake não foi chamado para ele; o progresso chega a 100%;
  - `TestLoad_OmitirTiposAusenteViraVazio`.
- [ ] **Passo 2:** Os testes falham.
- [ ] **Passo 3:** Implementar o backend. Rodar `wails generate module` se `models.Config` aparecer nos bindings. Implementar o campo em `Settings.svelte`.
- [ ] **Passo 4:** `go test -race ./internal/...`, `npx svelte-check` e os testes do frontend passam.
- [ ] **Passo 5:** Commit `feat: omitir tipos de documento escolhidos pelo usuário`.

### Tarefa 13: Interface dos quatro níveis

**Arquivos:**
- Modificar: `frontend/src/lib/atividade.js` (`OPCOES_LIMPEZA`)
- Modificar: `frontend/src/components/Settings.svelte` (`<select>`, `AJUDA_LIMPEZA`, padrão `standard`)
- Modificar: `scripts/visual/estados.mjs` (config fictícia com `standard`)
- Teste: `frontend/atividade.test.mjs` e os testes de convenção que citam os níveis

- `OPCOES_LIMPEZA`: `none`/Nenhuma, `basic`/Básica, `standard`/Padrão e `aggressive`/Máxima, nessa ordem. Ajuda, copiada da spec:
  - `none`: "Texto como extraído";
  - `basic`: "Remove espaços e linhas vazias";
  - `standard`: "Remove cabeçalhos, rodapés e duplicatas; resume assinaturas";
  - `aggressive`: "Padrão + ruído de OCR e parágrafos contínuos".

- [ ] **Passo 1:** Testes que falham:
  - `proximaOpcao(OPCOES_LIMPEZA, 'standard')` é `'aggressive'`;
  - `proximaOpcao(OPCOES_LIMPEZA, 'aggressive')` é `'none'`;
  - `rotuloOpcao(OPCOES_LIMPEZA, 'standard')` é `'Padrão'`;
  - um teste de convenção diz que o `<select>` de `Settings.svelte` tem as 4 `option` com esses valores.
- [ ] **Passo 2:** Os testes falham.
- [ ] **Passo 3:** Implementar. O indicador de nível da barra recolhida precisa mostrar 4 níveis: conferir o componente que usa `nivelOpcao`.
- [ ] **Passo 4:** Gate de frontend e captura `node scripts/visual/capturar.mjs --saida <scratchpad>/visual --estados configuracoes --rapido`. Abrir a captura e conferir o select e a ajuda.
- [ ] **Passo 5:** Commit `feat(frontend): quatro níveis de higienização (Nenhuma, Básica, Padrão, Máxima)`.

### Tarefa 14: Medição real, documentação e gate final

**Arquivos:**
- Modificar: `CLAUDE.md` (Architecture: os níveis em `higienizacao.go`, H7 e H10 na pré-população)
- Criar: `cmd/zzmedir/` só no scratchpad, nunca no repositório. Fica fora de qualquer commit.

- [ ] **Passo 1:** Copiar os ZIPs de `%USERPROFILE%\OneDrive\Área de Trabalho\Processos` (ou `Desktop\Processos`) para o scratchpad. Rodar `ProcessZip` em cada um com os 4 níveis e o OCR `fast`. Imprimir só a tabela `ZIP (A/B/C/D) × nível → tokens, Δ% sobre a basic de main`.
  - Para a linha de base, a mesma medição no main num worktree temporário.
- [ ] **Passo 2:** Conferir a meta: a Padrão economiza pelo menos 15% sobre a Básica de main na média dos 4 processos. Se não atingir, parar e relatar ao dono os números, sem ajustar limiares às cegas.
- [ ] **Passo 3:** Conferir por amostragem, só olhando e sem copiar trechos para relatório nem commit:
  - acentos nos HTMLs;
  - `[assinado: ...]` com nome nos HTMLs;
  - extratos com um lançamento por linha na Máxima;
  - contestação legível.
- [ ] **Passo 4:** Apagar as cópias, as saídas e o worktree. Rodar `pwsh scripts/build/gate.ps1`, o gate de frontend e os testes de harness (`node --test .claude/hooks/hooks.test.mjs .claude/statusline.test.mjs .claude/workflows/workflows.test.mjs scripts/visual/estados.test.mjs`).
- [ ] **Passo 5:** Atualizar o `CLAUDE.md` e fazer o commit `docs: CLAUDE.md descreve os níveis de higienização`.
