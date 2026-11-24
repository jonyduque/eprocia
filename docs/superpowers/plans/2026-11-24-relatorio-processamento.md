# Relatório do processamento, Forçar OCR, log e telemetria: plano de implementação

> **Para agentes:** sub-skill obrigatória: use superpowers:subagent-driven-development (recomendado) ou superpowers:executing-plans para implementar este plano tarefa por tarefa. Os passos usam checkbox (`- [ ]`) para acompanhamento. Regras comuns a todo implementador e revisor: `docs/superpowers/plans/2026-11-24-relatorio-processamento-regras.md`.

**Goal:** Ao fim de cada ZIP, mostrar quanto foi lido de cada jeito (com aviso e botão "Ler com OCR" só quando há página que precisava de OCR), permitir forçar o OCR em todas as páginas de PDF (global e por Reprocessar) e, só com `EPROCIA_LOG` definida, gravar um log JSON Lines saneado e uma telemetria local sem nomes.

**Architecture:** Dois pacotes novos sem dependência externa: `internal/registro` (modo lido de `EPROCIA_LOG`, saneamento, arquivo rotativo, manipulador `slog` assíncrono que nunca bloqueia, logger no `context`) e `internal/telemetria` (registro por ZIP, coleta da máquina, gravação própria; importa só `internal/models`). Os tipos do relatório (`RelatorioZip` e partes) e `ExecucaoZip` ficam em `internal/models`, que etl, telemetria e frontend (por JSON) compartilham. O pipeline grava fatos por página no mapa que já existe, deriva a `categoria` com uma função pura, mede fases e OCR em `medidasOCR` e entrega o relatório pela `Atividade` (o mesmo caminho do `UnreadDocs`), sem mudar a assinatura de `ProcessZip`. O formatter ganha `GerarSaida` (contadores da limpeza e a linha do cabeçalho); `GenerateConsolidatedOutput` vira um invólucro dela. O frontend lê `QueueItem.relatorio` por libs puras (`relatorio.js`, com todos os textos novos em `textos.js`) e um painel novo (`RelatorioItem.svelte`).

**Tech Stack:** Go 1.27 (`log/slog`, `sync/atomic`, `golang.org/x/sys/windows` já direto no `go.mod`), Wails v2.13.0, Svelte 5 em sintaxe legada, `node --test`, Playwright das capturas (`scripts/visual/capturar.mjs`). Nenhuma dependência nova.

**Spec:** `docs/superpowers/specs/2026-11-22-relatorio-processamento-design.md` (com as "Decisões do dono, 2026-11-23"). Contrato do `.txt`: `PRD.md` §6. Visual: `DESIGN.md`.

## Global Constraints

- Regras de ambiente, gate, commit e relatório: `docs/superpowers/plans/2026-11-24-relatorio-processamento-regras.md`.
- `EPROCIA_LOG`: ausente ou vazia = desligado (nenhum arquivo nem pasta de log/telemetria, `log` padrão no stderr, interface sem nada de log); `1` = log `INFO` e telemetria; `detalhado` = o mesmo com `DEBUG` (registro por página); qualquer outro valor = `1`, com um `WARN` de valor não reconhecido. Lida uma vez, na inicialização, depois da trava de instância.
- Log: `%AppData%\eprocIA\logs\eprocia.log`, JSON Lines (`slog.NewJSONHandler`), rotação a 5 MB para `eprocia.1.log` (no máximo 5 arquivados), arquivos com mais de 30 dias apagados na abertura. Campos fixos: `ts` (ISO 8601 com fuso), `nivel`, `msg`; `item_id`, `doc_id`, `pag` quando couber. `msg` em pt-BR.
- Telemetria: `%AppData%\eprocIA\telemetria\telemetria.jsonl`, um registro por ZIP (também por reprocessamento, com `reprocesso: true`), `O_APPEND`, rotação a 10 MB para `telemetria.1.jsonl` (no máximo 5 arquivados), sem apagar por idade. Nunca leva nome de ZIP, de documento, de pasta nem texto de página. O pacote não importa `net` nem `net/http`, nem direta nem indiretamente.
- Escrita do log: canal com buffer de 4096, uma goroutine gravadora, `bufio.Writer` descarregado a cada 1 s e ao encerrar. Canal cheio: `DEBUG`/`INFO` descartadas na hora; `WARN`/`ERROR` esperam no máximo 50 ms e então são descartadas; o total vai em `linhas_perdidas`. Falha de gravação: pausa de 60 s, conta as perdidas, registra uma vez, nunca sobe ao pipeline. Registrar depois de encerrar descarta a linha, nunca entra em pânico.
- Saneamento (em toda mensagem, todo atributo e na linha final): perfil do usuário vira `%USERPROFILE%`, diretório temporário vira `%TEMP%`, o segmento logo depois de `\Users\` ou `/Users/` vira `<usuario>`. Nomes de ZIP, de documento e do `.txt` não são saneados. Texto de página nunca entra no log (nem trecho nem hash).
- Nenhuma chamada de log por página (nem por atualização de progresso) fora de `DEBUG`. Única exceção: a linha do Form XObject ignorado, que sobe para `WARN`.
- `models.Config.ForcarOCR` (`bool`, JSON `forcar_ocr`), padrão `false`, completado por `DefaultConfig()` no `Load`; com `ocr_mode = none` o valor é preservado e não tem efeito (global e override).
- Linha nova do `.txt`, só com N > 0, entre `Documentos não lidos: %d` e `Data de Consolidação`: `Páginas que precisavam de OCR e ficaram sem: N`. Com N = 0 o `.txt` fica byte a byte como hoje.
- Categorias (ordem de decisão): `com_erro`, `sem_ocr`, `vazia`, `lida_ocr`, `lida_nativa`. Sub-contadores de `lida_nativa`: `ocr_sem_ganho`, `ocr_falhou_com_reserva`. Tokens estimados: um a cada 4 caracteres (`EstimateTokens`).
- O aviso de página nunca muda o status `success` do item. A linha-resumo aparece em todo item concluído com páginas lidas (ruling do orquestrador, 2026-11-24).
- Todo texto novo de interface fica em `frontend/src/lib/textos.js` (troca rápida depois das capturas); os componentes e as libs importam de lá. O dono aprova os textos pelas capturas da Task 19.
- Linhas de log e textos de interface em pt-BR. Sem chave de log ou telemetria nas Configurações; o botão "Abrir pasta de logs" e todo texto sobre log ficam atrás de `logAtivo` (`App.LogAtivo()`).
- `go.mod`, `go.sum`, `frontend/package.json` e `internal/ocr/embed/` não mudam. `frontend/src/wailsjs/` só muda por `wails generate module`.

## Review Focus

- **Log aberto em outro programa na hora da rotação.** O dono abre `eprocia.log` num editor ou num `Get-Content -Wait` com o app rodando; o Windows recusa renomear. Esperado: a rotação falha calada (um `WARN` uma vez), o app segue gravando no arquivo atual e tenta de novo no próximo limite. Teste na Task 3 (`TestArquivoRotativo_RenomearFalhaSegueGravando`).
- **Caminho do perfil em outra grafia.** `c:\users\fulano` em minúsculas, com `/`, com barras escapadas no texto de um erro (`C:\\Users\\fulano`), o nome 8.3 (`C:\Users\FULANO~1\AppData\Local\Temp`) e um perfil vizinho (`C:\Users\fulano2`). Esperado: nenhuma forma do nome do usuário sobra; o vizinho não vira `%USERPROFILE%2`; `C:\Users\5000000.zip` e `Users-fulano.zip` mantêm o nome. Teste na Task 2 (`TestSanear_GrafiasDoPerfil`).
- **ZIP sem nada lido.** Todos os documentos omitidos por tipo, ou todos não lidos, ou limpeza `none`: total 0, "antes" 0, nenhum bloco de OCR. Esperado: nenhuma divisão por zero, nenhum `NaN` ou `Infinity` no JSON ou na tela, linha-resumo ausente. Testes nas Tasks 11 (`TestRelatorio_ZipSemPaginasLidas`) e 15 (`resumo com total 0`).
- **Relatório velho num item reprocessado ou cancelado.** Esperado: `processItem` zera `Relatorio` ao começar; `Reprocessar` zera; uma execução cancelada ou com erro termina com `Relatorio == nil`, e o frontend não mostra a linha de uma execução anterior. Teste na Task 11 (`TestQueue_RelatorioZeradoAoReprocessarECancelar`).
- **Forçar OCR em página que não tem o que ler.** Página em branco (sem texto, sem imagem), página nula e página cuja leitura falhou. Esperado: a em branco vai ao OCR e, com OCR vazio, sai `vazia` (não `com_erro`); a nula e a com erro de leitura não são despachadas e mantêm a categoria. Teste na Task 12 (`TestForcarOCR_PaginasSemConteudo`).

---

### Task 1: capturas "antes" (base 7e44221)

**Files:**
- Create: `docs/revisao/capturas/2026-11-24-relatorio/antes/` (PNGs)

**Interfaces:**
- Produces: as capturas "antes" dos estados existentes que este plano muda, para os pares antes/depois da skill `eprocia-visual` (Task 19).

- [ ] **Passo 1:** confira que o HEAD da worktree só difere de `7e44221` por documentos (`git diff --stat 7e44221 HEAD -- frontend scripts` vazio). Se não for assim, capture a partir de uma worktree de `7e44221` no scratchpad (com `npm ci` nela).
- [ ] **Passo 2:** `node scripts/visual/capturar.mjs --saida docs/revisao/capturas/2026-11-24-relatorio/antes --estados fila,configuracoes,nao-lidos` (matriz completa desses estados, incluindo `-reprocessar` e `-atividade`). Confira que termina com `[visual] N capturas em ...` e que o Vite (porta 5199) foi encerrado.
- [ ] **Passo 3: commit** `docs(capturas): estados de antes do relatório de processamento`.

---

### Task 2: internal/registro, modo de EPROCIA_LOG e saneamento

**Files:**
- Create: `internal/registro/modo.go`, `internal/registro/sanear.go`
- Test: `internal/registro/modo_test.go`, `internal/registro/sanear_test.go`

**Interfaces:**
- Produces: `type Modo int` com `ModoDesligado`, `ModoNormal`, `ModoDetalhado`; `func LerModo(valor string) (m Modo, reconhecido bool)`; `type Saneador struct`; `func NovoSaneador(perfil, temp string) *Saneador`; `func SaneadorDoSistema() *Saneador` (usa `os.UserHomeDir()` e `os.TempDir()`); `func (s *Saneador) Sanear(texto string) string`.

- [ ] **Passo 1: testes.**
  - `TestLerModo`: `""` → `(ModoDesligado, true)`; `"   "` → `(ModoDesligado, true)`; `"1"` → `(ModoNormal, true)`; `"detalhado"` e `"DETALHADO"` → `(ModoDetalhado, true)`; `"sim"` → `(ModoNormal, false)`.
  - `TestSanear_PerfilTempEUsuario`: com `NovoSaneador(`C:\Users\fulano`, `C:\Users\fulano\AppData\Local\Temp`)`, `C:\Users\fulano\AppData\Local\Temp\eprocia_123\doc.pdf` vira `%TEMP%\eprocia_123\doc.pdf`; `C:\Users\fulano\Downloads\5000000-00.2026.8.26.0001.zip` vira `%USERPROFILE%\Downloads\5000000-00.2026.8.26.0001.zip`; `D:\Users\beltrano\x.zip` vira `D:\Users\<usuario>\x.zip`.
  - `TestSanear_GrafiasDoPerfil` (Review Focus): `c:\users\FULANO\x`, `C:/Users/fulano/x`, `C:\\Users\\fulano\\x` (barras dobradas) e `C:\Users\FULANO~1\AppData\Local\Temp\y` não contêm `fulano` (sem caixa) depois de `Sanear`; `C:\Users\fulano2\x` vira `C:\Users\<usuario>\x` (nunca `%USERPROFILE%2`); `C:\Users\5000000.zip` (o último segmento, sem separador depois) fica igual; `Users-fulano.zip` e `a fulano disse` ficam iguais.
  - `TestSanear_TextoSemCaminho`: texto sem caminho volta idêntico.
- [ ] **Passo 2: rodar** `go test ./internal/registro -run "TestLerModo|TestSanear" -v`: falham (esqueleto).
- [ ] **Passo 3: implementar.** `Sanear` troca primeiro o temporário (mais longo, costuma ficar dentro do perfil), depois o perfil, sem diferenciar caixa e aceitando `\`, `/` e `\\` como separador, e só quando o trecho termina num limite de segmento (separador, aspas ou fim do texto). Por último, uma regex sem caixa troca o segmento entre `[\\/]+Users[\\/]+` e o separador seguinte por `<usuario>`; um segmento sem separador depois (nome de arquivo) não é tocado. Pré-compile tudo em `NovoSaneador`; `Sanear` não aloca quando o texto não tem `users` (sem caixa).
- [ ] **Passo 4: rodar** os testes e o gate Go.
- [ ] **Passo 5: commit** `feat(registro): modo de EPROCIA_LOG e saneamento do perfil do usuário`.

---

### Task 3: internal/registro, arquivo rotativo e manipulador assíncrono

**Files:**
- Create: `internal/registro/rotativo.go`, `internal/registro/gravador.go`, `internal/registro/manipulador.go`, `internal/registro/registro.go`
- Test: `internal/registro/rotativo_test.go`, `internal/registro/gravador_test.go`, `internal/registro/registro_test.go`

**Interfaces:**
- Consumes: `LerModo`, `Saneador` (Task 2).
- Produces:
  - `type ArquivoRotativo struct`; `func AbrirRotativo(dir, base, ext string, limite int64, arquivados int) (*ArquivoRotativo, error)`; `(*ArquivoRotativo).Write(p []byte) (int, error)`; `Close() error`. Nomes: `<base><ext>` ativo, `<base>.1<ext>` … `<base>.<arquivados><ext>`.
  - `func ApagarAntigos(dir string, idade time.Duration, agora time.Time) error` (só `eprocia*.log`).
  - `type Opcoes struct { Valor string; PastaDados string; Saneador *Saneador; Agora func() time.Time; Saida io.Writer }` (`Saida` só para teste: substitui o arquivo rotativo).
  - `func Iniciar(o Opcoes) (ativo bool, err error)`: com modo desligado não cria nada e devolve `(false, nil)`; com modo ligado cria `<PastaDados>\logs`, aplica `ApagarAntigos(30 dias)`, abre `eprocia.log` (5 MB, 5 arquivados) e guarda o estado no pacote; falha de pasta ou arquivo devolve `(false, err)` e deixa o pacote como desligado.
  - Estado do pacote: `func Ativo() bool`, `func Detalhado() bool`, `func Pasta() string`, `func Manipulador() slog.Handler` (nil se inativo), `func ValorNaoReconhecido() bool`, `func Descarregar()`, `func Encerrar()` (descarrega e fecha o arquivo; idempotente; o canal nunca é fechado: uma flag atômica faz o manipulador descartar o que chegar depois), `func ResetarParaTeste()` (encerra e zera o estado do pacote; exportada para os testes do pacote `main`). `func Logger(ctx context.Context) *slog.Logger` (o de `ComLogger` ou `slog.Default()`), `func ComLogger(ctx context.Context, l *slog.Logger) context.Context`.
  - Constantes: `TamanhoCanal = 4096`, `IntervaloDescarga = time.Second`, `EsperaAvisoCheio = 50 * time.Millisecond`, `PausaAposFalha = 60 * time.Second`, `LimiteLog = 5 << 20`, `ArquivadosLog = 5`, `IdadeMaximaLog = 30 * 24 * time.Hour`.

- [ ] **Passo 1: testes do arquivo rotativo** (`rotativo_test.go`, em `t.TempDir()`):
  - `TestArquivoRotativo_GiraNoLimite`: limite 100 bytes, 3 arquivados; escrever 7 linhas de 40 bytes deixa `x.log` com menos de 100 bytes, `x.1.log`, `x.2.log`, `x.3.log` existem, `x.4.log` não; a linha mais antiga sumiu e a mais nova está em `x.log`.
  - `TestArquivoRotativo_RenomearFalhaSegueGravando` (Review Focus): troque o renomear por uma função que falha (campo `renomear func(de, para string) error` no tipo, injetado no teste); escrever além do limite não devolve erro e o conteúdo continua indo para `x.log`; a próxima escrita tenta girar de novo.
  - `TestApagarAntigos`: `eprocia.1.log` com mtime de 31 dias é apagado; `eprocia.log` de 29 dias e `outro.txt` de 40 dias ficam.
- [ ] **Passo 2: testes do gravador e do manipulador** (`gravador_test.go`, relógio injetável; um gancho de teste interno dá acesso ao canal):
  - `TestManipulador_LinhaJSONComCamposFixos`: `slog.New(h).With("item_id","u1").Info("zip concluído", "doc_id", 3)` gera uma linha que `json.Unmarshal` aceita, com `ts` em RFC 3339 com fuso, `nivel == "INFO"`, `msg`, `item_id`, `doc_id`; não há chaves `time` nem `level`.
  - `TestManipulador_Saneia`: mensagem, atributo string, atributo `error` (`fmt.Errorf("abrir: %w", &fs.PathError{Path: C:\Users\fulano\AppData\Local\Temp\a.pdf ...})`) e atributo dentro de grupo: o arquivo não contém `fulano`.
  - `TestManipulador_SaneiaAnyStruct`: `slog.Any("config", models.Config{MonitoredPath: C:\Users\fulano\Processos})` e `slog.Any("m", map[string]string{"p": C:\Users\fulano\x})`: o arquivo não contém `fulano`.
  - `TestGravador_NaoBloqueia`: `Saida` é um writer que, no primeiro `Write`, avisa por um canal que entrou e trava até o fim do teste. O teste espera o aviso (a gravadora está presa), enche o canal pelo gancho até a primeira recusa e só então mede: `Info` volta em menos de 5 ms, `Warn` e `Error` em menos de 200 ms, e o contador de perdidas sobe exatamente 3.
  - `TestGravador_FalhaDeEscritaPausa60s`: writer que devolve erro até um sinal do teste e depois aceita. Nenhuma chamada de log devolve erro nem entra em pânico; com o writer já aceitando, avançar o relógio para 59 s não grava nada; a 61 s a gravação volta, e a primeira linha gravada é a de `linhas_perdidas` maior que 0.
  - `TestGravador_DescargaPeriodicaENoEncerrar`: linha escrita sem `Encerrar` aparece no arquivo depois de 1 s (relógio injetado ou espera com prazo de 3 s); `Encerrar` descarrega o resto e chamar `Encerrar` duas vezes não entra em pânico.
  - `TestGravador_LogDepoisDeEncerrar`: `Info` e `Error` depois de `Encerrar` voltam sem pânico e não gravam; com `-race`, 20 goroutines registrando enquanto outra chama `Encerrar` terminam sem pânico.
  - `TestGravador_Concorrencia`: 50 goroutines × 200 linhas com `-race`; toda linha do arquivo é JSON válido e a soma de linhas gravadas e perdidas é 10.000.
- [ ] **Passo 3: testes de `Iniciar`** (`registro_test.go`, `ResetarParaTeste` no cleanup):
  - `TestIniciar_Desligado`: `Valor: ""` com `PastaDados: t.TempDir()` devolve `(false, nil)`, `Ativo()` é `false`, `Manipulador()` é nil e a pasta continua vazia.
  - `TestIniciar_Valores`: `"1"` liga com nível `INFO` (`Manipulador().Enabled(ctx, slog.LevelDebug)` é `false`); `"detalhado"` liga com `DEBUG` e `Detalhado()` é `true`; `"talvez"` liga como `1` e `ValorNaoReconhecido()` é `true`.
  - `TestIniciar_PastaSomenteLeitura`: `PastaDados` apontando para um arquivo comum devolve `(false, err)` e `Ativo()` é `false`.
- [ ] **Passo 4: rodar** `go test -race ./internal/registro -v`: os novos falham.
- [ ] **Passo 5: implementar.** O manipulador formata cada registro com um `slog.NewJSONHandler` sobre um buffer próprio da chamada (sem estado compartilhado entre goroutines), com `ReplaceAttr` que renomeia `time`→`ts` (`time.RFC3339Nano` com fuso local), `level`→`nivel`, mantém `msg` e passa por `Sanear` todo valor string, `error` e `fmt.Stringer`; depois passa a linha final inteira por `Sanear` (alcança o que o `json.Marshal` serializa de struct, map e slice, com as barras escapadas). Entrega `[]byte` e nível ao canal; com a flag de encerrado, descarta sem enviar. A goroutine gravadora é a única que toca o `bufio.Writer` e o `ArquivoRotativo`. Depois de uma falha, a pausa de 60 s termina com `bufio.Writer.Reset(destino)` (o `bufio.Writer` guarda o primeiro erro para sempre); as linhas no buffer naquele momento contam como perdidas. A linha de perdidas é um registro `WARN` com `msg` "linhas de log perdidas" e `linhas_perdidas`, escrito antes da próxima linha depois que a gravação volta.
- [ ] **Passo 6: rodar** os testes com `-race` e o gate Go.
- [ ] **Passo 7: commit** `feat(registro): log JSON Lines assíncrono com rotação, retenção e saneamento`.

---

### Task 4: chamadas por página em Debug e o Form XObject em Warn

**Files:**
- Modify: `internal/etl/pdf_parser.go`, `internal/etl/pdf_poppler.go`, `internal/etl/pdf_ocr_despacho.go`, `internal/etl/pdf_texto.go` (se tiver `log.Print*`), `internal/etl/queue.go` (só a linha de progresso em `onProgress`)
- Modify: `internal/etl/pdf_log_form_test.go` (a linha do Form passa a sair por `slog` em `WARN`)
- Test: `internal/etl/registro_volume_test.go` (novo)

**Interfaces:**
- Consumes: `registro.Logger(ctx)`, `registro.ComLogger` (Task 3).
- Produces: nos quatro arquivos `pdf_*`, nenhuma chamada a `log.Print*`; as chamadas viram `Debug` (ou `Warn` só onde listado abaixo) com atributos (`pag`, `bytes`, `motivo`...) em vez de `%v` na mensagem.

- [ ] **Passo 1: testes.**
  - `TestRegistro_SemLogPorPaginaForaDeDebug`: lê `pdf_parser.go`, `pdf_poppler.go`, `pdf_ocr_despacho.go` e `pdf_texto.go` com `go/parser` e reprova qualquer chamada `log.Print*`; reprova chamada `.Info(`, `.Warn(` ou `.Error(` cuja mensagem (primeiro argumento string literal) não esteja na lista fixa do teste: `"Form XObject ignorado"`, `"página com falha de OCR"` (a linha de `ATENÇÃO` por documento), `"OCR em bloco falhou; refazendo página a página"`, `"falha ao abrir o PDF"` e `"árvore de páginas inválida"`, todas `Warn`. Em `queue.go`, a chamada dentro de `onProgress` é `Debug`.
  - `TestRegistro_SemVariavelStderrComoHoje` (**guarda de regressão**: passa antes da mudança): sem trocar `slog.Default()`, com `log.SetOutput(&buf)` (restaure `log.Writer()` e `log.Flags()` no cleanup): `slog.Info`, `slog.Warn` e `slog.Error` chegam a `buf`; `slog.Debug` não.
  - Atualize `pdf_log_form_test.go`: o teste põe no `ctx`, por `registro.ComLogger`, um `slog.Logger` JSON em memória com nível `Debug` e confere uma linha `WARN` com `msg` "Form XObject ignorado", `pag` e `motivo`, e nenhuma outra linha acima de `DEBUG` para essa página.
- [ ] **Passo 2: rodar** `go test ./internal/etl -run "TestRegistro_|FormIgnorado" -v`: os não marcados como guarda falham.
- [ ] **Passo 3: converter.** Linhas por página e por atualização de progresso → `Debug`; o Form XObject → `Warn`; as de documento que sobram nesses arquivos → as mensagens da lista acima. `p.filePath` sai da mensagem e vai como atributo `doc` com `filepath.Base(p.filePath)` (no construtor por caminho ele é o temporário sob o perfil; em `NewPDFParserDeLeitor` já é o nome no ZIP). Onde a função não recebe `ctx` (`analisarPagina`), passe o `*slog.Logger` por parâmetro. Mensagens em pt-BR, sem `fmt.Sprintf` no caminho quente (atributos). Não chame `slog.SetDefault` nesta tarefa.
- [ ] **Passo 4: rodar** os testes e o gate Go.
- [ ] **Passo 5: commit** `refactor(etl): log por página só em Debug e Form XObject em Warn`.

---

### Task 5: ligar o registro no app (main.go, app.go)

**Files:**
- Modify: `main.go`, `app.go`
- Create: `registro_app.go` (raiz, pacote `main`)
- Test: `registro_app_test.go` (raiz)
- Generated: `frontend/src/wailsjs/go/main/App.js`, `App.d.ts`

**Interfaces:**
- Consumes: `registro.Iniciar`, `registro.Ativo`, `registro.Pasta`, `registro.Manipulador`, `registro.Encerrar`, `registro.Descarregar`, `registro.ResetarParaTeste`, `registro.SaneadorDoSistema` (Task 3).
- Produces:
  - `func iniciarRegistro(valor, pastaDados string, cfg *models.Config) bool` (chama `registro.Iniciar`; ativo, faz `slog.SetDefault(slog.New(registro.Manipulador()))`, grava a linha de início e, se o valor não foi reconhecido, o `WARN` "valor de EPROCIA_LOG não reconhecido"; inativo, não toca em `slog.Default()` nem em `log`).
  - `func prepararInstancia(travar func() (liberar func(), ok bool, err error), aoTravar func()) (sair bool, liberar func())`: `aoTravar` só roda quando a trava foi obtida (`ok`) ou quando a trava está indisponível (`err != nil`, que segue como hoje, com a linha `slog.Warn("trava de instância indisponível", "erro", err)` no lugar do `log.Printf` de `main.go:29`); com outra instância aberta (`!ok`), devolve `sair = true` sem chamar `aoTravar`.
  - `func versaoApp() string`: lê `info.productVersion` de `wails.json` embutido (`//go:embed wails.json`).
  - `func (a *App) LogAtivo() bool` (= `registro.Ativo()`); `func (a *App) AbrirPastaLogs() error` (inativo: `errors.New("o log não está ativo")`; ativo: `explorer.exe <pasta>`).
- Linha de início (`INFO`, msg "eprocIA iniciado"): `versao`, `eprocia_log` (`1` ou `detalhado`), `windows` (versão e build por `windows.RtlGetVersion()`), `cpus`, `semaforo_docs` e `semaforo_ocr` (`max(NumCPU()-1, 1)`), `pasta_logs`, `config` (a config efetiva como grupo, com `monitored_path`; o manipulador saneia).

- [ ] **Passo 1: testes** (`registro_app_test.go`, pasta de dados em `t.TempDir()`, `slog.Default()`, `log.Writer()` e `log.Flags()` restaurados e `registro.ResetarParaTeste()` no cleanup):
  - `TestIniciarRegistro_SemVariavel`: `iniciarRegistro("", dir, cfg)` devolve `false`; `dir` continua vazio; `slog.Default()` é o mesmo ponteiro de antes e `log.Writer()` o mesmo writer; `(&App{}).LogAtivo()` é `false`; `AbrirPastaLogs()` devolve erro com "não está ativo".
  - `TestIniciarRegistro_Normal`: `"1"` devolve `true`, cria `logs\eprocia.log`; depois de `registro.Encerrar()` o arquivo tem a linha "eprocIA iniciado" com `versao` igual à de `wails.json`, `cpus`, `config.monitored_path` começando por `%USERPROFILE%` quando a config usa `os.UserHomeDir()` + `\Downloads`, e nenhuma linha `DEBUG`.
  - `TestIniciarRegistro_ValorDesconhecido`: `"xyz"` grava um `WARN` com `msg` "valor de EPROCIA_LOG não reconhecido".
  - `TestIniciarRegistro_LidoSoNaPartida`: depois de `iniciarRegistro("1", ...)`, `t.Setenv("EPROCIA_LOG", "detalhado")` não muda `registro.Detalhado()`.
  - `TestPrepararInstancia`: com `ok=false`, `aoTravar` não é chamado e `sair` é `true`; com `ok=true`, é chamado uma vez; com `err != nil`, é chamado.
- [ ] **Passo 2: rodar** `go test . -run "TestIniciarRegistro|TestPrepararInstancia" -v`: falham.
- [ ] **Passo 3: implementar.** Em `main.go`, `prepararInstancia` substitui o `switch` da trava e chama `iniciarRegistro(os.Getenv("EPROCIA_LOG"), <UserConfigDir>\eprocIA, app.cfg)` depois de carregar a config. `OnShutdown` chama `registro.Encerrar()` depois de `app.shutdown`; `App.Quit` chama `registro.Descarregar()` antes de `runtime.Quit`; o `log.Fatal(err)` vira `slog.Error` + `registro.Encerrar()` + `os.Exit(1)`. Rode `wails generate module`.
- [ ] **Passo 4: rodar** os testes e o gate (`pwsh scripts/build/gate.ps1`, sem `EPROCIA_LOG` definida).
- [ ] **Passo 5: commit** `feat(app): registro só com EPROCIA_LOG, LogAtivo e AbrirPastaLogs`.

---

### Task 6: erro de processo externo com código de saída e stderr (internal/ocr)

**Files:**
- Modify: `internal/ocr/prazo.go`, `internal/ocr/tesseract.go`, `internal/ocr/poppler.go`
- Test: `internal/ocr/prazo_test.go`, `internal/ocr/main_test.go` (modo novo do binário de teste)

**Interfaces:**
- Produces: `type ErroProcesso struct { Programa string; Codigo int; Stderr string; Err error }` com `Error()` (`"<programa> saiu com código <n>: <err>"`, sem o stderr) e `Unwrap() error`. `rodarComPrazo` captura o stderr do comando (até `maxStderr` = 1024 bytes, reusando `limitado` de `poppler.go`) quando `cmd.Stderr` é nil, e embrulha a falha em `*ErroProcesso`. `errors.Is(err, ErrPrazo)`, `errors.Is(err, context.Canceled)` e `errors.Is(err, ErrSenha)` continuam valendo pela cadeia.

- [ ] **Passo 1: testes.** Modo `EPROCIA_TESTE_STDERR` no `TestMain` (escreve `erro de teste em C:\Users\fulano\x.png` no stderr e sai com 3), sempre com `EPROCIA_TESTE_FILHO=1`.
  - `TestRodarComPrazo_StderrNoErro`: `errors.As(err, &ep)` dá `ep.Codigo == 3`, `ep.Stderr` contém "erro de teste", `ep.Programa` é o nome do executável; `err.Error()` não contém o stderr.
  - `TestRodarComPrazo_PrazoContinuaErrPrazo`: com o modo `DORMIR` e prazo curto, `errors.Is(err, ErrPrazo)` e `errors.As(err, &ep)` são verdadeiros.
  - Os testes existentes de `poppler_test.go` (`ErrSenha`, cancelamento) passam sem mudança.
- [ ] **Passo 2: rodar** `go test ./internal/ocr -run "TestRodarComPrazo_" -v`: falham.
- [ ] **Passo 3: implementar** `ErroProcesso` em `prazo.go` e o uso em `tesseract.go`/`poppler.go`. O texto que vai ao `.txt` não muda: `fraseOCR` e `textoErroSaida` continuam decidindo a frase fixa (confira `TestSaida_*` e `erro_saida_test.go`).
- [ ] **Passo 4: rodar** `go test -race ./internal/ocr` sem `-short`, `go test -race ./internal/etl` e o gate Go.
- [ ] **Passo 5: commit** `feat(ocr): código de saída e stderr no erro do processo externo, para o log`.

---

### Task 7: logger por item e documento, ErroEtapa e linhas de ZIP e de documento

**Files:**
- Create: `internal/etl/etapa.go`
- Modify: `internal/etl/pipeline.go`, `internal/etl/queue.go`
- Test: `internal/etl/registro_campos_test.go` (novo)

**Interfaces:**
- Consumes: `registro.Logger`, `registro.ComLogger` (Task 3); `ocr.ErroProcesso` (Task 6).
- Produces:
  - `type ErroEtapa struct { Etapa string; Err error }` com `Error()` igual a `Err.Error()` (o texto da linha e da notificação não muda) e `Unwrap()`. Etapas: `abrir_zip`, `ler_pdf`, `ocr`, `html`, `formatar`, `gravar`. `ProcessZipComAtividade` embrulha os erros de ZIP inteiro (`abrir_zip`, `gravar`).
  - `func atributosErro(err error) []any`: `erro` (cadeia completa), `etapa` (de `ErroEtapa`, se houver), `classe` (`string(classificarErro(err))`), `codigo_saida` e `stderr` (de `ocr.ErroProcesso`, se houver).
  - `Queue.processItem` põe no `ctx` do pipeline `registro.ComLogger(ctx, slog.Default().With("item_id", item.ID))`; o pipeline deriva o logger do documento com `doc_id` (índice no ZIP) e o põe no `ctx` passado a `ParsePages`.
  - Linhas de ZIP (`INFO`, no pipeline): "zip iniciado" (`zip` = nome, `bytes`, `pdf`, `html`, `config` com `ocr_mode` e `cleaning_level`) e "zip concluído"/"zip com erro"/"zip cancelado" (`duracao_ms`, `bytes_txt`, `tokens`, e `atributosErro` no erro). Linha de documento (`INFO`, "documento concluído"): `doc` (nome), `evento`, `extensao`, `bytes`, `paginas`, `leitor` (`biblioteca` ou `poppler`), `estado`, `leitura`, `detalhe`, `duracao_ms`; documento com erro leva `atributosErro`. Pânico: `ERROR` com `pilha` (`debug.Stack()`) em `pipeline.go` e em `queue.go`.

- [ ] **Passo 1: testes** (`registro_campos_test.go`, logger JSON em memória no `ctx`, nível `INFO`):
  - `TestRegistro_CamposDeZipEDocumento`: ZIP sintético (geradores de `pdf_sinteticos_test.go`, nome `5000000-00.2026.8.26.0001.zip`, um PDF nativo e um HTML): exatamente uma linha "zip iniciado" e uma "zip concluído" com o `item_id` do contexto; uma "documento concluído" por documento, com `doc_id` distinto e `doc` igual ao nome no ZIP; nenhuma linha tem `pag`; nenhuma contém o texto do PDF.
  - `TestRegistro_ErroComEtapaEClasse`: ZIP que não abre (bytes aleatórios) devolve erro com `errors.As(err, &ErroEtapa{})` e `Etapa == "abrir_zip"`; `err.Error()` igual à mensagem do `zip.OpenReader` direto no mesmo arquivo; a linha "zip com erro" tem `etapa` e `classe`.
  - `TestRegistro_PanicComPilha`: com um documento que entra em pânico (gancho de teste existente ou PDF que já provoca pânico), a linha `ERROR` tem `pilha` não vazia.
- [ ] **Passo 2: rodar** e ver falhar.
- [ ] **Passo 3: implementar.**
- [ ] **Passo 4: rodar** os testes e o gate Go.
- [ ] **Passo 5: commit** `feat(etl): log com item_id, doc_id, etapa e classe; linhas de ZIP e documento`.

---

### Task 8: migração mecânica das chamadas restantes para slog

**Files:**
- Modify: `app.go`, `main.go` (o que sobrar depois da Task 5), `bandeja.go` (se tiver log), `internal/etl/pipeline.go` e `internal/etl/queue.go` (o que sobrar), `internal/etl/abertura.go`, `internal/etl/zip_reader.go`, `internal/ocr/tesseract.go`, `internal/ocr/poppler.go`, `internal/watchdog/watchdog.go`, `internal/watchdog/falhas.go`
- Test: `internal/etl/registro_volume_test.go` (estende)

**Interfaces:**
- Consumes: `registro.Logger` (Task 3); `atributosErro` (Task 7).
- Produces: nenhuma chamada `log.Print*`/`log.Fatal*` em produção. Fila e janela (pausa, retomada, cancelamento, reprocessar, watchdog, bandeja, notificação) em `INFO`/`WARN` com atributos; `MonitoredPath` e caminhos vão como atributo (o manipulador saneia); nomes de ZIP e de documento ficam.

- [ ] **Passo 1: teste** `TestRegistro_SemLogPrintfEmProducao` (em `registro_volume_test.go`): nenhum `.go` de produção em `.` e `internal/...` (fora de `_test.go` e de `internal/ocr/versao/gerar`) chama `log.Print*` ou `log.Fatal*`. O caminho do teste até a raiz é `../..`.
- [ ] **Passo 2: rodar** e ver falhar (lista os arquivos que sobram).
- [ ] **Passo 3: migrar** arquivo por arquivo, sem mudar comportamento.
- [ ] **Passo 4: rodar** o teste e o gate Go (com `go test -race ./internal/ocr` sem `-short`).
- [ ] **Passo 5: commit** `refactor: chamadas de log restantes em slog estruturado`.

---

### Task 9: fatos por página e a categoria

**Files:**
- Modify: `internal/etl/pdf_parser.go`, `internal/etl/pdf_poppler.go`, `internal/etl/pdf_ocr_despacho.go`, `internal/etl/pipeline.go` (grava `categoria` depois de `ParsePages` e do HTML)
- Create: `internal/etl/categoria.go`
- Test: `internal/etl/categoria_test.go`

**Interfaces:**
- Produces:
  - Chaves novas no mapa da página, gravadas onde o mapa é montado: `"erro_leitura": true` (página `[ERRO ao ler página...]`), `"motivo_ocr": string` (página que foi ao OCR: `padrao`, `ilegivel`, `imagem_descoberta`, `isca`; `forcado` vem na Task 12), `"ocr_sem_ganho": true` (OCR sem erro, `conteudoAposOCR` devolveu `ocr == false`).
  - Constantes `CatComErro = "com_erro"`, `CatSemOCR = "sem_ocr"`, `CatVazia = "vazia"`, `CatLidaOCR = "lida_ocr"`, `CatLidaNativa = "lida_nativa"`.
  - `func categoriaPagina(p map[string]any) string`: primeira regra que serve, na ordem da spec: `erro_leitura`, ou `ocr_erro` sem `ocr_aviso` → `com_erro`; `sem_ocr` → `sem_ocr`; `strings.TrimSpace(content) == ""` → `vazia`; `ocr == true` → `lida_ocr`; resto → `lida_nativa`.
  - `func (m motivoOCR) String() string` com os nomes acima.
  - O pipeline grava `"categoria"` em cada página dos documentos **lidos** (depois de `classificarPaginas` decidir que não é `NaoLido`), na goroutine do documento, só no mapa dele.
  - Linhas `Debug`: "página analisada" em `ParsePages` (`pag`, `bytes_nativo`, `imagem`, `vetorial`, `motivo`, `fracao_imagem`, `fracao_texto`, `fracao_descoberta` de `metricasPagina`) e "página classificada" no pipeline (`pag`, `categoria`, `motivo`, `juncao` quando `motivo == imagem_descoberta` e `ocr`).

- [ ] **Passo 1: testes** (`categoria_test.go`; PDFs sintéticos dos geradores existentes; OCR injetado onde preciso):
  - `TestCategoriaPagina_Tabela`: um mapa por linha da tabela da spec, inclusive `ocr_erro` + `ocr_aviso` → `lida_nativa` e `ocr_erro` sem `ocr_aviso` com marcador → `com_erro`.
  - `TestCategorias_PorCaminho`: texto nativo → `lida_nativa`; escaneado com OCR → `lida_ocr`; escaneado com OCR desligado → `sem_ocr`; sem texto e sem imagem → `vazia`; OCR que falha sem nativo → `com_erro`; OCR que falha com nativo → `lida_nativa` e `ocr_aviso`; OCR que devolve branco com nativo → `lida_nativa` e `ocr_sem_ganho`; página nula → `vazia`; leitura que falha (`paginaComObjetoTrocado`) → `com_erro`; Poppler (`trocarPoppler` com fakes) sem texto e OCR desligado → `sem_ocr`.
  - `TestCategorias_PoucoTextoSemImagem`: página com 30 caracteres e sem imagem nunca é `sem_ocr` nem `com_erro`, com OCR ligado ou desligado.
  - `TestCategorias_RegrasNovasSemOCR`: com OCR desligado, a página de imagem descoberta e a de isca (geradores de `pdf_regras_test.go`) saem `lida_nativa`.
  - `TestCategorias_UmaPorPagina`: num ZIP com nativo, escaneado, omitido por tipo e não lido, toda página de documento lido tem exatamente uma `categoria`, e as de omitido e não lido não têm.
  - `TestRegistro_PaginaClassificadaSoEmDebug`: o mesmo ZIP com logger JSON em memória no `ctx`: em nível `Debug`, uma linha "página classificada" por página lida, com `pag`, `categoria` e `motivo` (este nas que foram ao OCR); em nível `Info`, nenhuma linha "página analisada" nem "página classificada".
- [ ] **Passo 2: rodar** e ver falhar.
- [ ] **Passo 3: implementar.** Nenhuma decisão de OCR muda (`pdf_regras_test.go`, `pdf_juncao_test.go` e `pdf_sem_ocr_test.go` passam sem alteração).
- [ ] **Passo 4: rodar** os testes e o gate Go (`go test -race ./internal/etl` sem `-short`).
- [ ] **Passo 5: commit** `feat(etl): categoria de cada página lida`.

---

### Task 10: GerarSaida, contadores da limpeza e a linha do cabeçalho

**Files:**
- Create: `internal/models/relatorio.go` (só `LimpezaRelatorio` nesta tarefa)
- Modify: `internal/etl/formatter.go`, `PRD.md` (§6)
- Test: `internal/etl/formatter_relatorio_test.go` (novo), `internal/etl/testdata/saida_antes/` (goldens novos)

**Interfaces:**
- Produces:
  - Em `models`: `type LimpezaRelatorio struct { Nivel string \`json:"nivel"\`; BytesAntes int64 \`json:"bytes_antes"\`; BytesDepois int64 \`json:"bytes_depois"\`; BytesDuplicatas int64 \`json:"bytes_duplicatas"\`; TokensAntes int \`json:"tokens_antes"\`; TokensDepois int \`json:"tokens_depois"\` }`.
  - Em `etl`: `type SaidaConsolidada struct { Texto string; Tokens int; Limpeza models.LimpezaRelatorio; DocsDuplicados int; PaginasDuplicadas int }`; `func GerarSaida(eventos map[string]any, nivel string, paginasSemOCR int) SaidaConsolidada`; `GenerateConsolidatedOutput(eventos, nivel)` passa a ser `s := GerarSaida(eventos, nivel, 0); return s.Texto, s.Tokens` (os 60 chamadores de teste não mudam).
- Regras de contagem, no laço que monta `saem` (não em `cleanPage`/`cleanLinhas`): antes += `len(p.content)`; depois += `len(texto limpo)` antes de acrescentar `avisoOCRComReserva`; uma página que a limpeza esvaziou conta no antes com depois 0. Ficam fora: páginas `semOCR`, páginas trocadas por referência de H7 e todas as páginas de documento trocado por referência (essas somam `len(p.content)` em `BytesDuplicatas` e contam em `PaginasDuplicadas`), documentos omitidos e não lidos. `TokensAntes`/`TokensDepois` = soma das runas dos mesmos textos dividida por 4 (a regra de `EstimateTokens`). Em `none` os contadores são preenchidos do mesmo jeito; quem decide não mostrar a redução é a interface e a telemetria.
- Linha do cabeçalho: com `paginasSemOCR > 0`, `fmt.Sprintf("Páginas que precisavam de OCR e ficaram sem: %d\n", n)` logo depois de `Documentos não lidos` e antes de `Data de Consolidação`; entra no cálculo de `Tokens estimados` como o resto do cabeçalho.

- [ ] **Passo 1: goldens de antes.** **Antes de mexer em `formatter.go`**, escreva `TestGerarSaida_SemLinhaIgualAHoje` com um modo de gravação (`-atualizar`): para três mapas de teste existentes (nativo, com H7, com `ocr_aviso`) e os quatro níveis, grava a saída de `GenerateConsolidatedOutput` em `testdata/saida_antes/<mapa>-<nivel>.txt`, com a data trocada por `DATA` pela mesma regex dos testes de golden. Rode com `-atualizar` uma vez, na versão de hoje; sem a flag, o teste compara `GerarSaida(e, n, 0).Texto` (com a mesma troca de data) com esses arquivos. Os goldens entram no commit.
- [ ] **Passo 2: demais testes** (`formatter_relatorio_test.go`):
  - `TestGerarSaida_LinhaSemOCR`: com N = 3 a linha existe, entre as duas citadas (compare índices); `Tokens == EstimateTokens(Texto)`.
  - `TestGerarSaida_DepoisIgualTextoEmitido`: oráculo independente: extrai do `.txt` o texto de cada `<pag>` (e do documento de uma página), retira a linha `avisoOCRComReserva` e as referências de H7, soma os bytes; a soma é igual a `Limpeza.BytesDepois`. O oráculo não chama `cleanPage`, `cleanLinhas` nem `prepararLinhas`.
  - `TestGerarSaida_AntesMaiorOuIgual`: nos quatro níveis, `BytesAntes >= BytesDepois`; em `none`, `BytesAntes == BytesDepois` para páginas sem espaços nas bordas.
  - `TestGerarSaida_ForaDasSomas`: página `sem_ocr` e página duplicada não entram em antes nem depois; `BytesDuplicatas` é o tamanho do conteúdo da duplicada; documento duplicado inteiro conta em `DocsDuplicados` e as páginas dele em `PaginasDuplicadas`.
- [ ] **Passo 3: rodar** e ver falhar (os goldens passam pelo invólucro antes da mudança só quando `GerarSaida` existir; com o esqueleto, falham).
- [ ] **Passo 4: implementar** e atualizar o PRD §6: a linha no exemplo do cabeçalho, entre `Documentos não lidos` e `Data de Consolidação`, e o item "Páginas que precisavam de OCR e ficaram sem: só aparece quando maior que 0; são as páginas com imagem ou desenho cujo texto do PDF não bastou e o OCR estava desligado".
- [ ] **Passo 5: rodar** os testes (todos os `formatter_*_test.go` passam sem alteração) e o gate Go.
- [ ] **Passo 6: commit** `feat(etl): contadores da limpeza e linha de páginas sem OCR no cabeçalho`.

---

### Task 11: RelatorioZip, medidas de OCR e de fases, e o relatório na fila

**Files:**
- Modify: `internal/models/relatorio.go` (os demais tipos), `internal/etl/pipeline.go`, `internal/etl/pdf_parser.go` (opção `comMedidas`), `internal/etl/pdf_ocr_despacho.go` (conta refações), `internal/etl/atividade.go`, `internal/etl/queue.go`
- Create: `internal/etl/relatorio.go` (`medidasOCR`, `estatDoc`, montagem)
- Test: `internal/etl/relatorio_test.go`, `internal/etl/queue_relatorio_test.go`

**Interfaces:**
- Consumes: `categoriaPagina` e as chaves de página (Task 9); `GerarSaida`, `models.LimpezaRelatorio` (Task 10).
- Produces (em `models`; o JSON é o contrato do frontend na Task 15 e da telemetria na Task 14):

```go
type RelatorioZip struct {
	Documentos DocsRelatorio    `json:"documentos"`
	Paginas    PaginasRelatorio `json:"paginas"`
	Tempo      TempoRelatorio   `json:"tempo"`
	OCR        OCRRelatorio     `json:"ocr"`
	Limpeza    LimpezaRelatorio `json:"limpeza"`
	BytesTxt   int64            `json:"bytes_txt"`
	Tokens     int              `json:"tokens"`
	Atencao    []DocAtencao     `json:"atencao,omitempty"`
}
type DocsRelatorio struct { // total, lidos, nao_lidos, omitidos, duplicados_h7, pdf, html
	Total, Lidos, NaoLidos, Omitidos, DuplicadosH7, PDF, HTML int
	TiposOmitidos []string       `json:"tipos_omitidos,omitempty"`
	PorLeitura    map[string]int `json:"por_leitura,omitempty"`
}
type PaginasRelatorio struct { // total, lida_nativa, lida_ocr, vazia, sem_ocr, com_erro,
	// ocr_sem_ganho, ocr_falhou_com_reserva, de_nao_lidos, de_omitidos, duplicadas
	Total, LidaNativa, LidaOCR, Vazia, SemOCR, ComErro       int
	OCRSemGanho, OCRFalhouComReserva                         int
	DeNaoLidos, DeOmitidos, Duplicadas                       int
	PorMotivo map[string]int `json:"por_motivo,omitempty"`
}
type TempoRelatorio struct { // em ms: total_ms, abrir_ms, pre_contagem_ms, leitura_parede_ms,
	// leitura_soma_ms, ocr_trabalho_ms, ocr_parede_ms, formatacao_ms, gravacao_ms
	TotalMs, AbrirMs, PreContagemMs, LeituraParedeMs, LeituraSomaMs, OCRTrabalhoMs, OCRParedeMs, FormatacaoMs, GravacaoMs int64
}
type OCRRelatorio struct { // processos, blocos, paginas_em_bloco, refacoes, prazos,
	// bloco_medio_ms, bloco_mediana_ms, bloco_p95_ms, pagina_estimada_ms
	Processos, Blocos, PaginasEmBloco, Refacoes, Prazos              int
	BlocoMedioMs, BlocoMedianaMs, BlocoP95Ms, PaginaEstimadaMs       int64
}
type DocAtencao struct { // nome, evento, sem_ocr, com_erro, ocr_falhou_com_reserva (números de página)
	Nome string; Evento int; SemOCR, ComErro, OCRFalhouComReserva []int
}
```

  - `PorLeitura` conta os documentos não lidos por `Leitura`. É também a contagem "por classe de `classificarErro`" da spec: um documento que falha recebe a `Leitura` que `classificarErro` devolve (`naoLido(classificarErro(e), ...)` em `pipeline.go`), então as duas classes são a mesma contagem.
  - `PaginaEstimadaMs` = tempo de trabalho dos blocos / `PaginasEmBloco` (0 sem blocos); é o valor que a interface rotula "estimativa por página". `BlocoMedioMs` = trabalho dos blocos / `Blocos`.
  - `type medidasOCR struct` (atômicos e um mutex só para a lista de durações de bloco) com `registrar(paginas int, dur time.Duration, err error)` (chamado nos executores de `ocrDoDoc`, depois de obter a vaga: conta processo, bloco quando `paginas > 1`, prazo quando `errors.Is(err, ocr.ErrPrazo)`, e atualiza o início mínimo e o fim máximo para a parede) e `refacao()`; `(*PDFParser).comMedidas(m *medidasOCR) *PDFParser`; `despachoOCR` chama `refacao()` quando um bloco cai para página a página.
  - `(*Atividade).definirRelatorio(r *models.RelatorioZip)` e `(*Atividade).Relatorio() *models.RelatorioZip` (aceitam receptor nil).
  - `QueueItem.Relatorio *models.RelatorioZip \`json:"relatorio,omitempty"\``: zerado no início de `processItem` (junto dos contadores), em `Reprocessar` (junto com `UnreadDocs`), e deixado `nil` em erro e cancelamento; preenchido no sucesso com `atv.Relatorio()`. `copiarItem` copia o ponteiro (o relatório não muda depois de pronto).
  - `N` da linha do cabeçalho: o pipeline soma as páginas `sem_ocr` dos documentos lidos e passa a `GerarSaida`.
  - Linhas de log: "documento concluído" ganha `categorias` (mapa); "zip concluído" ganha `relatorio` (`slog.Any`; o manipulador saneia a linha final); blocos de OCR em `Debug` ("bloco de OCR", `paginas`, `duracao_ms`), refação em `Warn`.

- [ ] **Passo 1: testes** (`relatorio_test.go`, ZIPs sintéticos, OCR injetado):
  - `TestRelatorio_SomaDasCategoriasIgualAoTotal`: ZIP com nativo, escaneado com OCR, escaneado sem OCR, página em branco, documento omitido por tipo, documento não lido e uma página duplicada (H7, nível `standard`): `LidaNativa+LidaOCR+Vazia+SemOCR+ComErro == Total`; `DeOmitidos` e `DeNaoLidos` iguais às páginas pré-contadas desses documentos; `Duplicadas >= 1` e fora da soma.
  - `TestRelatorio_SubContadores`: `OCRFalhouComReserva` igual ao número de páginas com `ocr_aviso` (inclusive as com `ocr_erro`, que não contam em `ComErro`); `OCRSemGanho` igual ao das páginas com `ocr_sem_ganho`.
  - `TestRelatorio_LinhaDoCabecalhoSoDosLidos`: documento lido com 2 páginas `sem_ocr` e documento inteiro sem OCR (não lido, `sem_ocr`): o `.txt` traz `Páginas que precisavam de OCR e ficaram sem: 2`; sem página `sem_ocr`, a linha não existe.
  - `TestRelatorio_TemposEOCR`: com OCR injetado que dorme 20 ms por chamada, `OCRTrabalhoMs >= 20 × processos`, `Processos > 0`, `BlocoP95Ms >= BlocoMedianaMs`, `PaginaEstimadaMs <= BlocoMedioMs`, `TotalMs >= LeituraParedeMs`; com bloco que falha uma vez, `Refacoes == 1`.
  - `TestRelatorio_Atencao`: documento com páginas 2 e 5 `sem_ocr` aparece em `Atencao` com `SemOCR == [2 5]`; documento sem problema não aparece.
  - `TestRelatorio_PorLeitura`: um documento com senha e um ilegível dão `PorLeitura == {"senha":1,"ilegivel":1}`.
  - `TestRelatorio_ZipSemPaginasLidas` (Review Focus): ZIP só com documentos omitidos: `Paginas.Total == 0`, `json.Marshal` do relatório não contém `NaN` nem `Inf`, `BlocoMedioMs == 0`, `PaginaEstimadaMs == 0`.
  - `queue_relatorio_test.go`, `TestQueue_RelatorioZeradoAoReprocessarECancelar` (Review Focus): o `processar` falso (teste interno do pacote) acha a atividade do item por `item.atividade` sob `q.mu` (a fila a cria e grava antes de chamar `processar`) e chama `definirRelatorio`. O item `success` tem `Relatorio != nil` e o `queue:item` serializado tem a chave `relatorio`; `Reprocessar` deixa `nil` antes de rodar; uma execução cancelada e uma com erro terminam com `nil`, mesmo que a anterior tivesse relatório.
- [ ] **Passo 2: rodar** e ver falhar.
- [ ] **Passo 3: implementar.** A soma final é feita uma vez, numa só goroutine, depois de `wg.Wait()`, a partir de um `[]estatDoc` indexado pelo documento que só a goroutine dele escreve; nada novo dentro do mutex de `Atividade`. Mediana e p95 por ordenação da lista de durações.
- [ ] **Passo 4: rodar** os testes e o gate Go (`-race`, `internal/etl` sem `-short`).
- [ ] **Passo 5: commit** `feat(etl): relatório do processamento por ZIP no item da fila`.

---

### Task 12: Forçar OCR no núcleo (config, motivo, parser, Poppler, pipeline)

**Files:**
- Modify: `internal/models/models.go`, `internal/config/config.go`, `internal/etl/pdf_juncao.go`, `internal/etl/pdf_parser.go`, `internal/etl/pdf_poppler.go`, `internal/etl/pdf_ocr_despacho.go`, `internal/etl/pipeline.go`
- Test: `internal/config/config_test.go`, `internal/etl/forcar_ocr_test.go` (novo)

**Interfaces:**
- Consumes: `motivoOCR.String()` e `motivo_ocr` (Task 9).
- Produces:
  - `models.Config.ForcarOCR bool \`json:"forcar_ocr"\``; `DefaultConfig()` com `false`; `configOverlay` ganha o campo (backfill).
  - `motivoForcado` (depois de `motivoIsca`; `String()` = `forcado`). Em `conteudoAposOCR` segue a regra de `motivoPadrao` (OCR legível substitui; em branco ou ilegível fica o nativo). Em `registrarOCR`, `regraNovaComReserva` passa a incluir `motivoForcado`.
  - `(*PDFParser).comForcarOCR(forcar bool) *PDFParser`. Em `ParsePages`, depois de `analisarPagina`, se `forcar && ocrEnabled && ocrRunner != nil && !an.precisaOcr` (e a página não é nula nem com erro), a página vai ao OCR com `motivoForcado` e o texto nativo como reserva. Em `parsePoppler`, a página com texto que não seria agendada vai do mesmo jeito.
  - O pipeline chama `comForcarOCR(cfg.ForcarOCR && cfg.OcrMode != "none")`. "zip iniciado" ganha `forcar_ocr` no grupo `config`.

- [ ] **Passo 1: testes.**
  - `config_test.go`: `TestLoad_ForcarOCRAusenteEhFalse` (JSON antigo sem a chave) e `TestLoad_ForcarOCRComNonePreservado` (`{"ocr_mode":"none","forcar_ocr":true}` carrega e salva com `true`).
  - `forcar_ocr_test.go` (OCR injetado; ZIP com PDF nativo de 3 páginas, HTML e um omitido):
    - `TestForcarOCR_TodaPaginaDePDFLidoVaiAoOCR`: 3 páginas despachadas com `motivo_ocr == "forcado"`; o HTML e o omitido não.
    - `TestForcarOCR_ResultadoSegueMotivoPadrao`: OCR legível substitui o nativo (`lida_ocr`); em branco ou ilegível fica o nativo (`lida_nativa`, `ocr_sem_ganho`); falha mantém o nativo com `ocr_aviso` e sem `ocr_erro`, e um documento de uma página continua lido.
    - `TestForcarOCR_PrecedenciaDoMotivo`: página escaneada (motivo padrão) com forçar ligado mantém `motivo_ocr == "padrao"` e, na falha sem nativo, `ocr_erro`.
    - `TestForcarOCR_PaginasSemConteudo` (Review Focus): página em branco vai ao OCR e, com OCR vazio, sai `vazia`; página nula e página com erro de leitura não são despachadas e mantêm `vazia`/`com_erro`.
    - `TestForcarOCR_NoneIgnora`: `ForcarOCR = true` com `OcrMode = none` não despacha nada.
    - `TestForcarOCR_Poppler`: com `trocarPoppler` (fakes), página com texto longo é despachada com `forcado`.
- [ ] **Passo 2: rodar** e ver falhar.
- [ ] **Passo 3: implementar.**
- [ ] **Passo 4: rodar** os testes e o gate Go (`internal/etl` e `internal/config` sem `-short`).
- [ ] **Passo 5: commit** `feat(etl): Forçar OCR em todas as páginas de PDF`.

---

### Task 13: Forçar OCR por Reprocessar (fila e App)

**Files:**
- Modify: `internal/etl/queue.go`, `app.go`
- Modify: `internal/etl/queue_reprocessar_test.go` e demais chamadores de `Reprocessar` (assinatura nova)
- Test: `internal/etl/queue_reprocessar_test.go`, `app_test.go`
- Generated: `frontend/src/wailsjs/go/main/App.js`, `App.d.ts`, `frontend/src/wailsjs/go/models.ts`

**Interfaces:**
- Consumes: `models.Config.ForcarOCR` (Task 12).
- Produces:
  - `ajusteConfig` ganha `forcarOCR *bool`; `func (q *Queue) Reprocessar(id, ocrMode, cleaningLevel string, forcarOCR *bool) error` (nil = usa a global); `processItem` aplica o ponteiro sobre a cópia da config. `QueueItem.ForcarOCR bool \`json:"forcar_ocr,omitempty"\`` guarda o efetivo da última execução.
  - `func (a *App) ReprocessQueueItem(id, ocrMode, cleaningLevel string, forcarOCR bool) error` (sempre explícito: passa `&forcarOCR`). Até a Task 16, o `ProcessList` chama com 3 argumentos; o binding gerado passa `undefined`, que chega como `false` (aceito no intervalo).

- [ ] **Passo 1: testes.**
  - `TestReprocessar_ForcarOCRSobreGlobal`: global `false` + `&true` força; global `true` + `&false` não força; global `true` + `nil` força.
  - `TestReprocessar_NaoMudaConfigGlobal`: depois de qualquer `Reprocessar`, `q.cfg` e o `config.json` de teste ficam iguais byte a byte.
  - `TestReprocessar_NoneComGlobalTrue`: não força.
  - `app_test.go`, `TestReprocessQueueItem_PassaForcarExplicito`: `ReprocessQueueItem(id, "normal", "standard", false)` com global `true` não força (é o caminho do botão "Ler com OCR").
- [ ] **Passo 2: rodar** e ver falhar.
- [ ] **Passo 3: implementar**, atualizar os chamadores de `Reprocessar` nos testes (passam `nil`) e rodar `wails generate module`.
- [ ] **Passo 4: rodar** os testes e o gate Go.
- [ ] **Passo 5: commit** `feat: Forçar OCR por Reprocessar, sem mudar a config global`.

---

### Task 14: telemetria local e origem do ZIP

**Files:**
- Create: `internal/telemetria/telemetria.go`, `internal/telemetria/maquina_windows.go`, `internal/telemetria/maquina_other.go`
- Modify: `internal/models/relatorio.go` (`ExecucaoZip`), `internal/etl/queue.go`, `app.go`, `registro_app.go`, `frontend/src/App.svelte` (o soltar usa o método novo)
- Test: `internal/telemetria/telemetria_test.go`, `internal/etl/queue_telemetria_test.go`, `registro_app_test.go` (estende)
- Generated: `frontend/src/wailsjs/go/main/App.js`, `App.d.ts`

**Interfaces:**
- Consumes: `models.RelatorioZip` (Task 11); `registro.AbrirRotativo`, `registro.Ativo` (Task 3); `QueueItem.ForcarOCR` (Task 13).
- Produces:
  - Em `models`: `type ExecucaoZip struct { ItemID string; Reprocesso bool; Origem string; Config Config; BytesZip int64; Status string; ClasseErro string; Relatorio *RelatorioZip }`.
  - Em `etl`: `func (q *Queue) SetAoTerminarZip(fn func(models.ExecucaoZip))`, chamado uma vez por execução para `success`, `error` (inclusive o retorno antecipado de "Erro ao carregar configurações" em `processItem`) e `cancelled`, sempre fora de `q.mu`; `func (q *Queue) AddFilesComOrigem(paths []string, origem string) int` (`AddFiles` = origem `manual`); origens `monitorada`, `arrastada`, `manual`. `Reprocesso` é verdadeiro quando o item tinha `ajuste`. Linha da fila `INFO` "item na vez" (`origem`, `reprocesso`), gravada em `processItem` antes do pipeline.
  - `func (a *App) AddDroppedFiles(filePaths []string) int` (origem `arrastada`); o watchdog usa `monitorada`. `App.svelte` chama `AddDroppedFiles` no `OnFileDrop`.
  - Em `telemetria` (importa só a stdlib, `golang.org/x/sys/windows` e `internal/models`; nunca `internal/etl`, que puxa `net` pelo runtime do Wails): `type Maquina struct { CPUs int; MemoriaTotal uint64; Windows string; Build uint32; Arquitetura string }`, `func ColetarMaquina() Maquina` (`windows.RtlGetVersion()` e `GlobalMemoryStatusEx` por `windows.NewLazySystemDLL("kernel32.dll")`); `type Registro struct` com JSON `versao`, `ts`, `item_id`, `reprocesso`, `maquina`, `config` (`ocr_mode`, `cleaning_level`, `forcar_ocr`, `tipos_omitidos` como contagem, `origem`), `tamanhos` (`bytes_zip`, `pdf`, `html`, `paginas`, `bytes_txt`, `tokens`), `tempos_ms` (as fases de `TempoRelatorio`), `paginas` (categorias, `por_motivo`, sub-contadores), `ocr` (`OCRRelatorio`), `limpeza` (bytes, `reducao_pct` só com nível diferente de `none` e antes > 0, `duplicadas`, `docs_duplicados`, `omitidos`), `falhas` (`por_leitura`), `resultado` (`status`, `classe_erro`); `func Montar(e models.ExecucaoZip, m Maquina, versao string, agora time.Time) Registro` (nunca copia `Atencao`, `TiposOmitidos` nem nome); `type Gravador struct`; `func NovoGravador(pastaDados string) (*Gravador, error)` (`<pasta>\telemetria\telemetria.jsonl`, 10 MB, 5 arquivados, canal de 64 com goroutine própria; cheio descarta e conta); `(*Gravador).Gravar(r Registro)`; `(*Gravador).Fechar()`. Falha de gravação: um `WARN` no log (uma vez até a próxima gravação bem-sucedida), registro descartado.
  - Em `registro_app.go`: `func ligarTelemetria(q *etl.Queue, pastaDados string) (fechar func())`, chamada em `App.startup` depois de criar a fila e só quando `registro.Ativo()`: cria o `Gravador`, coleta a máquina uma vez e chama `q.SetAoTerminarZip`; `fechar` vai para `App.shutdown`. Inativo, nada disso executa (a fila nasce depois de `iniciarRegistro`, por isso a ligação fica no `startup`).

- [ ] **Passo 1: testes.**
  - `TestTelemetria_SemRede`: `go list -deps eprocia/internal/telemetria` (via `exec.Command` no teste) não contém `net` nem `net/http`.
  - `TestTelemetria_SemNomes`: `Montar` com `ExecucaoZip` cujo relatório tem `Atencao` com `5000000-00.2026.8.26.0001 - Evento 3 - PET1.pdf`, `TiposOmitidos` `["EXTR"]` e `Config.MonitoredPath` `C:\Users\fulano\Processos`: o JSON não contém `5000000`, `PET1`, `EXTR`, `fulano` nem `Processos`; contém `tipos_omitidos: 1` e `reducao_pct`.
  - `TestTelemetria_NoneSemReducao`: nível `none` (e antes 0) não tem a chave `reducao_pct`.
  - `TestGravador_UmaLinhaPorRegistroERotacao`: limite reduzido no teste (campo interno); 3 registros = 3 linhas JSON válidas; acima do limite gira para `telemetria.1.jsonl`; nenhum arquivo é apagado por idade.
  - `TestGravador_FalhaDescartaEAvisaUmaVez`: destino que devolve erro (gancho interno); dois `Gravar` não entram em pânico nem bloqueiam; o log em memória (`slog.Default()` trocado, restaurado no cleanup) tem exatamente um `WARN` de falha da telemetria; nenhum registro foi gravado.
  - `queue_telemetria_test.go`, `TestQueue_AoTerminarZip`: um `success`, um `error` (com `ClasseErro`), um `cancelled` e um erro de "carregar configurações" (`carregarConfig` falso que falha) chamam o callback uma vez cada, fora da trava (o callback chama `q.GetItems()` sem travar); item reprocessado chega com `Reprocesso == true`; origem `arrastada` chega como tal.
  - `registro_app_test.go`, `TestIniciarRegistro_SemVariavelSemTelemetria`: processar um ZIP sintético por uma fila de teste, com `iniciarRegistro("", dir, cfg)` seguido do mesmo caminho do `startup` (`ligarTelemetria` só se `registro.Ativo()`), não cria `telemetria\` nem `logs\`; com `"1"`, existe `telemetria\telemetria.jsonl` com uma linha e o mesmo `item_id` da linha "zip concluído" do log; o log contém o nome do ZIP sintético e o do documento, e não contém o texto do PDF; a telemetria não contém nenhum dos dois nomes.
- [ ] **Passo 2: rodar** e ver falhar.
- [ ] **Passo 3: implementar** e rodar `wails generate module`.
- [ ] **Passo 4: rodar** os testes, o gate Go e o gate frontend (mudou `App.svelte`).
- [ ] **Passo 5: commit** `feat(telemetria): registro local por ZIP, sem nomes e sem rede`.

---

### Task 15: frontend, linha-resumo, painel do relatório e "Ler com OCR"

**Files:**
- Create: `frontend/src/lib/textos.js`, `frontend/src/lib/relatorio.js`, `frontend/src/components/RelatorioItem.svelte`
- Modify: `frontend/src/components/ProcessList.svelte`, `frontend/src/lib/reprocessar.js`
- Test: `frontend/relatorio.test.mjs` (novo), `frontend/reprocessar.test.mjs`

**Interfaces:**
- Consumes: `QueueItem.relatorio` (JSON da Task 11); `ReprocessQueueItem(id, ocr, limpeza, forcar)` (Task 13).
- Produces:
  - `textos.js`: **todos** os textos novos de interface deste plano (Tasks 15 e 16), como constantes e funções de plural, para troca rápida depois que o dono vir as capturas:
    - `textoPaginasSemOCR(n)`: `"6 páginas têm imagem e o OCR estava desligado"` / `"1 página tem imagem e o OCR estava desligado"`.
    - `textoPaginasComErro(n)`: `"3 páginas não puderam ser lidas"` / `"1 página não pôde ser lida"`.
    - `textoOCRFalhouComReserva(n)`: `"2 páginas ficaram com o texto do PDF porque o OCR falhou"` / `"1 página ficou com o texto do PDF porque o OCR falhou"`.
    - `EXPLICACAO_OCR = "Página em branco ou só com texto não vai ao OCR: isso é normal. O aviso aparece só quando a página tem imagem ou desenho e o texto do PDF não bastou."`
    - `ROTULO_LER_COM_OCR = "Ler com OCR"`; rótulos do painel (`"Texto do PDF"`, `"Lida por OCR"`, `"Em branco"`, `"Precisava de OCR e ficou sem"`, `"Com erro"`, `"Tempo total"`, `"Tempo de trabalho de OCR"`, `NOTA_TEMPO_OCR = "Soma dos processos de OCR em paralelo; pode passar do tempo total."`, `"Estimativa por página"`, `"Limpeza"`, `"Páginas repetidas"`, `"Tipos omitidos"`).
    - Para a Task 16: `ROTULO_FORCAR_OCR_CONFIG = "Forçar OCR em todas as páginas de PDF"`, `AJUDA_FORCAR_OCR_CONFIG = "Cada página passa pelo OCR, inclusive as que já têm texto: o processamento fica bem mais lento e, numa página com texto bom, o resultado pode sair um pouco pior. Páginas repetidas também passam pelo OCR."`, `ROTULO_FORCAR_OCR_REPROCESSAR = "Forçar OCR em todas as páginas"`, `AJUDA_FORCAR_OCR_REPROCESSAR = "Mais lento; páginas repetidas também passam pelo OCR."`, `ROTULO_ABRIR_LOGS = "Abrir pasta de logs"`, `TEXTO_LOG_ATIVO = "Log e telemetria ativos. Estes arquivos ficam só neste computador."`.
  - `relatorio.js` (funções puras, textos importados de `textos.js`):
    - `resumoPaginas(rel)` → string ou `null`: `null` sem relatório ou com `paginas.total === 0`; senão `"<total> páginas: <n> do texto do PDF, <n> por OCR, <n> em branco"`, omitindo as partes zeradas e com singular ("1 página: 1 do texto do PDF").
    - `avisosPaginas(rel)` → lista de `{ tipo, texto }`, só para `sem_ocr`, `com_erro`, `ocr_falhou_com_reserva` maiores que 0, nunca para `vazia`.
    - `mostrarLerComOCR(rel)` → `paginas.sem_ocr > 0`.
    - `reducaoLimpeza(rel)` → inteiro de 0 a 100 ou `null` (`null` com `limpeza.nivel === 'none'` ou `bytes_antes === 0`).
  - Em `reprocessar.js`: `argumentosLerComOCR(item, config)` → `{ ocr, limpeza, forcar: false }`, com `ocr = config.ocr_mode` se não for `'none'`, senão `'normal'`, e `limpeza = item.cleaning_level || config.cleaning_level`.
- `RelatorioItem.svelte` (no estilo de `AtividadeItem.svelte`, classes de tema, sem cor fixa): categorias, tempo total e tempo de trabalho de OCR com `NOTA_TEMPO_OCR`, `ocr.pagina_estimada_ms` rotulado "Estimativa por página", limpeza (bytes antes/depois e redução só quando `reducaoLimpeza` não é `null`), duplicatas, tipos omitidos, e a lista `atencao` com os números de página. `ProcessList.svelte` mostra a linha-resumo em todo item `success` com `resumoPaginas` não nulo (clicável, `aria-expanded`, abre o painel), os avisos em tom de aviso abaixo dela, a `EXPLICACAO_OCR` quando há aviso, e o botão "Ler com OCR" quando `mostrarLerComOCR`, que chama `ReprocessQueueItem(id, ocr, limpeza, false)` com `argumentosLerComOCR`.

- [ ] **Passo 1: testes.**
  - `relatorio.test.mjs`: `resumo com total 0` (Review Focus: `null`, e nenhuma função devolve `NaN` com relatório zerado); singular e plural; aviso só com as três categorias e nunca para `vazia`; `reducaoLimpeza` `null` em `none` e com antes 0, e `25` para antes 400 / depois 300; `relatorio` ausente (item antigo) não lança.
  - `reprocessar.test.mjs`: `argumentosLerComOCR` com global `ocr_mode: 'none'` dá `normal`; com `'best'` dá `best`; `forcar` é sempre `false`, inclusive com `config.forcar_ocr = true`; mantém o nível do item.
  - Convenção por regex (em `relatorio.test.mjs`): `ProcessList.svelte` importa `RelatorioItem.svelte` e mostra `ROTULO_LER_COM_OCR` dentro de `{#if mostrarLerComOCR(`; `RelatorioItem.svelte` não tem cor literal (`#[0-9a-f]{3,6}` ou `rgb(`) fora de `var(`; nenhum `.svelte` tocado nesta tarefa tem literal com "OCR estava desligado", "não puderam ser lidas" ou "Ler com OCR" (o texto vem de `textos.js`).
- [ ] **Passo 2: rodar** `node --test frontend/relatorio.test.mjs frontend/reprocessar.test.mjs` e ver falhar.
- [ ] **Passo 3: implementar.**
- [ ] **Passo 4: rodar** o gate frontend (`contraste.test.mjs` e `paleta.test.mjs` passam sem afrouxar regex).
- [ ] **Passo 5: commit** `feat(frontend): resumo do processamento, painel de detalhes e Ler com OCR`.

---

### Task 16: frontend, Forçar OCR nas Configurações e no Reprocessar, e o log

**Files:**
- Create: `frontend/src/lib/log.js`
- Modify: `frontend/src/components/Settings.svelte`, `frontend/src/components/ReprocessarItem.svelte`, `frontend/src/components/ProcessList.svelte` (repassa `forcar` ao `ReprocessQueueItem`), `frontend/src/App.svelte` (chama `LogAtivo()` uma vez e repassa `logAtivo`)
- Test: `frontend/forcar-ocr.test.mjs` (novo), `frontend/log-ui.test.mjs` (novo), `frontend/reprocessar.test.mjs`

**Interfaces:**
- Consumes: `config.forcar_ocr` (Task 12); `LogAtivo()`, `AbrirPastaLogs()` (Task 5); `ReprocessQueueItem(id, ocr, limpeza, forcar)` (Task 13); os textos de `textos.js` (Task 15).
- Produces:
  - `log.js`: `itensLog(logAtivo)` → `[]` com `false`; `[{ tipo: 'botao', texto: ROTULO_ABRIR_LOGS }, { tipo: 'texto', texto: TEXTO_LOG_ATIVO }]` com `true`.
  - `Settings.svelte`: `export let logAtivo = false`; caixa `ROTULO_FORCAR_OCR_CONFIG` ligada a `config.forcar_ocr`, `disabled={config.ocr_mode === 'none'}` (o valor não é alterado ao desabilitar), salva por `SaveConfig` como os outros campos, com `AJUDA_FORCAR_OCR_CONFIG`. Só dentro de `{#if logAtivo}`: o que `itensLog(logAtivo)` devolve, com o botão chamando `AbrirPastaLogs`.
  - `ReprocessarItem.svelte`: `export let forcarInicial = false` (o valor da config global); caixa `ROTULO_FORCAR_OCR_REPROCESSAR`, desabilitada com `ocr === 'none'`, com `AJUDA_FORCAR_OCR_REPROCESSAR`; `aoConfirmar(ocr, limpeza, forcar)`.
- [ ] **Passo 1: testes.**
  - `forcar-ocr.test.mjs` (regex nos `.svelte`): a caixa existe nos dois componentes, com `disabled` ligado a `=== 'none'`; o valor inicial do Reprocessar vem de `forcarInicial`; `aoConfirmar` recebe três argumentos; o `ProcessList` passa `config.forcar_ocr` como `forcarInicial` e repassa o terceiro argumento ao `ReprocessQueueItem`.
  - `log-ui.test.mjs`: extrai do template de `Settings.svelte` (sem `<script>`, `<style>` e comentários) os trechos fora de `{#if logAtivo}`…`{/if}` e confere que nenhum casa `/\blogs?\b|telemetria/i` (sem pegar "login" nem "catálogo"); confere `export let logAtivo = false`; confere que `App.svelte` chama `LogAtivo()` uma vez e passa `{logAtivo}` ao `Settings`; `itensLog(false)` é vazio e `itensLog(true)` tem o botão e o texto.
  - `reprocessar.test.mjs` atualizado para o terceiro argumento.
- [ ] **Passo 2: rodar** e ver falhar.
- [ ] **Passo 3: implementar.**
- [ ] **Passo 4: rodar** o gate frontend.
- [ ] **Passo 5: commit** `feat(frontend): Forçar OCR nas Configurações e no Reprocessar; pasta de logs só com log ativo`.

---

### Task 17: hook proteger-dados e .gitignore para *.log e *.jsonl

**Files:**
- Modify: `.claude/hooks/proteger-dados.mjs`, `.gitignore`
- Test: `.claude/hooks/hooks.test.mjs`

**Interfaces:**
- Produces: `arquivoSensivel(caminho)` devolve `true` para `*.log` e `*.jsonl` fora de `docs/`, `scripts/` e `.claude/` (a mesma exceção de pasta de `TXT_PERMITIDO`); `.gitignore` ganha `*.log` e `*.jsonl`.

- [ ] **Passo 1: testes** em `hooks.test.mjs` (bloco `proteger-dados`): `eprocia.log`, `logs/eprocia.1.log`, `telemetria.jsonl` e `C:/Users/x/AppData/Roaming/eprocIA/telemetria/telemetria.1.jsonl` são sensíveis; `docs/exemplo.log` e `scripts/anonimizar-log/testdata/entrada.jsonl` não; `git add eprocia.log` é bloqueado e `git add -A` com um `.jsonl` não ignorado também.
- [ ] **Passo 2: rodar** `node --test .claude/hooks/hooks.test.mjs` e ver falhar.
- [ ] **Passo 3: implementar.**
- [ ] **Passo 4: rodar** o gate de harness.
- [ ] **Passo 5: commit** `chore(harness): proteger-dados e .gitignore barram *.log e *.jsonl`.

---

### Task 18: scripts/anonimizar-log

**Files:**
- Create: `scripts/anonimizar-log/main.go`, `scripts/anonimizar-log/anonimizar.go`
- Test: `scripts/anonimizar-log/anonimizar_test.go`, `scripts/anonimizar-log/testdata/entrada.jsonl` (sintético)

**Interfaces:**
- Consumes: o formato de linha da Task 3 e os atributos `zip`, `doc` (Task 7) e `relatorio.atencao[].nome` (Task 11).
- Produces: `func anonimizar(r io.Reader, w io.Writer) (sobras []int, err error)`: troca cada nome de ZIP (atributo `zip`, e o mesmo texto onde aparecer em `msg` ou `erro`) por `zip-N` e cada nome de documento (`doc`, `nome` dentro de `relatorio.atencao`) por `doc-N`, com N estável na ordem da primeira aparição; mantém o resto da linha; `sobras` lista os números das linhas que ainda contêm `\Users\` ou `/Users/` seguido de algo que não seja `<usuario>`. `main` lê o arquivo do argumento e escreve `<nome>.anon.log` ao lado; imprime as sobras e sai com 1 se houver.
- [ ] **Passo 1: testes:** `TestAnonimizar_RotulosEstaveis` (o mesmo ZIP em três linhas vira sempre `zip-1`; o segundo ZIP vira `zip-2`; documentos viram `doc-1`, `doc-2`); `TestAnonimizar_NomeDentroDoErro` (o nome do ZIP dentro de `erro` também é trocado); `TestAnonimizar_SobraDeCaminho` (linha com `C:\Users\fulano\x` aparece em `sobras`; `C:\Users\<usuario>\x` não); `TestAnonimizar_LinhaInvalidaSegue` (linha que não é JSON é copiada e entra em `sobras` se tiver caminho).
- [ ] **Passo 2: rodar** `go test ./scripts/anonimizar-log -v` e ver falhar.
- [ ] **Passo 3: implementar.**
- [ ] **Passo 4: rodar** os testes e o gate Go.
- [ ] **Passo 5: commit** `feat(scripts): anonimizar-log troca nomes de ZIP e documento por rótulos`.

---

### Task 19: estados visuais e capturas "depois"

**Files:**
- Modify: `scripts/visual/estados.mjs`, `scripts/visual/capturar.mjs`, `scripts/visual/estados.test.mjs`, `.claude/skills/eprocia-visual/SKILL.md` (estados e contagem)
- Create: `docs/revisao/capturas/2026-11-24-relatorio/depois/` (PNGs)

**Interfaces:**
- Consumes: JSON de `relatorio` (Task 11), `forcar_ocr` (Task 12), `LogAtivo` (Task 5), seletores e textos das Tasks 15 e 16; capturas "antes" (Task 1).
- Produces: estados novos em `ESTADOS`, só com dados fictícios: `relatorio` (fila com três itens `success`: um só nativo, um com OCR e páginas em branco, um com `sem_ocr`, `com_erro` e `ocr_falhou_com_reserva` e `atencao` preenchida), com as variações `-relatorio` (painel aberto no item com aviso) e `-ler-ocr` (foco no botão "Ler com OCR"); `configuracoes` passa a ter `LogAtivo: false` explícito e `forcar_ocr: false`; `configuracoes-log` (`LogAtivo: true`, `forcar_ocr: true`, `ocr_mode: 'normal'`) e `configuracoes-none` (`forcar_ocr: true`, `ocr_mode: 'none'`, caixa desabilitada marcada); a variação `-reprocessar` do estado `fila` mostra a caixa Forçar OCR.

- [ ] **Passo 1: testes.** `estados.test.mjs` confere que `relatorio` de cada item tem as chaves da Task 11 e que a soma das cinco categorias é igual a `paginas.total`; que os novos estados só usam `processo(n)` e `C:\Usuarios\exemplo`; e que o stub atende `LogAtivo`. Rode e veja falhar.
- [ ] **Passo 2: implementar** os estados e as variações em `capturar.mjs`; atualize a matriz e o total de capturas no `SKILL.md`.
- [ ] **Passo 3: capturar** `node scripts/visual/capturar.mjs --saida docs/revisao/capturas/2026-11-24-relatorio/depois --estados fila,configuracoes,nao-lidos,relatorio,configuracoes-log,configuracoes-none`. Confira nos dois temas e em 800×600: nada transborda, o aviso usa as cores de aviso do tema, o botão de log só aparece em `configuracoes-log`. Encerre o Vite pelo PID se o script não encerrar.
- [ ] **Passo 4: relatório de pares.** No relatório da tarefa, liste os pares antes/depois (Task 1) de `fila`, `configuracoes` e `nao-lidos`, e as capturas novas sem par; liste também todos os textos de `textos.js`, para o dono aprovar pelas capturas.
- [ ] **Passo 5: rodar** o gate de harness e o gate frontend.
- [ ] **Passo 6: commit** `docs(capturas): relatório, Forçar OCR e log nas Configurações`.

---

### Task 20: documentação

**Files:**
- Modify: `README.md`, `CLAUDE.md`, `DESIGN.md`, `PRD.md` (o relatório na interface e o Forçar OCR nos requisitos), `.claude/review.json` (arquivos novos nas lentes: `internal/registro/*.go`, `internal/telemetria/*.go`, `scripts/anonimizar-log/*.go`; os `.svelte` já entram pelo glob)
- Test: nenhum teste novo; `node --test .claude/workflows/workflows.test.mjs` cobre o `review.json`.

**Interfaces:**
- Consumes: nomes e comportamentos das Tasks 2 a 18.

- [ ] **Passo 1: `README.md`:** seção "Log e telemetria (opcional)": definir `EPROCIA_LOG` nas Variáveis de Ambiente do usuário ou com `setx EPROCIA_LOG 1` (ou `detalhado`); a variável só chega a processos novos: sair pela bandeja ("Sair") e abrir de novo; quem abre pelo atalho, pelo início automático com o Windows ou pelo `install.ps1` herda o ambiente do Explorer, que só relê a variável depois de reiniciar o Explorer ou sair e entrar no Windows; onde ficam os arquivos; o que contém (nomes de ZIP e de documento no log; telemetria sem nomes); partilhar só a telemetria para não expor nomes, ou passar o log por `scripts/anonimizar-log`. Seção curta sobre o relatório e o Forçar OCR.
- [ ] **Passo 2: `CLAUDE.md`:** em Commands, `EPROCIA_LOG=1` / `detalhado` e onde ficam os arquivos; na arquitetura, `internal/registro`, `internal/telemetria` (sem `internal/etl`, por causa do `net` do Wails), `models.RelatorioZip` e `categoria`, `GerarSaida`, `ForcarOCR`/`motivoForcado`, textos novos em `frontend/src/lib/textos.js`; troque a frase "that is the single log line v0.4.2 added (no per-page diagnostic log...)" por: o log por página só existe em `Debug` (`EPROCIA_LOG=detalhado`), e o Form XObject ignorado é `Warn`; no hook `proteger-dados`, `*.log` e `*.jsonl`; a armadilha de `slog.SetDefault` (redireciona o pacote `log`; teste restaura `log.Writer()` e `log.Flags()`).
- [ ] **Passo 3: `DESIGN.md`:** linha-resumo, tom de aviso, painel `RelatorioItem`, botão "Ler com OCR", caixas Forçar OCR, bloco de log condicionado. **`PRD.md`:** requisito do relatório do processamento e do Forçar OCR (o §6 já foi na Task 10).
- [ ] **Passo 4: rodar** o gate de harness.
- [ ] **Passo 5: commit** `docs: relatório do processamento, Forçar OCR e EPROCIA_LOG`.

---

### Task 21: benchmark e gate completo

**Files:**
- Nenhum arquivo do repositório muda. Saídas no scratchpad.

**Interfaces:**
- Consumes: a branch inteira.

- [ ] **Passo 1: base.** Skill `eprocia-bench`, só fixtures sintéticas, sem `EPROCIA_BENCH_DIR` e sem `EPROCIA_LOG`: worktree de `7e44221` no scratchpad (copie `go.sum` e `internal/ocr/embed/{poppler,tesseract}`), `-count=10` para `BenchmarkSintetico` (`none` e `fast`) e `BenchmarkParsePDF`.
- [ ] **Passo 2: depois.** Os mesmos comandos no HEAD; `go tool benchstat` base contra HEAD. Critério: nenhuma piora com p < 0.05 e nenhuma métrica pior que 5%. Se piorar, rode os dois binários alternados como a skill manda antes de concluir; piora que persiste vai como `DONE_WITH_CONCERNS` com os números.
- [ ] **Passo 3: gate completo:** `pwsh scripts/build/gate.ps1`, gate frontend e gate de harness, sem `EPROCIA_LOG`.
- [ ] **Passo 4: remova** a worktree de base (`git worktree remove`) e registre no relatório a tabela do benchstat e a saída decisiva dos gates. Sem commit.
