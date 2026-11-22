# Residuais do conserto dos PDFs não lidos (v0.4.1)

Itens menores que as revisões do conserto (PR #8) apontaram e que ficaram de fora da v0.4.1 por decisão registrada. Nenhum deixa um documento fora do texto sem aviso. Entram na próxima rodada, junto dos achados do `2026-11-10-relatorio.md`.

Os ids são os do registro de execução do plano (`docs/superpowers/plans/2026-11-11-pdfs-nao-lidos.md`): `M` vem da revisão final da branch, `P` das revisões por tarefa. Os arquivos citados são os do commit `b10fa0f`; confira o trecho antes de mexer, porque a `main` pode ter andado.

## Comportamento

| Id | Onde | O que é | Sugestão |
|---|---|---|---|
| M1 | `internal/etl/pdf_parser.go` (marcador de página), `leitura.go`, `pdf_poppler.go` | Página digitalizada com um pouco de texto nativo (carimbo, linha de assinatura) e OCR desligado sai só com esse texto, conta como lida e não leva marcador. Está fixado por `TestPipeline_SemOCR_TextoNativoCurtoComImagemContaComoLido`: é decisão, não acidente. No caminho do Poppler a página curta nem recebe a marca `sem_ocr`. | Acrescentar o marcador depois do texto curto, em vez de só trocar o texto vazio. Muda a aparência de um documento lido: decisão do dono. |
| M2 (resolvido na v0.4.2, commit `66a1c67`) | `internal/etl/pdf_ocr_despacho.go` (sucesso do OCR) | Quando o OCR devolve texto em branco, o texto nativo curto que servia de reserva é descartado. No caminho do Poppler toda página com menos de 100 bytes vai ao OCR, então o trecho roda mais do que antes. | Feito: `conteudoAposOCR` (`internal/etl/pdf_juncao.go`) mantém a reserva quando o OCR vem em branco ou ilegível. |
| M5 | `frontend/src/lib/atividade.js`, `AtividadeItem.svelte` | Documento com só algumas páginas sem OCR conta como lido; o painel não mostra nada e o botão "Reprocessar com OCR" não aparece. O único sinal é o marcador dentro do `.txt`. | Levar a contagem de páginas `sem_ocr` no `AtividadeDoc`. |
| P18 | `internal/etl/queue.go` (`QueueItem.Error`/`Message`), `app.go` (notificação de falha) | Quando o ZIP inteiro falha, a linha do item e a notificação mostram o erro bruto, que pode trazer o caminho do ZIP ou do `.txt`. Nenhum `.txt` é gerado nesse caso. | Encurtar o caminho para o nome do arquivo. |
| P6 (medido em 2026-11-22, 2243370: o rótulo some em 800×600 com o divisor no máximo; o conserto espera decisão do dono) | `ProcessList.svelte` (`.details-row`) | `overflow: hidden` no pai poderia cortar o rótulo "N não lidos" numa linha bem mais estreita que 800 px. Não reproduz em 800×600. | Conferir com captura abaixo de 800 px. |
| P10 | `internal/etl/pdf_parser.go` (`registrarOCR`, `runOCRBloco`) | `DeadlineExceeded` vindo do contexto de quem chama não grava `ocr_prazo` nem conta para a parada depois de 2 prazos. Sem efeito hoje: a produção só usa `WithCancel`. | Tratar junto se um prazo por ZIP for criado. |
| P14 (resolvido em 2026-11-22, 23083f7, 74f40cf e 90c2326; `initMu` e o log duplo ficam sem ação) | `internal/ocr/poppler.go`, `tesseract.go` | `initPoppler` e `tessdataDoModo` seguram `initMu` durante a extração (só latência na primeira chamada); a falha é registrada duas vezes no log; a regex `Pages:\s+` casa quebra de linha; a saída do `pdftotext` não tem limite de memória. | Limite de leitura na saída e `Pages:[ \t]+`. |

## Notas sem ação pedida (M7)

- `leitura.go` e `formatter.go` reconhecem um erro pelo prefixo `[ERRO` do texto. Um documento real cuja primeira página começasse com esse texto seria classificado como ilegível. Uma marca no mapa da página tiraria o acoplamento.
- `exec.CommandContext` mata o processo direto quando o prazo estoura; não foi confirmado se `pdftoppm` ou `tesseract` poderiam deixar um filho para trás.
- Depois de `manterSoDisco`, `readerAt()` embrulha `dados` nulo; inofensivo, porque o caminho do Poppler não lê por ali.

## Testes e limpeza de código

| Id | Onde | O que é |
|---|---|---|
| P2 (resolvido em 2026-11-22, 1d63aed) | `pipeline_leitura_test.go` | O teste das entradas duplicadas deveria conferir `docs[0].Leitura == "lido"` e o `Estado`. |
| P3 | `pipeline.go`, frontend | Documento `sem_ocr` termina com `Estado=erro` e `Erro=frase`. Quem avisa deve usar `Leitura`, não só `Estado == erro`. Conferir os consumidores. |
| P4 (resolvido em 2026-11-22, 1d63aed) | `TestSaida_PaginaSemOCRForaDasRegras` | A asserção (b) no nível `aggressive` depende de como `refluir` junta as linhas. |
| P5 (resolvido em 2026-11-22, 1d63aed) | `queue_nao_lidos_test.go` | `esperarStatus` tem nome genérico no pacote `etl`. |
| P7 | `ProcessList.svelte` | Os mapas por id (`geracaoReprocessar`, `ocrSugerido`, `reprocessarAberto`) guardam entradas de itens removidos. Inofensivo. |
| P8 | testes do frontend | O `{#key}` do `ReprocessarItem` só é conferido por regex. |
| P9 | `pipeline.go` | `executarOCR` e `executarOCRPaginas` são variáveis de pacote para injeção em teste; campos no `Pipeline` seriam mais limpos. |
| P11 | `internal/ocr` | Falta teste de que a reserva em inglês do tesseract ainda roda numa falha que não é prazo. |
| P12 (resolvido em 2026-11-22, 1d63aed) | `pdf_parser.go` (blocos de OCR) | O `min(..., maxPaginasBlocoOCR)` na sobra nunca age; falta teste de resto pequeno (41 páginas: soma 41, máximo 20). |
| P13 (resolvido em 2026-11-22, 7d4b40a) | `pdf_ocr_despacho.go`, `pdf_parser.go` | `esperar()` não zera `d.pendentes` (uma segunda chamada despacharia de novo; hoje há uma só), e `aguardar`/`esperar` têm nomes quase iguais com efeitos diferentes. |
| P14 (resolvido em 2026-11-22, 23083f7, 74f40cf e 90c2326) | `internal/ocr` | Sem teste unitário de `ErrSenha` nem de cancelamento diferente de `ErrPrazo` em `rodarPoppler` (o `pdf_classes_test.go` cobre `ErrSenha` com o binário real). |
| P16 (resolvido em 2026-11-22, 316c9e6) | `pdf_classes_test.go` | A pasta fixa `%TEMP%\eprocia_testes_classes` fica na máquina (dezenas de MB, reaproveitada entre execuções): limpar no `TestMain`. Separar os geradores num arquivo próprio e usar tabela nos casos. |

## Residuais da v0.4.2 (páginas de conteúdo perdido)

Itens que as revisões da v0.4.2 (spec `docs/superpowers/specs/2026-11-15-conteudo-perdido-design.md`) apontaram e que ficaram de fora por decisão registrada. Nenhum deixa a página pior do que na v0.4.1. Os ids `CP` são desta seção.

| Id | Onde | O que é | Sugestão |
|---|---|---|---|
| CP1 (resolvido na v0.4.2) | `internal/etl/pdf_texto.go` (limite de Forms), `pdf_parser.go` (`checkContentStream`), `pdf_estrutura.go` (malha de `medirPagina`) | O limite contava Forms, não bytes: uma página podia mandar ler 256 Forms de até 64 MiB cada, sem prazo, e um `/Contents` em array somava streams de até 64 MiB sem teto (a mesma exposição já existia na v0.4.1). Apontado de novo pela revisão de segurança automática do push do PR #11; o dono mandou corrigir antes da v0.4.2. | Feito: teto de 64 MiB descomprimidos para a soma do `/Contents` da página e outro de 64 MiB para a soma dos Forms lidos nela (cada chamada conta, legível ou não), mais 200 mil trechos de texto por página. A malha de `medirPagina` segue sem prazo próprio. |
| CP2 | `internal/etl/lingua.go` (`iscaMinImplausiveis`) | Texto-isca feito só de letras minúsculas aleatórias fica com 35% a 43% de palavras implausíveis, perto do limiar de 35%: cerca de metade das páginas assim passa. O falso negativo deixa a página como na v0.4.1. Na medição de 2026-11-16 não havia página-isca entre 733 páginas reais, e páginas reais chegam a 25% a 35%. | Só mexer no limiar com uma página-isca real para medir. |
| CP3 (resolvido na v0.4.2) | `internal/etl/pdf_juncao.go` (`linhaNativaPresente`) | A linha nativa contava como presente quando 60% das suas palavras de 3 ou mais caracteres apareciam em qualquer lugar do texto do OCR, e o pedaço que o OCR não trouxe (um valor, uma data) se perdia. Medido em 2026-11-17 em 9 ZIPs reais (116 páginas na junção, 4.971 palavras nativas): 182 palavras perdidas, 33 com dígito. Por decisão do dono, a linha só conta como presente com todas as palavras no OCR: 0 perdidas, 4,7% de texto a mais nessas páginas (linhas que repetem em parte o OCR). | Feito. As outras regras medidas perdiam mais: linha curta sempre inserida, 180 a 182; presença só na linha-âncora do OCR, 49. |
| CP4 (resolvido em 2026-11-22) | `internal/etl/pdf_estrutura.go` (`topoImagens`) | Conta imagem útil mesmo fora do MediaBox, que `medirPagina` recorta. Só muda a ordem do encaixe na junção. | Feito: `topoImagens` usa o mesmo `cruzaMediaBox` de `medirPagina`. |
| CP5 (resolvido em 2026-11-22) | `internal/etl/pdf_juncao.go` (`proximaAncora`, normalização) | `proximaAncora` é quadrática (1,36 s com 50 mil linhas ausentes); uma linha com acento decomposto (NFD) conta como ausente e sai duas vezes (duplica, não perde). | Feito: vetor da próxima linha presente, numa passada; comparação em NFC, texto original na saída. |
| CP9 | `internal/etl/pdf_parser.go` (`HasImages`) | Não tem `recover` próprio. Numa página de texto curto com um XObject cuja tabela de objetos está inconsistente, o pânico cai no `recover` de fora e a página sai como erro de leitura, como na v0.4.1. A página com texto suficiente não chama `HasImages` e sai lida. | `recover` dentro de `HasImages`, tratando o pânico como página sem imagens. |
| CP10 (resolvido em 2026-11-22) | `internal/etl/pdf_juncao.go` (`juntarOCRComNativo`) | A junção decide linha a linha pelo conjunto de palavras do OCR, sem alinhar as duas sequências. Com a regra de 100% (CP3) nenhuma palavra nativa se perde, mas a linha nativa entra inteira mesmo quando o OCR já trouxe parte dela: na medição de 2026-11-17 em 9 ZIPs reais, 76 linhas assim e 4,7% de texto a mais nas 116 páginas da junção. | Medido em 2026-11-22 (117 páginas, 9 ZIPs): R100 perdia, contadas por ocorrência, 18 palavras nativas (14 com dígito) e 32 fichas com dígito (o 0 do CP3 era contado por conjunto: a palavra aparecer ao menos uma vez na saída); R100m (presença por contagem com consumo em ordem, fichas com dígito de qualquer tamanho) perde 0, com 411 linhas inseridas contra 402, 1.287 palavras repetidas contra 1.221 e +538 bytes (20 páginas mudam). RA perdia 3.530 palavras do OCR sem equivalente nativo, RB 879; RBc não perde, corta 4,7% dos bytes, mas tira 379 fichas com dígito do OCR, 256 delas com dígito diferente do nativo, onde o nativo prevalece, e pede um segundo tesseract por página. Adotada a R100m; RBc fica para decisão do dono. A comparação em NFC (CP5) entrou depois e não foi medida: os números acima são só da R100m. |

Testes:

| Id | Onde | O que é |
|---|---|---|
| CP6 (resolvido em 2026-11-22, 488f889: oráculo próprio, `fichasOraculo`) | `pdf_juncao_test.go` | O teste de propriedade usa a `linhaNativaPresente` da própria implementação e não vê linha duplicada nem o caso do CP3. |
| CP7 (resolvido em 2026-11-22, 1d63aed) | `pdf_juncao.go`, `leitura.go` | Reserva igual ao marcador de ilegível com OCR em branco faz o documento passar de `sem_texto` para `ilegivel`; falta o teste. |
| CP8 (resolvido em 2026-11-22, 39d3e64) | `pdf_conteudo_perdido_test.go` | Os filtros sobre `paginasSinteticas()` não têm guarda de quantidade mínima (um filtro vazio passaria sem testar nada); o laço das palavras-isca é redundante. |

Decisões registradas, sem ação pedida:

- A página sintética `mosaico-legenda-curta` (legenda curta sob imagens) fica na regra antiga de texto curto, e o OCR substitui a legenda, como na v0.4.1. Fixado por `TestConteudoPerdido_MosaicoLegendaCurta`.
- `ParsePDF/escaneado` ficou 23% mais lento (304 para 375 microssegundos, 456 para 503 alocações): é a medição de estrutura numa página que depois vai ao OCR, que leva segundos. Os outros benchmarks não mudaram (comparação intercalada de 25 rodadas).
- A falha do OCR numa página que só foi ao OCR pelas regras novas não grava `ocr_erro` (ver a spec, seção da junção): a página conta como lida, leva `ocr_aviso` e sai no `.txt` com a linha de aviso de falha do OCR antes do texto nativo (pedido do dono em 2026-11-16).
- O pedido de aviso ao usuário só existe para a falha do OCR. Os outros motivos de documento não lido (senha, arquivo ilegível, OCR desligado, nenhum texto) seguem com `consulte o PDF original`, sem pedir ao modelo que avise o usuário.

## Residuais da revisão da branch revisao/2026-11-10 (2026-11-19)

Três achados baixos da revisão final da branch, deixados fora do PR. Os ids `RB` são desta seção. (O quarto achado da revisão, o anúncio de "arquivo adicionado" para um ZIP já na fila, foi corrigido na própria branch.)

| Id | Onde | Cenário | Conserto proposto |
|---|---|---|---|
| RB1 (resolvido em 2026-11-22, f3a1e68 e b67a7a7) | `frontend/src/components/ProcessList.svelte:96-102` (mesmo padrão em `App.svelte:273-278` e `Settings.svelte:130-134`) | `anunciar()` não é serializado: duas chamadas no mesmo ciclo zeram, esperam `tick()` e escrevem em sequência, e só a última vai ao DOM. Com o filtro ativo, chega um ZIP que corresponde ao filtro no mesmo `queue:update` em que outro item conclui: `registrarMarcos` escreve "X: concluído, N documentos" e `registrarFiltro` escreve "2 itens correspondem ao filtro" logo depois, e o leitor de tela só ouve o segundo. O mesmo vale para Ctrl+C durante a conclusão de um item. | Juntar as mensagens do mesmo tick numa fila (concatenar com ". " antes de escrever). |
| RB2 (resolvido em 2026-11-22, 8df01a1) | `frontend/src/lib/atividade.js:36-40` e `:49` | `ordenarDocs` ficou sem uso em produção depois do F-DF-04 (o `AtividadeItem` usa `prepararDocs`); só os testes o usam, como oráculo de ordem. `resumoAtividade` (usado por `anuncios.js`) repete o laço de `prepararDocs`, e as duas contagens podem divergir numa mudança futura. | Mover `ordenarDocs` para o teste, ou documentar que ele é o oráculo; fazer `prepararDocs` e `marcosFila` usarem um contador comum. |
| RB3 (resolvido em 2026-11-22, dd363ed) | `.claude/review.json:105` | A chave `alto_contraste` não é lida por nenhum script: `scripts/visual/capturar.mjs` repete a lista no código (`ALTO_CONTRASTE`). Quem acrescentar um estado em `review.json`, como o arquivo de configuração sugere, não ganha captura nenhuma. | `capturar.mjs` ler a lista do `review.json`, ou a chave sair. |
