# Residuais da atualização pelo programa (v0.5.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fechar os quatro residuais anotados na revisão final da atualização pelo programa: a recusa de entrada com prefixo genérico, a segunda instância aberta por `--apos-atualizar` digitado à mão, erros crus de `SaveConfig`/`watchdog:error` na interface e a frase "Não há telemetria." do README.

**Architecture:** Correções mínimas, uma por residual. Frontend: um helper puro em `src/lib/atualizacao.js` decide texto e tipo do aviso de erro de entrada na fila. Backend: `internal/atualizacao` ganha a leitura do processo pai e uma função pura que decide se o `--apos-atualizar` veio mesmo da troca; `main.go` passa a usar um helper testável em `app_atualizacao.go`. `app.go` troca os erros com `%w`/`%q` por constantes em pt-BR, com o erro completo só no `slog`.

**Tech Stack:** Go 1.27 (Wails v2, `golang.org/x/sys/windows`), Svelte 5 (sintaxe legada), `node --test`.

**Spec:** `docs/superpowers/specs/2026-11-18-autoupdate-design.md` (contrato de troca, seção 3.1; segunda instância, seção 4) e o bloco "Update contracts" do `CLAUDE.md`.

**Regras do implementador:** `docs/superpowers/plans/2026-11-26-residuais-autoupdate-regras.md` (worktree `C:\Users\jonyd\Projetos\eprocia\.worktrees\residuais-autoupdate`, branch `fix/residuais-autoupdate`).

## Global Constraints

- Contrato de troca intocado: flags `--apos-atualizar=<pid>` e `--limpar-anterior`, arquivo `atualizacao-sinal-<pid>`, nomes `eprocia.exe`, `eprocia.anterior.exe`, `eprocia.falhou.exe`, `eprocia.exe.download-*`. A v0.5.0 instalada no campo inicia a versão nova com esses argumentos e precisa continuar funcionando.
- Ruling anterior do dono: o processo novo **não sai** por prazo; na dúvida sobre a origem do `--apos-atualizar`, prefere-se abrir (comportamento atual) a deixar o usuário sem programa.
- Texto que vai à interface é fixo, em pt-BR, sem caminho, nome de usuário nem `err.Error()` cru; o erro completo vai só ao `slog` (o registro já troca perfil e temp por `%USERPROFILE%`/`%TEMP%`).
- O prefixo `configurações salvas, mas ` é mantido nas falhas parciais de `SaveConfig`: `App.svelte:323` e `Settings.svelte:134` decidem a forma do aviso por `/^configurações salvas/i`.
- `ATUALIZACAO.soltarDepois` (`frontend/src/lib/textos.js:130`) e `atualizacao.MsgSoltarDepois` (`internal/atualizacao/mensagens.go:25`) são a mesma frase (`TestMsgSoltarDepois_IgualAoFrontend`); nenhuma das duas muda.
- Nenhuma dependência nova; nenhum método exportado de `App` muda de assinatura (sem `wails generate module`).

## Review Focus

1. Reinício legítimo depois da correção do residual 2: o processo antigo (v0.5.0, rodando como `eprocia.anterior.exe` depois do rename) inicia o novo; o novo precisa reconhecer o pai. Esperado: o pai é o antigo e o nome da imagem no instantâneo de processos é `eprocia.exe` (nome da criação) ou `eprocia.anterior.exe`; os dois são aceitos (Task 2, `TestConfirmarAposAtualizar`). Custo de um falso negativo (flag legítima recusada): o novo nasce em partida normal, acha a trava ocupada pelo antigo, mostra o aviso de instância já aberta e sai sem sinal de vida; o antigo espera os 90 s, reverte e mostra "A versão X não abriu". Nada se perde, mas a atualização falha. O ensaio manual do `eprocia-release` (`-tags ensaio`) confere o caminho real antes da próxima release: o reinício abre a versão nova sem o aviso de instância aberta e sem reversão, e o log do processo novo (`EPROCIA_LOG=1`) não tem a linha `--apos-atualizar ignorado`; registrar isso no relatório final.
2. Falha ao ler o instantâneo de processos (API recusada, antivírus): esperado seguir como hoje, confiando na flag, porque o ruling prefere abrir (Task 2, caso "erro do pai" em `TestConfirmarAposAtualizar` e `TestLerArranque`).
3. `--apos-atualizar=<pid do próprio shell>` (`$PID` no PowerShell): o pai bate com o pid, mas a imagem é `pwsh.exe`; esperado partida normal, com a trava de instância única avisando se já houver um eprocIA aberto (Task 2, caso "pai não é eprocIA").
4. A recusa chegando como string (o Wails rejeita a promise com o texto do erro Go) ou como `Error`: as duas formas devem dar o aviso da recusa sem prefixo e com tipo `aviso` (Task 1, `avisoErroEntrada`).
5. Caminho com nome de usuário na pasta monitorada inexistente: a mensagem devolvida por `SaveConfig` e a do evento `watchdog:error` não podem conter a pasta (Task 3, igualdade exata com a constante, usando `t.TempDir()`, que contém o perfil do usuário da máquina).

## Divergências da spec com o código atual

- A spec (seção 3.1) foi escrita antes da trava de instância única; a correção de 2026-11-24 (`travaAposAtualizar`, `app_atualizacao.go:41-72`; `main.go:34-42`) adiou a trava com `--apos-atualizar`, e é ela que abre a segunda instância quando a flag é digitada à mão. A spec só diz que a flag "recebe só um PID" (seção 5). O plano acrescenta a conferência da origem sem mudar a flag nem o protocolo: a versão antiga já inicia a nova como filha (`troca.go:232-235`, `exec.Command` + `cmd.Start`), então "o pai é o pid informado" vale para toda versão em campo.
- Risco aceito: um antigo rodando de outro nome dentro da pasta de instalação (por exemplo `eprocia (1).exe`, que `Detectar` aceita como modo `trocar` se estiver na pasta certa) passa na troca hoje e, com a conferência do nome da imagem, passa a ter o novo recusado e a troca revertida ("A versão X não abriu"). O código não muda por isso: a instalação por usuário sempre nomeia o exe `eprocia.exe`, e o log do processo novo registra o motivo.
- Nenhum ouvinte do frontend escuta `watchdog:error` hoje (só `app_atualizacao.go:101` o emite). O plano troca o texto do evento mesmo assim (residual 3), sem acrescentar ouvinte.
- O residual 3 cita `SaveConfig`; `SetTheme` (`app.go:376-377`) tem o mesmo `%w` e entra na mesma tarefa (o texto só chega ao `console.error` de `App.svelte:360`, mas a regra é a mesma). Os `%q` de `lerSaidaParaCopiar` (`app.go:422-434`) ficam fora: não são residuais desta feature.
- README e spec: a spec diz "Nenhuma telemetria, nem contagem de atualizações" (seção 6, linha 201), referindo-se à consulta; o README (linha 247) generalizou para "Não há telemetria.", o que conflita com `EPROCIA_LOG`. Só o README muda.

Os quatro residuais se confirmaram no código atual:

1. `App.svelte:228-232` mostra `'Erro ao adicionar à fila: ' + msg` também para `MsgSoltarDepois` (devolvido por `recusaEntrada`, `app_atualizacao.go:339-344`, em `AddDroppedFiles`, `AddFilesToQueue` e `ReprocessQueueItem`). O mesmo acontece em `App.svelte:421-424` (`triggerManualSelect`, prefixo "Erro ao selecionar arquivos: ") e em `ProcessList.svelte:235-237` e `:256-258` ("Erro ao reprocessar: ").
2. `main.go:32-42`: qualquer `--apos-atualizar=<inteiro positivo>` adia a trava; com outro eprocIA aberto, `travaAposAtualizar` devolve ok, a janela abre, e `arranqueApos` só desiste do watcher depois de até 60 s + 60 s.
3. `app_atualizacao.go:101` (`err.Error()` no `watchdog:error`), `app.go:316` (`%w` do autostart), `app.go:326` (erro cru de `config.Save`, que traz o caminho do `config.json`), `app.go:353` (`%q` da pasta e `%w`), `app.go:377` (`%w` do tema).
4. `README.md:247`: "Não há telemetria."

---

### Task 1: Recusa de entrada sem o prefixo genérico

**Files:**
- Modify: `frontend/src/lib/atualizacao.js` (acrescentar a função depois de `aceitaEntrada`, linhas 107-109; o arquivo já importa `ATUALIZACAO as T` na linha 4)
- Modify: `frontend/src/App.svelte:58` (import), `:228-232` (catch do `AddDroppedFiles`), `:421-424` (catch do `triggerManualSelect`)
- Modify: `frontend/src/components/ProcessList.svelte` (import junto dos outros de `../lib/`; catches em `:235-237` e `:256-258`)
- Test: `frontend/atualizacao.test.mjs`, `frontend/avisos.test.mjs:173` e `:176-195` (testes antigos que exigem o texto concatenado)

**Interfaces:**
- Produces: `export function avisoErroEntrada(err, prefixo) -> { texto: string, tipo: 'aviso' | 'error' }`. `msg = String(err?.message ?? err)`; se `msg === ATUALIZACAO.soltarDepois`, devolve `{ texto: ATUALIZACAO.soltarDepois, tipo: 'aviso' }`; senão `{ texto: prefixo + msg, tipo: 'error' }`.

- [ ] **Step 1: Write the failing tests** em `frontend/atualizacao.test.mjs` (acrescentar `avisoErroEntrada` ao import da linha 7):

```js
describe('avisoErroEntrada', () => {
  it('recusa da atualização sai sem prefixo e como aviso', () => {
    assert.deepEqual(avisoErroEntrada(T.soltarDepois, 'Erro ao adicionar à fila: '), { texto: T.soltarDepois, tipo: 'aviso' });
    assert.deepEqual(avisoErroEntrada(new Error(T.soltarDepois), 'Erro ao reprocessar: '), { texto: T.soltarDepois, tipo: 'aviso' });
  });
  it('outro erro mantém o prefixo e o tipo error', () => {
    assert.deepEqual(avisoErroEntrada('fila não inicializada', 'Erro ao reprocessar: '), { texto: 'Erro ao reprocessar: fila não inicializada', tipo: 'error' });
    assert.deepEqual(avisoErroEntrada(new Error('x'), 'Erro ao selecionar arquivos: '), { texto: 'Erro ao selecionar arquivos: x', tipo: 'error' });
  });
});
```

e, no `describe` que lê as fontes (linhas 267-270 definem `app` e `lista`; `corpoFuncao` está na linha 231), mais dois casos:

```js
it('App: erros de pôr na fila passam por avisoErroEntrada', () => {
  const ini = app.indexOf('OnFileDrop(');
  const cb = app.slice(ini, app.indexOf('}, false)', ini));
  assert.match(cb, /avisoErroEntrada\(err, 'Erro ao adicionar à fila: '\)/);
  assert.doesNotMatch(cb, /'Erro ao adicionar à fila: ' \+/);
  const f = corpoFuncao(app, 'async function triggerManualSelect');
  assert.match(f, /avisoErroEntrada\(err, 'Erro ao selecionar arquivos: '\)/);
  assert.doesNotMatch(f, /'Erro ao selecionar arquivos: ' \+/);
});
it('ProcessList: erros de reprocessar passam por avisoErroEntrada', () => {
  for (const nome of ['async function lerComOCR', 'async function confirmarReprocessar']) {
    const f = corpoFuncao(lista, nome);
    assert.match(f, /avisoErroEntrada\(err, 'Erro ao reprocessar: '\)/, nome);
    assert.doesNotMatch(f, /'Erro ao reprocessar: ' \+/, nome);
  }
});
```

(Se `corpoFuncao` não achar a função pela assinatura dada, adapte a chamada, não a asserção.)

Em `frontend/avisos.test.mjs`, ajuste os dois testes antigos que fixam o texto concatenado: na linha 173 (`App não usa alert()...`), troque a asserção por `assert.match(sel.slice(sel.indexOf('catch')), /avisoErroEntrada\(err, 'Erro ao selecionar arquivos: '\)/)`; no mapa `casos` de `ações da fila que falham avisam pelo avisoAcao` (linhas 178-187), tire a entrada `confirmarReprocessar` (o teste novo de `atualizacao.test.mjs` passa a cobri-la; `lerComOCR` não está no mapa).

- [ ] **Step 2: Run to verify it fails**

Run: `node --test frontend/atualizacao.test.mjs` (Git Bash, raiz da worktree)
Expected: FAIL; `avisoErroEntrada` não é exportada (crie antes o esqueleto `export function avisoErroEntrada(err, prefixo) { return null; }` para a falha ser de asserção) e as fontes ainda concatenam o prefixo.

- [ ] **Step 3: Implement** `avisoErroEntrada` em `frontend/src/lib/atualizacao.js` e use-a nos quatro catches: `const r = avisoErroEntrada(err, '<prefixo>'); aviso.mostrar(r.texto, r.tipo);` (no `triggerManualSelect`, `avisarPainel(r.texto, r.tipo)`; no `ProcessList`, `avisoAcao.mostrar(r.texto, r.tipo)`). O `console.error` de cada catch fica como está.

- [ ] **Step 4: Run to verify it passes**

Run: `node --test frontend/*.test.mjs`, depois `npx svelte-check` e `npm run build` em `frontend`.
Expected: todos os testes passam; svelte-check com 0 erros; build termina.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/atualizacao.js frontend/src/App.svelte frontend/src/components/ProcessList.svelte frontend/atualizacao.test.mjs frontend/avisos.test.mjs
git commit -F <mensagem>   # fix(frontend): recusa durante a atualização sem o prefixo de erro
```

### Task 2: `--apos-atualizar` só vale quando vem da troca

**Files:**
- Modify: `internal/atualizacao/arranque.go` (nova função depois de `LimparAnteriorPedido`, linha 35)
- Modify: `internal/atualizacao/processo_windows.go` (nova função depois de `inexistente`, linha 75)
- Modify: `internal/atualizacao/processo_outros.go` (mesma função, versão fora do Windows)
- Modify: `app_atualizacao.go` (novo helper perto de `travaAposAtualizar`, linhas 34-72)
- Modify: `app.go:69` (campo `arranque` em `App`)
- Modify: `main.go:32-33` e `:45-48` (callback `aoTravar`)
- Test: `internal/atualizacao/arranque_test.go`, `internal/atualizacao/processo_windows_test.go` (criar, `//go:build windows`), `app_atualizacao_test.go`

**Interfaces:**
- Produces (pacote `atualizacao`):
  - `func PaiDoProcesso() (ppid int, exe string, err error)`. Windows: `windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)` (fechar o handle com `defer windows.CloseHandle`), uma `windows.ProcessEntry32` com `e.Size = uint32(unsafe.Sizeof(e))` **antes** de `Process32First` (sem isso a chamada falha com `ERROR_BAD_LENGTH`), e uma passada só por `Process32First`/`Process32Next`, guardando num mapa `pid -> (ParentProcessID, windows.UTF16ToString(ExeFile[:]))`; `ERROR_NO_MORE_FILES` vindo de `Process32Next` (`errors.Is(err, windows.ERROR_NO_MORE_FILES)`) é o fim da lista, não erro. A decisão vem só depois da passada, porque o pai pode aparecer antes ou depois da entrada do processo no instantâneo: acha `os.Getpid()` no mapa e devolve o ppid dele e o nome da imagem do pai (só o nome, sem pasta; o `ExeFile` já vem assim). Pai ausente do mapa: `exe == ""`, `err == nil`. Próprio processo ausente ou falha da API (outro erro que não o fim da lista): `err != nil`. Fora do Windows: `os.Getppid(), "", nil`.
  - Motivos de recusa, texto fixo (constantes em `arranque.go`): `MotivoPidNaoEPai = "o pid informado não é o processo pai"` e `MotivoPaiNaoEprocia = "o processo pai não é o eprocIA"`.
  - `func ConfirmarAposAtualizar(pid, ppid int, exe string, errPai error) (ok bool, motivo string)`: `pid <= 0` dá `(false, MotivoPidNaoEPai)`; `errPai != nil` dá `(true, "")` (ruling: na dúvida, abrir como hoje); `ppid != pid` dá `(false, MotivoPidNaoEPai)`; `exe == ""` dá `(true, "")`; `exe` igual, sem diferenciar caixa (`strings.EqualFold`), a `NomeExe` ou `NomeAnterior` dá `(true, "")`; senão `(false, MotivoPaiNaoEprocia)`.
- Produces (pacote `main`, em `app_atualizacao.go`):

```go
// arranque é o que a main leu da linha de comando sobre a troca.
type arranque struct {
	apos           int    // pid do antigo aceito; 0 = partida normal
	limparAnterior bool
	pidFlag        int    // pid pedido na flag, mesmo recusado
	recusa         string // motivo fixo da recusa; "" = aceito ou sem flag
	paiPID         int
	paiExe         string // só o nome da imagem do pai
}
```

  - `func lerArranque(args []string, pai func() (int, string, error)) arranque`. Sem `AposAtualizar(args)` válido, devolve `arranque{}` sem chamar `pai`. Com ele, chama `pai` uma vez e `ConfirmarAposAtualizar`; aceito: `apos = pid`, `limparAnterior = LimparAnteriorPedido(args)`; recusado: `apos = 0`, `limparAnterior = false` (partida normal: trava de instância única imediata, sem sinal de vida, sem `--limpar-anterior`; a limpeza da abertura normal segue como já é). Em ambos, `pidFlag`, `paiPID`, `paiExe` e `recusa` preenchidos.
  - Campo novo `arranque arranque` em `App` (`app.go`, junto de `apos`, linha 69) e `func (a *App) logarRecusaApos()`: com `a.arranque.recusa != ""`, faz `slog.Warn("atualização: --apos-atualizar ignorado; partida normal", "motivo", recusa, "pid_flag", pidFlag, "ppid", paiPID, "pai_exe", paiExe)`; senão nada.
  - `main.go:32-33` vira `app.arranque = lerArranque(os.Args, atualizacao.PaiDoProcesso)`, `app.apos = app.arranque.apos`, `app.limparAnterior = app.arranque.limparAnterior`. `lerArranque` roda antes do registro (o `if app.apos > 0` da trava adiada depende dela), por isso o log fica para depois: no callback `aoTravar` de `prepararInstancia` (`main.go:45-48`), chame `app.logarRecusaApos()` logo depois de `iniciarRegistro`. Com outro eprocIA aberto, `aoTravar` não roda e a cópia sai sem log (a segunda cópia nunca mexe no log da primeira), o que é o comportamento esperado.
- Consumes: `atualizacao.AposAtualizar`, `atualizacao.LimparAnteriorPedido` (`arranque.go:12`, `:33`), `atualizacao.NomeExe`, `atualizacao.NomeAnterior` (`sinal.go:14-15`).

- [ ] **Step 1: Write the failing tests**

`internal/atualizacao/arranque_test.go`, `TestConfirmarAposAtualizar`, tabela com pid `4242`:

| caso | ppid | exe | errPai | ok | motivo |
|---|---|---|---|---|---|
| pai é o eprocia.exe | 4242 | `eprocia.exe` | nil | true | `""` |
| caixa diferente | 4242 | `EPROCIA.EXE` | nil | true | `""` |
| pai já renomeado | 4242 | `eprocia.anterior.exe` | nil | true | `""` |
| pai fora do instantâneo | 4242 | `""` | nil | true | `""` |
| erro do pai | 0 | `""` | `errors.New("snapshot")` | true | `""` |
| pai não é eprocIA | 4242 | `pwsh.exe` | nil | false | `"o processo pai não é o eprocIA"` |
| pid não é o pai | 777 | `eprocia.exe` | nil | false | `"o pid informado não é o processo pai"` |

mais `ConfirmarAposAtualizar(0, 0, "eprocia.exe", nil)` e `ConfirmarAposAtualizar(-1, -1, "eprocia.exe", nil)` dando `(false, MotivoPidNaoEPai)`.

`internal/atualizacao/processo_windows_test.go`, `TestPaiDoProcesso`: `ppid, exe, err := PaiDoProcesso()`; `err == nil`; `ppid == os.Getppid()`; `exe != ""`, sem `\` e terminando em `.exe` (`strings.HasSuffix(strings.ToLower(exe), ".exe")`).

`app_atualizacao_test.go`, `TestLerArranque`, com um `pai` falso que conta chamadas (compare o `arranque` inteiro com `==`):

- `["eprocia.exe", "--apos-atualizar=4242", "--limpar-anterior"]`, pai `(4242, "eprocia.anterior.exe", nil)`: `arranque{apos: 4242, limparAnterior: true, pidFlag: 4242, paiPID: 4242, paiExe: "eprocia.anterior.exe"}`.
- mesmos args, pai `(9999, "pwsh.exe", nil)`: `arranque{pidFlag: 4242, recusa: "o pid informado não é o processo pai", paiPID: 9999, paiExe: "pwsh.exe"}`.
- mesmos args, pai `(4242, "pwsh.exe", nil)`: `recusa == "o processo pai não é o eprocIA"`, `apos == 0`.
- mesmos args, pai `(0, "", errors.New("snapshot"))`: `apos == 4242`, `limparAnterior == true`, `recusa == ""`.
- `["eprocia.exe", "--apos-atualizar=4242"]`, pai `(4242, "eprocia.exe", nil)`: `apos == 4242`, `limparAnterior == false`.
- `["eprocia.exe", "--limpar-anterior"]` e `["eprocia.exe"]`: `arranque{}` e o pai chamado 0 vezes.

`app_atualizacao_test.go`, `TestLogarRecusaApos` (sem `t.Parallel()`; guarda e restaura `slog.Default()`, `log.Writer()` e `log.Flags()` como manda o arquivo de regras): `slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))`; com `app.arranque = arranque{pidFlag: 4242, recusa: atualizacao.MotivoPaiNaoEprocia, paiPID: 4242, paiExe: "pwsh.exe"}`, `logarRecusaApos()` escreve uma linha `level=WARN` que contém `--apos-atualizar ignorado`, `motivo="o processo pai não é o eprocIA"`, `pid_flag=4242`, `ppid=4242` e `pai_exe=pwsh.exe`; com `app.arranque = arranque{}` e com um arranque aceito (`apos: 4242`, `recusa: ""`), o buffer fica vazio.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/atualizacao -run "TestConfirmarAposAtualizar|TestPaiDoProcesso" -v` e `go test . -run "TestLerArranque|TestLogarRecusaApos" -v`
Expected: FAIL por asserção (crie antes os esqueletos com `panic("falta")` ou retorno zero para não falhar na compilação).

- [ ] **Step 3: Implement** `PaiDoProcesso` (dois arquivos), os motivos, `ConfirmarAposAtualizar`, `arranque`, `lerArranque` e `logarRecusaApos`; em `main.go`, as linhas 32-33 e o `aoTravar` como descrito em Interfaces. O resto de `main.go` (o `if app.apos > 0` da trava adiada) não muda. Comentário curto em `lerArranque`: o processo novo da troca é filho do antigo (`troca.go`, `exec.Command`), e o nome da imagem no instantâneo é o da criação, por isso `eprocia.anterior.exe` também vale.

- [ ] **Step 4: Run to verify they pass**

Run: `go test -race ./internal/atualizacao` (sem `-short`), `go test -race . -run "TestLerArranque|TestLogarRecusaApos|TestTravaAposAtualizar|TestAtualizacao_"`, depois `go vet ./...` e `go test -race ./...`.
Expected: PASS; `TestTravaAposAtualizar` e os `TestAtualizacao_*` seguem passando sem mudança.

- [ ] **Step 5: Commit**

```bash
git add internal/atualizacao/arranque.go internal/atualizacao/arranque_test.go internal/atualizacao/processo_windows.go internal/atualizacao/processo_windows_test.go internal/atualizacao/processo_outros.go app.go app_atualizacao.go app_atualizacao_test.go main.go
git commit -F <mensagem>   # fix(atualizacao): --apos-atualizar só adia a trava quando o pai é o eprocIA antigo
```

### Task 3: Textos fixos nos erros de `SaveConfig`, `SetTheme` e `watchdog:error`

**Files:**
- Modify: `app.go:298-359` (`SaveConfig`, `aplicarMonitor`), `app.go:363-384` (`SetTheme`); constantes novas no topo de `app.go` ou junto de `SaveConfig`
- Modify: `app_atualizacao.go:99-102` (`ligarMonitor`)
- Test: `app_test.go`

**Interfaces:**
- Produces (pacote `main`, valores exatos):

```go
const (
	msgConfigNaoGravada = "o arquivo de configuração não pôde ser gravado"
	msgMonitorFalhou    = "não foi possível monitorar a pasta escolhida"
	msgAutostartFalhou  = "configurações salvas, mas não foi possível alterar a inicialização com o Windows"
	msgTemaNaoGravado   = "não foi possível salvar o tema"
)
```

  - `SaveConfig`: falha do autostart devolve `errors.New(msgAutostartFalhou)` (o `slog.Warn` da linha 314 já leva o erro); falha de `config.Save` faz `slog.Warn("falha ao gravar as configurações", "erro", err)` e devolve `errors.New(msgConfigNaoGravada)` (o desfazer do autostart, linhas 321-325, fica igual).
  - `aplicarMonitor`: devolve `errors.New("configurações salvas, mas " + msgMonitorFalhou)` (o `slog.Warn` da linha 352, com pasta e erro, fica).
  - `SetTheme`: `slog.Warn("falha ao gravar o tema", "erro", err)` e `errors.New(msgTemaNaoGravado)`. O `fmt.Errorf("tema inválido: %q", tema)` da linha 365 fica (o valor vem do próprio frontend, não tem caminho).
  - `ligarMonitor`: `a.emitirEm(ctx, "watchdog:error", msgMonitorFalhou)`.

- [ ] **Step 1: Write the failing tests** em `app_test.go` (cada um com `t.Setenv("APPDATA", t.TempDir())`, `app := NewApp()`, `defer app.watcher.Stop()`):

  - `TestSaveConfig_ErroDoMonitorSemCaminho`: pasta `filepath.Join(t.TempDir(), "nao-existe")`, `WatchdogEnabled: true`; `err.Error() == "configurações salvas, mas não foi possível monitorar a pasta escolhida"`.
  - `TestSaveConfig_ErroDoAutostartSemDetalhe`: `app.autostart` devolve `errors.New(`acesso negado em C:\Users\fulano\x`)`, `AutoStart: true`; `err.Error() == msgAutostartFalhou`.
  - `TestSaveConfig_ErroDeGravacaoSemCaminho`: `dados := t.TempDir()`, `t.Setenv("APPDATA", dados)`, cria o **arquivo** `filepath.Join(dados, "eprocIA")` (o `MkdirAll` do `config.SaveToFile` falha); `err.Error() == "o arquivo de configuração não pôde ser gravado"`.
  - `TestSetTheme_ErroDeGravacaoSemCaminho`: mesma montagem; `app.SetTheme("dark")` devolve erro com `err.Error() == "não foi possível salvar o tema"`.
  - `TestLigarMonitor_ErroComTextoFixo`: `app.emitir` guarda evento e `dados`; `app.guardarConfig(&models.Config{MonitoredPath: filepath.Join(t.TempDir(), "nao-existe"), WatchdogEnabled: true, OcrMode: "normal", CleaningLevel: "basic"})`; `app.ligarMonitor(nil)`; um único evento `watchdog:error` com `dados == []any{"não foi possível monitorar a pasta escolhida"}`.

  Os testes antigos (`TestSaveConfig_DevolveErroQuandoPastaNaoPodeSerMonitorada`, `TestSaveConfig_DevolveErroQuandoAutostartFalha`, `TestSaveConfig_AutostartFalhoNaoFicaGravado`) só conferem `err != nil` e seguem valendo.

- [ ] **Step 2: Run to verify they fail**

Run: `go test . -run "TestSaveConfig_Erro|TestSetTheme_Erro|TestLigarMonitor_Erro" -v`
Expected: FAIL por asserção (mensagens com `%q`/`%w`; para compilar, declare as constantes antes de mudar o código).

- [ ] **Step 3: Implement** as trocas acima. Nenhuma mudança no frontend: `/^configurações salvas/i` continua casando com as duas falhas parciais.

- [ ] **Step 4: Run to verify they pass**

Run: `go test -race . -v -run "TestSaveConfig|TestSetTheme|TestLigarMonitor|TestAtualizacao_"`, depois `go vet ./...` e `go test -race ./...`; `node --test frontend/*.test.mjs` (nenhum teste do frontend deve depender do texto antigo).
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app.go app_atualizacao.go app_test.go
git commit -F <mensagem>   # fix(app): erros de configuração e do monitoramento com texto fixo na interface
```

### Task 4: README e CLAUDE.md

**Files:**
- Modify: `README.md:247` (última frase do parágrafo de "Atualização pelo programa")
- Modify: `CLAUDE.md` (bloco "Update contracts" em "Conventions worth knowing"; uma linha sobre textos fixos)
- Test: `frontend/atualizacao.test.mjs` (CI já roda o glob `frontend/*.test.mjs`)

**Interfaces:**
- Consumes: `lerArranque`, `ConfirmarAposAtualizar` (Task 2); constantes de mensagem de `app.go` (Task 3).

- [ ] **Step 1: Write the failing test** em `frontend/atualizacao.test.mjs`, `README: atualização não nega a telemetria local`: lê `README.md` da raiz do repositório (`join(dirname(fileURLToPath(import.meta.url)), '..', 'README.md')`), recorta do título `## 🔄 Atualização pelo programa` até o próximo `\n## `, e confere: `doesNotMatch(secao, /Não há telemetria/)`, `match(secao, /EPROCIA_LOG/)`, `match(secao, /fica só no computador/)`.

- [ ] **Step 2: Run to verify it fails**

Run: `node --test frontend/atualizacao.test.mjs`
Expected: FAIL em `doesNotMatch` (o README ainda diz "Não há telemetria.").

- [ ] **Step 3: Edit the docs**

  - `README.md:247`: troque "Não há telemetria." por exatamente: "Nada mais sai do computador: a telemetria local, opcional e desligada por padrão (`EPROCIA_LOG`, veja [Log e telemetria](#-log-e-telemetria-opcional)), fica só no computador."
  - `CLAUDE.md`, no item "Update contracts", depois de "with `--apos-atualizar` the single-instance lock is deferred until the old one exits;": acrescente "the flag counts only when its pid is the new process's parent and the parent's image is `eprocia.exe` or `eprocia.anterior.exe` (`lerArranque`, `atualizacao.ConfirmarAposAtualizar`; if the process snapshot fails the flag is trusted, so an update never ends with no app open); otherwise it is a normal start, single-instance lock included;".
  - `CLAUDE.md`, no parágrafo **Update** de "Architecture", depois da frase sobre `mensagens.go`: "Config and watcher errors that reach the UI (`SaveConfig`, `SetTheme`, `watchdog:error`) are fixed pt-BR constants in `app.go`; the full error goes only to `slog`."
  - Capturas visuais: não precisa. Nenhuma tarefa muda layout nem componente visível; só o texto de avisos já existentes (o da recusa já é capturado pelo estado `atualizacao` do `eprocia-visual`, com a mesma frase).

- [ ] **Step 4: Run the full gate**

Run: `node --test frontend/*.test.mjs`, depois `pwsh scripts/build/gate.ps1` na raiz da worktree (saída em arquivo).
Expected: testes do frontend passam; gate termina com vet, `test -race`, `wails build` e smoke verdes. No relatório, registre que o reinício real com `--apos-atualizar` fica para o ensaio manual da skill `eprocia-release` (`-tags ensaio`) antes da próxima release.

- [ ] **Step 5: Commit**

```bash
git add README.md CLAUDE.md frontend/atualizacao.test.mjs
git commit -F <mensagem>   # docs: telemetria local na seção de atualização e origem do --apos-atualizar
```
