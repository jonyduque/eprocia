# Revisão do eprocIA — 2026-11-10

## Escopo

Revisão focada em desempenho do aplicativo e em melhoria de UI/UX, com acréscimo de efeitos visuais e ícones (escopo `tudo`). No backend (Go) rodou a lente EF (eficiência). No frontend (Svelte) rodaram DF (desempenho do frontend), VI (visual), AN (animação e movimento) e AC (acessibilidade e usabilidade). As telas foram capturadas com dados fictícios em quatro tamanhos (800x600, 1024x768, 1440x900 e 1920x1080), nos temas claro e escuro; as capturas ficam em [`capturas/2026-11-10/antes/`](capturas/2026-11-10/antes/).

Cada achado passou por um cético, que conferiu o código e, quando havia número, mediu de novo. Sobreviveram 57 achados e 5 foram derrubados; nenhum ficou sem verificação. Os achados que descreviam o mesmo problema no mesmo trecho foram fundidos (11 achados absorvidos em 10 fusões), e cada um cita as lentes que o encontraram e o que a versão fundida acrescentava. Ficaram **46 achados**: 5 no backend e 41 no frontend.

A ordem é gravidade × ganho ÷ custo (alta = 3, média = 2, baixa = 1; P = 1, M = 2, G = 3), com os empates resolvidos pelo ganho esperado. A gravidade e o ganho são os que ficaram depois do cético: onde o veredito é "rebaixado", a justificativa traz o número ou a ressalva que corrige o achado original. Os ids são numerados por lente, nessa ordem. Nada foi corrigido nesta etapa.

Dependências que vale conhecer antes de aprovar:

- **F-VI-01** é pré-requisito de qualquer mudança que aumente a altura do painel lateral (F-VI-02, F-VI-04, F-VI-07, F-AC-03, F-AC-13).
- **F-AN-03** (testes de movimento) convém entrar antes dos efeitos novos (F-AC-01, F-AN-08, F-AN-09, F-AN-11), para que já nasçam cobertos.
- **F-VI-02** (controle segmentado) fornece o componente que **F-AC-14** reaproveita; **F-VI-08** (aviso unificado) é citado por F-VI-01 e F-AC-07.
- **F-AC-07** e **F-AN-01** tocam o mesmo `alert()`; **F-AC-09** e **F-VI-04** precisam do mesmo ícone de monitoramento.
- **F-DF-03** e **F-DF-04** dependem da passagem dos componentes para runes, prevista como pré-requisito do `/corrigir`.
- **B-EF-05** só deve ser medido depois de **B-EF-02** e **B-EF-04**, que reduzem o mesmo trecho com custo menor.

## Resumo

| # | Id | Título | Gravidade | Custo | Ganho esperado | Veredito |
|---|----|--------|-----------|-------|----------------|----------|
| 1 | F-VI-01 | Em 800x600 o cartão de Configurações é cortado e esconde "Iniciar com o Windows" | alta | P | Todos os controles do painel alcançáveis em 800x600 | mantido |
| 2 | F-AC-01 | Copiar o TXT, a ação principal do app, dá retorno mínimo e nenhum anúncio | média | P | Confirmação clara de que o TXT foi copiado, com anúncio e destaque da ação principal | mantido |
| 3 | B-EF-01 | pdftoppm gasta cerca de 0,5 s por página codificando PNG que o tesseract só decodifica | média | P | ≈ −0,33 s de CPU por página que vai ao OCR (7% a 11% do tempo de OCR) | rebaixado |
| 4 | F-AC-02 | Texto colorido de 11px abaixo de 4,5:1 na fila e no painel de atividade | média | P | Textos de estado acima de 4,5:1 nos dois temas, com teste que impede a regressão | rebaixado |
| 5 | F-AC-03 | Pasta monitorada: lixeira que apaga a pasta com o rótulo "Desativar", campo somente leitura com cara de editável e interruptor ligado sem pasta | média | P | Ninguém perde a pasta configurada achando que só pausou; um vocabulário único de estado | mantido |
| 6 | B-EF-02 | linhasBasicas roda três vezes por página e realoca toda linha mesmo quando ela já está limpa | média | P | ≈ −40% do tempo do formatador no nível padrão; ≈ −290 mil alocações a cada 1000 páginas | mantido |
| 7 | F-AC-04 | Foco cai no body ao fechar ou confirmar o painel Reprocessar | média | P | Reprocessar utilizável por teclado do início ao fim, sem perder o foco | mantido |
| 8 | F-AC-05 | Regiões vivas: resumo da atividade em `role="status"` muda a cada 200 ms e avisos são montados já com conteúdo | média | P | Leitor de tela anuncia marcos (terminou, pausou, filtro vazio), não contadores repetidos | mantido |
| 9 | F-AC-06 | Limpar Fila cancela o item em andamento e apaga tudo com um clique, sem confirmar nem desfazer | média | M | Um clique errado ao lado do Pausar não derruba o processamento nem apaga a lista | mantido |
| 10 | F-VI-02 | Comboboxes nativos de OCR e Higienização: trocar por controle segmentado | média | M | As 4 opções visíveis, troca em 1 clique e o mesmo controle em Settings e Reprocessar | mantido |
| 11 | F-AN-01 | Soltar ZIP: overlay aparece para qualquer arrasto, arquivo errado abre alert() nativo e não há confirmação do que foi aceito | média | M | Retorno ao arrastar em três momentos (pode soltar, aceito, recusado), sem overlay indevido | mantido |
| 12 | F-DF-01 | Barras de progresso animam width (layout) de forma quase contínua, contra a regra de só animar transform/opacity | baixa | P | Progresso animado no compositor: de 8 a 10 ms para ≈1,3 ms por quadro com o painel de atividade aberto | mantido |
| 13 | F-AC-07 | Falhas de ações da fila vão só para o console, e o ZIP inválido usa `alert()` nativo | baixa | P | Toda ação que falha avisa na tela, no mesmo lugar; some o único diálogo nativo do app | mantido |
| 14 | F-VI-03 | Linha com erro não se destaca e traz dois ícones vermelhos circulados e dois expansores | baixa | P | Falhas saltam à vista numa fila longa; um só ponto vermelho e um só expansor por linha | mantido |
| 15 | F-AC-08 | Estado vazio e rodapé mandam procurar o controle em outro lugar | baixa | P | Primeiro uso resolvido onde o olho já está, com o painel aberto ou recolhido | mantido |
| 16 | F-AC-09 | Ponto de 6px como botão de monitoramento, com `aria-pressed` e rótulo que muda ao mesmo tempo | baixa | P | Monitoramento com cara de controle, estado lido pela forma e um só anúncio no leitor de tela | mantido |
| 17 | F-VI-04 | Inventário de ícones: 33 passam por Icone.svelte; rótulos do painel, filtro, botões de texto e um spinner ainda não | baixa | P | O mesmo ícone identifica a mesma configuração no painel, na barra recolhida e no reprocessar | mantido |
| 18 | F-AN-02 | Hover e estado pressionado variam de botão para botão | baixa | P | Todo botão responde do mesmo jeito ao ponteiro e ao clique; menos CSS repetido | mantido |
| 19 | F-AN-03 | Testes de movimento não cobrem AtividadeItem nem ReprocessarItem, onde entrarão efeitos novos | baixa | P | Efeitos novos desta rodada já nascem presos aos tokens e a prefers-reduced-motion | mantido |
| 20 | F-VI-05 | Botões da janela não encostam no canto e parecem botões de barra de ferramentas | baixa | P | Fechar, maximizar e minimizar no canto, com alvo maior | mantido |
| 21 | F-VI-06 | Barra de rolagem quase invisível e nenhuma pista de conteúdo rolado | baixa | P | Barra de rolagem visível e pista de que há conteúdo rolado | mantido |
| 22 | F-AN-04 | Limpar Fila e remover o último item não têm fade; o estado vazio aparece seco | baixa | P | A mesma saída com fade para um item ou para todos | mantido |
| 23 | F-AN-05 | Chip "Fila pausada" e indicador "N ativos" surgem sem transição; o chip desloca a lista inteira | baixa | P | Pausar e retomar sem sacudir a lista | mantido |
| 24 | F-AN-06 | Salvar de novo enquanto o toast de sucesso está visível não dá retorno novo | baixa | P | Cada salvamento tem um sinal próprio | mantido |
| 25 | B-EF-03 | Página sem operadores de texto é interpretada uma segunda vez pelo GetTextByRow | baixa | P | ≈ −40% de CPU na extração nativa de páginas sem texto (≈ −9% no benchmark sintético) | mantido |
| 26 | F-DF-02 | Todas as linhas da fila entram no layout a cada quadro do arrasto do divisor; não há content-visibility nem janela | baixa | P | Layout do arrasto do divisor proporcional às linhas visíveis (4x com 300 itens; nulo até 50) | mantido |
| 27 | F-AC-10 | Sem regras para o modo de alto contraste do Windows: interruptores, pontos de estado e barras são só `background-color` | baixa | P | Interruptores, níveis e progresso visíveis em tema de alto contraste do Windows | mantido |
| 28 | F-AC-11 | Semântica miúda: marcos, títulos, rótulo do divisor, ajuda sem vínculo e anel de foco igual ao botão primário | baixa | P | Marcos, títulos e ajuda coerentes; foco nítido no botão primário | mantido |
| 29 | F-AC-12 | "Remover da fila" no item pendente não remove: deixa uma linha "Cancelado" que pede outro clique | baixa | P | Rótulo e efeito alinhados; sem a linha "Cancelado" que o usuário não pediu | rebaixado |
| 30 | F-VI-07 | Painel lateral com 8px de padding, desalinhado dos 12px das barras | baixa | P | Linha-guia única de 12px e respiro entre campos e divisor | rebaixado |
| 31 | F-AN-07 | Ao recolher o painel, o botão salta cerca de 135 px no primeiro quadro e o chevron troca seco | baixa | P | Recolher e expandir simétricos, sem o botão saltar sob o ponteiro | rebaixado |
| 32 | F-AN-08 | Painéis dentro da linha (atividade, reprocessar, detalhes do erro) abrem e fecham sem transição coerente | baixa | P | Painéis da linha entram de modo coerente com o giro do chevron | rebaixado |
| 33 | F-AN-09 | Documentos do painel de atividade mudam de posição aos saltos, até 5 vezes por segundo | baixa | P | Dá para seguir um documento enquanto ele muda de estado | rebaixado |
| 34 | B-EF-04 | Regex executada em toda linha sem pré-filtro barato na limpeza | baixa | P | ≈ −20% do formatador no nível Máxima; ganho dentro do ruído no Padrão | rebaixado |
| 35 | F-AC-13 | Omitir tipos: campo de texto livre que exige lembrar os códigos e aceita qualquer coisa sem aviso | média | G | Escolher os tipos em vez de lembrar e digitar; nenhum código inválido salvo calado | rebaixado |
| 36 | F-AN-10 | Spinner, pulso e barras provavelmente seguem animando com a janela oculta na bandeja (inferência do código, falta medir) | baixa | M | Nenhuma renderização com o app na bandeja (≈23% de um núcleo durante o OCR, medido) | mantido |
| 37 | F-DF-03 | Cada queue:item refaz todas as linhas da fila e mede a posição de todas elas (animate:flip), não só a linha que mudou | baixa | M | Custo por flush de progresso deixa de crescer com a fila (12 ms com 100 itens, 24,6 ms com 300) | rebaixado |
| 38 | F-DF-04 | Painel de atividade reordena, clona e reavalia todos os documentos a cada 200 ms, e mantém no DOM todos os que aguardam | baixa | M | Custo por evento proporcional aos documentos que mudaram; painel abre rápido em ZIP grande (hoje 219 ms com 400 documentos) | rebaixado |
| 39 | B-EF-05 | Higienização e montagem da saída rodam em uma thread só depois da fase paralela | baixa | M | Formatação até 3,6x mais rápida na limpeza (menos de 0,3 s a cada 1000 páginas) | rebaixado |
| 40 | F-VI-08 | Toast do painel cobre o interruptor que o disparou e não tem ícone de estado | baixa | M | O aviso deixa de cobrir o controle, ganha ícone e vira um componente só | rebaixado |
| 41 | F-VI-09 | Linha da fila trunca o número do processo enquanto mensagem redundante e rótulos ocupam espaço; contadores podem usar ícones | baixa | M | Número do processo legível em 800x600; contadores com cerca de metade da largura | rebaixado |
| 42 | F-AN-11 | Conclusão e erro de um item não têm retorno: o ícone troca seco e a barra some antes de chegar ao fim | baixa | M | Término ou falha de um ZIP perceptível de relance | rebaixado |
| 43 | F-AN-12 | Troca de tema acontece em três velocidades (0, 150 e 220 ms) em vez de uma transição única | baixa | M | Troca de tema uniforme em 220 ms | rebaixado |
| 44 | F-AC-14 | Barra recolhida: OCR e higienização são botões de ciclo com três pontinhos, sem mostrar as opções | baixa | M | Qualquer opção a um clique e uma gravação, com nome e explicação à vista | rebaixado |
| 45 | F-AC-15 | Fila: cinco paradas de Tab por linha, sem setas nem atalhos | baixa | M | Fila longa navegável com poucas teclas | rebaixado |
| 46 | F-AC-16 | Nomes dos ícones só no `title`: não aparecem no foco por teclado e não dizem de qual processo são | baixa | M | Ícones se explicam também no foco por teclado; leitor de tela sabe de qual processo é cada botão | rebaixado |

## Achados

### F-VI-01 — Em 800x600 o cartão de Configurações é cortado e esconde "Iniciar com o Windows"

- **Arquivo:** `frontend/src/components/Settings.svelte:270`
- **Lente:** VI
- **Gravidade:** alta
- **Evidência:** Capturas 800x600-claro-configuracoes.png e 800x600-claro-erro.png: depois de "Omitir documentos dos tipos" vem direto "Importação Manual"; o interruptor "Iniciar com o Windows" não aparece. Em 800x600-claro-processando-divisor-minimo.png (painel a 280px) até o texto de ajuda é cortado no meio ("...vira um aviso no texto"). Causa no código: `.settings-card { ... position: relative; overflow: hidden; }` (Settings.svelte:264-271) é item flex de `.sidebar-content` (`display: flex; flex-direction: column; ... overflow-y: auto`, App.svelte:893-899). Com overflow hidden o min-height automático vira 0, então o cartão encolhe e corta o conteúdo em vez de o contêiner rolar. 600px é altura suportada (`MinHeight: 600`, main.go:54).
- **Proposta:** `flex: none` em `.settings-card` e `.manual-card`, para a rolagem acontecer em `.sidebar-content`. Tirar o toast de dentro do cartão (ver F-VI-08) elimina a necessidade do `overflow: hidden`. Opcional: deixar "Importação Manual" fixa no pé do painel, fora da área rolável, para a ação não depender de rolagem. Acrescentar em layout.test.mjs uma verificação de que as seções do painel não encolhem.
- **Custo:** P
- **Ganho esperado:** Todos os controles do painel ficam alcançáveis no tamanho mínimo da janela; pré-requisito para qualquer redesenho da barra lateral que aumente a altura.
- **Veredito:** mantido
- **Justificativa do cético:** Confirmado em Settings.svelte:264-271 (`overflow: hidden` num item flex de `.sidebar-content`, App.svelte:893-899) e nas capturas 800x600-claro-configuracoes.png e 800x600-claro-processando-divisor-minimo.png: "Iniciar com o Windows" some e a ajuda é cortada no meio, sem rolagem (a seção "Importação Manual" continua visível logo abaixo). O conteúdo precisa de cerca de 650px de altura e main.go:54 aceita 600; não há outro caminho para esse interruptor e layout.test.mjs não cobre o caso.
- **Nota da síntese:** O toast dentro do cartão, que motiva o `overflow: hidden`, é tratado em F-VI-08; o `flex: none` resolve o corte sem depender dele.

Status: pendente

### F-AC-01 — Copiar o TXT, a ação principal do app, dá retorno mínimo e nenhum anúncio

- **Arquivo:** `frontend/src/components/ProcessList.svelte:425`
- **Lentes:** VI, AN, AC
- **Gravidade:** média
- **Evidência:** `handleCopy` (linhas 192-202) só define `copiadoId` por 1,5 s. O efeito visível é a troca do ícone `Copy` por `Check`, 14px, na mesma cor `--text-secondary` (linhas 432-436), e o `title` virar "Copiado!" (linha 429), que só aparece se o ponteiro ficar parado. O `aria-label` continua "Copiar o TXT" e não há região `role="status"` na lista (a única região viva é a `role="alert"` de erro, linha 531): leitor de tela não recebe confirmação, o que foge da regra de anúncios do DESIGN.md §2. Além disso o botão é o segundo de quatro ícones cinza idênticos (ordem no DOM: reprocessar, copiar, abrir, remover; linhas 414-453; ver 1024x768-claro-processando-recolhido.png), sem destaque para a ação que é o objetivo do produto.
- **Proposta:** 1. Retorno visível: durante 1,5 s o botão mostra `Check` em `--success` e o texto "Copiado" em `--success-strong` (`--fs-meta`) ao lado, entrando com `fade` em `--dur-fast`; pode ocupar o lugar do selo de contadores para não mudar a altura da linha. 2. Anúncio: uma região `.sr-only role="status" aria-live="polite"` sempre montada em ProcessList recebe "TXT de {nome} copiado". 3. Hierarquia: Copiar passa a ser o primeiro botão da linha concluída e o único em `--accent`; Reprocessar vai para depois de Abrir pasta. 4. Atalho: com o foco na linha, Ctrl+C copia o TXT. Ajustar `escolherAcao` e fila.test.mjs se a ordem mudar.
- **Custo:** P
- **Ganho esperado:** O usuário tem certeza de que o texto está na área de transferência antes de trocar para a IA, com ou sem leitor de tela, e acha a ação principal sem ler quatro dicas.
- **Veredito:** mantido
- **Justificativa do cético:** handleCopy (ProcessList.svelte:192-202) só define copiadoId; o retorno é a troca Copy/Check de 14px na mesma cor de .btn-action e o title, com aria-label fixo e nenhuma região role="status" para o sucesso (a role="alert" da linha 531 só recebe erro), o que foge da regra de anúncios do DESIGN.md §2 na ação principal. O retorno visual existe (troca de ícone por 1,5 s), mas a falta de anúncio é real e o custo é P.
- **Nota da síntese:** Fundido com o achado de AN "Copiar o TXT, a ação principal do item concluído, dá retorno quase invisível" (média, P, mantido) e com o de VI "Copiar o TXT, a ação principal, não tem destaque nem retorno visível" (baixa, P, rebaixado), todos sobre ProcessList.svelte:425-436. Complementos: AN propõe envolver o ícone em `{#key copiadoId === item.id}` com `in:scale` (início em 0,6, `duracaoMovimento(DUR_FAST)`, `cubicOut`) e fundo `--success-soft` no botão enquanto dura, aproveitando a transição de `background-color` que `.btn-action` já tem; VI propõe a classe `copiado` no botão e, com a fila larga (container acima de cerca de 640px), o rótulo "Copiar" em `--fs-meta`. Ressalvas dos céticos de AN e VI: o retorno visual existe (troca de ícone por 1,5 s e `title` "Copiado!"; com o ponteiro em cima a cor é `--text-primary`), então o que falta é cor, movimento e o anúncio.

Status: pendente

### B-EF-01 — pdftoppm gasta cerca de 0,5 s por página codificando PNG que o tesseract só decodifica

- **Arquivo:** `internal/ocr/tesseract.go:242`
- **Lente:** EF
- **Gravidade:** média
- **Evidência:** `renderizar` chama o pdftoppm com `"-png", "-r", "150"` (tesseract.go:239-246) e `ocrPaginas` só aceita o sufixo `.png` (tesseract.go:436). Medição com os binários de %TEMP%\eprocIA_bin sobre um PDF escaneado fictício de 12 páginas (40 linhas cada, gerado com scripts/bench/pdfgen; script scratchpad da sessão, ef/ocr/medir3.sh), 6 repetições, um pdftoppm para as 12 páginas: `-png` 6479-8703 ms (mediana ~7,5 s, 0,63 s/página); `-gray` sem `-png` (PGM) 1162-2189 ms (0,11 s/página); sem `-png` (PPM colorido) 1720-3064 ms (0,16 s/página). O tesseract levou o mesmo tempo nas três entradas (6 páginas: PNG 21,4-26,1 s, PGM 22,6-22,8 s, dentro do ruído) e devolveu texto idêntico (`cmp` das saídas: PNG == PGM em 6 páginas, PNG == PPM em 2), desde que receba `--dpi 150`; sem ele, avisa `Estimating resolution as 212`, porque PNM não guarda a resolução. A máquina estava com outros agentes rodando, então os valores absolutos estão inflados; a razão entre os formatos se manteve nas 6 rodadas.
- **Proposta:** Renderizar em PNM: trocar `-png` por `-gray` (saída `p-N.pgm`), aceitar o sufixo novo na leitura da pasta temporária (tesseract.go:435-436) e passar `--dpi 150` ao tesseract em `reconhecer` (tesseract.go:349/362). Custo colateral a aceitar: cada página passa a ocupar 2,2 MB no temporário (52 KB em PNG), apagado ao fim do bloco. Como a conversão para cinza passa a ser do Poppler, validar com `TestFixtures_LegibleToOCR` e com o corpus real (carimbos e destaques coloridos); se o texto mudar, usar PPM colorido, que tem os mesmos pixels do PNG (6,5 MB por página, ainda 4x mais rápido que PNG).
- **Custo:** P
- **Ganho esperado:** Cerca de -0,5 s de CPU por página que vai ao OCR: -80% do tempo do pdftoppm e, no modo fast, cerca de -11% do tempo de OCR por página (0,63 s de 4,5 s medidos). Num ZIP com 500 páginas escaneadas, perto de 250 s de CPU a menos, uns 20 s de relógio com 11 workers.
- **Veredito:** rebaixado
- **Justificativa do cético:** Confirmei `-png -r 150` em internal/ocr/tesseract.go:242 e o sufixo `.png` na linha 436, e medi de novo com os binários de %TEMP%\eprocIA_bin sobre o esc12.pdf (6 rodadas, 12 páginas): PNG 4,4-5,2 s, PGM 0,80-0,94 s, PPM 1,2-1,9 s; o tesseract devolveu texto idêntico (`cmp`) com `--dpi 150` nos modos fast e normal. A economia medida é de ~0,33 s por página (não 0,5 s), o que dá ~11% no fast mas ~7% no modo padrão do app (`normal`, ~4,4 s/página em config.go:41), e o temporário cresce de 52 KB para 2,2 MB por página com todas as páginas do bloco renderizadas antes do tesseract: ganho real de custo P, mas gravidade média.

Status: pendente

### F-AC-02 — Texto colorido de 11px abaixo de 4,5:1 na fila e no painel de atividade

- **Arquivo:** `frontend/src/components/ProcessList.svelte:895`
- **Lentes:** VI, AC
- **Gravidade:** média
- **Evidência:**
  O DESIGN.md §2 reserva `--success`, `--error` e `--accent` para ícones e manda texto usar as variantes `-strong`, mas vários textos de 11px usam os tokens base. Calculei os pares com os hex de style.css (mesma fórmula de frontend/contraste.test.mjs):
  - `.meta-badge.success { color: var(--success) }` (ProcessList.svelte:895-898, o selo "57 docs | 142 pág | ~98.450 tokens"): claro 2,68:1 (2,44 com a linha em hover); escuro 4,30:1.
  - `.queue-chip { color: var(--warning) }` (ProcessList.svelte:636, "Fila pausada..."): claro 2,81:1. Visível em 1024x768-claro-erro.png.
  - `.paused-message { color: var(--warning) }` (ProcessList.svelte:931): claro 3,19:1.
  - `.error-message { color: var(--error) }` (ProcessList.svelte:944): escuro 3,68:1 (3,34 em hover); claro 4,53 e 4,07 em hover.
  - `.progress-message { color: var(--accent) }` (ProcessList.svelte:908): escuro 4,19:1.
  - AtividadeItem.svelte: `.chip.ok` (220) claro 2,97:1; `.chip.ocr`/`.doc-ocr` (215, 290) claro 3,78 e 3,34 sobre a linha destacada; `.chip.erro` (225) 3,72 a 3,83 nos dois temas; `.doc-erro` (296) 4,14 a 4,30.
  - App.svelte:780 `.pulse-indicator` ("1 ativo"): escuro 3,72:1.
  O contraste.test.mjs só confere `--success-strong`/`--error-strong` sobre `--surface` e `--bg-main` (linhas 71-80); nenhum desses seletores é testado.
- **Proposta:** Trocar a cor de texto nesses seletores pelos tokens que já existem: `--success-strong`, `--error-strong` e `--accent-text` (conferi: escuro `--error-strong` dá 5,99:1 sobre `--surface-hover`; `--accent-text` 6,27 no escuro e 6,68 no claro). Criar `--warning-strong` só para o tema claro (por exemplo `#9A4A00`, 5,52:1 sobre `--warning-soft`; `#B45309` fica em 4,42 e não serve). Atenção ao selo verde no claro: `--success-strong` sobre `--success-soft` dá 4,39:1, então o fundo precisa cair para `--success-faint` ou superfície neutra. Manter `--success`/`--error`/`--warning` nos ícones e bordas. Acrescentar em contraste.test.mjs os pares por seletor, incluindo o fundo `--surface-hover` e os fundos `color-mix`.
- **Custo:** P
- **Ganho esperado:** Os contadores de documentos, páginas e tokens, as mensagens de erro e o aviso de fila pausada passam de 4,5:1 nos dois temas, inclusive com a linha sob o ponteiro, e o teste impede a regressão.
- **Veredito:** rebaixado
- **Justificativa do cético:** Recalculei todos os pares com os hex de frontend/src/assets/css/style.css (fundos color-mix compostos sobre --surface/--surface-hover/--bg-main) e os números batem: .meta-badge.success 2,68/2,44 claro e 4,30 escuro; .queue-chip 2,81; .paused-message 3,19; .error-message 3,68/3,34 escuro; .progress-message 4,19; .chip.ok 2,97; .pulse-indicator 3,72; os seletores usam mesmo os tokens base (ProcessList.svelte:895-946, AtividadeItem.svelte:215-299, App.svelte:780) e o contraste.test.mjs não os cobre. É violação real da regra de 4,5:1 do DESIGN.md §2, mas o texto segue legível nas capturas e nada fica bloqueado: media, não alta.
- **Nota da síntese:** Fundido com o achado de VI "Texto de estado usa tokens de preenchimento (--success, --error, --warning, --processing) em vez das variantes de texto" (média, P, rebaixado), que aponta os mesmos seletores e as mesmas capturas (1024x768-claro-fila.png, 1024x768-escuro-erro.png, 1440x900-claro-processando-atividade.png). Divergência a resolver na correção: VI sugere `--warning-strong` `#9A5B00` (5,43:1 sobre branco e 4,78:1 sobre `--warning-soft`) e AC sugere `#9A4A00` (5,52:1 sobre `--warning-soft`); as duas passam de 4,5:1, basta escolher uma e registrar o par em contraste.test.mjs e na §2 do DESIGN.md. As duas lentes concordam que o selo de sucesso precisa do fundo `--success-faint`.

Status: pendente

### F-AC-03 — Pasta monitorada: lixeira que apaga a pasta com o rótulo "Desativar", campo somente leitura com cara de editável e interruptor ligado sem pasta

- **Arquivo:** `frontend/src/components/Settings.svelte:163`
- **Lentes:** VI, AC
- **Gravidade:** média
- **Evidência:**
  1. Settings.svelte:163 tem `title="Desativar monitoramento"` e ícone `Trash2`, mas `handleClearDirectory` (linhas 87-90) faz `config.monitored_path = ''` e salva: esquece a pasta, sem confirmar e sem desfazer; para voltar é preciso navegar de novo até ela no diálogo. Pausar já existe duas linhas acima (interruptor "Monitoramento ativo", 134-145) e no ponto do rodapé. O botão não tem `aria-label`.
  2. Três vocabulários para o mesmo assunto: "Monitoramento ativo" (135), "Desativar monitoramento" (163), "Nenhuma pasta (inativo)" (155), "Monitoramento pausado" e "Monitoramento inativo" (App.svelte:624 e 627).
  3. O caminho é um `<input type="text" readonly>` (151-158) com o mesmo estilo do campo editável de tipos (800x600-claro-configuracoes.png): entra na ordem de Tab, convida a digitar ou colar e não reage; o texto "Nenhuma pasta (inativo)" é o `value`. O caminho cortado só aparece inteiro no `title`.
  4. Sem pasta, o interruptor "Monitoramento ativo" pode ficar ligado (padrão `watchdog_enabled: true`, linha 24) enquanto o rodapé diz "Monitoramento inativo".
- **Proposta:** Mostrar a pasta como texto (não input), com reticências no meio para preservar o fim do caminho e a dica com o caminho inteiro; "Alterar" vira "Escolher pasta..." quando não há nenhuma, com ícone `FolderOpen`. Tirar a lixeira: pausar fica com o interruptor, e esquecer a pasta, se continuar existindo, vira um botão de texto "Esquecer pasta" (ícone `X`, não lixeira) com rótulo honesto. Sem pasta, o interruptor aparece desligado e desabilitado, com a ajuda "Escolha uma pasta para monitorar" ligada por `aria-describedby`. Fixar dois estados e dois nomes em todo o app: "Monitorando" e "Pausado" (mais "Sem pasta"), no painel, na barra recolhida e no rodapé.
- **Custo:** P
- **Ganho esperado:** Ninguém perde a pasta configurada achando que estava só pausando; o painel e o rodapé passam a dizer a mesma coisa com as mesmas palavras.
- **Veredito:** mantido
- **Justificativa do cético:** Conferido: Settings.svelte:163 tem title "Desativar monitoramento" com Trash2 e handleClearDirectory (87-90) apaga monitored_path e salva sem confirmar; o caminho é input readonly com o estilo do campo editável (151-158, visível na captura); os rótulos divergem entre Settings e App.svelte:624-627; e, após a lixeira, watchdog_enabled segue true com o rodapé dizendo inativo. Ressalva: o DefaultConfig do Go já vem com a pasta Downloads, então o estado "ligado sem pasta" só aparece depois de limpar a pasta.
- **Nota da síntese:** Fundido com o achado de VI "Pasta monitorada: o botão "Alterar" é o elemento mais forte da janela e o caminho esconde a pasta final" (baixa, M, rebaixado), sobre a mesma linha do formulário (Settings.svelte:151-163). Complementos de VI: "Alterar" como botão secundário (borda `--border-strong`, `FolderOpen` 12px), azul cheio só em "Escolher pasta..." quando não há pasta, ícone `Folder` 14px antes do caminho e função de truncamento em `src/lib` com teste. Divergência a resolver: AC propõe reticências no meio do caminho e VI propõe cortar o começo; nos dois casos a reticência no fim é decisão registrada no DESIGN.md:104, que precisa ser atualizado junto. Ressalva do cético de VI: o caminho completo já aparece no rodapé e no `title`, e os interruptores usam o mesmo azul cheio de "Alterar".

Status: pendente

### B-EF-02 — linhasBasicas roda três vezes por página e realoca toda linha mesmo quando ela já está limpa

- **Arquivo:** `internal/etl/higienizacao.go:46`
- **Lente:** EF
- **Gravidade:** média
- **Evidência:** Nos níveis standard e aggressive cada página passa por `linhasBasicas` em `chaveConteudo` (formatter.go:275, pré-passada H7), em `edgeLines` (formatter.go:88, via `repeatedEdgeLines`) e em `cleanPage` (formatter.go:140). A função faz, por linha, `strings.Map` com closure + `strings.Fields` + `strings.Join` (higienizacao.go:49-58). `BenchmarkEFPartes/linhasBasicas` (overlay, -count=6, 1000 páginas de 45 linhas): 119-171 ms, 14,7 MB e 97.000 alocações por passada; três passadas somam ~330-390 ms dos 680-853 ms que `GenerateConsolidatedOutput` leva no nível standard. No perfil do `BenchmarkSintetico`, `linhasBasicas` soma 0,83 s dos 4,55 s do formatador. Protótipo de caminho rápido no arquivo de teste scratchpad da sessão, ef/zz_ef6_test.go (a linha sem controle, tab, NBSP, espaço dobrado ou espaço nas pontas é reaproveitada sem cópia; saída conferida igual com `slices.Equal` nas 1000 páginas e numa linha suja): `BenchmarkEFLinhasBasicas` atual 103-117 ms / 97.000 allocs contra caminho-rapido 32-47 ms / 1.000 allocs / 0,77 MB.
- **Proposta:** (1) Calcular as linhas básicas uma vez por página e reaproveitar o resultado nas três etapas: `documentosOrdenados` (ou a pré-passada H7) guarda `[]string` por página, `chaveConteudo`, `edgeLines` e `cleanPage` passam a receber as linhas prontas. (2) Em `linhasBasicas`, testar antes se a linha já está limpa e, nesse caso, usar a própria fatia do texto, sem Map/Fields/Join; pré-dimensionar `out` com `strings.Count(text, "\n")+1`.
- **Custo:** P
- **Ganho esperado:** De ~330 ms para ~35 ms a cada 1000 páginas nessa função (-90%), o que dá cerca de -35% a -40% do tempo do formatador no nível padrão, e ~-290.000 alocações / ~-43 MB alocados a cada 1000 páginas.
- **Veredito:** mantido
- **Justificativa do cético:** Confirmei as três chamadas de `linhasBasicas` por página (formatter.go:88, 140 e 275) e o Map+Fields+Join por linha (higienizacao.go:49-58); no meu benchmark cada passada custa 63-108 ms e 97.000 alocações por 1000 páginas, contra 384-444 ms do formatador no nível padrão, e o caminho rápido do protótipo fica em 27-31 ms com 1.000 alocações e saída igual. O ganho relativo alegado (~-40% do formatador) se confirma; em valor absoluto são ~0,2 s por 1000 páginas, sem efeito em ZIP dominado por OCR.

Status: pendente

### F-AC-04 — Foco cai no body ao fechar ou confirmar o painel Reprocessar

- **Arquivo:** `frontend/src/components/ProcessList.svelte:520`
- **Lente:** AC
- **Gravidade:** média
- **Evidência:** ReprocessarItem.svelte:18 leva o foco para o primeiro select ao montar. Ao sair, nada devolve o foco: `aoCancelar={() => alternarReprocessar(item.id)}` (ProcessList.svelte:520) só inverte o mapa (linhas 104-106), e Esc (ReprocessarItem.svelte:30-35) e o botão Cancelar (linha 57) passam por aí. O `{#if reprocessarAberto[item.id] ...}` (linha 514) desmonta o formulário com o foco dentro dele. Em `confirmarReprocessar` (linhas 108-118) acontece o mesmo: o painel fecha, o item vira `pending` e os botões da linha trocam, sem `guardarFoco`. O mecanismo `guardarFoco`/`devolverFoco` (linhas 126-169) só é chamado por cancelar, remover e limpar. Contraria "Foco não se perde" do DESIGN.md §2: quem usa teclado volta ao começo da janela.
- **Proposta:** Ao cancelar ou apertar Esc, focar o botão `[data-acao="reprocessar"]` da própria linha depois de `tick()`. Ao confirmar, chamar `guardarFoco(item.id, ...)` com a ação `cancelar` como preferida, para o foco ir ao X do item que voltou para a fila (o `escolherAcao` já cobre a troca de conjunto de botões). Acrescentar um caso em fila.test.mjs. Em Svelte 5 isso cabe num attachment do painel, que guarda quem abriu e devolve o foco no desmonte, e serve também para o menu da barra recolhida.
- **Custo:** P
- **Ganho esperado:** Reprocessar vira um fluxo completo por teclado: abrir, escolher, confirmar ou desistir e continuar na mesma linha.
- **Veredito:** mantido
- **Justificativa do cético:** ReprocessarItem.svelte:18 foca o select ao montar; alternarReprocessar (ProcessList.svelte:104-106) e confirmarReprocessar (108-118) só mexem no mapa e o {#if} da linha 514 desmonta o form com o foco dentro. Os únicos .focus() do componente estão em devolverFoco, que só roda com focoPendente (cancelar, remover, limpar), então o foco cai no body, contra "Foco não se perde" (DESIGN.md:22).

Status: pendente

### F-AC-05 — Regiões vivas: resumo da atividade em `role="status"` muda a cada 200 ms e avisos são montados já com conteúdo

- **Arquivo:** `frontend/src/components/AtividadeItem.svelte:69`
- **Lente:** AC
- **Gravidade:** média
- **Evidência:** AtividadeItem.svelte:69 marca `<div class="resumo font-tabular" role="status">`, cujo conteúdo são os contadores "N em andamento", "N pág. em OCR", "N aguardando", "N concluídos" (linhas 70-80) e ainda o botão "Mostrar concluídos" (82-90). Esses números vêm de `queue:atividade`, emitido a cada 200 ms no máximo (CLAUDE.md, seção de arquitetura; App.svelte:138-140). `role="status"` é região viva com leitura do conjunto: com o painel aberto, o leitor de tela relê a faixa inteira continuamente durante todo o processamento. No sentido oposto, dois avisos nascem junto com o elemento que os anuncia, contra a regra "regiões vivas sempre montadas" do DESIGN.md §2: `{#if aviso} ... <span class="queue-chip" role="status">` (ProcessList.svelte:285-292) e `<p role="status">Nenhum item corresponde ao filtro</p>` dentro de `{#if}` (linhas 295-298). O anuncios.test.mjs:101 só confere que o texto fica dentro de um `role="status"`.
- **Proposta:** Tirar o `role="status"` do resumo (vira texto comum dentro da região "Atividade dos documentos") e mover o botão para fora dele. Anunciar só marcos: fim do item ("{nome}: concluído, 57 documentos" ou "erro") e mudança no número de documentos com erro, numa única região `.sr-only role="status"` sempre montada no ProcessList. Passar por ela também "Fila pausada"/"Fila retomada", o resultado do filtro ("3 de 12 itens", "Nenhum item corresponde ao filtro") e o "TXT copiado"; o chip e o parágrafo continuam visíveis, sem `role`. Atualizar anuncios.test.mjs para a nova região.
- **Custo:** P
- **Ganho esperado:** O leitor de tela para de repetir contadores e passa a dizer o que importa: o processo terminou, a fila pausou, o filtro não achou nada.
- **Veredito:** mantido
- **Justificativa do cético:** AtividadeItem.svelte:69 marca como role="status" (atômica por padrão) a faixa de contadores alimentada por queue:atividade (App.svelte:138-140) e ainda contém o botão "Mostrar concluídos" (82-90); o chip de fila pausada (ProcessList.svelte:285-292) e o aviso do filtro (295-298) nascem dentro de {#if} já com o conteúdo, contra "regiões vivas sempre montadas" (DESIGN.md:21). Os contadores mudam por documento/bloco de páginas, não a cada 200 ms cravados, mas o excesso de leitura com o painel aberto é real.

Status: pendente

### F-AC-06 — Limpar Fila cancela o item em andamento e apaga tudo com um clique, sem confirmar nem desfazer

- **Arquivo:** `frontend/src/components/ProcessList.svelte:277`
- **Lente:** AC
- **Gravidade:** média
- **Evidência:** ProcessList.svelte:277 chama `handleClear` (linhas 240-248), que vai direto a `ClearQueue()`. No Go, `Queue.Clear` (internal/etl/queue.go:379-389) cancela o item em processamento e zera a lista ("cancels all running/pending tasks and clears the queue list"). O botão fica a 6px do Pausar (`.header-actions { gap: 6px }`, linha 666; ver 1024x768-claro-erro.png) e apaga também os itens escondidos pelo filtro. Some junto o único lugar de onde se copia o TXT de cada processo e o histórico da sessão; um OCR de vários minutos é perdido. O X de cancelar o item em processamento (linhas 395-402) também age sem confirmar. Não existe ação intermediária: para tirar só os concluídos é preciso remover linha por linha.
- **Proposta:** Separar em duas ações. "Limpar concluídos" (ícone `ListX`) remove só sucesso/erro/cancelado e não pede confirmação. Quando houver item pendente ou em processamento, "Limpar tudo" abre uma confirmação em linha na faixa do `.queue-notice`, com o texto exato do efeito ("Cancelar 1 em andamento e remover 5 itens?") e os botões Cancelar / Limpar tudo; sem prazo para sumir (WCAG 2.2.1), Esc cancela e o foco volta ao botão. O cancelamento de item em processamento pode seguir de um clique, porque o Reprocessar recupera. A faixa entra com `fade` em `--dur-fast`. O teste de foco pós-limpeza em fila.test.mjs continua valendo para o caminho confirmado.
- **Custo:** M
- **Ganho esperado:** Um clique errado ao lado do Pausar deixa de derrubar o processamento e apagar a lista; arrumar a fila sem perder os pendentes passa a ser uma ação só.
- **Veredito:** mantido
- **Justificativa do cético:** handleClear (ProcessList.svelte:240-248) chama ClearQueue sem confirmar; App.ClearQueue (app.go:449) vai direto a Queue.Clear (queue.go:379-389), que cancela o item em processamento e zera a lista, inclusive os itens escondidos pelo filtro; o botão fica a 6px do Pausar (linha 666). Ressalva: os TXT já gerados continuam no disco, só o item em andamento e o histórico da sessão se perdem, o que cabe em media.

Status: pendente

### F-VI-02 — Comboboxes nativos de OCR e Higienização: trocar por controle segmentado

- **Arquivo:** `frontend/src/components/Settings.svelte:173`
- **Lentes:** VI, AC
- **Gravidade:** média
- **Evidência:** `<select id="ocr-mode" bind:value={config.ocr_mode} on:change={handleSave}>` (Settings.svelte:173-182) e `<select id="cleaning-level" ...>` (189-198); mais dois em ReprocessarItem.svelte:42 e 50. Nas capturas 1024x768-claro-configuracoes.png e 1024x768-claro-fila-reprocessar.png aparecem como select padrão do Chromium (seta nativa, lista do sistema). docs/backlog.md:12: "tirar os comboboxes. Eles são feios, e o usuário só vê as opções depois de abrir". As opções são 4 níveis curtos e ordenados (frontend/src/lib/atividade.js:4-16), que a barra recolhida já desenha como escala de pontos (App.svelte:549-553). Settings repete as opções à mão em vez de usar OPCOES_OCR/OPCOES_LIMPEZA.
- **Proposta:** Componente `Segmentado.svelte` (role="radiogroup" com inputs radio ocultos, setas do teclado nativas): faixa de 28px com 4 segmentos iguais, contorno 1px `--border-strong`, raio 4px, divisórias 1px `--border`. Selecionado: fundo `--accent-soft`, texto `--accent-text` em 600, troca de fundo em `--dur-fast`; anel de foco no segmento. Rótulos em `--fs-meta` para caber a 280px (4 x 64px; "Desativado" é o mais longo). O `.help-text` da opção escolhida continua abaixo. Usar o mesmo componente no ReprocessarItem. Atualizar DESIGN.md §5.3 e os testes que leem o select (limpeza-niveis.test.mjs:10, tema.test.mjs:128, contraste.test.mjs:120). Com Svelte 5, um componente com valor `$bindable` e `{#each}` sobre OPCOES_* atende os 4 usos.
- **Custo:** M
- **Ganho esperado:** As 4 opções ficam visíveis sem abrir nada, a troca cai de 2 cliques para 1, a altura do painel não muda e o painel expandido passa a falar a mesma língua da escala de pontos da barra recolhida.
- **Veredito:** mantido
- **Justificativa do cético:** Os quatro `<select>` existem (Settings.svelte:173-198, ReprocessarItem.svelte:42 e 50), Settings repete as opções à mão em vez de usar OPCOES_OCR/OPCOES_LIMPEZA (lib/atividade.js:4-16) e docs/backlog.md:12 traz o pedido do dono. Os testes citados existem (limpeza-niveis:10, tema:128, contraste:120) e 4 segmentos cabem em 264px úteis a 280px.
- **Nota da síntese:** Fundido com o achado de AC "Selects de OCR e higienização escondem as opções e só explicam a que está escolhida" (média, M, mantido). Complementos de AC: o `.help-text` só descreve a opção atual (`AJUDA_OCR[config.ocr_mode]`, `AJUDA_LIMPEZA[config.cleaning_level]`) e cada seleção já grava, então comparar "Padrão" com "Máxima" exige gravar as duas; no ReprocessarItem não há ajuda nenhuma. AC propõe `<fieldset>` com `<legend>` e rádios nativos em `.sr-only` (setas, Home/End e "2 de 4" sem ARIA manual), ajuda que descreve a opção sob o ponteiro ou em foco e volta para a escolhida ao sair (ligada por `aria-describedby`), indicador que desliza por `transform` em `--dur-fast`, e trocar "Desativado" por "Nenhum" se o rótulo cortar a 280px (conferir na captura de divisor mínimo). O mesmo componente serve de conteúdo para o menu da barra recolhida (F-AC-14).

Status: pendente

### F-AN-01 — Soltar ZIP: overlay aparece para qualquer arrasto, arquivo errado abre alert() nativo e não há confirmação do que foi aceito

- **Arquivo:** `frontend/src/App.svelte:164`
- **Lente:** AN
- **Gravidade:** média
- **Evidência:** `handleDragOver` (App.svelte:370-373) liga `dragActive` sem olhar `e.dataTransfer.types`: arrastar texto selecionado (o `.erro-detalhe` tem `user-select: text`, ProcessList.svelte:1037) ou qualquer conteúdo que não seja arquivo cobre a janela com "Solte o arquivo ZIP..." sobre `--overlay` a 92% (style.css:28). Ao soltar algo que não é ZIP, o retorno é `alert('Por favor, arraste apenas arquivos ZIP do eproc.')` (linha 164), um diálogo modal do WebView numa janela frameless, embora o app já tenha o aviso animado `aviso-rapido` (642-655). Ao soltar um ZIP válido, o overlay só some (linha 158); a confirmação é a linha nova, que pode entrar fora da área visível. `handleDragLeave` (375-380) depende de `clientX === 0 || clientY === 0`. Não há captura do overlay; avaliado pelo código.
- **Proposta:** (1) Mostrar o overlay só quando `e.dataTransfer?.types?.includes('Files')` e controlar entrada/saída com contador de `dragenter`/`dragleave`. (2) Trocar o `alert` por `aviso.mostrar('Arraste apenas arquivos ZIP do eproc.', 'error')`, que já entra com `fly` e sai com `fade`. (3) Confirmar a soltura aceita: antes de fechar, a caixa troca o ícone por `Check` e o texto por "N arquivo(s) adicionados" por cerca de `--dur-base`, e sai com o fade atual; mesmo texto na região `role="status"` (`anuncio`). (4) Levar a linha nova para a área visível com `scrollIntoView({ block: 'nearest' })`, sem rolagem suave em reduced-motion. A entrada da caixa (`scale(0.98)`, linhas 694 e 703-706) fica como está.
- **Custo:** M
- **Ganho esperado:** Arrastar e soltar dá retorno em três momentos (pode soltar, aceito, recusado) sem diálogo modal e sem overlay em arrastos que não são de arquivo.
- **Veredito:** mantido
- **Justificativa do cético:** handleDragOver (App.svelte:370-373) liga dragActive sem olhar dataTransfer.types, então o overlay aparece ao arrastar texto selecionável (.erro-detalhe tem user-select:text, ProcessList.svelte:1037), e a linha 164 usa alert nativo, que o Wails v2.13.0 não desabilita (nenhuma configuração de script dialogs no frontend Windows). A parte sobre a linha nova entrar fora da tela é fraca: AddFiles insere no topo (queue.go).
- **Nota da síntese:** A troca do `alert()` por `aviso.mostrar` é a mesma de F-AC-07; corrigir uma vez só. O que é próprio deste achado: overlay só para arrasto de arquivos, contador de `dragenter`/`dragleave` e confirmação da soltura aceita.

Status: pendente

### F-DF-01 — Barras de progresso animam width (layout) de forma quase contínua, contra a regra de só animar transform/opacity

- **Arquivo:** `frontend/src/components/ProcessList.svelte:973`
- **Lentes:** DF, AN
- **Gravidade:** baixa
- **Evidência:** ProcessList.svelte:388: `<div class="progress-bar" style="width: {item.progress * 100}%"></div>` com `.progress-bar { ... transition: width var(--dur-base) var(--ease-out); }` (linhas 970-974). AtividadeItem.svelte:118: `<span style="width: {pct(d)}%"></span>` com `.doc-barra span { ... transition: width var(--dur-base) var(--ease-out); }` (linhas 313-318). O progresso chega até 10 vezes por segundo (queue:item) e a cada 200 ms (queue:atividade), e cada mudança abre uma transição de 220 ms: durante o OCR as barras passam a maior parte do tempo em transição de `width`, que exige layout e pintura na thread principal a cada quadro. DESIGN.md §7 diz: 'Animações contínuas só mexem em transform/opacity ... para não repintar a cada quadro'. As únicas `transition: width` do frontend são essas duas e a do painel lateral (App.svelte:845, pontual).
- **Proposta:** Trocar por transform: barra com `width: 100%`, `transform: scaleX(var(--p))`, `transform-origin: left center` e `transition: transform var(--dur-base) var(--ease-out)`, passando `style:--p={item.progress || 0}` (e `pct(d) / 100` nos documentos). O contêiner já tem `overflow: hidden` e os atributos do `role="progressbar"` não mudam. Com `prefers-reduced-motion` a duração já cai a 0 pelos tokens. Atualizar DESIGN.md §5.1/§7 para dizer que a barra anima por transform. frontend/paleta.test.mjs:205 e frontend/densidade.test.mjs:67 leem essas regras: conferir depois da troca.
- **Custo:** P
- **Ganho esperado:** A animação das barras sai do layout e vai para o compositor: sem reflow por quadro na linha ativa nem nos documentos em andamento, e movimento mais liso enquanto a CPU está tomada pelo OCR.
- **Veredito:** mantido
- **Justificativa do cético:** Confirmado em ProcessList.svelte:970-974 e AtividadeItem.svelte:313-318, e DESIGN.md:122 pede só transform/opacity. Medi com animações ligadas: durante a transição de width cada quadro custa na thread principal 1,3 ms (5 linhas), 2,0 ms (25) e 5,4 ms (100), contra 0,4 a 0,5 ms sem a transição; com o painel de atividade aberto (90 documentos, 100 linhas) são 8 a 10 ms por quadro contra 1,3 ms. O custo é real, cresce com a lista e é maior do que o achado supunha; a gravidade baixa fica como piso.
- **Nota da síntese:** Fundido com o achado de AN "Barras de progresso animam `width` (layout) em vez de `transform`" (baixa, P, rebaixado), com a mesma proposta. O cético de AN registrou que a alegação de `NaN%` com `item.progress` ausente é falsa (queue.go:25 serializa `progress` sempre, sem omitempty); o `|| 0` da proposta é só defensivo. A medição que sustenta o achado é a do cético de DF.

Status: pendente

### F-AC-07 — Falhas de ações da fila vão só para o console, e o ZIP inválido usa `alert()` nativo

- **Arquivo:** `frontend/src/App.svelte:164`
- **Lentes:** VI, AC
- **Gravidade:** baixa
- **Evidência:** App.svelte:164: `alert('Por favor, arraste apenas arquivos ZIP do eproc.')` abre a caixa modal do WebView, fora do visual do app e bloqueando a janela, enquanto existe o aviso próprio `aviso.mostrar` (linhas 201-202, usado em 215 e 238). Soltar ZIPs junto com outros arquivos ignora os outros sem dizer nada (linhas 160-162). Erros que só fazem `console.error`, sem nenhum retorno na tela: abrir pasta (ProcessList.svelte:171-177), pausar e retomar (204-218), cancelar, remover e limpar (220-248) e selecionar ZIP (App.svelte:304-313). Só copiar e reprocessar mostram erro (ProcessList.svelte:116 e 200).
- **Proposta:** Trocar o `alert` por `aviso.mostrar('Só arquivos ZIP do eproc são aceitos. Ignorados: N', 'error')`, que já sai em `role="alert"` com botão de fechar, e avisar também quando parte dos arquivos soltos foi ignorada. Nos `catch` de ProcessList, chamar `avisoCopia.mostrar` com a mensagem da ação ("Erro ao abrir a pasta: ..."), renomeando o aviso para algo geral (`avisoFila`).
- **Custo:** P
- **Ganho esperado:** Toda ação que falha diz que falhou, no mesmo lugar e com o mesmo visual; some a única caixa de diálogo do sistema dentro do app.
- **Veredito:** mantido
- **Justificativa do cético:** App.svelte:164 usa alert() nativo (único em frontend/src) apesar de existir aviso.mostrar; ZIPs soltos junto com outros arquivos descartam o resto em silêncio (160-162); e os catch de abrir pasta, pausar, retomar, cancelar, remover, limpar (ProcessList.svelte:171-248) e selecionar ZIP (App.svelte:304-313) só fazem console.error. Gravidade baixa está adequada.
- **Nota da síntese:** Fundido com o achado de VI "`alert()` nativo para arquivo que não é ZIP" (baixa, P, mantido), que sugere o ícone `FileX` 12px no aviso quando o componente unificado existir (F-VI-08). O mesmo `alert()` é citado em F-AN-01.

Status: pendente

### F-VI-03 — Linha com erro não se destaca e traz dois ícones vermelhos circulados e dois expansores

- **Arquivo:** `frontend/src/components/ProcessList.svelte:469`
- **Lente:** VI
- **Gravidade:** baixa
- **Evidência:** `class="item-row {item.status}"` (ProcessList.svelte:310-312) aplica a classe do status, mas não há regra CSS para `.item-row.error`, `.success` etc. (busca sem resultado). Em 1024x768-claro-erro.png a linha com falha tem a mesma borda das outras e dois ícones vermelhos circulados parecidos: `CircleAlert` do status à esquerda e `CircleQuestionMark` à direita (linha 469, com `.btn-detalhes { color: var(--error); }` em 1020-1022), ao lado do chevron da atividade. São dois controles de expandir na mesma linha.
- **Proposta:** Realce por estado só na borda da linha: erro com `--error-line` (token novo, `color-mix` de 30% como `--warning-line`), em processamento com `--accent-line`; fundo segue `--surface` (ajustar a frase da §6 do DESIGN.md). O botão de detalhes passa a `Info` em `--text-secondary`, vermelho só quando aberto. Alternativa: mostrar a mensagem completa do erro no topo do painel de atividade e ficar com um único expansor por linha.
- **Custo:** P
- **Ganho esperado:** Falhas saltam à vista numa fila longa sem ler linha por linha e a linha com erro fica com um só ponto vermelho, o do estado.
- **Veredito:** mantido
- **Justificativa do cético:** Não há regra para `.item-row.error` (só `.item-row` e `:hover`, ProcessList.svelte:780-798). A captura 1024x768-claro-erro.png mostra CircleAlert e CircleQuestionMark vermelhos na mesma linha, mais o chevron: `temAtividade` inclui 'erro' (linha 80-82), logo são dois expansores.

Status: pendente

### F-AC-08 — Estado vazio e rodapé mandam procurar o controle em outro lugar

- **Arquivo:** `frontend/src/components/ProcessList.svelte:303`
- **Lentes:** VI, AC
- **Gravidade:** baixa
- **Evidência:** ProcessList.svelte:303: 'Arraste arquivos ZIP em qualquer lugar ou clique em "Selecionar" no painel ao lado.' O estado vazio não tem botão; o usuário precisa achar "Selecionar arquivos ZIP..." no fim do painel (800x600-claro-configuracoes.png, abaixo da dobra junto com "Iniciar com o Windows"). Com o painel recolhido não existe nada escrito "Selecionar": só o ícone `Upload` (App.svelte:536). O rodapé faz o mesmo: "Monitoramento inativo. Configure uma pasta no painel ao lado." (App.svelte:627), embora o ponto de 6px ao lado já abra a escolha de pasta (`alternarMonitoramento`, linhas 231-241), sem nada que indique isso.
- **Proposta:** Pôr no estado vazio o próprio botão "Selecionar arquivos ZIP..." (ícone `Upload`, estilo do `.manual-select-btn`), chamando a mesma função por uma prop de callback, e encurtar o texto para "ou arraste os ZIPs para a janela". No rodapé sem pasta, trocar a frase por um botão de texto "Escolher pasta para monitorar" que chama `alternarMonitoramento`. O ícone `Archive` do vazio pode ganhar um `fade` de entrada em `--dur-base`.
- **Custo:** P
- **Ganho esperado:** O primeiro uso se resolve onde o olho já está, sem depender de o painel estar aberto nem de rolar até a seção de importação.
- **Veredito:** mantido
- **Justificativa do cético:** ProcessList.svelte:303 manda clicar em "Selecionar" no painel ao lado sem oferecer botão, com o painel recolhido só existe o ícone Upload (App.svelte:536), e o rodapé manda configurar a pasta no painel (627) embora o ponto ao lado já chame alternarMonitoramento. Um detalhe da evidência é falso: em 800x600-claro-configuracoes.png o botão "Selecionar arquivos ZIP..." está visível, quem fica cortado é "Iniciar com o Windows"; o núcleo do achado se mantém como baixa.
- **Nota da síntese:** Fundido com o achado de VI "Estado vazio pobre: ícone de 32px perdido na área e instrução que não vale com o painel recolhido" (baixa, M, rebaixado), sobre ProcessList.svelte:299-304 (captura 1920x1080-claro-vazio.png). VI propõe uma versão mais rica, opcional e de custo M: o vazio vira a própria zona de soltar (caixa com borda tracejada 1px `--border-strong`, raio 6px, cerca de 420x220; `FileArchive` 48px sobre um disco `--accent-faint`; título "Solte aqui os ZIPs do eproc" em `--fs-title`; uma linha com o estado do monitoramento) e o vazio do filtro ganha `SearchX` 32px e um botão "Limpar filtro". Sem gradiente nem blur. O cético de VI lembrou que o vazio atual funciona (soltar em qualquer lugar é aceito) e que o defeito concreto é a frase imprecisa.

Status: pendente

### F-AC-09 — Ponto de 6px como botão de monitoramento, com `aria-pressed` e rótulo que muda ao mesmo tempo

- **Arquivo:** `frontend/src/App.svelte:607`
- **Lentes:** VI, AC
- **Gravidade:** baixa
- **Evidência:** O botão do rodapé (App.svelte:607-617) mostra só um ponto de 6px (`.status-dot`, linhas 1182-1188) numa área de 18x18 (`.status-dot-btn`, 1159-1172); o da barra recolhida (522-534) é o mesmo ponto em 24x24. Nada no repouso indica que é clicável, e o clique liga ou pausa o monitoramento na hora (`alternarMonitoramento`, 231-244). Para leitor de tela o estado é dito duas vezes e em sentidos opostos: `aria-pressed={monitorando}` com `aria-label={acaoMonitor}`, que alterna entre "Pausar o monitoramento" e "Ativar o monitoramento" (linhas 248-250, 613-614). Ligado, lê-se "Pausar o monitoramento, pressionado". O estado também depende só de cor: verde ou âmbar (linhas 1237-1243).
- **Proposta:** Dar forma de controle: no rodapé, um botão com ícone de 12px que muda com o estado (`Eye` monitorando, `Pause`/`EyeOff` pausado, `FolderOpen` sem pasta) ao lado do texto, com fundo `--surface-hover` no hover e alvo de 24px; na barra recolhida, o mesmo ícone em 16px no lugar do ponto. Escolher um modelo de estado: rótulo fixo "Monitoramento" com `aria-pressed`, ou rótulo de ação sem `aria-pressed` (fila.test.mjs:161 fixa o `aria-label` atual e precisa acompanhar). A troca de ícone pode usar `fade` em `--dur-fast`.
- **Custo:** P
- **Ganho esperado:** O controle de monitoramento passa a parecer um controle, o estado deixa de depender só da cor e o leitor de tela ouve uma informação só.
- **Veredito:** mantido
- **Justificativa do cético:** O botão do rodapé (App.svelte:607-617) é um ponto de 6px em área de 18x18 (1159-1188) e combina aria-pressed={monitorando} com aria-label={acaoMonitor}, que alterna entre "Pausar" e "Ativar o monitoramento" (248-250), gerando "Pausar o monitoramento, pressionado". A parte "só cor" vale para a barra recolhida; no rodapé há texto ao lado, mas isso não muda a gravidade baixa.
- **Nota da síntese:** Fundido com o achado de VI "Indicador de monitoramento é só um ponto de 6px, inclusive quando é botão" (baixa, P, mantido), que olha o mesmo ponto na barra recolhida (App.svelte:522-534 e 995-1000; capturas 1024x768-claro-processando-recolhido.png e 800x600-escuro-processando-recolhido.png). Divergência a resolver: VI propõe a moldura `.collapsed-tool-btn` com `FolderSearch` 14px (ativo) ou `FolderX` 14px (pausado ou sem pasta) e o ponto como selo no canto; AC propõe `Eye`, `Pause`/`EyeOff` e `FolderOpen`. Usar um conjunto só, coerente com o ícone do rótulo "Monitoramento ativo" em F-VI-04 (`FolderSearch`). Os ícones citados existem no lucide-svelte 1.0.1 instalado.

Status: pendente

### F-VI-04 — Inventário de ícones: 33 passam por Icone.svelte; rótulos do painel, filtro, botões de texto e um spinner ainda não

- **Arquivo:** `frontend/src/components/Settings.svelte:172`
- **Lente:** VI
- **Gravidade:** baixa
- **Evidência:** Icone.svelte:9 recebe 33 ícones lucide: Archive, Check, ChevronDown, ChevronLeft, ChevronRight, CircleAlert, CircleCheck, CirclePause, CircleQuestionMark, CircleSlash, Clock, Copy, FileArchive, FilePlusCorner, FileText, FolderOpen, List, ListX, LoaderCircle, Minus, Monitor, Moon, Pause, Play, RotateCcw, ScanText, Settings, Sparkles, Square, Sun, Trash2, Upload, X. Só texto ou caractere: rótulos do formulário (Settings.svelte:135, 149, 172, 188, 204, 218), embora a barra recolhida represente OCR com `ScanText` e Higienização com `Sparkles` sem rótulo (App.svelte:548, 564), ou seja, o painel aberto nunca ensina esses ícones; filtro só com `placeholder="Filtrar..."` (ProcessList.svelte:258-264); botões "Cancelar" e "Reprocessar" (ReprocessarItem.svelte:57-58); "Mostrar concluídos" (AtividadeItem.svelte:82-89); "OCR · {d.ocr} pág." (AtividadeItem.svelte:113); carregamento com spinner de borda CSS (`.spinner`, Settings.svelte:128 e 521-528) em vez do `LoaderCircle` da §4 do DESIGN.md. Os ícones de seção em 12px (Settings.svelte:122, App.svelte:504) viram um borrão nas capturas (1024x768-claro-configuracoes.png).
- **Proposta:** Rótulos com ícone de 14px em `--text-secondary`: `FolderSearch` (Monitoramento ativo), `Folder` (Pasta), `ScanText` (OCR), `Sparkles` (Higienização), `EyeOff` (Omitir tipos), `Power` (Iniciar com o Windows). `Search` 12px dentro do filtro (padding-left de 24px); `RotateCcw` 12px no botão "Reprocessar"; `Eye`/`EyeOff` 12px em Mostrar/Ocultar concluídos; `ScanText` 12px no lugar de "OCR ·"; `LoaderCircle` no carregamento; ícones de seção em 14px. Todos existem em lucide-svelte 1.0.1 (conferido em node_modules). Registrar o mapa na §4 do DESIGN.md e em icones.test.mjs.
- **Custo:** P
- **Ganho esperado:** O mesmo ícone identifica a mesma configuração no painel aberto, na barra recolhida e no reprocessamento; os campos se localizam mais rápido sem aumentar a altura.
- **Veredito:** mantido
- **Justificativa do cético:** Contei 33 imports distintos de lucide em frontend/src, iguais à lista; os rótulos só com texto, o filtro sem ícone e o spinner de borda CSS (Settings.svelte:128, 521-528) conferem. Os ícones propostos (folder-search, folder, eye, eye-off, power, search) existem em node_modules/lucide-svelte 1.0.1; ganho pequeno, compatível com a gravidade baixa.

Status: pendente

### F-AN-02 — Hover e estado pressionado variam de botão para botão

- **Arquivo:** `frontend/src/components/ProcessList.svelte:688`
- **Lentes:** VI, AN
- **Gravidade:** baixa
- **Evidência:** Com transição de 150 ms no hover: `.titlebar-btn` (App.svelte:767), `.btn-action` (ProcessList.svelte:1002), `.manual-select-btn` (App.svelte:1104), `.aviso-fechar` (ProcessList.svelte:577), `.toast-fechar` (Settings.svelte:504). Sem transição, mudando cor, fundo ou borda no hover: `.btn-clear` (ProcessList.svelte:688-706), `.btn-pause` (708-742), `.btn-toggle-sidebar` (App.svelte:876-891), `.status-dot-btn` (App.svelte:1159-1176), `.aviso-rapido-fechar` (App.svelte:1218-1235, idêntico em função aos dois "fechar" que têm). `.btn-danger-icon` herda só `background-color` da regra genérica (Settings.svelte:372) e a borda vira seca (399-402). Estado pressionado só existe em `.btn-primary:active { transform: scale(0.98); }` (Settings.svelte:385-387). DESIGN.md §7 reserva `--dur-fast` para hover e foco.
- **Proposta:** Uma regra de especificidade zero em style.css: `:where(button) { transition: background-color var(--dur-fast) var(--ease-out), border-color var(--dur-fast) var(--ease-out), color var(--dur-fast) var(--ease-out); }` e `:where(button):active:not(:disabled) { transform: scale(0.97); }`, com `transform` na lista de transições; remover as cópias por componente. Em reduced-motion os tokens já zeram a duração; o `scale` instantâneo pode ficar ou ser retirado na media query. Combina com a troca de tema por View Transition, que evita essas transições aparecerem durante a troca.
- **Custo:** P
- **Ganho esperado:** Todo botão responde ao ponteiro e ao clique do mesmo jeito; menos CSS repetido.
- **Veredito:** mantido
- **Justificativa do cético:** Conferi a inconsistência: .btn-clear (ProcessList.svelte:688-706), .btn-pause (708-742), .btn-toggle-sidebar (App.svelte:876-891), .status-dot-btn (1159-1176) e .aviso-rapido-fechar (1218-1235) não têm transição de hover, enquanto .btn-action (1002), .titlebar-btn (767) e os botões de Settings (372) têm. Ressalva para a correção: um :active scale global também encolheria .info-btn, que é a linha inteira.
- **Nota da síntese:** Fundido com o achado de VI "Estados de hover, pressionado e entrada de painéis desiguais entre controles" (baixa, M, mantido). Complementos de VI: campos e selects sem hover (Settings.svelte:331-362), `.alternar` que só sublinha (AtividadeItem.svelte:231-244) e "Pausar" preenchido ao lado de "Limpar Fila" transparente, sem razão de hierarquia (1024x768-claro-fila.png). VI propõe classes utilitárias em style.css (`.btn`, `.btn-icone`, `.campo`), `:active` com fundo `--border` e sem escala, hover de campo com borda `--text-secondary` e uma tabela de estados na §7 do DESIGN.md. A entrada dos painéis da linha está em F-AN-08. Ressalva do cético de AN: um `:active` com `scale` global encolheria `.info-btn`, que é a linha inteira; excluir esse botão ou adotar o `:active` de fundo proposto por VI.

Status: pendente

### F-AN-03 — Testes de movimento não cobrem AtividadeItem nem ReprocessarItem, onde entrarão efeitos novos

- **Arquivo:** `frontend/movimento.test.mjs:117`
- **Lente:** AN
- **Gravidade:** baixa
- **Evidência:** `const folhas = { 'App.svelte': ..., 'ProcessList.svelte': ..., 'Settings.svelte': ..., 'style.css': css }` (movimento.test.mjs:117) e o laço `for (const [nome, f] of [['App', app], ['ProcessList', lista], ['Settings', settings]])` (linha 174) deixam de fora AtividadeItem.svelte (que tem `transition` e `animation` em 179, 317 e 333) e ReprocessarItem.svelte (linha 112). A regex da linha 175 só reconhece `fly` e `fade`: um `in:scale`, `in:slide` ou `animate:flip` com duração literal passaria sem `duracaoMovimento`, ou seja, sem respeitar `prefers-reduced-motion`. Hoje os dois arquivos usam os tokens corretamente; o risco é dos efeitos novos desta rodada.
- **Proposta:** Montar `folhas` e a lista de componentes a partir de todos os `.svelte` de `src` (glob), e ampliar a regex para `(?:transition|in|out|animate):(?:fly|fade|scale|slide|flip)`. Acrescentar uma verificação de que todo `@keyframes` infinito fora de `spin` tem regra correspondente em `@media (prefers-reduced-motion: reduce)` e só mexe em `transform`/`opacity`.
- **Custo:** P
- **Ganho esperado:** As microinterações novas já nascem presas à regra de tokens e de reduced-motion em qualquer componente.
- **Veredito:** mantido
- **Justificativa do cético:** Em frontend/movimento.test.mjs, 'folhas' (linha 117) e o laço da linha 174 cobrem só App, ProcessList, Settings e style.css, e a regex só reconhece fly|fade; nenhum frontend/*.test.mjs menciona AtividadeItem ou ReprocessarItem (grep vazio). Lacuna de teste real, gravidade baixa.
- **Nota da síntese:** Vale fazer antes dos achados que acrescentam `scale`, `flip` ou keyframes (F-AC-01, F-AN-08, F-AN-09, F-AN-11).

Status: pendente

### F-VI-05 — Botões da janela não encostam no canto e parecem botões de barra de ferramentas

- **Arquivo:** `frontend/src/App.svelte:716`
- **Lente:** VI
- **Gravidade:** baixa
- **Evidência:** `.custom-titlebar { height: 38px; ... padding: 0 12px; }` (App.svelte:709-717), `.titlebar-actions { gap: 4px; }` (750-754) e `.titlebar-btn { border-radius: 4px; width: 28px; height: 28px; }` (756-768); janela `Frameless: true` (main.go:55). Nas capturas (1024x768-claro-fila.png) o X fica a 12px da borda direita e 5px do topo: com a janela maximizada, o canto superior direito cai na barra arrastável (`--wails-draggable: drag`, App.svelte:410) e não em Fechar.
- **Proposta:** Botões de janela com 44x38px (altura inteira da barra), sem raio e sem intervalo, o último colado à borda (`padding-right: 0` na barra). O botão de tema continua com 28px, separado por um divisor de 1px. Hover `--surface-hover`; Fechar com `--error` e `--on-accent`, como hoje.
- **Custo:** P
- **Ganho esperado:** Fechar, maximizar e minimizar ficam onde o Windows 11 acostumou a mão, com alvo maior e sem errar o clique no canto.
- **Veredito:** mantido
- **Justificativa do cético:** Confirmado em App.svelte:709-768: `padding: 0 12px`, botões de 28x28 com raio de 4px e intervalo de 4px, numa janela `Frameless` (main.go:55). O canto superior direito cai na área arrastável e não em Fechar, como se vê nas capturas (X a 12px da borda). O custo do erro é pequeno porque Fechar só oculta para a bandeja.

Status: pendente

### F-VI-06 — Barra de rolagem quase invisível e nenhuma pista de conteúdo rolado

- **Arquivo:** `frontend/src/assets/css/style.css:129`
- **Lente:** VI
- **Gravidade:** baixa
- **Evidência:** `::-webkit-scrollbar-thumb { background: var(--border); border-radius: 3px; }` (style.css:129-132) com largura de 6px (120-123): o polegar fica em 1,19:1 sobre `--surface` no escuro e 1,30:1 no claro (calculado dos hex). As áreas que rolam são `.items-container` (ProcessList.svelte:744-748), `.sidebar-content` (App.svelte:893-899) e `.docs` com `max-height: 220px` (AtividadeItem.svelte:246-255). Avaliado pelo código: as capturas escondem barras de rolagem. O cabeçalho da lista não muda quando há itens rolados por baixo dele.
- **Proposta:** Polegar em `--border-strong` (3,56:1 no escuro, 4,69:1 no claro), 8px de largura com 2px transparentes em volta (`background-clip: padding-box`), hover `--text-secondary`. Sombra de rolagem sem gradiente: `--shadow-sm` sob `.list-header` e sobre o pé do painel só quando há conteúdo rolado (sentinela com IntersectionObserver ou `animation-timeline: scroll()`, que o WebView2 atual aceita). Registrar na §2.4 do DESIGN.md esse segundo uso de `--shadow-sm`.
- **Custo:** P
- **Ganho esperado:** Fica visível que a fila, o painel e a lista de documentos têm mais conteúdo, e a barra vira um alvo que dá para pegar com o mouse.
- **Veredito:** mantido
- **Justificativa do cético:** style.css:120-132 confere (6px, polegar em `--border`). Recalculei: 1,19:1 no escuro e 1,30:1 no claro sobre `--surface`; `--border-strong` daria 3,56 e 4,69:1. As três áreas roláveis citadas existem; avaliado só pelo código, pois as capturas escondem as barras.

Status: pendente

### F-AN-04 — Limpar Fila e remover o último item não têm fade; o estado vazio aparece seco

- **Arquivo:** `frontend/src/components/ProcessList.svelte:315`
- **Lente:** AN
- **Gravidade:** baixa
- **Evidência:** A saída é local: `out:fade={{ duration: mov.removido(item.id) ? ... : 0 }}` (ProcessList.svelte:315), mas as linhas vivem dentro do ramo `{:else}` de `{#if items.length > 0 && filteredItems.length === 0} ... {:else if filteredItems.length === 0} ... {:else}` (295-305). Quando a lista esvazia, quem troca é o `{#if}` pai, e transição local não roda quando o bloco pai é destruído (conferido em `pause_children`, node_modules/svelte/src/internal/client/reactivity/effects.js:644-676: o `each` não é transparente, então só transições `|global` são coletadas). Remover um item entre vários sai com fade de 150 ms; remover o último, ou Limpar Fila, corta para o estado vazio no mesmo quadro. O `.empty-state` (296-304) não tem transição de entrada. DESIGN.md §7 diz que "o que sai da fila sai com fade".
- **Proposta:** Tornar a saída global (`out:fade|global`), mantendo o gatilho `mov.removido(item.id)` (o filtro continua com duração 0), e dar ao estado vazio `in:fade={{ duration: duracaoMovimento(DUR_FAST), delay: duracaoMovimento(DUR_FAST) }}`. Durante a saída os dois ramos coexistem por 150 ms: posicionar o `.empty-state` de forma absoluta dentro de `.items-container`, para a lista em fade não ser empurrada. Ajustar a regex de movimento.test.mjs:235.
- **Custo:** P
- **Ganho esperado:** Mesma regra de saída para um item ou para todos; Limpar Fila deixa de ser um corte seco.
- **Veredito:** mantido
- **Justificativa do cético:** O out:fade da linha é local (ProcessList.svelte:314-316) e, no Svelte 5.57.1, pause_children (reactivity/effects.js:644-676) não coleta transições locais de um each quando o ramo {#if} pai (295-305) é destruído; o último item some sem fade. Contradiz DESIGN.md §7 ('o que sai da fila sai com fade'); gravidade baixa.

Status: pendente

### F-AN-05 — Chip "Fila pausada" e indicador "N ativos" surgem sem transição; o chip desloca a lista inteira

- **Arquivo:** `frontend/src/components/ProcessList.svelte:285`
- **Lente:** AN
- **Gravidade:** baixa
- **Evidência:** `{#if aviso}<div class="queue-notice">` (ProcessList.svelte:285-292) entra no fluxo entre o cabeçalho e a lista, sem `in:`/`out:`. Comparando `1024x768-escuro-erro.png` (com o chip, primeira linha em y≈141) com `1024x768-escuro-processando.png` (sem o chip, primeira linha em y≈114), pausar ou retomar move todas as linhas ~27 px em um quadro. `{#if processingCount > 0}<div class="pulse-indicator">` (App.svelte:418-423) também aparece e some sem transição, e os botões Pausar/Retomar trocam por `{#if isPaused}` (266-276).
- **Proposta:** Chip e indicador com `transition:fade={{ duration: duracaoMovimento(DUR_FAST) }}`. Para a lista não saltar sem animar altura, compensar com transform: ao montar ou desmontar o chip, animar `.items-container` de `translateY(∓altura do chip)` para `none` com `element.animate(..., { duration: duracaoMovimento(DUR_FAST), easing: 'cubic-bezier(.2,.8,.2,1)' })` numa action ou attachment pequena. Alternativa mais simples: reservar o lugar do chip na linha do cabeçalho.
- **Custo:** P
- **Ganho esperado:** Pausar e retomar deixa de sacudir a lista; os indicadores de estado entram e saem como o resto da interface.
- **Veredito:** mantido
- **Justificativa do cético:** O chip de aviso da fila ({#if aviso}, ProcessList.svelte:285-292) e o indicador 'N ativos' ({#if processingCount > 0}, App.svelte:418-423) entram e saem sem transição, e o chip empurra a lista em cerca de 27 px. Evidência exata, impacto baixo.

Status: pendente

### F-AN-06 — Salvar de novo enquanto o toast de sucesso está visível não dá retorno novo

- **Arquivo:** `frontend/src/components/Settings.svelte:236`
- **Lente:** AN
- **Gravidade:** baixa
- **Evidência:** Cada `handleSave` chama `showStatus('Configurações salvas com sucesso!', 'success')` (Settings.svelte:95). `criarAviso.mostrar` cancela o timer e redefine o mesmo texto (aviso.js:25-32); o `{#if statusMessage && statusType !== 'error'}` (236) continua verdadeiro, então o `in:fly` (239) não roda de novo. Quem muda dois ou três controles em menos de 3 s vê o primeiro toast e depois nada: o texto é idêntico e só o prazo é renovado.
- **Proposta:** Dar ao aviso um contador (`{ texto, tipo, n }`) em `criarAviso` e envolver o conteúdo em `{#key n}` com um `in:fade` curto (`--dur-fast`), para cada salvamento reafirmar a mensagem sem repetir o deslocamento. Alternativa mais localizada: um `Check` de 12 px que aparece com fade ao lado do rótulo do controle alterado e some em 1,5 s, deixando o toast só para erro.
- **Custo:** P
- **Ganho esperado:** Cada alteração confirmada tem um sinal próprio, sem aumentar o tempo de tela do toast.
- **Veredito:** mantido
- **Justificativa do cético:** criarAviso.mostrar (aviso.js) cancela o timer e redefine o mesmo {texto, tipo}; com o mesmo texto o {#if} de Settings.svelte:236 continua verdadeiro, o DOM não muda e o in:fly (239) não roda de novo, então salvar duas vezes seguidas não dá retorno visível. Evidência exata, ganho pequeno.

Status: pendente

### B-EF-03 — Página sem operadores de texto é interpretada uma segunda vez pelo GetTextByRow

- **Arquivo:** `internal/etl/pdf_parser.go:625`
- **Lente:** EF
- **Gravidade:** baixa
- **Evidência:** `ExtractNativeText` cai em `page.GetTextByRow()` (pdf_parser.go:625) sempre que `textoPosicionalSeguro` devolve texto vazio num stream único com fontes legíveis (pdf_parser.go:608-619), mesmo quando o extrator posicional rodou sem erro e não viu nenhum Tj/TJ/'/": é o caso de toda página escaneada ou desenhada em vetores, em que o GetTextByRow reinterpreta o stream inteiro e também não acha nada. `BenchmarkEFPassadas` (overlay, -count=6; arquivo scratchpad da sessão, ef/zz_ef2_test.go) na página de vetorial.pdf (stream de 199 KB): `ExtractNativeText` 260.958 allocs / 4,54 MB = checkContentStream 52.204 + posicional 104.351 + GetTextByRow 104.376 allocs (1,88 MB), com GetTextByRow levando 11,8-16,4 ms, o mesmo que a passada posicional (14,6-18,2 ms). Em escaneado.pdf: GetTextByRow 22-28 us dos 42-57 us de `ExtractNativeText`. No perfil do `BenchmarkSintetico`, `Page.GetTextByRow` soma 1,93 s dos 5,51 s de `ExtractNativeText` (9,5% de toda a CPU do benchmark), vindos de 5 documentos em 50.
- **Proposta:** Fazer `extrairTextoPosicional` devolver também quantas operações de mostrar texto viu (contador incrementado em `mostrar`, pdf_texto.go:191) e, em `ExtractNativeText`, pular o fallback quando `errPos == nil` e o contador é zero. O fallback continua valendo para pânico/erro do extrator posicional e para páginas em que houve Tj mas a decodificação saiu vazia.
- **Custo:** P
- **Ganho esperado:** Cerca de -40% de CPU e de alocações na extração nativa de páginas sem texto (vetoriais e escaneadas): -12 a -16 ms por página vetorial de 200 KB e cerca de -9% de CPU no `BenchmarkSintetico`. Em ZIP com OCR ligado o efeito no tempo total é pequeno, porque essas páginas seguem para o OCR, que custa segundos.
- **Veredito:** mantido
- **Justificativa do cético:** Confirmei em pdf_parser.go:608-625 que o texto posicional vazio sem erro cai em `GetTextByRow`, que trata os mesmos operadores de mostrar texto (Tj, TJ, ', ") que `extrairTextoPosicional` (page.go:712-749 da dslipak v0.0.2), então a segunda passada não acha nada novo. Reproduzi: em vetorial.pdf `GetTextByRow` custa 9,2-10 ms e 104.376 alocações dos 24-36 ms de `ExtractNativeText`, em escaneado.pdf 20-23 us de 50-56 us, e no perfil do revisor soma 1,09 s de 11,91 s (9,15%); o ganho real é pequeno (páginas vetoriais são raras e seguem para o OCR), como a gravidade baixa já diz.

Status: pendente

### F-DF-02 — Todas as linhas da fila entram no layout a cada quadro do arrasto do divisor; não há content-visibility nem janela

- **Arquivo:** `frontend/src/components/ProcessList.svelte:780`
- **Lente:** DF
- **Gravidade:** baixa
- **Evidência:** No arrasto, a largura do painel muda uma vez por quadro (App.svelte:316-318 e 473, `style="width: {sidebarCollapsed ? '48px' : larguraPainel + 'px'}"`), e recolher/expandir anima `width` por 150 ms (App.svelte:845). A fila é `flex: 1` com `container-type: inline-size` (ProcessList.svelte:594) e regras `@container` (linhas 723, 913, 920), então cada quadro refaz o layout de todas as `.item-row` (linhas 780-793: flex com wrap, nome com reticências, até 4 botões com svg), inclusive as que estão fora da área visível de `.items-container` (`overflow-y: auto`, linhas 744-748). A busca por `content-visibility`, `contain:` e `will-change` em frontend/src não retorna nada, e a fila não tem teto de itens. Impacto não medido: é linear no tamanho da fila e só pesa em histórico longo. Conferido e sem achado: bundle de produção com 119,7 kB de JS (39,4 kB gzip) e 31,6 kB de CSS (5,6 kB gzip), servido do exe; ícones lucide importados um a um; as quatro assinaturas EventsOn, o OnFileDrop, o listener de matchMedia, o blur do arrasto e os timers têm limpeza no onDestroy.
- **Proposta:** Em `.item-row`: `content-visibility: auto; contain-intrinsic-size: auto 36px;` (o `auto` guarda a última altura real, o que cobre linhas com painel aberto). O WebView2 é Chromium e suporta. Validar três pontos: o `animate:flip` (mede com getBoundingClientRect, que funciona com tamanho intrínseco), a devolução de foco de `linhaDe()` em linha fora da tela, e o recorte do anel de foco pela contenção de pintura (o padding de 4px 8px da linha comporta o outline de 2px + 1px dos botões). Se a fila passar a ter centenas de itens no uso real, o passo seguinte é janela virtual, que hoje não se justifica.
- **Custo:** P
- **Ganho esperado:** Layout e pintura durante arrasto do divisor, recolher/expandir e redimensionar a janela passam a depender das linhas visíveis (cerca de 15 a 25), não do histórico inteiro.
- **Veredito:** mantido
- **Justificativa do cético:** Confirmado: sem content-visibility/contain em frontend/src, fila com container-type e regras @container. Medi o layout forçado por quadro ao variar a largura do painel: 1,8 ms (5 linhas), 3,5 (25), 5,5 (50), 10 (100) e 26 ms (300); injetando a regra proposta em .item-row: 3,7 (25), 5,6 (50), 6,0 (100) e 6,6 ms (300). O ganho é nulo até 50 itens, 40% com 100 e 4 vezes com 300, então só aparece em histórico longo, como o próprio achado diz; fica em baixa, com os três pontos de validação (flip, foco, anel de foco) como condição.

Status: pendente

### F-AC-10 — Sem regras para o modo de alto contraste do Windows: interruptores, pontos de estado e barras são só `background-color`

- **Arquivo:** `frontend/src/components/Settings.svelte:419`
- **Lente:** AC
- **Gravidade:** baixa
- **Evidência:** Busca por `forced-colors` e `prefers-contrast` em frontend/src: nenhum resultado. Vários indicadores existem só como cor de fundo, sem borda nem ícone: o trilho e a bolinha do interruptor (Settings.svelte:419-455, `background-color: var(--border-strong)` / `var(--on-accent)`, input com `opacity: 0`), os pontos de estado (App.svelte:1182-1188 e 995-1000), os pontos de nível (App.svelte:1014-1024), a barra de progresso (ProcessList.svelte:960-974) e as vagas e barras do painel de atividade (AtividadeItem.svelte:174-188, 305-322). Em `forced-colors: active` (temas de contraste do Windows, que o WebView2 respeita) o navegador substitui os fundos pelas cores do sistema, e esses elementos deixam de se distinguir. Avaliado pelo código; as capturas não cobrem esse modo.
- **Proposta:** Acrescentar em style.css um bloco `@media (forced-colors: active)` que dá borda `1px solid ButtonText`/`CanvasText` ao trilho, à bolinha, aos pontos e aos trilhos das barras, e usa `Highlight` para o estado ligado e o preenchimento (com `forced-color-adjust: none` só nesses preenchimentos). Conferir o anel de foco (o `outline` já é preservado nesse modo). Incluir uma captura com `forcedColors: 'active'` na matriz de `.claude/review.json` para validar em vez de presumir.
- **Custo:** P
- **Ganho esperado:** Quem usa tema de contraste do Windows continua vendo se o monitoramento está ligado, em que nível está o OCR e quanto falta do processamento.
- **Veredito:** mantido
- **Justificativa do cético:** Busca por forced-colors e prefers-contrast em frontend/src não retorna nada, e trilho/bolinha do interruptor (Settings.svelte:419-455), pontos de estado e de nível (App.svelte:995-1024, 1182-1188), barra de progresso (ProcessList.svelte:960-974) e vagas/barras (AtividadeItem.svelte:174-188, 305-322) são só background-color, que o Chromium força para a cor do sistema nesse modo. Não reproduzi em captura (o próprio achado declara isso); público pequeno, gravidade baixa adequada.

Status: pendente

### F-AC-11 — Semântica miúda: marcos, títulos, rótulo do divisor, ajuda sem vínculo e anel de foco igual ao botão primário

- **Arquivo:** `frontend/src/App.svelte:392`
- **Lente:** AC
- **Gravidade:** baixa
- **Evidência:**
  Itens pequenos, todos conferidos no código:
  - App.svelte:392-393: `role="region" aria-label="Aplicativo eprocIA"` envolve a janela inteira, um marco que não ajuda a navegar; o `<aside id="sidebar">` fica dentro do `<main>` (468-470) e não tem nome.
  - Títulos: "Configurações" é `h2` (Settings.svelte:123) e a seção irmã "Importação Manual" é `h3` (App.svelte:505).
  - App.svelte:579: `aria-label="Redimensionador de Menu"`; o app chama essa área de "painel" em todos os outros rótulos ("Recolher painel", linha 484).
  - Settings.svelte:183, 199 e 214: os `.help-text` não estão ligados aos campos por `aria-describedby`.
  - Settings.svelte:137-142 e 221-226: interruptores são `type="checkbox"` sem `role="switch"`.
  - ProcessList.svelte:340: `role="button"` redundante num `<button>`.
  - style.css:114-117: o anel de foco é `--focus-ring` com afastamento de 1px; no tema claro `--focus-ring` e o fundo do botão primário são o mesmo `#0056B3` (contraste 1,00:1; no escuro 1,77:1), então em "Alterar" e "Reprocessar" o foco aparece só como um fio de 1px da cor da superfície.
- **Proposta:** Tirar o `role="region"` do contêiner; dar `aria-label="Configurações e importação"` ao `aside` e tirá-lo de dentro do `main` (ou trocar o `main` para envolver só a fila). Igualar "Importação Manual" a `h2`. Renomear o divisor para "Largura do painel". Ligar cada ajuda ao campo com `aria-describedby`. Acrescentar `role="switch"` aos dois interruptores. Remover o `role` redundante. No botão primário, usar `outline-offset: 2px` para o anel se separar do preenchimento. Tudo cabe na passada de sintaxe da migração para runes, que já toca essas marcações.
- **Custo:** P
- **Ganho esperado:** Navegação por marcos e títulos coerente, ajuda lida junto com o campo e foco mais nítido nos botões primários, sem mudança visual perceptível.
- **Veredito:** mantido
- **Justificativa do cético:** Todos os itens conferem no código: role="region" no contêiner (App.svelte:392-393) e aside sem nome dentro do main (468-470); h2 em Settings.svelte:123 contra h3 em App.svelte:505; "Redimensionador de Menu" (579); nenhum aria-describedby em frontend/src; checkboxes sem role="switch"; role="button" redundante (ProcessList.svelte:340). Calculei --focus-ring contra --accent-strong: 1,00:1 no claro e 1,77:1 no escuro, com outline-offset de 1px (style.css:114-117).

Status: pendente

### F-AC-12 — "Remover da fila" no item pendente não remove: deixa uma linha "Cancelado" que pede outro clique

- **Arquivo:** `frontend/src/components/ProcessList.svelte:408`
- **Lente:** AC
- **Gravidade:** baixa
- **Evidência:** O X do item pendente tem `title="Remover da fila"` (linha 408), mas chama `handleCancel` (linha 407), isto é, `CancelQueueItem`. No Go, `CancelItem` (internal/etl/queue.go:347-350) só troca o estado: `target.Status = "cancelled"; target.Message = "Cancelado"`. A linha continua na lista com reprocessar e lixeira (item 5 em 1024x768-escuro-fila-reprocessar.png) e precisa de um segundo clique, agora em "Remover da lista", para sair. O mesmo ícone `X` com `data-acao="cancelar"` tem dois rótulos ("Cancelar processamento ativo" na linha 399 e "Remover da fila" na 408) e "remover" significa duas coisas a duas linhas de distância.
- **Proposta:** Alinhar rótulo e efeito. Opção mais simples: o X do pendente passa a chamar `RemoveQueueItem` (ícone `X`, "Tirar da fila"), já que um ZIP que nem começou não tem resultado a preservar e o arquivo continua na pasta; `data-acao="remover"` mantém a regra de foco. Se a intenção for manter o registro, o rótulo vira "Cancelar (fica na lista como cancelado)". Em qualquer caso, usar o mesmo verbo para o mesmo efeito: "Cancelar" quando sobra a linha, "Remover" quando ela some.
- **Custo:** P
- **Ganho esperado:** Um clique faz o que o rótulo promete; some o passo extra de limpar a linha "Cancelado" que o usuário não pediu.
- **Veredito:** rebaixado
- **Justificativa do cético:** Confirmado: o X do pendente tem title "Remover da fila" (ProcessList.svelte:408) e chama CancelQueueItem, que só marca cancelled (queue.go:347-350), deixando a linha com reprocessar e lixeira. O rótulo é defensável ("fila" de pendentes contra "lista" na lixeira) e manter a linha permite reprocessar; é ambiguidade de texto com um clique extra, não defeito de fluxo: baixa.

Status: pendente

### F-VI-07 — Painel lateral com 8px de padding, desalinhado dos 12px das barras

- **Arquivo:** `frontend/src/App.svelte:894`
- **Lente:** VI
- **Gravidade:** baixa
- **Evidência:** `.sidebar-content { padding: 8px; ... gap: 8px; }` (App.svelte:893-897), enquanto barra de título (`padding: 0 12px`, 716), rodapé (1127), `.sidebar-header-toggle` (`8px 12px`, 864) e `.list-header` (ProcessList.svelte:604) usam 12px. Na captura 1024x768-claro-configuracoes.png os rótulos começam em x=8 e a marca e o ponto do rodapé em x=12; os campos chegam a 8px do divisor. A faixa `.sidebar-header-toggle` (38px, 860-869) aparece vazia, só com o chevron. docs/backlog.md:9 pede "aumentar um pouco o padding da barra lateral".
- **Proposta:** Padding horizontal de 12px no conteúdo do painel e 16px entre seções (com o divisor de 1px); 12px entre grupos do formulário continua. Agrupar em três blocos com rótulo `--fs-section`: Monitoramento (interruptor + pasta), Processamento (OCR, higienização, tipos omitidos), Sistema (iniciar com o Windows). Para pagar a altura extra, levar o rótulo "Configurações" para a faixa do botão de recolher, hoje vazia (ganha cerca de 26px; exige mudar DESIGN.md §5.3 e os testes densidade.test.mjs:139 e movimento.test.mjs:262). Registrar na §6 que o painel usa 12px.
- **Custo:** P
- **Ganho esperado:** Uma única linha-guia vertical de 12px na janela inteira, respiro entre campos e divisor, e grupos que se leem de relance.
- **Veredito:** rebaixado
- **Justificativa do cético:** Confirmado: `.sidebar-content` tem 8px (App.svelte:894) contra 12px na barra de título, rodapé e cabeçalhos, e o backlog:9 pede mais padding. É um desalinhamento cosmético de 4px; a parte de levar "Configurações" para a faixa do botão desfaz uma decisão registrada (DESIGN.md §5.3 "Sem título geral", comentário em App.svelte:859 e teste movimento.test.mjs:262).
- **Nota da síntese:** O cético separou duas partes: o padding de 12px (pedido em docs/backlog.md:9) e levar o rótulo "Configurações" para a faixa do botão de recolher, que desfaz uma decisão registrada (DESIGN.md §5.3 "Sem título geral", comentário em App.svelte:859 e movimento.test.mjs:262). A segunda parte só entra se o dono aprovar a mudança no DESIGN.md.

Status: pendente

### F-AN-07 — Ao recolher o painel, o botão salta cerca de 135 px no primeiro quadro e o chevron troca seco

- **Arquivo:** `frontend/src/App.svelte:871`
- **Lente:** AN
- **Gravidade:** baixa
- **Evidência:** `.sidebar-header-toggle` usa `justify-content: flex-end; padding: 8px 12px` (App.svelte:860-869) e a regra `.sidebar.collapsed .sidebar-header-toggle { justify-content: center; padding: 8px 0; }` (871-874) entra no mesmo quadro em que a largura começa a animar de ~320 px para 48 px (`transition: width var(--dur-fast)`, linha 845). Com o painel ainda em 320 px, o botão sai da borda direita (centro em ~295 px) para o centro do cabeçalho (~160 px) de uma vez e só então desliza. Na volta não há salto, então recolher e expandir ficam assimétricos. As capturas `1024x768-escuro-processando.png` (botão na borda direita) e `1024x768-claro-processando-recolhido.png` (botão centrado em 48 px) mostram as duas posições. O ícone troca por `{#if sidebarCollapsed} ChevronRight {:else} ChevronLeft` (488-492), sem transição.
- **Proposta:** Remover a regra de 871-874: com `flex-end` e `padding-right: 12px`, um botão de 24 px numa coluna de 48 px (47 px úteis) já fica centrado (11 px à esquerda, 12 px à direita), então ele acompanha a borda direita durante toda a animação, sem salto. Usar um único `ChevronLeft` com `transform: rotate(180deg)` quando recolhido e `transition: transform var(--dur-fast) var(--ease-out)`, como já faz `.chevron.aberto` em ProcessList.svelte:836-845. Opcional: adiantar o fade do conteúdo ao expandir (atraso de metade de `--dur-fast`), porque hoje são 150 ms de coluna vazia crescendo antes do conteúdo aparecer (linha 900).
- **Custo:** P
- **Ganho esperado:** Recolher e expandir ficam simétricos e contínuos; o botão sob o ponteiro não muda de lugar de repente; o giro do chevron indica a direção.
- **Veredito:** rebaixado
- **Justificativa do cético:** A regra .sidebar.collapsed .sidebar-header-toggle (App.svelte:871-874) troca justify-content e padding no mesmo quadro em que começa a transição de largura (845), e .btn-toggle-sidebar (876-891) não tem transição, então o salto do botão existe. O movimento todo dura 150 ms e o botão sai de baixo do ponteiro de qualquer forma; é polimento.

Status: pendente

### F-AN-08 — Painéis dentro da linha (atividade, reprocessar, detalhes do erro) abrem e fecham sem transição coerente

- **Arquivo:** `frontend/src/components/ProcessList.svelte:506`
- **Lente:** AN
- **Gravidade:** baixa
- **Evidência:** `<AtividadeItem>` (ProcessList.svelte:506-512) e `<ReprocessarItem>` (514-522) entram e saem sem `in:`/`out:`; só `.erro-detalhe` tem `in:fade` (499), e mesmo ele fecha seco. O painel de atividade chega a ~300 px (captura `1024x768-escuro-processando-atividade.png`) e aparece em um quadro, empurrando as linhas de baixo, enquanto o chevron do mesmo botão gira em 150 ms (836-845): dois elementos do mesmo gesto com tempos diferentes. Dentro do painel, "Carregando a atividade..." troca pelo conteúdo também sem transição (AtividadeItem.svelte:41-45).
- **Proposta:** Dar a mesma entrada aos três painéis: `in:fly={{ y: -4, duration: duracaoMovimento(DUR_FAST), easing: cubicOut }}` no elemento raiz (`.painel-atividade`, `.painel-reprocessar`, `.erro-detalhe`); a saída pode ser instantânea ou `out:fade` em `--dur-fast`. Não animar altura (`slide` anima layout): o deslocamento das linhas de baixo continua em um quadro, mas o conteúdo novo deixa de aparecer de uma vez. Em `prefers-reduced-motion`, `duracaoMovimento` já zera. Documentar em DESIGN.md §7 e cobrir no teste de movimento.
- **Custo:** P
- **Ganho esperado:** Abrir detalhes fica coerente com o giro do chevron e entre os três painéis, sem custo de layout por quadro.
- **Veredito:** rebaixado
- **Justificativa do cético:** Só .erro-detalhe tem in:fade (ProcessList.svelte:499); AtividadeItem (raiz na linha 40) e ReprocessarItem (form na linha 39) abrem e fecham sem transição (ProcessList.svelte:506-522). É polimento, e a proposta mantém o salto de layout da linha.

Status: pendente

### F-AN-09 — Documentos do painel de atividade mudam de posição aos saltos, até 5 vezes por segundo

- **Arquivo:** `frontend/src/components/AtividadeItem.svelte:95`
- **Lente:** AN
- **Gravidade:** baixa
- **Evidência:** `{#each visiveis as d (d.i)}` (AtividadeItem.svelte:95) tem chave mas nenhum `animate:` ou transição. `ordenarDocs` reordena por estado a cada instantâneo (atividade.js:34-40: OCR, lendo, erro, aguardando, concluído) e o backend manda `queue:atividade` a cada 200 ms (internal/etl/queue.go:692). Cada documento que começa a ser lido pula para o topo; ao concluir, some na hora (filtro `d.estado !== 'concluido'`, linha 32). Na captura `1024x768-escuro-processando-atividade.png` há 10 documentos visíveis em 4 estados; ao vivo essa ordem se refaz sem continuidade e o olho perde o documento que acompanhava.
- **Proposta:** Adicionar `animate:flip={{ duration: duracaoMovimento(DUR_FAST), easing: cubicOut }}` ao `<li class="doc">` e `out:fade={{ duration: duracaoMovimento(DUR_FAST) }}` para o documento que conclui (com `animate`, o Svelte fixa a posição do item que sai, então os demais deslizam ao mesmo tempo). Medir com um ZIP de centenas de documentos antes de fechar: o `flip` mede todos os `<li>` a cada instantâneo (ver F-DF-03); se pesar, desligar acima de N documentos ou paginar o painel. Incluir AtividadeItem.svelte nos testes de movimento.
- **Custo:** P
- **Ganho esperado:** O usuário consegue seguir um documento enquanto ele muda de estado; o painel deixa de piscar durante o OCR.
- **Veredito:** rebaixado
- **Justificativa do cético:** O each com chave em AtividadeItem.svelte:95 não tem animate, e ordenarDocs (atividade.js:34-40) reordena por estado a cada instantâneo de 200 ms (queue.go:692), então as linhas trocam de lugar sem movimento. O painel é opcional, a ordenação é intencional e um flip a 5 Hz não foi medido e pode virar movimento constante.
- **Nota da síntese:** O cético pede medir antes de fechar: um `flip` a 5 Hz pode virar movimento constante. O custo de reordenar a lista inteira está em F-DF-04, que convém resolver antes.

Status: pendente

### B-EF-04 — Regex executada em toda linha sem pré-filtro barato na limpeza

- **Arquivo:** `internal/etl/formatter.go:177`
- **Lente:** EF
- **Gravidade:** baixa
- **Evidência:** `cleanPage` testa `pageMarkerRegex.MatchString(line)` em toda linha (formatter.go:177; a regex é `(?i)^p[áa]g(ina|\.)?\s*\d+...$`, sem prefixo literal por causa do `(?i)`); `linhaLetraSolta` roda `romanoRegex` e `alineaRegex` antes de olhar o tamanho da linha (higienizacao.go:322), embora só linhas de 1-2 letras interessem; `paginaCertificado` roda duas regex `(?i)` em todas as linhas de todas as páginas no nível aggressive (higienizacao.go:359-362); `normalizeEdgeLine` roda `valorEdgeRegex` (`R\$|US\$|\d+,\d{2}|%`) em cada linha de borda (formatter.go:73). Perfil do `BenchmarkSintetico` (aggressive): `regexp.(*Regexp).find` 2,10 s dos 4,55 s de `GenerateConsolidatedOutput` (46%), `normalizeEdgeLine` 0,95 s, `paginaCertificado` 0,78 s, `linhaLetraSolta` 0,42 s. Micro-benchmarks por overlay (-count=6, 1000 páginas, 45.000 linhas): `filtrosPorLinha` (a condição da linha 177) 159-171 ms; a mesma condição com pré-filtro (`len(l) <= 40 && (l[0]=='p'||l[0]=='P')` antes de `pageMarkerRegex`, `len(l) <= 4` antes de `linhaLetraSolta`) 52-63 ms; `paginaCertificado` 89-189 ms.
- **Proposta:** Pôr um teste barato antes de cada regex, sem mudar a regra: em `cleanPage`, só chamar `pageMarkerRegex` se a linha começa com p/P e é curta; em `linhaLetraSolta`, sair cedo quando `len(l) > 4` (duas runas de até 2 bytes) antes das regex e dos mapas; em `paginaCertificado`, só chamar `certificadoCampoRegex` se o primeiro byte é E/V/A/N (ou minúsculo) e `certificadoTituloRegex` se a linha contém "ssinatura" (comparação sem caixa feita à mão); em `normalizeEdgeLine`, trocar a regex por `strings.ContainsAny(l, "$%")` mais uma busca manual de `dígito,dígito dígito`. Os testes de higienização existentes cobrem a equivalência.
- **Custo:** P
- **Ganho esperado:** -65% no filtro por linha (medido: ~165 ms para ~56 ms a cada 1000 páginas) e a maior parte dos ~90-190 ms de `paginaCertificado`; somados, cerca de -20% a -25% do tempo do formatador no nível aggressive e -10% no padrão (onde pesam `pageMarkerRegex` e `normalizeEdgeLine`).
- **Veredito:** rebaixado
- **Justificativa do cético:** As regex sem pré-filtro existem (formatter.go:73 e 177, higienizacao.go:322 e 359-362) e reproduzi `filtrosPorLinha` 92-130 ms contra 36-45 ms com pré-filtro e `paginaCertificado` 102-140 ms, mas isso só vale no nível Máxima: no padrão, `pageMarkerRegex || linhaSoSimbolos` custa 34-45 ms e o pré-filtro 30-40 ms (ganho dentro do ruído), restando só o `normalizeEdgeLine`. Além disso dois pré-filtros propostos mudam a regra, ao contrário do que o achado afirma: `len(l) <= 4` deixa de pegar `"ﬁﬁ"` (6 bytes, `linhaLetraSolta` = true hoje) e `ContainsAny(l, "$%")` passa a casar `"$ 100 pagos"`, que `valorEdgeRegex` não casa.
- **Nota da síntese:** Atenção do cético: dois dos pré-filtros propostos mudam a regra (`len(l) <= 4` e `ContainsAny(l, "$%")`). A correção precisa de pré-filtros estritamente equivalentes, com caso de teste para cada contraexemplo citado na justificativa.

Status: pendente

### F-AC-13 — Omitir tipos: campo de texto livre que exige lembrar os códigos e aceita qualquer coisa sem aviso

- **Arquivo:** `frontend/src/components/Settings.svelte:205`
- **Lentes:** VI, AC
- **Gravidade:** média
- **Evidência:**
  Pedido do dono em docs/backlog.md:13 ("o usuário digita os tipos e precisa lembrar de todos"). Settings.svelte:205-214 é um `<input type="text" placeholder="EXTR, COMP">` com a ajuda "Ex.: EXTR, COMP. O documento vira um aviso no texto e não é lido." Problemas conferidos:
  1. Nenhuma lista dos tipos que existem: o código vem do nome do arquivo (internal/etl/formatter.go:43, `tipoDocumento`: "Evento 3 - EXTR2.pdf" vira "EXTR") e não aparece em lugar nenhum da interface.
  2. `normalizarTipos` (linhas 43-49) só tira espaços e põe em maiúsculas: digitar "EXTRATO" ou "EXTR;COMP" é salvo sem aviso e nunca casa com nada.
  3. O placeholder repete o exemplo da ajuda; em 800x600-claro-configuracoes.png o campo vazio mostra "EXTR, COMP" em cinza e parece já configurado.
  4. Salva em `on:change` (linha 212), isto é, ao sair do campo; o único retorno é o toast genérico "Configurações salvas com sucesso!", sem dizer o que passou a ser omitido.
  5. A ajuda não está ligada ao campo (`aria-describedby` ausente) e o rótulo "Omitir documentos dos tipos" não diz que "tipo" é o código do nome do arquivo.
- **Proposta:** Trocar o campo por uma lista de chips marcáveis (`<button aria-pressed>` ou checkboxes estilizados, altura de 20-22px, `--fs-meta`), com o código e, no `title`/dica, a descrição ("EXTR: extrato"). A lista começa com um catálogo curto dos tipos mais comuns do eproc e cresce com os tipos já vistos nos processos, como o dono sugeriu; isso pede um binding novo no Go (por exemplo `GetTiposConhecidos`, alimentado no pré-povoamento do pipeline, onde `tipoDocumento` já roda) e por isso cruza para o backend. Um campo pequeno "Outro código..." cobre o que não está na lista e valida o formato (letras e dígitos) antes de salvar. Marcar/desmarcar salva na hora e o anúncio diz o efeito ("EXTR omitido"). Rótulo sugerido: "Não ler documentos dos tipos", com a ajuda ligada por `aria-describedby`. Etapa barata, sem backend: mostrar os códigos atuais como chips removíveis (ícone `X` de 12px), tirar o placeholder que imita valor e recusar código fora do formato. Movimento: chip entra com `fade` em `--dur-fast`; nada em `prefers-reduced-motion`.
- **Custo:** G
- **Ganho esperado:** O usuário escolhe em vez de lembrar e digitar, vê de relance o que está sendo omitido e não salva um código inválido sem perceber. A etapa barata (chips do que já está salvo e validação) sai com custo P.
- **Veredito:** rebaixado
- **Justificativa do cético:** Confirmado: Settings.svelte:205-214 é texto livre salvo em on:change, normalizarTipos (43-49) e config.NormalizarTipos só separam por vírgula e põem em maiúsculas, e o pipeline casa por igualdade exata com tipoDocumento (pipeline.go:379), então "EXTRATO" ou "EXTR;COMP" é gravado e nunca casa; o pedido está em docs/backlog.md:13. O recurso é opcional (padrão vazio) e o erro só faz o documento ser lido normalmente, sem perda: media, não alta.
- **Nota da síntese:** Fundido com o achado de VI "Omissão de tipos é um campo de texto livre: trocar por chips de tipos" (média, G, mantido), sobre Settings.svelte:205-214. Complementos de VI: chip marcado com `EyeOff` 12px, fundo `--accent-soft` e borda `--accent-line`, desmarcado com borda `--border-strong`; chip "+ Outro" (`Plus` 12px) abre o campo de texto atual; rótulo com contagem ("Omitir tipos (2)"); limite de duas linhas com "ver todos"; a parte visual pode começar com uma lista fixa em `src/lib`. A lista de tipos já vistos depende de um binding novo no Go, que cruza para o backend. A etapa barata descrita no ganho esperado (chips do que já está salvo e validação do formato) sai com custo P e pode ser aprovada sozinha.

Status: pendente

### F-AN-10 — Spinner, pulso e barras provavelmente seguem animando com a janela oculta na bandeja (inferência do código, falta medir)

- **Arquivo:** `frontend/src/components/ProcessList.svelte:1055`
- **Lente:** AN
- **Gravidade:** baixa
- **Evidência:** Conferido: ocultar passa por `runtime.WindowHide` (app.go:182), que no Wails v2.13.0 é só `ShowWindow(hwnd, SW_HIDE)` (internal/frontend/desktop/windows/frontend.go:316-320 e winc/controlbase.go:364-366); `chromium.Hide()` (IsVisible do WebView2) só é chamado no ajuste de inicialização (frontend.go:971). O frontend não escuta `visibilitychange` nem evento de janela oculta (busca em frontend/src sem resultado). Inferido, não medido: sem `IsVisible = false` o WebView2 continua se considerando visível, e os laços `spin` (ProcessList.svelte:1055), `pulse` (App.svelte:808), `pulsar` (AtividadeItem.svelte:333), as transições de progresso e o `requestAnimationFrame` de quadro.js continuam rodando durante um OCR longo com a janela na bandeja, disputando CPU/GPU com o tesseract. Sobre reduced-motion: revisei todas as animações e transições existentes e não encontrei nenhuma que o ignore (tokens zerados em style.css:79-86, `duracaoMovimento` nas transições do Svelte, pulso e `.pulsa` desligados).
- **Proposta:** Medir primeiro: CPU e GPU do `msedgewebview2.exe` com um ZIP em OCR, janela visível e depois oculta. Se confirmar, emitir um evento Wails ao ocultar e ao reabrir (em `App.Hide`/`antesDeFechar` e na ação Abrir) e, no frontend, pôr uma classe `oculta` em `:root` com `animation-play-state: paused` para `.spinner-small`, `.pulse-dot::after` e `.pulsa`; enquanto oculta, guardar só o último `queue:update`/`queue:item` sem aplicar, e aplicar uma vez ao reabrir. Não repetir no frontend o fade da janela, que já é feito em internal/janela.
- **Custo:** M
- **Ganho esperado:** Se a medição confirmar: nenhum trabalho de renderização com o app na bandeja, justamente quando o OCR usa todos os núcleos menos um.
- **Veredito:** mantido
- **Justificativa do cético:** No Wails v2.13.0, WindowHide é só mainWindow.Hide() (frontend.go:316-320), sem chromium.Hide(). Medi com build/bin/eprocia.exe em APPDATA isolado e um ZIP sintético em OCR: os processos msedgewebview2 somaram cerca de 23% de um núcleo com a janela visível e 23 a 26% com ela oculta na bandeja (processo de GPU 16 a 20%), ou seja, o custo de renderização não cai ao ocultar. Gravidade baixa; ao final encerrei a instância de teste e um tesseract.exe remanescente por nome de imagem.
- **Nota da síntese:** O título é o do revisor, que não tinha medido. O cético mediu depois (ver a justificativa): o custo dos processos do WebView2 não cai ao ocultar a janela, então o passo "medir primeiro" da proposta já está cumprido.

Status: pendente

### F-DF-03 — Cada queue:item refaz todas as linhas da fila e mede a posição de todas elas (animate:flip), não só a linha que mudou

- **Arquivo:** `frontend/src/components/ProcessList.svelte:307`
- **Lente:** DF
- **Gravidade:** baixa
- **Evidência:** O limitador por quadro (frontend/src/lib/quadro.js:21, `aplicar(trocas.size ? base.map((i) => trocas.get(i.id) ?? i) : base)`) entrega um array novo a cada flush, e o backend manda queue:item até 10 vezes por segundo (internal/etl/queue.go:695). Em ProcessList.svelte:307 o bloco é `{#each filteredItems as item (item.id)}` com `animate:flip` (linha 316). Conferi no Svelte instalado (5.57.1) e no componente compilado com svelte/compiler: (1) o each sai com flags 13 (ITEM_REACTIVE + CONTROLLED + ANIMATED, sem EACH_ITEM_IMMUTABLE, que o compilador só liga em modo runes: compiler/phases/3-transform/client/visitors/EachBlock.js:83-85); (2) em each.js:318, `if (item.v) internal_set(item.v, value)` roda para toda linha, e a fonte é `mutable_source` com `safe_equals` (reactivity/sources.js:117; equality.js:13-16 trata todo objeto como diferente), então até o item de identidade inalterada notifica; (3) cada linha compilada tem cerca de 30 reações que leem `$.get(item)` (13 template_effect, ~12 blocos if, 6 derived); (4) com animate, reconcile chama `measure()` em todas as linhas (each.js:466-478) e `apply()` em todas numa microtarefa (each.js:650-656), ou seja, 2 `getBoundingClientRect` por linha (transitions.js:102-108), o segundo depois das escritas no DOM (layout síncrono forçado). No mesmo flush ainda rodam O(N): `contarAtivos` duas vezes (App.svelte:97 e estado.js:28 via `avisoFila`), `filtrarItens` (ProcessList.svelte:63) e `mov.atualizar`, que cria dois Sets (fila.js:66-69). A fila não tem teto de itens (não há limite em internal/etl/queue.go). O custo em milissegundos não foi medido: os comandos permitidos não incluem navegador.
- **Proposta:** Separar progresso de estrutura. Ao passar ProcessList (e App) para runes, que é o passo que a migração já decidida facilita (hoje o projeto está no Svelte 5.57.1 em sintaxe legada): `items` vira `$state` e o queue:item aplica `Object.assign(itemExistente, novo)` em vez de trocar o array; assim o each não reconcilia, o flip não mede nada e só os textos e atributos da linha ativa rodam. O queue:update (mudança de estrutura) continua trocando a lista, e é só aí que o flip precisa medir. Em runes o each já usa igualdade estrita por item (EACH_ITEM_IMMUTABLE). Extrair a linha para um componente (LinhaFila.svelte) ajuda a isolar. `bind:items` em App.svelte:600 pode virar prop simples: ProcessList nunca atribui `items`. Atenção aos testes por regex: frontend/desempenho.test.mjs:85-88 exige `filaQuadro.lista(`/`filaQuadro.item(` e proíbe `items =` no trecho dos eventos; frontend/movimento.test.mjs:236 fixa o texto do `animate:flip`.
- **Custo:** M
- **Ganho esperado:** Por flush de progresso (até 10/s), o trabalho deixa de ser proporcional à fila: com 100 itens, de cerca de 3.000 reações e 200 getBoundingClientRect para as poucas reações da linha em processamento. Importa mais durante o OCR, quando o backend ocupa NumCPU-1 núcleos e a UI disputa o que sobra.
- **Veredito:** rebaixado
- **Justificativa do cético:** Evidência confirmada: quadro.js:21 entrega array novo, each.js:318 e 466-478 (Svelte 5.57.1) notificam e medem todas as linhas. Medi o bundle de produção em Chromium headless com stubs do Wails (build no scratchpad, nada no repo): cada flush de queue:item custa 2,3 ms com 5 linhas, 4,7 com 25, 7,1 com 50, 12 com 100 e 24,6 com 300, sempre com 2 getBoundingClientRect por linha. O custo é linear como alegado, mas só pesa com 100 ou mais itens e fica abaixo de um quadro até cerca de 130; nas filas usuais (até 25 a 50 itens; a varredura inicial só pega downloads recentes, internal/watchdog/pendentes.go:11) são no máximo 5 a 7% da thread principal a 10 flushes/s, e durante o OCR as páginas chegam mais devagar que isso.
- **Nota da síntese:** O achado equivalente da lente AN ("`animate:flip` mede todas as linhas da fila a cada evento de progresso") foi derrubado com base em uma página sintética (1,2 ms com 200 linhas). A medição deste, feita no bundle de produção, deu valores maiores (12 ms com 100 linhas) e é a que vale; mesmo assim só pesa em fila longa. Depende da passagem de ProcessList e App para runes.

Status: pendente

### F-DF-04 — Painel de atividade reordena, clona e reavalia todos os documentos a cada 200 ms, e mantém no DOM todos os que aguardam

- **Arquivo:** `frontend/src/components/AtividadeItem.svelte:27`
- **Lente:** DF
- **Gravidade:** baixa
- **Evidência:** O backend manda o instantâneo inteiro a cada 200 ms (internal/etl/queue.go:669 e 692; `Docs: slices.Clone(a.docs)` em internal/etl/atividade.go:154-166), não um delta. Com o painel aberto, AtividadeItem.svelte:27 faz `$: docs = ordenarDocs(atividade?.docs)`, e ordenarDocs (frontend/src/lib/atividade.js:36-40) faz `.map((d, i) => ({ ...d, i })).sort(...)`: um objeto novo por documento e uma ordenação completa a cada evento. Como todo objeto é novo, o `{#each visiveis as d (d.i)}` (linha 95) reexecuta todas as linhas; compilado, cada linha tem 4 template_effect, 3 blocos if e um Icone (3 componentes aninhados até o svg). `resumoAtividade` (linha 28) percorre a lista de novo. A linha 32, `visiveis = mostrarConcluidos ? docs : docs.filter((d) => d.estado !== 'concluido')`, deixa todos os documentos em 'aguardando' renderizados; a captura 1024x768-escuro-processando-atividade.png mostra as linhas de relógio (Evento 19 a 24, chip '6 aguardando') dentro da lista. A lista tem `max-height: 220px; overflow-y: auto` (linhas 250-251), cerca de 10 linhas visíveis, sem janela nem content-visibility. Num ZIP com centenas de documentos, quase todos começam aguardando: são centenas de `li` montados e reavaliados 5 vezes por segundo. Custo em milissegundos não medido.
- **Proposta:** (1) Estabilizar identidade e ordem: guardar os documentos num array `$state` (runes) e, a cada instantâneo, copiar para o objeto existente só os campos que mudaram (estado, feitas, ocr, erro); ordenar índices apenas quando algum `estado` mudou. Assim só as linhas dos documentos ativos (no máximo NumCPU-1) rodam. (2) Limitar o DOM: mostrar ativos e erros e apenas os primeiros K aguardando (por exemplo 20), com uma linha '+ N aguardando' que expande, no mesmo padrão do botão 'Mostrar concluídos'; ou, mais barato, `content-visibility: auto; contain-intrinsic-size: auto 20px` em `.doc`. (3) Calcular o resumo no mesmo laço da ordenação. Opcional entre camadas: o backend mandar só os documentos alterados desde a última versão (já existe `atv.Versao()`).
- **Custo:** M
- **Ganho esperado:** O trabalho por evento passa a ser proporcional aos documentos que mudaram, não ao total do ZIP; num ZIP de 400 documentos, de cerca de 2.800 reações e 400 clones a cada 200 ms para algumas dezenas. Abrir o painel num processo grande deixa de montar centenas de linhas de uma vez.
- **Veredito:** rebaixado
- **Justificativa do cético:** Evidência confirmada (AtividadeItem.svelte:27 e 32, atividade.js:36-40, lista com max-height 220px sem janela; a captura mostra as linhas aguardando). Medido com o painel aberto: cada queue:atividade custa 0,7 ms de script com 24 documentos, 1,4 ms com 100 e 4,0 ms com 400 (7,1 ms com layout), a 5 eventos/s, ou seja, no máximo cerca de 3,5% da thread principal no pior caso. O único custo perceptível é abrir o painel: 58 ms com 100 documentos e 219 ms com 400, uma vez por abertura.

Status: pendente

### B-EF-05 — Higienização e montagem da saída rodam em uma thread só depois da fase paralela

- **Arquivo:** `internal/etl/formatter.go:347`
- **Lente:** EF
- **Gravidade:** baixa
- **Evidência:** `ProcessZipComAtividade` chama `GenerateConsolidatedOutput(eventos, cfg.CleaningLevel)` na goroutine principal depois do `wg.Wait()` (pipeline.go:612-625), e o laço de documentos (formatter.go:421-483) limpa página por página em série. Perfil de CPU de `BenchmarkSintetico/ocr=none` (-benchtime=40x, 20,34 s de amostras): `GenerateConsolidatedOutput` 4,55 s = 86% da goroutine principal (5,27 s), cerca de 114 ms dos ~279 ms por ZIP (40% do tempo de relógio), enquanto as 11 goroutines de documento somam 6,79 s divididos entre os núcleos. Micro-benchmark injetado por `go test -overlay` (arquivo scratchpad da sessão, ef/zz_ef_test.go, -count=6) com 1000 páginas fictícias de 45 linhas (100 documentos): `BenchmarkEFFormatador` none 14-21 ms, basic 160-189 ms, standard (padrão do app) 680-853 ms, aggressive 885-1082 ms, com 107-113 MB alocados por operação para 3,7 MB de saída. A leitura nativa das mesmas páginas custa cerca de 0,38 ms/página (BenchmarkEFFonte/memoria: 30 páginas em 10,8-12,7 ms) e é dividida por 11 workers, então, em ZIP de texto nativo, o formatador serial passa a ser a maior parcela do tempo. Limpando os mesmos 100 documentos em goroutines (semáforo de 11, um `ctxDocumento` por documento): `cleanPage-aggressive` 523-695 ms contra `cleanPage-aggressive-paralelo` 103-117 ms.
- **Proposta:** Limpar os documentos em paralelo: a pré-passada H7 (`duplicadasPorConteudo`) já é calculada antes do laço e o `ctxDocumento` é por documento (formatter.go:440), então cada documento pode produzir o seu trecho de saída numa goroutine (limitada a `numWorkers`), e a goroutine principal só concatena na ordem de evento/índice. Alternativa equivalente: fazer a limpeza no fim da goroutine de cada documento no pipeline, e deixar para o fim só H7 e a concatenação. A ordem das páginas dentro do documento precisa continuar serial (timbre em `jaVistas`, marcadores em `assinados`).
- **Custo:** M
- **Ganho esperado:** Fase de formatação 4-5x mais rápida em 12 núcleos lógicos (medido 5,4x só na limpeza): de ~0,7-1,0 s para ~0,2 s a cada 1000 páginas; cerca de -30% do tempo total por ZIP sem OCR no benchmark sintético. Sem efeito relevante em ZIP dominado por OCR.
- **Veredito:** rebaixado
- **Justificativa do cético:** A formatação é mesmo serial depois do `wg.Wait()` (pipeline.go:612-625, formatter.go:421-483) e o perfil do revisor confere (GenerateConsolidatedOutput 2,77 s de 3,19 s da goroutine principal), mas reproduzi `GenerateConsolidatedOutput` em 384-444 ms por 1000 páginas no nível padrão (não 680-853) e o paralelo só 3,6x mais rápido na limpeza (357-413 ms contra 103-108 ms), com a pré-passada H7 (~75-100 ms) e a concatenação ainda seriais. O ganho fica abaixo de ~0,3 s por 1000 páginas, encolhe se B-EF-02 e B-EF-04 (custo P) entrarem antes, e custa M numa parte sensível a ordem e estado.
- **Nota da síntese:** Só faz sentido medir de novo depois de B-EF-02 e B-EF-04, que reduzem o mesmo trecho com custo P.

Status: pendente

### F-VI-08 — Toast do painel cobre o interruptor que o disparou e não tem ícone de estado

- **Arquivo:** `frontend/src/components/Settings.svelte:458`
- **Lente:** VI
- **Gravidade:** baixa
- **Evidência:** `.toast-regiao { position: absolute; bottom: 0; left: 0; right: 0; }` (Settings.svelte:458-463) fica dentro de `.settings-card { padding: 0 0 8px; ... position: relative; overflow: hidden; }` (264-271), cujo último controle é a linha de 32px "Iniciar com o Windows" (218-229). O toast tem cerca de 34px (padding 8+8, linha de 11px x 1,5, borda de 1px), então cobre os 8px de padding e 26 dos 32px dessa linha: o próprio interruptor recém-clicado. O de erro fica até ser fechado. Avaliado pelo código, não há captura com toast. DESIGN.md §5.3 pede o toast "no rodapé do painel". Os avisos só têm texto e cor: `<span>{statusMessage}</span>` (242), App.svelte:649, ProcessList.svelte:538. São três implementações com CSS repetido (Settings.svelte:465-509, ProcessList.svelte:548-585, App.svelte:1199-1235).
- **Proposta:** Um componente `Aviso.svelte` (ícone `CircleCheck` ou `CircleAlert` 12px + texto + fechar), numa região fixa no pé do painel, fora da área rolável, e no pé da lista; como cartão flutuante pode levar `--shadow-sm`. Mantém `fly`/`fade` com `duracaoMovimento`. Com Svelte 5, props e snippets deixam um componente servir os três lugares. Ajustar anuncios.test.mjs:60 e contraste.test.mjs:88.
- **Custo:** M
- **Ganho esperado:** O retorno de "salvo" deixa de esconder o controle, sucesso e erro se distinguem também pela forma e some a triplicação de CSS.
- **Veredito:** rebaixado
- **Justificativa do cético:** A geometria confere: `.toast-regiao` absoluta em `bottom: 0` dentro do cartão (Settings.svelte:458-463), toast de cerca de 33,5px sobre 8px de padding e a linha de 32px do último interruptor; avisos sem ícone e CSS repetido em três arquivos também conferem. Porém o de sucesso some em 3 s (lib/aviso.js) e o de erro tem botão fechar, então a cobertura é passageira; não há captura do estado.

Status: pendente

### F-VI-09 — Linha da fila trunca o número do processo enquanto mensagem redundante e rótulos ocupam espaço; contadores podem usar ícones

- **Arquivo:** `frontend/src/components/ProcessList.svelte:357`
- **Lente:** VI
- **Gravidade:** baixa
- **Evidência:** Capturas: 1024x768-claro-erro.png (item em processamento: "5000000-00.2026.4.00.00..." + "Processando document..." + "Docs: 43/57 | Págs: 107/1...", três textos cortados); 800x600-claro-processando-divisor-minimo.png ("5000000-00.20..." e um "F" solto, resto da mensagem); 800x600-claro-fila-reprocessar.png (os cinco nomes terminam em "...", e o número CNJ difere justamente no fim). Código: `<span class="progress-message">{item.message || 'Processando...'}</span>` seguido de `Docs: {..}/{..} | Págs: {..}/{..}` (ProcessList.svelte:357-360): a mensagem "Processando documento 24 de 57" repete "Docs: 24/57". `.file-name { flex: 1 1 auto; min-width: 96px; }` (851-860) encolhe junto com `.details-row` (863-873). O selo de sucesso sai "142 pág| ~98.450 tokens" (353-354), com espaço desigual em volta da barra.
- **Proposta:** (1) O nome tem prioridade: os detalhes encolhem antes (`flex-shrink` alto em `.details-row`), a `.progress-message` some sempre que há contadores (ou sobe a container query de 480px para cerca de 720px) e a extensão ".zip" sai do texto exibido. (2) Contadores com ícone no lugar de "Docs:", "Págs:" e das barras verticais: `Files` 12px + "24/57", `FileText` 12px + "60/142", tokens abreviados ("~98 mil") com o valor exato no `title`; `tabular-nums`, sem encolher, nomes em `.sr-only`. (3) Tirar o itálico de "Aguardando na fila..." (935-938): o relógio e o `.sr-only` já dizem isso. Atualizar DESIGN.md §5.1 e os testes densidade/layout.
- **Custo:** M
- **Ganho esperado:** O identificador do processo fica inteiro em 1024x768 e legível em 800x600; os contadores ocupam cerca de metade da largura e não cortam o total de páginas.
- **Veredito:** rebaixado
- **Justificativa do cético:** Os três textos cortados aparecem em 1024x768-claro-erro.png, e o "F" solto e o "pág|" nas capturas de 800x600 e em ProcessList.svelte:353-360. Mas "Processando documento 24 de 57" é texto do stub (scripts/visual/estados.mjs:49); o real é "Processando \<doc> (pág. x/y) [Docs: d/t]" (pipeline.go:563), também redundante. E o número CNJ real difere nos 7 primeiros dígitos, que seguem visíveis, não no fim; o nome completo está no `title`.

Status: pendente

### F-AN-11 — Conclusão e erro de um item não têm retorno: o ícone troca seco e a barra some antes de chegar ao fim

- **Arquivo:** `frontend/src/components/ProcessList.svelte:379`
- **Lente:** AN
- **Gravidade:** baixa
- **Evidência:** O ícone de estado é uma cadeia `{#if estado === 'processando'} ... {:else if estado === 'sucesso'}` sem transição (ProcessList.svelte:320-332). A barra está em `{#if estado === 'processando'}` (379-390) e é desmontada no mesmo quadro em que o status vira `success`/`error`, interrompendo a transição de 220 ms em curso (linha 973); o usuário nunca vê a barra completa. Os botões de ação e o selo de contadores também trocam no mesmo quadro (351-369, 394-492). Terminar um ZIP é o evento que o usuário espera, muitas vezes olhando outra janela, e a linha só muda de ícone de 16 px.
- **Proposta:** (1) Ícone: envolver em `{#key estado}` com `in:scale={{ start: 0.6, duration: duracaoMovimento(DUR_FAST), easing: cubicOut }}`. Por ser transição local dentro do `{#key}`, só roda quando o estado muda, não na montagem da linha nem ao filtrar. (2) Barra: `out:fade={{ duration: duracaoMovimento(DUR_BASE) }}` no contêiner, para sair suave em vez de sumir. (3) Realce único da linha: `::after` com `inset: 0`, `background-color: var(--success-soft)` (ou `--error-soft`), `pointer-events: none` e uma animação de opacidade 1 para 0, uma vez, com token novo `--dur-realce` (~600 ms), desligada em `prefers-reduced-motion`. Aplicar só a itens que mudaram de estado nesta sessão: estender `criarMovimentoFila` (fila.js) com `terminou(id)`, comparando o status anterior, testável em `node --test`. Sem tremor nem brilho; apenas tom de fundo, como DESIGN.md §2 já permite. Com runes, `$effect.pre` facilita comparar o status anterior sem variável auxiliar. Ampliar o teste `transições do Svelte passam por duracaoMovimento` para aceitar `scale`.
- **Custo:** M
- **Ganho esperado:** O término (ou a falha) de um ZIP fica perceptível de relance, só com transform e opacity, sem aumentar a altura da linha.
- **Veredito:** rebaixado
- **Justificativa do cético:** Confirmado que a barra é desmontada sem transição (ProcessList.svelte:379-390) no mesmo broadcast em que queue.go:543-545 grava Status=success e Progress=1.0. Mas o fim já é sinalizado pela notificação do Windows em app.go (aoTerminar) e pela troca de ícone, cor e ações da linha.

Status: pendente

### F-AN-12 — Troca de tema acontece em três velocidades (0, 150 e 220 ms) em vez de uma transição única

- **Arquivo:** `frontend/src/App.svelte:273`
- **Lente:** AN
- **Gravidade:** baixa
- **Evidência:** `toggleTheme` (App.svelte:273-277) só troca a classe `light` (App.svelte:105-107); a animação depende de cada regra CSS. Fundo em 220 ms: `body` (style.css:110), `.custom-titlebar` (App.svelte:719), `.sidebar` (845), `footer` (1132), `.list-card` (ProcessList.svelte:596). Em 150 ms: `.item-row` (ProcessList.svelte:791). Sem transição nenhuma: `.list-header` com `background-color: var(--surface)` próprio (ProcessList.svelte:599-607), `select` e `input` do painel (Settings.svelte:331-362), `.painel-atividade` (AtividadeItem.svelte:131-144), `.collapsed-tool-btn` (App.svelte:1045, só `color` e `border-color`) e todo texto com cor explícita (`--text-secondary`, chips, ícones). O cabeçalho da fila, os campos e os textos viram na hora enquanto as superfícies ao redor ainda estão no meio do caminho. Cada transição de `background-color`/`border-color` também repinta a cada quadro.
- **Proposta:** Fazer a troca com a View Transitions API (WebView2 é Chromium; detectar `document.startViewTransition`): `document.startViewTransition(async () => { temaPref = ...; await tick(); })` em `toggleTheme` e em `mudouTemaDoSistema`. Em style.css: `::view-transition-old(root), ::view-transition-new(root) { animation-duration: var(--dur-base); animation-timing-function: var(--ease-out); }` e `::view-transition { pointer-events: none; }`. Pular a chamada quando `duracaoMovimento(DUR_BASE) === 0`. Remover as transições de `background-color`/`border-color` em `--dur-base` das superfícies (body, titlebar, sidebar, manual-card, footer, list-card) ou neutralizá-las com uma classe `trocando-tema` durante a troca. O crossfade anima só a opacidade de duas capturas, no compositor. Atualizar DESIGN.md §7 e os testes que fixam essas regras. O spinner congela por 220 ms durante a captura, o que é aceitável.
- **Custo:** M
- **Ganho esperado:** Uma troca de tema uniforme de 220 ms, sem elementos virando antes dos outros, e sem repintura por quadro em seis superfícies.
- **Veredito:** rebaixado
- **Justificativa do cético:** Conferi as velocidades mistas na troca de tema: 220 ms em body (style.css:110), titlebar (App.svelte:719), sidebar, footer e .list-card; 150 ms em .item-row (ProcessList.svelte:791); instantâneo em .list-header (599-607), nos campos de Settings e em .painel-atividade. O descompasso existe, mas é uma ação rara que dura 220 ms e o ganho de repintura alegado é desprezível.

Status: pendente

### F-AC-14 — Barra recolhida: OCR e higienização são botões de ciclo com três pontinhos, sem mostrar as opções

- **Arquivo:** `frontend/src/App.svelte:540`
- **Lente:** AC
- **Gravidade:** baixa
- **Evidência:** App.svelte:540-570: cada clique chama `ciclarOcr`/`ciclarLimpeza` (linhas 220-228), que avança para a próxima de 4 opções e chama `SaveConfig` na hora. Para ir de "Preciso" a "Normal" são 3 cliques e 3 gravações, passando por "Desativado". O estado visível são três pontos de 4px (`.nivel-ponto`, linhas 1014-1024); o nome da opção só existe no `title` ("OCR: Normal. Clique para Preciso") e no `aria-label`. A confirmação da troca vai só para a região `.sr-only` (`anuncio`, linha 641): quem enxerga vê um ponto mudar de cor. Em 1024x768-claro-processando-recolhido.png os dois botões são ícones com três pontos azuis iguais, sem pista do que são nem de qual nível está ativo. Os ícones (`ScanText`, `Sparkles`) também não aparecem ao lado dos rótulos no painel aberto, então não há como aprender a associação.
- **Proposta:** Trocar o ciclo por um menu curto: o botão abre, à direita, um cartão flutuante com as 4 opções e a descrição de cada uma (o mesmo conteúdo do controle segmentado, em coluna), com `aria-haspopup`/`aria-expanded`, foco na opção atual, setas para mover, Enter para escolher, Esc para fechar e devolver o foco ao botão. Usar a Popover API (`popover` + posicionamento por âncora, disponíveis no WebView2 atual) para não ser cortado pelo `overflow: hidden` da barra. Entrada com `fade` e deslocamento de 4px em `--dur-fast`; sem deslocamento em `prefers-reduced-motion`. Ao escolher, mostrar o nome da opção por 1,5 s ao lado do botão, além do anúncio que já existe. Usar os mesmos ícones ao lado dos rótulos "Processamento de Imagens (OCR)" e "Higienização de texto" no painel aberto. Em Svelte 5, um attachment (`{@attach}`) cuida de fechar ao clicar fora e de devolver o foco.
- **Custo:** M
- **Ganho esperado:** Qualquer opção a um clique e uma gravação, com o nome e a explicação à vista; o usuário deixa de passar por "Desativado" sem querer no meio do ciclo.
- **Veredito:** rebaixado
- **Justificativa do cético:** Confirmado: ciclarOcr/ciclarLimpeza (App.svelte:220-228) avançam com proximaOpcao circular e gravam a cada clique, de Preciso a Normal são 3 cliques passando por Desativado, e o estado visível são três pontos de 4px (captura processando-recolhido). Mas é um atalho da barra recolhida, com title/aria-label corretos e o painel completo a um clique, e a proposta se sobrepõe ao controle segmentado de F-VI-02: baixa.
- **Nota da síntese:** Depende do componente de F-VI-02, cujo conteúdo o menu reaproveita.

Status: pendente

### F-AC-15 — Fila: cinco paradas de Tab por linha, sem setas nem atalhos

- **Arquivo:** `frontend/src/components/ProcessList.svelte:337`
- **Lente:** AC
- **Gravidade:** baixa
- **Evidência:** Cada linha concluída tem cinco elementos na ordem de tabulação: o botão do nome (`svelte:element this="button"`, linhas 337-376) e reprocessar, copiar, abrir, remover (linhas 414-453). Com 20 processos são cerca de 100 Tabs para chegar ao último, e Shift+Tab a partir do rodapé percorre tudo de volta. O único `on:keydown` do frontend fora do divisor é o Esc do ReprocessarItem (busca por `keydown` em frontend/src: App.svelte:592 e ReprocessarItem.svelte:39). Não há setas entre linhas, Delete para remover nem Ctrl+F para o filtro (linhas 258-264).
- **Proposta:** Tratar a lista como um conjunto com foco itinerante: uma parada de Tab para a lista; setas para cima/baixo movem entre linhas (foco no botão do nome ou na própria linha com `tabindex`), setas esquerda/direita entre as ações da linha, Home/End para a primeira e a última, Enter abre a atividade, Delete remove (ou cancela), Ctrl+C copia o TXT, Ctrl+F leva ao filtro e Esc no filtro limpa o texto. A lógica de "qual é o próximo foco" fica em `src/lib/fila.js`, ao lado de `alvoFoco`, com teste em fila.test.mjs. Documentar as teclas no DESIGN.md §5.1, como já é feito para o divisor. Em Svelte 5 as ações da linha viram um snippet, e o `tabindex` itinerante sai de um `$state` único em vez de mapas recriados a cada clique.
- **Custo:** M
- **Ganho esperado:** Chegar a qualquer processo e copiar o TXT com poucas teclas; a fila longa deixa de ser uma barreira para quem não usa o mouse.
- **Veredito:** rebaixado
- **Justificativa do cético:** Os fatos conferem (5 paradas de Tab por linha concluída em ProcessList.svelte:337-453; keydown só em App.svelte:592 e ReprocessarItem.svelte:39), mas a navegação por Tab funciona e nenhum critério é violado: é uma melhoria de eficiência (foco itinerante e atalhos) de custo M, não uma barreira. Baixa.

Status: pendente

### F-AC-16 — Nomes dos ícones só no `title`: não aparecem no foco por teclado e não dizem de qual processo são

- **Arquivo:** `frontend/src/components/ProcessList.svelte:395`
- **Lente:** AC
- **Gravidade:** baixa
- **Evidência:** Os botões da linha são ícones de 14px sem texto. Quatro deles têm só `title`, sem `aria-label`: cancelar (linhas 395-402), "Remover da fila" (403-411), "Abrir pasta contendo o TXT" (438-445) e "Remover da lista" (446-453, 483-490); o DESIGN.md §2 pede `aria-label` em todo botão só com ícone. Vale o mesmo para a lixeira de Settings.svelte:163. O `title` nativo demora e nunca aparece no foco por teclado, então quem navega com Tab vê quatro ícones cinza sem nome (1024x768-escuro-fila-reprocessar.png). Nenhum nome inclui o arquivo: com leitor de tela são N botões "Remover da lista" iguais, e as linhas são `<div class="item-row">` dentro de `<div class="items-list">` (linhas 306-310), sem semântica de lista (busca por `role="list"` em frontend/src: nenhum resultado). Informação que só existe em `title`: caminho do ZIP (linha 348), explicação da estimativa de tokens (354), dos medidores "Documentos em paralelo" e "Processos de OCR" (AtividadeItem.svelte:48 e 57) e o erro completo de cada documento (AtividadeItem.svelte:96, cortado em `.doc-erro`), ao contrário do erro do item, que o DESIGN.md exige acessível por teclado.
- **Proposta:** 1. Dica própria, sóbria: um attachment/ação `dica` que mostra um rótulo de `--fs-meta` sobre `--surface` com borda `--border-strong` e `--shadow-sm`, em `:hover` (com atraso de `--dur-fast`) e em `:focus-visible` (sem atraso), com `fade` em `--dur-fast` e nada em `prefers-reduced-motion`. Como `.list-card` e `.settings-card` têm `overflow: hidden`, usar um único elemento na camada superior (Popover API) em vez de `::after`. Tirar o `title` onde a dica entra, para não duplicar; os testes que fixam `title=` (painel.test.mjs:133, tema.test.mjs:141, estado.test.mjs:31) precisam acompanhar. 2. Nomes acessíveis: `aria-label` em todos, com o arquivo ("Remover 5000000-...zip da lista"). 3. Semântica: `role="list"` em `.items-list` e `role="listitem"` em `.item-row`. 4. Erro por documento: tornar a linha com erro expansível ou mostrar o texto inteiro com quebra, selecionável, como em `.erro-detalhe`.
- **Custo:** M
- **Ganho esperado:** Os ícones passam a se explicar para mouse e teclado do mesmo jeito, o leitor de tela diz de qual processo é cada botão e quantos itens há, e nenhuma informação fica presa ao ponteiro.
- **Veredito:** rebaixado
- **Justificativa do cético:** Confirmado que cancelar, remover, abrir pasta (ProcessList.svelte:395-453, 483-490) e a lixeira de Settings.svelte:163 têm só title, que não há role="list" e que o erro por documento fica cortado em .doc-erro com o texto inteiro apenas no title (AtividadeItem.svelte:96, 296-303). Porém o title já serve de nome acessível para o leitor de tela, então a afirmação de botões sem nome é exagerada; sobra descumprimento da convenção de aria-label do DESIGN.md:20 e falta de dica no foco por teclado: baixa.

Status: pendente

## Capturas

As capturas "antes" foram feitas com dados fictícios, em 800x600, 1024x768, 1440x900 e 1920x1080, nos temas claro e escuro, nos estados vazio, fila, fila com o painel de reprocessar, processando, processando com o painel de atividade, erro, configurações, processando com o painel recolhido e processando com o divisor no mínimo e no máximo. Ficam em [`capturas/2026-11-10/antes/`](capturas/2026-11-10/antes/). As citadas nos achados:

- [800x600-claro-configuracoes.png](capturas/2026-11-10/antes/800x600-claro-configuracoes.png): "Iniciar com o Windows" cortado, selects nativos, campo de tipos e caminho da pasta (F-VI-01, F-VI-02, F-AC-03, F-AC-13).
- [800x600-claro-erro.png](capturas/2026-11-10/antes/800x600-claro-erro.png): mesmo corte do cartão de configurações (F-VI-01).
- [800x600-claro-processando-divisor-minimo.png](capturas/2026-11-10/antes/800x600-claro-processando-divisor-minimo.png): ajuda cortada no meio com o painel a 280px e nome do processo truncado (F-VI-01, F-VI-09).
- [800x600-claro-fila-reprocessar.png](capturas/2026-11-10/antes/800x600-claro-fila-reprocessar.png): os cinco nomes terminam em reticências (F-VI-09).
- [800x600-escuro-processando-recolhido.png](capturas/2026-11-10/antes/800x600-escuro-processando-recolhido.png): ponto de monitoramento solto na barra recolhida (F-AC-09).
- [1024x768-claro-fila.png](capturas/2026-11-10/antes/1024x768-claro-fila.png): selo de contadores lavado, ações com o mesmo peso, botões da janela e "Pausar" ao lado de "Limpar Fila" (F-AC-02, F-AC-01, F-VI-05, F-AN-02).
- [1024x768-claro-erro.png](capturas/2026-11-10/antes/1024x768-claro-erro.png): linha com erro sem destaque e com dois ícones vermelhos, chip "Fila pausada", textos cortados na linha em processamento (F-VI-03, F-AC-02, F-VI-09, F-AC-06).
- [1024x768-escuro-erro.png](capturas/2026-11-10/antes/1024x768-escuro-erro.png): "Falha" em vermelho de baixo contraste e chip que desloca a lista (F-AC-02, F-AN-05).
- [1024x768-claro-configuracoes.png](capturas/2026-11-10/antes/1024x768-claro-configuracoes.png): padding de 8px, rótulos sem ícone, selects e linha da pasta (F-VI-07, F-VI-04, F-VI-02, F-AC-03).
- [1024x768-claro-fila-reprocessar.png](capturas/2026-11-10/antes/1024x768-claro-fila-reprocessar.png): selects nativos no painel de reprocessar (F-VI-02).
- [1024x768-escuro-fila-reprocessar.png](capturas/2026-11-10/antes/1024x768-escuro-fila-reprocessar.png): ícones de ação sem nome visível e linha "Cancelado" (F-AC-16, F-AC-12).
- [1024x768-claro-processando-recolhido.png](capturas/2026-11-10/antes/1024x768-claro-processando-recolhido.png): barra recolhida: ponto de monitoramento, botões de ciclo com três pontos e botão de recolher centrado (F-AC-09, F-AC-14, F-AN-07).
- [1024x768-escuro-processando.png](capturas/2026-11-10/antes/1024x768-escuro-processando.png): botão de recolher na borda direita e lista sem o chip (F-AN-07, F-AN-05).
- [1024x768-escuro-processando-atividade.png](capturas/2026-11-10/antes/1024x768-escuro-processando-atividade.png): painel de atividade com 10 documentos em 4 estados e as linhas aguardando (F-AN-08, F-AN-09, F-DF-04).
- [1440x900-claro-processando-atividade.png](capturas/2026-11-10/antes/1440x900-claro-processando-atividade.png): chips e textos coloridos do painel de atividade (F-AC-02).
- [1920x1080-claro-vazio.png](capturas/2026-11-10/antes/1920x1080-claro-vazio.png): estado vazio sem botão numa área grande (F-AC-08).

O que as capturas não mostram e foi avaliado só pelo código: toasts e avisos (F-VI-08, F-AN-06), overlay de arrasto (F-AN-01), barras de rolagem, que o capturador esconde (F-VI-06), estados de hover, foco e pressionado (F-AN-02), modo de alto contraste (F-AC-10) e todo o movimento (lente AN).

## Teste manual guiado

As capturas rodam no navegador com os bindings do Wails simulados, então não cobrem a janela nativa nem o movimento. Antes de aprovar e depois de corrigir, vale conferir no `wails dev` ou no executável de `build/bin/`, sempre com ZIPs fictícios:

1. **Bordas da janela frameless.** Redimensionar pelas quatro bordas e pelos quatro cantos até o mínimo de 800x600. O cursor de redimensionamento deve aparecer em toda a borda e a janela não pode ficar abaixo do mínimo. Em 800x600, rolar o painel lateral e confirmar que "Iniciar com o Windows" é alcançável (F-VI-01). Com um histórico longo (100 itens ou mais), observar se o redimensionamento engasga (F-DF-02).
2. **Maximizar e restaurar.** Usar o botão da barra de título, o duplo clique na barra e Win+Seta para cima/baixo. O layout deve se reajustar sem salto, o rótulo do botão deve refletir o estado e, com a janela maximizada, levar o ponteiro ao canto superior direito e clicar: hoje o clique cai na área arrastável, não em Fechar (F-VI-05).
3. **Arrastar a barra de título.** Mover a janela entre monitores de escalas diferentes (100% e 150%, se houver) e arrastá-la para a borda superior para o snap. Nenhum botão da barra pode iniciar o arraste por engano; depois de F-VI-05, conferir que os botões de 44x38px não roubam a área de arraste ao lado.
4. **Divisor nos limites.** Arrastar rápido para além de 280px e do máximo, soltar o botão fora da janela e trocar de janela com Alt+Tab no meio do arrasto. O painel deve encostar no limite e encerrar o arrasto. Repetir pelo teclado (Tab até o divisor, setas, Home e End) e ouvir o rótulo no leitor de tela (F-AC-11). Com a fila cheia, o arrasto deve seguir fluido (F-DF-02); a 280px, conferir se os quatro segmentos de OCR e higienização cabem sem cortar (F-VI-02).
5. **Recolher e expandir o painel.** Clicar várias vezes seguidas no botão de recolher e observar o salto do botão no primeiro quadro e a troca seca do chevron (F-AN-07). Na barra recolhida, usar o controle de monitoramento e os de OCR e higienização (F-AC-09, F-AC-14).
6. **Arrastar e soltar.** Arrastar para a janela um texto selecionado (por exemplo, do detalhe de um erro), um arquivo que não é ZIP, um ZIP e uma mistura dos dois. Observar quando o overlay aparece, o diálogo nativo e a falta de confirmação do que foi aceito (F-AN-01, F-AC-07).
7. **Copiar, limpar e reprocessar.** Copiar o TXT e conferir o retorno com o ponteiro parado e fora do botão, e com leitor de tela (F-AC-01). Clicar em "Limpar Fila" com um item em processamento (F-AC-06); remover o último item da lista (F-AN-04); pausar e retomar olhando o deslocamento da lista (F-AN-05). Abrir o painel Reprocessar, sair com Esc e com Cancelar e confirmar, vendo onde o foco vai parar (F-AC-04).
8. **Painel de configurações.** Mudar dois ou três controles em menos de 3 s e ver se cada salvamento dá retorno (F-AN-06) e se o toast cobre o interruptor "Iniciar com o Windows" (F-VI-08). Clicar na lixeira da pasta monitorada e conferir o que acontece com a pasta e com o rodapé (F-AC-03).
9. **Movimento e tema.** Trocar o tema e observar cabeçalho da fila, campos e textos virando antes das superfícies (F-AN-12). Abrir e fechar os painéis da linha (F-AN-08) e acompanhar o painel de atividade durante um OCR (F-AN-09, F-DF-01). Repetir com "efeitos de animação" do Windows desligados: nada deve se mover.
10. **Bandeja durante o OCR.** Com um ZIP em OCR, ocultar a janela para a bandeja e comparar, no Gerenciador de Tarefas, a CPU dos processos `msedgewebview2.exe` com a janela visível e oculta (F-AN-10). Reabrir pelo duplo clique e conferir que a fila mostra o estado atual.
11. **Desempenho do backend.** Para B-EF-01, processar o mesmo ZIP fictício com páginas escaneadas antes e depois e comparar o texto de saída (carimbos e destaques coloridos são o risco da conversão para cinza). Para B-EF-02 a B-EF-05, o gate é o benchmark (`-count=10`, benchstat com p < 0,05) e a saída idêntica byte a byte.
12. **Alto contraste e teclado.** Ativar um tema de contraste do Windows e conferir interruptores, pontos de nível e barras (F-AC-10). Percorrer a fila só com Tab e anotar quantas paradas são necessárias para chegar ao último item (F-AC-15) e se os ícones se explicam no foco (F-AC-16).

## Derrubados

Ficam registrados para auditoria. Não entram na correção.

| Título | Lente | Justificativa do cético |
|--------|-------|-------------------------|
| `animate:flip` mede todas as linhas da fila a cada evento de progresso | F-AN | O mecanismo existe (Svelte 5.57.1, each.js:466-479 e 650-656: dois getBoundingClientRect por linha a cada reconciliação), mas medi numa página sintética com o mesmo padrão: o custo extra por atualização é de 0,1 ms com 20 linhas e 1,2 ms com 200 (11 ms só com 1000), a no máximo 10 atualizações por segundo. Como a fila vive só em memória por sessão, o ganho é de cerca de 1% de um núcleo no pior caso plausível; desprezível diante do custo da mudança. |
| Na abertura, todo o histórico entra com `fly` de uma vez | F-AN | Cenário inalcançável: a fila não é persistida (etl.NewQueue em App.startup) e, na abertura, GetQueue (App.svelte:123) devolve nada ou os poucos ZIPs da varredura de 24 h do watchdog, que são de fato novos. Depois disso a janela vai para a bandeja sem remontar o frontend, então não há 'dezenas ou centenas' de linhas entrando com fly. |
| queue:atividade é recebido e guardado mesmo sem painel aberto, e o último instantâneo de cada item terminado fica retido | F-DF | O trecho existe (App.svelte:138-140), mas o ganho no frontend é desprezível: com o painel fechado medi 0,1 ms por evento (handler + flush + layout), mesmo com instantâneo de 400 documentos. A retenção é de um instantâneo por item ainda na fila, que é o que alimenta o painel de itens terminados sem nova chamada. A parte que poderia custar (serialização e ExecJS, wails frontend.go:632) é mudança de backend, fora desta lente, e não foi medida. |
| Cada evento resize da janela dispara uma chamada IPC WindowIsMaximised, sem coalescer | F-DF | O código é como descrito (App.svelte:388 e 281-287), mas a chamada é só um IsMaximised no Go (wails systemcalls.go:39-40) e acontece apenas durante o gesto de redimensionar, quando o layout da própria janela custa de 2 a 11 ms por quadro (medido); o ganho é desprezível. Não deu para medir o IPC sem o app real, e o timer proposto ainda atrasaria o rótulo do botão em 150 ms; na dúvida, derrubo. |
| Estado por id das linhas (painéis abertos e atividades buscadas) nunca é podado quando o item sai da fila | F-DF | É verdade que os quatro mapas não são podados (ProcessList.svelte:70-113, handleRemove/handleClear em 230-248), mas o ganho é desprezível: três deles guardam um booleano por linha clicada (cerca de 100 bytes cada). O atividadesBuscadas só é preenchido quando o App ainda não tem instantâneo do item (linha 87: primeiros 200 ms do processamento, ou item cancelado antes de começar, que guarda a string 'nenhuma'), porque App.svelte:139 já mantém o instantâneo de todo item processado. Não há instantâneos acumulando por item removido no uso normal. |

Observação: o primeiro derrubado (lente AN) e o achado F-DF-03 descrevem o mesmo mecanismo do `animate:flip`. Os céticos mediram de formas diferentes (página sintética contra o bundle de produção); F-DF-03 ficou, rebaixado para baixa, com a medição mais próxima do app real.

## Como aprovar

Escolha os ids e rode `/corrigir <ids>`, por exemplo `/corrigir F-VI-01 F-AC-01 B-EF-01`, ou `/corrigir todos`. Cada id vira um commit `fix(<camada>): <título> [<id>]` no branch `revisao/2026-11-10`, depois de passar pelos gates de build, benchmark e visual. Nada é enviado ao remoto nem mesclado.
