# Histórico da fila ao reabrir o eprocIA

Data: 2026-11-18. Base lida: worktree `revisao-2026-11-10` (main com a v0.4.2). Revisada por um revisor sem contexto em 2026-11-18 (aprovada com ajustes, todos aplicados ou respondidos na seção 11). Decisões do dono h1 a h4 (2026-11-19) e e6 (2026-11-20) aplicadas (seção 10). Ajustada em 2026-11-20 depois de uma segunda revisão sem contexto, conferida no código da `main` com a rodada 2026-11-10 já juntada (seção 11); não há pontos em aberto.

Pedido do dono: "Quando o usuário reabrir o programa, quero que mostre o histórico de documentos de quando fechou."

## 1. Problema e comportamento esperado

Hoje a fila vive só na memória (`Queue.items` em `internal/etl/queue.go`). Ao sair pela bandeja ("Sair"), ao reiniciar o Windows ou ao atualizar o programa, a lista some. O usuário perde o atalho para copiar o TXT de um processo que converteu ontem e precisa ir ao Explorer procurar o arquivo.

Comportamento esperado:

- **O histórico é opcional e nasce desligado** (h3). Em Configurações, o interruptor "Guardar a lista da última sessão". Desligado, nada muda em relação a hoje: nenhum arquivo é gravado e a lista abre vazia.
- **Guarda só a última sessão** (h2). Com o interruptor ligado, ao abrir, a lista mostra os itens da sessão anterior, na mesma ordem em que estavam: o mais novo em cima. Sessões mais antigas não aparecem. "Sessão" é o período entre abrir e fechar o programa; uma sessão em que nada foi processado não apaga a anterior (e6).
- **Na mesma lista, com data e hora na linha** (h4): "14:32" para hoje, "ontem 14:32", "12/11 14:32" para dias anteriores. Hoje `formatarHora` (`frontend/src/lib/fila.js`) mostra só a hora, o que fica ambíguo quando a sessão anterior foi em outro dia. Os itens restaurados ficam abaixo dos novos, sem separador.
- Cada item concluído continua útil: "Copiar TXT", "Abrir pasta" e "Reprocessar" funcionam como antes de fechar, se os arquivos ainda existirem.
- **Interrompido** (h1): o item que estava processando ou aguardando quando o programa fechou aparece como "Interrompido", com a ação "Processar de novo". Com o monitoramento ligado, a varredura inicial do watchdog põe de novo na fila, sozinha, todo ZIP das últimas 24 h sem `.txt` mais novo (`internal/watchdog/pendentes.go`), e isso inclui o interrompido: nesse caso ele volta a processar sozinho, na mesma linha.
- Se o TXT foi apagado ou movido, a linha diz "TXT não encontrado" e o botão de copiar fica desativado; se o ZIP também sumiu, "Reprocessar" fica desativado com o motivo no `title`.
- Remover um item (X) o apaga também do histórico. "Limpar tudo" apaga o histórico inteiro (com a confirmação que F-AC-06 cria).
- O estado global de pausa não é restaurado: o programa sempre abre com a fila rodando, como hoje.
- Não se guarda o painel de atividade por documento (`etl.Atividade`): num item restaurado, o painel diz "Detalhes por documento só ficam disponíveis durante a sessão em que o item foi processado". O número de não lidos (`unread_docs`) é guardado e continua aparecendo.

## 2. O que muda em cada camada

### 2.1 Go

**Regra do caminho de saída num pacote folha.** A regra "entrada para `.txt`" (hoje `pipeline.go:736`, e a do PDF avulso se ele entrar) sai de `internal/etl` para um pacote folha, `internal/saida`, importado por `etl`, `watchdog` e `historico`. Assim `historico` não importa `etl` e não há ciclo de import. Com a decisão p1 do PDF avulso (spec `2026-11-18-pdf-avulso-design.md`), o TXT de um PDF pode ser `<nome>_2.txt`; e o TXT de um item de vários PDFs tem o nome do processo, não o do primeiro PDF (e2). Por isso a validação usa `saida.SaidaDoItem(kind, path, paths, saida)`, a mesma função de `lerSaidaParaCopiar` (ZIP: `<nome>.txt`; PDF único: `<nome>.txt` ou `<nome>_N.txt`; vários PDFs: conferido contra `NomeLote(paths)`), não a igualdade com um caminho único. **O pacote `internal/saida` é entregue pela spec do PDF avulso** e não entra na estimativa desta; se o histórico for implementado antes, a tarefa do pacote `saida` daquela spec vai na frente.

**Regra de ordem entre as duas specs.** A reutilização da linha interrompida e `Restaurar` gravam `saidaAnterior`, campo de `QueueItem` definido pela spec do PDF avulso (seção 2.1, queue), junto com `Entrada.SaidaAnterior`. Se o histórico for implementado antes, essas duas escritas não existem ainda: a linha reaproveitada e `Restaurar` simplesmente não preenchem `saidaAnterior`, e a tarefa do PDF avulso, ao criar o campo, passa a preenchê-las (teste de cada uma entra com ela). Se o PDF vier antes, o histórico usa o campo já existente.

**Pacote novo `internal/historico`** (`historico.go`, `historico_test.go`), que não importa `etl`:

- `type Item`: só os campos persistidos (seção 3), sem `cancel`, `ajuste`, `atividade` nem campos do limitador.
- `Carregar(arquivo string) ([]Item, error)`: lê, valida, aplica o teto de itens e devolve do mais novo para o mais antigo. Arquivo ausente devolve lista vazia sem erro. JSON corrompido: renomeia para `historico.json.corrompido-AAAAMMDD-HHMMSS`, registra no log e devolve lista vazia (nunca impede a abertura). Versão de formato maior que a conhecida: ignora o arquivo e não o sobrescreve até o usuário mexer na fila (um downgrade não apaga o histórico de uma versão nova).
- `Gravador`: dono único da escrita. Tem um mutex que serializa escrita e rename, um timer de debounce de 1 s e um estado `fechado`.
  - `Agendar(itens)`: guarda a última lista e (re)arma o timer. Ignorado se `fechado`.
  - `GravarAgora(itens)`: para o timer e grava na hora, sob o mutex. A lista que fica no arquivo é a da última escrita concluída sob o mutex; como `Agendar` e `GravarAgora` concorrentes não têm ordem definida, o contrato é: o arquivo é sempre um JSON válido igual a uma das listas recebidas e, depois de uma chamada final sem concorrência (`GravarAgora`, ou `Agendar` seguido do debounce), igual à lista dela.
  - `Fechar(itensFinais)`: sob o mutex, para o timer, marca `fechado` e faz a escrita final. Depois disso nenhum `Agendar` ou `GravarAgora` grava. Usado pelo Shutdown, pelo desligar do histórico (h3, com `itensFinais` nil, que apaga o arquivo) e pela troca de versão do autoupdate.
  - `Reabrir()`: desfaz o `fechado` (só a reversão do autoupdate usa).
  - A escrita usa `os.CreateTemp` na pasta do histórico e renomeia por cima (`os.Rename` no Windows substitui o destino). Se o rename falhar (antivírus segurando o arquivo), tenta de novo uma vez após 200 ms; falhando, registra no log, apaga o temporário e segue.
- `Podar(itens, teto)`: aplica o teto de 500 itens (seção 3.3).
- Validação ao carregar, item a item (o inválido é descartado, o resto fica): `Path` (e cada um de `Paths`, no item de vários PDFs) absoluto com extensão aceita pela fila; `OutputPath` vazio ou com `saida.SaidaDoItem(Kind, Path, Paths, OutputPath)` verdadeiro (`Kind` derivado do `Path` quando ausente); status num conjunto conhecido. É defesa em profundidade, não barreira: quem edita o JSON também escolhe o `Path` (e quem escreve em `%APPDATA%` já roda como o usuário). O ganho é que um arquivo estragado ou montado à mão não faz `CopyOutput` (`app.go`, `lerSaidaParaCopiar`, que confia em itens com status `success`) ler um `.txt` sem relação com a entrada do item.

**`internal/etl/queue.go`:**

- `QueueItem` ganha `FinishedAt int64` (`json:"finished_at"`) e três campos só de exibição ou de controle, não persistidos: `OutputMissing bool` (`"output_missing"`), `SourceMissing bool` (`"source_missing"`) e `Restaurado bool` (`"restaurado"`, item que veio do histórico e ainda não foi processado nesta sessão). `copiarItem` copia os quatro.
- `FinishedAt` é preenchido em todo caminho que encerra um item: sucesso, erro e cancelamento em `processItem`; o `recover` de panic (`queue.go:455-470`); a falha ao carregar a config (`queue.go:517-529`); e o `CancelItem` de item `pending` (`queue.go:356-359`), que não passa por `processItem`.
- Status novo `"interrupted"`. Não entra em `startProcessing` (que só pega `pending`). `LimparConcluidos` (`queue.go:402-415`, que hoje tira `success`, `error` e `cancelled`) passa a tirar também `interrupted`: é um item parado, que o usuário pode querer limpar junto com os concluídos. O botão só aparece com `contarConcluidos(items) > 0` (`ProcessList.svelte:352` e `:520`), e o conjunto `CONCLUIDO` de `frontend/src/lib/fila.js:129` hoje é `success`, `error` e `cancelled`: ele ganha `interrupted`, senão uma lista só com itens interrompidos (o caso típico depois de reabrir) não mostraria o botão. O `title` do botão (`ProcessList.svelte:521`, "Remover da lista os concluídos, com erro e cancelados") ganha "e interrompidos".
- A conversão `historico.Item` para `QueueItem` fica em `etl` (`queue_historico.go`): `Queue.Restaurar(itens []historico.Item)`, chamada uma vez, antes do watcher, e só com o histórico ligado. Monta os `QueueItem` com IDs novos (uuid) e `Restaurado: true`, converte `pending` e `processing` em `interrupted` (h1) e emite um único broadcast. Não inicia processamento. (O status `paused` nunca é atribuído a um item hoje, só aparece no comentário de `queue.go:24`, então não entra na conversão.)
- **Lista gravada (h2, e6):** a `Queue` guarda um sinal `sessaoComEntrada bool`, falso ao abrir, que vira verdadeiro na primeira entrada nova da sessão (item criado por `AddFiles`, linha `interrupted` reaproveitada, ou `Reprocessar`/"Processar de novo" de qualquer item) e nunca volta a falso. Com o sinal falso, a lista que vai ao gravador é a dos restaurados que restam; com ele verdadeiro, só a dos itens sem `Restaurado`, mesmo que vazia (se o usuário remover todos os itens da sessão, o arquivo fica com a lista vazia e a sessão anterior não volta). Contar os itens presentes não serviria: remover os itens novos faria a sessão anterior reaparecer no arquivo. Assim uma sessão sem processamento (abrir e fechar, ou a troca de versão do autoupdate) não apaga a anterior, e a primeira entrada nova da sessão faz os itens da sessão anterior saírem do arquivo (continuam na tela até o programa fechar). Um item restaurado que volta à fila ("Processar de novo", "Reprocessar", ou a varredura do watchdog pela reutilização abaixo) perde `Restaurado` e passa a ser da sessão atual.
- `Queue.SetGravador(g *historico.Gravador)` (ou uma interface pequena com `Agendar`, para os testes). O `broadcast` (`queue.go:593`), que já copia a lista sob `q.mu`, chama `g.Agendar` com a lista gravada **antes** do retorno antecipado por `wailsCtx == nil` (`queue.go:599`), para que a gravação não dependa do contexto do Wails. Progresso (`queue:item`, `emitirProgresso`) não grava: só mudanças de estado. Sem gravador (histórico desligado), nada muda.
- `AddFiles`: hoje só deduplica contra `pending`/`processing` (`queue.go:181`). Passa a reaproveitar a linha `"interrupted"` da mesma entrada (vira `pending`, mantém a posição, perde `Restaurado`), para que a varredura inicial do watchdog não crie uma segunda linha do mesmo processo (h1). "Mesma entrada" é: ZIP com o mesmo `Path`; item de PDF com o mesmo conjunto de `Paths` (sem diferença de caixa). A linha reaproveitada é zerada como `Reprocessar` faz hoje (`queue.go:280-291`): `Message` "Na fila...", `Progress`, `Error`, `OutputPath`, contadores e `UnreadDocs` zerados, `Timestamp` novo e `FinishedAt` zerado; o `ajuste` fica nil (vale a config global, como num arquivo novo), e `saidaAnterior` (spec do PDF) recebe o `OutputPath` de antes. Um PDF solto que é só um dos `Paths` de um item de vários PDFs interrompido não reaproveita a linha: cria um item novo; o item de vários PDFs só volta por "Processar de novo". Como o watchdog chama com um caminho por vez, a reutilização automática vale para ZIP e para item de um PDF só.
- `Reprocessar` (`queue.go:262`) já aceita qualquer estado fora de `pending` e `processing` (`queue.go:276`), então `interrupted` passa sem mudança. A única mudança é conferir se a entrada existe antes de pôr na fila (erro "arquivo de origem não encontrado").
- Cancelamento por encerramento: `processItem` distingue o cancelamento por Shutdown (`q.encerrada`) do cancelamento pelo usuário. O item cancelado por encerramento vai para `interrupted`, nunca para `cancelled`.
- `Shutdown` (`queue.go:114`): depois de esperar o item em curso (ou estourar o prazo de 5 s), chama `g.Fechar` com a lista gravada, os itens ainda `pending`/`processing` marcados como `interrupted`. Como o gravador fica fechado, nem o broadcast tardio do `processItem` que segue rodando após o prazo (`queue.go:133`) nem um timer pendente regravam por cima. `Shutdown` só volta depois dessa escrita.

**`app.go`:**

- `startup`: logo após a config e antes de `a.watcher.Start` (`app.go:90`), se `GuardarHistorico` estiver ligado, carrega `historico.json` (mesma pasta de `config.json` e `falhas.json`, via `config.GetConfigPath`), cria o `Gravador` e chama `queue.Restaurar`. A ordem importa: se o watcher começar antes, a varredura dos pendentes entra antes do histórico e a reutilização da linha interrompida não acontece. O filtro `ignorar` de `pendingZips` não muda: o interrompido volta pela varredura (h1).
- Em seguida, numa goroutine, confere a existência de `Path` (e de `Paths`) e `OutputPath` de cada item restaurado (`os.Stat`) e emite um broadcast com `OutputMissing`/`SourceMissing`. Fica fora do caminho de abertura porque um item numa pasta de rede fora do ar pode levar segundos por `Stat`.
- `CopyOutput`: se a leitura falhar com `fs.ErrNotExist`, marca `OutputMissing` no item, faz broadcast e devolve o texto fixo "O TXT não está mais na pasta; ele pode ter sido movido ou apagado", sem caminho; o caminho vai só ao `slog.Warn` (d9).
- `OpenDirectory` (`app.go:334-342`): hoje, com o `Stat` falhando, chama `explorer.exe` com o próprio caminho inexistente, e o Explorer abre em Documentos ou dá erro. Passa a usar `filepath.Dir(path)` quando o arquivo não existe, e a pasta só se ela existir. (Vale com o histórico ligado ou não.)
- Interruptor no `SaveConfig` (h3): desligar chama `g.Fechar(nil)`, que apaga o arquivo sob a trava, e solta o gravador da fila (os itens na tela ficam até fechar). Ligar cria um gravador novo e grava na hora a lista atual (todos os itens da sessão; não há restaurados, porque nada foi restaurado com o histórico desligado).

**`internal/models/models.go` e `internal/config/config.go`:** `Config` ganha `GuardarHistorico bool` (`json:"guardar_historico"`, padrão `false`), com o padrão em `DefaultConfig` e no overlay de `LoadFromFile`.

### 2.2 Bindings

Nenhum método novo é obrigatório. `QueueItem` ganha campos e `Config` ganha `guardar_historico`: rodar `wails generate module`.

### 2.3 Frontend

- `frontend/src/lib/fila.js`: `formatarQuando(timestamp, agora)` substitui `formatarHora` na linha (hoje, ontem, dd/mm, e dd/mm/aaaa se de outro ano). Usa `finished_at` quando houver, senão `timestamp`.
- `frontend/src/lib/fila.js`: `CONCLUIDO` inclui `interrupted` (o contador e, por ele, a condição do botão "Limpar concluídos"; teste em `fila.test.mjs`, seção 8).
- `frontend/src/components/ProcessList.svelte`: estado `interrupted` (ícone, rótulo "Interrompido", ação "Processar de novo", que chama `ReprocessQueueItem` com a config atual; sai com "Limpar concluídos"); `output_missing` desativa "Copiar TXT" e mostra "TXT não encontrado"; `source_missing` desativa "Reprocessar" com `title` explicando. Os itens restaurados ficam na mesma lista, sem seção à parte (h4).
- `frontend/src/components/AtividadeItem.svelte`: mensagem fixa quando não há instantâneo de atividade de um item restaurado.
- `frontend/src/components/Settings.svelte`: interruptor "Guardar a lista da última sessão", desligado por padrão, com a frase "Ao reabrir, a lista mostra os processos da última sessão. Os nomes dos arquivos ficam guardados neste computador até a sessão seguinte em que algo for processado."
- `DESIGN.md`: data na linha, estado Interrompido, o interruptor e a regra de que o monitoramento processa o que achou sem saída.

### 2.4 Instalador

`install.ps1 -Uninstall -Purge` já apaga `%APPDATA%\eprocIA` inteiro (`install.ps1:521-525`), o que leva o `historico.json` junto. Só a ajuda do parâmetro `-Purge` passa a citar "histórico da fila". O NSIS não apaga dados do usuário hoje e não muda.

### 2.5 Documentação

`CLAUDE.md` (Config: `historico.json` ao lado de `falhas.json`, opcional e desligado por padrão, gravado pelo `Gravador` com debounce e fechado no Shutdown, só a última sessão; pacote `internal/saida`) e `PRD.md` (RF09 "Histórico da fila" e a nota de privacidade).

## 3. Formato de dados e local

### 3.1 Local

`%APPDATA%\eprocIA\historico.json` (`os.UserConfigDir()/eprocIA`, mesma pasta de `config.json` e `falhas.json`). Fica no perfil do usuário, fora da pasta de instalação: sobrevive a atualizações e reinstalações. Só existe com o interruptor ligado.

### 3.2 Formato (UTF-8, sem BOM)

```json
{
  "versao": 1,
  "itens": [
    {
      "name": "5000000-00.2026.4.00.0001.zip",
      "path": "C:\\Users\\FULANO DE TAL\\Downloads\\5000000-00.2026.4.00.0001.zip",
      "status": "success",
      "message": "Concluído",
      "output_path": "C:\\Users\\FULANO DE TAL\\Downloads\\5000000-00.2026.4.00.0001.txt",
      "docs_count": 42,
      "pages_count": 318,
      "estimated_tokens": 151000,
      "unread_docs": 1,
      "error": "",
      "timestamp": 1795000000000,
      "finished_at": 1795000090000,
      "ocr_mode": "fast",
      "cleaning_level": "standard"
    }
  ]
}
```

Um item de vários PDFs (spec do PDF avulso, p3) leva também `"kind": "pdf"` e `"paths": [...]`. Não se grava o `id` (gerado de novo ao restaurar), `restaurado`, progresso, contadores parciais, atividade por documento nem o ajuste de reprocessamento de um item que ainda aguardava.

_Nota da revisão final (2026-11-26):_ grava-se também `"saida_anterior"` (omitido quando vazio), o `saidaAnterior` do item. Sem ele, um PDF reprocessado que ainda aguardava ou processava ao fechar voltava interrompido sem saída, e "Processar de novo" gravava `<nome>_2.txt`. Na carga ele passa pela mesma regra do `output_path` (`saida.SaidaDoItem`); inválido, só o campo é descartado, não o item. Ao restaurar, `saidaAnterior` vem dele e, sem ele, do `output_path`.

_Nota da decisão d3 (2026-11-26):_ grava-se também `"relatorio"` (omitido quando o item não tem), o `models.RelatorioZip` do último sucesso, **só com contagens**: documentos, páginas por categoria e por motivo de OCR, tempos, OCR, limpeza, `bytes_txt`, `tokens` e os tipos omitidos (os códigos de tipo da config, como `EXTR`). A lista `atencao`, que traz os nomes dos documentos a conferir, nunca vai para o arquivo: `paraHistorico` grava uma cópia do relatório com `atencao` vazia, e o relatório do item na fila não muda. Ao restaurar, o item concluído volta com a linha-resumo, os avisos de OCR, o "Ler com OCR" e o painel Detalhes (sem a lista de documentos a conferir). Um arquivo gravado antes disso não tem `relatorio`, e o item restaurado fica sem a linha-resumo, como antes.

### 3.3 Retenção

Só a última sessão (h2), pela regra da lista gravada (seção 2.1 e e6). Não há prazo em dias. Fica um teto de 500 itens, para uma sessão muito longa (o programa pode ficar dias aberto na bandeja) não gerar um arquivo sem limite; acima dele, saem os mais antigos. Com 500 itens o arquivo fica em torno de 300 KB, lido uma vez na abertura; com o relatório de cada item (d3), no máximo 1,5 MB (teto de `TestHistorico_TamanhoPorItem`; medido em torno de 1,2 MB, cerca de 2,3 KB por item, com o JSON indentado).

## 4. Casos de borda e falhas

- **Histórico desligado (padrão):** nenhuma leitura nem escrita de `historico.json`; `Restaurar` não é chamado; o comportamento é o de hoje, salvo a data na linha, a correção de `OpenDirectory` e o `FinishedAt`.
- **TXT movido ou apagado:** `OutputMissing`, "Copiar TXT" desativado, "Abrir pasta" abre a pasta do TXT se ela existir (com a correção de `OpenDirectory`). "Reprocessar" gera o TXT de novo se a entrada existir.
- **ZIP movido ou apagado:** `SourceMissing`, "Reprocessar" e "Processar de novo" desativados; o TXT, se existir, continua copiável.
- **Item processando ao fechar:** `Shutdown` cancela o contexto e espera até 5 s; o item vai para `interrupted` (nunca `cancelled`). O pipeline confere o contexto antes de gravar (`pipeline.go:727`), então no encerramento normal não sai TXT. Mas a gravação é direta, com `O_TRUNC` (`pipeline.go:738`): uma queda de energia durante a escrita deixa um TXT truncado. A janela é curta; gravar por temporário e rename fica fora do escopo.
- **Encerramento brusco** (queda de energia, processo morto pelo Gerenciador de Tarefas): vale a última gravação, no máximo 1 s atrasada. O item que constava como `processing` é tratado como interrompido ao carregar, pela mesma regra.
- **Interrompido que não é processado de novo:** aparece na abertura seguinte; se nessa sessão algo novo for processado, ele sai do arquivo e não aparece na outra abertura; se nada for processado, continua (e6). Com o monitoramento ligado e o ZIP das últimas 24 h, ele volta sozinho e passa a ser da sessão atual.
- **Mesmo ZIP baixado de novo com o mesmo nome:** cria linha nova se a anterior terminou (comportamento atual de `AddFiles`); reaproveita só a linha `interrupted`.
- **Varredura inicial do watchdog:** `pendingZips` pega ZIPs das últimas 24 h sem `.txt` mais novo; um item interrompido do monitoramento volta à fila sozinho por esse caminho, na mesma linha (h1).
- **Dois eprocIA abertos ao mesmo tempo** (não há `SingleInstanceLock` em `main.go`): o último a gravar vence. Limitação registrada.
- **Troca de versão pelo autoupdate:** o processo antigo fecha o gravador antes de renomear o exe (spec do autoupdate, seção 3.1); o novo carrega o histórico já gravado. Na reversão, o antigo reabre o gravador. Pela regra da lista gravada, uma troca sem processamento no meio não apaga a sessão anterior.
- **Disco cheio ou sem permissão na pasta de config:** a gravação falha, o log registra e a fila segue só em memória. Uma vez por sessão, aviso discreto "Não foi possível guardar o histórico".
- **Item com erro de ZIP inteiro:** `QueueItem.Error` traz o erro cru (pode conter caminho com o nome do usuário do Windows). Já aparece na tela; guardá-lo no perfil do próprio usuário não expõe nada novo. Não vai para nenhum TXT.
- **Downgrade para versão sem histórico:** o arquivo fica intocado e reaparece ao voltar para a versão nova.

## 5. Privacidade

O histórico guarda nomes de arquivos, que no Eproc são números de processo (ex.: `5000000-00.2026.4.00.0001`), e caminhos de pasta, que trazem o nome do usuário do Windows. Não guarda texto de documento. Desde a decisão d3 (2026-11-26) guarda também o relatório de cada item concluído só com contagens, sem nomes de documento (a lista `atencao` fica fora).

- **Se:** só com o interruptor ligado; o padrão é desligado (h3).
- **Onde:** só em `%APPDATA%\eprocIA\historico.json`, no perfil do usuário. Nunca sai da máquina.
- **Por quanto tempo:** só a última sessão com processamento, no máximo 500 itens (h2).
- **Como limpar:** X na linha (apaga o item), "Limpar tudo" (apaga o histórico inteiro), desligar o interruptor (apaga o arquivo) e `install.ps1 -Uninstall -Purge`.
- Em máquina compartilhada com o mesmo login, com o interruptor ligado, outra pessoa vê os processos da última sessão. A frase do interruptor em Configurações deixa isso claro.

## 6. Conflito com o RNF01

Nenhum. Tudo é local; nada trafega pela rede.

## 7. Conflito com a rodada de correção

A rodada 2026-11-10 (plano `docs/revisao/2026-11-18-plano-correcao.md`) já está na `main` (PR #13). O que ela deixou e este recurso aproveita:

- **F9** (F-AC-06, F-AC-12): `LimparConcluidos` (`queue.go:402-415`) e o binding `ClearFinishedQueue` (`app.go:500`) já existem, e "Limpar tudo" já pede confirmação em linha (`ProcessList.svelte:531-547`). Não há binding novo: este recurso só acrescenta `interrupted` ao que `LimparConcluidos` tira (seção 2.1), e a gravação sai do broadcast que ele já faz.
- **F12** (F-VI-09, largura do nome na linha): a data mais longa ("ontem 14:32", "12/11 14:32") compete com o nome e com o rótulo de não lidos; conferir nas capturas com a lista estreita.
- **F10** (F-AC-16) e **F11** (F-AC-15): o estado `interrupted` segue as mesmas regras de foco e de nomes dos botões da linha.
- **F16** (F-DF-04, painel de atividade): a mensagem de "sem detalhes" de item restaurado entra no mesmo componente.

A parte visual parte da ProcessList já corrigida. O backend (`internal/historico`, `Restaurar`, gravador, `OpenDirectory`) não conflita; `internal/saida` vem da spec do PDF avulso.

## 8. Testes que provam cada comportamento

`internal/historico/historico_test.go`:

- ida e volta: gravar e carregar devolvem os mesmos itens, na mesma ordem, inclusive `kind` e `paths`;
- arquivo ausente devolve vazio sem erro; JSON corrompido devolve vazio e deixa o `.corrompido-*` ao lado; `versao` 2 é ignorada e o arquivo não é sobrescrito;
- teto de 500 itens: os mais antigos saem;
- item com `OutputPath` que não passa em `saida.SaidaDoItem` é descartado; `OutputPath` `<nome>_2.txt` de PDF é aceito; ida e volta de um item de três PDFs cujo TXT tem o número CNJ comum (`<CNJ>.txt`, pasta do primeiro PDF por evento) mantém o item; `Path` relativo ou com extensão estranha é descartado;
- gravação atômica: com o rename falhando, o `historico.json` original fica intacto e nenhum temporário sobra;
- concorrência: `Agendar` em rajada e `GravarAgora` ao mesmo tempo, em goroutines (com `-race`): lido a qualquer momento durante a rajada, o arquivo é um JSON válido igual a uma das listas enviadas; depois de juntar as goroutines, um `GravarAgora(L)` final deixa o arquivo igual a `L`, e um `Agendar(M)` final, passado o debounce, igual a `M`;
- `Fechar` com timer pendente: o timer não grava depois; `Agendar` e `GravarAgora` depois de `Fechar` não gravam;
- `Fechar(nil)` (desligar) com timer pendente: o arquivo é apagado e não é recriado.

`internal/etl` (`queue_historico_test.go`, com `emitir` e `processar` falsos):

- broadcast seguido de outros três em menos de 1 s gera uma só gravação; progresso não gera gravação; com `wailsCtx` nil o broadcast ainda agenda a gravação; sem gravador, nada é agendado;
- `Shutdown` com item processando grava o item como `interrupted`, nunca como `cancelled`;
- broadcast do `processItem` que termina depois do prazo do `Shutdown` não regrava;
- `Restaurar` com itens `pending`/`processing` os converte em `interrupted`, marca todos como `Restaurado` e não inicia processamento;
- lista gravada: sem item novo, grava os restaurados restantes (remover um restaurado o tira do arquivo); com um item novo, grava só os da sessão; um restaurado que volta a processar passa a ser da sessão, e o sinal `sessaoComEntrada` liga também em `Reprocessar` de um item que não foi restaurado (item da sessão atual), e em `ReprocessarMonitorado` do PDF spec sobre um item restaurado; depois de um item novo, remover todos os itens da sessão grava a lista vazia (os restaurados não voltam ao arquivo);
- `AddFiles` do mesmo `Path` de um item `interrupted` reaproveita a linha (e tira `Restaurado`), com `OutputPath`, `Error`, contadores, `UnreadDocs` e `FinishedAt` zerados e sem `ajuste` (o `processar` falso recebe a config global); de um item `success` cria linha nova; um dos PDFs de um item de vários PDFs `interrupted` cria item novo e deixa a linha interrompida como está; os mesmos `Paths` juntos reaproveitam a linha;
- `LimparConcluidos` tira `interrupted` junto com `success`, `error` e `cancelled`, e deixa `pending` e `processing`;
- `Reprocessar` de `interrupted` com ZIP ausente devolve erro e não muda a fila;
- `FinishedAt` é preenchido em sucesso, erro, cancelamento, panic, falha ao carregar a config e `CancelItem` de pendente.

`internal/config`: `guardar_historico` ausente carrega `false`.

`app_test.go`:

- com o histórico ligado, `startup` restaura antes de iniciar o watcher (watcher falso que registra a ordem das chamadas); desligado, não lê `historico.json`;
- ligar o interruptor grava a lista atual; desligar apaga o arquivo;
- `CopyOutput` de item restaurado cujo TXT sumiu marca `OutputMissing` e devolve a mensagem;
- `OpenDirectory` com TXT ausente abre a pasta (comando falso registra o argumento);
- `lerSaidaParaCopiar` recusa um `OutputPath` que não passa em `saida.SaidaDoItem` e aceita a saída `<CNJ>.txt` de um item de três PDFs.

Frontend: `fila.test.mjs` para `formatarQuando` e para `contarConcluidos` contando `interrupted` (uma lista só com itens interrompidos dá contagem maior que zero, e a condição `{#if concluidos > 0}` do botão fica como está) (hoje, ontem, outro dia do mesmo ano, outro ano, virada de meia-noite, timestamp inválido); teste por regex da ProcessList (`interrupted` tem rótulo, ícone e ação; `output_missing` desativa copiar) e de Settings (interruptor de `guardar_historico`). Captura visual (`eprocia-visual`): itens de hoje e da sessão anterior em outro dia, um interrompido e um com TXT ausente, nos dois temas e com a lista estreita.

## 9. Estimativa

Médio. Go: cerca de 280 linhas de código e 420 de teste (pacote `historico` com o gravador, mudanças na fila, a regra da lista gravada com o sinal da sessão, a reutilização da linha interrompida, o interruptor, o startup e `OpenDirectory`; o pacote `saida` saiu daqui e conta na spec do PDF avulso, e o que a segunda revisão acrescentou cabe no que ele liberou). Frontend: cerca de 120 linhas e 80 de teste. Uma tarefa de backend e uma de frontend, mais capturas. Dois a três dias de agente com revisão (e6 confirmou a recomendação e não muda a estimativa; sem mudança: sai a poda por dias e o filtro estrito da h1, entram a regra da sessão e o interruptor).

## 10. Decisões do dono

De 2026-11-19:

- **h1.** Processo interrompido ao fechar: "Mostrar Interrompido; os do monitoramento voltam sozinhos". Aplicado nas seções 1, 2.1 e 4.
- **h2.** Retenção: "Só a última sessão". Aplicado nas seções 1, 2.1, 3.3 e 5.
- **h3.** Interruptor do histórico: "Sim, mas desligado por padrão". O histórico só existe se o usuário ligar. Aplicado nas seções 1, 2.1, 2.3, 4 e 5.
- **h4.** Exibição: "Na mesma lista, com data e hora na linha". Aplicado nas seções 1 e 2.3.

De 2026-11-20:

- **e6.** Histórico só da última sessão e o item interrompido: uma sessão vazia (nada processado) não apaga a anterior; o item interrompido sai na sessão seguinte, se não for processado de novo. Aplicado nas seções 1, 2.1, 3.3, 4 e 8 (regra da lista gravada, testes de `queue_historico_test.go`). Vale também para a troca de versão do autoupdate: abrir e fechar sem processar não apaga a lista.

De 2026-11-26:

- **d3.** Relatório no histórico: "Gravar o relatório só com contagens, sem nomes de documento". O item concluído restaurado volta com a linha-resumo, os avisos, o "Ler com OCR" e o painel Detalhes; a lista de documentos a conferir (`atencao`) não é gravada. Aplicado nas seções 3.2, 3.3 e 5 (Task 10 do plano `2026-11-26-historico.md`).
- **d8.** Ícone do Interrompido: `CircleStop` em `--warning`, próprio do estado; o `CirclePause` fica só no pendente pausado e no monitoramento. Task 11.
- **d9.** `CopyOutput` com o TXT ausente devolve "O TXT não está mais na pasta; ele pode ter sido movido ou apagado", sem caminho; o caminho vai ao log. Aplicado na seção 2.1. Task 11.

## 11. Resposta à revisão

Todos os achados foram aceitos: H1 (pacote folha `internal/saida`, conversão em `etl`), H2 (`Gravador.Fechar`), H3 (mutex e `os.CreateTemp`), H4 (`OpenDirectory` com `filepath.Dir`), H5 (h1 reescrita, com a opção estrita; o dono escolheu a não estrita), H6 (grupos renumerados conforme o plano em 05e7e7f), H7, H8, H9, H10, H11 (justificativa reescrita como defesa em profundidade) e H12. Nenhuma recusa.

**Segunda revisão sem contexto (2026-11-20),** conferida no código da `main` com a rodada 2026-11-10 juntada. Todos os achados foram conferidos e aceitos:

- H1 (item de vários PDFs sumiria ao carregar, porque `DaEntrada(Path, OutputPath)` compara com o primeiro PDF): validação por `saida.SaidaDoItem`, a mesma de `lerSaidaParaCopiar` (seção 2.1); teste de ida e volta com item de três PDFs (seção 8).
- H2: `LimparConcluidos` tira `interrupted` (seções 2.1 e 2.3; teste na seção 8).
- H3: a linha reaproveitada é zerada como no `Reprocessar`; item de vários PDFs só reaproveita com o mesmo conjunto de `Paths` (seção 2.1; testes na seção 8).
- H4: sinal `sessaoComEntrada` no lugar de contar itens presentes (seção 2.1; teste na seção 8).
- H5: contrato do gravador sob concorrência definido (seção 2.1) e teste reescrito (seção 8).
- H6: `internal/saida` é entregue e estimado pela spec do PDF avulso (seções 2.1 e 9).
- P8 (referências): `app.go`, `queue.go` e `install.ps1` conferidos no código atual; a seção 7 foi reescrita para a rodada já juntada (F9 não precisa de binding novo).
