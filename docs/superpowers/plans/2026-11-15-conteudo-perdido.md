# Páginas "lidas" que perdem conteúdo (v0.4.2): plano de implementação

> **Para agentes:** sub-skill obrigatória: use superpowers:subagent-driven-development (recomendado) ou superpowers:executing-plans para implementar este plano tarefa por tarefa. Os passos usam checkbox (`- [ ]`) para acompanhamento.

**Objetivo:** O texto dentro de Form XObject passa a sair no `.txt`; a página com imagem cobrindo boa parte da área sem texto por perto, e a página com imagem cujo texto nativo não parece português, vão ao OCR sozinhas; e o OCR nunca troca um texto nativo por um resultado em branco ou ilegível.

**Arquitetura:**
- **Uma passada.** O interpretador de `internal/etl/pdf_texto.go` vira `interpretarPagina`, que devolve os trechos de texto (da página e dos Forms), as imagens colocadas e o MediaBox. `extrairTextoPosicional` continua com a mesma assinatura e passa a ser uma casca.
- **Estrutura.** `internal/etl/pdf_estrutura.go` transforma a passada em três frações (`fImg`, `fTxt`, `fDesc`) numa grade de 48×68 células. É função pura, testada sem PDF.
- **Língua.** `internal/etl/lingua.go` conta palavras, implausíveis e funcionais do texto nativo. Também pura.
- **Junção.** `ocrTask` leva o motivo da candidatura, e `conteudoAposOCR` (`internal/etl/pdf_juncao.go`) decide o conteúdo da página. `registrarOCR` só chama essa função.
- **Decisão.** `analisarPagina` aplica as regras de hoje e, depois delas, as duas novas. Com o OCR desligado as regras novas não fazem nada.

**Tecnologias:** Go 1.27.3, `github.com/dslipak/pdf` v0.0.2. Sem dependência nova, sem binário novo, sem mudança no frontend.

**Spec:** `docs/superpowers/specs/2026-11-15-conteudo-perdido-design.md`. Leia junto com este plano; os valores (limiares, limites, lista de palavras funcionais) estão lá e são copiados aqui sem mudança.

## Restrições globais

- A branch é `ocr-conteudo-perdido`, criada a partir da `main` (`2a8564f`). Um commit por tarefa, com a mensagem indicada na tarefa, corpo curto em pt-BR e as linhas de atribuição da sessão.
- Nunca dê push, merge ou release. Só a sessão principal faz isso, e só quando o dono pede.
- Não faça commit de `README.md`, `build/windows/installer/project.nsi`, `install.ps1` nem `.genkit/`: são mudanças locais do dono. Use `git add` com caminhos explícitos, nunca `-A` ou `.`.
- Todo PDF de teste é sintético e gerado no próprio código de teste (`buildPDF` em `pdf_parser_test.go`), sem nome real, número de processo, CPF ou endereço. Placeholders: `FULANO DE TAL`, `5000000-00.2026.8.26.0001`.
- Os ZIPs reais só entram na Task 7, pelo controlador, em cópias no scratchpad, e dela só saem contagens. Nenhum subagente abre arquivo real.
- No máximo 3 subagentes ao mesmo tempo. Tarefas de implementação rodam uma por vez.
- Decisão do dono, "só automático": nenhum aviso, marcador, botão, status ou valor de `Leitura` novo. O formato do `.txt` (PRD §6) não muda. O frontend não muda.
- Uma página sem Form e sem imagem útil sai byte a byte igual à v0.4.1. Os testes existentes de `internal/etl` continuam passando sem alteração, salvo os que fixam o comportamento antigo do OCR em branco (Task 4), que são listados no relatório.
- `go.mod` e `go.sum` não mudam. Nada entra em `internal/ocr/embed/`.
- As linhas de log novas levam só números e textos fixos: nenhum texto de página, nome de arquivo, caminho, nome de objeto do PDF ou erro bruto.
- A pipeline não muda de ordem nem de concorrência.
- Logs e comentários em pt-BR.
- Antes de cada commit: `go vet ./...` e `go test -race ./internal/...`. A suíte tem `TestModernizacao_GoFixSemDiff`, que reprova código que o `go fix` reescreveria (por exemplo `strings.Split` em laço `range`, que vira `SplitSeq`).
- O formato de `gofmt` é conferido com `git show HEAD:<arquivo> | gofmt -d` (o checkout usa `core.autocrlf=true`).
- O protótipo do brainstorm está em `.superpowers/sdd/2026-11-15-conteudo-perdido/referencia/` (`estrutura.go`, `gerar_pdfs.py`). É referência de como medir e de como montar as páginas sintéticas, não código a copiar: foi escrito para medir, sem as proteções deste plano.

## Foco da revisão

Entradas que a spec implica e que um usuário vai encontrar. Cada uma tem um teste na tarefa dona:

1. **Form que derruba o interpretador** (filtro não suportado, que faz a biblioteca entrar em pânico; Form que chama a si mesmo; dois Forms que se chamam; 300 Forms numa página): o texto da própria página continua saindo, sem travar. Antes deste plano o Form era ignorado, então perder a página inteira por causa dele seria regressão. Task 1.
2. **MediaBox com origem deslocada ou degenerado** (`[100 100 695 942]`, `[0 0 0 0]`, herdado do `/Parent`, ausente): a grade e o descarte de texto fora da página usam a origem real; sem MediaBox legível não há estrutura nem descarte. Tasks 1 e 2.
3. **Imagem com a CTM espelhada ou girada** (`595 0 0 -842 0 842 cm`, comum em digitalizações; giro de 90°): cobre as mesmas células que a imagem reta. Task 2.
4. **A junção nunca perde uma linha nativa**, e um OCR só de espaços, quebras de linha ou `\f` conta como em branco. Task 4.
5. **Comprovante em português com muitos códigos e uma imagem** (chave aleatória, identificador de transação, autenticação em hexadecimal, rótulos em português): não é isca, o texto nativo fica. Tasks 3 e 5.

---

### Task 1: Texto de Form XObject no texto nativo

**Arquivos:**
- Modificar: `internal/etl/pdf_texto.go`
- Criar: `internal/etl/pdf_estrutura.go` (nesta tarefa só `caixa` e `mediaBoxDe`), `internal/etl/pdf_texto_form_test.go`

**Interfaces:**
- Consome: `lerStream`, `carregarFonte`, `montarLinhas`, `matriz`, `trecho` (todos em `pdf_texto.go`); auxiliares de teste `paginaComFonte`, `extrairComPrazo`, `primeiraPagina`, `helvetica`.
- Produz:

```go
const (
	formMaxProfundidade = 4
	formMaxPorPagina    = 256
)

// caixa é um retângulo em pontos, já normalizado: x0 <= x1 e y0 <= y1.
type caixa struct{ x0, y0, x1, y1 float64 }

// mediaBoxDe lê o /MediaBox da página, subindo por /Parent (no máximo 32
// níveis). Devolve false quando não há, não tem 4 números ou tem área zero.
func mediaBoxDe(page pdf.Page) (caixa, bool)

// passadaPagina é o que uma passada pelos operadores da página produz.
type passadaPagina struct {
	trechos     []trecho // da página e dos Forms, em coordenadas da página
	trechosForm int      // quantos vieram de Form, depois das proteções
	mediaBox    caixa
	temMediaBox bool
}

func interpretarPagina(page pdf.Page) (passadaPagina, error)

// Assinatura inalterada: interpretarPagina mais montarLinhas.
func extrairTextoPosicional(page pdf.Page) (string, error)
```

Auxiliares de teste (em `pdf_texto_form_test.go`):

```go
// paginaComXObjects é paginaComFonte com a Helvetica em /F1 e com
// /XObject << xobjects >> nos recursos da página; extras viram os objetos 6, 7...
func paginaComXObjects(stream, xobjects string, extras ...string) []string

// formXObject devolve o corpo de um objeto Form (dicionário com /Type /XObject
// /Subtype /Form /BBox [0 0 595 842], mais dicExtra, e o stream), pronto para
// ser passado em extras.
func formXObject(conteudo, dicExtra string) string
```

**Regras (da spec, seção 1):**
- `Do` de um XObject `/Subtype /Form`: o stream passa por `lerStream` antes de o lexer vê-lo; `/Matrix` multiplica a CTM; o estado gráfico (CTM, fonte, tamanho, leading) é salvo antes e restaurado depois, mesmo se a interpretação do Form parar no meio.
- Fontes e XObjects vêm dos `/Resources` do Form; sem eles, dos recursos de quem o chamou. O cache de fontes é por escopo de recursos: `/F1` dentro de um Form com recursos próprios é a fonte do Form. Toda fonte passa por `carregarFonte`, como hoje.
- Recursão até a profundidade 4; no máximo 256 Forms interpretados por página, contando os aninhados.
- Cada Form roda sob o seu próprio `recover`: um pânico descarta os trechos daquele Form e a página segue.
- Ignorado por inteiro: Form com `/OC` no dicionário; `Do` dentro de `/OC … BDC … EMC`. Dentro de um Form, texto mostrado num trecho `/OC … BDC … EMC` também é ignorado. O texto do `/Contents` da página dentro de um trecho desses segue saindo, como hoje. Isso pede uma pilha para `BDC`/`BMC`/`EMC`.
- Trecho de Form cuja origem cai fora do MediaBox é descartado; sem MediaBox legível o trecho entra.
- Trechos de Form com o mesmo texto e a mesma posição (arredondada a 0,5 pt) contam uma vez.
- As duas últimas proteções valem só para trechos de Form. `Do` de imagem ou de nome que não existe nos recursos não faz nada nesta tarefa.
- Form ignorado gera uma linha de log com motivo fixo (`stream ilegível`, `profundidade`, `limite por página`, `pânico`), sem o erro bruto e sem o nome do objeto.

- [ ] **Passo 1: escrever os testes que falham** em `pdf_texto_form_test.go`

Todos usam `extrairComPrazo` (falha se travar em 5 s), salvo os que leem `interpretarPagina` direto. Nos streams abaixo, `T(x, y, s)` abrevia `BT /F1 12 Tf x y Td (s) Tj ET`.

| teste | página e Form | o que o teste afirma |
|---|---|---|
| `TestForm_CorpoEmFormMaisLegenda` | página: `T(72,60,"Legenda da figura") /Fm1 Do`; Form: `T(72,700,"PRIMEIRA LINHA DO CORPO") T(72,680,"SEGUNDA LINHA DO CORPO") T(72,660,"TERCEIRA LINHA DO CORPO")` | as quatro linhas saem, nessa ordem (as três do corpo, depois a legenda) |
| `TestForm_MatrizDoForm` | página: `/Fm1 Do T(72,400,"ACIMA")`; Form com `/Matrix [1 0 0 1 0 -600]`: `T(72,700,"ABAIXO")` | `ACIMA` vem antes de `ABAIXO` |
| `TestForm_EstadoRestaurado` | página: `/Fm1 Do T(72,400,"ESQUERDA")`; Form: `1 0 0 1 300 -300 cm T(72,700,"DIREITA")` | o texto é exatamente `ESQUERDA DIREITA` (uma linha; se a CTM vazasse, `ESQUERDA` cairia em y=100) |
| `TestForm_RecursosDoForm` | página: `T(72,700,"AAA") /Fm1 Do`; Form com `/Resources << /Font << /F1 7 0 R >> >>`, objeto 7 = Helvetica com `/Encoding << /Type /Encoding /Differences [65 /Z] >>`: `T(72,600,"AAA")` | sai `AAA` e depois `ZZZ`. Se a biblioteca não aplicar `/Differences` nesse formato, use um `/ToUnicode` próprio: o que o teste fixa é que `/F1` dentro do Form é a fonte do Form |
| `TestForm_SemRecursosUsaOsDaPagina` | Form sem `/Resources`: `T(72,600,"HERDADO")` | sai `HERDADO` |
| `TestForm_CamadaOpcional` | (a) Form com `/OC << /Type /OCG /Name (Oculta) >>`; (b) página `/OC /MC0 BDC /Fm1 Do EMC`; (c) Form `/OC /MC0 BDC T(72,700,"OCULTO") EMC T(72,680,"VISIVEL")`; (d) página `/OC /MC0 BDC T(72,700,"NA PAGINA") EMC` | (a) e (b): o texto do Form não sai; (c): só `VISIVEL`; (d): `NA PAGINA` sai |
| `TestForm_ForaDaPagina` | (a) página `q 1 0 0 1 0 2000 cm /Fm1 Do Q T(72,700,"DENTRO")`, Form `T(72,700,"FORA")`; (b) MediaBox `[100 100 695 942]`, Form `T(50,500,"FORA") T(650,900,"DENTRO")`; (c) como (a), sem `/MediaBox` | (a) e (b): só `DENTRO`; (c): `FORA` e `DENTRO` saem |
| `TestForm_PintadoDuasVezes` | (a) `/Fm1 Do /Fm1 Do`; (b) `/Fm1 Do q 1 0 0 1 0 -100 cm /Fm1 Do Q` | (a) o texto do Form aparece 1 vez; (b) 2 vezes |
| `TestForm_Recursao` | (a) Form cujos recursos apontam `/Fm1` para ele mesmo e cujo conteúdo é `T(72,700,"EU") /Fm1 Do`; (b) `/Fm1` chama `/Fm2`, que chama `/Fm1` | termina; `EU` aparece 1 vez; em (b) os dois textos saem 1 vez cada |
| `TestForm_Limite256PorPagina` | página com 300 linhas `q 1 0 0 1 <i> 0 cm /Fm1 Do Q` (i de 0 a 299); Form `T(72,700,"X")` | `interpretarPagina(...).trechosForm == 256` |
| `TestForm_StreamIlegivel` | página `T(72,700,"DA PAGINA") /Fm1 Do`; stream do Form: `BT /F1 12 Tf (sem fecho` | sai `DA PAGINA`, sem travar |
| `TestForm_FiltroNaoSuportado` | como o anterior, Form com `/Filter /DCTDecode` | sai `DA PAGINA` |
| `TestForm_DoQueNaoEForm` | página `T(72,700,"TEXTO") /Im9 Do /Im1 Do`, com `/Im1` imagem e `/Im9` inexistente | sai exatamente `TEXTO` |
| `TestForm_ContaTrechos` | o PDF de `CorpoEmFormMaisLegenda`; e uma página sem Form | `trechosForm == 3`; `trechosForm == 0` |
| `TestMediaBoxDe` | próprio `[0 0 595 842]`; herdado do `/Parent`; ausente; `[0 0 0 0]`; `[595 842 0 0]`; `[0 0 595]` | caixa certa e `true`; herdada e `true`; `false`; `false`; normalizada (`0,0,595,842`) e `true`; `false` |

- [ ] **Passo 2: rodar e ver falhar**

`go test ./internal/etl -run 'TestForm_|TestMediaBoxDe' -v`. Esperado: não compila (`interpretarPagina`, `mediaBoxDe` indefinidos). Depois de criar cascas vazias, `TestForm_CorpoEmFormMaisLegenda` falha porque só a legenda sai. Guarde a linha decisiva.

- [ ] **Passo 3: implementar** `caixa` e `mediaBoxDe` em `pdf_estrutura.go`, e `interpretarPagina` em `pdf_texto.go`

O corpo de `extrairTextoPosicional` passa para um interpretador com estado (uma struct com os campos que hoje são variáveis locais), para que o `Do` possa chamá-lo de novo sobre o stream do Form. O tratamento dos operadores de texto não muda.

- [ ] **Passo 4: rodar e ver passar**

`go test ./internal/etl -run 'TestForm_|TestMediaBoxDe|TestTextoPosicional' -v`, depois o gate (`go vet ./...`, `go test -race ./internal/...`). Nenhum teste existente muda.

- [ ] **Passo 5: commit**

`git add internal/etl/pdf_texto.go internal/etl/pdf_estrutura.go internal/etl/pdf_texto_form_test.go`; mensagem `feat(etl): texto de Form XObject entra no texto nativo`.

---

### Task 2: Estrutura da página (imagem, texto, imagem descoberta)

**Arquivos:**
- Modificar: `internal/etl/pdf_estrutura.go`, `internal/etl/pdf_texto.go` (registrar as imagens), `internal/etl/pdf_parser.go` (`ExtractNativeText`, `textoPosicionalSeguro`)
- Criar: `internal/etl/pdf_estrutura_test.go`, `internal/etl/pdf_sinteticos_test.go`

**Interfaces:**
- Consome: `passadaPagina`, `interpretarPagina`, `caixa`, `paginaComXObjects`, `formXObject` (Task 1).
- Produz:

```go
const (
	gradeColunas    = 48
	gradeLinhas     = 68
	imagemMinPixels = 256
	imagemMinDPI    = 50.0
)

// imagemColocada é uma imagem pintada na página: a CTM leva o quadrado
// unitário ao lugar onde ela aparece.
type imagemColocada struct {
	ctm            matriz
	largPx, altPx  int
}

// passadaPagina ganha o campo:  imagens []imagemColocada

type metricasPagina struct {
	valida            bool // false: sem MediaBox legível, ou o texto veio do GetTextByRow
	fImg, fTxt, fDesc float64
	trechosForm       int
}

// medirPagina é pura. Sem imagem útil devolve valida=true e as três frações em
// zero, sem montar a grade.
func medirPagina(mediaBox caixa, imagens []imagemColocada, textos []caixa) metricasPagina

func (p passadaPagina) metricas() metricasPagina

func extrairNativo(page pdf.Page) (string, metricasPagina, error)       // extrairTextoPosicional vira casca desta
func extrairNativoSeguro(page pdf.Page) (string, metricasPagina, error) // ExtractNativeText vira casca desta
```

Auxiliares de teste (em `pdf_sinteticos_test.go`):

```go
// imagemXObject devolve o corpo de um objeto imagem que declara w×h pixels, com
// um stream mínimo (nada lê o stream nestes testes), no padrão de
// objetosPaginasComImagem.
func imagemXObject(w, h int) string

type paginaSintetica struct {
	nome            string
	objetos         []string // para buildPDF e primeiraPagina
	candidataImagem bool     // fDesc >= 0,30 e fTxt <= 0,10
	isca            bool     // a regra de isca inteira (língua e fImg >= 0,10)
}

func paginasSinteticas() []paginaSintetica
func paginaSinteticaDe(t *testing.T, nome string) paginaSintetica
```

**Regras (da spec, seção 2):**
- Imagem: cada `Do` de `/Subtype /Image`, na página ou dentro de Form, e cada imagem inline (`BI … ID … EI`; o protótipo em `referencia/estrutura.go` mostra como a biblioteca a entrega). Guarda a CTM do momento e `/Width` e `/Height` (ou `/W` e `/H` na inline).
- Imagem útil: `largPx*altPx >= 256` e pelo menos 50 dpi nos dois eixos. O tamanho em pontos de cada eixo é o comprimento do vetor transformado (`hypot`), então vale para imagem espelhada ou girada.
- Caixa de texto: de `x` a `fimX` na largura, de `y` a `y + tamanho` na altura. Texto invisível (`Tr 3`) conta: o interpretador não distingue.
- Grade de 48 colunas por 68 linhas sobre o MediaBox, com a origem real dele. Célula de imagem: o centro cai dentro de uma imagem útil (teste pelo inverso da CTM, para valer com espelho e giro). Célula de texto: alguma caixa de texto a toca.
- `fImg` e `fTxt`: fração de células de imagem e de texto. `fDesc`: fração de células de imagem sem célula de texto a até uma célula de distância (as 8 vizinhas e ela mesma).
- `valida = false` quando não há MediaBox legível, e em `extrairNativoSeguro` quando o texto veio do `GetTextByRow` ou a extração posicional falhou.
- Esta tarefa não muda o comportamento: `analisarPagina` continua chamando `ExtractNativeText`.

**Páginas sintéticas** (`paginasSinteticas()`; A4 `[0 0 595 842]`; texto fictício em português). As legendas das páginas que têm de ir ao OCR têm 100 bytes ou mais: com menos, a regra de hoje já as pega.

| nome | conteúdo | fImg | fTxt | fDesc | candidataImagem | isca |
|---|---|---|---|---|---|---|
| `texto-puro` | página cheia de texto | 0 | (não medido) | 0 | não | não |
| `extrato-nativo` | tabela de datas e valores em texto | 0 | (não medido) | 0 | não | não |
| `certidao-hashes` | texto com hashes hexadecimais | 0 | (não medido) | 0 | não | não |
| `rodapes-urls` | texto com URLs e códigos nos rodapés | 0 | (não medido) | 0 | não | não |
| `logotipo` | texto mais imagem pequena no topo | 0,01–0,02 | 0,46–0,51 | 0,01–0,02 | não | não |
| `assinatura-digitalizada` | texto mais imagem pequena no fim | 0,01–0,02 | 0,46–0,51 | 0,01–0,02 | não | não |
| `timbre-meia-pagina` | imagem de página inteira, meia página de texto | 1,00 | 0,25 | 0,71 | não | não |
| `timbre-pagina-cheia` | imagem de página inteira, página cheia de texto | 1,00 | > 0,10 | (livre) | não | não |
| `digitalizado-camada-ocr` | imagem de página inteira, texto `3 Tr` sobre ela | 1,00 | 0,55 | 0,39 | não | não |
| `peticao-print` | texto mais imagem de 400×300 pt | 0,24 | 0,25 | 0,24 | não | não |
| `comprovante-codigos` | rótulos em português, códigos, imagem de 25% da página ao lado do texto | ≥ 0,10 | > 0,10 | (livre) | não | não |
| `legenda-imagem-direta` | legenda de 2 linhas mais imagem (`Do` na página) | 0,96 | 0,02 | 0,95 | sim | não |
| `legenda-imagem-form` | a mesma, com a imagem dentro de um Form | 0,96 | 0,02 | 0,95 | sim | não |
| `legenda-imagem-inline` | a mesma, com imagem inline em hexadecimal | 0,96 | 0,02 | 0,95 | sim | não |
| `carimbo-sobre-digitalizacao` | imagem de página inteira, carimbo de 2 linhas | 1,00 | 0,01 | 0,97 | sim | não |
| `mosaico-legenda` | 237 imagens pequenas mais legenda de 2 linhas | 0,42 | 0,02 | 0,42 | sim | não |
| `mosaico-legenda-curta` | o mosaico, com legenda de menos de 100 bytes | 0,42 | ≤ 0,02 | 0,42 | sim | não |
| `timbre-3-linhas` | imagem de página inteira mais 3 linhas | 1,00 | 0,02 | 0,97 | sim (falso positivo documentado) | não |
| `isca-cinza` | 80 palavras aleatórias em cinza claro espalhadas; imagens pequenas com a informação | ≥ 0,10 | (livre) | (livre) | (livre) | sim |
| `isca-preta` | a mesma, em preto | ≥ 0,10 | (livre) | (livre) | (livre) | sim |
| `isca-linhas` | a mesma, em linhas regulares | ≥ 0,10 | (livre) | (livre) | (livre) | sim |

Os números vêm da tabela da spec; o teste aceita ±0,05 em cada um. "(livre)" não é conferido. O campo `isca` é só o esperado: quem o confere é a Task 5.

- [ ] **Passo 1: escrever os testes que falham**

Em `pdf_estrutura_test.go`, sobre `medirPagina` (sem PDF), com MediaBox `caixa{0, 0, 595, 842}` salvo indicação:

| teste | entrada | afirma |
|---|---|---|
| `TestMedir_SemImagem` | nenhuma imagem, 40 caixas de texto | `valida`, e as três frações iguais a 0 |
| `TestMedir_PaginaInteira` | imagem 1240×1754 px, CTM `{595, 0, 0, 842, 0, 0}`, sem texto | `fImg` e `fDesc` ≥ 0,99; `fTxt` = 0 |
| `TestMedir_Espelhada` | a mesma com CTM `{595, 0, 0, -842, 0, 842}` | igual ao anterior |
| `TestMedir_Girada90` | imagem 1754×1240 px, CTM `{0, 842, -595, 0, 595, 0}` | `fImg` ≥ 0,99 |
| `TestMedir_ImagemInutil` | 1×1 px na página inteira; 100×100 px na página inteira (12 dpi) | `fImg` = 0 nos dois |
| `TestMedir_MeiaPaginaTextoLonge` | imagem na metade de baixo; uma caixa de texto no topo | `fImg` e `fDesc` entre 0,48 e 0,52 |
| `TestMedir_TextoEncostado` | a mesma imagem; uma linha de texto de margem a margem logo acima dela | `fDesc` < `fImg` |
| `TestMedir_TextoSobreAImagem` | imagem de página inteira; 30 linhas de texto de margem a margem | `fImg` ≥ 0,99; `fDesc` < 0,60 |
| `TestMedir_OrigemDeslocada` | MediaBox `{100, 100, 695, 942}`; CTM `{595, 0, 0, 842, 100, 100}` | `fImg` ≥ 0,99 |
| `TestMedir_ImagemParteFora` | imagem que começa em x = 297 e vai até x = 1190 | `fImg` entre 0,48 e 0,52 |

Em `pdf_sinteticos_test.go`:

```go
func TestSinteticas_Metricas(t *testing.T) {
	for _, p := range paginasSinteticas() {
		_, m, err := extrairNativoSeguro(primeiraPagina(t, p.objetos))
		// err == nil; m.valida; cada fração dentro do intervalo da tabela (±0,05);
		// (m.fDesc >= 0.30 && m.fTxt <= 0.10) == p.candidataImagem, salvo "(livre)"
	}
}
```

Mais `TestExtrairNativoSeguro_IgualAoExtractNativeText` (para toda página sintética o texto das duas funções é idêntico) e `TestExtrairNativoSeguro_SemMediaBoxNaoEValida` (a página `legenda-imagem-direta` sem `/MediaBox`: `valida == false`, texto igual).

- [ ] **Passo 2: rodar e ver falhar**

`go test ./internal/etl -run 'TestMedir_|TestSinteticas_|TestExtrairNativoSeguro_' -v`. Esperado: não compila; com cascas, `TestMedir_PaginaInteira` falha com `fImg = 0`.

- [ ] **Passo 3: implementar** `medirPagina`, `metricas`, o registro das imagens no interpretador, `extrairNativo` e `extrairNativoSeguro`

`textoPosicionalSeguro` pode mudar de assinatura para devolver as métricas (só `ExtractNativeText` a chama). `ExtractNativeText` e `extrairTextoPosicional` mantêm a assinatura.

- [ ] **Passo 4: rodar e ver passar**, depois o gate. Nenhum teste existente muda.

- [ ] **Passo 5: commit**

`git add internal/etl/pdf_estrutura.go internal/etl/pdf_texto.go internal/etl/pdf_parser.go internal/etl/pdf_estrutura_test.go internal/etl/pdf_sinteticos_test.go`; mensagem `feat(etl): estrutura da página (imagem, texto, imagem descoberta)`.

---

### Task 3: Teste de língua para o texto-isca

**Arquivos:**
- Criar: `internal/etl/lingua.go`, `internal/etl/lingua_test.go`

**Interfaces:**
- Consome: nada.
- Produz:

```go
const (
	iscaMinPalavras     = 30
	iscaMinImplausiveis = 0.35
	iscaMaxFuncionais   = 0.05
)

type contagemLingua struct{ palavras, implausiveis, funcionais int }

func contarLingua(texto string) contagemLingua

// isca: palavras >= 30, implausíveis >= 35% delas e funcionais < 5% delas.
// A exigência de imagem (fImg >= 0,10) fica com quem chama (Task 5).
func (c contagemLingua) isca() bool

// normalizarPalavra devolve a palavra em minúsculas e sem acento, por uma
// tabela própria dos acentos do português (sem dependência nova).
func normalizarPalavra(s string) string
```

**Regras (da spec, seção 3):** uma palavra é um pedaço entre espaços, sem a pontuação das pontas.
- **Conta como palavra:** tem letra e 3 caracteres ou mais; ou é funcional (qualquer tamanho).
- **Funcionais:** `de da do das dos a o as os e em no na nos nas para por com que não ao à aos se um uma é`, comparadas em minúsculas.
- **Neutras (não entram em nenhuma conta):** só dígitos e pontuação; hexadecimal com pelo menos um dígito.
- **Implausíveis:** duas ou mais trocas entre letra e dígito; minúscula seguida de maiúscula no meio; 4 letras ou mais com menos de 15% de vogais (com acento contam), salvo sigla toda em maiúsculas de até 5 letras.

- [ ] **Passo 1: escrever os testes que falham** em `lingua_test.go`

`TestContarLingua_Palavras`, tabela por palavra (entrada de uma palavra; esperado `{palavras, implausiveis, funcionais}`):

| entrada | esperado |
|---|---|
| `k7Qz9X` | `{1, 1, 0}` |
| `aBc` | `{1, 1, 0}` |
| `xkcdt` | `{1, 1, 0}` |
| `QWRTPS` | `{1, 1, 0}` |
| `STJ`, `CNPJ`, `TJSP` | `{1, 0, 0}` cada |
| `petição`, `Requerente,`, `(artigo)` | `{1, 0, 0}` cada |
| `de`, `É`, `não`, `à` | `{1, 0, 1}` cada |
| `12/03/2026`, `1.234,56`, `123.456.789-00`, `5000000-00.2026.8.26.0001` | `{0, 0, 0}` cada |
| `a3f9c2e1b7d4` | `{0, 0, 0}` |
| `ab`, `--`, `` (vazio) | `{0, 0, 0}` cada |

`TestContarLingua_Textos`:

| texto | afirma |
|---|---|
| parágrafo fictício em português de 60 palavras ou mais | `implausiveis == 0`; `funcionais*100 >= 15*palavras`; `!isca()` |
| 80 palavras aleatórias fixas no teste, do tipo `k7Qz9X`, `Xq4Lm2`, `bTrWq` | `isca()` |
| as mesmas, cortadas em 29 | `!isca()` |
| rodapés com URLs e códigos entre frases em português (cerca de 30% de implausíveis) | `!isca()` |
| comprovante: `Comprovante de transferência Pix Valor R$ 1.234,56 Data 12/03/2026 Pagador FULANO DE TAL Banco do Exemplo Agência 0001 Conta 12345-6 Recebedor BELTRANO DE TAL Chave Pix a1b2c3d4-e5f6-7890-abcd-ef1234567890 ID da transação E12345678202603121234abcdEFGH567 Autenticação 9F8E7D6C5B4A39281706F5E4D3C2B1A0` | `!isca()` e `funcionais*100 >= 5*palavras` |
| texto vazio | `contagemLingua{}` e `!isca()` |

`TestNormalizarPalavra`: `Petição` → `peticao`; `ÀS` → `as`; `Órgão` → `orgao`; `abc123` → `abc123`.

- [ ] **Passo 2: rodar e ver falhar** (`go test ./internal/etl -run 'TestContarLingua_|TestNormalizarPalavra' -v`; não compila)
- [ ] **Passo 3: implementar** `lingua.go`
- [ ] **Passo 4: rodar e ver passar**, depois o gate
- [ ] **Passo 5: commit**

`git add internal/etl/lingua.go internal/etl/lingua_test.go`; mensagem `feat(etl): teste de língua para texto-isca`.

---

### Task 4: Junção do OCR com o texto nativo

**Arquivos:**
- Criar: `internal/etl/pdf_juncao.go`, `internal/etl/pdf_juncao_test.go`
- Modificar: `internal/etl/pdf_parser.go` (`ocrTask`), `internal/etl/pdf_ocr_despacho.go` (`registrarOCR`)
- Testes existentes que fixam o comportamento antigo do OCR em branco: podem ser atualizados; liste cada um no relatório, com o que mudou.

**Interfaces:**
- Consome: `normalizarPalavra` (Task 3), `textoIlegivel` (`pdf_parser.go`), auxiliares `parsePagesWithDeadline`, `pdfTextoCurtoEImagem`.
- Produz:

```go
type motivoOCR int

const (
	motivoPadrao           motivoOCR = iota // valor zero: texto curto ou ausente (regra de hoje) e o caminho do segundo leitor
	motivoIlegivel                          // texto nativo ilegível (regra de hoje)
	motivoImagemDescoberta                  // regra nova
	motivoIsca                              // regra nova
)

type ocrTask struct {
	pageNum  int
	fallback string // a reserva: o texto nativo, ou o marcador de ilegível
	motivo   motivoOCR
}

const juncaoMinPresenca = 0.60

// conteudoAposOCR decide o conteúdo de uma página cujo OCR terminou sem erro.
func conteudoAposOCR(task ocrTask, ocrText string) (content string, ocr bool)

// juntarOCRComNativo devolve o texto do OCR e, depois de uma linha em branco,
// as linhas de nativo que o OCR não trouxe, na ordem em que estavam.
func juntarOCRComNativo(ocrText, nativo string) string
```

**Regras (da spec, seção 4):**
- `conteudoAposOCR`:
  - sem reserva (`task.fallback == ""`): `(ocrText, true)`, qualquer que seja o texto, como hoje;
  - com reserva e OCR em branco (`strings.TrimSpace(ocrText) == ""`) ou reprovado por `textoIlegivel`: `(task.fallback, false)`;
  - com reserva e OCR aproveitável: `motivoPadrao`, `motivoIlegivel` e `motivoIsca` devolvem `(ocrText, true)`; `motivoImagemDescoberta` devolve `(juntarOCRComNativo(ocrText, task.fallback), true)`.
- `juntarOCRComNativo`: o conjunto de palavras do OCR são os pedaços de letras e dígitos com 3 caracteres ou mais, passados por `normalizarPalavra`. Uma linha nativa (sem espaços nas pontas, não vazia) está presente se 60% ou mais das suas palavras de 3 caracteres ou mais estão no conjunto. Linha sem nenhuma palavra dessas está presente se o texto dela aparece no texto do OCR. Sem linha faltando, devolve `ocrText` sem mudança. Com linhas faltando: `ocrText` sem o espaço do fim, uma linha em branco, e as linhas que faltam, uma por linha.
- `registrarOCR`, no ramo sem erro, grava `content` e `ocr` vindos de `conteudoAposOCR`. O ramo de erro não muda (`ocr_erro`, `ocr_prazo`, a reserva).
- Nesta tarefa ninguém ainda preenche `motivo` com as regras novas: `analisarPagina` muda na Task 5.

- [ ] **Passo 1: escrever os testes que falham** em `pdf_juncao_test.go`

`TestJuntar`, tabela:

| OCR | nativo | resultado |
|---|---|---|
| `PETICAO INICIAL\nFULANO DE TAL requer` | `Petição inicial` | igual ao OCR |
| `TEXTO DA IMAGEM` | `Figura 1: fachada do imóvel` | `TEXTO DA IMAGEM\n\nFigura 1: fachada do imóvel` |
| `alfa beta gama` | `alfa beta gama delta epsilon` (3 de 5) | igual ao OCR |
| `alfa beta` | `alfa beta gama delta epsilon` (2 de 5) | OCR, linha em branco, a linha nativa |
| `Página 12 de 40` | `12` | igual ao OCR |
| `Texto qualquer` | `12` | `Texto qualquer\n\n12` |
| `TEXTO\n\n` | `linha um aqui\nlinha dois aqui` | `TEXTO\n\nlinha um aqui\nlinha dois aqui` |
| `TEXTO` | `` (vazio) | igual ao OCR |

`TestJuntar_NuncaPerdeLinhaNativa`: com um OCR sem nenhuma palavra em comum, toda linha não vazia de um nativo de 6 linhas aparece inteira no resultado, na mesma ordem, depois do texto do OCR.

`TestConteudoAposOCR`, para cada motivo com reserva `"RESERVA NATIVA da página"`:

| OCR | esperado |
|---|---|
| `"TEXTO BOM DO OCR"` | padrão, ilegível e isca: `("TEXTO BOM DO OCR", true)`; imagem descoberta: `("TEXTO BOM DO OCR\n\nRESERVA NATIVA da página", true)` |
| `""`, `"  \n\t"`, `"\f"`, `"\n\f\n"` | `("RESERVA NATIVA da página", false)` em todos os motivos |
| um texto que `textoIlegivel` reprova (o teste confere essa premissa antes) | `("RESERVA NATIVA da página", false)` em todos |

E sem reserva (`fallback: ""`), em todos os motivos: OCR `"X"` dá `("X", true)`; OCR `""` dá `("", true)`.

`TestParsePages_OCREmBrancoMantemAReserva` (por `parsePagesWithDeadline`, com `pdfTextoCurtoEImagem` e um OCR falso que devolve `""`): o conteúdo da página é o texto nativo curto, `ocr == false`, sem `ocr_erro`. `TestParsePages_OCRComErroSegueComoHoje`: OCR falso que devolve erro; conteúdo é a reserva e `ocr_erro` está preenchido. Um teste pelo caminho do segundo leitor (com `trocarPoppler` e falsos, como os testes existentes desse caminho): página com texto curto e OCR em branco mantém o texto.

- [ ] **Passo 2: rodar e ver falhar** (`go test ./internal/etl -run 'TestJuntar|TestConteudoAposOCR|TestParsePages_OCR' -v`)
- [ ] **Passo 3: implementar** `pdf_juncao.go`, o campo `motivo` e a chamada em `registrarOCR`
- [ ] **Passo 4: rodar e ver passar**, depois o gate. Se um teste existente quebrar por fixar o OCR em branco substituindo a reserva, atualize-o para o comportamento novo e liste no relatório; qualquer outra quebra é para investigar.
- [ ] **Passo 5: commit**

`git add` dos arquivos tocados, por caminho; mensagem `feat(etl): junção do OCR com o texto nativo; OCR em branco não apaga a reserva`.

---

### Task 5: Regras novas na decisão de OCR

**Arquivos:**
- Modificar: `internal/etl/pdf_parser.go` (`analisePagina`, `analisarPagina`, a chamada de `d.agendar` em `ParsePages`)
- Criar: `internal/etl/pdf_regras.go`, `internal/etl/pdf_regras_test.go`

**Interfaces:**
- Consome: `extrairNativoSeguro`, `metricasPagina` (Task 2); `contarLingua`, `contagemLingua` (Task 3); `motivoOCR`, `ocrTask.motivo` (Task 4); `paginasSinteticas`, `primeiraPagina`.
- Produz:

```go
const (
	descobertaMinFDesc = 0.30
	descobertaMaxFTxt  = 0.10
	iscaMinFImg        = 0.10
)

// regraNova diz se uma das duas regras novas manda a página ao OCR. Pura.
// Isca vem antes de imagem descoberta. Com m.valida == false devolve false.
func regraNova(m metricasPagina, c contagemLingua) (motivoOCR, bool)

// linhaDiagnostico monta a linha de log da página: só números e a decisão.
func linhaDiagnostico(pageNum, bytesNativo int, m metricasPagina, c contagemLingua, decisao string) string

// analisePagina ganha o campo:  motivo motivoOCR
```

**Regras (da spec, seções 2, 3, "OCR desligado" e "Log de diagnóstico"):**
- `analisarPagina` passa a chamar `extrairNativoSeguro`. A ordem das regras:
  1. texto ilegível (como hoje), com `motivo = motivoIlegivel`;
  2. texto com menos de 100 bytes e imagem grande ou desenho vetorial (como hoje), com `motivoPadrao`;
  3. se nenhuma das duas mandou ao OCR: `regraNova`. Isca: `m.fImg >= 0,10` e `c.isca()`. Imagem descoberta: `m.fDesc >= 0,30` e `m.fTxt <= 0,10`.
- `contarLingua` só roda quando `m.valida` e `m.fImg >= 0,10`; fora disso a contagem é zero.
- Com o OCR desligado, as regras novas não mudam nada: `precisaOcr` e `semOcr` ficam falsos e o texto nativo fica. As regras 1 e 2 seguem como hoje.
- Quando a regra nova manda ao OCR, a reserva (`an.texto`) é o texto nativo, e `ParsePages` passa `motivo: an.motivo` para `d.agendar`.
- Log: quando `m.valida` e (`m.fImg > 0` ou `m.trechosForm > 0`), uma linha `linhaDiagnostico`, neste formato exato:

```
[PDFParser] Estrutura da página 3: nativo=1532 fImg=0.96 fTxt=0.02 fDesc=0.95 trechosForm=0 palavras=0 implausiveis=0 funcionais=0 decisao=ocr_imagem_descoberta
```

  Decisões: `nativo`, `ocr_ilegivel`, `ocr_curto`, `ocr_imagem_descoberta`, `ocr_isca`, `ocr_desligado` (alguma regra mandaria ao OCR, e ele está desligado).

- [ ] **Passo 1: escrever os testes que falham** em `pdf_regras_test.go`

`TestRegraNova`, tabela sobre a função pura (`isca` abaixo é `contagemLingua{palavras: 80, implausiveis: 66, funcionais: 0}`; `normal` é `{palavras: 200, implausiveis: 0, funcionais: 60}`):

| métricas | contagem | esperado |
|---|---|---|
| `{valida, fImg 0.96, fTxt 0.02, fDesc 0.95}` | normal | `(motivoImagemDescoberta, true)` |
| `{valida, fImg 0.40, fTxt 0.10, fDesc 0.30}` | normal | `(motivoImagemDescoberta, true)` |
| `{valida, fImg 0.40, fTxt 0.10, fDesc 0.29}` | normal | `(_, false)` |
| `{valida, fImg 0.40, fTxt 0.11, fDesc 0.35}` | normal | `(_, false)` |
| `{valida, fImg 0.10, fTxt 0.50, fDesc 0.02}` | isca | `(motivoIsca, true)` |
| `{valida, fImg 0.09, fTxt 0.50, fDesc 0.02}` | isca | `(_, false)` |
| `{valida, fImg 0.90, fTxt 0.05, fDesc 0.80}` | isca | `(motivoIsca, true)` (as duas regras valem; vale a isca) |
| `{valida: false, fImg 0.96, fTxt 0.02, fDesc 0.95}` | isca | `(_, false)` |
| `{valida, fImg 1.00, fTxt 0.55, fDesc 0.39}` | normal | `(_, false)` |

`TestAnalisarPagina_Sinteticas`: para cada página de `paginasSinteticas()`, com `analisarPagina(primeiraPagina(t, p.objetos), 1, true)`:
- `p.isca`: `precisaOcr` e `motivo == motivoIsca`;
- `p.candidataImagem` e não isca: `precisaOcr` e `motivo == motivoImagemDescoberta`;
- as demais (inclusive `comprovante-codigos`): `!precisaOcr`;
- em todas, `!semOcr` e `an.texto` igual ao texto de `ExtractNativeText` sem espaços nas pontas.

`TestAnalisarPagina_SinteticasComOCRDesligado`: para as páginas `isca` ou `candidataImagem`, com `ocrEnabled = false`: `!precisaOcr`, `!semOcr`, texto nativo igual.

`TestLinhaDiagnostico_SoNumeros`: a linha de uma chamada com valores conhecidos é exatamente a do formato acima; e a linha casa com `^\[PDFParser\] Estrutura da página \d+: ( ?[a-zA-Z]+=[0-9a-z_.]+)+$`.

- [ ] **Passo 2: rodar e ver falhar** (`go test ./internal/etl -run 'TestRegraNova|TestAnalisarPagina_|TestLinhaDiagnostico' -v`)
- [ ] **Passo 3: implementar** `pdf_regras.go` e as mudanças em `pdf_parser.go`
- [ ] **Passo 4: rodar e ver passar**, depois o gate. `TestPipeline_SemOCR_TextoNativoCurtoComImagemContaComoLido` e os demais testes existentes passam sem mudança.
- [ ] **Passo 5: commit**

`git add internal/etl/pdf_regras.go internal/etl/pdf_regras_test.go internal/etl/pdf_parser.go`; mensagem `feat(etl): página de imagem descoberta ou texto-isca vai ao OCR`.

---

### Task 6: Páginas sintéticas de ponta a ponta

**Arquivos:**
- Criar: `internal/etl/pdf_conteudo_perdido_test.go`
- Modificar código de produção só se um teste mostrar defeito; nesse caso, descreva no relatório.

**Interfaces:**
- Consome: `paginasSinteticas`, `paginaSinteticaDe`, `parsePagesWithDeadline`, `parsearSemOCR`, `buildPDF`, `extrairComPrazo`, `classificarPaginas`, o PDF de `TestForm_CorpoEmFormMaisLegenda`.
- Produz: nada para outras tarefas.

O OCR falso devolve `"TEXTO LIDO PELO OCR linha um\nTEXTO LIDO PELO OCR linha dois"` e conta as chamadas.

- [ ] **Passo 1: escrever os testes**

| teste | páginas | afirma |
|---|---|---|
| `TestConteudoPerdido_ControlesNaoVaoAoOCR` | as que não são `isca` nem `candidataImagem` | o OCR falso não é chamado; o conteúdo da página é o texto de `extrairComPrazo`; `ocr == false` |
| `TestConteudoPerdido_ImagemDescobertaJuntaOCRENativo` | `candidataImagem` e não `isca` | 1 chamada; o conteúdo começa com o texto do OCR; cada linha nativa aparece depois dele; `ocr == true` |
| `TestConteudoPerdido_TimbreTresLinhasMantemONativo` | `timbre-3-linhas` | 1 chamada (falso positivo documentado); as 3 linhas nativas estão no conteúdo |
| `TestConteudoPerdido_IscaTrocaPeloOCR` | as três iscas | 1 chamada; o conteúdo é exatamente o texto do OCR; nenhuma das palavras aleatórias aparece |
| `TestConteudoPerdido_OCREmBrancoMantemONativo` | `isca` ou `candidataImagem`, com OCR falso que devolve `"\n\f"` | conteúdo igual ao texto nativo; `ocr == false`; sem `ocr_erro` |
| `TestConteudoPerdido_OCRComErroMantemONativo` | as mesmas, com OCR falso que devolve erro | conteúdo igual ao texto nativo; `ocr_erro` preenchido |
| `TestConteudoPerdido_OCRDesligado` | as mesmas, por `parsearSemOCR(t, buildPDF(p.objetos), false)` | conteúdo igual ao texto nativo; o conteúdo não tem `[página digitalizada`; `classificarPaginas` devolve `LeituraLido` |
| `TestConteudoPerdido_CorpoEmFormSemOCR` | corpo em Form mais legenda | o OCR falso não é chamado; as 3 linhas do corpo e a legenda estão no conteúdo |
| `TestConteudoPerdido_MosaicoLegendaCurta` | `mosaico-legenda-curta` | 1 chamada; o conteúdo tem o texto do OCR e a legenda |

- [ ] **Passo 2: rodar** (`go test ./internal/etl -run 'TestConteudoPerdido_' -v`). Como as tarefas anteriores já entregaram o comportamento, o esperado é passar. Para a evidência de TDD, quebre uma regra de propósito (por exemplo `descobertaMinFDesc = 2`), veja `TestConteudoPerdido_ImagemDescobertaJuntaOCRENativo` falhar, guarde a linha e desfaça. Um teste que falhe sem essa quebra é um defeito: investigue a causa antes de mexer no teste.
- [ ] **Passo 3: gate** (`go vet ./...`, `go test -race ./internal/...`)
- [ ] **Passo 4: commit**

`git add internal/etl/pdf_conteudo_perdido_test.go` (e o que mais tiver mudado, por caminho); mensagem `test(etl): páginas sintéticas de conteúdo perdido de ponta a ponta`.

---

### Task 7: Medição, documentação e gate (controlador)

Esta tarefa é da sessão principal. Não é despachada: lê os ZIPs reais do dono.

**Arquivos:**
- Modificar: `CLAUDE.md`, `PRD.md` (RF06), `docs/revisao/2026-11-14-residuais-pdfs-nao-lidos.md` (M2 resolvido), a spec (tabela de limiares, se a medição mandar ajustar)

- [ ] **Passo 0 (antes da Task 1):** com a máquina sem outros agentes, `go test ./scripts/bench -run '^$' -bench 'ParsePDF|Sintetico' -count=10 -benchmem`, guardado em `.superpowers/sdd/2026-11-15-conteudo-perdido/bench-antes.txt`.
- [ ] **Passo 1: desempenho.** O mesmo comando depois da Task 6, em `bench-depois.txt`, e `go tool benchstat bench-antes.txt bench-depois.txt`. Critério: `ParsePDF/nativo`, `ParsePDF/contents-array` e `Sintetico` não ficam mais de 5% piores (com p < 0,05). Se ficarem, uma rodada de conserto com o número na mão, antes de seguir.
- [ ] **Passo 2: medição real.** Com o harness de medição da v0.4.1 (scratchpad), sobre cópias dos três ZIPs do dono, com `TEMP`/`TMP` redirecionados: a `main` e a branch, OCR ligado. Saem só contagens: blocos `<documento>` iguais aos da v0.4.1; páginas por decisão (das linhas `Estrutura da página`); páginas com `trechosForm > 0`; distribuição de `fDesc`, com a faixa de 0,15 a 0,30 à parte; tempo total antes e depois.
- [ ] **Passo 3: limiares.** Se a medição mostrar um limiar mal calibrado, ajustar a constante, o teste e a tabela da spec, e medir de novo.
- [ ] **Passo 4: documentação.** `CLAUDE.md`: o passo 3 da pipeline (as regras de OCR), o Form XObject, a junção, o log de números e as armadilhas novas. `PRD.md` RF06: quando o OCR é aplicado. Residuais: M2 resolvido, com o commit.
- [ ] **Passo 5: gate.** `pwsh scripts/build/gate.ps1`.
- [ ] **Passo 6: commit** `docs: páginas de conteúdo perdido (CLAUDE.md, PRD RF06, residuais)`.
- [ ] **Passo 7: revisão final** da branch inteira por um revisor novo, no modelo mais capaz; uma rodada de conserto; depois perguntar ao dono, no Pinboard, como integrar. Push, PR, `productVersion` 0.4.2 e release (skill `eprocia-release`) só quando o dono mandar.
