# Revisão do eprocIA — 2026-10-15

## Escopo

Revisão geral (escopo `tudo`). No backend (Go) rodaram as lentes EF (eficiência), ES (estabilidade), GM (Go moderno), TE (testes) e RN (regras de negócio e produto). No frontend (Svelte) rodaram VI (visual), AN (animação e movimento), RS (responsividade e redimensionamento), AC (acessibilidade) e DF (desempenho do frontend). As telas foram capturadas com dados fictícios em quatro tamanhos, dois temas e oito estados.

Cada achado passou por um cético. Sobreviveram 80 achados e 4 foram derrubados. Os achados que descreviam o mesmo problema no mesmo trecho foram fundidos, e cada um cita todas as lentes que o encontraram. Ficaram **65 achados**: 32 no backend e 33 no frontend.

A ordem é gravidade × ganho ÷ custo (alta = 3, média = 2, baixa = 1; P = 1, M = 2, G = 3). Os empates foram resolvidos pelo ganho esperado. Os ids são numerados por lente, nessa ordem. Nada foi corrigido nesta etapa.

## Resumo

| # | Id | Título | Gravidade | Custo | Ganho esperado | Veredito |
|---|----|--------|-----------|-------|----------------|----------|
| 1 | B-ES-01 | Chamadas à biblioteca de PDF fora de recover derrubam o app inteiro | alta | P | PDF com xref inconsistente vira erro do documento, sem derrubar o app | mantido |
| 2 | B-RN-01 | "Iniciar com o Windows" aparece na interface mas não faz nada | alta | P | Opção funciona e o watchdog fica ativo desde o logon | mantido |
| 3 | B-ES-02 | systray.Run em goroutine sem LockOSThread pode deixar o tray sem resposta | média | P | Tray sempre responde | mantido |
| 4 | B-ES-03 | Falhas ao iniciar o watchdog e ao carregar a config são engolidas | média | P | Usuário sabe quando a pasta não está sendo monitorada | mantido |
| 5 | B-ES-04 | Falha de OCR vira página vazia silenciosa e o erro de init fica preso no sync.Once | média | P | Fim dos .txt "concluídos" com páginas em branco | mantido |
| 6 | B-EF-01 | Três eventos Wails por página, um com a fila inteira, e process:* sem ouvinte | média | P | −1 de 3 eventos por página; ≈ −90% de mensagens ao webview | mantido |
| 7 | B-EF-02 | A extração dos binários embutidos aloca ≈354 MB e grava 160 MB por execução | média | P | ≈ −350 MB alocados, ≈ −100 MB de pico, ≈ −39 MB de executável | rebaixado |
| 8 | B-TE-01 | Os recovers contra os panics do dslipak/pdf nunca são exercitados | média | P | Armadilhas conhecidas protegidas por teste de regressão | mantido |
| 9 | B-TE-02 | Cancelamento dentro de ParsePages não tem teste nem checagem de vazamento | média | P | Travamento ou vazamento no cancelamento passa a falhar o teste | mantido |
| 10 | B-TE-03 | Config corrompida: ramo de JSON inválido e efeito no startup sem teste | média | P | Contrato da config corrompida fixado em teste | mantido |
| 11 | F-VI-01 | Fontes carregadas do Google Fonts num app offline | média | P | Tipografia determinística e nenhuma requisição externa | mantido |
| 12 | F-RS-01 | Cabeçalho da fila não encolhe: título invade o filtro | média | P | Cabeçalho legível em uma linha | mantido |
| 13 | F-VI-02 | Rodapé com "itemns", contador quebrado, pasta sem truncar e sem versão | média | P | Rodapé correto e estável | mantido |
| 14 | F-AC-01 | Contraste insuficiente de texto sobre accent, sucesso e erro | média | P | Texto legível nos dois temas (WCAG 1.4.3) | mantido |
| 15 | F-AC-02 | Contorno de controles abaixo de 3:1 | média | P | Limites e estado dos controles perceptíveis (WCAG 1.4.11) | mantido |
| 16 | F-AC-03 | Interruptores sem indicador de foco | média | P | Toggles operáveis por teclado de forma visível | rebaixado |
| 17 | F-AC-04 | Campos de formulário removem o outline sem substituto | média | P | Foco visível e consistente | rebaixado |
| 18 | F-AN-01 | Toast de status: entrada de 300ms, saída abrupta e timer que apaga mensagens novas | média | P | Toast simétrico e previsível | mantido |
| 19 | F-VI-03 | Estado pausado contraditório: ícone de pausa com item ainda processando | média | P | Estado da fila legível sem contradição | rebaixado |
| 20 | F-RS-02 | Teto do divisor (480px) ignora a largura da janela | média | P | Fila usável na janela mínima | rebaixado |
| 21 | F-AN-02 | Arraste do divisor: transição atrasa o cursor e posições fora da faixa são descartadas | média | P | Divisor acompanha o cursor 1:1 e encosta nos limites | rebaixado |
| 22 | F-AN-03 | Movimento sem tokens e sem prefers-reduced-motion | média | P | Movimento consistente e preferência do sistema respeitada | rebaixado |
| 23 | F-VI-04 | Paleta de tokens diverge do DESIGN.md | média | P | Identidade coerente com a especificação | mantido |
| 24 | B-EF-03 | Pré-contagem serial grava cada PDF em disco e o reabre | alta | M | ≈ −90% de tempo de parede sem OCR | mantido |
| 25 | B-TE-04 | Pipeline.ProcessZip só roda em benchmark que checa err == nil | alta | M | ProcessZip de 0% para ≈70% de cobertura | mantido |
| 26 | B-TE-05 | Fila (Queue) sem nenhum teste | alta | M | queue.go de 0% para mais de 80% | mantido |
| 27 | B-ES-05 | Árvore /Pages cíclica trava reader.Page para sempre e prende a fila | média | M | PDF malformado não congela a fila | rebaixado |
| 28 | B-RN-02 | ZIPs baixados com o app fechado nunca são processados | baixa | P | Downloads anteriores ao início são processados | mantido |
| 29 | B-RN-03 | A higienização agressiva não remove cabeçalhos e rodapés repetidos | média | M | Estimativa de 5% a 15% menos tokens em documentos longos | mantido |
| 30 | F-AC-05 | Mudanças de estado não são anunciadas | média | M | Leitor de tela informa salvamento e progresso | mantido |
| 31 | F-DF-01 | Cada página processada substitui a fila inteira no frontend | média | M | De centenas para 10 a 60 renderizações por segundo | rebaixado |
| 32 | F-VI-05 | Glassmorphism, gradientes, sombras grandes e hover com deslocamento | média | M | Visual sóbrio e menos composição no WebView2 | mantido / rebaixado |
| 33 | F-VI-06 | Lista pouco densa: linhas de 52px contra os 40px do DESIGN | média | M | ≈ 40% mais itens visíveis | mantido |
| 34 | B-ES-06 | Pré-cálculo de páginas ignora cancelamento | baixa | P | Cancelamento responde em uma iteração | mantido |
| 35 | B-ES-07 | Panic no pré-cálculo vaza PDFs temporários e o contexto do item | baixa | P | Nenhum temporário órfão após falha | rebaixado |
| 36 | B-ES-08 | Data race em Queue.broadcast e SetContext | baixa | P | Remove corrida real | mantido |
| 37 | B-ES-09 | handleNewFile lê w.done sem lock e não entra no WaitGroup do Stop | baixa | P | Stop passa a ser definitivo | mantido |
| 38 | B-ES-10 | config.json gravado sem atomicidade | baixa | P | Config não se corrompe por interrupção | mantido |
| 39 | B-ES-11 | Shutdown não espera a fila | baixa | P | Sair pelo tray não deixa processos nem lixo | rebaixado |
| 40 | B-RN-04 | Falta copiar a saída para a área de transferência | baixa | P | Fluxo principal de 4 ações para 1 clique | mantido |
| 41 | B-RN-05 | A saída não estima tokens | baixa | P | Usuário sabe se o .txt cabe no contexto do LLM | mantido |
| 42 | B-RN-06 | Nenhuma notificação quando o processamento termina com a janela oculta | baixa | P | Aviso de conclusão sem abrir a janela | rebaixado |
| 43 | B-TE-06 | Nenhum teste garante CREATE_NO_WINDOW nos processos do OCR | baixa | P | Regressão de armadilha documentada coberta | mantido |
| 44 | F-VI-07 | Formulário com alinhamentos mistos e textos cortados no painel estreito | baixa | P | Formulário legível em 280px | mantido |
| 45 | F-VI-08 | Tema ignora a preferência do sistema e o fundo claro pisca | baixa | P | Sem flash e conforme o DESIGN | mantido |
| 46 | F-VI-09 | Cores fixas no código ignoram o tema | baixa | P | Uma única fonte de verdade de cor | mantido |
| 47 | F-VI-10 | Hierarquia tipográfica fora da escala do DESIGN | baixa | P | Hierarquia legível e menos cromo | mantido |
| 48 | F-AC-06 | Campo de filtro sem rótulo, só com placeholder | baixa | P | Nome acessível e estado vazio correto | mantido |
| 49 | F-AC-07 | Botões com estado não expõem esse estado | baixa | P | Leitor de tela informa estado real | mantido |
| 50 | F-AC-08 | Divisor sem teclado completo (Home/End, preventDefault) | baixa | P | Divisor operável por teclado | rebaixado |
| 51 | F-RS-03 | Largura e estado recolhido do painel não são persistidos | baixa | P | Layout mantido entre sessões | rebaixado |
| 52 | F-AN-04 | Overlay de arrastar e soltar some de uma vez | baixa | P | Overlay sem piscar | mantido |
| 53 | F-DF-02 | Recolher a barra lateral desmonta Settings e refaz GetConfig | baixa | P | Uma leitura de config e sem salto de layout | rebaixado |
| 54 | F-DF-03 | formatTime cria Date e formata a hora em cada linha a cada atualização | baixa | P | Menos trabalho síncrono na UI | mantido |
| 55 | B-GM-01 | Modernizadores do go fix pendentes | baixa | P | Contadores só acessíveis via atomic; `go fix -diff` zerado | rebaixado |
| 56 | B-GM-02 | sort.Slice no lugar de slices.SortFunc/cmp.Compare | baixa | P | Comparadores tipados | mantido |
| 57 | B-GM-03 | os.IsNotExist e UnixNano()/Millisecond no lugar das APIs atuais | baixa | P | Checagem robusta a erros embrulhados | mantido |
| 58 | B-GM-04 | sync.WaitGroup.Go no lugar de Add/go/Done separados | baixa | P | Some a classe de erro Add/Done desbalanceados | rebaixado |
| 59 | B-EF-04 | Um pdftoppm e um tesseract por página | baixa | M | −13% a −27% de tempo de OCR (medido em série) | rebaixado |
| 60 | B-TE-07 | RunOCR: caminhos de erro sem teste e TestRunOCR vira skip | baixa | M | Quebra da extração deixa de passar como skip | rebaixado |
| 61 | F-AC-09 | Foco se perde ao remover ou cancelar item | baixa | M | Teclado consegue limpar vários itens seguidos | mantido |
| 62 | F-AC-10 | Mensagem completa de erro só acessível por title num div não focável | baixa | M | Diagnóstico acessível ao teclado | rebaixado |
| 63 | F-AN-05 | O fly em cada linha anima ao filtrar e as outras linhas pulam | baixa | M | Lista estável ao filtrar | rebaixado |
| 64 | F-AN-06 | Ao recolher ou expandir, o conteúdo da sidebar aparece e some de repente | baixa | M | Recolher e expandir contínuos | rebaixado |
| 65 | F-VI-11 | Ícones SVG inline com traço inconsistente e fora do mapeamento do DESIGN | baixa | M | Traço uniforme e troca de ícone trivial | rebaixado |

## Achados

### B-ES-01 — Chamadas à biblioteca de PDF fora de recover derrubam o app inteiro

- **Arquivo:** `internal/etl/pdf_parser.go:116`
- **Lente:** ES
- **Evidência:** Só ExtractNativeText e HasDrawnContent têm recover. `reader.Page(pageNum)` (linha 116), `HasImages(page)` (linha 149, que usa Resources/Key/Keys) e `pdf.NewReader` (linha 60) resolvem objetos por `Reader.resolve`, que dá panic com xref inconsistente (dslipak/pdf@v0.0.2, read.go:794/798 `loading %v: found %v`, e 753-780 em object streams). Essas chamadas rodam no goroutine por documento (pipeline.go:241), que não tem recover, e o recover de processItem (queue.go:272) não pega panic de outro goroutine. Prova de conceito: um PDF cujo xref do objeto 3 aponta para o objeto 4 passa por NewReader e por NumPage() = 1, e depois `reader.Page(1)` dá `PANIC: loading {3 0}: found {4 0}`. No app isso encerra o processo, inclusive o tray.
- **Proposta:** Envolver em recover o corpo do goroutine de pipeline.go:241 (marcar o documento com `[ERRO ...]` e seguir) e também o laço por página de ParsePages (Page/HasImages), ou criar um helper `safePage(reader, n) (Page, error)` com recover, para tratar a página como erro e não como queda.
- **Custo:** P
- **Ganho esperado:** PDF com xref inconsistente gera erro só no documento, sem fechar o app nem perder a fila.
- **Veredito:** mantido
- **Justificativa do cético:** pdf_parser.go:116 (reader.Page) e :149 (HasImages) rodam sem recover, e o goroutine de pipeline.go:241 também não tem. O resolve do dslipak (read.go:794-798) dá panic em xref inconsistente, e o recover de processItem não alcança outro goroutine, então o processo inteiro cai.

Status: pendente

### B-RN-01 — "Iniciar com o Windows" aparece na interface mas não faz nada no backend

- **Arquivo:** `internal/models/models.go:10`
- **Lente:** RN
- **Evidência:** `AutoStart bool json:"auto_start"` é salvo e lido (config.go:25, :94-96), e Settings.svelte:159-167 mostra o toggle "Iniciar com o Windows". Um grep por AutoStart/auto_start nos .go só acha config, models e testes. Nenhum código grava a chave `HKCU\...\Run` nem usa `options.App.StartHidden` (o campo existe no wails v2.12.0, pkg/options/options.go:45). O PRD §2 pede uma ferramenta "invisível", que já esteja rodando quando o ZIP é baixado.
- **Proposta:** Criar `internal/autostart` com build tag windows, que grava ou apaga `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\eprocIA` = `"<os.Executable()>" --tray` usando `golang.org/x/sys/windows/registry` (x/sys v0.45.0 já é dependência indireta). `App.SaveConfig` chama o pacote quando `AutoStart` muda, e `startup` sincroniza o valor uma vez. Em main.go, `StartHidden: slices.Contains(os.Args, "--tray")`. Um stub vazio para as demais plataformas e um teste do construtor da linha de comando.
- **Custo:** P
- **Ganho esperado:** A opção que a interface já mostra passa a funcionar, o watchdog fica ativo desde o logon e os ZIPs baixados antes de alguém abrir o app deixam de ser perdidos.
- **Veredito:** mantido
- **Justificativa do cético:** AutoStart só aparece em config.go, models.go e testes, o toggle está em Settings.svelte:155-168 e main.go não tem StartHidden. O plano de 2026-10-14 (Task 14) previa `internal/autostart`, que não existe. O toggle não tem efeito nenhum.

Status: feito

- **Commit:** `8d6b54add404c33597ada6d2f8b1d6ac630657ea`

### B-ES-02 — systray.Run em goroutine sem LockOSThread pode deixar o tray sem resposta

- **Arquivo:** `main.go:22`
- **Lente:** ES
- **Evidência:** O goroutine de main.go:22-24 chama `systray.Run`, que cria a janela oculta (registerSystray) e bombeia mensagens com `GetMessage(hwnd=0)` (systray_windows.go:781-792). Mensagens Win32 ficam presas à thread que criou a janela, mas o goroutine não chama `runtime.LockOSThread()`: o LockOSThread de systray.go:27-29 roda em `init()`, na goroutine principal, que é a do Wails. Se o scheduler mover o goroutine para outra thread, os cliques em Abrir/Sair não chegam, e com HideWindowOnClose não resta outra forma de reabrir ou sair.
- **Proposta:** Chamar `runtime.LockOSThread()` como primeira instrução do goroutine que executa `systray.Run`.
- **Custo:** P
- **Ganho esperado:** O tray responde sempre, sem o cenário de janela oculta impossível de reabrir ou encerrar.
- **Veredito:** mantido
- **Justificativa do cético:** systray.go:27-29 chama LockOSThread só no init, na goroutine principal. O goroutine de main.go:22 cria a janela e bombeia GetMessage sem se prender à thread. A correção é trivial e a perda do tray com HideWindowOnClose é grave.

Status: pendente

### B-ES-03 — Falhas ao iniciar o watchdog e ao carregar a config são engolidas

- **Arquivo:** `app.go:117`
- **Lente:** ES (relacionado: B-TE-03, B-ES-10)
- **Evidência:** Os erros de `watcher.Start` são descartados em app.go:53 e app.go:117 (`_ = a.watcher.Start(...)`). SaveConfig devolve nil (linha 123) mesmo quando a pasta não existe ou o fsnotify falha, e a interface mostra o monitoramento como ativo sem que ele esteja. Em startup (linhas 47-55), um erro de `config.Load` (por exemplo, JSON corrompido, config.go:79-81) é ignorado sem log: o watchdog não sobe e `a.cfg` fica nil.
- **Proposta:** Devolver o erro de Start em SaveConfig, ou emitir o evento `watchdog:error`, e registrar em log (pt-BR) as falhas de Start e de `config.Load` no startup. Com JSON inválido, cair para DefaultConfig com aviso.
- **Custo:** P
- **Ganho esperado:** O usuário fica sabendo quando a pasta monitorada não está sendo observada, em vez de downloads ignorados em silêncio.
- **Veredito:** mantido
- **Justificativa do cético:** Conferidos o `_ = a.watcher.Start` em app.go:53 e :117, o `return nil` de SaveConfig e o erro de config.Load ignorado no startup. Start falha de fato com pasta inexistente (watchdog.go:55-72), e a interface não fica sabendo.

Status: pendente

### B-ES-04 — Falha de OCR vira página vazia silenciosa e o erro de init fica preso no sync.Once

- **Arquivo:** `internal/etl/pdf_parser.go:90`
- **Lentes:** ES, TE (fundido com "Falha do OCR com fallback para texto nativo nunca é exercitada")
- **Evidência:** Quando ocrRunner falha, runOCRTask só registra em log e grava `content: task.fallback` (linhas 90-98), que nas páginas escaneadas é vazio (menos de 100 caracteres). O item termina como `success` sem nenhum marcador. InitBinaries (tesseract.go:27-50) guarda `initErr` dentro de `sync.Once`: uma falha transitória na extração (por exemplo, `os.WriteFile` sobre tesseract.exe em uso ou bloqueado pelo antivírus, linha 86) faz todo OCR falhar até reiniciar o app. A lente TE mostrou que esse bloco (pdf_parser.go:91-97) tem contagem 0 no coverprofile: todos os ocrRunner falsos dos testes devolvem sucesso (pdf_parser_test.go:117-120, 194-196, 236-238; scripts/bench/fixtures_test.go:33-36), e o teste chamado `OcrFallback` (pdf_parser_test.go:92) só cobre o caminho de OCR bem-sucedido.
- **Proposta:** Quando o OCR falhar e o fallback estiver vazio, gravar um marcador `[ERRO de OCR: ...]` na página e contar as falhas no item (mensagem ou aviso). Trocar o `sync.Once` por mutex com nova tentativa quando `initErr != nil`. Teste (TDD): ocrRunner que devolve `errors.New("tesseract ausente")` numa página com imagem e pouco texto nativo, verificando `ocr=false`, `image=""`, conteúdo igual ao texto nativo aparado (ou o marcador, quando vazio) e `onPageComplete` chamado uma vez. Renomear o teste atual para `..._UsesOcrForImagePage`.
- **Custo:** P
- **Ganho esperado:** Fim dos .txt "concluídos" com páginas em branco por falha de OCR que ninguém percebe, e o único caminho de degradação do OCR no parser passa a ter teste.
- **Veredito:** mantido (ES); mantido (TE)
- **Justificativa do cético:** Em pdf_parser.go:90-98 a página recebe `task.fallback` sem marcador e o item termina como success. InitBinaries guarda `initErr` no `sync.Once`, então uma falha de `os.WriteFile` bloqueia todo OCR até reiniciar. O bloco de fallback tem contagem 0 no coverprofile.

Status: pendente

### B-EF-01 — Cada página dispara três eventos Wails, um deles com a fila inteira, e os eventos process:* não têm ouvinte

- **Arquivo:** `internal/etl/queue.go:311`
- **Lentes:** EF, DF (fundido com "O app escuta só a fila, mas o backend também emite process:* a cada página", pipeline.go:220; relacionado: F-DF-01)
- **Evidência:** Cada página concluída chama reportProgress (pipeline.go:206-230), que emite `process:progress` e chama onProgress. onProgress (queue.go:298-312) registra um log e chama `broadcast()`. broadcast (367-376) faz `GetItems()`, que copia todos os QueueItem do histórico (nunca podado, a não ser por Clear), emite `queue:update` com a lista completa e ainda emite `queue:status`. No frontend só `queue:update` e `queue:status` têm EventsOn (App.svelte:64-75); `process:start/progress/success/error` são serializados e enviados ao webview sem ninguém ouvindo.
- **Proposta:** Remover os EventsEmit de `process:*` do Pipeline, que a fila já cobre (ou reaproveitar `process:progress` como canal leve de progresso). Limitar o broadcast de progresso a no máximo 10 por segundo por item, coalescendo e sempre enviando a última atualização. Mandar só o item alterado, e o status só quando isPaused/isWorking mudar, em vez da lista inteira mais o status a cada página.
- **Custo:** P
- **Ganho esperado:** −1 de 3 eventos por página de imediato, e o JSON de progresso deixa de crescer com o histórico (O(itens) para O(1)). Em ZIPs com centenas de páginas nativas, ≈ −90% de mensagens ao webview.
- **Veredito:** mantido (EF); mantido (DF)
- **Justificativa do cético:** reportProgress (pipeline.go:219-229) emite `process:progress` e onProgress chama broadcast (queue.go:298-312), que emite a lista inteira e mais `queue:status` a cada página. O grep em frontend/src só acha EventsOn para `queue:update` e `queue:status`.

Status: pendente

### B-EF-02 — A extração dos binários embutidos aloca ≈354 MB e grava 160 MB a cada execução, inclusive modelos que não são usados

- **Arquivo:** `internal/ocr/tesseract.go:74`
- **Lente:** EF
- **Evidência:** extractFS faz `fs.ReadFile(srcPath)`, que copia o conteúdo embutido para o heap, e depois `os.ReadFile(destPath)` mais `bytes.Equal` (linhas 74-84) para todos os arquivos de embed/: 160 MB no total (poppler 24 MB, tesseract 69 MB, tessdata 67 MB). Isso acontece no primeiro RunOCR de cada processo, mesmo com tudo já extraído. O tessdata leva os três modos completos, cada um com osd.traineddata (10,5 MB) e eng.traineddata (4,1 MB) repetidos, e só o modo configurado é usado (linha 120); o osd nunca é pedido (`-l por`, PSM padrão). Com `EPROCIA_BENCH_OCR=normal go test -bench Sintetico -benchtime=1x -count=3 -benchmem` e os binários já extraídos, a primeira operação deu 354.082.472 B/op e as seguintes ≈20,9 MB/op; o pico-mem-B passou de 23,9 MB (ocr=none) para 129 MB.
- **Proposta:** Comparar por tamanho mais um carimbo de versão (por exemplo, um arquivo `.versao` com o hash do build gerado no go:generate) em vez de ler os dois conteúdos inteiros. Quando precisar gravar, fazer streaming com `fs.Open` e `io.Copy`. Extrair sob demanda só o tessdata do modo pedido, e tirar do embed o osd.traineddata e as cópias duplicadas de eng.traineddata, mantendo um eng compartilhado para o fallback.
- **Custo:** P
- **Ganho esperado:** ≈ −350 MB alocados no primeiro OCR de cada sessão, ≈ −100 MB de pico residente (de 129 MB para ≈25-30 MB), −160 MB de leitura de disco por execução e ≈ −39 MB no executável.
- **Veredito:** rebaixado (de alta para média)
- **Justificativa do cético:** tesseract.go:74-84 confirma ReadFile + ReadFile + bytes.Equal para todo o embed, e os três modos repetem osd e eng. Mas o custo é pago uma vez por processo (initOnce), a memória é temporária e recolhida pelo GC, e a comparação byte a byte foi escolhida de propósito para evitar binário velho.

Status: pendente

### B-TE-01 — Os recovers que contornam os panics do dslipak/pdf e o ramo de /Contents em array de HasDrawnContent nunca são exercitados

- **Arquivo:** `internal/etl/pdf_parser.go:256`
- **Lente:** TE
- **Evidência:** Contagem 0 em pdf_parser.go:258-259 (recover de ExtractNativeText), 204-205 (recover de HasDrawnContent) e 211-214 (laço de /Contents em array em HasDrawnContent). O teste ContentsArray (pdf_parser_test.go:186) usa uma página com imagem 200x200, e por causa de `hasDrawn := !hasImages && HasDrawnContent(page)` (linha 150) HasDrawnContent nem é chamado. O stream ilegível (245-246) e o fallback GetPlainText (270-278) também têm contagem 0. Se alguém remover os recovers (armadilha conhecida), nenhum teste falha.
- **Proposta:** Adicionar casos: (a) página com stream de conteúdo usando `/Filter` não suportado (por exemplo `/JBIG2Decode` ou `/Foo`), exigindo que ParsePages não entre em panic e que a página vá ao OCR ou saia vazia; (b) página vetorial com `/Contents [4 0 R 5 0 R]` somando mais de 4096 bytes e sem imagem, exigindo OCR; (c) stream com `/Length` maior que os dados, dentro de parsePagesWithDeadline.
- **Custo:** P
- **Ganho esperado:** Os contornos das armadilhas conhecidas (panic e loop do dslipak/pdf) ficam protegidos por teste de regressão; a cobertura de ExtractNativeText sobe de 62% para mais de 90%.
- **Veredito:** mantido
- **Justificativa do cético:** Os blocos 204-205, 211-214 e 258-259 têm contagem 0, e a linha 150 impede a chamada de HasDrawnContent no teste ContentsArray. Os contornos de armadilhas já documentadas ficam sem regressão.

Status: feito

- **Commit:** `a758577a4572a8759412ef630b5e5a1e86770d53`
- **Benchstat:**

```
Modo padrão sem OCR (EPROCIA_BENCH_OCR não definido), -count=6, antes=HEAD a7dc62e, depois=árvore atual.
sec/op:
Sintetico/ocr=none-12       60.34m ±9%  -> 53.11m ±16%  -11.98% (p=0.004 n=6)
ParsePDF/nativo-12          1.123m ±5%  -> 1.081m ±5%   -3.80% (p=0.002 n=6)
ParsePDF/escaneado-12       259.2µ ±11% -> 235.7µ ±9%   ~ (p=0.093 n=6)
ParsePDF/vetorial-12        15.76m ±6%  -> 14.90m ±5%   -5.42% (p=0.026 n=6)
ParsePDF/contents-array-12  149.8µ ±11% -> 160.5µ ±9%   ~ (p=0.240 n=6)
geomean                     2.106m -> 2.004m  -4.84%
gc-pausa-sec/op: Sintetico 4.268m ±27% -> 3.827m ±13% ~ (p=0.310)
pico-mem-B: Sintetico 26.86Mi ±15% -> 27.11Mi ±0% +0.93% (p=0.002)
procs/op: Sintetico 0 -> 0 ~
B/op:
Sintetico/ocr=none-12       16.66Mi -> 16.77Mi  +0.66% (p=0.002)
ParsePDF/nativo-12          262.7Ki -> 262.6Ki  ~ (p=0.121)
ParsePDF/escaneado-12       54.07Ki -> 54.09Ki  +0.04% (p=0.015)
ParsePDF/vetorial-12        1.844Mi -> 1.844Mi  ~ (p=0.729)
ParsePDF/contents-array-12  34.85Ki -> 34.86Ki  +0.01% (p=0.004)
geomean 437.1Ki -> 437.7Ki +0.14%
allocs/op: Sintetico 293.8k -> 293.8k +0.02% (p=0.041); demais iguais.
Segunda rodada de sec/op não foi necessária: nenhum sec/op piorou com p<0,05.
```

### B-TE-02 — Cancelamento dentro de ParsePages não tem teste nem checagem de vazamento de goroutines

- **Arquivo:** `internal/etl/pdf_parser.go:110`
- **Lentes:** TE, GM (fundido com "Nenhum teste usa o perfil goroutineleak (Go 1.27)", pdf_parser.go:108)
- **Evidência:** Os blocos pdf_parser.go:111-114 (ctx cancelado no laço de páginas, wg.Wait e retorno de erro) e 184-186 (ctx cancelado após o wg.Wait) têm contagem 0. Todos os testes passam `context.Background()`; o parse() de fixtures_test.go:37 usa WithTimeout de 10 s, que nunca expira. A busca por `WithCancel` e `cancel()` em `internal/**/*_test.go` não acha nada. O Go 1.27 registra o perfil `goroutineleak` por padrão (`$GOROOT/src/runtime/pprof/pprof.go:194` e :265), e ParsePages (pdf_parser.go:108-114) e ProcessZip (pipeline.go:239-250) disparam goroutines que bloqueiam em semáforos e dependem de `ctx.Done()` para sair. É o caminho de que o botão Cancelar depende para parar um PDF longo.
- **Proposta:** Teste com PDF de várias páginas com imagem e um ocrRunner que, na primeira chamada, cancela o ctx e devolve `ctx.Err()`. Verificar que ParsePages devolve `context.Canceled`, que o wg.Wait retorna e que nenhuma página posterior chama o runner. Um segundo caso com o ctx já cancelado antes da chamada. Ao final, `runtime.GC()` e `pprof.Lookup("goroutineleak").Count() == 0` (gravando o perfil com `WriteTo(w, 1)` quando falhar), num helper `assertNoLeaks(t)` reaproveitável em queue e watchdog.
- **Custo:** P
- **Ganho esperado:** Um travamento ou vazamento de goroutine OCR no cancelamento passa a falhar em `go test`, em vez de virar acúmulo silencioso numa sessão longa do app na bandeja.
- **Veredito:** mantido (TE); mantido (GM)
- **Justificativa do cético:** Os blocos 111-114 e 184-186 têm contagem 0, não há nenhum teste de cancelamento em internal e o pprof do Go 1.27.1 registra o perfil goroutineleak.

Status: feito

- **Commit:** `a758577a4572a8759412ef630b5e5a1e86770d53`
- **Benchstat:**

```
Modo padrão sem OCR (EPROCIA_BENCH_OCR não definido), -count=6, antes=HEAD a7dc62e, depois=árvore atual.
sec/op:
Sintetico/ocr=none-12       60.34m ±9%  -> 53.11m ±16%  -11.98% (p=0.004 n=6)
ParsePDF/nativo-12          1.123m ±5%  -> 1.081m ±5%   -3.80% (p=0.002 n=6)
ParsePDF/escaneado-12       259.2µ ±11% -> 235.7µ ±9%   ~ (p=0.093 n=6)
ParsePDF/vetorial-12        15.76m ±6%  -> 14.90m ±5%   -5.42% (p=0.026 n=6)
ParsePDF/contents-array-12  149.8µ ±11% -> 160.5µ ±9%   ~ (p=0.240 n=6)
geomean                     2.106m -> 2.004m  -4.84%
gc-pausa-sec/op: Sintetico 4.268m ±27% -> 3.827m ±13% ~ (p=0.310)
pico-mem-B: Sintetico 26.86Mi ±15% -> 27.11Mi ±0% +0.93% (p=0.002)
procs/op: Sintetico 0 -> 0 ~
B/op:
Sintetico/ocr=none-12       16.66Mi -> 16.77Mi  +0.66% (p=0.002)
ParsePDF/nativo-12          262.7Ki -> 262.6Ki  ~ (p=0.121)
ParsePDF/escaneado-12       54.07Ki -> 54.09Ki  +0.04% (p=0.015)
ParsePDF/vetorial-12        1.844Mi -> 1.844Mi  ~ (p=0.729)
ParsePDF/contents-array-12  34.85Ki -> 34.86Ki  +0.01% (p=0.004)
geomean 437.1Ki -> 437.7Ki +0.14%
allocs/op: Sintetico 293.8k -> 293.8k +0.02% (p=0.041); demais iguais.
Segunda rodada de sec/op não foi necessária: nenhum sec/op piorou com p<0,05.
```

### B-TE-03 — Config corrompida: o ramo de JSON inválido não é testado e o efeito no app (erro engolido no startup) nunca é exercitado

- **Arquivo:** `internal/config/config.go:79`
- **Lente:** TE (relacionado: B-ES-03, B-ES-10)
- **Evidência:** O bloco config.go:80 (`return nil, err` após json.Unmarshal) tem contagem 0 no coverprofile, assim como o erro de leitura diferente de NotExist (70) e Load/Save/GetConfigPath (0,0%). Em app.go:47-55, `cfg, err := config.Load(); if err == nil {...}` descarta o erro: com config.json corrompido, `a.cfg` e `q.cfg` ficam nil, o watchdog não sobe e cada item da fila cai em queue.go:314-328 ("Erro ao carregar configurações").
- **Proposta:** Adicionar `TestLoad_CorruptJSON` (conteúdo `{oops`) e `TestLoad_UnreadableFile` (caminho que é diretório). Se o comportamento desejado for "mesclar com padrões" (ver B-ES-03), o teste fixa esse contrato. Para o efeito na fila, cobrir em queue_test o ramo `cfg == nil` com Load falhando, injetando o loader.
- **Custo:** P
- **Ganho esperado:** LoadFromFile de 86% para 100% de cobertura, com o contrato para config corrompida fixado em teste.
- **Veredito:** mantido
- **Justificativa do cético:** O retorno de erro do Unmarshal não tem teste, Load/Save/GetConfigPath estão em 0,0% e app.go:47-55 descarta o erro, levando cada item a "Erro ao carregar configurações".

Status: feito

- **Commit:** `249dfd83e9a3a64d369983e96826024b93f7b338`
- **Benchstat:**

```
Rodada 1 (-count=6, antes=HEAD ce2706a, depois=árvore atual, sem OCR):
                           antes              depois              vs base
sec/op
Sintetico/ocr=none-12      1.367 ± 10%        1.269 ± 14%         ~ (p=0.240 n=6)
ParsePDF/nativo-12         1.060m ± 8%        1.113m ± 8%         ~ (p=0.240 n=6)
ParsePDF/escaneado-12      353.0µ ± 10%       344.1µ ± 12%        ~ (p=0.589 n=6)
ParsePDF/vetorial-12       13.09m ± 9%        13.45m ± 5%         ~ (p=0.240 n=6)
ParsePDF/contents-array-12 260.9µ ± 4%        260.7µ ± 7%         ~ (p=0.589 n=6)
geomean                    4.451m             4.429m              -0.48%
gc-pausa-sec/op
Sintetico/ocr=none-12      5.461m ± 52%       5.520m ± 47%        ~ (p=0.818 n=6)
pico-mem-B
Sintetico/ocr=none-12      18.61Mi ± 23%      22.92Mi ± 0%        +23.17% (p=0.002 n=6)
procs/op
Sintetico/ocr=none-12      0.000 ± 0%         0.000 ± 0%          ~ (p=1.000 n=6)
B/op
Sintetico/ocr=none-12      17.52Mi ± 1%       17.47Mi ± 1%        ~ (p=0.180 n=6)
ParsePDF/nativo-12         262.8Ki ± 0%       262.9Ki ± 0%        ~ (p=0.818 n=6)
ParsePDF/escaneado-12      54.42Ki ± 0%       54.42Ki ± 0%        ~ (p=0.394 n=6)
ParsePDF/vetorial-12       1.844Mi ± 0%       1.845Mi ± 0%        ~ (p=0.132 n=6)
ParsePDF/contents-array-12 35.17Ki ± 0%       35.17Ki ± 0%        ~ (p=0.515 n=6)
geomean                    442.9Ki            442.7Ki             -0.05%
allocs/op: todos ~ (geomean -0.00%)
Amostras brutas de pico-mem-B: antes 19515640 x4, 19777784, 23972088; depois 24004856 a 24037624 (degraus quantizados).

Rodada 2, alternada (-count=1, 6 vezes cada, antes e depois intercalados):
                           antes              depois              vs base
sec/op       Sintetico     1.375 ± 4%         1.352 ± 6%          ~ (p=0.240 n=6)
gc-pausa     Sintetico     4.132m ± 58%       4.599m ± 68%        ~ (p=0.485 n=6)
pico-mem-B   Sintetico     18.84Mi ± 21%      18.62Mi ± 3%        ~ (p=0.255 n=6)
procs/op     Sintetico     0 ± 0%             0 ± 0%              ~
B/op         Sintetico     17.54Mi ± 0%       17.53Mi ± 1%        ~ (p=0.937 n=6)
allocs/op    Sintetico     294.9k ± 0%        295.0k ± 0%         +0.06% (p=0.015 n=6)
```

### F-VI-01 — Fontes carregadas do Google Fonts num app offline: Inter e JetBrains Mono não chegam e o visual cai para Segoe UI

- **Arquivo:** `frontend/index.html:9`
- **Lente:** VI
- **Evidência:** index.html:7-9 tem `<link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono...">`. O PRD RNF01 exige funcionamento offline, então sem rede a tipografia muda conforme o ambiente. `--font-mono` (style.css:16) é declarada e nunca usada.
- **Proposta:** Empacotar a Inter variável localmente (`@fontsource-variable/inter` importado em main.js, que entra no `frontend/dist` embutido) ou adotar `"Segoe UI Variable", "Segoe UI", system-ui` como primária, já que o app é só para Windows. Remover os links externos e a mono até que seja usada.
- **Custo:** P
- **Ganho esperado:** Tipografia determinística e nenhuma requisição externa, coerente com o produto offline.
- **Veredito:** mantido
- **Justificativa do cético:** index.html:7-9 carrega o Google Fonts por rede, contra o RNF01, e `--font-mono` só é declarada em style.css:16.

Status: feito

- **Commit:** `1362d764b9578aad5dd58031c64acfcac2afc098`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-VI-01/)

<details><summary>Pares antes/depois (64)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-claro-configuracoes.png)
- 1024x768-claro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-claro-erro.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-claro-processando.png)
- 1024x768-claro-vazio.png: [antes](capturas/2026-10-15/antes/1024x768-claro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-claro-vazio.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-escuro-erro.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-escuro-processando.png)
- 1024x768-escuro-vazio.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1024x768-escuro-vazio.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-claro-configuracoes.png)
- 1440x900-claro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-claro-erro.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-claro-processando.png)
- 1440x900-claro-vazio.png: [antes](capturas/2026-10-15/antes/1440x900-claro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-claro-vazio.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-escuro-erro.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-escuro-processando.png)
- 1440x900-escuro-vazio.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1440x900-escuro-vazio.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-claro-erro.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-claro-processando.png)
- 1920x1080-claro-vazio.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-claro-vazio.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-escuro-erro.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-escuro-processando.png)
- 1920x1080-escuro-vazio.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-01/1920x1080-escuro-vazio.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-claro-configuracoes.png)
- 800x600-claro-erro.png: [antes](capturas/2026-10-15/antes/800x600-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-claro-erro.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-claro-processando.png)
- 800x600-claro-vazio.png: [antes](capturas/2026-10-15/antes/800x600-claro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-claro-vazio.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-escuro-configuracoes.png)
- 800x600-escuro-erro.png: [antes](capturas/2026-10-15/antes/800x600-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-escuro-erro.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-escuro-processando.png)
- 800x600-escuro-vazio.png: [antes](capturas/2026-10-15/antes/800x600-escuro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-01/800x600-escuro-vazio.png)

</details>

### F-RS-01 — Cabeçalho da fila não encolhe: o título invade o campo Filtrar e o foco do campo empurra os botões

- **Arquivo:** `frontend/src/components/ProcessList.svelte:283`
- **Lente:** RS
- **Evidência:** `.header-left` (269-274) e `.header-actions` (283-287) não têm `min-width: 0`, `flex-shrink` nem `white-space: nowrap`/ellipsis no `h2`. `.search-input:focus { width: 140px; }` (301-302) aumenta o campo em 40px no foco. Em 800x600-escuro-erro.png (janela mínima, painel padrão de 320px, botões Retomar e Limpar Fila), "Fila de Processamento" encosta e se sobrepõe ao campo "Filtrar...". Em 800x600-escuro-processando-divisor-maximo.png o título quebra em duas linhas.
- **Proposta:** No `h2`, `white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-width: 0`; no `.header-left`, `min-width: 0; flex: 1`; `.header-actions` com `flex-shrink: 0`. Trocar o crescimento no foco por `flex: 0 1 140px` com `min-width: 80px`. Abaixo de uma largura útil (`@container (max-width: 420px)`), esconder os rótulos "Pausar"/"Limpar Fila" e deixar só ícones com `title`/`aria-label`.
- **Custo:** P
- **Ganho esperado:** O cabeçalho fica em uma linha e legível em qualquer combinação de janela e divisor.
- **Veredito:** mantido
- **Justificativa do cético:** Os seletores não têm min-width nem ellipsis, o foco aumenta o campo para 140px e a captura 800x600-escuro-erro.png mostra a sobreposição já com a janela mínima e o painel padrão.

Status: feito

- **Commit:** `969c860b74a7286a851debf1e5e1caacdb888fdd`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/)

<details><summary>Pares antes/depois (48)</summary>

- 1024x768-claro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-claro-erro.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-claro-processando.png)
- 1024x768-escuro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-escuro-erro.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-escuro-processando.png)
- 1440x900-claro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-claro-erro.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-claro-processando.png)
- 1440x900-escuro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-escuro-erro.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-escuro-processando.png)
- 1920x1080-claro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-claro-erro.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-claro-processando.png)
- 1920x1080-escuro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-escuro-erro.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-escuro-processando.png)
- 800x600-claro-erro.png: [antes](capturas/2026-10-15/antes/800x600-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-claro-erro.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-claro-processando.png)
- 800x600-escuro-erro.png: [antes](capturas/2026-10-15/antes/800x600-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-escuro-erro.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-escuro-processando.png)

</details>

### F-VI-02 — Rodapé com erro de plural ("itemns"), contador quebrado, pasta sem truncar no estado pausado e sem a versão do app

- **Arquivo:** `frontend/src/App.svelte:365`
- **Lentes:** VI, RS (fundido com "Rodapé com altura fixa quebra o contador em duas linhas e não trunca a pasta no estado pausado", App.svelte:364)
- **Evidência:** App.svelte:365 tem `<span>{items.length} item{items.length !== 1 ? 'ns' : ''} na fila</span>`, que gera "4 itemns na fila" (1440x900-escuro-processando.png). O `footer` tem `height: 26px` (774) e os `.status-item` têm `min-width: 0` (795-800), sem `flex-shrink: 0` nem `nowrap` no contador. Em 800x600-claro-configuracoes.png aparece "0 itemns na / fila" em duas linhas, cortado pela altura fixa. No ramo "Monitoramento pausado" (356-359), o `<span class="status-text">` não tem a classe `truncate` nem `title`, e um caminho longo transborda sem reticências. O DESIGN §5.3 põe a versão no rodapé, mas ela está fixa na barra de título como "v1.0 (Offline)" (App.svelte:230).
- **Proposta:** Corrigir para `items.length === 1 ? 'item' : 'itens'`. Contador com `flex-shrink: 0; white-space: nowrap`; primeiro item com `flex: 1; min-width: 0`, e `truncate` + `title={config.monitored_path}` nos dois ramos que mostram a pasta (opcionalmente truncando o meio do caminho). Levar a versão para o rodapé, lida do build, e deixar a barra de título só com a marca.
- **Custo:** P
- **Ganho esperado:** Texto correto e rodapé sempre em uma linha, com o contador íntegro e o caminho truncado de forma legível.
- **Veredito:** mantido (VI); mantido (RS)
- **Justificativa do cético:** App.svelte:365 gera "itemns", as capturas mostram o erro e a quebra, o footer tem altura fixa de 26px e o ramo pausado não tem truncate nem title. Caminhos longos do OneDrive são um cenário realista.

Status: feito

- **Commit:** `b4060377e70622c354e174c74220ed67f9b36b88`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/)

<details><summary>Pares antes/depois (64)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-configuracoes.png)
- 1024x768-claro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-erro.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-processando.png)
- 1024x768-claro-vazio.png: [antes](capturas/2026-10-15/antes/1024x768-claro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-vazio.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-erro.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-processando.png)
- 1024x768-escuro-vazio.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-vazio.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-configuracoes.png)
- 1440x900-claro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-erro.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-processando.png)
- 1440x900-claro-vazio.png: [antes](capturas/2026-10-15/antes/1440x900-claro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-vazio.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-erro.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-processando.png)
- 1440x900-escuro-vazio.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-vazio.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-erro.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-processando.png)
- 1920x1080-claro-vazio.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-vazio.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-erro.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-processando.png)
- 1920x1080-escuro-vazio.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-vazio.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-configuracoes.png)
- 800x600-claro-erro.png: [antes](capturas/2026-10-15/antes/800x600-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-erro.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-processando.png)
- 800x600-claro-vazio.png: [antes](capturas/2026-10-15/antes/800x600-claro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-vazio.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-configuracoes.png)
- 800x600-escuro-erro.png: [antes](capturas/2026-10-15/antes/800x600-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-erro.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-processando.png)
- 800x600-escuro-vazio.png: [antes](capturas/2026-10-15/antes/800x600-escuro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-vazio.png)

</details>

### F-AC-01 — Contraste insuficiente de texto branco sobre accent, sucesso e erro, e de verde sobre fundo claro

- **Arquivo:** `frontend/src/components/Settings.svelte:296`
- **Lente:** AC (relacionado: F-VI-04)
- **Evidência:** Contrastes pela fórmula WCAG: botão `.btn-primary` "Alterar" (296-299), branco sobre `--accent` #3B82F6 no escuro, 3,68:1. Toast `.toast-status.success` (384-387), 2,54:1 no escuro (#10B981) e 3,77:1 no claro (#059669). `.toast-status.error` no escuro (#EF4444), 3,76:1. Botão "Retomar" (ProcessList.svelte:340-344), #059669 sobre fundo claro, cerca de 3,4-3,8:1. Todos são textos de 11-12px, abaixo do mínimo de 4,5:1.
- **Proposta:** Definir no DESIGN.md e no style.css os tokens `--on-accent`, `--success-strong` e `--error-strong` com pares acima de 4,5:1 (por exemplo #2563EB/#1D4ED8 para botões no escuro, #047857 para verde no claro, #B91C1C para erro). Nos toasts, preferir texto colorido sobre superfície neutra. Fazer junto com F-VI-04.
- **Custo:** P
- **Ganho esperado:** Texto de ação e de feedback legível nos dois temas (WCAG 1.4.3).
- **Veredito:** mantido
- **Justificativa do cético:** Os tokens e o texto #fff sobre eles foram conferidos no style.css, e os contrastes calculados batem com a fórmula WCAG.

Status: feito

- **Commit:** `a6d2951403185dfacc054d6e6b0ed9c6c87a67b5`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/)

<details><summary>Pares antes/depois (56)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-configuracoes.png)
- 1024x768-claro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-erro.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-erro.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-configuracoes.png)
- 1440x900-claro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-erro.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-erro.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-erro.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-erro.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-configuracoes.png)
- 800x600-claro-erro.png: [antes](capturas/2026-10-15/antes/800x600-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-erro.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-configuracoes.png)
- 800x600-escuro-erro.png: [antes](capturas/2026-10-15/antes/800x600-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-erro.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando.png)

</details>

### F-AC-02 — Contorno de controles abaixo de 3:1: trilho do interruptor desligado e bordas de campos quase invisíveis

- **Arquivo:** `frontend/src/components/Settings.svelte:346`
- **Lente:** AC (relacionado: F-VI-04)
- **Evidência:** `.slider { background-color: var(--border); }` (346). `--border` é #1F2937 sobre `--surface` #111827 (1,21:1) no escuro e #E5E7EB sobre #FFFFFF (1,24:1) no claro, então o interruptor desligado mal aparece. Inputs e selects usam a mesma borda (258, 274).
- **Proposta:** Criar `--border-strong` (por exemplo #4B5563 no escuro e #9CA3AF no claro, ambos com pelo menos 3:1) para limites de controles interativos e para o trilho do switch; `--border` fica só para divisórias decorativas.
- **Custo:** P
- **Ganho esperado:** Estado e limites dos controles perceptíveis (WCAG 1.4.11).
- **Veredito:** mantido
- **Justificativa do cético:** `.slider` usa `--border` sobre `--surface`, cerca de 1,2:1 nos dois temas.

Status: feito

- **Commit:** `a6d2951403185dfacc054d6e6b0ed9c6c87a67b5`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/)

<details><summary>Pares antes/depois (56)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-configuracoes.png)
- 1024x768-claro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-erro.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-erro.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-configuracoes.png)
- 1440x900-claro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-erro.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-erro.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-erro.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-erro.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-configuracoes.png)
- 800x600-claro-erro.png: [antes](capturas/2026-10-15/antes/800x600-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-erro.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-configuracoes.png)
- 800x600-escuro-erro.png: [antes](capturas/2026-10-15/antes/800x600-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-erro.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando.png)

</details>

### F-AC-03 — Interruptores (toggle) sem indicador de foco

- **Arquivo:** `frontend/src/components/Settings.svelte:333`
- **Lente:** AC
- **Evidência:** `.switch input { opacity: 0; width: 0; height: 0; }` (333-337) e só existe `input:checked + .slider` (363). Não há regra `input:focus-visible + .slider`. Quem navega com Tab chega em "Monitoramento Ativo" e "Iniciar com o Windows" (91-96, 158-163) sem ver nada.
- **Proposta:** Adicionar `.switch input:focus-visible + .slider { outline: 2px solid var(--border-focus); outline-offset: 2px; }`. Na migração, extrair um componente `Switch.svelte` com o anel de foco embutido.
- **Custo:** P
- **Ganho esperado:** Os dois controles mais importantes da configuração passam a ser operáveis por teclado de forma visível (WCAG 2.4.7).
- **Veredito:** rebaixado (de alta para média)
- **Justificativa do cético:** O foco por teclado fica mesmo invisível, mas é um utilitário local de um usuário só, usado sobretudo com mouse, e o defeito não impede a operação.

Status: feito

- **Commit:** `a6d2951403185dfacc054d6e6b0ed9c6c87a67b5`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/)

<details><summary>Pares antes/depois (56)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-configuracoes.png)
- 1024x768-claro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-erro.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-erro.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-configuracoes.png)
- 1440x900-claro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-erro.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-erro.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-erro.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-erro.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-configuracoes.png)
- 800x600-claro-erro.png: [antes](capturas/2026-10-15/antes/800x600-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-erro.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-configuracoes.png)
- 800x600-escuro-erro.png: [antes](capturas/2026-10-15/antes/800x600-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-erro.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando.png)

</details>

### F-AC-04 — Campos de formulário removem o outline sem colocar outro indicador de foco

- **Arquivo:** `frontend/src/components/Settings.svelte:263`
- **Lente:** AC
- **Evidência:** `input[type="text"] { ... outline: none; }` (263) e `select { ... outline: none; }` (279), sem regra `:focus`. Os selects de OCR e higienização (129, 144) e o campo da pasta ficam sem foco visível. Em ProcessList.svelte:296 o filtro também usa `outline: none` e no foco (301-304) só troca a borda de 1px para `var(--accent)`.
- **Proposta:** Regra global em style.css: `:where(button, input, select, [tabindex]):focus-visible { outline: 2px solid var(--border-focus); outline-offset: 1px; }`, apagando os `outline: none` locais. Registrar no DESIGN.md o token `--focus-ring`.
- **Custo:** P
- **Ganho esperado:** Foco visível e consistente em todos os controles nos dois temas.
- **Veredito:** rebaixado (de alta para média)
- **Justificativa do cético:** O problema é real e não há regra global de foco, mas é cosmético para o público do app e não bloqueia o uso.

Status: feito

- **Commit:** `a6d2951403185dfacc054d6e6b0ed9c6c87a67b5`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/)

<details><summary>Pares antes/depois (56)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-configuracoes.png)
- 1024x768-claro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-erro.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-claro-processando.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-erro.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1024x768-escuro-processando.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-configuracoes.png)
- 1440x900-claro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-erro.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-claro-processando.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-erro.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1440x900-escuro-processando.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-erro.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-claro-processando.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-erro.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/1920x1080-escuro-processando.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-configuracoes.png)
- 800x600-claro-erro.png: [antes](capturas/2026-10-15/antes/800x600-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-erro.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-claro-processando.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-configuracoes.png)
- 800x600-escuro-erro.png: [antes](capturas/2026-10-15/antes/800x600-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-erro.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-01%2BF-AC-02%2BF-AC-03%2BF-AC-04/800x600-escuro-processando.png)

</details>

### F-AN-01 — Toast de status: entra em 300ms, desaparece de uma vez e o timer antigo apaga mensagens novas

- **Arquivo:** `frontend/src/components/Settings.svelte:381`
- **Lentes:** AN, DF (fundido com "O timer do toast em Settings não é cancelado e apaga mensagens novas", Settings.svelte:67; relacionado: F-AC-05)
- **Evidência:** `.toast-status { ... animation: slideUp 0.3s ease-out; }` com `from { transform: translateY(100%); }` só cobre a entrada. `showStatus` zera `statusMessage` num `setTimeout(..., 3000)` (64-71) sem guardar o id, e o `{#if statusMessage}` (173) remove o elemento sem transição. Vários `on:change={handleSave}` seguidos acumulam timers, e o primeiro a disparar apaga a mensagem mais recente antes dos 3 s. Como Settings é desmontado ao recolher a barra (App.svelte:289), os timers sobrevivem ao componente, sem `onDestroy`. Os 300ms passam da faixa de 150-220ms.
- **Proposta:** Trocar o keyframe por `in:fly={{ y: 8, duration: 220, easing: cubicOut }}` e `out:fade={{ duration: 150 }}`, com valores dos tokens de F-AN-03 e zerados em reduced-motion. Guardar o id do timer, chamar `clearTimeout` antes de agendar o próximo e limpar no `onDestroy` (no Svelte 5, um `$effect` com função de limpeza).
- **Custo:** P
- **Ganho esperado:** Entrada e saída simétricas na duração padrão, toast previsível e sem callbacks pendentes depois que o componente é desmontado.
- **Veredito:** mantido (AN); mantido (DF)
- **Justificativa do cético:** O keyframe só cobre a entrada, o `{#if}` remove sem transição e showStatus cria um setTimeout novo sem limpar o anterior, então dois salvamentos seguidos cortam o segundo toast antes da hora.

Status: feito

- **Commit:** `dfba0ef238c8cbac0a7c3b349afaca4abd86ab03`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-AN-01/)

<details><summary>Pares antes/depois (8)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AN-01/1024x768-claro-configuracoes.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AN-01/1024x768-escuro-configuracoes.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AN-01/1440x900-claro-configuracoes.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AN-01/1440x900-escuro-configuracoes.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AN-01/1920x1080-claro-configuracoes.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AN-01/1920x1080-escuro-configuracoes.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AN-01/800x600-claro-configuracoes.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AN-01/800x600-escuro-configuracoes.png)

</details>

### F-VI-03 — Estado pausado contraditório: ícone de pausa num item que continua processando

- **Arquivo:** `frontend/src/components/ProcessList.svelte:152`
- **Lente:** VI
- **Evidência:** ProcessList.svelte:126-128 troca só o ícone (`{#if isPaused} ... pause-circle`). As linhas 152-155 continuam mostrando `{item.message || 'Processando...'}` em `var(--accent)` e a barra azul (168-171). Em App.svelte:39 e 235-238 o `processingCount` não considera `isPaused`, e o chip "1 ativo" segue pulsando. Nas capturas 1440x900-claro-erro.png e 1024x768-escuro-erro.png aparecem juntos o botão "Retomar", o ícone de pausa laranja, "Processando documento 43 de 57" em azul, a barra azul e "1 ativo".
- **Proposta (ajustada pelo cético):** `Queue.Pause` só impede que o próximo item comece; o item em curso continua de fato. O texto, a barra e o "1 ativo" estão corretos, e o que engana é o ícone de pausa no item. Manter o ícone de processamento no item em curso e mostrar o estado da fila à parte, por exemplo um chip "Fila pausada: termina o item atual" na barra da lista, e marcar como "pausados" só os itens pendentes. Derivar tudo de um único valor que combine `item.status` e `isPaused`.
- **Custo:** P
- **Ganho esperado:** O usuário entende que o item atual vai terminar e que os próximos esperam, sem leituras contraditórias.
- **Veredito:** rebaixado
- **Justificativa do cético:** Queue.Pause (queue.go:156-161) só liga `isPaused`, lido em startProcessing, e não interrompe o item em curso. A proposta original de mostrar "nada está rodando" estaria errada. O problema é real, mas é outro: o ícone de pausa engana.

Status: feito

- **Commit:** `b4060377e70622c354e174c74220ed67f9b36b88`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/)

<details><summary>Pares antes/depois (64)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-configuracoes.png)
- 1024x768-claro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-erro.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-processando.png)
- 1024x768-claro-vazio.png: [antes](capturas/2026-10-15/antes/1024x768-claro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-claro-vazio.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-erro.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-processando.png)
- 1024x768-escuro-vazio.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1024x768-escuro-vazio.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-configuracoes.png)
- 1440x900-claro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-erro.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-processando.png)
- 1440x900-claro-vazio.png: [antes](capturas/2026-10-15/antes/1440x900-claro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-claro-vazio.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-erro.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-processando.png)
- 1440x900-escuro-vazio.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1440x900-escuro-vazio.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-erro.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-processando.png)
- 1920x1080-claro-vazio.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-claro-vazio.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-erro.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-processando.png)
- 1920x1080-escuro-vazio.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/1920x1080-escuro-vazio.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-configuracoes.png)
- 800x600-claro-erro.png: [antes](capturas/2026-10-15/antes/800x600-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-erro.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-processando.png)
- 800x600-claro-vazio.png: [antes](capturas/2026-10-15/antes/800x600-claro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-claro-vazio.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-configuracoes.png)
- 800x600-escuro-erro.png: [antes](capturas/2026-10-15/antes/800x600-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-erro.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-processando.png)
- 800x600-escuro-vazio.png: [antes](capturas/2026-10-15/antes/800x600-escuro-vazio.png) · [depois](capturas/2026-10-15/depois-F-VI-02%2BF-VI-03/800x600-escuro-vazio.png)

</details>

### F-RS-02 — Limite máximo do divisor (480px) ignora a largura da janela; na largura mínima a fila fica espremida e a barra de ferramentas é cortada

- **Arquivo:** `frontend/src/App.svelte:164`
- **Lente:** RS (relacionado: F-AN-02)
- **Evidência:** App.svelte:164 `if (newWidth >= 280 && newWidth <= 480)` e :179 `Math.min(480, sidebarWidth + 10)` usam um teto fixo, enquanto main.go:31 permite `MinWidth: 800`. Com 800px e o painel no máximo sobram cerca de 320px para a fila: 800x600-escuro-processando-divisor-maximo.png mostra "Fila de Processamento" em duas linhas e o botão "Limpar Fila" cortado (`.list-card { overflow: hidden }`, ProcessList.svelte:244). O `<aside>` (271-273) não tem `flex-shrink: 0`, então o flexbox encolhe o painel abaixo de `sidebarWidth`, e `aria-valuenow` (333) informa uma largura que não é a renderizada.
- **Proposta:** Teto dinâmico, `max = Math.min(480, window.innerWidth - LARGURA_MIN_FILA)` com LARGURA_MIN_FILA perto de 460px, reaplicando o clamp no `resize` da janela (`<svelte:window bind:innerWidth>`). `flex-shrink: 0` no `.sidebar` e `min-width: 0` no `.content-area`.
- **Custo:** P
- **Ganho esperado:** Na janela mínima (800x600) a fila continua usável, com todos os botões visíveis, e o estado do slider reflete a largura real.
- **Veredito:** rebaixado (de alta para média)
- **Justificativa do cético:** Teto fixo e ausência de flex-shrink confirmados, e na captura o painel fica em cerca de 455px. Mas o problema só aparece com o divisor no máximo e a janela mínima, e nada é perdido.

Status: feito

- **Commit:** `969c860b74a7286a851debf1e5e1caacdb888fdd`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/)

<details><summary>Pares antes/depois (48)</summary>

- 1024x768-claro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-claro-erro.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-claro-processando.png)
- 1024x768-escuro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-escuro-erro.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1024x768-escuro-processando.png)
- 1440x900-claro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-claro-erro.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-claro-processando.png)
- 1440x900-escuro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-escuro-erro.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1440x900-escuro-processando.png)
- 1920x1080-claro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-claro-erro.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-claro-processando.png)
- 1920x1080-escuro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-escuro-erro.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/1920x1080-escuro-processando.png)
- 800x600-claro-erro.png: [antes](capturas/2026-10-15/antes/800x600-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-claro-erro.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-claro-processando.png)
- 800x600-escuro-erro.png: [antes](capturas/2026-10-15/antes/800x600-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-escuro-erro.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-RS-01%2BF-RS-02/800x600-escuro-processando.png)

</details>

### F-AN-02 — Arraste do divisor: a transição de largura atrasa o cursor e posições fora da faixa são descartadas em vez de presas no limite

- **Arquivo:** `frontend/src/App.svelte:559`
- **Lentes:** AN, RS, DF (fundidos: "A transição de width na sidebar faz o arraste ficar atrasado" (AN), "Transição de largura do painel continua ativa durante o arrasto" (RS), "Arrastar o divisor descarta posições fora da faixa" (RS, App.svelte:163) e "Arrastar o divisor sofre com a transição de largura e não usa requestAnimationFrame" (DF))
- **Evidência:** `.sidebar { transition: width 0.15s ease-in-out, ... }` (559) vale o tempo todo. Durante o arrasto, `handleMouseMove` (158-167) muda `sidebarWidth` a cada mousemove pelo `style="width: ..."` (273), e cada mudança reinicia uma interpolação de 150ms; o divisor fica atrás do cursor. A classe `resizing` só vai para o `.resize-handle` (337), nunca para o `<aside>`, e o cursor `col-resize` só existe sobre a alça de 4px (627-633), voltando ao padrão quando o ponteiro sai dela. Além disso, `if (newWidth >= 280 && newWidth <= 480)` (163-166) descarta o evento fora da faixa: em 800x600-claro-processando-divisor-minimo.png o divisor parou perto de 296px, e não em 280.
- **Proposta:** `class:resizing={isResizing}` também no `<aside>` com `.sidebar.resizing { transition: none; }` e, durante o arrasto, `cursor: col-resize; user-select: none` no `document.documentElement`. A transição de largura fica só para recolher e expandir, com os tokens de F-AN-03. Trocar o `if` por clamp, `Math.min(max, Math.max(280, e.clientX))` (com o teto dinâmico de F-RS-02), aplicado em `requestAnimationFrame`. Usar Pointer Events com `setPointerCapture` e encerrar o arrasto também em `blur`/`pointercancel`. Área de captura da alça de 8px, mantendo 1-2px visíveis.
- **Custo:** P
- **Ganho esperado:** O divisor acompanha o cursor 1:1, encosta exatamente nos limites e o cursor de redimensionamento não pisca, com um único layout por quadro.
- **Veredito:** rebaixado (AN, de alta para média); mantido (RS, transição); rebaixado (RS, clamp); rebaixado (DF)
- **Justificativa do cético:** A transição sem exceção no arrasto, o cursor restrito à alça e o descarte fora da faixa foram confirmados. É incômodo visual num gesto raro, e o efeito do clamp quase some com mouse real, que dispara muito mais eventos que os 12 passos sintéticos da captura.

Status: feito

- **Commit:** `833373e047fe117da6c00de3e7fa3c601db12954`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-AN-02+F-AN-03+F-AN-04/`](capturas/2026-10-15/depois-F-AN-02%2BF-AN-03%2BF-AN-04/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-AN-02%2BF-AN-03%2BF-AN-04/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### F-AN-03 — Movimento sem tokens, durações e curvas soltas, animações infinitas e nenhuma regra de prefers-reduced-motion

- **Arquivo:** `frontend/src/components/ProcessList.svelte:122`
- **Lentes:** AN, VI, AC, DF (fundidos: "Nenhuma regra de prefers-reduced-motion" (AN), "Durações e curvas soltas e inconsistentes" (AN, Settings.svelte:347), "Movimento sem tokens..." (VI, App.svelte:524), "Nenhum respeito a prefers-reduced-motion" (AC, App.svelte:524) e "Animações infinitas pintam box-shadow" (DF, App.svelte:524))
- **Evidência:** A busca por `reduced-motion` e `@media` em frontend/src não acha nada. Animações infinitas: App.svelte:524 `animation: pulse 1.5s infinite;` (os keyframes, 518-540, animam `box-shadow`, o que repinta a cada quadro durante todo o processamento), ProcessList.svelte:593 e Settings.svelte:415 `spin 0.8s linear infinite`. Deslocamentos: Settings.svelte:381 `slideUp 0.3s`, App.svelte:413 `zoomIn 0.15s`, ProcessList.svelte:407 `translateY(-1px)`, e o `transition:fly={{ y: 8, duration: 200 }}` (ProcessList.svelte:122), que é JS e não obedece a CSS. Durações e curvas sem tokens: switch com `transition: .3s` sem propriedade (Settings.svelte:347 e 359), `cubic-bezier(0.4, 0, 0.2, 1)` (ProcessList.svelte:399), `ease-in-out` (App.svelte:559), 0.3s nas trocas de tema (style.css:49; App.svelte:440, 718, 787; ProcessList.svelte:246), 0.1s no botão (Settings.svelte:292). O DESIGN.md não tem seção de movimento, e o spinner é CSS, não o ícone `Loader` do DESIGN §4.
- **Proposta:** Acrescentar ao DESIGN.md uma seção "Movimento" e ao style.css os tokens `--dur-fast: 150ms` (hover, toggle, cor), `--dur-base: 220ms` (entrada, saída, sidebar) e `--ease-out: cubic-bezier(.2,.8,.2,1)`. Trocar todas as durações literais pelos tokens, com propriedades explícitas em vez de `transition: .3s`. Bloco `@media (prefers-reduced-motion: reduce)` que zera os tokens e troca deslocamento e escala por troca direta ou fade curto. O pulso vira um ponto estático (ou anima só `opacity`/`transform` num pseudo-elemento), e o spinner fica parado ou mais lento. Para o `fly`, ler `matchMedia('(prefers-reduced-motion: reduce)')` (ou `prefersReducedMotion` de `svelte/motion` no Svelte 5) e passar `duration: 0`.
- **Custo:** P
- **Ganho esperado:** Movimento consistente e discreto com um único ponto de ajuste, sem repintura contínua durante a fila, e a preferência de acessibilidade do sistema respeitada (WCAG 2.3.3).
- **Veredito:** rebaixado (AN, de alta para média); rebaixado (VI); mantido (AN, durações); mantido (AC); mantido (DF)
- **Justificativa do cético:** Confirmados a ausência de `@media`/`reduced-motion`, as animações infinitas e as durações soltas. É ajuste de acessibilidade e consistência num app desktop interno, não quebra nada, e a repintura de um ponto de 5px é quase desprezível.

Status: feito

- **Commit:** `833373e047fe117da6c00de3e7fa3c601db12954`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-AN-02+F-AN-03+F-AN-04/`](capturas/2026-10-15/depois-F-AN-02%2BF-AN-03%2BF-AN-04/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-AN-02%2BF-AN-03%2BF-AN-04/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### F-VI-04 — Paleta de tokens diverge do DESIGN.md (cinzas e acentos estilo Tailwind no lugar das cores definidas)

- **Arquivo:** `frontend/src/assets/css/style.css:3`
- **Lente:** VI (relacionado: F-AC-01, F-AC-02, F-VI-09)
- **Evidência:** style.css:3-14 define `--bg-main: #0B0F19; --surface: #111827; --border: #1F2937; --accent: #3B82F6; --success: #10B981; --error: #EF4444`. O DESIGN.md §2.2 manda `#121212 / #1E1E1E / #2C2C2C`. No claro (20-31), `--bg-main: #F3F4F6` e `--accent: #2563EB` também diferem do DESIGN (`#F8F9FA` e o "Azul Jurídico" `#0056b3`). A borda escura (#1F2937) é igual a `--surface-hover`.
- **Proposta:** Reescrever os tokens a partir do DESIGN.md: fundo, superfície, textos, borda e acentos (#0056b3 para ação, #28A745, #DC3545, #007BFF para processamento). Acrescentar no DESIGN.md os tokens que faltam (`--surface-hover`, `--warning`, `--focus-ring`) e os de movimento (F-AN-03). Se a decisão for evoluir a paleta, atualizar o DESIGN.md junto. Validar os pares de contraste de F-AC-01 e F-AC-02 na mesma passada.
- **Custo:** P
- **Ganho esperado:** Identidade sóbria e coerente com a especificação, e bordas visíveis no tema escuro.
- **Veredito:** mantido
- **Justificativa do cético:** A divergência da especificação é real (style.css:3-31 contra DESIGN.md:16-30). A afirmação de que os cards "quase somem" é exagerada, porque na captura as bordas aparecem.

Status: feito

- **Commit:** `517ec8a92d48d349f958c450ee12abbbffabc49d`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-VI-04+F-VI-05/`](capturas/2026-10-15/depois-F-VI-04%2BF-VI-05/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-VI-04%2BF-VI-05/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### B-EF-03 — Pré-contagem serial grava cada PDF em disco e o reabre, e essa reabertura domina o tempo sem OCR

- **Arquivo:** `internal/etl/pipeline.go:113`
- **Lente:** EF
- **Evidência:** No laço de pré-contagem (pipeline.go:91-148), que é serial, cada PDF passa por `os.CreateTemp`, `io.Copy` e `Close`, e depois por um `os.Open(tPath)` do mesmo arquivo (linha 113) só para chamar `pdf.NewReader` e `NumPage`. Depois, ParsePages abre o arquivo de novo (pdf_parser.go:46) e refaz o NewReader e a leitura do xref. Com `go test ./scripts/bench -bench Sintetico -cpuprofile`, ocr=none e um ZIP de 50 entradas (cerca de 45 PDFs), o closure da pré-contagem somou 7,81 s de 7,89 s, e `os.OpenFile`/`syscall.createFile` sozinhos somaram 7,08 s. O ParsePages inteiro ficou em 0,56 s. Num micro-teste com nativo.pdf (40 repetições), fechar e reabrir o arquivo recém-gravado custou em média 63,7 ms, contra 0,6 ms reaproveitando o handle do CreateTemp, o que é compatível com o antivírus varrendo o arquivo no primeiro open. O benchmark ficou entre 2,6 e 8,5 s/op (-count=6), quase tudo espera de I/O.
- **Proposta:** Ler cada entrada do ZIP para a memória (com teto de tamanho e disco acima dele) e usar o mesmo `bytes.Reader` para NumPage e para ParsePages, guardando o `*pdf.Reader` em vez de abrir o arquivo duas vezes. O PDF só vai para arquivo temporário quando alguma página precisar de OCR (o pdftoppm exige caminho), gravado uma única vez. Se o disco continuar no caminho, ao menos usar o próprio `*os.File` do CreateTemp com ReadAt em vez de Close seguido de Open, e paralelizar a pré-contagem.
- **Custo:** M
- **Ganho esperado:** Com ocr=none no misto.zip, cerca de −90% de tempo de parede (de ≈3 s/op para ≈0,3 s/op). Também −1 escrita em disco e −1 parse de xref por PDF que não vai para o OCR.
- **Veredito:** mantido
- **Justificativa do cético:** pipeline.go:100-131 confirma a pré-contagem serial com CreateTemp/io.Copy/Close seguido de `os.Open(tPath)`, e ParsePages abre o arquivo outra vez. Reproduzido num micro-teste fora do repo (20 repetições): fechar e reabrir custou 59,5 ms em média, contra 2,9 ms reaproveitando o handle.

Status: feito

- **Commit:** `22fd61d26496b1e70590576c29417accf0fdabd0`
- **Benchstat:**

```
goos: windows
goarch: amd64
pkg: eprocia/scripts/bench
cpu: Intel(R) Core(TM) i7-10750H CPU @ 2.60GHz
                           │     antes      │                depois                │
                           │     sec/op     │    sec/op     vs base                │
Sintetico/ocr=none-12        1390.18m ± 10%   51.73m ± 11%  -96.28% (p=0.000 n=10)
ParsePDF/nativo-12             1.169m ±  4%   1.170m ±  5%        ~ (p=0.796 n=10)
ParsePDF/escaneado-12          369.3µ ±  4%   353.7µ ±  6%        ~ (p=0.165 n=10)
ParsePDF/vetorial-12           14.67m ±  9%   14.26m ±  8%        ~ (p=0.315 n=10)
ParsePDF/contents-array-12     289.7µ ±  9%   274.6µ ± 13%        ~ (p=0.089 n=10)
Sintetico/ocr=fast-12           3.648 ±  4%    2.405 ±  4%  -34.07% (p=0.000 n=10)
geomean                        14.50m         7.657m        -47.20%

                      │      antes      │                 depois                  │
                      │ gc-pausa-sec/op │ gc-pausa-sec/op  vs base                │
Sintetico/ocr=none-12      6.329m ± 42%     3.710m ±  16%  -41.38% (p=0.000 n=10)
Sintetico/ocr=fast-12      5.635m ± 21%     4.077m ± 102%        ~ (p=0.190 n=10)
geomean                    5.972m           3.889m         -34.87%

                      │    antes     │                depois                │
                      │  pico-mem-B  │  pico-mem-B    vs base               │
Sintetico/ocr=none-12   22.86Mi ± 0%   23.11Mi ±  0%  +1.09% (p=0.000 n=10)
Sintetico/ocr=fast-12   18.86Mi ± 0%   18.86Mi ± 21%       ~ (p=0.387 n=10)
geomean                 20.77Mi        20.88Mi        +0.55%

                      │    antes     │               depois                │
                      │   procs/op   │  procs/op   vs base                 │
Sintetico/ocr=none-12   0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=10) ¹
Sintetico/ocr=fast-12   10.00 ± 0%     10.00 ± 0%       ~ (p=1.000 n=10) ¹
geomean                            ²               +0.00%                ²
¹ all samples are equal
² summaries must be >0 to compute geomean

                           │    antes     │               depois                │
                           │     B/op     │     B/op      vs base               │
Sintetico/ocr=none-12        17.47Mi ± 1%   16.51Mi ± 0%  -5.52% (p=0.000 n=10)
ParsePDF/nativo-12           262.9Ki ± 0%   263.0Ki ± 0%  +0.03% (p=0.004 n=10)
ParsePDF/escaneado-12        54.42Ki ± 0%   54.45Ki ± 0%  +0.06% (p=0.001 n=10)
ParsePDF/vetorial-12         1.845Mi ± 0%   1.845Mi ± 0%       ~ (p=0.631 n=10)
ParsePDF/contents-array-12   35.18Ki ± 0%   35.20Ki ± 0%  +0.05% (p=0.000 n=10)
Sintetico/ocr=fast-12        19.26Mi ± 1%   18.15Mi ± 1%  -5.76% (p=0.000 n=10)
geomean                      833.5Ki        817.8Ki       -1.89%

                           │    antes    │               depois               │
                           │  allocs/op  │  allocs/op   vs base               │
Sintetico/ocr=none-12        294.1k ± 0%   291.5k ± 0%  -0.89% (p=0.000 n=10)
ParsePDF/nativo-12           2.306k ± 0%   2.307k ± 0%  +0.04% (p=0.000 n=10)
ParsePDF/escaneado-12         329.0 ± 0%    330.0 ± 0%  +0.30% (p=0.000 n=10)
ParsePDF/vetorial-12         104.6k ± 0%   104.6k ± 0%  +0.00% (p=0.007 n=10)
ParsePDF/contents-array-12    234.0 ± 0%    235.0 ± 0%  +0.43% (p=0.000 n=10)
Sintetico/ocr=fast-12        299.1k ± 0%   296.6k ± 0%  -0.84% (p=0.000 n=10)
geomean                      10.85k        10.84k       -0.16%
```

### B-TE-04 — Pipeline.ProcessZip só roda em benchmark que checa apenas err == nil; os caminhos de erro e o arquivo de saída nunca são verificados

- **Arquivo:** `internal/etl/pipeline.go:49`
- **Lente:** TE
- **Evidência:** Cobertura de ProcessZip, NewPipeline, SetContext e getContext em internal/etl: 0,0%. A única chamada fica em scripts/bench/bench_test.go:128, dentro de BenchmarkSintetico (`if _, _, _, err := pipe.ProcessZip(...); err != nil { b.Fatalf }`), que não roda em `go test ./...` sem -bench e nunca lê o .txt gerado. Ficam sem teste: ZIP sem PDF/HTML (pipeline.go:67-76), ZIP ilegível (55-64), placeholder `[ERRO ao analisar PDF: ...]` para PDF corrompido (335-341), cancelamento devolvendo erro sem gravar saída (326-333, 407-410), falha de gravação (417-426), limpeza dos temporários `eprocia_pdf_*` (394-400) e as contagens docs/pages devolvidas.
- **Proposta:** Criar pipeline_test.go com as fixtures de scripts/bench/fixtures, ou ZIPs montados em memória como em TestReadZipFiles, com OcrMode `none`. Verificar o conteúdo e a ordem das tags `<evento>`/`<documento>` no .txt, docsCount/pagesCount, o erro para ZIP só com .txt, o placeholder de erro para um PDF truncado sem abortar os demais, ctx já cancelado devolvendo `context.Canceled` sem criar o .txt e nenhum `eprocia_pdf_*.pdf` sobrando (definindo TMP/TEMP para `t.TempDir()`). Esse teste é a rede de segurança para B-EF-03.
- **Custo:** M
- **Ganho esperado:** ProcessZip de 0% para cerca de 70% de cobertura; o formato de saída (PRD §6) e a ausência de vazamento de temporários passam a ter teste.
- **Veredito:** mantido
- **Justificativa do cético:** NewPipeline, SetContext, getContext e ProcessZip estão em 0,0%, e a única referência em teste é um benchmark.

Status: feito

- **Commit:** `056b96adae8238e313bc7f70630fe326583d338e`
- **Benchstat:**

```
Pulado: go list -deps ./scripts/bench lista só eprocia/scripts/bench como pacote do módulo, sem internal/etl. A tarefa B-TE-04 alterou apenas internal/etl/pipeline_saida_test.go, arquivo de teste que não entra no benchmark. Sem tabela.
```

### B-TE-05 — Fila (Queue) sem nenhum teste: cancelamento, pausa, remoção e encadeamento nunca são exercitados

- **Arquivo:** `internal/etl/queue.go:182`
- **Lente:** TE
- **Evidência:** `go test -coverprofile` em internal/etl: NewQueue, AddFiles, Pause, Resume, CancelItem, RemoveItem, Clear, startProcessing, processItem e broadcast estão em 0,0% (queue.go:52-367), e nenhum *_test.go referencia NewQueue nem CancelItem. Sem teste ficam o status `cancelled` quando `ctx.Err() == context.Canceled` (339-342), o cancelamento de item `pending` (195-197), o RemoveItem de item em processamento seguido de startProcessing (211-223), o recover de panic que zera isWorking (271-283) e a deduplicação de caminho (90-95).
- **Proposta:** queue_test.go com um processador falso (interface `processor` com ProcessZip, ou uma func injetada) que bloqueia até o ctx ser cancelado. Casos: (a) CancelItem em `processing` termina em `cancelled` com Progress 0; (b) CancelItem em `pending` impede o processamento; (c) pendentes rodam do mais antigo para o mais novo, um por vez; (d) Pause impede o próximo e Resume retoma; (e) panic no processador vira `error` e a fila segue; (f) Shutdown cancela o item em curso. Rodar com -race (cobre também B-ES-08 se o teste definir contexto).
- **Custo:** M
- **Ganho esperado:** Cobertura de queue.go de 0% para mais de 80%; regressões no cancelamento e no encadeamento passam a ser detectadas em `go test`.
- **Veredito:** mantido
- **Justificativa do cético:** Todas as funções de queue.go estão em 0,0% no coverprofile, e a fila sequencial com cancelamento é o núcleo do app.

Status: feito

- **Commit:** `ce2706a6c12c42ac67cd3a22f5e5f578dc51fa87`
- **Benchstat:**

```
Benchmark pulado. go list -deps ./scripts/bench lista só eprocia/scripts/bench como pacote do módulo. A única alteração é internal/etl/queue_fila_test.go, um arquivo de teste que não faz parte de nenhuma dependência do benchmark. Nenhum worktree foi criado.
```

### B-ES-05 — Árvore /Pages cíclica trava reader.Page para sempre e prende a fila

- **Arquivo:** `internal/etl/pdf_parser.go:116`
- **Lente:** ES
- **Evidência:** `Reader.Page` (dslipak/pdf page.go:25-54) desce por `/Kids` com `continue Search` sem conjunto de visitados. Com `2 0 obj << /Type /Pages /Kids [2 0 R] /Count 1 >>`, NumPage() = 1 retorna e `reader.Page(1)` gira até o `timeout 10` matar o processo (exit 124). ParsePages só olha `ctx.Err()` entre páginas (linha 110), então cancelar não adianta; `wg.Wait()` em pipeline.go:392 nunca volta, `q.isWorking` fica true (queue.go:335 nunca é alcançado) e nenhum outro ZIP é processado até reiniciar o app.
- **Proposta:** Fazer a própria travessia da árvore de páginas uma vez por PDF (com visitados por objptr e limite de profundidade) e obter as páginas por índice, ou rodar `reader.Page` em goroutine com prazo e sair quando o ctx for cancelado. Em ambos os casos, marcar o documento como erro.
- **Custo:** M
- **Ganho esperado:** Um PDF malformado não congela mais a fila, e o cancelamento volta a funcionar.
- **Veredito:** rebaixado (de alta para média)
- **Justificativa do cético:** O laço sem visitados foi confirmado e prende `wg.Wait` e `isWorking`, mas uma árvore /Pages cíclica é rara em PDFs reais do Eproc.

Status: pendente

### B-RN-02 — ZIPs baixados com o app fechado nunca são processados

- **Arquivo:** `internal/watchdog/watchdog.go:122`
- **Lente:** RN (relacionado: B-RN-01)
- **Evidência:** O watchLoop só reage a eventos Create/Write/Rename do fsnotify (watchdog.go:122). `App.startup` só chama `a.watcher.Start(cfg.MonitoredPath)` (app.go:52-54) e não varre a pasta. Um ZIP do Eproc que chegou antes do app subir fica sem .txt até ser arrastado manualmente.
- **Proposta:** Em `Watcher.Start`, depois do `Add`, listar com `os.ReadDir` os `.zip` com `ModTime` nas últimas 24 h (ou desde um `last_seen` no Config) que não têm `<nome>.txt` mais novo ao lado, e passar cada um a `onFileAdded`. Função pura `pendingZips(dir, since)` com teste em `t.TempDir()`. A janela de tempo impede reprocessar a pasta Downloads inteira.
- **Custo:** P
- **Ganho esperado:** Downloads feitos antes do app iniciar passam a ser processados, e o RF02 vale também no boot.
- **Veredito:** mantido
- **Justificativa do cético:** O watchLoop só reage a eventos do fsnotify e o startup não varre a pasta.

Status: feito

- **Commit:** `8d6b54add404c33597ada6d2f8b1d6ac630657ea`

### B-RN-03 — A higienização agressiva não remove cabeçalhos e rodapés repetidos, como a interface diz

- **Arquivo:** `internal/etl/formatter.go:13`
- **Lente:** RN
- **Evidência:** O modo agressivo só descarta linhas iguais a `pageMarkerRegex` ("Página N de M") ou que começam com os prefixos fixos de `signaturePrefixes` (formatter.go:13-38, :82). Settings.svelte:151 promete "Agressivo (Básico + remove rodapés e cabeçalhos repetitivos)", e o PRD RNF02 pede higienizar "rodapés repetitivos". `CleanText` recebe uma página por vez (formatter.go:212), então não tem como saber o que se repete entre as páginas.
- **Proposta:** Antes do laço de páginas de cada documento em `GenerateConsolidatedOutput`, contar a frequência das primeiras e últimas k linhas (k = 3, com TrimSpace e dígitos normalizados para `#`). No nível aggressive, descartar as que aparecem em pelo menos 60% das páginas, quando o documento tem 3 páginas ou mais. Função pura `repeatedEdgeLines(pages []string) map[string]bool` com teste de tabela.
- **Custo:** M
- **Ganho esperado:** Estimativa (não medida em dados reais) de 5% a 15% menos tokens em documentos longos com cabeçalho e rodapé institucional, cumprindo o RNF02 e a promessa da interface.
- **Veredito:** mantido
- **Justificativa do cético:** CleanText recebe uma página por vez e só remove os padrões fixos, o que contraria o rótulo da interface e o RNF02. O ganho de 5% a 15% é só estimativa.

Status: feito

- **Commit:** `cb54174a65dab4ef41de190af42883aa15acd04f`

### F-AC-05 — Mudanças de estado não são anunciadas: toast de configuração, progresso e status da fila sem aria-live nem progressbar

- **Arquivo:** `frontend/src/components/Settings.svelte:173`
- **Lente:** AC (relacionado: F-AN-01)
- **Evidência:** `{#if statusMessage}<div class="toast-status {statusType}">` (173-177) não tem `role="status"`/`aria-live` e some após `setTimeout(..., 3000)` (67), inclusive quando é um erro ao salvar. Em ProcessList.svelte:169-171 a barra `<div class="progress-bar" style="width: ...">` não tem `role="progressbar"`/`aria-valuenow`. Os ícones de status (126-140) são SVGs sem rótulo, e um item concluído não tem texto "Concluído" (148-151).
- **Proposta:** `role="status" aria-live="polite"` no toast de sucesso e `role="alert"` no de erro; erros não somem sozinhos e ganham um botão de fechar. `role="progressbar" aria-valuemin=0 aria-valuemax=100 aria-valuenow` na barra. Ícones de status com `aria-hidden="true"` e texto `.sr-only` ("Concluído", "Em processamento", "Erro").
- **Custo:** M
- **Ganho esperado:** Leitores de tela informam o resultado do salvamento e o andamento da fila, e erros deixam de sumir antes de serem lidos (WCAG 4.1.3 e 2.2.1).
- **Veredito:** mantido
- **Justificativa do cético:** O toast não tem role nem aria-live e some em 3 s mesmo em erro, e a barra não tem role=progressbar.

Status: feito

- **Commit:** `46ad443f6f877e1867dd55d1971e0856db15b00e`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/)

<details><summary>Pares antes/depois (56)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-configuracoes.png)
- 1024x768-claro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-erro.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-processando.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-erro.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-processando.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-configuracoes.png)
- 1440x900-claro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-erro.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-processando.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-erro.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-processando.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-erro.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-processando.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-erro.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-processando.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-configuracoes.png)
- 800x600-claro-erro.png: [antes](capturas/2026-10-15/antes/800x600-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-erro.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-processando.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-configuracoes.png)
- 800x600-escuro-erro.png: [antes](capturas/2026-10-15/antes/800x600-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-erro.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-processando.png)

</details>

### F-DF-01 — Cada página processada substitui a fila inteira no frontend, sem limite de frequência

- **Arquivo:** `frontend/src/App.svelte:65`
- **Lente:** DF (relacionado: B-EF-01, F-DF-03)
- **Evidência:** App.svelte:65-67 `EventsOn('queue:update', (data) => { items = data || []; })`. No backend, onProgress chama `q.broadcast()` a cada página (queue.go:311), e broadcast manda a lista inteira em `queue:update` e depois `queue:status` (367-374), a partir de até NumCPU()-1 goroutines (pipeline.go:318-321). Páginas nativas levam milissegundos, então chegam centenas de eventos por segundo, cada um com um array novo: `$: filteredItems` recalcula, o `{#each}` refaz o diff de todas as linhas e o JSON é desserializado de novo. O histórico nunca é podado.
- **Proposta:** No frontend, aplicar no máximo uma atualização por quadro (guardar o último payload e aplicar com `requestAnimationFrame`, ou throttle de cerca de 100 ms). Do lado do backend, B-EF-01 já propõe progresso leve por item e `queue:update` só em mudanças de status. Com Svelte 5, atualizar só o item afetado.
- **Custo:** M
- **Ganho esperado:** De centenas de re-renderizações e desserializações por segundo para cerca de 10 a 60, com UI fluida e menos CPU enquanto o OCR satura a máquina.
- **Veredito:** rebaixado (de alta para média)
- **Justificativa do cético:** O fluxo foi confirmado, mas o payload são dezenas de itens pequenos e ninguém mediu travamento na UI.

Status: feito

- **Commit:** `8c35250c530d50981787c551e361b0058828a0c2`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/)

<details><summary>Pares antes/depois (48)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-claro-configuracoes.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-claro-processando.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-escuro-processando.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-claro-configuracoes.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-claro-processando.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-escuro-processando.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-claro-processando.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-escuro-processando.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-claro-configuracoes.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-claro-processando.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-escuro-configuracoes.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-escuro-processando.png)

</details>

### F-VI-05 — Glassmorphism, gradientes radiais, sombras grandes, hover com deslocamento e raios de 8 a 12px contrariam a "sobriedade jurídica"

- **Arquivo:** `frontend/src/components/ProcessList.svelte:235`
- **Lentes:** VI, DF, AN (fundidos: "backdrop-filter: blur em cada linha da fila" (DF, ProcessList.svelte:396) e "Hover das linhas com translateY e sombra grande" (AN, ProcessList.svelte:407))
- **Evidência:** ProcessList.svelte:236-246 tem `background-color: rgba(17, 24, 39, 0.45); backdrop-filter: blur(12px); border-radius: 12px; box-shadow: 0 8px 32px 0 rgba(0,0,0,0.2)`. As linhas (386-411) têm `border-radius: 8px; backdrop-filter: blur(8px)`, `box-shadow: 0 4px 12px` e, no hover, `transform: translateY(-1px)` com `box-shadow: 0 8px 24px` e transição de 0.2s em transform, box-shadow e padding-bottom. A barra de título, a barra lateral, o rodapé e o card manual também usam blur (App.svelte:431, 554, 776, 709). App.svelte:376-377 usa `radial-gradient(...)`, o manual-card (707-718) tem raio de 10px, e a barra de progresso (ProcessList.svelte:531-533) usa gradiente com brilho. O DESIGN.md §6 fixa raio de 4px, "bordas sutis e sombras leves".
- **Proposta:** Superfícies opacas (`var(--surface)`, `var(--surface-hover)` no hover) com borda de 1px `var(--border)`, raio de 4px (até 6px no contêiner da lista), uma sombra leve só na camada da lista e nenhum blur ou gradiente de fundo (se o blur ficar, só na barra de título). Barra de progresso de cor sólida (`--processing`), sem brilho. Hover da linha só muda fundo e borda em `--dur-fast`, sem deslocamento.
- **Custo:** M
- **Ganho esperado:** Visual atual e sóbrio, lista estável ao passar o mouse e menos camadas de composição e repintura no WebView2.
- **Veredito:** mantido (VI); rebaixado (DF); mantido (AN)
- **Justificativa do cético:** Os valores foram confirmados em ProcessList.svelte e App.svelte, contra o raio de 4px do DESIGN.md:78. O custo de GPU do blur por linha é especulativo e não foi medido; a parte estética se sustenta.

Status: feito

- **Commit:** `517ec8a92d48d349f958c450ee12abbbffabc49d`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-VI-04+F-VI-05/`](capturas/2026-10-15/depois-F-VI-04%2BF-VI-05/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-VI-04%2BF-VI-05/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### F-VI-06 — Lista pouco densa: linhas com 52px de altura mínima e 8px de espaço contra os 40px e 4px do DESIGN

- **Arquivo:** `frontend/src/components/ProcessList.svelte:389`
- **Lente:** VI
- **Evidência:** ProcessList.svelte:389 `min-height: 52px;`, 382-383 `gap: 8px; padding: 10px;`, 392 `padding: 8px 12px`, e a linha em processamento ganha `padding-bottom: 14px` (413-415). O DESIGN.md §5.1 pede "altura máxima de 40px" e o §6, gap de 4px. Em 1440x900-escuro-processando.png cada item ocupa cerca de 52px mais o espaço, e a linha em processamento passa de 60px.
- **Proposta:** Linha de uma faixa com 36 a 40px (ícone, nome, horário em 11px tabular, meta e ações), mensagem e contadores à direita na mesma linha, barra de progresso de 2px colada à borda inferior, gap de 4px e padding de 8px no contêiner. Avaliar divisores de 1px em vez de cards separados.
- **Custo:** M
- **Ganho esperado:** Cerca de 40% mais itens visíveis sem rolagem.
- **Veredito:** mantido
- **Justificativa do cético:** Valores confirmados em ProcessList.svelte:382-415 contra DESIGN.md:60 e :77, e a captura mostra as linhas com cerca de 52px.

Status: feito

- **Commit:** `406dacab40c19397c71d58e3d88a651b5fab8fe1`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-VI-06+F-VI-10/`](capturas/2026-10-15/depois-F-VI-06%2BF-VI-10/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-VI-06%2BF-VI-10/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### B-ES-06 — Pré-cálculo de páginas ignora cancelamento

- **Arquivo:** `internal/etl/pipeline.go:91`
- **Lente:** ES (relacionado: B-EF-03, B-ES-07)
- **Evidência:** O laço de pré-cálculo (91-148) extrai cada PDF para disco com io.Copy e abre um pdf.Reader sem olhar `ctx.Err()`. Cancelar ou remover um ZIP grande nessa fase só tem efeito depois que todos os PDFs forem copiados para %TEMP%. O checkpoint seguinte é no goroutine por documento (linha 247).
- **Proposta:** Checar `ctx.Err()` no início de cada iteração e, quando cancelado, remover os temporários já criados e retornar `ctx.Err()`.
- **Custo:** P
- **Ganho esperado:** O cancelamento responde em uma iteração.
- **Veredito:** mantido
- **Justificativa do cético:** O laço não consulta o ctx em nenhum ponto. Ganho modesto, mas real em ZIPs grandes.

Status: pendente

### B-ES-07 — Panic no pré-cálculo vaza PDFs temporários e o contexto do item

- **Arquivo:** `internal/etl/pipeline.go:131`
- **Lente:** ES
- **Evidência:** O pré-cálculo (91-148) chama `pdf.NewReader` e `reader.NumPage()` (que pode dar panic em resolve) sem recover e sem defer de limpeza. Num panic, tPath (linha 104) e os temporários de docTempFiles ficam em %TEMP%, porque a limpeza das linhas 396-400 nunca roda. O recover de processItem (queue.go:271-283) só zera `item.cancel = nil` sem chamar `cancel()`, então o `context.WithCancel` da linha 289 vaza.
- **Proposta:** Pôr a limpeza de docTempFiles num defer logo após criar o mapa, com recover no closure do pré-cálculo (que remove tPath e usa o fallback de 1 página). Em processItem, `defer cancel()` logo após o WithCancel.
- **Custo:** P
- **Ganho esperado:** Nenhum `eprocia_pdf_*.pdf` órfão em %TEMP% após falha, e o contexto do item liberado.
- **Veredito:** rebaixado
- **Justificativa do cético:** O pré-cálculo roda no mesmo goroutine de processItem, então o panic é pego e o app não cai. Sobra o vazamento de temporários e do contexto filho, de efeito pequeno.

Status: pendente

### B-ES-08 — Data race em Queue.broadcast e SetContext

- **Arquivo:** `internal/etl/queue.go:372`
- **Lente:** ES
- **Evidência:** broadcast lê `q.isPaused` e `q.isWorking` (372-373) e `q.wailsCtx` (368) sem `q.mu`, enquanto esses campos são escritos sob lock em Pause/Resume/startProcessing/processItem, e broadcast é chamado de vários goroutines (onProgress de cada documento, pipeline.go:216). SetContext (71) escreve wailsCtx sem lock. `go test -race ./internal/...` passa porque os testes não definem wailsCtx e broadcast vira no-op.
- **Proposta:** Ler isPaused, isWorking e wailsCtx sob `q.mu.RLock()` dentro de broadcast (copiando para locais antes do EventsEmit) e proteger SetContext com `q.mu.Lock()`.
- **Custo:** P
- **Ganho esperado:** Remove uma corrida real, detectável com -race num teste que defina contexto.
- **Veredito:** mantido
- **Justificativa do cético:** As leituras sem lock e a escrita sem lock em SetContext foram confirmadas.

Status: pendente

### B-ES-09 — handleNewFile lê w.done sem lock e não entra no WaitGroup do Stop

- **Arquivo:** `internal/watchdog/watchdog.go:152`
- **Lente:** ES
- **Evidência:** handleNewFile (disparado em watchdog.go:125) lê `w.done` a cada iteração (152), enquanto Start o reatribui sob `w.mu` (65): corrida de dados. Esses goroutines não fazem `wg.Add`, então stopLocked (94-96) não espera por eles. Depois de Stop + Start em outra pasta, um goroutine antigo que leia o novo `w.done` segue vivo e chama onFileAdded com um ZIP da pasta antiga.
- **Proposta:** Capturar `done := w.done` em watchLoop e passá-lo como argumento a handleNewFile, com `wg.Add(1)`/`Done` para esses goroutines.
- **Custo:** P
- **Ganho esperado:** Stop passa a ser definitivo, sem disparos tardios da pasta antiga.
- **Veredito:** mantido
- **Justificativa do cético:** A leitura de `w.done` a cada iteração e a ausência no wg foram confirmadas.

Status: pendente

### B-ES-10 — config.json gravado sem atomicidade

- **Arquivo:** `internal/config/config.go:126`
- **Lente:** ES (relacionado: B-ES-03, B-TE-03)
- **Evidência:** SaveToFile usa `os.WriteFile` direto no arquivo final (126). Uma queda no meio da escrita deixa JSON truncado; na próxima execução LoadFromFile devolve o erro do Unmarshal (79-81) e app.startup o ignora (app.go:47-55), e o app sobe sem watchdog e sem aviso.
- **Proposta:** Gravar num arquivo temporário no mesmo diretório e chamar `os.Rename` por cima do config.json. Em Load, tratar JSON inválido como defaults com log (B-ES-03).
- **Custo:** P
- **Ganho esperado:** A configuração não se corrompe por interrupção da escrita.
- **Veredito:** mantido
- **Justificativa do cético:** Real e de baixa probabilidade.

Status: pendente

### B-ES-11 — Shutdown não espera a fila: tesseract/pdftoppm órfãos e temporários deixados

- **Arquivo:** `app.go:59`
- **Lente:** ES
- **Evidência:** `app.shutdown` chama `a.queue.Shutdown()`, que só faz `q.baseCancel()` (queue.go:65-67) e retorna na hora; em seguida wails.Run retorna e main termina (main.go:27-54). `exec.CommandContext` (tesseract.go:136/181) só mata o filho de forma assíncrona, e no Windows os filhos não morrem com o pai. Os defers que removem `eprocia_ocr_run*` (tesseract.go:128-131) e `eprocia_pdf_*.pdf` (pipeline.go:323/396) não chegam a rodar.
- **Proposta:** Guardar um `sync.WaitGroup` (ou canal done) na Queue para o processItem em curso e fazer Shutdown cancelar e esperar com prazo (por exemplo, 5 s). Opcionalmente, associar os filhos a um Job Object com KILL_ON_JOB_CLOSE.
- **Custo:** P
- **Ganho esperado:** Sair pelo tray não deixa tesseract.exe/pdftoppm.exe rodando nem lixo em %TEMP%.
- **Veredito:** rebaixado
- **Justificativa do cético:** Shutdown de fato não espera, mas os filhos órfãos terminam sozinhos em poucos segundos (uma página cada), e o resultado é só lixo pontual em %TEMP%.

Status: pendente

### B-RN-04 — Falta copiar a saída para a área de transferência

- **Arquivo:** `app.go:132`
- **Lente:** RN
- **Evidência:** O PRD §1 descreve o fluxo final como "copie e cole apenas um arquivo ou texto na IA". O App só expõe `OpenDirectory` (app.go:132-141), que abre o Explorer. Não há clipboard em frontend/src (fora de wailsjs). O wails v2.12.0 tem `runtime.ClipboardSetText(ctx, text)` (pkg/runtime/clipboard.go:10).
- **Proposta:** `func (a *App) CopyOutput(path string) error`, que valida que `path` termina em .txt e é o `OutputPath` de algum item da fila, lê o arquivo e chama `runtime.ClipboardSetText`. No frontend, um botão "Copiar" ao lado de "Abrir pasta" no item concluído. A validação fica testável sem Wails.
- **Custo:** P
- **Ganho esperado:** O fluxo principal do PRD cai de 4 ações (abrir pasta, abrir .txt, Ctrl+A, Ctrl+C) para 1 clique.
- **Veredito:** mantido
- **Justificativa do cético:** Só existe OpenDirectory e não há clipboard no frontend; melhoria real de baixo custo.

Status: feito

- **Commit:** `cfb5673f4f4769b767a5287f9e5be0b454eb10db`

### B-RN-05 — A saída não estima tokens, a métrica central do produto

- **Arquivo:** `internal/etl/formatter.go:120`
- **Lente:** RN
- **Evidência:** O cabeçalho `<contexto>` só traz o total de documentos e a data (formatter.go:118-121), e `process:success` só traz docs_count e pages_count (pipeline.go:441-446). O PRD §1 e o RNF02 põem a redução de tokens como objetivo, mas o usuário não sabe se o .txt cabe no contexto do LLM nem quanto a higienização economizou.
- **Proposta:** `estimatedTokens := utf8.RuneCountInString(outputText)/4` (heurística local, sem tokenizador nem rede), devolvido em ProcessZip, guardado em `QueueItem.EstimatedTokens` e mostrado no item concluído. Opcional: linha "Tokens estimados: N" no `<contexto>`, sem mudar a estrutura de tags do §6.
- **Custo:** P
- **Ganho esperado:** O usuário vê antes de colar se o processo cabe no limite do LLM e pode comparar os níveis de higienização.
- **Veredito:** mantido
- **Justificativa do cético:** Melhoria real e barata; a heurística de dividir por 4 é aproximada, o que a gravidade baixa já reflete.

Status: feito

- **Commit:** `708b6a8c86029d8e9ffe2c1047f8906a247fc1be`

### B-RN-06 — Nenhuma notificação quando o processamento termina com a janela oculta

- **Arquivo:** `internal/etl/queue.go:349`
- **Lente:** RN
- **Evidência:** Com HideWindowOnClose (main.go:43), o app costuma rodar só na bandeja. Ao terminar, processItem só emite `queue:update` (queue.go:349-364) e o pipeline só emite `process:success` (pipeline.go:440-447), que apenas o WebView consome. O wails v2.12.0 traz `runtime.SendNotification` (pkg/runtime/notifications.go:66), com toast local via go-toast (dependência indireta no go.mod), e não há chamada a ele no repositório.
- **Proposta:** Nos ramos de sucesso e de erro de processItem, chamar um callback opcional `onFinished(item)` injetado pelo App, que usa `runtime.InitializeNotifications` uma vez no startup e `runtime.SendNotification` com título "eprocIA" e corpo "<nome>.txt pronto (N docs, M págs)" ou a mensagem de erro. Sem rede; nada é feito se `IsNotificationAvailable` for falso. Opcional: flag `notify_on_finish` no Config, padrão true.
- **Custo:** P
- **Ganho esperado:** O usuário sabe que terminou sem abrir a janela; cerca de 30 linhas e nenhuma dependência nova.
- **Veredito:** rebaixado
- **Justificativa do cético:** Não há notificação, mas o PRD não a exige e a falta dela não quebra nenhum fluxo; é conveniência.

Status: feito

- **Commit:** `cfb5673f4f4769b767a5287f9e5be0b454eb10db`

### B-TE-06 — Nenhum teste garante CREATE_NO_WINDOW nos processos filhos do OCR

- **Arquivo:** `internal/ocr/metrics_test.go:8`
- **Lente:** TE
- **Evidência:** TestPrepareCmd_CountsSpawnedProcesses só verifica o contador SpawnedProcesses. Nenhum *_test.go menciona `CREATE_NO_WINDOW`, `SysProcAttr` ou `HideWindow`. Se hideConsole (proc_windows.go:16-21) virar no-op ou alguém remover a chamada em prepareCmd, todos os testes continuam passando e a armadilha conhecida (console piscando a cada página) volta.
- **Proposta:** proc_windows_test.go (build tag windows) que chama prepareCmd numa exec.Cmd e exige `cmd.SysProcAttr != nil && cmd.SysProcAttr.CreationFlags&0x08000000 != 0 && cmd.SysProcAttr.HideWindow`.
- **Custo:** P
- **Ganho esperado:** Teste de regressão de poucas linhas para uma armadilha já documentada.
- **Veredito:** mantido
- **Justificativa do cético:** hideConsole tem 100% de cobertura, mas nenhum teste confere SysProcAttr/CreationFlags.

Status: feito

- **Commit:** `d9b91f7462753336b8cc363ea468ad8fc0a8d148`
- **Benchstat:**

```
Não rodei o benchmark. Nenhum pacote alterado pela tarefa aparece em go list -deps ./scripts/bench, que só lista o próprio eprocia/scripts/bench. A tarefa só mexeu em arquivos _test.go de internal/ocr.
```

### F-VI-07 — Formulário de configurações com alinhamentos mistos e textos cortados no painel estreito

- **Arquivo:** `frontend/src/components/Settings.svelte:224`
- **Lentes:** VI, RS (fundido com "Painel na largura mínima corta o texto do caminho e das opções de select sem reticências", Settings.svelte:150)
- **Evidência:** Settings.svelte:222-224 tem `.toggle-group { flex-direction: row; justify-content: center; ... }`, que centraliza os interruptores enquanto rótulos e campos ficam à esquerda. As opções longas (149-151, por exemplo "Agressivo (Básico + remove rodapés e cabeçalhos repetitivos)") e o input readonly da pasta (108-114, sem `title`) aparecem cortados no meio da palavra em 1440x900-escuro-processando.png, 800x600-claro-configuracoes.png e 800x600-claro-processando-divisor-minimo.png: "Básico (Corrige espaçamento e quebras r", "Agressivo (Básico + remove rodapés e ca" e "C:\Usuarios\exemplo\OneDr". A classe `.help-text` (243) existe e nunca é usada.
- **Proposta:** Alinhar os interruptores à esquerda (rótulo à esquerda e switch à direita, em linha de 32px). Encurtar as opções ("Nenhuma", "Básica", "Agressiva") e mover a explicação para um `.help-text` de 11px abaixo do select, que muda conforme a opção. No input da pasta, `title={config.monitored_path}` e `text-overflow: ellipsis`, ou mostrar o fim do caminho com `direction: rtl` + `text-align: left`.
- **Custo:** P
- **Ganho esperado:** Formulário escaneável e legível com o painel em 280px, sem texto truncado no meio da palavra.
- **Veredito:** mantido (VI); mantido (RS)
- **Justificativa do cético:** O `.toggle-group` centralizado, o `.help-text` sem uso e os cortes nas capturas foram confirmados.

Status: feito

- **Commit:** `56a53ab5ce260a5d1cdd5434c9d07268d26bd018`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/)

<details><summary>Pares antes/depois (48)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-configuracoes.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-processando.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-processando.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-configuracoes.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-processando.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-processando.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-processando.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-processando.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-configuracoes.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-processando.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-configuracoes.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-processando.png)

</details>

### F-VI-08 — O tema ignora a preferência do sistema e o fundo nativo claro pisca ao abrir no tema escuro padrão

- **Arquivo:** `frontend/src/App.svelte:43`
- **Lente:** VI
- **Evidência:** App.svelte:43-50 aplica escuro sempre que não há `'light'` salvo, sem consultar `prefers-color-scheme`, e o DESIGN §2 diz "respeitando as preferências do sistema operacional por padrão". main.go:37 tem `BackgroundColour: &options.RGBA{R: 248, G: 249, B: 250, A: 1}`, um fundo claro enquanto o tema inicial é escuro.
- **Proposta:** Três estados (sistema, claro, escuro), com padrão "sistema" via `matchMedia('(prefers-color-scheme: dark)')` e escuta de mudança. Aplicar a classe antes da montagem (script inline em index.html) e alinhar o `BackgroundColour` ao fundo escuro ou ao tema do Windows.
- **Custo:** P
- **Ganho esperado:** Primeira impressão sem flash e conformidade com o DESIGN.
- **Veredito:** mantido
- **Justificativa do cético:** App.svelte:43-50 não consulta prefers-color-scheme e main.go:37 usa fundo claro com tema inicial escuro.

Status: feito

- **Commit:** `56a53ab5ce260a5d1cdd5434c9d07268d26bd018`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/)

<details><summary>Pares antes/depois (48)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-configuracoes.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-processando.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-processando.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-configuracoes.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-processando.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-processando.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-processando.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-processando.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-configuracoes.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-processando.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-configuracoes.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-processando.png)

</details>

### F-VI-09 — Cores fixas no código ignoram o tema: overlay de arrastar sempre escuro e badge azul fixo

- **Arquivo:** `frontend/src/App.svelte:395`
- **Lente:** VI (relacionado: F-VI-04)
- **Evidência:** App.svelte:395 tem `background-color: rgba(11, 15, 25, 0.95);` no `.drag-overlay`, sem variante clara. ProcessList.svelte:483 usa `color: #93C5FD;`, e `rgba(59, 130, 246, ...)` aparece em ProcessList.svelte:337 e 482 e em App.svelte:512, mesmo no tema claro (accent #2563EB). Os vermelhos `rgba(220, 53, 69, 0.1)` (ProcessList.svelte:317, 572, 577) vêm de uma terceira paleta, diferente de `--error`.
- **Proposta:** Trocar os valores fixos por tokens derivados, com `color-mix(in srgb, var(--accent) 12%, transparent)` para fundos tonais e `--overlay` por tema. Nenhum hex ou rgba solto nos componentes.
- **Custo:** P
- **Ganho esperado:** Tema claro coerente e uma única fonte de verdade de cor.
- **Veredito:** mantido
- **Justificativa do cético:** O rgba fixo do overlay foi confirmado, e os demais valores citados são coerentes com o padrão do arquivo.

Status: feito

- **Commit:** `56a53ab5ce260a5d1cdd5434c9d07268d26bd018`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/)

<details><summary>Pares antes/depois (48)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-configuracoes.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-claro-processando.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1024x768-escuro-processando.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-configuracoes.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-claro-processando.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1440x900-escuro-processando.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-claro-processando.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/1920x1080-escuro-processando.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-configuracoes.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-claro-processando.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-configuracoes.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-VI-07%2BF-VI-08%2BF-VI-09/800x600-escuro-processando.png)

</details>

### F-VI-10 — Hierarquia tipográfica fora da escala do DESIGN, com cabeçalhos redundantes no painel

- **Arquivo:** `frontend/src/components/Settings.svelte:203`
- **Lente:** VI
- **Evidência:** `.card-header h2 { font-size: 14px }` (Settings.svelte:204), `.header-left h2 { font-size: 14px }` (ProcessList.svelte:277), `.manual-card h3 { font-size: 13px }` (App.svelte:737), `.brand-title h1 { font-size: 13px; font-weight: 700 }` (App.svelte:459-460) e `.sidebar-title { 11px; uppercase; 700 }` (App.svelte:593-596). O DESIGN §3 fixa títulos em 16px semibold, base em 13px e metadados em 11px. O nome do arquivo usa 12px/600 (ProcessList.svelte:446). No painel se empilham "PAINEL DE CONTROLE", "Configurações" com ícone e linha e "Importação Manual" com ícone e linha (1440x900-escuro-processando.png).
- **Proposta:** Escala única em tokens (`--fs-title: 16px/600`, `--fs-section: 12px/600 uppercase` em cor secundária, `--fs-base: 13px`, `--fs-meta: 11px`). Nome do arquivo em 13px/500. Remover o "Painel de Controle" ou fundi-lo com o botão de recolher, e trocar os cards do painel por seções com rótulo pequeno e divisor de 1px.
- **Custo:** P
- **Ganho esperado:** Hierarquia legível e menos cromo no painel lateral.
- **Veredito:** mantido
- **Justificativa do cético:** Settings.svelte:204 tem h2 de 14px contra os 16px do DESIGN.md:40, e a captura mostra os três níveis de cabeçalho.

Status: feito

- **Commit:** `406dacab40c19397c71d58e3d88a651b5fab8fe1`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-VI-06+F-VI-10/`](capturas/2026-10-15/depois-F-VI-06%2BF-VI-10/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-VI-06%2BF-VI-10/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### F-AC-06 — Campo de filtro sem rótulo, só com placeholder

- **Arquivo:** `frontend/src/components/ProcessList.svelte:84`
- **Lente:** AC
- **Evidência:** `<input type="text" placeholder="Filtrar..." bind:value={filterText} class="search-input" />` (84-89) não tem `<label>`, `aria-label` nem `type="search"`. Quando o filtro não acha nada, aparece o mesmo "Nenhum processo no histórico." (113), o que engana.
- **Proposta:** `type="search" aria-label="Filtrar fila por nome ou mensagem"` e, com `filterText` preenchido, mostrar "Nenhum item corresponde ao filtro" com `role="status"`.
- **Custo:** P
- **Ganho esperado:** Nome acessível estável e retorno correto quando o filtro não acha nada.
- **Veredito:** mantido
- **Justificativa do cético:** O input só tem placeholder, e o estado vazio (111-114) não distingue filtro sem resultado de fila vazia.

Status: feito

- **Commit:** `46ad443f6f877e1867dd55d1971e0856db15b00e`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/)

<details><summary>Pares antes/depois (56)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-configuracoes.png)
- 1024x768-claro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-erro.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-claro-processando.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-erro.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-erro.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1024x768-escuro-processando.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-configuracoes.png)
- 1440x900-claro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-erro.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-claro-processando.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-erro.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-erro.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1440x900-escuro-processando.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-erro.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-claro-processando.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-erro.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-erro.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/1920x1080-escuro-processando.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-configuracoes.png)
- 800x600-claro-erro.png: [antes](capturas/2026-10-15/antes/800x600-claro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-erro.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-claro-processando.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-configuracoes.png)
- 800x600-escuro-erro.png: [antes](capturas/2026-10-15/antes/800x600-escuro-erro.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-erro.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-AC-05%2BF-AC-06/800x600-escuro-processando.png)

</details>

### F-AC-07 — Botões com estado não expõem esse estado (aria-expanded, aria-pressed, rótulo de maximizar)

- **Arquivo:** `frontend/src/App.svelte:280`
- **Lente:** AC
- **Evidência:** `<button class="btn-toggle-sidebar" ... title={sidebarCollapsed ? "Expandir menu" : "Recolher menu"}>` (280) não tem `aria-expanded`/`aria-controls`. O botão de tema (243) tem só `title="Alternar tema"`. O botão maximizar (257) diz sempre "Maximizar", mesmo com a janela maximizada. O nome acessível vem só do `title`.
- **Proposta:** `aria-label` explícito, `aria-expanded={!sidebarCollapsed}` com `aria-controls` apontando para o id do aside, `aria-pressed={!isDarkMode}` (ou rótulo "Usar tema claro/escuro") e rótulo Maximizar/Restaurar via `WindowIsMaximised`. SVGs decorativos com `aria-hidden="true"`.
- **Custo:** P
- **Ganho esperado:** Leitores de tela informam o estado real dos controles da barra de título e da sidebar.
- **Veredito:** mantido
- **Justificativa do cético:** Os botões em 243, 257 e 280 têm só title, e o de maximizar tem o title fixo.

Status: feito

- **Commit:** `3b0912daeee3d4b273c18ff7aa06c1a95fe922d9`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-AC-07+F-AC-08+F-RS-03/`](capturas/2026-10-15/depois-F-AC-07%2BF-AC-08%2BF-RS-03/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-AC-07%2BF-AC-08%2BF-RS-03/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### F-AC-08 — Divisor redimensionável com teclado incompleto (sem Home/End nem preventDefault)

- **Arquivo:** `frontend/src/App.svelte:326`
- **Lente:** AC (relacionado: F-AN-02, F-RS-02)
- **Evidência:** `role="slider" aria-label="Redimensionador de Menu" aria-orientation="horizontal" ... tabindex="0"` (326-338). `handleResizeKeydown` (175-181) trata só ArrowLeft/ArrowRight, não chama preventDefault e ignora Home/End. Não há `aria-valuetext` nem `aria-controls`. O CSS só destaca o divisor em hover e em `.resizing` (638).
- **Proposta:** Tratar Home/End (mínimo/máximo, com o teto dinâmico de F-RS-02) e Enter (recolher/expandir), chamar `e.preventDefault()`, adicionar `aria-valuetext="{sidebarWidth} pixels"` e `aria-controls="sidebar"`, e um destaque explícito em `.resize-handle:focus-visible` coerente com o anel de foco de F-AC-04. A troca de papel para `separator` é opcional.
- **Custo:** P
- **Ganho esperado:** Divisor operável por teclado de forma completa, com retorno visual consistente.
- **Veredito:** rebaixado
- **Justificativa do cético:** O teclado incompleto se confirma. Mas o div com tabindex não tem `outline: none` e recebe o anel `:focus-visible` padrão do WebView2, então o foco aparece. Para um slider que responde a ArrowLeft/Right, `aria-orientation="horizontal"` é coerente, e a troca para separator é preferência.

Status: feito

- **Commit:** `3b0912daeee3d4b273c18ff7aa06c1a95fe922d9`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-AC-07+F-AC-08+F-RS-03/`](capturas/2026-10-15/depois-F-AC-07%2BF-AC-08%2BF-RS-03/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-AC-07%2BF-AC-08%2BF-RS-03/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### F-RS-03 — Largura e estado recolhido do painel não são persistidos

- **Arquivo:** `frontend/src/App.svelte:28`
- **Lente:** RS
- **Evidência:** App.svelte:28-29 `let sidebarWidth = 320;` e `let sidebarCollapsed = false;` ficam só em memória, e `toggleSidebar` (183-185) e `stopResizing` (169-173) não gravam nada, enquanto o tema já é persistido no mesmo arquivo (`localStorage.setItem('theme', ...)`, 123/126). O tamanho da janela também não é guardado (main.go:29-30 fixa 1024x768).
- **Proposta:** Gravar `sidebarWidth` e `sidebarCollapsed` em `localStorage` (por exemplo `eprocia.sidebar`, com try/catch) em stopResizing/toggleSidebar e ler no onMount, aplicando o clamp dinâmico de F-RS-02. Opcional: guardar tamanho e estado maximizado da janela com `WindowGetSize`/`WindowIsMaximised` e aplicar com `WindowSetSize` no startup.
- **Custo:** P
- **Ganho esperado:** O layout escolhido se mantém entre sessões.
- **Veredito:** rebaixado
- **Justificativa do cético:** Confirmado, mas é conveniência, sem defeito funcional: o painel volta ao padrão razoável de 320px.

Status: feito

- **Commit:** `3b0912daeee3d4b273c18ff7aa06c1a95fe922d9`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-AC-07+F-AC-08+F-RS-03/`](capturas/2026-10-15/depois-F-AC-07%2BF-AC-08%2BF-RS-03/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-AC-07%2BF-AC-08%2BF-RS-03/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### F-AN-04 — Overlay de arrastar e soltar some de uma vez e só a caixa anima na entrada

- **Arquivo:** `frontend/src/App.svelte:216`
- **Lente:** AN
- **Evidência:** `{#if dragActive}` monta `.drag-overlay` com fundo `rgba(11, 15, 25, 0.95)` (395) sem transição; só `.overlay-box` tem `animation: zoomIn 0.15s ease-out` (413). O véu de 95% cobre a janela num frame e some num frame no dragleave ou no drop, o que pisca quando o cursor passa pela borda.
- **Proposta:** `transition:fade={{ duration: 150 }}` no `.drag-overlay`, manter o zoomIn da caixa com `--dur-fast`/`--ease-out` (scale 0.98) e desligar tudo em reduced-motion (F-AN-03). A cor do overlay é tratada em F-VI-09.
- **Custo:** P
- **Ganho esperado:** Entrada e saída do overlay sem piscar.
- **Veredito:** mantido
- **Justificativa do cético:** Confirmado; problema cosmético real de gravidade baixa.

Status: feito

- **Commit:** `833373e047fe117da6c00de3e7fa3c601db12954`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-AN-02+F-AN-03+F-AN-04/`](capturas/2026-10-15/depois-F-AN-02%2BF-AN-03%2BF-AN-04/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-AN-02%2BF-AN-03%2BF-AN-04/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### F-DF-02 — Recolher a barra lateral desmonta Settings, e cada expansão refaz GetConfig e mostra o spinner

- **Arquivo:** `frontend/src/App.svelte:292`
- **Lente:** DF
- **Evidência:** `<Settings on:configChanged={handleConfigChanged} />` está dentro de `{#if !sidebarCollapsed}` (App.svelte:289-292). Settings carrega a configuração por conta própria no onMount (Settings.svelte:19-35, `loading = true` e `await GetConfig()`), enquanto App já carregou a mesma configuração (App.svelte:53, 107-113). São duas chamadas GetConfig na abertura e uma a cada expansão, com o card trocado por "Carregando configurações..." e salto de layout.
- **Proposta:** Fazer de App a fonte única, passando `config` como prop (ou, em Svelte 5, `$bindable` ou store `$state` compartilhada), e tirar o GetConfig do Settings. Alternativa: manter Settings montado e escondê-lo por CSS ao recolher.
- **Custo:** P
- **Ganho esperado:** Uma única leitura de configuração, sem spinner nem salto ao expandir e sem divergência entre App e Settings.
- **Veredito:** rebaixado
- **Justificativa do cético:** Confirmado, mas GetConfig é uma leitura local barata e o spinner só pisca ao expandir.

Status: feito

- **Commit:** `8c35250c530d50981787c551e361b0058828a0c2`
- **Capturas:** [antes](capturas/2026-10-15/antes/) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/)

<details><summary>Pares antes/depois (48)</summary>

- 1024x768-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-claro-configuracoes.png)
- 1024x768-claro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-claro-fila.png)
- 1024x768-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-claro-processando-divisor-maximo.png)
- 1024x768-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-claro-processando-divisor-minimo.png)
- 1024x768-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-claro-processando-recolhido.png)
- 1024x768-claro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-claro-processando.png)
- 1024x768-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-escuro-configuracoes.png)
- 1024x768-escuro-fila.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-escuro-fila.png)
- 1024x768-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-escuro-processando-divisor-maximo.png)
- 1024x768-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-escuro-processando-divisor-minimo.png)
- 1024x768-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-escuro-processando-recolhido.png)
- 1024x768-escuro-processando.png: [antes](capturas/2026-10-15/antes/1024x768-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1024x768-escuro-processando.png)
- 1440x900-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-claro-configuracoes.png)
- 1440x900-claro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-claro-fila.png)
- 1440x900-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-claro-processando-divisor-maximo.png)
- 1440x900-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-claro-processando-divisor-minimo.png)
- 1440x900-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-claro-processando-recolhido.png)
- 1440x900-claro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-claro-processando.png)
- 1440x900-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-escuro-configuracoes.png)
- 1440x900-escuro-fila.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-escuro-fila.png)
- 1440x900-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-escuro-processando-divisor-maximo.png)
- 1440x900-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-escuro-processando-divisor-minimo.png)
- 1440x900-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-escuro-processando-recolhido.png)
- 1440x900-escuro-processando.png: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1440x900-escuro-processando.png)
- 1920x1080-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-claro-configuracoes.png)
- 1920x1080-claro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-claro-fila.png)
- 1920x1080-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-claro-processando-divisor-maximo.png)
- 1920x1080-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-claro-processando-divisor-minimo.png)
- 1920x1080-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-claro-processando-recolhido.png)
- 1920x1080-claro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-claro-processando.png)
- 1920x1080-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-escuro-configuracoes.png)
- 1920x1080-escuro-fila.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-escuro-fila.png)
- 1920x1080-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-escuro-processando-divisor-maximo.png)
- 1920x1080-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-escuro-processando-divisor-minimo.png)
- 1920x1080-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-escuro-processando-recolhido.png)
- 1920x1080-escuro-processando.png: [antes](capturas/2026-10-15/antes/1920x1080-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/1920x1080-escuro-processando.png)
- 800x600-claro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-claro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-claro-configuracoes.png)
- 800x600-claro-fila.png: [antes](capturas/2026-10-15/antes/800x600-claro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-claro-fila.png)
- 800x600-claro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-claro-processando-divisor-maximo.png)
- 800x600-claro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-claro-processando-divisor-minimo.png)
- 800x600-claro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-claro-processando-recolhido.png)
- 800x600-claro-processando.png: [antes](capturas/2026-10-15/antes/800x600-claro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-claro-processando.png)
- 800x600-escuro-configuracoes.png: [antes](capturas/2026-10-15/antes/800x600-escuro-configuracoes.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-escuro-configuracoes.png)
- 800x600-escuro-fila.png: [antes](capturas/2026-10-15/antes/800x600-escuro-fila.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-escuro-fila.png)
- 800x600-escuro-processando-divisor-maximo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-escuro-processando-divisor-maximo.png)
- 800x600-escuro-processando-divisor-minimo.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-minimo.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-escuro-processando-divisor-minimo.png)
- 800x600-escuro-processando-recolhido.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando-recolhido.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-escuro-processando-recolhido.png)
- 800x600-escuro-processando.png: [antes](capturas/2026-10-15/antes/800x600-escuro-processando.png) · [depois](capturas/2026-10-15/depois-F-DF-01%2BF-DF-02/800x600-escuro-processando.png)

</details>

### F-DF-03 — formatTime cria Date e chama toLocaleTimeString para cada linha em cada atualização

- **Arquivo:** `frontend/src/components/ProcessList.svelte:147`
- **Lente:** DF (relacionado: F-DF-01)
- **Evidência:** `<span class="timestamp">{formatTime(item.timestamp)}</span>` (147) chama `new Date(timestamp).toLocaleTimeString([], {...})` (30-34). Como cada `queue:update` traz objetos novos, o Svelte 4 reavalia a expressão em todas as linhas a cada evento. O filtro chama `filterText.toLowerCase()` duas vezes por item (17-20).
- **Proposta:** Um único `Intl.DateTimeFormat('pt-BR', {hour, minute, second})` no módulo, reaproveitado (com memoização por timestamp se preciso). Calcular `const q = filterText.toLowerCase()` uma vez e, com o filtro vazio, devolver `items` sem filtrar.
- **Custo:** P
- **Ganho esperado:** Elimina trabalho síncrono repetido na thread da UI, proporcional ao tamanho da fila vezes a taxa de eventos.
- **Veredito:** mantido
- **Justificativa do cético:** O custo é real, mas pequeno e derivado de F-DF-01.

Status: feito

- **Commit:** `8719411b0a85e8db4fb7fd6468159cf09e17423f`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-AC-09+F-AC-10+F-DF-03/`](capturas/2026-10-15/depois-F-AC-09%2BF-AC-10%2BF-DF-03/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-AC-09%2BF-AC-10%2BF-DF-03/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### B-GM-01 — Modernizadores do go fix pendentes: atomic.Int32, max, maps.Copy, slices.Backward, for range n, any

- **Arquivo:** `internal/etl/pipeline.go:202`
- **Lente:** GM
- **Evidência:** `go fix -diff ./internal/... .` (Go 1.27.1) propõe mudanças reais em três arquivos (o resto do diff é CRLF e indentação): pipeline.go:202-203 `var processedDocsVal int32`/`processedPagesVal int32` com `atomic.LoadInt32/AddInt32(&...)` (207-208, 279, 319-320, 387), que viram `atomic.Int32`; pipeline.go:185-188, que vira `max(stdRuntime.NumCPU()-1, 1)`; pipeline.go:345-347, que vira `maps.Copy`; `map[string]interface{}` em pipeline.go:220 e 441 e `interface{}` em queue.go:371, que viram `any`; queue.go:250, que vira `slices.Backward`; watchdog.go:150, que vira `for range retries`; e main.go:47 `[]interface{}`. Subir a diretiva do go.mod para `go 1.27` só acrescenta o `[]any` de main.go.
- **Proposta:** Aplicar `go fix ./...`, depois `gofmt -w` só nos arquivos tocados, e revisar o diff. O ganho principal é `atomic.Int32`, que impede acessar os contadores sem atomic.
- **Custo:** P
- **Ganho esperado:** Contadores compartilhados só acessíveis via atomic (erro pego na compilação), cerca de 15 linhas a menos e `go fix -diff` zerado como referência para regressões.
- **Veredito:** rebaixado
- **Justificativa do cético:** Os trechos existem, mas hoje todo acesso aos contadores já passa por atomic; o ganho é prevenção e legibilidade, sem bug atual.

Status: feito

- **Commit:** `a7dc62ef81fc2fb82bdbb8f9c8a4c8196ba82595`
- **Benchstat:**

```
Sintético e ParsePDF, ocr=none, -count=6, antes=HEAD f339b05, depois=árvore atual
                           sec/op antes      sec/op depois     vs base
Sintetico/ocr=none-12      55.03m ± 15%      54.15m ± 15%      ~ (p=0.485 n=6)
ParsePDF/nativo-12         1.010m ±  3%      1.001m ±  8%      ~ (p=0.818 n=6)
ParsePDF/escaneado-12      220.0µ ±  4%      224.2µ ±  3%      ~ (p=0.394 n=6)
ParsePDF/vetorial-12       14.44m ±  2%      14.66m ±  2%      ~ (p=0.093 n=6)
ParsePDF/contents-array-12 149.6µ ±  4%      146.2µ ±  3%      -2.25% (p=0.041 n=6)
geomean                    1.925m            1.920m            -0.27%

                           gc-pausa-sec/op
Sintetico/ocr=none-12      4.185m ± 32%      3.913m ± 39%      ~ (p=0.394 n=6)

                           pico-mem-B
Sintetico/ocr=none-12      26.86Mi ± 15%     22.86Mi ± 0%      -14.89% (p=0.015 n=6)

                           procs/op
Sintetico/ocr=none-12      0.000 ± 0%        0.000 ± 0%        ~ (p=1.000 n=6) (todas as amostras iguais)

                           B/op
Sintetico/ocr=none-12      16.70Mi ± 0%      16.76Mi ± 0%      ~ (p=0.065 n=6)
ParsePDF/nativo-12         262.7Ki ± 0%      262.7Ki ± 0%      ~ (p=0.310 n=6)
ParsePDF/escaneado-12      54.09Ki ± 0%      54.09Ki ± 0%      ~ (p=0.665 n=6)
ParsePDF/vetorial-12       1.845Mi ± 0%      1.844Mi ± 0%      ~ (p=0.394 n=6)
ParsePDF/contents-array-12 34.87Ki ± 0%      34.86Ki ± 0%      -0.04% (p=0.002 n=6)
geomean                    437.4Ki           437.6Ki           +0.05%

                           allocs/op
Sintetico/ocr=none-12      291.9k ± 0%       293.9k ± 0%       +0.66% (p=0.002 n=6)
ParsePDF/nativo-12         2.303k ± 0%       2.303k ± 0%       ~ (p=1.000 n=6)
ParsePDF/escaneado-12      326.0 ± 0%        326.0 ± 0%        ~ (p=1.000 n=6)
ParsePDF/vetorial-12       104.6k ± 0%       104.6k ± 0%       ~ (p=0.242 n=6)
ParsePDF/contents-array-12 231.0 ± 0%        231.0 ± 0%        ~ (p=1.000 n=6)
geomean                    5.557k            5.564k            +0.13%
```

### B-GM-02 — sort.Slice com closures por índice no lugar de slices.SortFunc/cmp.Compare

- **Arquivo:** `internal/etl/formatter.go:145`
- **Lente:** GM
- **Evidência:** formatter.go:145, 177 e 207 usam `sort.Slice(docs, func(i, j int) bool { return docs[i].index < docs[j].index })` (e o mesmo para `pages`), a linha 107 usa `sort.Ints(eventNums)` e sorter.go:71 faz `sort.Slice(files, ...)` com três níveis de desempate. Com closures por índice é fácil indexar o slice errado, já que os nomes se repetem na mesma função.
- **Proposta:** `slices.SortFunc(docs, func(a, b docEntry) int { return cmp.Compare(a.index, b.index) })` e `slices.Sort(eventNums)`. Em sorter.go, `cmp.Or(cmp.Compare(a.EventNum, b.EventNum), cmp.Compare(a.DocNum, b.DocNum), ...)`, mantendo o desempate natural (coberto por TestSortProcessFiles).
- **Custo:** P
- **Ganho esperado:** Comparadores tipados, sem índices.
- **Veredito:** mantido
- **Justificativa do cético:** Modernização legítima de legibilidade; o ganho de desempenho é desprezível e o risco de indexar errado é hipotético.

Status: feito

- **Commit:** `a7dc62ef81fc2fb82bdbb8f9c8a4c8196ba82595`
- **Benchstat:**

```
Sintético e ParsePDF, ocr=none, -count=6, antes=HEAD f339b05, depois=árvore atual
                           sec/op antes      sec/op depois     vs base
Sintetico/ocr=none-12      55.03m ± 15%      54.15m ± 15%      ~ (p=0.485 n=6)
ParsePDF/nativo-12         1.010m ±  3%      1.001m ±  8%      ~ (p=0.818 n=6)
ParsePDF/escaneado-12      220.0µ ±  4%      224.2µ ±  3%      ~ (p=0.394 n=6)
ParsePDF/vetorial-12       14.44m ±  2%      14.66m ±  2%      ~ (p=0.093 n=6)
ParsePDF/contents-array-12 149.6µ ±  4%      146.2µ ±  3%      -2.25% (p=0.041 n=6)
geomean                    1.925m            1.920m            -0.27%

                           gc-pausa-sec/op
Sintetico/ocr=none-12      4.185m ± 32%      3.913m ± 39%      ~ (p=0.394 n=6)

                           pico-mem-B
Sintetico/ocr=none-12      26.86Mi ± 15%     22.86Mi ± 0%      -14.89% (p=0.015 n=6)

                           procs/op
Sintetico/ocr=none-12      0.000 ± 0%        0.000 ± 0%        ~ (p=1.000 n=6) (todas as amostras iguais)

                           B/op
Sintetico/ocr=none-12      16.70Mi ± 0%      16.76Mi ± 0%      ~ (p=0.065 n=6)
ParsePDF/nativo-12         262.7Ki ± 0%      262.7Ki ± 0%      ~ (p=0.310 n=6)
ParsePDF/escaneado-12      54.09Ki ± 0%      54.09Ki ± 0%      ~ (p=0.665 n=6)
ParsePDF/vetorial-12       1.845Mi ± 0%      1.844Mi ± 0%      ~ (p=0.394 n=6)
ParsePDF/contents-array-12 34.87Ki ± 0%      34.86Ki ± 0%      -0.04% (p=0.002 n=6)
geomean                    437.4Ki           437.6Ki           +0.05%

                           allocs/op
Sintetico/ocr=none-12      291.9k ± 0%       293.9k ± 0%       +0.66% (p=0.002 n=6)
ParsePDF/nativo-12         2.303k ± 0%       2.303k ± 0%       ~ (p=1.000 n=6)
ParsePDF/escaneado-12      326.0 ± 0%        326.0 ± 0%        ~ (p=1.000 n=6)
ParsePDF/vetorial-12       104.6k ± 0%       104.6k ± 0%       ~ (p=0.242 n=6)
ParsePDF/contents-array-12 231.0 ± 0%        231.0 ± 0%        ~ (p=1.000 n=6)
geomean                    5.557k            5.564k            +0.13%
```

### B-GM-03 — APIs antigas: os.IsNotExist no lugar de errors.Is(fs.ErrNotExist), e UnixNano()/Millisecond no lugar de UnixMilli()

- **Arquivo:** `internal/config/config.go:67`
- **Lente:** GM
- **Evidência:** config.go:67 e watchdog.go:157 usam `os.IsNotExist(err)`, que não desembrulha erros com `%w` (a documentação do pacote os recomenda `errors.Is(err, fs.ErrNotExist)` em código novo). queue.go:109 usa `time.Now().UnixNano() / int64(time.Millisecond)`, que é exatamente `time.Now().UnixMilli()`.
- **Proposta:** Trocar por `errors.Is(err, fs.ErrNotExist)` nos dois pontos e por `time.Now().UnixMilli()` em queue.go:109.
- **Custo:** P
- **Ganho esperado:** A checagem continua certa se alguém embrulhar o erro, e some uma conversão manual de unidade.
- **Veredito:** mantido
- **Justificativa do cético:** Hoje os erros vêm direto de os.Stat/ReadFile, então a troca é preventiva e de estilo.

Status: feito

- **Commit:** `a7dc62ef81fc2fb82bdbb8f9c8a4c8196ba82595`
- **Benchstat:**

```
Sintético e ParsePDF, ocr=none, -count=6, antes=HEAD f339b05, depois=árvore atual
                           sec/op antes      sec/op depois     vs base
Sintetico/ocr=none-12      55.03m ± 15%      54.15m ± 15%      ~ (p=0.485 n=6)
ParsePDF/nativo-12         1.010m ±  3%      1.001m ±  8%      ~ (p=0.818 n=6)
ParsePDF/escaneado-12      220.0µ ±  4%      224.2µ ±  3%      ~ (p=0.394 n=6)
ParsePDF/vetorial-12       14.44m ±  2%      14.66m ±  2%      ~ (p=0.093 n=6)
ParsePDF/contents-array-12 149.6µ ±  4%      146.2µ ±  3%      -2.25% (p=0.041 n=6)
geomean                    1.925m            1.920m            -0.27%

                           gc-pausa-sec/op
Sintetico/ocr=none-12      4.185m ± 32%      3.913m ± 39%      ~ (p=0.394 n=6)

                           pico-mem-B
Sintetico/ocr=none-12      26.86Mi ± 15%     22.86Mi ± 0%      -14.89% (p=0.015 n=6)

                           procs/op
Sintetico/ocr=none-12      0.000 ± 0%        0.000 ± 0%        ~ (p=1.000 n=6) (todas as amostras iguais)

                           B/op
Sintetico/ocr=none-12      16.70Mi ± 0%      16.76Mi ± 0%      ~ (p=0.065 n=6)
ParsePDF/nativo-12         262.7Ki ± 0%      262.7Ki ± 0%      ~ (p=0.310 n=6)
ParsePDF/escaneado-12      54.09Ki ± 0%      54.09Ki ± 0%      ~ (p=0.665 n=6)
ParsePDF/vetorial-12       1.845Mi ± 0%      1.844Mi ± 0%      ~ (p=0.394 n=6)
ParsePDF/contents-array-12 34.87Ki ± 0%      34.86Ki ± 0%      -0.04% (p=0.002 n=6)
geomean                    437.4Ki           437.6Ki           +0.05%

                           allocs/op
Sintetico/ocr=none-12      291.9k ± 0%       293.9k ± 0%       +0.66% (p=0.002 n=6)
ParsePDF/nativo-12         2.303k ± 0%       2.303k ± 0%       ~ (p=1.000 n=6)
ParsePDF/escaneado-12      326.0 ± 0%        326.0 ± 0%        ~ (p=1.000 n=6)
ParsePDF/vetorial-12       104.6k ± 0%       104.6k ± 0%       ~ (p=0.242 n=6)
ParsePDF/contents-array-12 231.0 ± 0%        231.0 ± 0%        ~ (p=1.000 n=6)
geomean                    5.557k            5.564k            +0.13%
```

### B-GM-04 — sync.WaitGroup.Go no lugar de Add(1)/go/defer Done separados (pipeline, pdf_parser, watchdog)

- **Arquivo:** `internal/etl/pdf_parser.go:159`
- **Lente:** GM
- **Evidência:** Em pdf_parser.go o `wg.Add(1)` fica na linha 159 e o `go runOCRTask(...)` na 160, mas o `defer wg.Done()` está dentro de runOCRTask (linha 77), 80 linhas acima. Em pipeline.go:240-242 há `wg.Add(1)` + `go func(i int, f models.ProcessFile) { defer wg.Done()`, com parâmetros desnecessários desde a semântica de variável de laço do Go 1.22 (`i` nem é usado). Em watchdog.go:75-76 há `w.wg.Add(1)` + `go w.watchLoop()`, com o Done em outra função (112). A diretiva `go 1.25.0` já permite `WaitGroup.Go`.
- **Proposta:** `wg.Go(func() { runOCRTask(task) })`, tirando o Done de runOCRTask; em pipeline.go, `wg.Go(func() { ... })` sem os parâmetros; em watchdog.go, `w.wg.Go(w.watchLoop)` sem o Done dentro de watchLoop. Se B-ES-09 for aplicado, usar o mesmo padrão nos goroutines de handleNewFile.
- **Custo:** P
- **Ganho esperado:** Some a classe de erro em que Add e Done ficam desbalanceados.
- **Veredito:** rebaixado
- **Justificativa do cético:** Os pares estão balanceados e corretos hoje; é refatoração de estilo para manutenção futura.

Status: feito

- **Commit:** `a7dc62ef81fc2fb82bdbb8f9c8a4c8196ba82595`
- **Benchstat:**

```
Sintético e ParsePDF, ocr=none, -count=6, antes=HEAD f339b05, depois=árvore atual
                           sec/op antes      sec/op depois     vs base
Sintetico/ocr=none-12      55.03m ± 15%      54.15m ± 15%      ~ (p=0.485 n=6)
ParsePDF/nativo-12         1.010m ±  3%      1.001m ±  8%      ~ (p=0.818 n=6)
ParsePDF/escaneado-12      220.0µ ±  4%      224.2µ ±  3%      ~ (p=0.394 n=6)
ParsePDF/vetorial-12       14.44m ±  2%      14.66m ±  2%      ~ (p=0.093 n=6)
ParsePDF/contents-array-12 149.6µ ±  4%      146.2µ ±  3%      -2.25% (p=0.041 n=6)
geomean                    1.925m            1.920m            -0.27%

                           gc-pausa-sec/op
Sintetico/ocr=none-12      4.185m ± 32%      3.913m ± 39%      ~ (p=0.394 n=6)

                           pico-mem-B
Sintetico/ocr=none-12      26.86Mi ± 15%     22.86Mi ± 0%      -14.89% (p=0.015 n=6)

                           procs/op
Sintetico/ocr=none-12      0.000 ± 0%        0.000 ± 0%        ~ (p=1.000 n=6) (todas as amostras iguais)

                           B/op
Sintetico/ocr=none-12      16.70Mi ± 0%      16.76Mi ± 0%      ~ (p=0.065 n=6)
ParsePDF/nativo-12         262.7Ki ± 0%      262.7Ki ± 0%      ~ (p=0.310 n=6)
ParsePDF/escaneado-12      54.09Ki ± 0%      54.09Ki ± 0%      ~ (p=0.665 n=6)
ParsePDF/vetorial-12       1.845Mi ± 0%      1.844Mi ± 0%      ~ (p=0.394 n=6)
ParsePDF/contents-array-12 34.87Ki ± 0%      34.86Ki ± 0%      -0.04% (p=0.002 n=6)
geomean                    437.4Ki           437.6Ki           +0.05%

                           allocs/op
Sintetico/ocr=none-12      291.9k ± 0%       293.9k ± 0%       +0.66% (p=0.002 n=6)
ParsePDF/nativo-12         2.303k ± 0%       2.303k ± 0%       ~ (p=1.000 n=6)
ParsePDF/escaneado-12      326.0 ± 0%        326.0 ± 0%        ~ (p=1.000 n=6)
ParsePDF/vetorial-12       104.6k ± 0%       104.6k ± 0%       ~ (p=0.242 n=6)
ParsePDF/contents-array-12 231.0 ± 0%        231.0 ± 0%        ~ (p=1.000 n=6)
geomean                    5.557k            5.564k            +0.13%
```

### B-EF-04 — Um pdftoppm e um tesseract por página, com o modelo recarregado a cada vez

- **Arquivo:** `internal/ocr/tesseract.go:136`
- **Lente:** EF
- **Evidência:** RunOCR cria um diretório temporário, executa pdftoppm com `-f N -l N` (136-143) e um tesseract.exe por página (181-185); o benchmark mostra 10 procs/op para 5 páginas de OCR no misto.zip. Com tessdata/normal e OMP_THREAD_LIMIT=1, 4 páginas, duas rodadas: em lote (um tesseract com lista de imagens), 10.658 ms e 9.893 ms; com 4 invocações separadas, 12.266 ms e 13.587 ms. A diferença de ≈0,4 a 0,9 s por página é carga de DLLs e de por.traineddata (15 MB). Só iniciar o tesseract (`-v`) variou de 158 a 1.659 ms; o pdftoppm `-v`, ≈124 ms.
- **Proposta:** Agrupar o OCR por documento em blocos de k páginas por worker (k = páginas_OCR/numWorkers, arredondado para cima), com um pdftoppm por bloco (`-f primeira -l última`) e um tesseract por bloco com arquivo de lista, separando a saída por `\f`. O paralelismo entre blocos continua.
- **Custo:** M
- **Ganho esperado:** Entre −13% e −27% de tempo de OCR em documentos de várias páginas (medido em execução serial de 4 páginas), e processos filhos de 2·N para 2·(N/k).
- **Veredito:** rebaixado
- **Justificativa do cético:** O ganho foi medido em série, mas no pipeline as páginas já correm em paralelo com NumCPU-1 workers, o que dilui o custo de inicialização no tempo de parede. Blocos de k páginas reduzem esse paralelismo, e o custo M não compensa o ganho incerto. Só vale aplicar com benchmark que mostre ganho com p < 0,05.

Status: feito

- **Commit:** `f339b05038f2ff92d731ee16d4d779b609b70c15`
- **Benchstat:**

```
Rodada principal (-count=10, base HEAD b3411e6 vs árvore atual):

sec/op                              antes             depois            vs base
Sintetico/ocr=none-12               173.93m ± 232%    60.27m ±  5%      -65.35% (p=0.000 n=10)
ParsePDF/nativo-12                  5.517m ± 117%     1.242m ±  7%      -77.48% (p=0.000 n=10)
ParsePDF/escaneado-12               1504.9µ ±  51%    334.1µ ± 21%      -77.80% (p=0.000 n=10)
ParsePDF/vetorial-12                63.32m ±  71%     17.22m ± 22%      -72.81% (p=0.000 n=10)
ParsePDF/contents-array-12          1104.9µ ± 1656%   210.3µ ± 27%      -80.97% (p=0.007 n=10)
Sintetico/ocr=fast-12               4.948 ±  38%      5.923 ± 51%       ~ (p=0.075 n=10)
Sintetico/escaneado24/ocr=fast-12   12.469 ±  15%     9.915 ± 12%       -20.49% (p=0.000 n=10)

gc-pausa-sec/op
Sintetico/ocr=none-12               23.693m ± 257%    3.556m ±  10%     -84.99% (p=0.000 n=10)
Sintetico/ocr=fast-12               282.32m ± 506%    91.94m ± 201%     -67.43% (p=0.043 n=10)
Sintetico/escaneado24/ocr=fast-12   196.96m ± 284%    24.42m ± 180%     -87.60% (p=0.001 n=10)

pico-mem-B
Sintetico/ocr=none-12               31.02Mi ± 28%     22.86Mi ± 0%      -26.30% (p=0.001 n=10)
Sintetico/ocr=fast-12               27.47Mi ± 15%     23.10Mi ± 1%      -15.94% (p=0.001 n=10)
Sintetico/escaneado24/ocr=fast-12   75.60Mi ±  5%     67.36Mi ± 0%      -10.90% (p=0.001 n=10)

procs/op
Sintetico/ocr=none-12               0.000 ± 0%        0.000 ± 0%        ~ (p=1.000 n=10)
Sintetico/ocr=fast-12               10.00 ± 0%        10.00 ± 0%        ~ (p=1.000 n=10)
Sintetico/escaneado24/ocr=fast-12   48.00 ± 0%        16.00 ± 0%        -66.67% (p=0.000 n=10)

B/op
Sintetico/ocr=none-12               16.20Mi ± 2%      16.58Mi ± 0%      +2.37% (p=0.000 n=10)
ParsePDF/nativo-12                  262.5Ki ± 0%      262.7Ki ± 0%      +0.09% (p=0.035 n=10)
ParsePDF/escaneado-12               53.96Ki ± 0%      54.12Ki ± 0%      +0.30% (p=0.000 n=10)
ParsePDF/vetorial-12                1.844Mi ± 0%      1.845Mi ± 0%      +0.03% (p=0.023 n=10)
ParsePDF/contents-array-12          34.79Ki ± 0%      34.88Ki ± 0%      +0.24% (p=0.000 n=10)
Sintetico/ocr=fast-12               17.88Mi ± 1%      17.91Mi ± 1%      ~ (p=0.684 n=10)
Sintetico/escaneado24/ocr=fast-12   9.242Mi ± 1%      4.178Mi ± 3%      -54.79% (p=0.000 n=10)

allocs/op
Sintetico/ocr=none-12               291.6k ± 0%       291.9k ± 0%       +0.07% (p=0.000 n=10)
ParsePDF/nativo-12                  2.301k ± 0%       2.303k ± 0%       +0.09% (p=0.000 n=10)
ParsePDF/escaneado-12               324.0 ± 0%        326.0 ± 0%        +0.62% (p=0.000 n=10)
ParsePDF/vetorial-12                104.6k ± 0%       104.6k ± 0%       +0.00% (p=0.000 n=10)
ParsePDF/contents-array-12          229.0 ± 0%        231.0 ± 0%        +0.87% (p=0.000 n=10)
Sintetico/ocr=fast-12               296.6k ± 0%       296.8k ± 0%       +0.07% (p=0.019 n=10)
Sintetico/escaneado24/ocr=fast-12   29.21k ± 0%       14.62k ± 0%       -49.94% (p=0.000 n=10)

Rodada de confirmação de misto.zip, base e atual intercalados (10 vezes cada):
Sintetico/ocr=fast-12  sec/op        3.874 ± 13%     3.558 ± 14%    ~ (p=0.123 n=10)
                       gc-pausa      19.60m ± 220%   17.24m ± 29%   ~ (p=0.684 n=10)
                       pico-mem-B    20.83Mi ± 11%   22.58Mi ± 18%  ~ (p=0.987 n=10)
                       procs/op      10.00 ± 0%      10.00 ± 0%     ~ (p=1.000 n=10)
                       B/op          18.16Mi ± 1%    18.19Mi ± 1%   ~ (p=0.912 n=10)
                       allocs/op     298.2k ± 0%     298.3k ± 0%    ~ (p=0.148 n=10)
```

### B-TE-07 — RunOCR: binário ausente, falha do pdftoppm e fallback do Tesseract sem teste, e TestRunOCR vira skip se a extração falhar

- **Arquivo:** `internal/ocr/tesseract.go:109`
- **Lente:** TE
- **Evidência:** tesseract_test.go:44-47: `if errInit != nil { t.Skipf(...) }`. Uma regressão em InitBinaries/extractFS (embed quebrado, %TEMP% sem permissão) deixa o pacote verde. Não há teste para tessPath/pdfToPpmPath ausentes (109-114), modo inválido caindo em `normal` (117-119), pdftoppm falhando com PDF inválido (151-154), PNG não encontrado (171-173) nem o fallback sem `-l por` (197-215). O único teste chama RunOCR com PDF válido e modo `fast`.
- **Proposta:** Trocar o Skipf por Fatalf, já que embed/ é versionado. Adicionar: RunOCR com PDF inexistente ou lixo exigindo o erro `embedded pdftoppm failed`; RunOCR com ctx cancelado exigindo erro sem processo órfão; e, com binDir injetável no teste, um diretório sem tesseract.exe exigindo a mensagem `not found`.
- **Custo:** M
- **Ganho esperado:** Os caminhos de erro do OCR passam a ser cobertos, e uma quebra na extração deixa de passar como skip.
- **Veredito:** rebaixado
- **Justificativa do cético:** O Skipf e o embed versionado foram confirmados, mas RunOCR já tem 83,2% de cobertura com o caminho feliz real; as lacunas são ramos de erro defensivos de baixo risco.

Status: feito

- **Commit:** `d9b91f7462753336b8cc363ea468ad8fc0a8d148`
- **Benchstat:**

```
Não rodei o benchmark. Nenhum pacote alterado pela tarefa aparece em go list -deps ./scripts/bench, que só lista o próprio eprocia/scripts/bench. A tarefa só mexeu em arquivos _test.go de internal/ocr.
```

### F-AC-09 — Foco se perde ao remover ou cancelar item e ao limpar a fila

- **Arquivo:** `frontend/src/components/ProcessList.svelte:201`
- **Lente:** AC
- **Evidência:** Os botões "Remover da lista" (201-207, 217-223) e "Cancelar" (177-191) somem com a linha ou com a mudança de status, que chega por `queue:update`. Nenhum código devolve o foco, que cai em `<body>`. O mesmo vale para "Limpar Fila" (102), que some quando `items.length` chega a 0 (90).
- **Proposta:** Antes de remover, guardar o índice do item e, no próximo `tick()`, focar o botão equivalente da linha seguinte ou anterior; sem vizinhos, focar o título da lista (`h2` com `tabindex="-1"`).
- **Custo:** M
- **Ganho esperado:** Quem usa teclado consegue limpar vários itens seguidos sem voltar ao início da página.
- **Veredito:** mantido
- **Justificativa do cético:** Os botões ficam em linhas `{#each ... (item.id)}` que somem com a atualização, e não há código que devolva o foco.

Status: feito

- **Commit:** `8719411b0a85e8db4fb7fd6468159cf09e17423f`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-AC-09+F-AC-10+F-DF-03/`](capturas/2026-10-15/depois-F-AC-09%2BF-AC-10%2BF-DF-03/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-AC-09%2BF-AC-10%2BF-DF-03/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### F-AC-10 — Mensagem completa de erro do item só fica acessível por title num div não focável

- **Arquivo:** `frontend/src/components/ProcessList.svelte:213`
- **Lente:** AC
- **Evidência:** `<div class="error-tooltip-container" title={item.message}>` (213) não tem tabindex nem role, e o texto visível do erro é cortado com `<span class="error-message truncate">` (162). O mesmo vale para o caminho completo (`title={item.path}`, 145) e para o indicador da sidebar recolhida (App.svelte:314, um div só com title).
- **Proposta:** Transformar o ícone de ajuda em `<button type="button" aria-label="Ver detalhes do erro">` que abre uma linha expandida ou popover com o texto completo, que pode ser copiado. No indicador recolhido, `role="status"` com texto `.sr-only`.
- **Custo:** M
- **Ganho esperado:** O diagnóstico de falhas fica acessível também a quem usa teclado.
- **Veredito:** rebaixado
- **Justificativa do cético:** O texto completo está no DOM, cortado só por CSS, então o leitor de tela lê tudo. Quem perde a mensagem é o usuário de teclado que enxerga, o que é bem menos grave.

Status: feito

- **Commit:** `8719411b0a85e8db4fb7fd6468159cf09e17423f`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-AC-09+F-AC-10+F-DF-03/`](capturas/2026-10-15/depois-F-AC-09%2BF-AC-10%2BF-DF-03/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-AC-09%2BF-AC-10%2BF-DF-03/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### F-AN-05 — O fly em cada linha da lista anima ao filtrar, e as outras linhas pulam

- **Arquivo:** `frontend/src/components/ProcessList.svelte:122`
- **Lente:** AN
- **Evidência:** `{#each filteredItems as item (item.id)}` aplica `transition:fly={{ y: 8, duration: 200 }}`. `filteredItems` é recalculado a cada tecla no filtro (17-20), então todas as linhas que entram ou saem fazem fly ao mesmo tempo. Sem `animate:flip`, as linhas que ficam saltam de posição enquanto as que saem ainda ocupam espaço por 200ms.
- **Proposta:** Só `in:fly` (8px, `--dur-base`, `--ease-out`) para itens novos de `queue:update`, remoção sem animação ou com `out:fade` de 150ms, e `animate:flip={{ duration: 150 }}`. Desligar a transição quando a mudança vier do filtro.
- **Custo:** M
- **Ganho esperado:** A lista fica estável ao filtrar, e o movimento sobra para o que é novo.
- **Veredito:** rebaixado
- **Justificativa do cético:** A parte do filtro é verdadeira. A alegação de que o histórico inteiro anima no boot é falsa: no Svelte 4.2.20 as transições são locais, e quando a lista sai de vazia é o bloco `{:else}` (116) que cria os itens, então o intro não roda.

Status: feito

- **Commit:** `42157c9afb1133dc710b84d29465e1bcaeecccd4`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-AN-05+F-AN-06/`](capturas/2026-10-15/depois-F-AN-05%2BF-AN-06/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-AN-05%2BF-AN-06/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### F-AN-06 — Ao recolher ou expandir, o conteúdo da sidebar aparece e some de repente enquanto a largura anima

- **Arquivo:** `frontend/src/App.svelte:289`
- **Lente:** AN (relacionado: F-DF-02)
- **Evidência:** A largura anima (`transition: width 0.15s`, 559), mas o conteúdo troca no mesmo frame por `{#if !sidebarCollapsed}` (277, 289, 325) e `{:else}` com `.collapsed-tools`. Ao expandir, Settings e o card manual de 280px ou mais aparecem dentro de uma coluna que ainda tem 48px e vazam ou se espremem durante 150ms (o aside tem `overflow: visible`, 558). Ao recolher, tudo some antes de a largura começar a diminuir.
- **Proposta:** `transition:fade={{ duration: 150 }}` no conteúdo (ou `in:fade` com delay igual a `--dur-fast` ao expandir) e `overflow: hidden` no `.sidebar-content` durante a animação; ou manter o conteúdo montado e animar só a opacidade, com `min-width` fixo e `overflow: hidden` no aside (o que também resolve F-DF-02). Respeitar tokens e reduced-motion.
- **Custo:** M
- **Ganho esperado:** Recolher e expandir contínuos, sem conteúdo espremido nem vazando.
- **Veredito:** rebaixado
- **Justificativa do cético:** O defeito existe, mas dura 150ms e só aparece ao clicar no botão de recolher.

Status: feito

- **Commit:** `42157c9afb1133dc710b84d29465e1bcaeecccd4`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-AN-05+F-AN-06/`](capturas/2026-10-15/depois-F-AN-05%2BF-AN-06/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-AN-05%2BF-AN-06/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

### F-VI-11 — Ícones copiados como SVG inline, com traço e versões inconsistentes e fora do mapeamento do DESIGN

- **Arquivo:** `frontend/src/components/ProcessList.svelte:135`
- **Lente:** VI
- **Evidência:** Os ícones de status usam `stroke-width="2.5"` (ProcessList.svelte:128, 135, 137), e os demais usam 2. O erro usa `lucide-alert-triangle` (137), e o DESIGN §4 pede `AlertCircle`. O sucesso usa o path antigo de `check-circle`. Não existe `Monitor` para a pasta monitorada nem `FileText`. A marca é uma estrela genérica (App.svelte:228). A lixeira aparece em duas variantes (`trash` em ProcessList.svelte:206, `trash-2` em Settings.svelte:120). São cerca de 25 SVGs colados à mão e nenhuma dependência `lucide-svelte`.
- **Proposta:** Adotar `@lucide/svelte` (após a migração para Svelte 5) com um componente `Icon` de padrão `size=16` e `stroke-width=1.75`, mapeando CircleCheck, LoaderCircle girando, CircleAlert, Clock, CircleSlash, CirclePause, FolderOpen, Trash2, Monitor e Settings. Trocar a estrela pela marca do app (skill eprocia-icone).
- **Custo:** M
- **Ganho esperado:** Traço uniforme e fino, como pede o DESIGN, menos marcação repetida e troca de ícone trivial.
- **Veredito:** rebaixado
- **Justificativa do cético:** É consistência fina de ícones e higiene de marcação, sem efeito no uso.

Status: feito

- **Commit:** `7625271fbb2d6db079d42f1f53221854e3ae3fb3`
- **Capturas:** antes em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/), depois em [`capturas/2026-10-15/depois-F-VI-11/`](capturas/2026-10-15/depois-F-VI-11/). Exemplo: [antes](capturas/2026-10-15/antes/1440x900-escuro-processando.png) e [depois](capturas/2026-10-15/depois-F-VI-11/1440x900-escuro-processando.png) em 1440x900 escuro, processando.

## Revisão final da branch

I found no blocking problems. `go vet ./...` printed nothing, and `go test ./... -count=1` passed in every package (eprocia, autostart, config, etl, ocr, watchdog, scripts/bench). None of the 15 commits adds a .zip, .pdf or processing .txt output.

Interaction between fixes:
- `frontend/src/components/Settings.svelte:66` (also `:51`): the error is built from `err.message`. Wails rejects with a plain string, so the toast reads "Erro ao salvar configurações: undefined". The new partial error from SaveConfig (B-RN-01, "configurações salvas, mas não foi possível alterar a inicialização…") is never shown. Using `err?.message ?? err`, as `App.svelte:144` already does, fixes it.
- `app.go:175-176`: `a.cfg = cfg` happens before the autostart call. If writing the registry fails, AutoStart=true is already in memory and on disk, so later saves don't retry (`cfg.AutoStart == autoAnterior`) and the switch stays on with nothing applied. Only a restart, through `sincronizarAutostart`, fixes it.
- `frontend/src/App.svelte:320`, `<Settings bind:config>` (F-DF-02): Settings now edits App's config object directly. When `SaveConfig` fails, the footer and the collapsed-sidebar indicator show values that were never saved. Settings is also still inside `{#if !sidebarCollapsed}`, so collapsing the sidebar still unmounts it and wipes its toast.
- `main.go:40` with `frontend/index.html` (F-VI-08): the native background follows the Windows theme. The page follows the choice saved in localStorage. A user who picked "escuro" on a light Windows, or the reverse, still sees the flash this fix was meant to remove.
- `internal/watchdog/watchdog.go:79` (B-RN-02): a ZIP that failed leaves no `.txt`, so it goes back in the queue on every app start for 24 h.

Regressions:
- `frontend/src/components/ProcessList.svelte:53`: if copying fails (B-RN-04), the error only goes to `console.error`. The user gets no message.
- `frontend/src/App.svelte:36,150`: `localStorage` is read and written without try/catch. The inline script in index.html does guard it, so if storage is blocked the component throws at mount.

Dead code:
- `internal/etl/formatter.go:123`: `CleanText` is now called only by tests; production uses `cleanPage`.
- `internal/etl/pdf_parser.go:97-114`: the branch that opens the PDF by file path in `abrir()` (from `NewPDFParser`) is now reached only by tests and `scripts/bench`. The pipeline always goes through `NewPDFParserDeLeitor`.
- `frontend/src/App.svelte:173-175`: blank lines left where the old handler was removed.

Messages not in pt-BR:
- `internal/etl/pdf_parser.go:93,101,106,112`: "failed to create pdf reader", "failed to open pdf", "failed to stat pdf". These were moved into `abrir()` by B-EF-03 and still read in English; they can end up in the output text when parsing fails.

## Revisão final da branch (segunda rodada)

Nenhum problema bloqueante. As verificações passaram: `go vet ./...` limpo, `go test ./... -count=1` todo ok, `svelte-check` com 0 erros e 0 avisos, e os 138 testes de `node --test frontend/*.test.mjs` passaram.

Nenhum zip, pdf ou saída .txt foi commitado. Os 368 arquivos binários são PNGs de captura em `docs/revisao/capturas/2026-10-15/`. A mudança em `models.ts` é a regenerada pelo Wails e casa com o campo `EstimatedTokens`. O `package.json.md5` passou a bater com o `package.json` atual. Não sobraram tokens CSS sem uso nem estilos glass ou de gradiente.

Achados menores:
- `internal/etl/pdf_parser.go:268` e `:291` (interação entre B-EF-04 e o progresso): com OCR em lote, as páginas pendentes só vão para o OCR depois que a leitura do texto nativo do documento inteiro termina. Antes, o OCR começava junto com a leitura. Além disso, o `onPageComplete` só dispara quando o bloco inteiro termina, então a barra de progresso fica parada e depois salta.
- `internal/etl/pdf_parser.go:218`: o OCR em lote grava `"image": ""`, enquanto o caminho de página única grava a imagem. As duas saídas ficam inconsistentes. O caminho de página única aponta para uma pasta temporária que já foi apagada, então nenhum dos dois serve.
- `internal/ocr/tesseract.go:362` (e `:217`, `:280`, `:319`): o erro reescrito "failed to find extracted image of page %d in run temp dir" continua em inglês, ao lado dos novos em português ("páginas repetidas no pedido de OCR", na `:329`). As mensagens do arquivo ficam misturadas.
- `internal/etl/formatter.go:326` e `pipeline.go:609`: o cálculo de tokens é feito duas vezes, uma no formatter (ponto fixo de até 4 rodadas) e outra no pipeline com `EstimateTokens(outputText)`. Os dois só batem se o ponto fixo convergir. Seria melhor o formatter devolver o número.
- `PRD.md:78-89`: a linha nova "Tokens estimados" no `<contexto>` não está no formato do §6. "Data de Consolidação" também não está, mas isso já vinha de antes da branch.

## Capturas

As capturas "antes" foram feitas com dados fictícios, em 800x600, 1024x768, 1440x900 e 1920x1080, nos temas claro e escuro, nos estados vazio, fila, processando, erro, configurações, processando com o painel recolhido e processando com o divisor no mínimo e no máximo. Ficam em [`capturas/2026-10-15/antes/`](capturas/2026-10-15/antes/). As citadas nos achados:

- [800x600-escuro-erro.png](capturas/2026-10-15/antes/800x600-escuro-erro.png): título da fila sobre o filtro (F-RS-01).
- [800x600-escuro-processando-divisor-maximo.png](capturas/2026-10-15/antes/800x600-escuro-processando-divisor-maximo.png): título em duas linhas e "Limpar Fila" cortado (F-RS-01, F-RS-02).
- [800x600-claro-configuracoes.png](capturas/2026-10-15/antes/800x600-claro-configuracoes.png): "0 itemns na / fila" e opções cortadas (F-VI-02, F-VI-07).
- [800x600-claro-processando-divisor-minimo.png](capturas/2026-10-15/antes/800x600-claro-processando-divisor-minimo.png): divisor parado perto de 296px e caminho cortado (F-AN-02, F-VI-07).
- [1024x768-claro-processando-divisor-maximo.png](capturas/2026-10-15/antes/1024x768-claro-processando-divisor-maximo.png): divisor longe do máximo (F-AN-02).
- [1440x900-escuro-processando.png](capturas/2026-10-15/antes/1440x900-escuro-processando.png): "4 itemns na fila", densidade da lista, cabeçalhos do painel e paleta (F-VI-02, F-VI-04, F-VI-06, F-VI-10).
- [1440x900-claro-erro.png](capturas/2026-10-15/antes/1440x900-claro-erro.png) e [1024x768-escuro-erro.png](capturas/2026-10-15/antes/1024x768-escuro-erro.png): estado pausado contraditório (F-VI-03).

## Teste manual guiado

As capturas rodam no navegador com os bindings do Wails simulados, então não cobrem o comportamento da janela nativa. Antes de aprovar os achados de RS e AN, e depois de corrigi-los, vale conferir no `wails dev` ou no executável de `build/bin/`:

1. **Bordas da janela frameless.** Redimensionar pelas quatro bordas e pelos quatro cantos até a largura mínima de 800px. O cursor de redimensionamento deve aparecer em toda a borda, a janela não pode ficar abaixo do mínimo e o cabeçalho da fila e o rodapé devem continuar em uma linha (F-RS-01, F-VI-02).
2. **Maximizar e restaurar.** Usar o botão da barra de título, o duplo clique na barra e Win+Seta para cima/baixo. O layout deve se reajustar sem salto, o divisor deve continuar dentro dos limites ao restaurar uma janela menor (F-RS-02) e o rótulo do botão deve refletir o estado (F-AC-07).
3. **Arrastar a barra de título.** Mover a janela entre monitores de escalas diferentes (100% e 150%, se houver) e arrastá-la para a borda superior para o snap. Nenhum botão da barra de título pode iniciar o arraste por engano.
4. **Divisor nos limites.** Arrastar rápido para além de 280px e de 480px, soltar o botão fora da janela e trocar de janela com Alt+Tab no meio do arrasto. O painel deve encostar exatamente no limite, acompanhar o cursor sem atraso e encerrar o arrasto (F-AN-02). Repetir pelo teclado: Tab até o divisor, setas, Home e End (F-AC-08).
5. **Recolher e expandir.** Clicar no botão de recolher várias vezes seguidas e observar se o conteúdo vaza ou pisca (F-AN-06) e se o card de configurações mostra "Carregando..." a cada expansão (F-DF-02).
6. **Tray e ocultação.** Fechar a janela (vai para o tray) e reabrir e sair pelo menu do tray várias vezes, inclusive durante um processamento (B-ES-02, B-ES-11).
7. **Tema na abertura.** Abrir o app com o Windows em tema escuro e em tema claro, observando se há flash de fundo claro antes da interface (F-VI-08).

## Derrubados

Ficam registrados para auditoria. Não entram na correção.

| Título | Lente | Justificativa do cético |
|--------|-------|-------------------------|
| Logs formatados por página no caminho quente (≈6 por página nativa, ≈13 por página com OCR) | B-EF | Os log.Printf por página existem, mas não há log.SetOutput, então a saída vai para o stderr do app GUI. Formatar poucas strings por página é desprezível diante do parse do PDF e mais ainda do OCR, e o próprio achado chama o ganho de marginal. |
| InitBinaries com sync.Once e globais binDir/initErr no lugar de sync.OnceValues | B-GM | binDir só é escrito dentro do Once e só é lido no return de InitBinaries; RunOCR declara um binDir local que esconde o global. O risco de ler binDir vazio não existe, e a troca é cosmética. (A troca do sync.Once por nova tentativa em caso de erro está em B-ES-04.) |
| Teste do watchdog não detecta callback disparado várias vezes para o mesmo ZIP nem arquivo que não estabiliza | B-TE | O dano alegado, o mesmo download enfileirado duas vezes, já é evitado por AddFiles (queue.go:85-95), que ignora caminhos pending ou processing. O ganho do teste é desprezível. |
| `bind:items` cria ligação bidirecional sem uso, e `transition:fly` global anima a lista inteira | F-DF | No Svelte 4.2.20 as transições já são locais por padrão, então a animação em massa alegada não ocorre. O `bind:items` sem uso tem custo desprezível. |

## Como aprovar

Escolha os ids e rode `/corrigir <ids>`, por exemplo `/corrigir B-ES-01 B-RN-01 B-ES-02`, ou `/corrigir todos`. Cada id vira um commit `fix(<camada>): <título> [<id>]` no branch `revisao/2026-10-15`, depois de passar pelos gates de build, benchmark e visual. Nada é enviado ao remoto nem mesclado.
