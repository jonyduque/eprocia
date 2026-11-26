# Processar PDF avulso (fora de ZIP)

Data: 2026-11-18. Base lida: worktree `revisao-2026-11-10` (main com a v0.4.2). Revisada por um revisor sem contexto em 2026-11-18 (aprovada com ajustes, todos aplicados ou respondidos na seção 11). Decisões do dono p1 a p3 (2026-11-19), e1 a e5 (2026-11-20) e q1 (2026-11-21) aplicadas (seção 10). Ajustada em 2026-11-20 depois de uma segunda revisão sem contexto, conferida no código da `main` com a rodada 2026-11-10 já juntada (seção 11); não há pontos em aberto.

Pedido do dono: "a possibilidade de processar arquivos PDF diretamente, sem ser em pastas zip."

**Rodada de correção:** já juntada à `main` (PR #13). F3, F4 e F5 estão feitas; os textos do arraste, da recusa e do botão do vazio mudam a partir do código atual (seções 2.3 e 7).

## 1. Problema e comportamento esperado

Hoje só entram ZIPs: o arraste filtra `.zip` no frontend (`App.svelte:189`, por `separarZips` em `frontend/src/lib/aviso.js:68`, com a recusa de `avisoSoltura` no componente de aviso), os diálogos filtram `*.zip` (`app.go:389-420`), o watchdog só reage a `.zip` (`watchdog.go:155`, `pendentes.go:30`) e o pipeline começa por `ReadZipFiles` (`pipeline.go:300`). Quem baixou do Eproc só a petição inicial, uma sentença ou um laudo, ou recebeu um PDF por e-mail, precisa zipar o arquivo à mão para usar o eprocIA.

Comportamento esperado:

- O usuário arrasta um ou mais PDFs para a janela, ou escolhe pelo botão de selecionar (agora "Selecionar arquivos..." com o filtro "ZIP do Eproc e PDF (*.zip;*.pdf)").
- **Um soltar (ou uma seleção) com vários PDFs vira um item só da fila e um TXT só** (p3), com os PDFs organizados por evento, como os documentos de um ZIP: cada PDF é um `<documento>` dentro do `<evento>` tirado do nome dele, na mesma ordem natural do ZIP. Um soltar com um PDF só é o caso particular de um item com um documento.
- O PDF passa pelo mesmo caminho de um documento de dentro do ZIP: texto nativo, segundo leitor (Poppler), OCR conforme a config, higienização, marcadores de não lido. A saída tem o formato `<contexto>/<evento>/<documento>/<pag>` do PRD §6.
- Exceção: a omissão de tipos (`OmitirTipos`, H10) não vale para o PDF avulso. Quem arrasta um PDF quer aquele PDF, mesmo que o tipo dele (ex.: `PROC1`) esteja na lista de omitidos.
- **Nome do TXT (p1):** `processo.pdf` gera `processo.txt`, na pasta do PDF. Se já existir um arquivo com esse nome (por exemplo, o TXT do ZIP de mesmo nome), gera `processo_2.txt`, depois `processo_3.txt`, e assim por diante. O TXT de um PDF avulso nunca sobrescreve arquivo existente, salvo o reprocessamento, que substitui o TXT que o próprio item gerou (e1), e o PDF da pasta monitorada editado depois de convertido, tratado como reprocessamento do item concluído (q1). Para vários PDFs, o nome é o número do processo comum aos PDFs ou, se não houver um comum, o nome do primeiro PDF na ordem por evento, com a mesma regra do `_2` (e2); a pasta é a do primeiro PDF nessa ordem, mesmo que os PDFs venham de pastas diferentes (e3).
- Copiar, Abrir pasta, Reprocessar, notificação de fim e contagem de não lidos funcionam igual.
- A linha mostra um ícone diferente para PDF e para ZIP; um item de vários PDFs mostra o nome do TXT e a contagem ("5000000-00.2026.4.00.0001 (3 PDFs)").
- Misturar ZIPs e PDFs no mesmo arraste funciona: cada ZIP vira seu item, e os PDFs daquele arraste juntos viram um item. Outros tipos continuam recusados, com a mensagem reescrita ("Arraste arquivos ZIP do Eproc ou PDF").
- **Pasta monitorada (p2):** interruptor em Configurações, "Também converter PDFs novos nesta pasta", **desligado por padrão**. Ligado, a pasta monitorada pega também os PDFs novos, cada um no seu item e no seu TXT, mesmo que cheguem juntos (e4). A varredura de abertura da pasta (ao ligar a opção e a cada abertura do eprocIA) também converte os PDFs das últimas 24 h que ainda não têm TXT, como já faz com os ZIPs; a ajuda do interruptor diz isso (decisão do orquestrador, 2026-11-26: o comportamento fica, muda só o texto).

## 2. O que muda em cada camada

### 2.1 Go

**Ponto central de risco.** `fontePDF.liberar` (`pipeline.go:157-185`) apaga do disco o arquivo em `fontePDF.caminho`, porque hoje ele é sempre um temporário. Se o PDF do usuário virar caminho de uma `fontePDF`, o eprocIA apaga o original. A especificação evita isso por construção: **o caminho do PDF do usuário nunca vira `fontePDF.caminho`**. Ele é lido pela mesma rotina de extração do ZIP (`extrairPDF`, `pipeline.go:205`: para a memória até 32 MiB, ou para um temporário próprio), e só o temporário é apagado. O custo é uma cópia para `%TEMP%` nos PDFs acima de 32 MiB, o mesmo de um PDF dentro de ZIP.

**`internal/models/models.go`:** `ProcessFile` ganha `Abrir func() (io.ReadCloser, error)` e `Tamanho int64` (`json:"-"`) e `Avulso bool`. Para entrada de ZIP, `Abrir` é `f.ZipFileRef.Open` e `Tamanho` é `UncompressedSize64`; para PDF avulso, `Abrir` é `os.Open` do caminho e `Tamanho` vem do `os.Stat`. `ZipFileRef` continua existindo para o que ainda depende dele. `Config` ganha `MonitorarPDF bool` (`json:"monitorar_pdf"`, padrão `false` em `DefaultConfig` e no overlay de `LoadFromFile`).

**`internal/etl/sorter.go`:** `ParseFileInfo(*zip.File)` passa a delegar a `ParseNome(nome string) models.ProcessFile`, que extrai "Evento N" e o número do documento do nome. O PDF avulso usa `ParseNome(filepath.Base(caminho))`: "5000000-00.2026.4.00.0001 - Evento 12 - SENT1.pdf" cai no evento 12; um nome sem "Evento N" cai em `noEventNum`, como acontece hoje com arquivos sem evento dentro do ZIP. Vários PDFs de um item são ordenados por `SortProcessFiles`, sem regra nova.

**`internal/saida` (pacote folha, compartilhado com a spec do histórico):** a regra única do nome de saída, usada pelo pipeline (hoje `pipeline.go:736`), pela varredura do watchdog (`pendentes.go:38`), por `lerSaidaParaCopiar` e pela validação do histórico, se ele existir.

- `Base(entrada string) string`: `<pasta>\<nome sem extensão>.txt`.
- `NomeLote(pdfs []string) string`: a base do TXT de um item de PDFs, conforme e2 e e3: número de processo comum a todos os nomes, senão o nome do primeiro PDF na ordem por evento; pasta do primeiro PDF nessa ordem. Com um PDF só, é `Base` dele.
- `GravarSemSobrescrever(base string, conteudo) (caminho string, err error)`: abre `base` com `O_CREATE|O_EXCL`; se o nome existir, tenta `<nome>_2.txt`, `<nome>_3.txt`, até `_999` (depois disso, erro "não há nome livre para o TXT"). O `O_EXCL` torna a escolha e a criação uma operação só, sem janela entre conferir e gravar. A gravação de um nome já criado segue a regra atual (direta, `pipeline.go:738`).
- Regra de convivência com o ZIP homônimo (e5): a base de PDF é tratada como ocupada também quando existe `<nome>.zip` na mesma pasta, ainda que `<nome>.txt` não exista, para não tomar o nome que o ZIP vai usar.
- `DaEntrada(entrada, saida string) bool`: a saída está na pasta esperada e se chama `<nome>.txt` ou `<nome>_N.txt` (N inteiro de 2 a 999). Substitui a igualdade exata com `Caminho(Path)` que a spec do histórico previa.
- `SaidaDoItem(kind, path string, paths []string, saida string) bool`: a regra única de "esta saída pertence a este item", usada por `lerSaidaParaCopiar`, pelo reprocessamento (e1) e pela validação do histórico. ZIP: `saida` igual a `Base(path)` (sem diferença de caixa). PDF com um caminho: `DaEntrada(path, saida)`. PDF com vários: a mesma conferência de `DaEntrada`, mas contra a base `NomeLote(paths)` (pasta do primeiro PDF na ordem por evento e nome do lote), porque o TXT não tem o nome do primeiro PDF (e2). `saida` vazia devolve falso.
- O ZIP mantém o comportamento atual: grava `<nome>.txt` e sobrescreve o próprio TXT.
- **Este pacote é entregue por esta spec** (com `Base`, `NomeLote`, `GravarSemSobrescrever`, `DaEntrada` e `SaidaDoItem`); a spec do histórico depende dele e não o conta na estimativa. Se o histórico for implementado antes, a tarefa do pacote `saida` desta spec vai na frente.

**`internal/etl/entrada.go` (novo; `zip_reader.go` fica como está):** `LerEntrada(caminhos []string) ([]models.ProcessFile, io.Closer, error)`. Um caminho `.zip` (sem diferença de caixa) chama `ReadZipFiles`; uma lista só de `.pdf` devolve um `ProcessFile` com `Avulso: true` por PDF e um `Closer` vazio; mistura, lista vazia ou outra extensão devolvem "tipo de arquivo não suportado". Não confere a assinatura `%PDF`: um PDF em envelope PKCS#7 não começa com ela e o `abrirPDF` (`abertura.go`) já o encaminha ao Poppler; um arquivo que não é PDF vira documento `ilegivel` pelo caminho normal.

**`internal/etl/pipeline.go`:**

- **Método novo `ProcessarEntrada(ctx, Entrada, cfg, onProgress, atv)`**, com `Entrada{Kind string; Caminhos []string; SaidaAnterior string}`, que faz o trabalho de hoje de `ProcessZipComAtividade` trocando `ReadZipFiles(zipPath)` por `LerEntrada(Caminhos)`. `ProcessZip` e `ProcessZipComAtividade` ficam **com a assinatura atual (string)**, como wrappers de uma linha que montam `Entrada{Kind: "zip", Caminhos: []string{zipPath}}`: as cerca de 40 chamadas em testes (`abertura_test`, `erro_saida_test`, `pdf_sem_ocr_test`, `pipeline_saida_test`, `scripts/bench` etc.) não mudam. O campo `processar` da fila (`queue.go:85`, chamado em `queue.go:549`), que hoje recebe só o caminho, passa a receber a `Entrada` e a chamar `ProcessarEntrada`; só os fakes dos testes da fila mudam. `rc.RegisterDecompressor` só se aplica quando a entrada é ZIP (o `Closer` é o `*zip.ReadCloser` nesse caso).
- `extrairPDF` usa `f.Abrir()` e `f.Tamanho` em vez de `ZipFileRef.Open` e `UncompressedSize64`. O caminho do HTML (`pipeline.go:683`) também passa a usar `f.Abrir()`, para não sobrarem dois jeitos de abrir. A conferência de "entrada maior que o tamanho declarado" vale também para o avulso, com a frase "o arquivo mudou durante a leitura" (o usuário pode estar salvando por cima).
- A omissão de tipos (`tipoOmitido`, `pipeline.go:251-256`, aplicada em `:346-348`) é pulada quando `f.Avulso`.
- A mensagem "no PDF or HTML documents found in ZIP" continua só para ZIP. O marcador de falha ao abrir um documento, hoje "[ERRO ao extrair PDF do ZIP: ...]" (`pipeline.go:638`), vira "[ERRO ao abrir PDF: ...]" quando `f.Avulso` (o texto depois dos dois-pontos continua vindo de `textoErroSaida`).
- A gravação usa `saida`: ZIP em `Base`, como hoje; item de PDF em `GravarSemSobrescrever(NomeLote(...))`, salvo no reprocessamento (e1): se `Entrada.SaidaAnterior` não for vazia, passar em `saida.SaidaDoItem` e o arquivo ainda existir, a gravação vai direto nela (como a do ZIP, `pipeline.go:738`); senão, escolhe nome livre de novo.
- **Relógio do formatter:** a linha "Data de Consolidação" usa `time.Now()` direto (`formatter.go:468`). Passa a ler uma variável do pacote (`agoraFormatter = time.Now`), trocada nos testes, para o teste de equivalência comparar byte a byte.

**`internal/etl/queue.go`:**

- `AddFiles` valida a extensão no Go (hoje só o frontend filtra, e o binding `AddFilesToQueue` aceita qualquer caminho vindo do WebView): aceita `.zip` e `.pdf`, sem diferença de caixa; o resto é descartado com log. Cada `.zip` vira um item; **todos os `.pdf` da mesma chamada viram um item só** (uma chamada é um soltar ou uma seleção; o watchdog chama com um caminho por vez). A deduplicação por `Path` continua igual para ZIP; um PDF que já está num item `pending`/`processing` sai do item novo, e um item novo que fique sem PDF não é criado. O retorno de `AddFiles` (e de `AddFilesToQueue`) continua sendo o número de **arquivos** que entraram, não de itens: três PDFs juntos num item devolvem 3, e o anúncio `textoAceitos` (`frontend/src/lib/soltura.js:13`, chamado em `App.svelte:195`) segue dizendo "3 arquivos adicionados".
- `QueueItem` ganha `Kind string` (`json:"kind"`, `"zip"` ou `"pdf"`) e `Paths []string` (`json:"paths,omitempty"`, os PDFs do item, na ordem por evento; vazio para ZIP). `Path` continua sendo o ZIP ou, no item de PDF, o primeiro PDF da ordem, para os usos atuais (log, título, deduplicação). Os dois campos são copiados em `copiarItem` (`queue.go:236-258`). Se o histórico existir, `Restaurar` deriva `Kind` do `Path` e guarda `Paths`.
- `Name` do item de PDF: no `AddFiles`, é a base provisória (`NomeLote` sem pasta e sem extensão), com " (N PDFs)" quando N > 1. Como o `_N` só se decide na gravação, ao terminar com sucesso o item troca `Name` pelo nome do `OutputPath` sem extensão (mais " (N PDFs)"), antes do broadcast final: a linha nunca diz "processo" com o TXT em `processo_2.txt`.
- `QueueItem` ganha o campo não exportado `saidaAnterior string`. `Reprocessar` o preenche com `OutputPath` **antes** de zerar `OutputPath` (`queue.go:287`), e `processItem` o passa ao pipeline em `Entrada.SaidaAnterior` (e1). `Restaurar` (histórico) o preenche com o `output_path` gravado, para o reprocessamento de um item restaurado também gravar por cima. Para ZIP o campo é ignorado (a saída do ZIP é sempre `Base`).
- `Reprocessar` de um item de PDF relê todos os `Paths`; se algum não existir mais, devolve "arquivo de origem não encontrado: <nome do PDF>" e não muda a fila.
- **PDF editado na pasta monitorada (q1):** `Queue.ReprocessarMonitorado(path string) bool`, chamada pelo callback do watcher antes de `AddFiles`. Se a fila tem um item de `success`, `Kind` `pdf`, com um `Paths` só igual a `path` (sem diferença de caixa), faz o que `Reprocessar` faz (`saidaAnterior` recebe o `OutputPath`, o item volta a `pending` na mesma posição, sem override: vale a config global) e devolve `true`, e o TXT do item é substituído (e1). Em qualquer outro caso (nenhum item, item `error`, `cancelled` ou de vários PDFs; `pending` e `processing` já são deduplicados por `AddFiles`) devolve `false` e o callback segue para `AddFiles`: TXT novo no próximo `_N` (p1). Sem item na fila (programa reaberto, histórico desligado) vale, portanto, o `_N`; com o histórico ligado, um item restaurado de `success` conta como item na fila (a `Restaurar` já preenche `saidaAnterior`). O descarte de `handleNewFile` (acima) continua vindo antes: só o PDF mais novo que o TXT chega aqui.

**`app.go`:** `SelectFiles` (novo) com os filtros "ZIP do Eproc e PDF (*.zip;*.pdf)", "Somente ZIP (*.zip)" e "Somente PDF (*.pdf)", com seleção múltipla. `SelectZipFiles` e `SelectZipFile` ficam como estão; a limpeza dos bindings legados não faz parte deste recurso. `lerSaidaParaCopiar` (`app.go:361-377`) passa a aceitar a saída de item com sucesso só se `saida.SaidaDoItem(Kind, Path, Paths, OutputPath)`, a mesma função que o histórico usa ao carregar.

**`internal/watchdog`:**

- `watchLoop` (`watchdog.go:155`) e `pendingZips` aceitam `.pdf` quando `Config.MonitorarPDF` estiver ligado. `pendingZips` passa a receber a lista de extensões e usa `saida`. Para ZIP, a regra continua: convertido se houver `<nome>.txt` igual ou mais novo. Para PDF: convertido se houver `<nome>_N.txt` igual ou mais novo, ou `<nome>.txt` igual ou mais novo quando não houver `<nome>.zip` ao lado (com o ZIP homônimo, `<nome>.txt` é do ZIP). Ignora nomes que começam com `eprocia_pdf_` (os temporários do pipeline, caso alguém monitore `%TEMP%`). Essa regra fica numa função só (`jaConvertido(caminho, mod)`), usada pela varredura e pelo evento ao vivo.
- **Evento ao vivo de PDF já convertido:** o `watchLoop` dispara `handleNewFile` a cada Create, Write e Rename (`watchdog.go:154-160`), sem deduplicar por caminho, e a fila só deduplica contra `pending`/`processing`. Para ZIP isso só reprocessa e sobrescreve o mesmo TXT; para PDF, pela regra de nunca sobrescrever (p1), cada evento que chegasse depois do fim do item (Write tardio do navegador ou do antivírus) geraria `processo_2.txt`, `_3`... Por isso `handleNewFile` (`watchdog.go:176-219`), depois de o tamanho estabilizar e antes de chamar o callback, descarta o PDF para o qual `jaConvertido` é verdadeiro (há TXT dele igual ou mais novo que o PDF). ZIP não muda. Um PDF alterado depois de convertido (mais novo que o TXT) passa nesse descarte e chega ao callback do watcher, que o trata como reprocessamento (q1, abaixo, em `queue.go`).
- Cada PDF novo detectado vira um item próprio, com seu TXT, sem juntar PDFs que cheguem perto um do outro (e4).
- O `Watcher` recebe a lista de extensões em `Start`. Hoje `Start` com a mesma pasta não faz nada (`watchdog.go:46-49`); passa a comparar pasta **e** extensões, e só é no-op se as duas forem iguais. Assim, mudar a opção em Configurações (o `SaveConfig` já chama `Start`, `app.go:293`) reinicia o watcher, como já acontece ao trocar de pasta.

> **Nota da execução (2026-11-26, Task 6):** `jaConvertido` não aceita, para um PDF, um TXT cujo radical é o nome de outro PDF ou ZIP da pasta: `processo_2.txt` ao lado de `processo_2.pdf` é desse PDF, não de `processo.pdf`. A spec era omissa; o custo, se a regra estiver errada, é uma reconversão num `_N` novo. Falso positivo conhecido: se `processo_2.txt` for a saída de `processo.pdf`, um `processo_2.pdf` copiado depois para a pasta com mtime mais antigo que esse TXT conta como convertido e não ganha TXT.

### 2.2 Bindings

`SelectFiles` e os campos `kind` e `paths` em `QueueItem`; rodar `wails generate module`.

### 2.3 Frontend

- `frontend/src/lib/aviso.js`: `separarZips` (`:68`) vira `separarEntradas(caminhos)`, que devolve `{ aceitos, ignorados }` com `.zip` e `.pdf` sem diferença de caixa; `avisoSoltura` (`:75`) troca "Só arquivos ZIP do eproc são aceitos" por "Arraste arquivos ZIP do Eproc ou PDF" (e a forma de atenção, "N arquivos ignorados: só ZIP do Eproc ou PDF"). A lógica fica em `src/lib`, com teste em `frontend/avisos.test.mjs`, que já cobre `separarZips` e `avisoSoltura`.
- `App.svelte:187-206` (`OnFileDrop`): usa `separarEntradas` e manda todos os aceitos numa chamada só de `AddFilesToQueue` (é o que faz o Go juntar os PDFs num item); `textoJaNaFila` recebe o número de aceitos.
- `App.svelte:380-390` (`triggerManualSelect`, que hoje chama `SelectZipFiles` em `:382`) e o botão do painel (`App.svelte:599`): chamar `SelectFiles`; rótulo "Selecionar arquivos...".
- `ProcessList.svelte`: ícone por `kind` (`Icone.svelte` ganha o de documento, se ainda não tiver); nome com a contagem de PDFs; botão e texto do vazio (`ProcessList.svelte:560-564`, hoje "Selecionar arquivos ZIP..." e "ou arraste os ZIPs para a janela") passam a "Selecionar arquivos..." e "ou arraste ZIPs do Eproc ou PDFs para a janela".
- `Settings.svelte`: interruptor "Também converter PDFs novos nesta pasta" sob a pasta monitorada, desligado por padrão, com a frase "Cada PDF novo na pasta ganha um TXT ao lado, inclusive boletos e outros PDFs que não são de processo. Ao ligar e a cada abertura, os PDFs das últimas 24 h ainda sem TXT também são convertidos".
- `DESIGN.md` e `PRD.md`: RF01 passa a dizer "ZIP do Eproc ou PDF avulso (vários PDFs de uma vez viram um TXT, por evento)"; RF02 ganha "e, se a opção estiver ligada, PDFs novos"; §5 passo 5 cita a regra do `_2`.

### 2.4 Instalador

Nenhuma mudança. Associar `.pdf` ao eprocIA no Explorer fica fora do escopo: o PDF tem dono, o leitor do usuário.

## 3. Formato de dados e onde fica gravado

- **Saída:** um TXT por item, no formato do PRD §6. Para "5000000-00.2026.4.00.0001 - Evento 12 - SENT1.pdf" sozinho: um `<evento>` 12 com um `<documento>`; se o PDF tiver uma página só, sem `<pag>`, como já acontece com documentos de uma página. Para vários PDFs: um `<evento>` por número de evento, com um `<documento>` por PDF, na ordem do ZIP; o `<summary>` lista todos.
- **Nome e pasta da saída (p1, p3):**
  - um PDF: `<nome>.txt` na pasta do PDF; ocupado, `<nome>_2.txt`, `<nome>_3.txt`...;
  - vários PDFs: base por e2 (o número de processo, no formato CNJ `NNNNNNN-DD.AAAA.J.TR.OOOO`, quando todos os nomes trazem o mesmo; senão o nome do primeiro PDF na ordem por evento), na pasta por e3 (a do primeiro PDF nessa ordem), com a mesma regra do `_2`;
  - "ocupado" é existir qualquer arquivo com esse nome ou, por e5, existir `<nome>.zip` na mesma pasta;
  - com as extensões ocultas do Windows, `processo_2.txt` aparece como "processo_2", sem se disfarçar de PDF.
- Nada novo na config, salvo `MonitorarPDF` (p2). Nada novo em `%APPDATA%`.

## 4. Casos de borda e falhas

- **PDF protegido por senha:** `abrirPDF` marca o documento como `senha` (não lido). O item termina como sucesso com 1 não lido; o TXT traz "Documentos não lidos: 1" e "[documento não lido: ...; consulte o PDF original]", como um PDF protegido dentro de ZIP. A linha mostra "1 não lido" com o motivo. Não se pede a senha nesta etapa. Num item de vários PDFs, só aquele documento fica não lido.
- **PDF digitalizado com OCR desligado:** mesmo comportamento atual ("[página digitalizada: o OCR está desativado]", não lido, faixa "Reprocessar com OCR").
- **Tipo omitido nas configurações:** ignorado para o avulso (seção 1); o PDF é lido.
- **PDF corrompido ou com outra coisa dentro e extensão `.pdf`:** documento ilegível; o item termina com 1 não lido.
- **Arquivo de 0 bytes, movido ou apagado entre o arraste e o processamento:** num item de um PDF, erro do item ("arquivo vazio" ou "arquivo não encontrado"), sem TXT; quem decide é `LerEntrada`, com `os.Stat` antes de montar a lista. Num item de vários PDFs, `LerEntrada` mantém o documento na lista e ele termina pelo caminho de hoje de um PDF que não abre: `Leitura` = `ilegivel` ("arquivo ilegível", por `classificarErro`), pelo mesmo marcador que um PDF quebrado dentro do ZIP recebe hoje. Arquivo ausente: `extrairPDF` falha no worker, marcador "[ERRO ao abrir PDF: falha de leitura ou gravação em disco]" (`textoErroSaida` sobre o `*fs.PathError`). Arquivo de 0 bytes: os dois leitores o recusam em `abrirPDF`, marcador "[ERRO ao analisar PDF: <texto de abrirPDF>]" (`pipeline.go:627`), sem caminho. Os outros documentos seguem; se todos falharem, erro do item, sem TXT.
- **Nome do TXT ocupado:** `_2`, `_3`... (p1). Esgotado o `_999`, erro do item "não há nome livre para o TXT".
- **PDF com o mesmo nome de um ZIP na mesma pasta:** quando o ZIP já existe na pasta, o ZIP fica com `<nome>.txt` e o PDF vai para `<nome>_2.txt` (e5). Limitação: se o PDF for convertido numa pasta onde o ZIP homônimo ainda não existe, ele fica com `<nome>.txt`, e um ZIP de mesmo nome que chegue depois sobrescreve esse TXT (o ZIP mantém o comportamento atual) e pode ser pulado pela varredura de abertura do monitoramento, que acha `<nome>.txt` mais novo. O caso exige baixar a peça avulsa antes do processo inteiro, com o mesmo nome; fica registrado.
- **Dois PDFs de pastas diferentes com o mesmo nome no mesmo soltar:** viram dois `<documento>` com o mesmo nome no TXT, como duas entradas de mesmo caminho num ZIP (as chaves do mapa `eventos` já levam o índice).
- **Arquivo apagado ou trocado entre a pré-contagem e o worker** (o PDF é lido duas vezes, como dentro do ZIP): a segunda leitura falha ou vem com tamanho diferente; o documento termina com "falha de leitura ou gravação em disco" (`textoErroSaida`). Alternativa a avaliar no plano: para o avulso, manter a `fontePDF` da pré-contagem até o worker, como já se faz no caso Poppler (`manterSoDisco`), e ler o arquivo uma vez só. **Essa alternativa só vale com um temporário próprio ou com os bytes em memória; nunca com o caminho do usuário em `fontePDF.caminho`**, pelo risco da seção 2.1.
- **PDF aberto em outro programa:** o Acrobat abre com compartilhamento de leitura, e `os.Open` funciona. Um programa que trava o arquivo com exclusividade faz `os.Open` falhar com violação de compartilhamento, um `*fs.PathError`, que `textoErroSaida` (`abertura.go:181`) já transforma em "falha de leitura ou gravação em disco". Vale a frase atual, sem caso novo: o documento termina como não lido com `Leitura` = `ilegivel` e essa frase no marcador (ou, num item de um PDF só, o item termina em erro com ela).
- **Usuário apagando o PDF durante o processamento:** enquanto o handle está aberto, o Windows pode recusar apagar ou renomear o arquivo (conferir o modo de compartilhamento do `os.Open` na versão do Go do `go.mod`); por isso a extração fecha o handle assim que termina de copiar (já é o padrão de `extrairPDF`, com `defer Close`).
- **O original nunca é alterado nem apagado,** inclusive com panic, cancelamento, prazo do OCR estourado e caminho do Poppler.
- **PDF muito grande** (centenas de MB): vai para temporário em `%TEMP%`, como dentro do ZIP; o disco precisa do espaço da cópia.
- **PDF na nuvem** (OneDrive com arquivos sob demanda): a leitura dispara o download; pode demorar, sem tratamento especial.
- **Extensão em maiúsculas (`.PDF`):** aceita.
- **Pasta monitorada com a opção ligada:** navegadores baixam como `.crdownload` ou `.part` e renomeiam no fim; o watchdog já trata o Rename e espera o tamanho estabilizar. Eventos atrasados do mesmo PDF (Write do navegador ou do antivírus depois do fim do item) não geram outro TXT: `handleNewFile` descarta o PDF que já tem TXT igual ou mais novo (seção 2.1). Um PDF editado e salvo na pasta depois de convertido substitui o TXT do item concluído que está na fila (q1); sem esse item na fila (programa reaberto, item com erro ou cancelado), ganha o próximo `_N`. PDFs que não são de processo (boletos, contracheques) também são convertidos e ganham um TXT ao lado, com OCR rodando à toa: por isso a opção nasce desligada e a frase do interruptor avisa. Com a opção desligada, nada muda em relação a hoje.
- **`falhas.json`** (`watchdog.Falhas`) funciona por caminho e vale para PDF sem mudança.

## 5. Segurança e privacidade

- Nada sai da máquina; nenhum dado novo é guardado.
- A validação de extensão passa para o Go (`AddFiles`), em vez de depender só do filtro do frontend.
- O TXT de um PDF avulso tem as mesmas garantias do TXT de ZIP: nenhum caminho, stderr ou dump de objeto PDF (`textoErroSaida`).
- A regra de nunca sobrescrever usa `O_EXCL`, então o eprocIA não apaga um `.txt` do usuário que por acaso tenha o nome do PDF.

## 6. Conflito com o RNF01

Nenhum.

## 7. Conflito com a rodada de correção

A rodada 2026-11-10 (plano `docs/revisao/2026-11-18-plano-correcao.md`) já está na `main` (PR #13). O que ela mudou e este recurso aproveita:

- **F3** e **F4**: o `alert()` saiu; o arraste usa `separarZips` e `avisoSoltura` (`frontend/src/lib/aviso.js:68-81`), o overlay de `soltura.js` e o anúncio `textoAceitos`. A mudança deste recurso é em `src/lib` (seção 2.3), com teste, sem mexer de novo no overlay.
- **F5** (F-AC-08): o botão do vazio existe (`ProcessList.svelte:560-564`), ainda com "Selecionar arquivos ZIP..."; este recurso troca o texto (seção 2.3).
- **F17** (F-VI-04, inventário de ícones): o ícone de PDF entra no mesmo inventário (`frontend/icones.test.mjs`).

O backend (seção 2.1) não conflita e pode ser feito antes ou em paralelo.

## 8. Testes que provam cada comportamento

`internal/etl` (com `OmitirTipos` vazio, salvo o teste próprio):

- **equivalência:** para cada PDF sintético já usado nos testes (fixtures de `scripts/bench/fixtures`: nativo, escaneado com OCR, vetorial; os gerados em `pdf_classes_test.go`: PDF 2.0, PKCS#7, tabela inconsistente, AESV2; e um protegido por senha), o TXT do PDF avulso é igual, byte a byte, ao TXT de um ZIP que contém só aquele PDF com o mesmo nome. E um item de vários PDFs (eventos fora de ordem, um sem "Evento N", um protegido) dá o mesmo TXT que um ZIP com esses PDFs. É o teste principal: prova que o avulso não criou um caminho paralelo. (Os nomes coincidem porque o formatter usa `DocName`, a base do nome, não o caminho dentro do ZIP; a linha "Data de Consolidação" é fixada por relógio injetado.)
- **omissão:** PDF avulso com nome de tipo omitido (ex.: "... PROC1.pdf", com `PROC` em `OmitirTipos`) é lido; o mesmo PDF dentro de ZIP continua omitido.
- **original preservado:** o original continua existindo e com o mesmo SHA-256 depois de sucesso, erro, cancelamento, panic injetado (`contarPaginasPDF` falso), prazo de OCR estourado e caminho do Poppler (`trocarPoppler`). Inclui um PDF acima de 32 MiB, que passa pelo temporário em disco.
- **temporários:** nenhum `eprocia_pdf_*.pdf` sobra depois de cada um desses casos, com **`TMP` e `TEMP`** apontando para `t.TempDir()` (no Windows, `os.TempDir` lê `TMP` antes de `TEMP`; `abertura_test.go:582-584` já faz assim).
- **`saida`:** `Base` com `.zip`, `.ZIP`, `.pdf`, `.PDF` e nome com pontos ("a.b.pdf"); `GravarSemSobrescrever` com o nome livre, com `processo.txt` existente (grava `processo_2.txt`), com `_2` e `_3` existentes (grava `_4`), com `processo.zip` ao lado e sem `processo.txt` (grava `processo_2.txt`), com `_999` ocupado (erro), e sem tocar no arquivo existente (mesmo SHA-256); `NomeLote` com número de processo comum, com números diferentes (cai no nome do primeiro PDF na ordem por evento), sem número e com um PDF só; PDFs de pastas diferentes (TXT na pasta do primeiro na ordem por evento, e3); `DaEntrada` aceita `<nome>.txt` e `<nome>_7.txt` e recusa outra pasta, `<nome>_1.txt`, `<nome>_abc.txt` e outro nome.
- **reprocessamento:** reprocessar um item de PDF grava por cima do próprio `OutputPath` e não cria `_2` (e1); com o `OutputPath` apagado, escolhe nome livre de novo; com um dos `Paths` apagado, devolve o erro e não muda a fila.
- **`ParseNome`:** com "Evento 12", sem evento, com o número do documento.
- **`LerEntrada`:** extensão não suportada; mistura de ZIP e PDF na mesma lista recusada; arquivo inexistente; 0 bytes; item de vários PDFs com um inexistente (esse documento não lido, os outros lidos) e com todos inexistentes (erro do item).
- **HTML:** os testes atuais de HTML dentro de ZIP continuam passando com `f.Abrir()`.
- **arquivo trocado entre a pré-contagem e o worker:** o documento termina com a frase fixa, sem caminho no TXT.
- **queue:** `AddFiles` recusa `.txt`, `.exe`, `.docx` e aceita `.pdf` e `.PDF`; uma chamada com dois ZIPs e três PDFs cria três itens (dois ZIPs e um de PDFs com três `Paths`) e devolve 5 (arquivos, não itens); um PDF já em item pendente sai do item novo; `Kind` e `Paths` preenchidos e presentes na cópia que vai ao frontend.
- **nome da linha:** item de um PDF cujo `processo.txt` já existe termina com `OutputPath` em `processo_2.txt` e `Name` "processo_2"; item de três PDFs termina com `Name` "<nome do TXT> (3 PDFs)"; o broadcast final já leva o nome trocado.
- **saída anterior (e1):** `Reprocessar` passa ao `processar` falso uma `Entrada` com `SaidaAnterior` igual ao `OutputPath` de antes (e `OutputPath` do item zerado na cópia enviada); item restaurado do histórico com `output_path` gravado passa o mesmo valor; `SaidaAnterior` que não passa em `SaidaDoItem` (outra pasta, outro nome) é ignorada e a gravação escolhe nome livre.
- **PDF editado (q1):** `ReprocessarMonitorado` com um item `success` de um PDF só e o mesmo `path` devolve `true`, põe o item em `pending` na mesma posição, passa ao `processar` falso `SaidaAnterior` igual ao `OutputPath` de antes e não cria item novo; sem item na fila, com item `error` ou `cancelled`, ou com o PDF só dentro de um item de vários PDFs, devolve `false` e a fila não muda (o callback então chama `AddFiles`, que cria o item novo).
- **`SaidaDoItem`:** ZIP aceita só `Base(path)`; PDF único aceita `<nome>.txt` e `<nome>_3.txt`; item de três PDFs com número CNJ comum aceita `<CNJ>.txt` e `<CNJ>_2.txt` na pasta do primeiro PDF por evento e recusa `<nome do primeiro PDF>.txt`; saída vazia recusada.
- **marcador do avulso:** documento ausente num item de vários PDFs termina com `Leitura` `ilegivel`, marcador "[ERRO ao abrir PDF: falha de leitura ou gravação em disco]" e nenhum caminho no TXT; documento de 0 bytes termina `ilegivel` com "[ERRO ao analisar PDF: ...]"; o mesmo PDF ausente dentro de ZIP continua com "[ERRO ao extrair PDF do ZIP: ...]".
- **arquivo travado:** PDF aberto com compartilhamento exclusivo (`windows.CreateFile` com `dwShareMode` 0, no teste) num item de vários PDFs termina `ilegivel` com "falha de leitura ou gravação em disco"; sozinho, o item termina em erro com a mesma frase e nenhum caminho.
- **relógio:** com `agoraFormatter` fixo, dois `GenerateConsolidatedOutput` iguais produzem bytes iguais.

`internal/watchdog`: com `MonitorarPDF` desligado, PDF novo não dispara; ligado, dispara um item por PDF; a varredura respeita a saída do PDF (`<nome>_N.txt`) e não a confunde com o TXT do ZIP homônimo; `eprocia_pdf_*.pdf` ignorado; dois PDFs que chegam com segundos de diferença geram dois itens e dois TXTs (e4); dois Write do mesmo PDF depois de o item terminar (TXT mais novo que o PDF) não chamam o callback de novo, e um ZIP nas mesmas condições continua chamando; um PDF salvo de novo depois do TXT (mtime mais novo) chama o callback, que chama `ReprocessarMonitorado` antes de `AddFiles` (callback e fila falsos); `Start` com a mesma pasta e as mesmas extensões não reinicia (o `fsnotify.Watcher` é o mesmo), e com a mesma pasta e extensões diferentes reinicia (a varredura inicial roda de novo).

`internal/config`: `monitorar_pdf` ausente carrega `false`.

Frontend: `avisos.test.mjs` para `separarEntradas` (`.zip`, `.ZIP`, `.pdf`, `.PDF` aceitos; `.txt` e `.docx` ignorados) e para os textos novos de `avisoSoltura`; teste por regex de que `OnFileDrop` usa `separarEntradas` e chama `AddFilesToQueue` uma vez por soltar, e de que nada chama `SelectZipFiles` no `App.svelte`; textos do vazio e da recusa; ícone por `kind`; nome com "(N PDFs)"; interruptor de `monitorar_pdf` em Settings. Capturas da fila com ZIP, PDF e item de vários PDFs lado a lado.

Manual: dez PDFs reais soltos, um a um e todos de uma vez (nativo, digitalizado, protegido), comparados com o TXT do ZIP do mesmo processo (a sessão principal faz, com os ZIPs reais fora do repositório).

## 9. Estimativa

Médio. Go: cerca de 320 linhas de código e 600 de teste (o grosso é o teste de equivalência, o de preservação do original e as regras de nome do pacote `saida`), já com o item de vários PDFs (p3) e o monitoramento de PDF (p2). Frontend: cerca de 80 linhas. Dois a três dias de agente com revisão; as decisões e1 a e5 de 2026-11-20 confirmaram as recomendações e não mudam a estimativa. Antes: um a dois dias; o aumento vem do item de vários PDFs, da regra do `_2` e do interruptor do watchdog, que passou a entrar. A segunda revisão (2026-11-20) somou cerca de 40 linhas de código e 80 de teste (saída anterior no reprocessamento, `SaidaDoItem`, descarte do evento ao vivo de PDF já convertido, `Start` com extensões, relógio do formatter, troca do `Name`) e 20 de frontend; cabe nos mesmos dois a três dias. O pacote `internal/saida` conta só nesta estimativa (a do histórico depende dele). A decisão q1 (2026-11-21) soma cerca de 15 linhas de código e 2 testes na fila (`ReprocessarMonitorado`), e a escolha do wrapper de string para `ProcessZip`/`ProcessZipComAtividade` poupa a reescrita das cerca de 40 chamadas de teste; ambos cabem nos mesmos dois a três dias.

## 10. Decisões do dono

De 2026-11-19:

- **p1.** Nome do TXT de um PDF solto: "3, mas sem sobrescrever, colocando um underline seguido de um número para diferenciar, em caso de já ter arquivo com mesmo nome." Aplicado: `processo.pdf` gera `processo.txt`; ocupado, `processo_2.txt`, `processo_3.txt`...; nunca sobrescreve (seções 1, 2.1, 3 e 4).
- **p2.** A pasta monitorada converte PDFs novos: "Sim, com opção desligada por padrão". Aplicado: `MonitorarPDF`, interruptor em Configurações, padrão desligado (seções 1, 2.1, 2.3 e 4).
- **p3.** Vários PDFs arrastados de uma vez: "Um TXT único juntando todos, por evento". Aplicado: um soltar ou uma seleção vira um item e um TXT, organizado por evento como um ZIP (seções 1, 2.1, 3 e 8).

De 2026-11-20:

- **e1.** PDF solto reprocessado: substitui o TXT que aquele item gerou; não cria um `_N` novo a cada reprocessamento. Aplicado nas seções 1, 2.1 e 8. Se o TXT foi apagado, o reprocessamento escolhe nome livre de novo (consequência direta; a decisão não cobria esse caso).
- **e2.** Nome do TXT único de vários PDFs: o número do processo comum aos PDFs; se não houver um comum, o nome do primeiro PDF na ordem por evento; com a regra `_2`, `_3` se o nome já existir. Aplicado nas seções 1, 2.1, 3 e 8.
- **e3.** Vários PDFs de pastas diferentes num só soltar: um TXT só, na pasta do primeiro PDF. Aplicado nas seções 1, 2.1 e 3.
- **e4.** Pasta monitorada, PDFs que chegam juntos: um TXT por PDF, sem janela de junção. Aplicado nas seções 1 e 2.1.
- **e5.** ZIP e PDF com o mesmo nome na pasta: o ZIP continua como hoje; o PDF vai para `_2` se houver ZIP ao lado. Aplicado nas seções 2.1, 3, 4 e 8.

De 2026-11-21:

- **q1.** PDF da pasta monitorada editado depois de convertido: tratado como reprocessamento (opção 2, a recomendada). Se a fila tem um item concluído de um PDF só com o mesmo `Path`, o TXT dele é substituído; sem item na fila, vale a p1 (`_N`). Aplicado nas seções 1, 2.1, 4 e 8 e na estimativa (seção 9); consulta à fila por `Queue.ReprocessarMonitorado`.

## 11. Resposta à revisão

Todos os achados foram aceitos: P1 (omissão de tipos não vale para o avulso, com teste), P2 (a alternativa só vale com temporário próprio ou memória), P3 (F-VI-04 é F17; urgência levada ao topo da spec e à sessão principal), P4 (HTML também usa `f.Abrir()`), P5 (sem renomear `ProcessZipComAtividade` e sem remover `SelectZipFile`), P6 (`TMP` e `TEMP`), P7 e P8 (`Kind` em `copiarItem`). Nenhuma recusa.

O motivo de P7 (o nome `processo.pdf.txt` se disfarçaria de PDF no Explorer) deixou de se aplicar: o dono escolheu `processo.txt` com a regra do `_2`.

**Segunda revisão sem contexto (2026-11-20),** conferida no código da `main` com a rodada 2026-11-10 juntada. Todos os achados foram conferidos e aceitos:

- P1 (`Reprocessar` zera `OutputPath` em `queue.go:287` e o pipeline só recebe o caminho): campo `saidaAnterior` e `Entrada.SaidaAnterior` (seção 2.1, pipeline e queue), também para item restaurado do histórico; testes na seção 8.
- P2 (eventos atrasados do watchdog gerariam `_2`, `_3`): `handleNewFile` descarta PDF já convertido pela mesma regra da varredura (seções 2.1 e 4; teste na seção 8). O caso do PDF alterado depois de convertido foi decidido pelo dono em 2026-11-21 (q1, seção 10), porque reaproveitar o TXT anterior estende e1.
- P3: "seja qual for a ordem" trocado por "quando o ZIP já existe na pasta" (seção 4).
- P4: vale a frase atual de `textoErroSaida` para o arquivo travado; documento ausente ou vazio num item de vários PDFs termina `ilegivel`, com marcador próprio do avulso (seções 2.1 e 4; testes na seção 8).
- P5: `Start` compara pasta e extensões (seção 2.1, watchdog).
- P6: relógio do formatter injetável (seção 2.1, pipeline).
- P7: `Name` trocado pelo nome real do TXT ao terminar; `AddFiles` devolve arquivos, não itens (seção 2.1, queue).
- P8: referências atualizadas; o "Prazo" do topo e a seção 7 reescritos para a rodada já juntada.
- H1 (cruzado com o histórico): `saida.SaidaDoItem`, usada por `lerSaidaParaCopiar` e pelo histórico.
- H6 (cruzado): `internal/saida` é entregue e estimado aqui (seções 2.1 e 9).
