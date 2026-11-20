# Processar PDF avulso (fora de ZIP)

Data: 2026-11-18. Base lida: worktree `revisao-2026-11-10` (main com a v0.4.2). Revisada por um revisor sem contexto em 2026-11-18 (aprovada com ajustes, todos aplicados ou respondidos na seção 11). Decisões do dono p1 a p3 (2026-11-19) e e1 a e5 (2026-11-20) aplicadas (seção 10); não há pontos em aberto.

Pedido do dono: "a possibilidade de processar arquivos PDF diretamente, sem ser em pastas zip."

**Prazo:** com p1 a p3 respondidas, o bloco 3 da rodada de correção em curso já pode escrever os textos de F4 (arrastar e soltar) e F5 (F-AC-08, botão do vazio) com "ZIP ou PDF" e o filtro das duas extensões (seção 7). As decisões e1 a e5 só mexem no nome e na pasta do TXT, não nesses textos.

## 1. Problema e comportamento esperado

Hoje só entram ZIPs: o arraste filtra `.zip` no frontend (`App.svelte:160`, com `alert` para o resto), os diálogos filtram `*.zip` (`app.go:356-386`), o watchdog só reage a `.zip` (`watchdog.go:155`, `pendentes.go:30`) e o pipeline começa por `ReadZipFiles` (`pipeline.go:300`). Quem baixou do Eproc só a petição inicial, uma sentença ou um laudo, ou recebeu um PDF por e-mail, precisa zipar o arquivo à mão para usar o eprocIA.

Comportamento esperado:

- O usuário arrasta um ou mais PDFs para a janela, ou escolhe pelo botão de selecionar (agora "Selecionar arquivos..." com o filtro "ZIP do Eproc e PDF (*.zip;*.pdf)").
- **Um soltar (ou uma seleção) com vários PDFs vira um item só da fila e um TXT só** (p3), com os PDFs organizados por evento, como os documentos de um ZIP: cada PDF é um `<documento>` dentro do `<evento>` tirado do nome dele, na mesma ordem natural do ZIP. Um soltar com um PDF só é o caso particular de um item com um documento.
- O PDF passa pelo mesmo caminho de um documento de dentro do ZIP: texto nativo, segundo leitor (Poppler), OCR conforme a config, higienização, marcadores de não lido. A saída tem o formato `<contexto>/<evento>/<documento>/<pag>` do PRD §6.
- Exceção: a omissão de tipos (`OmitirTipos`, H10) não vale para o PDF avulso. Quem arrasta um PDF quer aquele PDF, mesmo que o tipo dele (ex.: `PROC1`) esteja na lista de omitidos.
- **Nome do TXT (p1):** `processo.pdf` gera `processo.txt`, na pasta do PDF. Se já existir um arquivo com esse nome (por exemplo, o TXT do ZIP de mesmo nome), gera `processo_2.txt`, depois `processo_3.txt`, e assim por diante. O TXT de um PDF avulso nunca sobrescreve arquivo existente, salvo o reprocessamento, que substitui o TXT que o próprio item gerou (e1). Para vários PDFs, o nome é o número do processo comum aos PDFs ou, se não houver um comum, o nome do primeiro PDF na ordem por evento, com a mesma regra do `_2` (e2); a pasta é a do primeiro PDF nessa ordem, mesmo que os PDFs venham de pastas diferentes (e3).
- Copiar, Abrir pasta, Reprocessar, notificação de fim e contagem de não lidos funcionam igual.
- A linha mostra um ícone diferente para PDF e para ZIP; um item de vários PDFs mostra o nome do TXT e a contagem ("5000000-00.2026.4.00.0001 (3 PDFs)").
- Misturar ZIPs e PDFs no mesmo arraste funciona: cada ZIP vira seu item, e os PDFs daquele arraste juntos viram um item. Outros tipos continuam recusados, com a mensagem reescrita ("Arraste arquivos ZIP do Eproc ou PDF").
- **Pasta monitorada (p2):** interruptor em Configurações, "Também converter PDFs novos nesta pasta", **desligado por padrão**. Ligado, a pasta monitorada pega também os PDFs novos, cada um no seu item e no seu TXT, mesmo que cheguem juntos (e4).

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
- O ZIP mantém o comportamento atual: grava `<nome>.txt` e sobrescreve o próprio TXT.

**`internal/etl/entrada.go` (novo; `zip_reader.go` fica como está):** `LerEntrada(caminhos []string) ([]models.ProcessFile, io.Closer, error)`. Um caminho `.zip` (sem diferença de caixa) chama `ReadZipFiles`; uma lista só de `.pdf` devolve um `ProcessFile` com `Avulso: true` por PDF e um `Closer` vazio; mistura, lista vazia ou outra extensão devolvem "tipo de arquivo não suportado". Não confere a assinatura `%PDF`: um PDF em envelope PKCS#7 não começa com ela e o `abrirPDF` (`abertura.go`) já o encaminha ao Poppler; um arquivo que não é PDF vira documento `ilegivel` pelo caminho normal.

**`internal/etl/pipeline.go`:**

- `ProcessZipComAtividade` mantém o nome e passa a receber o item (ou a lista de caminhos dele), trocando `ReadZipFiles(zipPath)` por `LerEntrada(caminhos)`. `rc.RegisterDecompressor` só se aplica quando a entrada é ZIP (o `Closer` é o `*zip.ReadCloser` nesse caso).
- `extrairPDF` usa `f.Abrir()` e `f.Tamanho` em vez de `ZipFileRef.Open` e `UncompressedSize64`. O caminho do HTML (`pipeline.go:683`) também passa a usar `f.Abrir()`, para não sobrarem dois jeitos de abrir. A conferência de "entrada maior que o tamanho declarado" vale também para o avulso, com a frase "o arquivo mudou durante a leitura" (o usuário pode estar salvando por cima).
- A omissão de tipos (`tipoOmitido`, `pipeline.go:251-256`, aplicada em `:346-348`) é pulada quando `f.Avulso`.
- A mensagem "no PDF or HTML documents found in ZIP" continua só para ZIP.
- A gravação usa `saida`: ZIP em `Base`, como hoje; item de PDF em `GravarSemSobrescrever(NomeLote(...))`, salvo no reprocessamento (e1: o reprocessamento grava por cima do `OutputPath` que o próprio item gerou, se ele ainda existir; se não existir, escolhe nome livre de novo).

**`internal/etl/queue.go`:**

- `AddFiles` valida a extensão no Go (hoje só o frontend filtra, e o binding `AddFilesToQueue` aceita qualquer caminho vindo do WebView): aceita `.zip` e `.pdf`, sem diferença de caixa; o resto é descartado com log. Cada `.zip` vira um item; **todos os `.pdf` da mesma chamada viram um item só** (uma chamada é um soltar ou uma seleção; o watchdog chama com um caminho por vez). A deduplicação por `Path` continua igual para ZIP; um PDF que já está num item `pending`/`processing` sai do item novo, e um item novo que fique sem PDF não é criado.
- `QueueItem` ganha `Kind string` (`json:"kind"`, `"zip"` ou `"pdf"`) e `Paths []string` (`json:"paths,omitempty"`, os PDFs do item, na ordem por evento; vazio para ZIP). `Path` continua sendo o ZIP ou, no item de PDF, o primeiro PDF da ordem, para os usos atuais (log, título, deduplicação). `Name` do item de PDF é o nome do TXT sem extensão, com " (N PDFs)" quando N > 1. Os dois campos são copiados em `copiarItem` (`queue.go:232-254`). Se o histórico existir, `Restaurar` deriva `Kind` do `Path` e guarda `Paths`.
- `Reprocessar` de um item de PDF relê todos os `Paths`; se algum não existir mais, devolve "arquivo de origem não encontrado: <nome do PDF>" e não muda a fila.

**`app.go`:** `SelectFiles` (novo) com os filtros "ZIP do Eproc e PDF (*.zip;*.pdf)", "Somente ZIP (*.zip)" e "Somente PDF (*.pdf)", com seleção múltipla. `SelectZipFiles` e `SelectZipFile` ficam como estão; a limpeza dos bindings legados não faz parte deste recurso. `lerSaidaParaCopiar` passa a aceitar a saída de item com sucesso só se `saida.DaEntrada(Path, OutputPath)` (no item de PDF de vários arquivos, conferido contra `NomeLote(Paths)`).

**`internal/watchdog`:**

- `watchLoop` e `pendingZips` aceitam `.pdf` quando `Config.MonitorarPDF` estiver ligado. `pendingZips` passa a receber a lista de extensões e usa `saida`. Para ZIP, a regra continua: convertido se houver `<nome>.txt` igual ou mais novo. Para PDF: convertido se houver `<nome>_N.txt` igual ou mais novo, ou `<nome>.txt` igual ou mais novo quando não houver `<nome>.zip` ao lado (com o ZIP homônimo, `<nome>.txt` é do ZIP). Ignora nomes que começam com `eprocia_pdf_` (os temporários do pipeline, caso alguém monitore `%TEMP%`).
- Cada PDF novo detectado vira um item próprio, com seu TXT, sem juntar PDFs que cheguem perto um do outro (e4).
- O `Watcher` recebe a lista de extensões em `Start`; mudar a opção em Configurações reinicia o watcher, como já acontece ao trocar de pasta.

### 2.2 Bindings

`SelectFiles` e os campos `kind` e `paths` em `QueueItem`; rodar `wails generate module`.

### 2.3 Frontend

- `App.svelte:157-167` (`OnFileDrop`): aceita `.zip` e `.pdf` e manda todos os aceitos numa chamada só de `AddFilesToQueue` (é o que faz o Go juntar os PDFs num item); texto da recusa pelo componente de aviso de F3.
- `App.svelte:306` e o botão do painel: chamar `SelectFiles`; rótulo "Selecionar arquivos...".
- `ProcessList.svelte`: ícone por `kind` (`Icone.svelte` ganha o de documento, se ainda não tiver); nome com a contagem de PDFs; texto do vazio (`ProcessList.svelte:327`) "Arraste arquivos ZIP do Eproc ou PDF em qualquer lugar...".
- `Settings.svelte`: interruptor "Também converter PDFs novos nesta pasta" sob a pasta monitorada, desligado por padrão, com a frase "Cada PDF que aparecer na pasta ganha um TXT ao lado, inclusive boletos e outros PDFs que não são de processo".
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
- **Arquivo de 0 bytes, movido ou apagado entre o arraste e o processamento:** num item de um PDF, erro do item ("arquivo vazio" ou "arquivo não encontrado"), sem TXT. Num item de vários PDFs, aquele documento fica não lido com a frase fixa e os outros seguem; se todos falharem, erro do item, sem TXT.
- **Nome do TXT ocupado:** `_2`, `_3`... (p1). Esgotado o `_999`, erro do item "não há nome livre para o TXT".
- **PDF com o mesmo nome de um ZIP na mesma pasta:** o ZIP fica com `<nome>.txt` e o PDF vai para `<nome>_2.txt` (e5), seja qual for a ordem em que chegam. Limitação: se o PDF for convertido numa pasta onde o ZIP homônimo ainda não existe, ele fica com `<nome>.txt`, e um ZIP de mesmo nome que chegue depois sobrescreve esse TXT (o ZIP mantém o comportamento atual) e pode ser pulado pela varredura de abertura do monitoramento, que acha `<nome>.txt` mais novo. O caso exige baixar a peça avulsa antes do processo inteiro, com o mesmo nome; fica registrado.
- **Dois PDFs de pastas diferentes com o mesmo nome no mesmo soltar:** viram dois `<documento>` com o mesmo nome no TXT, como duas entradas de mesmo caminho num ZIP (as chaves do mapa `eventos` já levam o índice).
- **Arquivo apagado ou trocado entre a pré-contagem e o worker** (o PDF é lido duas vezes, como dentro do ZIP): a segunda leitura falha ou vem com tamanho diferente; o documento termina com "falha de leitura ou gravação em disco" (`textoErroSaida`). Alternativa a avaliar no plano: para o avulso, manter a `fontePDF` da pré-contagem até o worker, como já se faz no caso Poppler (`manterSoDisco`), e ler o arquivo uma vez só. **Essa alternativa só vale com um temporário próprio ou com os bytes em memória; nunca com o caminho do usuário em `fontePDF.caminho`**, pelo risco da seção 2.1.
- **PDF aberto em outro programa:** o Acrobat abre com compartilhamento de leitura, e `os.Open` funciona. Um programa que trava o arquivo com exclusividade faz o documento terminar como não lido (ou o item em erro, se for o único) com "o arquivo está em uso por outro programa".
- **Usuário apagando o PDF durante o processamento:** enquanto o handle está aberto, o Windows pode recusar apagar ou renomear o arquivo (conferir o modo de compartilhamento do `os.Open` na versão do Go do `go.mod`); por isso a extração fecha o handle assim que termina de copiar (já é o padrão de `extrairPDF`, com `defer Close`).
- **O original nunca é alterado nem apagado,** inclusive com panic, cancelamento, prazo do OCR estourado e caminho do Poppler.
- **PDF muito grande** (centenas de MB): vai para temporário em `%TEMP%`, como dentro do ZIP; o disco precisa do espaço da cópia.
- **PDF na nuvem** (OneDrive com arquivos sob demanda): a leitura dispara o download; pode demorar, sem tratamento especial.
- **Extensão em maiúsculas (`.PDF`):** aceita.
- **Pasta monitorada com a opção ligada:** navegadores baixam como `.crdownload` ou `.part` e renomeiam no fim; o watchdog já trata o Rename e espera o tamanho estabilizar. PDFs que não são de processo (boletos, contracheques) também são convertidos e ganham um TXT ao lado, com OCR rodando à toa: por isso a opção nasce desligada e a frase do interruptor avisa. Com a opção desligada, nada muda em relação a hoje.
- **`falhas.json`** (`watchdog.Falhas`) funciona por caminho e vale para PDF sem mudança.

## 5. Segurança e privacidade

- Nada sai da máquina; nenhum dado novo é guardado.
- A validação de extensão passa para o Go (`AddFiles`), em vez de depender só do filtro do frontend.
- O TXT de um PDF avulso tem as mesmas garantias do TXT de ZIP: nenhum caminho, stderr ou dump de objeto PDF (`textoErroSaida`).
- A regra de nunca sobrescrever usa `O_EXCL`, então o eprocIA não apaga um `.txt` do usuário que por acaso tenha o nome do PDF.

## 6. Conflito com o RNF01

Nenhum.

## 7. Conflito com a rodada de correção

Grupos conforme `docs/revisao/2026-11-18-plano-correcao.md` no commit 05e7e7f:

- **F4** (F-AN-01, bloco 3): reescreve o arrastar e soltar (overlay só para arquivos e confirmação do aceito) no mesmo trecho que este recurso muda (`App.svelte:155-169` e `369-389`). Com p1 a p3 respondidas, F4 já pode aceitar `.zip` e `.pdf` e mandar os aceitos numa chamada só.
- **F3** (F-AC-07, bloco 2, já feita): tirou o `alert()` que respondia ao arraste de não-ZIP; a mensagem de recusa vai pelo componente de aviso.
- **F5** (F-AC-08, F-AC-03, F-AC-09, bloco 3): F-AC-08 cria o botão do vazio e a frase curta; F5 fixa o vocabulário. Recomendação: F5 já escrever "Selecionar arquivos..." e "ZIP do Eproc ou PDF", para não refazer os textos.
- **F17** (F-VI-04, inventário de ícones, bloco 4): o ícone de PDF entra no mesmo inventário.

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
- **queue:** `AddFiles` recusa `.txt`, `.exe`, `.docx` e aceita `.pdf` e `.PDF`; uma chamada com dois ZIPs e três PDFs cria três itens (dois ZIPs e um de PDFs com três `Paths`); um PDF já em item pendente sai do item novo; `Kind` e `Paths` preenchidos e presentes na cópia que vai ao frontend.

`internal/watchdog`: com `MonitorarPDF` desligado, PDF novo não dispara; ligado, dispara um item por PDF; a varredura respeita a saída do PDF (`<nome>_N.txt`) e não a confunde com o TXT do ZIP homônimo; `eprocia_pdf_*.pdf` ignorado; dois PDFs que chegam com segundos de diferença geram dois itens e dois TXTs (e4); mudar a opção reinicia o watcher.

`internal/config`: `monitorar_pdf` ausente carrega `false`.

Frontend: teste por regex de que `OnFileDrop` aceita `.pdf` e chama `AddFilesToQueue` uma vez por soltar; textos do vazio e da recusa; ícone por `kind`; nome com "(N PDFs)"; interruptor de `monitorar_pdf` em Settings. Capturas da fila com ZIP, PDF e item de vários PDFs lado a lado.

Manual: dez PDFs reais soltos, um a um e todos de uma vez (nativo, digitalizado, protegido), comparados com o TXT do ZIP do mesmo processo (a sessão principal faz, com os ZIPs reais fora do repositório).

## 9. Estimativa

Médio. Go: cerca de 280 linhas de código e 520 de teste (o grosso é o teste de equivalência, o de preservação do original e as regras de nome do pacote `saida`), já com o item de vários PDFs (p3) e o monitoramento de PDF (p2). Frontend: cerca de 60 linhas. Dois a três dias de agente com revisão; as decisões e1 a e5 de 2026-11-20 confirmaram as recomendações e não mudam a estimativa. Antes: um a dois dias; o aumento vem do item de vários PDFs, da regra do `_2` e do interruptor do watchdog, que passou a entrar.

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

## 11. Resposta à revisão

Todos os achados foram aceitos: P1 (omissão de tipos não vale para o avulso, com teste), P2 (a alternativa só vale com temporário próprio ou memória), P3 (F-VI-04 é F17; urgência levada ao topo da spec e à sessão principal), P4 (HTML também usa `f.Abrir()`), P5 (sem renomear `ProcessZipComAtividade` e sem remover `SelectZipFile`), P6 (`TMP` e `TEMP`), P7 e P8 (`Kind` em `copiarItem`). Nenhuma recusa.

O motivo de P7 (o nome `processo.pdf.txt` se disfarçaria de PDF no Explorer) deixou de se aplicar: o dono escolheu `processo.txt` com a regra do `_2`.
