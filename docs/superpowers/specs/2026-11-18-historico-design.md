# Histórico da fila ao reabrir o eprocIA

Data: 2026-11-18. Base lida: worktree `revisao-2026-11-10` (main com a v0.4.2). Revisada por um revisor sem contexto em 2026-11-18 (aprovada com ajustes, todos aplicados ou respondidos na seção 11). Decisões do dono h1 a h4 (2026-11-19) e e6 (2026-11-20) aplicadas (seção 10); não há pontos em aberto.

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

**Regra do caminho de saída num pacote folha.** A regra "entrada para `.txt`" (hoje `pipeline.go:736`, e a do PDF avulso se ele entrar) sai de `internal/etl` para um pacote folha, `internal/saida`, importado por `etl`, `watchdog` e `historico`. Assim `historico` não importa `etl` e não há ciclo de import. Com a decisão p1 do PDF avulso (spec `2026-11-18-pdf-avulso-design.md`), o TXT de um PDF pode ser `<nome>_2.txt`; por isso a validação usa `saida.DaEntrada(entrada, saida)` (pasta certa e nome `<nome>.txt` ou `<nome>_N.txt`), não a igualdade com um caminho único.

**Pacote novo `internal/historico`** (`historico.go`, `historico_test.go`), que não importa `etl`:

- `type Item`: só os campos persistidos (seção 3), sem `cancel`, `ajuste`, `atividade` nem campos do limitador.
- `Carregar(arquivo string) ([]Item, error)`: lê, valida, aplica o teto de itens e devolve do mais novo para o mais antigo. Arquivo ausente devolve lista vazia sem erro. JSON corrompido: renomeia para `historico.json.corrompido-AAAAMMDD-HHMMSS`, registra no log e devolve lista vazia (nunca impede a abertura). Versão de formato maior que a conhecida: ignora o arquivo e não o sobrescreve até o usuário mexer na fila (um downgrade não apaga o histórico de uma versão nova).
- `Gravador`: dono único da escrita. Tem um mutex que serializa escrita e rename, um timer de debounce de 1 s e um estado `fechado`.
  - `Agendar(itens)`: guarda a última lista e (re)arma o timer. Ignorado se `fechado`.
  - `GravarAgora(itens)`: para o timer e grava na hora, sob o mutex.
  - `Fechar(itensFinais)`: sob o mutex, para o timer, marca `fechado` e faz a escrita final. Depois disso nenhum `Agendar` ou `GravarAgora` grava. Usado pelo Shutdown, pelo desligar do histórico (h3, com `itensFinais` nil, que apaga o arquivo) e pela troca de versão do autoupdate.
  - `Reabrir()`: desfaz o `fechado` (só a reversão do autoupdate usa).
  - A escrita usa `os.CreateTemp` na pasta do histórico e renomeia por cima (`os.Rename` no Windows substitui o destino). Se o rename falhar (antivírus segurando o arquivo), tenta de novo uma vez após 200 ms; falhando, registra no log, apaga o temporário e segue.
- `Podar(itens, teto)`: aplica o teto de 500 itens (seção 3.3).
- Validação ao carregar, item a item (o inválido é descartado, o resto fica): `Path` (e cada um de `Paths`, no item de vários PDFs) absoluto com extensão aceita pela fila; `OutputPath` vazio ou com `saida.DaEntrada` verdadeiro; status num conjunto conhecido. É defesa em profundidade, não barreira: quem edita o JSON também escolhe o `Path` (e quem escreve em `%APPDATA%` já roda como o usuário). O ganho é que um arquivo estragado ou montado à mão não faz `CopyOutput` (`app.go`, `lerSaidaParaCopiar`, que confia em itens com status `success`) ler um `.txt` sem relação com a entrada do item.

**`internal/etl/queue.go`:**

- `QueueItem` ganha `FinishedAt int64` (`json:"finished_at"`) e três campos só de exibição ou de controle, não persistidos: `OutputMissing bool` (`"output_missing"`), `SourceMissing bool` (`"source_missing"`) e `Restaurado bool` (`"restaurado"`, item que veio do histórico e ainda não foi processado nesta sessão). `copiarItem` copia os quatro.
- `FinishedAt` é preenchido em todo caminho que encerra um item: sucesso, erro e cancelamento em `processItem`; o `recover` de panic (`queue.go:435-449`); a falha ao carregar a config (`queue.go:497-508`); e o `CancelItem` de item `pending` (`queue.go:352-355`), que não passa por `processItem`.
- Status novo `"interrupted"`. Não entra em `startProcessing` (que só pega `pending`).
- A conversão `historico.Item` para `QueueItem` fica em `etl` (`queue_historico.go`): `Queue.Restaurar(itens []historico.Item)`, chamada uma vez, antes do watcher, e só com o histórico ligado. Monta os `QueueItem` com IDs novos (uuid) e `Restaurado: true`, converte `pending` e `processing` em `interrupted` (h1) e emite um único broadcast. Não inicia processamento. (O status `paused` nunca é atribuído a um item hoje, só aparece no comentário de `queue.go:24`, então não entra na conversão.)
- **Lista gravada (h2, e6):** a que vai ao gravador é a dos itens da sessão atual (os que não têm `Restaurado`), se houver ao menos um; se não houver, a dos restaurados que restam. Assim uma sessão sem processamento (abrir e fechar, ou a troca de versão do autoupdate) não apaga a anterior, e a primeira entrada nova da sessão faz os itens da sessão anterior saírem do arquivo (continuam na tela até o programa fechar). Um item restaurado que volta à fila ("Processar de novo", "Reprocessar", ou a varredura do watchdog pela reutilização abaixo) perde `Restaurado` e passa a ser da sessão atual.
- `Queue.SetGravador(g *historico.Gravador)` (ou uma interface pequena com `Agendar`, para os testes). O `broadcast` (`queue.go:572`), que já copia a lista sob `q.mu`, chama `g.Agendar` com a lista gravada **antes** do retorno antecipado por `wailsCtx == nil` (`queue.go:578`), para que a gravação não dependa do contexto do Wails. Progresso (`queue:item`, `emitirProgresso`) não grava: só mudanças de estado. Sem gravador (histórico desligado), nada muda.
- `AddFiles`: hoje só deduplica contra `pending`/`processing` (`queue.go:179`). Passa a reaproveitar a linha `"interrupted"` com o mesmo `Path` (vira `pending`, mantém a posição), para que a varredura inicial do watchdog não crie uma segunda linha do mesmo processo (h1).
- `Reprocessar` (`queue.go:258`) já aceita qualquer estado fora de `pending` e `processing` (`queue.go:272`), então `interrupted` passa sem mudança. A única mudança é conferir se a entrada existe antes de pôr na fila (erro "arquivo de origem não encontrado").
- Cancelamento por encerramento: `processItem` distingue o cancelamento por Shutdown (`q.encerrada`) do cancelamento pelo usuário. O item cancelado por encerramento vai para `interrupted`, nunca para `cancelled`.
- `Shutdown` (`queue.go:114`): depois de esperar o item em curso (ou estourar o prazo de 5 s), chama `g.Fechar` com a lista gravada, os itens ainda `pending`/`processing` marcados como `interrupted`. Como o gravador fica fechado, nem o broadcast tardio do `processItem` que segue rodando após o prazo (`queue.go:133`) nem um timer pendente regravam por cima. `Shutdown` só volta depois dessa escrita.

**`app.go`:**

- `startup`: logo após a config e antes de `a.watcher.Start` (`app.go:80`), se `GuardarHistorico` estiver ligado, carrega `historico.json` (mesma pasta de `config.json` e `falhas.json`, via `config.GetConfigPath`), cria o `Gravador` e chama `queue.Restaurar`. A ordem importa: se o watcher começar antes, a varredura dos pendentes entra antes do histórico e a reutilização da linha interrompida não acontece. O filtro `ignorar` de `pendingZips` não muda: o interrompido volta pela varredura (h1).
- Em seguida, numa goroutine, confere a existência de `Path` (e de `Paths`) e `OutputPath` de cada item restaurado (`os.Stat`) e emite um broadcast com `OutputMissing`/`SourceMissing`. Fica fora do caminho de abertura porque um item numa pasta de rede fora do ar pode levar segundos por `Stat`.
- `CopyOutput`: se a leitura falhar com `fs.ErrNotExist`, marca `OutputMissing` no item, faz broadcast e devolve "O TXT não está mais em <pasta>; ele pode ter sido movido ou apagado".
- `OpenDirectory` (`app.go:301-308`): hoje, com o `Stat` falhando, chama `explorer.exe` com o próprio caminho inexistente, e o Explorer abre em Documentos ou dá erro. Passa a usar `filepath.Dir(path)` quando o arquivo não existe, e a pasta só se ela existir. (Vale com o histórico ligado ou não.)
- Interruptor no `SaveConfig` (h3): desligar chama `g.Fechar(nil)`, que apaga o arquivo sob a trava, e solta o gravador da fila (os itens na tela ficam até fechar). Ligar cria um gravador novo e grava na hora a lista atual (todos os itens da sessão; não há restaurados, porque nada foi restaurado com o histórico desligado).

**`internal/models/models.go` e `internal/config/config.go`:** `Config` ganha `GuardarHistorico bool` (`json:"guardar_historico"`, padrão `false`), com o padrão em `DefaultConfig` e no overlay de `LoadFromFile`.

### 2.2 Bindings

Nenhum método novo é obrigatório. `QueueItem` ganha campos e `Config` ganha `guardar_historico`: rodar `wails generate module`.

### 2.3 Frontend

- `frontend/src/lib/fila.js`: `formatarQuando(timestamp, agora)` substitui `formatarHora` na linha (hoje, ontem, dd/mm, e dd/mm/aaaa se de outro ano). Usa `finished_at` quando houver, senão `timestamp`.
- `frontend/src/components/ProcessList.svelte`: estado `interrupted` (ícone, rótulo "Interrompido", ação "Processar de novo", que chama `ReprocessQueueItem` com a config atual); `output_missing` desativa "Copiar TXT" e mostra "TXT não encontrado"; `source_missing` desativa "Reprocessar" com `title` explicando. Os itens restaurados ficam na mesma lista, sem seção à parte (h4).
- `frontend/src/components/AtividadeItem.svelte`: mensagem fixa quando não há instantâneo de atividade de um item restaurado.
- `frontend/src/components/Settings.svelte`: interruptor "Guardar a lista da última sessão", desligado por padrão, com a frase "Ao reabrir, a lista mostra os processos da última sessão. Os nomes dos arquivos ficam guardados neste computador até a sessão seguinte em que algo for processado."
- `DESIGN.md`: data na linha, estado Interrompido, o interruptor e a regra de que o monitoramento processa o que achou sem saída.

### 2.4 Instalador

`install.ps1 -Uninstall -Purge` já apaga `%APPDATA%\eprocIA` inteiro (`install.ps1:520-524`), o que leva o `historico.json` junto. Só a ajuda do parâmetro `-Purge` passa a citar "histórico da fila". O NSIS não apaga dados do usuário hoje e não muda.

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

### 3.3 Retenção

Só a última sessão (h2), pela regra da lista gravada (seção 2.1 e e6). Não há prazo em dias. Fica um teto de 500 itens, para uma sessão muito longa (o programa pode ficar dias aberto na bandeja) não gerar um arquivo sem limite; acima dele, saem os mais antigos. Com 500 itens o arquivo fica em torno de 300 KB, lido uma vez na abertura.

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

O histórico guarda nomes de arquivos, que no Eproc são números de processo (ex.: `5000000-00.2026.4.00.0001`), e caminhos de pasta, que trazem o nome do usuário do Windows. Não guarda texto de documento.

- **Se:** só com o interruptor ligado; o padrão é desligado (h3).
- **Onde:** só em `%APPDATA%\eprocIA\historico.json`, no perfil do usuário. Nunca sai da máquina.
- **Por quanto tempo:** só a última sessão com processamento, no máximo 500 itens (h2).
- **Como limpar:** X na linha (apaga o item), "Limpar tudo" (apaga o histórico inteiro), desligar o interruptor (apaga o arquivo) e `install.ps1 -Uninstall -Purge`.
- Em máquina compartilhada com o mesmo login, com o interruptor ligado, outra pessoa vê os processos da última sessão. A frase do interruptor em Configurações deixa isso claro.

## 6. Conflito com o RNF01

Nenhum. Tudo é local; nada trafega pela rede.

## 7. Conflito com a rodada de correção

Grupos conforme `docs/revisao/2026-11-18-plano-correcao.md` no commit 05e7e7f:

- **F9** (F-AC-06 "Limpar concluídos" com binding novo e "Limpar tudo" com confirmação; F-AC-12 semântica de remover e cancelar; F-AN-04 saída com fade): mexe em `Clear`, `RemoveItem` e nas ações da linha, as mesmas que o histórico usa. "Limpar concluídos" apaga do histórico também; com o gravador ligado ao broadcast isso sai de graça, desde que o binding novo termine com broadcast.
- **F12** (F-VI-09 largura do nome na linha, P6): a data mais longa ("ontem 14:32", "12/11 14:32") compete com o nome e com o rótulo de não lidos.
- **F10** (F-AC-16, nomes dos botões da linha) e **F11** (F-AC-15, teclado e Delete): o estado `interrupted` entra nas mesmas regras de foco e de nomes.
- **F16** (F-DF-04, painel de atividade): a mensagem de "sem detalhes" de item restaurado entra no mesmo componente.

Recomendação: implementar a parte visual depois da rodada, sobre a ProcessList já corrigida. O backend (`internal/saida`, `internal/historico`, `Restaurar`, gravador, `OpenDirectory`) não conflita e pode ser feito antes.

## 8. Testes que provam cada comportamento

`internal/historico/historico_test.go`:

- ida e volta: gravar e carregar devolvem os mesmos itens, na mesma ordem, inclusive `kind` e `paths`;
- arquivo ausente devolve vazio sem erro; JSON corrompido devolve vazio e deixa o `.corrompido-*` ao lado; `versao` 2 é ignorada e o arquivo não é sobrescrito;
- teto de 500 itens: os mais antigos saem;
- item com `OutputPath` que não passa em `saida.DaEntrada` é descartado; `OutputPath` `<nome>_2.txt` é aceito; `Path` relativo ou com extensão estranha é descartado;
- gravação atômica: com o rename falhando, o `historico.json` original fica intacto e nenhum temporário sobra;
- concorrência: `Agendar` em rajada e `GravarAgora` ao mesmo tempo (com `-race`) produzem um arquivo válido, igual à última lista;
- `Fechar` com timer pendente: o timer não grava depois; `Agendar` e `GravarAgora` depois de `Fechar` não gravam;
- `Fechar(nil)` (desligar) com timer pendente: o arquivo é apagado e não é recriado.

`internal/etl` (`queue_historico_test.go`, com `emitir` e `processar` falsos):

- broadcast seguido de outros três em menos de 1 s gera uma só gravação; progresso não gera gravação; com `wailsCtx` nil o broadcast ainda agenda a gravação; sem gravador, nada é agendado;
- `Shutdown` com item processando grava o item como `interrupted`, nunca como `cancelled`;
- broadcast do `processItem` que termina depois do prazo do `Shutdown` não regrava;
- `Restaurar` com itens `pending`/`processing` os converte em `interrupted`, marca todos como `Restaurado` e não inicia processamento;
- lista gravada: sem item novo, grava os restaurados restantes (remover um restaurado o tira do arquivo); com um item novo, grava só os da sessão; um restaurado que volta a processar passa a ser da sessão;
- `AddFiles` do mesmo `Path` de um item `interrupted` reaproveita a linha (e tira `Restaurado`); de um item `success` cria linha nova;
- `Reprocessar` de `interrupted` com ZIP ausente devolve erro e não muda a fila;
- `FinishedAt` é preenchido em sucesso, erro, cancelamento, panic, falha ao carregar a config e `CancelItem` de pendente.

`internal/config`: `guardar_historico` ausente carrega `false`.

`app_test.go`:

- com o histórico ligado, `startup` restaura antes de iniciar o watcher (watcher falso que registra a ordem das chamadas); desligado, não lê `historico.json`;
- ligar o interruptor grava a lista atual; desligar apaga o arquivo;
- `CopyOutput` de item restaurado cujo TXT sumiu marca `OutputMissing` e devolve a mensagem;
- `OpenDirectory` com TXT ausente abre a pasta (comando falso registra o argumento);
- `lerSaidaParaCopiar` recusa um `OutputPath` que não passa em `saida.DaEntrada`.

Frontend: `fila.test.mjs` para `formatarQuando` (hoje, ontem, outro dia do mesmo ano, outro ano, virada de meia-noite, timestamp inválido); teste por regex da ProcessList (`interrupted` tem rótulo, ícone e ação; `output_missing` desativa copiar) e de Settings (interruptor de `guardar_historico`). Captura visual (`eprocia-visual`): itens de hoje e da sessão anterior em outro dia, um interrompido e um com TXT ausente, nos dois temas e com a lista estreita.

## 9. Estimativa

Médio. Go: cerca de 290 linhas de código e 440 de teste (pacote `saida`, pacote `historico` com o gravador, mudanças na fila, a regra da lista gravada, o interruptor, o startup e `OpenDirectory`). Frontend: cerca de 120 linhas e 80 de teste. Uma tarefa de backend e uma de frontend, mais capturas. Dois a três dias de agente com revisão (e6 confirmou a recomendação e não muda a estimativa; sem mudança: sai a poda por dias e o filtro estrito da h1, entram a regra da sessão e o interruptor).

## 10. Decisões do dono

De 2026-11-19:

- **h1.** Processo interrompido ao fechar: "Mostrar Interrompido; os do monitoramento voltam sozinhos". Aplicado nas seções 1, 2.1 e 4.
- **h2.** Retenção: "Só a última sessão". Aplicado nas seções 1, 2.1, 3.3 e 5.
- **h3.** Interruptor do histórico: "Sim, mas desligado por padrão". O histórico só existe se o usuário ligar. Aplicado nas seções 1, 2.1, 2.3, 4 e 5.
- **h4.** Exibição: "Na mesma lista, com data e hora na linha". Aplicado nas seções 1 e 2.3.

De 2026-11-20:

- **e6.** Histórico só da última sessão e o item interrompido: uma sessão vazia (nada processado) não apaga a anterior; o item interrompido sai na sessão seguinte, se não for processado de novo. Aplicado nas seções 1, 2.1, 3.3, 4 e 8 (regra da lista gravada, testes de `queue_historico_test.go`). Vale também para a troca de versão do autoupdate: abrir e fechar sem processar não apaga a lista.

## 11. Resposta à revisão

Todos os achados foram aceitos: H1 (pacote folha `internal/saida`, conversão em `etl`), H2 (`Gravador.Fechar`), H3 (mutex e `os.CreateTemp`), H4 (`OpenDirectory` com `filepath.Dir`), H5 (h1 reescrita, com a opção estrita; o dono escolheu a não estrita), H6 (grupos renumerados conforme o plano em 05e7e7f), H7, H8, H9, H10, H11 (justificativa reescrita como defesa em profundidade) e H12. Nenhuma recusa.
