# Higienização e extração de texto: design

Data: 2026-11-08. Branch: `higienizacao`, criada a partir de `concorrencia-e-interface` (PR #3, ainda sem merge), porque o Reprocessar e os atalhos de higienização da barra recolhida estão nela.

## Por quê

O `.txt` do eprocIA é colado num LLM para análise jurídica. Cada token gasto com ruído tira espaço da análise, e texto ilegível piora a leitura do LLM. Em 4 processos reais medidos em 2026-11-08 (estudo de um subagente, sem dados de processo neste documento), a higienização agressiva atual economiza só 1,4% e 6,4% em relação à básica. Ao mesmo tempo, os extratores desperdiçam muito mais:

- PDFs com letras espaçadas ocupam de 7% a 18% do texto;
- os HTMLs do eproc perdem todos os acentos;
- páginas com texto nativo ilegível não passam pelo OCR.

## Objetivos

1. Corrigir os quatro defeitos de extração (E1 a E4). Valem em qualquer nível, inclusive `none`.
2. Trocar os níveis de higienização por quatro: Nenhuma, Básica, Padrão e Máxima.
3. Acrescentar uma opção, combinável com qualquer nível, para omitir tipos de documento.
4. Nenhuma palavra de conteúdo jurídico pode sumir sem deixar marcador.

## Não-objetivos

- Fotos dentro dos documentos: o dono do repositório desistiu desse tema.
- Tokenizador real. A estimativa continua sendo runas/4.
- Remover o timbre ou o cabeçalho repetido entre documentos diferentes. Cada documento mantém o seu primeiro timbre (decisão do dono, abaixo).

## Antiobjetivos

Mesmo que funcione, a mudança fracassa se:

- apagar quem assinou um ato ou uma peça;
- juntar itens de lista ou linhas de tabela (extratos precisam continuar com um lançamento por linha);
- apagar uma frase no meio da página só porque ela se repete em outro lugar;
- deixar a Básica com menos conteúdo que hoje.

## Restrições

- Fora do formatter, o pipeline não muda de ordem nem de concorrência. As regras de `CLAUDE.md` para o mapa `eventos` continuam valendo.
- Todo texto de teste é sintético. Fixtures novas vão para `scripts/bench/fixtures/`, nunca com dado real.
- O config antigo continua válido: `none`, `basic` e `aggressive` são aceitos como hoje, e `aggressive` passa a significar Máxima.
- A regra "remover linha só nas bordas da página" vale para tudo que apaga uma linha inteira. As exceções são as transformações que não apagam palavra (espaços, controle, formato) e a deduplicação com referência.
- Interface em pt-BR. Depois de mudar um método de `App`, rodar `wails generate module`.

## Parte 1: extratores (todos os níveis)

**E1. Espaço entre palavras no PDF.** `ExtractNativeText` (`internal/etl/pdf_parser.go`) junta cada `row.Content[i].S` com `" "`. Quando o PDF posiciona cada letra separadamente, a linha sai como `C O N T E S T A Ç Ã O`.
- Limite da biblioteca (conferido em `dslipak/pdf` v0.0.2, `page.go:623`): `GetTextByRow` só preenche `S`, `X` e `Y`. O X só muda no operador `Tm`. `Td` e os deslocamentos numéricos do `TJ` são ignorados, e `FontSize` e `W` ficam em zero. Não dá para calcular a distância entre glifos por esse caminho.
- Correção (decisão minha):
  - escrever um extrator próprio sobre `pdf.Interpret`, que acompanha `Tm`, `Td`, `TD`, `T*`, `Tf` (tamanho), `TL` e os deslocamentos do `TJ`;
  - inserir espaço quando um deslocamento do `TJ` for ≤ cerca de −200 milésimos de em, ou quando o salto horizontal entre dois operadores de texto na mesma linha passar de cerca de 0,25 × tamanho da fonte além da largura estimada do trecho anterior;
  - o builder calibra os dois limiares com as fixtures.
- Não usar `page.Content()`: ele tem o histórico de travar descrito em `CLAUDE.md`.
- O extrator continua protegido por `checkContentStream` e pelo `recover`.
- Agrupa as linhas por Y como hoje.
- Se der errado em alguma página (pânico ou zero linhas), volta ao `GetTextByRow` atual.
- Teste: fixture sintética com kerning (letra a letra e por sílaba) e com texto normal, que não pode mudar.

**E2. Charset do HTML.** `html_parser` não decodifica ISO-8859-1/Windows-1252, então as páginas do eproc saem com `�`. Correção: `golang.org/x/net/html/charset`, usando o meta ou o BOM, com Windows-1252 como reserva quando o conteúdo não é UTF-8 válido.

**E3. HTML inline.** Hoje cada nó de texto vira uma linha (`AUTOR` / `:` / `<NOME>`). Correção: quebrar a linha só em elemento de bloco (`p`, `div`, `br`, `tr`, `li`, `h1`–`h6`, `table`) e juntar o texto inline com espaço.

**E4. Texto nativo ilegível vai para o OCR.** Uma página que tem texto nativo, mas com razão alfanumérica abaixo de cerca de 0,4 ou com caracteres de controle demais (fonte com codificação quebrada), passa a ser tratada como sem texto nativo e vai para o OCR. O builder calibra os limiares com fixtures sintéticas. O caso real que motivou E4 é uma matrícula de imóvel.
- A página ilegível vai para o OCR mesmo sem imagem grande nem conteúdo vetorial: em `analisarPagina`, "ilegível" é motivo próprio de OCR, ao lado de `HasImages` e `HasDrawnContent`.
- Com o OCR desligado, ou se o OCR falhar, a página sai como `[texto ilegível na página: N caracteres omitidos]`, em qualquer nível, inclusive `none`.

## Parte 2: níveis

Os ids de config são `none`, `basic`, `standard` (novo) e `aggressive`. Cada nível inclui tudo do anterior.

### Nenhuma (`none`)

Como hoje: o texto sai como foi extraído.

### Básica (`basic`)

Como hoje (trim e sem linhas vazias), mais:
- **H1:** sequências de 2 ou mais espaços, tabs e NBSP dentro da linha viram um espaço só.
- **H2:** remover caracteres de controle `[\x00-\x08\x0b\x0c\x0e-\x1f]` e linhas formadas só por `-`, `_`, `=`, `.`, `*` ou `•` repetidos 5 vezes ou mais.

As quebras de linha ficam e nenhuma palavra se perde.

### Padrão (`standard`)

Básica, mais:

- **Marcadores de página:** remover as linhas que são só marcador de página, como hoje na agressiva.
- **H4, bordas repetidas por documento:** janela de 5 linhas em cada borda, documentos com 2 ou mais páginas, limiar de 60% e dígitos normalizados.
- **Timbre só no primeiro (decisão do dono):** dentro de um documento, a linha de borda repetida fica na **primeira página onde aparece** e sai das seguintes. Hoje sai de todas. Assim o timbre e o cabeçalho continuam uma vez por documento, e o nome do escritório ou do perito não se perde.
- **H6, chancelas viram marcador**, sempre na borda (últimas linhas da página ou do documento; no carimbo de margem do e-SAJ, também as primeiras):
  - **HTML (decisão do dono):** o bloco de assinatura digital vira `[assinado: <NOME>]`, só com o nome do assinante. Exemplo: "Documento eletrônico assinado por <NOME>, <CARGO>, na forma do artigo 1º, inciso III, da Lei 11.419 ... código verificador <N> e do código CRC <X>.", junto com o rodapé `<NÚMERO CNJ> <N> .V<N>` que vem depois.
  - **PDF:** `[assinado: <NOME>, <CARGO>, <DATA>]`, com o que a chancela trouxer. As famílias são:
    - "Este documento é cópia do original, assinado digitalmente por ..." e "Para conferir o original, acesse o site ..." (e-SAJ);
    - "Documento registrado eletronicamente ..." e "Assinado digitalmente por <NOME> - <CARGO>" (cartório);
    - "Documento eletrônico <ID> assinado eletronicamente nos termos do Art.1º §2º inciso III da Lei 11.419/2006 Signatário(a): ...";
    - "Este laudo foi assinado eletronicamente";
    - "Você pode conferir a autenticidade do documento em <URL> com o código <X>".
  - Uma frase sobre autenticidade no meio da peça nunca casa, porque a regra só olha as bordas.
  - H6 substitui a regra atual `signaturePrefixes`, que apaga em qualquer posição da página as linhas que começam com "Documento assinado" ou "Assinatura eletrônica" e, com elas, o assinante. A regra sai em todos os níveis.
- **H7, duplicadas viram referência:** uma página com 200 ou mais caracteres idêntica, depois de normalizar os espaços mas sem trocar dígitos, a uma página anterior do processo vira `[conteúdo idêntico a: evento E, <DOC>, pág. P]`. Se o documento inteiro é duplicado, uma referência só vale pelo documento.
  - A comparação usa o texto depois da Básica (H1 e H2) e antes das regras de borda (marcador, H4, timbre, H6). Assim, duas cópias do mesmo documento casam mesmo que o timbre seja tratado de forma diferente em cada uma.
  - Ordem no formatter:
    1. uma pré-passada sobre o processo inteiro, na ordem de saída, calcula a Básica e os hashes de cada página;
    2. em seguida, cada documento é limpo como hoje, e as páginas duplicadas viram a referência.
  - Estado do processo inteiro: só o mapa de hashes.
- **H3a:** remover as linhas sem nenhum caractere alfanumérico, exceto as que contêm `§`, `º`, `ª` ou `%`.
- **H8, formato enxuto, em todos os níveis, inclusive `none`** (decisão minha). É formato, não higienização, e assim o PRD §6 fica com um formato só:
  - sem indentação;
  - `<pag n="3">` no lugar de `<pag num="3">`;
  - documento de 1 página sem `<pag>`;
  - página vazia não emite `<pag>`.

  Isso muda o PRD §6: atualizar o PRD e os testes `TestGenerateConsolidatedOutput*`.

### Máxima (`aggressive`)

Padrão, mais:

- **H3b:** remover as linhas de 1 a 2 letras sem dígito. Nunca remove numeral romano, alínea (`a)`, `x)`) nem sigla de UF.
- **Certificado de plataforma:** a página de certificado de assinatura de plataforma privada ("Termos de Assinatura e Registro Eletrônico", "Nível de segurança", "Enviado/Visualizado/Assinado: <DATA>") vira `[certificado de assinatura eletrônica]`.
- **H9, reflow por parágrafo:** substitui a "página numa linha" de hoje. Junta a linha seguinte quando a atual não termina em `.`, `:` ou `;` e a próxima continua a frase. Mantém a quebra:
  - em linha curta;
  - em linha que parece de tabela (2 ou mais números ou valores `R$`);
  - em marcador de lista (`a)`, `I -`, `•`, `1.`).

  Também junta a hifenização do fim da linha. Um extrato continua com um lançamento por linha.

## Parte 3: omitir tipos de documento (H10)

- Opção no config, independente do nível, com backfill vazio.
- O filtro roda no pipeline, antes da leitura e do OCR (decisão minha): o documento omitido não é lido nem passa pelo OCR, o que poupa tempo também.
  - A pré-contagem de páginas continua, porque o marcador informa o número de páginas.
  - O documento omitido entra no progresso como concluído assim que começa.
  - Na atividade, ele aparece no estado "concluido", com 0 páginas de OCR.
- O usuário marca códigos de documento do eproc (o sufixo do nome do arquivo depois de "Evento N - ", sem os dígitos finais, por exemplo `EXTR`, `COMP`, `PRONT`).
- Cada documento marcado vira `[documento omitido: <TIPO>, N páginas]` no lugar do corpo. O resumo do `<contexto>` continua listando todos os documentos.
- Interface em `Settings.svelte`: lista de códigos editável. O padrão não omite nada.
- O Reprocessar aplica a configuração atual da lista; não há override por item nesta versão.

## Interface e compatibilidade

- `config.LimpezaValida` aceita os quatro ids.
- `Settings.svelte`, `lib/atividade.js` (`OPCOES_LIMPEZA`, que alimenta o atalho cíclico da barra recolhida e o Reprocessar) e os testes de convenção acompanham os novos rótulos.
- Descrições curtas:
  - Nenhuma: "Texto como extraído";
  - Básica: "Remove espaços e linhas vazias";
  - Padrão: "Remove cabeçalhos, rodapés e duplicatas; resume assinaturas";
  - Máxima: "Padrão + ruído de OCR e parágrafos contínuos".

## Decisões do dono (2026-11-08)

- Planejar as correções de extração e os níveis juntos.
- O timbre fica só na primeira ocorrência de cada documento, a partir da Padrão.
- A assinatura digital dos HTMLs fica só com o nome do assinante, a partir da Padrão.
- O restante segue a proposta do estudo.

## Decisões minhas (o dono pode revisar)

- **Padrão do config:** `standard` passa a ser o valor de `DefaultConfig()`. Configs existentes mantêm o valor gravado, porque o backfill só preenche campos ausentes. Não há migração: o `SaveToFile` grava todos os campos, então não dá para distinguir quem escolheu `basic` de quem só recebeu o padrão. O usuário muda o nível pelo atalho da barra recolhida.
- **Id da Máxima:** a Máxima mantém o id `aggressive` para compatibilidade.
- **Escopo de H7 e do timbre:** H7 compara páginas do processo inteiro. O timbre continua por documento.
- **Ordem dos commits:** E2, E3, E1, E4, níveis e H10, um commit por parte.

## Verificação

- TDD com texto sintético para cada regra e fixtures PDF sintéticas para E1 e E4.
- Gate: `pwsh scripts/build/gate.ps1`, `svelte-check`, os testes `frontend/*.test.mjs` e as capturas visuais das configurações.
- Medição com os ZIPs reais da pasta Processos, feita em cópias no scratchpad e sem publicar os dados: tokens por nível antes e depois. Meta: a Padrão economiza pelo menos 15% em relação à Básica de hoje.
- Conferência manual por amostragem nas saídas reais:
  - assinantes preservados;
  - extratos com um lançamento por linha;
  - acentos corretos nos HTMLs.

## Fica para o builder

- Os limiares de E1 e E4 e a forma de obter a posição e a largura dos glifos na `dslipak/pdf`.
- As regexes exatas de cada família de chancela.
- A heurística de "linha curta" e de "continuação de frase" em H9.
- A organização do código. Sugestão: cada regra como uma função pura testável, e `cleanPage` compondo as funções por nível.
