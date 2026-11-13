# Ler os PDFs que a biblioteca recusa e avisar quando um documento fica de fora

* **Author:** jonyduque (redigido pelo Claude Code a partir de seis revisões independentes)
* **Date:** 2026-11-11

## Overview

O eprocIA lê cada PDF de um ZIP do Eproc com a biblioteca `github.com/dslipak/pdf` v0.0.2. Em três ZIPs reais (144 PDFs), 16 PDFs (11%) não abrem nessa biblioteca. O documento inteiro fica fora do `.txt`: no lugar entra uma linha `[ERRO ao analisar PDF: ...]` com o texto da biblioteca em inglês, e o item da fila termina como "sucesso", com o mesmo visto verde de um processo lido por inteiro.

Quem usa o aplicativo cola o `.txt` em um LLM para analisar o processo. Uma peça que some sem aviso é o pior resultado possível para esse uso: a análise sai incompleta e ninguém percebe.

Este documento reúne o que seis revisores independentes (dívida técnica, simplicidade, custo, experiência de uso, compatibilidade e pesquisa externa) levantaram sobre como resolver. Eles receberam só contagens e classes de falha, nenhum conteúdo de processo.

A proposta que sai da análise tem duas partes:

1. **Avisar.** O item, a notificação, o painel e o próprio `.txt` passam a dizer quantos documentos não foram lidos e por quê, em português.
2. **Ler.** Quando a biblioteca não abre um PDF, o aplicativo usa o Poppler, que ele já embute para o OCR, como segundo leitor: `pdfinfo.exe` conta as páginas, `pdftotext.exe` extrai o texto nativo, e as páginas sem texto vão para o OCR que já existe.

## Analysis

### O que está errado hoje, antes de qualquer solução

O defeito mais grave não é a biblioteca recusar arquivos. É o aplicativo esconder isso.

- O item termina com `Status = "success"` mesmo com documentos em erro (`internal/etl/queue.go:544`). A linha da lista, a notificação "pronto" e o botão Copiar são iguais aos de um processo completo. O "N com erro" só aparece dentro do painel de atividade, que fica fechado.
- O `.txt` não avisa. O `<contexto>` não traz contagem de falhas, o `<summary>` lista o documento não lido como "1 página", e o marcador no corpo carrega a mensagem da biblioteca em inglês (`internal/etl/pipeline.go:577`). Um LLM que recebe esse texto não tem como saber que falta uma peça.
- Uma página digitalizada processada com o OCR desligado sai vazia e o documento conta como concluído. É a mesma falha silenciosa por outro caminho.
- Nenhum processo externo tem prazo. `pdftoppm` e `tesseract` rodam com `exec.CommandContext` (`internal/ocr/tesseract.go:239` e `:349`) recebendo só o contexto de cancelamento. Um `pdftoppm` preso em um arquivo malformado segura uma vaga do OCR e, como a fila é sequencial, o aplicativo inteiro. Qualquer solução que mande mais arquivos malformados ao Poppler aumenta esse risco.
- A biblioteca é aberta em dois pontos: na pré-contagem de páginas (`pipeline.go:28`) e de novo no parser (`pdf_parser.go:333`). O conserto precisa valer nos dois, ou o progresso fica errado. Hoje um PDF que não abre conta como 1 página.

Esses pontos existem desde a v0.2.0. A v0.4.0 só tornou as falhas visíveis no painel; por isso pareceu regressão.

### O que foi medido

| Classe | Quantos | O que a biblioteca devolve | Têm texto nativo |
|---|---|---|---|
| Cabeçalho `%PDF-2.0` | 8 | `not a PDF file: invalid header` | 1 (7 páginas); os outros 7 são digitalizações de 1 página |
| Envelope PKCS#7/CAdES (o PDF começa no byte 52 ou 53) | 3 | `invalid header` | 3 |
| Tabela de objetos inconsistente | 4 | pânico `loading {N 0}: found {N-1 0}` ao contar páginas | 0 (digitalizações de 2 a 6 páginas) |
| Criptografia `V=4` (AESV2, sem senha de abertura) | 1 | `unsupported PDF: encryption version V=4` | 1 (4 páginas) |

Medido nesta sessão:

- O `pdftoppm.exe` embutido (Poppler 25.07.0) renderizou a página 1 dos 16 arquivos.
- Um `pdftotext` de outra origem (xpdf 4.06) leu os 16, entre 60 e 160 ms por arquivo. Dos 16, 11 não têm texto nativo e precisam de OCR em qualquer solução; 5 têm texto nativo (19 páginas no total).
- Trocando só os bytes do cabeçalho de `2.0` para `1.7`, a biblioteca abre os 8 da primeira classe. Recortando a partir de `%PDF-`, ela abre os 3 envelopes e conta as páginas.
- O pacote oficial do Poppler 25.07.0 para Windows traz `pdftotext.exe` (70.656 bytes) e `pdfinfo.exe` (65.024 bytes). Os arquivos `pdftoppm.exe`, `poppler.dll`, `freetype.dll` e `zlib.dll` desse pacote têm o mesmo SHA-256 dos embutidos em `internal/ocr/embed/poppler`, e as duas pastas têm 26 DLLs. É o mesmo build: os dois executáveis novos usam as DLLs que já são distribuídas.
- O `pdfinfo.exe` e o `pdftotext.exe` desse pacote foram rodados nos 144 PDFs em 2026-11-11, com autorização do dono (só contagens). Leram os 16 recusados, entre 48 e 95 ms por arquivo, e os outros 128 (762 páginas; cerca de 77 ms no `pdfinfo` e 86 ms no `pdftotext` por arquivo), sem falha. O resultado repete o do xpdf: 11 sem texto nativo (os 7 avisos de recebimento em PDF 2.0 e os 4 do pânico de xref) e 5 com texto (1 em PDF 2.0 com 7 páginas, os 3 envelopes lidos sem desembrulhar e o AESV2 com 4 páginas).
- Nos 3 envelopes, o PDF está num único OCTET STRING primitivo, e o arquivo acaba logo depois do `%%EOF`. O recorte a partir de `%PDF-` não deixa cabeçalhos de pedaço no meio do PDF.

Não medido, e por isso tratado como hipótese:

- A causa do defeito de xref dentro dos 4 arquivos do pânico. Sem ela, ainda não há como gerar um PDF sintético com o mesmo defeito para os testes.
- A extração de texto pela biblioteca depois do recorte do envelope não foi medida (só a abertura e a contagem de páginas). Com o Poppler como segundo leitor, ela deixa de ser necessária.
- A causa da inconsistência na tabela de objetos dos 4 arquivos não foi investigada.
- As referências de linha vieram dos revisores. As citadas neste documento foram conferidas em 2026-11-11; as demais devem ser conferidas antes de uso.

### Onde os revisores concordam

- **O aviso é necessário em qualquer cenário.** Sempre haverá PDF que nenhum leitor abre (protegido por senha, truncado). Enquanto o item terminar igual a um perfeito, o problema volta com outra classe de arquivo.
- **Não criar um status novo.** Um status "sucesso com avisos" quebra a notificação de fim (`avisarFimLocked`, `queue.go:160`), o texto da notificação e o Copiar (`app.go`), o mapeamento de estado do frontend (`frontend/src/lib/estado.js`) e passaria a gravar o ZIP em `falhas.json`. O caminho é manter `"success"` e acrescentar ao item um contador de documentos não lidos.
- **Não trocar a biblioteca agora.** Nenhuma alternativa em Go resolve as quatro classes sem custo alto (ver Alternatives Weighed).
- **Não passar todos os PDFs pelo Poppler.** Mudaria o texto dos 128 PDFs que hoje funcionam, obrigaria a gravar todo PDF em disco e não aposentaria a biblioteca, que ainda decide se a página tem imagem grande ou é desenhada em vetores.
- **Os arquivos `.txt` já gerados não se corrigem sozinhos.** O ZIP processado não volta para a fila. As notas da versão precisam ensinar a procurar `[ERRO ao analisar PDF` nos `.txt` antigos e a adicionar o ZIP de novo.
- **Os testes podem ser sintéticos.** Dá para gerar, no próprio código de teste em Go, um PDF com cabeçalho 2.0, um envelope em volta de um PDF e um PDF com AESV2 (md5, rc4 e aes da biblioteca padrão). O CI já roda o OCR real.

### Onde discordam

**1. Qual mecanismo de leitura.**

- A lente de simplicidade defende só o OCR do documento inteiro quando a biblioteca falha: 60 a 90 linhas, nenhum executável novo, encaixado no ponto em que `abrir()` falha em `ParsePages`. Sem `pdfinfo`: renderiza tudo e conta os PNGs.
- As lentes de custo e de dívida técnica respondem que a contagem de páginas precisa existir na pré-varredura, antes de renderizar, ou o progresso mostra "página 7 de 1"; e que o OCR do documento inteiro troca texto exato por reconhecimento em 5 documentos (19 páginas) que têm texto nativo.
- A pesquisa externa propõe o Poppler como segundo leitor (`pdfinfo` + `pdftotext`, e OCR só nas páginas com pouco texto).

Posição deste documento: o segundo leitor. O ganho medido sobre o OCR puro é pequeno em número (5 documentos em 144), mas esses documentos são petições e um documento pessoal, em que um nome ou número reconhecido errado pelo OCR tem consequência. Com o OCR desligado, o OCR puro não recupera nada; o segundo leitor recupera os 5. O custo é 136 KB a mais no instalador e dois executáveis do mesmo build já distribuído. Contra: é mais código que a opção mínima, e a regra "página com pouco texto vai ao OCR" é mais grosseira que a atual, que olha imagens e vetores.

**2. Consertar antes da biblioteca (cabeçalho e envelope) ou não.**

- As lentes de custo e de dívida técnica põem esse conserto em primeiro lugar: mantém 11 dos 16 no caminho normal, em processo, com as mesmas regras de OCR por página dos outros PDFs.
- A lente de simplicidade aceita o do envelope e pede medição antes do de cabeçalho.

Posição deste documento: com o segundo leitor, esse conserto deixa de ser necessário para ler os arquivos e passa a ser uma otimização. O de cabeçalho é barato (cerca de 15 linhas no ponto único que alimenta as duas aberturas) e pode entrar depois, se a medição mostrar diferença de texto ou de tempo. O de envelope fica de fora: é a parte com mais risco (um leitor DER de cerca de 60 linhas, com o caso do conteúdo em pedaços) e o Poppler já lê o envelope sem recorte.

**3. O aviso primeiro ou depois.**

- A lente de simplicidade adia o aviso: se os 16 passam a ser lidos, o aviso perde urgência.
- As lentes de experiência de uso e de dívida técnica tratam como bloqueio.

Posição deste documento: o aviso entra primeiro. Ele é o que protege o usuário das classes que ainda não foram vistas, e é pré-requisito do segundo leitor: sem um resultado por documento com mais que "erro ou não", não há onde registrar "lido pelo segundo leitor" ou "digitalizado, OCR desligado".

**4. Quanto reestruturar.**

- A lente de dívida técnica pede abertura única, os tipos `pdf.*` atrás de um adaptador e a origem das páginas separada do despacho de OCR (hoje closures dentro de `ParsePages`).
- A lente de simplicidade diz que o encaixe cabe sem isso.

Posição deste documento: reestruturar só o que o segundo leitor exige. Uma função única decide "biblioteca ou Poppler" e é chamada nos dois pontos de abertura; o despacho de OCR sai das closures para poder ser usado pelos dois leitores. O adaptador completo sobre `pdf.*` fica para quando houver um terceiro leitor.

**5. Ordem de lançamento.**

A lente de compatibilidade recomenda publicar a v0.4.0 como está e lançar o conserto como v0.4.1, ensaiada antes com uma tag `v0.4.1-rc1`. A v0.4.0 não piora nada em relação à v0.3.0 e já mostra as falhas no painel. O conserto proposto aqui mexe no formato do `.txt`, na interface e nos binários embutidos; é maior que um remendo e não deveria segurar o que já está pronto. Isso inverte a recomendação anterior da decisão d2 ("corrigir agora"), feita antes desta análise.

O dono decidiu o contrário em 2026-11-11: segurar a v0.4.0, que fica como rascunho, e lançar só com o conserto, como v0.4.1.

### Requisitos que saem da análise

- **Prazo em todo processo externo.** Cada chamada a `pdftoppm`, `tesseract`, `pdfinfo` e `pdftotext` recebe um prazo proporcional ao número de páginas. Estourar o prazo vira erro daquele documento, não trava da fila.
- **Contagem de páginas na pré-varredura.** Quando a biblioteca falha, `pdfinfo` conta. O total do progresso fica certo desde o início.
- **Resultado por documento com motivo classificado.** Classes iniciais: lido; lido pelo segundo leitor; texto vindo só de OCR; digitalizado com OCR desligado; protegido por senha; arquivo ilegível; prazo estourado. Cada classe tem uma frase em português e, quando existe, uma ação ("Reprocessar com OCR").
- **O `.txt` avisa.** `Documentos não lidos: N` no `<contexto>`; `NÃO LIDO` no `<summary>` no lugar de "1 página"; no corpo, `[documento não lido: motivo em português; consulte o PDF original]`. Quando tudo falha, o marcador `[ERRO ...]` atual continua valendo (há teste que o fixa). O PRD §6 é atualizado junto.
- **A interface avisa sem abrir o painel.** Ícone de alerta e "2 não lidos" na linha do item, na notificação de fim e ao copiar. Documento recuperado por OCR aparece só no painel.
- **A saída dos PDFs que hoje funcionam não muda.** O teste de aceitação compara o `.txt` dos 128 PDFs antes e depois, byte a byte.
- **Blocos de OCR com teto de páginas.** Hoje cada bloco renderiza todos os PNGs a 150 dpi antes do `tesseract`. Um documento grande que cai inteiro no OCR não pode encher o disco temporário.
- **Binários novos do mesmo build.** `pdfinfo.exe` e `pdftotext.exe` vêm do pacote 25.07.0 já usado, sem trocar DLLs. A pasta `%TEMP%/eprocIA_bin` é única para todas as versões instaladas e é regravada quando o carimbo do build muda; duas versões abertas ao mesmo tempo disputam os arquivos. As notas da versão pedem para fechar outras cópias antes de atualizar.
- **`falhas.json` e o status dos itens não mudam de formato.**

### Como medir que deu certo

- Nos três ZIPs de referência, com OCR ligado: 0 documentos não lidos (hoje 16). Com OCR desligado: os 5 com texto nativo lidos e os 11 digitalizados marcados como "digitalizado; o OCR está desativado", contados no item e no `.txt`.
- Um PDF sintético de cada classe nos testes, mais um protegido por senha e um truncado, que precisam terminar como "não lido" com o motivo certo.
- Um processo externo preso de propósito no teste encerra no prazo e a fila segue.

## Goals vs. Non-Goals

* **In-Scope Goals:**
  * O item, a notificação e o `.txt` dizem quantos documentos não foram lidos e por quê.
  * As quatro classes medidas passam a ser lidas: texto nativo onde existe, OCR nas páginas digitalizadas.
  * Página digitalizada com o OCR desligado deixa de sair vazia sem aviso.
  * Prazo em todos os processos externos.
  * Contagem de páginas e progresso corretos para os PDFs lidos pelo segundo leitor.
  * Notas da versão que ensinam a refazer os `.txt` antigos.
* **Out-of-Scope Non-Goals:**
  * Abrir PDF protegido por senha de abertura. Ele continua não lido, agora com aviso.
  * Reprocessar sozinho os ZIPs já processados por versões anteriores.
  * Trocar o extrator de texto dos PDFs que a biblioteca já abre.
  * Criar um status novo para o item da fila.
  * Botão "Copiar diagnóstico" por documento (classe, versão do cabeçalho, tamanho, produtor, erro, versão do aplicativo, sem nome de arquivo nem conteúdo). A ideia é boa para relatos de erro, mas não é necessária para esta entrega.
  * Adaptador completo sobre os tipos da biblioteca.

## Alternatives Weighed

- **Só o OCR do documento inteiro quando a biblioteca falha (opção B sozinha).** É a menor mudança e não acrescenta binário. Rejeitada como solução única: troca texto exato por reconhecimento em 5 documentos com texto nativo, não recupera nada com o OCR desligado e, sem `pdfinfo`, o progresso fica errado até o fim do documento. Continua sendo a alternativa viável se o dono preferir o menor código possível.
- **Consertar cabeçalho e envelope antes da biblioteca como solução principal (opção A).** Resolve 11 de 16 e mantém o caminho em processo. Rejeitada como principal: deixa 5 de fora (os 4 de tabela inconsistente e o criptografado) e exige um leitor DER próprio para o envelope. O conserto de cabeçalho fica como otimização posterior, condicionado a medição.
- **Poppler para todos os PDFs (opção C total).** Rejeitada: muda a saída dos 128 que funcionam, grava todo PDF em disco, não aposenta a biblioteca (as decisões de OCR por página dependem dela) e transforma uma falha de extração dos binários em `%TEMP%`, que hoje só degrada o OCR, em falha total.
- **Trocar a biblioteca por outra em Go (opção D).** `ledongthuc/pdf` tem os mesmos defeitos (mesma origem). `pdfcpu` não extrai texto. `rsc/pdf` está arquivada. `unipdf` é comercial. MuPDF (`go-fitz`) é AGPL e exige cgo, que o build não usa. Rejeitada.
- **PDFium via WebAssembly (`go-pdfium`, MIT, sem cgo).** Tecnicamente a mais completa: texto estruturado e contagem de objetos por página. Rejeitada agora: acrescenta 5,8 MB, é cerca de 2 vezes mais lenta que a versão nativa e obriga a reescrever o extrator e as regras de OCR por página. Fica como candidata se a `dslipak/pdf`, sem commits desde janeiro de 2024, passar a custar mais do que as guardas atuais.
- **Fork da `dslipak/pdf` com os consertos.** Rejeitada: um mantenedor só assumiria uma biblioteca abandonada, e o espelho do lexer (`pdf_lexer.go`) já mostra o custo de acompanhar o código dela.
- **Reparar com `qpdf` antes de ler (opção E).** Conserta as quatro classes para a biblioteca, mas reescreve cada arquivo em disco, acrescenta outro binário e outra passada. O Poppler já embutido lê os mesmos arquivos sem reparo. Rejeitada.
- **Status novo "sucesso com avisos".** Rejeitada pelos efeitos em notificação, cópia, estado do frontend e `falhas.json`. Substituída por um contador no item.
- **`pdfimages -list` para decidir o OCR no segundo leitor.** Daria ao caminho do Poppler a mesma regra de "imagem grande" do caminho normal. Adiada: a regra "pouco texto vai ao OCR" basta para os casos medidos, e o custo de errar é um OCR desnecessário.

## Key Decisions & Staging for Next Steps

Decisões tomadas nesta etapa. O dono aprovou as de número 1 e 2 em 2026-11-11 ("Aviso de não lidos + Poppler como segundo leitor") e mudou a 5; as de número 3 e 4 são posição do redator:

1. O aviso de documentos não lidos entra antes da leitura.
2. O mecanismo de leitura é o Poppler como segundo leitor, só quando a biblioteca falha.
3. O item mantém `Status = "success"` e ganha um contador de não lidos.
4. Reestruturação mínima: uma função de abertura para os dois pontos e o despacho de OCR fora das closures.
5. A v0.4.0 não é publicada (fica como rascunho). O conserto sai como v0.4.1, ensaiada com `v0.4.1-rc1`. Decisão do dono.

Decisões que dependem do dono:

- Embutir `pdfinfo.exe` e `pdftotext.exe` em `internal/ocr/embed/` (o hook `proteger-gerados` protege a pasta).
- Se o formato do `.txt` pode ganhar a contagem de não lidos e o marcador em português (muda o PRD §6).
- O que fazer com o rascunho da v0.4.0 no GitHub (apagar ou deixar até a v0.4.1).

Etapas propostas:

0. **Conferir as hipóteses.** Feito em 2026-11-11 para o Poppler (leu os 144 PDFs) e para os envelopes (um OCTET STRING primitivo). Falta investigar a tabela de objetos dos 4 arquivos do pânico o suficiente para gerar um PDF sintético com o mesmo defeito.
1. **Aviso.** Resultado por documento com motivo classificado; contador no item; `<contexto>`, `<summary>` e marcador no `.txt`; linha do item, notificação e cópia; página digitalizada com OCR desligado marcada. PRD §6 atualizado.
2. **Prazo nos processos externos** e teto de páginas por bloco de OCR.
3. **Segundo leitor.** `pdfinfo.exe` e `pdftotext.exe` embutidos; função única de abertura usada na pré-contagem e no parser; páginas com pouco texto vão ao OCR existente; testes sintéticos por classe; comparação byte a byte dos PDFs que já funcionam.
4. **Opcional, depois de medir:** tratar o cabeçalho `%PDF-2.x` antes da biblioteca.
5. **Lançamento.** Notas com: defeito presente desde a v0.2.0; como achar e refazer os `.txt` antigos; o OCR pode errar; sem OCR, digitalizações ficam marcadas como não lidas; fechar outras cópias antes de atualizar.

Ao concluir, o registro das quatro classes sai da memória da sessão e vai para as "Known traps" do `CLAUDE.md`.

**Nota de 2026-11-13.** O dono decidiu avisar também o documento sem texto (`sem_texto`, "nenhum texto encontrado"): em branco, ou digitalizado em que o OCR não achou texto. Isso revoga a regra "documento em branco conta como lido".
