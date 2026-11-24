---
name: eprocia-visual
description: Captura telas do frontend do eprocIA com dados fictícios (Vite + stubs dos bindings Wails + Playwright) numa matriz de tamanhos, temas e estados, e roda o gate visual (svelte-check, npm run build, pares antes/depois). Use na Fase 2 da revisão e em toda correção de frontend.
---

# Capturas e gate visual

## Por que não `wails dev`

Com o backend real, a fila mostraria nomes de processos reais e as capturas são versionadas. O script sobe **só o Vite** e injeta, antes do código da página, stubs de `window.go.main.App` e `window.runtime` (`scripts/visual/estados.mjs`) com dados fictícios: números `5000000-00.2026.4.00.NNNN` e caminhos `C:\Usuarios\exemplo\...`.

## Capturar

```bash
node scripts/visual/capturar.mjs --saida docs/revisao/capturas/AAAA-MM-DD/antes
node scripts/visual/capturar.mjs --saida <dir> --rapido               # 1024x768, tema escuro
node scripts/visual/capturar.mjs --saida <dir> --estados fila,erro    # só alguns estados
```

- Matriz completa: 800×600, 1024×768, 1440×900, 1920×1080 × escuro/claro × 9 estados (`vazio`, `fila`, `processando`, `erro`, `configuracoes`, `nao-lidos`, `relatorio`, `configuracoes-log`, `configuracoes-none`), mais as variações por tamanho e tema:
  - `processando`: `-atividade` (painel do item em processamento), `-divisor-minimo` (arrastado até x=20), `-divisor-maximo` (até a borda direita) e `-recolhido`;
  - `fila`: `-reprocessar` (formulário aberto, com a caixa Forçar OCR);
  - `nao-lidos`: `-atividade` (painel, sem o aviso), `-copiado` (faixa de aviso depois de copiar) e `-divisor-maximo`;
  - `relatorio` (três concluídos com `relatorio`, o terceiro com os três avisos e `atencao`): `-relatorio` (painel de detalhes aberto no item com aviso), `-relatorio-fim` (o mesmo painel rolado até o fim) e `-ler-ocr` (foco de teclado no "Ler com OCR"); só a 800 px, `-fila-492` e `-fila-493` (caixa de conteúdo do `.list-card` nos dois lados do `@container (max-width: 492px)` dos botões da fila, com a largura do painel gravada em `eprocia.sidebar`);
  - `configuracoes-log` (`LogAtivo: true`): `-log` (rolado até "Abrir pasta de logs") e `-log-280` (o mesmo com o painel em 280 px); o script mede o `.btn-log` e escreve no log.
  - Mais o alto contraste de `processando` e `configuracoes` a 1024×768.
  Total: 182 (24 com `--rapido`). A atividade fictícia fica em `ESTADOS.<estado>.atividades` (por id do item) e é servida pelo stub de `GetQueueActivity`; `ESTADOS.<estado>.LogAtivo` é servido pelo stub de `LogAtivo` (falso quando ausente).
- Nome: `<L>x<A>-<tema>-<estado>[-variação].png`.
- O Playwright `1.63.0` é instalado uma vez em `%LOCALAPPDATA%\eprocia-harness\` (fora do `package.json`) e usa o Chrome instalado (`channel: 'chrome'`), com o Chromium do Playwright como reserva.
- As capturas usam `reducedMotion: 'reduce'` para não pegar animações no meio.
- O script encerra o Vite (porta 5199) ao terminar, inclusive em erro.

Novo estado de interface? Acrescente em `ESTADOS` (`estados.mjs`) e rode `node --test scripts/visual/estados.test.mjs`, que confere o formato de `etl.QueueItem`, as chaves de `models.RelatorioZip` (com as cinco categorias de página somando `paginas.total`) e que só há dados fictícios.

## Limitação

O Playwright renderiza no Chromium, não no WebView2 da janela frameless. Bordas de resize da janela nativa, arrasto da barra de título (`--wails-draggable`) e `MinWidth`/`MinHeight` de `main.go` não aparecem nas capturas. Verifique-os pelo código e descreva, no relatório, um teste manual guiado: abrir o app, redimensionar pelas 4 bordas e cantos, maximizar/restaurar, arrastar o divisor até os limites, recolher e expandir o painel, e conferir que nada transborda ou fica sem alcance.

## Gate visual (cada correção de frontend)

1. `node scripts/visual/capturar.mjs --saida docs/revisao/capturas/AAAA-MM-DD/depois` (ou `--estados` com os afetados).
2. `cd frontend && npx svelte-check` sem erros (disponível depois da migração para Svelte 5).
3. `cd frontend && npm run build` sem warnings novos em relação à linha de base.
4. No relatório, ponha o par antes/depois de cada captura afetada pelo achado. Quem aprova é o usuário, olhando as imagens.

## Validação desta skill

`node --test scripts/visual/estados.test.mjs` passa, e `node scripts/visual/capturar.mjs --saida <scratchpad> --rapido` termina com `[visual] 24 capturas em ...`.
