# Relatório do processamento, forçar OCR, log e telemetria local

Data: 2026-11-22. Autor: jonyduque (redigido pelo Claude Code). Estado: decidida pelo dono em 2026-11-23 (seção "Decisões do dono, 2026-11-23"); pronta para o plano de implementação.

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
- Da v0.4.2: **sem log diagnóstico por página** como comportamento padrão. As respostas de 2026-11-23 mantêm isso: o registro por página só existe no modo `detalhado`, que o dono escolhe pela variável de ambiente (Q6 e Q7).
- Log e telemetria **só existem se a variável de ambiente `EPROCIA_LOG` estiver definida** (dono, 2026-11-23: "só vamos ativar os logs se houver uma determinada configuração nas variáveis do ambiente. Assim, só serão geradas por quem efetivamente escolher"). Sem ela, o app se comporta como hoje.

## Objetivos

1. Ao fim de cada ZIP, o usuário vê um relatório curto e honesto do que foi lido e de que jeito, sem alarme onde não há problema.
2. O usuário pode forçar o OCR em todas as páginas de PDF, de um ZIP (Reprocessar) ou de todos (Configurações), sem que o Reprocessar mude a configuração global.
3. Quem ligar `EPROCIA_LOG` ganha um log em arquivo, estruturado, que permite reconstruir o que aconteceu em cada ZIP, documento e (no modo `detalhado`) página, sem expor o nome do usuário do Windows.
4. Quem ligar `EPROCIA_LOG` ganha também um arquivo de telemetria local, uma linha por ZIP e sem nomes.
5. Nada disso atrasa nem trava o pipeline, e nada sai da máquina. Sem a variável, nada é criado em disco e a interface não menciona log.

## Fora do escopo

- Qualquer envio de log ou telemetria para a rede. Quem partilha um log é o dono, à mão.
- Painel de análise da telemetria dentro do app.
- Mudar as regras de decisão de OCR (continuam as da v0.4.2).
- Avisos novos no `.txt` além dos que o PRD já define, **exceto** a linha de cabeçalho da Q3 (aprovada).
- Chave de log ou de telemetria nas Configurações: o único interruptor é a variável de ambiente.

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
- **Botão "Ler com OCR"** ao lado desse aviso, quando há página `sem_ocr`: refaz o ZIP com o OCR ligado pelo Reprocessar: `forcarOCR = false` **explícito** (nunca herda a global), modo de OCR igual ao global se este não for `none` e `normal` caso contrário, nível de limpeza do item mantido. É uma ação nova visível, aprovada pelo dono (Q2). Não é o "Forçar OCR": só liga o OCR para as páginas candidatas.
- **Painel de detalhes** por clique (novo `RelatorioItem.svelte`, no estilo de `AtividadeItem.svelte`): as categorias, tempos, limpeza, duplicatas e omitidos, e a lista dos documentos com páginas `sem_ocr`, `com_erro` ou `ocr_falhou_com_reserva`, com os números de página, para o usuário abrir o PDF original.
- Texto explicativo, em linguagem simples: "Página em branco ou só com texto não vai ao OCR: isso é normal. O aviso aparece só quando a página tem imagem ou desenho e o texto do PDF não bastou."
- Estado em `QueueItem` (campo `Relatorio *RelatorioZip` com tag JSON, enviado em `queue:item` e `queue:update`), zerado quando o item volta a `pending` (junto com `UnreadDocs`, em `Reprocessar`). Só na memória; o histórico vai para a telemetria.
- O aviso **não** muda o status `success` nem vira falha. É assim que o M5 é resolvido.

### O relatório vai ao `.txt`?

**Só uma linha, aprovada pelo dono (Q3, opção b).** O relatório em si não vai ao `.txt` (é colado num LLM; cada linha extra custa tokens). A única adição é, no cabeçalho do `<contexto>`, a linha

`Páginas que precisavam de OCR e ficaram sem: N`

**só quando N > 0**, onde N é o total da categoria `sem_ocr` do ZIP, **somando só páginas de documentos lidos** (um documento inteiro sem OCR é "não lido", já está em `Documentos não lidos` e fica fora de N, para o modelo não somar duas vezes), para o modelo avisar o usuário de que a análise pode estar incompleta. Com N = 0 a linha não sai e o `.txt` fica byte a byte como hoje.

Onde entra: em `GenerateConsolidatedOutput` (`formatter.go:467`), logo depois de `Documentos não lidos: %d` e antes de `Data de Consolidação`. O PRD §6 hoje lista `Total de Documentos`, `Documentos não lidos`, `Data de Consolidação`, `Tokens estimados`; a linha nova fica entre a 2ª e a 3ª. Ela entra no cálculo de `Tokens estimados`, que já conta o cabeçalho inteiro ("inclusive desta linha"). O formatter recebe N por parâmetro novo, somado pelo pipeline a partir da categoria `sem_ocr` das páginas (as mesmas que o relatório conta); o PRD §6 ganha a linha e um item na lista de explicações ("Páginas que precisavam de OCR e ficaram sem: só aparece quando maior que 0; são as páginas com imagem ou desenho cujo texto do PDF não bastou e o OCR estava desligado").

## Parte 2: forçar OCR

### O que significa

"Forçar OCR" faz **toda página de PDF** de documento lido ir ao OCR, inclusive as de texto nativo legível. HTML e documentos omitidos ficam de fora. **Precedência:** `motivoForcado` vale só para a página que `analisarPagina` **não** mandou ao OCR; a página que já vai (motivo padrão, texto ilegível, imagem descoberta, isca) mantém o motivo e as regras de falha dele (por exemplo, `ocr_erro` no padrão e no ilegível). No segundo leitor (Poppler), `parsePoppler` (`pdf_poppler.go`, laço próprio) já manda ao OCR a página sem texto; a com texto passa a receber `motivoForcado` do mesmo modo. Com `ocr_mode = none` a opção não tem efeito (ver "Onde se liga").

### Como junta com o nativo

Achado da revisão: `juntarOCRComNativo` (`pdf_juncao.go:157`) usa o **OCR como base** e encaixa as linhas nativas que o OCR perdeu (só vale para `motivoImagemDescoberta`). Ela não serve para "o nativo vence": numa página de texto bom, o OCR lê o mesmo texto e gera variantes mal lidas de cada linha, e inverter os papéis acrescentaria essas variantes como se fossem conteúdo novo. Por isso a recomendação **não reusa a junção**: um novo motivo `motivoForcado` com a regra mais simples e previsível, a de `motivoPadrao` (`conteudoAposOCR`, `pdf_juncao.go:27-39`):

- OCR com texto legível: **o OCR substitui** o nativo;
- OCR em branco ou ilegível: fica o nativo (o OCR nunca apaga o nativo);
- OCR falhou (erro ou prazo) numa página com nativo: fica o nativo, com `ocr_aviso` e sem `ocr_erro`, como nas regras novas (`regraNovaComReserva` em `pdf_ocr_despacho.go:156-157` passa a incluir `motivoForcado`); o documento de uma página não vira "não lido" por causa de um OCR que o próprio usuário pediu a mais;
- sem nativo: como hoje.

Isso faz do "forçar" uma ferramenta para quem **sabe** que o nativo é ruim (PDF de texto trocado, fonte quebrada que o app não detectou); numa página de texto bom, o OCR é um pouco pior que o nativo, e a interface avisa isso. A alternativa mais ambiciosa (nativo como base e só entram linhas que só o OCR viu, com um algoritmo próprio de comparação) foi descartada pelo dono na Q5, que escolheu a regra simples acima.

**Custo extra:** cada página duplicada (H7) também faz OCR, porque a duplicata só é decidida depois, no formatter. Aceito e dito ao usuário; evitar isso exigiria decidir H7 antes do OCR e não vale a complexidade.

### Onde se liga

Decisão do dono (Q4, opção b): nos dois lugares.

- **Global, nas Configurações:** campo `models.Config.ForcarOCR` (`bool`, JSON `forcar_ocr`), padrão `false` em `DefaultConfig()`. Como todo campo, `Load`/`LoadFromFile` o completam a partir do padrão quando falta no `config.json` (config antiga carrega com `false`). `Settings.svelte` ganha a caixa "Forçar OCR em todas as páginas de PDF", com o aviso de custo de tempo e de leve perda em página boa; desabilitada quando o modo de OCR é `none`. Salva por `SaveConfig`. **Com `ocr_mode = none`, `forcar_ocr` não tem efeito:** a caixa fica desabilitada enquanto o modo for `none`, o valor salvo é **preservado** (não desmarca nem recusa; voltando a um modo com OCR, a caixa reaparece como estava), e o pipeline ignora `forcarOCR` com o OCR desligado. Vale igual para a config global e para o override do Reprocessar (inclusive global `true` com o Reprocessar em `none`).
- **No Reprocessar, por item:** `ReprocessarItem.svelte` ganha a caixa "Forçar OCR em todas as páginas", cujo valor inicial é o da config global; `Queue.Reprocessar` e `App.ReprocessarItem` ganham `forcarOCR bool`, guardado em `ajusteConfig` (como ponteiro ou par valor/definido, para distinguir "não mexeu" de "desligou"). **Precedência:** o valor do Reprocessar, quando enviado, vale sobre a config global **nessa execução**, nos dois sentidos (pode forçar com a global desligada e pode desligar com a global ligada); o `ajusteConfig` é aplicado sobre a cópia da config (`queue.go:533-536`), então **a config global e o `config.json` não mudam ao reprocessar**. Um ZIP novo (não reprocessado) usa a global. Exige `wails generate module` e atualizar `frontend/reprocessar.test.mjs`.
- Nos dois lugares o aviso diz que cada página duplicada (H7) também faz OCR.
- O botão "Ler com OCR" do aviso (Parte 1) resolve páginas `sem_ocr` (liga o OCR). É coisa diferente de forçar em tudo.

### Custo de tempo

O OCR é a fase mais cara: um `tesseract` por bloco, limitado a `NumCPU()-1`. Um PDF nativo de 100 páginas, hoje em segundos, passa a levar o tempo de 100 páginas de OCR. A interface avisa antes de confirmar; o relatório mostra o tempo de trabalho de OCR depois. A medição real fica para a implementação, com o corpus sintético do `scripts/bench`.

## Parte 3: arquivo de log

### Interruptor: variável de ambiente `EPROCIA_LOG`

Decisão do dono (Q6) e ruling do orquestrador: **log e telemetria só existem quando a variável de ambiente `EPROCIA_LOG` está definida**.

| Valor | Efeito |
|---|---|
| ausente ou vazio | **desligado**: nenhum arquivo nem pasta de log/telemetria é criado, o `log` padrão continua indo ao stderr, a interface não mostra nada sobre log. É o comportamento de hoje |
| `1` | log normal (`INFO`, sem registro por página) e telemetria |
| `detalhado` | o mesmo, mais o registro por página (`DEBUG`, Q7) |
| qualquer outro valor | tratado como `1`, com um `WARN` no próprio log dizendo que o valor não foi reconhecido |

A variável é lida **uma vez, na inicialização** (em `main.go`, depois da trava de instância). Mudar o valor exige reiniciar o app (inclusive sair pela bandeja).

**Como o dono define a variável:** nas Variáveis de Ambiente do usuário do Windows (Configurações do sistema) ou por `setx EPROCIA_LOG detalhado` (ou `1`). Uma variável de usuário só chega a **processos novos**: o app já aberto não a vê; é preciso "Sair" pela bandeja e abrir de novo. Quem inicia o app por atalho, pelo autostart (`HKCU\...\Run`) ou pelo `install.ps1` herda o ambiente do Explorer, que só relê a variável depois de reiniciar o Explorer ou sair e entrar no Windows; um app lançado de um shell ou tarefa agendada já aberto herda o ambiente antigo. O plano atualiza o README e o CLAUDE.md com esse procedimento e esse caso (bandeja e autostart). Não há `log_nivel` no `config.json` nem tela para isso. A decisão é exposta ao resto do app por um pacote só (`internal/registro`), que também diz se está ativo (ver Q8).

### Local, nome, rotação e retenção

- Só com a variável definida. Pasta: `%AppData%\eprocIA\logs\`, criada só então. Arquivo ativo `eprocia.log`. Rotação por tamanho, **própria, sem dependência nova**: ao passar de 5 MB vira `eprocia.1.log`, os anteriores se deslocam, no máximo 5 arquivados (cerca de 30 MB no pior caso). Retenção: arquivos com mais de 30 dias são apagados na abertura. (Decisão técnica do autor: uma dependência só para isso não se justifica, e o app é distribuído como executável único.)
- O registro é **iniciado depois da trava de instância** (`main.go:21-30`): a segunda cópia do app, que só avisa e sai, não pode rodar rotação nem retenção no arquivo da primeira (o Windows não renomeia arquivo aberto, e a rotação falharia no meio). Antes disso, e se a trava falhar, o `log` continua indo ao stderr, como hoje. O `log.Fatal` de `main.go:80` passa a descarregar o registro antes de sair (ele faz `os.Exit` sem rodar nada).
- Falha ao abrir a pasta ou o arquivo nunca impede o app de iniciar: cai no stderr, e o app passa a se comportar como "log desligado" (a interface também não mostra o botão, pois `LogAtivo` devolve `false`).
- **Botão "Abrir pasta de logs"** em Configurações, no mesmo `explorer.exe` que o app já usa, **só com o log ativo** (Q8, abaixo).

### Formato

**JSON Lines**, com `log/slog` (`slog.NewJSONHandler`), uma linha por evento. Decisão técnica do autor: filtra com `jq`, Python ou planilha ("todos os erros de OCR do ZIP tal"), não precisa de regex em texto livre, não precisa de dependência. A mensagem `msg` fica em português (convenção do projeto).

### Níveis e volume

`DEBUG`, `INFO`, `WARN`, `ERROR`. Com `EPROCIA_LOG=1` o nível é `INFO`; com `EPROCIA_LOG=detalhado` é `DEBUG`. Não há outro caminho para mudar o nível.

O volume atual não cabe em `INFO`: as ~3 linhas por página do `pdf_parser.go` somariam milhares de linhas por ZIP grande e encheriam 5 MB × 5 com um só ZIP. Regra: **nenhuma chamada por página (nem por atualização de progresso) fora de `DEBUG`**, com um teste que varre o código-fonte à procura de `log.Printf` por página fora da lista. Antes de apontar o pacote `log` para o novo arquivo (`slog.SetDefault`), convertem-se para `Debug` as chamadas por página de `pdf_parser.go` (`:108,134,147,168,182,366-373,375-397` e as do extrator posicional `:550,560,566,570`), de `pdf_poppler.go`, de `pdf_ocr_despacho.go:123,176` e o progresso de `queue.go:492` (uma linha a cada atualização). Fica de fora e **sobe para `WARN`** a linha do Form XObject ignorado (`pdf_parser.go:374`), a única linha de log que a v0.4.2 acrescentou por decisão do dono. Em `INFO` ficam só as de documento e de ZIP. Resultado: um ZIP de 500 documentos gera algumas centenas de linhas (da ordem de 150 KB) em `INFO`. O log por página sempre ligado reverteria a decisão do dono na v0.4.2; por isso (Q7) o registro por página só existe com `EPROCIA_LOG=detalhado`, que o dono liga quando precisar.

### O que registra

Campos fixos em toda linha: `ts` (ISO 8601 com fuso), `nivel`, `msg`, `item_id` (o `ID` uuid do item da fila, `queue.go:192`; o mesmo da telemetria; serve só para correlacionar linhas de workers concorrentes e ligar log e telemetria, não substitui nenhum nome), `doc_id` (índice do arquivo no ZIP, que distingue entradas de mesmo nome) e `pag` quando couber.

- **Início do app:** versão (de `wails.json`), valor de `EPROCIA_LOG` reconhecido (e o aviso se desconhecido), versão do Windows, nº de CPUs, tamanho dos dois semáforos (`NumCPU()-1`), pasta de logs e a **configuração efetiva** (inclui `forcar_ocr`), já saneada (a pasta monitorada aparece com `%USERPROFILE%` no lugar do perfil).
- **ZIP:** **nome do ZIP** (e o `item_id`), origem (monitorada, arrastada, manual), tamanho, nº de PDF e HTML, config efetiva do item (com o ajuste do Reprocessar e `forcar_ocr`), duração, resultado, tamanho do `.txt`, tokens e o relatório final (Parte 1) como objeto.
- **Documento:** nome do documento no ZIP, evento, extensão, tamanho, nº de páginas, leitor usado (biblioteca ou Poppler), estado final, `Leitura` e `Detalhe`, duração, páginas por categoria.
- **Página (só com `EPROCIA_LOG=detalhado`):** categoria, motivo de OCR (`motivoPadrao/Ilegivel/ImagemDescoberta/Isca/Forcado`), bytes de texto nativo, se tem imagem ou desenho vetorial, as métricas de `metricasPagina`, se manteve o nativo, se houve junção. Tempo de OCR **por bloco**, não por página.
- **OCR:** bloco ou página a página, páginas do bloco, duração, e se foi preciso refazer página a página (`pdf_ocr_despacho.go:195-211`). **Código de saída e stderr** do `tesseract`/`pdftoppm` exigem mudança em `internal/ocr` (`tesseract.go`, `prazo.go`): hoje o erro volta como `error`; o stderr passa a ser devolvido no erro para entrar no log (nível `WARN`/`ERROR`), **nunca** no `.txt`, que só leva a frase fixa (regra do PRD e do CLAUDE.md).
- **Erros:** a cadeia completa (`%w`), a etapa (`abrir_zip`, `ler_pdf`, `ocr`, `html`, `formatar`, `gravar`), a classe (`classificarErro`) e, em `panic`, a pilha (hoje só uma linha, `pipeline.go:583`).
- **Limpeza:** nível, bytes antes e depois, duplicatas e tipos omitidos.
- **Fila e janela:** pausa, retomada, cancelamento, reprocessar, falha ao gravar o `.txt`, watchdog, bandeja e notificações (`app.go:101-134`).

### Migração do `log.Printf`

As ~112 chamadas viram `slog` com campos, em etapas. Para nenhuma se perder no meio, `slog.SetDefault` redireciona o pacote `log` ao mesmo manipulador, **mas só quando `EPROCIA_LOG` está definido e só depois** de duas coisas: o saneamento (abaixo) estar no manipulador; e as chamadas por página estarem em `Debug`. Com a variável ausente, `SetDefault` não é chamado e o `log` segue para o stderr como hoje: as linhas que continuam em `log.Printf` saem no stderr como antes; as chamadas por página convertidas para `Debug` deixam de sair no stderr (mudança aceita: o stderr de um executável Wails sem console já se perde). **Custo zero sem a variável:** fica o manipulador padrão do `slog` (que escreve pelo pacote `log` no stderr, no formato de hoje) com nível mínimo `Info`, então só `Debug` é descartado na checagem de `Enabled`, sem formatar; as chamadas migradas em `Info`, `Warn` e `Error` continuam saindo no stderr como hoje (teste: sem a variável, com `log.SetOutput` de teste, `Info`/`Warn`/`Error` chegam e `Debug` não); a telemetria e a coleta de máquina (memória, versão do Windows) **não executam**; nenhuma goroutine de gravação nasce. O relatório da interface continua calculado, porque são só contadores baratos. Como o dono aceitou todos os nomes no log (Q6), as linhas com nome de ZIP ou de documento **não** precisam ser trocadas por código; o que precisa sair delas é o **caminho do usuário**: `p.filePath` (arquivo temporário sob o perfil) em `pdf_parser.go:129,134,165` e `pdf_poppler.go:29,46,89`, e `MonitoredPath` em `app.go:91,294`, passam a sair saneados (pelo manipulador) ou só com o nome do arquivo.

## Parte 4: telemetria local

### Arquivo

Só com `EPROCIA_LOG` definido (qualquer valor). Sem a variável, nem a pasta nem o arquivo são criados. `%AppData%\eprocIA\telemetria\telemetria.jsonl`, **um registro por ZIP processado** (também por reprocessamento, com `reprocesso: true`), gravado no fim do ZIP, por acréscimo (`O_APPEND`), uma linha por escrita. Rotação própria: ao passar de 10 MB vira `telemetria.1.jsonl`, com no máximo 5 arquivados. Um registro tem cerca de 1 KB, ou seja, dezenas de milhares de ZIPs antes de passar do primeiro arquivo; sem apagar por idade, para o dono ter a série longa. Decisão técnica do autor.

**Sem chave nas Configurações.** O interruptor é a mesma variável do log (ligada, a telemetria acompanha). Com o log ativo, o texto das Configurações, ao lado do botão da pasta de logs, diz "estes arquivos ficam só neste computador"; sem log, nada aparece.

### Campos (sem nomes)

A telemetria continua **sem nomes** (de ZIP, de documento, de pasta), mesmo que o log os leve: ela é a parte pensada para ser agregada e partilhada.

- Versão do app, `ts`, `item_id` (o mesmo uuid do log, que não deriva do nome do arquivo), `reprocesso`.
- Máquina: nº de CPUs, memória total, versão e build do Windows, arquitetura. Sem nome do computador nem do usuário.
- Configuração: modo de OCR, nível de limpeza, `forcar_ocr` (efetivo no ZIP, global ou do Reprocessar), quantidade (não os nomes) de tipos omitidos, origem do ZIP.
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

- **Quando existem:** só com `EPROCIA_LOG` definido, por escolha de quem roda o app. Sem a variável, nada é gravado.
- **Onde ficam:** só em `%AppData%\eprocIA\logs` e `...\telemetria`, fora do repositório e do `.txt`. Defesa em profundidade: `*.log` e `*.jsonl` no `.gitignore` e no hook `proteger-dados` (`.claude/hooks/proteger-dados.mjs`, `arquivoSensivel`), com as mesmas exceções de pasta de documentação que o hook já tem para `.txt`, e teste em `.claude/hooks/hooks.test.mjs`.
- **O log atual NÃO é partilhável como está.** Ele grava `p.filePath` (arquivo temporário sob o perfil do usuário, `pdf_parser.go:129,134,165`), a configuração efetiva inclui `monitored_path`, e a cadeia de erro (`%w`) e o stderr do Tesseract trazem caminhos. Por isso o **manipulador faz o saneamento**, em toda mensagem e em todo atributo de texto ou de erro.
  - **É saneado** (dado do usuário do Windows que não é nome de arquivo processado): o perfil do usuário vira `%USERPROFILE%`, o diretório temporário vira `%TEMP%`, e o nome do usuário **só quando aparece como segmento de caminho** depois de `\Users\` ou `/Users/` vira `<usuario>` (trocar o nome solto mutilaria palavras comuns se o usuário tiver nome curto). Vale para a pasta monitorada, caminhos de arquivos temporários e caminhos dentro de erros e do stderr.
  - **NÃO é saneado**, por decisão do dono (Q6): o nome do ZIP, o nome de cada documento dentro do ZIP e o nome do `.txt` gerado. Eles saem como são. Se o nome do ZIP vier dentro de um caminho completo, o trecho do perfil é trocado e o nome do arquivo permanece.
  - **Nunca entra**, em nenhum nível: texto de página (nem trecho nem hash; o hash de H7 não é gravado).
- **Nomes no log (Q6, opção b):** o log leva **todos os nomes**, inclusive o do ZIP, que costuma ser o número do processo e identifica as partes. É mais cômodo para depurar e mais difícil de partilhar; por isso o log só existe quando quem roda o app escolhe ligá-lo pela variável. Não há código de ZIP, índice de nomes nem pasta separada de nomes.
- **Telemetria:** nunca leva nome de ZIP, de documento, de pasta nem texto de página. Só contagens, tempos e classes.
- **Como o dono partilha um log:** pelo botão (com log ativo), pega `eprocia.log`, que já não tem o nome do usuário do Windows mas **tem os nomes dos ZIPs e documentos**. Para partilhar sem eles, um script opcional em `scripts/` (`anonimizar-log`) troca nomes de ZIP e de documento por rótulos estáveis (`zip-3`, `doc-17`) e confere se sobrou algum caminho de usuário.

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

- **Novos:** `internal/registro` (leitura única de `EPROCIA_LOG`, manipulador `slog` assíncrono, saneamento, rotação, retenção; `Ativo()` e `Detalhado()`), `internal/telemetria` (registro por ZIP, coleta da máquina), `frontend/src/components/RelatorioItem.svelte`, `frontend/src/lib/relatorio.js`, `scripts/anonimizar-log`.
- **`internal/etl`:** `RelatorioZip` e o campo `categoria` na página; total de páginas `sem_ocr` para o cabeçalho (parâmetro novo de `GenerateConsolidatedOutput`, linha em `formatter.go:467`); contadores de limpeza no laço de saída de `formatter.go` (não em `higienizacao.go`); tempos de fase em `pipeline.go`; `motivoForcado` em `pdf_juncao.go` e em `pdf_ocr_despacho.go`; `forcarOCR` em `Queue`, `ajusteConfig` e `ProcessZip`; conversão dos `log.Printf` por página para `Debug` em `pdf_parser.go`, `pdf_poppler.go`, `pdf_ocr_despacho.go`.
- **`internal/ocr`** (`tesseract.go`, `prazo.go`): devolver código de saída e stderr no erro, para o log.
- **`internal/config` e `internal/models`:** `ForcarOCR` (`forcar_ocr`, padrão `false`, completado por `DefaultConfig()` no `Load`; valor preservado e sem efeito com `ocr_mode = none`). Não há `LogNivel`.
- **`app.go` e `main.go`:** iniciar o registro depois da trava de instância e só com `EPROCIA_LOG`; descarregar em `OnShutdown`, `App.Quit` e antes do `log.Fatal`; métodos `LogAtivo() bool` (verdadeiro só se a variável está definida **e** o arquivo abriu) e `AbrirPastaLogs()` (que não faz nada, ou devolve erro em português, com o log inativo); `ReprocessarItem` com `forcarOCR`. Exige `wails generate module`.
- **Frontend:** `ProcessList.svelte`, `ReprocessarItem.svelte`, `Settings.svelte` (caixa "Forçar OCR" e, só com `LogAtivo()`, o botão e o texto de log). `App.svelte` chama `LogAtivo()` uma vez ao iniciar e repassa a `Settings.svelte`.
- **Docs e harness:** `PRD.md` (§6, a linha nova do cabeçalho e a explicação; e o novo relatório), documentação da variável `EPROCIA_LOG` (README e CLAUDE.md), `DESIGN.md`, `CLAUDE.md`, `.gitignore`, `.claude/hooks/proteger-dados.mjs` e `hooks.test.mjs`.

## Testes

**Go:**

- Categorias: um PDF sintético por categoria (texto nativo; escaneado com OCR; escaneado com OCR desligado; página sem texto e sem imagem; falha de OCR com e sem nativo; leitura que falha; página nula; caminho do Poppler) e a verificação de que cada página cai em **exatamente uma** categoria e a soma das cinco é igual ao total, inclusive com duplicatas H7, documentos omitidos e não lidos no mesmo ZIP.
- Página com pouco texto e sem imagem nunca é `sem_ocr` nem `com_erro` (o ponto do dono); com OCR desligado, página de imagem descoberta ou isca fica `lida_nativa` (as regras novas não agem sem OCR).
- Sub-contadores: `ocr_falhou_com_reserva` igual ao número de páginas com `ocr_aviso` (inclusive as que também têm `ocr_erro`, que não contam em `com_erro`); `ocr_sem_ganho` igual ao das páginas que foram ao OCR e ficaram com o nativo sem falha.
- Limpeza: "antes" e "depois" calculados no laço de saída, com um teste que recalcula a soma independente a partir do texto das páginas emitidas; `antes >= depois`; redução 0 em `none`; páginas duplicadas e `sem_ocr` fora das somas.
- Forçar OCR, com o `ocrRunner` injetável: toda página de PDF de documento lido é despachada; OCR legível substitui o nativo; OCR em branco, ilegível ou com falha mantém o nativo (com `ocr_aviso` na falha, sem `ocr_erro`); com `ocr_mode = none` (global ou override) o `forcarOCR` é ignorado e nenhuma página vai ao OCR; HTML e omitidos não são afetados; o ajuste do Reprocessar não muda a config global.
- **Forçar OCR global e override:** `ForcarOCR` ausente no `config.json` carrega como `false` (backfill); com `ForcarOCR = true` na config, um ZIP novo leva todas as páginas de PDF ao OCR; `Reprocessar` com `forcarOCR = false` sobre global `true` não força, e com `true` sobre global `false` força; depois de qualquer Reprocessar, `config.json` e a config em memória continuam como estavam; `ForcarOCR = true` com `ocr_mode = none` é salvo e carregado como está (preservado) e não tem efeito; Reprocessar em `none` com global `true` também não força; no teste da interface, a caixa fica desabilitada com `none` e mantém o valor.
- **Botão "Ler com OCR":** chama `Reprocessar` com `forcarOCR = false` explícito (mesmo com a global `true`), modo de OCR igual ao global se não for `none`, senão `normal`, e o nível de limpeza do item; teste com global `forcar_ocr = true` confirmando que só as páginas candidatas vão ao OCR.
- **Linha do cabeçalho (Q3):** com N páginas `sem_ocr` > 0 o cabeçalho tem `Páginas que precisavam de OCR e ficaram sem: N` entre `Documentos não lidos` e `Data de Consolidação`; com N = 0 a linha não existe e a saída é idêntica à de hoje (teste de golden); `Tokens estimados` inclui a linha.
- **Variável ausente (log desligado):** com `EPROCIA_LOG` não definida ou vazia, processar um ZIP sintético não cria `logs\` nem `telemetria\` (nem arquivos) sob uma pasta de dados temporária, `LogAtivo()` é `false` e o pacote `log` não é redirecionado.
- **Valores:** `1` liga log e telemetria em `INFO` sem nenhuma linha por página; `detalhado` liga `DEBUG` e registra as páginas (categoria, motivo de OCR); valor desconhecido liga como `1` e grava um `WARN` de valor não reconhecido; mudar a variável depois da inicialização não tem efeito.
- Log: arquivo criado, linha JSON válida com os campos fixos, rotação aos 5 MB, retenção por idade e por quantidade, descarga no encerramento, e teste de concorrência com `-race` e dezenas de goroutines.
- **Saneamento:** um teste grava mensagens, atributos e erros com o perfil do usuário e o diretório temporário e confirma que nenhum aparece no arquivo.
- **Não bloqueia:** com um `io.Writer` que trava e canal cheio, `Info`, `Warn` e `Error` voltam em tempo curto e `linhas_perdidas` sobe; com `Writer` que devolve erro, nada sobe ao chamador e a gravação volta depois dos 60 s (relógio injetável).
- Privacidade: o log de um ZIP sintético com nome falso **contém** o nome do ZIP e o dos documentos (decisão do dono) mas não texto de página; a telemetria não contém nome de ZIP nem de documento nem texto de página; o pacote de telemetria não importa `net`/`net/http`.
- Falha ao criar a pasta de logs (somente leitura) não impede o início; a segunda instância não toca no arquivo da primeira.

**Frontend (`frontend/*.test.mjs`):**

- `relatorio.test.mjs`: `src/lib/relatorio.js` formata a linha-resumo e as frases; o aviso só aparece com `sem_ocr`, `com_erro` ou `ocr_falhou_com_reserva`, nunca para "em branco".
- Leitura dos `.svelte` por regex, como nos testes atuais: caixa "Forçar OCR" em `ReprocessarItem.svelte`, desligada por padrão e desabilitada com OCR desligado; caixa "Forçar OCR" em `Settings.svelte`, desabilitada com OCR desligado; botão "Ler com OCR" no item com aviso;
- **UI sem log não mostra nada de log:** o botão "Abrir pasta de logs" e qualquer texto sobre log/telemetria em `Settings.svelte` ficam atrás de `{#if logAtivo}`, com `logAtivo` vindo de `LogAtivo()` e `false` por padrão; um teste por regex, **restrito à marcação do template** (ignora `<script>`, `<style>`, comentários e palavras como "login"), confirma que nenhuma ocorrência de "log" ou "telemetria" fica fora desse bloco, e um teste de lógica confirma que `logAtivo = false` não renderiza o botão nem o texto (e que `true` renderiza os dois); o painel usa as classes de tema e não cor fixa (`contraste.test.mjs`, `paleta.test.mjs`).
- `reprocessar.test.mjs` atualizado.
- Hooks: `hooks.test.mjs` cobre o bloqueio de `git add` de `.log` e `.jsonl`.

## Critérios de aceite

1. Ao fim de um ZIP, o item mostra as páginas por categoria, e a soma das cinco é igual ao total de páginas dos documentos lidos (definido acima); duplicatas, omitidos e não lidos aparecem à parte.
2. Nenhuma página sem imagem aparece como "precisava de OCR e ficou sem" nem como erro.
3. Com o OCR desligado, páginas candidatas aparecem no aviso; o item continua `success`.
4. O painel mostra tempo total e tempo de trabalho de OCR, bytes antes e depois da limpeza com a redução %, duplicatas e tipos omitidos; "depois" é igual à soma do texto das páginas emitidas (teste independente).
5. "Forçar OCR" (no Reprocessar, por item, e nas Configurações, para todos) leva todas as páginas de PDF ao OCR; o override do Reprocessar vale sobre a global só naquela execução e **não muda** a configuração global; `forcar_ocr` padrão `false`; com um OCR injetado, o resultado segue a regra da Parte 2 (substitui se legível, mantém o nativo se vazio, ilegível ou falho).
6. Com `EPROCIA_LOG` definida, existe `%AppData%\eprocIA\logs\eprocia.log` em JSON Lines com versão, configuração efetiva saneada, cada ZIP (com o nome), cada documento (com o nome) e os erros com etapa, classe e causa; `detalhado` acrescenta o registro por página, `1` não; a rotação e a retenção funcionam; nenhum caminho com o usuário do Windows aparece.
7. Com `EPROCIA_LOG` definida, existe `telemetria.jsonl` com um registro por ZIP, com os campos listados e sem nome de ZIP, de documento, de pasta nem texto.
7a. **Sem `EPROCIA_LOG`** (ausente ou vazia): nenhum arquivo nem pasta de log ou telemetria é criado, o comportamento é o de hoje, e a interface não mostra botão, texto ou configuração de log.
7b. Com N = 0, o `.txt` é idêntico ao de hoje; com N > 0, o cabeçalho tem a linha `Páginas que precisavam de OCR e ficaram sem: N` e o PRD §6 a descreve.
8. Nenhuma chamada de rede foi acrescentada; o log e a telemetria nunca travam um worker, mesmo com disco cheio.
9. O bench não piora o pipeline (p < 0.05, nenhuma métrica pior que 5%); `go test -race ./...`, `go vet`, `svelte-check`, `npm run build` e o glob do frontend passam.
10. `*.log` e `*.jsonl` estão no `.gitignore` e no hook `proteger-dados` (aprovado pelo dono em 2026-11-23).

## Decisões técnicas do autor (não são perguntas)

JSON Lines com `slog` (formato); rotação própria do log, 5 MB × 5 arquivos e 30 dias (sem dependência); telemetria em arquivo único com rotação a 10 MB e 5 arquivados, sem apagar por idade; telemetria ligada junto com o log, sem chave própria; `item_id` igual ao uuid do item nos dois arquivos; variável `EPROCIA_LOG` lida só na inicialização; `LogAtivo()` exposto ao frontend.

## Decisões do dono, 2026-11-23

- **Q1:** linha-resumo no item concluído e painel de detalhes ao clicar.
- **Q2:** aviso na linha e botão "Ler com OCR", que refaz o ZIP com o OCR ligado.
- **Q3 (b):** uma linha no cabeçalho do `.txt`, "Páginas que precisavam de OCR e ficaram sem: N", só quando N > 0 (entre `Documentos não lidos` e `Data de Consolidação`; PRD §6 atualizado).
- **Q4 (b):** "Forçar OCR" no Reprocessar (por item) e nas Configurações (todos os ZIPs); `Config.ForcarOCR`, padrão `false`; o Reprocessar vale sobre a global só naquela execução e não a altera.
- **Q5:** o OCR legível substitui o texto do PDF; o texto do PDF fica se o OCR vier em branco, ilegível ou falhar.
- **Q6 (b):** todos os nomes no log, inclusive o do ZIP. Dono: "só vamos ativar os logs se houver uma determinada configuração nas variáveis do ambiente. Assim, só serão geradas por quem efetivamente escolher." Ruling do orquestrador: log e telemetria só existem com `EPROCIA_LOG` definida (ausente/vazio desligado; `1` normal; `detalhado` com registro por página; valor desconhecido = `1` com aviso; leitura só na inicialização). Sem código de ZIP, índice nem pasta separada de nomes. Saneamento: só o perfil do usuário do Windows (perfil, `%TEMP%`, nome após `\Users\`), nunca nomes de ZIP ou documento.
- **Q7:** registro por página só no modo `detalhado` da variável.
- **Q8:** "Só quando os logs estiverem ativados, caso contrário, não deve mostrar nada sobre log." Botão "Abrir pasta de logs" só com log ativo, informado por `App.LogAtivo() bool`; sem log, nenhum texto ou controle de log na interface.
- **Q9 (a):** tudo junto, numa release só.
- **Q10:** aprovado estender o hook `proteger-dados` e o `.gitignore` para barrar `*.log` e `*.jsonl`.

## Riscos

- O contador de "antes e depois" da limpeza fica no laço de saída de `formatter.go`, que tem muitos testes; a mudança é só somar na entrada e na saída da página.
- Forçar OCR (inclusive o global) pode dobrar o tempo e deixar a página boa um pouco pior; a interface avisa, e o texto do PDF fica se o OCR falhar. A global é fácil de esquecer ligada; o texto da caixa diz o custo.
- Com o log ligado, ele leva nomes de ZIP e de documento (identificam as partes): só quem define `EPROCIA_LOG` o produz, e partilhar exige o script `anonimizar-log` ou aceitar os nomes. O nome do usuário do Windows continua saneado no manipulador, e o teste dele é critério de aceite.
- O `item_id` é o mesmo no log (que leva nomes) e na telemetria (sem nomes): quem partilhar os dois arquivos permite ligar a telemetria aos nomes. Orientação: partilhar **só a telemetria** quando não quiser expor nomes; sem mudança de código.
- O pacote `log` ainda é usado por código não migrado: a migração por etapas e a ordem (saneamento e `Debug` primeiro, `SetDefault` depois, e só com a variável) importam.
- Mudar `internal/ocr` para devolver stderr toca o código que roda os processos externos (prazos, `CREATE_NO_WINDOW`): cobrir com os testes existentes do pacote.
- A linha extra no `.txt` (Q3) altera um contrato que o PRD §6 descreve e que a análise em LLM usa; só sai com N > 0, então a saída comum não muda.
- Release única (Q9): a implementação é grande; o plano deve dividir em tarefas, mas o corte da release é um só (pelo menos o PRD, o `.txt` e a interface saem juntos).
