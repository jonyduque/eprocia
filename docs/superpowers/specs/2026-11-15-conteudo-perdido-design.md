# Páginas "lidas" que perdem conteúdo: desenho da v0.4.2

Data: 2026-11-15. Autor: jonyduque (redigido pelo Claude Code a partir de um brainstorm de três lentes: estrutura do PDF, ferramentas externas e produto). Branch: `ocr-conteudo-perdido`.

## Por que

Quem usa o eprocIA cola o `.txt` num LLM para analisar um processo. A v0.4.1 passou a avisar quando um documento inteiro não é lido. Sobrou um caso pior de perceber: a página conta como lida, o documento conta como lido, e parte do conteúdo da página não está no `.txt`.

O dono relatou três situações, vistas em processos reais:

1. **Legenda bloqueia o OCR.** A página tem um pouco de texto nativo (legenda, cabeçalho, rodapé) e o resto é imagem. Com 100 bytes ou mais de texto nativo, a página nunca vai ao OCR; só a legenda sai.
2. **Mosaico.** O texto da página é formado por várias imagens pequenas. Nenhuma passa de 100×100 px, então a página não é candidata ao OCR.
3. **Texto-isca.** Um documento bancário cujo texto nativo são letras e números aleatórios, em baixo contraste, espalhados pela página; a informação está em imagens pequenas. O `.txt` recebe o lixo e perde a informação.

O brainstorm achou uma quarta causa, que explica parte da situação 1: `extrairTextoPosicional` ignora o operador `Do`. Texto dentro de um Form XObject some, e a página conta como lida. O dono confirmou que, na página em que só a legenda sai, o corpo é selecionável num leitor de PDF: é esse caso.

Restrição do dono: essas situações são excepcionais. Não vale aumentar muito o tempo dos documentos normais para melhorar alguns documentos.

## Decisões do dono (2026-11-15)

- **Só automático.** OCR automático nas páginas de sinal forte, sem aviso novo no item nem no `.txt` e sem botão novo. As páginas de sinal fraco continuam saindo incompletas sem aviso. Exceção pedida pelo dono em 2026-11-16: a falha do OCR deixa no `.txt` um texto que diz que houve erro no OCR e pede ao modelo de IA que avise o usuário (ver a seção da junção).
- **Form XObject:** o texto dentro de Form entra no texto nativo, sem OCR.
- **Texto-isca:** automático. Página com imagem cujo texto nativo não parece português vai ao OCR, e o texto nativo só é descartado se o OCR devolver texto.
- **Release:** v0.4.2 própria, só com este pacote, antes dos 46 achados da revisão de 2026-11-10.

## Decisões do dono (2026-11-16, sobre esta spec)

- **Limite da imagem descoberta:** fica em 30% da página. Um print ou comprovante que cobre de 15% a 30% continua de fora, como hoje.
- **Ordem na junção:** quando a página vai ao OCR e tem texto nativo que o OCR não trouxe, tentar montar o texto na ordem em que ele está na página, em vez de pôr o nativo no fim.
- **Sem log de diagnóstico.** A linha de log por página, só com números, não entra. A calibração dos limiares usa a medição final, fora do aplicativo.

## Objetivos

1. O texto dentro de Form XObject sai no `.txt`, na posição em que aparece na página.
2. Página com imagem cobrindo boa parte da área, sem texto nativo por perto, vai ao OCR mesmo com 100 bytes ou mais de texto nativo. Vale para imagem única, imagem dentro de Form, imagem inline e mosaico.
3. Página com imagem cujo texto nativo não parece português vai ao OCR.
4. O OCR nunca piora a página: se devolver texto em branco ou ilegível, fica o texto nativo (residual M2 de `docs/revisao/2026-11-14-residuais-pdfs-nao-lidos.md`).
5. O tempo de uma página normal não sobe de forma mensurável: a decisão vem da mesma passada que já extrai o texto. Não há processo novo nem binário novo.

## Fora do escopo

- Aviso, marcador ou botão novo (decisão do dono).
- Página de sinal fraco: imagem que cobre menos de 30% da página (um recibo digitalizado colado numa petição), ou imagem grande ao lado de muito texto nativo. Continua como hoje.
- OCR só do recorte sem texto ("OCR do restante"). O OCR segue na página inteira; o recorte exigiria um lote com recortes diferentes por página.
- Checagem de tinta em baixa resolução (um `pdftoppm -r 12` por página suspeita) para reduzir falsos positivos. Entra só se o falso positivo aparecer na prática.
- Caminho do segundo leitor (PDF que a biblioteca recusa, lido pelo `pdftotext`): lá não há a estrutura da página. Só o objetivo 4 vale nesse caminho.
- Imagem inline com dados binários (faz `lexerTermina` rejeitar o stream; a página perde o texto nativo e hoje só vai ao OCR se o stream passar de 4.096 bytes), imagem em anotação (`/Annots /AP`), preenchimento com `/Pattern`.
- Mascarar a imagem ou o texto antes do OCR, trocar o motor de OCR, embutir `pdfimages.exe`.

## O que seria um fracasso mesmo funcionando

- O `.txt` de um documento normal mudar. Uma página sem Form e sem imagem grande tem de sair byte a byte igual à v0.4.1.
- O texto nativo bom de uma página ser trocado por um OCR pior ou vazio.
- Texto oculto (camada opcional desligada, Form pintado fora da página) aparecer no `.txt`.
- O processamento de um ZIP normal ficar visivelmente mais lento.
- Um caminho, erro bruto, nome de objeto ou trecho de conteúdo do PDF chegar ao log: a única linha de log nova (Form ignorado) leva um motivo fixo e o número da página.

## Desenho

Quatro partes, todas em Go, em `internal/etl`.

### 1. Texto de Form XObject no texto nativo

O interpretador de `pdf_texto.go` passa a tratar `Do`. Quando o XObject é `/Subtype /Form`:

- o stream passa por `lerStream` antes de o lexer da biblioteca vê-lo (mesma proteção do `/Contents`; um Form ilegível é ignorado e registrado no log, o resto da página segue);
- a matriz do Form (`/Matrix`) multiplica a CTM; o estado gráfico é salvo antes e restaurado depois, como num `q … Q`;
- as fontes e os XObjects vêm dos `/Resources` do Form; sem eles, dos recursos de quem o chamou;
- a recursão para na profundidade 4, e uma página interpreta no máximo 256 Forms (o que passar é ignorado e registrado no log);
- os trechos de texto do Form entram na mesma lista da página e passam por `montarLinhas`, então saem na ordem de leitura.

Proteções contra texto que não se vê:

- Form com a chave `/OC` no dicionário, ou `Do` dentro de um trecho marcado `/OC … BDC … EMC`, é ignorado por inteiro (como hoje). Decidir se a camada está ligada exigiria ler `/OCProperties`; ignorar é o comportamento atual e não piora nada. Dentro de um Form, o texto mostrado num trecho `/OC … BDC … EMC` também é ignorado. O texto do `/Contents` da página dentro de um trecho desses segue saindo, como hoje.
- Trecho de Form cuja origem cai fora do MediaBox é descartado. Sem MediaBox legível, essa proteção não se aplica e o trecho entra.
- O mesmo Form pintado duas vezes no mesmo lugar não duplica o texto: trechos de Form com o mesmo texto e a mesma posição (arredondada a 0,5 pt) contam uma vez.

As duas últimas proteções valem só para trechos vindos de Form. O texto do `/Contents` da página segue exatamente como hoje.

Medido no protótipo (PDF sintético, corpo em Form mais legenda): 177 bytes hoje, 4.009 com o conserto (o `pdftotext` dá 4.057); a extração da página vai de 171 µs para cerca de 840 µs, proporcional ao texto a mais.

### 2. Imagem descoberta

Na mesma passada, o interpretador registra a estrutura da página:

- **Imagens:** cada `Do` de `/Subtype /Image` (na página ou dentro de Form) e cada imagem inline (`BI … ID … EI`, que o lexer entrega no operador `ID`) vira uma caixa: o quadrado unitário transformado pela CTM, mais `/Width` e `/Height` em pixels.
- **Texto:** cada trecho vira uma caixa, de `x` a `fimX` na largura e com a altura do corpo. Texto invisível (`Tr 3`, a camada de OCR de um PDF digitalizado) conta como texto.
- **Página:** o MediaBox, herdado por `/Parent` quando a página não tem o seu.

Imagem útil: pelo menos 256 px de área e resolução efetiva de 50 dpi ou mais nos dois eixos. Isso descarta o preenchimento de 1×1 px esticado e o fundo em baixa resolução.

A página é dividida numa grade de 48×68 células (cerca de 12 pt numa A4). Uma célula é de imagem se o centro dela cai dentro de uma imagem útil, e de texto se alguma caixa de texto a toca. Três frações da página saem daí:

- `fImg`: células de imagem;
- `fTxt`: células de texto;
- `fDesc`: células de imagem sem nenhuma célula de texto a até uma célula de distância.

A passada guarda mais um número, usado só pela junção (seção 4): **linhas acima das imagens**, a quantidade de linhas do texto nativo cuja linha de base fica na altura do topo da imagem útil mais alta, ou acima dele. Como o texto nativo sai de cima para baixo, essas são sempre as primeiras linhas.

**Regra:** com o OCR ligado, a página é candidata se `fDesc ≥ 0,30` e `fTxt ≤ 0,10`. A regra vale para qualquer tamanho de texto nativo e se soma às de hoje (texto ilegível; texto curto com imagem grande; texto curto com desenho vetorial), que não mudam.

Sem MediaBox legível, ou quando o texto veio do `GetTextByRow` (o extrator posicional falhou), não há estrutura e a página segue só as regras de hoje.

Os limiares vêm de 25 páginas sintéticas e são valores iniciais:

| página sintética | fImg | fTxt | fDesc | resultado |
|---|---|---|---|---|
| texto puro, extrato nativo, certidão com hashes | 0 | 0,34–0,51 | 0 | nativo |
| logotipo; assinatura digitalizada | 0,01–0,02 | 0,46–0,51 | 0,01–0,02 | nativo |
| timbre em imagem de página inteira, meia página de texto | 1,00 | 0,25 | 0,71 | nativo |
| digitalizado com camada de OCR invisível | 1,00 | 0,55 | 0,39 | nativo |
| petição com print de 400×300 pt | 0,24 | 0,25 | 0,24 | nativo |
| legenda mais imagem (direta, em Form, inline em hexa) | 0,96 | 0,02 | 0,95 | OCR |
| carimbo sobre a digitalização | 1,00 | 0,01 | 0,97 | OCR |
| mosaico de 237 imagens mais legenda | 0,42 | 0,02 | 0,42 | OCR |
| timbre em imagem de página inteira mais 3 linhas | 1,00 | 0,02 | 0,97 | OCR (falso positivo conhecido) |

A calibração em processos reais acontece na medição final (ver "Testes e medição").

### 3. Texto-isca

Arquivo novo `internal/etl/lingua.go`. Sobre o texto nativo da página, conta:

- **palavras:** as que têm letra e 3 caracteres ou mais, e as palavras funcionais;
- **implausíveis:** duas ou mais trocas entre letra e dígito (`k7Qz9X`); minúscula seguida de maiúscula no meio da palavra (`aBc`); 4 letras ou mais com menos de 15% de vogais, salvo sigla em maiúsculas de até 5 letras;
- **neutras** (não entram na conta): só dígitos e pontuação (datas, valores, CPF, número do processo) e hexadecimal com dígito (hashes);
- **funcionais:** `de da do das dos a o as os e em no na nos nas para por com que não ao à aos se um uma é`.

**Regra:** com o OCR ligado, a página é isca se tem 30 palavras ou mais, 35% ou mais delas implausíveis, menos de 5% funcionais, **e** `fImg ≥ 0,10`. O teste de língua só roda quando `fImg ≥ 0,10`, então página sem imagem não paga por ele e nunca é tratada como isca (uma certidão de hashes ou um rodapé cheio de URLs não têm imagem).

Medido nas sintéticas: iscas com 80–85% de palavras implausíveis e nenhuma funcional; páginas normais com 0%, salvo a de rodapés com URLs e códigos (30% de implausíveis, com 17,5% de funcionais).

### 4. Junção do OCR com o texto nativo

`ocrTask` passa a levar o motivo da candidatura, e `registrarOCR` decide o conteúdo da página por ele. Em todos os motivos, quando há reserva (o texto nativo, ou o marcador de ilegível), um OCR que devolve texto em branco ou que `textoIlegivel` reprova não a substitui: fica a reserva, e a página sai com `ocr: false`. Sem reserva (página digitalizada sem texto nativo), o resultado do OCR é gravado como hoje, qualquer que seja.

| motivo | reserva | OCR com texto aproveitável |
|---|---|---|
| texto ilegível (hoje) | marcador `[texto ilegível na página: N caracteres omitidos]` | substitui (como hoje) |
| texto curto com imagem ou desenho (hoje) | o texto nativo curto | substitui (como hoje) |
| imagem descoberta (novo) | o texto nativo | texto do OCR, com as linhas nativas que o OCR não trouxe encaixadas na ordem da página |
| isca (novo) | o texto nativo | substitui: o nativo é descartado |

Na imagem descoberta, uma linha nativa está "presente" no OCR se 60% ou mais das palavras de 3 caracteres ou mais dela estão no conjunto de palavras do OCR, comparadas em minúsculas e sem acento. Linha nativa sem nenhuma palavra de 3 caracteres ou mais (um número de página) está presente se o texto dela aparece no OCR. Se nenhuma linha falta, o resultado é o texto do OCR, sem mudança.

O OCR devolve texto sem posição, então a ordem da página é reconstruída por âncoras. A âncora de uma linha nativa presente é a linha do OCR que tem mais palavras dela (no empate, a primeira). As linhas nativas ficam de um de dois lados: acima das imagens (as primeiras, contadas na seção 2) ou abaixo (as demais). Cada linha nativa que falta entra, na ordem em que estava:

1. logo depois da âncora da linha nativa presente mais próxima antes dela, se as duas estão do mesmo lado;
2. sem essa linha: no começo, antes do texto do OCR, se ela é uma das linhas acima das imagens;
3. senão, logo antes da âncora da próxima linha nativa presente;
4. sem nenhuma dessas âncoras: no fim, depois do texto do OCR.

O lado importa na regra 1 porque a imagem fica entre os dois: uma legenda embaixo da figura não pode ser colada na linha do cabeçalho só porque essa foi a última que o OCR achou.

As linhas postas no começo ou no fim ficam separadas do texto do OCR por uma linha em branco; as encaixadas no meio, não. Nenhuma linha nativa é perdida, qualquer que seja o OCR. É uma tentativa: sem âncora, o que se sabe é só se a linha está acima das imagens ou não, e uma linha que fica entre duas imagens vai para o fim.

Uma falha do OCR (erro ou prazo) numa página das regras de hoje segue como hoje: fica a reserva e a página leva `ocr_erro`. Numa página que só foi ao OCR pelas duas regras novas, a falha deixa a página como a v0.4.1 a gravava: o texto nativo, sem `ocr_erro`. Com `ocr_erro` a página não contaria como texto, e um documento de uma página só sairia como não lido, sem o texto que antes saía (corrigido na revisão final, em 2026-11-16). Em qualquer regra, a página cujo OCR falhou e ficou com a reserva leva `ocr_aviso`, e o formatter põe antes do texto dela uma linha fixa: `[aviso de OCR: houve erro no OCR desta página e o texto dela pode estar incompleto; avise o usuário e recomende a consulta ao PDF original]`. A linha entra depois da limpeza, e a página com aviso não é duplicata nem origem de duplicata. A página sem reserva leva o marcador `[ERRO de OCR: <frase>; avise o usuário de que houve erro no OCR desta página e recomende a consulta ao PDF original]`. O texto não precisa dizer qual foi o erro (pedido do dono em 2026-11-16).

### OCR desligado

Com o OCR desligado, as páginas das duas regras novas saem como hoje: o texto nativo fica, a página conta como lida e não leva marcador nem `sem_ocr`. É o mesmo tratamento que a página digitalizada com um pouco de texto nativo já tem (residual M1, fixado por `TestPipeline_SemOCR_TextoNativoCurtoComImagemContaComoLido`), e segue a decisão "sem aviso novo". O conserto do Form XObject não depende do OCR e vale sempre.

### Log

Não há log de diagnóstico por página (decisão do dono). A única linha de log nova é a do Form ignorado (seção 1): o número da página e um motivo fixo (`stream ilegível`, `profundidade`, `limite por página`, `pânico`), sem o erro bruto e sem o nome do objeto. Nenhuma outra linha de log muda.

## Erros de cada regra e o efeito no `.txt`

- **Falso positivo de imagem descoberta:** timbre em imagem de página inteira com poucas linhas; página de fotos com legenda; gráfico em imagem. Custa um OCR a mais (4 a 8 s por página, medido na lente de ferramentas). O texto nativo fica, porque a junção só acrescenta; o `.txt` ganha o texto do timbre (que a higienização de timbre tende a tirar) ou algum lixo do OCR de uma foto.
- **Falso negativo de imagem descoberta:** imagem que cobre de 15% a 30% da página; meia página de texto ao lado de meia página digitalizada. Nada muda em relação a hoje: o conteúdo da imagem some sem aviso.
- **Falso positivo de isca:** comprovante com imagem grande e muitos códigos alfanuméricos (chave PIX aleatória, QR code). O texto nativo é trocado pelo OCR e os códigos perdem exatidão. A exigência de menos de 5% de palavras funcionais é a proteção: um comprovante real costuma ter rótulos em português.
- **Falso negativo de isca:** isca feita de palavras reais embaralhadas, ou só de dígitos. Nada muda em relação a hoje.
- **Form XObject:** texto de carimbo ou marca-d'água dentro de Form passa a aparecer. É texto visível na página, então é o comportamento esperado; a higienização trata o que se repete.

## Testes e medição

- **PDFs sintéticos gerados em Go**, com dados fictícios (`FULANO DE TAL`, `5000000-00.2026.8.26.0001`), nos testes de `internal/etl`. Nenhum PDF real entra no repositório.
- **Controles que não podem mudar:** texto puro; logotipo; assinatura digitalizada; timbre em página inteira com meia página e com página cheia de texto; digitalizado com camada de OCR invisível; extrato nativo; petição com print de meia página; certidão com hashes; rodapés com URLs. Nenhum é candidato, e o texto nativo deles sai igual ao de hoje.
- **Falso positivo documentado:** timbre em página inteira mais 3 linhas é candidato; o teste fixa que o texto nativo permanece no resultado.
- **Casos que têm de ir ao OCR:** legenda mais imagem (direta, em Form, inline em hexa); carimbo sobre a digitalização; mosaico com legenda; as três iscas (cinza claro, preta, em linhas regulares).
- **Form:** corpo em Form mais legenda sai inteiro no texto nativo, sem OCR; Form sob `/OC`, Form fora da página, Form pintado duas vezes, Form que chama a si mesmo e Form com stream ilegível.
- **Junção:** tabela de casos para cada motivo, com OCR aproveitável, em branco, ilegível e com erro; e a ordem da página: linha que falta depois da âncora anterior, antes da próxima, no começo (acima das imagens) e no fim.
- **Desempenho:** `BenchmarkParsePDF` e `BenchmarkSintetico` com `-count=10`, comparados por benchstat com a `main`. A página de texto puro não pode ficar pior que 5%.
- **Medição real:** os três ZIPs reais do dono, em cópias no scratchpad, com `TEMP`/`TMP` redirecionados. Saem só contagens: quantos blocos `<documento>` ficaram iguais aos da v0.4.1, quantas páginas foram ao OCR por cada regra nova, quantas páginas ganharam texto de Form, a distribuição de `fDesc` (inclusive quantas páginas caem na faixa de 0,15 a 0,30) e o tempo total antes e depois. Como não há log de diagnóstico, os números das páginas vêm de um arquivo de teste temporário, que chama as funções de estrutura direto sobre as cópias, imprime só contagens e não entra no repositório. Se a medição mostrar um limiar mal calibrado, ele é ajustado antes do PR e a tabela desta spec é atualizada.

## Escolhas do redator (não foram decididas pelo dono)

1. Com o OCR desligado, as páginas das regras novas não levam marcador (o relatório de estrutura propunha `sem_ocr` mais o marcador). Motivo: a decisão "sem aviso novo" e a coerência com o M1.
2. Os limiares (`fTxt ≤ 0,10`, `fImg ≥ 0,10`, 30 palavras, 35%, 5%, 60%) são os do protótipo, medidos só em PDFs sintéticos. O de `fDesc ≥ 0,30` foi confirmado pelo dono em 2026-11-16.
3. A ordem da página na junção (pedido do dono) é reconstruída por âncoras, com as quatro regras da seção 4. O que é escolha do redator: a âncora é a linha do OCR com mais palavras da linha nativa, e só vale para linhas do mesmo lado das imagens; sem âncora, só as linhas acima das imagens vão para o começo e o resto vai para o fim; a rotação da página (`/Rotate`) não é considerada.
4. O lixo do OCR não é filtrado linha a linha: os dois testes existentes (`textoIlegivel` e o de língua) precisam de mais texto do que uma linha tem. Vale o teste do OCR inteiro.
5. Form sob camada opcional (`/OC`) é ignorado sem consultar se a camada está ligada.
6. Sem o log de diagnóstico (decisão do dono), fica uma linha de log quando um Form é ignorado, com motivo fixo, no padrão das linhas que o leitor de PDF já escreve. Sem ela, um Form que não pôde ser lido some sem rastro.
7. O caminho do segundo leitor (Poppler) recebe só o objetivo 4.
8. Página de texto ilegível cujo OCR volta em branco passa a sair com o marcador `[texto ilegível na página: N caracteres omitidos]`, que já existe, em vez de vazia. Um documento só com páginas assim passa de "nenhum texto encontrado" para "arquivo ilegível"; continua não lido.
9. Quando a página cumpre as duas regras novas, vale a de isca: o texto nativo é trocado pelo do OCR, em vez de juntado a ele. Juntar traria o lixo da isca de volta para o `.txt`.
