---
name: eprocia-icone
description: Cria o ícone personalizado do eprocIA — propõe conceitos ao usuário, desenha o SVG mestre e gera .ico (bandeja e executável) e appicon.png com ImageMagick. Use na tarefa de ícone da revisão de frontend.
---

# Ícone do eprocIA

## Fluxo

1. **Conceitos.** Desenhe 3 ou 4 propostas em SVG (viewBox `0 0 256 256`) em arquivos temporários fora do repositório, gere uma prévia PNG de cada uma em 256, 32 e 16 px e mostre ao usuário lado a lado (página HTML no scratchpad ou imagens). Ideias que dialogam com o produto: documento/pilha de páginas, marcador de evento numerado, colchetes de tag (`< >`) sobre página, lupa de OCR. Paleta do `DESIGN.md` (accent azul), fundo com cantos arredondados para legibilidade na bandeja clara e escura. **O usuário escolhe**; não siga sem a escolha.
2. **SVG mestre.** Salve o escolhido em `build/icon.svg`. Se os detalhes sumirem em 16/24 px, crie `build/icon-pequeno.svg` com a forma simplificada (sem traços finos, sem texto).
3. **Gerar:**
   ```bash
   pwsh scripts/icone/gerar.ps1
   ```
   Gera `build/windows/icon.ico` (16, 24, 32, 48, 64 e 256 px, com transparência) e `build/appicon.png` (1024 px).
4. **Conferir:** `magick identify build/windows/icon.ico` lista os 6 tamanhos; abra os PNGs de 16 e 32 px e veja se a forma continua reconhecível.
5. **Integrar:** `icon.go` já embute `build/windows/icon.ico` para a bandeja (o `getlantern/systray` no Windows exige ICO; PNG falha com `Unable to set icon`). O `wails build` usa `build/appicon.png` e `build/windows/icon.ico` no executável. Troque também o ícone da barra de título do frontend (`brand-icon` em `App.svelte`) pela mesma forma, como componente SVG inline.
6. **Gate:** `pwsh scripts/build/gate.ps1` (o smoke test sobe o exe; confira o ícone na bandeja e na barra de tarefas) e capturas visuais da barra de título.

## Validação desta skill

```bash
pwsh scripts/icone/gerar.ps1 -Svg <teste.svg> -Saida <scratchpad>/icone-teste
```

Deve listar 6 camadas no `.ico` e gerar `appicon.png` 1024×1024, sem tocar `build/`.
