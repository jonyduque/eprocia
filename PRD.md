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
| **RF01** | **Entrada** | ZIP do Eproc ou PDF avulso (vários PDFs de uma vez viram um TXT, por evento). Leitura de arquivos `.zip` sem necessidade de descompactação prévia no disco. | O sistema deve ler o stream de bytes diretamente do ZIP; o PDF avulso é lido sem nunca alterar o original. |
| **RF02** | **Automação** | Monitoramento de diretório (Watchdog). | O sistema deve detectar novos arquivos ZIP na pasta configurada e, se a opção estiver ligada, PDFs novos, e iniciar o processamento automaticamente. |
| **RF03** | **Ordenação** | Ordenação "Natural" baseada no padrão de nomenclatura do Eproc/PJe. | Arquivos devem ser ordenados pelo número do evento (`Evento X`) e não alfabeticamente. Ex: Evento 2 deve vir antes de Evento 10. |
| **RF04** | **Extração PDF** | Extração de texto de arquivos PDF nativos (digital). | Deve-se utilizar alternativa técnica de maior performance para obter o texto limpo. |
| **RF05** | **Extração HTML** | Parsing de arquivos HTML (Eventos do sistema). | O sistema deve extrair apenas o texto visível no corpo do documento, ignorando tags de script e estilo. |
| **RF06** | **OCR** | Reconhecimento óptico de caracteres para PDFs de imagem. | Se a extração de texto retornar vazio ou insuficiente, o sistema deve tentar aplicar OCR (Tesseract) na página. Com o OCR ligado, também vai ao OCR a página que tem texto nativo mas em que 30% ou mais da área é imagem sem texto por perto (com texto em no máximo 10% da página), e a página com imagem cujo texto nativo não parece português (texto-isca). Um OCR em branco, ilegível ou que falha não apaga o texto nativo da página, e a falha do OCR (erro ou prazo) deixa no arquivo um texto fixo que pede ao modelo de IA que avise o usuário (seção 6); na página de imagem descoberta, as linhas nativas que faltam no texto do OCR (menos de 60% das palavras da linha presentes) entram nele, na ordem da página. O texto de Form XObject faz parte do texto nativo. |
| **RF07** | **Saída** | Geração de Arquivo Único Estruturado. | O output deve ser um arquivo `.txt` contendo todos os documentos concatenados. |
| **RF08** | **Estrutura** | Tagueamento XML/Markdown no texto final. | Cada documento deve ser envolvido por tags indicando seu ID e nome original. Ex: `<doc id="Evento 1">...</doc>`. |
| **RF09** | **Relatório** | Relatório do processamento de cada ZIP concluído. | O item da fila mostra quantas páginas vieram do texto do PDF, do OCR ou estavam em branco, e avisa só o que pede atenção: páginas com imagem ou desenho que precisavam de OCR e ficaram sem (OCR desligado), páginas com erro, páginas em que o OCR falhou e ficou o texto do PDF, documentos digitalizados não lidos por falta de OCR. Quando faltou OCR, oferece "Ler com OCR", que refaz só aquele ZIP com o OCR ligado, sem mudar a configuração. Os detalhes trazem tempos, a redução da higienização, os tipos omitidos e os documentos e páginas a conferir no PDF original. O `.txt` traz a linha de páginas sem OCR no cabeçalho (seção 6). |
| **RF10** | **OCR** | Forçar OCR em todas as páginas de PDF. | Opção desligada por padrão, nas configurações (vale para todo ZIP) e no reprocessamento de um item (vale só para ele). Ligada, toda página de PDF vai ao OCR, inclusive as que têm texto nativo e as repetidas; sem efeito com o OCR desligado. Uma página que só foi ao OCR por essa opção nunca fica pior do que sem ela quando o OCR falha, sai em branco ou ilegível: fica o texto nativo, com o aviso de falha do OCR quando houve falha. |
| **RF11** | **Histórico da fila** | Lista da última sessão ao reabrir o programa. | Opcional e desligado por padrão (interruptor "Guardar a lista da última sessão", em Configurações). Ligado, ao reabrir a lista mostra os itens da última sessão em que algo foi processado (uma sessão sem processamento não apaga a anterior), no máximo 500, com data e hora na linha e as ações de antes (copiar, abrir pasta, reprocessar) enquanto os arquivos existirem. O item que processava ou aguardava ao fechar aparece como "Interrompido", com "Processar de novo"; com o monitoramento ligado, a varredura da abertura o processa de novo sozinha na mesma linha quando ele é das últimas 24 h e não tem TXT mais novo. TXT apagado ou movido mostra "TXT não encontrado" e desativa o copiar. Tudo fica só neste computador (RNF07). |

## 4. Requisitos Não-Funcionais

| ID | Categoria | Descrição |
| --- | --- | --- |
| **RNF01** | **Privacidade** | Todo o processamento deve ser **Local (Offline)**. Nenhum dado pode trafegar para APIs externas de conversão, exceto a consulta de atualização iniciada pelo botão, regida pelo RNF06. |
| **RNF02** | **Eficiência** | O foco é a redução de tokens. Elementos como números de página, rodapés repetitivos e quebras de linha excessivas devem ser higienizados sempre que possível. |
| **RNF03** | **Portabilidade** | O software deve ser distribuído preferencialmente como um executável standalone (Windows `.exe`). |
| **RNF04** | **Dependência** | Eventuais binários devem ser empacotados no binário final |
| **RNF05** | **Diagnóstico** | Log em arquivo e telemetria só com a variável de ambiente `EPROCIA_LOG` (`1` ou `detalhado`), lida na partida; sem ela, nada é gravado e a interface não mostra nada sobre log. Os dois ficam só no computador, sem envio pela rede. O log leva nomes de ZIP e de documento, com o nome do usuário do Windows trocado; a telemetria não leva nome nenhum. |
| **RNF06** | **Atualização** | A única conexão de rede do programa é a consulta e o download de versões no GitHub (`jonyduque/eprocia`), e ela só acontece quando o usuário clica em "Procurar atualização" (e, depois, em "Atualizar"). Não há procura automática, periódica nem na abertura. A consulta não envia conteúdo, nome nem caminho de processo; o GitHub vê só o IP, a hora e o `User-Agent` `eprocIA/<versão>`. Sem telemetria, nem contagem de atualizações. O log local (RNF05, quando ligado) registra hora e resultado de cada consulta e de cada troca. |
| **RNF07** | **Privacidade do histórico** | O histórico da fila (RF11) guarda nomes de arquivos, que no Eproc são números de processo, e caminhos de pasta, que trazem o nome do usuário do Windows; não guarda texto de documento. Só existe com o interruptor ligado (o padrão é desligado), só em `%APPDATA%\eprocIA\historico.json`, no perfil do usuário, e nunca sai da máquina. Guarda só a última sessão com processamento, no máximo 500 itens. Limpa-se pelo X da linha (apaga o item), por "Limpar tudo" (apaga a lista inteira), desligando o interruptor (apaga o arquivo) e por `install.ps1 -Uninstall -Purge`. Em máquina compartilhada com o mesmo login, com o interruptor ligado, outra pessoa vê os processos da última sessão; a frase do interruptor diz isso. |

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
5. **Gravação:** Salva `processo.txt` na mesma pasta do ZIP original. PDF avulso grava `<nome>.txt` na pasta do PDF (com vários PDFs, o nome é o número CNJ comum a todos ou o do primeiro PDF por evento, e a pasta é a desse primeiro) e nunca sobrescreve arquivo alheio: se o nome estiver ocupado (ou, só para o nome base, existir `<nome>.zip` na mesma pasta), usa `<nome>_2.txt` até `<nome>_999.txt`.

## 6. Estrutura do Arquivo de Saída (Output Design)

Para garantir que o Gemini entenda a separação dos documentos, o arquivo final seguirá este padrão rigoroso:

```xml
<contexto>
PROCESSO JUDICIAL - ORDEM CRONOLÓGICA
Total de Documentos: [N]
Documentos não lidos: [N]
Páginas que precisavam de OCR e ficaram sem: [N]
Data de Consolidação: [DD/MM/AAAA HH:MM:SS]
Tokens estimados: [T]

<summary>
- Evento 1 - INIC1: 3 páginas
- Evento 1 - PROC2: 1 página
- ...
- Evento 5 - DESPACHO: 1 página
- Evento 6 - LAUDO: NÃO LIDO
- ...
</summary>
</contexto>

<evento num="1">
<documento index="1" nome="Evento 1 - INIC1.pdf">
<pag n="1">
[CONTEÚDO TEXTUAL DA PETIÇÃO INICIAL...]
</pag>
<pag n="2">
[CONTEÚDO TEXTUAL DA PETIÇÃO INICIAL...]
</pag>
<pag n="3">
[CONTEÚDO TEXTUAL DA PETIÇÃO INICIAL...]
</pag>
...
</documento>
<documento index="2" nome="Evento 1 - PROC2.pdf">
[CONTEÚDO TEXTUAL DA PROCURAÇÃO...]
</documento>
...
</evento>
<evento num="5">
<documento index="1" nome="Evento 5 - DESPACHO.html">
[CONTEÚDO TEXTUAL DO DESPACHO...]
</documento>
</evento>
...
```

- **Documentos não lidos:** quantos documentos o app não conseguiu ler (arquivo ilegível, PDF digitalizado com o OCR desligado, nenhum texto encontrado etc.). Sai sempre, também com 0.
- **Páginas que precisavam de OCR e ficaram sem:** só aparece quando maior que 0; são as páginas com imagem ou desenho cujo texto do PDF não bastou e o OCR estava desligado.
- **`NÃO LIDO` no `<summary>`:** o documento não lido aparece como `- Evento N - NOME: NÃO LIDO`, no lugar do total de páginas.
- **Corpo de um documento não lido:** uma única linha `[documento não lido: <motivo>; consulte o PDF original]` (em HTML, `consulte o arquivo original`), seguida do `[ERRO ...]` técnico quando houver. Documento não lido não entra na detecção de duplicatas.
- **Falha do OCR:** o texto diz que houve erro no OCR e pede ao modelo de IA que avise o usuário; do erro só leva uma frase fixa (`tempo esgotado` ou `o OCR não conseguiu ler a página`), nunca caminho nem saída do programa. A página que ficou sem texto leva `[ERRO de OCR: <frase>; avise o usuário de que houve erro no OCR desta página e recomende a consulta ao PDF original]` como conteúdo. A página que ficou com o texto nativo leva, antes dele, a linha `[aviso de OCR: houve erro no OCR desta página e o texto dela pode estar incompleto; avise o usuário e recomende a consulta ao PDF original]`; essa página não entra na detecção de duplicatas. No documento não lido por falha do OCR, o `[ERRO de OCR: ...]` vem logo depois da linha `[documento não lido: ...]`.
- **Data de Consolidação:** momento em que o arquivo foi gerado, no horário local.
- **Tokens estimados:** estimativa de tokens do arquivo inteiro, inclusive desta linha, a um token a cada quatro caracteres. É o mesmo número que a fila mostra no item concluído.
- **`<summary>`:** uma linha por documento, na ordem do arquivo, com o nome sem o prefixo "Evento N - " e sem a extensão.
- **`nome`:** nome do arquivo dentro do ZIP, com a extensão; `&`, aspas e sinais de menor e maior viram entidades XML.
- **Recuo:** nenhuma linha de marcação começa com espaço. No nível Nenhuma, o texto das páginas fica como extraído.
- **`<pag n="N">`:** um por página com conteúdo, com o número original (a peça é citada por página). Documento com exatamente 1 página na origem (um HTML, por exemplo, ou um PDF de uma página) escreve o texto direto dentro de `<documento>`, sem `<pag>`. Com 2 ou mais páginas na origem, a página que sobra também sai com `<pag n="N">`. Página vazia depois da limpeza não emite `<pag>`, e documento sem nenhum conteúdo sai sem corpo.
- **Páginas no `<summary>`:** é o total original do documento, inclusive as páginas que saíram vazias.
- **Erros:** página que falhou na leitura ou no OCR traz um marcador `[ERRO ...]` no lugar do texto.
