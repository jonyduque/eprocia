# Regras comuns: rodada de residuais de 2026-11-22

Valem para todo implementador e revisor de `docs/superpowers/plans/2026-11-22-residuais.md`. O brief de cada tarefa prevalece sobre este arquivo só no que ele disser explicitamente.

## Onde trabalhar

- Worktree `C:\Users\jonyd\Projetos\eprocia\.worktrees\residuais`, branch `residuais/2026-11-22` (da `main` `aa9e066`). Use caminhos absolutos: o cwd volta ao início entre chamadas.
- A worktree não tem `frontend/dist`. Antes do primeiro `go test` ou `go vet`, rode em `frontend`: `npm ci` e depois `npm run build`. Sem isso `main.go` não compila. Se o `npm ci` falhar com `ENOTEMPTY` ou `EPERM`, outro processo pode estar mexendo em `node_modules`: espere e tente de novo uma vez, e se falhar de novo responda `BLOCKED`.
- Só os arquivos listados em **Files** da tarefa. Precisou de outro, mexa e justifique no relatório.
- Pode haver um arquivo `internal/etl/zz_*_test.go` de outra frente (medidor temporário). Não o edite, não o apague, não o inclua em commit. Se ele quebrar a compilação do pacote, responda `BLOCKED` com a mensagem do compilador.

## TDD

Teste primeiro: escreva o teste, rode e veja falhar pelo motivo certo (não por erro de compilação de outra coisa), implemente o mínimo, rode e veja passar. Tarefa só de teste (sem mudança de produção): o teste novo ou alterado tem de passar, e o relatório diz qual asserção pegaria a regressão que ele protege (quebre a produção de propósito numa cópia local, rode, desfaça, e registre o resultado; não faça commit da quebra).

## Gate

Rode na raiz da worktree, com a saída em arquivo no scratchpad (leia só o trecho decisivo):

- Go: `go vet ./...` e `go test -race ./internal/...`. Na tarefa que mexe só num pacote, rode também `go test -race` nesse pacote sem `-short` (os testes de `pdf_classes_test.go` e de `internal/ocr` usam os binários reais do Poppler e do tesseract).
- Formato: `git show HEAD:<arquivo> | gofmt -d` não serve para arquivo novo; para conferir o que você escreveu, rode `gofmt -l <arquivo>` numa cópia com LF ou confie no hook `go-check`. O checkout usa `core.autocrlf=true`, então `gofmt -l` direto na cópia de trabalho lista tudo.
- A suíte tem `TestModernizacao_GoFixSemDiff`: código que o `go fix` reescreveria reprova (ex.: `strings.Split` num `range` vira `SplitSeq`).
- Frontend (tarefa que mexe em `frontend/`): `node --test` em todos os `frontend/*.test.mjs` (no Git Bash: `node --test frontend/*.test.mjs`), `npx svelte-check` e `npm run build` em `frontend`. Tarefa que mexe só em `scripts/visual/` (Tasks 6 e 7): `node --test scripts/visual/estados.test.mjs` e `node --test frontend/*.test.mjs`, sem `svelte-check` nem `npm run build` (o script não entra no bundle).
- Com outros agentes rodando, a suíte pode estourar tempo: rode de novo só os pacotes ou arquivos que falharam, com `-p 1` (Go) ou `--test-concurrency=1` (node). Falha que persiste com a máquina ociosa é falha.
- Binário de teste como executável falso (`internal/ocr`): todo teste que copia `os.Args[0]` define `EPROCIA_TESTE_FILHO=1` junto do modo (`EPROCIA_TESTE_DORMIR`, `EPROCIA_TESTE_SENHA`, `EPROCIA_TESTE_JORRAR`), e o modo existe no `TestMain` antes de o teste rodar. Sem isso o filho roda a suíte inteira e se copia de novo, em cascata.
- A partir da Task 5, cada `go test ./internal/etl` cria a sua pasta `%TEMP%\eprocia_testes_classes_*` e a apaga no fim; execuções em paralelo (outra worktree, outro agente) não se atrapalham. Não apague pastas `eprocia_testes_classes*` à mão durante a rodada.

## Commit

- Um commit por tarefa, com a mensagem indicada na tarefa, corpo curto em pt-BR, e terminando com:

```
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01YKHKdzvxGBDhYx5JAnxJoZ
```

- `git add` só com os caminhos explícitos da tarefa. Nunca `git add -A`, `git add .` nem `git commit -a`.
- Nunca inclua `zz_*_test.go`, `README.md`, `build/windows/installer/project.nsi`, `install.ps1`, `.genkit/`, `*.zip`, `*.pdf` (fora de `scripts/bench/fixtures/`) nem `.txt` de saída.
- Crie o arquivo da mensagem com a ferramenta Write no scratchpad e use `git commit -F <caminho>`: heredoc e `$(...)` são recusados em worktree isolada.

## Proibido

- `git push`, merge, rebase da `main`, tag ou release.
- Rodar `install.ps1` (nem com `-DryRun`).
- Matar `eprocia.exe` ou qualquer processo que você não iniciou; apagar ou mexer em `%TEMP%\eprocIA_bin` (é o do executável do dono). Processo que você subir (vite, navegador, servidor), encerre pelo PID antes de responder.
- Editar `frontend/src/wailsjs/**`, `frontend/dist/**`, `internal/ocr/embed/**` ou `build/bin/**`.
- Usar PDF, ZIP ou texto real. Todo dado de teste é sintético (`FULANO DE TAL`, `5000000-00.2026.8.26.0001`).
- Despachar subagentes.

## Relatório

- Relatório completo em arquivo `.txt` no caminho que o despacho indicar (nunca `.md`): o que mudou por arquivo, os testes novos com a falha vista antes e o resultado depois, a saída decisiva do gate, desvios do brief e por quê, e preocupações.
- Resposta final em no máximo 15 linhas: status (`DONE`, `DONE_WITH_CONCERNS`, `NEEDS_CONTEXT` ou `BLOCKED`), hash do commit, uma linha de testes, preocupações e o caminho do relatório.
- Faltou informação para decidir: pare e responda `NEEDS_CONTEXT` com a pergunta exata. Travou: `BLOCKED` com o motivo.
