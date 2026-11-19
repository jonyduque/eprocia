# Plano da correção dos achados de 2026-11-10

Plano para a rodada do `/corrigir` sobre os 46 achados pendentes de [`2026-11-10-relatorio.md`](2026-11-10-relatorio.md), na branch `revisao/2026-11-10` (worktree `.worktrees/revisao-2026-11-10`, criada da `main` com a v0.4.2 mesclada, HEAD `333a371`). Nada foi corrigido. Este documento decide validade, grupos, ordem, perguntas ao dono, residuais e riscos. Foi revisado por um agente sem contexto; a resposta a essa revisão está na seção 7.

Resumo: **30 achados valem como estão, 16 valem com ajuste e nenhum ficou obsoleto**. O dono respondeu 13 das 14 perguntas em 2026-11-18 (seção 4). Três achados ficam adiados: B-EF-05 (medir depois do bloco 1), F-DF-03 (decisão d10) e F-AC-13 (backlog, decisão d8). F-AC-14 aguarda a resposta de d2 e fica sozinho no bloco 5, o último. A rodada tem **22 tarefas em 5 blocos** (4 de backend e 18 de frontend).

As decisões e os ajustes já estão no começo do item Proposta de cada achado do relatório, e os três adiados têm `Status: adiado` com o motivo. Isso foi feito no commit `docs(revisao): plano de correção e decisões do dono`, que cumpre o passo 0.5.

## 0. Antes de rodar

1. **Commit de harness antes do bloco 1.** Três mudanças em `.claude/`, com teste em `.claude/workflows/workflows.test.mjs`:
   - **Testes do frontend no gate.** `scripts/build/gate.ps1` roda vet, `go test`, `wails build` e o smoke. O `gateVisual` do workflow roda capturas, `svelte-check` e `npm run build`. Ninguém roda `node --test frontend/*.test.mjs`, e quase todos esses testes leem o `.svelte` com regex (o CLAUDE.md pede esse glob em todo gate de frontend). Acrescentar ao `gateVisual` de `.claude/workflows/eprocia-corrigir.js` "rode `node --test` sobre `frontend/*.test.mjs` e reprove em qualquer falha".
   - **Os mesmos testes para o implementador e o revisor.** Acrescentar o glob ao item 6 de `.claude/agents/implementador.md` (hoje só `npm run build` e `svelte-check`) e a `.claude/agents/revisor-correcao.md`. Sem isso, cada regex quebrada gasta uma das 3 tentativas no gate.
   - **Métrica-alvo no gate de benchmark.** O `GATE_BENCH` exige p < 0,05 na métrica-alvo e nenhuma outra métrica pior que 5%, mas o agente do gate não vê a Proposta do achado, só a lista de arquivos. Passar ao prompt do gate o campo `observacoes` do implementador, em que ele declara a métrica-alvo e o benchmark. Tirar do critério dos 5% o `pico-mem-B` do modo com OCR, que oscila de uma rodada para outra (o próprio workflow já registra isso no `GATE_SEM_REGRESSAO`).
2. **Sessão na worktree.** O workflow roda `git status` e `git switch` no cwd da sessão. A checkout principal está em `docs/readme-2026-11`, com arquivos modificados, e não pode trocar para `revisao/2026-11-10`, que está em uso na worktree. Abrir a sessão do `/corrigir` com cwd em `.worktrees/revisao-2026-11-10`. Depois do commit de harness, chamar o Workflow com o `scriptPath` absoluto da worktree (ou abrir sessão nova), porque um workflow registrado no começo da sessão pode seguir com a versão antiga. No primeiro gate do bloco 2, conferir no log que o `node --test` rodou.
3. **`frontend/dist` e `node_modules` na worktree.** A worktree não tem nenhum dos dois, e sem `frontend/dist` o `main.go` não compila. Rodar `npm ci` e `npm run build` em `frontend` antes do primeiro bloco.
4. **Pré-requisitos do workflow.** Conferidos: `go.mod` com `go 1.27` e `tool golang.org/x/perf/cmd/benchstat`; `frontend/package.json` com `svelte` 5.57.1, `lucide-svelte` 1.0.1 e `svelte-check` 4.7.6; Wails CLI em `~/go/bin/wails`, compilado com go1.27.3 e `golang.org/x/tools` v0.52.0. O passo 0 e a migração para Svelte 5 devem sair como feitos. Resta um risco: o critério do workflow diz "svelte ^5" e o pacote fixa `5.57.1`, sem circunflexo. Conferir no log da Preparação que `svelte5_feito` veio `true`. Se vier `false`, interromper, porque o workflow abriria uma tarefa de migração desnecessária.
5. **Ajustes e decisões dentro do relatório.** O workflow manda ao implementador só o que o `auxiliar` extrai de cada seção do relatório (título, arquivo, evidência, proposta). Os ajustes da seção 1 e as respostas do dono da seção 4 vão **no começo** do item **Proposta** de cada achado, curtos ("Ajuste de 2026-11-18: ..." e "Decisão do dono: ..."). Um item novo não serviria: o esquema do `auxiliar` não tem campo para ele. No começo e curtos, porque o `auxiliar` roda com esforço baixo e pode resumir uma Proposta longa. Feito no commit `docs(revisao): plano de correção e decisões do dono`: cada Proposta afetada começa com "Decisão do dono (2026-11-18): ..." e/ou "Ajuste de 2026-11-18 ...", seguidos de "Proposta original: ...". Falta só a decisão de F-AC-14 (d2), que a sessão principal grava no lugar de "pendente" e commita antes do bloco 5; o workflow para se encontrar arquivo rastreado modificado. Na Preparação de cada bloco, conferir no log que os ajustes vieram.
6. **eprocia.exe fechado** e nenhum outro agente pesado rodando durante o bloco 1: o benchmark varia até ±70% com a máquina ocupada.
7. **Base sem o commit de versão.** A branch sai de `333a371`, sem a troca de `productVersion` da v0.4.2 (commit `4462a2a`, só na branch `versao-0.4.2`). Se a release sair antes do merge desta branch, mesclar a `main` nela antes do merge final.

## 1. Validade de cada achado no código atual

Conferido no HEAD `333a371`. Desde o relatório, `App.svelte` e `Settings.svelte` não mudaram, e as linhas citadas continuam certas. `ProcessList.svelte` e `AtividadeItem.svelte` ganharam o aviso de documentos não lidos da v0.4.1 (rótulo `.nao-lidos`, aviso de cópia do tipo `aviso`, chip "N não lidos", botão "Reprocessar com OCR" e o `{#key}` do formulário de reprocessar), e as linhas se deslocaram de 25 a 55. No backend, `formatter.go`, `higienizacao.go`, `pdf_parser.go` e `tesseract.go` mudaram, mas o trecho de cada achado continua lá.

"Como está" inclui achados em que só a linha mudou; a coluna "Onde hoje" traz a linha atual.

| Id | Veredito | Onde hoje | Ajuste |
|----|----------|-----------|--------|
| B-EF-01 | com ajuste | `internal/ocr/tesseract.go:282-283` (`-png`, `-r 150`), `:487` (sufixo `.png`) | O bloco de OCR agora tem até 20 páginas (`maxPaginasBlocoOCR`, `pdf_parser.go:53`), com até `numWorkers` blocos simultâneos. Em PGM, o temporário chega a cerca de 2,2 MB × 20 × 11 ≈ 480 MB no pico; em PPM, perto de 1,4 GB. Métrica-alvo: sec/op do `BenchmarkSintetico` no modo OCR `fast` (o modo padrão sem OCR não vê o ganho). O pico de memória desse modo não é critério. PGM, como a Proposta já fixa, com volta para PPM se o texto mudar. A conferência do texto em ZIPs reais (carimbos e destaques coloridos) fica com a sessão principal antes do merge. Ver d11. |
| B-EF-02 | com ajuste | `formatter.go:88` (`edgeLines`), `:140` (`cleanPage`), `:310` (`chaveConteudo`); `higienizacao.go:46-63` | A v0.4.2 tirou da pré-passada H7 as páginas com `ocr_aviso` e as `sem_ocr` (`formatter.go:288-289`). O cache de linhas por página precisa servir as três etapas sem mudar quem entra no H7. O teste de saída idêntica precisa cobrir uma página com `ocr_aviso` (a linha `avisoOCRComReserva` entra depois da limpeza). B-EF-02 cria em `internal/etl` um benchmark versionado de `GenerateConsolidatedOutput` (níveis `standard` e `aggressive`, páginas sintéticas), que B-EF-04 reaproveita. Métrica-alvo: alocações e sec/op desse benchmark. |
| B-EF-03 | com ajuste | `pdf_parser.go:542-561` (`extrairNativoSeguro`); contador em `pdf_texto.go:292` (`mostrar`) | O extrator posicional agora segue Forms (`Do`): o contador de operações de texto precisa contar também as que ocorrem dentro de Forms. Pular o `GetTextByRow` deixa válidas as métricas de uma página sem texto (hoje elas são zeradas na linha 556). O teste precisa provar que a rota da página não muda: escaneada e vetorial sem texto continuam indo ao OCR pelo texto curto e, com OCR desligado, continuam com o marcador de página digitalizada. Métrica-alvo: alocações do `BenchmarkParsePDF` nas fixtures escaneada e vetorial. |
| B-EF-04 | com ajuste | `formatter.go:73` e `:177`; `higienizacao.go:322` e `:359-362` | Valem as ressalvas do cético: pré-filtros estritamente equivalentes, com caso de teste para `"ﬁﬁ"` e `"$ 100 pagos"`. O pré-filtro não muda as alocações e só mexe em sec/op, que oscila. Por isso, a métrica-alvo é o sec/op do benchmark do formatador que B-EF-02 cria, no nível `aggressive`, comparado em rodadas alternadas (skill `eprocia-bench`). Os micro-benchmarks do relatório eram overlays fora do repositório. |
| B-EF-05 | com ajuste | `pipeline.go:733` (chamada serial de `GenerateConsolidatedOutput`) | O relatório pede medir de novo depois de B-EF-02 e B-EF-04. Adiado: fica fora desta rodada e é medido depois do bloco 1 (seção 5). |
| F-VI-01 | como está | `Settings.svelte:264-271`, `App.svelte:893-899` | |
| F-AC-01 | com ajuste | `ProcessList.svelte:214-219` (`handleCopy`), `:455-466` (botão) | Desde a v0.4.1, copiar um TXT com documentos não lidos mostra um aviso do tipo `aviso` na região `role="alert"`. O anúncio "TXT copiado" e esse aviso não podem se sobrepor nem ser lidos duas vezes: com não lidos, vale o aviso; sem eles, o anúncio de sucesso. O Ctrl+C da Proposta sai daqui e vai para F-AC-15, que define o foco de linha (hoje a linha não é focável). Pela d5, Copiar não muda de lugar nem ganha destaque. |
| F-AC-02 | com ajuste | `ProcessList.svelte:684` (`.queue-chip`), `:951` (`.meta-badge.success`), `:964`, `:987`, `:1000`; `AtividadeItem.svelte` (chips e `.doc-erro`); `App.svelte:780` | A v0.4.1 acrescentou cinco textos em `var(--warning)`, que dá 2,81:1 no tema claro: `.nao-lidos` e `.aviso-erro.aviso-atencao` (ProcessList), `.chip.aviso`, `.btn-ocr` e `.doc-erro.aviso` (AtividadeItem). Eles entram na mesma troca para `--warning-strong` e no teste de pares de contraste. Cor: `#9A5B00` (5,43:1 sobre branco, 4,78:1 sobre `--warning-soft`, mais perto do âmbar atual). |
| F-AC-03 | como está | `Settings.svelte:87-90`, `:151-164` | Reticências no meio do caminho, como a Proposta já diz; atualizar o DESIGN.md:104. Ver d3. |
| F-AC-04 | com ajuste | `ProcessList.svelte:128` (`confirmarReprocessar`), `:545-555` | O formulário agora está dentro de `{#key geracaoReprocessar[item.id]}` e também abre pelo botão "Reprocessar com OCR" do painel de atividade. A devolução do foco precisa cobrir a remontagem pelo `{#key}` e voltar ao botão que abriu o formulário, o da linha ou o do painel. |
| F-AC-05 | com ajuste | `AtividadeItem.svelte:73` (`role="status"`), `ProcessList.svelte:311` e `:321` | O resumo agora contém também o chip "N não lidos" e, ao fim, a faixa "Reprocessar com OCR". O número de não lidos ao terminar entra como marco na região única ("{nome}: concluído, 2 documentos não lidos"); o botão sai da região viva. |
| F-AC-06 | como está | `ProcessList.svelte:264` (`handleClear`), `:301`; `queue.go:384` | Segue a Proposta: "Limpar concluídos" sem confirmar e "Limpar tudo" com confirmação em linha. "Limpar concluídos" pede um binding novo no Go ou várias chamadas de `RemoveQueueItem`. Se o implementador criar o binding, roda `wails generate module` e declara o escopo ampliado. |
| F-VI-02 | como está | `Settings.svelte:173-198`, `ReprocessarItem.svelte:42` e `:50` | Usar `OPCOES_OCR`/`OPCOES_LIMPEZA` de `lib/atividade.js`. Conferir a 280 px; trocar "Desativado" por "Nenhum" só se cortar. |
| F-AN-01 | como está | `App.svelte:158-164`, `:370-384` | A troca do `alert()` sai em F-AC-07, que roda antes (grupo F3). |
| F-DF-01 | como está | `ProcessList.svelte:418` e `:1029`; `AtividadeItem.svelte:381` | |
| F-AC-07 | com ajuste | `App.svelte:164`, `ProcessList.svelte` (catches) | O `avisoCopia` da ProcessList já tem dois tipos: `error` e `aviso`, este para não lidos. Renomear para `avisoAcao`, não `avisoFila`, que já é importado de `lib/estado.js` (`ProcessList.svelte:39` e `:63`). Manter o tipo `aviso` e o teste `naoLidos.test.mjs`. |
| F-VI-03 | como está | `ProcessList.svelte:335`, `:491-499`, `:1076-1080` | Ver d12. |
| F-AC-08 | como está | `ProcessList.svelte:327`, `App.svelte:627` | Segue a Proposta: botão "Selecionar arquivos ZIP..." no vazio e frase curta. A zona de soltar desenhada, mais rica, fica de fora. |
| F-AC-09 | como está | `App.svelte:522-534`, `:607-617`, `:248-250` | Ver d4. Modelo para leitor de tela: rótulo fixo "Monitoramento" com `aria-pressed` (acompanhar `fila.test.mjs:161`). |
| F-VI-04 | como está | `Settings.svelte:122-229`, `ProcessList.svelte` (filtro), `ReprocessarItem.svelte:57-58`, `AtividadeItem.svelte` | Tarefa própria no bloco 4, depois do vocabulário de F-AC-03 e F-AC-09, do controle segmentado (F-VI-02) e do menu da barra recolhida (F-AC-14). O ícone do monitoramento segue d4. Inclui os ícones ao lado dos rótulos OCR e Higienização que F-AC-14 também pede. |
| F-AN-02 | como está | vários; ver relatório | Usar o `:active` de fundo, sem `scale`, ou excluir `.info-btn`, como dizem os céticos. |
| F-AN-03 | como está | `frontend/movimento.test.mjs:117` e `:174-175` | |
| F-VI-05 | como está | `App.svelte:709-768` | |
| F-VI-06 | como está | `style.css:120-132` | |
| F-AN-04 | como está | `ProcessList.svelte:319-339` | |
| F-AN-05 | como está | `ProcessList.svelte:309-315`, `App.svelte:418-423` | Ver d13. |
| F-AN-06 | como está | `Settings.svelte:95` e `:236-239`, `lib/aviso.js` | Ver d14. |
| F-DF-02 | como está | `ProcessList.svelte:836` (`.item-row`) | |
| F-AC-10 | como está | sem `forced-colors` em `frontend/src` | A captura com `forcedColors: 'active'` mexe em `scripts/visual/capturar.mjs` e em `.claude/review.json`: escopo ampliado a declarar. |
| F-AC-11 | como está | `App.svelte:392` (`role="region"`), `:468-505`, `:578-579` (divisor); `Settings.svelte:123`, `:137-226`; `ProcessList.svelte:364` (`role` redundante); `style.css:114-117` | |
| F-AC-12 | como está | `ProcessList.svelte:436-438` (`title="Remover da fila"` chama cancelar); `queue.go:339` | Pela d1, o comportamento fica: o X só cancela, e a linha cancelada mostra Reprocessar e o botão de excluir. A correção é alinhar o rótulo ("Cancelar"), não trocar a ação. |
| F-VI-07 | como está | `App.svelte:893-897` | Ver d7. |
| F-AN-07 | como está | `App.svelte:860-874`, `:488-492` | |
| F-AN-08 | com ajuste | `ProcessList.svelte:530-555`, `AtividadeItem.svelte` | O `ReprocessarItem` agora remonta pelo `{#key}` quando o usuário clica em "Reprocessar com OCR" com o formulário aberto. A entrada vai rodar de novo nessa remontagem, o que serve de retorno e está certo; o teste não pode tratar isso como erro. |
| F-AN-09 | como está | `AtividadeItem.svelte:109`, `lib/atividade.js:36-40` | Medir com o stub de muitos documentos antes de fechar, como pede o cético. Vai junto com F-DF-04. |
| F-AC-13 | como está | `Settings.svelte:205-214` | Adiado para o backlog (d8): a direção escolhida é o catálogo de tipos que cresce com os tipos já vistos, com binding novo no Go. |
| F-AN-10 | como está | `app.go` (ocultar e Abrir); `frontend/src` sem `visibilitychange` | Cruza camadas (evento emitido no Go): escopo ampliado a declarar. |
| F-DF-03 | com ajuste | `ProcessList.svelte:340` (`animate:flip`), `lib/quadro.js:21` | O relatório conta com a passagem para runes "prevista como pré-requisito do `/corrigir`". Ela não existe: o pré-requisito do workflow é o Svelte 5 em sintaxe legada, já feito, e o `EACH_ITEM_IMMUTABLE` de que o ganho depende só liga em runes. Corrigir F-DF-03 exige migrar ProcessList (e o trecho de eventos do App) para runes dentro da própria tarefa, o que quebra boa parte dos testes por regex. Adiado pela d10. |
| F-DF-04 | com ajuste | `AtividadeItem.svelte:31` e `:36`, `lib/atividade.js:36-60` | A estabilização de identidade precisa copiar também `leitura` e `detalhe` (v0.4.1). Sem runes, a parte que dá ganho é a (2), limitar os aguardando no DOM, e a (3), resumo no mesmo laço; a (1) só rende com runes. Fazer só (2) e (3). |
| F-VI-08 | com ajuste | `Settings.svelte:458-509`, `ProcessList.svelte:564-585`, `App.svelte:1199-1235` | O aviso da ProcessList ganhou a variante `aviso-atencao` (âmbar, para não lidos). O componente único precisa de três tipos (sucesso, erro e atenção), com ícone próprio para o de atenção: `CircleAlert` em `--warning`, como no rótulo de não lidos. Pela d14, o salvamento com sucesso em Settings deixa de usar toast. |
| F-VI-09 | com ajuste | `ProcessList.svelte:376-389`, `:907`, `:964-985` | A linha agora tem o rótulo `.nao-lidos` com `flex: none`, que compete com o nome do processo; a prioridade do nome precisa conviver com ele. Conferir também o residual P6 (rótulo cortado com a lista estreita) nas capturas com o divisor no máximo. Ver d6. |
| F-AN-11 | como está | `ProcessList.svelte` (cadeia de ícones; barra em `:416-420`) | Segue a Proposta inteira: ícone com escala, barra com fade e realce de fundo uma vez, desligado em reduced-motion. |
| F-AN-12 | como está | `App.svelte:273-277`, `style.css:110` e superfícies | |
| F-AC-14 | como está | `App.svelte:540-570`, `:220-228` | Depende de F-VI-02. Os ícones ao lado dos rótulos do painel aberto ficam com F-VI-04 (bloco 4); esta tarefa faz só o menu. A d2 ainda não tem resposta: o achado fica sozinho no bloco 5. |
| F-AC-15 | como está | `ProcessList.svelte` (linha e ações) | Recebe o Ctrl+C de F-AC-01. Depende de F-AC-06 e F-AC-12 (semântica de remover e cancelar para o Delete). Ver d9. |
| F-AC-16 | com ajuste | `ProcessList.svelte:427-520`, `Settings.svelte:163`, `AtividadeItem.svelte` (`title` do documento) | O `title` do documento agora traz o `detalhe` do não lido, além do erro. A parte 4 (erro por documento acessível) vale também para esse detalhe. A lixeira de Settings sai antes, em F-AC-03 (bloco 3), e deixa de ser assunto daqui. Depende de F-AC-12 para os nomes de remover e cancelar. |

## 2. Grupos e tarefas

Cada grupo é da mesma camada e muda o mesmo componente ou a mesma regra. Separei o que um revisor poderia aprovar de um lado e reprovar do outro:

- B-EF-02 e B-EF-04 mexem no mesmo laço, mas B-EF-04 traz a armadilha dos pré-filtros que mudam a regra. Vão em tarefas separadas.
- F-VI-02 e F-AC-14 também vão separados: o menu da barra recolhida é bem maior que o controle segmentado e depende dele. F-AC-14 ainda espera a d2.
- F-VI-04 saiu do grupo do painel lateral: o inventário de ícones passa pela fila, pelo reprocessar e pelo painel de atividade, e depende de tarefas do bloco 3.

| Tarefa | Bloco | Ids | O que muda em comum | Custo somado |
|--------|-------|-----|---------------------|--------------|
| B1 | 1 | B-EF-02 | `linhasBasicas` uma vez por página; cria o benchmark versionado do formatador | P |
| B2 | 1 | B-EF-04 | pré-filtros antes das regex da limpeza | P |
| B3 | 1 | B-EF-03 | contador de operações de texto no extrator posicional | P |
| B4 | 1 | B-EF-01 | PGM entre pdftoppm e tesseract | P |
| F1 | 2 | F-AN-03, F-DF-01 | regras de movimento: testes em todos os componentes e barras por `transform` | P+P |
| F2 | 2 | F-VI-01, F-VI-07, F-AC-11 | painel lateral e marcos: rolagem, padding, grupos, título na faixa do botão de recolher (d7), semântica dos campos, do `aside`, do divisor e da linha | 3×P |
| F3 | 2 | F-VI-08, F-AN-06, F-AC-07 | avisos: componente único, Check ao lado do controle salvo com toast só para erro (d14), falhas na tela e fim do `alert()` | M+P+P |
| F4 | 3 | F-AN-01 | arrastar e soltar: overlay só para arquivos, confirmação do aceito | M |
| F5 | 3 | F-AC-03, F-AC-09, F-AC-08 | monitoramento e primeiro uso: pasta sem lixeira (d3), controle com ícones (d4), vocabulário único, vazio e rodapé | 3×P |
| F6 | 3 | F-VI-02 | controle segmentado em Settings e Reprocessar | M |
| F7 | 3 | F-AC-02, F-AC-10, F-VI-06 | cor: contraste dos textos de estado, alto contraste do Windows, barra de rolagem | 3×P |
| F8 | 3 | F-AC-05, F-AC-01 | região viva única e retorno de copiar, sem mudar a ordem dos botões (d5) | 2×P |
| F9 | 4 | F-AC-06, F-AC-12, F-AN-04 | limpar e remover: confirmação, rótulo "Cancelar" no X do pendente (d1), saída com fade | M+P+P |
| F10 | 4 | F-AC-04, F-AC-16 | foco e nomes dos botões da linha | P+M |
| F11 | 4 | F-AC-15 | foco itinerante completo na fila, com o Ctrl+C (d9) | M |
| F12 | 4 | F-VI-03, F-VI-09, F-DF-02 | linha da fila: borda de erro e Info cinza (d12), nome com prioridade e contadores com ícones (d6), `content-visibility`, conferência do P6 | P+M+P |
| F13 | 4 | F-AN-02, F-VI-05, F-AN-07, F-AN-12 | transições globais e cromo da janela: botões, barra de título, recolher, tema | P+P+P+M |
| F14 | 4 | F-AN-05, F-AN-08, F-AN-11 | entradas e saídas dentro da fila: lugar reservado do chip (d13), painéis da linha, conclusão | P+P+M |
| F15 | 4 | F-AN-10 | pausar a renderização com a janela na bandeja (cruza para o Go) | M |
| F16 | 4 | F-DF-04, F-AN-09 | painel de atividade: menos trabalho por instantâneo e movimento dos documentos | M+P |
| F17 | 4 | F-VI-04 | inventário de ícones: rótulos do painel, filtro, botões de texto, spinner | P |
| F18 | 5 | F-AC-14 | menu da barra recolhida com o conteúdo do segmentado (depende da d2) | M |

Adiados, com `Status: adiado` e o motivo no relatório:

- **B-EF-05**: medir de novo depois do bloco 1.
- **F-DF-03**: decisão d10, outra rodada.
- **F-AC-13**: decisão d8. Vai para o backlog, na direção do catálogo de tipos com binding no Go.

Se a d2 for "deixar como está", F-AC-14 também passa a adiado, e o bloco 5 não roda.

**Rodadas e tempo.** São 22 tarefas, contra 46 sem grupos. Cada tarefa de EF roda o benchmark `-count=10` duas vezes (a linha de base do implementador e o `GATE_BENCH` em duas árvores, incluindo o modo OCR `fast`), além do `gate.ps1` com `-race` e o `TestFixtures_LegibleToOCR`: são de 40 a 60 minutos por tarefa. As de frontend levam de 25 a 35 minutos, com capturas, `svelte-check`, `npm run build` e os testes do frontend. Os grupos de 3 ou 4 ids (F3, F5, F7, F9, F12, F13 e F14) devem ser reprovados ao menos uma vez cada, e F11 (foco itinerante completo) também é candidato. Previsão: **11 a 15 horas**, e 8 a 10 horas só no melhor caso.

| Bloco | Tarefas | Estimativa |
|-------|---------|------------|
| 1, backend | B1 a B4 | 3h a 4h |
| 2, base do frontend | F1, F2, F3 | 1h30 a 2h |
| 3, painel, cor e anúncios | F4 a F8 | 2h30 a 3h30 |
| 4, fila, movimento e ícones | F9 a F17 | 4h30 a 5h30 |
| 5, barra recolhida | F18 | 30 a 45 min |

## 3. Ordem

A ordem do repositório (`ES, EF, GM, TE, RS, DF, AC, AN, VI, RN`) não serve numa execução só:

- Com AC antes de AN, F-AC-01 entra antes de F-AN-03, e o efeito novo de copiar nasce sem a cobertura dos testes de movimento.
- Com AC antes de VI, F-AC-03 entra antes de F-VI-01, que é o pré-requisito de tudo que aumenta a altura do painel.
- Também com AC antes de VI, F-AC-14 entra antes de F-VI-02, cujo componente ele reaproveita.

Uma cópia com `AN, VI, AC, DF` na frente resolveria as três dependências. A execução, porém, dependeria também da ordem dentro de cada lente: F-AN-03 antes dos outros AN, F-VI-01 antes dos outros VI, F-AC-06 e F-AC-12 antes de F-AC-15 e F-AC-16. O workflow ordena as tarefas pela lente do primeiro membro, com ordenação estável sobre a lista que o `auxiliar` devolve, e nada garante que essa lista venha na ordem dos `ids`.

Por isso a recomendação é rodar o `/corrigir` uma vez por bloco, cada um com a sua cópia do config. Assim sobram poucas dependências dentro da mesma lente, e o dono ganha um ponto de parada para olhar as capturas antes de seguir. Isso também cabe na regra de uma fase por sessão. O workflow aceita várias execuções na mesma branch (troca para ela se já existir) e preserva o `estado.json`.

Em todo bloco, passar os `ids` na ordem da tabela e conferir no log a ordem dos rótulos das tarefas antes da primeira implementação.

| Bloco | `ids`, nesta ordem | `grupos` | `correcao.ordem` da cópia | Ordem resultante |
|-------|--------------------|----------|----------------------------|------------------|
| 1 | B-EF-02 B-EF-04 B-EF-03 B-EF-01 | nenhum | a do repositório | as quatro de EF; B-EF-02 precisa vir antes de B-EF-04, porque cria o benchmark que B-EF-04 usa |
| 2 | F-AN-03 F-DF-01 F-AN-06 F-VI-08 F-AC-07 F-VI-01 F-VI-07 F-AC-11 | F1, F2, F3 | `AN, VI, AC, DF` e o resto | F1 (AN-03), F3 (puxada por AN-06), F2 |
| 3 | F-VI-02 F-VI-06 F-AC-02 F-AC-10 F-AN-01 F-AC-03 F-AC-09 F-AC-08 F-AC-05 F-AC-01 | F5, F7, F8 | a do repositório | F5 e F7 (puxadas por AC-03 e AC-02), F8, F4, F6, em qualquer ordem entre elas |
| 4 | F-DF-02 F-VI-03 F-VI-09 F-DF-04 F-AN-09 F-AC-06 F-AC-12 F-AN-04 F-AC-04 F-AC-16 F-AC-15 F-AN-02 F-VI-05 F-AN-07 F-AN-12 F-AN-05 F-AN-08 F-AN-11 F-AN-10 F-VI-04 | F9, F10, F12, F13, F14, F16 | a do repositório | F12 e F16 (DF), F9, F10, F11 (AC), F13, F14, F15 (AN), F17 (VI) |
| 5 | F-AC-14 | nenhum | a do repositório | F18 |

Dependências que restam:

- **Bloco 1:** B-EF-02 antes de B-EF-04 (mesma lente). Se o log mostrar outra ordem, interromper e rodar B-EF-02 sozinho antes.
- **Bloco 4:** F9 (F-AC-06, F-AC-12) antes de F10 e F11. A ordem dos `ids` acima garante isso se o `auxiliar` a respeitar; se não, interromper e rodar F9 sozinho antes.
- **Bloco 3:** sem dependência interna. As do bloco 2 (F-VI-01, F-AC-07, F-AN-03) já estão feitas.
- **Bloco 2:** F3 antes de F2 não atrapalha. O toast sai do cartão, o `overflow: hidden` deixa de ser necessário, e isso só ajuda o `flex: none` de F-VI-01.
- **Bloco 5:** precisa da d2 gravada na Proposta de F-AC-14 e de F-VI-02 (bloco 3).

Alternativa, se o dono preferir uma execução só: um parâmetro novo no workflow (por exemplo `args.ordemPorIds: true`) que ordena as tarefas pela posição do primeiro membro em `ids`, com teste em `workflows.test.mjs`. A mudança é pequena, mas mexe no harness e precisa de revisão própria. Não recomendo para esta rodada.

## 4. Decisões do dono

Perguntas feitas em 2026-11-18 (no Pinboard, com os ids deslocados em +1: d2 de lá é a d1 daqui). As respostas já estão no começo da Proposta de cada achado no relatório.

| # | Achado | Pergunta | Resposta do dono |
|---|--------|----------|------------------|
| d1 | F-AC-12 | O X do item pendente faz o quê? | Só cancela, não remove. Depois de cancelado, a linha mostra o botão de excluir. O usuário pode querer mudar os parâmetros e reprocessar antes de excluir. Diferente da recomendação. |
| d2 | F-AC-14 | OCR e higienização na barra recolhida | **Sem resposta.** O achado fica no bloco 5, que roda por último. |
| d3 | F-AC-03 | A lixeira da pasta monitorada | Sai sem substituto; trocar a pasta só por "Alterar". Diferente da recomendação. |
| d4 | F-AC-09 e F-VI-04 | Ícones do monitoramento | `FolderSearch`, `CirclePause` e `FolderX` (a recomendada). |
| d5 | F-AC-01 | Copiar o TXT na linha concluída | Não mover Copiar; só retorno visual e anúncio. Diferente da recomendação. |
| d6 | F-VI-09 | Contadores da linha | Ícones e números abreviados, com o valor exato na dica (a recomendada). |
| d7 | F-VI-07 | Título "Configurações" | Levar para a faixa do botão de recolher e mudar o DESIGN.md §5.3. Diferente da recomendação. |
| d8 | F-AC-13 | Omitir tipos | Catálogo de tipos (mexe no Go), mas no backlog, fora desta rodada. Achado adiado. |
| d9 | F-AC-15 | Teclado na fila | Foco itinerante completo: setas, Home/End, Delete, Ctrl+C, Ctrl+F. Diferente da recomendação. |
| d10 | F-DF-03 | Fila em runes | Fica para outra rodada (a recomendada). Achado adiado. |
| d11 | B-EF-01 | PGM entre pdftoppm e tesseract | Fazer agora; a sessão principal compara o texto nos ZIPs reais antes do merge (a recomendada). |
| d12 | F-VI-03 | Linha com erro | Borda vermelha; o botão de detalhes vira `Info` cinza (a recomendada). |
| d13 | F-AN-05 | Chip "Fila pausada" | Reservar o lugar do chip no cabeçalho (a recomendada). |
| d14 | F-AN-06 | Salvar várias vezes seguidas | Check ao lado do controle alterado; toast só para erro. Diferente da recomendação. |

Escolhas que ficaram como recomendação, sem pergunta, porque a Proposta do relatório já fixa o caminho ou um deles é claramente melhor:

- `--warning-strong` `#9A5B00` (F-AC-02).
- Rótulo fixo "Monitoramento" com `aria-pressed` (F-AC-09).
- `:active` de fundo em vez de `scale` (F-AN-02).
- "Limpar concluídos" mais confirmação em linha (F-AC-06).
- Reticências no meio do caminho (F-AC-03).
- Retorno completo com realce ao terminar (F-AN-11).
- Botão simples no estado vazio (F-AC-08).
- PGM com volta para PPM se o texto mudar (B-EF-01).
- B-EF-05 adiado.

## 5. Residuais

Os ids do arquivo de residuais não são aceitos pelo workflow, que só lê o relatório. O que for corrigido vai num plano próprio depois desta rodada, ou num adendo com ids no formato do relatório, se o dono quiser passar pelo `/corrigir`.

**Correção de código pequena, cabe numa rodada própria depois desta:**

| Id | O quê | Nota |
|----|-------|------|
| P18 | Erro de ZIP inteiro na linha e na notificação com o caminho: encurtar para o nome do arquivo (`queue.go:501` e `:545`, `app.go`) | Mesmo princípio do `textoErroSaida`; o CLAUDE.md ainda registra o caso como exceção. |
| CP9 | `recover` dentro de `HasImages` (`pdf_parser.go:583`), tratando o pânico como página sem imagens | |
| P14 (comportamento) | `Pages:[ \t]+` em `poppler.go:24` e teto de leitura na saída do `pdftotext` | O resto desse P14 (trava durante a extração, log duplicado) é só latência e não vale a pena. |
| Testes P2, P5, P11, P12, P13, P14 (testes), P16, CP6, CP7, CP8 | Asserções que faltam, nomes, guarda de quantidade mínima, limpeza da pasta temporária no `TestMain`; o P14 da tabela de testes pede teste unitário de `ErrSenha` e de cancelamento diferente de `ErrPrazo` em `rodarPoppler` | Uma rodada só de testes, sem mudar produção. A exceção é o P13, que inclui zerar `d.pendentes` em `esperar()`: é produção, mas trivial. |
| P3 | Conferir que quem avisa usa `Leitura`, e não só `Estado == erro` | O frontend já usa `naoLido(d)`; falta conferir `app.go` e a notificação. É leitura, provavelmente sem código. |
| M5 | Contar as páginas `sem_ocr` no `AtividadeDoc` e mostrar no painel | Cruza camadas e muda o painel: perguntar ao dono antes. |

**Pedem medição em dados reais e ficam com a sessão principal:**

- CP10: junção por alinhamento ou por posição; é só medição.
- CP2: limiar de isca, que só dá para medir com uma página-isca real.
- CP4 e CP5: vivem na junção e no encaixe por posição e devem esperar o resultado do CP10. O quadrático do CP5 só pesa com 50 mil linhas ausentes, e a duplicação por NFD não perde texto.
- A conferência do texto de B-EF-01 em ZIPs reais.
- A nova medição de B-EF-05 depois do bloco 1. Esta pode usar o benchmark sintético (e o benchmark do formatador que B1 cria), mas a decisão de seguir depende do ganho em ZIPs reais de texto nativo.

**Não valem a pena agora:**

- P10: sem efeito enquanto não houver prazo por ZIP.
- P7: mapas por id sem poda; o relatório derrubou o mesmo ponto como desprezível.
- P9: variáveis de pacote para injeção; refatoração sem ganho.
- P4: nota sobre o teste, sem defeito.
- P8: o `{#key}` só é conferido por regex, e sem navegador nos testes não há alternativa barata.
- As notas do M7 (prefixo `[ERRO`, filho de processo morto, `readerAt` nulo), salvo se aparecer um caso real.

M1 é decisão do dono já registrada (muda a aparência de um documento lido), não correção pendente. P6 entra na conferência de F12.

## 6. Riscos e proteções

| Risco | Proteção |
|-------|----------|
| Testes `frontend/*.test.mjs` fora do gate: uma tarefa pode quebrar `movimento`, `desempenho`, `fila`, `anuncios`, `contraste`, `limpeza-niveis`, `tema`, `densidade`, `layout`, `painel`, `estado`, `icones` ou `paleta` e ainda assim passar | Passo 0.1: `node --test` no `gateVisual`, no implementador e no revisor. Sem isso, a revisão final da branch é o único ponto que roda tudo, tarde demais. |
| Implementador "conserta" um teste por regex enfraquecendo a regra | O pedido de cada achado já cita os testes a acompanhar. O revisor confere que cada mudança em `.test.mjs` reflete a regra nova (a decisão do dono ou o ajuste da Proposta) e não só afrouxa a regex. |
| Gate visual não vê toasts, overlay de arrasto, hover, foco, alto contraste, barras de rolagem nem movimento (F3, F4, F7, F13, F14, F15 e parte de F2 e F5) | Aprovação humana pelo "Teste manual guiado" do relatório (itens 5 a 10 e 12), no executável da branch, antes do merge. F-AC-10 acrescenta a captura com `forcedColors`. |
| Gate de benchmark reprova correção boa: o pré-filtro de B-EF-04 não muda alocações e só mexe em sec/op, que oscila até ±70%; no modo OCR de B-EF-01, o pico de memória oscila e estoura os 5% | Passo 0.1 (métrica-alvo declarada pelo implementador e passada ao gate; `pico-mem-B` do modo OCR fora do critério); benchmark versionado do formatador criado em B1 e usado em B2 com rodadas alternadas; bloco 1 com nada mais rodando; saída idêntica byte a byte como teste. Sem o passo 0.1, contar com reprovação tripla e descarte de B2 e B4. |
| Sessão do `/corrigir` aberta na checkout principal, ou com o workflow antigo carregado | Passo 0.2: cwd na worktree e `scriptPath` absoluto depois do commit de harness. |
| `auxiliar` resume uma Proposta longa e o ajuste não chega ao implementador | Passo 0.5: ajuste e decisão curtos, no começo da Proposta, conferidos no log da Preparação de cada bloco. |
| B-EF-03 muda a rota de uma página sem texto (métricas válidas onde antes eram zeradas) | Teste de rota no ajuste: escaneada e vetorial seguem ao OCR e, com OCR desligado, mantêm o marcador. Os testes de `pdf_classes_test.go` e de conteúdo perdido rodam no `go test` do gate. |
| B-EF-01 troca o texto do OCR em páginas com cor | Decisão d11 e conferência em ZIPs reais pela sessão principal antes do merge; `TestFixtures_LegibleToOCR` no gate. |
| Tarefa reprovada 3 vezes descarta só os próprios arquivos; se sobrar arquivo modificado, a execução para | Blocos curtos. Ao parar, olhar `git status` antes de rodar o bloco seguinte. O grupo mais exposto é F13, com 4 ids; se ele bloquear, rodar os ids de novo em tarefas separadas. |
| Ordem dentro da mesma lente não garantida (F11 antes de F12 e F13 no bloco 4) | `ids` na ordem da seção 3 e conferência da ordem dos rótulos no log antes da primeira implementação. |
| Tarefas de frontend que cruzam para o Go (F-AC-06 com binding novo, F-AN-10) | Declarar o escopo ampliado; rodar `wails generate module` depois de mudar o `App`. O hook `proteger-gerados` bloqueia edição à mão de `wailsjs`. |
| Svelte 5 em sintaxe legada: propostas que dizem "com runes" ou "attachment" (F-AC-04, F-AC-14, F-AN-11, F-DF-04) | O implementador pode usar `{@attach}` e componentes novos em runes, mas não converte componentes inteiros (F-DF-03 adiado pela d10). |
| `svelte5_feito` lido como falso por causa da versão fixa (`5.57.1`, sem `^`) | Conferir o log da Preparação e interromper se a migração for agendada (seção 0, passo 4). |
| F-VI-01 e o aumento da altura do painel (F2 com o título na faixa do botão, F5, F6, F17) | F-VI-01 entra no bloco 2, antes de todas. O teste de `layout.test.mjs` que ele acrescenta passa a proteger os blocos seguintes, e as capturas de 800x600 entram em todo bloco que toca o painel. |
| Release da v0.4.2 sai antes do merge desta branch | Passo 0.7: mesclar a `main` na branch antes do merge final. |

## 7. Resposta à revisão

A revisão sem contexto (aprovado com ajustes: 7 importantes e 8 menores) foi aceita por inteiro. Onde cada ponto entrou:

| # | Achado da revisão | Onde entrou |
|---|-------------------|-------------|
| 1 | Gate de benchmark reprova B-EF-04 e B-EF-01 por ruído | Passo 0.1 (métrica-alvo passada ao gate, pico de memória do modo OCR fora do critério); ajustes de B-EF-01 a B-EF-04 na seção 1; tabela de riscos |
| 2 | Sessão na worktree e workflow pelo `scriptPath` | Passo 0.2 |
| 3 | Justificativa errada da seção 3 | Seção 3 reescrita: uma ordem `AN, VI, AC, DF` resolveria as três dependências; os blocos se justificam pela ordem dentro da lente e pelo ponto de parada. `ids` na ordem desejada e conferência no log |
| 4 | Referências deslocadas às decisões | Seção 1 refeita com a numeração nova (d1 a d14) |
| 5 | F-VI-04 fora do painel lateral | F-VI-04 virou tarefa própria no bloco 4 (hoje F17), depois de F5 e F6 (bloco 3). F2 ficou com F-VI-01, F-VI-07 e F-AC-11, e a descrição de F-AC-11 inclui App e ProcessList. Fui para o bloco 4, não para o 3 como a revisão sugeria, porque no bloco 3 F-VI-04 (lente VI) precisaria vir depois de F5 (lente AC), e F-VI-02 (VI) antes de F-AC-14 (AC): nenhuma ordem de lentes dá as duas coisas. Depois das decisões, F-AC-14 foi para o bloco 5 |
| 6 | Conflito do Ctrl+C entre d7 e d11, e F11 antes de F12 e F13 | O Ctrl+C saiu de F-AC-01 (d5) e entrou em F-AC-15 (d9, nas opções a e b). A dependência de F11 está na seção 3 e nos riscos |
| 7 | Implementador sem os testes do frontend | Passo 0.1 (implementador.md e revisor-correcao.md) |
| 8 | `avisoFila` já existe | Ajuste de F-AC-07: `avisoAcao` |
| 9 | Wails CLI encontrado | Passo 0.4 |
| 10 | Decisões que o relatório já fixa | d1, d5, d13 e d15 antigas viraram recomendação; d14 antiga ficou "agora ou adiar" (d11). São 14 perguntas |
| 11 | Pausado e sem pasta com o mesmo ícone | d4 (a): `CirclePause` para pausado, `FolderX` só sem pasta |
| 12 | Estimativa otimista | Seção 2: 11 a 15 horas, bloco 1 com 3 a 4 horas |
| 13 | Proposta longa resumida pelo `auxiliar` | Passo 0.5 e tabela de riscos |
| 14 | Segundo P14 ausente | Linha de testes da seção 5 |
| 15 | Base sem o commit de versão | Passo 0.7 e tabela de riscos |
