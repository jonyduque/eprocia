# Relatório do processamento, forçar OCR, log e telemetria local

Data: 2026-11-22. Autor: jonyduque (redigido pelo Claude Code). Estado: proposta revisada (uma revisão sem contexto já ajustou o texto), aguardando as respostas da seção "Perguntas para o dono".

## Por que

O achado M5 da revisão diz que um documento com algumas páginas sem OCR conta como lido, sem aviso. O dono questionou o enquadramento: página sem OCR não é necessariamente erro. Se a página não tem imagem, ela não vai ao OCR, e isso é normal. O que importa ao usuário é saber, ao fim de cada ZIP, quanto foi lido de cada jeito, o que ficou de fora e o que a limpeza economizou.

O dono pediu, ao pé da letra (2026-11-22):

1. Mostrar estatísticas finais com páginas com OCR e sem, a redução conseguida pela limpeza etc., com opção de forçar OCR em tudo.
2. Um arquivo de log bem completo, com os arquivos processados e os detalhes para depurar, e dados de telemetria para melhorar o programa depois. Nada é enviado para lugar algum: o dono só analisa os próprios logs e telemetrias.

Tudo continua local (PRD RNF01).

## O que existe hoje (levantamento)

**Log.** Todo o registro é `log.Printf` da biblioteca padrão (cerca de 112 chamadas `log.Printf`/`Println`/`Fatal` fora dos testes), sem destino configurado: em `main.go` só há `log.Fatal` (`main.go:80`) e uma linha da trava de instância (`main.go:29`), e nenhum `log.SetOutput`. Num executável Wails sem console, a saída vai para o stderr e se perde. Ou seja, **não há arquivo de log**. As chamadas estão em `app.go:91,101,107,134,164,270,294`, `pipeline.go:390-405,583,612`, `pdf_parser.go:108-208,366-397`, `pdf_ocr_despacho.go:123-211`, `pdf_poppler.go:29-89`, `abertura.go:70-92`, `queue.go:297` e outras. Três problemas: (1) o `pdf_parser.go` grava **cerca de três linhas por página, sem condição** (`:160-200`, `:380-397`), com o caminho do PDF (`:129,134,165`), que é um arquivo temporário sob o perfil do usuário do Windows; (2) não há ID de ZIP nem de documento para correlacionar linhas de workers concorrentes; (3) tempos de fase não são medidos em lugar nenhum. O CLAUDE.md registra que o erro completo (com caminho e stderr) vai só ao log, justamente porque o `.txt` não pode levar o nome do usuário: logo o arquivo de log **vai conter** esses dados se nada for feito.

**Atividade.** `etl.AtividadeDoc` (`internal/etl/atividade.go:18-30`) tem nome, evento, estado, páginas, feitas, OCR em andamento, erro, `Leitura` e `Detalhe`. `InstantaneoAtividade` (`atividade.go:33-40`) acrescenta limites e `OCRAtivos`. `Atividade` (`atividade.go:45-53`) guarda isso sob um mutex e vai ao frontend como `queue:atividade`, no máximo a cada 200 ms (`GetQueueActivity` devolve o instantâneo). É estado ao vivo: não guarda nada por página nem tempo.

**Leitura do documento.** `Leitura` (`internal/etl/leitura.go:20-31,44-120`): `lido`, `segundo_leitor`, `ocr`, e os não lidos `sem_ocr`, `senha`, `ilegivel`, `prazo`, `ocr_falhou`, `sem_texto`. `QueueItem.UnreadDocs` (`queue.go:37`) é o único número de qualidade que a fila mostra hoje.

**Marcador `sem_ocr`.** `analisarPagina` (`pdf_parser.go:380-405`) define `precisaOcr` e `semOcr`: texto ilegível; texto nativo com menos de 100 caracteres **e** imagem ou desenho vetorial. Com OCR desligado, essas páginas ficam com `semOcr`, a marca `sem_ocr` (`pdf_parser.go:189-192`; no Poppler, `pdf_poppler.go:75-77`) e, sem texto nativo, o conteúdo `marcadorPaginaSemOCR` (`pdf_parser.go:350`). As regras novas da v0.4.2 (imagem descoberta, isca) só agem com o OCR **ligado** (`if ocrEnabled && !an.precisaOcr`, `pdf_parser.go:~400`): com OCR desligado a página fica como lida, sem marca. O formatter lê a marca em `formatter.go:322` e `prepararLinhas` não limpa essas páginas (`formatter.go:337-345`). Se o documento todo fica assim, é "não lido" (`leitura.go:102,120`); se só algumas páginas, **nada aparece na fila**: esse é o M5. Página sem imagem e com pouco texto não é candidata: segue como lida, curta ou vazia. O dono tem razão, e a spec o adota.

**Falha do OCR.** Em `pdf_ocr_despacho.go:134-159`: toda página cujo OCR falhou e que tinha texto nativo de reserva leva `ocr_aviso` (em qualquer regra); `ocr_erro` só existe quando a página ficou sem texto, ou quando a falha ocorreu numa regra que não é a de texto curto (regra nova com reserva fica sem `ocr_erro`, para o documento de uma página não virar "não lido"). Isso vira marcador no `.txt` (PRD §6, "Falha do OCR").

**Limpeza.** `cleanPage` (`formatter.go:162`) e o caminho por linhas `cleanLinhas` (`formatter.go:~594`, chamado em `:588-596`) aplicam `higienizacao.go` (níveis `none/basic/standard/aggressive` em `:12-15`). Nas páginas duplicadas a limpeza roda e o resultado é descartado de propósito (`formatter.go:588-592`), para o estado do documento avançar. `GenerateConsolidatedOutput` (`formatter.go:439`) devolve `(texto, tokens)`. Duplicatas H7: pré-passada `duplicadasPorConteudo` (`formatter.go:372`, `duplicadasH7` em `:352`, mínimo de 200 runas em `:232`). Tipos omitidos (H10): `pipeline.go:249-252,612`, com `omitido` e `total_paginas` no mapa. Nenhum desses números chega a uma estrutura de saída.

**Reprocessar.** `Queue.Reprocessar(id, ocrMode, cleaningLevel)` (`queue.go:262-300`) grava um `ajusteConfig` no item (struct em `queue.go:56`) e o aplica sobre uma cópia da config (`queue.go:533-536`); a config global não muda. `ReprocessarItem.svelte` escolhe o modo de OCR (`none/fast/normal/best`) e o nível de limpeza. O "modo" hoje é só o modelo do Tesseract, ou desligado.

**Config e pasta de dados.** `models.Config` (`monitored_path`, `ocr_mode`, `cleaning_level`, `auto_start`, `watchdog_enabled`, `theme`, `omitir_tipos`); `config.Load` usa `os.UserConfigDir()` (`internal/config/config.go:67`). A pasta `%AppData%\eprocIA` já guarda `config.json` e `falhas.json` (`app.go:85`). O item da fila tem `ID` uuid (`queue.go:192`).

## Decisões do dono já tomadas

- Página sem imagem não vai ao OCR e não é falha.
- Estatísticas finais com páginas com OCR e sem, redução da limpeza etc., e opção de forçar OCR.
- Log bem completo e telemetria, tudo local, sem envio.
- Da v0.4.2: **sem log diagnóstico por página**. Esta spec não o reverte sozinha: a pergunta Q7 devolve a decisão ao dono.

## Objetivos

1. Ao fim de cada ZIP, o usuário vê um relatório curto e honesto do que foi lido e de que jeito, sem alarme onde não há problema.
2. O usuário pode forçar o OCR em todas as páginas de PDF de um ZIP, sem mexer na configuração global.
3. Existe um log em arquivo, estruturado, que permite reconstruir o que aconteceu em cada ZIP, documento e (se o dono quiser) página, sem expor o nome do usuário do Windows.
4. Existe um arquivo de telemetria local, uma linha por ZIP e sem nomes.
5. Nada disso atrasa nem trava o pipeline, e nada sai da máquina.

## Fora do escopo

- Qualquer envio de log ou telemetria para a rede. Quem partilha um log é o dono, à mão.
- Painel de análise da telemetria dentro do app.
- Mudar as regras de decisão de OCR (continuam as da v0.4.2).
- Avisos novos no `.txt` além dos que o PRD já define, **exceto** a linha de cabeçalho da pergunta Q3, se o dono aprovar.

## Parte 1: o relatório do processamento

### Categorias de página: exaustivas e disjuntas

O **total** do relatório é o número de páginas dos documentos **lidos** (PDF e HTML; um HTML conta como 1 página). Páginas de documentos omitidos por tipo e de documentos não lidos ficam fora do total e aparecem num contador à parte ("páginas de documentos não lidos", "páginas de documentos omitidos"). Cada página do total entra em **uma** categoria, escolhida **pela primeira regra que servir**, nesta ordem:

| Ordem | Categoria | Nome na interface | Regra (campos que o pipeline já tem) |
|---|---|---|---|
| 1 | com_erro | Com erro | Falha ao ler a página (`[ERRO ao ler página...]`) ou `ocr_erro` presente **sem** `ocr_aviso` (falha do OCR e a página ficou sem texto de reserva). Uma página com `ocr_aviso` tem texto e vai para `lida_nativa`, mesmo que também tenha `ocr_erro` (que `pdf_ocr_despacho.go:156-163` põe nas regras padrão e de texto ilegível, com ou sem reserva) |
| 2 | sem_ocr | Precisava de OCR e ficou sem | Marca `sem_ocr` (`an.semOcr`): OCR desligado e a página é candidata por texto ilegível, ou por texto curto com imagem ou desenho vetorial; no segundo leitor (Poppler), página sem texto com OCR desligado |
| 3 | vazia | Em branco | Conteúdo final vazio e nenhuma das duas acima: página nula, página sem texto e sem imagem, ou OCR que devolveu nada |
| 4 | lida_ocr | Lida por OCR | O OCR rodou e o resultado entrou (inclui a junção com o nativo) |
| 5 | lida_nativa | Texto do PDF | Todo o resto: texto nativo legível, página curta sem imagem, HTML, e página que foi ao OCR mas ficou com o nativo |

Sub-contadores dentro de `lida_nativa`, para o relatório e a telemetria, sem formar categoria nova: `ocr_sem_ganho` (foi ao OCR, o OCR veio em branco ou ilegível e o nativo ficou, `conteudoAposOCR`, `pdf_juncao.go:27-39`) e `ocr_falhou_com_reserva` (a página tem `ocr_aviso`, com ou sem `ocr_erro`: o OCR falhou e o nativo ficou, em qualquer regra; o número é exatamente o de páginas com `ocr_aviso`). Esse segundo é mostrado ao usuário em tom de aviso, porque o texto da página pode estar incompleto.

**Duplicatas H7 não são categoria.** A página duplicada fica na categoria que teria; `paginas_duplicadas` é um número a mais, informativo, e não entra na soma. Assim, a soma das cinco categorias é sempre igual ao total, por construção.

Como medir: o pipeline já sabe tudo por página. A decisão está em `analisarPagina`; o desfecho em `paginaPronta` e no despacho do OCR (`ocr`, `sem_ocr`, `ocr_erro`, `ocr_aviso`). A mudança é gravar o campo `categoria` no mapa da página onde o mapa é montado e somar por documento. Nenhuma decisão de OCR muda.

### Números do relatório

Uma struct `RelatorioZip` preenchida pelo pipeline:

- **Documentos:** total, lidos, não lidos (por `Leitura`), omitidos por tipo (e quais), duplicados omitidos por H7.
- **Páginas:** total e as cinco categorias, mais os sub-contadores, mais páginas de documentos não lidos e omitidos, mais `paginas_duplicadas`.
- **Tempo:** total do ZIP (parede) e "tempo de trabalho de OCR", que é a **soma** do tempo dos processos de OCR e por isso pode passar do tempo total, porque rodam em paralelo; além do nº de blocos de OCR, o nº de páginas por bloco e o tempo médio por bloco. Tempo por página de OCR **não é medido**: o OCR roda em bloco (`RunOCRPaginas`, um `tesseract` para a lista inteira), então só existe o tempo do bloco; a média por página é o tempo do bloco dividido pelas páginas dele, e é rotulada como estimativa.
- **Limpeza:** ver abaixo.
- **O `.txt`:** tamanho em bytes e tokens estimados (já existem).

### Contadores da limpeza: onde medir

Há dois caminhos de limpeza (`cleanPage` e `cleanLinhas`) e a limpeza roda também nas páginas duplicadas, cujo resultado é descartado. Um contador dentro de `cleanPage` mediria errado. Definição única: o contador fica **no ponto onde o formatter emite o texto da página** (o laço de `GenerateConsolidatedOutput` que escreve `<pag>`), e considera só as páginas que de fato saem com texto limpo:

- **antes** = bytes do texto extraído da página (`p.content`), sem nenhuma limpeza;
- **depois** = bytes do texto dessa página como entra no `.txt` (sem as tags, sem a linha de `ocr_aviso`, que é acrescentada depois da limpeza);
- ficam **fora** das duas somas: páginas `sem_ocr` (nunca limpas), páginas com referência de duplicata, documentos omitidos e não lidos;
- `bytes_duplicatas` = bytes dos conteúdos das páginas trocadas por referência de H7, informado à parte;
- redução % = 1 − depois/antes, só quando o nível é `basic` ou acima; em `none` a linha não aparece. Tokens estimados antes e depois, na mesma regra de um token a cada 4 caracteres.

O relatório **não** promete que "depois" bata com o tamanho do arquivo (que tem tags, cabeçalho e avisos); promete que bate com a soma do texto das páginas emitidas, e há teste para isso.

### Onde aparece na interface

- **Linha-resumo** no item concluído (`ProcessList.svelte`), só quando há algo a dizer: "142 páginas: 120 do texto do PDF, 18 por OCR, 4 em branco". Se houver página `sem_ocr`, `com_erro` ou `ocr_falhou_com_reserva`, a linha fica em tom de aviso, por exemplo: "6 páginas têm imagem e o OCR estava desligado".
- **Botão "Ler com OCR"** ao lado desse aviso, quando há página `sem_ocr`: refaz o ZIP com o OCR ligado (modo `normal`, pelo Reprocessar). É uma ação nova visível; Q2 pergunta se o dono a quer.
- **Painel de detalhes** por clique (novo `RelatorioItem.svelte`, no estilo de `AtividadeItem.svelte`): as categorias, tempos, limpeza, duplicatas e omitidos, e a lista dos documentos com páginas `sem_ocr`, `com_erro` ou `ocr_falhou_com_reserva`, com os números de página, para o usuário abrir o PDF original.
- Texto explicativo, em linguagem simples: "Página em branco ou só com texto não vai ao OCR: isso é normal. O aviso aparece só quando a página tem imagem ou desenho e o texto do PDF não bastou."
- Estado em `QueueItem` (campo `Relatorio *RelatorioZip` com tag JSON, enviado em `queue:item` e `queue:update`), zerado quando o item volta a `pending` (junto com `UnreadDocs`, em `Reprocessar`). Só na memória; o histórico vai para a telemetria.
- O aviso **não** muda o status `success` nem vira falha. É assim que o M5 é resolvido.

### O relatório vai ao `.txt`?

Recomendação: **não**. O `.txt` é colado num LLM; cada linha extra custa tokens e não ajuda a análise. O cabeçalho já leva "Documentos não lidos" e "Tokens estimados", e os marcadores de erro de página. A única adição que vale discutir (Q3) é uma linha com o número de páginas que precisavam de OCR e ficaram sem, para o modelo avisar o usuário de que a análise pode estar incompleta. Isso muda o `.txt` e o PRD §6; por isso é do dono.

## Parte 2: forçar OCR

### O que significa

"Forçar OCR" faz **toda página de PDF** de documento lido ir ao OCR, inclusive as de texto nativo legível. HTML e documentos omitidos ficam de fora. **Precedência:** `motivoForcado` vale só para a página que `analisarPagina` **não** mandou ao OCR; a página que já vai (motivo padrão, texto ilegível, imagem descoberta, isca) mantém o motivo e as regras de falha dele (por exemplo, `ocr_erro` no padrão e no ilegível). No segundo leitor (Poppler), `parsePoppler` (`pdf_poppler.go`, laço próprio) já manda ao OCR a página sem texto; a com texto passa a receber `motivoForcado` do mesmo modo. Com `ocr_mode = none` a opção é recusada (a interface desabilita a caixa e o back-end devolve erro em português).

### Como junta com o nativo

Achado da revisão: `juntarOCRComNativo` (`pdf_juncao.go:157`) usa o **OCR como base** e encaixa as linhas nativas que o OCR perdeu (só vale para `motivoImagemDescoberta`). Ela não serve para "o nativo vence": numa página de texto bom, o OCR lê o mesmo texto e gera variantes mal lidas de cada linha, e inverter os papéis acrescentaria essas variantes como se fossem conteúdo novo. Por isso a recomendação **não reusa a junção**: um novo motivo `motivoForcado` com a regra mais simples e previsível, a de `motivoPadrao` (`conteudoAposOCR`, `pdf_juncao.go:27-39`):

- OCR com texto legível: **o OCR substitui** o nativo;
- OCR em branco ou ilegível: fica o nativo (o OCR nunca apaga o nativo);
- OCR falhou (erro ou prazo) numa página com nativo: fica o nativo, com `ocr_aviso` e sem `ocr_erro`, como nas regras novas (`regraNovaComReserva` em `pdf_ocr_despacho.go:156-157` passa a incluir `motivoForcado`); o documento de uma página não vira "não lido" por causa de um OCR que o próprio usuário pediu a mais;
- sem nativo: como hoje.

Isso faz do "forçar" uma ferramenta para quem **sabe** que o nativo é ruim (PDF de texto trocado, fonte quebrada que o app não detectou); numa página de texto bom, o OCR é um pouco pior que o nativo, e a interface avisa isso. A alternativa mais ambiciosa (nativo como base e só entram linhas que só o OCR viu, com um algoritmo próprio de comparação) está na Q5 como opção, com o custo de ser um algoritmo novo, a medir em corpus real.

**Custo extra:** cada página duplicada (H7) também faz OCR, porque a duplicata só é decidida depois, no formatter. Aceito e dito ao usuário; evitar isso exigiria decidir H7 antes do OCR e não vale a complexidade.

### Onde se liga

- **No Reprocessar**, por item (recomendado): `ReprocessarItem.svelte` ganha a caixa "Forçar OCR em todas as páginas" (desligada); `Queue.Reprocessar` e `App.ReprocessarItem` ganham `forcarOCR bool`, guardado em `ajusteConfig`; só vale para essa execução. Exige `wails generate module` e atualizar `frontend/reprocessar.test.mjs`.
- **Global** (`models.Config.ForcarOCR`, padrão `false`): só se o dono quiser (Q4); é fácil esquecer ligada e dobrar o tempo de todo ZIP.
- O botão "Ler com OCR" do aviso (Parte 1) resolve páginas `sem_ocr` (liga o OCR). É coisa diferente de forçar em tudo.

### Custo de tempo

O OCR é a fase mais cara: um `tesseract` por bloco, limitado a `NumCPU()-1`. Um PDF nativo de 100 páginas, hoje em segundos, passa a levar o tempo de 100 páginas de OCR. A interface avisa antes de confirmar; o relatório mostra o tempo de trabalho de OCR depois. A medição real fica para a implementação, com o corpus sintético do `scripts/bench`.

## Parte 3: arquivo de log

### Local, nome, rotação e retenção

- Pasta: `%AppData%\eprocIA\logs\`. Arquivo ativo `eprocia.log`. Rotação por tamanho, **própria, sem dependência nova**: ao passar de 5 MB vira `eprocia.1.log`, os anteriores se deslocam, no máximo 5 arquivados (cerca de 30 MB no pior caso). Retenção: arquivos com mais de 30 dias são apagados na abertura. (Decisão técnica do autor: uma dependência só para isso não se justifica, e o app é distribuído como executável único.)
- O registro é **iniciado depois da trava de instância** (`main.go:21-30`): a segunda cópia do app, que só avisa e sai, não pode rodar rotação nem retenção no arquivo da primeira (o Windows não renomeia arquivo aberto, e a rotação falharia no meio). Antes disso, e se a trava falhar, o `log` continua indo ao stderr, como hoje. O `log.Fatal` de `main.go:80` passa a descarregar o registro antes de sair (ele faz `os.Exit` sem rodar nada).
- Falha ao abrir a pasta ou o arquivo nunca impede o app de iniciar: cai no stderr.
- **Botão "Abrir pasta de logs"** em Configurações, no mesmo `explorer.exe` que o app já usa (Q8). A pasta aberta tem só `eprocia.log*`; o índice de nomes (abaixo) fica **em outra pasta**, para quem partilha a pasta inteira não levar os nomes junto.

### Formato

**JSON Lines**, com `log/slog` (`slog.NewJSONHandler`), uma linha por evento. Decisão técnica do autor: filtra com `jq`, Python ou planilha ("todos os erros de OCR do ZIP tal"), não precisa de regex em texto livre, não precisa de dependência. A mensagem `msg` fica em português (convenção do projeto).

### Níveis e volume

`DEBUG`, `INFO`, `WARN`, `ERROR`. Padrão: `INFO`. `DEBUG` liga por `EPROCIA_LOG=debug` ou `log_nivel` no `config.json`, sem tela.

O volume atual não cabe em `INFO`: as ~3 linhas por página do `pdf_parser.go` somariam milhares de linhas por ZIP grande e encheriam 5 MB × 5 com um só ZIP. Regra: **nenhuma chamada por página (nem por atualização de progresso) fora de `DEBUG`**, com um teste que varre o código-fonte à procura de `log.Printf` por página fora da lista. Antes de apontar o pacote `log` para o novo arquivo (`slog.SetDefault`), convertem-se para `Debug` as chamadas por página de `pdf_parser.go` (`:108,134,147,168,182,366-373,375-397` e as do extrator posicional `:550,560,566,570`), de `pdf_poppler.go`, de `pdf_ocr_despacho.go:123,176` e o progresso de `queue.go:492` (uma linha a cada atualização). Fica de fora e **sobe para `WARN`** a linha do Form XObject ignorado (`pdf_parser.go:374`), a única linha de log que a v0.4.2 acrescentou por decisão do dono. Em `INFO` ficam só as de documento e de ZIP. Resultado: um ZIP de 500 documentos gera algumas centenas de linhas (da ordem de 150 KB) em `INFO`. O log **por página sempre ligado** reverteria a decisão do dono na v0.4.2 (sem log diagnóstico por página): é a pergunta Q7. A recomendação mantém a decisão: por página só em `DEBUG`, que o dono liga quando precisar.

### O que registra

Campos fixos em toda linha: `ts` (ISO 8601 com fuso), `nivel`, `msg`, `zip_id` (o `ID` uuid do item da fila, `queue.go:192`; o mesmo da telemetria), `doc_id` (índice do arquivo no ZIP, que distingue entradas de mesmo nome) e `pag` quando couber.

- **Início do app:** versão (de `wails.json`), versão do Windows, nº de CPUs, tamanho dos dois semáforos (`NumCPU()-1`), pasta de logs e a **configuração efetiva**, já saneada (a pasta monitorada aparece com `%USERPROFILE%` no lugar do perfil).
- **ZIP:** origem (monitorada, arrastada, manual), tamanho, nº de PDF e HTML, config efetiva do item (com o ajuste do Reprocessar e `forcar_ocr`), duração, resultado, tamanho do `.txt`, tokens e o relatório final (Parte 1) como objeto.
- **Documento:** nome (ver Privacidade), evento, extensão, tamanho, nº de páginas, leitor usado (biblioteca ou Poppler), estado final, `Leitura` e `Detalhe`, duração, páginas por categoria.
- **Página (só `DEBUG`):** categoria, motivo de OCR (`motivoPadrao/Ilegivel/ImagemDescoberta/Isca/Forcado`), bytes de texto nativo, se tem imagem ou desenho vetorial, as métricas de `metricasPagina`, se manteve o nativo, se houve junção. Tempo de OCR **por bloco**, não por página.
- **OCR:** bloco ou página a página, páginas do bloco, duração, e se foi preciso refazer página a página (`pdf_ocr_despacho.go:195-211`). **Código de saída e stderr** do `tesseract`/`pdftoppm` exigem mudança em `internal/ocr` (`tesseract.go`, `prazo.go`): hoje o erro volta como `error`; o stderr passa a ser devolvido no erro para entrar no log (nível `WARN`/`ERROR`), **nunca** no `.txt`, que só leva a frase fixa (regra do PRD e do CLAUDE.md).
- **Erros:** a cadeia completa (`%w`), a etapa (`abrir_zip`, `ler_pdf`, `ocr`, `html`, `formatar`, `gravar`), a classe (`classificarErro`) e, em `panic`, a pilha (hoje só uma linha, `pipeline.go:583`).
- **Limpeza:** nível, bytes antes e depois, duplicatas e tipos omitidos.
- **Fila e janela:** pausa, retomada, cancelamento, reprocessar, falha ao gravar o `.txt`, watchdog, bandeja e notificações (`app.go:101-134`).

### Migração do `log.Printf`

As ~112 chamadas viram `slog` com campos, em etapas. Para nenhuma se perder no meio, `slog.SetDefault` redireciona o pacote `log` ao mesmo manipulador, **mas só depois** de três coisas: o saneamento (abaixo) estar no manipulador; as chamadas por página estarem em `Debug`; e as chamadas que gravam **nome ou caminho do ZIP** estarem migradas para `zip_id`: `queue.go:173,187,205,297,445,473,518,558,569`, `app.go:91,294` (que gravam `MonitoredPath`), `abertura.go:70-72`, `pdf_parser.go:129,134,165` e `pdf_poppler.go:29,46,89`. Um teste roda um ZIP sintético com nome falso pela fila e confirma que o nome não aparece em `eprocia.log` (só no índice).

## Parte 4: telemetria local

### Arquivo

`%AppData%\eprocIA\telemetria\telemetria.jsonl`, **um registro por ZIP processado** (também por reprocessamento, com `reprocesso: true`), gravado no fim do ZIP, por acréscimo (`O_APPEND`), uma linha por escrita. Rotação própria: ao passar de 10 MB vira `telemetria.1.jsonl`, com no máximo 5 arquivados. Um registro tem cerca de 1 KB, ou seja, dezenas de milhares de ZIPs antes de passar do primeiro arquivo; sem apagar por idade, para o dono ter a série longa. Decisão técnica do autor.

**Sem chave para desligar.** Ela fica sempre ligada: é local, sem nomes, e só o dono a lê; uma chave nas Configurações seria uma tela a mais que ninguém pediu. O texto das Configurações, ao lado do botão da pasta de logs, diz "estes arquivos ficam só neste computador".

### Campos (sem nomes)

- Versão do app, `ts`, `zip_id` (o mesmo uuid do log, que não deriva do nome do arquivo), `reprocesso`.
- Máquina: nº de CPUs, memória total, versão e build do Windows, arquitetura. Sem nome do computador nem do usuário.
- Configuração: modo de OCR, nível de limpeza, `forcar_ocr`, quantidade (não os nomes) de tipos omitidos, origem do ZIP.
- Tamanhos: bytes do ZIP, nº de documentos PDF e HTML, nº de páginas, bytes do `.txt`, tokens.
- Tempos por fase (ms): abrir e listar, pré-cálculo de páginas, leitura e extração (soma dos workers e parede), OCR (tempo de trabalho e parede), formatação e limpeza, gravação; total.
- Páginas por categoria e por motivo de OCR; sub-contadores `ocr_sem_ganho` e `ocr_falhou_com_reserva`.
- OCR: nº de processos, tamanho médio dos blocos, tempo por bloco (mediana e p95), nº de refações página a página, nº de prazos esgotados.
- Limpeza: bytes antes e depois, redução %, duplicatas e tipos omitidos (contagens).
- Falhas por classe (`Leitura` e `classificarErro`), só contagens.
- Resultado: `success`, `error`, `cancelled`, e a classe do erro.

Serve para: achar qual fase domina o tempo, calibrar `NumCPU()-1` e o tamanho do bloco, ver quantas páginas caem em cada motivo (calibrar os limiares da v0.4.2 com dados reais), medir o que cada nível de limpeza economiza, e ver quais falhas mais ocorrem.

### Sem envio

Não há cliente de rede, URL nem destino configurável. Um teste impede o pacote de telemetria de importar `net` e `net/http`.

## Privacidade

- **Onde ficam:** só em `%AppData%\eprocIA\logs`, `...\telemetria` e `...\dados`, fora do repositório e do `.txt`. Defesa em profundidade: `*.log` e `*.jsonl` no `.gitignore` e no hook `proteger-dados` (`.claude/hooks/proteger-dados.mjs`, `arquivoSensivel`), com as mesmas exceções de pasta de documentação que o hook já tem para `.txt`, e teste em `.claude/hooks/hooks.test.mjs`.
- **O log atual NÃO é partilhável como está.** Ele grava `p.filePath` (arquivo temporário sob o perfil do usuário, `pdf_parser.go:129,134,165`), a configuração efetiva inclui `monitored_path`, e a cadeia de erro (`%w`) e o stderr do Tesseract trazem caminhos. Por isso o **manipulador faz o saneamento**, em toda mensagem e em todo atributo de texto ou de erro: troca o perfil do usuário por `%USERPROFILE%`, o diretório temporário por `%TEMP%` e o nome do usuário **só quando aparece como segmento de caminho** depois de `\Users\` ou `/Users/` por `<usuario>` (trocar o nome solto mutilaria palavras comuns se o usuário tiver nome curto). O saneamento é a rede de segurança; a garantia de que o nome do ZIP não entra vem da migração das chamadas acima, não dele. `slog.SetDefault` só é ligado depois disso, e há teste (abaixo).
- **Nomes no log:** o nome do documento dentro do ZIP ajuda muito a depurar e é, em geral, genérico ("Evento 12 - PETICAO INICIAL.pdf"). O nome do **ZIP** costuma ser o número do processo e identifica as partes. Recomendação: o log leva o nome do documento, e o ZIP **só pelo `zip_id`**; o mapa `zip_id` para nome real fica em `%AppData%\eprocIA\dados\indice.jsonl`, outra pasta (não a que o botão abre), e o dono não precisa partilhá-lo. Alternativa: todos os nomes no log (Q6).
- **Telemetria:** nunca leva nome de ZIP, de documento, de pasta nem texto de página. Só contagens, tempos e classes.
- **Conteúdo das páginas:** nunca vai a log nem telemetria, em nenhum nível, nem trecho nem hash (o hash de H7 não é gravado).
- **Como o dono partilha um log:** pelo botão, pega `eprocia.log`. Com o padrão acima ele já não tem nome do ZIP nem do usuário. Para o caso de dúvida, um script em `scripts/` (`anonimizar-log`) troca os nomes de documento por rótulos estáveis (`doc-17`) e confere se sobrou algum caminho de usuário.

## Desempenho e robustez

O pipeline tem workers por documento limitados por semáforo (`NumCPU()-1`) e um segundo semáforo do OCR (`ocrRunner`). O log não pode virar ponto de contenção nem de travamento:

1. **Escrita assíncrona.** O manipulador põe a linha já formatada num canal com buffer (4096) e **uma única** goroutine grava no arquivo, com `bufio.Writer` descarregado a cada 1 s e ao encerrar. Os workers só enviam ao canal.
2. **Nunca bloqueia, em nenhum nível.** Canal cheio: a linha `DEBUG`/`INFO` é descartada na hora; `WARN`/`ERROR` esperam no máximo 50 ms e, passado isso, também são descartadas. O descarte é contado e o total (`linhas_perdidas`) é gravado quando o disco voltar. Um disco parado nunca segura um worker.
3. **Falha de gravação.** Se escrever ou descarregar falha (disco cheio, permissão), a goroutine para de gravar no arquivo por 60 s, conta as linhas perdidas, tenta de novo e registra o fato uma vez; o erro nunca sobe para o pipeline. A telemetria segue a mesma regra: falha de gravação vira um aviso uma vez no log e o registro é descartado; o item da fila continua `success`.
4. **Sem formatação cara no caminho quente.** Nada de `fmt.Sprintf` por página em `INFO`; `DEBUG` checa `Enabled` antes de montar campos.
5. **Contadores sem lock.** As estatísticas por página usam `atomic` (como `processedDocsVal`), e a soma final se faz em uma só thread, fora do mutex de `Atividade`.
6. **Encerramento limpo.** `OnShutdown`, `App.Quit` e o caminho do `log.Fatal` descarregam o canal antes de sair; numa queda brusca perde-se no máximo o último segundo.
7. **Medição.** O bench do harness (`eprocia-bench`) antes e depois com `-count=10` e benchstat; critério: sem piora com p < 0.05 e nenhuma métrica pior que 5%.

## Mudanças por arquivo

- **Novos:** `internal/registro` (manipulador `slog` assíncrono, saneamento, rotação, retenção, índice de nomes), `internal/telemetria` (registro por ZIP, coleta da máquina), `frontend/src/components/RelatorioItem.svelte`, `frontend/src/lib/relatorio.js`, `scripts/anonimizar-log`.
- **`internal/etl`:** `RelatorioZip` e o campo `categoria` na página; contadores de limpeza no laço de saída de `formatter.go` (não em `higienizacao.go`); tempos de fase em `pipeline.go`; `motivoForcado` em `pdf_juncao.go` e em `pdf_ocr_despacho.go`; `forcarOCR` em `Queue`, `ajusteConfig` e `ProcessZip`; conversão dos `log.Printf` por página para `Debug` em `pdf_parser.go`, `pdf_poppler.go`, `pdf_ocr_despacho.go`.
- **`internal/ocr`** (`tesseract.go`, `prazo.go`): devolver código de saída e stderr no erro, para o log.
- **`internal/config` e `internal/models`:** `ForcarOCR` (só se Q4 for global) e `LogNivel`.
- **`app.go` e `main.go`:** iniciar o registro depois da trava de instância; descarregar em `OnShutdown`, `App.Quit` e antes do `log.Fatal`; método `AbrirPastaLogs`; `ReprocessarItem` com `forcarOCR`.
- **Frontend:** `ProcessList.svelte`, `ReprocessarItem.svelte`, `Settings.svelte`.
- **Docs e harness:** `PRD.md` (§6, se Q3 for sim, e o novo relatório), `DESIGN.md`, `CLAUDE.md`, `.gitignore`, `.claude/hooks/proteger-dados.mjs` e `hooks.test.mjs`.

## Testes

**Go:**

- Categorias: um PDF sintético por categoria (texto nativo; escaneado com OCR; escaneado com OCR desligado; página sem texto e sem imagem; falha de OCR com e sem nativo; leitura que falha; página nula; caminho do Poppler) e a verificação de que cada página cai em **exatamente uma** categoria e a soma das cinco é igual ao total, inclusive com duplicatas H7, documentos omitidos e não lidos no mesmo ZIP.
- Página com pouco texto e sem imagem nunca é `sem_ocr` nem `com_erro` (o ponto do dono); com OCR desligado, página de imagem descoberta ou isca fica `lida_nativa` (as regras novas não agem sem OCR).
- Sub-contadores: `ocr_falhou_com_reserva` igual ao número de páginas com `ocr_aviso` (inclusive as que também têm `ocr_erro`, que não contam em `com_erro`); `ocr_sem_ganho` igual ao das páginas que foram ao OCR e ficaram com o nativo sem falha.
- Limpeza: "antes" e "depois" calculados no laço de saída, com um teste que recalcula a soma independente a partir do texto das páginas emitidas; `antes >= depois`; redução 0 em `none`; páginas duplicadas e `sem_ocr` fora das somas.
- Forçar OCR, com o `ocrRunner` injetável: toda página de PDF de documento lido é despachada; OCR legível substitui o nativo; OCR em branco, ilegível ou com falha mantém o nativo (com `ocr_aviso` na falha, sem `ocr_erro`); combinação com `none` rejeitada; HTML e omitidos não são afetados; o ajuste do Reprocessar não muda a config global.
- Log: arquivo criado, linha JSON válida com os campos fixos, rotação aos 5 MB, retenção por idade e por quantidade, descarga no encerramento, e teste de concorrência com `-race` e dezenas de goroutines.
- **Saneamento:** um teste grava mensagens, atributos e erros com o perfil do usuário e o diretório temporário e confirma que nenhum aparece no arquivo.
- **Não bloqueia:** com um `io.Writer` que trava e canal cheio, `Info`, `Warn` e `Error` voltam em tempo curto e `linhas_perdidas` sobe; com `Writer` que devolve erro, nada sobe ao chamador e a gravação volta depois dos 60 s (relógio injetável).
- Privacidade: o log e a telemetria de um ZIP sintético com nome falso não contêm o nome do ZIP nem texto de página; a telemetria não tem nome de documento; o pacote de telemetria não importa `net`/`net/http`.
- Falha ao criar a pasta de logs (somente leitura) não impede o início; a segunda instância não toca no arquivo da primeira.

**Frontend (`frontend/*.test.mjs`):**

- `relatorio.test.mjs`: `src/lib/relatorio.js` formata a linha-resumo e as frases; o aviso só aparece com `sem_ocr`, `com_erro` ou `ocr_falhou_com_reserva`, nunca para "em branco".
- Leitura dos `.svelte` por regex, como nos testes atuais: caixa "Forçar OCR" em `ReprocessarItem.svelte`, desligada por padrão e desabilitada com OCR desligado; botão "Abrir pasta de logs" em `Settings.svelte` (se Q8 for sim); botão "Ler com OCR" (se Q2 for sim); o painel usa as classes de tema e não cor fixa (`contraste.test.mjs`, `paleta.test.mjs`).
- `reprocessar.test.mjs` atualizado.
- Hooks: `hooks.test.mjs` cobre o bloqueio de `git add` de `.log` e `.jsonl`.

## Critérios de aceite

1. Ao fim de um ZIP, o item mostra as páginas por categoria, e a soma das cinco é igual ao total de páginas dos documentos lidos (definido acima); duplicatas, omitidos e não lidos aparecem à parte.
2. Nenhuma página sem imagem aparece como "precisava de OCR e ficou sem" nem como erro.
3. Com o OCR desligado, páginas candidatas aparecem no aviso; o item continua `success`.
4. O painel mostra tempo total e tempo de trabalho de OCR, bytes antes e depois da limpeza com a redução %, duplicatas e tipos omitidos; "depois" é igual à soma do texto das páginas emitidas (teste independente).
5. "Forçar OCR" no Reprocessar leva todas as páginas de PDF ao OCR e não muda a configuração global; com um OCR injetado, o resultado segue a regra da Parte 2 (substitui se legível, mantém o nativo se vazio, ilegível ou falho).
6. Existe `%AppData%\eprocIA\logs\eprocia.log` em JSON Lines com versão, configuração efetiva saneada, cada ZIP, cada documento e os erros com etapa, classe e causa; a rotação e a retenção funcionam; nenhum caminho com o usuário do Windows aparece.
7. Existe `telemetria.jsonl` com um registro por ZIP, com os campos listados e sem nome de ZIP, de documento, de pasta nem texto.
8. Nenhuma chamada de rede foi acrescentada; o log e a telemetria nunca travam um worker, mesmo com disco cheio.
9. O bench não piora o pipeline (p < 0.05, nenhuma métrica pior que 5%); `go test -race ./...`, `go vet`, `svelte-check`, `npm run build` e o glob do frontend passam.
10. `*.log` e `*.jsonl` estão no `.gitignore` e no hook `proteger-dados`.

## Decisões técnicas do autor (não são perguntas)

JSON Lines com `slog` (formato); rotação própria do log, 5 MB × 5 arquivos e 30 dias (sem dependência); telemetria em arquivo único com rotação a 10 MB e 5 arquivados, sem apagar por idade; telemetria sempre ligada, sem chave; `zip_id` igual ao uuid do item nos dois arquivos; nível padrão `INFO`, com `DEBUG` por variável de ambiente.

## Perguntas para o dono

**Q1. Onde o relatório aparece?** Opções: (a) uma linha-resumo no item concluído e um painel de detalhes ao clicar; (b) só a linha-resumo. Recomendada: (a).

**Q2. Aviso e botão para as páginas que tinham imagem e ficaram sem OCR.** Opções: (a) aviso na linha do item e um botão "Ler com OCR" ao lado, que refaz o ZIP com o OCR ligado; (b) só o aviso, sem botão; (c) nada de aviso, só os números no painel. Recomendada: (a). É comportamento visível novo, além do que você pediu.

**Q3. O arquivo `.txt` ganha alguma informação?** Opções: (a) nada; (b) uma linha no cabeçalho "Páginas que precisavam de OCR e ficaram sem: N", só quando N > 0, para o modelo de IA avisar o usuário. Recomendada: (a). Mudaria o `.txt` e o PRD §6.

**Q4. Onde "Forçar OCR" fica?** Opções: (a) só no Reprocessar, por item; (b) no Reprocessar e também nas Configurações, para todos os ZIPs. Recomendada: (a), porque a opção global é fácil de esquecer ligada e dobra o tempo de tudo.

**Q5. O que "Forçar OCR" faz com a página que já tem texto bom?** Opções: (a) o OCR lido substitui o texto do PDF, e o texto do PDF fica se o OCR vier em branco, ilegível ou falhar (simples e previsível; pode ficar um pouco pior em página boa); (b) o texto do PDF fica e só entram as linhas que só o OCR viu (exige um algoritmo novo, a medir em corpus real); (c) o OCR sempre substitui, mesmo com falha. Recomendada: (a).

**Q6. Nomes dos arquivos no log.** Opções: (a) o log leva o nome do documento, e o ZIP só por um código, com a lista de nomes numa pasta separada; (b) o log leva todos os nomes, inclusive do ZIP (mais cômodo, mais difícil de partilhar). Recomendada: (a).

**Q7. Registrar cada página no log?** Isso reverte sua decisão da v0.4.2 de não ter log por página. Opções: (a) não por padrão: o detalhe por página só aparece quando você ligar o modo detalhado; (b) sempre, mas só números (o arquivo cresce muito e gira mais rápido). Recomendada: (a).

**Q8. Botão "Abrir pasta de logs" nas Configurações?** Opções: (a) sim; (b) não. Recomendada: (a).

**Q9. Em que release isso vai?** Opções: (a) tudo junto numa release; (b) em duas: primeiro log e telemetria (quase sem mudança visível), depois relatório e Forçar OCR. Recomendada: (b), porque o log ajuda a medir o próprio relatório.

## Riscos

- O contador de "antes e depois" da limpeza fica no laço de saída de `formatter.go`, que tem muitos testes; a mudança é só somar na entrada e na saída da página.
- Forçar OCR pode dobrar o tempo e deixar a página boa um pouco pior; a interface avisa, e o texto do PDF fica se o OCR falhar.
- O nome do ZIP e o nome do usuário do Windows estão nos dados que hoje vão ao log: o saneamento no manipulador é obrigatório, e o teste dele é um critério de aceite.
- O pacote `log` ainda é usado por código não migrado: a migração por etapas e a ordem (saneamento e `Debug` primeiro, `SetDefault` depois) importam.
- Mudar `internal/ocr` para devolver stderr toca o código que roda os processos externos (prazos, `CREATE_NO_WINDOW`): cobrir com os testes existentes do pacote.
- A linha extra no `.txt` (Q3) altera um contrato que o PRD §6 descreve e que a análise em LLM usa.
