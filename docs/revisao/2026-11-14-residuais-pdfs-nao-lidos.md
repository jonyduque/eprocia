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
| P6 | `ProcessList.svelte` (`.details-row`) | `overflow: hidden` no pai poderia cortar o rótulo "N não lidos" numa linha bem mais estreita que 800 px. Não reproduz em 800×600. | Conferir com captura abaixo de 800 px. |
| P10 | `internal/etl/pdf_parser.go` (`registrarOCR`, `runOCRBloco`) | `DeadlineExceeded` vindo do contexto de quem chama não grava `ocr_prazo` nem conta para a parada depois de 2 prazos. Sem efeito hoje: a produção só usa `WithCancel`. | Tratar junto se um prazo por ZIP for criado. |
| P14 | `internal/ocr/poppler.go`, `tesseract.go` | `initPoppler` e `tessdataDoModo` seguram `initMu` durante a extração (só latência na primeira chamada); a falha é registrada duas vezes no log; a regex `Pages:\s+` casa quebra de linha; a saída do `pdftotext` não tem limite de memória. | Limite de leitura na saída e `Pages:[ \t]+`. |

## Notas sem ação pedida (M7)

- `leitura.go` e `formatter.go` reconhecem um erro pelo prefixo `[ERRO` do texto. Um documento real cuja primeira página começasse com esse texto seria classificado como ilegível. Uma marca no mapa da página tiraria o acoplamento.
- `exec.CommandContext` mata o processo direto quando o prazo estoura; não foi confirmado se `pdftoppm` ou `tesseract` poderiam deixar um filho para trás.
- Depois de `manterSoDisco`, `readerAt()` embrulha `dados` nulo; inofensivo, porque o caminho do Poppler não lê por ali.

## Testes e limpeza de código

| Id | Onde | O que é |
|---|---|---|
| P2 | `pipeline_leitura_test.go` | O teste das entradas duplicadas deveria conferir `docs[0].Leitura == "lido"` e o `Estado`. |
| P3 | `pipeline.go`, frontend | Documento `sem_ocr` termina com `Estado=erro` e `Erro=frase`. Quem avisa deve usar `Leitura`, não só `Estado == erro`. Conferir os consumidores. |
| P4 | `TestSaida_PaginaSemOCRForaDasRegras` | A asserção (b) no nível `aggressive` depende de como `refluir` junta as linhas. |
| P5 | `queue_nao_lidos_test.go` | `esperarStatus` tem nome genérico no pacote `etl`. |
| P7 | `ProcessList.svelte` | Os mapas por id (`geracaoReprocessar`, `ocrSugerido`, `reprocessarAberto`) guardam entradas de itens removidos. Inofensivo. |
| P8 | testes do frontend | O `{#key}` do `ReprocessarItem` só é conferido por regex. |
| P9 | `pipeline.go` | `executarOCR` e `executarOCRPaginas` são variáveis de pacote para injeção em teste; campos no `Pipeline` seriam mais limpos. |
| P11 | `internal/ocr` | Falta teste de que a reserva em inglês do tesseract ainda roda numa falha que não é prazo. |
| P12 | `pdf_parser.go` (blocos de OCR) | O `min(..., maxPaginasBlocoOCR)` na sobra nunca age; falta teste de resto pequeno (41 páginas: soma 41, máximo 20). |
| P13 | `pdf_ocr_despacho.go`, `pdf_parser.go` | `esperar()` não zera `d.pendentes` (uma segunda chamada despacharia de novo; hoje há uma só), e `aguardar`/`esperar` têm nomes quase iguais com efeitos diferentes. |
| P14 | `internal/ocr` | Sem teste unitário de `ErrSenha` nem de cancelamento diferente de `ErrPrazo` em `rodarPoppler` (o `pdf_classes_test.go` cobre `ErrSenha` com o binário real). |
| P16 | `pdf_classes_test.go` | A pasta fixa `%TEMP%\eprocia_testes_classes` fica na máquina (dezenas de MB, reaproveitada entre execuções): limpar no `TestMain`. Separar os geradores num arquivo próprio e usar tabela nos casos. |

## Residuais da v0.4.2 (páginas de conteúdo perdido)

Itens que as revisões da v0.4.2 (spec `docs/superpowers/specs/2026-11-15-conteudo-perdido-design.md`) apontaram e que ficaram de fora por decisão registrada. Nenhum deixa a página pior do que na v0.4.1. Os ids `CP` são desta seção.

| Id | Onde | O que é | Sugestão |
|---|---|---|---|
| CP1 | `internal/etl/pdf_texto.go` (limite de Forms), `pdf_parser.go` (`checkContentStream`), `pdf_estrutura.go` (malha de `medirPagina`) | O limite conta Forms, não bytes: uma página pode mandar ler 256 Forms de até 64 MiB cada, sem prazo. A mesma exposição já existia na v0.4.1 para um `/Contents` em array. | Teto de bytes por página, ou prazo para a leitura do texto nativo. |
| CP2 | `internal/etl/lingua.go` (`iscaMinImplausiveis`) | Texto-isca feito só de letras minúsculas aleatórias fica com 35% a 43% de palavras implausíveis, perto do limiar de 35%: cerca de metade das páginas assim passa. O falso negativo deixa a página como na v0.4.1. Na medição de 2026-11-16 não havia página-isca entre 733 páginas reais, e páginas reais chegam a 25% a 35%. | Só mexer no limiar com uma página-isca real para medir. |
| CP3 | `internal/etl/pdf_juncao.go` (`linhaNativaPresente`) | A linha nativa conta como presente quando 60% das suas palavras de 3 ou mais caracteres aparecem em qualquer lugar do texto do OCR. Uma linha curta de palavras comuns pode contar como presente e o pedaço que o OCR não trouxe (um valor, uma data) se perde. Medido nos ZIPs reais: 1 de 27 e 2 de 3492 palavras. Segue a spec. | Exigir a presença na linha-âncora, ou tratar como ausente a linha com menos de 3 palavras contáveis. Decisão do dono. |
| CP4 | `internal/etl/pdf_estrutura.go` (`topoImagens`) | Conta imagem útil mesmo fora do MediaBox, que `medirPagina` recorta. Só muda a ordem do encaixe na junção. | Usar só as imagens que cruzam o MediaBox. |
| CP5 | `internal/etl/pdf_juncao.go` (`proximaAncora`, normalização) | `proximaAncora` é quadrática (1,36 s com 50 mil linhas ausentes); uma linha com acento decomposto (NFD) conta como ausente e sai duas vezes (duplica, não perde). | Índice da próxima âncora; normalizar para NFC antes de comparar. |
| CP9 | `internal/etl/pdf_parser.go` (`HasImages`) | Não tem `recover` próprio. Numa página de texto curto com um XObject cuja tabela de objetos está inconsistente, o pânico cai no `recover` de fora e a página sai como erro de leitura, como na v0.4.1. A página com texto suficiente não chama `HasImages` e sai lida. | `recover` dentro de `HasImages`, tratando o pânico como página sem imagens. |

Testes:

| Id | Onde | O que é |
|---|---|---|
| CP6 | `pdf_juncao_test.go` | O teste de propriedade usa a `linhaNativaPresente` da própria implementação e não vê linha duplicada nem o caso do CP3. |
| CP7 | `pdf_juncao.go`, `leitura.go` | Reserva igual ao marcador de ilegível com OCR em branco faz o documento passar de `sem_texto` para `ilegivel`; falta o teste. |
| CP8 | `pdf_conteudo_perdido_test.go` | Os filtros sobre `paginasSinteticas()` não têm guarda de quantidade mínima (um filtro vazio passaria sem testar nada); o laço das palavras-isca é redundante. |

Decisões registradas, sem ação pedida:

- A página sintética `mosaico-legenda-curta` (legenda curta sob imagens) fica na regra antiga de texto curto, e o OCR substitui a legenda, como na v0.4.1. Fixado por `TestConteudoPerdido_MosaicoLegendaCurta`.
- `ParsePDF/escaneado` ficou 23% mais lento (304 para 375 microssegundos, 456 para 503 alocações): é a medição de estrutura numa página que depois vai ao OCR, que leva segundos. Os outros benchmarks não mudaram (comparação intercalada de 25 rodadas).
- A falha do OCR numa página que só foi ao OCR pelas regras novas não grava `ocr_erro` (ver a spec, seção da junção): o único rastro é a linha de log da falha.
