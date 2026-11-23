# CP9, P3 e P6 conforme as decisões do dono: plano de implementação

> **Para os implementadores:** siga `docs/superpowers/plans/2026-11-22-residuais-regras.md` (onde trabalhar, TDD, gate, commit, proibições, relatório). Cada tarefa é um commit. Os passos usam a sintaxe `- [ ]`.

**Meta:** aplicar três decisões do dono de 2026-11-22 sobre os residuais de `docs/revisao/2026-11-14-residuais-pdfs-nao-lidos.md`: CP9 (proteger `HasImages` e mandar a página ao OCR), P3 (o leitor de tela anuncia "Não lido" em vez de "Erro:") e P6 (o divisor não deixa a fila abaixo da largura em que o rótulo "N não lidos" aparece).

**Arquitetura:** CP9 é só `internal/etl/pdf_parser.go`. P3 e P6 são só frontend (`AtividadeItem.svelte`, `lib/divisor.js`). Sem mudança de binding Wails.

**Stack:** Go (`go test`), Svelte 5 sintaxe legada, `node --test`.

## Global Constraints

- Decisões do dono (2026-11-22), ao pé da letra:
  - CP9: "Proteger e fazer OCR." Recover dentro de `HasImages`; a página cujo exame der pânico conta como página COM imagens e vai ao OCR (se o OCR estiver ligado). Uma linha de log em pt-BR.
  - P3: "Anuncia": o leitor de tela anuncia "Não lido" em vez de "Erro:". O critério é `Leitura` (`naoLido(d)`), não só `Estado == erro`.
  - P6: limitar o divisor para que a fila mantenha a largura mínima em que o rótulo "N não lidos" aparece (medição no commit `2243370`: some em 800x600 com o divisor no máximo).
- Logs e textos de interface em pt-BR. Dados de teste sintéticos.
- Não editar `frontend/src/wailsjs/**`, `frontend/dist/**`, `internal/ocr/embed/**`.
- Tasks 1 a 3 independentes entre si; a Task 4 (documentação) vem por último e usa os hashes dos commits das outras.

## Review Focus

- CP9: o `recover` fica dentro de `HasImages` e devolve `true`; o `recover` de fora de `analisarPagina` não pode ser a rede que pega este caso. Com o OCR desligado, a página vira `sem_ocr` (por `candidata`), nunca erro de leitura. A página com texto suficiente continua sem chamar `HasImages`.
- P3: nenhum caminho anuncia "Erro" para documento `sem_ocr`, `senha`, `ilegivel`, `prazo`, `ocr_falhou` ou `sem_texto`; o erro real (`estado=erro` sem leitura de não lido) continua como "Erro".
- P6: a constante mede o que o rótulo exige (valor medido, não chutado); o limite continua cabendo na janela mínima de `main.go` sem inverter o intervalo (`larguraMaxPainel` nunca abaixo de `LARGURA_MIN_PAINEL`).

---

### Task 1: CP9, recover em HasImages com a página indo ao OCR

**Files:**
- Modify: `internal/etl/pdf_parser.go` (`HasImages`, linhas ~585-610)
- Modify: `internal/etl/pdf_parser_test.go`

- [ ] **Passo 1: teste que falha.** Em `pdf_parser_test.go`, ao lado de `TestPDFParser_ParsePages_XrefInconsistenteEmHasImagesNaoEntraEmPanic` (linha ~360), acrescente `TestHasImages_PanicContaComoComImagens` e `TestPDFParser_ParsePages_PanicEmHasImagesVaiAoOCR`.
  - Monte (com `buildPDF`) uma página de texto curto (`(Oi) Tj`, menos de 100 caracteres) cujo `/Resources << /XObject 5 0 R >>` aponte para um dicionário `5 0 obj << /Im1 6 0 R >>` e onde o objeto `6 0 R` tenha a entrada da xref apontando para outro número (ex.: o objeto está gravado como `9 0 obj`), a falha de tabela inconsistente que faz a biblioteca dar `panic("loading 6 0 R: found 9 0")` quando `obj.Key("Subtype")` resolve a imagem. O implementador confirma, rodando `HasImages` direto numa cópia local, que este PDF panica de fato; se não panicar, ajuste a corrupção até panicar (o teste precisa exercitar o `recover` novo, não passar por acaso).
  - Teste 1 (unitário): abre a página 1 pelo leitor e afirma `HasImages(page) == true` sem panic (`defer recover` no teste que chama `t.Fatalf`).
  - Teste 2 (`ParsePages` com `ocrEnabled=true` e `ocrRunner` falso que devolve `"OCR Extracted Text"`): a página 1 sai com conteúdo do OCR e sem marcador `[ERRO`; o runner foi chamado uma vez. Variante com `ocrEnabled=false`: a página sai marcada como não lida (`sem_ocr`), sem `[ERRO` de leitura.
- [ ] **Passo 2: ver falhar.**
  `go test ./internal/etl -run "TestHasImages_PanicContaComoComImagens|TestPDFParser_ParsePages_PanicEmHasImagesVaiAoOCR" -v > <scratchpad>\cp9-antes.txt 2>&1`
  Esperado: FAIL pelo motivo certo (panic propagando de `HasImages`, ou a página saindo com `[ERRO` em vez do texto do OCR). Registre a linha da falha no relatório.
- [ ] **Passo 3: implementar o mínimo.** Dê a `HasImages` um retorno nomeado e um `defer`:

  ```go
  func HasImages(page pdf.Page) (temImagens bool) {
  	defer func() {
  		if r := recover(); r != nil {
  			log.Printf("[PDFParser] Falha ao examinar as imagens da página (%v); tratada como página com imagens para ir ao OCR", r)
  			temImagens = true
  		}
  	}()
  	// corpo atual, sem mudança
  }
  ```

  Atualize o comentário de `HasImages` com a decisão do dono (2026-11-22).
- [ ] **Passo 4: ver passar.** Mesmo comando do Passo 2: PASS. Depois `go test -race ./internal/etl` (sem `-short`) e `go vet ./...`, saída em arquivo.
- [ ] **Passo 5: commit** `fix(etl): HasImages com recover, a página vai ao OCR [CP9]`. Arquivos: os dois acima.

---

### Task 2: P3, leitor de tela anuncia "Não lido" em vez de "Erro:"

**Files:**
- Modify: `frontend/src/lib/atividade.js` (nova função exportada `rotuloEstadoDoc`)
- Modify: `frontend/src/components/AtividadeItem.svelte` (linha ~148)
- Modify: `frontend/atividade.test.mjs`
- Modify: `frontend/naoLidos.test.mjs` (só se a regex de fonte pedir)

Consumidores levantados (tudo que fala de erro ao leitor de tela): (a) `AtividadeItem.svelte:148`, o `<span class="sr-only">{ROTULO[d.estado]}:</span>` de cada linha, que hoje dá "Erro:" para documento `sem_ocr`/`senha`/etc., pois eles terminam com `Estado=erro`; (b) `lib/anuncios.js` `marcosFila`, que já usa `resumoAtividade(...).erros`, e este já exclui os não lidos (`d.estado === 'erro' && !nl`), então não muda; (c) as regiões `role="status"` de `App.svelte`, `ProcessList.svelte` e `Settings.svelte` só repetem o texto de `anuncios.js`; (d) `Aviso.svelte` (`role="alert"`) recebe o texto de `avisoCopiaNaoLidos`, que já diz "não lidos". O implementador refaz esta busca (`rg -n "Erro|sr-only|aria-live|role=\"(status|alert)\"" frontend/src`) e registra no relatório se achou outro consumidor.

- [ ] **Passo 1: teste que falha.** Em `atividade.test.mjs`:
  - `rotuloEstadoDoc({ estado: 'erro', leitura: 'sem_ocr' }) === 'Não lido'`, idem para cada valor de `LEITURA_NAO_LIDO`;
  - `rotuloEstadoDoc({ estado: 'erro' }) === 'Erro'`; `{ estado: 'concluido' }` dá `'Concluído'`; `{ estado: 'ocr' }` dá `'Em OCR'`; estado desconhecido devolve o próprio estado.
  - Teste de fonte (regex, no padrão dos outros): `AtividadeItem.svelte` usa `rotuloEstadoDoc(d)` no `sr-only` e não contém mais `ROTULO[d.estado]`.
- [ ] **Passo 2: ver falhar.** `node --test frontend/atividade.test.mjs` (Git Bash). Esperado: falha por `rotuloEstadoDoc` inexistente (import) e pela regex de fonte. Registre.
- [ ] **Passo 3: implementar.** Mova `ROTULO` de `AtividadeItem.svelte` para `atividade.js` e exporte `rotuloEstadoDoc(d)`: `naoLido(d) ? 'Não lido' : (ROTULO[d?.estado] ?? d?.estado)`. No componente, importe-a e troque o `sr-only` por `{rotuloEstadoDoc(d)}:`. Remova o `ROTULO` local se ficar sem uso.
- [ ] **Passo 4: ver passar e gate de frontend.** `node --test frontend/*.test.mjs`, depois em `frontend`: `npx svelte-check` e `npm run build`, saídas em arquivo. Rode em especial `atividade`, `naoLidos`, `anuncios`, `regiaoViva`, `estado`, `painel`.
- [ ] **Passo 5: commit** `fix(frontend): leitor de tela anuncia "Não lido" em vez de "Erro:" [P3]`.

---

### Task 3: P6, o divisor mantém a fila na largura em que o rótulo "N não lidos" aparece

**Files:**
- Modify: `frontend/src/lib/divisor.js` (`LARGURA_MIN_FILA` e o comentário do cabeçalho)
- Modify: `frontend/layout.test.mjs`
- Modify (se a medição pedir): `frontend/src/components/ProcessList.svelte`

Contexto: `2243370` mostrou (capturas `docs/revisao/capturas/2026-11-22/P6/800x600-*-nao-lidos-divisor-maximo.png`) que em 800x600 com o divisor no máximo (`larguraMaxPainel(800) = 800 - 340 = 460`) o rótulo some. O `.nao-lidos` fica em `.details-row` (linha com `overflow: hidden`, `row-reverse`, quebra para uma segunda linha recortada), e o nome do ZIP ocupa o que precisa antes.

- [ ] **Passo 1: medir a largura mínima da fila em que o rótulo aparece.** Leia `.claude/skills/eprocia-visual/SKILL.md`. Com o Vite e os stubs (estado `nao-lidos`), em 800x600, arraste o divisor em passos (por exemplo `larguraPainel` de 460 para baixo, de 10 em 10 px) e meça, por Playwright no mesmo contexto do `scripts/visual/capturar.mjs` (script de medição no scratchpad, nunca no repositório), a menor largura de `.list-card` em que `.nao-lidos` tem `getBoundingClientRect().width > 0` e fica dentro de `.details-row` (não recortado: `bottom` e `right` do rótulo dentro do retângulo da linha). Repita para os nomes de ZIP mais longos dos stubs. Registre a tabela (largura da janela, painel, largura da fila, rótulo visível sim/não) no relatório. Encerre o Vite e o navegador pelo PID ao fim.
- [ ] **Passo 2: teste que falha.** Em `layout.test.mjs`, acrescente `LARGURA_MIN_FILA_NAO_LIDOS` (o valor medido, com a margem de 8 px acima do último ponto em que ainda aparece, para o `gap` e a barra de rolagem) e afirme: `800 - larguraMaxPainel(800) >= LARGURA_MIN_FILA_NAO_LIDOS`, e `limitarPainel(480, 800) === larguraMaxPainel(800)`. Ajuste os asserts existentes que citam 340 só se o novo valor os invalidar (`larguraMaxPainel(800)` passa a `800 - LARGURA_MIN_FILA`, que o teste já expressa pela constante). Mantenha `larguraMaxPainel(500) === LARGURA_MIN_PAINEL` e `larguraMaxPainel(1920) === 480`.
- [ ] **Passo 3: ver falhar.** `node --test frontend/layout.test.mjs`. Esperado: falha no assert novo (460 de painel deixa 340 de fila, menos que o medido). Registre.
- [ ] **Passo 4: implementar.** Suba `LARGURA_MIN_FILA` em `divisor.js` para o valor medido (o limite é por janela: se o valor deixar `larguraMaxPainel(800)` abaixo de `LARGURA_MIN_PAINEL`, o piso de 280 vale e o relatório diz que o rótulo não cabe nem assim, caso em que se para e responde `NEEDS_CONTEXT`). Atualize o comentário do arquivo: o mínimo é o que mantém o "N não lidos" visível, além dos botões do cabeçalho. Se a medição mostrar que o corte vem de CSS (nome sem `min-width: 0`, por exemplo) e não só da largura, descreva e corrija o mínimo possível em `ProcessList.svelte`, justificando no relatório.
- [ ] **Passo 5: ver passar e gate.** `node --test frontend/*.test.mjs`, `npx svelte-check`, `npm run build` (em `frontend`), saídas em arquivo.
- [ ] **Passo 6: verificação visual.** Rode `node scripts/visual/capturar.mjs --saida docs/revisao/capturas/2026-11-22/P6-depois --estados nao-lidos` (gera `-divisor-maximo` em todos os tamanhos e temas) e confira em 800x600, claro e escuro, que "N não lidos" aparece; veja as imagens. Só as capturas de 800x600 `...-nao-lidos-divisor-maximo.png` (as 2 de depois) entram no commit; as demais descarte. Cite no relatório o par antes (`docs/revisao/capturas/2026-11-22/P6/`) e depois. Limitação da skill: o Chromium não é o WebView2; descreva o teste manual de arrastar o divisor até o limite.
- [ ] **Passo 7: commit** `fix(frontend): divisor deixa a fila na largura do rótulo "N não lidos" [P6]`. Arquivos: `divisor.js`, `layout.test.mjs`, as 2 capturas de depois e, se mexer, `ProcessList.svelte`.

---

### Task 4: documentação dos residuais

**Files:**
- Modify: `docs/revisao/2026-11-14-residuais-pdfs-nao-lidos.md`

Sem teste. Use os hashes curtos dos commits das Tasks 1 a 3 (`git log --oneline -5`). Marque no padrão das linhas já resolvidas (`(resolvido em 2026-11-22, <hash>)` logo depois do id, na coluna Id):

- [ ] CP9: `CP9 (resolvido em 2026-11-22, <hash Task 1>)`.
- [ ] P3: `P3 (resolvido em 2026-11-22, <hash Task 2>)`.
- [ ] P6: a linha "P6 (medido em 2026-11-22, 2243370: ...)" vira `P6 (resolvido em 2026-11-22, <hash Task 3>; medido em 2243370)`; mantenha o texto da medição.
- [ ] P18: `P18 (sem ação por decisão do dono, 2026-11-22)`.
- [ ] CP10: no fim da coluna descritiva, acrescente "A RBc será planejada depois, por decisão do dono (2026-11-22)." Troque "RBc fica para decisão do dono" pela mesma ideia sem duplicar a frase.
- [ ] M5: `M5 (substituído pela spec docs/superpowers/specs/2026-11-22-relatorio-processamento-design.md, 2026-11-22)`. Não toque o arquivo da spec (outra sessão o escreve).
- [ ] B-EF-05 (formatador em paralelo): vive em `docs/revisao/2026-11-10-relatorio.md` (seção `### B-EF-05`, linha 3200, e linha 62 da tabela) e em `docs/revisao/2026-11-18-plano-correcao.md` (linhas 34, 112, 191, 214, "adiado, medir depois do bloco 1"). Nos dois, acrescente ao lado do id: "não implementar por decisão do dono (2026-11-22): ZIP sem OCR é raro". Não reescreva o histórico; só a anotação.
- [ ] `git diff --stat` mostra só esses três arquivos de `docs/revisao/`.
- [ ] **Commit** `docs: residuais CP9, P3, P6 resolvidos e decisões do dono de 2026-11-22`. Arquivos: os três de `docs/revisao/` citados.
