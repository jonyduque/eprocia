# Regras comuns: atualização pelo aplicativo (autoupdate)

Valem para todo implementador e revisor de `docs/superpowers/plans/2026-11-24-autoupdate.md`. O brief de cada tarefa prevalece sobre este arquivo só no que disser explicitamente. A spec (`docs/superpowers/specs/2026-11-18-autoupdate-design.md`, decisões do dono na seção 10) é a autoridade sobre o comportamento; o plano, sobre nomes, arquivos e ordem.

## Base e onde trabalhar

- O plano roda **depois** do relatório de processamento (`feat/relatorio-processamento`, plano `2026-11-24-relatorio-processamento.md`), sobre a `main` que já o contém. O orquestrador cria a worktree `C:\Users\jonyd\Projetos\eprocia\.worktrees\autoupdate`, branch `feat/autoupdate`, a partir dessa `main`. Use caminhos absolutos: o cwd volta ao início entre chamadas.
- Antes da primeira tarefa, confira que a base tem `internal/registro/`, `registro_app.go` (com `versaoApp()` e `prepararInstancia`) e `frontend/src/lib/textos.js`. Faltando qualquer um, responda `BLOCKED`: o plano assume essa base.
- A worktree não tem `frontend/dist`. Antes do primeiro `go test`, `go vet` ou `wails generate module`, rode em `frontend`: `npm ci` e depois `npm run build`. Sem isso `main.go` não compila. Se o `npm ci` falhar com `ENOTEMPTY` ou `EPERM`, espere e tente de novo uma vez; falhando de novo, `BLOCKED`.
- Só os arquivos listados em **Files** da tarefa. Precisou de outro (um teste antigo que quebra pela mudança de assinatura, por exemplo), mexa e justifique no relatório.
- Nunca crie, edite, apague nem inclua em commit um `zz_*_test.go` (medidor temporário de outra frente). Se um deles quebrar a compilação, `BLOCKED` com a mensagem do compilador.

## Log

- O pacote `log` é proibido em código de produção (o teste `TestRegistro_SemLogPrintfEmProducao` reprova `log.Print*`/`log.Fatal*`). Use `slog`: `registro.Logger(ctx)` quando houver `ctx`, senão `slog.Default()`. Mensagens em pt-BR.
- O log da atualização registra hora e resultado de cada consulta e de cada troca; nunca URL, corpo de resposta nem caminho do perfil sem passar pelo saneamento do `internal/registro`.

## Bindings do Wails

- Mudou ou acrescentou método exportado de `App`? Rode `wails generate module` na raiz da worktree e inclua no commit só os arquivos de `frontend/src/wailsjs/go/main/` e `frontend/src/wailsjs/go/models.ts` que mudaram de fato. Nunca edite esses arquivos à mão (o hook `proteger-gerados` bloqueia).
- Falhou com `package "bytes" without types was imported`: rode uma vez `pwsh scripts/build/recompilar-wails.ps1` e tente de novo. Continuando, `BLOCKED`.
- `wails generate module` e `wails build` reescrevem arquivos só com diferença de fim de linha (`frontend/src/wailsjs/**`, `go.mod`, `frontend/package.json.md5`). Antes do commit, `git diff --ignore-cr-at-eol --stat`; reverta (`git checkout -- <arquivo>`) o que for só ruído.

## TDD

Teste primeiro: escreva o teste, rode, veja falhar pelo motivo certo (asserção, não compilação de outra coisa), implemente o mínimo, rode e veja passar. Quando o teste depende de símbolo novo, crie antes o esqueleto (assinatura com `panic("falta")`).

Guarda de regressão (o plano marca): o teste passa antes da mudança. Quebre a produção de propósito numa cópia local, sem commit, veja falhar, desfaça e registre no relatório qual quebra ele pegou.

## Rede, processos e a máquina do dono

- **Nenhum teste fala com a internet.** Toda requisição vai para `httptest.NewTLSServer` (cliente com o certificado do servidor e as `Regras` de host e prefixo injetadas). Um teste que precise da rede real está errado: `NEEDS_CONTEXT`.
- Nunca rode o `install.ps1` (nem com `-DryRun`). Conferir a sintaxe com o parser do PowerShell (`[System.Management.Automation.Language.Parser]::ParseFile`) não é rodar e é permitido.
- Nunca encerre o `eprocia.exe` do dono nem processo que você não iniciou; não mexa em `%TEMP%\eprocIA_bin`, em `%APPDATA%\eprocIA`, em `%LOCALAPPDATA%\eprocIA`, em `%LOCALAPPDATA%\Programs\eprocIA` nem no registro de verdade (`HKCU\...\Uninstall\eprocIA`, `HKLM`). Toda pasta é parâmetro e o teste passa `t.TempDir()`; todo acesso ao registro passa por uma interface trocada por fake.
- Processo que você subir (vite, navegador, o auxiliar dos testes, o exe do gate), encerre pelo PID antes de responder.
- Executável auxiliar dos testes de `internal/atualizacao`: é o programa pequeno `internal/atualizacao/testdata/aux`, compilado uma vez pelo `TestMain` num diretório temporário e copiado/carimbado por teste em `t.TempDir()`; o modo vem de `EPROCIA_TESTE_AUX`. Não use o binário de teste (`os.Args[0]`) como auxiliar nesse pacote. Em `internal/ocr`, que já usa o binário de teste, continua valendo: todo teste que copia `os.Args[0]` define `EPROCIA_TESTE_FILHO=1` junto do modo, tratado no `TestMain`.
- Chamadas Win32 que o `golang.org/x/sys/windows` v0.49.0 não tem (`ShellExecuteExW`, `BeginUpdateResourceW`/`UpdateResourceW`/`EndUpdateResourceW`) vão por `windows.NewLazySystemDLL(...).NewProc(...)`, com struct própria; não suba a versão do x/sys para ganhá-las.
- `ShellExecuteEx` real nunca roda em teste (fake injetado).

## Gate

Rode na raiz da worktree, com a saída em arquivo no scratchpad (leia só o trecho decisivo):

- Go: `go vet ./...` e `go test -race ./...`. Na tarefa que mexe num pacote, rode também `go test -race -count=1` nele sem `-short`.
- `TestModernizacao_GoFixSemDiff` faz parte da suíte: código que o `go fix` reescreveria reprova.
- Formato: com `core.autocrlf=true`, `gofmt -l` na cópia de trabalho lista tudo. Confira o que você escreveu numa cópia com LF ou confie no hook `go-check`.
- Frontend (tarefa que mexe em `frontend/`): `node --test frontend/*.test.mjs` (Git Bash), `npx svelte-check` e `npm run build` em `frontend`.
- Harness (tarefa que mexe em `.claude/` ou `scripts/visual/`): `node --test .claude/hooks/hooks.test.mjs .claude/statusline.test.mjs .claude/workflows/workflows.test.mjs scripts/visual/estados.test.mjs`.
- `pwsh scripts/build/gate.ps1` só na última tarefa. Se o `build\bin\eprocia.exe` da worktree estiver aberto por outro processo, rode com `-SemBuild`, registre que build e smoke ficaram sem verificar e responda `DONE_WITH_CONCERNS`.
- Com outros agentes rodando, a suíte pode estourar tempo: rode de novo só o que falhou, com `-p 1` (Go) ou `--test-concurrency=1` (node). Falha que persiste com a máquina ociosa é falha.

## Commit

- Um commit por tarefa, com a mensagem indicada na tarefa, corpo curto em pt-BR, terminando com:

```
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01YKHKdzvxGBDhYx5JAnxJoZ
```

- `git add` só com os caminhos explícitos da tarefa. Nunca `git add -A`, `git add .` nem `git commit -a`.
- Nunca inclua `zz_*_test.go`, `*.zip`, `*.pdf` (fora de `scripts/bench/fixtures/`), `.txt` de saída, `*.log`, `*.jsonl` nem executável (`*.exe`). Os exe dos testes são gerados em `t.TempDir()`, nunca em `testdata`.
- Crie a mensagem com a ferramenta Write no scratchpad e use `git commit -F <caminho>`: heredoc e `$(...)` são recusados em worktree isolada.

## Proibido

- `git push`, merge, rebase da `main`, tag, release, `gh release`.
- Acrescentar dependência ao `go.mod` ou ao `frontend/package.json` (`golang.org/x/sys/windows` já é direta).
- Usar PDF, ZIP, log ou texto real, ou definir `EPROCIA_BENCH_DIR`/`EPROCIA_LOG` fora de `t.Setenv`. Dado de teste é sintético (`FULANO DE TAL`, `5000000-00.2026.8.26.0001`, perfil `C:\Users\fulano`).
- Mexer em `build/windows/installer/project.nsi` sem a confirmação do dono registrada no brief da Task 18.
- Despachar subagentes.

## Relatório

- Relatório completo em arquivo `.txt` no caminho que o despacho indicar (nunca `.md`): o que mudou por arquivo, os testes novos com a falha vista antes e o resultado depois, a saída decisiva do gate, desvios do brief e por quê, preocupações.
- Resposta final em no máximo 15 linhas: status (`DONE`, `DONE_WITH_CONCERNS`, `NEEDS_CONTEXT` ou `BLOCKED`), hash do commit, uma linha de testes, preocupações e o caminho do relatório.
- Faltou informação para decidir: pare e responda `NEEDS_CONTEXT` com a pergunta exata. Travou: `BLOCKED` com o motivo.
