# Design System - eprocIA

Este documento define a identidade visual e as diretrizes de interface (UI/UX) do eprocIA. O design foca em **eficiência**, **sobriedade jurídica** e **alta densidade de informação**.

## 1. Conceito Visual
O eprocIA é uma ferramenta de produtividade. A interface deve ser "invisível" e funcional, priorizando o conteúdo processual sobre adornos visuais.

- **Densidade:** Alta. Espaçamentos reduzidos (compact) para permitir a visualização de muitos itens na lista sem rolagem excessiva.
- **Estilo:** Moderno, limpo, utilizando bordas sutis e sombras leves para separação de camadas.

## 2. Cores e Temas

O aplicativo suporta troca dinâmica entre temas Claro e Escuro, respeitando as preferências do sistema operacional por padrão.

- **Preferência em três estados:** sistema (padrão), claro e escuro, alternados pelo botão da barra de título e salvos em `localStorage.theme`. No modo sistema o app escuta `prefers-color-scheme` e troca na hora (regra em `frontend/src/lib/tema.js`).
- **Sem flash na abertura:** um script inline em `index.html` aplica a classe `light` antes da montagem, e o `BackgroundColour` da janela nativa (`main.go`) segue o tema do Windows com o mesmo `--bg-main` do CSS.
- **Uma fonte de verdade:** cores só por tokens de `frontend/src/assets/css/style.css`, definidos para os dois temas (`--overlay`, `--glass`, `--card-*`, `--shadow-*`, `--on-accent`...). Fundos tonais derivam do token base com `color-mix(in srgb, var(--accent) 12%, transparent)` (`--accent-soft`, `--error-soft`, `--warning-soft`...). Nenhum hex, `rgb()`/`rgba()` ou `white`/`black` solto nos componentes.
- **Contraste (WCAG 1.4.3 e 1.4.11):** texto pequeno precisa de 4,5:1 nos dois temas. Texto branco (`--on-accent`) só sobre `--accent-strong` (`#2563EB`, hover `--accent-strong-hover` `#1D4ED8`); `--accent`, `--success` e `--error` ficam para ícones, bordas e preenchimentos. Texto verde ou vermelho usa `--success-strong` / `--error-strong` (escuro `#34D399` / `#F87171`, claro `#047857` / `#B91C1C`) sobre superfície neutra; os toasts seguem esse padrão. Contorno de controles (campos, selects, trilho do interruptor desligado) usa `--border-strong` (`#6B7280`, ao menos 3:1 contra `--surface` e `--bg-main`); `--border` fica só para divisórias. O teste `frontend/contraste.test.mjs` calcula esses pares.
- **Foco visível:** uma regra global em `style.css` (`:where(button, input, select, textarea, a, [tabindex]):focus-visible`) desenha `outline: 2px solid var(--focus-ring)` com `outline-offset: 1px`. Componentes não removem o outline. No interruptor o input é invisível, então o anel vai no trilho (`.switch input:focus-visible + .slider`, offset `2px`).
- **Botões com estado:** recolher/expandir o painel tem `aria-expanded` e `aria-controls="sidebar"`; o de tema diz o atual e o próximo no `aria-label` ("Tema: sistema. Mudar para claro"); maximizar vira "Restaurar" (ícone `copy`) quando `WindowIsMaximised` diz que a janela está maximizada, conferido na montagem e a cada `resize`. Botões só com ícone têm `aria-label`, e o SVG leva `aria-hidden="true"`.
- **Anúncios (WCAG 4.1.3):** mensagens de status vão em regiões vivas sempre montadas (o conteúdo entra e sai dentro delas): sucesso em `role="status" aria-live="polite"`, erro em `role="alert"`. Barras de progresso têm `role="progressbar"` com `aria-valuemin`/`aria-valuemax`/`aria-valuenow` (0 a 100) e `aria-label`. Ícones de estado levam `aria-hidden="true"` e um texto `.sr-only` (Em processamento, Aguardando, Pausado, Concluído, Erro, Cancelado). Campos sem rótulo visível usam `aria-label` descritivo; o filtro da fila é `type="search"` e, sem resultado, mostra "Nenhum item corresponde ao filtro" em `role="status"`. O ponto da barra recolhida tem `role="status"` e o texto do monitoramento em `.sr-only`.
- **Foco não se perde:** quando Cancelar/Remover some ou troca de botão, o foco vai para o botão equivalente (mesmo `data-acao`, senão Remover/Cancelar) da própria linha, da seguinte ou da anterior; sem linhas (inclusive depois de Limpar Fila), vai para o título "Fila de Processamento" (`tabindex="-1"`). Regra em `frontend/src/lib/fila.js`.
- **Erro completo por teclado:** o ícone de ajuda de um item com erro é um botão "Ver detalhes do erro" (`aria-expanded`, `aria-controls`) que abre, abaixo da linha, a mensagem completa e o caminho do ZIP, em texto selecionável (`user-select: text`). O `title` fica só como atalho para o mouse.

### 2.1 Tema Claro (Light Mode)
- **Fundo Principal:** `#F8F9FA` (Cinza muito claro)
- **Superfícies (Cards/Lista):** `#FFFFFF`
- **Texto Primário:** `#212529` (Cinza quase preto)
- **Texto Secundário:** `#6C757D`
- **Bordas:** `#DEE2E6`

### 2.2 Tema Escuro (Dark Mode)
- **Fundo Principal:** `#121212`
- **Superfícies (Cards/Lista):** `#1E1E1E`
- **Texto Primário:** `#E9ECEF`
- **Texto Secundário:** `#ADB5BD`
- **Bordas:** `#2C2C2C`

### 2.3 Cores de Destaque (Accent)
- **Primária (Ação/Branding):** `#0056b3` (Azul Jurídico)
- **Sucesso:** `#28A745` (Verde)
- **Erro/Alerta:** `#DC3545` (Vermelho)
- **Processamento/Link:** `#007BFF`

## 3. Tipografia
Utilizaremos fontes sans-serif modernas que garantem legibilidade em tamanhos pequenos.

- **Família Principal:** `"Segoe UI Variable"`, `"Segoe UI"`, `system-ui` (fontes do Windows; nada é baixado da rede, conforme RNF01 do PRD).
- **Tamanhos:**
    - Títulos: `16px` (Semibold)
    - Texto Base: `13px` (Regular)
    - Metadados/Logs: `11px` (Regular)
    - Mono (Tags XML), se for usada: `12px` (`Consolas`, fonte do sistema)

## 4. Ícones
Utilizaremos a biblioteca **Lucide React** (ou similar) por sua consistência e traços finos.

- **Estados:**
    - `CheckCircle`: Processamento concluído.
    - `Loader`: Em processamento (com animação de rotação).
    - `AlertCircle`: Erro no arquivo.
    - `FileText`: Representação de documento.
    - `Settings`: Configurações.
    - `Monitor`: Pasta monitorada.
- **Tamanho padrão:** `16px` a `18px`.

## 5. Componentes Principais (UX)

### 5.1 Lista de Processos (Compacta)
Cada linha deve ter altura máxima de `40px`, contendo:
- Ícone de status.
- Nome do arquivo ZIP.
- Timestamp de processamento.
- Contador de erros (se houver).
- Botão discreto para abrir a pasta de saída.

Cabeçalho da fila sempre em uma linha: o título trunca com reticências, o campo Filtrar encolhe de `140px` até `80px` (não cresce no foco) e os botões não encolhem. Com a fila abaixo de `420px` (container query), Pausar/Retomar/Limpar Fila mostram só o ícone, com o nome no `aria-label` e no `title`.

Divisor entre painel e fila: de `280px` até `480px`, limitado também à largura da janela menos `340px` reservados à fila (`frontend/src/lib/divisor.js`). O limite é reaplicado quando a janela muda, e o slider (`aria-valuemax`/`aria-valuenow`, `aria-valuetext="N pixels"`, `aria-controls="sidebar"`) mostra a largura real. Teclado no divisor: setas mudam 10 px, Home vai ao mínimo, End ao máximo da janela, Enter recolhe o painel e leva o foco ao botão de expandir; essas teclas não rolam a página (`preventDefault`). No foco por teclado o divisor fica em `--accent`, como no hover.

Largura e estado recolhido do painel são salvos em `localStorage['eprocia.sidebar']` (`{largura, recolhido}`, `frontend/src/lib/painel.js`) ao recolher/expandir e ao terminar de arrastar, e lidos antes da montagem com o limite da janela aplicado. Armazenamento bloqueado só faz o painel voltar ao padrão (`320px`, expandido).

Fila pausada: pausar só impede o próximo item de começar. O item em curso mantém o spinner, a barra e o "1 ativo" da barra de título; os pendentes trocam o relógio por pausa (`--warning`); um chip abaixo da barra da lista diz "Fila pausada: termina o item atual" (ou só "Fila pausada"). O estado visual de cada linha vem de `estadoItem(status, isPaused)` em `frontend/src/lib/estado.js`.

### 5.2 Drag & Drop Zone
- Uma área sutil no topo ou o fundo da própria lista.
- Quando um arquivo é arrastado sobre a janela, uma borda azul pontilhada aparece com o texto: "Solte o ZIP para processar".

### 5.3 Painel de Configurações
- Precisa ficar legível a 280px de largura sem cortar palavra.
- Interruptores: rótulo à esquerda, switch à direita, linha de `32px`.
- Selects com opções curtas (ex.: Nenhuma, Básica, Agressiva); a explicação da opção escolhida vai num `.help-text` de `11px` abaixo do select.
- Caminho da pasta monitorada com reticências e o caminho completo no `title`.
- Toast de status no rodapé do painel: entra com `fly` (`y: 8`, `--dur-base`, `cubicOut`) e sai com `fade` (`--dur-fast`), ambos zerados em `prefers-reduced-motion`; o de sucesso dura 3 s e cada mensagem nova reinicia o prazo (um único timer, cancelado no desmonte); o de erro não some sozinho e tem botão fechar (ícone `x`, `aria-label="Fechar aviso"`). As durações em JS ficam em `src/lib/aviso.js`.

### 5.4 Barra de Status (Footer)
- Indicador se o Watchdog está ativo (ex: "Monitorando: C:/Users/..."). Ocupa o espaço livre e trunca o caminho com reticências, com o caminho completo no `title`, inclusive no estado pausado.
- À direita, sem quebrar linha: contador ("1 item na fila", "N itens na fila") e a versão do app, lida de `info.productVersion` do `wails.json` no build. A barra de título mostra só a marca.

## 6. Layout e Espaçamento (Grades)
- **Padding padrão:** `8px` (em vez dos tradicionais 16px ou 24px).
- **Gap entre itens da lista:** `4px`.
- **Raio de borda (Border Radius):** `4px` (cantos levemente arredondados, mantendo a sobriedade).

## 7. Movimento
- **Tokens** (`frontend/src/assets/css/style.css`): `--dur-fast` `150ms` (hover, foco, entradas pequenas), `--dur-base` `220ms` (troca de tema, toasts, barra de progresso), `--ease-out` `cubic-bezier(.2,.8,.2,1)` como curva única. Laços contínuos: `--dur-spin` `800ms` (spinner) e `--dur-pulse` `1500ms` (indicador "N ativos").
- Toda `transition` e `animation` usa os tokens e nomeia as propriedades; nada de duração, `ease-*` ou `cubic-bezier` solto nos componentes.
- Transições do Svelte (`fly`, `fade`) rodam em JS e não leem CSS: usam `duracaoMovimento(DUR_FAST | DUR_BASE)` de `src/lib/aviso.js`, que devolve `0` em `prefers-reduced-motion`.
- `prefers-reduced-motion: reduce` zera `--dur-fast` e `--dur-base`, troca deslocamentos (hover da linha, `fly`) por troca direta, desliga o pulso e deixa o spinner em `2400ms` em vez de pará-lo, porque ele é o sinal de que o item está em processamento.
- Animações contínuas só mexem em `transform`/`opacity` (o pulso é um anel num `::after`), para não repintar a cada quadro.
- Overlay de soltar ZIP: entra e sai com `fade` (`--dur-fast`); a caixa cresce de `scale(0.98)`.
- Divisor do painel: Pointer Events com captura, largura aplicada uma vez por quadro e presa aos limites (encosta no mínimo/máximo em vez de travar), sem transição de largura durante o arrasto; cursor `col-resize` e sem seleção de texto na janela toda enquanto arrasta. A alça captura `8px` e desenha uma linha de `2px`. O arrasto termina em `pointerup`, `pointercancel`, perda de captura ou quando a janela perde o foco.

---
*Este design deve ser implementado no frontend do Wails v3 utilizando CSS Vanilla ou um framework leve que suporte as variáveis de cores definidas acima.*
