# Residuais do conserto dos PDFs não lidos (v0.4.1)

Itens menores que as revisões do conserto (PR #8) apontaram e que ficaram de fora da v0.4.1 por decisão registrada. Nenhum deixa um documento fora do texto sem aviso. Entram na próxima rodada, junto dos achados do `2026-11-10-relatorio.md`.

Os ids são os do registro de execução do plano (`docs/superpowers/plans/2026-11-11-pdfs-nao-lidos.md`): `M` vem da revisão final da branch, `P` das revisões por tarefa. Os arquivos citados são os do commit `b10fa0f`; confira o trecho antes de mexer, porque a `main` pode ter andado.

## Comportamento

| Id | Onde | O que é | Sugestão |
|---|---|---|---|
| M1 | `internal/etl/pdf_parser.go` (marcador de página), `leitura.go`, `pdf_poppler.go` | Página digitalizada com um pouco de texto nativo (carimbo, linha de assinatura) e OCR desligado sai só com esse texto, conta como lida e não leva marcador. Está fixado por `TestPipeline_SemOCR_TextoNativoCurtoComImagemContaComoLido`: é decisão, não acidente. No caminho do Poppler a página curta nem recebe a marca `sem_ocr`. | Acrescentar o marcador depois do texto curto, em vez de só trocar o texto vazio. Muda a aparência de um documento lido: decisão do dono. |
| M2 | `internal/etl/pdf_ocr_despacho.go` (sucesso do OCR) | Quando o OCR devolve texto em branco, o texto nativo curto que servia de reserva é descartado. No caminho do Poppler toda página com menos de 100 bytes vai ao OCR, então o trecho roda mais do que antes. | Manter a reserva quando `strings.TrimSpace(ocrText) == ""`. |
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
