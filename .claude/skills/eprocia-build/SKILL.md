---
name: eprocia-build
description: Toolchain e gate de build do eprocIA — Passo 0 (Wails CLI compatível com Go 1.27, go.mod em go 1.27, benchstat como tool) e o gate vet + test -race + wails build + smoke test. Use antes de qualquer correção e ao fim de cada tarefa do /corrigir.
---

# Build do eprocIA

## Passo 0 — toolchain (uma vez, antes da primeira correção)

1. **Wails CLI compatível com Go 1.27.** O CLI v2.12.0 publicado traz `x/tools v0.30.0` e falha com `package "bytes" without types was imported`. Confira com:
   ```bash
   go version -m "$(go env GOPATH)/bin/wails.exe" | grep x/tools
   ```
   Se vier `v0.30.0`, rode `pwsh scripts/build/recompilar-wails.ps1` (copia o módulo do cache, atualiza `x/tools`, instala). O hook `ambiente` mostra o estado no início de cada sessão.
2. **Validar:** `wails build -o eprocia-passo0.exe` sob Go 1.27 e apague o exe gerado.
3. **go.mod:**
   ```bash
   go mod edit -go=1.27
   go get -tool golang.org/x/perf/cmd/benchstat@latest
   go mod tidy
   ```
   `go get` pode subir dependências indiretas (`x/net`, `x/sys`, `x/text`); isso faz parte do passo. Rode o gate completo e faça um commit próprio: `chore(build): Go 1.27 e benchstat como tool`.

Se qualquer etapa falhar, pare: nenhuma correção começa sem o Passo 0.

Contorno, só enquanto o Passo 0 não foi feito: `GOTOOLCHAIN=go1.25.5 wails build`.

## Gate (toda tarefa de correção)

```bash
pwsh scripts/build/gate.ps1            # vet, test -race, wails build, smoke de 5 s
pwsh scripts/build/gate.ps1 -SemBuild  # só vet e testes
pwsh scripts/build/gate.ps1 -SemRace   # sem gcc/CGO
```

- `-race` exige CGO e `gcc` no PATH (há um MinGW em `C:\ProgramData\mingw64`); sem gcc o script avisa e roda sem `-race`.
- O `wails build` falha se `build/bin/eprocia.exe` estiver aberto. O script detecta e para com uma mensagem; feche o app pela bandeja (Sair). Nunca encerre um eprocia.exe que você não iniciou.
- O smoke test encerra apenas o PID que o próprio script iniciou.
- O `wails build` regenera `frontend/src/wailsjs/` e roda `go mod tidy`. Inclua o `go.mod` no commit se ele mudar.

## Validação desta skill

`pwsh scripts/build/gate.ps1 -SemBuild` termina com `[gate] build e smoke pulados (-SemBuild)` e todas as etapas `ok`.
