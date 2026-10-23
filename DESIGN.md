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

Divisor entre painel e fila: de `280px` até `480px`, limitado também à largura da janela menos `340px` reservados à fila (`frontend/src/lib/divisor.js`). O limite é reaplicado quando a janela muda, e o slider (`aria-valuemax`/`aria-valuenow`) mostra a largura real.

Fila pausada: pausar só impede o próximo item de começar. O item em curso mantém o spinner, a barra e o "1 ativo" da barra de título; os pendentes trocam o relógio por pausa (`--warning`); um chip abaixo da barra da lista diz "Fila pausada: termina o item atual" (ou só "Fila pausada"). O estado visual de cada linha vem de `estadoItem(status, isPaused)` em `frontend/src/lib/estado.js`.

### 5.2 Drag & Drop Zone
- Uma área sutil no topo ou o fundo da própria lista.
- Quando um arquivo é arrastado sobre a janela, uma borda azul pontilhada aparece com o texto: "Solte o ZIP para processar".

### 5.3 Painel de Configurações
- Precisa ficar legível a 280px de largura sem cortar palavra.
- Interruptores: rótulo à esquerda, switch à direita, linha de `32px`.
- Selects com opções curtas (ex.: Nenhuma, Básica, Agressiva); a explicação da opção escolhida vai num `.help-text` de `11px` abaixo do select.
- Caminho da pasta monitorada com reticências e o caminho completo no `title`.

### 5.4 Barra de Status (Footer)
- Indicador se o Watchdog está ativo (ex: "Monitorando: C:/Users/..."). Ocupa o espaço livre e trunca o caminho com reticências, com o caminho completo no `title`, inclusive no estado pausado.
- À direita, sem quebrar linha: contador ("1 item na fila", "N itens na fila") e a versão do app, lida de `info.productVersion` do `wails.json` no build. A barra de título mostra só a marca.

## 6. Layout e Espaçamento (Grades)
- **Padding padrão:** `8px` (em vez dos tradicionais 16px ou 24px).
- **Gap entre itens da lista:** `4px`.
- **Raio de borda (Border Radius):** `4px` (cantos levemente arredondados, mantendo a sobriedade).

---
*Este design deve ser implementado no frontend do Wails v3 utilizando CSS Vanilla ou um framework leve que suporte as variáveis de cores definidas acima.*
