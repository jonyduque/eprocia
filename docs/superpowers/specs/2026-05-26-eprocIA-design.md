# Especificação Técnica - eprocIA

> **Status:** Finalizada
> **Data:** 2026-05-26
> **Tecnologia:** Golang + Wails v3

## 1. Visão Geral
O **eprocIA** é uma ferramenta de ETL local projetada para consolidar processos judiciais exportados do sistema Eproc em um único arquivo de texto estruturado. O objetivo é otimizar o consumo desses dados por Modelos de Linguagem (LLMs), reduzindo o uso de tokens e simplificando a gestão de anexos.

## 2. Arquitetura do Sistema

### 2.1 Stack Tecnológica
- **Linguagem:** Go 1.21+
- **Framework GUI:** Wails v3 (Desktop App)
- **Frontend:** React ou Svelte (Interface leve para monitoramento)
- **Bibliotecas Core:**
    - `fsnotify/fsnotify`: Monitoramento de diretórios.
    - `razvandimescu/gopdf`: Extração de texto de PDFs (Pure Go).
    - `jaytaylor/html2text`: Conversão de HTML para texto.
    - `os/exec` + `//go:embed`: Execução de Tesseract OCR embutido.

### 2.2 Componentes de Backend
1.  **Watchdog Module:** Monitora a pasta configurada. Dispara o processamento para qualquer arquivo `.zip` que contenha PDFs, HTMLs ou TXTs.
2.  **Orchestrator:** Gerencia a fila de processamento e a comunicação com a UI.
3.  **Extraction Engine:**
    - Abre o ZIP em memória.
    - Identifica e ordena os arquivos.
    - Processa página por página conforme lógica de decisão.
4.  **OCR Wrapper:** Gerencia a extração do binário Tesseract para pasta temporária e sua execução silenciosa.
5.  **Output Formatter:** Gera o arquivo `.txt` final usando templates estruturados.

## 3. Fluxo de Processamento e Ordenação

### 3.1 Identificação de Arquivos
O sistema deve listar todos os arquivos dentro do ZIP e identificar:
1.  **Capa:** Arquivo nomeado exatamente como `Capa do processo.pdf`.
2.  **Eventos:** Arquivos que seguem o padrão Regex: `Evento (\d+) - [A-Za-z_ ]+(\d+)\.(html|pdf)`.
    - `$1`: ID do Evento (Nível 1 de ordenação).
    - `$2`: ID do Documento (Nível 2 de ordenação).

### 3.2 Ordem Cronológica Rigorosa
A consolidação deve seguir esta ordem:
1.  `Capa do processo.pdf` (se existir).
2.  Agrupamento por **Evento ($1)** em ordem crescente.
3.  Dentro de cada evento, agrupamento por **Documento ($2)** em ordem crescente.
4.  Outros arquivos que não seguem o padrão (anexados ao final por ordem alfabética).

## 4. Lógica de Extração de Conteúdo (Página por Página)

O processamento de PDFs deve avaliar cada página individualmente para decidir entre extração nativa ou OCR.

### 4.1 Critérios de Decisão
| Condição | Ação | Critério de Sucesso |
| :--- | :--- | :--- |
| Sem texto E sem imagens | **Página em Branco** | Marca como página em branco no XML. |
| Imagens > 10% da área | Extração Nativa | Sucesso se > 100 caracteres extraídos. |
| Imagens <= 10% da área | Extração Nativa | Sucesso se qualquer texto for extraído. |
| Falha no critério acima | **OCR (Tesseract)** | Executa OCR na página em Português. |

### 4.2 OCR Embutido
- O binário do Tesseract e o arquivo `por.traineddata` serão embutidos no `.exe` via `//go:embed`.
- Em tempo de execução, se necessário, são extraídos para `%TEMP%/eprocIA/ocr/`.

## 5. Estrutura do Arquivo de Saída (Output Design)

O arquivo final `.txt` deve seguir o padrão XML/Markdown para facilitar o parsing por IAs.

### 5.1 Cabeçalho e Sumário
```xml
<contexto>
    PROCESSO [BASE NAME DO ARQUIVO ZIP] - ORDEM CRONOLÓGICA
    Total de Documentos: [N]

    <summary>
    - Capa do processo: [P] páginas
    - Evento 1 - INIC1: [P] páginas
    - Evento 1 - PROC2: [P] páginas
    - ...
    </summary>
</contexto>
```

### 5.2 Corpo dos Documentos
```xml
<evento num="[ID_EVENTO]">
    <documento index="[ID_DOC]" nome="[NOME_ORIGINAL]">
        <pag num="1">
            [CONTEÚDO TEXTUAL...]
        </pag>
        <pag num="2">
            [PÁGINA EM BRANCO]
        </pag>
    </documento>
</evento>
```

## 6. Interface e Experiência do Usuário (UX)

### 6.1 Janela Principal (Wails v3)
- Lista de processos recentes e status (Sucesso / Processando / Erro).
- Botão "Processar Manualmente" para selecionar ZIPs ou pastas.
- Aceita drag & drop de arquivos zip.
- Quando arrastar para fora o registro de um arquivo processado, será tratado como se estivesse arrastando um texto, sendo que o texto será o resultado processado.
- Área de configurações:
    - Pasta monitorada (default: Downloads).
    - Toggle para OCR (Ativado/Desativado).
    - Nível de limpeza de texto (Espaços, Quebras de linha).

### 6.2 Notificações e Erros
- **Notificação Nativa:** Exibida ao concluir um processo. Se houver erros, a notificação deve indicar: "Processo concluído com [X] erros de leitura".
- **Visualização de Erros:** A interface deve destacar quais arquivos ou páginas falharam para que o usuário possa verificar manualmente.

## 7. Requisitos Não-Funcionais
- **Privacidade:** Processamento 100% local.
- **Portabilidade:** Executável único (.exe) para Windows.
- **Desempenho:** Uso de Goroutines para processamento paralelo de páginas e arquivos dentro de um mesmo ZIP.
