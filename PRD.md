# Product Requirements Document (PRD)

| Metadado | Detalhe |
| --- | --- |
| **Nome do Produto** | **eprocIA** |
| **Versão** | 1.0 (MVP - Minimum Viable Product) |
| **Status** | Especificação Aprovada |
| **Objetivo Principal** | Automatizar a extração e estruturação de autos judiciais para consumo eficiente por LLMs. |

## 1. Introdução e Propósito

O "Do eproc para a IA" é uma ferramenta de ETL (Extract, Transform, Load) projetada especificamente para o fluxo de trabalho jurídico. O problema atual reside na dificuldade de inserir múltiplos arquivos (PDFs e HTMLs) em ferramentas de IA (como Gemini/ChatGPT), que possuem limites de quantidade de anexos e limite de contexto (tokens). Além disso, arquivos brutos contêm formatação visual desnecessária que consome "espaço" na memória da IA.

A solução monitora downloads de arquivos ZIP do sistema Eproc, extrai seu conteúdo, ordena cronologicamente pela nomenclatura jurídica e consolida tudo em um único arquivo de texto estruturado. Isso permite que o servidor copie e cole apenas um arquivo ou texto na IA, economizando tempo e tokens.

## 2. Personas e Cenários de Uso

**Persona Primária: O Servidor/Assessor**
Servidor público com alta carga de processos para analisar. Possui pouco conhecimento de programação. Precisa de uma ferramenta "invisível" que faça o trabalho sujo assim que ele baixa o processo.

* **Cenário:** O servidor baixa o "Evento 1 ao 50.zip" do Eproc. A ferramenta detecta o arquivo, processa em segundo plano e, em segundos, entrega um arquivo `.txt` pronto para análise na mesma pasta.

**Persona Secundária: A Inteligência Artificial (LLM)**
O "consumidor" final dos dados. Precisa de dados limpos, sem cabeçalhos repetitivos de PDFs, e com marcações claras de onde começa e termina cada documento processual.

## 3. Requisitos Funcionais

Abaixo estão as funcionalidades obrigatórias que o sistema deve executar.

| ID | Área | Descrição do Requisito | Critério de Aceite |
| --- | --- | --- | --- |
| **RF01** | **Entrada** | Leitura de arquivos `.zip` sem necessidade de descompactação prévia no disco. | O sistema deve ler o stream de bytes diretamente do ZIP. |
| **RF02** | **Automação** | Monitoramento de diretório (Watchdog). | O sistema deve detectar novos arquivos ZIP na pasta configurada e iniciar o processamento automaticamente. |
| **RF03** | **Ordenação** | Ordenação "Natural" baseada no padrão de nomenclatura do Eproc/PJe. | Arquivos devem ser ordenados pelo número do evento (`Evento X`) e não alfabeticamente. Ex: Evento 2 deve vir antes de Evento 10. |
| **RF04** | **Extração PDF** | Extração de texto de arquivos PDF nativos (digital). | Deve-se utilizar alternativa técnica de maior performance para obter o texto limpo. |
| **RF05** | **Extração HTML** | Parsing de arquivos HTML (Eventos do sistema). | O sistema deve extrair apenas o texto visível no corpo do documento, ignorando tags de script e estilo. |
| **RF06** | **OCR** | Reconhecimento óptico de caracteres para PDFs de imagem. | Se a extração de texto retornar vazio ou insuficiente, o sistema deve tentar aplicar OCR (Tesseract) na página. |
| **RF07** | **Saída** | Geração de Arquivo Único Estruturado. | O output deve ser um arquivo `.txt` contendo todos os documentos concatenados. |
| **RF08** | **Estrutura** | Tagueamento XML/Markdown no texto final. | Cada documento deve ser envolvido por tags indicando seu ID e nome original. Ex: `<doc id="Evento 1">...</doc>`. |

## 4. Requisitos Não-Funcionais

| ID | Categoria | Descrição |
| --- | --- | --- |
| **RNF01** | **Privacidade** | Todo o processamento deve ser **Local (Offline)**. Nenhum dado pode trafegar para APIs externas de conversão. |
| **RNF02** | **Eficiência** | O foco é a redução de tokens. Elementos como números de página, rodapés repetitivos e quebras de linha excessivas devem ser higienizados sempre que possível. |
| **RNF03** | **Portabilidade** | O software deve ser distribuído preferencialmente como um executável standalone (Windows `.exe`). |
| **RNF04** | **Dependência** | Eventuais binários devem ser empacotados no binário final |

## 5. Arquitetura Técnica Proposta

**Linguagem:**

**Stack de Bibliotecas:**

1. **Monitoramento:**
2. **Manipulação de Arquivos:**
3. **Processamento PDF:** 
4. **Processamento HTML:**
5. **OCR:**
6. **Interface:**

**Fluxo de Dados:**

1. **Trigger:** detecta `processo.zip`.
2. **Leitura:** abre o arquivo em memória.
3. **Indexação:** Lista de arquivos interna é capturada e ordenada via Regex `r"Evento (\d+)"`.
4. **Loop de Processamento:**
Itera sobre cada arquivo:
Verifica extensão -> Seleciona Extrator (HTML/PDF) -> Valida se tem texto -> Se não, aplica OCR -> Se tem, extrai -> Limpa string -> Adiciona buffer.
5. **Gravação:** Salva `processo.txt` na mesma pasta do ZIP original.

## 6. Estrutura do Arquivo de Saída (Output Design)

Para garantir que o Gemini entenda a separação dos documentos, o arquivo final seguirá este padrão rigoroso:

```xml
<contexto>
    PROCESSO JUDICIAL - ORDEM CRONOLÓGICA
    Total de Documentos: [N]
    Data de Consolidação: [DD/MM/AAAA HH:MM:SS]
    Tokens estimados: [T]

    <summary>
    - Evento 1 - INIC1: 3 páginas
    - Evento 1 - PROC2: 1 página
    - ...
    - Evento 5 - DESPACHO: 1 página
    - ...
    </summary>
</contexto>

<evento num="1">
    <documento index="1" nome="Evento 1 - INIC1.pdf">
        <pag num="1">
            [CONTEÚDO TEXTUAL DA PETIÇÃO INICIAL...]
        </pag>
        <pag num="2">
            [CONTEÚDO TEXTUAL DA PETIÇÃO INICIAL...]
        </pag>
        <pag num="3">
            [CONTEÚDO TEXTUAL DA PETIÇÃO INICIAL...]
        </pag>
        ...
    </documento>
    <documento index="2" nome="Evento 1 - PROC2.pdf">
        <pag num="1">
            [CONTEÚDO TEXTUAL DA PROCURAÇÃO...]
        </pag>
    </documento>
    ...
</evento>
<evento num="5">
    <documento index="1" nome="Evento 5 - DESPACHO.html">
        <pag num="1">
            [CONTEÚDO TEXTUAL DO DESPACHO...]
        </pag>
    </documento>
</evento>
...
```

- **Data de Consolidação:** momento em que o arquivo foi gerado, no horário local.
- **Tokens estimados:** estimativa de tokens do arquivo inteiro, inclusive desta linha, a um token a cada quatro caracteres. É o mesmo número que a fila mostra no item concluído.
- **`<summary>`:** uma linha por documento, na ordem do arquivo, com o nome sem o prefixo "Evento N - " e sem a extensão.
- **`nome`:** nome do arquivo dentro do ZIP, com a extensão; `&`, aspas e sinais de menor e maior viram entidades XML.
- **`<pag>`:** um por página do PDF; um HTML vira um único `<pag num="1">`. Página que falhou na leitura ou no OCR traz um marcador `[ERRO ...]` no lugar do texto.
