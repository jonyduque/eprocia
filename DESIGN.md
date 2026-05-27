# Design System - eprocIA

Este documento define a identidade visual e as diretrizes de interface (UI/UX) do eprocIA. O design foca em **eficiência**, **sobriedade jurídica** e **alta densidade de informação**.

## 1. Conceito Visual
O eprocIA é uma ferramenta de produtividade. A interface deve ser "invisível" e funcional, priorizando o conteúdo processual sobre adornos visuais.

- **Densidade:** Alta. Espaçamentos reduzidos (compact) para permitir a visualização de muitos itens na lista sem rolagem excessiva.
- **Estilo:** Moderno, limpo, utilizando bordas sutis e sombras leves para separação de camadas.

## 2. Cores e Temas

O aplicativo suporta troca dinâmica entre temas Claro e Escuro, respeitando as preferências do sistema operacional por padrão.

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

- **Família Principal:** `Inter`, `Segoe UI` ou o padrão do sistema (San Francisco/Roboto).
- **Tamanhos:**
    - Títulos: `16px` (Semibold)
    - Texto Base: `13px` (Regular)
    - Metadados/Logs: `11px` (Regular)
    - Mono (Tags XML): `12px` (`JetBrains Mono` ou `Consolas`)

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

### 5.2 Drag & Drop Zone
- Uma área sutil no topo ou o fundo da própria lista.
- Quando um arquivo é arrastado sobre a janela, uma borda azul pontilhada aparece com o texto: "Solte o ZIP para processar".

### 5.3 Barra de Status (Footer)
- Indicador se o Watchdog está ativo (ex: "Monitorando: C:/Users/...").
- Versão do app.

## 6. Layout e Espaçamento (Grades)
- **Padding padrão:** `8px` (em vez dos tradicionais 16px ou 24px).
- **Gap entre itens da lista:** `4px`.
- **Raio de borda (Border Radius):** `4px` (cantos levemente arredondados, mantendo a sobriedade).

---
*Este design deve ser implementado no frontend do Wails v3 utilizando CSS Vanilla ou um framework leve que suporte as variáveis de cores definidas acima.*
