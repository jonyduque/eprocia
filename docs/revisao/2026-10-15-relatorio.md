# Revisão do eprocIA — 2026-10-15

## Escopo

A revisão cobriu só o backend (Go), com a lente TE (testes). Seis achados passaram pela refutação cética: quatro foram mantidos e dois foram rebaixados. Nenhum foi derrubado. Também não houve duplicatas, já que só uma lente rodou.

A ordem segue gravidade × ganho ÷ custo, com alta=3, média=2, baixa=1 e P=1, M=2, G=3. Os empates foram desfeitos pelo ganho esperado. B-TE-01 e B-TE-02 têm nota 2,0, e B-TE-01 vem antes porque um config.json corrompido inutiliza a fila inteira. B-TE-04 e B-TE-05 têm nota 1,0, e B-TE-04 vem antes porque, além da cobertura, corrige o furo de progresso e protege contra o vazamento de temporários.

## Resumo

| id | título | gravidade | custo | ganho | veredito |
|---|---|---|---|---|---|
| B-TE-01 | O erro de LoadFromFile com config corrompida não tem teste, e esse erro faz a fila falhar em silêncio | média | P | Um config.json corrompido deixa de inutilizar a fila; o contrato de mesclar com os defaults passa a ser verificado | mantido |
| B-TE-02 | ParsePages: falha do OCR (fallback para texto nativo) e cancelamento no meio do PDF sem teste | média | P | ParsePages chega a cerca de 90%; o fallback de OCR e o cancelamento passam a ser verificados | mantido |
| B-TE-03 | A fila (Queue) não tem teste: cancelamento, pausa, deduplicação e recuperação de panic nunca são exercitados | alta | M | queue.go passa de 0% para cerca de 80%; uma regressão no cancelamento ou no autoencadeamento quebra o teste | mantido |
| B-TE-04 | Pipeline.ProcessZip não tem teste unitário; os caminhos de erro e de cancelamento nunca rodam (e há um defeito de progresso escondido) | média | M | ProcessZip ganha cobertura de unidade; o furo de progresso fecha e o vazamento de temporários fica coberto | rebaixado |
| B-TE-05 | Watchdog: os erros de Start e a troca de pasta não têm teste | baixa | P | Os caminhos de erro da pasta monitorada, que vêm da tela de Settings, passam a ter teste | mantido |
| B-TE-06 | OCR: binário ausente, falha do pdftoppm e fallback do Tesseract não têm teste; o único teste é pulado quando a extração falha | baixa | M | O caminho "OCR indisponível" ganha um teste determinístico; a suíte deixa de passar verde quando a extração quebra | rebaixado |

## Achados

### B-TE-01 — O erro de LoadFromFile com config corrompida não tem teste, e esse erro faz a fila falhar em silêncio

- Arquivo: `internal/config/config.go:79`
- Lente: TE

**Evidência.** Quando o JSON é inválido, config.go:79-80 devolve erro. Nenhum teste em config_test.go grava um JSON malformado: os testes atuais cobrem só arquivo ausente, JSON parcial e strings vazias. A cobertura de LoadFromFile é 86,4%, e a de Load, Save e GetConfigPath é 0,0%. Fora dos testes, o efeito é este: app.go:47-55 ignora o erro (`if err == nil`), e q.cfg fica nil. Depois, a fila chama config.Load() de novo para cada ZIP e marca todos com "Erro ao carregar configurações". Isso contraria o contrato do CLAUDE.md ("merge with defaults, not strict parsing").

**Proposta.** Criar TestLoad_CorruptJSON com dois casos: `{"ocr_mode": ` truncado e um arquivo vazio. O teste fixa o comportamento desejado. Pelo contrato do CLAUDE.md, ele exige cfg == DefaultConfig() e err == nil (ou um erro sentinela acompanhado dos defaults), e a correção vai em LoadFromFile. Criar também TestLoad_TypeMismatch com `{"auto_start":"sim"}`.

**Custo.** P

**Ganho esperado.** Um config.json corrompido deixa de inutilizar a fila sem nenhum aviso, e o contrato "sempre mescla com defaults" passa a ser verificado.

**Veredito do cético.** Mantido. LoadFromFile retorna o erro do json.Unmarshal (config.go:79-80), app.go:47-55 só aplica a config quando err == nil, e queue.go:314-321 recarrega a config a cada item e marca "Erro ao carregar configurações". Nenhum teste grava JSON malformado, e Load e Save estão em 0,0%. Com um config.json truncado, a fila para de funcionar sem nenhum aviso, o que contraria o contrato de mesclar com os defaults.

Status: pendente

### B-TE-02 — ParsePages: falha do OCR (fallback para texto nativo) e cancelamento no meio do PDF sem teste

- Arquivo: `internal/etl/pdf_parser.go:90`
- Lente: TE

**Evidência.** Todos os ocrRunner fake de pdf_parser_test.go (linhas 117, 194, 236 e 256) devolvem err == nil, e todos os testes usam context.Background(). Por isso três trechos nunca rodam: o ramo de erro do OCR, que grava `ocr:false` e `content: task.fallback` (pdf_parser.go:90-97), e os dois desvios de cancelamento (110-114 e 183-186). A cobertura de ParsePages é 75,8%. E o fallback é justamente o que deve acontecer quando o Tesseract falha em produção.

**Proposta.** Criar dois testes, ambos via parsePagesWithDeadline, que precisa passar a aceitar um ctx e o erro esperado:

- TestPDFParser_ParsePages_OcrErrorFallsBackToNative: uma página com uma imagem 200x200 e um pouco de texto nativo, e um ocrRunner que devolve errors.New("x"). O teste exige ocr==false e content==texto nativo.
- TestPDFParser_ParsePages_CancelledContext: um PDF de 3 páginas, com o ctx cancelado dentro do primeiro ocrRunner. O teste exige o erro context.Canceled e pages == nil.

**Custo.** P

**Ganho esperado.** ParsePages chega a cerca de 90% de cobertura, e o fallback de OCR e o cancelamento, de que a fila depende, passam a ser verificados.

**Veredito do cético.** Mantido. ParsePages está em 75,8%, os quatro ocrRunner fake devolvem err == nil e todos os testes usam context.Background(). Assim, o fallback (pdf_parser.go:90-97) e o cancelamento (l.110-114) nunca rodam. Os testes são baratos e cobrem exatamente o que acontece em produção quando o Tesseract falha.

Status: pendente

### B-TE-03 — A fila (Queue) não tem teste: cancelamento, pausa, deduplicação e recuperação de panic nunca são exercitados

- Arquivo: `internal/etl/queue.go:182`
- Lente: TE

**Evidência.** O `go test -coverprofile` em ./internal/etl mostra NewQueue, AddFiles, Pause, Resume, CancelItem, RemoveItem, Clear, startProcessing, processItem e broadcast todos em 0,0%. Nenhum `*_test.go` chama NewQueue ou CancelItem. Ficam sem cobertura:

- o desvio `ctx.Err() == context.Canceled` que leva ao status "cancelled";
- o cancelamento de um item "pending";
- o encadeamento `isWorking=false` seguido de `startProcessing()` depois de um erro ou panic;
- a deduplicação por Path em AddFiles;
- a ordem "mais antigo primeiro".

Os números de linha da evidência original (466-491) não batem com o arquivo, que tem 376 linhas. Os trechos certos são: recover na l.272, desvio context.Canceled na l.339 e deduplicação na l.91.

**Proposta.** Trocar o `*Pipeline` de Queue por uma interface mínima, por exemplo `type zipProcessor interface{ ProcessZip(ctx, path, cfg, onProgress) (string,int,int,error) }`, e escrever os testes com um fake:

1. O fake bloqueia até ctx.Done(). CancelItem deixa o item em "cancelled", e o próximo pendente começa.
2. CancelItem num item pendente deixa o item em "cancelled", e o fake nunca o recebe.
3. Um fake que dá panic deixa o item em "error", e a fila continua.
4. AddFiles chamado duas vezes com o mesmo path gera 1 item.
5. Com Pause antes de AddFiles, nada roda até Resume.
6. Com três itens, o fake os recebe na ordem de chegada.
7. Depois de Shutdown, o item em curso fica "cancelled".

Rodar todos com -race.

**Custo.** M

**Ganho esperado.** A cobertura de queue.go passa de 0% para cerca de 80%. Uma regressão no cancelamento ou no autoencadeamento (a fila travada com isWorking=true) passa a quebrar o teste.

**Veredito do cético.** Mantido. O coverprofile confirma que todas as funções de queue.go estão em 0,0% e que nenhum teste chama NewQueue ou CancelItem. O código tem o recover (l.272), o desvio context.Canceled (l.339), a deduplicação por Path (l.91) e o encadeamento por isWorking. Os números de linha da evidência estão errados, mas a lacuna é real e atinge a parte concorrente mais frágil.

Status: pendente

### B-TE-04 — Pipeline.ProcessZip não tem teste unitário; os caminhos de erro e de cancelamento nunca rodam (e há um defeito de progresso escondido)

- Arquivo: `internal/etl/pipeline.go:325`
- Lente: TE

**Evidência.** A cobertura de ProcessZip em `go test ./internal/etl` é 0,0%. Só scripts/bench/bench_test.go:121 chama a função, como benchmark e só no caminho feliz. Ficam sem cobertura: o ZIP sem PDF nem HTML (pipeline.go:67-76), o cancelamento durante o parse (pipeline.go:326-334 e 407-410), a limpeza dos temporários (pipeline.go:396-400), a falha ao gravar a saída (pipeline.go:417-426) e o ramo de PDF corrompido (pipeline.go:335-341).

O ramo de PDF corrompido retorna sem somar páginas (`processedPagesVal`) nem documentos (`processedDocsVal`). Com um ZIP que tem um PDF inválido, a tela mostra "Docs: N-1/N", e o progresso não chega a 100% antes do sucesso. Nenhum teste pegaria isso.

**Proposta.** Criar internal/etl/pipeline_test.go, montando os ZIPs em memória com buildPDF e zip.Writer, com estes casos:

- (a) Um ZIP só com .txt devolve o erro "no PDF or HTML".
- (b) Um ZIP com 1 HTML e 1 PDF válido gera o .txt ao lado; o teste confere a contagem de documentos e páginas e a ordem dos `<evento>`.
- (c) Um PDF com bytes lixo faz a saída conter "[ERRO ao analisar PDF", e o último onProgress tem de reportar processedDocs==totalDocs. Hoje isso falha.
- (d) Com o ctx já cancelado, a função retorna context.Canceled e nenhum .txt é criado.
- (e) Depois de (b), (c) e (d), não sobra nenhum eprocia_pdf_*.pdf em os.TempDir().

Usar cfg.OcrMode="none" para não depender dos binários. Como os ramos de erro de HTML também deixam de somar o documento, a correção do progresso deve cobri-los também.

**Custo.** M

**Ganho esperado.** ProcessZip ganha cobertura de unidade, o furo de progresso no ramo de PDF corrompido fecha e o vazamento de temporários fica coberto por teste.

**Veredito do cético.** Rebaixado. ProcessZip está mesmo em 0,0%, e o ramo de PDF corrompido (pipeline.go:335-341) retorna sem somar páginas nem documentos. Mas a evidência original dizia que só esse ramo deixa de somar, "ao contrário de todos os outros ramos de erro", e isso é falso para a contagem de documentos: os ramos de erro de HTML (l.357 e 371) também retornam antes do AddInt32(&processedDocsVal) da l.387. Então o teste (c), do jeito que foi proposto, também falharia nesses ramos. Além disso, o defeito de progresso é cosmético, porque a saída é gravada e o sucesso é emitido normalmente.

Status: pendente

### B-TE-05 — Watchdog: os erros de Start e a troca de pasta não têm teste

- Arquivo: `internal/watchdog/watchdog.go:50`
- Lente: TE

**Evidência.** A cobertura de Start é 78,6% e a de watchLoop é 71,4%. Nenhum teste chega a estes trechos: o erro `failed to access path` (watchdog.go:51-53), o erro `path is not a directory` (54-56), o atalho para quando o caminho é o mesmo (38-40) e a troca de pasta (l.43, stopLocked seguido de um novo watcher). TestWatcher_StartAndStop só confere GetCurrentPath; não verifica se o callback para de disparar depois de Stop.

**Proposta.** Três testes:

- Um teste de tabela para Start com um caminho inexistente e com um arquivo no lugar da pasta. Os dois casos exigem erro e GetCurrentPath()=="".
- Um teste de troca de pasta: chamar Start(dirA) e depois Start(dirB). Um .zip criado em dirA não pode ser detectado em 1 s; um .zip criado em dirB tem de ser detectado.
- Um teste pós-Stop: um .zip criado depois de Stop não pode disparar o callback.

**Custo.** P

**Ganho esperado.** Os caminhos de erro de configuração da pasta monitorada, que vêm direto da tela de Settings, passam a ter teste.

**Veredito do cético.** Mantido. Start está em 78,6% e watchLoop em 71,4%. Os erros "failed to access path" e "path is not a directory" (watchdog.go:51-56) e a troca de pasta não têm teste, e watchdog_test.go só tem StartAndStop, DetectZipFile e IgnoreNonZipFile. A gravidade e o custo são baixos, como declarado.

Status: pendente

### B-TE-06 — OCR: binário ausente, falha do pdftoppm e fallback do Tesseract não têm teste; o único teste é pulado quando a extração falha

- Arquivo: `internal/ocr/tesseract.go:110`
- Lente: TE

**Evidência.** tesseract_test.go tem um só teste de integração, TestRunOCR. Se InitBinaries falhar, ele chama `t.Skipf` (tesseract_test.go:46), e a suíte passa sem testar nada. Pelo coverprofile, RunOCR está em 83,2%, extractFS em 72,0% e InitBinaries em 76,2%.

Ficam sem teste quatro ramos: o `os.Stat` de tesseract.exe e de pdftoppm.exe com o binário ausente (tesseract.go:110 e 113), a falha do pdftoppm (l.153), a imagem não encontrada e o fallback sem `-l por` (próximo da l.184). Hoje não dá para apontar RunOCR para um diretório sem os .exe, porque binDir é global e é fixado por sync.Once. Os números de linha da evidência original (612-718) estavam errados, porque o arquivo tem 227 linhas.

**Proposta.** Extrair `runOCRIn(ctx, binDir, pdfPath, page, mode)` e deixar RunOCR como um wrapper fino sobre InitBinaries. Depois, quatro testes:

1. Com binDir vazio em t.TempDir(), o erro contém "tesseract.exe not found".
2. Só com o tesseract presente, o erro contém "pdftoppm.exe not found".
3. Com um PDF inexistente e os binários reais, o erro é "embedded pdftoppm failed".
4. Com o ctx cancelado, há erro e a pasta temporária é removida.

Trocar o Skipf por Fatalf quando o diretório embed existe: nesse caso, a extração falhar é regressão, não um problema do ambiente.

**Custo.** M

**Ganho esperado.** O caminho "OCR indisponível", que cai no fallback de texto nativo, ganha um teste determinístico, e a suíte deixa de passar verde quando a extração dos binários quebra.

**Veredito do cético.** Rebaixado. Os ramos existem (tesseract.go:110, 113, 153 e 184) e o Skipf também (tesseract_test.go:46), mas os números de linha citados (612-718) são falsos. Na prática, quando o OCR falha, o ParsePages já cai no fallback de texto nativo. E a refatoração proposta (runOCRIn) custa M só para testar mensagens de erro de baixo impacto.

Status: pendente

## Derrubados

Nenhum achado foi derrubado na refutação cética desta rodada.

| título | lente | justificativa |
|---|---|---|
| — | — | — |

## Como aprovar

Para aprovar todos os achados:

```
/corrigir B-TE-01 B-TE-02 B-TE-03 B-TE-04 B-TE-05 B-TE-06
```

Para aprovar só alguns, passe apenas os ids escolhidos, por exemplo `/corrigir B-TE-01 B-TE-02 B-TE-03`.
