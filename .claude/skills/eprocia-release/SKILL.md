---
name: eprocia-release
description: Publicar uma release do eprocIA — subir a productVersion, ensaiar com tag rc, conferir nomes, SHA256SUMS e atestação, escrever as notas e publicar. Use quando o dono pedir uma release; só a sessão principal executa, nunca um subagente.
---

# Release do eprocIA

Quem compila é o `release.yml`, a partir da tag. Nada sai da máquina local. A publicação muda o que o `irm .../install.ps1 | iex` instala (`releases/latest`), então cada passo abaixo só roda com o pedido do dono.

## Antes

1. A `main` está verde (CI e security) no commit que vai virar a release.
2. `wails.json`: `info.productVersion` igual à base da tag (`0.4.1` para `v0.4.1` e para `v0.4.1-rc1`). O workflow recusa a tag se for diferente. A mudança entra por PR, junto com o README.
3. As notas cobrem tudo desde a última release **publicada** (`gh release list`): um rascunho que nunca foi publicado não tem notas (foi o caso da v0.4.0). Escreva num arquivo fora do repositório.
4. A primeira release que sai com a atualização pelo programa é a **0.5.0** (decisão d1 do dono, 2026-11-24): antes de subir a tag dela, confira que `wails.json` `info.productVersion` é `0.5.0`. A `main` ainda traz o recurso desligado em build com `productVersion` `0.0.0` ou vazia.

## Contratos que a release cumpre

O programa instalado procura e instala a partir do que a release publica, então estes dois contratos não mudam sem plano de compatibilidade (detalhe no `CLAUDE.md`, "Update contracts").

- **Assets e `SHA256SUMS.txt`:** a tag é `vX.Y.Z`; os assets são `eprocia-X.Y.Z-windows-amd64.exe`, `eprocia-X.Y.Z-windows-amd64-instalador.exe` e `SHA256SUMS.txt`, com linhas `<sha256> *<nome>` e as somas dos dois exe. O programa só instala o arquivo cujo tamanho, SHA-256, início `MZ` e `FileVersion` (campos numéricos) batem com a release. Uma release sem um desses assets aparece como "incompleta" e nada é instalado.
- **Protocolo de troca (entre versões):** `--apos-atualizar=<pid>` e `--limpar-anterior`; `%APPDATA%\eprocIA\atualizacao-sinal-<pid>`; `eprocia.anterior.exe`, `eprocia.falhou.exe`, `eprocia.exe.download-*`. A versão nova abre com esses argumentos, dá o sinal de vida em até 90 s e apaga `eprocia.anterior.exe` depois de a antiga sair. Não há versão anterior guardada (d5): voltar é `install.ps1 -Version`.
- **Retirar uma release não desfaz instalações:** quem já atualizou fica com a versão; a consulta só enxerga `releases/latest`. Se a retirada for por defeito, publique uma versão corrigida (`X.Y.Z+1`), que é o que o programa de quem atualizou vai oferecer.

## Ensaio

Tags `v*` não podem ser apagadas nem movidas (ruleset), as de ensaio também. Se o ensaio falhar, conserte e use `-rc2`.

```bash
git tag vX.Y.Z-rc1 <commit da main> && git push origin vX.Y.Z-rc1
gh run list --workflow release.yml --limit 1
gh run watch <id> --exit-status
```

O resultado é um rascunho marcado como pre-release. Baixe para uma pasta nova e vazia, fora do repositório, e não rode nada de dentro dela:

```bash
gh release download vX.Y.Z-rc1 --dir <pasta>
```

Confira:

- cinco arquivos: `eprocia-<versão>-windows-amd64.exe`, `eprocia-<versão>-windows-amd64-instalador.exe`, `install.ps1`, `LICENSE`, `SHA256SUMS.txt`;
- `SHA256SUMS.txt` com duas linhas no formato `<sha256 minúsculo> *<nome>`, fim de linha LF, só os dois exe, e as somas iguais às de `sha256sum <pasta>/*.exe`;
- `gh attestation verify <arquivo> -R jonyduque/eprocia --format json` com saída 0 para os dois exe e o `install.ps1`, e `sourceRepositoryRef` igual a `refs/tags/<tag>`. O `install.ps1` não muda entre releases, então a lista traz também atestações de tags antigas: procure a da tag atual;
- versão dos exe pelos campos numéricos (`[Diagnostics.FileVersionInfo]::GetVersionInfo(...).FileMajorPart` etc.). O `ProductVersion` do exe principal sai 0.0.0.0, e isso é esperado;
- `install.ps1` igual ao da `main` (`MSYS_NO_PATHCONV=1 git show origin/main:install.ps1 | sha256sum`) e sem BOM.

Depois apague só o rascunho, sem `--cleanup-tag`: `gh release delete vX.Y.Z-rc1 --yes`.

## Release

```bash
git tag vX.Y.Z <mesmo commit> && git push origin vX.Y.Z
```

Repita a conferência no rascunho novo. O exe principal sai com a mesma soma do ensaio (a tag não entra no binário); o instalador muda.

```bash
gh release edit vX.Y.Z --title "eprocIA vX.Y.Z" --notes-file <notas> --draft=false --latest
gh api repos/jonyduque/eprocia/releases/latest --jq .tag_name   # vX.Y.Z
curl -sL https://github.com/jonyduque/eprocia/releases/latest/download/SHA256SUMS.txt
```

## Ensaio manual da atualização

Faça uma vez antes da release que muda o recurso e, na 0.5.0, antes de publicá-la (decisão d4, 2026-11-24): **só releases rc e uma conta** de usuário do Windows. A consulta de produção ignora pré-releases; por isso o ensaio usa um build local `-tags ensaio` (nunca publicado), que lê `EPROCIA_ENSAIO_TAG` (uma rc fixa, `vX.Y.Z-rcN`) e, com `EPROCIA_ENSAIO_SEM_SINAL=1`, ignora o sinal de vida para provar a reversão. Só o dono roda; nada disso vai para um subagente. Ordem:

1. **Preparar a conta:** anote o que está instalado e como (por usuário em `%LOCALAPPDATA%\Programs\eprocIA` ou NSIS, com a versão); feche o eprocIA pela bandeja ("Sair", não só a janela) e só então copie `%APPDATA%\eprocIA` inteira para fora (o ensaio usa a mesma pasta de dados e o mesmo registro da instalação real da conta). Com a cópia feita, edite o `config.json` da conta para monitorar uma pasta vazia criada para o ensaio (o build de ensaio lê a config real e varre a pasta monitorada ao abrir: ZIPs reais pendentes seriam processados e o `falhas.json` mudaria). No ensaio, só se arrasta um ZIP sintético (dados fictícios), nunca um processo real.
2. **Publicar a rc:** tag `vX.Y.Z-rc1` com `productVersion` `X.Y.Z`; confira o rascunho como na seção "Ensaio"; publique como pré-release com `gh release edit vX.Y.Z-rc1 --draft=false --prerelease` (a API sem token não vê rascunho; `releases/latest`, o `install.ps1` e o recurso em produção continuam na última estável).
3. **Versão N do ensaio:** build local `wails build -clean -nsis -tags ensaio`. Na primeira release com o recurso (0.5.0), do mesmo commit da rc, com a `productVersion` do `wails.json` trocada só na cópia local (sem commit) para `0.4.99`; reverta o `wails.json` e o ruído do build depois. Da `0.5.1` em diante, N é o código da última estável publicada, não o commit novo com rótulo antigo: `git worktree add <pasta fora do repositório> vA.B.C`, `npm ci` e `npm run build` em `frontend` dela, e o mesmo `wails build -clean -nsis -tags ensaio` lá (a `productVersion` já é `A.B.C`; a 0.5.0 já tem `ensaio_on.go`); assim o ensaio prova o protocolo entre duas versões publicadas (o `Reiniciar` de uma passando `--apos-atualizar` e `--limpar-anterior` à outra). Remova a worktree no fim. Com `EPROCIA_ENSAIO_TAG=vX.Y.Z-rc1` no ambiente do usuário, o build de ensaio consulta essa rc fixa; o exe publicado não tem esse código.
4. **Instalação por usuário** (instalar e reiniciar são dois cliques, d5): `install.ps1 -From <exe de ensaio>`; abra; primeiro com `EPROCIA_ENSAIO_SEM_SINAL=1`: procura pelo botão, "Atualizar", "Versão X instalada" com `eprocia.anterior.exe` (N) e `eprocia.exe` (rc) na pasta e o programa ainda em uso (arrastar um ZIP sintético funciona), "Reiniciar agora" e, depois de 90 s, a reversão ("A versão X não abriu..."; `eprocia.exe` volta a N, `eprocia.falhou.exe` fica, não sobra `eprocia.anterior.exe`). Tire a variável, reabra e repita: instalação, "Reiniciar agora", sinal de vida, primeiro arranque do exe novo medido abaixo de 45 s, metade do prazo de 90 s (`PrazoSinal`), com o antivírus ligado (se passar, o prazo é revisto antes da release), `eprocia.anterior.exe` apagado poucos segundos depois de o antigo sair, sem atrasar a janela; reinício com a janela na bandeja (o processo novo nasce com `--tray`, dá o sinal pelo `startup` sem `OnDomReady` e não aparece na tela); "Reiniciar agora" com a fila ocupada ("Reiniciar quando a fila terminar", e o reinício acontece quando ela esvazia); arrastar ZIP depois do reinício. Depois, reinstale N e confira o caminho sem reinício: "Atualizar", **não** clicar em "Reiniciar agora", "Sair" pela bandeja e abrir pelo atalho: abre a rc, `eprocia.anterior.exe` some em segundo plano e a `DisplayVersion` da chave HKCU é a da rc.
5. **Instalação NSIS:** `install.ps1 -Uninstall`, instale pelo instalador de ensaio (páginas do instalador e do desinstalador todas em português, atalho da área de trabalho marcado por padrão e desmarcável, d2 e d3); procura pelo botão; UAC recusado (nada muda) e aceito; "Abrir o eprocIA" marcado (o programa aberto roda sem administrador e aceita arrastar um ZIP do Explorer) e, reinstalando N, desmarcado; o "Tentar de novo" do NSIS passando rápido pelas páginas; chave HKLM `eprociaeprocia` com a `DisplayVersion` nova; `FileVersion` do instalador igual à base da tag.
6. **`install.ps1` com versão já instalada (d6)**, com a rc publicada (passo 2) e a conta preparada (passo 1); nenhum agente faz isto:
   1. **Por usuário:** com N instalado por `install.ps1 -From <exe de ensaio>` e aberto ocioso, `install.ps1 -Version vX.Y.Z-rc1`: pergunta a ação e depois "Encerrar o eprocIA aberto?"; "Cancelar" em qualquer das duas não muda nada (app aberto, `eprocia.exe` = N); repita e escolha "Encerrar": o app só fecha depois do download, a rc fica instalada e abre. Repita derrubando a rede no meio do download: o app continua aberto. Repita com um ZIP sintético em OCR: aparece o aviso de processamento em curso. Repita por `[scriptblock]::Create` com o script da branch (o filho usa o texto da branch, não o da `main`) e com a entrada redirecionada (`cmd /c "powershell.exe -File install.ps1 -Version vX.Y.Z-rc1 < NUL"`), sem `-Yes` (recusa com a mensagem de console) e com `-Yes` (segue e encerra sem perguntar).
   2. **Para todos:** com N instalado pelo instalador de ensaio (passo 5) e aberto, `install.ps1 -Version vX.Y.Z-rc1` num terminal comum: pergunta a ação e o encerramento e pede UAC. UAC recusado: "Permissao de administrador recusada. Nada foi alterado.", app ainda aberto, `DisplayVersion` HKLM = N. Aceito: app fechado pelo filho, `eprocia.exe` em Arquivos de Programas = rc, `DisplayVersion` HKLM = rc, atalho da área de trabalho como estava, programa reaberto **sem** administrador (arrastar um ZIP do Explorer funciona), nenhuma cópia por usuário criada, nenhuma pasta `eprocia-elevar-*` sobrando em `%ProgramData%` nem no `%TEMP%`. Durante o UAC, confira que a pasta em `%ProgramData%` ainda não existe; durante a instalação, que a ACL dela (`icacls`) só tem Administradores e SYSTEM com controle total e o usuário com leitura, sem herança, e que o dono é Administradores. Repita com caminhos longos: `$env:TEMP` apontado (só naquele terminal) para uma pasta de mais de 200 caracteres: o UAC aparece e a ação vai até o fim. Abra o eprocIA entre a pergunta e o UAC (sem `-Yes`): o filho para com "O eprocIA foi aberto depois da pergunta" e nada muda; com `-Yes` do usuário, ele é encerrado. Repita de um terminal elevado: sem UAC, sem tarefa agendada, programa aberto sem administrador. `install.ps1 -Uninstall` com só a instalação para todos: UAC, e a pasta e a chave HKLM somem.
   3. **Terminal elevado com instalação por usuário:** `install.ps1` pergunta "Rodar como <usuario> numa janela oculta?"; "Cancelar" não muda nada; "Continuar" instala no perfil do usuário, como antes.
   4. **`-DryRun`** em cada um dos três casos: mostra o alvo, as perguntas ("Perguntaria: ...") e a elevação, e nada muda (`DisplayVersion`, pastas e app aberto como antes).
7. **Desfazer:** feche o eprocIA pela bandeja ("Sair"); apague `EPROCIA_ENSAIO_TAG` e `EPROCIA_ENSAIO_SEM_SINAL` do ambiente do usuário; desinstale o que o ensaio pôs; apague a `%APPDATA%\eprocIA` do ensaio inteira (com `atualizacao-sinal-*`, config e `falhas.json` do ensaio) e ponha no lugar a cópia do passo 1, inteira; apague `%LOCALAPPDATA%\eprocIA\atualizacao`; só então reinstale a última estável do mesmo tipo anotado no passo 1 (por usuário: `irm .../install.ps1 | iex`, que abre o programa já com os dados devolvidos; NSIS: o instalador da última estável); `gh release delete vX.Y.Z-rc1 --yes` (sem `--cleanup-tag`: a tag é imutável).

**Recomendação:** publique uma 0.5.1 logo depois da 0.5.0. O protocolo entre duas versões publicadas com o recurso (o `Reiniciar` da 0.5.0 abrindo a 0.5.1 com `--limpar-anterior`) só se ensaia a partir dela.

## Depois

- Tire do README o que só valia antes da release (notas do tipo "a main está à frente da versão publicada").
- Não rode o `install.ps1` real para testar: ele encerra e substitui o eprocIA instalado do dono.
