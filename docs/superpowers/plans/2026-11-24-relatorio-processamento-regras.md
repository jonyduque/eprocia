# Regras comuns: relatório de processamento, Forçar OCR, log e telemetria

Valem para todo implementador e revisor de `docs/superpowers/plans/2026-11-24-relatorio-processamento.md`. O brief de cada tarefa prevalece sobre este arquivo só no que ele disser explicitamente. A spec (`docs/superpowers/specs/2026-11-22-relatorio-processamento-design.md`) é a autoridade sobre o comportamento; o plano, sobre nomes, arquivos e ordem.

## Onde trabalhar

- Worktree `C:\Users\jonyd\Projetos\eprocia\.worktrees\relatorio`, branch `feat/relatorio-processamento`. Use caminhos absolutos: o cwd volta ao início entre chamadas.
- A worktree não tem `frontend/dist`. Antes do primeiro `go test`, `go vet` ou `wails generate module`, rode em `frontend`: `npm ci` e depois `npm run build`. Sem isso `main.go` não compila. Se o `npm ci` falhar com `ENOTEMPTY` ou `EPERM`, outro processo pode estar mexendo em `node_modules`: espere e tente de novo uma vez; se falhar de novo, responda `BLOCKED`.
- Só os arquivos listados em **Files** da tarefa. Precisou de outro (um teste antigo que quebra pela mudança de assinatura, por exemplo), mexa e justifique no relatório.
- Pode haver um arquivo `internal/etl/zz_*_test.go` de outra frente (medidor temporário). Não o edite, não o apague, não o inclua em commit. Se ele quebrar a compilação do pacote, responda `BLOCKED` com a mensagem do compilador.

## Bindings do Wails

- Mudou a assinatura de um método exportado de `App` (ou acrescentou um)? Rode `wails generate module` na raiz da worktree e inclua no commit só os arquivos de `frontend/src/wailsjs/go/main/` (`App.js`, `App.d.ts`) e `frontend/src/wailsjs/go/models.ts` que mudaram de fato. Nunca edite esses arquivos à mão (o hook `proteger-gerados` bloqueia).
- Se o `wails generate module` falhar com `package "bytes" without types was imported`, rode uma vez `pwsh scripts/build/recompilar-wails.ps1` e tente de novo. Se continuar falhando, responda `BLOCKED`.
- O `wails generate module` e o `wails build` do gate costumam reescrever arquivos só com diferença de fim de linha (CRLF): `frontend/src/wailsjs/**`, `go.mod`, `frontend/package.json.md5`. Antes do commit, `git diff --ignore-cr-at-eol --stat` mostra o que mudou de verdade; reverta (`git checkout -- <arquivo>`) o que for só ruído.

## TDD

Teste primeiro: escreva o teste, rode e veja falhar pelo motivo certo (não por erro de compilação de outra coisa), implemente o mínimo, rode e veja passar. Quando o teste novo depende de um símbolo que ainda não existe, crie primeiro o esqueleto (assinatura com corpo vazio ou `panic("falta")`) para a falha ser de asserção e não de compilação.

Exceção: teste que o plano marca como **guarda de regressão** fixa um comportamento que já existe e passa antes da mudança. Nesse caso, quebre a produção de propósito numa cópia local (sem commit), veja o teste falhar, desfaça e registre no relatório qual quebra ele pegou.

## Gate

Rode na raiz da worktree, com a saída em arquivo no scratchpad (leia só o trecho decisivo):

- Go: `go vet ./...` e `go test -race ./...`. Na tarefa que mexe num pacote, rode também `go test -race` nesse pacote sem `-short` (os testes de `pdf_classes_test.go` e de `internal/ocr` usam os binários reais do Poppler e do tesseract).
- A suíte tem `TestModernizacao_GoFixSemDiff`: código que o `go fix` reescreveria reprova.
- Formato: o checkout usa `core.autocrlf=true`, então `gofmt -l` na cópia de trabalho lista tudo. Para conferir o que você escreveu, rode `gofmt -l` numa cópia com LF ou confie no hook `go-check`.
- Frontend (tarefa que mexe em `frontend/`): `node --test frontend/*.test.mjs` (Git Bash), `npx svelte-check` e `npm run build` em `frontend`.
- Harness (tarefa que mexe em `.claude/` ou `scripts/visual/`): `node --test .claude/hooks/hooks.test.mjs .claude/statusline.test.mjs .claude/workflows/workflows.test.mjs scripts/visual/estados.test.mjs`.
- O `gate.ps1` recusa o build se o `build\bin\eprocia.exe` da própria worktree estiver aberto, e você não pode matá-lo se não foi você quem o abriu. Nesse caso rode `pwsh scripts/build/gate.ps1 -SemBuild`, registre no relatório que build e smoke ficaram sem verificar e responda `DONE_WITH_CONCERNS`.
- Com outros agentes rodando, a suíte pode estourar tempo: rode de novo só os pacotes ou arquivos que falharam, com `-p 1` (Go) ou `--test-concurrency=1` (node). Falha que persiste com a máquina ociosa é falha.
- Binário de teste como executável falso (`internal/ocr`): todo teste que copia `os.Args[0]` define `EPROCIA_TESTE_FILHO=1` junto do modo, e o modo existe no `TestMain` antes de o teste rodar. Sem isso o filho roda a suíte inteira e se copia de novo, em cascata.

## Variável EPROCIA_LOG e pastas de dados nos testes

- Nunca defina `EPROCIA_LOG` fora de um teste (`setx`, perfil do shell, variável do processo do agente). Nos testes, use `t.Setenv` ou passe o valor por parâmetro.
- Nenhum teste grava em `%AppData%\eprocIA` de verdade: a pasta de dados é sempre parâmetro, e o teste passa `t.TempDir()`.
- Teste que troca `slog.Default()` ou o pacote `log` guarda, antes de mexer, `slog.Default()`, `log.Writer()` e `log.Flags()`, e restaura os três no `t.Cleanup` (`slog.SetDefault`, depois `log.SetOutput` e `log.SetFlags`, nessa ordem). Motivo: `slog.SetDefault` redireciona o pacote `log` (`SetOutput` e `SetFlags(0)`), e voltar ao logger antigo com `SetDefault` não desfaz isso; sem restaurar, o teste seguinte escreve num manipulador encerrado. Esses testes não rodam com `t.Parallel()`.
- Teste que liga o registro chama `registro.ResetarParaTeste()` no `t.Cleanup` (depois de restaurar o `log`).

## Commit

- Um commit por tarefa, com a mensagem indicada na tarefa, corpo curto em pt-BR, e terminando com:

```
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01YKHKdzvxGBDhYx5JAnxJoZ
```

- `git add` só com os caminhos explícitos da tarefa. Nunca `git add -A`, `git add .` nem `git commit -a`.
- Nunca inclua `zz_*_test.go`, `.genkit/`, `*.zip`, `*.pdf` (fora de `scripts/bench/fixtures/`), `.txt` de saída, `*.log` nem `*.jsonl`. `README.md` só entra na Task 20.
- Crie o arquivo da mensagem com a ferramenta Write no scratchpad e use `git commit -F <caminho>`: heredoc e `$(...)` são recusados em worktree isolada.

## Proibido

- `git push`, merge, rebase da `main`, tag ou release.
- Rodar `install.ps1` (nem com `-DryRun`).
- Matar `eprocia.exe` ou qualquer processo que você não iniciou; apagar ou mexer em `%TEMP%\eprocIA_bin` (é o do executável do dono). Processo que você subir (vite, navegador, servidor, o exe do gate), encerre pelo PID antes de responder.
- Editar à mão `frontend/src/wailsjs/**`, `frontend/dist/**`, `internal/ocr/embed/**` ou `build/bin/**`.
- Usar PDF, ZIP, log ou texto real, ou definir `EPROCIA_BENCH_DIR`. Todo dado de teste é sintético (`FULANO DE TAL`, `5000000-00.2026.8.26.0001`, perfil `C:\Users\fulano`).
- Acrescentar dependência a `go.mod` ou `frontend/package.json`.
- Despachar subagentes.

## Relatório

- Relatório completo em arquivo `.txt` no caminho que o despacho indicar (nunca `.md`): o que mudou por arquivo, os testes novos com a falha vista antes e o resultado depois, a saída decisiva do gate, desvios do brief e por quê, e preocupações.
- Resposta final em no máximo 15 linhas: status (`DONE`, `DONE_WITH_CONCERNS`, `NEEDS_CONTEXT` ou `BLOCKED`), hash do commit, uma linha de testes, preocupações e o caminho do relatório.
- Faltou informação para decidir: pare e responda `NEEDS_CONTEXT` com a pergunta exata. Travou: `BLOCKED` com o motivo.
